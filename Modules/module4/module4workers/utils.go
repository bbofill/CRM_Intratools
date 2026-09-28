package module4workers

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
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	if mC.Module4ApiKey == inkey {
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

// ------------------------------------- HELPER FUNCTIONS -----------------------------------------
// GET user_id by username
func jsonUsernameID(user string) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id, role",
		"condition": fmt.Sprintf("username = '%s'", user),
	}
	return query
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
			createLog(fmt.Sprintf("Superadmin accessing module 4 from IP %s", r.RemoteAddr), 0, apiKey, client, w)
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

// returns true if there is an equal map
func containsMap(slice []map[string]interface{}, candidate map[string]interface{}) bool {
	for _, m := range slice {
		if len(m) != len(candidate) {
			continue
		}
		match := true
		for k, v := range m {
			if cv, ok := candidate[k]; !ok || cv != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// returns string suitable for json format
func escapeComma(s string) string {
	s = strings.ReplaceAll(s, "'", "¤")
	return strings.ReplaceAll(s, ",", "§")

}

// returns string in front-end format
func unescapeComma(s string) string {
	s = strings.ReplaceAll(s, "¤", "'")
	return strings.ReplaceAll(s, "§", ",")
}

// formats filepath for users' files
func saveGenericUserFile(userID int, firstName, surname, tempFilePath, fileName string) (string, error) {
	firstLetter := ""
	if len(firstName) > 0 {
		firstLetter = strings.ToLower(string(firstName[0]))
	}
	re := regexp.MustCompile(`[^a-zA-ZÀ-ÿ]`)
	surname = re.ReplaceAllString(surname, "")
	cleanSurname := strings.ReplaceAll(strings.ToLower(surname), " ", "")
	userDirName := fmt.Sprintf("%04d_%s%s", userID, firstLetter, cleanSurname)

	baseDir := filepath.Join("..", "..", "Uploads", "00_Users", userDirName)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("no se pudo crear el directorio: %v", err)
	}

	finalPath := filepath.Join(baseDir, fileName)

	src, err := os.Open(tempFilePath)
	if err != nil {
		return "", fmt.Errorf("no se pudo abrir archivo temporal: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(finalPath)
	if err != nil {
		return "", fmt.Errorf("no se pudo crear archivo destino: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("error copiando archivo: %v", err)
	}

	relativePath := filepath.ToSlash(filepath.Join("Uploads", "00_Users", userDirName, fileName))
	return relativePath, nil
}

// formats filepath for users' profile pictures
func saveProfilePicture(userID int, firstName, surname, tempFilePath string) (string, error) {
	firstLetter := ""
	if len(firstName) > 0 {
		firstLetter = strings.ToLower(string(firstName[0]))
	}
	re := regexp.MustCompile(`[^a-zA-ZÀ-ÿ]`)
	surname = re.ReplaceAllString(surname, "")
	cleanSurname := strings.ReplaceAll(strings.ToLower(surname), " ", "")
	userDirName := fmt.Sprintf("%04d_%s%s", userID, firstLetter, cleanSurname)
	//basepath
	baseDir := filepath.Join("..", "..", "Uploads", "00_Users", userDirName)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("ERROR creando directorio %s: %v\n", baseDir, err)
		return "", fmt.Errorf("no se pudo crear el directorio: %v", err)
	}
	// filename
	today := time.Now().Format("02012006")
	fileExt := strings.ToLower(filepath.Ext(tempFilePath))
	if fileExt == "" {
		fileExt = ".png"
	}
	fileName := fmt.Sprintf("%s_profilePicture%s", today, fileExt)
	finalPath := filepath.Join(baseDir, fileName)
	src, err := os.Open(tempFilePath)
	if err != nil {
		fmt.Printf("ERROR abriendo archivo temporal %s: %v\n", tempFilePath, err)
		return "", fmt.Errorf("no se pudo abrir archivo temporal: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(finalPath)
	if err != nil {
		fmt.Printf("ERROR creando archivo destino %s: %v\n", finalPath, err)
		return "", fmt.Errorf("no se pudo crear archivo destino: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		fmt.Printf("ERROR copiando archivo a %s: %v\n", finalPath, err)
		return "", fmt.Errorf("error copiando archivo: %v", err)
	}
	relativePath := filepath.ToSlash(filepath.Join("Uploads", "00_Users", userDirName, fileName))
	return relativePath, nil
}

func handleCheckRole(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, _, userRole := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"role": userRole,
	})
}

func handleGetUsersAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "users",
		"columns": "username, access_userhub",
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Error interno al preparar la consulta in getUsersAccess for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		return
	}
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla users para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD en getUsersAccess. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}
	type dbUser struct {
		Username      string `json:"username"`
		AccessUserhub int    `json:"access_userhub"`
	}

	var dbUsers []dbUser
	if err := json.NewDecoder(resp.Body).Decode(&dbUsers); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en getUsersAccess para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de usuarios", http.StatusInternalServerError)
		return
	}

	users := make([]map[string]string, 0, len(dbUsers))
	allowed := make([]string, 0)

	for _, u := range dbUsers {
		users = append(users, map[string]string{
			"username": u.Username,
		})

		if u.AccessUserhub == 1 {
			allowed = append(allowed, u.Username)
		}
	}

	response := map[string]interface{}{
		"users":   users,
		"allowed": allowed,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en getUsersAccess para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func handleSetUsersAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	w.Header().Set("Content-Type", "application/json")

	// Leer body enviado desde frontend
	var requestData struct {
		Users []string `json:"users"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		createLog(fmt.Sprintf("Error al decodificar solicitud JSON en handleSetUsersAccess para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar la solicitud", http.StatusBadRequest)
		return
	}

	// Resetear accessos a tothom menys a crmAdmin
	resetQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "access_userhub",
		"value":     "0",
		"condition": "people_id != '304'",
	}

	resetJson, _ := json.Marshal(resetQuery)

	resp := putReq(resetJson, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error reseteando accesos en handleSetUsersAccess para user %s", userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error reseteando accesos", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Para cada usuario enviado, activar acceso
	for _, u := range requestData.Users {
		updateQuery := map[string]interface{}{
			"table":     "users",
			"columns":   "access_userhub",
			"value":     "1",
			"condition": fmt.Sprintf("username = '%s'", u),
		}

		updateJson, _ := json.Marshal(updateQuery)
		resp := putReq(updateJson, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error asignando acceso a usuario %s en handleSetUsersAccess para user %s", u, userACCESSING), 1, apiKey, client, w)
		}
	}

	response := map[string]string{
		"message": "Access permissions updated successfully",
	}
	createLog(fmt.Sprintf("Access permissions updated successfully in handleSetUsersAccess for user %s", userACCESSING), 0, apiKey, client, w)
	json.NewEncoder(w).Encode(response)
}

func handleCheckAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table":     "users",
		"columns":   "access_userhub",
		"condition": fmt.Sprintf("people_id = '%d'", userID),
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		createLog(fmt.Sprintf("Error interno al preparar la consulta in handleCheckAccess for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		return
	}
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla users in handleCheckAccess for user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	type accessResult struct {
		AccessUserhub int `json:"access_userhub"`
	}

	var results []accessResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		http.Error(w, "Error al procesar datos de acceso", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en handleCheckAccess for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		return
	}

	if len(results) == 0 {
		json.NewEncoder(w).Encode(map[string]bool{
			"can_access_userhub": false,
		})
		return
	}

	canAccess := results[0].AccessUserhub == 1

	json.NewEncoder(w).Encode(map[string]bool{
		"can_access_userhub": canAccess,
	})

}

// sends option tables to front-end
func handleGetOptions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	tables := map[string][]string{
		"country":        {"code", "name_EN", "name_nationality"},
		"provinces":      {"code", "name"},
		"cities":         {"code", "name", "provinceCode"},
		"educationGrade": {"code", "name"},
		"universities":   {"code", "name"},
		"studies":        {"code", "name"},
		"people":         {"id", "name", "surname", "secondSurname"},
		"researchArea":   {"id", "name"},
		"training":       {"id", "name"},
		"researchGroup":  {"intern_code", "name", "category"},
		"knowledge_area": {"code", "name"},
		"hottable":       {"id", "name"},
	}

	result := make(map[string][]map[string]interface{})

	for table, columns := range tables {
		query := map[string]interface{}{
			"table":   table,
			"columns": strings.Join(columns, ", "),
		}
		jsonQuery, err := json.Marshal(query)
		if err != nil {
			createLog(fmt.Sprintf("Error al generar JSON para %s: %v del usuario %s", table, err, userACCESSING), 1, apiKey, client, w)
			http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
			return
		}
		resp := getReq(jsonQuery, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla: %s del usuario %s", table, userACCESSING), 1, apiKey, client, w)
			return
		}
		defer resp.Body.Close()

		var tableData []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tableData); err != nil {
			createLog(fmt.Sprintf("Error al decodificar la respuesta de %s: %v del usuario %s", table, err, userACCESSING), 1, apiKey, client, w)
			http.Error(w, "Error al decodificar respuesta de "+table, http.StatusInternalServerError)
			return
		}
		result[table] = tableData
	}

	query := map[string]interface{}{
		"table":     "room",
		"columns":   "id, name, uab_code",
		"condition": "category = 'office'",
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		createLog(fmt.Sprintf("Error al generar JSON para room: %v del usuario %s", err, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		return
	}

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla: room del usuario %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	var tableData []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tableData); err != nil {
		createLog(fmt.Sprintf("Error al decodificar la respuesta de room: %v del usuario %s", err, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al decodificar respuesta de room", http.StatusInternalServerError)
		return
	}

	result["room"] = tableData

	query = map[string]interface{}{
		"table":   "fundings",
		"columns": "id, name, code, active",
	}

	jsonQuery, err = json.Marshal(query)
	if err != nil {
		createLog(fmt.Sprintf("Error al generar JSON para room: %v del usuario %s", err, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		return
	}

	resp = getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla: room del usuario %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	var fundingsData []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&fundingsData); err != nil {
		createLog(fmt.Sprintf("Error al decodificar la respuesta de fundings: %v del usuario %s", err, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al decodificar respuesta de fundings", http.StatusInternalServerError)
		return
	}

	for i := range fundingsData {
		fundingsData[i]["name"] = unescapeComma(fundingsData[i]["name"].(string))
	}

	result["fundings"] = fundingsData

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		createLog(fmt.Sprintf("Error al codificar JSON final: %v del usuario %s", err, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al generar respuesta JSON", http.StatusInternalServerError)
		return
	}
}

// -------------------------------------- NOTIFICATIONS --------------------------------------
// returns all users' names to forward notifications
func handleGetUsers(apiKey string, client *http.Client, w http.ResponseWriter) {
	query := map[string]interface{}{
		"table":   "users u JOIN people p ON u.people_id = p.id",
		"columns": "p.id, p.name, p.surname, p.secondSurname",
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding response in handleGetUsers: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// creates an entry at notification table
func handleCreateNotification(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	creatorID, creatorUsername, _ := getUserInfo(apiKey, client, w, r)
	// Parse form with file
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		createLog(fmt.Sprintf("Error parsing multipart form: %v for user %s", err, creatorUsername), 1, apiKey, client, w)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Extract fields
	title := r.FormValue("title")
	content := r.FormValue("content")
	specificUserIDs := r.Form["specificUsers[]"]
	exceptUserIDs := r.Form["exceptUsers[]"]

	if strings.TrimSpace(title) == "" {
		createLog(fmt.Sprintf("Notification rejected: missing title for user %s", creatorUsername), 1, apiKey, client, w)
		http.Error(w, "Title required", http.StatusBadRequest)
		return
	}

	// Parse specific/except IDs as integers
	parseIDs := func(strs []string) []int {
		var ids []int
		for _, s := range strs {
			if id, err := strconv.Atoi(s); err == nil {
				ids = append(ids, id)
			}
		}
		return ids
	}
	specificIDs := parseIDs(specificUserIDs)
	exceptIDs := parseIDs(exceptUserIDs)

	// Handle file upload
	var filePath string
	file, handler, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		paddedID := fmt.Sprintf("%04d", creatorID)
		basePath := "../../Uploads/00_Users"
		dir := path.Join(basePath, fmt.Sprintf("%s_%s", paddedID, creatorUsername))
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			createLog(fmt.Sprintf("Error creating directory for user %s: %v", creatorUsername, err), 1, apiKey, client, w)
			http.Error(w, "Could not create directory", http.StatusInternalServerError)
			return
		}

		ext := filepath.Ext(handler.Filename)
		if ext == "" || len(ext) > 5 {
			ext = ".bin"
		}
		timestamp := time.Now().Format("2006-01-02_1504") // YYYY-MM-DD_HHMM
		safeName := fmt.Sprintf("notification_%s%s", timestamp, ext)

		filePath = path.Join(dir, safeName)

		createLog(fmt.Sprintf("File '%s' uploaded by %s at %s", safeName, creatorUsername, filePath), 0, apiKey, client, w)
		dst, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "Could not save file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		io.Copy(dst, file)
	}

	// Determine target users
	var targetUserIDs []int
	if len(specificIDs) > 0 {
		targetUserIDs = specificIDs
	} else {
		condition := ""
		// if option selected is all except x users:
		if len(exceptIDs) > 0 {
			var parts []string
			for _, id := range exceptIDs {
				parts = append(parts, strconv.Itoa(id))
			}
			condition = fmt.Sprintf("people_id NOT IN (%s)", strings.Join(parts, ","))
		}
		query := map[string]interface{}{
			"table":     "users",
			"columns":   "people_id",
			"condition": condition,
		}
		jsonQuery, _ := json.Marshal(query)
		resp := getReq(jsonQuery, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Failed to fetch user list for user %s", creatorUsername), 1, apiKey, client, w)
			http.Error(w, "Failed to fetch user list", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			createLog(fmt.Sprintf("Error parsing user list for user %s: %v", creatorUsername, err), 1, apiKey, client, w)
			http.Error(w, "Error parsing user list", http.StatusInternalServerError)
			return
		}
		for _, row := range result {
			if uid, ok := row["people_id"].(float64); ok {
				targetUserIDs = append(targetUserIDs, int(uid))
			}
		}
	}
	if len(targetUserIDs) == 0 {
		createLog(fmt.Sprintf("No target users found for notification by %s", creatorUsername), 1, apiKey, client, w)
		http.Error(w, "No target users specified", http.StatusBadRequest)
		return
	}

	// Insert notification
	encodedContent := b64.StdEncoding.EncodeToString([]byte(content))
	now := time.Now().Format("2006-01-02 15:04")
	insert := map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, file_path, user_id, created_at",
		"value":   fmt.Sprintf("admin-message, %s, '%s', %s, %d, %s", title, encodedContent, filePath, creatorID, now),
	}
	notifJSON, _ := json.Marshal(insert)
	resp := postReq(notifJSON, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Failed to insert notification '%s' by %s", title, creatorUsername), 1, apiKey, client, w)
		http.Error(w, "Failed to create notification", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	createLog(fmt.Sprintf("Notification '%s' created successfully by %s", title, creatorUsername), 0, apiKey, client, w)
	// Get notification ID
	getID := map[string]interface{}{
		"table":     "notifications",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = '%d' AND created_at = '%s'", creatorID, now),
	}
	idJSON, _ := json.Marshal(getID)
	idResp := getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog(fmt.Sprintf("Failed to retrieve new notification ID from database for user %s", creatorUsername), 1, apiKey, client, w)
		http.Error(w, "Failed to get notification ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var rows []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&rows)
	notifID := int(rows[0]["id"].(float64))

	// Sends to users creating an entry to notification_user table
	for _, uid := range targetUserIDs {
		link := map[string]interface{}{
			"table":   "notification_user",
			"columns": "notification_id, user_id, can_read, can_download, seen",
			"value":   fmt.Sprintf("%d, %d, 1, 0, 0", notifID, uid),
		}
		jsonLink, _ := json.Marshal(link)
		resp := postReq(jsonLink, apiKey, client, w)
		if resp != nil {
			resp.Body.Close()
		}
	}
	createLog(fmt.Sprintf("Notification '%s' (ID %d) distributed successfully to %d users", title, notifID, len(targetUserIDs)), 0, apiKey, client, w)
	w.WriteHeader(http.StatusOK)
}

// ------------------------------------- USERS HUB ----------------------------------------------

// returns lightweight users' information to render UsersHub cards
func handleGetUneixUsersSummary(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table": `people p
				  LEFT JOIN contract c ON c.id = (
					  SELECT c2.id
					  FROM contract c2
					  WHERE c2.people_id = p.id
					  ORDER BY date(c2.start_date) DESC, c2.id DESC
					  LIMIT 1
				  )`,
		"columns": `
			p.id AS people_id,
			p.picture_path,
			p.name AS people_name,
			p.surname,
			p.secondSurname,
			p.birth_date,
			p.gender,
			p.active,

			c.vinculation_type,
			c.contracting_institution,
			c.start_date AS contract_start_date
		`,
		"condition": "p.id != 304",
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Failed to get lightweight user list: no response from database API for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to get user list", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		createLog(fmt.Sprintf("Error decoding lightweight user list response: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error parsing users", http.StatusInternalServerError)
		return
	}

	for _, user := range users {
		for _, k := range []string{"people_name", "surname", "secondSurname"} {
			if str, ok := user[k].(string); ok {
				user[k] = unescapeComma(str)
			}
		}
	}

	if users == nil {
		users = []map[string]interface{}{}
	}

	createLog(fmt.Sprintf("User %s accessed UsersHub list", username), 0, apiKey, client, w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// returns all users' information to frontend
func handleGetUneixUsers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)

	condition := "p.id != 304"

	if idParam := strings.TrimSpace(r.URL.Query().Get("id")); idParam != "" {
		id, err := strconv.Atoi(idParam)
		if err != nil || id <= 0 {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}
		condition = fmt.Sprintf(`
			p.id != 304
			AND p.id = %d
		`, id)
	}
	//query differentiating fields with the same name to avoid missplacing data
	query := map[string]interface{}{
		"table": `people p
				  LEFT JOIN residence r ON p.id = r.people_id
				  LEFT JOIN people_nationality n ON p.id = n.people_id
				  LEFT JOIN contract c ON p.id = c.people_id
				  LEFT JOIN people_grade pg ON p.id = pg.people_id
				  LEFT JOIN people_phd phd ON p.id = phd.people_id
				  LEFT JOIN responsible resp ON phd.id = resp.tesis_id
				  LEFT JOIN people_training t ON p.id = t.people_id
				  LEFT JOIN people_group gr ON p.id = gr.people_id
				  LEFT JOIN people_supervisor ps ON c.id = ps.contract_id
				  LEFT JOIN reservation_user ru
					ON p.id = ru.user_id
					AND (ru.contract_id = c.id OR ru.contract_id IS NULL)
				  LEFT JOIN reservation re 
					ON ru.reservation_id = re.id
					AND re.type = 'assigned'
					AND (
						ru.contract_id = c.id
						OR (
							ru.contract_id IS NULL
							AND c.start_date IS NOT NULL
							AND re.start_date IS NOT NULL
							AND date(re.start_date) = date(c.start_date)
						)
					)`,
		"columns": `
			-- Campos de people
			p.id AS people_id,
			p.picture_path,
			p.name AS people_name,
			p.prefered_name,
			p.surname,
			p.secondSurname,
			p.gender,
			p.birth_date,
			p.birth_country,
			p.birth_province,
			p.birth_city,
			p.nif,
			p.nif_extended,
			p.user_phone,
			p.emergencyContact_name,
			p.emergencyContact_phone,
			p.user_email,
			p.people_idExternal,
			p.webUser_idExternal,
			p.crm_email,
			p.observations,
			p.academic_grade,
			p.research_interests,
			p.orcid,
			p.certificat_I3,
			p.personal_webPage,
			p.agreesToUneix,
			p.active,

			-- Campos de residence
			r.residence_country,
			r.residence_province,
			r.residence_city,
			r.postal_code,
			r.address,
			r.actual,

			-- Campos de people_nationality
			n.nationality_code,

			-- Campos de contract
			c.id AS contract_id,
			c.vinculation_type,
			c.trainee_type,
			c.internship,
			c.trainee_studies,
			c.start_date AS contract_start_date,
			c.end_date AS contract_end_date,
			c.job_category,
			c.type AS contract_type,
			c.position,
			c.file_path AS contract_file_path,
			c.totalDedication_hours,
			ps.supervisor_id,
			c.contracting_institution,
			c.other_contracting_institution,
			c.belongs_to_contract_program,
			c.office_location,
			re.table_id,
			c.research_area,
			c.funding,

			-- Campos de people_grade
			pg.id AS grade_id,
			pg.grade_master_doctorate,
			pg.code AS grade_code,
			pg.gradeName,
			pg.universityName AS grade_university_name,
			pg.graduation_university,
			pg.graduation_country,
			pg.graduation_year,

			-- Campos de people_phd
			phd.id AS phd_id,
			phd.phd_program,
			phd.phd_startYear,
			phd.phd_tesisDirector,
			phd.phd_university,
			phd.phd_tesisTitle,
			phd.phd_plannedPresentationDate,
			phd.phd_presentationDate,
			phd.phd_link,
			resp.ip_or_tutor,
			resp.name AS responsible_name,
			resp.people_id AS responsible_people_id,

			-- Campos de people_training
			t.training_id,
			t.enrolled AS training_enrolled,
			t.date AS training_date,
			t.diploma_path AS training_diploma_path,

			-- Campos de people_group
			gr.group_intern_code,
			gr.start_date AS group_start_date,
			gr.end_date AS group_end_date,
			gr.ip AS group_ip
		`,
		// ignorem id administratiu
		"condition": condition,
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Failed to get user list: no response from database API for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to get user list", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	var raw []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		createLog(fmt.Sprintf("Error decoding user list response: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error parsing users", http.StatusInternalServerError)
		return
	}
	if len(raw) == 0 {
		createLog(fmt.Sprintf("No users found in database for user %s", username), 0, apiKey, client, w)
	}

	//group data in sections
	grouped := make(map[string]map[string]interface{})
	for _, row := range raw {
		peopleID := fmt.Sprintf("%v", row["people_id"])
		if _, exists := grouped[peopleID]; !exists {
			grouped[peopleID] = map[string]interface{}{}
			for k, v := range row {
				if k == "observations" || k == "people_name" || k == "surname" || k == "secondSurname" || k == "prefered_name" || k == "research_interests" {
					if str, ok := v.(string); ok {
						v = unescapeComma(str)
					}
				}
				if v != nil {
					grouped[peopleID][k] = v
				}
			}
			grouped[peopleID]["nationality"] = []map[string]interface{}{}
			grouped[peopleID]["residence"] = []map[string]interface{}{}
			grouped[peopleID]["contract"] = []map[string]interface{}{}
			grouped[peopleID]["education"] = []map[string]interface{}{}
			grouped[peopleID]["phd"] = []map[string]interface{}{}
			grouped[peopleID]["training"] = []map[string]interface{}{}
			grouped[peopleID]["groups"] = []map[string]interface{}{}
		}

		// nationality
		if row["nationality_code"] != nil {
			curr := grouped[peopleID]["nationality"].([]map[string]interface{})
			entry := map[string]interface{}{"nationality_code": row["nationality_code"]}
			if !containsMap(curr, entry) {
				grouped[peopleID]["nationality"] = append(curr, entry)
			}
		}

		// residence
		if row["residence_country"] != nil || row["residence_city"] != nil {
			residence := map[string]interface{}{}
			for _, k := range []string{"residence_country", "residence_province", "residence_city", "postal_code", "address", "actual"} {
				if v := row[k]; v != nil {
					if k == "address" {
						if str, ok := v.(string); ok {
							v = unescapeComma(str)
						}
					}
					residence[k] = v
				}
			}
			if len(residence) > 0 {
				curr := grouped[peopleID]["residence"].([]map[string]interface{})
				if !containsMap(curr, residence) {
					grouped[peopleID]["residence"] = append(curr, residence)
				}
			}
		}

		// contract
		if row["contract_id"] != nil {
			contractID := fmt.Sprintf("%v", row["contract_id"])
			contract := map[string]interface{}{}
			for _, k := range []string{
				"contract_id", "vinculation_type", "trainee_type", "trainee_studies", "internship",
				"contract_start_date", "contract_end_date", "job_category", "contract_type", "position",
				"totalDedication_hours", "office_location", "table_id", "research_area",
				"funding", "contracting_institution", "other_contracting_institution",
				"belongs_to_contract_program", "contract_file_path",
			} {
				if v := row[k]; v != nil {
					if str, ok := v.(string); ok {
						v = unescapeComma(str)
					}
					contract[k] = v
				}
			}
			if contract["table_id"] == nil {
				uid, _ := strconv.Atoi(peopleID)
				ref, err := loadDeskReservationRef(
					uid,
					normalizeNumericID(contractID),
					"",
					normalizeDateOnly(fmt.Sprintf("%v", contract["contract_start_date"])),
					apiKey,
					client,
					w,
				)
				if err == nil && ref != nil && ref.TableID != 0 {
					contract["table_id"] = ref.TableID
				}
			}
			// look for repeated contracts
			currContracts := grouped[peopleID]["contract"].([]map[string]interface{})
			var existingContract map[string]interface{}
			for _, p := range currContracts {
				if fmt.Sprintf("%v", p["contract_id"]) == contractID {
					existingContract = p
					break
				}
			}

			// supervisor entry creation
			var supervisor map[string]interface{}
			if row["supervisor_id"] != nil {
				supervisor = map[string]interface{}{
					"supervisor_id": row["supervisor_id"],
				}
			}

			// avoid duplicates
			if existingContract != nil {
				// ensure supervisor list exists
				supList, ok := existingContract["supervisor"].([]map[string]interface{})
				if !ok {
					supList = []map[string]interface{}{}
				}
				if supervisor != nil {
					exists := false
					supID := fmt.Sprintf("%v", supervisor["supervisor_id"])
					for _, s := range supList {
						if fmt.Sprintf("%v", s["supervisor_id"]) == supID {
							exists = true
							break
						}
					}
					if !exists {
						supList = append(supList, supervisor)
						existingContract["supervisor"] = supList
					}
				}
			} else {
				if supervisor != nil {
					contract["supervisor"] = []map[string]interface{}{supervisor}
				} else {
					contract["supervisor"] = []map[string]interface{}{}
				}
				grouped[peopleID]["contract"] = append(currContracts, contract)
			}
		}

		// education (people_grade)
		if row["grade_code"] != nil {
			education := map[string]interface{}{}
			for _, k := range []string{
				"grade_id", "grade_master_doctorate", "grade_code", "gradeName",
				"grade_university_name", "graduation_university",
				"graduation_country", "graduation_year",
			} {
				if v := row[k]; v != nil {
					if str, ok := v.(string); ok {
						v = unescapeComma(str)
					}
					education[k] = v
				}
			}
			curr := grouped[peopleID]["education"].([]map[string]interface{})
			if !containsMap(curr, education) {
				grouped[peopleID]["education"] = append(curr, education)
			}
		}

		// phd
		if row["phd_id"] != nil {
			phdID := row["phd_id"]
			phdData := map[string]interface{}{}
			for _, k := range []string{
				"phd_id", "phd_program", "phd_university", "phd_startYear",
				"phd_tesisDirector", "phd_tesisTitle",
				"phd_plannedPresentationDate", "phd_presentationDate", "phd_link",
			} {
				if v := row[k]; v != nil {
					if str, ok := v.(string); ok {
						v = unescapeComma(str)
					}
					phdData[k] = v
				}
			}

			// look for repeated phds
			currPhds := grouped[peopleID]["phd"].([]map[string]interface{})
			var existingPhd map[string]interface{}
			for _, p := range currPhds {
				if p["phd_id"] == phdID {
					existingPhd = p
					break
				}
			}

			// responsible entry creation
			var responsible map[string]interface{}
			if row["responsible_people_id"] != nil || row["responsible_name"] != nil {
				responsible = map[string]interface{}{}
				if v := row["ip_or_tutor"]; v != nil {
					responsible["ip_or_tutor"] = v
				}
				if v := row["responsible_name"]; v != nil {
					responsible["name"] = v
				}
				if v := row["responsible_people_id"]; v != nil {
					responsible["people_id"] = v
				}
			}

			// avoid duplicates
			if existingPhd != nil {
				if responsible != nil {
					exists := false
					for _, r := range existingPhd["responsible"].([]map[string]interface{}) {
						if r["name"] == responsible["name"] &&
							r["ip_or_tutor"] == responsible["ip_or_tutor"] &&
							r["people_id"] == responsible["people_id"] {
							exists = true
							break
						}
					}
					if !exists {
						existingPhd["responsible"] = append(
							existingPhd["responsible"].([]map[string]interface{}),
							responsible,
						)
					}
				}
			} else {
				if responsible != nil {
					phdData["responsible"] = []map[string]interface{}{responsible}
				} else {
					phdData["responsible"] = []map[string]interface{}{}
				}
				grouped[peopleID]["phd"] = append(currPhds, phdData)
			}
		}

		// training
		if row["training_id"] != nil {
			training := map[string]interface{}{}
			for _, k := range []string{
				"training_id", "training_date", "training_enrolled",
				"training_completed", "training_diploma_path",
			} {
				if v := row[k]; v != nil {
					training[k] = v
				}
			}
			curr := grouped[peopleID]["training"].([]map[string]interface{})
			if !containsMap(curr, training) {
				grouped[peopleID]["training"] = append(curr, training)
			}
		}

		// groups
		if row["group_intern_code"] != nil {
			group := map[string]interface{}{}
			for _, k := range []string{
				"group_intern_code", "group_start_date", "group_end_date", "group_ip",
			} {
				if v := row[k]; v != nil {
					group[k] = v
				}
			}
			curr := grouped[peopleID]["groups"].([]map[string]interface{})
			if !containsMap(curr, group) {
				grouped[peopleID]["groups"] = append(curr, group)
			}
		}
	}

	//sends to frontend
	var final []map[string]interface{}
	for _, user := range grouped {
		final = append(final, user)
	}

	createLog(fmt.Sprintf("User %s accesed to UsersHub", username), 0, apiKey, client, w)
	w.Header().Set("Content-Type", "application/json")
	if final == nil {
		final = []map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(final)
}

// manages update of general information
func saveOrUpdatePeople(payload struct {
	ID       *int                   `json:"id"`
	FormData map[string]interface{} `json:"formData"`
}, apiKey string, client *http.Client, w http.ResponseWriter) (int, error) {

	var userID int

	formatSQLValue := func(val interface{}) string {
		if val == nil {
			return ""
		}
		str := strings.TrimSpace(fmt.Sprintf("%v", val))
		if str == "" || str == "0x0" {
			return ""
		}
		if str == "true" || str == "1" {
			return "1"
		}
		if str == "false" || str == "0" {
			return "0"
		}
		if _, err := strconv.Atoi(str); err == nil {
			return str
		}
		return strings.ReplaceAll(str, "'", "'")
	}

	generalData, ok := payload.FormData["general"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("FormData['general'] no es un mapa válido")
	}

	//profile picture
	var newProfilePath string
	if imgPath, ok := generalData["profile_image"].(string); ok && imgPath != "" {
		//creating route
		firstName := fmt.Sprintf("%v", generalData["people_name"])
		surname := fmt.Sprintf("%v", generalData["surname"])
		re := regexp.MustCompile(`[^a-zA-ZÀ-ÿ]`)
		surname = re.ReplaceAllString(surname, "")
		if payload.ID != nil {
			newPath, err := saveProfilePicture(*payload.ID, firstName, surname, imgPath)
			if err != nil {
				fmt.Println("Error al guardar imagen de perfil:", err)
			} else {
				newProfilePath = newPath
				generalData["picture_path"] = newPath
			}
		}
	}
	if generalData["nif_extended"] == "" {
		generalData["nif_extended"] = generalData["nif"]
	}
	columns := []string{
		"people_name", "picture_path", "prefered_name", "surname", "secondSurname", "gender", "birth_date",
		"birth_country", "birth_province", "birth_city", "nif", "nif_extended",
		"user_phone", "emergencyContact_name", "emergencyContact_phone",
		"user_email", "people_idExternal", "webUser_idExternal", "crm_email",
		"observations", "academic_grade", "research_interests", "orcid",
		"certificat_I3", "personal_webPage", "agreesToUneix", "active",
	}

	// --------------------------- CREATING NEW USER ------------------------------------
	if payload.ID == nil {
		createLog("Creating new person entry in database", 0, apiKey, client, w)
		fields := []string{}
		values := []string{}

		for _, col := range columns {
			val := formatSQLValue(generalData[col])
			if val != "" && col != "people_name" {
				fields = append(fields, col)
				val = escapeComma(val)
				values = append(values, val)
			} else if val != "" && col == "people_name" {
				//corresponding field name db <-> frontend
				fields = append(fields, "name")
				val = escapeComma(val)
				values = append(values, val)
			} else if val == "" && col == "picture_path" {
				//if there's no picture, default
				fields = append(fields, col)
				values = append(values, "Uploads/00_Users/default_profile.png")
			}
		}

		insertPeople := map[string]interface{}{
			"table":   "people",
			"columns": strings.Join(fields, ","),
			"value":   strings.Join(values, ","),
		}

		insertJSON, _ := json.Marshal(insertPeople)
		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil {
			createLog("Failed to insert new person (nil response)", 1, apiKey, client, w)
			return 0, fmt.Errorf("failed to insert new person (nil response)")
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return 0, fmt.Errorf("failed to insert new person")
		}

		getID := map[string]interface{}{
			"table":   "people",
			"columns": "last_insert_rowid() AS id",
		}
		idJSON, _ := json.Marshal(getID)
		idResp := getReq(idJSON, apiKey, client, w)
		if idResp == nil {
			return 0, fmt.Errorf("failed to get new user ID")
		}
		defer idResp.Body.Close()

		var rows []map[string]interface{}
		if err := json.NewDecoder(idResp.Body).Decode(&rows); err != nil || len(rows) == 0 {
			return 0, fmt.Errorf("error reading new user ID")
		}
		userID = int(rows[0]["id"].(float64))

		//saving profile picture
		if newProfilePath == "" && generalData["profile_image"] != nil {
			imgPath := fmt.Sprintf("%v", generalData["profile_image"])
			surname := fmt.Sprintf("%v", generalData["surname"])
			re := regexp.MustCompile(`[^a-zA-ZÀ-ÿ]`)
			surname = re.ReplaceAllString(surname, "")
			newPath, err := saveProfilePicture(userID, fmt.Sprintf("%v", generalData["people_name"]), surname, imgPath)
			if err == nil {
				newProfilePath = newPath
				update := map[string]interface{}{
					"table":     "people",
					"columns":   "picture_path",
					"value":     newProfilePath,
					"condition": fmt.Sprintf("id = %d", userID),
				}
				updateJSON, _ := json.Marshal(update)
				putReq(updateJSON, apiKey, client, w)
			}
		}
		createLog(fmt.Sprintf("New person created successfully with ID %d", userID), 0, apiKey, nil, w)

		created := time.Now().Format("2006-01-02 15:04")
		newHires_query := map[string]interface{}{
			"table":   "newHires_tasks",
			"columns": "people_id, added",
			"value":   fmt.Sprintf("%d, %s", userID, created),
		}
		newHiresJSON, _ := json.Marshal(newHires_query)
		respHires := postReq(newHiresJSON, apiKey, client, w)
		if respHires == nil {
			createLog("Failed to insert new person (nil response)", 1, apiKey, client, w)
			return 0, fmt.Errorf("failed to insert new person (nil response)")
		}
		defer respHires.Body.Close()

	} else {
		// ----------------- UPDATE -----------------
		userID = *payload.ID

		updateCols := []string{}
		updateVals := []string{}
		for _, col := range columns {
			val := formatSQLValue(generalData[col])
			if val != "" && col != "people_name" {
				updateCols = append(updateCols, col)
				val = escapeComma(val)
				updateVals = append(updateVals, val)
			} else if val != "" && col == "people_name" {
				updateCols = append(updateCols, "name")
				val = escapeComma(val)
				updateVals = append(updateVals, val)
			}
		}

		//only changes picture if there's a new one
		if newProfilePath != "" {
			updateCols = append(updateCols, "picture_path")
			updateVals = append(updateVals, newProfilePath)
		}

		if len(updateCols) > 0 {
			update := map[string]interface{}{
				"table":     "people",
				"columns":   strings.Join(updateCols, ", "),
				"value":     strings.Join(updateVals, ", "),
				"condition": fmt.Sprintf("id = %d", userID),
			}
			jsonUpdate, _ := json.Marshal(update)

			resp := putReq(jsonUpdate, apiKey, client, w)
			if resp == nil || resp.StatusCode >= 400 {
				return 0, fmt.Errorf("failed to update user")
			}
			defer resp.Body.Close()
		}
		createLog(fmt.Sprintf("Person ID %d successfully updated", userID), 0, apiKey, nil, w)

	}

	return userID, nil
}

// manages update of nationalities
func saveNationalities(userID int, nationalities []map[string]string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	// deletes old nationalities
	del := map[string]interface{}{
		"table":     "people_nationality",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delJSON, _ := json.Marshal(del)
	delResp := deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		delResp.Body.Close()
	} else {
		fmt.Println("WARNING: deleteReq devolvió nil (no se pudo borrar nacionalidades anteriores)")
	}

	// inserts new nationalities
	if len(nationalities) == 0 {
		createLog(fmt.Sprintf("No nationalities to insert for userID %d", userID), 0, apiKey, client, w)
		return nil
	}
	for i, n := range nationalities {
		code := n["nationality_code"]
		if code == "" {
			continue
		}
		insert := map[string]interface{}{
			"table":   "people_nationality",
			"columns": "people_id, nationality_code",
			"value":   fmt.Sprintf("%d, %s", userID, code),
		}
		insertJSON, _ := json.Marshal(insert)
		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error updating nationalities for userID %d", userID), 0, apiKey, client, w)
			return fmt.Errorf("[Insert %d] postReq devolvió nil", i+1)
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("[Insert %d] failed to insert nationality (HTTP %d)", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}
	return nil
}

// manages update of residences
func saveResidences(userID int, residences []map[string]string, apiKey string, client *http.Client, w http.ResponseWriter) error {

	// deletes old residences
	del := map[string]interface{}{
		"table":     "residence",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delJSON, _ := json.Marshal(del)
	delResp := deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		delResp.Body.Close()
	} else {
		createLog(fmt.Sprintf("Error deleting old residences for userID %d", userID), 1, apiKey, client, w)
	}

	// inserts new residences
	if len(residences) == 0 {
		return nil
	}
	for i, r := range residences {
		isEmpty :=
			strings.TrimSpace(r["residence_country"]) == "" &&
				strings.TrimSpace(r["residence_province"]) == "" &&
				strings.TrimSpace(r["residence_city"]) == "" &&
				strings.TrimSpace(r["postal_code"]) == "" &&
				strings.TrimSpace(r["address"]) == ""

		if isEmpty {
			// saltar sin error
			continue
		}
		actual := r["actual"]
		if actual == "" {
			actual = r["actual_hidden"]
		}
		if actual == "" {
			actual = "0"
		}
		values := []string{
			fmt.Sprintf("%d", userID),
			r["residence_country"],
			r["residence_province"],
			r["residence_city"],
			r["postal_code"],
			escapeComma(r["address"]),
			actual,
		}
		insert := map[string]interface{}{
			"table":   "residence",
			"columns": "people_id, residence_country, residence_province, residence_city, postal_code, address, actual",
			"value":   strings.Join(values, ", "),
		}
		insertJSON, _ := json.Marshal(insert)
		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error inserting residence #%d for userID %d", i+1, userID), 1, apiKey, client, w)
			return fmt.Errorf("[Insert %d] postReq devolvió nil", i+1)
		}
		if resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting residence #%d for userID %d", i+1, userID), 1, apiKey, client, w)
			return fmt.Errorf("[Insert %d] failed to insert Residences (HTTP %d)", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}
	return nil
}

// formats file names for contract files
func saveContractFile(userID int, firstName, surname, tempFilePath string) (string, error) {
	today := time.Now().Format("02012006")
	return saveGenericUserFile(userID, firstName, surname, tempFilePath, today+"_Contract.pdf")
}

type DeskOverlapWarning struct {
	ContractID    int    `json:"contract_id"`
	ReservationID int    `json:"reservation_id"`
	TableID       int    `json:"table_id"`
	DeskName      string `json:"desk_name"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Message       string `json:"message"`
}

type deskReservationRef struct {
	ID      int
	TableID int
}

// formats file names for diploma training files
func saveTrainingFile(userID int, firstName, surname, tempFilePath, trainingID string) (string, error) {
	today := time.Now().Format("02012006")
	fileName := fmt.Sprintf("%s_%s_Diploma%s", today, trainingID, filepath.Ext(tempFilePath))
	return saveGenericUserFile(userID, firstName, surname, tempFilePath, fileName)
}

// manages update of contracts
func saveContracts(userID int, contracts []map[string]string, apiKey string, client *http.Client, w http.ResponseWriter) ([]DeskOverlapWarning, error) {

	// corresponding field names (frontend <-> DB)
	columnMap := map[string]string{
		"contract_start_date": "start_date",
		"contract_end_date":   "end_date",
		"internship":          "internship",
		"contract_file_path":  "file_path",
		"contract_type":       "type",
	}

	// deletes old responsible entries
	delResp := map[string]interface{}{
		"table":     "people_supervisor",
		"condition": fmt.Sprintf("contract_id IN (SELECT id FROM contract WHERE people_id = %d)", userID),
	}
	delRespJSON, _ := json.Marshal(delResp)
	deleteReq(delRespJSON, apiKey, client, w)
	// deletes old contracts
	del := map[string]interface{}{
		"table":     "contract",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delJSON, _ := json.Marshal(del)
	deleteReq(delJSON, apiKey, client, w)

	hasContracted := false

	deskWarnings := []DeskOverlapWarning{}

	// creates new contract entries
	for _, c := range contracts {
		fields := []string{"people_id"}
		values := []string{fmt.Sprintf("%d", userID)}

		var supervisors []map[string]interface{}

		var startDate, endDate, ctype, tableID string
		oldContractID := strings.TrimSpace(c["contract_id"])

		for k, v := range c {
			if strings.HasSuffix(k, "_hidden") {
				baseKey := strings.TrimSuffix(k, "_hidden")
				if _, hasRealValue := c[baseKey]; hasRealValue {
					continue
				}
				k = baseKey
			}

			if v == "" {
				continue
			}
			if k == "contract_id" {
				continue
			}
			if k == "table_id" {
				tableID = v
				continue
			}
			if k == "supervisor_id" {
				continue
			}
			if k == "supervisors" {
				var arr []map[string]interface{}
				if err := json.Unmarshal([]byte(v), &arr); err == nil {
					supervisors = append(supervisors, arr...)
					//println("  Parsed supervisors:", supervisors)
				} else {
					fmt.Printf("bad supervisors json: %q err=%v\n", v, err)
				}
				continue
			}

			dbCol, ok := columnMap[k]
			if !ok {
				dbCol = k
			}
			switch dbCol {
			case "start_date":
				startDate = v
			case "end_date":
				endDate = v
			case "type":
				ctype = v
			}
			v = escapeComma(v)
			fields = append(fields, dbCol)
			values = append(values, v)

			if dbCol == "vinculation_type" && strings.EqualFold(v, "Contracted worker") {
				hasContracted = true
			}
		}

		insert := map[string]interface{}{
			"table":   "contract",
			"columns": strings.Join(fields, ", "),
			"value":   strings.Join(values, ", "),
		}

		insertJSON, _ := json.Marshal(insert)
		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting contract for userID %d", userID), 1, apiKey, client, w)
			return deskWarnings, fmt.Errorf("failed to insert contract for user %d", userID)
		}

		resp.Body.Close()

		condParts := []string{fmt.Sprintf("people_id = %d", userID)}

		if startDate != "" {
			condParts = append(condParts, fmt.Sprintf("start_date = '%s'", escapeComma(startDate)))
		}
		if endDate != "" {
			condParts = append(condParts, fmt.Sprintf("end_date = '%s'", escapeComma(endDate)))
		}
		if ctype != "" {
			condParts = append(condParts, fmt.Sprintf("type = '%s'", escapeComma(ctype)))
		}

		cond := strings.Join(condParts, " AND ") + " ORDER BY id DESC LIMIT 1"

		getID := map[string]interface{}{
			"table":     "contract",
			"columns":   "id",
			"condition": cond,
		}

		idJSON, _ := json.Marshal(getID)
		idResp := getReq(idJSON, apiKey, client, w)
		if idResp == nil {
			createLog(fmt.Sprintf("Error retrieving last inserted PhD ID for userID %d", userID), 1, apiKey, client, w)
			return deskWarnings, fmt.Errorf("failed to get phd id for user %d", userID)
		}
		defer idResp.Body.Close()

		var rows []map[string]interface{}
		if err := json.NewDecoder(idResp.Body).Decode(&rows); err != nil || len(rows) == 0 {
			createLog(fmt.Sprintf("Error decoding PhD ID for userID %d: %v", userID, err), 1, apiKey, client, w)
			return deskWarnings, fmt.Errorf("error reading phd id for user %d", userID)
		}

		tesisID := int(rows[0]["id"].(float64))

		warnings, err := syncContractDeskReservation(userID, oldContractID, tesisID, tableID, startDate, endDate, apiKey, client, w)
		if err != nil {
			return deskWarnings, err
		}
		deskWarnings = append(deskWarnings, warnings...)

		// insert responsibles
		for _, r := range supervisors {
			// --- No insertar responsables vacíos ---
			sidVal, ok := r["supervisor_id"]
			sidStr := strings.TrimSpace(fmt.Sprintf("%v", sidVal))
			if !ok || sidStr == "" || sidStr == "<nil>" {
				continue
			}

			insertSup := map[string]interface{}{
				"table":   "people_supervisor",
				"columns": "contract_id, supervisor_id",
				"value":   fmt.Sprintf("%d, %s", tesisID, sidStr),
			}

			insertRespJSON, _ := json.Marshal(insertSup)
			resp := postReq(insertRespJSON, apiKey, client, w)
			if resp != nil {
				//fmt.Println("Supervisor insertat")
				resp.Body.Close()
			} else {
				createLog(fmt.Sprintf("Error inserting supervisor entry for userID %d", userID), 1, apiKey, client, w)
			}
		}
	}

	// if contract is type "Contracted worker" -> create user entry
	if hasContracted {
		//verify if there's already a user
		check := map[string]interface{}{
			"table":     "users",
			"columns":   "people_id",
			"condition": fmt.Sprintf("people_id = %d", userID),
		}
		checkJSON, _ := json.Marshal(check)
		checkResp := getReq(checkJSON, apiKey, client, w)
		if checkResp == nil {
			createLog(fmt.Sprintf("Error verifying user existence for contracted person %d", userID), 1, apiKey, client, w)
			return deskWarnings, fmt.Errorf("failed to verify user account for people_id %d", userID)
		}
		defer checkResp.Body.Close()

		var existing []map[string]interface{}
		_ = json.NewDecoder(checkResp.Body).Decode(&existing)
		if len(existing) == 0 {
			personQuery := map[string]interface{}{
				"table":     "people",
				"columns":   "name, surname",
				"condition": fmt.Sprintf("id = %d", userID),
			}
			personJSON, _ := json.Marshal(personQuery)
			personResp := getReq(personJSON, apiKey, client, w)
			if personResp == nil {
				createLog(fmt.Sprintf("Error fetching person data for contracted user %d", userID), 1, apiKey, client, w)
				return deskWarnings, fmt.Errorf("failed to fetch person data for user %d", userID)
			}
			defer personResp.Body.Close()

			var personRows []map[string]interface{}
			if err := json.NewDecoder(personResp.Body).Decode(&personRows); err != nil || len(personRows) == 0 {
				createLog(fmt.Sprintf("Error decoding person info for user %d: %v", userID, err), 1, apiKey, client, w)
				return deskWarnings, fmt.Errorf("person not found for user %d", userID)
			}
			name := strings.TrimSpace(personRows[0]["name"].(string))
			surname := strings.TrimSpace(personRows[0]["surname"].(string))

			//make sure there's only letters in username
			re := regexp.MustCompile(`[^A-Za-z]`)
			surname = re.ReplaceAllString(surname, "")
			base := sanitizeUsername(strings.ToLower(string(name[0]) + surname))

			final := base
			counter := 1

			// Comprobar repetición como en M0
			for usernameExists(final, apiKey, client, w) {
				counter++
				final = fmt.Sprintf("%s%d", base, counter)
			}

			password := hashString("Crm2026@" + final)

			insertUser := map[string]interface{}{
				"table":   "users",
				"columns": "people_id, username, password, role",
				"value":   fmt.Sprintf("%d, %s, %s, user", userID, final, password),
			}
			userJSON, _ := json.Marshal(insertUser)
			userResp := postReq(userJSON, apiKey, client, w)
			if userResp == nil || userResp.StatusCode >= 400 {
				createLog(fmt.Sprintf("Error creating linked user account for contracted userID %d", userID), 1, apiKey, client, w)
				return deskWarnings, fmt.Errorf("failed to create user for people_id %d", userID)
			}
			defer userResp.Body.Close()
		}
	}
	return deskWarnings, nil
}

func normalizeDateOnly(value string) string {
	value = strings.TrimSpace(value)

	if value == "" || value == "<nil>" {
		return ""
	}

	// Si viene como "2026-05-27T00:00:00Z"
	if strings.Contains(value, "T") {
		return strings.Split(value, "T")[0]
	}

	// Si viene como "2026-05-27 00:00:00"
	if strings.Contains(value, " ") {
		return strings.Split(value, " ")[0]
	}

	return value
}

func intFromDBValue(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func normalizeNumericID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return ""
	}
	if strings.Contains(value, ".") {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return fmt.Sprintf("%d", int(parsed))
		}
	}
	return value
}

func loadDeskReservationRef(userID int, oldContractID string, tableID string, contractStartDate string, apiKey string, client *http.Client, w http.ResponseWriter) (*deskReservationRef, error) {
	conditions := []string{}
	oldContractID = normalizeNumericID(oldContractID)
	tableID = normalizeNumericID(tableID)
	contractStartDate = normalizeDateOnly(contractStartDate)

	if oldContractID != "" && oldContractID != "0" && oldContractID != "<nil>" {
		conditions = append(conditions, fmt.Sprintf("ru.contract_id = %s", oldContractID))
	}
	if tableID != "" && tableID != "0" && tableID != "<nil>" {
		conditions = append(conditions, fmt.Sprintf("ru.user_id = %d AND r.table_id = %s", userID, tableID))
	}
	if contractStartDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"ru.user_id = %d AND ru.contract_id IS NULL AND date(r.start_date) = date('%s')",
			userID,
			contractStartDate,
		))
	}

	for _, condition := range conditions {
		query := map[string]interface{}{
			"table":   "reservation r JOIN reservation_user ru ON ru.reservation_id = r.id",
			"columns": "r.id, r.table_id",
			"condition": fmt.Sprintf(
				"%s AND r.type = 'assigned' ORDER BY r.start_date DESC LIMIT 1",
				condition,
			),
		}

		body, _ := json.Marshal(query)
		resp := getReq(body, apiKey, client, w)
		if resp == nil {
			return nil, fmt.Errorf("failed to retrieve desk reservation for user %d", userID)
		}

		var rows []map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&rows)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			continue
		}

		return &deskReservationRef{
			ID:      intFromDBValue(rows[0]["id"]),
			TableID: intFromDBValue(rows[0]["table_id"]),
		}, nil
	}

	return nil, nil
}

