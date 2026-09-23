package module8workers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// ------------------------------ IMPORTED FUNCTIONS -------------------------------------------
// read any file
func readAnyFile(f string) string {
	fileContent, readError := os.ReadFile(f)
	if readError != nil {
		//logAnyError("Error reading file "+f, readError, 3) //http -> /api
	}
	return string(fileContent)
}

// read config json file
func readConfigFile(cf string) Config {
	var configContent Config
	jsonerror := json.Unmarshal([]byte(readAnyFile(cf)), &configContent)
	if jsonerror != nil {
		//logAnyError("Error reading config file: "+cf, jsonerror, 3)
	}
	return configContent
}

// verifyAPIKey
func verifyAPIKey(inkey string) (isthesame bool) {
	if mC.Module8ApiKey == inkey {
		isthesame = true
	}
	return isthesame
}

// if development accept untrusted certs
func setInsecureRequest() (client *http.Client) {
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // best add used certs exception
		},
	}
	return client
}

// returns string suitable for json format
func escapeComma(s string) string {
	s = strings.ReplaceAll(s, "'", "¤")
	s = strings.ReplaceAll(s, "\"", "¶")
	return strings.ReplaceAll(s, ",", "§")

}

// returns string in front-end format
func unescapeComma(s string) string {
	s = strings.ReplaceAll(s, "¤", "'")
	s = strings.ReplaceAll(s, "¶", "\"")
	return strings.ReplaceAll(s, "§", ",")
}

// decodeB64String
func decodeB64String(encoded string) (decoded []byte) {
	decoded, err := b64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Fatalln("Error de-base64ing string (" + encoded + "): " + err.Error())
	}
	return decoded
}

// aes decrypt with mC.CookieEncryptionKey
func decryptCookie(enc string) (string, error) {
	data := decodeB64String(enc)
	block, err := aes.NewCipher(decodeB64String(mC.CookieEncryptionKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", err
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func decryptLogWithGPG(encrypted string) (string, error) {
	privKeyFile, err := os.Open("../../" + mC.PrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("could not open private key: %w", err)
	}
	defer privKeyFile.Close()

	entities, err := openpgp.ReadArmoredKeyRing(privKeyFile)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	for _, entity := range entities {
		if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
			err := entity.PrivateKey.Decrypt([]byte(mC.LogCypherKeyPassphrase))
			if err != nil {
				return "", fmt.Errorf("cannot decrypt private key: %w", err)
			}
		}

		for _, sub := range entity.Subkeys {
			if sub.PrivateKey != nil && sub.PrivateKey.Encrypted {
				err := sub.PrivateKey.Decrypt([]byte(mC.LogCypherKeyPassphrase))
				if err != nil {
					return "", fmt.Errorf("cannot decrypt subkey: %w", err)
				}
			}
		}
	}

	// now is readeable
	debase64d, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}
	md, err := openpgp.ReadMessage(bytes.NewReader(debase64d), entities, nil, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}
	plaintext, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}

	return string(plaintext), nil
}

func getPDFCertificatePassword() (string, error) {
	password := strings.TrimSpace(mC.PDFCertificatePassword)

	return password, nil
}

// get time since unix clock start
func getUnixTimestamp() int64 {
	return time.Now().Unix()
}

func getTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

// check if user role is allowed to acces route
func roleAllowed(role string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	return false
}

// extract content from cypher cookie
func parseAuthCookie(cipherText string, allowedRoles []string) (*AuthCookieClaims, bool) {
	var claims AuthCookieClaims
	plain, err := decryptCookie(cipherText)
	if err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(plain), &claims); err != nil {
		return nil, false
	}
	if claims.Exp < getUnixTimestamp() {
		return nil, false
	}
	if !roleAllowed(claims.Role, allowedRoles) {
		return nil, false
	}
	return &claims, true
}

// GET user_id by username
func jsonUsernameID(user string) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id, role",
		"condition": fmt.Sprintf("username = '%s'", user),
	}
	return query
}

func hashString(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	hashedBytes := hash.Sum(nil)
	return hex.EncodeToString(hashedBytes)
}

func jsonUsernames() map[string]interface{} {
	query := map[string]interface{}{
		"table":   "users",
		"columns": "username",
	}
	return query
}

// get hash user string return user unhashed
func getUsernameByHash(userHash string, apiKey string, client *http.Client, w http.ResponseWriter) string {
	query := jsonUsernames()
	jsonData, _ := json.Marshal(query)
	results := getReq(jsonData, apiKey, client, w)
	defer results.Body.Close()
	var rows []map[string]interface{}
	_ = json.NewDecoder(results.Body).Decode(&rows)
	for _, row := range rows {
		uname, _ := row["username"].(string)
		if hashString(uname) == userHash {
			return uname
		}
	}
	return "superadmin"
}

