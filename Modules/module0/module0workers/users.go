package module0workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// handle users api get req
func handleGetUsers(w http.ResponseWriter) {
	payload := map[string]interface{}{
		"method":  "GET",
		"table":   "users INNER JOIN people ON users.people_id = people.id",
		"columns": "users.people_id, users.username, users.role, people.name, people.surname",
	}
	sendToBackend(w, payload)
}

func handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// validate required fields
	required := []string{"people_id", "role"}
	for _, key := range required {
		val, ok := data[key]
		if !ok || strings.TrimSpace(fmt.Sprint(val)) == "" {
			http.Error(w, "Missing required field: "+key, http.StatusBadRequest)
			return
		}
	}

	peopleID := fmt.Sprint(data["people_id"])
	role := fmt.Sprint(data["role"])

	// construct UPDATE payload
	update := map[string]interface{}{
		"method":    "PUT",
		"table":     "users",
		"columns":   "role",
		"value":     role,
		"condition": "people_id = " + peopleID,
	}

	sendToBackend(w, update)
}

func handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	println("llega2")
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// validate required fields
	required := []string{"people_id", "newPassword"}
	for _, key := range required {
		val, ok := data[key]
		if !ok || strings.TrimSpace(fmt.Sprint(val)) == "" {
			http.Error(w, "Missing required field: "+key, http.StatusBadRequest)
			return
		}
	}

	peopleID := fmt.Sprint(data["people_id"])
	password := fmt.Sprint(data["newPassword"])

	println("people id", peopleID)
	println("password", password)

	// construct UPDATE payload
	update := map[string]interface{}{
		"method":    "PUT",
		"table":     "users",
		"columns":   "password",
		"value":     password,
		"condition": "people_id = " + peopleID,
	}

	sendToBackend(w, update)
}

// GET /api/available-people
func handleGetAvailablePeople(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	query := map[string]interface{}{
		"table":     "people",
		"columns":   "id, name, surname, secondSurname",
		"condition": "id NOT IN (SELECT people_id FROM users WHERE people_id IS NOT NULL)",
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)
	json.NewEncoder(w).Encode(users)
}

func handleGenerateUsername(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.URL.Query().Get("name"))
	surname := strings.ToLower(r.URL.Query().Get("surname"))

	base := sanitizeUsername(string(name[0]) + surname)

	final := base
	counter := 1

	// Check existing usernames and iterate
	for usernameExists(final, apiKey, client, w) {
		counter++
		final = fmt.Sprintf("%s%d", base, counter)
	}

	password := fmt.Sprintf("Crm2026@%s", final)

	json.NewEncoder(w).Encode(map[string]string{
		"username": final,
		"password": password,
	})
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

// handle users api post req
func handlePostUser(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	fmt.Println("Received data:", data)
	// validate req fields
	required := []string{"people_id", "username", "role", "password"}
	for _, key := range required {
		if _, ok := data[key]; !ok {
			http.Error(w, "Missing field: "+key, http.StatusBadRequest)
			return
		}
	}

	peopleID, ok := data["people_id"].(float64)
	if !ok {
		http.Error(w, "Invalid people_id", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(data["username"].(string))
	role := strings.TrimSpace(data["role"].(string))
	password := strings.TrimSpace(data["password"].(string))

	if username == "" || role == "" || password == "" {
		http.Error(w, "Empty input fields", http.StatusBadRequest)
		return
	}

	// hash passwd
	passRaw := data["password"].(string)
	data["password"] = hashString(passRaw)

	// check if exists
	checkPayload := map[string]interface{}{
		"method":    "GET",
		"table":     "users",
		"columns":   "people_id",
		"condition": "username='" + data["username"].(string) + "'", // sanitize in here
	}
	if userExists(checkPayload) {
		http.Error(w, "duplicated user", http.StatusConflict)
		return
	}

	value := fmt.Sprintf("%d,%s,%s,%s",
		int(peopleID),
		data["username"],
		data["role"],
		data["password"],
	)

	// create payload
	payload := map[string]interface{}{
		"table":   "users",
		"columns": "people_id, username,role,password",
		"value":   value,
	}
	body, _ = json.Marshal(payload)
	resp := postReq(body, apiKey, client, w)
	if resp == nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

}

// delete user from db
func handleDeleteUser(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body) // sanitize here
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// ensure "id" is present and valid
	idRaw, ok := data["people_id"]
	if !ok {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	var idStr string
	switch id := idRaw.(type) {
	case float64:
		idStr = fmt.Sprintf("%d", int(id)) // from JSON, numbers come as float64
	case string:
		idStr = id
	default:
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	// construct payload
	payload := map[string]interface{}{
		"method":    "DELETE",
		"table":     "users",
		"condition": "people_id=" + idStr,
	}

	sendToBackend(w, payload)
}

// check if user exists
func userExists(payload map[string]interface{}) bool {
	jsonPayload, _ := json.Marshal(payload)
	client := &http.Client{Transport: setInsecureRequest()}

	// set auth header
	req, _ := http.NewRequest(http.MethodPost, "https://localhost:"+cfg.BackendPort+"/module/api/db", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return len(result) > 0
}

func getId(payload map[string]interface{}) []map[string]interface{} {
	jsonPayload, _ := json.Marshal(payload)
	client := &http.Client{Transport: setInsecureRequest()}

	// set auth header
	req, _ := http.NewRequest(http.MethodPost, "https://localhost:"+cfg.BackendPort+"/module/api/db", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		println("error getting user id")
	}

	defer resp.Body.Close()

	var result []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	return result
}

/*
// forward to controller backend
func postUneix(payload map[string]interface{}) {
	jsonPayload, _ := json.Marshal(payload)
	log.Println("Posting to uneix_user:", string(jsonPayload))
	client := &http.Client{Transport: setInsecureRequest()}

	req, _ := http.NewRequest(http.MethodPost, "https://localhost:"+cfg.BackendPort+"/module/api/db", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Error contacting backend: %v - %s", err, string(bodyBytes))
		return
	}
	defer resp.Body.Close()
}

*/
