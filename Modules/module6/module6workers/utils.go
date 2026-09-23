package module6workers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/csv"
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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

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
	if mC.Module6ApiKey == inkey {
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
			createLog(fmt.Sprintf("Superadmin accessing module 6 from IP %s", r.RemoteAddr), 0, apiKey, client, w)
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

// --------------------------------------------------------------------------------------------------------
// --------------------------------------------------------------------------------------------------------
// --------------------------------------------- TICKETING ------------------------------------------------
// --------------------------------------------------------------------------------------------------------
// --------------------------------------------------------------------------------------------------------

func handleSendUserRole(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, _, userRole := getUserInfo(apiKey, client, w, r)

	// Si no hay rol definido en getUserInfo, probamos el contexto
	if userRole == "" {
		if ctxRole, ok := r.Context().Value("userRole").(string); ok && ctxRole != "" {
			userRole = ctxRole
		} else {
			userRole = "guest"
		}
	}
	json.NewEncoder(w).Encode(map[string]string{
		"role": userRole,
	})
}

func handleGetUserInfo(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userId, username, role := getUserInfo(apiKey, client, w, r)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":  userId,
		"username": username,
		"role":     role,
	})
}

func handleGetManagedDepartments(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}
	// Preparamos la consulta en formato JSON para getReq
	query := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "department_name, worker_id, manager",
		"condition": fmt.Sprintf("worker_id = '%s' AND manager = 1", userID),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch departments: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog("Error reading getReq response body", 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}
	// Si no devuelve resultados, retornamos lista vacía
	if len(body) == 0 || string(body) == "[]" {
		w.Write([]byte(`[]`))
		return
	}

	w.Write(body)
}

func handleGetUserDepartments(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "department_name, worker_id",
		"condition": fmt.Sprintf("worker_id = '%s'", userID),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 || string(body) == "[]" {
		w.Write([]byte(`[]`))
		return
	}

	w.Write(body)
}