// returns user (who is using the application) id, username and role
func getUserInfo(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) (int, string, string) {
	// get user's cookie
	var user string
	if c, err := r.Cookie("CRMINTRATOOLS"); err == nil {
		if claims, ok := parseAuthCookie(c.Value, nil); ok {
			user = getUsernameByHash(claims.Sub, apiKey, client, w)
		}
	}
	// get user's ID
	query := jsonUsernameID(user)
	jsonData, _ := json.Marshal(query)
	respID := getReq(jsonData, apiKey, client, w)
	if respID == nil {
		createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 130, apiKey, client, w)
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return 0, "", ""
	}
	defer respID.Body.Close()
	bodyB, _ := io.ReadAll(respID.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(bodyB, &result); err != nil {
		http.Error(w, "Error parsing modification JSON", http.StatusBadRequest)
		createLog(fmt.Sprintf("Error parsing modification JSON for user %s: %v", user, err), 1, apiKey, client, w)
		return 0, "", ""
	}
	if len(result) == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		if user == "superadmin" {
			createLog(fmt.Sprintf("Superadmin accessing module 8 from IP %s", r.RemoteAddr), 0, apiKey, client, w)
			return 0, "superadmin", "superadmin"
		}
		createLog(fmt.Sprintf("User %s not found for cookie", user), 130, apiKey, client, w)
		return 0, "", ""
	}
	idRaw := result[0]["people_id"]
	var userRow int
	roleRow := result[0]["role"].(string)
	switch id := idRaw.(type) {
	case float64:
		userRow = int(id)
	default:
		http.Error(w, "Invalid user ID type", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Invalid user ID type for user %s", user), 130, apiKey, client, w)
		return 0, "", ""
	}
	//returns user_id, username and user_role
	return userRow, user, roleRow
}

// --------------------------------------------- LOGS --------------------------------------------------
// cypher
// Cifra un log usando la clave pública GPG del sistema
func encryptLogWithGPG(logContent string) string {
	pubKeyFile, err := os.Open("../../GpgKeys/public.key.asc")
	if err != nil {
		fmt.Println("Error abriendo clave pública:", err)
		return ""
	}
	defer pubKeyFile.Close()

	entityList, err := openpgp.ReadArmoredKeyRing(pubKeyFile)
	if err != nil {
		fmt.Println("Error leyendo keyring:", err)
		return ""
	}

	var buf bytes.Buffer
	writer, err := openpgp.Encrypt(&buf, entityList, nil, nil, nil)
	if err != nil {
		fmt.Println("Error cifrando:", err)
		return ""
	}
	_, err = writer.Write([]byte(logContent))
	if err != nil {
		fmt.Println("Error escribiendo mensaje cifrado:", err)
		return ""
	}
	writer.Close()

	return buf.String()
}

// BASE64 ENCODE
func base64_encode(input string) string {

	encoded := b64.StdEncoding.EncodeToString([]byte(input))

	return encoded

}

func createLog(logMessage string, code int, apiKey string, client *http.Client, w http.ResponseWriter) {

	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	} else {
		encryptedMessage := encryptLogWithGPG(logMessage)
		encodedMessage := base64_encode(encryptedMessage)

		payload := map[string]interface{}{
			"msg":  encodedMessage,
			"code": code,
		}

		jsonData, _ := json.Marshal(payload)

		resp := sendReqLog(jsonData, apiKey, client, w)
		defer resp.Body.Close()
	}

}

func sendReqLog(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	reqInsert, _ := http.NewRequest("POST", "https://localhost:"+mC.ServerPort+"/module/api/log", bytes.NewBuffer(jsonData))
	reqInsert.Header.Set("Content-Type", "application/json")
	reqInsert.Header.Set("Authorization", "Bearer "+apiKey)
	respInsert, err := client.Do(reqInsert)
	if err != nil {
		fmt.Println("Error inserting log:", err)
		http.Error(w, "Failed to insert log", http.StatusInternalServerError)
	}
	return respInsert
}

// -----------------------------------------------------------------------------------------------------------------
func sendReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	url := "https://localhost:" + mC.ServerPort + "/module/api/db"

	reqInsert, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creando la solicitud HTTP:", err)
		http.Error(w, "Internal request creation error", http.StatusInternalServerError)
		return nil
	}

	reqInsert.Header.Set("Content-Type", "application/json")
	reqInsert.Header.Set("Authorization", "Bearer "+apiKey)

	respInsert, err := client.Do(reqInsert)
	if err != nil {
		fmt.Printf("Error al enviar la petición POST a %s: %v\n", url, err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return nil
	}
	return respInsert
}

