package module7workers

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Creates a new distribution list
func handleCreateNewList(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Conditions  map[string]interface{} `json:"conditions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Filtrar condiciones vacías
	filtered := make(map[string]interface{})
	for k, v := range req.Conditions {
		switch val := v.(type) {
		case string:
			if val != "" {
				filtered[k] = val
			}
		case []interface{}:
			if len(val) > 0 {
				filtered[k] = val
			}
		default:
			// si algún campo es otro tipo
			if v != nil {
				filtered[k] = v
			}
		}
	}

	userID, username, _ := getUserInfo(apiKey, client, w, r)

	condsJSON, err := json.Marshal(filtered)
	if err != nil {
		createLog(fmt.Sprintf("Error encoding conditions by user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error encoding conditions", http.StatusInternalServerError)
		return
	}

	query := map[string]interface{}{
		"table":   "distribution_lists",
		"columns": "name, created_by, conditions, description",
		"value":   fmt.Sprintf("%s, %d, %s, %s", escapeComma(req.Name), userID, escapeComma(string(condsJSON)), escapeComma(req.Description)),
	}

	postJSON, _ := json.Marshal(query)
	resp := postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("createLists: postReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDuplicateList(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		createLog(fmt.Sprintf("duplicateList: missing id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Missing list ID", http.StatusBadRequest)
		return
	}

	// Parsear ID
	id, err := strconv.Atoi(idStr)
	if err != nil {
		createLog(fmt.Sprintf("duplicateList: invalid id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	duplicateQuery := map[string]interface{}{
		"table":     "distribution_lists",
		"columns":   "*",
		"condition": fmt.Sprintf("id = '%d'", id),
	}

	getJSON, _ := json.Marshal(duplicateQuery)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("duplicateLists: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		http.Error(w, "Error decoding DB response", http.StatusInternalServerError)
		return
	}
	if len(results) == 0 {
		createLog(fmt.Sprintf("duplicateList: record not found for user %s", username), 2, apiKey, client, w)
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}
	original := results[0]

	newName := fmt.Sprintf("%s (copy)", original["name"])

	insertQuery := map[string]interface{}{
		"table":   "distribution_lists",
		"columns": "name, created_by, conditions, description",
		"value": fmt.Sprintf("%s, %d, %s, %s",
			newName,
			userID,
			original["conditions"],
			original["description"],
		),
	}

	jsonInsert, _ := json.Marshal(insertQuery)
	insertResp := postReq(jsonInsert, apiKey, client, w)

	if insertResp == nil {
		createLog(fmt.Sprintf("duplicateLists: postReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Write([]byte(`{"status":"ok"}`))
}

// Updates an existing distribution list
func handleUpdateList(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	userID, username, _ := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")

	// Obtener ID de la URL
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		createLog(fmt.Sprintf("updateList: missing id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Missing list ID", http.StatusBadRequest)
		return
	}

	// Parsear ID
	id, err := strconv.Atoi(idStr)
	if err != nil {
		createLog(fmt.Sprintf("updateList: invalid id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	// Estructura esperada
	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Conditions  map[string]interface{} `json:"conditions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("updateList: invalid JSON body by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Filtrar condiciones vacías
	cleaned := map[string]interface{}{}
	for k, v := range req.Conditions {
		switch val := v.(type) {
		case string:
			if val != "" && val != "All" {
				cleaned[k] = val
			}
		case []interface{}:
			if len(val) > 0 {
				cleaned[k] = val
			}
		}
	}

	// Convertir a JSON
	condsJSON, err := json.Marshal(cleaned)
	if err != nil {
		createLog(fmt.Sprintf("Error encoding conditions by user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error encoding conditions", http.StatusInternalServerError)
		return
	}

	// Construir query de actualización
	query := map[string]interface{}{
		"table":     "distribution_lists",
		"columns":   "name, created_by, conditions, description",
		"value":     fmt.Sprintf("%s, %d, %s, %s", escapeComma(req.Name), userID, escapeComma(string(condsJSON)), escapeComma(req.Description)),
		"condition": fmt.Sprintf("id = '%d'", id),
	}

	postJSON, _ := json.Marshal(query)
	resp := putReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("updateList: putReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Write([]byte(`{"status":"ok"}`))
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

// creates descriptor shown in frontend corresponding to the conditions of a distribution list
func describeConditions(cond map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) string {
	var parts []string

	for key, val := range cond {
		//fmt.Printf("Describing condition %s: %v\n", key, val) // Debug log
		switch key {
		case "current_workers":
			if s, ok := val.(string); ok {
				if s == "Si" {
					parts = append(parts, "Active people.")
				} else if s == "No" {
					parts = append(parts, "No longer working for or related to the center.")
				}
			}
		case "gender":
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				switch arr[0] {
				case "Dona":
					parts = append(parts, "Only women.")
				case "Home":
					parts = append(parts, "Only men.")
				}
			}
		case "contract_types":
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				for _, v := range arr {
					s := fmt.Sprint(v)
					switch s {
					case "1":
						parts = append(parts, "Contracted workers.")
					case "2":
						parts = append(parts, "Affiliated people.")
					case "3":
						parts = append(parts, "Visitors.")
					case "4":
						parts = append(parts, "People with an internship, TFG or TFM.")
					default:
						parts = append(parts, fmt.Sprintf("Contract type %s.", s))
					}
				}
			}

		case "category":
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				var categories []string
				for _, v := range arr {
					categories = append(categories, fmt.Sprint(v))
				}

				joined := strings.Join(categories, ", ")
				parts = append(parts, fmt.Sprintf("Pertaining to %s.", joined))
			}
		case "institution":
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				var institutions []string
				for _, v := range arr {
					institutions = append(institutions, fmt.Sprint(v))
				}

				joined := strings.Join(institutions, ", ")
				parts = append(parts, fmt.Sprintf("Contracted by %s.", joined))
			}
		case "groups":
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				var groups []string
				for _, v := range arr {
					groups = append(groups, fmt.Sprint(v))
				}

				joined := strings.Join(groups, ", ")
				parts = append(parts, fmt.Sprintf("Pertaining to %s.", joined))
			}
		case "funding":
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				var fundings []string
				for _, v := range arr {
					//fmt.Printf("Fetching funding name for ID %v\n", v) // Debug log
					queryName := map[string]interface{}{
						"table":     "fundings",
						"columns":   "name",
						"condition": fmt.Sprintf("code = '%v'", v),
					}
					js, _ := json.Marshal(queryName)
					//fmt.Printf("Fundings query: %s\n", string(js)) // Debug log
					resp := getReq(js, apiKey, client, w)
					if resp != nil {
						defer resp.Body.Close()
						var rows []map[string]interface{}
						if err := json.NewDecoder(resp.Body).Decode(&rows); err == nil && len(rows) > 0 {
							fundings = append(fundings, rows[0]["name"].(string))
						}
					}
				}

				joined := strings.Join(fundings, ", ")
				parts = append(parts, fmt.Sprintf("Funded by %s.", joined))
			}
		}

	}

	if len(parts) == 0 {
		return "No defined condition."
	}

	// Unir en una frase legible
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " " + parts[i]
	}
	return strings.TrimSuffix(result, " ")
}