func handleGetDepartments(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := map[string]interface{}{
		"table":   "departments",
		"columns": "name",
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("getDepartments: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Write(body)
}

func handleGetDepartmentWorkers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := map[string]interface{}{
		"table":   "department_workers",
		"columns": "department_name, worker_id, manager",
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("getDepartmentWorkers: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Write(body)
}

func handleGetAllWorkers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Obtener los people_id desde users
	queryUsers := map[string]interface{}{
		"table":   "users",
		"columns": "people_id",
	}

	getJSON, _ := json.Marshal(queryUsers)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("getAllWorkers: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed (users)", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	userBody, _ := io.ReadAll(resp.Body)

	var userRows []map[string]interface{}
	if err := json.Unmarshal(userBody, &userRows); err != nil {
		createLog("Error parsing users result", 1, apiKey, client, w)
		http.Error(w, "Invalid DB response (users)", http.StatusInternalServerError)
		return
	}

	if len(userRows) == 0 {
		w.Write([]byte(`[]`))
		return
	}

	// Extraer IDs para consultar en people
	var ids []string
	for _, row := range userRows {
		if pid, ok := row["people_id"].(string); ok {
			ids = append(ids, pid)
		} else if f, ok := row["people_id"].(float64); ok {
			ids = append(ids, fmt.Sprintf("%.0f", f))
		}
	}
	idList := strings.Join(ids, ",")

	// Consultar people
	queryPeople := map[string]interface{}{
		"table":     "people",
		"columns":   "id, name, surname",
		"condition": fmt.Sprintf("id IN (%s)", idList),
	}

	peopleJSON, _ := json.Marshal(queryPeople)
	resp2 := getReq(peopleJSON, apiKey, client, w)
	if resp2 == nil {
		createLog("getAllWorkers: getReq returned nil (people)", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed (people)", http.StatusInternalServerError)
		return
	}
	defer resp2.Body.Close()

	peopleBody, _ := io.ReadAll(resp2.Body)

	var peopleRows []map[string]interface{}
	if err := json.Unmarshal(peopleBody, &peopleRows); err != nil {
		createLog("Error parsing people result", 1, apiKey, client, w)
		http.Error(w, "Invalid DB response (people)", http.StatusInternalServerError)
		return
	}

	// Combinar resultados
	var workers []map[string]interface{}
	for _, p := range peopleRows {
		id := fmt.Sprintf("%.0f", p["id"].(float64))
		fullName := fmt.Sprintf("%s %s", p["name"], p["surname"])
		workers = append(workers, map[string]interface{}{
			"id":   id,
			"name": fullName,
		})
	}

	finalJSON, _ := json.Marshal(workers)
	w.Write(finalJSON)
}

func handleCreateDepartment(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":   "departments",
		"columns": "name",
		"value":   req.Name,
	}

	postJSON, _ := json.Marshal(query)
	resp := postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("createDepartment: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDeleteDepartment(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing department name", http.StatusBadRequest)
		return
	}

	// También borramos las asignaciones en department_workers
	subQuery := map[string]interface{}{
		"table":     "department_workers",
		"condition": fmt.Sprintf("department_name = '%s'", name),
	}
	subJSON, _ := json.Marshal(subQuery)
	deleteReq(subJSON, apiKey, client, w)

	query := map[string]interface{}{
		"table":     "departments",
		"condition": fmt.Sprintf("name = '%s'", name),
	}

	delJSON, _ := json.Marshal(query)
	resp := deleteReq(delJSON, apiKey, client, w)
	if resp == nil {
		createLog("deleteDepartment: deleteReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Write([]byte(`{"status":"ok"}`))
}

func handleAddWorkerToDepartment(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		DepartmentName string `json:"department_name"`
		WorkerID       int    `json:"worker_id"`
		Manager        int    `json:"manager"` // 0 o 1
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":   "department_workers",
		"columns": "department_name, worker_id, manager",
		"value":   fmt.Sprintf("%s, %d, %d", req.DepartmentName, req.WorkerID, req.Manager),
	}

	postJSON, _ := json.Marshal(query)
	resp := postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("addWorkerToDepartment: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Write([]byte(`{"status":"ok"}`))
}

func handleUpdateWorkerManagerStatus(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}

	var req struct {
		DepartmentName string `json:"department_name"`
		WorkerID       int    `json:"worker_id"`
		Manager        int    `json:"manager"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	query := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "manager",
		"value":     fmt.Sprintf("%d", req.Manager),
		"condition": fmt.Sprintf("department_name = '%s' AND worker_id = '%d'", req.DepartmentName, req.WorkerID),
	}

	putJSON, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		http.Error(w, "Internal error preparing request", http.StatusInternalServerError)
		return
	}
	resp := putReq(putJSON, apiKey, client, w)
	if resp == nil {
		createLog("updateWorkerManagerStatus: putReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Write([]byte(`{"status":"ok"}`))
}

func handleRemoveWorkerFromDepartment(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		DepartmentName string `json:"department_name"`
		WorkerID       int    `json:"worker_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "department_workers",
		"condition": fmt.Sprintf("department_name = '%s' AND worker_id = %d", req.DepartmentName, req.WorkerID),
	}

	delJSON, _ := json.Marshal(query)
	resp := deleteReq(delJSON, apiKey, client, w)
	if resp == nil {
		createLog("removeWorkerFromDepartment: deleteReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	io.ReadAll(resp.Body)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDownloadDepartmentTickets(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	dept := r.URL.Query().Get("dept")
	if dept == "" {
		http.Error(w, "Missing department", http.StatusBadRequest)
		return
	}

	// 1. Pedir tickets del departamento
	ticketQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id, department, issue, description, creation_date, status, urgency, assigned_to, user_id",
		"condition": fmt.Sprintf("department = '%s'", dept),
	}
	qJSON, _ := json.Marshal(ticketQuery)
	resp := getReq(qJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tickets []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&tickets)

	// 2. Convertir user_id a nombre+apellidos
	for _, t := range tickets {
		assigned := ""
		if a, ok := t["assigned_to"].(float64); ok {
			assigned = getPeopleName(int(a), apiKey, client, w)
			t["assigned_to"] = assigned
		}

		if uidFloat, ok := t["user_id"].(float64); ok {
			uid := int(uidFloat)
			fullName := getPeopleName(uid, apiKey, client, w)
			t["user_name"] = fullName
		}

		rawDate := fmt.Sprintf("%v", t["creation_date"])

		var parsed time.Time
		var err error

		parsed, err = time.Parse(time.RFC3339, rawDate)
		if err != nil {
			parsed, err = time.Parse("2006-01-02 15:04:05", rawDate)
		}

		if err == nil {
			t["creation_date_formatted"] = parsed.Format("02/01/2006 15:04")
		} else {
			t["creation_date_formatted"] = rawDate
		}
	}

	// 3. Preparar CSV
	now := time.Now().Format("2006-01-02 15:04")
	filename := fmt.Sprintf("%s_tickets_%s.csv", dept, now)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	csvW := csv.NewWriter(w)

	// Cabecera CSV
	csvW.Write([]string{
		"ID", "Department", "Issue", "Description", "Creation Date",
		"Status", "Urgency", "Assigned To", "Submitted By",
	})

	clean := func(v interface{}) string {
		if v == nil {
			return ""
		}
		s := fmt.Sprintf("%v", v)
		if s == "<nil>" {
			return ""
		}
		return s
	}

	// Rows
	for _, t := range tickets {
		csvW.Write([]string{
			clean(fmt.Sprintf("%v", t["id"])),
			clean(fmt.Sprintf("%v", t["department"])),
			clean(fmt.Sprintf("%v", t["issue"])),
			clean(unescapeComma(fmt.Sprintf("%v", t["description"]))),
			clean(fmt.Sprintf("%v", t["creation_date_formatted"])),
			clean(fmt.Sprintf("%v", t["status"])),
			clean(fmt.Sprintf("%v", t["urgency"])),
			clean(fmt.Sprintf("%v", t["assigned_to"])),
			clean(fmt.Sprintf("%v", t["user_name"])),
		})
	}

	csvW.Flush()
}

func handleDownloadDepartmentTicketHistory(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	dept := r.URL.Query().Get("dept")
	if dept == "" {
		http.Error(w, "Missing department", http.StatusBadRequest)
		return
	}

	// --- 1. Obtener todos los tickets del departamento ---
	ticketQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id, department, issue, description, creation_date, status, urgency, assigned_to, user_id",
		"condition": fmt.Sprintf("department = '%s'", dept),
	}
	tJSON, _ := json.Marshal(ticketQuery)
	resp := getReq(tJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tickets []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&tickets)

	// Preparar mapa por ID para lookup rápido
	ticketMap := map[int]map[string]interface{}{}
	for _, t := range tickets {
		id := int(t["id"].(float64))
		ticketMap[id] = t
	}

	// --- 2. Para cada ticket, obtener historial ---
	type FullRow struct {
		TicketID int
		Ticket   map[string]interface{}
		History  map[string]interface{}
	}
	var rows []FullRow

	for _, t := range tickets {
		tid := int(t["id"].(float64))

		hQuery := map[string]interface{}{
			"table":     "ticket_history",
			"columns":   "id, ticket_id, status, updated, old_status, comment",
			"condition": fmt.Sprintf("ticket_id = %d", tid),
		}
		hJSON, _ := json.Marshal(hQuery)
		resp2 := getReq(hJSON, apiKey, client, w)
		if resp2 == nil {
			continue
		}
		var hist []map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&hist)
		resp2.Body.Close()

		for _, entry := range hist {
			rows = append(rows, FullRow{
				TicketID: tid,
				Ticket:   t,
				History:  entry,
			})
		}
	}

	// Ordenar por ticket_id y luego por updated
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TicketID == rows[j].TicketID {
			return fmt.Sprintf("%v", rows[i].History["updated"]) <
				fmt.Sprintf("%v", rows[j].History["updated"])
		}
		return rows[i].TicketID < rows[j].TicketID
	})

	// --- 3. Mejorar campos: convertir nombres, fechas... ---

	clean := func(v interface{}) string {
		if v == nil {
			return ""
		}
		s := fmt.Sprintf("%v", v)
		if s == "<nil>" {
			return ""
		}
		return s
	}

	formatDate := func(raw string) string {
		if raw == "" {
			return ""
		}
		var parsed time.Time
		var err error

		parsed, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			parsed, err = time.Parse("2006-01-02 15:04:05", raw)
		}
		if err != nil {
			return raw
		}
		return parsed.Format("02/01/2006 15:04")
	}

	// Preconvertir nombres
	for _, r := range rows {
		t := r.Ticket

		if uidf, ok := t["user_id"].(float64); ok {
			t["user_name"] = getPeopleName(int(uidf), apiKey, client, w)
		}

		if a, ok := t["assigned_to"].(float64); ok {
			t["assigned_to_name"] = getPeopleName(int(a), apiKey, client, w)
		}

		t["creation_date_formatted"] = formatDate(clean(t["creation_date"]))
	}

	// --- 4. Construir CSV de salida ---
	now := time.Now().Format("2006-01-02 15:04")
	filename := fmt.Sprintf("%s_ticket_history_%s.csv", dept, now)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	csvW := csv.NewWriter(w)

	// Cabecera
	csvW.Write([]string{
		"Ticket ID", "Department", "Issue", "Description", "Created At",
		"Ticket Status", "Urgency", "Assigned To", "Submitted By",
		"History ID", "History Status", "Updated At", "Old Status", "Comment",
	})

	// Filas
	for _, r := range rows {
		t := r.Ticket
		h := r.History

		csvW.Write([]string{
			clean(r.TicketID),
			clean(t["department"]),
			clean(t["issue"]),
			clean(unescapeComma(fmt.Sprintf("%v", t["description"]))),
			clean(t["creation_date_formatted"]),
			clean(t["status"]),
			clean(t["urgency"]),
			clean(t["assigned_to_name"]),
			clean(t["user_name"]),

			// history
			clean(h["id"]),
			clean(h["status"]),
			formatDate(clean(h["updated"])),
			clean(h["old_status"]),
			clean(unescapeComma(fmt.Sprintf("%v", h["comment"]))),
		})
	}

	csvW.Flush()
}

// --------------------------------- Creacio de tickets ---------------------------------------