// ------------------------------- http requests ------------------------------------------
func getReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "GET"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReq(modifiedJSON, apiKey, client, w)
}
func postReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "POST"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReq(modifiedJSON, apiKey, client, w)
}
func putReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "PUT"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReq(modifiedJSON, apiKey, client, w)
}
func deleteReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "DELETE"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReq(modifiedJSON, apiKey, client, w)
}

// -------------------------------------------- MSSQL -------------------------------------------------

func sendReqMSSQL(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) *http.Response {
	url := "https://localhost:" + mC.ServerPort + "/module/api/dbmssql"

	fmt.Println("---- INTERNAL REQUEST ----")
	fmt.Println("URL:", url)
	fmt.Println("API KEY:", apiKey)

	reqInsert, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("❌ Error creando request MSSQL:", err)
		return nil
	}

	reqInsert.Header.Set("Content-Type", "application/json")
	reqInsert.Header.Set("Authorization", "Bearer "+apiKey)

	// copiar cookies de sesión
	for _, c := range r.Cookies() {
		fmt.Println("COPIANDO COOKIE:", c.Name, "=", c.Value)
		reqInsert.AddCookie(c)
	}

	respInsert, err := client.Do(reqInsert)
	if err != nil {
		fmt.Println("❌ Error en client.Do():", err)
		return nil
	}

	fmt.Println("---- INTERNAL REQUEST END ----")
	return respInsert
}

func getReqMSSQL(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "GET"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReqMSSQL(modifiedJSON, apiKey, client, w, r)
}

func postReqMSSQL(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "POST"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReqMSSQL(modifiedJSON, apiKey, client, w, r)
}

func putReqMSSQL(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "PUT"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReqMSSQL(modifiedJSON, apiKey, client, w, r)
}

func deleteReqMSSQL(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "DELETE"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReqMSSQL(modifiedJSON, apiKey, client, w, r)
}

func handleCheckRole(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, _, userRole := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"role": userRole,
	})
}

// connects to host and sends email
func sendEmail(to string, subject string, body string) error {
	from := mC.SmtpUser
	headers := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + subject + "\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\n\n"

	msg := headers + body
	conn, _ := net.Dial("tcp", mC.SmtpHost)
	client, _ := smtp.NewClient(conn, "crm.cat")
	defer client.Quit()
	_ = client.Mail(from)
	_ = client.Rcpt(to)
	wc, _ := client.Data()
	_, _ = wc.Write([]byte(msg))
	_ = wc.Close()
	return nil
}

func sendEmailWithAttachments(to string, subject string, body string, attachmentPaths []string) error {
	from := mC.SmtpUser
	boundary := "crm_boundary_" + strconv.FormatInt(time.Now().UnixNano(), 10)

	var msg bytes.Buffer

	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/mixed; boundary=" + boundary + "\r\n\r\n")

	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(body + "\r\n")

	for _, attachmentPath := range attachmentPaths {
		attachmentPath = strings.TrimSpace(attachmentPath)
		if attachmentPath == "" {
			continue
		}

		fileBytes, err := os.ReadFile(attachmentPath)
		if err != nil {
			return fmt.Errorf("failed to read attachment %s: %w", attachmentPath, err)
		}

		filename := filepath.Base(attachmentPath)
		mimeType := http.DetectContentType(fileBytes)
		encoded := b64.StdEncoding.EncodeToString(fileBytes)

		msg.WriteString("--" + boundary + "\r\n")
		msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, filename))
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filename))

		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			msg.WriteString(encoded[i:end] + "\r\n")
		}
	}

	msg.WriteString("--" + boundary + "--\r\n")

	conn, err := net.Dial("tcp", mC.SmtpHost)
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, "crm.cat")
	if err != nil {
		return err
	}
	defer client.Quit()

	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	wc, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(msg.Bytes()); err != nil {
		_ = wc.Close()
		return err
	}
	return wc.Close()
}

func asBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case int:
		return val != 0
	case int64:
		return val != 0
	case string:
		s := strings.TrimSpace(strings.ToLower(val))
		return s == "1" || s == "true" || s == "yes"
	default:
		return false
	}
}

// funció per comprovar que si agafem l'email personal del treballador, aquest sigui un email educatiu
func isAllowedUserDomain(email string) bool {
	allowedSuffixes := []string{
		"crm.cat",
		"uab.cat",
		"ub.edu",
		"upc.edu",
		"icrea.cat",
	}

	at := strings.LastIndex(email, "@")
	if at == -1 || at == len(email)-1 {
		return false
	}

	domain := email[at+1:]

	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(strings.ToLower(domain), strings.ToLower(suffix)) {
			return true
		}
	}

	return false
}