func handleGetDistributionLists(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "distribution_lists",
		"columns": "id, name, created_by, conditions, description",
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getLists: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getLists: error reading body for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading backend response", http.StatusInternalServerError)
		return
	}

	var lists []map[string]interface{}
	if err := json.Unmarshal(body, &lists); err != nil {
		createLog(fmt.Sprintf("getLists: invalid JSON from backend for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid backend response", http.StatusInternalServerError)
		return
	}

	for _, l := range lists {
		if nameStr, ok := l["name"].(string); ok && nameStr != "" {
			l["name"] = unescapeComma(nameStr)
		}
		if descStr, ok := l["description"].(string); ok && descStr != "" {
			l["description"] = unescapeComma(descStr)
		}
		if condStr, ok := l["conditions"].(string); ok && condStr != "" {
			condStr = unescapeComma(condStr)
			var cond map[string]interface{}
			if err := json.Unmarshal([]byte(condStr), &cond); err == nil {
				l["conditions"] = describeConditions(cond, apiKey, client, w)
			}
		}
		l["userID"] = l["created_by"]
		switch v := l["created_by"].(type) {
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				l["created_by"] = getPeopleName(id, apiKey, client, w)
			}
		case float64:
			l["created_by"] = getPeopleName(int(v), apiKey, client, w)
		case int:
			l["created_by"] = getPeopleName(v, apiKey, client, w)
		}
	}

	response, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"data":   lists,
	})
	w.Write(response)
}