func handleSendEmail(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, _, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		http.Error(w, "Invalid user session", http.StatusUnauthorized)
		return
	}

	// Preparamos la consulta en formato JSON para getReq
	query := map[string]interface{}{
		"table":     "people",
		"columns":   "user_email, crm_email",
		"condition": fmt.Sprintf("id = %d", userID),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch user emails: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []struct {
		CrmEmail  string `json:"crm_email"`
		UserEmail string `json:"user_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("Error decoding user emails: %v", err), 1, apiKey, client, w)
		http.Error(w, "Failed to parse response", http.StatusInternalServerError)
		return
	}

	// Si no hay datos
	if len(result) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"crm_email":  "",
			"user_email": "",
		})
		return
	}

	// Devolvemos los valores al frontend
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"crm_email":  result[0].CrmEmail,
		"user_email": result[0].UserEmail,
	})
}

func handleCreateTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Aceptar multipart/form-data (10MB máx.)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	department := r.FormValue("department")
	issue := r.FormValue("issue")
	description := escapeComma(r.FormValue("description"))
	email := r.FormValue("email")

	// Validar email si se usa
	if email != "" {
		regex := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
		if !regex.MatchString(email) {
			http.Error(w, "Invalid email format", http.StatusBadRequest)
			return
		}
	}

	// Obtener información del usuario
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	// --- Guardar archivo si existe ---
	var filePath string
	file, handler, err := r.FormFile("file")
	if err == nil {
		defer file.Close()

		// Limitar tamaño
		if handler.Size > 5*1024*1024 {
			http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
			return
		}

		// Leer encabezado para detectar MIME real, no lo que dice el navegador
		buffer := make([]byte, 512)
		if _, err := file.Read(buffer); err != nil {
			http.Error(w, "Error reading file", http.StatusBadRequest)
			return
		}
		filetype := http.DetectContentType(buffer)

		allowedTypes := map[string]bool{
			"application/pdf": true,
			"image/jpeg":      true,
			"image/png":       true,
			"text/plain":      true,
		}

		if !allowedTypes[filetype] {
			http.Error(w, "Unsupported file type", http.StatusBadRequest)
			return
		}

		file.Seek(0, io.SeekStart)

		// Crear archivo temporal
		tempFile, err := os.CreateTemp("", "ticket_upload_*")
		if err != nil {
			http.Error(w, "Error creating temp file", http.StatusInternalServerError)
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := io.Copy(tempFile, file); err != nil {
			http.Error(w, "Error copying uploaded file", http.StatusInternalServerError)
			return
		}

		// Obtener extensión original
		ext := filepath.Ext(handler.Filename)

		timestamp := time.Now().Format("02012006_150405")
		newFilename := fmt.Sprintf("%s_ticket%s", timestamp, ext)

		// Guardar en carpeta del usuario
		savedPath, err := saveGenericUserFile(userID, "", username, tempFile.Name(), newFilename)
		if err != nil {
			createLog(fmt.Sprintf("Error saving file for ticket: %v", err), 1, apiKey, client, w)
			http.Error(w, "Error saving file", http.StatusInternalServerError)
			return
		}
		filePath = savedPath
	}

	created := time.Now().Format("2006-01-02 15:04")

	// ---  Construir query SQL ---
	query := map[string]interface{}{
		"table":   "tickets",
		"columns": "department, issue, description, file_path, email, creation_date, user_id",
		"value":   fmt.Sprintf("%s,%s,%s,%s,%s,%s,%d", department, issue, description, filePath, email, created, userID),
	}

	postJSON, _ := json.Marshal(query)
	resp := postReq(postJSON, apiKey, client, w)
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
		"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s' AND type = 'Ticket'", created),
	}
	idJSON, _ := json.Marshal(getID)
	idResp := getReq(idJSON, apiKey, client, w)
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

		tmplContent, err := os.ReadFile("module6workers/assets/emailTicketCreation.html")
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
			"Status":      "Open",
			"StatusClass": getStatusClass("Open"),
			"UpdatedAt":   created,
			"Message":     unescapeComma(notifBody),
		}

		var buf bytes.Buffer
		tmpl.Execute(&buf, data)
		sendEmail(email, fmt.Sprintf("Ticket #%d Creation", ticketID), buf.String())
	}

	// enviar mail als encarregats del departament
	managers := map[string]interface{}{
		"table":     "people p LEFT JOIN department_workers d ON p.id = d.worker_id",
		"columns":   "p.crm_email, p.name, p.surname",
		"condition": fmt.Sprintf("d.department_name = '%s' AND d.manager = '1'", department),
	}

	managersMail, _ := json.Marshal(managers)
	idResp = getReq(managersMail, apiKey, client, w)

	if idResp == nil {
		createLog("Failed to retrieve managers mail from database", 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve managers mail", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()

	var mails []map[string]interface{}
	err = json.NewDecoder(idResp.Body).Decode(&mails)
	if err != nil {
		createLog("Error leyendo mails: "+err.Error(), 1, apiKey, client, w)
		return
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

		tmplContent, err := os.ReadFile("module6workers/assets/notifyAdministrator.html")
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
				"Status":       "Assigned",
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

	w.Write([]byte(`{"status":"ok"}`))
}

func handleGetDepartmentTicketsAndWorkers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Info del usuario
	userID, _, _ := getUserInfo(apiKey, client, w, r)

	// Departamentos gestionados
	deptQuery := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "department_name",
		"condition": fmt.Sprintf("worker_id = '%d' AND manager = 1", userID),
	}
	deptJSON, _ := json.Marshal(deptQuery)
	deptResp := getReq(deptJSON, apiKey, client, w)
	if deptResp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer deptResp.Body.Close()

	var managedDepts []map[string]interface{}
	if err := json.NewDecoder(deptResp.Body).Decode(&managedDepts); err != nil {
		http.Error(w, "Error reading departments", http.StatusInternalServerError)
		return
	}
	if len(managedDepts) == 0 {
		w.Write([]byte(`{"tickets":[],"workers":[]}`))
		return
	}

	// Nombres de los departamentos
	var deptNames []string
	for _, d := range managedDepts {
		if name, ok := d["department_name"].(string); ok {
			deptNames = append(deptNames, fmt.Sprintf("'%s'", name))
		}
	}
	deptList := strings.Join(deptNames, ",")

	// Tickets sin asignar
	ticketQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id, department, issue, description, file_path, creation_date, urgency, assigned_to, user_id",
		"condition": fmt.Sprintf("department IN (%s) AND (assigned_to IS NULL OR assigned_to = '')", deptList),
	}
	ticketJSON, _ := json.Marshal(ticketQuery)
	ticketResp := getReq(ticketJSON, apiKey, client, w)
	if ticketResp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer ticketResp.Body.Close()

	var tickets []map[string]interface{}
	if err := json.NewDecoder(ticketResp.Body).Decode(&tickets); err != nil {
		http.Error(w, "Error decoding tickets", http.StatusInternalServerError)
		return
	}

	// Limpieza de texto y nombre de usuario
	for _, t := range tickets {
		if desc, ok := t["description"].(string); ok {
			t["description"] = unescapeComma(desc)
		}
		if iss, ok := t["issue"].(string); ok {
			t["issue"] = unescapeComma(iss)
		}
		uid := int(t["user_id"].(float64))
		t["user_name"] = getPeopleName(uid, apiKey, client, w)
	}

	// Trabajadores de esos departamentos
	workerQuery := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "worker_id, department_name",
		"condition": fmt.Sprintf("department_name IN (%s)", deptList),
	}
	workerJSON, _ := json.Marshal(workerQuery)
	workerResp := getReq(workerJSON, apiKey, client, w)
	if workerResp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer workerResp.Body.Close()

	var workers []map[string]interface{}
	if err := json.NewDecoder(workerResp.Body).Decode(&workers); err != nil {
		http.Error(w, "Error decoding workers", http.StatusInternalServerError)
		return
	}

	for i := range workers {
		id := int(workers[i]["worker_id"].(float64))
		workers[i]["worker_name"] = getPeopleName(id, apiKey, client, w)
	}

	// Empaquetar todo en un JSON combinado
	result := map[string]interface{}{
		"tickets": tickets,
		"workers": workers,
	}
	out, _ := json.Marshal(result)
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func handleGetDepartmentTicketsAndWorkersAssigned(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Info del usuario
	userID, _, _ := getUserInfo(apiKey, client, w, r)

	// Departamentos gestionados
	deptQuery := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "department_name",
		"condition": fmt.Sprintf("worker_id = '%d' AND manager = 1", userID),
	}
	deptJSON, _ := json.Marshal(deptQuery)
	deptResp := getReq(deptJSON, apiKey, client, w)
	if deptResp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer deptResp.Body.Close()

	var managedDepts []map[string]interface{}
	if err := json.NewDecoder(deptResp.Body).Decode(&managedDepts); err != nil {
		http.Error(w, "Error reading departments", http.StatusInternalServerError)
		return
	}
	if len(managedDepts) == 0 {
		w.Write([]byte(`{"tickets":[],"workers":[]}`))
		return
	}

	// Nombres de los departamentos
	var deptNames []string
	for _, d := range managedDepts {
		if name, ok := d["department_name"].(string); ok {
			deptNames = append(deptNames, fmt.Sprintf("'%s'", name))
		}
	}
	deptList := strings.Join(deptNames, ",")

	// Tickets sin asignar
	ticketQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id, department, issue, description, file_path, creation_date, urgency, assigned_to, user_id",
		"condition": fmt.Sprintf("department IN (%s) AND status NOT IN ('Closed', 'ClosedConfirmed')", deptList),
	}
	ticketJSON, _ := json.Marshal(ticketQuery)
	ticketResp := getReq(ticketJSON, apiKey, client, w)
	if ticketResp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer ticketResp.Body.Close()

	var tickets []map[string]interface{}
	if err := json.NewDecoder(ticketResp.Body).Decode(&tickets); err != nil {
		http.Error(w, "Error decoding tickets", http.StatusInternalServerError)
		return
	}

	// Limpieza de texto y nombre de usuario
	for _, t := range tickets {
		if desc, ok := t["description"].(string); ok {
			t["description"] = unescapeComma(desc)
		}
		if iss, ok := t["issue"].(string); ok {
			t["issue"] = unescapeComma(iss)
		}
		uid := int(t["user_id"].(float64))
		t["user_name"] = getPeopleName(uid, apiKey, client, w)
	}

	// Trabajadores de esos departamentos
	workerQuery := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "worker_id, department_name",
		"condition": fmt.Sprintf("department_name IN (%s)", deptList),
	}
	workerJSON, _ := json.Marshal(workerQuery)
	workerResp := getReq(workerJSON, apiKey, client, w)
	if workerResp == nil {
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer workerResp.Body.Close()

	var workers []map[string]interface{}
	if err := json.NewDecoder(workerResp.Body).Decode(&workers); err != nil {
		http.Error(w, "Error decoding workers", http.StatusInternalServerError)
		return
	}

	for i := range workers {
		id := int(workers[i]["worker_id"].(float64))
		workers[i]["worker_name"] = getPeopleName(id, apiKey, client, w)
	}

	// Empaquetar todo en un JSON combinado
	result := map[string]interface{}{
		"tickets": tickets,
		"workers": workers,
	}
	out, _ := json.Marshal(result)
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func getPeopleName(userID int, apiKey string, client *http.Client, w http.ResponseWriter) string {
	q := map[string]interface{}{
		"table":     "people",
		"columns":   "name, surname",
		"condition": fmt.Sprintf("id = %d", userID),
	}
	js, _ := json.Marshal(q)
	resp := getReq(js, apiKey, client, w)
	if resp == nil {
		return ""
	}
	defer resp.Body.Close()
	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil || len(rows) == 0 {
		return ""
	}
	name := rows[0]["name"].(string)
	if s, ok := rows[0]["surname"].(string); ok && s != "" {
		name += " " + s
	}
	return name
}

func handleAssignDepartmentTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read body", http.StatusBadRequest)
		return
	}

	var payload struct {
		TicketID   string `json:"ticket_id"`
		Urgency    string `json:"urgency"`
		AssignedTo string `json:"assigned_to"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		println("JSON decode error:", err.Error())
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.TicketID == "" || payload.AssignedTo == "" {
		http.Error(w, "Missing ticket_id or assigned_to", http.StatusBadRequest)
		return
	}

	// Actualizar el ticket
	updateQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "assigned_to, urgency, status",
		"value":     fmt.Sprintf("%s, %s, Assigned", payload.AssignedTo, payload.Urgency),
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}
	updateJSON, _ := json.Marshal(updateQuery)
	resp := putReq(updateJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Failed to update ticket", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	now := time.Now().Format("2006-01-02 15:04")

	comment := escapeComma(fmt.Sprintf("Ticket assigned to worker ID %s with urgency %s", payload.AssignedTo, payload.Urgency))
	auxId, _ := strconv.Atoi(payload.TicketID)
	prevID := getLastHistoryID(auxId, apiKey, client, w) // 0 si no hay anterior
	var values string
	historyQuery := map[string]interface{}{}
	if prevID > 0 {
		values = fmt.Sprintf("%d, %s, %s, %d, %s", auxId, "Assigned", now, prevID, comment)
		historyQuery = map[string]interface{}{
			"table":   "ticket_history",
			"columns": "ticket_id, status, updated, old_status, comment",
			"value":   values,
		}
	} else {
		values = fmt.Sprintf("%d, %s, %s, %s", auxId, "Assigned", now, comment)
		historyQuery = map[string]interface{}{
			"table":   "ticket_history",
			"columns": "ticket_id, status, updated, comment",
			"value":   values,
		}
	}

	// Insertar en ticket_history

	historyJSON, _ := json.Marshal(historyQuery)
	hResp := postReq(historyJSON, apiKey, client, w)
	if hResp != nil {
		defer hResp.Body.Close()
		body, _ := io.ReadAll(hResp.Body)
		fmt.Println("DEBUG ticket_history insert:", string(body))
	}

	//get userid
	getQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "user_id",
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}
	getJSON, _ := json.Marshal(getQuery)
	resp = getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to retrieve user id from DB", 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve user id from DB", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var ids []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&ids)
	userID := int(ids[0]["user_id"].(float64))

	// crear notificació

	notifBody := escapeComma("Your ticket has been assigned to a worker to resolve it. You will be notified of any status change.")
	query := map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Ticket, Ticket assigned successfully, %s, 304, %s", notifBody, now),
	}

	postJSON, _ := json.Marshal(query)
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
		"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s'", now),
	}
	idJSON, _ := json.Marshal(getID)
	idResp := getReq(idJSON, apiKey, client, w)
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

	getEmail := map[string]interface{}{
		"table":     "tickets",
		"columns":   "email",
		"condition": fmt.Sprintf("id = '%s'", payload.TicketID),
	}
	emailJSON, _ := json.Marshal(getEmail)
	emailResp := getReq(emailJSON, apiKey, client, w)

	getTech := map[string]interface{}{
		"table":     "people",
		"columns":   "name, surname, crm_email",
		"condition": fmt.Sprintf("id = '%s'", payload.AssignedTo),
	}
	techJSON, _ := json.Marshal(getTech)
	techResp := getReq(techJSON, apiKey, client, w)
	var techName string
	var techEmail string

	if techResp != nil {
		defer techResp.Body.Close()
		var techData []map[string]interface{}
		if err := json.NewDecoder(techResp.Body).Decode(&techData); err == nil && len(techData) > 0 {
			name := fmt.Sprint(techData[0]["name"])
			surname := fmt.Sprint(techData[0]["surname"])
			techName = strings.TrimSpace(name + " " + surname)
			techEmail = fmt.Sprint(techData[0]["crm_email"])
		}
	}

	if techName == "" {
		techName = "a CRM technician"
	}

	if emailResp != nil {
		defer emailResp.Body.Close()
		var data []map[string]interface{}
		if err := json.NewDecoder(emailResp.Body).Decode(&data); err == nil && len(data) > 0 {
			emailAddr := fmt.Sprint(data[0]["email"])
			if emailAddr != "" {

				tmplContent, err := os.ReadFile("module6workers/assets/emailTicketAssigned.html")
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
					"TicketID":    payload.TicketID,
					"AssignedTo":  techName,
					"Status":      "Assigned",
					"StatusClass": getStatusClass("assigned"),
					"AssignedAt":  now,
				}

				var buf bytes.Buffer
				tmpl.Execute(&buf, data)
				sendEmail(emailAddr, fmt.Sprintf("Ticket #%s Update", payload.TicketID), buf.String())
			}
		}
	}

	// enviar mail als encarregats del departament

	if techEmail != "" {
		// aconseguir nom de qui ha creat el ticket
		creationUser := map[string]interface{}{
			"table":     "people",
			"columns":   "name, surname",
			"condition": fmt.Sprintf("id = %d", userID),
		}

		fmt.Println("DEBUG -> Managers query:", creationUser)

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

		// aconseguir informació del ticket
		ticketQuery := map[string]interface{}{
			"table":     "tickets",
			"columns":   "department, issue, description, creation_date, urgency",
			"condition": fmt.Sprintf("id = %s", payload.TicketID),
		}

		ticketJSON, _ := json.Marshal(ticketQuery)
		idResp = getReq(ticketJSON, apiKey, client, w)

		if idResp == nil {
			createLog("Failed to retrieve ticket from database", 1, apiKey, client, w)
			http.Error(w, "Failed to retrieve ticket", http.StatusInternalServerError)
			return
		}
		defer idResp.Body.Close()

		var ticket []map[string]interface{}
		json.NewDecoder(idResp.Body).Decode(&ticket)

		department, _ := ticket[0]["department"].(string)
		issue, _ := ticket[0]["issue"].(string)
		description, _ := ticket[0]["description"].(string)
		created, _ := ticket[0]["creation_date"].(string)
		urgency, _ := ticket[0]["urgency"].(string)

		t, _ := time.Parse(time.RFC3339, created)
		createdFinal := t.Format("02/01/2006 15:04")

		tmplContent, err := os.ReadFile("module6workers/assets/notifyWorker.html")
		if err != nil {
			createLog("Error leyendo plantilla de managers: "+err.Error(), 1, apiKey, client, w)
			return
		}

		tmpl, err := template.New("email").Parse(string(tmplContent))
		if err != nil {
			createLog("Error parseando plantilla de managers: "+err.Error(), 1, apiKey, client, w)
			return
		}

		data := map[string]string{
			"TicketID":     payload.TicketID,
			"Department":   department,
			"Issue":        issue,
			"Description":  unescapeComma(description),
			"CreationDate": createdFinal,
			"UserName":     techName,
			"CreatedBy":    createdBy,
			"Urgency":      urgency,
			"UrgencyClass": strings.ToLower(urgency),
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			createLog("Error ejecutando plantilla manager: "+err.Error(), 1, apiKey, client, w)
		} else {
			// enviar email
			sendEmail(techEmail, fmt.Sprintf("New Ticket with %s urgency Assigned: #%s", urgency, payload.TicketID), buf.String())
		}

	}

	w.Write([]byte(`{"status":"ok"}`))
}

