package module4workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// -------------------------------------------- MSSQL -------------------------------------------------

func sendReqMSSQL(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) *http.Response {
	url := "https://localhost:" + mC.ServerPort + "/module/api/dbmssql"

	reqInsert, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("❌ Error creando request MSSQL:", err)
		return nil
	}

	reqInsert.Header.Set("Content-Type", "application/json")
	reqInsert.Header.Set("Authorization", "Bearer "+apiKey)

	// copiar cookies de sesión
	for _, c := range r.Cookies() {
		reqInsert.AddCookie(c)
	}

	respInsert, err := client.Do(reqInsert)
	if err != nil {
		fmt.Println("❌ Error en client.Do():", err)
		return nil
	}

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

// -------------------------------------------- INTRANET -------------------------------------------------

func handleAddToIntranet(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	// Obtener información del usuario
	_, username, _ := getUserInfo(apiKey, client, w, r)

	var payload struct {
		NewHireID int `json:"people_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("createModality: invalid JSON from user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	createdIntranet := time.Now().Format("2006-01-02 15:04:00")
	createdIntratools := time.Now().Format("2006-01-02 15:04")

	//fmt.Printf("New hire id", payload.NewHireID)
	query := map[string]interface{}{
		"table": `people p LEFT JOIN contract c ON p.id = c.people_id
					LEFT JOIN people_group pg ON p.id = pg.people_id
					LEFT JOIN researchGroup g ON pg.group_intern_code = g.intern_code
					LEFT JOIN room r ON c.office_location = r.id`,
		"columns": "p.name, p.surname, p.secondSurname, c.vinculation_type, c.job_category, c.start_date, c.end_date, g.name AS group_name, c.funding, c.contracting_institution, r.name AS office_name, r.uab_code, p.user_phone, p.crm_email, p.orcid",
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

	//fmt.Printf("New hire info: %+v\n", result[0])

	if result[0]["name"] == nil || result[0]["surname"] == nil || result[0]["vinculation_type"] == nil || result[0]["start_date"] == nil {
		createLog(fmt.Sprintf("Missing required fields for new hire with people_id %d", payload.NewHireID), 1, apiKey, client, w)
		http.Error(w, "Missing required fields in new hire info", http.StatusBadRequest)
		return
	}
	var position_id int
	var position_other string
	var person_classification_id int
	isAdmin := false

	//fmt.Printf("tipus de vinculacio %s", result[0]["vinculation_type"])
	//fmt.Printf("job category %s", result[0]["job_category"])

	if result[0]["vinculation_type"] == "Affiliated" {
		position_id = 1 // Faculty
	} else {
		if result[0]["job_category"] == "PhD" {
			position_id = 2              // PhD Student
			person_classification_id = 4 // PhD student
		} else if result[0]["job_category"] == "Postdoc" {
			position_id = 3              // Postdoctoral Fellow
			person_classification_id = 3 // Postdoc researcher
		} else if result[0]["job_category"] == "Research Technician" {
			position_id = 4 // Research Technician
		} else if result[0]["job_category"] == "KTU" {
			position_id = 7              // KTU
			person_classification_id = 6 // KTU
		} else {
			if result[0]["job_category"] != nil {
				position_other = result[0]["job_category"].(string)
				if position_other != "Researcher" {
					isAdmin = true
					position_id = -1
					person_classification_id = 5 // Management
				} else {
					person_classification_id = 2 // Researcher
				}
			} else {
				position_id = -1
				person_classification_id = -1
			}

		}
	}

	//fmt.Printf("Posició: %d,\n", position_id)
	//fmt.Printf("Position other: %s\n", position_other)
	//fmt.Printf("person classification: %d", person_classification_id)

	var group_id int
	if isAdmin {
		group_id = 9 // Management & Administration
	} else {
		if result[0]["group_name"] != nil {
			/*
				switch result[0]["group_name"].(string) {
				case "Mathematical Biology":
					group_id = 2
				case "Computational and Mathematical Neuroscience":
					group_id = 5
				case "BioGeoMap":
					group_id = 10
				case "Mathematics for the Environment and Society":
					group_id = 11
				case "Analysis":
					group_id = 12
				case "Dynamical Systems":
					group_id = 13
				case "Algebra and Algebraic Geometry":
					group_id = 14
				case "Combinatorics and Mathematics of Computer Science":
					group_id = 15
				case "Dynamics In Neuronal Networks":
					group_id = 16
				case "Collaborative Mathematic Unit (COLLMATH)":
					group_id = 17
				case "Geometry and Topology":
					group_id = 21
				case "Number Theory":
					group_id = 22
				case "Partial Differential Equations":
					group_id = 23
				}
			*/

			group_id_fetched, err := fetchGroups(result[0]["group_name"].(string), apiKey, client, w, r)
			if err != nil {
				createLog(fmt.Sprintf("Error fetching group ID for group '%s' for user %s: %v", result[0]["group_name"].(string), username, err), 1, apiKey, client, w)
				http.Error(w, "Error fetching group information", http.StatusInternalServerError)
				return
			}
			group_id = group_id_fetched
		} else {
			group_id = -1 // No group
		}
	}

	//fmt.Printf("grup id %d", group_id)

	var person_funding_id int
	if result[0]["funding"] != nil {

		/*
			switch result[0]["funding"].(string) {
			case "0000000469":
				person_funding_id = 2
			case "0000002650":
				person_funding_id = 3
			case "0000002204":
				person_funding_id = 4
			case "0000000533":
				person_funding_id = 5
			case "0000002048":
				person_funding_id = 6
			case "0000002325":
				person_funding_id = 8
			case "0000000175":
				person_funding_id = 10
			case "0000000332":
				person_funding_id = 11
			case "0000000375":
				person_funding_id = 12

			}
		*/

		person_funding_id_fetched, err := fetchFunding(result[0]["funding"].(string), apiKey, client, w, r)
		if err != nil {
			createLog(fmt.Sprintf("Error fetching funding ID for funding code '%s' for user %s: %v", result[0]["funding"].(string), username, err), 1, apiKey, client, w)
			http.Error(w, "Error fetching funding information", http.StatusInternalServerError)
			return
		}
		person_funding_id = person_funding_id_fetched

	} else {
		person_funding_id = -1 // No funding
	}

	//fmt.Printf("person funding id %d", person_funding_id)

	var person_affiliation_id int
	if result[0]["contracting_institution"] != nil {

		switch result[0]["contracting_institution"].(string) {
		case "0000001672":
			person_affiliation_id = 1
		case "0000000469":
			person_affiliation_id = 5
		case "E BARCELO02":
			person_affiliation_id = 6
		case "E BARCELO03":
			person_affiliation_id = 7
		case "E BARCELO01":
			person_affiliation_id = 8
		}
	} else {
		person_affiliation_id = -1 // No affiliation
	}

	//fmt.Printf("person affiliation id %d", person_affiliation_id)

	var office_id int
	if result[0]["office_name"] != nil && result[0]["uab_code"] != nil {

		office_id_fetched, err := fetchOffice(result[0]["office_name"].(string), result[0]["uab_code"].(string), apiKey, client, w, r)
		if err != nil {
			createLog(fmt.Sprintf("Error fetching office ID for office '%s (%s)' for user %s: %v", result[0]["office_name"].(string), result[0]["uab_code"].(string), username, err), 1, apiKey, client, w)
			http.Error(w, "Error fetching office information", http.StatusInternalServerError)
			return
		}
		office_id = office_id_fetched

	} else {
		office_id = -1 // No office
	}

	//fmt.Printf("office id %d", office_id)

	var orcid string
	if result[0]["orcid"] != nil {
		orcid = parseOrcid(result[0]["orcid"].(string))
	}

	//fmt.Printf("orcid %s", orcid)

	var email string
	if result[0]["crm_email"] != nil {
		email = result[0]["crm_email"].(string)
	}

	//fmt.Printf("email %s", email)

	var secondSurname string
	if result[0]["secondSurname"] != nil {
		secondSurname = result[0]["secondSurname"].(string)
	}

	//fmt.Printf("second surname %s", secondSurname)

	twoSurnames := result[0]["surname"].(string) + " " + secondSurname

	//fmt.Printf("two surnames %s", twoSurnames)

	var Fi_Contracte interface{} = nil

	if v := result[0]["end_date"]; v != nil {
		if s, ok := v.(string); ok && s != "" {
			Fi_Contracte = s
		}
	}
	Fi_Contracte = sqlNullableString(Fi_Contracte)

	//fmt.Printf("Fi_Contracte %v", Fi_Contracte)

	queryInsert := map[string]interface{}{
		"table": "people",
		"columns": `position_id, other_position, group_id, person_funding_id, 
		person_affiliation_id, person_classification_id, office_id, visible_on_web, name, surname, email, created, modified, orcid, Inici_Contracte, Fi_Contracte, Cognom1, Cognom2`,
		"value": fmt.Sprintf(`%d, %s, %d, %d, %d, %d, %d, 0, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s`,
			position_id, position_other, group_id, person_funding_id, person_affiliation_id, person_classification_id, office_id,
			result[0]["name"].(string), twoSurnames, email, createdIntranet, createdIntranet, orcid, result[0]["start_date"].(string), Fi_Contracte,
			result[0]["surname"].(string), secondSurname),
	}
	postJSON, _ := json.Marshal(queryInsert)
	resp = postReqMSSQL(postJSON, apiKey, client, w, r)

	if resp == nil {
		createLog(fmt.Sprintf("Error inserting new hire into intranet for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to add new hire to intranet", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("MSSQL API error %d: %s while adding new hire to intranet for user %s", resp.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error adding new hire to intranet: "+string(bodyBytes), http.StatusInternalServerError)
		return
	}

	createLog(fmt.Sprintf("Successfully added new hire with people_id %d to intranet for user %s", payload.NewHireID, username), 0, apiKey, client, w)

	queryHires := map[string]interface{}{
		"table":     "newHires_tasks",
		"columns":   "intra_req, intra_req_time",
		"value":     fmt.Sprintf(`1, %s`, createdIntratools),
		"condition": fmt.Sprintf(`people_id = %d`, payload.NewHireID),
	}
	updateJSON, _ := json.Marshal(queryHires)
	resp = putReq(updateJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error updating new hire task status for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to update new hire task status", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// get id acabado de añadir para actualizar external_id
	queryExternalID := map[string]interface{}{
		"table":     "people",
		"columns":   "id",
		"condition": fmt.Sprintf(`created = '%s' AND surname = '%s'`, createdIntranet, twoSurnames),
	}
	updateExternalJSON, _ := json.Marshal(queryExternalID)
	resp = getReqMSSQL(updateExternalJSON, apiKey, client, w, r)
	if resp == nil {
		createLog(fmt.Sprintf("Error getting new hire external_id for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to get new hire external_id", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var externalResult []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&externalResult); err != nil {
		createLog(fmt.Sprintf("Error decoding external_id response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	if len(externalResult) == 0 {
		createLog(fmt.Sprintf("No user found when fetching external_id for people_id %d", payload.NewHireID), 1, apiKey, client, w)
		http.Error(w, "No user found when fetching external_id", http.StatusBadRequest)
		return
	}
	externalID := strconv.FormatFloat(externalResult[0]["id"].(float64), 'f', 0, 64)

	// añadir external id a people
	queryUpdateExternalID := map[string]interface{}{
		"table":     "people",
		"columns":   "people_idExternal",
		"value":     externalID,
		"condition": fmt.Sprintf(`id = %d`, payload.NewHireID),
	}
	updateExternalIDJSON, _ := json.Marshal(queryUpdateExternalID)
	resp = putReq(updateExternalIDJSON, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error updating new hire external_id for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to update new hire external_id", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("MSSQL API error %d: %s while updating new hire external_id for user %s", resp.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error updating new hire external_id: "+string(bodyBytes), http.StatusInternalServerError)
		return
	}

	// enviar correu a comunicacions

	mail := "CRMComm@crm.cat"

	jobCat := ""
	supervisor := ""

	if result[0]["job_category"] != nil {
		jobCat = result[0]["job_category"].(string)
	} else {
		jobCat = result[0]["vinculation_type"].(string)
	}

	querySup := map[string]interface{}{
		"table": `people p LEFT JOIN contract c ON p.id = c.people_id
					LEFT JOIN people_supervisor ps ON c.id = ps.contract_id
					LEFT JOIN people s ON ps.supervisor_id = s.id`,
		"columns": "s.name, s.surname",
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
	jsonData, _ = json.Marshal(querySup)
	resp = getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error retrieving new hire supervisor info for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Failed to retrieve new hire supervisor info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var supResult []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&supResult); err != nil {
		createLog(fmt.Sprintf("Error decoding new hire supervisor info for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	if len(supResult) > 0 && supResult[0]["name"] != nil && supResult[0]["surname"] != nil {
		supervisor = fmt.Sprintf("%s %s", supResult[0]["name"].(string), supResult[0]["surname"].(string))
	} else {
		supervisor = "No supervisor assigned"
	}

	tmplContent, err := os.ReadFile("module4workers/assets/notifyComms.html")
	if err != nil {
		println("Error leyendo plantilla:", err.Error())
		return
	}

	tmpl, err := template.New("email").Parse(string(tmplContent))
	if err != nil {
		println("Error parseando plantilla:", err.Error())
		return
	}

	// datos que usará la plantilla
	data := map[string]string{
		"Worker":      fmt.Sprintf("%s %s", result[0]["name"].(string), twoSurnames),
		"JobCategory": jobCat,
		"Supervisor":  supervisor,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		createLog("Error ejecutando plantilla comunicación: "+err.Error(), 1, apiKey, client, w)
	}

	// enviar email
	sendEmail(mail, "A new person has been added to Intranet", buf.String())

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func fetchGroups(groupName string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) (int, error) {
	_, username, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table":     `groups`,
		"columns":   `id`,
		"condition": fmt.Sprintf(`name = '%s'`, groupName),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReqMSSQL(getJSON, apiKey, client, w, r)
	if resp == nil {
		createLog(fmt.Sprintf("fetchGroups: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchGroups: getReqMSSQL returned nil")
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchGroups: read body error for user %s: %v", username, err), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchGroups: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("fetchGroups: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchGroups: MSSQL API error %d: %s", resp.StatusCode, string(b))
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(b, &rows); err != nil {
		createLog(fmt.Sprintf("fetchGroups: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchGroups: unmarshal: %w", err)
	}
	return int(rows[0]["id"].(float64)), nil
}

func fetchFunding(fundingCode string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) (int, error) {
	_, username, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table":     `person_fundings`,
		"columns":   `id`,
		"condition": fmt.Sprintf(`CodiSubv = '%s'`, fundingCode),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReqMSSQL(getJSON, apiKey, client, w, r)
	if resp == nil {
		createLog(fmt.Sprintf("fetchFunding: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchFunding: getReqMSSQL returned nil")
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchFunding: read body error for user %s: %v", username, err), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchFunding: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("fetchFunding: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchFunding: MSSQL API error %d: %s", resp.StatusCode, string(b))
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(b, &rows); err != nil {
		createLog(fmt.Sprintf("fetchFunding: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchFunding: unmarshal: %w", err)
	}
	return int(rows[0]["id"].(float64)), nil
}
func fetchOffice(officeName string, uabCode string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) (int, error) {
	_, username, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table":     `offices`,
		"columns":   `id`,
		"condition": fmt.Sprintf(`name = '%s (%s)'`, officeName, uabCode),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReqMSSQL(getJSON, apiKey, client, w, r)
	if resp == nil {
		createLog(fmt.Sprintf("fetchOffice: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchOffice: getReqMSSQL returned nil")
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchOffice: read body error for user %s: %v", username, err), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchOffice: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("fetchOffice: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchOffice: MSSQL API error %d: %s", resp.StatusCode, string(b))
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(b, &rows); err != nil {
		createLog(fmt.Sprintf("fetchOffice: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
		return 0, fmt.Errorf("fetchOffice: unmarshal: %w", err)
	}
	return int(rows[0]["id"].(float64)), nil
}

func parseOrcid(orcid string) string {
	return fmt.Sprintf("%s-%s-%s-%s", orcid[:4], orcid[4:8], orcid[8:12], orcid[12:])
}

func parseTelephone(phone string) string {
	if phone == "+34" {
		return ""
	} else {
		return phone
	}
}

func sqlNullableString(v interface{}) string {
	if v == nil {
		return "9999-12-31"
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "9999-12-31"
	}
	s = strings.ReplaceAll(s, "'", "''")
	return fmt.Sprintf("%s", s)
}