func detectDeskOverlaps(contractID int, reservationID int, tableID int, startDate string, endDate string, apiKey string, client *http.Client, w http.ResponseWriter) ([]DeskOverlapWarning, error) {
	if tableID == 0 || startDate == "" {
		return nil, nil
	}

	if endDate == "" {
		endDate = "2099-12-31"
	}

	query := map[string]interface{}{
		"table":   "reservation r LEFT JOIN hottable h ON r.table_id = h.id",
		"columns": "r.id, r.table_id, h.name AS desk_name, r.start_date, r.end_date",
		"condition": fmt.Sprintf(
			"r.table_id = %d AND r.id != %d AND (r.type IS NULL OR r.type != 'released') AND substr(r.start_date, 1, 10) <= '%s' AND COALESCE(substr(r.end_date, 1, 10), '2099-12-31') >= '%s'",
			tableID,
			reservationID,
			endDate,
			startDate,
		),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to detect desk overlaps for reservation %d", reservationID)
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	warnings := []DeskOverlapWarning{}
	for _, row := range rows {
		overlapStart := normalizeDateOnly(fmt.Sprintf("%v", row["start_date"]))
		overlapEnd := normalizeDateOnly(fmt.Sprintf("%v", row["end_date"]))

		if overlapStart == "" || overlapStart < startDate {
			overlapStart = startDate
		}
		if overlapEnd == "" || overlapEnd > endDate {
			overlapEnd = endDate
		}

		deskName := ""
		if row["desk_name"] != nil {
			deskName = unescapeComma(fmt.Sprintf("%v", row["desk_name"]))
		}
		if deskName == "" {
			deskName = fmt.Sprintf("desk %d", tableID)
		}

		warning := DeskOverlapWarning{
			ContractID:    contractID,
			ReservationID: intFromDBValue(row["id"]),
			TableID:       tableID,
			DeskName:      deskName,
			StartDate:     overlapStart,
			EndDate:       overlapEnd,
		}
		warning.Message = fmt.Sprintf("%s overlaps from %s to %s.", warning.DeskName, warning.StartDate, warning.EndDate)
		warnings = append(warnings, warning)
	}

	return warnings, nil
}

func reservationUserHasContract(reservationID int, contractID int, apiKey string, client *http.Client, w http.ResponseWriter) bool {
	query := map[string]interface{}{
		"table":     "reservation_user",
		"columns":   "reservation_id",
		"condition": fmt.Sprintf("reservation_id = %d AND contract_id = %d", reservationID, contractID),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		return false
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false
	}
	return len(rows) > 0
}

func updateReservationUserContract(reservationID int, userID int, contractID int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	conditions := []string{
		fmt.Sprintf("reservation_id = %d AND user_id = %d", reservationID, userID),
		fmt.Sprintf("reservation_id = %d", reservationID),
	}

	for _, condition := range conditions {
		updateReservationUser := map[string]interface{}{
			"table":     "reservation_user",
			"columns":   "contract_id",
			"value":     fmt.Sprintf("%d", contractID),
			"condition": condition,
		}

		updateReservationUserJSON, _ := json.Marshal(updateReservationUser)
		updateRuResp := putReq(updateReservationUserJSON, apiKey, client, w)
		if updateRuResp == nil || updateRuResp.StatusCode >= 400 {
			if updateRuResp != nil {
				updateRuResp.Body.Close()
			}
			continue
		}
		updateRuResp.Body.Close()

		if reservationUserHasContract(reservationID, contractID, apiKey, client, w) {
			return nil
		}
	}

	createLog(fmt.Sprintf("Error updating reservation_user contract_id for reservationID %d", reservationID), 1, apiKey, client, w)
	return fmt.Errorf("failed to update reservation_user contract_id for reservation %d", reservationID)
}

func syncContractDeskReservation(userID int, oldContractID string, newContractID int, tableID string, startDate string, endDate string, apiKey string, client *http.Client, w http.ResponseWriter) ([]DeskOverlapWarning, error) {
	startDate = normalizeDateOnly(startDate)
	endDate = normalizeDateOnly(endDate)
	if startDate == "" {
		return nil, nil
	}

	ref, err := loadDeskReservationRef(userID, oldContractID, tableID, startDate, apiKey, client, w)
	if err != nil || ref == nil || ref.ID == 0 {
		return nil, err
	}

	effectiveEndDate := endDate
	if effectiveEndDate == "" {
		effectiveEndDate = "2099-12-31"
	}

	warnings, err := detectDeskOverlaps(newContractID, ref.ID, ref.TableID, startDate, effectiveEndDate, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	updateReservation := map[string]interface{}{
		"table":     "reservation",
		"columns":   "start_date, end_date",
		"value":     fmt.Sprintf("%s 08:00, %s 17:00", startDate, effectiveEndDate),
		"condition": fmt.Sprintf("id = %d", ref.ID),
	}

	updateReservationJSON, _ := json.Marshal(updateReservation)
	updateResp := putReq(updateReservationJSON, apiKey, client, w)
	if updateResp == nil || updateResp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error updating reservation dates for reservationID %d", ref.ID), 1, apiKey, client, w)
		return nil, fmt.Errorf("failed to update reservation dates for reservation %d", ref.ID)
	}
	updateResp.Body.Close()

	if err := updateReservationUserContract(ref.ID, userID, newContractID, apiKey, client, w); err != nil {
		return nil, err
	}

	return warnings, nil
}

func usernameExists(username string, apiKey string, client *http.Client, w http.ResponseWriter) bool {
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": fmt.Sprintf("username = '%s'", username),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		return false
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	println("[usernameExists] Rows found:", len(rows))
	return len(rows) > 0
}

func sanitizeUsername(s string) string {
	s = strings.ToLower(s)
	replacements := map[string]string{
		"ß": "ss",
		"æ": "ae",
		"œ": "oe",
		"ð": "d",
		"þ": "th",
		"ł": "l",
		"ø": "o",
		"đ": "d",
		"ħ": "h",
		"ı": "i",
		"ĸ": "k",
		"ŧ": "t",
		"ñ": "n",
	}
	for k, v := range replacements {
		s = strings.ReplaceAll(s, k, v)
	}

	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "'", "")

	re := regexp.MustCompile(`[^a-z]`)
	out := re.ReplaceAllString(s, "")

	return out

}