func handleGetList(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		createLog(fmt.Sprintf("getList: missing id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "distribution_lists",
		"columns":   "id, name, created_by, conditions, description",
		"condition": fmt.Sprintf("id = '%s'", idStr),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getList: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		createLog(fmt.Sprintf("getList: empty response body for user %s", username), 2, apiKey, nil, w)
		http.Error(w, "List not found", http.StatusNotFound)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) == 0 {
		createLog(fmt.Sprintf("getList: invalid JSON from backend for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid backend response", http.StatusInternalServerError)
		return
	}

	l := rows[0]
	if nameStr, ok := l["name"].(string); ok && nameStr != "" {
		l["name"] = unescapeComma(nameStr)
	}
	if descStr, ok := l["description"].(string); ok && descStr != "" {
		l["description"] = unescapeComma(descStr)
	}
	if condStr, ok := l["conditions"].(string); ok {
		condStr = unescapeComma(condStr)
		var cond map[string]interface{}
		if err := json.Unmarshal([]byte(condStr), &cond); err == nil {
			l["conditions"] = cond
		} else {
			createLog(fmt.Sprintf("getList: failed to unmarshal conditions for user %s: %v", username, err), 1, apiKey, client, w)
		}
	}

	response, _ := json.Marshal(l)
	w.Write(response)
}

func handleExportList(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)

	id := r.URL.Query().Get("id")
	if id == "" {
		createLog(fmt.Sprintf("exportList: missing id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Missing list ID", http.StatusBadRequest)
		return
	}

	// Obtener la lista de la BD
	query := map[string]interface{}{
		"table":     "distribution_lists",
		"columns":   "name, conditions",
		"condition": fmt.Sprintf("id = '%s'", id),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("exportList: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("exportList: error reading backend response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading backend response", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) == 0 {
		createLog(fmt.Sprintf("exportList: invalid JSON from backend for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "List not found", http.StatusNotFound)
		return
	}

	list := rows[0]
	listName := fmt.Sprintf("%v", list["name"])
	condStr := fmt.Sprintf("%v", list["conditions"])
	condStr = unescapeComma(condStr)

	var conditions map[string]interface{}
	if err := json.Unmarshal([]byte(condStr), &conditions); err != nil {
		createLog(fmt.Sprintf("exportList: invalid conditions format for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid conditions format", http.StatusInternalServerError)
		return
	}

	// Buscar los usuarios que cumplen las condiciones
	users := queryUsersByConditions(conditions, apiKey, client, w)

	if len(users) == 0 {

		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		writer.Write([]string{"Email"})
		writer.Write([]string{"No users found for these conditions"})
		writer.Flush()

		filename := fmt.Sprintf("distribution_%s.csv", strings.ReplaceAll(listName, " ", "_"))

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
		return
	}

	// Generar CSV en memoria
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Encabezados
	writer.Write([]string{"Name", "Email"})

	seenEmails := make(map[string]bool)

	// Filas
	countValid := 0
	for _, u := range users {
		crmEmail := strings.TrimSpace(fmt.Sprintf("%v", u["crm_email"]))
		userEmail := strings.TrimSpace(fmt.Sprintf("%v", u["user_email"]))
		workerName := unescapeComma(strings.TrimSpace(fmt.Sprintf("%v", u["worker"])))
		var chosenEmail string

		// si té email del crm, agafem aquest
		if crmEmail != "" && crmEmail != "<nil>" {
			chosenEmail = crmEmail
			// si no, comprovem si el email personal és educatiu
		} else if userEmail != "" && userEmail != "<nil>" && isAllowedUserDomain(userEmail) {
			chosenEmail = userEmail
		}

		if chosenEmail == "" {
			continue
		}

		if seenEmails[chosenEmail] {
			continue
		}
		seenEmails[chosenEmail] = true

		countValid++

		writer.Write([]string{workerName, chosenEmail})
	}
	writer.Flush()

	timestamp := time.Now().Format("2006-01-02")

	filename := fmt.Sprintf(
		"distribution_%s_%s.csv",
		strings.ReplaceAll(listName, " ", "_"),
		timestamp,
	)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
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

// queries users matching the given conditions and returns a list of user maps
func queryUsersByConditions(cond map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) []map[string]interface{} {

	var whereParts []string

	currentDate := time.Now().Format("2006-01-02")

	categoryMap := map[string]string{
		"phd":            "PhD",
		"postdoc":        "Postdoc",
		"researcher":     "Researcher",
		"researchtech":   "Research Technician",
		"openScience":    "Open Science",
		"activities":     "Activities",
		"communication":  "Communication",
		"management":     "Management",
		"rrhh":           "RRHH",
		"administration": "Administration",
		"finances":       "Finances",
		"projects":       "Projects",
		"ktu":            "KTU",
		"it":             "IT-Maintenance",
		"secretary":      "Secretary",
	}

	institutionMap := map[string]string{
		"crm":   "0000001672",
		"uab":   "E  BARCELO02",
		"ub":    "E  BARCELO01",
		"upc":   "E  BARCELO03",
		"icrea": "0000000469",
	}

	contractTypeMap := map[string]string{
		"1":                 "Contracted worker",
		"2":                 "Affiliated",
		"3":                 "Visitor",
		"4":                 "Internship§ TFG or TFM",
		"contracted worker": "Contracted worker",
		"affiliated":        "Affiliated",
		"visitor":           "Visitor",
		"internship":        "Internship§ TFG or TFM",
	}

	// --- gender (tabla people) ---
	if val, ok := cond["gender"].([]interface{}); ok && len(val) > 0 {
		var mapped []string
		for _, g := range val {
			switch strings.ToLower(fmt.Sprintf("%v", g)) {
			case "home":
				mapped = append(mapped, "'H'")
			case "dona":
				mapped = append(mapped, "'D'")
			default:
				continue
			}
		}
		if len(mapped) > 0 {
			whereParts = append(whereParts, fmt.Sprintf("p.gender IN (%s)", strings.Join(mapped, ",")))
		}
	}
	// --- institution (tabla contract) ---
	icreaSelected := false

	if val, ok := cond["institution"].([]interface{}); ok && len(val) > 0 {
		var mapped []string
		for _, inst := range val {
			key := strings.ToLower(fmt.Sprintf("%v", inst))
			if key == "icrea" {
				icreaSelected = true
			}
			if v, ok := institutionMap[key]; ok {
				mapped = append(mapped, fmt.Sprintf("'%s'", v))
			}
		}
		if len(mapped) > 0 {
			whereParts = append(whereParts, fmt.Sprintf("c.contracting_institution IN (%s)", strings.Join(mapped, ",")))
		}
	}

	// --- funding (tabla contract) ---

	if val, ok := cond["funding"].([]interface{}); ok && len(val) > 0 {
		fmt.Printf("Processing funding conditions: %v\n", val) // Debug log
		var mapped []string
		for _, fund := range val {
			fmt.Printf("contains funding", fund)

			mapped = append(mapped, fmt.Sprintf("'%s'", fund))

		}
		if len(mapped) > 0 {
			whereParts = append(whereParts, fmt.Sprintf("c.funding IN (%s)", strings.Join(mapped, ",")))
		}
	}

	// --- category (tabla contract) ---
	if val, ok := cond["category"].([]interface{}); ok && len(val) > 0 {
		var mapped []string
		for _, c := range val {
			key := strings.ToLower(fmt.Sprintf("%v", c))
			if v, ok := categoryMap[key]; ok {
				mapped = append(mapped, fmt.Sprintf("'%s'", v))
			}
		}

		if len(mapped) > 0 {
			if icreaSelected {
				fmt.Printf("ICREA selected", mapped)
				// Si ICREA está en instituciones, no se excluye por no tener job_category
				whereParts = append(whereParts, fmt.Sprintf(
					"((c.contracting_institution = '%s' AND c.job_category IN (%s)) OR (c.contracting_institution = '%s'))",
					institutionMap["crm"], strings.Join(mapped, ","), institutionMap["icrea"],
				))
			} else {
				whereParts = append(whereParts, fmt.Sprintf("c.job_category IN (%s)", strings.Join(mapped, ",")))
			}
		}
	}

	// --- contract_types (tabla contract) ---
	if val, ok := cond["contract_types"].([]interface{}); ok && len(val) > 0 {
		var mapped []string
		requestedContractedWorker := false

		for _, ct := range val {
			key := strings.ToLower(fmt.Sprintf("%v", ct))
			if v, ok := contractTypeMap[key]; ok {
				mapped = append(mapped, fmt.Sprintf("'%s'", v))
				if strings.EqualFold(v, "Contracted worker") {
					requestedContractedWorker = true
				}
			}
		}

		if len(mapped) > 0 {
			base := fmt.Sprintf("c.vinculation_type IN (%s)", strings.Join(mapped, ","))

			// Si se ha pedido "Contracted worker", permitimos también ICREA
			// a contarlos como contracted worker.
			if requestedContractedWorker {
				icreaAsContracted := "(c.contracting_institution = '0000000469')"
				whereParts = append(whereParts, fmt.Sprintf("(%s OR %s)", base, icreaAsContracted))
			} else {
				whereParts = append(whereParts, base)
			}
		}
	}

	// --- current_workers ---
	if val, ok := cond["current_workers"].(string); ok {
		switch strings.ToLower(val) {
		case "si":
			whereParts = append(whereParts, "p.active = 1")
		case "no":
			whereParts = append(whereParts, "p.active = 0")
		}
	}

	// --- groups (tabla people_group) ---
	if val, ok := cond["groups"].([]interface{}); ok && len(val) > 0 {
		var groupCodes []string
		for _, g := range val {
			groupCodes = append(groupCodes, fmt.Sprintf("'%v'", g))
		}
		if len(groupCodes) > 0 {
			whereParts = append(whereParts, fmt.Sprintf(`
				g.group_intern_code IN (%s)
				AND (
					g.start_date IS NULL OR g.start_date = '' OR g.start_date <= '%s'
				)
				AND (
					g.end_date IS NULL OR g.end_date = '' OR g.end_date >= '%s'
				)
			`, strings.Join(groupCodes, ","), currentDate, currentDate))
		}
	}

	// Si no hay condiciones -> no filtrar nada
	whereClause := strings.Join(whereParts, " AND ")
	fmt.Printf("WHERE clause: %s\n", whereClause)

	contractSubquery := `
	SELECT c1.*
	FROM contract c1
	WHERE c1.start_date = (
		SELECT MAX(c2.start_date)
		FROM contract c2
		WHERE c2.people_id = c1.people_id
	)`

	// --- Construir query completa ---
	getQuery := map[string]interface{}{
		"table": fmt.Sprintf(`
		people p
		LEFT JOIN (%s) c ON p.id = c.people_id
		LEFT JOIN people_group g ON p.id = g.people_id
	`, contractSubquery),
		"columns":   "p.id, CONCAT(p.name, ' ', p.surname) AS worker, p.crm_email, p.user_email",
		"condition": whereClause,
	}

	body, _ := json.Marshal(getQuery)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog("queryUsersByConditions: getReq returned nil", 1, apiKey, client, w)
		return nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog("queryUsersByConditions: error reading body", 1, apiKey, client, w)
		return nil
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(respBody, &users); err != nil {
		createLog("queryUsersByConditions: invalid JSON", 1, apiKey, client, w)
		return nil
	}

	return users
}

func handleGetGroups(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":     "researchGroup",
		"columns":   "intern_code, name",
		"condition": "outdated = 0",
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getGroups: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Error fetching groups", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getGroups: error reading response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		createLog(fmt.Sprintf("getGroups: invalid JSON from DB for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}

	// Formatear respuesta
	var result []map[string]string
	for _, r := range rows {
		code := fmt.Sprintf("%v", r["intern_code"])
		name := fmt.Sprintf("%v", r["name"])
		if code != "" && code != "<nil>" {
			result = append(result, map[string]string{
				"code": code,
				"name": name,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   result,
	})
}

func handleGetFundings(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "fundings",
		"columns": "code, name",
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getFundings: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Error fetching fundings", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getFundings: error reading response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		createLog(fmt.Sprintf("getFundings: invalid JSON from DB for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}

	// Formatear respuesta
	var result []map[string]string
	for _, r := range rows {
		code := fmt.Sprintf("%v", r["code"])
		name := fmt.Sprintf("%v", r["name"])
		if code != "" && code != "<nil>" {
			result = append(result, map[string]string{
				"code": code,
				"name": unescapeComma(name),
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   result,
	})
}

// Deletes an existing distribution list
func handleDeleteList(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")

	// Obtener ID de la URL
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		createLog(fmt.Sprintf("deleteList: missing id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Missing list ID", http.StatusBadRequest)
		return
	}

	// Parsear ID
	id, err := strconv.Atoi(idStr)
	if err != nil {
		createLog(fmt.Sprintf("deleteList: invalid id parameter by user %s", username), 2, apiKey, nil, w)
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	// Construir query de actualización
	query := map[string]interface{}{
		"table":     "distribution_lists",
		"condition": fmt.Sprintf("id = '%d'", id),
	}

	delJSON, _ := json.Marshal(query)
	resp := deleteReq(delJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("deleteList: deleteReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	createLog(fmt.Sprintf("List succesfully deleted by %s", username), 0, apiKey, client, w)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleSendNotification(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	// Obtener info del creador
	creatorID, creatorUsername, _ := getUserInfo(apiKey, client, w, r)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		createLog(fmt.Sprintf("handleSendNotification: error parsing form by user %s: %v", creatorUsername, err), 1, apiKey, client, w)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	listID := r.FormValue("distributionListId")

	if strings.TrimSpace(title) == "" {
		createLog(fmt.Sprintf("handleSendNotification: missing title by user %s", creatorUsername), 2, apiKey, nil, w)
		http.Error(w, "Title required", http.StatusBadRequest)
		return
	}

	// Cargar conditions JSON de la lista
	query := map[string]interface{}{
		"table":     "distribution_lists",
		"columns":   "conditions",
		"condition": fmt.Sprintf("id = '%s'", listID),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("handleSendNotification: getReq returned nil for user %s", creatorUsername), 1, apiKey, client, w)
		http.Error(w, "Failed to load distribution list", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var row []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&row); err != nil || len(row) == 0 {
		createLog(fmt.Sprintf("handleSendNotification: invalid JSON from backend for user %s: %v", creatorUsername, err), 1, apiKey, client, w)
		http.Error(w, "Invalid list", http.StatusInternalServerError)
		return
	}

	condStr := fmt.Sprintf("%v", row[0]["conditions"])
	condStr = unescapeComma(condStr)

	var conditions map[string]interface{}
	json.Unmarshal([]byte(condStr), &conditions)

	// Obtener usuarios que cumplen condiciones
	users := queryUsersByConditions(conditions, apiKey, client, w)
	if len(users) == 0 {
		createLog(fmt.Sprintf("handleSendNotification: no users found for list %s by user %s", listID, creatorUsername), 2, apiKey, nil, w)
		http.Error(w, "No users found for this list", http.StatusNotFound)
		return
	}

	// Extraer people_id
	var targetUserIDs []int
	for _, u := range users {
		if pid, ok := u["id"].(float64); ok {
			targetUserIDs = append(targetUserIDs, int(pid))
		}
	}

	if len(targetUserIDs) == 0 {
		createLog(fmt.Sprintf("handleSendNotification: no target people IDs extracted for list %s by user %s", listID, creatorUsername), 2, apiKey, nil, w)
		http.Error(w, "No target people IDs extracted", http.StatusInternalServerError)
		return
	}

	// --- File upload (igual que original)
	var filePath string
	file, handler, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		paddedID := fmt.Sprintf("%04d", creatorID)
		basePath := "../../Uploads/00_Users"
		dir := path.Join(basePath, fmt.Sprintf("%s_%s", paddedID, creatorUsername))
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			createLog(fmt.Sprintf("handleSendNotification: could not create directory %s for user %s: %v", dir, creatorUsername, err), 1, apiKey, client, w)
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

		dst, err := os.Create(filePath)
		if err != nil {
			createLog(fmt.Sprintf("handleSendNotification: could not create file %s for user %s: %v", filePath, creatorUsername, err), 1, apiKey, client, w)
			http.Error(w, "Could not save file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		io.Copy(dst, file)
	}

	// --- Crear notificación principal
	encodedContent := b64.StdEncoding.EncodeToString([]byte(content))
	now := time.Now().Format("2006-01-02 15:04")

	insert := map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, file_path, user_id, created_at",
		"value":   fmt.Sprintf("distributionList-notif, %s, %s, %s, %d, %s", title, encodedContent, filePath, creatorID, now),
	}

	notifJSON, _ := json.Marshal(insert)
	resp2 := postReq(notifJSON, apiKey, client, w)
	if resp2 == nil || resp2.StatusCode >= 400 {
		createLog(fmt.Sprintf("handleSendNotification: failed to create notification for user %s", creatorUsername), 1, apiKey, client, w)
		http.Error(w, "Failed to create notification", http.StatusInternalServerError)
		return
	}
	defer resp2.Body.Close()

	// Obtener ID recién creada
	getID := map[string]interface{}{
		"table":     "notifications",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = '%d' AND created_at = '%s'", creatorID, now),
	}
	idJSON, _ := json.Marshal(getID)
	resp3 := getReq(idJSON, apiKey, client, w)
	defer resp3.Body.Close()

	var rows2 []map[string]interface{}
	json.NewDecoder(resp3.Body).Decode(&rows2)
	notifID := int(rows2[0]["id"].(float64))

	// Insertar en notification_user
	for _, uid := range targetUserIDs {
		link := map[string]interface{}{
			"table":   "notification_user",
			"columns": "notification_id, user_id, can_read, can_download, seen",
			"value":   fmt.Sprintf("%d, %d, 1, 0, 0", notifID, uid),
		}
		linkJSON, _ := json.Marshal(link)
		postReq(linkJSON, apiKey, client, w)
	}

	w.WriteHeader(http.StatusOK)
}

func handleCheckUserInformation(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, _, userRole := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"role":   userRole,
		"userID": userID,
	})
}
