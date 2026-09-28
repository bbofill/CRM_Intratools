package module3workers

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
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	if mC.Module3ApiKey == inkey {
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

// returns string in front-end format
func unescapeComma(s string) string {
	s = strings.ReplaceAll(s, "¤", "'")
	return strings.ReplaceAll(s, "§", ",")
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

// ------------------------------ Get data of user using module --------------------------------------------
// GET user_id by username
func jsonUsernameID(user string) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id, role",
		"condition": fmt.Sprintf("username = '%s'", user),
	}
	return query
}

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
			return 0, "superadmin", "superadmin"
		} else {
			createLog(fmt.Sprintf("User %s not found for cookie", user), 130, apiKey, client, w)
			return 0, "", ""
		}
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

//////////////////////////////////////////////////////////////////////////////////////////////////
//                                  PROFILE VISUALIZATION                                       //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Returns all user info
func handleGetUserProfile(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, _, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		createLog("Failed to retrieve userID in handleGetUserProfile", 1, apiKey, nil, w)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// ------------------- PEOPLE -------------------
	queryPeople := map[string]interface{}{
		"table":     "people",
		"columns":   "*",
		"condition": fmt.Sprintf("id = %d", userID),
	}
	jsonPeople, _ := json.Marshal(queryPeople)
	respPeople := getReq(jsonPeople, apiKey, client, w)
	if respPeople == nil {
		createLog("DB response nil in people query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respPeople.Body.Close()

	var peopleRes []map[string]interface{}
	if err := json.NewDecoder(respPeople.Body).Decode(&peopleRes); err != nil {
		createLog(fmt.Sprintf("Error decoding people response: %v", err), 1, apiKey, nil, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	if len(peopleRes) == 0 {
		createLog(fmt.Sprintf("No people entry found for user %d", userID), 1, apiKey, nil, w)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	p := unescapeMap(peopleRes[0])

	// ------------------- CONTRACTS -------------------
	queryContracts := map[string]interface{}{
		"table":     "contract",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonContracts, _ := json.Marshal(queryContracts)
	respContracts := getReq(jsonContracts, apiKey, client, w)
	if respContracts == nil {
		createLog("DB response nil in contracts query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respContracts.Body.Close()
	var contractsRaw []map[string]interface{}
	if err := json.NewDecoder(respContracts.Body).Decode(&contractsRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding contracts: %v", err), 1, apiKey, nil, w)
	}
	for i := range contractsRaw {
		contractsRaw[i] = unescapeMap(contractsRaw[i])
	}

	// map to module4 field names
	contracts := make([]map[string]interface{}, 0, len(contractsRaw))
	for _, c := range contractsRaw {

		idVal, ok := c["id"].(float64)
		if !ok {
			createLog(fmt.Sprintf("Invalid PhD ID format: %v", c["id"]), 1, apiKey, nil, w)
			continue
		}
		contractID := int(idVal)

		// ------------------- RESPONSIBLES -------------------
		queryResp := map[string]interface{}{
			"table":     "people_supervisor",
			"columns":   "*",
			"condition": fmt.Sprintf("contract_id = %d", contractID),
		}
		jsonResp, _ := json.Marshal(queryResp)

		respResp := getReq(jsonResp, apiKey, client, w)
		if respResp == nil {
			createLog(fmt.Sprintf("DB response nil in responsibles query (contract_id=%d)", contractID), 1, apiKey, nil, w)
			continue
		}
		var respRaw []map[string]interface{}
		if err := json.NewDecoder(respResp.Body).Decode(&respRaw); err != nil {
			createLog(fmt.Sprintf("Error decoding responsibles (contract_id=%d): %v", contractID, err), 1, apiKey, nil, w)
		}
		respResp.Body.Close()

		for i := range respRaw {
			respRaw[i] = unescapeMap(respRaw[i])
		}

		supervisors := make([]map[string]interface{}, 0, len(respRaw))
		for _, r0 := range respRaw {
			supervisors = append(supervisors, map[string]interface{}{
				"supervisor_id": r0["supervisor_id"],
			})
		}

		contracts = append(contracts, map[string]interface{}{
			"contract_id":             c["id"],
			"vinculation_type":        c["vinculation_type"],
			"trainee_type":            c["trainee_type"],
			"internship":              c["internship"],
			"trainee_studies":         c["trainee_studies"],
			"contract_start_date":     c["start_date"],
			"contract_end_date":       c["end_date"],
			"job_category":            c["job_category"],
			"contract_type":           c["type"],
			"position":                c["position"],
			"totalDedication_hours":   c["totalDedication_hours"],
			"office_location":         c["office_location"],
			"research_area":           c["research_area"],
			"funding":                 c["funding"],
			"contracting_institution": c["contracting_institution"],
			"contract_file_path":      c["file_path"],
			"supervisor":              supervisors,
		})

	}

	// ------------------- RESIDENCE -------------------
	queryResidences := map[string]interface{}{
		"table":     "residence",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonRes, _ := json.Marshal(queryResidences)
	respRes := getReq(jsonRes, apiKey, client, w)
	if respRes == nil {
		createLog("DB response nil in residence query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respRes.Body.Close()
	var residencesRaw []map[string]interface{}
	if err := json.NewDecoder(respRes.Body).Decode(&residencesRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding residence: %v", err), 1, apiKey, nil, w)
	}
	for i := range residencesRaw {
		residencesRaw[i] = unescapeMap(residencesRaw[i])
	}
	residences := make([]map[string]interface{}, 0, len(residencesRaw))
	for _, r0 := range residencesRaw {
		residences = append(residences, map[string]interface{}{
			"residence_country":  r0["residence_country"],
			"residence_province": r0["residence_province"],
			"residence_city":     r0["residence_city"],
			"postal_code":        r0["postal_code"],
			"address":            r0["address"],
			"actual":             r0["actual"],
		})
	}

	// ------------------- NATIONALITIES -------------------
	queryNat := map[string]interface{}{
		"table":     "people_nationality",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonNat, _ := json.Marshal(queryNat)
	respNat := getReq(jsonNat, apiKey, client, w)
	if respNat == nil {
		createLog("DB response nil in nationality query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respNat.Body.Close()
	var nationalitiesRaw []map[string]interface{}
	if err := json.NewDecoder(respNat.Body).Decode(&nationalitiesRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding nationality: %v", err), 1, apiKey, nil, w)
	}
	for i := range nationalitiesRaw {
		nationalitiesRaw[i] = unescapeMap(nationalitiesRaw[i])
	}
	nationalities := make([]map[string]interface{}, 0, len(nationalitiesRaw))
	for _, n0 := range nationalitiesRaw {
		nationalities = append(nationalities, map[string]interface{}{
			"nationality_code": n0["nationality_code"],
		})
	}

	// ------------------- GRADE / EDUCATION -------------------
	queryGrades := map[string]interface{}{
		"table":     "people_grade",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonGrades, _ := json.Marshal(queryGrades)
	respGrades := getReq(jsonGrades, apiKey, client, w)
	if respGrades == nil {
		createLog("DB response nil in education query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respGrades.Body.Close()
	var gradesRaw []map[string]interface{}
	if err := json.NewDecoder(respGrades.Body).Decode(&gradesRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding education: %v", err), 1, apiKey, nil, w)
	}
	for i := range gradesRaw {
		gradesRaw[i] = unescapeMap(gradesRaw[i])
	}
	education := make([]map[string]interface{}, 0, len(gradesRaw))
	for _, g := range gradesRaw {
		education = append(education, map[string]interface{}{
			"grade_id":               g["id"],
			"grade_master_doctorate": g["grade_master_doctorate"],
			"grade_code":             g["code"],
			"gradeName":              g["gradeName"],
			"grade_university_name":  g["universityName"],
			"graduation_university":  g["graduation_university"],
			"graduation_country":     g["graduation_country"],
			"graduation_year":        g["graduation_year"],
		})
	}

	// ------------------- GROUPS -------------------
	queryGroups := map[string]interface{}{
		"table":     "people_group",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonGroups, _ := json.Marshal(queryGroups)
	respGroups := getReq(jsonGroups, apiKey, client, w)
	if respGroups == nil {
		createLog("DB response nil in groups query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respGroups.Body.Close()
	var groupsRaw []map[string]interface{}
	if err := json.NewDecoder(respGroups.Body).Decode(&groupsRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding groups: %v", err), 1, apiKey, nil, w)
	}
	for i := range groupsRaw {
		groupsRaw[i] = unescapeMap(groupsRaw[i])
	}
	groups := make([]map[string]interface{}, 0, len(groupsRaw))
	for _, gr := range groupsRaw {
		groups = append(groups, map[string]interface{}{
			"group_intern_code": gr["group_intern_code"],
			"group_start_date":  gr["start_date"],
			"group_end_date":    gr["end_date"],
			"group_ip":          gr["ip"],
		})
	}

	// ------------------- PROJECTS -------------------
	queryProjects := map[string]interface{}{
		"table": `people_projects pp
			LEFT JOIN projects pr ON pp.project_id = pr.id`,
		"columns": `
			pp.project_id,
			pp.start_date AS project_start_date,
			pp.end_date AS project_end_date,
			pp.researcher_type,
			pr.name AS project_name,
			pr.short_name AS project_short_name,
			pr.number AS project_number,
			pr.type AS project_type
		`,
		"condition": fmt.Sprintf("pp.people_id = %d", userID),
	}
	jsonProjects, _ := json.Marshal(queryProjects)
	respProjects := getReq(jsonProjects, apiKey, client, w)
	if respProjects == nil {
		createLog("DB response nil in projects query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respProjects.Body.Close()
	var projectsRaw []map[string]interface{}
	if err := json.NewDecoder(respProjects.Body).Decode(&projectsRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding projects: %v", err), 1, apiKey, nil, w)
	}
	for i := range projectsRaw {
		projectsRaw[i] = unescapeMap(projectsRaw[i])
	}
	projects := make([]map[string]interface{}, 0, len(projectsRaw))
	for _, pr := range projectsRaw {
		projects = append(projects, map[string]interface{}{
			"project_id":         pr["project_id"],
			"project_name":       pr["project_name"],
			"project_short_name": pr["project_short_name"],
			"project_number":     pr["project_number"],
			"project_type":       pr["project_type"],
			"researcher_type":    pr["researcher_type"],
			"project_start_date": pr["project_start_date"],
			"project_end_date":   pr["project_end_date"],
		})
	}

	// ------------------- TRAINING -------------------
	queryTrain := map[string]interface{}{
		"table":     "people_training",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonTrain, _ := json.Marshal(queryTrain)
	respTrain := getReq(jsonTrain, apiKey, client, w)
	if respTrain == nil {
		createLog("DB response nil in training query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respTrain.Body.Close()
	var trainingsRaw []map[string]interface{}
	if err := json.NewDecoder(respTrain.Body).Decode(&trainingsRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding training: %v", err), 1, apiKey, nil, w)
	}
	for i := range trainingsRaw {
		trainingsRaw[i] = unescapeMap(trainingsRaw[i])
	}
	trainings := make([]map[string]interface{}, 0, len(trainingsRaw))
	for _, t := range trainingsRaw {
		trainings = append(trainings, map[string]interface{}{
			"training_id":           t["training_id"],
			"training_date":         t["date"],
			"training_enrolled":     t["enrolled"],
			"training_diploma_path": t["diploma_path"],
		})
	}

	// ------------------- PHD -------------------
	queryPhd := map[string]interface{}{
		"table":     "people_phd",
		"columns":   "*",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonPhd, _ := json.Marshal(queryPhd)
	respPhd := getReq(jsonPhd, apiKey, client, w)
	if respPhd == nil {
		createLog("DB response nil in phd query", 1, apiKey, nil, w)
		http.Error(w, "Database error", http.StatusBadGateway)
		return
	}
	defer respPhd.Body.Close()

	var phdRaw []map[string]interface{}
	if err := json.NewDecoder(respPhd.Body).Decode(&phdRaw); err != nil {
		createLog(fmt.Sprintf("Error decoding phd: %v", err), 1, apiKey, nil, w)
	}

	for i := range phdRaw {
		phdRaw[i] = unescapeMap(phdRaw[i])
	}

	phd := make([]map[string]interface{}, 0, len(phdRaw))
	for _, ph := range phdRaw {
		idVal, ok := ph["id"].(float64)
		if !ok {
			createLog(fmt.Sprintf("Invalid PhD ID format: %v", ph["id"]), 1, apiKey, nil, w)
			continue
		}
		phdID := int(idVal)

		// ------------------- RESPONSIBLES -------------------
		queryResp := map[string]interface{}{
			"table":     "responsible",
			"columns":   "*",
			"condition": fmt.Sprintf("tesis_id = %d", phdID),
		}
		jsonResp, _ := json.Marshal(queryResp)

		respResp := getReq(jsonResp, apiKey, client, w)
		if respResp == nil {
			createLog(fmt.Sprintf("DB response nil in responsibles query (tesis_id=%d)", phdID), 1, apiKey, nil, w)
			continue
		}
		var respRaw []map[string]interface{}
		if err := json.NewDecoder(respResp.Body).Decode(&respRaw); err != nil {
			createLog(fmt.Sprintf("Error decoding responsibles (tesis_id=%d): %v", phdID, err), 1, apiKey, nil, w)
		}
		respResp.Body.Close()

		for i := range respRaw {
			respRaw[i] = unescapeMap(respRaw[i])
		}

		responsibles := make([]map[string]interface{}, 0, len(respRaw))
		for _, r0 := range respRaw {
			responsibles = append(responsibles, map[string]interface{}{
				"ip_or_tutor": r0["ip_or_tutor"],
				"people_id":   r0["people_id"],
				"name":        r0["name"],
			})
		}

		phd = append(phd, map[string]interface{}{
			"phd_id":                      phdID,
			"phd_program":                 ph["phd_program"],
			"phd_university":              ph["phd_university"],
			"phd_startYear":               ph["phd_startYear"],
			"phd_tesisDirector":           ph["phd_tesisDirector"],
			"phd_tesisTitle":              ph["phd_tesisTitle"],
			"phd_plannedPresentationDate": ph["phd_plannedPresentationDate"],
			"phd_presentationDate":        ph["phd_presentationDate"],
			"phd_link":                    ph["phd_link"],
			"responsible":                 responsibles,
		})
	}

	// ------------------- PROFILE OBJECT -------------------
	profile := map[string]interface{}{
		"general": map[string]interface{}{
			"id":                     p["id"],
			"picture_path":           p["picture_path"],
			"people_name":            p["name"], // <- módulo 4 usa 'people_name'
			"prefered_name":          p["prefered_name"],
			"surname":                p["surname"],
			"secondSurname":          p["secondSurname"],
			"gender":                 p["gender"],
			"birth_date":             p["birth_date"],
			"birth_country":          p["birth_country"],
			"birth_province":         p["birth_province"],
			"birth_city":             p["birth_city"],
			"nif":                    p["nif"],
			"nif_extended":           p["nif_extended"],
			"user_phone":             p["user_phone"],
			"emergencyContact_name":  p["emergencyContact_name"],
			"emergencyContact_phone": p["emergencyContact_phone"],
			"user_email":             p["user_email"],
			"people_idExternal":      p["people_idExternal"],
			"webUser_idExternal":     p["webUser_idExternal"],
			"crm_email":              p["crm_email"],
			"observations":           p["observations"],
			"academic_grade":         p["academic_grade"],
			"research_interests":     p["research_interests"],
			"orcid":                  p["orcid"],
			"certificat_I3":          p["certificat_I3"],
			"personal_webPage":       p["personal_webPage"],
			"agreesToUneix":          p["agreesToUneix"],
			"active":                 p["active"],
		},
		"nationality": nationalities,
		"residence":   residences,
		"contract":    contracts,
		"education":   education,
		"phd":         phd,
		"training":    trainings,
		"groups":      groups,
		"projects":    projects,
	}
	createLog(fmt.Sprintf("Profile data successfully retrieved for user %d", userID), 0, apiKey, nil, w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
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
		"researchGroup":  {"intern_code", "name"},
	}

	result := make(map[string][]map[string]interface{})

	for table, columns := range tables {
		query := map[string]interface{}{
			"table":   table,
			"columns": strings.Join(columns, ", "),
		}
		jsonQuery, err := json.Marshal(query)
		if err != nil {
			log.Printf("Error al generar JSON para %s: %v", table, err)
			http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
			return
		}
		resp := getReq(jsonQuery, apiKey, client, w)
		if resp == nil {
			log.Printf("No se obtuvo respuesta para la tabla: %s", table)
			return
		}
		defer resp.Body.Close()

		var tableData []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tableData); err != nil {
			log.Printf("Error al decodificar la respuesta de %s: %v", table, err)
			http.Error(w, "Error al decodificar respuesta de "+table, http.StatusInternalServerError)
			return
		}
		result[table] = tableData
	}

	query := map[string]interface{}{
		"table":     "room",
		"columns":   "id, name",
		"condition": "category = 'office'",
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		log.Printf("Error al generar JSON para room: %v", err)
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		return
	}

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		log.Printf("No se obtuvo respuesta para la tabla: room")
		return
	}
	defer resp.Body.Close()

	var tableData []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tableData); err != nil {
		log.Printf("Error al decodificar la respuesta de room: %v", err)
		http.Error(w, "Error al decodificar respuesta de room", http.StatusInternalServerError)
		return
	}

	result["room"] = tableData

	query = map[string]interface{}{
		"table":   "fundings",
		"columns": "id, name, code",
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
		log.Printf("Error al codificar JSON final: %v", err)
		http.Error(w, "Error al generar respuesta JSON", http.StatusInternalServerError)
		return
	}
}

// ------------------------ Formats data visualization --------------------------------------
// unescapeMap recorre un map[string]interface{} y aplica unescapeComma a todos los strings
func unescapeMap(m map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case string:
			cleaned[k] = unescapeComma(val)
		case map[string]interface{}:
			cleaned[k] = unescapeMap(val) // recursivo
		case []interface{}:
			cleaned[k] = unescapeSlice(val)
		default:
			cleaned[k] = v
		}
	}
	return cleaned
}

// unescapeSlice recorre slices y aplica unescape a strings y maps
func unescapeSlice(arr []interface{}) []interface{} {
	cleaned := make([]interface{}, len(arr))
	for i, v := range arr {
		switch val := v.(type) {
		case string:
			cleaned[i] = unescapeComma(val)
		case map[string]interface{}:
			cleaned[i] = unescapeMap(val)
		case []interface{}:
			cleaned[i] = unescapeSlice(val)
		default:
			cleaned[i] = v
		}
	}
	return cleaned
}

// DEPRECATED - Change suggestions are no longer used
// Sends notification of type = profile_request
func handleSendProfileSuggestion(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Changes map[string]interface{} `json:"changes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	//Ignore camps with no change or empty
	filtered := make(map[string]interface{})
	for k, v := range payload.Changes {
		if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
			filtered[k] = strings.TrimSpace(str)
		}
	}
	if len(filtered) == 0 {
		http.Error(w, "No valid changes provided", http.StatusBadRequest)
		return
	}
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	//Prepares notification content
	title := fmt.Sprintf("Profile change request from %s", username)
	contentBytes, err := json.Marshal(filtered)
	if err != nil {
		http.Error(w, "Error encoding content", http.StatusInternalServerError)
		return
	}
	now := time.Now().Format("2006-01-02 15:04")
	//Content sent encoded to avoid errors with punctuation
	encodedContent := b64.StdEncoding.EncodeToString(contentBytes)
	valueStr := fmt.Sprintf("profile-request, %s, '%s', %s, %d", title, encodedContent, now, userID)

	//payload to insert notification
	insertNotification := map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, created_at, user_id",
		"value":   valueStr,
	}
	notifJSON, _ := json.Marshal(insertNotification)
	respNotif := postReq(notifJSON, apiKey, client, w)
	defer respNotif.Body.Close()

	//Get id of inserted notification
	getLastID := map[string]interface{}{
		"table":   "notifications",
		"columns": "last_insert_rowid() AS id",
	}
	lastIDJSON, _ := json.Marshal(getLastID)
	respID := getReq(lastIDJSON, apiKey, client, w)
	if respID == nil {
		fmt.Println("Error: respID es nil al intentar obtener last_insert_rowid")
		http.Error(w, "Failed to get last insert ID", http.StatusInternalServerError)
		return
	}
	defer respID.Body.Close()
	var rows []map[string]interface{}
	_ = json.NewDecoder(respID.Body).Decode(&rows)
	notifID := int(rows[0]["id"].(float64))

	//IMPORTANTE!! CAMBIAR USUARIO DE RRHH ASIGNADO A QUIEN LE LLEGA ESTE TIPO DE NOTIFICACIONES
	//hardcoded to 1 (ltorrescusa)

	//payload to insert notification_user
	insertForHR := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read, can_download, seen",
		"value":   fmt.Sprintf("%d, %d, 1, 0, 0", notifID, 1),
	}
	jsonNotifUser, _ := json.Marshal(insertForHR)

	respInsertUser := postReq(jsonNotifUser, apiKey, client, w)
	if respInsertUser == nil || respInsertUser.StatusCode >= 400 {
		fmt.Println("Failed to insert into notification_user")
		if respInsertUser != nil {
			body, _ := io.ReadAll(respInsertUser.Body)
			fmt.Println("Response body:", string(body))
		}
		http.Error(w, "Failed to assign notification to RRHH", http.StatusInternalServerError)
		return
	}
	defer respInsertUser.Body.Close()
	w.WriteHeader(http.StatusOK)
}

// Sends notifications of user
func handleGetNotifications(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	userID, _, _ := getUserInfo(apiKey, client, w, r)

	//Inner join returns data that is the same in both tables
	//Left join always includes the data from the left table and fills with null if there is no data on the right one
	query := map[string]interface{}{
		"table": "notifications n " +
			"INNER JOIN notification_user nu ON n.id = nu.notification_id " +
			"LEFT JOIN users u ON n.user_id = u.people_id " +
			"LEFT JOIN people ux ON n.user_id = ux.id",
		"columns": "n.id, n.user_id, n.title, n.type, n.content, n.created_at, n.file_path, nu.seen, nu.can_read, nu.can_download, nu.profile_applied, " +
			"ux.name || ' ' || ux.surname AS sender_name, ux.picture_path AS sender_picture",
		"condition": fmt.Sprintf("nu.user_id = %d AND nu.can_read = 1", userID),
	}
	//GET data from notifications (content), notification_user (who can visualize) and users (sender)
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog("DB request returned nil in handleGetNotifications", 1, apiKey, nil, w)
		fmt.Println("Error: la respuesta HTTP es nil. No se pudieron obtener notificaciones.")
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("JSON decode error in handleGetNotifications: %v", err), 1, apiKey, nil, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	for _, notif := range result {
		if title, ok := notif["title"].(string); ok {
			notif["title"] = unescapeComma(title)
		}
		if content, ok := notif["content"].(string); ok {
			notif["content"] = unescapeComma(content)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// returns notifications sent by user
func handleGetSentNotifications(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	userID, _, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table": "notifications n LEFT JOIN people p ON n.user_id = p.id",
		"columns": "n.id, n.user_id, n.title, n.type, n.content, n.created_at, n.file_path, " +
			"p.name || ' ' || p.surname AS sender_name, p.picture_path AS sender_picture",
		"condition": fmt.Sprintf("n.user_id = %d", userID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog("DB request returned nil in handleGetNotifications", 1, apiKey, nil, w)
		fmt.Println("Error: la respuesta HTTP es nil. No se pudieron obtener notificaciones.")
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("JSON decode error in handleGetNotifications: %v", err), 1, apiKey, nil, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	for _, notif := range result {
		if title, ok := notif["title"].(string); ok {
			notif["title"] = unescapeComma(title)
		}
		if content, ok := notif["content"].(string); ok {
			notif["content"] = unescapeComma(content)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Seen=1 when user visualizes notification
func handleMarkNotificationSeen(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	//get notification id
	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog("Invalid JSON payload in handleMarkNotificationSeen", 1, apiKey, nil, w)
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	userID, _, _ := getUserInfo(apiKey, client, w, r)
	//payload to mark notification_user as seen
	update := map[string]interface{}{
		"table":     "notification_user",
		"columns":   "seen",
		"value":     "1",
		"condition": fmt.Sprintf("notification_id = %d AND user_id = %d", payload.ID, userID),
	}
	jsonUpdate, _ := json.Marshal(update)
	resp := putReq(jsonUpdate, apiKey, client, w)
	if resp == nil {
		createLog("DB update failed in handleMarkNotificationSeen", 1, apiKey, nil, w)
		http.Error(w, "Failed to mark as seen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
}

// DEPRECATED -Data is no longer modified in module 3- Updates data from uneix_user
func handleApplyProfileChanges(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		UserID         int                    `json:"user_id"`
		NotificationID int                    `json:"notification_id"`
		Changes        map[string]interface{} `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Println("Error decoding JSON payload:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.UserID == 0 || len(payload.Changes) == 0 {
		fmt.Println("Missing user_id or changes")
		http.Error(w, "Missing user_id or changes", http.StatusBadRequest)
		return
	}

	//Parse correctly requested changes in columns and values
	columns := []string{}
	values := []string{}
	for k, v := range payload.Changes {
		columns = append(columns, k)
		valStr := ""
		switch val := v.(type) {
		case string:
			valStr = strings.ReplaceAll(val, "'", "")
		default:
			valStr = fmt.Sprintf("%v", val)
		}
		values = append(values, valStr)
	}

	//payload to update table
	update := map[string]interface{}{
		"table":     "uneix_user",
		"columns":   strings.Join(columns, ", "),
		"value":     strings.Join(values, ", "),
		"condition": fmt.Sprintf("user_id = %d", payload.UserID),
	}
	updateJSON, _ := json.Marshal(update)
	resp := putReq(updateJSON, apiKey, client, w)
	if resp == nil {
		fmt.Println("putReq devolvió nil")
		http.Error(w, "Failed to apply profile changes", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error del servidor. Status: %d\nRespuesta: %s\n", resp.StatusCode, string(body))
		http.Error(w, "Failed to apply profile changes", http.StatusInternalServerError)
		return
	}

	//Mark column profile_applied to 1 on notification_user to block the possibility of reapplying changes
	update = map[string]interface{}{
		"table":     "notification_user",
		"columns":   "profile_applied",
		"value":     "1",
		"condition": fmt.Sprintf("notification_id = %d", payload.NotificationID),
	}
	updateJSON, _ = json.Marshal(update)
	putReq(updateJSON, apiKey, client, w)
	w.WriteHeader(http.StatusOK)
}

// Delete notification_user and notification (if no user is receiving it )
func handleDeleteNotification(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	//notification_id
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	userID, _, _ := getUserInfo(apiKey, client, w, r)
	//payload to delete the notification from user's view
	delete := map[string]interface{}{
		"table":     "notification_user",
		"condition": fmt.Sprintf("notification_id = %d and user_id = %d", req.ID, userID),
	}
	deleteJSON, _ := json.Marshal(delete)
	resp := deleteReq(deleteJSON, apiKey, client, w)
	if resp == nil {
		createLog("DB delete failed in handleDeleteNotification", 1, apiKey, nil, w)
		http.Error(w, "Failed to delete notification_user", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error deleting notification_user. Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		http.Error(w, "Failed to delete notification_user", http.StatusInternalServerError)
		return
	}

	//Check if there are notification_user left for the deleted notification_id (another user can visualize it)
	query := map[string]interface{}{
		"table":     "notification_user",
		"columns":   "COUNT(*) AS total",
		"condition": fmt.Sprintf("notification_id = %d", req.ID),
	}
	jsonQuery, _ := json.Marshal(query)
	resp = getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog("DB check returned nil in handleDeleteNotification", 1, apiKey, nil, w)
		return
	}
	defer resp.Body.Close()
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		createLog(fmt.Sprintf("JSON decode error in handleDeleteNotification: %v", err), 1, apiKey, nil, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	remaining, _ := result[0]["total"].(float64)
	//if no other user can see it, delete notification
	if remaining == 0 {
		delete := map[string]interface{}{
			"table":     "notifications",
			"condition": fmt.Sprintf("id = %d", req.ID),
		}
		deleteJSON, _ := json.Marshal(delete)
		resp := deleteReq(deleteJSON, apiKey, client, w)
		if resp == nil {
			createLog("Final delete failed in handleDeleteNotification", 1, apiKey, nil, w)
			http.Error(w, "Failed to delete notification", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		createLog(fmt.Sprintf("Notification %d deleted (no remaining users)", req.ID), 0, apiKey, nil, w)
	}
	w.WriteHeader(http.StatusOK)
}

// Checks if user is introducing correct actual password, and updates to the new one
func handleChangePassword(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	// New and old passwords are sent hashed
	type reqData struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	var req reqData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog("Invalid JSON payload in handleChangePassword", 1, apiKey, nil, w)
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	userID, username, _ := getUserInfo(apiKey, client, w, r)
	if username == "" {
		createLog("Unauthorized password change attempt", 1, apiKey, nil, w)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	//payload to get actual password
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "password",
		"condition": fmt.Sprintf("username = '%s' AND people_id = %d", username, userID),
	}
	jsonData, _ := json.Marshal(query)
	results := getReq(jsonData, apiKey, client, w)
	if results == nil {
		createLog("DB read failed in handleChangePassword", 1, apiKey, nil, w)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer results.Body.Close()
	var rows []map[string]string
	if err := json.NewDecoder(results.Body).Decode(&rows); err != nil || len(rows) == 0 {
		createLog("User not found or JSON decode failed in handleChangePassword", 1, apiKey, nil, w)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	currentHash := rows[0]["password"]

	//Check actual password with introduced password
	if currentHash != req.OldPassword {
		http.Error(w, "Incorrect current password", http.StatusForbidden)
		return
	}

	//payload to update password
	query = map[string]interface{}{
		"table":     "users",
		"columns":   "password",
		"value":     req.NewPassword,
		"condition": fmt.Sprintf("username = '%s' AND people_id = %d", username, userID),
	}
	jsonData, _ = json.Marshal(query)
	results = putReq(jsonData, apiKey, client, w)
	if results == nil {
		createLog("DB update failed in handleChangePassword", 1, apiKey, nil, w)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	createLog(fmt.Sprintf("Password successfully updated for user %s (ID %d)", username, userID), 0, apiKey, nil, w)

	defer results.Body.Close()
}

// DEPRECATED - Modifying profile pictures is done in module 4
// changes profile picture
func handleUploadPhoto(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	//checks max image weight
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	//reads photo
	file, handler, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	if len(fileBytes) < 512 {
		http.Error(w, "File too small or corrupt", http.StatusBadRequest)
		return
	}
	filetype := http.DetectContentType(fileBytes[:512])
	if !strings.HasPrefix(filetype, "image/") {
		http.Error(w, "Only image files are allowed", http.StatusUnsupportedMediaType)
		return
	}

	//checks square shape
	img, _, err := image.Decode(bytes.NewReader(fileBytes))
	if err != nil {
		http.Error(w, "Invalid image format", http.StatusUnsupportedMediaType)
		return
	}
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	if abs(width-height) > 30 {
		http.Error(w, "Image must be square or cropped first", http.StatusBadRequest)
		return
	}

	//makes a new directory or access to an existing one
	//xxxx_username where xxxx is userID filled with 0s
	paddedID := fmt.Sprintf("%04d", userID)
	basePath := "Uploads/00_Users"
	folderName := fmt.Sprintf("%s/%s_%s", basePath, paddedID, username)
	if err := os.MkdirAll(folderName, os.ModePerm); err != nil {
		log.Printf("Error creating folder: %v", err)
		http.Error(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	//deletes previous profile pictures (in case they have a different extension)
	existingFiles, err := filepath.Glob(filepath.Join(folderName, "profile_picture.*"))
	if err != nil {
		log.Printf("Error searching existing profile pictures: %v", err)
		http.Error(w, "Failed to clean old photos", http.StatusInternalServerError)
		return
	}
	for _, existing := range existingFiles {
		if err := os.Remove(existing); err != nil {
			log.Printf("Error deleting old profile picture: %v", err)
		}
	}

	//save new picture
	ext := filepath.Ext(handler.Filename)
	finalPath := filepath.Join(folderName, "profile_picture"+ext)
	if err := os.WriteFile(finalPath, fileBytes, 0644); err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
		return
	}

	//save route to user table
	relativePath := filepath.ToSlash(finalPath)
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "picture",
		"value":     relativePath,
		"condition": fmt.Sprintf("id = %d", userID),
	}
	jsonData, _ := json.Marshal(query)
	results := putReq(jsonData, apiKey, client, w)
	if results == nil {
		log.Println("putReq devolvió nil (fallo de red o DB)")
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer results.Body.Close()

	w.WriteHeader(http.StatusOK)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