func sendNotification(title, content string, to int, apiKey string, client *http.Client, w http.ResponseWriter) error {

	today := time.Now().Format("2006-01-02 15:04")

	notification := map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Budgeting, %s, %s, 304, %s", title, content, today),
	}
	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("error marshaling notification payload: %w", err)
	}

	postResp := postReq(jsonData, apiKey, client, w)
	if postResp == nil {
		return fmt.Errorf("error inserting notification: empty response")
	}
	defer postResp.Body.Close()
	if postResp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("error inserting notification: %s", strings.TrimSpace(string(body)))
	}

	// Get last inserted notification ID
	query := map[string]interface{}{
		"table":     "notifications",
		"columns":   "id",
		"condition": "user_id = 304 AND title = '" + title + "' AND content = '" + content + "' AND created_at = '" + today + "'",
	}
	jsonData, err = json.Marshal(query)
	if err != nil {
		return fmt.Errorf("error marshaling notification query payload: %w", err)
	}

	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("error fetching notification ID: empty response")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error fetching notification ID: %s", strings.TrimSpace(string(body)))
	}

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error decoding notification query response: %v\n", err)
		return err
	}
	if len(result) == 0 {
		fmt.Println("No notification found after insertion")
		return nil
	}

	idValue, ok := result[0]["id"].(float64)
	if !ok {
		return fmt.Errorf("invalid notification id type")
	}
	notificationID := int(idValue)

	notificationUser := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read",
		"value":   fmt.Sprintf("%d, %d, 1", notificationID, to),
	}
	jsonData, err = json.Marshal(notificationUser)
	if err != nil {
		return fmt.Errorf("error marshaling notification_user payload: %w", err)
	}

	postUserResp := postReq(jsonData, apiKey, client, w)
	if postUserResp == nil {
		return fmt.Errorf("error inserting notification_user: empty response")
	}
	defer postUserResp.Body.Close()
	if postUserResp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(postUserResp.Body)
		return fmt.Errorf("error inserting notification_user: %s", strings.TrimSpace(string(body)))
	}

	return nil
}

type PersonInfo struct {
	ID      int
	Name    string
	Surname string
	Email   string
}