// manages update of education
func saveEducation(userID int, education []map[string]string, apiKey string, client *http.Client, w http.ResponseWriter) error {

	// elimina entradas previas
	del := map[string]interface{}{
		"table":     "people_grade",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delJSON, _ := json.Marshal(del)

	delResp := deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		delResp.Body.Close()
	} else {
		createLog(fmt.Sprintf("Error deleting old education records for userID %d", userID), 1, apiKey, client, w)
	}

	filtered := []map[string]string{}

	important := []string{
		"grade_master_doctorate",
		"grade_code",
		"gradeName",
		"grade_university_name",
		"graduation_year",
		"graduation_country",
	}

	for _, c := range education {

		isEmpty := true

		for _, key := range important {
			v := c[key]

			if strings.TrimSpace(v) != "" {
				isEmpty = false
			}
		}

		if !isEmpty {
			filtered = append(filtered, c)
		}
	}

	// si no hay educación, terminamos
	if len(filtered) == 0 {
		return nil
	}

	// inserta nuevas entradas
	for i, c := range education {
		fields := []string{"people_id"}
		values := []string{fmt.Sprintf("%d", userID)}

		for k, v := range c {
			if v == "" {
				continue
			}

			switch k {
			case "grade_code":
				fields = append(fields, "code")
			case "grade_university_name":
				v = escapeComma(v)
				fields = append(fields, "universityName")
			case "grade_master_doctorate", "gradeName":
				v = escapeComma(v)
				fields = append(fields, k)
			default:
				fields = append(fields, k)
			}

			values = append(values, v)
		}

		columnsStr := strings.Join(fields, ", ")
		valuesStr := strings.Join(values, ", ")

		insert := map[string]interface{}{
			"table":   "people_grade",
			"columns": columnsStr,
			"value":   valuesStr,
		}

		insertJSON, _ := json.Marshal(insert)

		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error inserting education record #%d for userID %d", i+1, userID), 1, apiKey, client, w)
			return fmt.Errorf("postReq devolvió nil al insertar education para user %d", userID)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting education record #%d for userID %d", i+1, userID), 1, apiKey, client, w)
			return fmt.Errorf("failed to insert education for user %d: %s", userID, string(body))
		}
	}

	return nil
}

