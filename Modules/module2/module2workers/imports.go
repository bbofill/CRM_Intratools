package module2workers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	if mC.Module2ApiKey == inkey {
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

// get time since unix clock start
func getUnixTimestamp() int64 {
	return time.Now().Unix()
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

// hash string
func hashString(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	hashedBytes := hash.Sum(nil)
	return hex.EncodeToString(hashedBytes)
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
func jsonUsernames() map[string]interface{} {
	query := map[string]interface{}{
		"table":   "users",
		"columns": "username",
	}
	return query
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

// ------------------------------ module functions -----------------------------------
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
			createLog(fmt.Sprintf("Superadmin accessing module 2 from IP %s", r.RemoteAddr), 0, apiKey, client, w)
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
	return userRow, user, roleRow
}

// ------------------------------- MODULE FUNCTIONS ---------------------------------------
//

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