func handleGetAssignedTickets(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Obtener usuario actual
	userID, _, _ := getUserInfo(apiKey, client, w, r)

	if userID == 0 {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Query: tickets asignados al usuario
	query := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id, department, file_path, issue, description, urgency, status, user_id, creation_date",
		"condition": fmt.Sprintf("assigned_to = %d AND status NOT IN ('Closed', 'ClosedConfirmed')", userID),
	}

	queryJSON, _ := json.Marshal(query)
	resp := getReq(queryJSON, apiKey, client, w)
	if resp == nil {
		createLog("getAssignedTickets: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Unescape de comas y apóstrofes
	var tickets []map[string]interface{}
	if err := json.Unmarshal(body, &tickets); err != nil {
		http.Error(w, "Error parsing tickets", http.StatusInternalServerError)
		return
	}
	for _, t := range tickets {
		if desc, ok := t["description"].(string); ok {
			t["description"] = unescapeComma(desc)
		}
	}

	// Añadir nombre del usuario que envió el ticket
	for _, t := range tickets {
		if uid, ok := t["user_id"].(float64); ok {
			t["submitted_by"] = getPeopleName(int(uid), apiKey, client, w)
		}
	}

	finalJSON, _ := json.Marshal(tickets)
	w.Write(finalJSON)
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

func handleUpdateTicketStatus(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	body, _ := io.ReadAll(r.Body)

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Extraer datos
	ticketID, _ := strconv.Atoi(fmt.Sprint(raw["ticket_id"]))
	status := fmt.Sprint(raw["status"])
	message := fmt.Sprint(raw["message"])

	if ticketID == 0 || status == "" {
		http.Error(w, "Missing ticket_id or status", http.StatusBadRequest)
		return
	}

	if status != "Open" {
		// Actualizar estado del ticket
		updateQuery := map[string]interface{}{
			"table":     "tickets",
			"columns":   "status",
			"value":     status,
			"condition": fmt.Sprintf("id=%d", ticketID),
		}
		updateJSON, _ := json.Marshal(updateQuery)
		resp := putReq(updateJSON, apiKey, client, w)
		if resp != nil {
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
		}
	} else {
		// Actualizar estado del ticket
		updateQuery := map[string]interface{}{
			"table":     "tickets",
			"columns":   "status, assigned_to",
			"value":     fmt.Sprintf(("%s, "), status),
			"condition": fmt.Sprintf("id=%d", ticketID),
		}
		updateJSON, _ := json.Marshal(updateQuery)
		resp := putReq(updateJSON, apiKey, client, w)
		if resp != nil {
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
		}
	}
	var comment string
	if strings.TrimSpace(message) == "" {
		comment = fmt.Sprintf("Status updated to ¤%s¤.", status)
	} else {
		comment = fmt.Sprintf("Status updated to ¤%s¤. Message: %s", status, escapeComma(message))
	}

	// Añadir entrada en ticket_history
	now := time.Now().Format("2006-01-02 15:04")
	prevID := getLastHistoryID(ticketID, apiKey, client, w)
	var values string
	if prevID > 0 {
		values = fmt.Sprintf("%d, %s, %s, %d, %s", ticketID, status, now, prevID, comment)
	} else {
		values = fmt.Sprintf("%d, %s, %s, NULL, %s", ticketID, status, now, comment)
	}

	historyQuery := map[string]interface{}{
		"table":   "ticket_history",
		"columns": "ticket_id, status, updated, old_status, comment",
		"value":   values,
	}
	historyJSON, _ := json.Marshal(historyQuery)
	hResp := postReq(historyJSON, apiKey, client, w)
	if hResp != nil {
		defer hResp.Body.Close()
		io.ReadAll(hResp.Body)
	}

	// només enviem notificació si no s'ha tornat a l'estat de reassignació
	// (evitar conflictes)

	if status != "Open" {
		//get userid
		getQuery := map[string]interface{}{
			"table":     "tickets",
			"columns":   "user_id",
			"condition": fmt.Sprintf("id=%d", ticketID),
		}
		getJSON, _ := json.Marshal(getQuery)
		resp := getReq(getJSON, apiKey, client, w)
		if resp == nil {
			createLog("Failed to retrieve user id from DB", 1, apiKey, client, w)
			http.Error(w, "Failed to retrieve user id from DB", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		var ids []map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&ids)
		userID := int(ids[0]["user_id"].(float64))

		// crear notificació
		var notifBody string
		if status == "Closed" {
			notifBody = escapeComma(fmt.Sprintf("Your ticket has been updated to status ¤%s¤. %s", status, message))
		} else {
			notifBody = escapeComma(fmt.Sprintf("Your ticket has been updated to status ¤%s¤. %s You will be notified of any status change.", status, message))
		}
		query := map[string]interface{}{
			"table":   "notifications",
			"columns": "type, title, content, user_id, created_at",
			"value":   fmt.Sprintf("Ticket, Ticket updated successfully, %s, 304, %s", notifBody, now),
		}

		postJSON, _ := json.Marshal(query)
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
			"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s'", now),
		}
		idJSON, _ := json.Marshal(getID)
		idResp := getReq(idJSON, apiKey, client, w)
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

		getEmail := map[string]interface{}{
			"table":     "tickets",
			"columns":   "email",
			"condition": fmt.Sprintf("id = %d", ticketID),
		}
		emailJSON, _ := json.Marshal(getEmail)
		emailResp := getReq(emailJSON, apiKey, client, w)
		if emailResp != nil {
			defer emailResp.Body.Close()
			var data []map[string]interface{}
			if err := json.NewDecoder(emailResp.Body).Decode(&data); err == nil && len(data) > 0 {
				emailAddr := fmt.Sprint(data[0]["email"])
				if emailAddr != "" {

					tmplContent, err := os.ReadFile("module6workers/assets/emailTicketUpdate.html")
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
						"Status":      status,
						"StatusClass": getStatusClass(status),
						"UpdatedAt":   now,
						"Message":     unescapeComma(notifBody),
					}

					var buf bytes.Buffer
					tmpl.Execute(&buf, data)
					sendEmail(emailAddr, fmt.Sprintf("Ticket #%d Update", ticketID), buf.String())
				}
			}
		}
	}

	w.Write([]byte(`{"status":"ok"}`))
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

// Devuelve el id de la última entrada de ticket_history de un ticket (o 0 si no hay)
func getLastHistoryID(ticketID int, apiKey string, client *http.Client, w http.ResponseWriter) int {

	query := map[string]interface{}{
		"table":     "ticket_history",
		"columns":   "MAX(id) AS id",
		"condition": fmt.Sprintf("ticket_id = %d", ticketID),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("getLastHistoryID: getReq returned nil", 1, apiKey, client, w)
		return 0
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) == 0 {
		println("[getLastHistoryID] cannot decode rows or empty result")
		return 0
	}

	raw := rows[0]["id"]
	if raw == nil {
		return 0
	}
	id, _ := strconv.Atoi(fmt.Sprint(raw))
	return id
}

func handleGetMyTickets(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Obtener usuario actual
	userID, _, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		createLog("Unauthorized - no userID", 1, apiKey, client, w)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Consultar tickets creados por el usuario
	query := map[string]interface{}{
		"table":     "tickets",
		"columns":   "id, department, issue, description, assigned_to, urgency, status, creation_date, file_path",
		"condition": fmt.Sprintf("user_id=%d", userID),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Failed to fetch tickets", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tickets []map[string]interface{}
	if err := json.Unmarshal(body, &tickets); err != nil {
		http.Error(w, "Invalid data", http.StatusInternalServerError)
		return
	}

	if len(tickets) == 0 {
		w.Write([]byte("[]"))
		return
	}

	// Para cada ticket: obtener el último estado y responsable
	for _, t := range tickets {
		ticketID, _ := strconv.Atoi(fmt.Sprint(t["id"]))

		// --- Obtener nombre del técnico asignado ---
		assignedRaw := t["assigned_to"]
		if assignedRaw == nil || fmt.Sprint(assignedRaw) == "0" || fmt.Sprint(assignedRaw) == "" {
			t["assigned_to"] = "Pending assignment"
		} else {
			assignedID, _ := strconv.Atoi(fmt.Sprint(assignedRaw))
			name := getPeopleName(assignedID, apiKey, client, w)
			if name == "" {
				name = fmt.Sprintf("Worker #%d", assignedID)
			}
			t["assigned_to"] = name
		}

		// --- Obtener última actualización (último registro en ticket_history) ---
		hQuery := map[string]interface{}{
			"table":     "ticket_history",
			"columns":   "status, updated, comment",
			"condition": fmt.Sprintf("ticket_id=%d ORDER BY id DESC LIMIT 1", ticketID),
		}
		hJSON, _ := json.Marshal(hQuery)
		hResp := getReq(hJSON, apiKey, client, w)
		if hResp != nil {
			defer hResp.Body.Close()
			hBody, _ := io.ReadAll(hResp.Body)

			var hist []map[string]interface{}
			if err := json.Unmarshal(hBody, &hist); err == nil && len(hist) > 0 {
				t["status"] = hist[0]["status"]
				t["updated"] = hist[0]["updated"]
				rawComment := fmt.Sprint(hist[0]["comment"])
				finalComment := ""
				const marker = "Message:"
				if idx := strings.Index(rawComment, marker); idx != -1 {
					msg := strings.TrimSpace(rawComment[idx+len(marker):])
					if msg != "" {
						finalComment = msg
					}
				}

				if finalComment != "" {
					t["comment"] = finalComment
				}
			} else {
				t["updated"] = t["creation_date"]
			}
		} else {
			t["updated"] = t["creation_date"]
		}
	}

	// Unescapear comas en descripción
	for i, t := range tickets {
		if desc, ok := t["description"].(string); ok {
			tickets[i]["description"] = unescapeComma(desc)
		}
		if comm, ok := t["comment"].(string); ok {
			tickets[i]["comment"] = unescapeComma(comm)
		}
	}

	// Enviar resultado
	out, _ := json.MarshalIndent(tickets, "", "  ")
	w.Write(out)
}

func handleConfirmCloseTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		TicketID string `json:"ticket_id"`
	}

	// Leer body
	bodyData, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyData))

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		println("ERROR decoding payload:", err.Error())
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	println("Parsed TicketID:", payload.TicketID)

	// Construcción del update
	update := map[string]interface{}{
		"table":     "tickets",
		"columns":   "status",
		"value":     "ClosedConfirmed",
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}

	updateJSON, _ := json.Marshal(update)

	// Enviar request a la DB
	resp := putReq(updateJSON, apiKey, client, w)
	if resp == nil {
		println("ERROR: putReq returned nil response")
		http.Error(w, "DB update failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Añadir entrada en ticket_history
	now := time.Now().Format("2006-01-02 15:04")
	ticketID, _ := strconv.Atoi(payload.TicketID)
	prevID := getLastHistoryID(ticketID, apiKey, client, w)
	var values string
	if prevID > 0 {
		values = fmt.Sprintf("%d, ClosedConfirmed, %s, %d", ticketID, now, prevID)
	} else {
		values = fmt.Sprintf("%d, ClosedConfirmed, %s, NULL", ticketID, now)
	}

	historyQuery := map[string]interface{}{
		"table":   "ticket_history",
		"columns": "ticket_id, status, updated, old_status",
		"value":   values,
	}
	historyJSON, _ := json.Marshal(historyQuery)
	hResp := postReq(historyJSON, apiKey, client, w)
	if hResp != nil {
		defer hResp.Body.Close()
		io.ReadAll(hResp.Body)
	}

	// Enviar respuesta final
	w.Write([]byte(`{"status":"ok"}`))
}

func handleReopenTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	fmt.Println("---- REOPEN TICKET START ----")

	var payload struct {
		TicketID string `json:"ticket_id"`
		Comment  string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	fmt.Printf("[PAYLOAD] TicketID=%s | Comment=%s\n", payload.TicketID, payload.Comment)

	// 1. Recuperar descripción actual
	ticketQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "description",
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}
	qbody, _ := json.Marshal(ticketQuery)

	fmt.Printf("[QUERY DESCRIPTION] %s\n", string(qbody))

	resp := getReq(qbody, apiKey, client, w)
	if resp == nil {
		fmt.Println("[ERROR] getReq returned nil while fetching description")
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	resp.Body.Close()

	oldDesc := ""
	if len(rows) > 0 {
		oldDesc, _ = rows[0]["description"].(string)
	}

	newDesc := oldDesc + "\n\n[Reopened by user]: " + payload.Comment

	update := map[string]interface{}{
		"table":     "tickets",
		"columns":   "status, assigned_to, urgency, description",
		"value":     fmt.Sprintf("Open, , , %s", escapeComma(newDesc)),
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}

	ubody, _ := json.Marshal(update)
	fmt.Printf("[REOPEN] UPDATE QUERY JSON → %s\n", string(ubody))
	resp2 := putReq(ubody, apiKey, client, w)
	if resp2 != nil {
		defer resp2.Body.Close()
		io.ReadAll(resp2.Body)
	}

	created := time.Now().Format("2006-01-02 15:04")

	// ---  Construir query SQL ---
	query := map[string]interface{}{
		"table":     "tickets",
		"columns":   "department, issue, creation_date, user_id, email",
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}

	getJSON, _ := json.Marshal(query)
	resp = getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	var tickets []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&tickets)
	resp.Body.Close()

	fmt.Printf("[REOPEN] BASE DATA RAW → %#v\n", tickets)

	department := ""
	issue := ""
	creationDate := ""
	userID := ""
	email := ""
	if len(tickets) > 0 {
		department, _ = tickets[0]["department"].(string)
		issue, _ = tickets[0]["issue"].(string)
		creationDate, _ = tickets[0]["creation_date"].(string)
		rawUID := tickets[0]["user_id"]

		switch v := rawUID.(type) {
		case string:
			userID = v
		case float64:
			userID = fmt.Sprintf("%d", int(v))
		default:
			fmt.Printf("[REOPEN][WARN] user_id has unexpected type: %T\n", v)
		}

		email, _ = tickets[0]["email"].(string)
	}

	fmt.Printf("[REOPEN] Parsed fields → DEPT=%s | ISSUE=%s | DATE=%s | USER=%s | EMAIL=%s\n",
		department, issue, creationDate, userID, email)

	// Añadir entrada en ticket_history
	now := time.Now().Format("2006-01-02 15:04")
	ticketID, _ := strconv.Atoi(payload.TicketID)
	prevID := getLastHistoryID(ticketID, apiKey, client, w)
	var values string
	if prevID > 0 {
		values = fmt.Sprintf("%d, Open, %s, %d, %s", ticketID, now, prevID, escapeComma(newDesc))
	} else {
		values = fmt.Sprintf("%d, Open, %s, NULL, %s", ticketID, now, escapeComma(newDesc))
	}

	historyQuery := map[string]interface{}{
		"table":   "ticket_history",
		"columns": "ticket_id, status, updated, old_status, comment",
		"value":   values,
	}
	historyJSON, _ := json.Marshal(historyQuery)
	hResp := postReq(historyJSON, apiKey, client, w)
	if hResp != nil {
		defer hResp.Body.Close()
		io.ReadAll(hResp.Body)
	}

	// crear notificació
	notifBody := escapeComma("Your ticket has been succesfully sent to the department administration. You will be notified for every status change.")
	query = map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Ticket, Ticket reopened successfully, %s, 304, %s", notifBody, created),
	}

	postJSON, _ := json.Marshal(query)
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
	idResp := getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new notification ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get notification ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var notifId []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&notifId)
	notifID := int(notifId[0]["id"].(float64))

	// Sends to users creating an entry to notification_user table

	link := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read, can_download, seen",
		"value":   fmt.Sprintf("%d, %s, 1, 0, 0", notifID, userID),
	}
	jsonLink, _ := json.Marshal(link)
	resp = postReq(jsonLink, apiKey, client, w)
	if resp == nil {
		createLog("Failed to send notification", 1, apiKey, client, w)
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	//enviar mail si ha proporcionat un

	if email != "" {

		tmplContent, err := os.ReadFile("module6workers/assets/emailTicketCreation.html")
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
			"TicketID":    payload.TicketID,
			"Status":      "Open",
			"StatusClass": getStatusClass("Open"),
			"UpdatedAt":   created,
			"Message":     unescapeComma(notifBody),
		}

		var buf bytes.Buffer
		tmpl.Execute(&buf, data)
		sendEmail(email, fmt.Sprintf("Ticket #%s Re-opened", payload.TicketID), buf.String())
	}

	// enviar mail als encarregats del departament
	managers := map[string]interface{}{
		"table":     "people p LEFT JOIN department_workers d ON p.id = d.worker_id",
		"columns":   "p.crm_email, p.name, p.surname",
		"condition": fmt.Sprintf("d.department_name = '%s' AND d.manager = '1'", department),
	}

	fmt.Println("DEBUG -> Managers query:", managers)

	managersMail, _ := json.Marshal(managers)
	idResp = getReq(managersMail, apiKey, client, w)

	if idResp == nil {
		createLog("Failed to retrieve managers mail from database", 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve managers mail", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()

	var mails []map[string]interface{}
	err := json.NewDecoder(idResp.Body).Decode(&mails)

	fmt.Println("DEBUG -> Error decodificando respuesta:", err)
	fmt.Println("DEBUG -> mails recibidos:", mails)
	fmt.Printf("DEBUG -> Total managers: %d\n", len(mails))

	if len(mails) > 0 {
		// aconseguir nom de qui ha creat el ticket

		// enviar mail als encarregats del departament
		creationUser := map[string]interface{}{
			"table":     "people",
			"columns":   "name, surname",
			"condition": fmt.Sprintf("id = '%s'", userID),
		}

		fmt.Println("DEBUG -> Managers query:", creationUser)

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

		tmplContent, err := os.ReadFile("module6workers/assets/notifyAdministrator.html")
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
			println(m)
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
				"TicketID":     payload.TicketID,
				"Department":   department,
				"Issue":        issue,
				"Description":  unescapeComma(newDesc),
				"CreationDate": creationDate,
				"UserName":     fullName,
				"CreatedBy":    createdBy,
				"Status":       "Open",
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				createLog("Error ejecutando plantilla manager: "+err.Error(), 1, apiKey, client, w)
				continue
			}

			// enviar email
			sendEmail(mail, fmt.Sprintf("Ticket Re-opened: #%s", payload.TicketID), buf.String())
		}
	}

	w.Write([]byte(`{"status":"ok"}`))
}