// manages update of phd
func savePhd(userID int, phdData []map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) error {
	// deletes old responsible entries
	delResp := map[string]interface{}{
		"table":     "responsible",
		"condition": fmt.Sprintf("tesis_id IN (SELECT id FROM people_phd WHERE people_id = %d)", userID),
	}
	delRespJSON, _ := json.Marshal(delResp)
	deleteReq(delRespJSON, apiKey, client, w)
	// deletes old phd entries
	delPhd := map[string]interface{}{
		"table":     "people_phd",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delPhdJSON, _ := json.Marshal(delPhd)
	deleteReq(delPhdJSON, apiKey, client, w)

	filtered := []map[string]interface{}{}

	important := []string{"phd_program"}

	for _, c := range phdData {

		isEmpty := true

		for _, key := range important {
			raw := c[key]

			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				isEmpty = false
			}
		}

		if !isEmpty {
			filtered = append(filtered, c)
		}
	}

	// si no hay phd, terminamos
	if len(filtered) == 0 {
		return nil
	}
	// inserts new entries
	for _, c := range filtered {
		// separate phd from responsible
		phdFields := []string{"people_id"}
		phdValues := []string{fmt.Sprintf("%d", userID)}

		var responsibles []map[string]interface{}

		for k, v := range c {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			if k == "responsibles" {
				if arr, ok := v.([]interface{}); ok {
					for _, r := range arr {
						if rmap, ok := r.(map[string]interface{}); ok {
							responsibles = append(responsibles, rmap)
						}
					}
				}
				continue
			}
			// skip responsible fields
			switch k {
			case "ip_or_tutor", "name", "people_id":
			case "phd_program", "phd_tesisTitle":
				phdFields = append(phdFields, k)
				phdValues = append(phdValues, escapeComma(val))
			default:
				phdFields = append(phdFields, k)
				phdValues = append(phdValues, val)
			}
		}
		// insert phd
		insertPhd := map[string]interface{}{
			"table":   "people_phd",
			"columns": strings.Join(phdFields, ", "),
			"value":   strings.Join(phdValues, ", "),
		}
		insertPhdJSON, _ := json.Marshal(insertPhd)
		resp := postReq(insertPhdJSON, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting PhD record for userID %d", userID), 1, apiKey, client, w)
			return fmt.Errorf("failed to insert phd for user %d", userID)
		}
		resp.Body.Close()

		// get phd_id to insert responsible
		getID := map[string]interface{}{
			"table":   "people_phd",
			"columns": "last_insert_rowid() AS id",
		}
		idJSON, _ := json.Marshal(getID)
		idResp := getReq(idJSON, apiKey, client, w)
		if idResp == nil {
			createLog(fmt.Sprintf("Error retrieving last inserted PhD ID for userID %d", userID), 1, apiKey, client, w)
			return fmt.Errorf("failed to get phd id for user %d", userID)
		}
		defer idResp.Body.Close()

		var rows []map[string]interface{}
		if err := json.NewDecoder(idResp.Body).Decode(&rows); err != nil || len(rows) == 0 {
			createLog(fmt.Sprintf("Error decoding PhD ID for userID %d: %v", userID, err), 1, apiKey, client, w)
			return fmt.Errorf("error reading phd id for user %d", userID)
		}
		tesisID := int(rows[0]["id"].(float64))

		// insert responsibles
		for _, r := range responsibles {

			// --- No insertar responsables vacíos ---
			hasContent := false
			for key, val := range r {
				if key == "tesis_id" {
					continue
				}
				if strings.TrimSpace(fmt.Sprintf("%v", val)) != "" {
					hasContent = true
					break
				}
			}
			if !hasContent {
				continue
			}

			fields := []string{"tesis_id"}
			values := []string{fmt.Sprintf("%d", tesisID)}

			for key, val := range r {
				s := strings.TrimSpace(fmt.Sprintf("%v", val))
				if s != "" {
					fields = append(fields, key)
					values = append(values, s)
				}
			}

			insertResp := map[string]interface{}{
				"table":   "responsible",
				"columns": strings.Join(fields, ", "),
				"value":   strings.Join(values, ", "),
			}
			insertRespJSON, _ := json.Marshal(insertResp)
			resp := postReq(insertRespJSON, apiKey, client, w)
			if resp != nil {
				resp.Body.Close()
			} else {
				createLog(fmt.Sprintf("Error inserting responsible entry for userID %d", userID), 1, apiKey, client, w)
			}
		}
	}

	return nil
}

// manages update of people_groups
func saveGroups(userID int, groups []map[string]string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	// deletes old entries
	del := map[string]interface{}{
		"table":     "people_group",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delJSON, _ := json.Marshal(del)
	delResp := deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		delResp.Body.Close()
	} else {
		createLog(fmt.Sprintf("Error deleting old groups for userID %d", userID), 1, apiKey, client, w)
	}
	// correspondence between names
	fieldMap := map[string]string{
		"group_intern_code": "group_intern_code",
		"group_start_date":  "start_date",
		"group_end_date":    "end_date",
		"group_ip":          "ip",
	}

	// insert new entries
	for _, c := range groups {
		rawCode, ok := c["group_intern_code"]
		if !ok || strings.TrimSpace(rawCode) == "" || rawCode == "0" {
			// simplemente ignorar la entrada vacía
			continue
		}
		columns := []string{"people_id"}
		values := []string{fmt.Sprintf("%d", userID)}
		for k, v := range c {
			if dbCol, ok := fieldMap[k]; ok {
				columns = append(columns, dbCol)
				values = append(values, v)
			} else {
				fmt.Printf("Campo no reconocido en groups: %s\n", k)
			}
		}
		insert := map[string]interface{}{
			"table":   "people_group",
			"columns": strings.Join(columns, ", "),
			"value":   strings.Join(values, ", "),
		}
		insertJSON, _ := json.Marshal(insert)
		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting group for userID %d", userID), 1, apiKey, client, w)
			return fmt.Errorf("failed to insert group for user %d", userID)
		}
		resp.Body.Close()
	}
	return nil
}

// manages update of people_training
func saveTraining(userID int, trainings []map[string]string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	// deletes old entrie
	del := map[string]interface{}{
		"table":     "people_training",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	delJSON, _ := json.Marshal(del)
	delResp := deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		defer delResp.Body.Close()
	} else {
		createLog(fmt.Sprintf("Error deleting old trainings for userID %d", userID), 1, apiKey, client, w)
	}

	// creates new entries
	for _, c := range trainings {
		rawTrainingID, ok := c["training_id"]
		if !ok || strings.TrimSpace(rawTrainingID) == "" || rawTrainingID == "0" {
			// NO crear entrada si no hay training_id válido
			continue
		}
		// correspondence between names
		fieldMap := map[string]string{
			"training_id":           "training_id",
			"training_enrolled":     "enrolled",
			"training_date":         "date",
			"training_file_path":    "diploma_path",
			"training_diploma_path": "diploma_path",
		}
		columns := []string{"people_id"}
		values := []string{fmt.Sprintf("%d", userID)}
		for k, v := range c {
			if mapped, ok := fieldMap[k]; ok {

				columns = append(columns, mapped)
				values = append(values, v)
			}
		}
		insert := map[string]interface{}{
			"table":   "people_training",
			"columns": strings.Join(columns, ", "),
			"value":   strings.Join(values, ", "),
		}
		insertJSON, _ := json.Marshal(insert)
		resp := postReq(insertJSON, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting training for userID %d", userID), 1, apiKey, client, w)
			return fmt.Errorf("failed to insert training for user %d", userID)
		}
		resp.Body.Close()
	}
	return nil
}