type ProjectInfo struct {
	ID   string
	Name string
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func getPersonInfoByID(personID int, apiKey string, client *http.Client, w http.ResponseWriter) (*PersonInfo, error) {
	payload := map[string]interface{}{
		"table":     "people",
		"columns":   "id, name, surname, secondSurname, crm_email, user_email",
		"condition": fmt.Sprintf("id = %d", personID),
	}
	jsonPayload, _ := json.Marshal(payload)

	resp := getReq(jsonPayload, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("nil response getting person info for id %d", personID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d getting person info for id %d", resp.StatusCode, personID)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decoding person info: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("person id %d not found", personID)
	}

	email := ""
	if rows[0]["crm_email"] != nil && toString(rows[0]["crm_email"]) != "" {
		email = toString(rows[0]["crm_email"])
	} else if rows[0]["user_email"] != nil && toString(rows[0]["user_email"]) != "" && isAllowedUserDomain(toString(rows[0]["user_email"])) {
		email = toString(rows[0]["user_email"])
	}

	surname := toString(rows[0]["surname"])
	secondSurname := ""
	if rows[0]["secondSurname"] != nil {
		secondSurname = toString(rows[0]["secondSurname"])
	}

	fullSurname := surname
	if secondSurname != "" {
		fullSurname += " " + secondSurname
	}

	row := rows[0]
	p := &PersonInfo{
		ID:      personID,
		Name:    unescapeComma(toString(row["name"])),
		Surname: unescapeComma(fullSurname),
		Email:   email,
	}
	return p, nil
}

func getProjectInfoByID(projectID string, apiKey string, client *http.Client, w http.ResponseWriter) (*ProjectInfo, error) {
	if isContractProgramProjectID(projectID) {
		return &ProjectInfo{
			ID:   contractProgramProjectID,
			Name: contractProgramProjectName,
		}, nil
	}

	payload := map[string]interface{}{
		"table":     "projects",
		"columns":   "id, short_name",
		"condition": fmt.Sprintf("id = '%s'", projectID),
	}
	jsonPayload, _ := json.Marshal(payload)

	resp := getReq(jsonPayload, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("nil response getting project info for id %s", projectID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d getting project info for id %s", resp.StatusCode, projectID)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decoding project info: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("project id %s not found", projectID)
	}

	row := rows[0]
	return &ProjectInfo{
		ID:   toString(row["id"]),
		Name: unescapeComma(toString(row["short_name"])),
	}, nil
}

func getRequestRejectionSummary(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) ([]string, []string, error) {
	query := map[string]interface{}{
		"table": `budget_parts bp
			LEFT JOIN projects p ON bp.project_id = p.id`,
		"columns":   "bp.category_id, bp.project_id, p.short_name, bp.purpose, bp.observations, bp.travel_fromPlace, bp.wherePlace, bp.fromDay, bp.untilDay, bp.registration_type, bp.equipment_category, bp.other_price",
		"condition": fmt.Sprintf("bp.id_combined = %d", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, nil, fmt.Errorf("failed to get request parts")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("unexpected status getting request parts: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, nil, fmt.Errorf("failed to decode request parts: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}

	projectNames := []string{}
	projectSeen := map[string]bool{}
	summaryParts := []string{}

	for _, row := range rows {
		projectName := unescapeComma(toString(row["short_name"]))
		if projectName != "" && !projectSeen[projectName] {
			projectNames = append(projectNames, projectName)
			projectSeen[projectName] = true
		}

		categoryKey := CATEGORY_ID_TO_KEY[asInt(row["category_id"])]
		switch categoryKey {
		case "travel":
			var parts []string
			parts = append(parts, "Travel request")
			if toString(row["travel_fromPlace"]) != "" || toString(row["wherePlace"]) != "" {
				parts = append(parts, fmt.Sprintf("route: %s → %s", unescapeComma(toString(row["travel_fromPlace"])), unescapeComma(toString(row["wherePlace"]))))
			}
			if toString(row["fromDay"]) != "" || toString(row["untilDay"]) != "" {
				parts = append(parts, fmt.Sprintf("dates: %s to %s", normalizeDateString(toString(row["fromDay"])), normalizeDateString(toString(row["untilDay"]))))
			}
			if toString(row["purpose"]) != "" {
				parts = append(parts, fmt.Sprintf("purpose: %s", unescapeComma(toString(row["purpose"]))))
			}
			summaryParts = append(summaryParts, strings.Join(parts, " | "))
		case "registration":
			var parts []string
			parts = append(parts, "Registration request")
			if toString(row["purpose"]) != "" {
				parts = append(parts, fmt.Sprintf("event: %s", unescapeComma(toString(row["purpose"]))))
			}
			if toString(row["observations"]) != "" {
				parts = append(parts, fmt.Sprintf("observations: %s", unescapeComma(toString(row["observations"]))))
			}
			summaryParts = append(summaryParts, strings.Join(parts, " | "))
		case "accommodation":
			var parts []string
			parts = append(parts, "Accommodation request")
			if toString(row["wherePlace"]) != "" {
				parts = append(parts, fmt.Sprintf("where: %s", unescapeComma(toString(row["wherePlace"]))))
			}
			if toString(row["fromDay"]) != "" || toString(row["untilDay"]) != "" {
				parts = append(parts, fmt.Sprintf("dates: %s to %s", normalizeDateString(toString(row["fromDay"])), normalizeDateString(toString(row["untilDay"]))))
			}
			if toString(row["purpose"]) != "" {
				parts = append(parts, fmt.Sprintf("purpose: %s", unescapeComma(toString(row["purpose"]))))
			}
			summaryParts = append(summaryParts, strings.Join(parts, " | "))
		case "equipment":
			var parts []string
			parts = append(parts, "Equipment request")
			if toString(row["equipment_category"]) != "" {
				parts = append(parts, fmt.Sprintf("category: %s", toString(row["equipment_category"])))
			}
			if toString(row["observations"]) != "" {
				parts = append(parts, fmt.Sprintf("description: %s", unescapeComma(toString(row["observations"]))))
			}
			summaryParts = append(summaryParts, strings.Join(parts, " | "))
		case "other":
			var parts []string
			parts = append(parts, "Other expense request")
			if toString(row["observations"]) != "" {
				parts = append(parts, fmt.Sprintf("description: %s", unescapeComma(toString(row["observations"]))))
			}
			if toString(row["other_price"]) != "" {
				parts = append(parts, fmt.Sprintf("price range: %s", parsePriceRange(toString(row["other_price"]))))
			}
			summaryParts = append(summaryParts, strings.Join(parts, " | "))
		}
	}

	return projectNames, summaryParts, nil
}

func buildCategorySummary(category string, req RequestPayload) string {
	switch category {
	case "travel":
		var parts []string
		parts = append(parts, "Travel request")
		if req.TravelFrom != "" || req.TravelTo != "" {
			parts = append(parts, fmt.Sprintf("route: %s → %s", unescapeComma(toString(req.TravelFrom)), unescapeComma(toString(req.TravelTo))))
		}
		if req.TravelSince != "" || req.TravelUntil != "" {
			parts = append(parts, fmt.Sprintf("dates: %s to %s", req.TravelSince, req.TravelUntil))
		}
		if req.TravelPurpose != "" {
			parts = append(parts, fmt.Sprintf("purpose: %s", unescapeComma(toString(req.TravelPurpose))))
		}
		return strings.Join(parts, " | ")

	case "registration":
		var parts []string
		parts = append(parts, "Registration request")
		if req.RegistrationEvent != "" {
			parts = append(parts, fmt.Sprintf("event: %s", unescapeComma(toString(req.RegistrationEvent))))
		}
		if req.RegistrationStartDay != "" || req.RegistrationEndDay != "" {
			parts = append(parts, fmt.Sprintf("dates: %s to %s", req.RegistrationStartDay, req.RegistrationEndDay))
		}
		if req.RegistrationObservations != "" {
			parts = append(parts, fmt.Sprintf("observations: %s", unescapeComma(toString(req.RegistrationObservations))))
		}
		return strings.Join(parts, " | ")

	case "accommodation":
		var parts []string
		parts = append(parts, "Accommodation request")
		if req.AccomodationWhere != "" {
			parts = append(parts, fmt.Sprintf("where: %s", unescapeComma(toString(req.AccomodationWhere))))
		}
		if req.AccomodationSince != "" || req.AccomodationUntil != "" {
			parts = append(parts, fmt.Sprintf("dates: %s to %s", req.AccomodationSince, req.AccomodationUntil))
		}
		if req.AccomodationPurpose != "" {
			parts = append(parts, fmt.Sprintf("purpose: %s", unescapeComma(toString(req.AccomodationPurpose))))
		}
		return strings.Join(parts, " | ")

	case "other":
		var parts []string
		parts = append(parts, "Other expense request")
		if req.OtherDescription != "" {
			parts = append(parts, fmt.Sprintf("description: %s", unescapeComma(toString(req.OtherDescription))))
		}
		if req.OtherPriceRange != "" {
			parts = append(parts, fmt.Sprintf("price range: %s", parsePriceRange(req.OtherPriceRange)))
		}
		return strings.Join(parts, " | ")

	case "equipment":
		var parts []string
		parts = append(parts, "Equipment request")
		if req.EquipmentCategory != "" {
			parts = append(parts, fmt.Sprintf("category: %s", req.EquipmentCategory))
		}
		if req.EquipmentDescription != "" {
			parts = append(parts, fmt.Sprintf("description: %s", unescapeComma(toString(req.EquipmentDescription))))
		}
		if req.EquipmentPriceRange != "" {
			parts = append(parts, fmt.Sprintf("price range: %s", parsePriceRange(req.EquipmentPriceRange)))
		}
		return strings.Join(parts, " | ")
	}
	return "Budget request submitted"
}

func buildGlobalRequestSummary(req RequestPayload) string {
	var summaries []string
	for _, cat := range req.Categories {
		summaries = append(summaries, buildCategorySummary(cat, req))
	}
	return strings.Join(summaries, " || ")
}

type RequestMailTarget struct {
	Category  string
	ProjectID string
	IPID      int
	Summary   string
}

func extractMailTargets(req RequestPayload) []RequestMailTarget {
	var targets []RequestMailTarget

	for _, cat := range req.Categories {
		switch cat {
		case "travel":
			targets = append(targets, RequestMailTarget{
				Category:  "travel",
				ProjectID: req.TravelProject,
				IPID:      req.TravelIP,
				Summary:   buildCategorySummary("travel", req),
			})
		case "registration":
			targets = append(targets, RequestMailTarget{
				Category:  "registration",
				ProjectID: req.RegistrationProject,
				IPID:      req.RegistrationIP,
				Summary:   buildCategorySummary("registration", req),
			})
		case "accommodation":
			targets = append(targets, RequestMailTarget{
				Category:  "accommodation",
				ProjectID: req.AccomodationProject,
				IPID:      req.AccomodationIP,
				Summary:   buildCategorySummary("accommodation", req),
			})
		case "other":
			targets = append(targets, RequestMailTarget{
				Category:  "other",
				ProjectID: req.OtherProject,
				IPID:      req.OtherIP,
				Summary:   buildCategorySummary("other", req),
			})
		case "equipment":
			targets = append(targets, RequestMailTarget{
				Category:  "equipment",
				ProjectID: req.EquipmentProject,
				IPID:      req.EquipmentIP,
				Summary:   buildCategorySummary("equipment", req),
			})
		}
	}

	return targets
}

func getProjectsUsersForNotifications(apiKey string, client *http.Client, w http.ResponseWriter) ([]int, error) {
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": "access_projects = 1",
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get projects users")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("database error getting projects users: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("failed to decode projects users: %w", err)
	}

	seen := make(map[int]bool)
	var userIDs []int
	for _, row := range rows {
		id := asInt(row["people_id"])
		if id > 0 && !seen[id] {
			seen[id] = true
			userIDs = append(userIDs, id)
		}
	}

	return userIDs, nil
}

func getRequestResearcherID(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (int, error) {
	query := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "people_id",
		"condition": fmt.Sprintf("id = %d", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return 0, fmt.Errorf("failed to get request researcher")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("database error getting request researcher: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return 0, fmt.Errorf("failed to decode request researcher: %w", err)
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("request researcher not found")
	}

	return asInt(rows[0]["people_id"]), nil
}

func getRequestIPRecipients(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) ([]rejectionRecipientInfo, error) {
	query := map[string]interface{}{
		"table":     "budget_parts bp LEFT JOIN people p ON bp.ip_id = p.id",
		"columns":   "bp.ip_id, p.name, p.surname, p.crm_email, p.user_email",
		"condition": fmt.Sprintf("bp.id_combined = %d", combinedID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get IP recipients")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status getting IP recipients: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("failed to decode IP recipients: %w", err)
	}

	seen := make(map[int]bool)
	recipients := make([]rejectionRecipientInfo, 0)

	for _, row := range rows {
		ipID := asInt(row["ip_id"])
		if ipID == 0 || seen[ipID] {
			continue
		}
		seen[ipID] = true

		email := ""
		if toString(row["crm_email"]) != "" {
			email = toString(row["crm_email"])
		} else if isAllowedUserDomain(toString(row["user_email"])) {
			email = toString(row["user_email"])
		}

		recipients = append(recipients, rejectionRecipientInfo{
			ID:      ipID,
			Name:    unescapeComma(toString(row["name"])),
			Surname: unescapeComma(toString(row["surname"])),
			Email:   email,
		})
	}

	return recipients, nil
}

func buildRejectedStakeholderContext(combinedID int, motive string, apiKey string, client *http.Client, w http.ResponseWriter) (
	*requestWorkerInfo,
	[]rejectionRecipientInfo,
	string,
	string,
	string,
	error,
) {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return nil, nil, "", "", "", err
	}

	ipRecipients, err := getRequestIPRecipients(combinedID, apiKey, client, w)
	if err != nil {
		return nil, nil, "", "", "", err
	}

	projectNames, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return nil, nil, "", "", "", err
	}

	projectName := strings.Join(projectNames, ", ")
	requestSummary := strings.Join(summaryParts, " || ")
	message := unescapeComma(strings.TrimSpace(motive))

	return workerInfo, ipRecipients, projectName, requestSummary, message, nil
}

func notifITAccepted(combinedID int, requestData RequestPayload, projectApproverID int, acceptedAt string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	// ------------------------------------------------------------
	// 1) Get requester info
	// ------------------------------------------------------------
	workerQuery := map[string]interface{}{
		"table":     "budget_requests br LEFT JOIN people p ON br.people_id = p.id",
		"columns":   "p.id, p.name, p.surname, p.secondSurname, p.crm_email, p.user_email",
		"condition": fmt.Sprintf("br.id = %d", combinedID),
	}

	jsonWorkerQuery, _ := json.Marshal(workerQuery)
	resp := getReq(jsonWorkerQuery, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("failed to get requester info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("database error getting requester info: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var workerRows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&workerRows); err != nil {
		return fmt.Errorf("failed to decode requester info: %w", err)
	}
	if len(workerRows) == 0 {
		return fmt.Errorf("requester info not found")
	}

	researcherID := asInt(workerRows[0]["id"])
	researcherName := unescapeComma(toString(workerRows[0]["name"]))
	researcherSurname := unescapeComma(toString(workerRows[0]["surname"]))
	researcherSecondSurname := unescapeComma(toString(workerRows[0]["secondSurname"]))

	researcherFullSurname := strings.TrimSpace(researcherSurname + " " + researcherSecondSurname)
	if researcherFullSurname == "" {
		researcherFullSurname = researcherSurname
	}

	// ------------------------------------------------------------
	// 2) Get request equipment part
	// ------------------------------------------------------------
	partsQuery := map[string]interface{}{
		"table": "budget_parts bp " +
			"LEFT JOIN request_categories bc ON bp.category_id = bc.id " +
			"LEFT JOIN projects pr ON bp.project_id = pr.id " +
			"LEFT JOIN people ip ON bp.ip_id = ip.id",
		"columns": "bp.category_id, bc.category AS category_name, " +
			"bp.project_id, pr.short_name AS project_name, " +
			"ip.name AS ip_name, ip.surname AS ip_surname, " +
			"bp.purpose, bp.observations, bp.equipment_category",
		"condition": fmt.Sprintf("bp.id_combined = %d AND bp.category_id = 4", combinedID),
	}

	jsonPartsQuery, _ := json.Marshal(partsQuery)
	respParts := getReq(jsonPartsQuery, apiKey, client, w)
	if respParts == nil {
		return fmt.Errorf("failed to get equipment request part")
	}
	defer respParts.Body.Close()

	if respParts.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respParts.Body)
		return fmt.Errorf("database error getting equipment request part: %d - %s", respParts.StatusCode, string(bodyBytes))
	}

	var partRows []map[string]interface{}
	if err := json.NewDecoder(respParts.Body).Decode(&partRows); err != nil {
		return fmt.Errorf("failed to decode equipment request part: %w", err)
	}
	if len(partRows) == 0 {
		return fmt.Errorf("equipment request part not found")
	}

	row := partRows[0]

	projectName := unescapeComma(asString(row["project_name"]))
	piName := unescapeComma(asString(row["ip_name"]))
	piSurname := unescapeComma(asString(row["ip_surname"]))

	requestSummaryParts := make([]string, 0, 3)

	categoryName := asString(row["category_name"])
	if strings.TrimSpace(categoryName) == "" {
		categoryName = "Equipment"
	}
	requestSummaryParts = append(requestSummaryParts, categoryName)

	if eqCategory := strings.TrimSpace(asString(row["equipment_category"])); eqCategory != "" {
		requestSummaryParts = append(requestSummaryParts, "Category: "+eqCategory)
	}
	if purpose := strings.TrimSpace(unescapeComma(asString(row["purpose"]))); purpose != "" {
		requestSummaryParts = append(requestSummaryParts, "Purpose: "+purpose)
	}

	if acceptedAt == "" {
		acceptedAt = time.Now().Format("2006-01-02 15:04")
	}

	mailData := BudgetAcceptedByProjectsMailData{
		Name:              "IT",
		Surname:           "Team",
		ResearcherName:    researcherName,
		ResearcherSurname: researcherFullSurname,
		PIName:            piName,
		PISurname:         piSurname,
		ProjectName:       projectName,
		UpdatedAt:         acceptedAt,
		RequestSummary:    strings.Join(requestSummaryParts, " || "),
	}

	// ------------------------------------------------------------
	// 3) Load template
	// ------------------------------------------------------------
	tmplContent, err := os.ReadFile("module8workers/assets/itConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read IT confirmation template: %w", err)
	}

	tmpl, err := template.New("itConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse IT confirmation template: %w", err)
	}
	approvalTrace, traceErr := buildApprovalTrace(combinedID, projectApproverID, "", 0, "", apiKey, client, w)
	if traceErr != nil {
		createLog(fmt.Sprintf("Could not build approval trace for accounting email of request %d: %v", combinedID, traceErr), 1, apiKey, client, w)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateDataWithApprovalTrace(mailData, approvalTrace)); err != nil {
		return fmt.Errorf("failed to execute IT confirmation template: %w", err)
	}

	// ------------------------------------------------------------
	// 4) Send email to generic IT inbox
	// ------------------------------------------------------------
	if err := sendEmail(itEmail, "New Equipment Budget Request pending your management", buf.String()); err != nil {
		return fmt.Errorf("failed to send IT email: %w", err)
	}

	// ------------------------------------------------------------
	// 5) Notify IT users
	// ------------------------------------------------------------
	itUsersQuery := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "worker_id",
		"condition": "department_name = 'IT' AND manager = 1",
	}

	jsonITUsersQuery, _ := json.Marshal(itUsersQuery)
	respITUsers := getReq(jsonITUsersQuery, apiKey, client, w)
	if respITUsers == nil {
		createLog(fmt.Sprintf("Failed to get IT users for notification of request %d", combinedID), 1, apiKey, client, w)
		return nil
	}
	defer respITUsers.Body.Close()

	if respITUsers.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respITUsers.Body)
		createLog(fmt.Sprintf("Database error getting IT users for notification of request %d: %d - %s", combinedID, respITUsers.StatusCode, string(bodyBytes)), 1, apiKey, client, w)
		return nil
	}

	var itRows []map[string]interface{}
	if err := json.NewDecoder(respITUsers.Body).Decode(&itRows); err != nil {
		createLog(fmt.Sprintf("Failed to decode IT users for notification of request %d: %v", combinedID, err), 1, apiKey, client, w)
		return nil
	}

	for _, itRow := range itRows {
		if err := sendNotification(
			"New Equipment Budget Request pending your management",
			"A new equipment budget request has been approved by Projects. You can review and manage it from the Budgeting module.",
			asInt(itRow["worker_id"]),
			apiKey,
			client,
			w,
		); err != nil {
			createLog(fmt.Sprintf("Failed to send IT notification to user %d for request %d: %v", asInt(itRow["worker_id"]), combinedID, err), 1, apiKey, client, w)
		}
	}

	createLog(
		fmt.Sprintf("IT notification sent for equipment request %d (researcher_id=%d)", combinedID, researcherID),
		2,
		apiKey,
		client,
		w,
	)

	return nil
}