func handleReplyTicket(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	fmt.Println("---- Reply TICKET START ----")

	var payload struct {
		TicketID string `json:"ticket_id"`
		Message  string `json:"message"`
		Comment  string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	fmt.Printf("[PAYLOAD] TicketID=%s | Comment=%s\n", payload.TicketID, payload.Comment)

	// 1. Recuperar descripción actual
	ticketQuery := map[string]interface{}{
		"table":     "tickets",
		"columns":   "description",
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}
	qbody, _ := json.Marshal(ticketQuery)

	fmt.Printf("[QUERY DESCRIPTION] %s\n", string(qbody))

	resp := getReq(qbody, apiKey, client, w)
	if resp == nil {
		fmt.Println("[ERROR] getReq returned nil while fetching description")
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	resp.Body.Close()

	oldDesc := ""
	if len(rows) > 0 {
		oldDesc, _ = rows[0]["description"].(string)
	}

	newDesc := oldDesc

	if strings.TrimSpace(payload.Message) != "" {
		newDesc += "\n\n[Technician's comment]: " + payload.Message
	}

	newDesc += "\n\n[Reply by user]: " + payload.Comment

	update := map[string]interface{}{
		"table":     "tickets",
		"columns":   "status, description",
		"value":     fmt.Sprintf("In progress, %s", escapeComma(newDesc)),
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}

	ubody, _ := json.Marshal(update)
	fmt.Printf("[REPLY] UPDATE QUERY JSON → %s\n", string(ubody))
	resp2 := putReq(ubody, apiKey, client, w)
	if resp2 != nil {
		defer resp2.Body.Close()
		io.ReadAll(resp2.Body)
	}

	created := time.Now().Format("2006-01-02 15:04")

	// ---  Construir query SQL ---
	query := map[string]interface{}{
		"table":     "tickets",
		"columns":   "department, issue, creation_date, user_id, email",
		"condition": fmt.Sprintf("id='%s'", payload.TicketID),
	}

	getJSON, _ := json.Marshal(query)
	resp = getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return
	}
	var tickets []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&tickets)
	resp.Body.Close()

	fmt.Printf("[REOPEN] BASE DATA RAW → %#v\n", tickets)

	department := ""
	issue := ""
	creationDate := ""
	userID := ""
	email := ""
	if len(tickets) > 0 {
		department, _ = tickets[0]["department"].(string)
		issue, _ = tickets[0]["issue"].(string)
		creationDate, _ = tickets[0]["creation_date"].(string)
		rawUID := tickets[0]["user_id"]

		switch v := rawUID.(type) {
		case string:
			userID = v
		case float64:
			userID = fmt.Sprintf("%d", int(v))
		default:
			fmt.Printf("[REOPEN][WARN] user_id has unexpected type: %T\n", v)
		}

		email, _ = tickets[0]["email"].(string)
	}

	fmt.Printf("[REOPEN] Parsed fields → DEPT=%s | ISSUE=%s | DATE=%s | USER=%s | EMAIL=%s\n",
		department, issue, creationDate, userID, email)

	// Añadir entrada en ticket_history
	now := time.Now().Format("2006-01-02 15:04")
	ticketID, _ := strconv.Atoi(payload.TicketID)
	prevID := getLastHistoryID(ticketID, apiKey, client, w)
	var values string
	if prevID > 0 {
		values = fmt.Sprintf("%d, In progress, %s, %d, %s", ticketID, now, prevID, escapeComma(newDesc))
	} else {
		values = fmt.Sprintf("%d, In progress, %s, NULL, %s", ticketID, now, escapeComma(newDesc))
	}

	historyQuery := map[string]interface{}{
		"table":   "ticket_history",
		"columns": "ticket_id, status, updated, old_status, comment",
		"value":   values,
	}
	historyJSON, _ := json.Marshal(historyQuery)
	hResp := postReq(historyJSON, apiKey, client, w)
	if hResp != nil {
		defer hResp.Body.Close()
		io.ReadAll(hResp.Body)
	}

	// crear notificació
	notifBody := escapeComma(fmt.Sprintf("Your ticket has been succesfully sent to the department administration with your reply: %s. You will be notified for every status change.", payload.Comment))
	query = map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Ticket, Ticket replied successfully, %s, 304, %s", notifBody, created),
	}

	postJSON, _ := json.Marshal(query)
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
		"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s' AND content = '%s'", created, notifBody),
	}
	idJSON, _ := json.Marshal(getID)
	idResp := getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new notification ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get notification ID", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()
	var notifId []map[string]interface{}
	_ = json.NewDecoder(idResp.Body).Decode(&notifId)
	notifID := int(notifId[0]["id"].(float64))

	// Sends to users creating an entry to notification_user table

	link := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read, can_download, seen",
		"value":   fmt.Sprintf("%d, %s, 1, 0, 0", notifID, userID),
	}
	jsonLink, _ := json.Marshal(link)
	resp = postReq(jsonLink, apiKey, client, w)
	if resp == nil {
		createLog("Failed to send notification", 1, apiKey, client, w)
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	//enviar mail si ha proporcionat un

	if email != "" {

		tmplContent, err := os.ReadFile("module6workers/assets/emailTicketCreation.html")
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
			"TicketID":    payload.TicketID,
			"Status":      "In progress",
			"StatusClass": getStatusClass("In progress"),
			"UpdatedAt":   created,
			"Message":     unescapeComma(notifBody),
		}

		var buf bytes.Buffer
		tmpl.Execute(&buf, data)
		sendEmail(email, fmt.Sprintf("Ticket #%s Replied", payload.TicketID), buf.String())
	}

	// enviar mail als encarregats del departament
	managers := map[string]interface{}{
		"table":     "people p LEFT JOIN department_workers d ON p.id = d.worker_id",
		"columns":   "p.crm_email, p.name, p.surname",
		"condition": fmt.Sprintf("d.department_name = '%s' AND d.manager = '1'", department),
	}

	fmt.Println("DEBUG -> Managers query:", managers)

	managersMail, _ := json.Marshal(managers)
	idResp = getReq(managersMail, apiKey, client, w)

	if idResp == nil {
		createLog("Failed to retrieve managers mail from database", 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve managers mail", http.StatusInternalServerError)
		return
	}
	defer idResp.Body.Close()

	var mails []map[string]interface{}
	err := json.NewDecoder(idResp.Body).Decode(&mails)

	fmt.Println("DEBUG -> Error decodificando respuesta:", err)
	fmt.Println("DEBUG -> mails recibidos:", mails)
	fmt.Printf("DEBUG -> Total managers: %d\n", len(mails))

	if len(mails) > 0 {
		// aconseguir nom de qui ha creat el ticket

		// enviar mail als encarregats del departament
		creationUser := map[string]interface{}{
			"table":     "people",
			"columns":   "name, surname",
			"condition": fmt.Sprintf("id = '%s'", userID),
		}

		fmt.Println("DEBUG -> Managers query:", creationUser)

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

		tmplContent, err := os.ReadFile("module6workers/assets/notifyAdministrator.html")
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
			println(m)
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
				"TicketID":     payload.TicketID,
				"Department":   department,
				"Issue":        issue,
				"Description":  unescapeComma(newDesc),
				"CreationDate": creationDate,
				"UserName":     fullName,
				"CreatedBy":    createdBy,
				"Status":       "In progress",
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				createLog("Error ejecutando plantilla manager: "+err.Error(), 1, apiKey, client, w)
				continue
			}

			// enviar email
			sendEmail(mail, fmt.Sprintf("Ticket Replied: #%s", payload.TicketID), buf.String())
		}
	}

	w.Write([]byte(`{"status":"ok"}`))
}