// ------------------- add or update user CONTROLLER -------------------------
func handleSaveUser(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		createLog(fmt.Sprintf("Error parsing multipart form: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}
	var payload struct {
		ID       *int                   `json:"id"`
		FormData map[string]interface{} `json:"formData"`
	}
	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		http.Error(w, "Missing payload data", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		createLog("Error parsing payload in handleSaveUser", 1, apiKey, client, w)
		return
	}

	var userID int
	var isNew = payload.ID == nil

	// saving profile picture
	file, handler, err := r.FormFile("profile_picture")
	if err == nil && handler != nil {
		defer file.Close()
		tempFile, err := os.CreateTemp("", "profile-*"+filepath.Ext(handler.Filename))
		if err != nil {
			http.Error(w, "No se pudo crear archivo temporal", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error updating profile picture for userID %d", userID), 1, apiKey, client, w)
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := io.Copy(tempFile, file); err != nil {
			http.Error(w, "No se pudo copiar la imagen", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error updating profile picture for userID %d", userID), 1, apiKey, client, w)
			return
		}
		if g, ok := payload.FormData["general"].(map[string]interface{}); ok {
			g["profile_image"] = tempFile.Name()
		}
	}

	// saving general data
	userID, err = saveOrUpdatePeople(payload, apiKey, client, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		createLog(fmt.Sprintf("Error inserting general information for userID %d", userID), 1, apiKey, client, w)

		return
	}

	// saving nationalities
	if raw, ok := payload.FormData["nationality"].([]interface{}); ok {
		nationalities := []map[string]string{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				n := map[string]string{}
				for k, v := range m {
					n[k] = fmt.Sprintf("%v", v)
				}
				nationalities = append(nationalities, n)
			}
		}

		if err := saveNationalities(userID, nationalities, apiKey, client, w); err != nil {
			http.Error(w, "Failed to save nationalities", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting nationalities for userID %d", userID), 1, apiKey, client, w)
			return
		}
	}

	// saving residences
	if raw, ok := payload.FormData["residence"].([]interface{}); ok {
		residences := []map[string]string{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				n := map[string]string{}
				for k, v := range m {
					n[k] = fmt.Sprintf("%v", v)
				}
				residences = append(residences, n)
			}
		}

		if err := saveResidences(userID, residences, apiKey, client, w); err != nil {
			http.Error(w, "Failed to save residences", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting nationalities for userID %d", userID), 1, apiKey, client, w)
			return
		}
	}

	// saving contract files
	for key, headers := range r.MultipartForm.File {
		if strings.HasPrefix(key, "contract_") {
			parts := strings.Split(key, "_")
			if len(parts) < 2 {
				continue
			}
			index, err := strconv.Atoi(parts[1])
			if err != nil || index < 0 {
				continue
			}
			for _, header := range headers {
				file, err := header.Open()
				if err != nil {
					http.Error(w, "Error opening contract file", http.StatusInternalServerError)
					createLog(fmt.Sprintf("Error saving contract file for userID %d", userID), 1, apiKey, client, w)
					return
				}
				defer file.Close()
				tempFile, err := os.CreateTemp("", "contract-*"+filepath.Ext(header.Filename))
				if err != nil {
					http.Error(w, "Error creating temp file", http.StatusInternalServerError)
					createLog(fmt.Sprintf("Error saving contract file for userID %d", userID), 1, apiKey, client, w)
					return
				}
				io.Copy(tempFile, file)
				tempFile.Close()
				var name, surname string
				if general, ok := payload.FormData["general"].(map[string]interface{}); ok {
					if v, ok := general["people_name"].(string); ok {
						name = v
					}
					if v, ok := general["surname"].(string); ok {
						surname = v
					}
				}
				newPath, err := saveContractFile(userID, name, surname, tempFile.Name())
				if err != nil {
					http.Error(w, "Error saving contract file", http.StatusInternalServerError)
					createLog(fmt.Sprintf("Error saving contract file for userID %d", userID), 1, apiKey, client, w)
					return
				}
				if rawContracts, ok := payload.FormData["contract"].([]interface{}); ok && index < len(rawContracts) {
					if m, ok := rawContracts[index].(map[string]interface{}); ok {
						m["contract_file_path"] = newPath
					}
				}
			}
		}
	}

	deskWarnings := []DeskOverlapWarning{}

	// saving contracts
	if raw, ok := payload.FormData["contract"].([]interface{}); ok {
		contracts := []map[string]string{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				n := map[string]string{}
				for k, v := range m {
					if k == "supervisors" {
						b, err := json.Marshal(v)
						if err != nil {
							n[k] = "[]"
						} else {
							n[k] = string(b)
						}
						continue
					}

					n[k] = fmt.Sprintf("%v", v)
				}
				contracts = append(contracts, n)
			}
		}

		warnings, err := saveContracts(userID, contracts, apiKey, client, w)
		if err != nil {
			http.Error(w, "Failed to save contracts", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting contract for userID %d", userID), 1, apiKey, client, w)
			return
		}
		deskWarnings = append(deskWarnings, warnings...)
	}

	// saving education
	if raw, ok := payload.FormData["education"].([]interface{}); ok {
		education := []map[string]string{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				n := map[string]string{}
				for k, v := range m {
					n[k] = fmt.Sprintf("%v", v)
				}
				nonEmpty := false
				for _, v := range n {
					if strings.TrimSpace(v) != "" {
						nonEmpty = true
						break
					}
				}
				if nonEmpty {
					education = append(education, n)
				}
			}
		}

		if err := saveEducation(userID, education, apiKey, client, w); err != nil {
			http.Error(w, "Failed to save education", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting education for userID %d", userID), 1, apiKey, client, w)
			return
		}
	}

	// saving phd
	if raw, ok := payload.FormData["phd"].([]interface{}); ok {
		phd := []map[string]interface{}{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				nonEmpty := false
				for _, v := range m {
					switch vv := v.(type) {
					case string:
						if strings.TrimSpace(vv) != "" {
							nonEmpty = true
						}
					case []interface{}:
						if len(vv) > 0 {
							nonEmpty = true
						}
					case map[string]interface{}:
						if len(vv) > 0 {
							nonEmpty = true
						}
					}
					if nonEmpty {
						break
					}
				}

				if nonEmpty {
					phd = append(phd, m)
				}
			}
		}
		if err := savePhd(userID, phd, apiKey, client, w); err != nil {
			http.Error(w, "Failed to save phd", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting contract for userID %d", userID), 1, apiKey, client, w)
			return
		}
	}

	// saving groups
	if raw, ok := payload.FormData["groups"].([]interface{}); ok {
		groups := []map[string]string{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				n := map[string]string{}
				for k, v := range m {
					n[k] = fmt.Sprintf("%v", v)
				}
				groups = append(groups, n)
			}
		}

		if err := saveGroups(userID, groups, apiKey, client, w); err != nil {
			http.Error(w, "Failed to save groups", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting groups for userID %d", userID), 1, apiKey, client, w)
			return
		}
	}

	// saving training diplomas
	for key, headers := range r.MultipartForm.File {
		if strings.HasPrefix(key, "training_") {
			parts := strings.Split(key, "_")
			if len(parts) < 2 {
				continue
			}
			index, err := strconv.Atoi(parts[1])
			if err != nil || index < 0 {
				continue
			}
			for _, header := range headers {
				file, err := header.Open()
				if err != nil {
					http.Error(w, "Error opening training file", http.StatusInternalServerError)
					createLog(fmt.Sprintf("Error opening training file for userID %d", userID), 1, apiKey, client, w)
					return
				}
				defer file.Close()

				tempFile, err := os.CreateTemp("", "training-*"+filepath.Ext(header.Filename))
				if err != nil {
					http.Error(w, "Error creating temp file", http.StatusInternalServerError)
					createLog(fmt.Sprintf("Error creating temporary file for userID %d", userID), 1, apiKey, client, w)
					return
				}
				io.Copy(tempFile, file)
				tempFile.Close()
				var name, surname string
				if general, ok := payload.FormData["general"].(map[string]interface{}); ok {
					if v, ok := general["people_name"].(string); ok {
						name = v
					}
					if v, ok := general["surname"].(string); ok {
						surname = v
					}
				}
				trainingID := fmt.Sprintf("training_%d", index)
				if rawTrainings, ok := payload.FormData["training"].([]interface{}); ok && index < len(rawTrainings) {
					if m, ok := rawTrainings[index].(map[string]interface{}); ok {
						if val, ok := m["training_id"]; ok {
							trainingID = fmt.Sprintf("%v", val)
						}
					}
				}
				newPath, err := saveTrainingFile(userID, name, surname, tempFile.Name(), trainingID)
				if err != nil {
					http.Error(w, "Error saving training file", http.StatusInternalServerError)
					createLog(fmt.Sprintf("Error saving training file for userID %d", userID), 1, apiKey, client, w)
					return
				}
				if rawTrainings, ok := payload.FormData["training"].([]interface{}); ok && index < len(rawTrainings) {
					if m, ok := rawTrainings[index].(map[string]interface{}); ok {
						m["training_file_path"] = newPath
					}
				}
			}
		}
	}

	//saving trainings
	if raw, ok := payload.FormData["training"].([]interface{}); ok {
		trainings := []map[string]string{}
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				n := map[string]string{}
				for k, v := range m {
					n[k] = fmt.Sprintf("%v", v)
				}
				trainings = append(trainings, n)
			}
		}

		if err := saveTraining(userID, trainings, apiKey, client, w); err != nil {
			http.Error(w, "Failed to save trainings", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Error inserting trainings for userID %d", userID), 1, apiKey, client, w)
			return
		}
	}
	_, username, _ := getUserInfo(apiKey, client, w, r)
	createLog(fmt.Sprintf("People entry %d correctly saved by user %s", userID, username), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                "ok",
		"people_id":             userID,
		"new":                   isNew,
		"desk_overlap_warnings": deskWarnings,
	})
}

// -------------------------------- MANAGE GROUPS ---------------------------------------
func handleGetAllGroups(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "researchGroup g LEFT JOIN uneix_groupReconeixement r ON g.research_code = r.codi_grupRecerca",
		"columns": "g.intern_code, g.research_code, g.name, g.former_name, g.start_date, g.end_date, g.outdated, g.project, g.sgr, g.type, g.category, g.cif, g.character, g.typology, r.codi_reconeixement, r.data_obtencio",
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog("Error: nil response in handleGetAllGroups for user "+username, 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve groups", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding groups response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleGetGroupMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	groupCode := r.URL.Query().Get("id")
	if groupCode == "" {
		createLog("Error: Missing groupCode in handleGetGroupMembers for user "+username, 1, apiKey, client, w)
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}
	query := map[string]interface{}{
		"table":     "people_group pg INNER JOIN people p ON pg.people_id = p.id",
		"columns":   "pg.start_date, pg.end_date, pg.ip, p.picture_path, p.name, p.surname, p.id",
		"condition": fmt.Sprintf("pg.group_intern_code = '%s'", groupCode),
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleGetGroupMembers for group %s and user %s", groupCode, username), 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve group members", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding group members for group %s and user %s: %v", groupCode, username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	for i := range result {
		if result[i]["name"] != nil {
			if nameStr, ok := result[i]["name"].(string); ok {
				result[i]["name"] = unescapeComma(nameStr)
			}
		}
		if result[i]["surname"] != nil {
			if surnameStr, ok := result[i]["surname"].(string); ok {
				result[i]["surname"] = unescapeComma(surnameStr)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func buildInsertValues(g Group, includeResearchCode bool) (string, string) {
	cols := []string{}
	vals := []string{}

	if includeResearchCode && g.ResearchCode != "" {
		cols = append(cols, "research_code")
		vals = append(vals, g.ResearchCode)
	}
	if g.InternCode != "" {
		cols = append(cols, "intern_code")
		vals = append(vals, g.InternCode)
	}
	if g.Name != "" {
		cols = append(cols, "name")
		vals = append(vals, g.Name)
	}
	if g.FormerName != "" {
		cols = append(cols, "former_name")
		vals = append(vals, g.FormerName)
	}
	if g.StartDate != "" {
		cols = append(cols, "start_date")
		vals = append(vals, g.StartDate)
	}
	if g.EndDate != "" {
		cols = append(cols, "end_date")
		vals = append(vals, g.EndDate)
	}
	if g.Type != "" {
		cols = append(cols, "type")
		vals = append(vals, g.Type)
	}
	if g.Category != "" {
		cols = append(cols, "category")
		vals = append(vals, "unit")
	} else if g.Project == 0 && g.Sgr == 0 {
		cols = append(cols, "category")
		vals = append(vals, "group")
	} else if g.Project == 1 && g.Sgr == 0 {
		cols = append(cols, "category")
		vals = append(vals, "project")
	} else if g.Project == 0 && g.Sgr == 1 {
		cols = append(cols, "category")
		vals = append(vals, "sgr")
	} else if g.Project == 1 && g.Sgr == 1 {
		cols = append(cols, "category")
		vals = append(vals, "sgr")
	}
	if g.Cif != "" {
		cols = append(cols, "cif")
		vals = append(vals, g.Cif)
	}
	if g.Character != "" {
		cols = append(cols, "character")
		vals = append(vals, g.Character)
	}
	if g.Typology != "" {
		cols = append(cols, "typology")
		vals = append(vals, g.Typology)
	}
	// siempre incluimos outdated y project aunque sean 0
	cols = append(cols, "outdated")
	vals = append(vals, fmt.Sprintf("%d", g.Outdated))
	cols = append(cols, "project")
	vals = append(vals, fmt.Sprintf("%d", g.Project))
	cols = append(cols, "sgr")
	vals = append(vals, fmt.Sprintf("%d", g.Sgr))

	return strings.Join(cols, ", "), strings.Join(vals, ", ")
}

func buildUpdateParts(g Group) (string, string) {
	cols := []string{}
	vals := []string{}

	if g.InternCode != "" {
		cols = append(cols, "intern_code")
		vals = append(vals, g.InternCode)
	}
	if g.Name != "" {
		cols = append(cols, "name")
		vals = append(vals, g.Name)
	}
	if g.FormerName != "" {
		cols = append(cols, "former_name")
		vals = append(vals, g.FormerName)
	}
	if g.StartDate != "" {
		cols = append(cols, "start_date")
		vals = append(vals, g.StartDate)
	}
	if g.EndDate != "" {
		cols = append(cols, "end_date")
		vals = append(vals, g.EndDate)
	}
	if g.Type != "" {
		cols = append(cols, "type")
		vals = append(vals, g.Type)
	}
	if g.Category != "" {
		cols = append(cols, "category")
		vals = append(vals, "unit")
	} else if g.Project == 0 && g.Sgr == 0 {
		cols = append(cols, "category")
		vals = append(vals, "group")
	} else if g.Project == 1 && g.Sgr == 0 {
		cols = append(cols, "category")
		vals = append(vals, "project")
	} else if g.Project == 0 && g.Sgr == 1 {
		cols = append(cols, "category")
		vals = append(vals, "sgr")
	} else if g.Project == 1 && g.Sgr == 1 {
		cols = append(cols, "category")
		vals = append(vals, "sgr")
	}
	if g.Cif != "" {
		cols = append(cols, "cif")
		vals = append(vals, g.Cif)
	}
	if g.Character != "" {
		cols = append(cols, "character")
		vals = append(vals, g.Character)
	}
	if g.Typology != "" {
		cols = append(cols, "typology")
		vals = append(vals, g.Typology)
	}
	// siempre estos dos aunque sean 0
	cols = append(cols, "outdated")
	vals = append(vals, fmt.Sprintf("%d", g.Outdated))

	cols = append(cols, "project")
	vals = append(vals, fmt.Sprintf("%d", g.Project))

	cols = append(cols, "sgr")
	vals = append(vals, fmt.Sprintf("%d", g.Sgr))

	return strings.Join(cols, ", "), strings.Join(vals, ", ")
}

func handleSaveGroup(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var g Group

	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		createLog(fmt.Sprintf("Error parsing group JSON: %v", err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	exists := false
	if g.ResearchCode != "" {
		check := map[string]interface{}{
			"table":     "researchGroup",
			"columns":   "COUNT(*) as cnt",
			"condition": fmt.Sprintf("research_code = '%s'", g.ResearchCode),
		}
		checkJSON, _ := json.Marshal(check)
		resp := getReq(checkJSON, apiKey, client, w)
		if resp != nil {
			defer resp.Body.Close()
			var result []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && len(result) > 0 {
				if cnt, ok := result[0]["cnt"].(float64); ok && cnt > 0 {
					exists = true
				}
			}
		} else {
			createLog(fmt.Sprintf("Error checking existence for group %s", g.ResearchCode), 1, apiKey, client, w)
		}
	}

	if !exists {
		// --------- INSERT ---------
		cols, vals := buildInsertValues(g, true)
		query := map[string]interface{}{
			"table":   "researchGroup",
			"columns": cols,
			"value":   vals,
		}
		jsonData, _ := json.Marshal(query)
		resp := postReq(jsonData, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting new group %s", g.ResearchCode), 1, apiKey, client, w)
			http.Error(w, "Failed to insert group", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if g.Category != "unit" {
			// Si hay reconocimiento → insert
			if g.CodiReconeixement != "" {
				q2 := map[string]interface{}{
					"table":   "uneix_groupReconeixement",
					"columns": "codi_grupRecerca, codi_reconeixement, data_obtencio, codi_entitat",
					"value":   fmt.Sprintf("%s, %s, %s, 0000001672", g.ResearchCode, g.CodiReconeixement, g.DataObtencio),
				}
				data2, _ := json.Marshal(q2)
				resp2 := postReq(data2, apiKey, client, w)
				if resp2 == nil || resp2.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error inserting recognition for group %s", g.ResearchCode), 1, apiKey, client, w)
				} else {
					resp2.Body.Close()
				}
			} else {
				q2 := map[string]interface{}{
					"table":   "uneix_groupReconeixement",
					"columns": "codi_grupRecerca, codi_entitat",
					"value":   fmt.Sprintf("%s, 0000001672", g.ResearchCode),
				}
				data2, _ := json.Marshal(q2)
				resp2 := postReq(data2, apiKey, client, w)
				if resp2 == nil || resp2.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error inserting recognition for group %s", g.ResearchCode), 1, apiKey, client, w)
				} else {
					resp2.Body.Close()
				}
			}
		}

	} else {
		// --------- UPDATE ---------
		cols, vals := buildUpdateParts(g)

		query := map[string]interface{}{
			"table":     "researchGroup",
			"columns":   cols,
			"value":     vals,
			"condition": fmt.Sprintf("research_code = '%s'", g.ResearchCode),
		}

		jsonData, _ := json.Marshal(query)

		resp := putReq(jsonData, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error updating group %s", g.ResearchCode), 1, apiKey, client, w)
			http.Error(w, "Failed to update group", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		createLog(fmt.Sprintf("Group %s successfully updated", g.ResearchCode), 0, apiKey, client, w)

		if g.Category != "unit" {
			// Borrar reconocimiento viejo
			del := map[string]interface{}{
				"table":     "uneix_groupReconeixement",
				"condition": fmt.Sprintf("codi_grupRecerca = '%s'", g.ResearchCode),
			}
			delJSON, _ := json.Marshal(del)
			delResp := deleteReq(delJSON, apiKey, client, w)
			if delResp == nil {
				createLog(fmt.Sprintf("Error deleting old recognition for group %s", g.ResearchCode), 1, apiKey, client, w)
			} else {
				delResp.Body.Close()
			}

			// Insertar de nuevo si hay
			if g.CodiReconeixement != "" {
				q2 := map[string]interface{}{
					"table":   "uneix_groupReconeixement",
					"columns": "codi_grupRecerca, codi_reconeixement, data_obtencio, codi_entitat",
					"value":   fmt.Sprintf("%s, %s, %s, 0000001672", g.ResearchCode, g.CodiReconeixement, g.DataObtencio),
				}
				data2, _ := json.Marshal(q2)
				resp2 := postReq(data2, apiKey, client, w)
				if resp2 == nil || resp2.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error reinserting recognition for group %s", g.ResearchCode), 1, apiKey, client, w)
				} else {
					resp2.Body.Close()
				}
			} else {
				q2 := map[string]interface{}{
					"table":   "uneix_groupReconeixement",
					"columns": "codi_grupRecerca, codi_entitat",
					"value":   fmt.Sprintf("%s, 0000001672", g.ResearchCode),
				}
				data2, _ := json.Marshal(q2)
				resp2 := postReq(data2, apiKey, client, w)
				if resp2 == nil || resp2.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error inserting recognition for group %s", g.ResearchCode), 1, apiKey, client, w)
				} else {
					resp2.Body.Close()
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleSaveGroupMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB límite
		createLog(fmt.Sprintf("Error parsing form in handleSaveGroupMembers: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	groupID := r.FormValue("group_intern_code")
	start := r.FormValue("start_date")
	end := r.FormValue("end_date")
	ip := r.FormValue("ip")

	if peopleID == "undefined" {
		createLog("Error: people_id undefined in handleSaveGroupMembers", 1, apiKey, client, w)
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	// -------- Construir UPDATE --------
	setCols := []string{}
	setValues := []string{}
	if start != "" {
		setCols = append(setCols, "start_date")
		setValues = append(setValues, start)
	}
	if end != "" {
		setCols = append(setCols, "end_date")
		setValues = append(setValues, end)
	}
	if ip != "" {
		setCols = append(setCols, "ip")
		setValues = append(setValues, ip)
	}

	query := map[string]interface{}{
		"table":   "people_group",
		"columns": strings.Join(setCols, ", "),
		"value":   strings.Join(setValues, ", "),
		"condition": fmt.Sprintf(
			"people_id = '%s' AND group_intern_code = '%s'",
			peopleID, groupID,
		),
	}

	jsonData, _ := json.Marshal(query)

	resp := putReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error updating member %s in group %s", peopleID, groupID), 1, apiKey, client, w)
		http.Error(w, "Failed to update group member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDeleteGroupMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB límite
		createLog(fmt.Sprintf("Error parsing form in handleSaveGroupMembers: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	groupID := r.FormValue("group_intern_code")

	if peopleID == "undefined" {
		createLog("Error: people_id undefined in handleSaveGroupMembers", 1, apiKey, client, w)
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table": "people_group",
		"condition": fmt.Sprintf(
			"people_id = '%s' AND group_intern_code = '%s'",
			peopleID, groupID,
		),
	}

	jsonData, _ := json.Marshal(query)

	resp := deleteReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error deleting member %s in group %s", peopleID, groupID), 1, apiKey, client, w)
		http.Error(w, "Failed to delete group member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleAddGroupMember(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB límite
		createLog(fmt.Sprintf("Error parsing form in handleAddGroupMember: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	groupID := r.FormValue("group_id")
	start := r.FormValue("start_date")
	end := r.FormValue("end_date")
	ip := r.FormValue("ip")

	// -------- Construir columnas y valores dinámicamente --------
	cols := []string{"people_id", "group_intern_code"}
	vals := []string{peopleID, groupID}

	if start != "" {
		cols = append(cols, "start_date")
		vals = append(vals, start)
	}
	if end != "" {
		cols = append(cols, "end_date")
		vals = append(vals, end)
	}
	if ip != "" {
		cols = append(cols, "ip")
		vals = append(vals, ip)
	}
	query := map[string]interface{}{
		"table":   "people_group",
		"columns": strings.Join(cols, ", "),
		"value":   strings.Join(vals, ", "),
	}

	jsonData, _ := json.Marshal(query)
	resp := postReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error adding member %s to group %s", peopleID, groupID), 1, apiKey, client, w)
		http.Error(w, "Failed to add group member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleGetAvailableUsersForGroup(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("id")
	if groupID == "" {
		createLog("Error: missing id in handleGetAvailableUsersForGroup", 1, apiKey, client, w)
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// Query: users from "people" that are NOT in this group
	query := map[string]interface{}{
		"table":     "people p LEFT JOIN people_group pt ON p.id = pt.people_id AND pt.group_intern_code = '" + groupID + "'",
		"columns":   "p.id, p.name, p.surname",
		"condition": "pt.group_intern_code IS NULL", // means not enrolled
	}

	jsonData, _ := json.Marshal(query)

	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error retrieving available users for group %s", groupID), 1, apiKey, client, w)
		http.Error(w, "No response from DB API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding available users for group %s: %v", groupID, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	// Return the JSON list
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// -------------------------------- MANAGE TRAININGS ---------------------------------------
func handleGetAllTrainings(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "training",
		"columns": "*",
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog("Error: nil response in handleGetAllTrainings for user "+username, 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve trainings", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding trainings for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleSaveTraining(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	var t Training
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		createLog(fmt.Sprintf("Error decoding training JSON for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	//Save new training
	if t.ID == "0" || t.ID == "" {
		// -------- INSERT --------
		query := map[string]interface{}{
			"table":   "training",
			"columns": "name, category, hours",
			"value":   fmt.Sprintf("%s, %s, %d", t.Name, t.Category, t.Hours),
		}
		jsonData, _ := json.Marshal(query)
		resp := postReq(jsonData, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting training for user %s: %+v", username, t), 1, apiKey, client, w)
			http.Error(w, "Failed to insert training", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		createLog(fmt.Sprintf("New training '%s' successfully inserted for user %s", t.Name, username), 0, apiKey, client, w)
		//Update existing training
	} else {
		// -------- UPDATE --------
		query := map[string]interface{}{
			"table":     "training",
			"columns":   "name, category, hours",
			"value":     fmt.Sprintf("%s, %s, %d", t.Name, t.Category, t.Hours),
			"condition": fmt.Sprintf("id = %s", t.ID),
		}
		jsonData, _ := json.Marshal(query)
		resp := putReq(jsonData, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error updating training ID %s for user %s", t.ID, username), 1, apiKey, client, w)
			http.Error(w, "Failed to update training", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		createLog(fmt.Sprintf("Training ID %s successfully updated for user %s", t.ID, username), 0, apiKey, client, w)

	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// deletes training and related information
func handleDeleteTraining(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var body deleteReqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		createLog(fmt.Sprintf("Error decoding deleteTraining JSON: %v", err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// deletes people_training
	del := map[string]interface{}{
		"table":     "people_training",
		"condition": fmt.Sprintf("training_id = %s", body.ID),
	}
	delJSON, _ := json.Marshal(del)
	delResp := deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		delResp.Body.Close()
	} else {
		createLog(fmt.Sprintf("Error deleting related people_training for training ID %s", body.ID), 1, apiKey, client, w)
	}

	// deletes training
	del = map[string]interface{}{
		"table":     "training",
		"condition": fmt.Sprintf("id = %s", body.ID),
	}
	delJSON, _ = json.Marshal(del)
	delResp = deleteReq(delJSON, apiKey, client, w)
	if delResp != nil {
		delResp.Body.Close()
		createLog(fmt.Sprintf("Training ID %s successfully deleted", body.ID), 0, apiKey, client, w)

	} else {
		createLog(fmt.Sprintf("Error deleting training ID %s", body.ID), 1, apiKey, client, w)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleGetTrainingMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	groupCode := r.URL.Query().Get("id")

	query := map[string]interface{}{
		"table":     "people_training pt INNER JOIN people p ON pt.people_id = p.id",
		"columns":   "pt.enrolled, pt.date, pt.diploma_path, p.picture_path, p.name, p.surname, p.secondSurname, p.id",
		"condition": fmt.Sprintf("pt.training_id = %s", groupCode),
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response fetching training members for training %s", groupCode), 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve training members", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding members for training %s: %v", groupCode, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	for i := range result {
		if result[i]["name"] != nil {
			if nameStr, ok := result[i]["name"].(string); ok {
				result[i]["name"] = unescapeComma(nameStr)
			}
		}
		if result[i]["surname"] != nil {
			if surnameStr, ok := result[i]["surname"].(string); ok {
				result[i]["surname"] = unescapeComma(surnameStr)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleSaveTrainingMembers updates a user's training info (date, diploma)
func handleSaveTrainingMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB límite
		createLog(fmt.Sprintf("Error parsing form in handleSaveTrainingMembers: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	surname := r.FormValue("surname")
	peopleID := r.FormValue("people_id")
	trainingID := r.FormValue("training_id")
	date := r.FormValue("date")

	if peopleID == "undefined" {
		createLog("Error: undefined people_id in handleSaveTrainingMembers", 1, apiKey, client, w)
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	// -------- Guardar diploma si viene un archivo --------
	diplomaPath := ""
	file, header, err := r.FormFile("diploma")
	if err == nil {
		defer file.Close()

		tempFile, err := os.CreateTemp("", "diploma-*"+filepath.Ext(header.Filename))
		if err != nil {
			createLog(fmt.Sprintf("Error creating temp file for diploma (people %s)", peopleID), 1, apiKey, client, w)
			http.Error(w, "Error creating temp file", http.StatusInternalServerError)
			return
		}
		io.Copy(tempFile, file)
		tempFile.Close()

		newPath, err := saveTrainingFile(
			atoiSafe(peopleID),
			name, surname,
			tempFile.Name(),
			trainingID,
		)
		if err != nil {
			createLog(fmt.Sprintf("Error saving diploma for training %s (people %s): %v", trainingID, peopleID, err), 1, apiKey, client, w)
			http.Error(w, "Error saving diploma file", http.StatusInternalServerError)
			return
		}
		diplomaPath = newPath
	}

	// -------- Construir UPDATE --------
	setCols := []string{}
	setValues := []string{}
	if date != "" {
		setCols = append(setCols, "date")
		setValues = append(setValues, date)
	}
	if diplomaPath != "" {
		setCols = append(setCols, "diploma_path")
		setValues = append(setValues, diplomaPath)
	}

	query := map[string]interface{}{
		"table":   "people_training",
		"columns": strings.Join(setCols, ", "),
		"value":   strings.Join(setValues, ", "),
		"condition": fmt.Sprintf(
			"people_id = %s AND training_id = %s",
			peopleID, trainingID,
		),
	}

	jsonData, _ := json.Marshal(query)
	resp := putReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error updating training member %s in training %s", peopleID, trainingID), 1, apiKey, client, w)
		http.Error(w, "Failed to update training member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	createLog(fmt.Sprintf("Training member %s successfully updated in training %s", peopleID, trainingID), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// Helper seguro para parsear IDs
func atoiSafe(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// handleAddTrainingMember inserts a new user into a training
func handleAddTrainingMember(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB límite
		createLog(fmt.Sprintf("Error parsing form in handleAddTrainingMember: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	trainingID := r.FormValue("training_id")
	date := r.FormValue("date")
	name := r.FormValue("name")
	surname := r.FormValue("surname")

	// -------- Guardar diploma si viene un archivo --------
	diplomaPath := ""
	file, header, err := r.FormFile("diploma")
	if err == nil {
		defer file.Close()
		tempFile, err := os.CreateTemp("", "diploma-*"+filepath.Ext(header.Filename))
		if err != nil {
			createLog(fmt.Sprintf("Error creating temp file for new diploma (people %s)", peopleID), 1, apiKey, client, w)
			http.Error(w, "Error creating temp file", http.StatusInternalServerError)
			return
		}
		io.Copy(tempFile, file)
		tempFile.Close()

		newPath, err := saveTrainingFile(atoiSafe(peopleID), name, surname, tempFile.Name(), trainingID)
		if err != nil {
			createLog(fmt.Sprintf("Error saving new diploma for training %s (people %s)", trainingID, peopleID), 1, apiKey, client, w)
			http.Error(w, "Error saving diploma", http.StatusInternalServerError)
			return
		}
		diplomaPath = newPath
	}

	// -------- Construir columnas y valores dinámicamente --------
	cols := []string{"training_id", "people_id", "enrolled"}
	vals := []string{trainingID, peopleID, "1"} // enrolled siempre 1

	if date != "" {
		cols = append(cols, "date")
		vals = append(vals, date)
	}
	if diplomaPath != "" {
		cols = append(cols, "diploma_path")
		vals = append(vals, diplomaPath)
	}

	query := map[string]interface{}{
		"table":   "people_training",
		"columns": strings.Join(cols, ", "),
		"value":   strings.Join(vals, ", "),
	}

	jsonData, _ := json.Marshal(query)
	resp := postReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error adding training member %s to training %s", peopleID, trainingID), 1, apiKey, client, w)
		http.Error(w, "Failed to add training member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	createLog(fmt.Sprintf("Member %s successfully added to training %s", peopleID, trainingID), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// handleGetAvailableUsers returns users not enrolled in a given training
func handleGetAvailableUsers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	trainingID := r.URL.Query().Get("id")
	if trainingID == "" {
		createLog("Error: missing training_id in handleGetAvailableUsers", 1, apiKey, client, w)
		http.Error(w, "training_id is required", http.StatusBadRequest)
		return
	}

	// Query: users from "people" that are NOT in people_training for this training
	query := map[string]interface{}{
		"table":     "people p LEFT JOIN people_training pt ON p.id = pt.people_id AND pt.training_id = " + trainingID,
		"columns":   "p.id, p.name, p.surname",
		"condition": "pt.training_id IS NULL", // means not enrolled
	}

	jsonData, _ := json.Marshal(query)

	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error retrieving available users for training %s", trainingID), 1, apiKey, client, w)
		http.Error(w, "No response from DB API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding available users for training %s: %v", trainingID, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	// Return the JSON list
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// -------------------------------- MANAGE SPINOFFS ---------------------------------------
// handleGetSpinoffs obtiene todas las spinoffs
func handleGetSpinoffs(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "uneix_spinoffs",
		"columns": "*",
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleGetSpinoffs for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch spinoffs", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Error reading spinoffs body for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// handleSaveSpinoff crea o actualiza una spinoff
func handleSaveSpinoff(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("Error decoding spinoff JSON for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	id := payload["id"]
	delete(payload, "id")

	// Mapear campos
	cols := []string{}
	vals := []string{}
	cols = append(cols, "codi_entitat")
	cols = append(cols, "codi_ens")
	vals = append(vals, "0000001672")
	vals = append(vals, "0000001672")

	for k, v := range payload {
		if v == nil || v == "" {
			continue
		}
		cols = append(cols, k)
		switch vv := v.(type) {
		case string:
			vals = append(vals, vv)
		default:
			vals = append(vals, fmt.Sprintf("%v", vv))
		}
	}

	query := map[string]interface{}{
		"table": "uneix_spinoffs",
	}

	if id == nil || id == "" {
		// INSERT
		query["columns"] = strings.Join(cols, ", ")
		query["value"] = strings.Join(vals, ", ")
		jsonData, _ := json.Marshal(query)
		resp := postReq(jsonData, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error inserting new spinoff for user %s: %+v", username, payload), 1, apiKey, client, w)
			http.Error(w, "Failed to insert spinoff", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		createLog(fmt.Sprintf("New spinoff successfully inserted (name: %v) by user %s", payload["nom"], username), 0, apiKey, client, w)

	} else {
		// UPDATE
		query["columns"] = strings.Join(cols, ", ")
		query["value"] = strings.Join(vals, ", ")
		query["condition"] = fmt.Sprintf("id = %v", id)

		jsonData, _ := json.Marshal(query)
		resp := putReq(jsonData, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error updating spinoff ID %v for user %s", id, username), 1, apiKey, client, w)
			http.Error(w, "Failed to update spinoff", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		createLog(fmt.Sprintf("Spinoff ID %v successfully updated by user %s", id, username), 0, apiKey, client, w)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// handleDeleteSpinoff elimina una spinoff
func handleDeleteSpinoff(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("Error decoding deleteSpinoff JSON for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "uneix_spinoffs",
		"condition": fmt.Sprintf("id = %s", payload.ID),
	}

	jsonData, _ := json.Marshal(query)
	resp := deleteReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error deleting spinoff ID %s for user %s", payload.ID, username), 1, apiKey, client, w)
		http.Error(w, "Failed to delete spinoff", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	createLog(fmt.Sprintf("Spinoff ID %s successfully deleted by user %s", payload.ID, username), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// -------------------------------- NEW HIRES -----------------------------
func handleGetNewHires(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	// get today's date
	today := time.Now().Format("2006-01-02")
	query := map[string]interface{}{
		"table":     "people p LEFT JOIN contract c ON p.id = c.people_id LEFT JOIN newHires_tasks n ON p.id = n.people_id",
		"columns":   "p.id, p.name, p.surname, p.secondSurname, p.people_idExternal, n.desk_request, n.account_request",
		"condition": fmt.Sprintf("c.start_date >= DATE('%s', '-15 day')", today),
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleGetNewHires for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch new hires", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func getStatusClass(status string) string {
	switch strings.ToLower(status) {
	case "in progress":
		return "in-progress"
	case "closed":
		return "closed"
	case "waiting for user":
		return "pending"
	case "assigned":
		return "assigned"
	default:
		return "in-progress"
	}
}

func handleDeskTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	// Obtener información del usuario
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	var payload struct {
		NewHireID int `json:"people_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("createModality: invalid JSON from user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	created := time.Now().Format("2006-01-02 15:04")

	query := map[string]interface{}{
		"table": `people p LEFT JOIN contract c ON p.id = c.people_id 
						LEFT JOIN people_supervisor ps ON c.id = ps.contract_id
						LEFT JOIN people pe ON ps.supervisor_id = pe.id`,
		"columns": "p.name, p.surname, c.position, c.job_category, c.start_date, pe.name as supervisorName, pe.surname as supervisorSurname",
		"condition": fmt.Sprintf(`
					p.id = %d
					AND c.id = (
						SELECT c2.id
						FROM contract c2
						WHERE c2.people_id = p.id
						ORDER BY c2.start_date DESC, c2.id DESC
						LIMIT 1
					)
					`, payload.NewHireID),
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error retrieving new hire info for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve new hire info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding new hire info for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	if len(result) == 0 {
		createLog(fmt.Sprintf("No user found with people_id %d", payload.NewHireID), 1, apiKey, client, w)
		http.Error(w, "No user found", http.StatusBadRequest)
		return
	}

	name := result[0]["name"].(string)
	surname := result[0]["surname"].(string)
	newHire := fmt.Sprintf("%s %s", name, surname)

	intraSupervisor := false
	supervisor := ""

	if result[0]["supervisorName"] != nil {
		intraSupervisor = true
		supervisorName := result[0]["supervisorName"].(string)
		supervisorSurname := result[0]["supervisorSurname"].(string)
		supervisor = fmt.Sprintf("%s %s", supervisorName, supervisorSurname)
	}

	intraJobCategory := false
	jobCategory := ""

	if result[0]["job_category"] != nil {
		intraJobCategory = true
		jobCategory = unescapeComma(result[0]["job_category"].(string))
	}

	intraPosition := false
	position := ""

	if result[0]["position"] != nil {
		intraPosition = true
		position = unescapeComma(result[0]["position"].(string))
	}

	startDateVal := result[0]["start_date"]
	startDate := ""

	switch v := startDateVal.(type) {
	case time.Time:
		startDate = v.Format("2006-01-02")
	case string:
		// "2026-02-23T00:00:00Z" -> "2026-02-23"
		if i := strings.IndexByte(v, 'T'); i != -1 {
			startDate = v[:i]
		} else {
			startDate = v
		}
	default:
		startDate = fmt.Sprint(v)
	}
	department := "IT"
	issue := "New Incorporation"
	description := ""

	if intraJobCategory && intraSupervisor {
		description = escapeComma(fmt.Sprintf("%s will be joining us as %s in the %s category starting from %s. Their supervisor will be %s. Please, make sure to find them an adequate workspace before their arrival.", newHire, position, jobCategory, startDate, supervisor))
	} else if intraPosition && intraSupervisor {
		description = escapeComma(fmt.Sprintf("%s will be joining us as %s starting from %s. Their supervisor will be %s. Please, make sure to find them an adequate workspace before their arrival.", newHire, position, startDate, supervisor))
	} else if intraJobCategory && !intraSupervisor {
		description = escapeComma(fmt.Sprintf("%s will be joining us as %s in the %s category starting from %s. Please, make sure to find them an adequate workspace before their arrival.", newHire, position, jobCategory, startDate))
	} else if intraPosition && !intraSupervisor {
		description = escapeComma(fmt.Sprintf("%s will be joining us as %s starting from %s. Please, make sure to find them an adequate workspace before their arrival.", newHire, position, startDate))
	} else if (!intraPosition || !intraJobCategory) && intraSupervisor {
		description = escapeComma(fmt.Sprintf("%s will be joining us starting from %s. Their supervisor will be %s. Please, make sure to find them an adequate workspace before their arrival.", newHire, startDate, supervisor))
	} else {
		description = escapeComma(fmt.Sprintf("%s will be joining us starting from %s. Please, make sure to find them an adequate workspace before their arrival.", newHire, startDate))
	}

	// DEBUG HARDCODED TO ME
	email := "RRHH@crm.cat"
	//email := "ltorrescusa@crm.cat"

	// enviar mail als encarregats del departament
	managers := map[string]interface{}{
		"table":     "people p LEFT JOIN department_workers d ON p.id = d.worker_id",
		"columns":   "p.crm_email, p.name, p.surname, p.id",
		"condition": "d.department_name = 'IT' AND d.manager = '1'",
	}

	managersMail, _ := json.Marshal(managers)
	idResp := getReq(managersMail, apiKey, client, w)

	if idResp == nil {
		createLog("Failed to retrieve managers mail from database for user "+username, 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve managers mail", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()

	var mails []map[string]interface{}
	err := json.NewDecoder(idResp.Body).Decode(&mails)
	if err != nil {
		createLog("Error leyendo mails: "+err.Error(), 1, apiKey, client, w)
		return
	}

	idResponsableInt := int(mails[0]["id"].(float64))

	query = map[string]interface{}{
		"table":   "tickets",
		"columns": "department, issue, description, email, creation_date, user_id, assigned_to, urgency, status",
		"value": fmt.Sprintf("%s,%s,%s,%s,%s,%d,%d,High,Assigned",
			department, issue, description, email, created, userID, idResponsableInt),
	}

	postJSON, _ := json.Marshal(query)
	resp = postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// crear notificació
	notifBody := escapeComma("Your ticket has been succesfully sent to the department administration. You will be notified for every status change.")
	query = map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Ticket, Ticket created successfully, %s, 304, %s", notifBody, created),
	}

	postJSON, _ = json.Marshal(query)
	resp = postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket notification: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Get notification ID
	getID := map[string]interface{}{
		"table":     "notifications",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s'", created),
	}
	idJSON, _ := json.Marshal(getID)
	idResp = getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new notification ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get notification ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var rows []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&rows)
	notifID := int(rows[0]["id"].(float64))

	// Sends to users creating an entry to notification_user table

	link := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read, can_download, seen",
		"value":   fmt.Sprintf("%d, %d, 1, 0, 0", notifID, userID),
	}
	jsonLink, _ := json.Marshal(link)
	resp = postReq(jsonLink, apiKey, client, w)
	if resp == nil {
		createLog("Failed to send notification", 1, apiKey, client, w)
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	//enviar mail si ha proporcionat un

	// Get ticket ID
	getTicketID := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = %d AND creation_date = '%s'", userID, created),
	}
	idJSON, _ = json.Marshal(getTicketID)
	idResp = getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new ticket ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get ticket ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var ids []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&ids)
	ticketID := int(ids[0]["id"].(float64))

	if email != "" {

		tmplContent, err := os.ReadFile("../module6/module6workers/assets/emailTicketCreation.html")
		if err != nil {
			println("Error leyendo plantilla:", err.Error())
			return
		}

		tmpl, err := template.New("email").Parse(string(tmplContent))
		if err != nil {
			println("Error parseando plantilla:", err.Error())
			return
		}
		data := map[string]string{
			"TicketID":    strconv.Itoa(ticketID),
			"Status":      "Assigned",
			"StatusClass": getStatusClass("Assigned"),
			"UpdatedAt":   created,
			"Message":     unescapeComma(notifBody),
		}

		var buf bytes.Buffer
		tmpl.Execute(&buf, data)
		sendEmail(email, fmt.Sprintf("Workspace for %s requested correctly", newHire), buf.String())
	}

	if len(mails) > 0 {
		// aconseguir nom de qui ha creat el ticket

		// enviar mail als encarregats del departament
		creationUser := map[string]interface{}{
			"table":     "people",
			"columns":   "name, surname",
			"condition": fmt.Sprintf("id = %d", userID),
		}

		creationUserJSON, _ := json.Marshal(creationUser)
		idResp = getReq(creationUserJSON, apiKey, client, w)

		if idResp == nil {
			createLog("Failed to retrieve users name from database", 1, apiKey, client, w)
			http.Error(w, "Failed to retrieve users name", http.StatusInternalServerError)
			return
		}
		defer idResp.Body.Close()

		var names []map[string]interface{}
		json.NewDecoder(idResp.Body).Decode(&names)

		name, _ := names[0]["name"].(string)
		surname, _ := names[0]["surname"].(string)
		createdBy := strings.TrimSpace(name + " " + surname)

		tmplContent, err := os.ReadFile("../module6/module6workers/assets/notifyWorker.html")
		if err != nil {
			createLog("Error leyendo plantilla de managers: "+err.Error(), 1, apiKey, client, w)
			return
		}

		tmpl, err := template.New("email").Parse(string(tmplContent))
		if err != nil {
			createLog("Error parseando plantilla de managers: "+err.Error(), 1, apiKey, client, w)
			return
		}

		for _, m := range mails {
			mail, ok := m["crm_email"].(string)
			if !ok || mail == "" {
				continue
			}

			// nombre del manager
			name, _ := m["name"].(string)
			surname, _ := m["surname"].(string)
			fullName := strings.TrimSpace(name + " " + surname)

			// datos que usará la plantilla
			data := map[string]string{
				"TicketID":     strconv.Itoa(ticketID),
				"Department":   department,
				"Issue":        issue,
				"Description":  unescapeComma(description),
				"CreationDate": created,
				"UserName":     fullName,
				"CreatedBy":    createdBy,
				"Urgency":      "High",
				"UrgencyClass": strings.ToLower("High"),
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				createLog("Error ejecutando plantilla manager: "+err.Error(), 1, apiKey, client, w)
				continue
			}

			// enviar email
			sendEmail(mail, fmt.Sprintf("New Ticket: #%d", ticketID), buf.String())
		}
	}

	// marcar tarea de new hire como completada
	updateTask := map[string]interface{}{
		"table":     "newHires_tasks",
		"columns":   "desk_request, desk_req_time",
		"value":     fmt.Sprintf("1, %s", created),
		"condition": fmt.Sprintf("people_id = %d", payload.NewHireID),
	}
	updateJSON, _ := json.Marshal(updateTask)
	resp = putReq(updateJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to update new hire task", 1, apiKey, client, w)
		http.Error(w, "Failed to update new hire task", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleAccountTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	// Obtener información del usuario
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	var payload struct {
		NewHireID int `json:"people_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("createModality: invalid JSON from user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	created := time.Now().Format("2006-01-02 15:04")

	//fmt.Printf("New hire id", payload.NewHireID)
	query := map[string]interface{}{
		"table": `people p LEFT JOIN contract c ON p.id = c.people_id 
						LEFT JOIN users u ON p.id = u.people_id`,
		"columns": "p.name, p.surname, c.position, c.job_category, c.start_date, u.username as newUsername",
		"condition": fmt.Sprintf(`
					p.id = %d
					AND c.id = (
						SELECT c2.id
						FROM contract c2
						WHERE c2.people_id = p.id
						ORDER BY c2.start_date DESC, c2.id DESC
						LIMIT 1
					)
					`, payload.NewHireID),
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error retrieving new hire info for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve new hire info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	//fmt.Println("Response status:", resp.Status)

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding new hire info for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	if len(result) == 0 {
		createLog(fmt.Sprintf("No user found with people_id %d", payload.NewHireID), 1, apiKey, client, w)
		http.Error(w, "No user found", http.StatusBadRequest)
		return
	}

	name := result[0]["name"].(string)
	surname := result[0]["surname"].(string)
	newHire := fmt.Sprintf("%s %s", name, surname)

	intraUsername := false
	newUsername := ""

	if result[0]["newUsername"] != nil {
		intraUsername = true
		newUsername = result[0]["newUsername"].(string)
	}

	intraPosition := false
	position := ""

	if result[0]["position"] != nil {
		intraPosition = true
		position = unescapeComma(result[0]["position"].(string))
	}

	intraJobCategory := false
	jobCategory := ""

	if result[0]["job_category"] != nil {
		intraJobCategory = true
		jobCategory = unescapeComma(result[0]["job_category"].(string))
	}

	startDateVal := result[0]["start_date"]
	startDate := ""

	switch v := startDateVal.(type) {
	case time.Time:
		startDate = v.Format("2006-01-02")
	case string:
		// "2026-02-23T00:00:00Z" -> "2026-02-23"
		if i := strings.IndexByte(v, 'T'); i != -1 {
			startDate = v[:i]
		} else {
			startDate = v
		}
	default:
		startDate = fmt.Sprint(v)
	}
	department := "IT"
	issue := "New Incorporation"
	description := ""

	if intraUsername && intraJobCategory {
		description = escapeComma(fmt.Sprintf("%s will be joining us in the %s category starting from %s. Their username in CRMIntratools is %s. Please, make sure to create their Microsoft account before their arrival.", newHire, jobCategory, startDate, newUsername))
	} else if intraUsername && intraPosition {
		description = escapeComma(fmt.Sprintf("%s will be joining us as %s starting from %s. Their username in CRMIntratools is %s. Please, make sure to create their Microsoft account before their arrival.", newHire, position, startDate, newUsername))
	} else if intraJobCategory && !intraUsername {
		description = escapeComma(fmt.Sprintf("%s will be joining us in the %s category starting from %s. Please, make sure to create their Microsoft account before their arrival.", newHire, jobCategory, startDate))
	} else if !intraUsername && intraPosition {
		description = escapeComma(fmt.Sprintf("%s will be joining us as %s starting from %s. Please, make sure to create their Microsoft account before their arrival.", newHire, position, startDate))
	} else if intraUsername && (!intraPosition || !intraJobCategory) {
		description = escapeComma(fmt.Sprintf("%s will be joining us starting from %s. Their username in CRMIntratools is %s. Please, make sure to create their Microsoft account before their arrival.", newHire, startDate, newUsername))
	} else {
		description = escapeComma(fmt.Sprintf("%s will be joining us starting from %s. Please, make sure to create their Microsoft account before their arrival.", newHire, startDate))
	}

	// DEBUG HARDCODED TO ME
	email := "RRHH@crm.cat"
	//email := "ltorrescusa@crm.cat"

	// enviar mail als encarregats del departament
	managers := map[string]interface{}{
		"table":     "people p LEFT JOIN department_workers d ON p.id = d.worker_id",
		"columns":   "p.crm_email, p.name, p.surname, p.id",
		"condition": "d.department_name = 'IT' AND d.manager = '1'",
	}

	managersMail, _ := json.Marshal(managers)
	idResp := getReq(managersMail, apiKey, client, w)

	if idResp == nil {
		createLog("Failed to retrieve managers mail from database for user "+username, 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve managers mail", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()

	var mails []map[string]interface{}
	err := json.NewDecoder(idResp.Body).Decode(&mails)
	if err != nil {
		createLog("Error leyendo mails: "+err.Error(), 1, apiKey, client, w)
		return
	}

	idResponsableInt := int(mails[0]["id"].(float64))

	query = map[string]interface{}{
		"table":   "tickets",
		"columns": "department, issue, description, email, creation_date, user_id, assigned_to, urgency, status",
		"value": fmt.Sprintf("%s,%s,%s,%s,%s,%d,%d,High,Assigned",
			department, issue, description, email, created, userID, idResponsableInt),
	}

	postJSON, _ := json.Marshal(query)
	resp = postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// crear notificació
	notifBody := escapeComma("Your ticket has been succesfully sent to the department administration. You will be notified for every status change.")
	query = map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Ticket, Ticket created successfully, %s, 304, %s", notifBody, created),
	}

	postJSON, _ = json.Marshal(query)
	resp = postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket notification: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Get notification ID
	getID := map[string]interface{}{
		"table":     "notifications",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s'", created),
	}
	idJSON, _ := json.Marshal(getID)
	idResp = getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new notification ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get notification ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var rows []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&rows)
	notifID := int(rows[0]["id"].(float64))

	// Sends to users creating an entry to notification_user table

	link := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read, can_download, seen",
		"value":   fmt.Sprintf("%d, %d, 1, 0, 0", notifID, userID),
	}
	jsonLink, _ := json.Marshal(link)
	resp = postReq(jsonLink, apiKey, client, w)
	if resp == nil {
		createLog("Failed to send notification", 1, apiKey, client, w)
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	//enviar mail si ha proporcionat un

	// Get ticket ID
	getTicketID := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = %d AND creation_date = '%s'", userID, created),
	}
	idJSON, _ = json.Marshal(getTicketID)
	idResp = getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new ticket ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get ticket ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var ids []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&ids)
	ticketID := int(ids[0]["id"].(float64))

	if email != "" {

		tmplContent, err := os.ReadFile("../module6/module6workers/assets/emailTicketCreation.html")
		if err != nil {
			println("Error leyendo plantilla:", err.Error())
			return
		}

		tmpl, err := template.New("email").Parse(string(tmplContent))
		if err != nil {
			println("Error parseando plantilla:", err.Error())
			return
		}
		data := map[string]string{
			"TicketID":    strconv.Itoa(ticketID),
			"Status":      "Assigned",
			"StatusClass": getStatusClass("Assigned"),
			"UpdatedAt":   created,
			"Message":     unescapeComma(notifBody),
		}

		var buf bytes.Buffer
		tmpl.Execute(&buf, data)
		sendEmail(email, fmt.Sprintf("Account for %s requested correctly", newHire), buf.String())
	}

	if len(mails) > 0 {
		// aconseguir nom de qui ha creat el ticket

		// enviar mail als encarregats del departament
		creationUser := map[string]interface{}{
			"table":     "people",
			"columns":   "name, surname",
			"condition": fmt.Sprintf("id = %d", userID),
		}

		creationUserJSON, _ := json.Marshal(creationUser)
		idResp = getReq(creationUserJSON, apiKey, client, w)

		if idResp == nil {
			createLog("Failed to retrieve users name from database", 1, apiKey, client, w)
			http.Error(w, "Failed to retrieve users name", http.StatusInternalServerError)
			return
		}
		defer idResp.Body.Close()

		var names []map[string]interface{}
		json.NewDecoder(idResp.Body).Decode(&names)

		name, _ := names[0]["name"].(string)
		surname, _ := names[0]["surname"].(string)
		createdBy := strings.TrimSpace(name + " " + surname)

		tmplContent, err := os.ReadFile("../module6/module6workers/assets/notifyWorker.html")
		if err != nil {
			createLog("Error leyendo plantilla de managers: "+err.Error(), 1, apiKey, client, w)
			return
		}

		tmpl, err := template.New("email").Parse(string(tmplContent))
		if err != nil {
			createLog("Error parseando plantilla de managers: "+err.Error(), 1, apiKey, client, w)
			return
		}

		for _, m := range mails {
			mail, ok := m["crm_email"].(string)
			if !ok || mail == "" {
				continue
			}

			// nombre del manager
			name, _ := m["name"].(string)
			surname, _ := m["surname"].(string)
			fullName := strings.TrimSpace(name + " " + surname)

			// datos que usará la plantilla
			data := map[string]string{
				"TicketID":     strconv.Itoa(ticketID),
				"Department":   department,
				"Issue":        issue,
				"Description":  unescapeComma(description),
				"CreationDate": created,
				"UserName":     fullName,
				"CreatedBy":    createdBy,
				"Urgency":      "High",
				"UrgencyClass": strings.ToLower("High"),
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				createLog("Error ejecutando plantilla manager: "+err.Error(), 1, apiKey, client, w)
				continue
			}

			// enviar email
			sendEmail(mail, fmt.Sprintf("New Ticket: #%d", ticketID), buf.String())
		}
	}

	// marcar tarea de new hire como completada
	updateTask := map[string]interface{}{
		"table":     "newHires_tasks",
		"columns":   "account_request, acc_req_time",
		"value":     fmt.Sprintf("1, %s", created),
		"condition": fmt.Sprintf("people_id = %d", payload.NewHireID),
	}
	updateJSON, _ := json.Marshal(updateTask)
	resp = putReq(updateJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to update new hire task", 1, apiKey, client, w)
		http.Error(w, "Failed to update new hire task", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))

}

func handleGetNewHiresHistory(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "newHires_tasks n LEFT JOIN people p ON n.people_id = p.id",
		"columns": "n.people_id, n.added, n.intra_req, n.intra_req_time, n.desk_request, n.desk_req_time, n.account_request, n.acc_req_time, p.name, p.surname, p.secondSurname",
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleGetNewHiresHistory for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch new hires history", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func handleGetFundings(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "fundings",
		"columns": "id, code, name, active",
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleGetFundings for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch fundings", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding trainings for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	for i := range result {
		result[i]["name"] = unescapeComma(result[i]["name"].(string))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleSaveFunding(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	// Obtener información del usuario
	_, username, _ := getUserInfo(apiKey, client, w, r)

	var payload struct {
		ID     string `json:"id"`
		Code   string `json:"code"`
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("handleSaveFunding: invalid JSON from user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	fundingName := escapeComma(payload.Name)

	var active int
	if payload.Active {
		active = 1
	} else {
		active = 0
	}

	if payload.ID == "" {
		// Insert new funding
		query := map[string]interface{}{
			"table":   "fundings",
			"columns": "code, name, active",
			"value":   fmt.Sprintf("%s,%s,%d", payload.Code, fundingName, active),
		}
		jsonData, _ := json.Marshal(query)
		resp := postReq(jsonData, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error: nil response in handleSaveFunding (insert) for user %s", username), 1, apiKey, client, w)
			http.Error(w, "Failed to save funding", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
	} else {
		// Update existing funding
		query := map[string]interface{}{
			"table":     "fundings",
			"columns":   "code, name, active",
			"value":     fmt.Sprintf("%s,%s,%d", payload.Code, fundingName, active),
			"condition": fmt.Sprintf("id = '%s'", payload.ID),
		}
		jsonData, _ := json.Marshal(query)
		resp := putReq(jsonData, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error: nil response in handleSaveFunding (update) for user %s", username), 1, apiKey, client, w)
			http.Error(w, "Failed to update funding", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDeleteFunding(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	// Obtener información del usuario
	_, username, _ := getUserInfo(apiKey, client, w, r)

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("handleDeleteFunding: invalid JSON from user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "fundings",
		"condition": fmt.Sprintf("id = '%s'", payload.ID),
	}
	jsonData, _ := json.Marshal(query)
	resp := deleteReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleDeleteFunding for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to delete funding", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// -------------------------------- MANAGE PROJECTS ---------------------------------------
func handleGetProjects(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "projects",
		"columns": "*",
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog("Error: nil response in handleGetAllProjects for user "+username, 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve projects", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding projects response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	for i := range result {
		if result[i]["name"] != nil {
			result[i]["name"] = unescapeComma(result[i]["name"].(string))
		}
		if result[i]["short_name"] != nil {
			result[i]["short_name"] = unescapeComma(result[i]["short_name"].(string))
		}
		if result[i]["number"] != nil {
			result[i]["number"] = unescapeComma(fmt.Sprint(result[i]["number"]))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleGetProjectTypes(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	values := map[string]bool{}

	queries := []map[string]interface{}{
		{
			"table":     "projects",
			"columns":   "DISTINCT type",
			"condition": "type IS NOT NULL AND type <> '' ORDER BY type ASC",
		},
		{
			"table":     "service_commission_pricing_tables",
			"columns":   "DISTINCT category AS type",
			"condition": "active = 1 AND category IS NOT NULL AND category <> '' ORDER BY category ASC",
		},
	}

	for _, query := range queries {
		jsonQuery, _ := json.Marshal(query)
		resp := getReq(jsonQuery, apiKey, client, w)
		if resp == nil {
			createLog("Error: nil response in handleGetProjectTypes for user "+username, 1, apiKey, client, w)
			continue
		}
		var rows []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
			createLog(fmt.Sprintf("Error decoding project types for user %s: %v", username, err), 1, apiKey, client, w)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, row := range rows {
			value := strings.TrimSpace(unescapeComma(fmt.Sprint(row["type"])))
			if value != "" {
				values[value] = true
			}
		}
	}

	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ShortName      string `json:"short_name"`
	Number         string `json:"number"`
	Type           string `json:"type"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	ManagementUnit string `json:"management_unit"`
	FonsRomanents  string `json:"fons_romanents"`
}

func buildInsertProjectValues(p Project) (string, string) {
	cols := []string{}
	vals := []string{}

	if p.ID != "" {
		cols = append(cols, "id")
		vals = append(vals, p.ID)
	}
	if p.Name != "" {
		cols = append(cols, "name")
		vals = append(vals, escapeComma(p.Name))
	}
	if p.ShortName != "" {
		cols = append(cols, "short_name")
		vals = append(vals, escapeComma(p.ShortName))
	}
	if p.Number != "" {
		cols = append(cols, "number")
		vals = append(vals, escapeComma(p.Number))
	}
	if p.Type != "" {
		cols = append(cols, "type")
		vals = append(vals, escapeComma(p.Type))
	}
	if p.StartDate != "" {
		cols = append(cols, "start_date")
		vals = append(vals, p.StartDate)
	}
	if p.EndDate != "" {
		cols = append(cols, "end_date")
		vals = append(vals, p.EndDate)
	}
	if p.ManagementUnit != "" {
		cols = append(cols, "management_unit")
		vals = append(vals, escapeComma(p.ManagementUnit))
	}
	if p.FonsRomanents != "" {
		cols = append(cols, "fons_romanents")
		vals = append(vals, escapeComma(p.FonsRomanents))
	}

	return strings.Join(cols, ", "), strings.Join(vals, ", ")
}

func buildUpdateProjectParts(p Project) (string, string) {
	cols := []string{}
	vals := []string{}

	if p.Name != "" {
		cols = append(cols, "name")
		vals = append(vals, escapeComma(p.Name))
	}
	if p.ShortName != "" {
		cols = append(cols, "short_name")
		vals = append(vals, escapeComma(p.ShortName))
	}
	cols = append(cols, "number")
	vals = append(vals, escapeComma(p.Number))
	cols = append(cols, "type")
	vals = append(vals, escapeComma(p.Type))
	if p.StartDate != "" {
		cols = append(cols, "start_date")
		vals = append(vals, p.StartDate)
	}
	if p.EndDate != "" {
		cols = append(cols, "end_date")
		vals = append(vals, p.EndDate)
	}
	if p.ManagementUnit != "" {
		cols = append(cols, "management_unit")
		vals = append(vals, escapeComma(p.ManagementUnit))
	}
	if p.FonsRomanents != "" {
		cols = append(cols, "fons_romanents")
		vals = append(vals, escapeComma(p.FonsRomanents))
	}

	return strings.Join(cols, ", "), strings.Join(vals, ", ")
}

func handleGetProjectMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)

	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		createLog("Error: Missing projectID in handleGetProjectMembers for user "+username, 1, apiKey, client, w)
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "people_projects pp INNER JOIN people p ON pp.people_id = p.id",
		"columns":   "pp.start_date, pp.end_date, pp.researcher_type AS role, p.picture_path, p.name, p.surname, p.secondSurname, p.id",
		"condition": fmt.Sprintf("pp.project_id = '%s'", projectID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: nil response in handleGetProjectMembers for project %s and user %s", projectID, username), 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve project members", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding project members for project %s and user %s: %v", projectID, username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	for i := range result {
		if result[i]["name"] != nil {
			if nameStr, ok := result[i]["name"].(string); ok {
				result[i]["name"] = unescapeComma(nameStr)
			}
		}
		if result[i]["surname"] != nil {
			if surnameStr, ok := result[i]["surname"].(string); ok {
				result[i]["surname"] = unescapeComma(surnameStr)
			}
		}
		if result[i]["secondSurname"] != nil {
			if secondSurnameStr, ok := result[i]["secondSurname"].(string); ok {
				result[i]["secondSurname"] = unescapeComma(secondSurnameStr)
			}
		}
		if result[i]["role"] != nil {
			if roleStr, ok := result[i]["role"].(string); ok {
				result[i]["role"] = unescapeComma(roleStr)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleSaveProject(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var p Project

	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		createLog(fmt.Sprintf("Error parsing project JSON: %v", err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(p.ID) == "" {
		createLog("Error: missing project ID in handleSaveProject", 1, apiKey, client, w)
		http.Error(w, "Project ID is required", http.StatusBadRequest)
		return
	}

	exists := false

	check := map[string]interface{}{
		"table":     "projects",
		"columns":   "COUNT(*) as cnt",
		"condition": fmt.Sprintf("id = '%s'", p.ID),
	}

	checkJSON, _ := json.Marshal(check)
	resp := getReq(checkJSON, apiKey, client, w)
	if resp != nil {
		defer resp.Body.Close()
		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && len(result) > 0 {
			if cnt, ok := result[0]["cnt"].(float64); ok && cnt > 0 {
				exists = true
			}
		}
	} else {
		createLog(fmt.Sprintf("Error checking existence for project %s", p.ID), 1, apiKey, client, w)
	}

	if !exists {
		cols, vals := buildInsertProjectValues(p)

		//fmt.Printf("Inserting project with columns: %s and values: %s\n", cols, vals)
		query := map[string]interface{}{
			"table":   "projects",
			"columns": cols,
			"value":   vals,
		}

		jsonData, _ := json.Marshal(query)
		resp := postReq(jsonData, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error inserting new project %s", p.ID), 1, apiKey, client, w)
			http.Error(w, "Failed to insert project", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		createLog(fmt.Sprintf("Project %s successfully inserted", p.ID), 0, apiKey, client, w)

	} else {
		cols, vals := buildUpdateProjectParts(p)
		//fmt.Printf("Inserting project with columns: %s and values: %s\n", cols, vals)

		query := map[string]interface{}{
			"table":     "projects",
			"columns":   cols,
			"value":     vals,
			"condition": fmt.Sprintf("id = '%s'", p.ID),
		}

		jsonData, _ := json.Marshal(query)
		resp := putReq(jsonData, apiKey, client, w)
		if resp == nil || resp.StatusCode >= 400 {
			createLog(fmt.Sprintf("Error updating project %s", p.ID), 1, apiKey, client, w)
			http.Error(w, "Failed to update project", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		createLog(fmt.Sprintf("Project %s successfully updated", p.ID), 0, apiKey, client, w)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleSaveProjectMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		createLog(fmt.Sprintf("Error parsing form in handleSaveProjectMembers: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	projectID := r.FormValue("project_id")
	start := r.FormValue("start_date")
	end := r.FormValue("end_date")
	role := r.FormValue("role")

	if peopleID == "" || peopleID == "undefined" {
		createLog("Error: people_id undefined in handleSaveProjectMembers", 1, apiKey, client, w)
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	if projectID == "" {
		createLog("Error: project_id missing in handleSaveProjectMembers", 1, apiKey, client, w)
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	setCols := []string{}
	setValues := []string{}

	if start != "" {
		setCols = append(setCols, "start_date")
		setValues = append(setValues, start)
	}
	if end != "" {
		setCols = append(setCols, "end_date")
		setValues = append(setValues, end)
	}
	if role != "" {
		setCols = append(setCols, "researcher_type")
		setValues = append(setValues, escapeComma(role))
	}

	if len(setCols) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":   "people_projects",
		"columns": strings.Join(setCols, ", "),
		"value":   strings.Join(setValues, ", "),
		"condition": fmt.Sprintf(
			"people_id = '%s' AND project_id = '%s'",
			peopleID, projectID,
		),
	}

	jsonData, _ := json.Marshal(query)

	resp := putReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error updating member %s in project %s", peopleID, projectID), 1, apiKey, client, w)
		http.Error(w, "Failed to update project member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDeleteProject(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		createLog("Error: missing id in handleDeleteProject", 1, apiKey, client, w)
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// Primero borrar relaciones en people_projects
	delMembers := map[string]interface{}{
		"table":     "people_projects",
		"condition": fmt.Sprintf("project_id = '%s'", projectID),
	}
	delMembersJSON, _ := json.Marshal(delMembers)
	delMembersResp := deleteReq(delMembersJSON, apiKey, client, w)
	if delMembersResp == nil || delMembersResp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error deleting members for project %s", projectID), 1, apiKey, client, w)
		http.Error(w, "Failed to delete project members", http.StatusInternalServerError)
		return
	}
	delMembersResp.Body.Close()

	// Luego borrar el proyecto
	delProject := map[string]interface{}{
		"table":     "projects",
		"condition": fmt.Sprintf("id = '%s'", projectID),
	}
	delProjectJSON, _ := json.Marshal(delProject)
	delProjectResp := deleteReq(delProjectJSON, apiKey, client, w)
	if delProjectResp == nil || delProjectResp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error deleting project %s", projectID), 1, apiKey, client, w)
		http.Error(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}
	defer delProjectResp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDeleteProjectMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		createLog(fmt.Sprintf("Error parsing form in handleDeleteProjectMembers: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	projectID := r.FormValue("project_id")

	if peopleID == "" || peopleID == "undefined" {
		createLog("Error: people_id undefined in handleDeleteProjectMembers", 1, apiKey, client, w)
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	if projectID == "" {
		createLog("Error: project_id missing in handleDeleteProjectMembers", 1, apiKey, client, w)
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table": "people_projects",
		"condition": fmt.Sprintf(
			"people_id = '%s' AND project_id = '%s'",
			peopleID, projectID,
		),
	}

	jsonData, _ := json.Marshal(query)

	resp := deleteReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error deleting member %s in project %s", peopleID, projectID), 1, apiKey, client, w)
		http.Error(w, "Failed to delete project member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleAddProjectMember(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		createLog(fmt.Sprintf("Error parsing form in handleAddProjectMember: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	peopleID := r.FormValue("people_id")
	projectID := r.FormValue("project_id")
	start := r.FormValue("start_date")
	end := r.FormValue("end_date")
	role := r.FormValue("role")

	if peopleID == "" || peopleID == "undefined" {
		createLog("Error: people_id undefined in handleAddProjectMember", 1, apiKey, client, w)
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	if projectID == "" {
		createLog("Error: project_id missing in handleAddProjectMember", 1, apiKey, client, w)
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	cols := []string{"people_id", "project_id"}
	vals := []string{peopleID, projectID}

	if role != "" {
		cols = append(cols, "researcher_type")
		vals = append(vals, escapeComma(role))
	}
	if start != "" {
		cols = append(cols, "start_date")
		vals = append(vals, start)
	}
	if end != "" {
		cols = append(cols, "end_date")
		vals = append(vals, end)
	}

	query := map[string]interface{}{
		"table":   "people_projects",
		"columns": strings.Join(cols, ", "),
		"value":   strings.Join(vals, ", "),
	}

	jsonData, _ := json.Marshal(query)
	resp := postReq(jsonData, apiKey, client, w)
	if resp == nil || resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error adding member %s to project %s", peopleID, projectID), 1, apiKey, client, w)
		http.Error(w, "Failed to add project member", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleGetAvailableUsersForProject(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		createLog("Error: missing id in handleGetAvailableUsersForProject", 1, apiKey, client, w)
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "people p LEFT JOIN people_projects pp ON p.id = pp.people_id AND pp.project_id = '" + projectID + "'",
		"columns":   "p.id, p.name, p.surname",
		"condition": "pp.project_id IS NULL",
	}

	jsonData, _ := json.Marshal(query)

	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error retrieving available users for project %s", projectID), 1, apiKey, client, w)
		http.Error(w, "No response from DB API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding available users for project %s: %v", projectID, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	for i := range result {
		if result[i]["name"] != nil {
			if nameStr, ok := result[i]["name"].(string); ok {
				result[i]["name"] = unescapeComma(nameStr)
			}
		}
		if result[i]["surname"] != nil {
			if surnameStr, ok := result[i]["surname"].(string); ok {
				result[i]["surname"] = unescapeComma(surnameStr)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
