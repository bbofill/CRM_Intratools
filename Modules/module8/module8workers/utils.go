package module8workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const itEmail = "it@crm.cat"                    // it@crm.cat
const accountingEmail = "comptabilitat@crm.cat" // comptabilitat@crm.cat
const projectsEmail = "crmprojects@crm.cat"     // crmprojects@crm.cat
const managementEmail = "gerencia@crm.cat"      // gerencia@crm.cat
const rrhhEmail = "rrhh@crm.cat"                // rrhh@crm.cat
const contractProgramProjectID = "001000001CP"
const contractProgramLegacyProjectID = "CONTRACTE_PROGRAMA"
const contractProgramProjectName = "Contracte Programa"

// ---------------- ACCESS MANAGEMENT ----------------

func handleGetUsersAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "users u LEFT JOIN people p ON u.people_id = p.id",
		"columns": "u.username, u.access_finances, u.access_projects, u.access_gerencia, p.name, p.surname",
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
		Username       string `json:"username"`
		AccessFinances int    `json:"access_finances"`
		AccessProjects int    `json:"access_projects"`
		AccessGerencia int    `json:"access_gerencia"`
		Name           string `json:"name"`
		Surname        string `json:"surname"`
	}

	var dbUsers []dbUser
	if err := json.NewDecoder(resp.Body).Decode(&dbUsers); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en getUsersAccess para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de usuarios", http.StatusInternalServerError)
		return
	}

	users := make([]map[string]string, 0, len(dbUsers))
	projects := make([]string, 0)
	finances := make([]string, 0)
	gerencia := make([]string, 0)

	for _, u := range dbUsers {
		users = append(users, map[string]string{
			"username": u.Username,
			"name":     unescapeComma(u.Name),
			"surname":  unescapeComma(u.Surname),
		})

		if u.AccessFinances == 1 {
			finances = append(finances, u.Username)
		}
		if u.AccessProjects == 1 {
			projects = append(projects, u.Username)
		}
		if u.AccessGerencia == 1 {
			gerencia = append(gerencia, u.Username)
		}
	}

	response := map[string]interface{}{
		"users":    users,
		"projects": projects,
		"finances": finances,
		"gerencia": gerencia,
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
		Finances []string `json:"financeUsers"`
		Projects []string `json:"projectUsers"`
		Gerencia []string `json:"gerenciaUsers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		createLog(fmt.Sprintf("Error al decodificar solicitud JSON en handleSetUsersAccess para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar la solicitud", http.StatusBadRequest)
		return
	}

	//fmt.Printf("Usuarios con acceso a finanzas: %v\n", requestData.Finances)
	//fmt.Printf("Usuarios con acceso a proyectos: %v\n", requestData.Projects)
	//fmt.Printf("Usuarios con acceso de gerencia: %v\n", requestData.Gerencia)

	// Resetear accessos a tothom
	resetQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "access_finances, access_projects, access_gerencia",
		"value":     "0, 0, 0",
		"condition": "1=1",
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
	for _, u := range requestData.Finances {
		updateQuery := map[string]interface{}{
			"table":     "users",
			"columns":   "access_finances",
			"value":     "1",
			"condition": fmt.Sprintf("username = '%s'", u),
		}
		updateJson, _ := json.Marshal(updateQuery)
		resp := putReq(updateJson, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error asignando acceso a usuario %s en handleSetUsersAccess para user %s", u, userACCESSING), 1, apiKey, client, w)
		}
	}

	for _, u := range requestData.Projects {
		updateQuery := map[string]interface{}{
			"table":     "users",
			"columns":   "access_projects",
			"value":     "1",
			"condition": fmt.Sprintf("username = '%s'", u),
		}

		updateJson, _ := json.Marshal(updateQuery)
		resp := putReq(updateJson, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error asignando acceso a usuario %s en handleSetUsersAccess para user %s", u, userACCESSING), 1, apiKey, client, w)
		}
	}

	for _, u := range requestData.Gerencia {
		updateQuery := map[string]interface{}{
			"table":     "users",
			"columns":   "access_gerencia",
			"value":     "1",
			"condition": fmt.Sprintf("username = '%s'", u),
		}
		updateJson, _ := json.Marshal(updateQuery)
		resp := putReq(updateJson, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error asignando acceso de gerencia a usuario %s en handleSetUsersAccess para user %s", u, userACCESSING), 1, apiKey, client, w)
		}
	}

	response := map[string]string{
		"message": "Access permissions updated successfully",
	}
	createLog(fmt.Sprintf("Access permissions updated successfully in handleSetUsersAccess for user %s", userACCESSING), 0, apiKey, client, w)
	json.NewEncoder(w).Encode(response)
}

// ---------------- PROJECTS ----------------

func handleGetProjects(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	today := time.Now().Format("2006-01-02")
	query := map[string]interface{}{
		"table":     "projects p LEFT JOIN people_projects pp ON p.id = pp.project_id",
		"columns":   "p.short_name, p.id",
		"condition": fmt.Sprintf("pp.people_id = %d AND p.end_date >= '%s'", userID, today),
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Error interno al preparar la consulta in getProjects for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		return
	}
	//fmt.Printf("Querying projects for user %s with payload: %s\n", userACCESSING, string(jsonQuery))
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla projects para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD en getProjects. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var projects []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en getProjects para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de proyectos", http.StatusInternalServerError)
		return
	}

	//fmt.Printf("Projects: %+v\n", projects)
	for i := range projects {
		if shortName, ok := projects[i]["short_name"].(string); ok {
			projects[i]["short_name"] = unescapeComma(shortName)
		}
		if id, ok := projects[i]["id"]; ok {
			//fmt.Printf("Obteniendo IPs para proyecto %v\n", id)
			queryIP := map[string]interface{}{
				"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
				"columns":   "p.name, p.surname, p.id",
				"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL' AND pp.end_date >= '%s'", id, today),
			}

			jsonQueryIP, _ := json.Marshal(queryIP)
			respIP := getReq(jsonQueryIP, apiKey, client, w)
			if respIP == nil {
				createLog(fmt.Sprintf("No se obtuvo respuesta para IPs del proyecto %v para user %s", id, userACCESSING), 1, apiKey, client, w)
				continue
			}

			if respIP.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(respIP.Body)
				respIP.Body.Close()
				createLog(fmt.Sprintf("Error BD al obtener IPs para proyecto %v. Status: %d, Body: %s para user %s",
					id, respIP.StatusCode, string(bodyBytes), userACCESSING), 1, apiKey, client, w)
				continue
			}

			var ips []map[string]interface{}
			if err := json.NewDecoder(respIP.Body).Decode(&ips); err != nil {
				respIP.Body.Close()
				createLog(fmt.Sprintf("Error al decodificar IPs del proyecto %v para user %s: %v", id, userACCESSING, err), 1, apiKey, client, w)
				continue
			}
			respIP.Body.Close()

			outIPs := make([]map[string]string, 0, len(ips))
			for _, ipr := range ips {
				outIPs = append(outIPs, map[string]string{
					"id":      fmt.Sprintf("%.0f", ipr["id"].(float64)),
					"name":    unescapeComma(ipr["name"].(string)),
					"surname": unescapeComma(ipr["surname"].(string)),
				})
			}
			projects[i]["IPs"] = outIPs

			if len(outIPs) == 0 {
				queryGerencia := map[string]interface{}{
					"table":     "people p LEFT JOIN users u ON p.id = u.people_id",
					"columns":   "p.name, p.surname, p.id",
					"condition": "u.access_gerencia = 1",
				}
				jsonQueryGerencia, _ := json.Marshal(queryGerencia)
				respGerencia := getReq(jsonQueryGerencia, apiKey, client, w)
				if respGerencia == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para gerencia al asignar IPs para user %s", userACCESSING), 1, apiKey, client, w)
					continue
				}
				var ips []map[string]interface{}
				if err := json.NewDecoder(respGerencia.Body).Decode(&ips); err != nil {
					respGerencia.Body.Close()
					createLog(fmt.Sprintf("Error al decodificar IPs para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
					continue
				}
				respGerencia.Body.Close()

				outIPs := make([]map[string]string, 0, len(ips))
				for _, ipr := range ips {
					outIPs = append(outIPs, map[string]string{
						"id":      fmt.Sprintf("%.0f", ipr["id"].(float64)),
						"name":    unescapeComma(ipr["name"].(string)),
						"surname": unescapeComma(ipr["surname"].(string)),
					})
				}
				projects[i]["IPs"] = outIPs
			}
			//fmt.Printf("Proyecto: %v, IPs: %v\n", projects[i]["short_name"], projects[i]["IPs"])

		}

	}

	projects = appendContractProgramProjectIfEligible(projects, userID, today, apiKey, client, w)

	//fmt.Printf("Projectes: %+v\n", projects)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(projects); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en getProjects para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func handleGetAllProjects(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	today := time.Now().Format("2006-01-02")
	query := map[string]interface{}{
		"table":     "projects p",
		"columns":   "p.short_name, p.id",
		"condition": fmt.Sprintf("p.end_date >= '%s'", today),
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Error interno al preparar la consulta in getProjects for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		return
	}
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para la tabla projects para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD en getProjects. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var projects []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en getProjects para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de proyectos", http.StatusInternalServerError)
		return
	}

	for i := range projects {
		if shortName, ok := projects[i]["short_name"].(string); ok {
			projects[i]["short_name"] = unescapeComma(shortName)
		}
		if id, ok := projects[i]["id"]; ok {
			//fmt.Printf("Obteniendo IPs para proyecto %v\n", id)
			queryIP := map[string]interface{}{
				"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
				"columns":   "p.name, p.surname, p.id",
				"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL' AND pp.end_date >= '%s'", id, today),
			}

			jsonQueryIP, _ := json.Marshal(queryIP)
			respIP := getReq(jsonQueryIP, apiKey, client, w)
			if respIP == nil {
				createLog(fmt.Sprintf("No se obtuvo respuesta para IPs del proyecto %v para user %s", id, userACCESSING), 1, apiKey, client, w)
				continue
			}

			if respIP.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(respIP.Body)
				respIP.Body.Close()
				createLog(fmt.Sprintf("Error BD al obtener IPs para proyecto %v. Status: %d, Body: %s para user %s",
					id, respIP.StatusCode, string(bodyBytes), userACCESSING), 1, apiKey, client, w)
				continue
			}

			var ips []map[string]interface{}
			if err := json.NewDecoder(respIP.Body).Decode(&ips); err != nil {
				respIP.Body.Close()
				createLog(fmt.Sprintf("Error al decodificar IPs del proyecto %v para user %s: %v", id, userACCESSING, err), 1, apiKey, client, w)
				continue
			}
			respIP.Body.Close()

			outIPs := make([]map[string]string, 0, len(ips))
			for _, ipr := range ips {
				outIPs = append(outIPs, map[string]string{
					"id":      fmt.Sprintf("%.0f", ipr["id"].(float64)),
					"name":    unescapeComma(ipr["name"].(string)),
					"surname": unescapeComma(ipr["surname"].(string)),
				})
			}
			projects[i]["IPs"] = outIPs

			if len(outIPs) == 0 {
				queryGerencia := map[string]interface{}{
					"table":     "people p LEFT JOIN users u ON p.id = u.people_id",
					"columns":   "p.name, p.surname, p.id",
					"condition": "u.access_gerencia = 1",
				}
				jsonQueryGerencia, _ := json.Marshal(queryGerencia)
				respGerencia := getReq(jsonQueryGerencia, apiKey, client, w)
				if respGerencia == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para gerencia al asignar IPs para user %s", userACCESSING), 1, apiKey, client, w)
					continue
				}
				var ips []map[string]interface{}
				if err := json.NewDecoder(respGerencia.Body).Decode(&ips); err != nil {
					respGerencia.Body.Close()
					createLog(fmt.Sprintf("Error al decodificar IPs para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
					continue
				}
				respGerencia.Body.Close()

				outIPs := make([]map[string]string, 0, len(ips))
				for _, ipr := range ips {
					outIPs = append(outIPs, map[string]string{
						"id":      fmt.Sprintf("%.0f", ipr["id"].(float64)),
						"name":    unescapeComma(ipr["name"].(string)),
						"surname": unescapeComma(ipr["surname"].(string)),
					})
				}
				projects[i]["IPs"] = outIPs
			}
			//fmt.Printf("Proyecto: %v, IPs: %v\n", projects[i]["short_name"], projects[i]["IPs"])

		}
	}

	projects = appendContractProgramProject(projects, nil)

	//fmt.Printf("Projectes: %+v\n", projects)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(projects); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en getProjects para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func appendContractProgramProjectIfEligible(projects []map[string]interface{}, userID int, today string, apiKey string, client *http.Client, w http.ResponseWriter) []map[string]interface{} {
	if !hasActiveContractProgramContract(userID, today, apiKey, client, w) {
		return projects
	}
	return appendContractProgramProject(projects, getContractProgramApprovers(apiKey, client, w))
}

func appendContractProgramProject(projects []map[string]interface{}, ips []map[string]string) []map[string]interface{} {
	for _, project := range projects {
		if isContractProgramProjectID(fmt.Sprint(project["id"])) {
			return projects
		}
	}

	project := map[string]interface{}{
		"id":                  contractProgramProjectID,
		"number":              contractProgramProjectID,
		"short_name":          contractProgramProjectName,
		"is_contract_program": true,
	}
	if ips != nil {
		project["IPs"] = ips
	}
	return append(projects, project)
}

func hasActiveContractProgramContract(userID int, today string, apiKey string, client *http.Client, w http.ResponseWriter) bool {
	if userID <= 0 {
		return false
	}

	query := map[string]interface{}{
		"table":   "contract",
		"columns": "id",
		"condition": fmt.Sprintf(
			`people_id = %d
			AND COALESCE(belongs_to_contract_program, 0) = 1
			AND (start_date IS NULL OR start_date = '' OR date(start_date) <= date('%s'))
			AND (end_date IS NULL OR end_date = '' OR date(end_date) >= date('%s'))
			LIMIT 1`,
			userID,
			today,
			today,
		),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false
	}
	return len(rows) > 0
}

func getContractProgramApprovers(apiKey string, client *http.Client, w http.ResponseWriter) []map[string]string {
	query := map[string]interface{}{
		"table":     "people p LEFT JOIN users u ON p.id = u.people_id",
		"columns":   "p.name, p.surname, p.id",
		"condition": "u.access_gerencia = 1",
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return []map[string]string{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []map[string]string{}
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return []map[string]string{}
	}

	ips := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		ips = append(ips, map[string]string{
			"id":      idString(row["id"]),
			"name":    unescapeComma(fmt.Sprint(row["name"])),
			"surname": unescapeComma(fmt.Sprint(row["surname"])),
		})
	}
	return ips
}

func idString(value interface{}) string {
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%.0f", v)
	case float32:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func requestUsesContractProgramProject(requestData RequestPayload) bool {
	projects := []string{
		requestData.TravelProject,
		requestData.RegistrationProject,
		requestData.AccomodationProject,
		requestData.OtherProject,
		requestData.EquipmentProject,
	}
	for _, project := range projects {
		if isContractProgramProjectID(project) {
			return true
		}
	}
	return false
}

func isContractProgramProjectID(projectID string) bool {
	projectID = strings.TrimSpace(projectID)
	return strings.EqualFold(projectID, contractProgramProjectID) ||
		strings.EqualFold(projectID, contractProgramLegacyProjectID)
}

func resolveRequestIP(projectID string, apiKey string, client *http.Client, w http.ResponseWriter) int {
	if projectID == "" {
		return 0
	}

	// 1. Buscar IP del proyecto
	ipPayload := map[string]interface{}{
		"table":   "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
		"columns": "p.id",
		"condition": fmt.Sprintf(
			"pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL'",
			projectID,
		),
	}

	jsonIPPayload, _ := json.Marshal(ipPayload)
	resp := getReq(jsonIPPayload, apiKey, client, w)

	if resp != nil {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var ips []map[string]interface{}

			if err := json.NewDecoder(resp.Body).Decode(&ips); err == nil && len(ips) > 0 {
				if id, ok := ips[0]["id"].(float64); ok {
					return int(id)
				}
			}
		}
	}

	// 2. Si no hay IP, usar gerencia
	gerenciaPayload := map[string]interface{}{
		"table":     "people p LEFT JOIN users u ON p.id = u.people_id",
		"columns":   "p.id",
		"condition": "u.access_gerencia = 1",
	}

	jsonGerenciaPayload, _ := json.Marshal(gerenciaPayload)
	respGerencia := getReq(jsonGerenciaPayload, apiKey, client, w)

	if respGerencia != nil {
		defer respGerencia.Body.Close()

		if respGerencia.StatusCode == http.StatusOK {
			var gerencia []map[string]interface{}

			if err := json.NewDecoder(respGerencia.Body).Decode(&gerencia); err == nil && len(gerencia) > 0 {
				if id, ok := gerencia[0]["id"].(float64); ok {
					return int(id)
				}
			}
		}
	}

	return 0
}

// ---------------- REQUEST VALIDATION ----------------

func validateBudgetRequest(requestData RequestPayload) error {
	if len(requestData.Categories) == 0 {
		//fmt.Println("Validation error: at least one category must be selected")
		return fmt.Errorf("at least one category must be selected")
	}

	hasCategory := func(target string) bool {
		for _, c := range requestData.Categories {
			if strings.EqualFold(strings.TrimSpace(c), target) {
				return true
			}
		}
		return false
	}

	if hasCategory("travel") {
		if strings.TrimSpace(requestData.TravelFrom) == "" {
			//fmt.Println("Validation error: travel from place is required")
			return fmt.Errorf("travel: from place is required")
		}
		if strings.TrimSpace(requestData.TravelTo) == "" {
			//fmt.Println("Validation error: travel destination is required")
			return fmt.Errorf("travel: destination is required")
		}
		if strings.TrimSpace(requestData.Institution) == "" {
			//fmt.Println("Validation error: travel destination institution is required")
			return fmt.Errorf("travel: destination institution is required")
		}
		if strings.TrimSpace(requestData.TravelSince) == "" {
			//fmt.Println("Validation error: travel start date is required")
			return fmt.Errorf("travel: start date is required")
		}
		if strings.TrimSpace(requestData.TravelUntil) == "" {
			//fmt.Println("Validation error: travel end date is required")
			return fmt.Errorf("travel: end date is required")
		}
		if strings.TrimSpace(requestData.TravelPurpose) == "" {
			//fmt.Println("Validation error: travel purpose is required")
			return fmt.Errorf("travel: purpose is required")
		}
		if strings.TrimSpace(requestData.TravelProject) == "" {
			//fmt.Println("Validation error: travel project is required")
			return fmt.Errorf("travel: project is required")
		}
		if requestData.TravelIP <= 0 {
			//fmt.Println("Validation error: travel IP is required")
			return fmt.Errorf("travel: IP is required")
		}
		if strings.TrimSpace(requestData.TravelLuggageType) == "" {
			//fmt.Println("Validation error: travel luggage type is required")
			return fmt.Errorf("travel: luggage type is required")
		}
		if strings.TrimSpace(requestData.TravelSeatPreference) == "" {
			//fmt.Println("Validation error: travel seat preference is required")
			return fmt.Errorf("travel: seat preference is required")
		}
		if strings.TrimSpace(requestData.TravelTimePreference) == "" {
			//fmt.Println("Validation error: travel time preference is required")
			return fmt.Errorf("travel: time preference is required")
		}
		if strings.TrimSpace(requestData.TravelArea) == "" {
			//fmt.Println("Validation error: travel area is required")
			return fmt.Errorf("travel: travel area is required")
		}
	}

	if hasCategory("registration") {
		if strings.TrimSpace(requestData.RegistrationEvent) == "" {
			//fmt.Println("Validation error: registration event is required")
			return fmt.Errorf("registration: event is required")
		}
		if strings.TrimSpace(requestData.RegistrationFile) == "" {
			//fmt.Println("Validation error: registration payment type is required")
			return fmt.Errorf("registration: payment type is required")
		}
		if strings.TrimSpace(requestData.RegistrationProject) == "" {
			//fmt.Println("Validation error: registration project is required")
			return fmt.Errorf("registration: project is required")
		}
		if requestData.RegistrationIP <= 0 {
			//fmt.Println("Validation error: registration IP is required")
			return fmt.Errorf("registration: IP is required")
		}
	}

	if hasCategory("accommodation") {
		if strings.TrimSpace(requestData.AccomodationWhere) == "" {
			//fmt.Println("Validation error: accommodation destination is required")
			return fmt.Errorf("accommodation: destination is required")
		}
		if strings.TrimSpace(requestData.AccomodationSince) == "" {
			//fmt.Println("Validation error: accommodation start date is required")
			return fmt.Errorf("accommodation: start date is required")
		}
		if strings.TrimSpace(requestData.AccomodationUntil) == "" {
			//fmt.Println("Validation error: accommodation end date is required")
			return fmt.Errorf("accommodation: end date is required")
		}
		if strings.TrimSpace(requestData.AccomodationPurpose) == "" {
			//fmt.Println("Validation error: accommodation purpose is required")
			return fmt.Errorf("accommodation: purpose is required")
		}
		if strings.TrimSpace(requestData.AccomodationProject) == "" {
			//fmt.Println("Validation error: accommodation project is required")
			return fmt.Errorf("accommodation: project is required")
		}
		if requestData.AccomodationIP <= 0 {
			//fmt.Println("Validation error: accommodation IP is required")
			return fmt.Errorf("accommodation: IP is required")
		}
	}

	if hasCategory("equipment") {
		if strings.TrimSpace(requestData.EquipmentCategory) == "" {
			//fmt.Println("Validation error: equipment category is required")
			return fmt.Errorf("equipment: category is required")
		}
		if strings.TrimSpace(requestData.EquipmentDescription) == "" {
			//fmt.Println("Validation error: equipment description is required")
			return fmt.Errorf("equipment: description is required")
		}
		if strings.TrimSpace(requestData.EquipmentProject) == "" {
			//fmt.Println("Validation error: equipment project is required")
			return fmt.Errorf("equipment: project is required")
		}
		if requestData.EquipmentIP <= 0 {
			//fmt.Println("Validation error: equipment IP is required")
			return fmt.Errorf("equipment: IP is required")
		}
	}

	if hasCategory("other") {
		if strings.TrimSpace(requestData.OtherDescription) == "" {
			//fmt.Println("Validation error: other description is required")
			return fmt.Errorf("other: description is required")
		}
		if strings.TrimSpace(requestData.OtherProject) == "" {
			//fmt.Println("Validation error: other project is required")
			return fmt.Errorf("other: project is required")
		}
		if requestData.OtherIP <= 0 {
			//fmt.Println("Validation error: other IP is required")
			return fmt.Errorf("other: IP is required")
		}
	}

	return nil
}

// ---------------- CREATE REQUEST ----------------

func handleSubmitRequest(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	//fmt.Printf("Received submitRequest API call with method %s\n", r.Method)

	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB
		createLog(fmt.Sprintf("Error parsing multipart form for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar el formulario multipart", http.StatusBadRequest)
		return
	}

	var requestData RequestPayload

	// ------------------------------ LOAD DATA ----------------------------------
	categoriesRaw := r.FormValue("categories")
	if categoriesRaw != "" {
		if err := json.Unmarshal([]byte(categoriesRaw), &requestData.Categories); err != nil {
			createLog(fmt.Sprintf("Error parsing categories for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al procesar categories", http.StatusBadRequest)
			return
		}
	}

	requestData.TravelFrom = escapeComma(r.FormValue("travel_from_where"))
	requestData.TravelTo = escapeComma(r.FormValue("travel_to_where"))
	requestData.Institution = escapeComma(r.FormValue("institution"))
	requestData.TravelSince = r.FormValue("travel_from_day")
	requestData.TravelUntil = r.FormValue("travel_until_day")
	requestData.TravelPurpose = escapeComma(r.FormValue("travel_purpose"))
	requestData.TravelProject = r.FormValue("travel_project")
	requestData.TravelObservations = escapeComma(r.FormValue("travel_observations"))
	requestData.TravelContactPhone = r.FormValue("travel_contact_phone")
	requestData.TravelContactPhoneOK = r.FormValue("travel_contact_phone_confirmed")
	requestData.TravelPassportType = r.FormValue("travel_passport_doc_type")
	requestData.TravelPassportNumber = r.FormValue("travel_passport_doc_number")
	requestData.TravelPassportExpiration = r.FormValue("travel_passport_expiration")
	requestData.TravelPassportOK = r.FormValue("travel_passport_confirmed")
	requestData.TravelLuggageType = r.FormValue("travel_luggage_type")
	requestData.TravelLuggageKg = nil

	if requestData.TravelLuggageType == "checked" || requestData.TravelLuggageType == "hand_checked" {
		if luggageKgRaw := r.FormValue("travel_checked_kg"); luggageKgRaw != "" {
			if luggageKg, err := strconv.Atoi(luggageKgRaw); err == nil {
				requestData.TravelLuggageKg = &luggageKg
			}
		}
	}
	requestData.TravelSeatPreference = r.FormValue("travel_seat_preference")
	requestData.TravelTimePreference = r.FormValue("travel_time_preference")
	requestData.TravelArea = r.FormValue("travel_area")

	requestData.RegistrationEvent = escapeComma(r.FormValue("registration_event"))
	requestData.RegistrationFile = r.FormValue("registration_registered")
	requestData.RegistrationPayment = r.FormValue("registration_payment")
	requestData.RegistrationProject = r.FormValue("registration_project")
	requestData.RegistrationObservations = escapeComma(r.FormValue("registration_observations"))
	requestData.RegistrationStartDay = r.FormValue("registration_from_day")
	requestData.RegistrationEndDay = r.FormValue("registration_until_day")

	requestData.AccomodationWhere = escapeComma(r.FormValue("accommodation_where"))
	requestData.AccomodationSince = r.FormValue("accommodation_from_day")
	requestData.AccomodationUntil = r.FormValue("accommodation_until_day")
	requestData.AccomodationPurpose = escapeComma(r.FormValue("accommodation_purpose"))
	requestData.AccomodationProject = r.FormValue("accommodation_project")
	requestData.AccomodationObservations = escapeComma(r.FormValue("accommodation_observations"))

	requestData.OtherDescription = escapeComma(r.FormValue("other_description"))
	requestData.OtherPriceRange = r.FormValue("other_price_range")
	requestData.OtherProject = r.FormValue("other_project")

	requestData.EquipmentCategory = r.FormValue("equipment_category")
	requestData.EquipmentOtherDescr = escapeComma(r.FormValue("equipment_other_text"))
	requestData.EquipmentPriceRange = r.FormValue("equipment_price_range")
	requestData.EquipmentDescription = escapeComma(r.FormValue("equipment_description"))
	requestData.EquipmentProject = r.FormValue("equipment_project")

	//fmt.Printf("Parsed request data for user %s: %+v\n", userACCESSING, requestData)
	// -------------------------- IPs -----------------------------------

	if travelIPRaw := r.FormValue("travel_ip"); travelIPRaw != "" {
		travelIP, err := strconv.Atoi(travelIPRaw)
		if err != nil {
			http.Error(w, "travel_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.TravelIP = travelIP
	}

	if requestData.TravelIP == 0 && contains(requestData.Categories, "travel") {
		requestData.TravelIP = resolveRequestIP(requestData.TravelProject, apiKey, client, w)
	}

	if registrationIPRaw := r.FormValue("registration_ip"); registrationIPRaw != "" {
		registrationIP, err := strconv.Atoi(registrationIPRaw)
		if err != nil {
			http.Error(w, "registration_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.RegistrationIP = registrationIP
	}

	if requestData.RegistrationIP == 0 && contains(requestData.Categories, "registration") {
		requestData.RegistrationIP = resolveRequestIP(requestData.RegistrationProject, apiKey, client, w)
	}

	if accomodationIPRaw := r.FormValue("accommodation_ip"); accomodationIPRaw != "" {
		accomodationIP, err := strconv.Atoi(accomodationIPRaw)
		if err != nil {
			http.Error(w, "accommodation_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.AccomodationIP = accomodationIP
	}

	if requestData.AccomodationIP == 0 && contains(requestData.Categories, "accommodation") {
		requestData.AccomodationIP = resolveRequestIP(requestData.AccomodationProject, apiKey, client, w)
	}

	if otherIPRaw := r.FormValue("other_ip"); otherIPRaw != "" {
		otherIP, err := strconv.Atoi(otherIPRaw)
		if err != nil {
			http.Error(w, "other_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.OtherIP = otherIP
	}

	if requestData.OtherIP == 0 && contains(requestData.Categories, "other") {
		requestData.OtherIP = resolveRequestIP(requestData.OtherProject, apiKey, client, w)
	}

	if equipmentIPRaw := r.FormValue("equipment_ip"); equipmentIPRaw != "" {
		equipmentIP, err := strconv.Atoi(equipmentIPRaw)
		if err != nil {
			http.Error(w, "equipment_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.EquipmentIP = equipmentIP
	}

	if requestData.EquipmentIP == 0 && contains(requestData.Categories, "equipment") {
		requestData.EquipmentIP = resolveRequestIP(requestData.EquipmentProject, apiKey, client, w)
	}

	if requestUsesContractProgramProject(requestData) && !hasActiveContractProgramContract(userID, time.Now().Format("2006-01-02"), apiKey, client, w) {
		http.Error(w, "Contracte Programa is only available for people with an active contract programme contract", http.StatusBadRequest)
		return
	}

	if err := validateBudgetRequest(requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// -------------------------- FILES -----------------------------------

	travelFiles := r.MultipartForm.File["travel_preferences_files"]
	if len(travelFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 5 archivos en travel_preferences_files", http.StatusBadRequest)
		return
	}

	savedTravelFiles, err := validateAndSaveFiles("travel", apiKey, client, w, r, travelFiles, uploadDir, requestData.TravelProject)
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//fmt.Printf("rutes de travel files %s", savedTravelFiles)

	registrationFiles := r.MultipartForm.File["registration_invoice"]
	if len(registrationFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 3 archivos en registration_invoice", http.StatusBadRequest)
		return
	}

	var docType string
	if contains(requestData.Categories, "registration") {
		docType = requestData.RegistrationPayment
	}

	savedRegistrationFiles, err := validateAndSaveFiles(docType, apiKey, client, w, r, registrationFiles, uploadDir, requestData.RegistrationProject)
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//fmt.Printf("Saved files: %+v\n", savedTravelFiles)
	//fmt.Printf("Saved registration files: %+v\n", savedRegistrationFiles)

	// -------------------------------- DB ENTRIES --------------------------------

	// create general request
	created := time.Now().Format("2006-01-02 15:04")

	reqPayload := map[string]interface{}{
		"table":   "budget_requests",
		"columns": "people_id, creation_date",
		"value":   fmt.Sprintf("%d, %s", userID, created),
	}
	jsonReqPayload, _ := json.Marshal(reqPayload)
	resp := postReq(jsonReqPayload, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error creating general request entry for user %s", userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al crear la solicitud", http.StatusInternalServerError)
		return
	}

	// get last combined id
	combId := map[string]interface{}{
		"table":   "budget_requests",
		"columns": "MAX(id) as max_id",
	}
	jsonCombId, _ := json.Marshal(combId)
	resp = getReq(jsonCombId, apiKey, client, w)
	if resp == nil {
		fmt.Errorf("error obteniendo último combined_id")
	}
	defer resp.Body.Close()

	var combIdResult []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&combIdResult); err != nil {
		fmt.Errorf("error decodificando último combined_id: %v", err)
	}

	var newCombId int
	if len(combIdResult) > 0 && combIdResult[0]["max_id"] != nil {
		newCombId = int(combIdResult[0]["max_id"].(float64))
	} else {
		return
	}

	// update intern id
	year := time.Now().Year()
	var project string
	switch {
	case requestData.TravelProject != "":
		project = requestData.TravelProject
	case requestData.RegistrationProject != "":
		project = requestData.RegistrationProject
	case requestData.AccomodationProject != "":
		project = requestData.AccomodationProject
	case requestData.OtherProject != "":
		project = requestData.OtherProject
	case requestData.EquipmentProject != "":
		project = requestData.EquipmentProject
	default:
		project = ""
	}
	reqID := fmt.Sprintf("%05d", newCombId)
	internalID := fmt.Sprintf("%d-%s-%s", year, project, reqID)

	updatePayload := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "id_intern",
		"value":     internalID,
		"condition": fmt.Sprintf("id = %d", newCombId),
	}
	jsonUpdatePayload, _ := json.Marshal(updatePayload)
	updateResp := putReq(jsonUpdatePayload, apiKey, client, w)
	if updateResp == nil {
		createLog(fmt.Sprintf("Error updating internal_id for request with combined_id %d for user %s", newCombId, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la solicitud", http.StatusInternalServerError)
		return
	}
	updateResp.Body.Close()

	if contains(requestData.Categories, "travel") {
		if err := createRequestEntry("travel", created, newCombId, requestData, savedTravelFiles, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating travel request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de viaje", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "registration") {
		if err := createRequestEntry("registration", created, newCombId, requestData, savedRegistrationFiles, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating registration request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de inscripción", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "accommodation") {
		if err := createRequestEntry("accommodation", created, newCombId, requestData, nil, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating accommodation request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de alojamiento", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "other") {
		if err := createRequestEntry("other", created, newCombId, requestData, nil, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating other request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de otro gasto", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "equipment") {
		if err := createRequestEntry("equipment", created, newCombId, requestData, nil, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating equipment request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de equipo", http.StatusInternalServerError)
			return
		}
	}

	createLog(fmt.Sprintf("Request submitted successfully with combined_id %d for user %s", newCombId, userACCESSING), 0, apiKey, client, w)

	// Notif to IP
	//ipConfirmation.html
	// Email confirmation to investigator
	//workerConfirmation.html

	if err := sendSubmitRequestEmails(userID, requestData, created, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Request %d created but email sending failed for user %s: %v", newCombId, userACCESSING, err), 1, apiKey, client, w)
		// No devolvemos error al frontend porque la request ya se ha guardado correctamente
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})
}

func createRequestEntry(category string, created string, combinedId int, data RequestPayload, files []SavedFile, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) error {

	userID, username, _ := getUserInfo(apiKey, client, w, r)

	switch category {
	case "travel":

		ipApproved := false
		if data.TravelIP == userID {
			ipApproved = true
		}

		luggageWeightValue := "NULL"
		if data.TravelLuggageKg != nil {
			luggageWeightValue = strconv.Itoa(*data.TravelLuggageKg)
		}

		var requestPayload map[string]interface{}
		if ipApproved {
			today := time.Now().Format("2006-01-02 15:04")
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, ip_response, ip_response_date, project_id, ip_id, purpose, observations, travel_fromPlace, wherePlace, institution, fromDay, untilDay, luggage_type, luggage_weight, seat_preference, time_preference, travel_area",
				"value":   fmt.Sprintf("%d, 1, 1, %s, %s, %d, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s", combinedId, today, data.TravelProject, data.TravelIP, data.TravelPurpose, data.TravelObservations, data.TravelFrom, data.TravelTo, data.Institution, data.TravelSince, data.TravelUntil, data.TravelLuggageType, luggageWeightValue, data.TravelSeatPreference, data.TravelTimePreference, data.TravelArea),
			}
		} else {
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, project_id, ip_id, purpose, observations, travel_fromPlace, wherePlace, institution, fromDay, untilDay, luggage_type, luggage_weight, seat_preference, time_preference, travel_area",
				"value":   fmt.Sprintf("%d, 1,  %s, %d, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s", combinedId, data.TravelProject, data.TravelIP, data.TravelPurpose, data.TravelObservations, data.TravelFrom, data.TravelTo, data.Institution, data.TravelSince, data.TravelUntil, data.TravelLuggageType, luggageWeightValue, data.TravelSeatPreference, data.TravelTimePreference, data.TravelArea),
			}
		}

		jsonPayload, _ := json.Marshal(requestPayload)
		resp := postReq(jsonPayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error creating travel request entry for user %s", username), 1, apiKey, client, w)
			return fmt.Errorf("error creando entrada de solicitud de viaje en la base de datos")
		}
		defer resp.Body.Close()

		if files != nil && len(files) > 0 {
			uploadFilePath(combinedId, 1, files, username, apiKey, client, w, r)
		}

		if err := notifyRRHHTravelIdentityCorrections(userID, data, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error notifying RRHH about travel identity correction for user %s: %v", username, err), 1, apiKey, client, w)
		}

		if err := upsertTravelPhone(userID, username, data.TravelContactPhone, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error saving travel phone for user %s in people_phone table: %v", username, err), 1, apiKey, client, w)
		}

		// check correct passport info

		hasPassportInput :=
			strings.TrimSpace(data.TravelPassportType) != "" ||
				strings.TrimSpace(data.TravelPassportNumber) != "" ||
				strings.TrimSpace(data.TravelPassportExpiration) != ""

		//fmt.Println("Passport inputs provided:", hasPassportInput, data.TravelPassportType, data.TravelPassportNumber, data.TravelPassportExpiration)

		if hasPassportInput {
			passportPayload := map[string]interface{}{
				"table":   "people_passport",
				"columns": "doc_type, doc_number, expiration",
				"condition": fmt.Sprintf(
					"people_id = %d AND doc_type = '%s'",
					userID,
					strings.TrimSpace(data.TravelPassportType),
				),
			}
			jsonPassportPayload, _ := json.Marshal(passportPayload)
			respPassport := getReq(jsonPassportPayload, apiKey, client, w)

			if respPassport != nil && respPassport.StatusCode == http.StatusOK {
				var passportResult []map[string]interface{}
				if err := json.NewDecoder(respPassport.Body).Decode(&passportResult); err == nil {
					//fmt.Println("Existing passport info:", passportResult)
					if len(passportResult) > 0 {
						// UPDATE existing passport row
						updateNeeded := false
						updateFields := make(map[string]string)

						currentDocType := strings.TrimSpace(asString(passportResult[0]["doc_type"]))
						currentDocNumber := strings.TrimSpace(asString(passportResult[0]["doc_number"]))
						currentExpiration := strings.TrimSpace(asString(passportResult[0]["expiration"]))

						if data.TravelPassportType != "" && currentDocType != data.TravelPassportType {
							updateNeeded = true
							updateFields["doc_type"] = data.TravelPassportType
						}
						if data.TravelPassportNumber != "" && currentDocNumber != data.TravelPassportNumber {
							updateNeeded = true
							updateFields["doc_number"] = data.TravelPassportNumber
						}
						if data.TravelPassportExpiration != "" && currentExpiration != data.TravelPassportExpiration {
							updateNeeded = true
							updateFields["expiration"] = data.TravelPassportExpiration
						}

						if updateNeeded {
							setClause := ""
							colNames := ""
							for k, v := range updateFields {
								setClause += fmt.Sprintf("%s, ", v)
								colNames += fmt.Sprintf("%s, ", k)
							}
							setClause = strings.TrimSuffix(setClause, ", ")
							colNames = strings.TrimSuffix(colNames, ", ")

							updatePassportPayload := map[string]interface{}{
								"table":     "people_passport",
								"columns":   colNames,
								"value":     setClause,
								"condition": fmt.Sprintf("people_id = %d AND doc_type = '%s'", userID, strings.TrimSpace(data.TravelPassportType)),
							}
							jsonUpdatePassport, _ := json.Marshal(updatePassportPayload)
							respUpdatePassport := putReq(jsonUpdatePassport, apiKey, client, w)
							if respUpdatePassport == nil {
								createLog(fmt.Sprintf("Error updating passport info for user %s in people_passport table", username), 1, apiKey, client, w)
							} else {
								respUpdatePassport.Body.Close()
							}
						}
					} else {
						// INSERT new passport row
						insertPassportPayload := map[string]interface{}{
							"table":   "people_passport",
							"columns": "people_id, doc_type, doc_number, expiration",
							"value": fmt.Sprintf(
								"%d, %s, %s, %s",
								userID,
								data.TravelPassportType,
								data.TravelPassportNumber,
								data.TravelPassportExpiration,
							),
						}
						jsonInsertPassport, _ := json.Marshal(insertPassportPayload)
						respInsertPassport := postReq(jsonInsertPassport, apiKey, client, w)
						if respInsertPassport == nil {
							createLog(fmt.Sprintf("Error creating passport info for user %s in people_passport table", username), 1, apiKey, client, w)
						} else {
							respInsertPassport.Body.Close()
						}
						fmt.Println("New passport info created for user:", username)
					}
				}
				respPassport.Body.Close()
			}
		}
		categoryID := 1 // travel category id

		partID, err := getLastBudgetPartID(combinedId, categoryID, apiKey, client, w)
		if err != nil {
			return err
		}

		err = auditBudgetPartCreation(
			combinedId,
			categoryID,
			"viatge",
			partID,
			ipApproved,
			userID,
			username,
			map[string]interface{}{
				"category":         "viatge",
				"project_id":       data.TravelProject,
				"ip_id":            data.TravelIP,
				"purpose":          unescapeComma(data.TravelPurpose),
				"from_place":       unescapeComma(data.TravelFrom),
				"where_place":      unescapeComma(data.TravelTo),
				"institution":      unescapeComma(data.Institution),
				"from_day":         data.TravelSince,
				"until_day":        data.TravelUntil,
				"ip_auto_approved": ipApproved,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			return err
		}

	case "registration":
		// fmt.Printf("Creating registration request entry for user %s with combinedId %d\n", username, combinedId)
		ipApproved := false
		if data.RegistrationIP == userID {
			ipApproved = true
		}

		regFile := 0
		if data.RegistrationFile == "yes" {
			regFile = 1
		}
		regType := ""
		if data.RegistrationPayment == "invoice" {
			regType = "I"
		} else {
			regType = "D"
		}

		var requestPayload map[string]interface{}
		if ipApproved {
			today := time.Now().Format("2006-01-02 15:04")
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, ip_response, ip_response_date, project_id, ip_id, observations, purpose, registration_invoice, registration_type, fromDay, untilDay",
				"value":   fmt.Sprintf("%d, 2, 1, %s, %s, %d, %s, %s, %d, %s, %s, %s", combinedId, today, data.RegistrationProject, data.RegistrationIP, data.RegistrationObservations, data.RegistrationEvent, regFile, regType, data.RegistrationStartDay, data.RegistrationEndDay),
			}
		} else {
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, project_id, ip_id, observations, purpose, registration_invoice, registration_type, fromDay, untilDay",
				"value":   fmt.Sprintf("%d, 2, %s, %d, %s, %s, %d, %s, %s, %s", combinedId, data.RegistrationProject, data.RegistrationIP, data.RegistrationObservations, data.RegistrationEvent, regFile, regType, data.RegistrationStartDay, data.RegistrationEndDay),
			}
		}
		jsonPayload, _ := json.Marshal(requestPayload)
		resp := postReq(jsonPayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error creating registration request entry for user %s", username), 1, apiKey, client, w)
			return fmt.Errorf("error creando entrada de solicitud de inscripción en la base de datos")
		}
		defer resp.Body.Close()

		if files != nil && len(files) > 0 {
			uploadFilePath(combinedId, 2, files, username, apiKey, client, w, r)
		}

		categoryID := 2 // registration category id

		partID, err := getLastBudgetPartID(combinedId, categoryID, apiKey, client, w)
		if err != nil {
			return err
		}

		err = auditBudgetPartCreation(
			combinedId,
			categoryID,
			"inscripció",
			partID,
			ipApproved,
			userID,
			username,
			map[string]interface{}{
				"category":         "inscripció",
				"project_id":       data.RegistrationProject,
				"ip_id":            data.RegistrationIP,
				"purpose":          unescapeComma(data.RegistrationEvent),
				"from_day":         data.RegistrationStartDay,
				"until_day":        data.RegistrationEndDay,
				"ip_auto_approved": ipApproved,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			return err
		}
	case "accommodation":
		ipApproved := false
		if data.AccomodationIP == userID {
			ipApproved = true
		}

		var requestPayload map[string]interface{}
		if ipApproved {
			today := time.Now().Format("2006-01-02 15:04")
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, ip_response, ip_response_date, project_id, ip_id, purpose, observations, wherePlace, fromDay, untilDay",
				"value":   fmt.Sprintf("%d, 3, 1, %s, %s, %d, %s, %s, %s, %s, %s", combinedId, today, data.AccomodationProject, data.AccomodationIP, data.AccomodationPurpose, data.AccomodationObservations, data.AccomodationWhere, data.AccomodationSince, data.AccomodationUntil),
			}
		} else {
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, project_id, ip_id, purpose, observations, wherePlace, fromDay, untilDay",
				"value":   fmt.Sprintf("%d, 3, %s, %d, %s, %s, %s, %s, %s", combinedId, data.AccomodationProject, data.AccomodationIP, data.AccomodationPurpose, data.AccomodationObservations, data.AccomodationWhere, data.AccomodationSince, data.AccomodationUntil),
			}
		}

		jsonPayload, _ := json.Marshal(requestPayload)
		resp := postReq(jsonPayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error creating accommodation request entry for user %s", username), 1, apiKey, client, w)
			return fmt.Errorf("error creando entrada de solicitud de alojamiento en la base de datos")
		}
		defer resp.Body.Close()

		categoryID := 3 // accommodation category id

		partID, err := getLastBudgetPartID(combinedId, categoryID, apiKey, client, w)
		if err != nil {
			return err
		}

		err = auditBudgetPartCreation(
			combinedId,
			categoryID,
			"allotjament",
			partID,
			ipApproved,
			userID,
			username,
			map[string]interface{}{
				"category":         "allotjament",
				"project_id":       data.AccomodationProject,
				"ip_id":            data.AccomodationIP,
				"purpose":          unescapeComma(data.AccomodationPurpose),
				"where_place":      unescapeComma(data.AccomodationWhere),
				"from_day":         data.AccomodationSince,
				"until_day":        data.AccomodationUntil,
				"ip_auto_approved": ipApproved,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			return err
		}
	case "other":
		ipApproved := false
		if data.OtherIP == userID {
			ipApproved = true
		}

		var requestPayload map[string]interface{}
		if ipApproved {
			today := time.Now().Format("2006-01-02 15:04")
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, ip_response, ip_response_date, project_id, ip_id, observations, other_price",
				"value":   fmt.Sprintf("%d, 5, 1, %s, %s, %d, %s, %s", combinedId, today, data.OtherProject, data.OtherIP, data.OtherDescription, data.OtherPriceRange),
			}
		} else {
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, project_id, ip_id, observations, other_price",
				"value":   fmt.Sprintf("%d, 5, %s, %d, %s, %s", combinedId, data.OtherProject, data.OtherIP, data.OtherDescription, data.OtherPriceRange),
			}
		}

		jsonPayload, _ := json.Marshal(requestPayload)
		resp := postReq(jsonPayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error creating other request entry for user %s", username), 1, apiKey, client, w)
			return fmt.Errorf("error creando entrada de solicitud de otro gasto en la base de datos")
		}
		defer resp.Body.Close()
		categoryID := 5 // other category id

		partID, err := getLastBudgetPartID(combinedId, categoryID, apiKey, client, w)
		if err != nil {
			return err
		}

		err = auditBudgetPartCreation(
			combinedId,
			categoryID,
			"altres despeses",
			partID,
			ipApproved,
			userID,
			username,
			map[string]interface{}{
				"category":         "altres despeses",
				"project_id":       data.OtherProject,
				"ip_id":            data.OtherIP,
				"purpose":          unescapeComma(data.OtherDescription),
				"price_range":      unescapeComma(data.OtherPriceRange),
				"ip_auto_approved": ipApproved,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			return err
		}
	case "equipment":
		ipApproved := false
		if data.EquipmentIP == userID {
			ipApproved = true
		}

		var requestPayload map[string]interface{}
		if ipApproved {
			today := time.Now().Format("2006-01-02 15:04")
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, ip_response, ip_response_date, project_id, ip_id, observations, equipment_category, purpose, other_price",
				"value":   fmt.Sprintf("%d, 4, 1, %s, %s, %d, %s, %s, %s, %s", combinedId, today, data.EquipmentProject, data.EquipmentIP, data.EquipmentDescription, data.EquipmentCategory, data.EquipmentOtherDescr, data.EquipmentPriceRange),
			}
		} else {
			requestPayload = map[string]interface{}{
				"table":   "budget_parts",
				"columns": "id_combined, category_id, project_id, ip_id, observations, equipment_category, purpose, other_price",
				"value":   fmt.Sprintf("%d, 4, %s, %d, %s, %s, %s, %s", combinedId, data.EquipmentProject, data.EquipmentIP, data.EquipmentDescription, data.EquipmentCategory, data.EquipmentOtherDescr, data.EquipmentPriceRange),
			}
		}
		jsonPayload, _ := json.Marshal(requestPayload)
		resp := postReq(jsonPayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error creating equipment request entry for user %s", username), 1, apiKey, client, w)
			return fmt.Errorf("error creando entrada de solicitud de equipo en la base de datos")
		}
		defer resp.Body.Close()
		categoryID := 4 // equipment category id

		partID, err := getLastBudgetPartID(combinedId, categoryID, apiKey, client, w)
		if err != nil {
			return err
		}

		err = auditBudgetPartCreation(
			combinedId,
			categoryID,
			"equipament",
			partID,
			ipApproved,
			userID,
			username,
			map[string]interface{}{
				"category":           "equipament",
				"project_id":         data.EquipmentProject,
				"ip_id":              data.EquipmentIP,
				"observations":       unescapeComma(data.EquipmentDescription),
				"equipment_category": data.EquipmentCategory,
				"purpose":            unescapeComma(data.EquipmentOtherDescr),
				"price_range":        unescapeComma(data.EquipmentPriceRange),
				"ip_auto_approved":   ipApproved,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func uploadFilePath(idCombined int, category int, files []SavedFile, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) error {
	getIDPayload := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "MAX(id) as id",
		"condition": fmt.Sprintf("id_combined = %d AND category_id = %d", idCombined, category),
	}
	jsonGetIDPayload, _ := json.Marshal(getIDPayload)
	respID := getReq(jsonGetIDPayload, apiKey, client, w)
	if respID == nil {
		createLog(fmt.Sprintf("Error retrieving ID for %d request entry for user %s", category, username), 1, apiKey, client, w)
		return fmt.Errorf("error obteniendo ID de la solicitud de viaje recién creada")
	}
	defer respID.Body.Close()

	var idResult []map[string]interface{}
	if err := json.NewDecoder(respID.Body).Decode(&idResult); err != nil {
		createLog(fmt.Sprintf("Error decoding ID retrieval response for %d request entry for user %s: %v", category, username, err), 1, apiKey, client, w)
		return fmt.Errorf("error decodificando respuesta al obtener ID de la solicitud de viaje: %v", err)
	}

	if len(idResult) == 0 || idResult[0]["id"] == nil {
		createLog(fmt.Sprintf("No ID found for %d request entry for user %s", category, username), 1, apiKey, client, w)
		return fmt.Errorf("no se encontró ID para la solicitud de viaje recién creada")
	}

	requestId := int(idResult[0]["id"].(float64))
	for _, f := range files {
		filepath := cleanPath(f.Path)
		filePayload := map[string]interface{}{
			"table":   "request_files",
			"columns": "request_id, file_path",
			"value":   fmt.Sprintf("%d, %s", requestId, filepath),
		}
		jsonFilePayload, _ := json.Marshal(filePayload)
		resp := postReq(jsonFilePayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error saving file record for travel request for user %s: file %s", username, f.OriginalName), 1, apiKey, client, w)
		} else {
			resp.Body.Close()
		}
	}
	return nil
}

// ---------------- SENT REQUESTS ----------------

func handleGetSentRequests(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "id as combined_id, id_intern, creation_date, projects_response, projects_response_date, acc_or_it_response, acc_or_it_response_date, denied_comment, prev_request",
		"condition": fmt.Sprintf("people_id = %d AND canceled = 0", userID),
	}
	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para las solicitudes enviadas para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	//fmt.Printf("Received response for sent requests query for user %s: Status %d\n", userACCESSING, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener solicitudes enviadas. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var requests []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&requests); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en getSentRequests para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de solicitudes", http.StatusInternalServerError)
		return
	}

	for i := range requests {
		//fmt.Printf("Processing request with combined_id %v for user %s\n", requests[i]["combined_id"], userACCESSING)

		// una mateixa request es separarà en dos a l'hora de mostrar si té dues parts amb project_id diferent
		reqPartsPayload := map[string]interface{}{
			"table":     "budget_parts bp LEFT JOIN request_files rf ON bp.id = rf.request_id",
			"columns":   "*",
			"condition": fmt.Sprintf("id_combined = %d", int(requests[i]["combined_id"].(float64))),
		}
		jsonReqPartsPayload, _ := json.Marshal(reqPartsPayload)
		respParts := getReq(jsonReqPartsPayload, apiKey, client, w)
		if respParts != nil && respParts.StatusCode == http.StatusOK {
			var rawParts []map[string]interface{}
			if err := json.NewDecoder(respParts.Body).Decode(&rawParts); err == nil {
				partsByID := make(map[string]map[string]interface{})

				for _, row := range rawParts {
					partID := fmt.Sprintf("%v", row["id"])
					if partID == "" || partID == "<nil>" {
						continue
					}

					existing, exists := partsByID[partID]
					if !exists {
						clone := make(map[string]interface{})
						for k, v := range row {
							if k == "file_path" {
								continue
							}
							clone[k] = v
						}
						clone["file_paths"] = []string{}
						partsByID[partID] = clone
						existing = clone
					}

					filePath, ok := row["file_path"]
					if ok && filePath != nil {
						pathStr := fmt.Sprintf("%v", filePath)
						if pathStr != "" && pathStr != "<nil>" {
							current := existing["file_paths"].([]string)

							alreadyExists := false
							for _, p := range current {
								if p == pathStr {
									alreadyExists = true
									break
								}
							}

							if !alreadyExists {
								existing["file_paths"] = append(current, pathStr)
							}
						}
					}
				}
				normalizedParts := make([]map[string]interface{}, 0, len(partsByID))
				for _, part := range partsByID {
					part["purpose"] = unescapeComma(fmt.Sprintf("%v", part["purpose"]))
					part["observations"] = unescapeComma(fmt.Sprintf("%v", part["observations"]))
					part["travel_fromPlace"] = unescapeComma(fmt.Sprintf("%v", part["travel_fromPlace"]))
					part["wherePlace"] = unescapeComma(fmt.Sprintf("%v", part["wherePlace"]))
					normalizedParts = append(normalizedParts, part)
				}
				grouped := make(map[string][]map[string]interface{})
				for _, part := range normalizedParts {
					ipID := fmt.Sprintf("%v", part["ip_id"])
					grouped[ipID] = append(grouped[ipID], part)
				}
				var groupedParts []map[string]interface{}
				for ipID, groupedItems := range grouped {
					groupedParts = append(groupedParts, map[string]interface{}{
						"ip_id": ipID,
						"parts": groupedItems,
					})
				}

				requests[i]["parts"] = groupedParts
				requests[i]["denied_comment"] = unescapeComma(asString(requests[i]["denied_comment"]))
			}
			respParts.Body.Close()
		}

		//fmt.Printf("Resposta: %v\n", requests[i]["parts"])
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requests); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en getSentRequests para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func handleCancelRequest(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	type CancelRequestBody struct {
		CombinedID string `json:"idCombined"`
		Motive     string `json:"motive"`
	}

	var body CancelRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.CombinedID == "" {
		http.Error(w, "combined_id is required", http.StatusBadRequest)
		return
	}

	//fmt.Println("Valors rebuts: ", body.CombinedID, body.Motive)
	update := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "canceled, cancelation_motive",
		"value":     fmt.Sprintf("1, %s", escapeComma(body.Motive)),
		"condition": fmt.Sprintf("id = %s", body.CombinedID),
	}

	jsonUpdate, _ := json.Marshal(update)
	resp := putReq(jsonUpdate, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para cancelar solicitud %s", body.CombinedID), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	//fmt.Println("Respuesta del servidor:", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al cancelar solicitud %s. Status: %d, Body: %s",
			body.CombinedID, resp.StatusCode, string(bodyBytes),
		), 1, apiKey, client, w)
		http.Error(w, "Error al cancelar la solicitud", http.StatusInternalServerError)
		return
	}

	combinedID, err := strconv.Atoi(body.CombinedID)
	if err != nil {
		createLog(fmt.Sprintf("Invalid combinedID on cancel flow: %s", body.CombinedID), 1, apiKey, client, w)
		http.Error(w, "Invalid combined_id", http.StatusBadRequest)
		return
	}

	if err := notifyCancellationRecipients(combinedID, body.Motive, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf(
			"Cancellation completed for request %d, but notification flow failed: %v",
			combinedID, err,
		), 1, apiKey, client, w)
		// No devolvemos error aquí porque la cancelación ya se ha aplicado.
	}

	personInfo, err := getPersonInfoByID(userID, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error retrieving person info for audit log after cancellation of request %d: %v", combinedID, err), 1, apiKey, client, w)
	}

	err = createBudgetAuditLog(
		combinedID,
		nil,
		nil,
		"cancel·lat",
		"cancel·lat",
		"Petició cancel·lada pel treballador",
		userID,
		userACCESSING,
		personInfo.Name,
		personInfo.Surname,
		map[string]interface{}{
			"motive": body.Motive,
		},
		apiKey,
		client,
		w,
	)
	if err != nil {
		createLog(fmt.Sprintf("Error creating cancellation audit log for request %d: %v", combinedID, err), 1, apiKey, client, w)
		http.Error(w, "Error creando audit log de cancelación", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleAddInvoice(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB
		createLog(fmt.Sprintf("Error parsing multipart form for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar el formulario multipart", http.StatusBadRequest)
		return
	}

	combinedIDStr := strings.TrimSpace(r.FormValue("idCombined"))
	if combinedIDStr == "" {
		http.Error(w, "idCombined es obligatorio", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(combinedIDStr)
	if err != nil {
		http.Error(w, "idCombined no es válido", http.StatusBadRequest)
		return
	}

	requestInfoQuery := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "id, project_id",
		"condition": fmt.Sprintf("id_combined = %d AND category_id = 2", combinedID),
	}

	jsonRequestInfoQuery, _ := json.Marshal(requestInfoQuery)
	respRequestInfo := getReq(jsonRequestInfoQuery, apiKey, client, w)
	if respRequestInfo == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para info de solicitud en add invoice para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer respRequestInfo.Body.Close()

	if respRequestInfo.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respRequestInfo.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al obtener info de solicitud en add invoice. Status: %d, Body: %s para user %s",
			respRequestInfo.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var requestInfo []map[string]interface{}
	if err := json.NewDecoder(respRequestInfo.Body).Decode(&requestInfo); err != nil {
		createLog(fmt.Sprintf("Error decoding request info for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar la respuesta de la base de datos", http.StatusInternalServerError)
		return
	}

	registrationFiles := r.MultipartForm.File["registration_invoice"]
	if len(registrationFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 3 archivos en registration_invoice", http.StatusBadRequest)
		return
	}

	savedRegistrationFiles, err := validateAndSaveFiles("invoice", apiKey, client, w, r, registrationFiles, uploadDir, requestInfo[0]["project_id"].(string))
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//fmt.Printf("Saved registration files: %+v\n", savedRegistrationFiles)

	for _, f := range savedRegistrationFiles {
		filepath := cleanPath(f.Path)
		filePayload := map[string]interface{}{
			"table":   "request_files",
			"columns": "request_id, file_path, type",
			"value":   fmt.Sprintf("%d, %s, updatedInvoice", int(requestInfo[0]["id"].(float64)), filepath),
		}
		jsonFilePayload, _ := json.Marshal(filePayload)
		resp := postReq(jsonFilePayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error saving file record for travel request for user %s: file %s", userACCESSING, f.OriginalName), 1, apiKey, client, w)
		} else {
			resp.Body.Close()
		}
	}
	// update registration status
	registrationQuery := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "registration_invoice, registration_type",
		"value":     "1, I",
		"condition": fmt.Sprintf("id_combined = %d AND category_id = 2", combinedID),
	}

	jsonRegistrationQuery, _ := json.Marshal(registrationQuery)
	respRegistration := putReq(jsonRegistrationQuery, apiKey, client, w)
	if respRegistration == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de registro en add invoice para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer respRegistration.Body.Close()

	if respRegistration.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respRegistration.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al actualizar estado de registro en add invoice. Status: %d, Body: %s para user %s",
			respRegistration.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la base de datos", http.StatusInternalServerError)
		return
	}

	if err := notifyProjectsInvoiceProvided(combinedID, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Error notifying Projects about updated invoice for request %d: %v", combinedID, err), 1, apiKey, client, w)
	}

	if err := notifyAccountingInvoiceProvided(combinedID, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Error notifying Accounting about updated invoice for request %d: %v", combinedID, err), 1, apiKey, client, w)
	}

	w.WriteHeader(http.StatusOK)

}

func handleModifyRequest(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB
		createLog(fmt.Sprintf("Error parsing multipart form for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar el formulario multipart", http.StatusBadRequest)
		return
	}

	var requestData RequestPayload

	// ------------------------ CANCEL PREVIOUS REQUEST --------------------------
	combinedIDStr := strings.TrimSpace(r.FormValue("idCombined"))
	if combinedIDStr == "" {
		http.Error(w, "idCombined es obligatorio", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(combinedIDStr)
	if err != nil {
		http.Error(w, "idCombined no es válido", http.StatusBadRequest)
		return
	}

	cancelUpdate := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "canceled, cancelation_motive",
		"value":     "1, New modified request",
		"condition": fmt.Sprintf("id = %d", combinedID),
	}

	jsonCancelUpdate, _ := json.Marshal(cancelUpdate)
	resp := putReq(jsonCancelUpdate, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para cancelar solicitud previa en modify request para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al cancelar solicitud previa en modify request. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al cancelar la solicitud previa", http.StatusInternalServerError)
		return
	}
	// ------------------------------ LOAD DATA ----------------------------------
	categoriesRaw := r.FormValue("categories")
	if categoriesRaw != "" {
		if err := json.Unmarshal([]byte(categoriesRaw), &requestData.Categories); err != nil {
			createLog(fmt.Sprintf("Error parsing categories for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al procesar categories", http.StatusBadRequest)
			return
		}
	}

	requestData.TravelFrom = escapeComma(r.FormValue("travel_from_where"))
	requestData.TravelTo = escapeComma(r.FormValue("travel_to_where"))
	requestData.Institution = escapeComma(r.FormValue("institution"))
	requestData.TravelSince = r.FormValue("travel_from_day")
	requestData.TravelUntil = r.FormValue("travel_until_day")
	requestData.TravelPurpose = escapeComma(r.FormValue("travel_purpose"))
	requestData.TravelProject = r.FormValue("travel_project")
	requestData.TravelObservations = escapeComma(r.FormValue("travel_observations"))
	requestData.TravelLuggageType = r.FormValue("travel_luggage_type")
	requestData.TravelLuggageKg = nil

	if requestData.TravelLuggageType == "checked" || requestData.TravelLuggageType == "hand_checked" {
		if luggageKgRaw := r.FormValue("travel_checked_kg"); luggageKgRaw != "" {
			if luggageKg, err := strconv.Atoi(luggageKgRaw); err == nil {
				requestData.TravelLuggageKg = &luggageKg
			}
		}
	}
	requestData.TravelSeatPreference = r.FormValue("travel_seat_preference")
	requestData.TravelTimePreference = r.FormValue("travel_time_preference")
	requestData.TravelArea = r.FormValue("travel_area")
	requestData.TravelContactPhone = r.FormValue("travel_contact_phone")
	requestData.TravelContactPhoneOK = r.FormValue("travel_contact_phone_confirmed")
	requestData.TravelPassportType = r.FormValue("travel_passport_doc_type")
	requestData.TravelPassportNumber = r.FormValue("travel_passport_doc_number")
	requestData.TravelPassportExpiration = r.FormValue("travel_passport_expiration")
	requestData.TravelPassportOK = r.FormValue("travel_passport_confirmed")

	requestData.RegistrationEvent = escapeComma(r.FormValue("registration_event"))
	requestData.RegistrationFile = r.FormValue("registration_registered")
	requestData.RegistrationPayment = r.FormValue("registration_payment")
	requestData.RegistrationProject = r.FormValue("registration_project")
	requestData.RegistrationObservations = escapeComma(r.FormValue("registration_observations"))
	requestData.RegistrationStartDay = r.FormValue("registration_from_day")
	requestData.RegistrationEndDay = r.FormValue("registration_until_day")

	requestData.AccomodationWhere = escapeComma(r.FormValue("accommodation_where"))
	requestData.AccomodationSince = r.FormValue("accommodation_from_day")
	requestData.AccomodationUntil = r.FormValue("accommodation_until_day")
	requestData.AccomodationPurpose = escapeComma(r.FormValue("accommodation_purpose"))
	requestData.AccomodationProject = r.FormValue("accommodation_project")
	requestData.AccomodationObservations = escapeComma(r.FormValue("accommodation_observations"))

	requestData.OtherDescription = r.FormValue("other_description")
	requestData.OtherPriceRange = r.FormValue("other_price_range")
	requestData.OtherProject = r.FormValue("other_project")

	requestData.EquipmentCategory = r.FormValue("equipment_category")
	requestData.EquipmentOtherDescr = escapeComma(r.FormValue("equipment_other_text"))
	requestData.EquipmentPriceRange = r.FormValue("equipment_price_range")
	requestData.EquipmentDescription = escapeComma(r.FormValue("equipment_description"))
	requestData.EquipmentProject = r.FormValue("equipment_project")

	//fmt.Printf("Parsed request data for user %s: %+v\n", userACCESSING, requestData)
	// -------------------------- IPs -----------------------------------

	if travelIPRaw := r.FormValue("travel_ip"); travelIPRaw != "" {
		travelIP, err := strconv.Atoi(travelIPRaw)
		if err != nil {
			http.Error(w, "travel_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.TravelIP = travelIP
	}

	if requestData.TravelIP == 0 && contains(requestData.Categories, "travel") {
		requestData.TravelIP = resolveRequestIP(requestData.TravelProject, apiKey, client, w)
	}

	if registrationIPRaw := r.FormValue("registration_ip"); registrationIPRaw != "" {
		registrationIP, err := strconv.Atoi(registrationIPRaw)
		if err != nil {
			http.Error(w, "registration_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.RegistrationIP = registrationIP
	}

	if requestData.RegistrationIP == 0 && contains(requestData.Categories, "registration") {
		requestData.RegistrationIP = resolveRequestIP(requestData.RegistrationProject, apiKey, client, w)
	}

	if accomodationIPRaw := r.FormValue("accommodation_ip"); accomodationIPRaw != "" {
		accomodationIP, err := strconv.Atoi(accomodationIPRaw)
		if err != nil {
			http.Error(w, "accommodation_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.AccomodationIP = accomodationIP
	}

	if requestData.AccomodationIP == 0 && contains(requestData.Categories, "accommodation") {
		requestData.AccomodationIP = resolveRequestIP(requestData.AccomodationProject, apiKey, client, w)
	}

	if otherIPRaw := r.FormValue("other_ip"); otherIPRaw != "" {
		otherIP, err := strconv.Atoi(otherIPRaw)
		if err != nil {
			http.Error(w, "other_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.OtherIP = otherIP
	}

	if requestData.OtherIP == 0 && contains(requestData.Categories, "other") {
		requestData.OtherIP = resolveRequestIP(requestData.OtherProject, apiKey, client, w)
	}

	if equipmentIPRaw := r.FormValue("equipment_ip"); equipmentIPRaw != "" {
		equipmentIP, err := strconv.Atoi(equipmentIPRaw)
		if err != nil {
			http.Error(w, "equipment_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.EquipmentIP = equipmentIP
	}

	if requestData.EquipmentIP == 0 && contains(requestData.Categories, "equipment") {
		requestData.EquipmentIP = resolveRequestIP(requestData.EquipmentProject, apiKey, client, w)
	}

	if requestUsesContractProgramProject(requestData) && !hasActiveContractProgramContract(userID, time.Now().Format("2006-01-02"), apiKey, client, w) {
		http.Error(w, "Contracte Programa is only available for people with an active contract programme contract", http.StatusBadRequest)
		return
	}

	if err := validateBudgetRequest(requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// -------------------------- FILES -----------------------------------

	travelFiles := r.MultipartForm.File["travel_preferences_files"]
	if len(travelFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 5 archivos en travel_preferences_files", http.StatusBadRequest)
		return
	}

	savedTravelFiles, err := validateAndSaveFiles("travel", apiKey, client, w, r, travelFiles, uploadDir, requestData.TravelProject)
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//fmt.Printf("rutes de travel files %s", savedTravelFiles)

	registrationFiles := r.MultipartForm.File["registration_invoice"]
	if len(registrationFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 3 archivos en registration_invoice", http.StatusBadRequest)
		return
	}

	var docType string
	if contains(requestData.Categories, "registration") {
		docType = requestData.RegistrationPayment
	}

	savedRegistrationFiles, err := validateAndSaveFiles(docType, apiKey, client, w, r, registrationFiles, uploadDir, requestData.RegistrationProject)
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//fmt.Printf("Saved files: %+v\n", savedTravelFiles)
	//fmt.Printf("Saved registration files: %+v\n", savedRegistrationFiles)

	// -------------------------------- DB ENTRIES --------------------------------

	// create general request
	created := time.Now().Format("2006-01-02 15:04")

	reqPayload := map[string]interface{}{
		"table":   "budget_requests",
		"columns": "people_id, creation_date, prev_request",
		"value":   fmt.Sprintf("%d, %s, %d", userID, created, combinedID),
	}
	jsonReqPayload, _ := json.Marshal(reqPayload)
	resp = postReq(jsonReqPayload, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error creating general request entry for user %s", userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al crear la solicitud", http.StatusInternalServerError)
		return
	}

	// get last combined id
	combId := map[string]interface{}{
		"table":   "budget_requests",
		"columns": "MAX(id) as max_id",
	}
	jsonCombId, _ := json.Marshal(combId)
	resp = getReq(jsonCombId, apiKey, client, w)
	if resp == nil {
		fmt.Errorf("error obteniendo último combined_id")
	}
	defer resp.Body.Close()

	var combIdResult []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&combIdResult); err != nil {
		fmt.Errorf("error decodificando último combined_id: %v", err)
	}

	var newCombId int
	if len(combIdResult) > 0 && combIdResult[0]["max_id"] != nil {
		newCombId = int(combIdResult[0]["max_id"].(float64))
	} else {
		return
	}

	// update intern id
	year := time.Now().Year()
	var project string
	switch {
	case requestData.TravelProject != "":
		project = requestData.TravelProject
	case requestData.RegistrationProject != "":
		project = requestData.RegistrationProject
	case requestData.AccomodationProject != "":
		project = requestData.AccomodationProject
	case requestData.OtherProject != "":
		project = requestData.OtherProject
	case requestData.EquipmentProject != "":
		project = requestData.EquipmentProject
	default:
		project = ""
	}
	reqID := fmt.Sprintf("%05d", newCombId)
	internalID := fmt.Sprintf("%d-%s-%s", year, project, reqID)

	updatePayload := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "id_intern",
		"value":     internalID,
		"condition": fmt.Sprintf("id = %d", newCombId),
	}
	jsonUpdatePayload, _ := json.Marshal(updatePayload)
	updateResp := putReq(jsonUpdatePayload, apiKey, client, w)
	if updateResp == nil {
		createLog(fmt.Sprintf("Error updating internal_id for request with combined_id %d for user %s", newCombId, userACCESSING), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la solicitud", http.StatusInternalServerError)
		return
	}
	updateResp.Body.Close()

	if contains(requestData.Categories, "travel") {
		if err := createRequestEntry("travel", created, newCombId, requestData, savedTravelFiles, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating travel request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de viaje", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "registration") {
		if err := createRequestEntry("registration", created, newCombId, requestData, savedRegistrationFiles, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating registration request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de inscripción", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "accommodation") {
		if err := createRequestEntry("accommodation", created, newCombId, requestData, nil, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating accommodation request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de alojamiento", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "other") {
		if err := createRequestEntry("other", created, newCombId, requestData, nil, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating other request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de otro gasto", http.StatusInternalServerError)
			return
		}
	}
	if contains(requestData.Categories, "equipment") {
		if err := createRequestEntry("equipment", created, newCombId, requestData, nil, apiKey, client, w, r); err != nil {
			createLog(fmt.Sprintf("Error creating equipment request entry for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al crear la solicitud de equipo", http.StatusInternalServerError)
			return
		}
	}

	createLog(fmt.Sprintf("Request submitted successfully with combined_id %d for user %s", newCombId, userACCESSING), 0, apiKey, client, w)

	if err := sendSubmitRequestEmails(userID, requestData, created, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf(
			"Modified request %d created from previous request %d but email sending failed for user %s: %v",
			newCombId,
			combinedID,
			userACCESSING,
			err,
		), 1, apiKey, client, w)

	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})
}

// ---------------- BUDGET PERMISSIONS ----------------

func handleGetBudgetPermissions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	ips := getIsIp(userID, userACCESSING, apiKey, client, w, r)
	accessProjects := getIsProjects(userID, userACCESSING, apiKey, client, w, r)
	accessFinances := getIsFinances(userID, userACCESSING, apiKey, client, w, r)
	accessITResults := getIsIT(userID, userACCESSING, apiKey, client, w, r)
	accessResults := getIsAdmin(userID, userACCESSING, apiKey, client, w, r)
	accessGerencia := getIsGerencia(userID, userACCESSING, apiKey, client, w, r)

	response := map[string]interface{}{
		"isIP":         len(ips) > 0,
		"isProjects":   len(accessProjects) > 0,
		"isAccounting": len(accessFinances) > 0,
		"isIT":         len(accessITResults) > 0,
		"isAdmin":      len(accessResults) > 0,
		"isGerencia":   len(accessGerencia) > 0,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en getBudgetPermissions para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func getIsIp(userID int, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) []map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	ipQuery := map[string]interface{}{
		"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
		"columns":   "p.id",
		"condition": fmt.Sprintf("pp.researcher_type = 'INVESTIGADOR PRINCIPAL' AND pp.end_date >= '%s' AND p.id = %d", today, userID),
	}
	jsonIPQuery, _ := json.Marshal(ipQuery)
	resp := getReq(jsonIPQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para permisos de presupuesto para user %s", username), 1, apiKey, client, w)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener permisos de presupuesto. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil
	}

	var ips []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&ips); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en getBudgetPermissions para user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de permisos de presupuesto", http.StatusInternalServerError)
		return nil
	}
	return ips
}

func getIsProjects(userID int, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) []map[string]interface{} {
	accessQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": fmt.Sprintf("people_id = %d AND access_projects = 1", userID),
	}
	jsonAccessQuery, _ := json.Marshal(accessQuery)
	respAccess := getReq(jsonAccessQuery, apiKey, client, w)
	if respAccess == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para permisos de acceso para user %s", username), 1, apiKey, client, w)
		return nil
	}
	defer respAccess.Body.Close()

	if respAccess.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAccess.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener permisos de acceso. Status: %d, Body: %s para user %s",
			respAccess.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil
	}

	var accessProjects []map[string]interface{}
	if err := json.NewDecoder(respAccess.Body).Decode(&accessProjects); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en permisos de acceso para user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de permisos de acceso", http.StatusInternalServerError)
		return nil
	}
	return accessProjects
}

func getIsFinances(userID int, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) []map[string]interface{} {
	accessQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": fmt.Sprintf("people_id = %d AND access_finances = 1", userID),
	}
	jsonAccessQuery, _ := json.Marshal(accessQuery)
	respAccess := getReq(jsonAccessQuery, apiKey, client, w)
	if respAccess == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para permisos de acceso para user %s", username), 1, apiKey, client, w)
		return nil
	}
	defer respAccess.Body.Close()

	if respAccess.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAccess.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener permisos de acceso. Status: %d, Body: %s para user %s",
			respAccess.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil
	}

	var accessFinances []map[string]interface{}
	if err := json.NewDecoder(respAccess.Body).Decode(&accessFinances); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en permisos de acceso para user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de permisos de acceso", http.StatusInternalServerError)
		return nil
	}
	return accessFinances
}

func getIsGerencia(userID int, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) []map[string]interface{} {
	accessQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": fmt.Sprintf("people_id = %d AND access_gerencia = 1", userID),
	}
	jsonAccessQuery, _ := json.Marshal(accessQuery)
	respAccess := getReq(jsonAccessQuery, apiKey, client, w)
	if respAccess == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para permisos de acceso para user %s", username), 1, apiKey, client, w)
		return nil
	}
	defer respAccess.Body.Close()

	if respAccess.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAccess.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener permisos de acceso. Status: %d, Body: %s para user %s",
			respAccess.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil
	}

	var accessGerencia []map[string]interface{}
	if err := json.NewDecoder(respAccess.Body).Decode(&accessGerencia); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en permisos de acceso para user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de permisos de acceso", http.StatusInternalServerError)
		return nil
	}
	return accessGerencia
}

func getIsIT(userID int, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) []map[string]interface{} {
	accessITQuery := map[string]interface{}{
		"table":     "department_workers",
		"columns":   "worker_id",
		"condition": fmt.Sprintf("worker_id = %d AND department_name = 'IT' AND manager = 1", userID),
	}
	jsonAccessITQuery, _ := json.Marshal(accessITQuery)
	respAccessIT := getReq(jsonAccessITQuery, apiKey, client, w)
	if respAccessIT == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para permisos de acceso IT para user %s", username), 1, apiKey, client, w)
		return nil
	}
	defer respAccessIT.Body.Close()

	if respAccessIT.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAccessIT.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener permisos de acceso IT. Status: %d, Body: %s para user %s",
			respAccessIT.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil
	}

	var accessITResults []map[string]interface{}
	if err := json.NewDecoder(respAccessIT.Body).Decode(&accessITResults); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en permisos de acceso IT para user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de permisos de acceso IT", http.StatusInternalServerError)
		return nil
	}
	return accessITResults
}

func getIsAdmin(userID int, username string, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) []map[string]interface{} {
	accessAdminQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": fmt.Sprintf("people_id = %d AND role = 'admin'", userID),
	}
	jsonAccessAdminQuery, _ := json.Marshal(accessAdminQuery)
	respAccessAdmin := getReq(jsonAccessAdminQuery, apiKey, client, w)
	if respAccessAdmin == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para permisos de admin para user %s", username), 1, apiKey, client, w)
		return nil
	}
	defer respAccessAdmin.Body.Close()

	if respAccessAdmin.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAccessAdmin.Body)
		createLog(fmt.Sprintf("Error desde el servicio de BD al obtener permisos de admin. Status: %d, Body: %s para user %s",
			respAccessAdmin.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil
	}

	var accessResults []map[string]interface{}
	if err := json.NewDecoder(respAccessAdmin.Body).Decode(&accessResults); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en permisos de admin para user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de permisos de admin", http.StatusInternalServerError)
		return nil
	}
	return accessResults
}

// ---------------- MANAGEMENT REQUESTS ----------------

func executeManagementQuery(
	query map[string]interface{},
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
	userACCESSING string,
) ([]map[string]interface{}, bool) {
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para solicitudes de gestión para user %s", userACCESSING), 1, apiKey, client, w)
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al obtener solicitudes de gestión. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)

		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return nil, false
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en solicitudes de gestión para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de solicitudes de gestión", http.StatusInternalServerError)
		return nil, false
	}

	return rows, true
}

func handleGetManagementRequests(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	allRows := []map[string]interface{}{}

	ips := getIsIp(userID, userACCESSING, apiKey, client, w, r)
	projects := getIsProjects(userID, userACCESSING, apiKey, client, w, r)
	finances := getIsFinances(userID, userACCESSING, apiKey, client, w, r)
	it := getIsIT(userID, userACCESSING, apiKey, client, w, r)
	gerencia := getIsGerencia(userID, userACCESSING, apiKey, client, w, r)

	// query ips
	if len(ips) > 0 {

		ipQuery := map[string]interface{}{
			"table": `budget_requests br LEFT JOIN budget_parts bp ON bp.id_combined = br.id
						LEFT JOIN people p ON br.people_id = p.id
						LEFT JOIN request_files rf ON bp.id = rf.request_id`,
			"columns": strings.Join([]string{
				"br.id as combined_id",
				"br.id_intern",
				"bp.id as part_id",
				"bp.ip_id",
				"p.name",
				"p.surname",
				"bp.category_id",
				"bp.project_id",
				"bp.purpose",
				"bp.observations",
				"br.creation_date",
				"bp.travel_fromPlace",
				"bp.wherePlace",
				"bp.institution",
				"bp.fromDay",
				"bp.untilDay",
				"bp.registration_invoice",
				"bp.registration_type",
				"bp.equipment_category",
				"bp.other_price",
				"bp.luggage_type",
				"bp.luggage_weight",
				"bp.seat_preference",
				"bp.time_preference",
				"bp.travel_area",
				"rf.file_path",
			}, ", "),
			"condition": fmt.Sprintf("bp.ip_id = %d AND bp.ip_response = 0 AND canceled = 0 AND br.denied_comment IS NULL", userID),
		}

		rows, ok := executeManagementQuery(ipQuery, apiKey, client, w, userACCESSING)
		if !ok {
			return
		}

		allRows = append(allRows, rows...)
	}
	if len(projects) > 0 {
		queryProjects := map[string]interface{}{
			"table": `budget_requests br LEFT JOIN budget_parts bp ON bp.id_combined = br.id
						LEFT JOIN people p ON br.people_id = p.id
						LEFT JOIN request_files rf ON bp.id = rf.request_id
						LEFT JOIN projects pr ON bp.project_id = pr.id`,
			"columns": strings.Join([]string{
				"br.id as combined_id",
				"br.id_intern",
				"bp.id as part_id",
				"bp.ip_id",
				"p.name",
				"p.surname",
				"bp.category_id",
				"bp.project_id",
				"bp.purpose",
				"bp.observations",
				"bp.ip_response_date",
				"bp.travel_fromPlace",
				"bp.wherePlace",
				"bp.institution",
				"bp.fromDay",
				"bp.untilDay",
				"bp.registration_invoice",
				"bp.registration_type",
				"bp.equipment_category",
				"bp.other_price",
				"bp.luggage_type",
				"bp.luggage_weight",
				"bp.seat_preference",
				"bp.time_preference",
				"bp.travel_area",
				"rf.file_path",
			}, ", "),
			"condition": "bp.ip_response = 1 AND br.projects_response = 0 AND canceled = 0 AND denied_comment IS NULL AND pr.fons_romanents = 0",
		}

		rows, ok := executeManagementQuery(queryProjects, apiKey, client, w, userACCESSING)
		if !ok {
			return
		}

		allRows = append(allRows, rows...)
	}
	if len(gerencia) > 0 {
		gerenciaQuery := map[string]interface{}{
			"table": `budget_requests br LEFT JOIN budget_parts bp ON bp.id_combined = br.id
						LEFT JOIN people p ON br.people_id = p.id
						LEFT JOIN request_files rf ON bp.id = rf.request_id
						LEFT JOIN projects pr ON bp.project_id = pr.id`,
			"columns": strings.Join([]string{
				"br.id as combined_id",
				"br.id_intern",
				"bp.id as part_id",
				"bp.ip_id",
				"bp.ip_response_date",
				"p.name",
				"p.surname",
				"bp.category_id",
				"bp.project_id",
				"bp.purpose",
				"bp.observations",
				"br.creation_date",
				"bp.travel_fromPlace",
				"bp.wherePlace",
				"bp.institution",
				"bp.fromDay",
				"bp.untilDay",
				"bp.registration_invoice",
				"bp.registration_type",
				"bp.equipment_category",
				"bp.other_price",
				"bp.luggage_type",
				"bp.luggage_weight",
				"bp.seat_preference",
				"bp.time_preference",
				"bp.travel_area",
				"rf.file_path",
			}, ", "),
			"condition": fmt.Sprintf(`
				(
					bp.ip_id = %d 
					AND bp.ip_response = 0 
					AND br.canceled = 0 
					AND br.denied_comment IS NULL
				)
				OR
				(
					bp.ip_response = 1 
					AND br.projects_response = 0 
					AND br.canceled = 0 
					AND br.denied_comment IS NULL 
					AND pr.fons_romanents = 1
				)
			`, userID),
		}

		rows, ok := executeManagementQuery(gerenciaQuery, apiKey, client, w, userACCESSING)
		if !ok {
			return
		}

		allRows = append(allRows, rows...)
	}
	if len(finances) > 0 {
		queryFinances := map[string]interface{}{
			"table": `budget_requests br LEFT JOIN budget_parts bp ON bp.id_combined = br.id
						LEFT JOIN people p ON br.people_id = p.id
						LEFT JOIN request_files rf ON bp.id = rf.request_id`,
			"columns": strings.Join([]string{
				"br.id as combined_id",
				"br.id_intern",
				"bp.id as part_id",
				"bp.ip_id",
				"p.name",
				"p.surname",
				"bp.category_id",
				"bp.project_id",
				"bp.purpose",
				"bp.observations",
				"br.projects_response_date",
				"bp.travel_fromPlace",
				"bp.wherePlace",
				"bp.fromDay",
				"bp.untilDay",
				"bp.registration_invoice",
				"bp.registration_type",
				"bp.equipment_category",
				"bp.other_price",
				"bp.institution",
				"bp.luggage_type",
				"bp.luggage_weight",
				"bp.seat_preference",
				"bp.time_preference",
				"bp.travel_area",
				"rf.file_path",
			}, ", "),
			"condition": "bp.ip_response = 1 AND br.projects_response = 1 AND br.acc_or_it_response = 0 AND canceled = 0 AND denied_comment IS NULL AND (bp.category_id = 1 OR bp.category_id = 2 OR bp.category_id = 3 OR bp.category_id = 5)",
		}

		rows, ok := executeManagementQuery(queryFinances, apiKey, client, w, userACCESSING)
		if !ok {
			return
		}

		allRows = append(allRows, rows...)
	}
	if len(it) > 0 {
		queryIT := map[string]interface{}{
			"table": `budget_requests br LEFT JOIN budget_parts bp ON bp.id_combined = br.id
						LEFT JOIN people p ON br.people_id = p.id
						LEFT JOIN request_files rf ON bp.id = rf.request_id`,
			"columns": strings.Join([]string{
				"br.id as combined_id",
				"br.id_intern",
				"bp.id as part_id",
				"bp.ip_id",
				"p.name",
				"p.surname",
				"bp.category_id",
				"bp.project_id",
				"bp.purpose",
				"bp.observations",
				"br.projects_response_date",
				"bp.travel_fromPlace",
				"bp.wherePlace",
				"bp.fromDay",
				"bp.untilDay",
				"bp.registration_invoice",
				"bp.registration_type",
				"bp.equipment_category",
				"bp.other_price",
				"bp.institution",
				"bp.luggage_type",
				"bp.luggage_weight",
				"bp.seat_preference",
				"bp.time_preference",
				"bp.travel_area",
				"rf.file_path",
			}, ", "),
			"condition": "bp.ip_response = 1 AND br.projects_response = 1 AND br.acc_or_it_response = 0 AND canceled = 0 AND denied_comment IS NULL AND bp.category_id = 4",
		}

		rows, ok := executeManagementQuery(queryIT, apiKey, client, w, userACCESSING)
		if !ok {
			return
		}

		allRows = append(allRows, rows...)
	}
	if len(ips)+len(projects)+len(finances)+len(it)+len(gerencia) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		return
	}

	if len(allRows) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		return
	}

	grouped := make(map[string]map[string]interface{})

	for _, row := range allRows {
		combinedID := fmt.Sprintf("%v", row["combined_id"])
		if combinedID == "" || combinedID == "<nil>" {
			continue
		}

		partID := fmt.Sprintf("%v", row["part_id"])
		if partID == "" || partID == "<nil>" {
			continue
		}

		if _, exists := grouped[combinedID]; !exists {
			grouped[combinedID] = map[string]interface{}{
				"combined_id":             row["combined_id"],
				"id_intern":               row["id_intern"],
				"creation_date":           row["creation_date"],
				"name":                    unescapeComma(asString(row["name"])),
				"surname":                 unescapeComma(asString(row["surname"])),
				"projects_response":       row["projects_response"],
				"projects_response_date":  row["projects_response_date"],
				"acc_or_it_response":      row["acc_or_it_response"],
				"acc_or_it_response_date": row["acc_or_it_response_date"],
				"denied_comment":          unescapeComma(asString(row["denied_comment"])),
				"parts":                   []map[string]interface{}{},
			}
		}

		parts := grouped[combinedID]["parts"].([]map[string]interface{})

		partIndex := -1
		for i, existingPart := range parts {
			if fmt.Sprintf("%v", existingPart["id"]) == partID {
				partIndex = i
				break
			}
		}

		if partIndex == -1 {
			newPart := map[string]interface{}{
				"id":                   row["part_id"],
				"category_id":          row["category_id"],
				"project_id":           row["project_id"],
				"purpose":              unescapeComma(asString(row["purpose"])),
				"observations":         unescapeComma(asString(row["observations"])),
				"ip_id":                row["ip_id"],
				"ip_response":          row["ip_response"],
				"ip_response_date":     row["ip_response_date"],
				"wherePlace":           unescapeComma(asString(row["wherePlace"])),
				"travel_fromPlace":     unescapeComma(asString(row["travel_fromPlace"])),
				"institution":          unescapeComma(asString(row["institution"])),
				"fromDay":              row["fromDay"],
				"untilDay":             row["untilDay"],
				"registration_invoice": row["registration_invoice"],
				"registration_type":    row["registration_type"],
				"equipment_category":   row["equipment_category"],
				"other_price":          row["other_price"],
				"luggage_type":         row["luggage_type"],
				"luggage_weight":       row["luggage_weight"],
				"seat_preference":      row["seat_preference"],
				"time_preference":      row["time_preference"],
				"travel_area":          row["travel_area"],
				"file_paths":           []string{},
			}

			if row["file_path"] != nil &&
				fmt.Sprintf("%v", row["file_path"]) != "<nil>" &&
				fmt.Sprintf("%v", row["file_path"]) != "" {
				newPart["file_paths"] = append(
					newPart["file_paths"].([]string),
					fmt.Sprintf("%v", row["file_path"]),
				)
			}

			parts = append(parts, newPart)
		} else {
			if row["file_path"] != nil &&
				fmt.Sprintf("%v", row["file_path"]) != "<nil>" &&
				fmt.Sprintf("%v", row["file_path"]) != "" {
				currentPaths := parts[partIndex]["file_paths"].([]string)
				newPath := fmt.Sprintf("%v", row["file_path"])

				exists := false
				for _, p := range currentPaths {
					if p == newPath {
						exists = true
						break
					}
				}

				if !exists {
					parts[partIndex]["file_paths"] = append(currentPaths, newPath)
				}
			}
		}

		grouped[combinedID]["parts"] = parts
	}

	result := make([]map[string]interface{}, 0, len(grouped))
	for _, req := range grouped {
		result = append(result, req)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en solicitudes de gestión para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

// ---------------- ALL REQUESTS ----------------

func handleGetAllRequests(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	accessProjects := getIsProjects(userID, userACCESSING, apiKey, client, w, r)
	accessGerencia := getIsGerencia(userID, userACCESSING, apiKey, client, w, r)
	accessAdmin := getIsAdmin(userID, userACCESSING, apiKey, client, w, r)
	canViewAttendanceCertificates := len(accessAdmin) > 0 || (len(accessProjects) > 0 && len(accessGerencia) > 0)
	canTrackAttendanceCertificates := len(accessProjects) > 0 || len(accessAdmin) > 0

	var resp *http.Response

	attendancePathColumn := "'' AS attendance_certificate_path"
	if canViewAttendanceCertificates {
		attendancePathColumn = "bac.file_path AS attendance_certificate_path"
	}

	query := map[string]interface{}{
		"table": fmt.Sprintf("budget_requests br LEFT JOIN budget_parts bp ON bp.id_combined = br.id LEFT JOIN people p ON br.people_id = p.id LEFT JOIN request_files rf ON bp.id = rf.request_id LEFT JOIN people psup ON bp.ip_id = psup.id LEFT JOIN budget_attendance_certificates bac ON bac.id_combined = br.id LEFT JOIN budget_attendance_certificate_views bacv ON bacv.id_combined = br.id AND bacv.people_id = %d", userID),
		"columns": strings.Join([]string{
			"br.id as combined_id",
			"br.id_intern",
			"br.creation_date",
			"br.projects_response",
			"br.projects_response_date",
			"br.acc_or_it_response",
			"br.acc_or_it_response_date",
			"br.denied_comment",
			"br.canceled",
			"br.cancelation_motive",
			"br.prev_request",
			"bp.id as part_id",
			"bp.ip_id",
			"psup.name AS ip_name",
			"psup.surname AS ip_surname",
			"p.name",
			"p.surname",
			"bp.category_id",
			"bp.ip_response",
			"bp.ip_response_date",
			"bp.project_id",
			"bp.purpose",
			"bp.observations",
			"bp.travel_fromPlace",
			"bp.wherePlace",
			"bp.fromDay",
			"bp.untilDay",
			"bp.registration_invoice",
			"bp.registration_type",
			"bp.equipment_category",
			"bp.institution",
			"bp.other_price",
			"bp.luggage_type",
			"bp.luggage_weight",
			"bp.seat_preference",
			"bp.time_preference",
			"bp.travel_area",
			"rf.file_path",
			"bac.file_uploaded AS attendance_certificate_uploaded",
			attendancePathColumn,
			"bac.uploaded_at AS attendance_certificate_uploaded_at",
			"CASE WHEN bacv.id IS NULL THEN 0 ELSE 1 END AS attendance_certificate_viewed",
		}, ", "),
		"condition": "1=1",
	}
	jsonQuery, _ := json.Marshal(query)
	//fmt.Println("Query: ", string(jsonQuery))

	resp = getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para todas las solicitudes para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al obtener todas las solicitudes. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en todas las solicitudes para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de solicitudes", http.StatusInternalServerError)
		return
	}

	//fmt.Printf("Received %d rows for all requests for user %s\n", len(rows), userACCESSING)

	// grouped[combinedID] = request
	grouped := make(map[string]map[string]interface{})

	for _, row := range rows {
		combinedID := fmt.Sprintf("%v", row["combined_id"])
		if combinedID == "" || combinedID == "<nil>" {
			continue
		}

		partID := fmt.Sprintf("%v", row["part_id"])
		if partID == "" || partID == "<nil>" {
			continue
		}

		if _, exists := grouped[combinedID]; !exists {
			grouped[combinedID] = map[string]interface{}{
				"combined_id":                        row["combined_id"],
				"id_intern":                          row["id_intern"],
				"creation_date":                      row["creation_date"],
				"name":                               unescapeComma(asString(row["name"])),
				"surname":                            unescapeComma(asString(row["surname"])),
				"projects_response":                  row["projects_response"],
				"projects_response_date":             row["projects_response_date"],
				"acc_or_it_response":                 row["acc_or_it_response"],
				"acc_or_it_response_date":            row["acc_or_it_response_date"],
				"denied_comment":                     unescapeComma(asString(row["denied_comment"])),
				"canceled":                           row["canceled"],
				"cancelation_motive":                 unescapeComma(asString(row["cancelation_motive"])),
				"prev_request":                       row["prev_request"],
				"attendance_certificate_uploaded":    row["attendance_certificate_uploaded"],
				"attendance_certificate_path":        row["attendance_certificate_path"],
				"attendance_certificate_uploaded_at": row["attendance_certificate_uploaded_at"],
				"attendance_certificate_viewed":      row["attendance_certificate_viewed"],
				"can_view_attendance_certificate":    canViewAttendanceCertificates,
				"can_track_attendance_certificate":   canTrackAttendanceCertificates,
				"parts":                              []map[string]interface{}{},
			}
		}

		parts := grouped[combinedID]["parts"].([]map[string]interface{})

		// Buscar si esta part ya existe
		partIndex := -1
		for i, existingPart := range parts {
			if fmt.Sprintf("%v", existingPart["id"]) == partID {
				partIndex = i
				break
			}
		}

		// Si no existe, crearla
		if partIndex == -1 {
			newPart := map[string]interface{}{
				"id":                   row["part_id"],
				"category_id":          row["category_id"],
				"project_id":           row["project_id"],
				"purpose":              unescapeComma(asString(row["purpose"])),
				"observations":         unescapeComma(asString(row["observations"])),
				"ip_id":                row["ip_id"],
				"ip_name":              unescapeComma(asString(row["ip_name"])),
				"ip_surname":           unescapeComma(asString(row["ip_surname"])),
				"ip_response":          row["ip_response"],
				"ip_response_date":     row["ip_response_date"],
				"wherePlace":           unescapeComma(asString(row["wherePlace"])),
				"travel_fromPlace":     unescapeComma(asString(row["travel_fromPlace"])),
				"institution":          unescapeComma(asString(row["institution"])),
				"fromDay":              row["fromDay"],
				"untilDay":             row["untilDay"],
				"registration_invoice": row["registration_invoice"],
				"registration_type":    row["registration_type"],
				"equipment_category":   row["equipment_category"],
				"other_price":          row["other_price"],
				"luggage_type":         row["luggage_type"],
				"luggage_weight":       row["luggage_weight"],
				"seat_preference":      row["seat_preference"],
				"time_preference":      row["time_preference"],
				"file_paths":           []string{},
				"travel_area":          row["travel_area"],
			}

			if row["file_path"] != nil && fmt.Sprintf("%v", row["file_path"]) != "<nil>" && fmt.Sprintf("%v", row["file_path"]) != "" {
				newPart["file_paths"] = append(newPart["file_paths"].([]string), fmt.Sprintf("%v", row["file_path"]))
			}

			parts = append(parts, newPart)
		} else {
			// Ya existe: solo añadir file_path si viene uno nuevo
			if row["file_path"] != nil && fmt.Sprintf("%v", row["file_path"]) != "<nil>" && fmt.Sprintf("%v", row["file_path"]) != "" {
				currentPaths := parts[partIndex]["file_paths"].([]string)
				newPath := fmt.Sprintf("%v", row["file_path"])

				exists := false
				for _, p := range currentPaths {
					if p == newPath {
						exists = true
						break
					}
				}

				if !exists {
					parts[partIndex]["file_paths"] = append(currentPaths, newPath)
				}
			}
		}

		grouped[combinedID]["parts"] = parts
	}

	result := make([]map[string]interface{}, 0, len(grouped))
	for _, req := range grouped {
		result = append(result, req)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		createLog(fmt.Sprintf("Error al codificar respuesta JSON en todas las solicitudes para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func handleMarkAttendanceCertificateViewed(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	accessProjects := getIsProjects(userID, username, apiKey, client, w, r)
	accessGerencia := getIsGerencia(userID, username, apiKey, client, w, r)
	accessAdmin := getIsAdmin(userID, username, apiKey, client, w, r)
	if len(accessAdmin) == 0 && !(len(accessProjects) > 0 && len(accessGerencia) > 0) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		CombinedID int `json:"combinedId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.CombinedID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	existingQuery := map[string]interface{}{
		"table":     "budget_attendance_certificate_views",
		"columns":   "id",
		"condition": fmt.Sprintf("id_combined = %d AND people_id = %d", payload.CombinedID, userID),
	}
	jsonExistingQuery, _ := json.Marshal(existingQuery)
	respExisting := getReq(jsonExistingQuery, apiKey, client, w)
	if respExisting == nil {
		http.Error(w, "Could not mark attendance certificate as viewed", http.StatusInternalServerError)
		return
	}
	defer respExisting.Body.Close()

	if respExisting.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respExisting.Body)
		createLog(fmt.Sprintf("Error checking attendance certificate view for request %d. Status: %d Body: %s", payload.CombinedID, respExisting.StatusCode, string(body)), 1, apiKey, client, w)
		http.Error(w, "Could not mark attendance certificate as viewed", http.StatusInternalServerError)
		return
	}

	var existingRows []map[string]interface{}
	if err := json.NewDecoder(respExisting.Body).Decode(&existingRows); err != nil {
		http.Error(w, "Could not mark attendance certificate as viewed", http.StatusInternalServerError)
		return
	}

	if len(existingRows) == 0 {
		now := time.Now().Format("2006-01-02 15:04")
		insert := map[string]interface{}{
			"table":   "budget_attendance_certificate_views",
			"columns": "id_combined, people_id, viewed_at",
			"value":   fmt.Sprintf("%d, %d, %s", payload.CombinedID, userID, now),
		}
		jsonInsert, _ := json.Marshal(insert)
		respInsert := postReq(jsonInsert, apiKey, client, w)
		if respInsert == nil {
			http.Error(w, "Could not mark attendance certificate as viewed", http.StatusInternalServerError)
			return
		}
		defer respInsert.Body.Close()

		if respInsert.StatusCode < http.StatusOK || respInsert.StatusCode >= http.StatusMultipleChoices {
			body, _ := io.ReadAll(respInsert.Body)
			createLog(fmt.Sprintf("Error inserting attendance certificate view for request %d. Status: %d Body: %s", payload.CombinedID, respInsert.StatusCode, string(body)), 1, apiKey, client, w)
			http.Error(w, "Could not mark attendance certificate as viewed", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// ---------------- EXCEL EXPORTS ----------------

func handleExportAllRequestsExcel(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	var resp *http.Response

	// 1 fila por cada budget_part, repitiendo los datos del budget_request
	// Ajusta "request_prices" si tu tabla real de rangos tiene otro nombre.
	query := map[string]interface{}{
		"table": strings.Join([]string{
			"budget_requests br",
			"LEFT JOIN budget_parts bp ON bp.id_combined = br.id",
			"LEFT JOIN people worker ON br.people_id = worker.id",
			"LEFT JOIN people ip_person ON bp.ip_id = ip_person.id",
			"LEFT JOIN request_categories rc ON bp.category_id = rc.id",
			"LEFT JOIN request_prices rp ON bp.other_price = rp.id",
			"LEFT JOIN projects p on p.id = bp.project_id",
		}, " "),
		"columns": strings.Join([]string{
			"br.id AS combined_id",
			"br.id_intern",
			"br.creation_date",
			"br.projects_response",
			"br.projects_response_date",
			"br.acc_or_it_response",
			"br.acc_or_it_response_date",
			"br.denied_comment",
			"br.prev_request",
			"br.canceled",
			"br.cancelation_motive",

			"worker.name AS worker_name",
			"worker.surname AS worker_surname",

			"bp.id AS part_id",
			"p.short_name AS project_id",
			"p.short_name AS project_name",
			"bp.purpose",
			"bp.observations",
			"bp.ip_response",
			"bp.ip_response_date",
			"bp.travel_fromPlace",
			"bp.travel_area",
			"bp.wherePlace",
			"bp.institution",
			"bp.fromDay",
			"bp.untilDay",
			"bp.registration_invoice",
			"bp.registration_type",
			"bp.equipment_category",
			"bp.luggage_type",
			"bp.luggage_weight",
			"bp.seat_preference",
			"bp.time_preference",

			"ip_person.name AS ip_name",
			"ip_person.surname AS ip_surname",

			"rc.category AS category_name",
			"rp.price_range AS other_price_range",
		}, ", "),
		"condition": "1=1",
		"order":     "br.id DESC, bp.id ASC",
	}

	jsonQuery, _ := json.Marshal(query)
	//fmt.Println("Export query:", string(jsonQuery))

	resp = getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para exportar todas las solicitudes para user %s", userACCESSING), 1, apiKey, client, w)
		http.Error(w, "No se pudo obtener la información para exportar", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al exportar todas las solicitudes. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en exportación para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de solicitudes", http.StatusInternalServerError)
		return
	}

	//fmt.Printf("Received %d rows for Excel export for user %s\n", len(rows), userACCESSING)

	// Crear Excel
	f := excelize.NewFile()
	sheetName := "All Requests"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{
		"Request ID",
		"Internal ID",
		"Creation date",
		"Worker",
		"Responsible",
		"Responsible response",
		"Responsible response date",
		"Projects response",
		"Projects response date",
		"Accounting/IT response",
		"Accounting/IT response date",
		"Denied comment",
		"Cancelled",
		"Cancelation motive",
		"Previous request",

		"Part ID",
		"Category",
		"Project ID",
		"Project Name",

		"Purpose",
		"Observations",

		"Travel from",
		"Travel area",
		"Travel to",
		"Institution",
		"From day",
		"Until day",
		"Luggage type",
		"Luggage weight",
		"Seat preference",
		"Time preference",

		"Registration invoice",
		"Registration type",

		"Equipment category",
		"Other price range",
	}

	// Cabecera
	for colIdx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D9EAF7"},
			Pattern: 1,
		},
	})

	lastHeaderCell, _ := excelize.CoordinatesToCellName(len(headers), 1)
	f.SetCellStyle(sheetName, "A1", lastHeaderCell, headerStyle)

	// Filas
	rowNum := 2
	for _, row := range rows {
		workerFullName := strings.TrimSpace(fmt.Sprintf("%s %s",
			asString(row["worker_name"]),
			asString(row["worker_surname"]),
		))

		ipFullName := strings.TrimSpace(fmt.Sprintf("%s %s",
			asString(row["ip_name"]),
			asString(row["ip_surname"]),
		))

		luggageWeight := asInt(row["luggage_weight"])
		luggageWeightStr := ""
		if luggageWeight > 0 {
			luggageWeightStr = fmt.Sprintf("%d kg", luggageWeight)
		}

		if asString(row["registration_invoice"]) == "1" && asString(row["registration_type"]) == "I" {
			row["registration_type"] = "Invoice"
		} else if asString(row["registration_invoice"]) == "1" && asString(row["registration_type"]) == "D" {
			row["registration_type"] = "Payment Document"
		} else {
			row["registration_type"] = ""
		}

		values := []interface{}{
			asString(row["combined_id"]),
			asString(row["id_intern"]),
			formatDateTimeNoSeconds(asString(row["creation_date"])),
			unescapeComma(workerFullName),
			unescapeComma(ipFullName),
			parseBoolean(asString(row["ip_response"])),
			formatDateTimeNoSeconds(asString(row["ip_response_date"])),
			parseBoolean(asString(row["projects_response"])),
			formatDateTimeNoSeconds(asString(row["projects_response_date"])),
			parseBoolean(asString(row["acc_or_it_response"])),
			formatDateTimeNoSeconds(asString(row["acc_or_it_response_date"])),
			unescapeComma(asString(row["denied_comment"])),
			parseBoolean(asString(row["canceled"])),
			unescapeComma(asString(row["cancelation_motive"])),
			asString(row["prev_request"]),
			asString(row["part_id"]),
			asString(row["category_name"]),
			asString(row["project_id"]),
			unescapeComma(asString(row["project_name"])),

			unescapeComma(asString(row["purpose"])),
			unescapeComma(asString(row["observations"])),

			unescapeComma(asString(row["travel_fromPlace"])),
			parseTravelArea(asString(row["travel_area"])),
			unescapeComma(asString(row["wherePlace"])),
			unescapeComma(asString(row["institution"])),
			normalizeDateString(asString(row["fromDay"])),
			normalizeDateString(asString(row["untilDay"])),

			parseLuggageType(asString(row["luggage_type"])),
			luggageWeightStr,
			parseSeatPreference(asString(row["seat_preference"])),
			parseTimePreference(asString(row["time_preference"])),

			parseBoolean(asString(row["registration_invoice"])),
			asString(row["registration_type"]),

			asString(row["equipment_category"]),
			asString(row["other_price_range"]),
		}

		for colIdx, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNum)
			f.SetCellValue(sheetName, cell, value)
		}

		rowNum++
	}

	_ = f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		createLog(fmt.Sprintf("Error al generar el Excel para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al generar el Excel", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;`))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func handleExportFilteredRequestsExcel(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	conditions := []string{"1=1"}

	// ---------------- FILTERS FROM QUERY PARAMS ----------------

	idIntern := strings.TrimSpace(r.URL.Query().Get("idIntern"))
	creationFrom := strings.TrimSpace(r.URL.Query().Get("creationFrom"))
	creationTo := strings.TrimSpace(r.URL.Query().Get("creationTo"))

	categories := r.URL.Query()["category"]
	travelAreas := r.URL.Query()["travelArea"]
	projectIDs := r.URL.Query()["projectId"]
	responsibleIDs := r.URL.Query()["responsibleId"]

	if idIntern != "" {
		conditions = append(
			conditions,
			fmt.Sprintf("br.id_intern LIKE '%%%s%%'", escapeSQLLike(idIntern)),
		)
	}

	if creationFrom != "" {
		conditions = append(
			conditions,
			fmt.Sprintf("date(br.creation_date) >= '%s'", escapeSQLValue(creationFrom)),
		)
	}

	if creationTo != "" {
		conditions = append(
			conditions,
			fmt.Sprintf("date(br.creation_date) <= '%s'", escapeSQLValue(creationTo)),
		)
	}

	categoryIDs := mapCategoriesToIDs(categories)
	if len(categoryIDs) > 0 {
		conditions = append(
			conditions,
			fmt.Sprintf("bp.category_id IN (%s)", strings.Join(categoryIDs, ",")),
		)
	}

	shouldApplyTravelAreaFilter := len(travelAreas) > 0 &&
		(len(categories) == 0 || containsString(categories, "travel"))

	if shouldApplyTravelAreaFilter {
		quotedTravelAreas := quoteSQLStringList(travelAreas)
		if len(quotedTravelAreas) > 0 {
			conditions = append(
				conditions,
				fmt.Sprintf("bp.travel_area IN (%s)", strings.Join(quotedTravelAreas, ",")),
			)
		}
	}

	if len(projectIDs) > 0 {
		quotedprojectIDs := quoteSQLStringList(projectIDs)
		conditions = append(
			conditions,
			fmt.Sprintf("bp.project_id IN (%s)", strings.Join(quotedprojectIDs, ",")),
		)
	}

	cleanResponsibleIDs := cleanNumericList(responsibleIDs)
	if len(cleanResponsibleIDs) > 0 {
		conditions = append(
			conditions,
			fmt.Sprintf("bp.ip_id IN (%s)", strings.Join(cleanResponsibleIDs, ",")),
		)
	}

	condition := strings.Join(conditions, " AND ")

	fmt.Printf("Constructed SQL condition for export: %s\n", condition)

	query := map[string]interface{}{
		"table": strings.Join([]string{
			"budget_requests br",
			"LEFT JOIN budget_parts bp ON bp.id_combined = br.id",
			"LEFT JOIN people worker ON br.people_id = worker.id",
			"LEFT JOIN people ip_person ON bp.ip_id = ip_person.id",
			"LEFT JOIN request_categories rc ON bp.category_id = rc.id",
			"LEFT JOIN request_prices rp ON bp.other_price = rp.id",
			"LEFT JOIN projects p on p.id = bp.project_id",
		}, " "),
		"columns": strings.Join([]string{
			"br.id AS combined_id",
			"br.id_intern",
			"br.creation_date",
			"br.projects_response",
			"br.projects_response_date",
			"br.acc_or_it_response",
			"br.acc_or_it_response_date",
			"br.denied_comment",
			"br.prev_request",
			"br.canceled",
			"br.cancelation_motive",

			"worker.name AS worker_name",
			"worker.surname AS worker_surname",

			"bp.id AS part_id",
			"p.short_name AS project_id",
			"p.short_name AS project_name",
			"bp.purpose",
			"bp.observations",
			"bp.ip_response",
			"bp.ip_response_date",
			"bp.travel_fromPlace",
			"bp.travel_area",
			"bp.wherePlace",
			"bp.institution",
			"bp.fromDay",
			"bp.untilDay",
			"bp.registration_invoice",
			"bp.registration_type",
			"bp.equipment_category",
			"bp.luggage_type",
			"bp.luggage_weight",
			"bp.seat_preference",
			"bp.time_preference",

			"ip_person.name AS ip_name",
			"ip_person.surname AS ip_surname",

			"rc.category AS category_name",
			"rp.price_range AS other_price_range",
		}, ", "),
		"condition": condition,
		"order":     "br.id DESC, bp.id ASC",
	}

	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para exportar solicitudes filtradas para user %s", userACCESSING), 1, apiKey, client, w)
		http.Error(w, "No se pudo obtener la información para exportar", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al exportar solicitudes filtradas. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		createLog(fmt.Sprintf("Error al decodificar respuesta de BD en exportación filtrada para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de solicitudes", http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	sheetName := "Filtered Requests"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{
		"Request ID",
		"Internal ID",
		"Creation date",
		"Worker",
		"Responsible",
		"Responsible response",
		"Responsible response date",
		"Projects response",
		"Projects response date",
		"Accounting/IT response",
		"Accounting/IT response date",
		"Denied comment",
		"Cancelled",
		"Cancelation motive",
		"Previous request",

		"Part ID",
		"Category",
		"Project ID",
		"Project Name",

		"Purpose",
		"Observations",

		"Travel from",
		"Travel area",
		"Travel to",
		"Institution",
		"From day",
		"Until day",
		"Luggage type",
		"Luggage weight",
		"Seat preference",
		"Time preference",

		"Registration invoice",
		"Registration type",

		"Equipment category",
		"Other price range",
	}

	for colIdx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D9EAF7"},
			Pattern: 1,
		},
	})

	lastHeaderCell, _ := excelize.CoordinatesToCellName(len(headers), 1)
	f.SetCellStyle(sheetName, "A1", lastHeaderCell, headerStyle)

	rowNum := 2
	for _, row := range rows {
		workerFullName := strings.TrimSpace(fmt.Sprintf("%s %s",
			asString(row["worker_name"]),
			asString(row["worker_surname"]),
		))

		ipFullName := strings.TrimSpace(fmt.Sprintf("%s %s",
			asString(row["ip_name"]),
			asString(row["ip_surname"]),
		))

		luggageWeight := asInt(row["luggage_weight"])
		luggageWeightStr := ""
		if luggageWeight > 0 {
			luggageWeightStr = fmt.Sprintf("%d kg", luggageWeight)
		}

		registrationType := ""
		if asString(row["registration_invoice"]) == "1" && asString(row["registration_type"]) == "I" {
			registrationType = "Invoice"
		} else if asString(row["registration_invoice"]) == "1" && asString(row["registration_type"]) == "D" {
			registrationType = "Payment Document"
		}

		values := []interface{}{
			asString(row["combined_id"]),
			asString(row["id_intern"]),
			formatDateTimeNoSeconds(asString(row["creation_date"])),
			unescapeComma(workerFullName),
			unescapeComma(ipFullName),
			parseBoolean(asString(row["ip_response"])),
			formatDateTimeNoSeconds(asString(row["ip_response_date"])),
			parseBoolean(asString(row["projects_response"])),
			formatDateTimeNoSeconds(asString(row["projects_response_date"])),
			parseBoolean(asString(row["acc_or_it_response"])),
			formatDateTimeNoSeconds(asString(row["acc_or_it_response_date"])),
			unescapeComma(asString(row["denied_comment"])),
			parseBoolean(asString(row["canceled"])),
			unescapeComma(asString(row["cancelation_motive"])),
			asString(row["prev_request"]),

			asString(row["part_id"]),
			asString(row["category_name"]),
			asString(row["project_id"]),
			unescapeComma(asString(row["project_name"])),

			unescapeComma(asString(row["purpose"])),
			unescapeComma(asString(row["observations"])),

			unescapeComma(asString(row["travel_fromPlace"])),
			parseTravelArea(asString(row["travel_area"])),
			unescapeComma(asString(row["wherePlace"])),
			unescapeComma(asString(row["institution"])),
			normalizeDateString(asString(row["fromDay"])),
			normalizeDateString(asString(row["untilDay"])),

			parseLuggageType(asString(row["luggage_type"])),
			luggageWeightStr,
			parseSeatPreference(asString(row["seat_preference"])),
			parseTimePreference(asString(row["time_preference"])),

			parseBoolean(asString(row["registration_invoice"])),
			registrationType,

			asString(row["equipment_category"]),
			asString(row["other_price_range"]),
		}

		for colIdx, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNum)
			f.SetCellValue(sheetName, cell, value)
		}

		rowNum++
	}

	_ = f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		createLog(fmt.Sprintf("Error al generar el Excel filtrado para user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al generar el Excel", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="filtered_requests.xlsx"`)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// ---------------- ACCEPT / REJECT FLOWS ----------------

func handleRejectRequest(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	ips := getIsIp(userID, username, apiKey, client, w, r)
	accessProjects := getIsProjects(userID, username, apiKey, client, w, r)
	accessFinances := getIsFinances(userID, username, apiKey, client, w, r)
	accessITResults := getIsIT(userID, username, apiKey, client, w, r)
	accessResults := getIsAdmin(userID, username, apiKey, client, w, r)
	accessGerencia := getIsGerencia(userID, username, apiKey, client, w, r)

	if len(ips) == 0 &&
		len(accessProjects) == 0 &&
		len(accessFinances) == 0 &&
		len(accessITResults) == 0 &&
		len(accessResults) == 0 &&
		len(accessGerencia) == 0 {
		http.Error(w, "No tienes permisos para realizar esta acción", http.StatusForbidden)
		return
	}

	type RejectRequestBody struct {
		CombinedID string `json:"idCombined"`
		Motive     string `json:"motive"`
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var body RejectRequestBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.CombinedID == "" {
		http.Error(w, "combined_id is required", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(body.CombinedID)
	if err != nil {
		http.Error(w, "combined_id no es válido", http.StatusBadRequest)
		return
	}

	getQuery := map[string]interface{}{
		"table": `budget_requests br 
			LEFT JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN projects pr ON bp.project_id = pr.id`,
		"columns": strings.Join([]string{
			"bp.*",
			"br.projects_response",
			"br.projects_response_date",
			"br.acc_or_it_response",
			"br.acc_or_it_response_date",
			"br.denied_comment",
			"br.canceled",
			"pr.fons_romanents",
		}, ", "),
		"condition": fmt.Sprintf("br.id = %d", combinedID),
	}

	jsonGetQuery, _ := json.Marshal(getQuery)
	resp := getReq(jsonGetQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta al cargar request %d para rechazo por user %s", combinedID, username), 1, apiKey, client, w)
		http.Error(w, "Error cargando la solicitud", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyResp, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error cargando request %d para rechazo. Status: %d, Body: %s, user: %s",
			combinedID,
			resp.StatusCode,
			string(bodyResp),
			username,
		), 1, apiKey, client, w)
		http.Error(w, "Error cargando la solicitud", http.StatusInternalServerError)
		return
	}

	var existingRows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&existingRows); err != nil {
		createLog(fmt.Sprintf("Error decoding existing request %d for reject by user %s: %v", combinedID, username, err), 1, apiKey, client, w)
		http.Error(w, "Error procesando la solicitud", http.StatusInternalServerError)
		return
	}

	hasFonsRomanents := requestHasFonsRomanents(existingRows)
	hasEquipment := requestHasEquipment(existingRows)
	projectsResponse := getRequestProjectsResponse(existingRows)
	accOrITResponse := getRequestAccOrItResponse(existingRows)

	// Si el usuario está guardado como ip_id en una parte pendiente,
	// actúa como IP. Aquí entra tanto un IP real como Gerencia asignada como IP fallback.
	rejectingAsIP := hasPendingIPPartAssignedToUser(existingRows, userID)

	// Si ya no actúa como IP y hay fons romanents, Gerencia actúa como Projects.
	rejectingAsGerenciaProjects :=
		!rejectingAsIP &&
			len(accessGerencia) > 0 &&
			hasFonsRomanents &&
			projectsResponse == 0

	rejectingAsProjects :=
		!rejectingAsIP &&
			!rejectingAsGerenciaProjects &&
			len(accessProjects) > 0 &&
			projectsResponse == 0

	rejectingAsFinances :=
		!rejectingAsIP &&
			!rejectingAsGerenciaProjects &&
			!rejectingAsProjects &&
			len(accessFinances) > 0 &&
			projectsResponse == 1 &&
			accOrITResponse == 0 &&
			!hasEquipment

	rejectingAsIT :=
		!rejectingAsIP &&
			!rejectingAsGerenciaProjects &&
			!rejectingAsProjects &&
			len(accessITResults) > 0 &&
			projectsResponse == 1 &&
			accOrITResponse == 0 &&
			hasEquipment

	switch {
	case rejectingAsIP:
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		rejectionFlowIP(userID, apiKey, client, w, r, username)
		return

	case rejectingAsGerenciaProjects:
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		rejectionFlowByDepartment(userID, apiKey, client, w, r, username, "projects", "gerencia")
		return

	case rejectingAsProjects:
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		rejectionFlowByDepartment(userID, apiKey, client, w, r, username, "projects", "projectes")
		return

	case rejectingAsFinances:
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		rejectionFlowByDepartment(userID, apiKey, client, w, r, username, "acc_or_it", "comptabilitat")
		return
	case rejectingAsIT:
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		rejectionFlowByDepartment(userID, apiKey, client, w, r, username, "acc_or_it", "it")
		return

	default:
		http.Error(w, "No tienes permisos para realizar esta acción", http.StatusForbidden)
		return
	}
}

func rejectionFlowIP(userID int, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, username string) {

	type RejectRequestBody struct {
		CombinedID string `json:"idCombined"`
		Motive     string `json:"motive"`
	}

	var body RejectRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.CombinedID == "" {
		http.Error(w, "combined_id is required", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(body.CombinedID)
	if err != nil {
		http.Error(w, "combined_id no es válido", http.StatusBadRequest)
		return
	}

	// edit requests parts where user is IP
	today := time.Now().Format("2006-01-02 15:04")

	query := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "ip_response, ip_response_date",
		"value":     fmt.Sprintf("1, %s", today),
		"condition": fmt.Sprintf("id_combined = %d AND ip_id = %d", combinedID, userID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := putReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para rechazar solicitud %d por IP %s", combinedID, username), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al rechazar solicitud %d por IP %s. Status: %d, Body: %s",
			combinedID, username, resp.StatusCode, string(bodyBytes),
		), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la solicitud", http.StatusInternalServerError)
		return
	}

	// edit request
	requestUpdate := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "denied_comment",
		"value":     fmt.Sprintf("%s", escapeComma(body.Motive)),
		"condition": fmt.Sprintf("id = %d", combinedID),
	}
	jsonRequestUpdate, _ := json.Marshal(requestUpdate)
	resp = putReq(jsonRequestUpdate, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar comentario de rechazo de solicitud %d por IP %s", combinedID, username), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al actualizar comentario de rechazo de solicitud %d por IP %s. Status: %d, Body: %s",
			combinedID, username, resp.StatusCode, string(bodyBytes),
		), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la solicitud", http.StatusInternalServerError)
		return
	}

	error := notifyWorkerOfRejection(userID, combinedID, body.Motive, username, "responsible", "", today, apiKey, client, w)
	if error != nil {
		createLog(fmt.Sprintf("Error notificando al trabajador del rechazo de solicitud %d por IP %s: %v", combinedID, username, error), 1, apiKey, client, w)
		http.Error(w, "Error al notificar al trabajador sobre el rechazo", http.StatusInternalServerError)
		return
	}

	// create audit log for each part where user is IP

	personInfo, _ := getPersonInfoByID(userID, apiKey, client, w)

	partsQuery := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "id, category_id",
		"condition": fmt.Sprintf("id_combined = %d AND ip_id = %d", combinedID, userID),
	}

	jsonPartsQuery, _ := json.Marshal(partsQuery)
	respParts := getReq(jsonPartsQuery, apiKey, client, w)
	if respParts != nil {
		defer respParts.Body.Close()

		var parts []map[string]interface{}
		if err := json.NewDecoder(respParts.Body).Decode(&parts); err == nil {
			for _, part := range parts {
				partID := asInt(part["id"])
				categoryID := asInt(part["category_id"])

				err := createBudgetAuditLog(
					combinedID,
					&partID,
					&categoryID,
					"rebutjat_responsable",
					"rebutjat",
					"Petició rebutjada per responsable",
					userID,
					username,
					personInfo.Name,
					personInfo.Surname,
					map[string]interface{}{
						"stage":    "responsable",
						"category": CATEGORY_ID_TO_KEY_CATALAN[categoryID],
						"motive":   body.Motive,
					},
					apiKey,
					client,
					w,
				)
				if err != nil {
					createLog(fmt.Sprintf("Error creating IP rejection audit log for request %d part %d: %v", combinedID, partID, err), 1, apiKey, client, w)
					http.Error(w, "Error creando audit log de rechazo", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func rejectionFlowByDepartment(userID int, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, username string, stage string, rol string) {
	type RejectRequestBody struct {
		CombinedID string `json:"idCombined"`
		Motive     string `json:"motive"`
	}

	var body RejectRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.CombinedID == "" {
		http.Error(w, "combined_id is required", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(body.CombinedID)
	if err != nil {
		http.Error(w, "combined_id no es válido", http.StatusBadRequest)
		return
	}

	today := time.Now().Format("2006-01-02 15:04")

	columns := "denied_comment"
	values := fmt.Sprintf("%s", escapeComma(body.Motive))

	switch stage {
	case "projects":
		columns = "projects_response, projects_response_date, denied_comment"
		values = fmt.Sprintf("1, %s, %s", today, escapeComma(body.Motive))
	case "acc_or_it":
		columns = "acc_or_it_response, acc_or_it_response_date, denied_comment"
		values = fmt.Sprintf("1, %s, %s", today, escapeComma(body.Motive))
	default:
		http.Error(w, "invalid rejection stage", http.StatusBadRequest)
		return
	}

	requestUpdate := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   columns,
		"value":     values,
		"condition": fmt.Sprintf("id = %d", combinedID),
	}

	jsonRequestUpdate, _ := json.Marshal(requestUpdate)
	resp := putReq(jsonRequestUpdate, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar solicitud %d en rechazo por %s (%s)", combinedID, stage, username), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la solicitud", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al rechazar solicitud %d por %s (%s). Status: %d, Body: %s",
			combinedID, stage, username, resp.StatusCode, string(bodyBytes),
		), 1, apiKey, client, w)
		http.Error(w, "Error al actualizar la solicitud", http.StatusInternalServerError)
		return
	}

	// 1) Mail / notif al trabajador
	if err := notifyWorkerOfRejection(userID, combinedID, body.Motive, username, stage, rol, today, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Error notificando al trabajador del rechazo de solicitud %d por %s (%s): %v", combinedID, stage, username, err), 1, apiKey, client, w)
		http.Error(w, "Error al notificar al trabajador sobre el rechazo", http.StatusInternalServerError)
		return
	}

	// 2) Mails informativos según etapa
	switch stage {
	case "projects":
		if err := notifyIPsOfRejection(combinedID, body.Motive, username, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error notificando a IPs del rechazo de solicitud %d por Projects (%s): %v", combinedID, username, err), 1, apiKey, client, w)
		}
	case "acc_or_it":
		if err := notifyProjectsOfFinalRejection(combinedID, body.Motive, username, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error notificando a Projects del rechazo final de solicitud %d por %s: %v", combinedID, username, err), 1, apiKey, client, w)
		}
		if err := notifyIPsOfRejection(combinedID, body.Motive, username, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error notificando a IPs del rechazo final de solicitud %d por %s: %v", combinedID, username, err), 1, apiKey, client, w)
		}
	}

	personInfo, _ := getPersonInfoByID(userID, apiKey, client, w)

	action := "rebutjat_projectes"
	summary := "Petició rebutjada per projectes"

	if rol == "gerencia" {
		action = "rebutjat_gerencia"
		summary = "Petició rebutjada per gerència"
	} else if rol == "comptabilitat" {
		action = "rebutjat_comptabilitat"
		summary = "Petició rebutjada per comptabilitat"
	} else if rol == "it" {
		action = "rebutjat_it"
		summary = "Petició rebutjada per IT"
	}

	err = createBudgetAuditLog(
		combinedID,
		nil,
		nil,
		action,
		"rebutjat",
		summary,
		userID,
		username,
		personInfo.Name,
		personInfo.Surname,
		map[string]interface{}{
			"stage":  stage,
			"motive": body.Motive,
		},
		apiKey,
		client,
		w,
	)
	if err != nil {
		createLog(fmt.Sprintf("Error creating department rejection audit log for request %d: %v", combinedID, err), 1, apiKey, client, w)
		http.Error(w, "Error creando audit log de rechazo", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleAcceptRequest(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB
		createLog(fmt.Sprintf("Error parsing multipart form for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar el formulario multipart", http.StatusBadRequest)
		return
	}

	var requestData RequestPayload

	// ------------------------------ LOAD DATA ----------------------------------
	categoriesRaw := r.FormValue("categories")
	if categoriesRaw != "" {
		if err := json.Unmarshal([]byte(categoriesRaw), &requestData.Categories); err != nil {
			createLog(fmt.Sprintf("Error parsing categories for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
			http.Error(w, "Error al procesar categories", http.StatusBadRequest)
			return
		}
	}

	requestData.TravelFrom = escapeComma(r.FormValue("travel_from_where"))
	requestData.TravelTo = escapeComma(r.FormValue("travel_to_where"))
	requestData.Institution = escapeComma(r.FormValue("institution"))
	requestData.TravelSince = r.FormValue("travel_from_day")
	requestData.TravelUntil = r.FormValue("travel_until_day")
	requestData.TravelPurpose = escapeComma(r.FormValue("travel_purpose"))
	requestData.TravelProject = r.FormValue("travel_project")
	requestData.TravelObservations = escapeComma(r.FormValue("travel_observations"))
	requestData.TravelLuggageType = r.FormValue("travel_luggage_type")
	requestData.TravelLuggageKg = nil

	if requestData.TravelLuggageType == "checked" || requestData.TravelLuggageType == "hand_checked" {
		if luggageKgRaw := r.FormValue("travel_checked_kg"); luggageKgRaw != "" {
			if luggageKg, err := strconv.Atoi(luggageKgRaw); err == nil {
				requestData.TravelLuggageKg = &luggageKg
			}
		}
	}
	requestData.TravelSeatPreference = r.FormValue("travel_seat_preference")
	requestData.TravelTimePreference = r.FormValue("travel_time_preference")
	requestData.TravelArea = r.FormValue("travel_area")

	requestData.RegistrationEvent = escapeComma(r.FormValue("registration_event"))
	requestData.RegistrationFile = r.FormValue("registration_registered")
	requestData.RegistrationPayment = r.FormValue("registration_payment")
	requestData.RegistrationProject = r.FormValue("registration_project")
	requestData.RegistrationObservations = escapeComma(r.FormValue("registration_observations"))
	requestData.RegistrationStartDay = r.FormValue("registration_from_day")
	requestData.RegistrationEndDay = r.FormValue("registration_until_day")

	requestData.AccomodationWhere = escapeComma(r.FormValue("accommodation_where"))
	requestData.AccomodationSince = r.FormValue("accommodation_from_day")
	requestData.AccomodationUntil = r.FormValue("accommodation_until_day")
	requestData.AccomodationPurpose = escapeComma(r.FormValue("accommodation_purpose"))
	requestData.AccomodationProject = r.FormValue("accommodation_project")
	requestData.AccomodationObservations = escapeComma(r.FormValue("accommodation_observations"))

	requestData.OtherDescription = r.FormValue("other_description")
	requestData.OtherPriceRange = r.FormValue("other_price_range")
	requestData.OtherProject = r.FormValue("other_project")

	requestData.EquipmentCategory = r.FormValue("equipment_category")
	requestData.EquipmentOtherDescr = escapeComma(r.FormValue("equipment_other_text"))
	requestData.EquipmentPriceRange = r.FormValue("equipment_price_range")
	requestData.EquipmentDescription = escapeComma(r.FormValue("equipment_description"))
	requestData.EquipmentProject = r.FormValue("equipment_project")

	//fmt.Printf("Parsed request data for user %s: %+v\n", userACCESSING, requestData)
	// -------------------------- IPs -----------------------------------

	if travelIPRaw := r.FormValue("travel_ip"); travelIPRaw != "" {
		travelIP, err := strconv.Atoi(travelIPRaw)
		if err != nil {
			http.Error(w, "travel_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.TravelIP = travelIP
	}
	if requestData.TravelIP == 0 && contains(requestData.Categories, "travel") {
		ipPayload := map[string]interface{}{
			"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
			"columns":   "p.id",
			"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL'", requestData.TravelProject),
		}
		jsonIPPayload, _ := json.Marshal(ipPayload)
		resp := getReq(jsonIPPayload, apiKey, client, w)
		if resp != nil && resp.StatusCode == http.StatusOK {
			var ips []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&ips); err == nil && len(ips) > 0 {
				requestData.TravelIP = int(ips[0]["id"].(float64))
			}
			resp.Body.Close()
		}
	}

	if registrationIPRaw := r.FormValue("registration_ip"); registrationIPRaw != "" {
		registrationIP, err := strconv.Atoi(registrationIPRaw)
		if err != nil {
			http.Error(w, "registration_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.RegistrationIP = registrationIP
	}
	if requestData.RegistrationIP == 0 && contains(requestData.Categories, "registration") {
		ipPayload := map[string]interface{}{
			"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
			"columns":   "p.id",
			"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL'", requestData.RegistrationProject),
		}
		jsonIPPayload, _ := json.Marshal(ipPayload)
		resp := getReq(jsonIPPayload, apiKey, client, w)
		if resp != nil && resp.StatusCode == http.StatusOK {
			var ips []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&ips); err == nil && len(ips) > 0 {
				requestData.RegistrationIP = int(ips[0]["id"].(float64))
			}
			resp.Body.Close()
		}
	}

	if accomodationIPRaw := r.FormValue("accommodation_ip"); accomodationIPRaw != "" {
		accomodationIP, err := strconv.Atoi(accomodationIPRaw)
		if err != nil {
			http.Error(w, "accommodation_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.AccomodationIP = accomodationIP
	}
	if requestData.AccomodationIP == 0 && contains(requestData.Categories, "accommodation") {
		ipPayload := map[string]interface{}{
			"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
			"columns":   "p.id",
			"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL'", requestData.AccomodationProject),
		}
		jsonIPPayload, _ := json.Marshal(ipPayload)
		resp := getReq(jsonIPPayload, apiKey, client, w)
		if resp != nil && resp.StatusCode == http.StatusOK {
			var ips []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&ips); err == nil && len(ips) > 0 {
				requestData.AccomodationIP = int(ips[0]["id"].(float64))
			}
			resp.Body.Close()
		}
	}

	if otherIPRaw := r.FormValue("other_ip"); otherIPRaw != "" {
		otherIP, err := strconv.Atoi(otherIPRaw)
		if err != nil {
			http.Error(w, "other_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.OtherIP = otherIP
	}
	if requestData.OtherIP == 0 && contains(requestData.Categories, "other") {
		ipPayload := map[string]interface{}{
			"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
			"columns":   "p.id",
			"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL'", requestData.OtherProject),
		}
		jsonIPPayload, _ := json.Marshal(ipPayload)
		resp := getReq(jsonIPPayload, apiKey, client, w)
		if resp != nil && resp.StatusCode == http.StatusOK {
			var ips []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&ips); err == nil && len(ips) > 0 {
				requestData.OtherIP = int(ips[0]["id"].(float64))
			}
			resp.Body.Close()
		}
	}

	if equipmentIPRaw := r.FormValue("equipment_ip"); equipmentIPRaw != "" {
		equipmentIP, err := strconv.Atoi(equipmentIPRaw)
		if err != nil {
			http.Error(w, "equipment_ip inválido", http.StatusBadRequest)
			return
		}
		requestData.EquipmentIP = equipmentIP
	}
	if requestData.EquipmentIP == 0 && contains(requestData.Categories, "equipment") {
		ipPayload := map[string]interface{}{
			"table":     "people_projects pp LEFT JOIN people p ON pp.people_id = p.id",
			"columns":   "p.id",
			"condition": fmt.Sprintf("pp.project_id = '%s' AND pp.researcher_type = 'INVESTIGADOR PRINCIPAL'", requestData.EquipmentProject),
		}
		jsonIPPayload, _ := json.Marshal(ipPayload)
		resp := getReq(jsonIPPayload, apiKey, client, w)
		if resp != nil && resp.StatusCode == http.StatusOK {
			var ips []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&ips); err == nil && len(ips) > 0 {
				requestData.EquipmentIP = int(ips[0]["id"].(float64))
			}
			resp.Body.Close()
		}
	}

	// -------------------------- FILES -----------------------------------

	travelFiles := r.MultipartForm.File["travel_preferences_files"]
	if len(travelFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 5 archivos en travel_preferences_files", http.StatusBadRequest)
		return
	}

	savedTravelFiles, err := validateAndSaveFiles("travel", apiKey, client, w, r, travelFiles, uploadDir, requestData.TravelProject)
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//fmt.Printf("rutes de travel files %s", savedTravelFiles)

	registrationFiles := r.MultipartForm.File["registration_invoice"]
	if len(registrationFiles) > maxFilesPerKey {
		http.Error(w, "Máximo 3 archivos en registration_invoice", http.StatusBadRequest)
		return
	}
	var docType string
	if contains(requestData.Categories, "registration") {
		docType = requestData.RegistrationPayment
	}
	savedRegistrationFiles, err := validateAndSaveFiles(docType, apiKey, client, w, r, registrationFiles, uploadDir, requestData.RegistrationProject)
	if err != nil {
		createLog(fmt.Sprintf("Upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// ------------------------ GET REQUEST DATA --------------------------
	combinedIDStr := strings.TrimSpace(r.FormValue("idCombined"))
	if combinedIDStr == "" {
		http.Error(w, "idCombined es obligatorio", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(combinedIDStr)
	if err != nil {
		http.Error(w, "idCombined no es válido", http.StatusBadRequest)
		return
	}

	getQuery := map[string]interface{}{
		"table": `budget_requests br 
		LEFT JOIN budget_parts bp ON br.id = bp.id_combined
		LEFT JOIN projects pr ON bp.project_id = pr.id`,
		"columns": strings.Join([]string{
			"bp.*",
			"br.projects_response",
			"br.projects_response_date",
			"br.acc_or_it_response",
			"br.acc_or_it_response_date",
			"br.denied_comment",
			"br.canceled",
			"pr.fons_romanents",
		}, ", "),
		"condition": fmt.Sprintf("br.id = %d", combinedID),
	}

	jsonGetUpdate, _ := json.Marshal(getQuery)
	resp := getReq(jsonGetUpdate, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No se obtuvo respuesta para cancelar solicitud previa en modify request para user %s", userACCESSING), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf(
			"Error desde el servicio de BD al cancelar solicitud previa en modify request. Status: %d, Body: %s para user %s",
			resp.StatusCode, string(bodyBytes), userACCESSING,
		), 1, apiKey, client, w)
		http.Error(w, "Error al cancelar la solicitud previa", http.StatusInternalServerError)
		return
	}

	var existingRows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&existingRows); err != nil {
		createLog(fmt.Sprintf("Error decoding existing request for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar la respuesta de la base de datos", http.StatusInternalServerError)
		return
	}

	existingParts := buildExistingPartsMap(existingRows)
	gerencia := getIsGerencia(userID, userACCESSING, apiKey, client, w, r)
	today := time.Now().Format("2006-01-02 15:04")

	accessProjects := getIsProjects(userID, userACCESSING, apiKey, client, w, r)
	accessFinances := getIsFinances(userID, userACCESSING, apiKey, client, w, r)
	accessITResults := getIsIT(userID, userACCESSING, apiKey, client, w, r)

	hasFonsRomanents := requestHasFonsRomanents(existingRows)
	hasEquipment := requestHasEquipment(existingRows)

	projectsResponse := getRequestProjectsResponse(existingRows)
	accOrItResponse := getRequestAccOrItResponse(existingRows)

	// This means the current user is accepting as the assigned IP/responsible,
	// even if the user also belongs to Gerencia.
	acceptingAsIP := hasPendingIPPartAssignedToUser(existingRows, userID)

	// Normal Projects approval.
	acceptingAsProjects :=
		!acceptingAsIP &&
			len(accessProjects) > 0 &&
			!hasFonsRomanents &&
			projectsResponse == 0

	// Gerencia approving in the Projects step because the project has fons romanents.
	acceptingAsGerenciaProjects :=
		!acceptingAsIP &&
			len(gerencia) > 0 &&
			hasFonsRomanents &&
			projectsResponse == 0

	// Accounting final step.
	acceptingAsFinances :=
		!acceptingAsIP &&
			!acceptingAsProjects &&
			!acceptingAsGerenciaProjects &&
			len(accessFinances) > 0 &&
			projectsResponse == 1 &&
			accOrItResponse == 0 &&
			!hasEquipment

	// IT final step for equipment requests.
	acceptingAsIT :=
		!acceptingAsIP &&
			!acceptingAsProjects &&
			!acceptingAsGerenciaProjects &&
			len(accessITResults) > 0 &&
			projectsResponse == 1 &&
			accOrItResponse == 0 &&
			hasEquipment

	personInfo, err := getPersonInfoByID(userID, apiKey, client, w)
	if err != nil {
		return
	}
	if contains(requestData.Categories, "travel") {
		existing, ok := existingParts["travel"]
		if ok {
			if acceptingAsIP {
				queryUpdateStatus := map[string]interface{}{
					"table":     "budget_parts",
					"columns":   "ip_response, ip_response_date",
					"value":     fmt.Sprintf("1, %s", today),
					"condition": fmt.Sprintf("id = %d", existing.ID),
				}
				jsonQuery, _ := json.Marshal(queryUpdateStatus)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de IP en modify request para user %s", userACCESSING), 1, apiKey, client, w)
					return
				}
				resp.Body.Close()

				categoryID := existing.CategoryID
				partID := existing.ID

				err = createBudgetAuditLog(
					combinedID,
					&partID,
					&categoryID,
					"aprovat_responsable",
					"aprovat",
					"El responsable ha aprovat la petició de viatge",
					userID,
					userACCESSING,
					personInfo.Name,
					personInfo.Surname,
					map[string]interface{}{
						"stage":    "responsable",
						"category": "viatge",
					},
					apiKey,
					client,
					w,
				)
				if err != nil {
					createLog(fmt.Sprintf("Error creating IP acceptance audit log for request %d part %d: %v", combinedID, partID, err), 1, apiKey, client, w)
					http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
					return
				}
			}
			changes := diffTravel(existing, requestData)
			if len(changes) > 0 {
				query := buildUpdateQuery(
					"budget_parts",
					changes,
					fmt.Sprintf("id = %d", existing.ID),
					userID, userACCESSING, apiKey, client, w, r)
				jsonQuery, _ := json.Marshal(query)
				//fmt.Println("Query de actualización travel: ", string(jsonQuery))

				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					http.Error(w, "Error actualizando travel", http.StatusInternalServerError)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					//bodyBytes, _ := io.ReadAll(resp.Body)
					//fmt.Println("PUT travel status:", resp.StatusCode)
					//fmt.Println("PUT travel body:", string(bodyBytes))
					http.Error(w, "Error actualizando travel", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	if contains(requestData.Categories, "registration") {
		existing, ok := existingParts["registration"]
		if ok {
			if acceptingAsIP {
				queryUpdateStatus := map[string]interface{}{
					"table":     "budget_parts",
					"columns":   "ip_response, ip_response_date",
					"value":     fmt.Sprintf("1, %s", today),
					"condition": fmt.Sprintf("id = %d", existing.ID),
				}
				jsonQuery, _ := json.Marshal(queryUpdateStatus)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de IP en modify request para user %s", userACCESSING), 1, apiKey, client, w)
					return
				}
				resp.Body.Close()
				categoryID := existing.CategoryID
				partID := existing.ID

				err = createBudgetAuditLog(
					combinedID,
					&partID,
					&categoryID,
					"aprovat_responsable",
					"aprovat",
					"El responsable ha aprovat la petició d'inscripció",
					userID,
					userACCESSING,
					personInfo.Name,
					personInfo.Surname,
					map[string]interface{}{
						"stage":    "responsable",
						"category": "inscripció",
					},
					apiKey,
					client,
					w,
				)
				if err != nil {
					createLog(fmt.Sprintf("Error creating IP acceptance audit log for request %d part %d: %v", combinedID, partID, err), 1, apiKey, client, w)
					http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
					return
				}
			}
			changes := diffRegistration(existing, requestData)
			if len(changes) > 0 {
				query := buildUpdateQuery(
					"budget_parts",
					changes,
					fmt.Sprintf("id = %d", existing.ID),
					userID, userACCESSING, apiKey, client, w, r)

				jsonQuery, _ := json.Marshal(query)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					http.Error(w, "Error actualizando registration", http.StatusInternalServerError)
					return
				}
				resp.Body.Close()
			}
		}
	}

	if contains(requestData.Categories, "accommodation") {
		existing, ok := existingParts["accommodation"]
		if ok {
			if acceptingAsIP {
				queryUpdateStatus := map[string]interface{}{
					"table":     "budget_parts",
					"columns":   "ip_response, ip_response_date",
					"value":     fmt.Sprintf("1, %s", today),
					"condition": fmt.Sprintf("id = %d", existing.ID),
				}
				jsonQuery, _ := json.Marshal(queryUpdateStatus)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de IP en modify request para user %s", userACCESSING), 1, apiKey, client, w)
					return
				}
				resp.Body.Close()
				categoryID := existing.CategoryID
				partID := existing.ID

				err = createBudgetAuditLog(
					combinedID,
					&partID,
					&categoryID,
					"aprovat_responsable",
					"aprovat",
					"El responsable ha aprovat la petició d'allotjament",
					userID,
					userACCESSING,
					personInfo.Name,
					personInfo.Surname,
					map[string]interface{}{
						"stage":    "responsable",
						"category": "allotjament",
					},
					apiKey,
					client,
					w,
				)
				if err != nil {
					createLog(fmt.Sprintf("Error creating IP acceptance audit log for request %d part %d: %v", combinedID, partID, err), 1, apiKey, client, w)
					http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
					return
				}
			}
			changes := diffAccommodation(existing, requestData)
			if len(changes) > 0 {
				query := buildUpdateQuery(
					"budget_parts",
					changes,
					fmt.Sprintf("id = %d", existing.ID),
					userID, userACCESSING, apiKey, client, w, r)

				jsonQuery, _ := json.Marshal(query)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					http.Error(w, "Error actualizando accomodation", http.StatusInternalServerError)
					return
				}
				resp.Body.Close()
			}
		}
	}

	if contains(requestData.Categories, "equipment") {
		existing, ok := existingParts["equipment"]
		if ok {
			if acceptingAsIP {
				queryUpdateStatus := map[string]interface{}{
					"table":     "budget_parts",
					"columns":   "ip_response, ip_response_date",
					"value":     fmt.Sprintf("1, %s", today),
					"condition": fmt.Sprintf("id = %d", existing.ID),
				}
				jsonQuery, _ := json.Marshal(queryUpdateStatus)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de IP en modify request para user %s", userACCESSING), 1, apiKey, client, w)
					return
				}
				resp.Body.Close()
				categoryID := existing.CategoryID
				partID := existing.ID

				err = createBudgetAuditLog(
					combinedID,
					&partID,
					&categoryID,
					"aprovat_responsable",
					"aprovat",
					"El responsable ha aprovat la petició d'equipament",
					userID,
					userACCESSING,
					personInfo.Name,
					personInfo.Surname,
					map[string]interface{}{
						"stage":    "responsable",
						"category": "equipament",
					},
					apiKey,
					client,
					w,
				)
				if err != nil {
					createLog(fmt.Sprintf("Error creating IP acceptance audit log for request %d part %d: %v", combinedID, partID, err), 1, apiKey, client, w)
					http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
					return
				}
			}
			changes := diffEquipment(existing, requestData)
			if len(changes) > 0 {
				query := buildUpdateQuery(
					"budget_parts",
					changes,
					fmt.Sprintf("id = %d", existing.ID),
					userID, userACCESSING, apiKey, client, w, r)

				jsonQuery, _ := json.Marshal(query)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					http.Error(w, "Error actualizando equipment", http.StatusInternalServerError)
					return
				}
				resp.Body.Close()
			}
		}
	}

	if contains(requestData.Categories, "other") {
		existing, ok := existingParts["other"]
		if ok {
			if acceptingAsIP {
				queryUpdateStatus := map[string]interface{}{
					"table":     "budget_parts",
					"columns":   "ip_response, ip_response_date",
					"value":     fmt.Sprintf("1, %s", today),
					"condition": fmt.Sprintf("id = %d", existing.ID),
				}
				jsonQuery, _ := json.Marshal(queryUpdateStatus)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de IP en modify request para user %s", userACCESSING), 1, apiKey, client, w)
					return
				}
				resp.Body.Close()
				categoryID := existing.CategoryID
				partID := existing.ID

				err = createBudgetAuditLog(
					combinedID,
					&partID,
					&categoryID,
					"aprovat_responsable",
					"aprovat",
					"El responsable ha aprovat la petició d'altres despeses",
					userID,
					userACCESSING,
					personInfo.Name,
					personInfo.Surname,
					map[string]interface{}{
						"stage":    "responsable",
						"category": "altres despeses",
					},
					apiKey,
					client,
					w,
				)
				if err != nil {
					createLog(fmt.Sprintf("Error creating IP acceptance audit log for request %d part %d: %v", combinedID, partID, err), 1, apiKey, client, w)
					http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
					return
				}

			}
			changes := diffOther(existing, requestData)
			if len(changes) > 0 {
				query := buildUpdateQuery(
					"budget_parts",
					changes,
					fmt.Sprintf("id = %d", existing.ID),
					userID, userACCESSING, apiKey, client, w, r)

				jsonQuery, _ := json.Marshal(query)
				resp := putReq(jsonQuery, apiKey, client, w)
				if resp == nil {
					http.Error(w, "Error actualizando other", http.StatusInternalServerError)
					return
				}
				resp.Body.Close()
			}
		}
	}

	if len(savedTravelFiles) > 0 {
		if existing, ok := existingParts["travel"]; ok {
			if err := uploadFilePathToRequestID(existing.ID, savedTravelFiles, userACCESSING, apiKey, client, w); err != nil {
				http.Error(w, "Error guardando archivos de travel", http.StatusInternalServerError)
				return
			}
		}
	}

	if len(savedRegistrationFiles) > 0 {
		if existing, ok := existingParts["registration"]; ok {
			if err := uploadFilePathToRequestID(existing.ID, savedRegistrationFiles, userACCESSING, apiKey, client, w); err != nil {
				http.Error(w, "Error guardando archivos de registration", http.StatusInternalServerError)
				return
			}
		}
	}

	var queryUpdateStatus map[string]interface{}
	if acceptingAsProjects || acceptingAsGerenciaProjects {
		queryUpdateStatus = map[string]interface{}{
			"table":     "budget_requests",
			"columns":   "projects_response, projects_response_date",
			"value":     fmt.Sprintf("1, %s", today),
			"condition": fmt.Sprintf("id = %d", combinedID),
		}
	} else if acceptingAsFinances || acceptingAsIT {
		queryUpdateStatus = map[string]interface{}{
			"table":     "budget_requests",
			"columns":   "acc_or_it_response, acc_or_it_response_date",
			"value":     fmt.Sprintf("1, %s", today),
			"condition": fmt.Sprintf("id = %d", combinedID),
		}
	}
	if queryUpdateStatus != nil {
		jsonUpdateQuery, _ := json.Marshal(queryUpdateStatus)
		resp := putReq(jsonUpdateQuery, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("No se obtuvo respuesta para actualizar estado de solicitud %d por user %s", combinedID, userACCESSING), 1, apiKey, client, w)
			return
		}
		resp.Body.Close()
	}

	// flux correus

	if acceptingAsIP {
		// NOTIFICAR A PROJECTS
		//projectsConfirmation.html
		researcherID, err := getRequestResearcherID(combinedID, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Error loading researcher for request %d: %v", combinedID, err), 1, apiKey, client, w)
		} else {
			if hasFonsRomanents {
				// Fons romanents: después de IP, Gerencia actúa como Projects.
				// Se manda el mismo correo de Projects, pero a Gerencia.
				if err := sendGerenciaEmailAfterIPAcceptance(researcherID, userID, requestData, today, apiKey, client, w); err != nil {
					createLog(fmt.Sprintf("Error sending gerencia-as-projects email for request %d after IP acceptance: %v", combinedID, err), 1, apiKey, client, w)
				}
			} else {
				if err := sendProjectsEmailAfterIPAcceptance(researcherID, userID, requestData, today, apiKey, client, w); err != nil {
					createLog(fmt.Sprintf("Error sending projects email for request %d after IP acceptance: %v", combinedID, err), 1, apiKey, client, w)
				}
				if err := sendManagementEmailAfterIPAcceptance(researcherID, userID, requestData, today, apiKey, client, w); err != nil {
					createLog(fmt.Sprintf("Error sending management email for request %d after IP acceptance: %v", combinedID, err), 1, apiKey, client, w)
				}
			}
		}
		// Cas equipment / other -> comprovar rang de preu, si preu
		// > 5000 notificar a gerencia

	} else if acceptingAsProjects || acceptingAsGerenciaProjects {
		// Projects/Gerencia step accepted.
		action := "aprovat_projectes"
		summary := "Projectes ha aprovat la petició pressupostària"

		if acceptingAsGerenciaProjects {
			action = "aprovat_gerencia"
			summary = "Gerència ha aprovat la petició pressupostària"
		}

		err := createBudgetAuditLog(
			combinedID,
			nil,
			nil,
			action,
			"aprovat",
			summary,
			userID,
			userACCESSING,
			personInfo.Name,
			personInfo.Surname,
			map[string]interface{}{
				"stage": "projectes",
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			createLog(fmt.Sprintf("Error creating projects acceptance audit log for request %d: %v", combinedID, err), 1, apiKey, client, w)
			http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
			return
		}
		// Now route to IT if equipment, otherwise Accounting.

		if hasEquipment {
			if err := notifITAccepted(combinedID, requestData, userID, today, apiKey, client, w); err != nil {
				createLog(fmt.Sprintf("Error sending IT email for request %d: %v", combinedID, err), 1, apiKey, client, w)
			}
		} else {
			if err := notifAccountingAccepted(combinedID, requestData, today, userID, apiKey, client, w); err != nil {
				createLog(fmt.Sprintf("Error sending accounting email for request %d: %v", combinedID, err), 1, apiKey, client, w)
			}
		}

	} else if acceptingAsFinances {
		snapshot, err := getBudgetRequestAuditSnapshot(combinedID, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Error creating accounting approval snapshot for request %d: %v", combinedID, err), 1, apiKey, client, w)
			http.Error(w, "Error generando snapshot de auditoría", http.StatusInternalServerError)
			return
		}

		err = createBudgetAuditLog(
			combinedID,
			nil,
			nil,
			"aprovat_comptabilitat",
			"aprovat",
			"Comptabilitat ha aprovat i confirmat la petició pressupostària",
			userID,
			userACCESSING,
			personInfo.Name,
			personInfo.Surname,
			map[string]interface{}{
				"stage":             "comptabilitat",
				"approved_snapshot": snapshot,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			createLog(fmt.Sprintf("Error creating final acceptance audit log for request %d: %v", combinedID, err), 1, apiKey, client, w)
			http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
			return
		}
		// Accounting confirms -> notify worker
		if err := notifyWorkerAfterAccountingConfirmation(combinedID, userID, today, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error sending final worker email for request %d: %v", combinedID, err), 1, apiKey, client, w)
		}

		workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
		if err == nil {
			if err := sendNotification(
				"Budget Request Confirmed",
				"Your budget request has been fully approved and confirmed. You can review it in the Budgeting module.",
				workerInfo.ID,
				apiKey, client, w,
			); err != nil {
				createLog(fmt.Sprintf("Error sending final worker notification for request %d: %v", combinedID, err), 1, apiKey, client, w)
			}
		}

		// If travel area is EU or NON_EU -> notify RRHH
		needsRRHH, err := hasTravelToRRHH(combinedID, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Error checking RRHH condition for request %d: %v", combinedID, err), 1, apiKey, client, w)
		} else if needsRRHH {
			if err := notifyRRHHAfterAccountingConfirmation(combinedID, apiKey, client, w); err != nil {
				createLog(fmt.Sprintf("Error sending RRHH email for request %d: %v", combinedID, err), 1, apiKey, client, w)
			}
		}

	} else if acceptingAsIT {
		snapshot, err := getBudgetRequestAuditSnapshot(combinedID, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Error creating IT approval snapshot for request %d: %v", combinedID, err), 1, apiKey, client, w)
			http.Error(w, "Error generando snapshot de auditoría", http.StatusInternalServerError)
			return
		}

		err = createBudgetAuditLog(
			combinedID,
			nil,
			nil,
			"aprovat_it",
			"aprovat",
			"IT ha aprovat i confirmat la petició pressupostària",
			userID,
			userACCESSING,
			personInfo.Name,
			personInfo.Surname,
			map[string]interface{}{
				"stage":             "it",
				"approved_snapshot": snapshot,
			},
			apiKey,
			client,
			w,
		)
		if err != nil {
			createLog(fmt.Sprintf("Error creating final acceptance audit log for request %d: %v", combinedID, err), 1, apiKey, client, w)
			http.Error(w, "Error creando audit log de aceptación", http.StatusInternalServerError)
			return
		}
		// IT confirms equipment requests coming from Projects
		if err := notifyWorkerAfterITConfirmation(combinedID, userID, today, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error sending final IT worker email for request %d: %v", combinedID, err), 1, apiKey, client, w)
		}

		workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
		if err == nil {
			if err := sendNotification(
				"Budget Request Confirmed",
				"Your equipment request has been approved by IT and confirmed. You can review it in the Budgeting module.",
				workerInfo.ID,
				apiKey, client, w,
			); err != nil {
				createLog(fmt.Sprintf("Error sending worker notification after IT approval for request %d: %v", combinedID, err), 1, apiKey, client, w)
			}
		}

		if err := notifyAccountingAfterITConfirmation(combinedID, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Error sending accounting email after IT approval for request %d: %v", combinedID, err), 1, apiKey, client, w)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})

}

// ---------------- REQUEST UPDATE HELPERS ----------------

func hasPendingIPPartAssignedToUser(rows []map[string]interface{}, userID int) bool {
	for _, row := range rows {
		ipID := asInt(row["ip_id"])
		ipResponse := asInt(row["ip_response"])

		if ipID == userID && ipResponse == 0 {
			return true
		}
	}

	return false
}

func requestHasFonsRomanents(rows []map[string]interface{}) bool {
	for _, row := range rows {
		if asInt(row["fons_romanents"]) == 1 {
			return true
		}
	}

	return false
}

func requestHasEquipment(rows []map[string]interface{}) bool {
	for _, row := range rows {
		if asInt(row["category_id"]) == 4 {
			return true
		}
	}

	return false
}

func getRequestProjectsResponse(rows []map[string]interface{}) int {
	if len(rows) == 0 {
		return 0
	}

	return asInt(rows[0]["projects_response"])
}

func getRequestAccOrItResponse(rows []map[string]interface{}) int {
	if len(rows) == 0 {
		return 0
	}

	return asInt(rows[0]["acc_or_it_response"])
}

func uploadFilePathToRequestID(requestID int, files []SavedFile, username string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	for _, f := range files {
		filepath := cleanPath(f.Path)
		filePayload := map[string]interface{}{
			"table":   "request_files",
			"columns": "request_id, file_path",
			"value":   fmt.Sprintf("%d, %s", requestID, filepath),
		}
		jsonFilePayload, _ := json.Marshal(filePayload)
		resp := postReq(jsonFilePayload, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error saving file record for request_id %d for user %s: file %s", requestID, username, f.OriginalName), 1, apiKey, client, w)
			return fmt.Errorf("error saving file record")
		}
		resp.Body.Close()
	}
	return nil
}

func buildExistingPartsMap(rows []map[string]interface{}) map[string]ExistingPart {
	result := make(map[string]ExistingPart)

	for _, row := range rows {
		categoryID := asInt(row["category_id"])
		key := CATEGORY_ID_TO_KEY[categoryID]
		if key == "" {
			continue
		}

		result[key] = ExistingPart{
			ID:                  asInt(row["id"]),
			CategoryID:          categoryID,
			ProjectID:           asString(row["project_id"]),
			IPID:                asInt(row["ip_id"]),
			Purpose:             asString(row["purpose"]),
			Observations:        asString(row["observations"]),
			FromDay:             normalizeDateString(asString(row["fromDay"])),
			UntilDay:            normalizeDateString(asString(row["untilDay"])),
			TravelFromPlace:     asString(row["travel_fromPlace"]),
			WherePlace:          asString(row["wherePlace"]),
			Institution:         asString(row["institution"]),
			RegistrationInvoice: asString(row["registration_invoice"]),
			RegistrationType:    asString(row["registration_type"]),
			EquipmentCategory:   asString(row["equipment_category"]),
			OtherPrice:          asString(row["other_price"]),
			LuggageType:         asString(row["luggage_type"]),
			LuggageWeight:       asNullableInt(row["luggage_weight"]),
			SeatPreference:      asString(row["seat_preference"]),
			TimePreference:      asString(row["time_preference"]),
			TravelArea:          asString(row["travel_area"]),
		}
	}

	return result
}

func diffTravel(existing ExistingPart, req RequestPayload) map[string]interface{} {
	changes := make(map[string]interface{})

	//fmt.Printf("Purpose existing=[%q] req=[%q]\n", existing.Purpose, req.TravelPurpose)
	//fmt.Printf("TravelArea existing=[%q] req=[%q]\n", existing.TravelArea, req.TravelArea)
	//fmt.Printf("LuggageType existing=[%q] req=[%q]\n", existing.LuggageType, req.TravelLuggageType)
	//fmt.Printf("SeatPreference existing=[%q] req=[%q]\n", existing.SeatPreference, req.TravelSeatPreference)
	//fmt.Printf("TimePreference existing=[%q] req=[%q]\n", existing.TimePreference, req.TravelTimePreference)

	if stringsDifferent(existing.TravelFromPlace, req.TravelFrom) {
		changes["travel_fromPlace"] = req.TravelFrom
	}
	if stringsDifferent(existing.WherePlace, req.TravelTo) {
		changes["wherePlace"] = req.TravelTo
	}
	if stringsDifferent(existing.Institution, req.Institution) {
		changes["institution"] = req.Institution
	}

	if stringsDifferent(existing.FromDay, req.TravelSince) {
		changes["fromDay"] = req.TravelSince
	}
	if stringsDifferent(existing.UntilDay, req.TravelUntil) {
		changes["untilDay"] = req.TravelUntil
	}
	if stringsDifferent(existing.Purpose, req.TravelPurpose) {
		changes["purpose"] = req.TravelPurpose
	}
	if stringsDifferent(existing.ProjectID, req.TravelProject) {
		changes["project_id"] = req.TravelProject
	}
	if intsDifferent(existing.IPID, req.TravelIP) {
		changes["ip_id"] = strconv.Itoa(req.TravelIP)
	}
	if stringsDifferent(existing.Observations, req.TravelObservations) {
		changes["observations"] = req.TravelObservations
	}
	if stringsDifferent(existing.LuggageType, req.TravelLuggageType) {
		changes["luggage_type"] = req.TravelLuggageType
	}
	incomingKg := req.TravelLuggageKg
	if req.TravelLuggageType != "checked" && req.TravelLuggageType != "hand_checked" {
		incomingKg = nil
	}

	if nullableIntDifferent(existing.LuggageWeight, incomingKg) {
		if incomingKg == nil {
			changes["luggage_weight"] = nil
		} else {
			changes["luggage_weight"] = *incomingKg
		}
	}

	if stringsDifferent(existing.SeatPreference, req.TravelSeatPreference) {
		changes["travel_seat_preference"] = req.TravelSeatPreference
	}
	if stringsDifferent(existing.TimePreference, req.TravelTimePreference) {
		changes["travel_time_preference"] = req.TravelTimePreference
	}
	if stringsDifferent(existing.TravelArea, req.TravelArea) {
		changes["travel_area"] = req.TravelArea
	}

	//fmt.Println("Travel changes: ", changes)
	return changes
}

func diffRegistration(existing ExistingPart, req RequestPayload) map[string]interface{} {
	changes := make(map[string]interface{})

	regInvoice := "0"
	if req.RegistrationFile == "yes" {
		regInvoice = "1"
	}

	regType := "P"
	if req.RegistrationPayment == "invoice" {
		regType = "I"
	}

	if stringsDifferent(existing.Purpose, req.RegistrationEvent) {
		changes["purpose"] = req.RegistrationEvent
	}
	if stringsDifferent(existing.ProjectID, req.RegistrationProject) {
		changes["project_id"] = req.RegistrationProject
	}
	if intsDifferent(existing.IPID, req.RegistrationIP) {
		changes["ip_id"] = strconv.Itoa(req.RegistrationIP)
	}
	if stringsDifferent(existing.Observations, req.RegistrationObservations) {
		changes["observations"] = req.RegistrationObservations
	}
	if stringsDifferent(existing.RegistrationInvoice, regInvoice) {
		changes["registration_invoice"] = regInvoice
	}
	if stringsDifferent(existing.RegistrationType, regType) {
		changes["registration_type"] = regType
	}
	if stringsDifferent(existing.FromDay, req.RegistrationStartDay) {
		changes["fromDay"] = req.RegistrationStartDay
	}
	if stringsDifferent(existing.UntilDay, req.RegistrationEndDay) {
		changes["untilDay"] = req.RegistrationEndDay
	}

	return changes
}

func diffAccommodation(existing ExistingPart, req RequestPayload) map[string]interface{} {
	changes := make(map[string]interface{})

	if stringsDifferent(existing.WherePlace, req.AccomodationWhere) {
		changes["wherePlace"] = req.AccomodationWhere
	}
	if stringsDifferent(existing.FromDay, req.AccomodationSince) {
		changes["fromDay"] = req.AccomodationSince
	}
	if stringsDifferent(existing.UntilDay, req.AccomodationUntil) {
		changes["untilDay"] = req.AccomodationUntil
	}
	if stringsDifferent(existing.Purpose, req.AccomodationPurpose) {
		changes["purpose"] = req.AccomodationPurpose
	}
	if stringsDifferent(existing.ProjectID, req.AccomodationProject) {
		changes["project_id"] = req.AccomodationProject
	}
	if intsDifferent(existing.IPID, req.AccomodationIP) {
		changes["ip_id"] = strconv.Itoa(req.AccomodationIP)
	}
	if stringsDifferent(existing.Observations, req.AccomodationObservations) {
		changes["observations"] = req.AccomodationObservations
	}

	return changes
}

func diffOther(existing ExistingPart, req RequestPayload) map[string]interface{} {
	changes := make(map[string]interface{})

	if stringsDifferent(existing.Observations, req.OtherDescription) {
		changes["observations"] = req.OtherDescription
	}
	if stringsDifferent(existing.OtherPrice, req.OtherPriceRange) {
		changes["other_price"] = req.OtherPriceRange
	}
	if stringsDifferent(existing.ProjectID, req.OtherProject) {
		changes["project_id"] = req.OtherProject
	}
	if intsDifferent(existing.IPID, req.OtherIP) {
		changes["ip_id"] = strconv.Itoa(req.OtherIP)
	}

	return changes
}

func diffEquipment(existing ExistingPart, req RequestPayload) map[string]interface{} {
	changes := make(map[string]interface{})

	if stringsDifferent(existing.EquipmentCategory, req.EquipmentCategory) {
		changes["equipment_category"] = req.EquipmentCategory
	}
	if stringsDifferent(existing.Purpose, req.EquipmentOtherDescr) {
		changes["purpose"] = req.EquipmentOtherDescr
	}
	if stringsDifferent(existing.OtherPrice, req.EquipmentPriceRange) {
		changes["other_price"] = req.EquipmentPriceRange
	}
	if stringsDifferent(existing.Observations, req.EquipmentDescription) {
		changes["observations"] = req.EquipmentDescription
	}
	if stringsDifferent(existing.ProjectID, req.EquipmentProject) {
		changes["project_id"] = req.EquipmentProject
	}
	if intsDifferent(existing.IPID, req.EquipmentIP) {
		changes["ip_id"] = strconv.Itoa(req.EquipmentIP)
	}

	return changes
}

func toSQLValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "NULL"
		}
		return escapeComma(v)

	case *string:
		if v == nil || strings.TrimSpace(*v) == "" {
			return "NULL"
		}
		return escapeComma(*v)

	case int:
		return strconv.Itoa(v)

	case *int:
		if v == nil {
			return "NULL"
		}
		return strconv.Itoa(*v)

	case int64:
		return strconv.FormatInt(v, 10)

	case *int64:
		if v == nil {
			return "NULL"
		}
		return strconv.FormatInt(*v, 10)

	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)

	case *float64:
		if v == nil {
			return "NULL"
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)

	case bool:
		if v {
			return "1"
		}
		return "0"

	default:
		return escapeComma(fmt.Sprintf("%v", v))
	}
}

func buildUpdateQuery(table string, changes map[string]interface{}, condition string, userID int, username, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) map[string]interface{} {

	columns := make([]string, 0, len(changes))
	values := make([]string, 0, len(changes))

	for col, val := range changes {
		columns = append(columns, col)
		values = append(values, escapeComma(toSQLValue(val)))
	}

	return map[string]interface{}{
		"table":     table,
		"columns":   strings.Join(columns, ", "),
		"value":     strings.Join(values, ", "),
		"condition": condition,
	}
}

// ---------------- TRAVEL CONTACT INFO ----------------

type travelIdentityCorrection struct {
	Field         string
	StoredValue   string
	ReportedValue string
}

func normalizeIdentityComparisonValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	return strings.ToUpper(value)
}

func getTravellerIdentityBaseData(userID int, apiKey string, client *http.Client, w http.ResponseWriter) (map[string]string, error) {
	payload := map[string]interface{}{
		"table":     "people",
		"columns":   "name, surname, secondSurname, user_email, user_phone, nif",
		"condition": fmt.Sprintf("id = %d", userID),
	}
	jsonPayload, _ := json.Marshal(payload)
	resp := getReq(jsonPayload, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("no response while reading traveller base data")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("traveller base data read failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode traveller base data response: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("traveller base data not found")
	}

	row := rows[0]
	return map[string]string{
		"name":          strings.TrimSpace(unescapeComma(asString(row["name"]))),
		"surname":       strings.TrimSpace(unescapeComma(asString(row["surname"]))),
		"secondSurname": strings.TrimSpace(unescapeComma(asString(row["secondSurname"]))),
		"user_email":    strings.TrimSpace(asString(row["user_email"])),
		"user_phone":    strings.TrimSpace(asString(row["user_phone"])),
		"nif":           strings.TrimSpace(asString(row["nif"])),
	}, nil
}

func notifyRRHHTravelIdentityCorrections(userID int, data RequestPayload, apiKey string, client *http.Client, w http.ResponseWriter) error {
	baseData, err := getTravellerIdentityBaseData(userID, apiKey, client, w)
	if err != nil {
		return err
	}

	changes := []travelIdentityCorrection{}

	if strings.EqualFold(strings.TrimSpace(data.TravelContactPhoneOK), "no") {
		storedPhone := strings.TrimSpace(baseData["user_phone"])
		reportedPhone := strings.TrimSpace(data.TravelContactPhone)

		if normalizeIdentityComparisonValue(storedPhone) != normalizeIdentityComparisonValue(reportedPhone) {
			changes = append(changes, travelIdentityCorrection{
				Field:         "Phone",
				StoredValue:   storedPhone,
				ReportedValue: reportedPhone,
			})
		}
	}

	if strings.EqualFold(strings.TrimSpace(data.TravelPassportOK), "no") &&
		strings.EqualFold(strings.TrimSpace(data.TravelPassportType), "dni") {
		storedDNI := strings.TrimSpace(baseData["nif"])
		reportedDNI := strings.TrimSpace(data.TravelPassportNumber)

		if normalizeIdentityComparisonValue(storedDNI) != normalizeIdentityComparisonValue(reportedDNI) {
			changes = append(changes, travelIdentityCorrection{
				Field:         "DNI/NIF",
				StoredValue:   storedDNI,
				ReportedValue: reportedDNI,
			})
		}
	}

	if len(changes) == 0 {
		return nil
	}

	fullName := strings.TrimSpace(strings.Join([]string{
		baseData["name"],
		baseData["surname"],
		baseData["secondSurname"],
	}, " "))

	type rrhhIdentityCorrectionMailData struct {
		TravellerName  string
		TravellerEmail string
		Changes        []travelIdentityCorrection
	}

	const mailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Traveller Information Correction Required</title>
    <style>
        body {
            font-family: "Segoe UI", Arial, sans-serif;
            background-color: #f7f7f7;
            color: #333;
            margin: 0;
            padding: 0;
        }
        .container {
            margin: 30px auto;
            padding: 30px 25px;
            max-width: 600px;
            background-color: #ffffff;
            border: 1px solid #ddd;
            border-radius: 10px;
            box-shadow: 0 2px 6px rgba(0,0,0,0.05);
        }
        h1 {
            text-align: center;
            font-size: 24px;
            margin-top: 10px;
            color: #2e7d32;
        }
        p {
            line-height: 1.6;
            font-size: 15px;
        }
        .status {
            border-left: 5px solid #2e7d32;
            padding: 12px 18px;
            margin: 25px 0;
            font-size: 1.05em;
            border-radius: 6px;
            background-color: #edf7ed;
            color: #1e5c24;
        }
        .summary {
            margin: 20px 0;
            padding: 14px 16px;
            background-color: #f8f9fb;
            border: 1px solid #d9e1ea;
            border-radius: 6px;
            color: #2f3b45;
        }
        .summary p {
            margin: 8px 0;
        }
        .summary strong {
            display: inline-block;
            min-width: 155px;
        }
        .highlight {
            margin-top: 10px;
            padding: 12px 14px;
            background-color: #f6fbf6;
            border: 1px solid #cfe8d1;
            border-radius: 6px;
            color: #2d4f2f;
        }
        .footer {
            margin-top: 35px;
            text-align: center;
            font-size: 0.9em;
            color: #777;
        }
        img {
            display: block;
            max-width: 120px;
            margin: 0 auto 15px;
        }
        .note {
            font-size: 13px;
            color: #999;
            text-align: center;
            margin-top: 20px;
        }
        .details {
            margin-top: 20px;
            padding: 14px 16px;
            background-color: #fcfcfc;
            border: 1px solid #e3e6ea;
            border-radius: 6px;
            color: #2f3b45;
        }
        .details h2 {
            font-size: 17px;
            margin: 0 0 12px 0;
            color: #444;
        }
        .identity-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 12px;
            font-size: 14px;
        }
        .identity-table th,
        .identity-table td {
            border: 1px solid #e3e6ea;
            padding: 9px 10px;
            text-align: left;
            vertical-align: top;
        }
        .identity-table th {
            background-color: #f8f9fb;
            color: #2f3b45;
            font-weight: 600;
        }
        .identity-table td {
            background-color: #ffffff;
        }
    </style>
</head>
<body>
    <div class="container">
        <img src="https://issues.crm.cat/assets/images/crmlogo.jfif" alt="CRM Logo">

        <h1>Traveller Information Correction Required</h1>

        <p>Hello <strong>RRHH</strong>,</p>

        <p>
            We would like to inform you that a traveller has indicated that some stored travel identity or contact information is incorrect.
        </p>

        <div class="highlight">
            The traveller <strong>{{.TravellerName}}</strong> has submitted different information from the data currently stored in the people record.
        </div>

        <div class="details">
            <h2>Reported differences</h2>
            <table class="identity-table">
                <thead>
                    <tr>
                        <th>Field</th>
                        <th>Stored in database</th>
                        <th>Reported by traveller</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Changes}}
                    <tr>
                        <td>{{.Field}}</td>
                        <td>{{.StoredValue}}</td>
                        <td>{{.ReportedValue}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <div class="status">
            Please review the information in the people record and update it if appropriate.
        </div>

        <p>
            Please use the information above for any internal processing related to this travel.
        </p>

        <p class="note">
            Please do not reply to this email — this inbox is not monitored.
        </p>

        <div class="footer">
            Thank you for using <br>
            <strong>CRM Intratools Budgeting</strong>
        </div>
    </div>
</body>
</html>`

	tmpl, err := template.New("rrhhIdentityCorrection").Parse(mailTemplate)
	if err != nil {
		return fmt.Errorf("parse RRHH identity correction email template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, rrhhIdentityCorrectionMailData{
		TravellerName:  fullName,
		TravellerEmail: baseData["user_email"],
		Changes:        changes,
	}); err != nil {
		return fmt.Errorf("execute RRHH identity correction email template: %w", err)
	}

	if err := sendEmail(rrhhEmail, "Traveller information marked as incorrect", buf.String()); err != nil {
		return fmt.Errorf("send RRHH identity correction email: %w", err)
	}

	return nil
}
func upsertTravelPhone(userID int, username string, phone string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil
	}

	phonePayload := map[string]interface{}{
		"table":     "people_phone",
		"columns":   "user_phone",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonPhonePayload, _ := json.Marshal(phonePayload)
	respPhone := getReq(jsonPhonePayload, apiKey, client, w)
	if respPhone == nil {
		return fmt.Errorf("no response while reading people_phone")
	}
	defer respPhone.Body.Close()

	if respPhone.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respPhone.Body)
		return fmt.Errorf("people_phone read failed: %d - %s", respPhone.StatusCode, string(bodyBytes))
	}

	var phoneResult []map[string]interface{}
	if err := json.NewDecoder(respPhone.Body).Decode(&phoneResult); err != nil {
		return fmt.Errorf("decode people_phone response: %w", err)
	}

	if len(phoneResult) > 0 {
		currentPhone := strings.TrimSpace(asString(phoneResult[0]["user_phone"]))
		if currentPhone == phone {
			return nil
		}

		updatePhonePayload := map[string]interface{}{
			"table":     "people_phone",
			"columns":   "user_phone",
			"value":     escapeComma(phone),
			"condition": fmt.Sprintf("people_id = %d", userID),
		}
		jsonUpdatePhone, _ := json.Marshal(updatePhonePayload)
		respUpdatePhone := putReq(jsonUpdatePhone, apiKey, client, w)
		if respUpdatePhone == nil {
			return fmt.Errorf("no response while updating people_phone")
		}
		defer respUpdatePhone.Body.Close()

		if respUpdatePhone.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(respUpdatePhone.Body)
			return fmt.Errorf("people_phone update failed: %d - %s", respUpdatePhone.StatusCode, string(bodyBytes))
		}

		return nil
	}

	insertPhonePayload := map[string]interface{}{
		"table":   "people_phone",
		"columns": "people_id, user_phone",
		"value":   fmt.Sprintf("%d, %s", userID, escapeComma(phone)),
	}
	jsonInsertPhone, _ := json.Marshal(insertPhonePayload)
	respInsertPhone := postReq(jsonInsertPhone, apiKey, client, w)
	if respInsertPhone == nil {
		return fmt.Errorf("no response while inserting people_phone")
	}
	defer respInsertPhone.Body.Close()

	if respInsertPhone.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respInsertPhone.Body)
		return fmt.Errorf("people_phone insert failed: %d - %s", respInsertPhone.StatusCode, string(bodyBytes))
	}

	createLog(fmt.Sprintf("Travel phone saved for user %s in people_phone table", username), 0, apiKey, client, w)
	return nil
}

func handleGetTravelContactInfo(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	authUserID, username, _ := getUserInfo(apiKey, client, w, r)
	userID := authUserID

	var resp TravelContactInfoResponse
	resp.OK = false

	combinedIDStr := strings.TrimSpace(r.URL.Query().Get("combinedId"))
	if combinedIDStr != "" {
		combinedID, err := strconv.Atoi(combinedIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{
				OK:    false,
				Error: "combinedId is not valid",
			})
			return
		}

		ownerQuery := map[string]interface{}{
			"table":     "budget_requests",
			"columns":   "people_id",
			"condition": fmt.Sprintf("id = %d", combinedID),
		}
		jsonOwnerQuery, _ := json.Marshal(ownerQuery)
		respOwner := getReq(jsonOwnerQuery, apiKey, client, w)
		if respOwner == nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error connecting to database"})
			return
		}
		if respOwner.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(respOwner.Body)
			respOwner.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{
				OK:    false,
				Error: fmt.Sprintf("error response from database: %d - %s", respOwner.StatusCode, string(bodyBytes)),
			})
			return
		}

		var ownerResult []map[string]interface{}
		if err := json.NewDecoder(respOwner.Body).Decode(&ownerResult); err != nil || len(ownerResult) == 0 || ownerResult[0]["people_id"] == nil {
			respOwner.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "request owner not found"})
			return
		}
		respOwner.Body.Close()

		userID = int(ownerResult[0]["people_id"].(float64))
		if userID != authUserID {
			accessFinances := getIsFinances(authUserID, username, apiKey, client, w, r)
			if len(accessFinances) == 0 {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{
					OK:    false,
					Error: "not allowed to view this traveller information",
				})
				createLog(fmt.Sprintf("User %s tried to access travel contact info for request %d owned by people_id %d", username, combinedID, userID), 1, apiKey, client, w)
				return
			}
		}
	}

	// Phone: use the travel-specific table first. If it has no row yet,
	// suggest the legacy phone from people.user_phone without writing it there.
	travelPhoneQuery := map[string]interface{}{
		"table":     "people_phone",
		"columns":   "user_phone",
		"condition": fmt.Sprintf("people_id = %d", userID),
	}
	jsonTravelPhoneQuery, _ := json.Marshal(travelPhoneQuery)
	respTravelPhone := getReq(jsonTravelPhoneQuery, apiKey, client, w)
	if respTravelPhone != nil && respTravelPhone.StatusCode == http.StatusOK {
		var travelPhoneResult []map[string]interface{}
		if err := json.NewDecoder(respTravelPhone.Body).Decode(&travelPhoneResult); err != nil {
			respTravelPhone.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error decoding database response"})
			return
		}
		respTravelPhone.Body.Close()

		if len(travelPhoneResult) > 0 {
			phone := strings.TrimSpace(asString(travelPhoneResult[0]["user_phone"]))
			if phone != "" {
				resp.HasPhone = true
				resp.Phone = phone
				resp.PhoneSource = "people_phone"
			}
		}
	} else if respTravelPhone != nil {
		bodyBytes, _ := io.ReadAll(respTravelPhone.Body)
		respTravelPhone.Body.Close()
		createLog(fmt.Sprintf("Could not read people_phone for user %s. Falling back to people.user_phone. Status: %d - %s", username, respTravelPhone.StatusCode, string(bodyBytes)), 1, apiKey, client, w)
	}

	if !resp.HasPhone {
		phoneQuery := map[string]interface{}{
			"table":     "people",
			"columns":   "user_phone",
			"condition": fmt.Sprintf("id = %d", userID),
		}
		jsonPhoneQuery, _ := json.Marshal(phoneQuery)
		respPhone := getReq(jsonPhoneQuery, apiKey, client, w)
		if respPhone == nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error connecting to database"})
			createLog(fmt.Sprintf("No se obtuvo respuesta para obtener teléfono de user %s", username), 2, apiKey, client, w)
			return
		}
		if respPhone.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(respPhone.Body)
			respPhone.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{
				OK:    false,
				Error: fmt.Sprintf("error response from database: %d - %s", respPhone.StatusCode, string(bodyBytes)),
			})
			return
		}

		var phoneResult []map[string]interface{}
		if err := json.NewDecoder(respPhone.Body).Decode(&phoneResult); err != nil {
			respPhone.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error decoding database response"})
			return
		}
		respPhone.Body.Close()

		if len(phoneResult) > 0 {
			phone := strings.TrimSpace(asString(phoneResult[0]["user_phone"]))
			if phone != "" {
				resp.HasPhone = true
				resp.Phone = phone
				resp.PhoneSource = "people"
			}
		}
	}

	// documents
	today := time.Now().Format("2006-01-02")
	docQuery := map[string]interface{}{
		"table":     "people_passport",
		"columns":   "doc_type, doc_number, expiration",
		"condition": fmt.Sprintf("people_id = %d AND expiration IS NOT NULL AND expiration >= '%s'", userID, today),
	}
	jsonDocQuery, _ := json.Marshal(docQuery)
	respDoc := getReq(jsonDocQuery, apiKey, client, w)
	if respDoc == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error connecting to database"})
		return
	}
	if respDoc.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respDoc.Body)
		respDoc.Body.Close()
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{
			OK:    false,
			Error: fmt.Sprintf("error response from database: %d - %s", respDoc.StatusCode, string(bodyBytes)),
		})
		return
	}

	var docs []map[string]interface{}
	if err := json.NewDecoder(respDoc.Body).Decode(&docs); err != nil {
		respDoc.Body.Close()
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error decoding database response"})
		return
	}
	respDoc.Body.Close()

	resp.HasValidDocument = false
	resp.HasValidPassport = false
	resp.HasPassportData = false
	resp.Passport = nil

	var fallback *TravelPassportInfo

	for _, d := range docs {
		info := &TravelPassportInfo{
			DocType:    strings.TrimSpace(asString(d["doc_type"])),
			DocNumber:  strings.TrimSpace(asString(d["doc_number"])),
			Expiration: strings.TrimSpace(asString(d["expiration"])),
		}

		if fallback == nil {
			fallback = info
		}

		if info.DocType == "passport" {
			resp.HasValidPassport = true
			resp.HasValidDocument = true
			resp.HasPassportData = true
			resp.Passport = info
			break
		}
	}

	if resp.Passport == nil && fallback != nil {
		resp.HasValidDocument = true
		resp.HasPassportData = true
		resp.Passport = fallback
	} else if resp.Passport == nil {
		dniQuery := map[string]interface{}{
			"table":     "people",
			"columns":   "nif",
			"condition": fmt.Sprintf("id = %d", userID),
		}
		jsonDniQuery, _ := json.Marshal(dniQuery)
		respDni := getReq(jsonDniQuery, apiKey, client, w)
		if respDni == nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error connecting to database"})
			return
		}
		if respDni.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(respDni.Body)
			respDni.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{
				OK:    false,
				Error: fmt.Sprintf("error response from database: %d - %s", respDni.StatusCode, string(bodyBytes)),
			})
			return
		}

		var dniResult []map[string]interface{}
		if err := json.NewDecoder(respDni.Body).Decode(&dniResult); err != nil {
			respDni.Body.Close()
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(TravelContactInfoResponse{OK: false, Error: "error decoding database response"})
			return
		}
		respDni.Body.Close()

		if len(dniResult) > 0 {
			dni := strings.TrimSpace(asString(dniResult[0]["nif"]))
			if dni != "" {
				resp.HasPassportData = true
				resp.HasValidDocument = false
				resp.HasValidPassport = false
				resp.Passport = &TravelPassportInfo{
					DocNumber: dni,
				}
			}
		}

	}

	resp.OK = true
	_ = json.NewEncoder(w).Encode(resp)
}

// ---------------- EMAILS: SUBMIT FLOW ----------------

func sendIPConfirmationEmail(
	ipEmail string,
	mailData IPConfirmationMailData,
) error {
	tmplContent, err := os.ReadFile("module8workers/assets/ipConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read IP confirmation template: %w", err)
	}

	tmpl, err := template.New("ipConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse IP confirmation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute IP confirmation template: %w", err)
	}

	if err := sendEmail(ipEmail, "New Budget Request Pending Review", buf.String()); err != nil {
		return fmt.Errorf("failed to send IP confirmation email: %w", err)
	}

	return nil
}

func sendWorkerConfirmationEmail(
	workerEmail string,
	mailData WorkerConfirmationMailData,
) error {
	tmplContent, err := os.ReadFile("module8workers/assets/workerConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read worker confirmation template: %w", err)
	}

	tmpl, err := template.New("workerConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse worker confirmation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute worker confirmation template: %w", err)
	}

	if err := sendEmail(workerEmail, "Budget Request Successfully Recorded", buf.String()); err != nil {
		return fmt.Errorf("failed to send worker confirmation email: %w", err)
	}

	return nil
}

func sendSubmitRequestEmails(
	userID int,
	requestData RequestPayload,
	created string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	workerInfo, err := getPersonInfoByID(userID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting worker info: %w", err)
	}

	targets := extractMailTargets(requestData)

	seenIPs := make(map[string]bool)
	var workerProjects []string
	var workerPIs []string
	var autoApprovedTargets []RequestMailTarget

	for _, t := range targets {
		if t.ProjectID != "" {
			projectInfo, err := getProjectInfoByID(t.ProjectID, apiKey, client, w)
			if err != nil {
				createLog(fmt.Sprintf("Warning: could not load project %s: %v", t.ProjectID, err), 1, apiKey, client, w)
				continue
			}

			if !containsString(workerProjects, projectInfo.Name) {
				workerProjects = append(workerProjects, projectInfo.Name)
			}

			if t.IPID > 0 {
				ipInfo, err := getPersonInfoByID(t.IPID, apiKey, client, w)
				if err != nil {
					createLog(fmt.Sprintf("Warning: could not load PI %d: %v", t.IPID, err), 1, apiKey, client, w)
					continue
				}

				if !containsString(workerPIs, strings.TrimSpace(ipInfo.Name+" "+ipInfo.Surname)) {
					workerPIs = append(workerPIs, strings.TrimSpace(ipInfo.Name+" "+ipInfo.Surname))
				}

				if ipInfo.ID == workerInfo.ID {
					autoApprovedTargets = append(autoApprovedTargets, t)
					continue
				}

				if ipInfo.Email != "" {
					key := fmt.Sprintf("%s|%s", ipInfo.Email, t.ProjectID)
					if !seenIPs[key] {
						mailData := IPConfirmationMailData{
							PIName:            ipInfo.Name,
							PISurname:         ipInfo.Surname,
							ResearcherName:    workerInfo.Name,
							ResearcherSurname: workerInfo.Surname,
							ProjectName:       projectInfo.Name,
							CreatedAt:         created,
							RequestSummary:    t.Summary,
						}

						if err := sendIPConfirmationEmail(ipInfo.Email, mailData); err != nil {
							createLog(fmt.Sprintf("Error sending IP email to %s: %v", ipInfo.Email, err), 1, apiKey, client, w)
						} else {
							seenIPs[key] = true
						}
					}
				}

				if err := sendNotification(
					"Budget Request Submitted",
					escapeComma("A new budget request related to one of your projects has been submitted. Please review and manage it in the Budgeting module."),
					ipInfo.ID, apiKey, client, w,
				); err != nil {
					return fmt.Errorf("sending notification: %w", err)
				}
			}
		}
	}

	workerMailData := WorkerConfirmationMailData{
		Name:                  workerInfo.Name,
		Surname:               workerInfo.Surname,
		RequestProject:        strings.Join(workerProjects, ", "),
		PrincipalInvestigator: strings.Join(workerPIs, ", "),
		CreatedAt:             created,
		RequestSummary:        buildGlobalRequestSummary(requestData),
	}

	if workerInfo.Email != "" {
		if err := sendWorkerConfirmationEmail(workerInfo.Email, workerMailData); err != nil {
			return fmt.Errorf("sending worker confirmation email: %w", err)
		}
	}

	if len(autoApprovedTargets) > 0 {
		if err := sendProjectsEmailForTargets(workerInfo.ID, workerInfo.ID, autoApprovedTargets, requestData, created, apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Request auto-approved by PI for user %s but projects email failed: %v", workerInfo.Email, err), 1, apiKey, client, w)
		}
		if autoApprovedTargetsRequireManagement(autoApprovedTargets, requestData) {
			if err := sendManagementEmailAfterIPAcceptance(workerInfo.ID, workerInfo.ID, requestData, created, apiKey, client, w); err != nil {
				createLog(fmt.Sprintf("Request auto-approved by PI for user %s but management email failed: %v", workerInfo.Email, err), 1, apiKey, client, w)
			}
		}
	}

	if err := sendNotification(
		"Budget Request Submitted",
		"Your budget request has been successfully submitted and will be processed as soon as possible. You will receive another notification once it has been resolved.",
		workerInfo.ID, apiKey, client, w,
	); err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}

	return nil
}

func autoApprovedTargetsRequireManagement(mailTargets []RequestMailTarget, requestData RequestPayload) bool {
	hasOther := false
	hasEquipment := false

	for _, t := range mailTargets {
		switch strings.TrimSpace(strings.ToLower(t.Category)) {
		case "other":
			hasOther = true
		case "equipment":
			hasEquipment = true
		}
	}

	if hasOther && priceRangeRequiresProcurement(requestData.OtherPriceRange) {
		return true
	}

	if hasEquipment &&
		strings.TrimSpace(requestData.EquipmentPriceRange) != "" &&
		priceRangeRequiresProcurement(requestData.EquipmentPriceRange) {
		return true
	}

	return false
}

// ---------------- EMAILS: PROJECTS / GERENCIA FLOW ----------------

func sendProjectsEmailForTargets(
	researcherID int,
	ipID int,
	mailTargets []RequestMailTarget,
	requestData RequestPayload,
	updatedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	researcherInfo, err := getPersonInfoByID(researcherID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting researcher info: %w", err)
	}

	ipInfo, err := getPersonInfoByID(ipID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting PI info: %w", err)
	}

	if len(mailTargets) == 0 {
		return fmt.Errorf("no mail targets extracted from request")
	}

	type groupedProjectData struct {
		SummaryParts    []string
		HasRegistration bool
	}

	grouped := make(map[string]*groupedProjectData)
	projectOrder := make([]string, 0)

	for _, t := range mailTargets {
		if t.ProjectID == "" {
			continue
		}
		if _, ok := grouped[t.ProjectID]; !ok {
			grouped[t.ProjectID] = &groupedProjectData{}
			projectOrder = append(projectOrder, t.ProjectID)
		}
		if strings.TrimSpace(t.Summary) != "" {
			grouped[t.ProjectID].SummaryParts = append(grouped[t.ProjectID].SummaryParts, t.Summary)
		}
		if t.Category == "registration" {
			grouped[t.ProjectID].HasRegistration = true
		}
	}

	for _, projectID := range projectOrder {
		projectInfo, err := getProjectInfoByID(projectID, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Warning: could not load project %s: %v", projectID, err), 1, apiKey, client, w)
			continue
		}

		projectData := grouped[projectID]
		mailData := ProjectsConfirmationMailData{
			ProjectsName:      "Projects",
			ProjectsSurname:   "Team",
			ResearcherName:    unescapeComma(researcherInfo.Name),
			ResearcherSurname: unescapeComma(researcherInfo.Surname),
			PIName:            unescapeComma(ipInfo.Name),
			PISurname:         unescapeComma(ipInfo.Surname),
			ProjectName:       unescapeComma(projectInfo.Name),
			UpdatedAt:         updatedAt,
			RequestSummary:    strings.Join(projectData.SummaryParts, " || "),
		}

		if projectData.HasRegistration {
			mailData.RegistrationEvent = unescapeComma(requestData.RegistrationEvent)
			mailData.RegistrationObservations = unescapeComma(requestData.RegistrationObservations)
			mailData.RegistrationHasSupportingFile = strings.EqualFold(strings.TrimSpace(requestData.RegistrationFile), "yes")
			mailData.RegistrationRequiresMeeting = !mailData.RegistrationHasSupportingFile
		}

		if err := sendProjectsConfirmationEmail(mailData); err != nil {
			return fmt.Errorf("sending projects confirmation email for project %s: %w", projectID, err)
		}
	}

	researcherFullName := strings.TrimSpace(researcherInfo.Name + " " + researcherInfo.Surname)
	if err := sendProjectsAccessNotifications(researcherFullName, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Projects notification warning for researcher %s: %v", researcherFullName, err), 1, apiKey, client, w)
	}

	return nil
}

func sendGerenciaEmailForTargets(
	researcherID int,
	ipID int,
	mailTargets []RequestMailTarget,
	requestData RequestPayload,
	updatedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	researcherInfo, err := getPersonInfoByID(researcherID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting researcher info: %w", err)
	}

	ipInfo, err := getPersonInfoByID(ipID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting PI info: %w", err)
	}

	if len(mailTargets) == 0 {
		return fmt.Errorf("no mail targets extracted from request")
	}

	type groupedProjectData struct {
		SummaryParts    []string
		HasRegistration bool
	}

	grouped := make(map[string]*groupedProjectData)
	projectOrder := make([]string, 0)

	for _, t := range mailTargets {
		if t.ProjectID == "" {
			continue
		}
		if _, ok := grouped[t.ProjectID]; !ok {
			grouped[t.ProjectID] = &groupedProjectData{}
			projectOrder = append(projectOrder, t.ProjectID)
		}
		if strings.TrimSpace(t.Summary) != "" {
			grouped[t.ProjectID].SummaryParts = append(grouped[t.ProjectID].SummaryParts, t.Summary)
		}
		if t.Category == "registration" {
			grouped[t.ProjectID].HasRegistration = true
		}
	}

	for _, projectID := range projectOrder {
		projectInfo, err := getProjectInfoByID(projectID, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Warning: could not load project %s: %v", projectID, err), 1, apiKey, client, w)
			continue
		}

		projectData := grouped[projectID]
		mailData := ProjectsConfirmationMailData{
			ProjectsName:      "Gerència",
			ResearcherName:    unescapeComma(researcherInfo.Name),
			ResearcherSurname: unescapeComma(researcherInfo.Surname),
			PIName:            unescapeComma(ipInfo.Name),
			PISurname:         unescapeComma(ipInfo.Surname),
			ProjectName:       unescapeComma(projectInfo.Name),
			UpdatedAt:         updatedAt,
			RequestSummary:    strings.Join(projectData.SummaryParts, " || "),
		}

		if projectData.HasRegistration {
			mailData.RegistrationEvent = unescapeComma(requestData.RegistrationEvent)
			mailData.RegistrationObservations = unescapeComma(requestData.RegistrationObservations)
			mailData.RegistrationHasSupportingFile = strings.EqualFold(strings.TrimSpace(requestData.RegistrationFile), "yes")
			mailData.RegistrationRequiresMeeting = !mailData.RegistrationHasSupportingFile
		}

		if err := sendGerenciaConfirmationEmail(mailData); err != nil {
			return fmt.Errorf("sending gerencia confirmation email for project %s: %w", projectID, err)
		}
	}

	return nil
}

func sendProjectsAccessNotifications(researcherFullName string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	projectUserIDs, err := getProjectsUsersForNotifications(apiKey, client, w)
	if err != nil {
		return err
	}

	for _, projectUserID := range projectUserIDs {
		if err := sendNotification(
			"Budget Request pending your management",
			escapeComma(fmt.Sprintf("A budget request from %s has been accepted by the Principal Investigator and is now pending Projects management. You can review it in the Budgeting module.", researcherFullName)),
			projectUserID,
			apiKey,
			client,
			w,
		); err != nil {
			createLog(fmt.Sprintf("Failed to send Projects notification to user %d: %v", projectUserID, err), 1, apiKey, client, w)
		}
	}

	return nil
}

func sendProjectsConfirmationEmail(
	mailData ProjectsConfirmationMailData,
) error {
	tmplContent, err := os.ReadFile("module8workers/assets/projectsConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read projects confirmation template: %w", err)
	}

	tmpl, err := template.New("projectsConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse projects confirmation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute projects confirmation template: %w", err)
	}

	if err := sendEmail(projectsEmail, "Budget Request Accepted by Responsible", buf.String()); err != nil {
		return fmt.Errorf("failed to send projects confirmation email: %w", err)
	}

	return nil
}

func sendGerenciaConfirmationEmail(
	mailData ProjectsConfirmationMailData,
) error {

	mailData.ProjectsName = "Gerència"
	mailData.ProjectsSurname = ""
	tmplContent, err := os.ReadFile("module8workers/assets/projectsConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read projects confirmation template: %w", err)
	}

	tmpl, err := template.New("projectsConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse projects confirmation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute projects confirmation template: %w", err)
	}

	if err := sendEmail(managementEmail, "Budget Request Accepted by Responsible", buf.String()); err != nil {
		return fmt.Errorf("failed to send projects confirmation email: %w", err)
	}

	return nil
}

func sendProjectsEmailAfterIPAcceptance(
	researcherID int,
	ipID int,
	requestData RequestPayload,
	updatedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	mailTargets := extractMailTargets(requestData)
	return sendProjectsEmailForTargets(researcherID, ipID, mailTargets, requestData, updatedAt, apiKey, client, w)
}

func sendGerenciaEmailAfterIPAcceptance(
	researcherID int,
	ipID int,
	requestData RequestPayload,
	updatedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	mailTargets := extractMailTargets(requestData)
	return sendGerenciaEmailForTargets(researcherID, ipID, mailTargets, requestData, updatedAt, apiKey, client, w)
}

// ---------------- EMAILS: MANAGEMENT / PROCUREMENT ----------------

func requiresManagementNotification(requestData RequestPayload) bool {
	if contains(requestData.Categories, "other") && priceRangeRequiresProcurement(requestData.OtherPriceRange) {
		return true
	}

	// Importante: si equipment_price_range viene vacío, NO avisar
	if contains(requestData.Categories, "equipment") &&
		strings.TrimSpace(requestData.EquipmentPriceRange) != "" &&
		priceRangeRequiresProcurement(requestData.EquipmentPriceRange) {
		return true
	}

	return false
}

func priceRangeRequiresProcurement(priceRange string) bool {
	switch strings.TrimSpace(priceRange) {
	case "2", "3", "4":
		return true
	default:
		return false
	}
}

func buildManagementMailData(
	researcherInfo *PersonInfo,
	ipInfo *PersonInfo,
	requestData RequestPayload,
	updatedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (ManagementConfirmationMailData, error) {
	var mailData ManagementConfirmationMailData

	mailData.ManagementName = "Gerència"
	mailData.ResearcherName = unescapeComma(researcherInfo.Name)
	mailData.ResearcherSurname = unescapeComma(researcherInfo.Surname)
	mailData.PIName = unescapeComma(ipInfo.Name)
	mailData.PISurname = unescapeComma(ipInfo.Surname)
	mailData.UpdatedAt = updatedAt

	if contains(requestData.Categories, "equipment") &&
		strings.TrimSpace(requestData.EquipmentPriceRange) != "" &&
		priceRangeRequiresProcurement(requestData.EquipmentPriceRange) {

		projectInfo, err := getProjectInfoByID(requestData.EquipmentProject, apiKey, client, w)
		if err != nil {
			return mailData, fmt.Errorf("getting equipment project info: %w", err)
		}

		mailData.ProjectName = unescapeComma(projectInfo.Name)
		mailData.CategoryName = "Equipment"
		mailData.PriceRange = parsePriceRange(requestData.EquipmentPriceRange)
		mailData.EquipmentCategory = requestData.EquipmentCategory
		mailData.Purpose = unescapeComma(requestData.EquipmentOtherDescr)
		mailData.Observations = unescapeComma(requestData.EquipmentDescription)
		mailData.RequestSummary = "Equipment request requiring public procurement procedure"

		return mailData, nil
	}

	if contains(requestData.Categories, "other") &&
		priceRangeRequiresProcurement(requestData.OtherPriceRange) {

		projectInfo, err := getProjectInfoByID(requestData.OtherProject, apiKey, client, w)
		if err != nil {
			return mailData, fmt.Errorf("getting other project info: %w", err)
		}

		mailData.ProjectName = unescapeComma(projectInfo.Name)
		mailData.CategoryName = "Other"
		mailData.PriceRange = parsePriceRange(requestData.OtherPriceRange)
		mailData.Observations = unescapeComma(requestData.OtherDescription)
		mailData.RequestSummary = "Other budget request requiring public procurement procedure"

		return mailData, nil
	}

	return mailData, fmt.Errorf("request does not require management notification")
}

func sendManagementConfirmationEmail(mailData ManagementConfirmationMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/managementConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read management confirmation template: %w", err)
	}

	tmpl, err := template.New("managementConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse management confirmation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute management confirmation template: %w", err)
	}

	if err := sendEmail(managementEmail, "Public Procurement Procedure Required", buf.String()); err != nil {
		return fmt.Errorf("failed to send management confirmation email: %w", err)
	}

	return nil
}

func sendManagementEmailAfterIPAcceptance(
	researcherID int,
	ipID int,
	requestData RequestPayload,
	updatedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	if !requiresManagementNotification(requestData) {
		return nil
	}

	researcherInfo, err := getPersonInfoByID(researcherID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting researcher info: %w", err)
	}

	ipInfo, err := getPersonInfoByID(ipID, apiKey, client, w)
	if err != nil {
		return fmt.Errorf("getting PI info: %w", err)
	}

	mailData, err := buildManagementMailData(researcherInfo, ipInfo, requestData, updatedAt, apiKey, client, w)
	if err != nil {
		return err
	}

	if err := sendManagementConfirmationEmail(mailData); err != nil {
		return err
	}

	return nil
}

// ---------------- EMAILS: REJECTION FLOW ----------------

func sendIPRejectedEmail(ipEmail string, mailData IPRejectedMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/ipRejected.html")
	if err != nil {
		return fmt.Errorf("failed to read ip rejected template: %w", err)
	}

	tmpl, err := template.New("ipRejected").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse ip rejected template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute ip rejected template: %w", err)
	}

	if err := sendEmail(ipEmail, "Budget Request Rejected", buf.String()); err != nil {
		return fmt.Errorf("failed to send ip rejected email: %w", err)
	}

	return nil
}

func notifyIPsOfRejection(combinedID int, motive, username, apiKey string, client *http.Client, w http.ResponseWriter) error {
	workerInfo, ipRecipients, projectName, requestSummary, message, err := buildRejectedStakeholderContext(combinedID, motive, apiKey, client, w)
	if err != nil {
		return err
	}

	updatedAt := time.Now().Format("2006-01-02 15:04")
	var firstErr error

	for _, ip := range ipRecipients {
		if ip.ID == workerInfo.ID {
			continue
		}

		if strings.TrimSpace(ip.Email) == "" {
			continue
		}

		mailData := IPRejectedMailData{
			PIName:            ip.Name,
			PISurname:         ip.Surname,
			ResearcherName:    workerInfo.Name,
			ResearcherSurname: workerInfo.Surname,
			ProjectName:       projectName,
			UpdatedAt:         updatedAt,
			RequestSummary:    requestSummary,
			Message:           message,
		}

		if err := sendIPRejectedEmail(ip.Email, mailData); err != nil {
			createLog(fmt.Sprintf("Error sending IP rejected email for request %d to %s (%s): %v", combinedID, ip.Email, username, err), 1, apiKey, client, w)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func sendProjectsRejectedEmail(mailData ProjectsRejectedMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/projectsRejected.html")
	if err != nil {
		return fmt.Errorf("failed to read projects rejected template: %w", err)
	}

	tmpl, err := template.New("projectsRejected").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse projects rejected template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute projects rejected template: %w", err)
	}

	if err := sendEmail(projectsEmail, "Budget Request Rejected in Final Step", buf.String()); err != nil {
		return fmt.Errorf("failed to send projects rejected email: %w", err)
	}

	return nil
}

func notifyProjectsOfFinalRejection(combinedID int, motive, username, apiKey string, client *http.Client, w http.ResponseWriter) error {
	workerInfo, ipRecipients, projectName, requestSummary, message, err := buildRejectedStakeholderContext(combinedID, motive, apiKey, client, w)
	if err != nil {
		return err
	}

	piNames := make([]string, 0, len(ipRecipients))
	for _, ip := range ipRecipients {
		fullName := strings.TrimSpace(ip.Name + " " + ip.Surname)
		if fullName != "" {
			piNames = append(piNames, fullName)
		}
	}

	piCombined := strings.Join(piNames, ", ")
	updatedAt := time.Now().Format("2006-01-02 15:04")

	mailData := ProjectsRejectedMailData{
		ProjectsName:      "Projects",
		ProjectsSurname:   "Team",
		ResearcherName:    workerInfo.Name,
		ResearcherSurname: workerInfo.Surname,
		PIName:            piCombined,
		PISurname:         "",
		ProjectName:       projectName,
		UpdatedAt:         updatedAt,
		RequestSummary:    requestSummary,
		Message:           message,
	}

	if err := sendProjectsRejectedEmail(mailData); err != nil {
		createLog(fmt.Sprintf("Error sending Projects rejected email for request %d by %s: %v", combinedID, username, err), 1, apiKey, client, w)
		return err
	}

	return nil
}

func getPersonNameByID(personID int, apiKey string, client *http.Client, w http.ResponseWriter) (string, string) {
	personInfo, err := getPersonInfoByID(personID, apiKey, client, w)
	if err != nil {
		return "", ""
	}

	return personInfo.Name, personInfo.Surname
}

func buildWorkerRejectionMailData(userID int, combinedID int, motive string, apiKey string, client *http.Client, w http.ResponseWriter) (*WorkerRejectionMailData, int, string, error) {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return nil, 0, "", err
	}

	projectNames, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return nil, 0, "", err
	}

	projectName := ""
	if len(projectNames) > 0 {
		projectName = strings.Join(projectNames, ", ")
	}

	requestSummary := ""
	if len(summaryParts) > 0 {
		requestSummary = strings.Join(summaryParts, " || ")
	}

	rejectionName, rejectionSurname := getPersonNameByID(userID, apiKey, client, w)

	updatedAt := time.Now().Format("2006-01-02 15:04")
	data := &WorkerRejectionMailData{
		Name:           workerInfo.Name,
		Surname:        workerInfo.Surname,
		RejName:        rejectionName,
		RejSurname:     rejectionSurname,
		ProjectName:    projectName,
		CreatedAt:      normalizeDateString(workerInfo.CreatedAt),
		UpdatedAt:      updatedAt,
		RequestSummary: requestSummary,
		Message:        unescapeComma(strings.TrimSpace(motive)),
	}

	return data, workerInfo.ID, workerInfo.Email, nil
}

type requestWorkerInfo struct {
	ID        int
	Name      string
	Surname   string
	Email     string
	CreatedAt string
}

type ApprovalTraceItem struct {
	Step string
	Name string
	Date string
}

func templateDataWithApprovalTrace(data interface{}, approvalTrace []ApprovalTraceItem) map[string]interface{} {
	payload := map[string]interface{}{}

	encoded, err := json.Marshal(data)
	if err == nil {
		_ = json.Unmarshal(encoded, &payload)
	}

	payload["ApprovalTrace"] = approvalTrace
	return payload
}

func normalizeApprovalTraceDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			if format == "2006-01-02" {
				return parsed.Format("2006-01-02")
			}
			return parsed.Format("2006-01-02 15:04")
		}
	}

	return value
}

func approvalTraceNameFromPersonID(personID int, apiKey string, client *http.Client, w http.ResponseWriter) string {
	if personID <= 0 {
		return ""
	}

	personInfo, err := getPersonInfoByID(personID, apiKey, client, w)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(unescapeComma(personInfo.Name) + " " + unescapeComma(personInfo.Surname))
}

func approvalTraceProjectApproverNameFromAudit(
	combinedID int,
	actions []string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) string {
	if combinedID <= 0 || len(actions) == 0 {
		return ""
	}

	actionSet := map[string]bool{}
	for _, action := range actions {
		action = strings.TrimSpace(action)
		if action != "" {
			actionSet[action] = true
		}
	}
	if len(actionSet) == 0 {
		return ""
	}

	query := map[string]interface{}{
		"table":     "budget_audit_logs",
		"columns":   "id, payload",
		"condition": fmt.Sprintf("combined_id = %d ORDER BY id DESC LIMIT 100", combinedID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return ""
	}

	for _, row := range rows {
		payloadText, err := decryptBudgetAuditPayload(unescapeComma(asString(row["payload"])))
		if err != nil {
			continue
		}

		var payload BudgetAuditPayload
		if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
			continue
		}
		if !actionSet[payload.Action] {
			continue
		}

		name := strings.TrimSpace(unescapeComma(payload.Actor.Name) + " " + unescapeComma(payload.Actor.Surname))
		if name != "" {
			return name
		}

		name = approvalTraceNameFromPersonID(payload.Actor.PeopleID, apiKey, client, w)
		if name != "" {
			return name
		}
	}

	return ""
}

func buildApprovalTrace(
	combinedID int,
	projectApproverID int,
	finalStep string,
	finalApproverID int,
	finalDate string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) ([]ApprovalTraceItem, error) {
	query := map[string]interface{}{
		"table": `budget_requests br
			LEFT JOIN people worker ON br.people_id = worker.id
			LEFT JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN people ip ON bp.ip_id = ip.id
			LEFT JOIN projects pr ON bp.project_id = pr.id`,
		"columns": strings.Join([]string{
			"br.people_id AS worker_id",
			"worker.name AS worker_name",
			"worker.surname AS worker_surname",
			"br.projects_response_date",
			"br.acc_or_it_response_date",
			"bp.ip_id",
			"bp.ip_response_date",
			"ip.name AS ip_name",
			"ip.surname AS ip_surname",
			"pr.fons_romanents",
		}, ", "),
		"condition": fmt.Sprintf("br.id = %d", combinedID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get approval trace")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("approval trace query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	trace := []ApprovalTraceItem{}
	seenResponsibles := map[string]bool{}

	workerID := asInt(rows[0]["worker_id"])
	workerFullName := strings.TrimSpace(unescapeComma(asString(rows[0]["worker_name"])) + " " + unescapeComma(asString(rows[0]["worker_surname"])))

	for _, row := range rows {
		ipID := asInt(row["ip_id"])
		if ipID == 0 {
			continue
		}

		key := strconv.Itoa(ipID)
		if seenResponsibles[key] {
			continue
		}
		seenResponsibles[key] = true

		name := strings.TrimSpace(unescapeComma(asString(row["ip_name"])) + " " + unescapeComma(asString(row["ip_surname"])))
		if name == "" && ipID == workerID {
			name = workerFullName
		}
		if name == "" {
			name = fmt.Sprintf("ID %d", ipID)
		}

		trace = append(trace, ApprovalTraceItem{
			Step: "Responsible",
			Name: name,
			Date: normalizeApprovalTraceDate(asString(row["ip_response_date"])),
		})
	}

	hasFonsRomanents := false
	for _, row := range rows {
		if asInt(row["fons_romanents"]) == 1 {
			hasFonsRomanents = true
			break
		}
	}

	projectStepDate := normalizeApprovalTraceDate(asString(rows[0]["projects_response_date"]))
	if projectStepDate != "" {
		step := "Projects"
		fallbackName := "Projects team"
		if hasFonsRomanents {
			step = "Gerència"
			fallbackName = "Gerència"
		}

		approverName := approvalTraceNameFromPersonID(projectApproverID, apiKey, client, w)
		if approverName == "" {
			actions := []string{"aprovat_projectes"}
			if hasFonsRomanents {
				actions = []string{"aprovat_gerencia", "aprovat_gerencia_projectes"}
			}
			approverName = approvalTraceProjectApproverNameFromAudit(combinedID, actions, apiKey, client, w)
		}
		if approverName == "" {
			approverName = fallbackName
		}

		trace = append(trace, ApprovalTraceItem{
			Step: step,
			Name: approverName,
			Date: projectStepDate,
		})
	}

	if strings.TrimSpace(finalStep) != "" {
		finalApproverName := approvalTraceNameFromPersonID(finalApproverID, apiKey, client, w)
		if finalApproverName == "" {
			finalApproverName = strings.TrimSpace(finalStep + " team")
		}

		date := normalizeApprovalTraceDate(finalDate)
		if date == "" {
			date = normalizeApprovalTraceDate(asString(rows[0]["acc_or_it_response_date"]))
		}

		trace = append(trace, ApprovalTraceItem{
			Step: finalStep,
			Name: finalApproverName,
			Date: date,
		})
	}

	return trace, nil
}

func getRequestWorkerInfo(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (*requestWorkerInfo, error) {
	workerInfo := map[string]interface{}{
		"table":     "budget_requests br LEFT JOIN people p ON br.people_id = p.id",
		"columns":   "p.id, p.name, p.surname, p.crm_email, p.user_email, br.creation_date",
		"condition": fmt.Sprintf("br.id = %d", combinedID),
	}
	jsonWorkerInfo, _ := json.Marshal(workerInfo)
	resp := getReq(jsonWorkerInfo, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get worker info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status getting worker info: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var workerInfoResult []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&workerInfoResult); err != nil {
		return nil, fmt.Errorf("failed to decode worker info: %w", err)
	}
	if len(workerInfoResult) == 0 {
		return nil, fmt.Errorf("no worker info found")
	}

	row := workerInfoResult[0]
	email := ""
	if toString(row["crm_email"]) != "" {
		email = toString(row["crm_email"])
	} else if isAllowedUserDomain(toString(row["user_email"])) {
		email = toString(row["user_email"])
	}

	return &requestWorkerInfo{
		ID:        asInt(row["id"]),
		Name:      unescapeComma(toString(row["name"])),
		Surname:   unescapeComma(toString(row["surname"])),
		Email:     email,
		CreatedAt: toString(row["creation_date"]),
	}, nil
}

func notifyWorkerOfRejection(userID int, combinedID int, motive, username, rejectionStage, rejectionRole, rejectedAt, apiKey string, client *http.Client, w http.ResponseWriter) error {
	mailData, workerID, workerEmail, err := buildWorkerRejectionMailData(userID, combinedID, motive, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("No se pudo construir el correo de rechazo para solicitud %d por %s: %v", combinedID, username, err), 1, apiKey, client, w)
		return err
	}

	created := time.Now().Format("2006-01-02 15:04")

	notifBody := escapeComma("Your budget request has been rejected. Please, access the Budgeting module to review the reason and make any necessary changes.")
	query := map[string]interface{}{
		"table":   "notifications",
		"columns": "type, title, content, user_id, created_at",
		"value":   fmt.Sprintf("Budget, Request denied, %s, 304, %s", notifBody, created),
	}

	postJSON, _ := json.Marshal(query)
	resp := postReq(postJSON, apiKey, client, w)
	if resp == nil {
		createLog("createTicket notification: postReq returned nil", 1, apiKey, client, w)
		http.Error(w, "DB request failed", http.StatusInternalServerError)
		return fmt.Errorf("failed to send notification")
	}
	defer resp.Body.Close()

	getID := map[string]interface{}{
		"table":     "notifications",
		"columns":   "id",
		"condition": fmt.Sprintf("user_id = '304' AND created_at = '%s' AND type = 'Budget'", created),
	}
	idJSON, _ := json.Marshal(getID)
	idResp := getReq(idJSON, apiKey, client, w)
	if idResp == nil {
		createLog("Failed to retrieve new notification ID from database", 1, apiKey, client, w)
		http.Error(w, "Failed to get notification ID", http.StatusInternalServerError)
		return fmt.Errorf("failed to get notification ID")
	}
	defer idResp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(idResp.Body).Decode(&rows); err != nil || len(rows) == 0 {
		return fmt.Errorf("failed to decode notification ID")
	}
	notifID := int(rows[0]["id"].(float64))

	link := map[string]interface{}{
		"table":   "notification_user",
		"columns": "notification_id, user_id, can_read, can_download, seen",
		"value":   fmt.Sprintf("%d, %d, 1, 0, 0", notifID, workerID),
	}
	jsonLink, _ := json.Marshal(link)
	resp = postReq(jsonLink, apiKey, client, w)
	if resp == nil {
		createLog("Failed to send notification", 1, apiKey, client, w)
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return fmt.Errorf("failed to send notification")
	}

	if workerEmail != "" {
		tmplContent, err := os.ReadFile("module8workers/assets/budgetRejectionWorker.html")
		if err != nil {
			return fmt.Errorf("failed to read rejection template: %w", err)
		}

		tmpl, err := template.New("budgetRejectionWorker").Parse(string(tmplContent))
		if err != nil {
			return fmt.Errorf("failed to parse rejection template: %w", err)
		}

		approvalTrace, traceErr := buildRejectionApprovalTrace(combinedID, userID, rejectionStage, rejectionRole, rejectedAt, apiKey, client, w)
		if traceErr != nil {
			createLog(fmt.Sprintf("Could not build rejection approval trace for worker email of request %d: %v", combinedID, traceErr), 1, apiKey, client, w)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, templateDataWithApprovalTrace(mailData, approvalTrace)); err != nil {
			return fmt.Errorf("failed to execute rejection template: %w", err)
		}

		if err := sendEmail(workerEmail, "Budget Request Rejected", buf.String()); err != nil {
			return fmt.Errorf("failed to send rejection email: %w", err)
		}
	}

	return nil
}

func buildRejectionApprovalTrace(
	combinedID int,
	rejecterID int,
	rejectionStage string,
	rejectionRole string,
	rejectedAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) ([]ApprovalTraceItem, error) {
	switch rejectionStage {
	case "responsible":
		return buildApprovalTrace(combinedID, 0, "", 0, "", apiKey, client, w)
	case "projects":
		return buildApprovalTrace(combinedID, rejecterID, "", 0, "", apiKey, client, w)
	case "acc_or_it":
		finalStep := "Accounting"
		if rejectionRole == "it" {
			finalStep = "IT"
		}
		return buildApprovalTrace(combinedID, 0, finalStep, rejecterID, rejectedAt, apiKey, client, w)
	default:
		return buildApprovalTrace(combinedID, 0, "", 0, "", apiKey, client, w)
	}
}

// ---------------- EMAILS: ACCOUNTING / IT FINAL APPROVAL ----------------

func notifAccountingAccepted(combinedID int, requestData RequestPayload, acceptedAt string, projectApproverID int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	// ------------------------------------------------------------
	// 1) Get requester info
	// ------------------------------------------------------------
	workerQuery := map[string]interface{}{
		"table":     "budget_requests br LEFT JOIN people p ON br.people_id = p.id",
		"columns":   "p.id, p.name, p.surname, p.secondSurname, p.crm_email, p.user_email, p.birth_date, p.user_phone",
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
	researcherBirthDate := normalizeDateString(toString(workerRows[0]["birth_date"]))
	researcherPhone := toString(workerRows[0]["user_phone"])

	researcherFullName := strings.TrimSpace(researcherName + " " + researcherSurname)
	if researcherSecondSurname != "" {
		researcherFullName = strings.TrimSpace(researcherName + " " + researcherSurname + " " + researcherSecondSurname)
	}

	researcherEmail := asString(workerRows[0]["crm_email"])
	userEmail := asString(workerRows[0]["user_email"])

	chosenEmail := "N/A"
	if researcherEmail != "" {
		chosenEmail = researcherEmail
	} else if userEmail != "" && isAllowedUserDomain(userEmail) {
		chosenEmail = userEmail
	}

	// ------------------------------------------------------------
	// 2) Get request parts with all needed fields
	// ------------------------------------------------------------
	partsQuery := map[string]interface{}{
		"table": "budget_parts bp " +
			"LEFT JOIN request_categories bc ON bp.category_id = bc.id " +
			"LEFT JOIN projects pr ON bp.project_id = pr.id " +
			"LEFT JOIN people ip ON bp.ip_id = ip.id",
		"columns": "bp.category_id, bc.category AS category_name, " +
			"bp.project_id, pr.short_name AS project_name, " +
			"ip.name AS ip_name, ip.surname AS ip_surname, " +
			"bp.purpose, bp.observations, bp.fromDay, bp.untilDay, " +
			"bp.travel_fromPlace, bp.wherePlace, bp.travel_area, " +
			"bp.luggage_type, bp.luggage_weight, bp.seat_preference, bp.time_preference, " +
			"bp.registration_type, bp.registration_invoice, " +
			"bp.other_price",
		"condition": fmt.Sprintf("bp.id_combined = %d", combinedID),
	}

	jsonPartsQuery, _ := json.Marshal(partsQuery)
	respParts := getReq(jsonPartsQuery, apiKey, client, w)
	if respParts == nil {
		return fmt.Errorf("failed to get request parts")
	}
	defer respParts.Body.Close()

	if respParts.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respParts.Body)
		return fmt.Errorf("database error getting request parts: %d - %s", respParts.StatusCode, string(bodyBytes))
	}

	var partRows []map[string]interface{}
	if err := json.NewDecoder(respParts.Body).Decode(&partRows); err != nil {
		return fmt.Errorf("failed to decode request parts: %w", err)
	}

	// ------------------------------------------------------------
	// 3) Detect if any travel is outside Europe
	// ------------------------------------------------------------
	isOutsideEuropeTravel := false
	for _, row := range partRows {
		travelArea := strings.TrimSpace(strings.ToLower(asString(row["travel_area"])))
		if travelArea == "" {
			continue
		}

		if travelArea != "spain" && travelArea != "eu" {
			isOutsideEuropeTravel = true
			break
		}
	}

	// ------------------------------------------------------------
	// 4) Get requester valid document / passport
	//    If travel is outside Europe -> force passport
	// ------------------------------------------------------------
	researcherDocumentType := ""
	researcherDocumentNumber := ""
	researcherDocumentExpiration := ""

	docCondition := fmt.Sprintf("people_id = %d ORDER BY expiration DESC", researcherID)

	if isOutsideEuropeTravel {
		docCondition = fmt.Sprintf(
			"people_id = %d AND doc_type='passport' ORDER BY expiration DESC",
			researcherID,
		)
	}

	docQuery := map[string]interface{}{
		"table":     "people_passport",
		"columns":   "doc_type, doc_number, expiration",
		"condition": docCondition,
	}

	jsonDocQuery, _ := json.Marshal(docQuery)
	respDoc := getReq(jsonDocQuery, apiKey, client, w)
	if respDoc != nil {
		defer respDoc.Body.Close()
		if respDoc.StatusCode == http.StatusOK {
			var docRows []map[string]interface{}
			if err := json.NewDecoder(respDoc.Body).Decode(&docRows); err == nil && len(docRows) > 0 {
				researcherDocumentType = asString(docRows[0]["doc_type"])
				researcherDocumentNumber = asString(docRows[0]["doc_number"])
				researcherDocumentExpiration = normalizeDateString(asString(docRows[0]["expiration"]))
			}
		}
	}

	// Si era fuera de Europa y no se encontró pasaporte, mejor dejar trazado
	if isOutsideEuropeTravel && (researcherDocumentNumber == "" || researcherDocumentType == "") {
		createLog(
			fmt.Sprintf("Warning: request %d is outside Europe but no passport was found for researcher %d", combinedID, researcherID),
			1,
			apiKey,
			client,
			w,
		)
	}

	// ------------------------------------------------------------
	// 5) Build budget parts
	// ------------------------------------------------------------
	budgetParts := make([]BudgetPartMailItem, 0, len(partRows))
	requestSummaryParts := make([]string, 0, len(partRows))

	firstProjectName := ""
	firstPIName := ""
	firstPISurname := ""

	for _, row := range partRows {
		categoryName := asString(row["category_name"])
		if strings.TrimSpace(categoryName) == "" {
			categoryName = CATEGORY_ID_TO_KEY[asInt(row["category_id"])]
		}

		projectName := unescapeComma(asString(row["project_name"]))
		ipFullName := unescapeComma(strings.TrimSpace(asString(row["ip_name"]) + " " + asString(row["ip_surname"])))

		if firstProjectName == "" {
			firstProjectName = projectName
		}
		if firstPIName == "" {
			firstPIName = asString(row["ip_name"])
			firstPISurname = asString(row["ip_surname"])
		}

		registrationType := asString(row["registration_type"])
		switch registrationType {
		case "I":
			registrationType = "Invoice"
		case "P":
			registrationType = "Payment document"
		}

		registrationInvoice := asString(row["registration_invoice"])
		switch registrationInvoice {
		case "1":
			registrationInvoice = "Yes"
		case "0":
			registrationInvoice = "No"
		}

		luggageWeight := ""
		if row["luggage_weight"] != nil {
			luggageWeight = strconv.Itoa(asInt(row["luggage_weight"])) + " kg"
		}

		item := BudgetPartMailItem{
			CategoryName:        categoryName,
			ProjectName:         projectName,
			IPName:              ipFullName,
			Purpose:             unescapeComma(toString(row["purpose"])),
			Observations:        unescapeComma(toString(row["observations"])),
			FromDay:             normalizeDateString(toString(row["fromDay"])),
			UntilDay:            normalizeDateString(toString(row["untilDay"])),
			FromPlace:           unescapeComma(toString(row["travel_fromPlace"])),
			WherePlace:          unescapeComma(toString(row["wherePlace"])),
			TravelArea:          parseTravelArea(asString(row["travel_area"])),
			LuggageType:         parseLuggageType(asString(row["luggage_type"])),
			LuggageWeight:       luggageWeight,
			SeatPreference:      parseSeatPreference(asString(row["seat_preference"])),
			TimePreference:      parseTimePreference(asString(row["time_preference"])),
			RegistrationType:    registrationType,
			RegistrationInvoice: registrationInvoice,
			OtherPrice:          parsePriceRange(asString(row["other_price"])),
		}
		budgetParts = append(budgetParts, item)

		if categoryName != "" {
			requestSummaryParts = append(requestSummaryParts, categoryName)
		}
	}

	if firstProjectName == "" {
		firstProjectName = requestData.TravelProject
	}
	if acceptedAt == "" {
		acceptedAt = time.Now().Format("2006-01-02 15:04")
	}

	mailData := BudgetAcceptedByProjectsMailData{
		Name:                         "Accounting",
		Surname:                      "Team",
		ResearcherName:               researcherName,
		ResearcherSurname:            strings.TrimSpace(researcherSurname + " " + researcherSecondSurname),
		PIName:                       firstPIName,
		PISurname:                    firstPISurname,
		ProjectName:                  firstProjectName,
		UpdatedAt:                    acceptedAt,
		RequestSummary:               strings.Join(requestSummaryParts, ", "),
		ResearcherDocumentType:       researcherDocumentType,
		ResearcherDocumentNumber:     researcherDocumentNumber,
		ResearcherDocumentExpiration: researcherDocumentExpiration,
		ResearcherBirthDate:          researcherBirthDate,
		ResearcherPhone:              researcherPhone,
		ResearcherCorporateEmail:     chosenEmail,
		BudgetParts:                  budgetParts,
	}

	// ------------------------------------------------------------
	// 6) Load template
	// ------------------------------------------------------------
	tmplContent, err := os.ReadFile("module8workers/assets/accConfirmation.html")
	if err != nil {
		return fmt.Errorf("failed to read accounting confirmation template: %w", err)
	}

	tmpl, err := template.New("accConfirmation").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse accounting confirmation template: %w", err)
	}

	approvalTrace, traceErr := buildApprovalTrace(combinedID, projectApproverID, "", 0, "", apiKey, client, w)
	if traceErr != nil {
		createLog(fmt.Sprintf("Could not build approval trace for accounting email of request %d: %v", combinedID, traceErr), 1, apiKey, client, w)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateDataWithApprovalTrace(mailData, approvalTrace)); err != nil {
		return fmt.Errorf("failed to execute accounting confirmation template: %w", err)
	}

	// ------------------------------------------------------------
	// 7) Send email to generic accounting inbox
	// ------------------------------------------------------------
	if err := sendEmail(accountingEmail, "New Budget Request pending your management", buf.String()); err != nil {
		return fmt.Errorf("failed to send accounting email: %w", err)
	}

	// get accounting users
	accountsQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": "access_finances = 1",
	}
	jsonAccountsQuery, _ := json.Marshal(accountsQuery)
	respAccounts := getReq(jsonAccountsQuery, apiKey, client, w)
	if respAccounts == nil {
		createLog(fmt.Sprintf("Failed to get accounting users for notification of request %d", combinedID), 1, apiKey, client, w)
		return nil
	}
	defer respAccounts.Body.Close()

	if respAccounts.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAccounts.Body)
		createLog(fmt.Sprintf("Database error getting accounting users for notification of request %d: %d - %s", combinedID, respAccounts.StatusCode, string(bodyBytes)), 1, apiKey, client, w)
		return nil
	}

	var accountRows []map[string]interface{}
	if err := json.NewDecoder(respAccounts.Body).Decode(&accountRows); err != nil {
		createLog(fmt.Sprintf("Failed to decode accounting users for notification of request %d: %v", combinedID, err), 1, apiKey, client, w)
		return nil
	}

	for _, row := range accountRows {
		if err := sendNotification("New Budget Request pending your management",
			"A new budget request has been aproved by Projects. You might visualize and manage it from the Budgeting module.",
			asInt(row["people_id"]), apiKey, client, w); err != nil {
			createLog(fmt.Sprintf("Failed to send notification to user %d for request %d: %v", asInt(row["people_id"]), combinedID, err), 1, apiKey, client, w)
		}
	}

	createLog(
		fmt.Sprintf("Accounting notification sent for request %d (%s)", combinedID, researcherFullName),
		2,
		apiKey,
		client,
		w,
	)

	return nil
}

func sendWorkerLastEmail(workerEmail string, mailData WorkerLastMailData, approvalTrace []ApprovalTraceItem) error {
	tmplContent, err := os.ReadFile("module8workers/assets/workerLast.html")
	if err != nil {
		return fmt.Errorf("failed to read worker last template: %w", err)
	}

	tmpl, err := template.New("workerLast").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse worker last template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateDataWithApprovalTrace(mailData, approvalTrace)); err != nil {
		return fmt.Errorf("failed to execute worker last template: %w", err)
	}

	if err := sendEmail(workerEmail, "Budget Request Confirmed", buf.String()); err != nil {
		return fmt.Errorf("failed to send worker last email: %w", err)
	}

	return nil
}

func sendRRHHTravelEmail(mailData RRHHTravelMailData, attachments []string) error {
	tmplContent, err := os.ReadFile("module8workers/assets/rrhhInforms.html")
	if err != nil {
		return fmt.Errorf("failed to read rrhh template: %w", err)
	}

	tmpl, err := template.New("rrhhInforms").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse rrhh template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute rrhh template: %w", err)
	}

	if err := sendEmailWithAttachments(rrhhEmail, "Travel Request Accepted by Accounting", buf.String(), attachments); err != nil {
		return fmt.Errorf("failed to send rrhh email: %w", err)
	}

	return nil
}

func notifyWorkerAfterAccountingConfirmation(combinedID int, finalApproverID int, approvedAt string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}
	if strings.TrimSpace(workerInfo.Email) == "" {
		return nil
	}

	projectNames, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	mailData := WorkerLastMailData{
		Name:           workerInfo.Name,
		Surname:        workerInfo.Surname,
		ProjectName:    strings.Join(projectNames, ", "),
		CreatedAt:      normalizeDateString(workerInfo.CreatedAt),
		UpdatedAt:      time.Now().Format("2006-01-02 15:04"),
		RequestSummary: strings.Join(summaryParts, " || "),
	}

	approvalTrace, traceErr := buildApprovalTrace(combinedID, 0, "Accounting", finalApproverID, approvedAt, apiKey, client, w)
	if traceErr != nil {
		createLog(fmt.Sprintf("Could not build approval trace for worker accounting email of request %d: %v", combinedID, traceErr), 1, apiKey, client, w)
	}

	return sendWorkerLastEmail(workerInfo.Email, mailData, approvalTrace)
}

func hasTravelToRRHH(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (bool, error) {
	query := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "travel_area",
		"condition": fmt.Sprintf("id_combined = %d AND category_id = 1", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return false, fmt.Errorf("failed to get travel parts")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("travel parts query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false, err
	}

	for _, row := range rows {
		area := strings.TrimSpace(strings.ToLower(asString(row["travel_area"])))
		if area == "eu" || area == "non_eu" {
			return true, nil
		}
	}

	return false, nil
}

func getTravelAttachmentPaths(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) ([]string, error) {
	query := map[string]interface{}{
		"table":     "budget_parts bp LEFT JOIN request_files rf ON bp.id = rf.request_id",
		"columns":   "rf.file_path",
		"condition": fmt.Sprintf("bp.id_combined = %d AND bp.category_id = 1 AND rf.file_path IS NOT NULL", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get travel attachments")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("attachments query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	var paths []string
	for _, row := range rows {
		p := strings.TrimSpace(asString(row["file_path"]))
		if p == "" {
			continue
		}

		// Si en DB guardas Uploads/..., pero el backend está un nivel distinto:
		absPath := p
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Clean("../../" + p)
		}

		if _, err := os.Stat(absPath); err == nil {
			paths = append(paths, absPath)
		}
	}

	return paths, nil
}

func notifyRRHHAfterAccountingConfirmation(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	query := map[string]interface{}{
		"table": "budget_parts bp " +
			"LEFT JOIN projects pr ON bp.project_id = pr.id " +
			"LEFT JOIN people ip ON bp.ip_id = ip.id",
		"columns": "pr.short_name AS project_name, ip.name AS ip_name, ip.surname AS ip_surname, " +
			"bp.purpose, bp.wherePlace, bp.institution, bp.fromDay, bp.untilDay, bp.travel_area",
		"condition": fmt.Sprintf("bp.id_combined = %d AND bp.category_id = 1", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("failed to get travel data for rrhh")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rrhh travel query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	row := rows[0]
	attachments, err := getTravelAttachmentPaths(combinedID, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Could not load rrhh attachments for request %d: %v", combinedID, err), 1, apiKey, client, w)
	}

	mailData := RRHHTravelMailData{
		ResearcherName:    workerInfo.Name,
		ResearcherSurname: workerInfo.Surname,
		PIName:            unescapeComma(asString(row["ip_name"])),
		PISurname:         unescapeComma(asString(row["ip_surname"])),
		ProjectName:       unescapeComma(asString(row["project_name"])),
		UpdatedAt:         time.Now().Format("2006-01-02 15:04"),
		TravelPurpose:     unescapeComma(asString(row["purpose"])),
		Destination:       unescapeComma(asString(row["wherePlace"])),
		Institution:       unescapeComma(asString(row["institution"])),
		DepartureDate:     normalizeDateString(asString(row["fromDay"])),
		ReturnDate:        normalizeDateString(asString(row["untilDay"])),
		HasInvitation:     len(attachments) > 0,
	}

	return sendRRHHTravelEmail(mailData, attachments)
}

func sendWorkerITLastEmail(workerEmail string, mailData WorkerLastMailData, approvalTrace []ApprovalTraceItem) error {
	tmplContent, err := os.ReadFile("module8workers/assets/workerITLast.html")
	if err != nil {
		return fmt.Errorf("failed to read worker IT last template: %w", err)
	}

	tmpl, err := template.New("workerITLast").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse worker IT last template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateDataWithApprovalTrace(mailData, approvalTrace)); err != nil {
		return fmt.Errorf("failed to execute worker IT last template: %w", err)
	}

	if err := sendEmail(workerEmail, "Budget Request Confirmed", buf.String()); err != nil {
		return fmt.Errorf("failed to send worker IT last email: %w", err)
	}

	return nil
}
func sendAccountingInformsEmail(mailData AccountingInformsMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/accountingInforms.html")
	if err != nil {
		return fmt.Errorf("failed to read accounting informs template: %w", err)
	}

	tmpl, err := template.New("accountingInforms").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse accounting informs template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute accounting informs template: %w", err)
	}

	if err := sendEmail(accountingEmail, "Equipment Request Accepted by IT", buf.String()); err != nil {
		return fmt.Errorf("failed to send accounting informs email: %w", err)
	}

	return nil
}

func notifyWorkerAfterITConfirmation(combinedID int, finalApproverID int, approvedAt string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}
	if strings.TrimSpace(workerInfo.Email) == "" {
		return nil
	}

	projectNames, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	mailData := WorkerLastMailData{
		Name:           workerInfo.Name,
		Surname:        workerInfo.Surname,
		ProjectName:    strings.Join(projectNames, ", "),
		CreatedAt:      normalizeDateString(workerInfo.CreatedAt),
		UpdatedAt:      time.Now().Format("2006-01-02 15:04"),
		RequestSummary: strings.Join(summaryParts, " || "),
	}

	approvalTrace, traceErr := buildApprovalTrace(combinedID, 0, "IT", finalApproverID, approvedAt, apiKey, client, w)
	if traceErr != nil {
		createLog(fmt.Sprintf("Could not build approval trace for worker IT email of request %d: %v", combinedID, traceErr), 1, apiKey, client, w)
	}

	return sendWorkerITLastEmail(workerInfo.Email, mailData, approvalTrace)
}

func notifyAccountingAfterITConfirmation(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	query := map[string]interface{}{
		"table": "budget_parts bp " +
			"LEFT JOIN projects pr ON bp.project_id = pr.id " +
			"LEFT JOIN people ip ON bp.ip_id = ip.id " +
			"LEFT JOIN request_prices rp ON bp.other_price = rp.id",
		"columns": "pr.short_name AS project_name, ip.name AS ip_name, ip.surname AS ip_surname, " +
			"rp.price_range AS estimated_amount",
		"condition": fmt.Sprintf("bp.id_combined = %d AND bp.category_id = 4", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("failed to get equipment data for accounting")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("accounting equipment query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	_, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	row := rows[0]
	mailData := AccountingInformsMailData{
		Name:              "Accounting",
		Surname:           "",
		ResearcherName:    workerInfo.Name,
		ResearcherSurname: workerInfo.Surname,
		PIName:            unescapeComma(asString(row["ip_name"])),
		PISurname:         unescapeComma(asString(row["ip_surname"])),
		ProjectName:       unescapeComma(asString(row["project_name"])),
		UpdatedAt:         time.Now().Format("2006-01-02 15:04"),
		EstimatedAmount:   asString(row["estimated_amount"]),
		RequestSummary:    strings.Join(summaryParts, " || "),
	}

	return sendAccountingInformsEmail(mailData)
}

// ---------------- EMAILS: CANCELLATION FLOW ----------------

func sendCanceledEmail(to string, mailData CanceledMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/canceled.html")
	if err != nil {
		return fmt.Errorf("failed to read canceled template: %w", err)
	}

	tmpl, err := template.New("canceled").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse canceled template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute canceled template: %w", err)
	}

	if err := sendEmail(to, "Budget Request Cancelled by Investigator", buf.String()); err != nil {
		return fmt.Errorf("failed to send canceled email: %w", err)
	}

	return nil
}

func getCancelFlowInfo(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (*CancelFlowInfo, error) {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	_, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	query := map[string]interface{}{
		"table": `budget_requests br
			LEFT JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN projects pr ON bp.project_id = pr.id
			LEFT JOIN people ip ON bp.ip_id = ip.id`,
		"columns": `br.creation_date, br.projects_response, br.acc_or_it_response,
			bp.ip_response, bp.category_id,
			pr.short_name AS project_name,
			ip.name AS ip_name, ip.surname AS ip_surname`,
		"condition": fmt.Sprintf("br.id = %d", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get cancel flow info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cancel flow query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data found for request %d", combinedID)
	}

	info := &CancelFlowInfo{
		CombinedID:        combinedID,
		CreatedAt:         normalizeDateString(asString(rows[0]["creation_date"])),
		UpdatedAt:         time.Now().Format("2006-01-02 15:04"),
		RequestSummary:    strings.Join(summaryParts, " || "),
		ResearcherID:      workerInfo.ID,
		ResearcherName:    workerInfo.Name,
		ResearcherSurname: workerInfo.Surname,
		ResearcherEmail:   workerInfo.Email,
		ProjectName:       unescapeComma(asString(rows[0]["project_name"])),
		PIName:            unescapeComma(asString(rows[0]["ip_name"])),
		PISurname:         unescapeComma(asString(rows[0]["ip_surname"])),
	}

	requestTypeSet := map[string]bool{}
	for _, row := range rows {
		if asBool(row["ip_response"]) {
			info.HasIPResponse = true
		}
		if asBool(row["projects_response"]) {
			info.HasProjectsResponse = true
		}
		if asBool(row["acc_or_it_response"]) {
			info.HasAccOrITResponse = true
		}

		categoryID := asString(row["category_id"])
		switch categoryID {
		case "1":
			requestTypeSet["Travel"] = true
		case "2":
			requestTypeSet["Registration"] = true
		case "3":
			requestTypeSet["Accommodation"] = true
		case "4":
			requestTypeSet["Equipment"] = true
			info.HasEquipmentCategory = true
		case "5":
			requestTypeSet["Other"] = true
		}

		fmt.Printf("Has IP response: %v, Has Projects response: %v, Has Acc/IT response: %v, Category ID: %s\n",
			info.HasIPResponse, info.HasProjectsResponse, info.HasAccOrITResponse, categoryID)
	}

	var requestTypes []string
	for _, label := range []string{"Travel", "Registration", "Accommodation", "Equipment", "Other"} {
		if requestTypeSet[label] {
			requestTypes = append(requestTypes, label)
		}
	}
	info.RequestType = strings.Join(requestTypes, ", ")

	return info, nil
}

func getCancelRecipients(info *CancelFlowInfo, apiKey string, client *http.Client, w http.ResponseWriter) ([]CancelRecipient, error) {
	var recipients []CancelRecipient
	seen := map[string]bool{}

	add := func(to, name, surname, roleKey string) {
		to = strings.TrimSpace(to)
		if to == "" {
			return
		}

		seenKey := roleKey + "|" + to
		if seen[seenKey] {
			return
		}
		seen[seenKey] = true

		recipients = append(recipients, CancelRecipient{
			To:      to,
			Name:    name,
			Surname: surname,
			RoleKey: roleKey,
		})
	}

	// 1) IP: si la request ya se había enviado, se le avisa.
	// Como mínimo, al hacer submit ya le llegó.
	ipEmail, ipName, ipSurname, err := getPIEmailForCombinedRequest(info.CombinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}
	add(ipEmail, ipName, ipSurname, "ip")

	// 2) Projects: si IP ya respondió, o si el flujo ya avanzó a Projects / siguiente nivel.
	if info.HasIPResponse || info.HasProjectsResponse || info.HasAccOrITResponse {
		add(projectsEmail, "Projects", "", "projects")
	}

	// 3) Último escalón: IT para equipment, Accounting para el resto.
	if info.HasProjectsResponse || info.HasAccOrITResponse {
		if info.HasEquipmentCategory {
			add(itEmail, "IT", "", "it")
		} else {
			add(accountingEmail, "Accounting", "", "accounting")
		}
	}

	return recipients, nil
}

func getPIEmailForCombinedRequest(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (string, string, string, error) {
	query := map[string]interface{}{
		"table": `budget_parts bp
			LEFT JOIN people p ON bp.ip_id = p.id`,
		"columns":   "p.crm_email, p.name, p.surname",
		"condition": fmt.Sprintf("bp.id_combined = %d", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return "", "", "", fmt.Errorf("failed to get PI email")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("PI email query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return "", "", "", err
	}
	if len(rows) == 0 {
		return "", "", "", nil
	}

	return strings.TrimSpace(asString(rows[0]["crm_email"])),
		unescapeComma(asString(rows[0]["name"])),
		unescapeComma(asString(rows[0]["surname"])),
		nil
}

func notifyCancellationRecipients(combinedID int, motive string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	info, err := getCancelFlowInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	recipients, err := getCancelRecipients(info, apiKey, client, w)
	if err != nil {
		return err
	}

	for _, recipient := range recipients {
		mailData := CanceledMailData{
			Name:              recipient.Name,
			Surname:           recipient.Surname,
			ResearcherName:    info.ResearcherName,
			ResearcherSurname: info.ResearcherSurname,
			PIName:            info.PIName,
			PISurname:         info.PISurname,
			ProjectName:       info.ProjectName,
			RequestType:       info.RequestType,
			CreatedAt:         info.CreatedAt,
			UpdatedAt:         info.UpdatedAt,
			RequestSummary:    info.RequestSummary,
			Message:           motive,
		}

		if err := sendCanceledEmail(recipient.To, mailData); err != nil {
			createLog(fmt.Sprintf(
				"Error sending cancellation email for request %d to %s (%s): %v",
				combinedID, recipient.RoleKey, recipient.To, err,
			), 1, apiKey, client, w)
		}
	}

	return nil
}

// ---------------- EMAILS: INVOICE PROVIDED FLOW ----------------

func sendInvoiceProvidedEmail(to string, subject string, mailData InvoiceProvidedMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/invoiceProvided.html")
	if err != nil {
		return fmt.Errorf("failed to read invoice provided template: %w", err)
	}

	tmpl, err := template.New("invoiceProvided").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse invoice provided template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute invoice provided template: %w", err)
	}

	if err := sendEmail(to, subject, buf.String()); err != nil {
		return fmt.Errorf("failed to send invoice provided email: %w", err)
	}

	return nil
}

func getInvoiceFlowInfo(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (*InvoiceFlowInfo, error) {
	workerInfo, err := getRequestWorkerInfo(combinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	_, summaryParts, err := getRequestRejectionSummary(combinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	query := map[string]interface{}{
		"table": `budget_requests br
			LEFT JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN projects pr ON bp.project_id = pr.id
			LEFT JOIN people ip ON bp.ip_id = ip.id`,
		"columns": `br.projects_response, br.acc_or_it_response,
			bp.ip_response,
			pr.short_name AS project_name,
			ip.name AS ip_name, ip.surname AS ip_surname`,
		"condition": fmt.Sprintf("br.id = %d AND bp.category_id = 2", combinedID),
	}
	jsonQuery, _ := json.Marshal(query)

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("failed to get invoice flow info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("invoice flow query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no registration part found for request %d", combinedID)
	}

	info := &InvoiceFlowInfo{
		CombinedID:        combinedID,
		ProjectName:       unescapeComma(asString(rows[0]["project_name"])),
		PIName:            unescapeComma(asString(rows[0]["ip_name"])),
		PISurname:         unescapeComma(asString(rows[0]["ip_surname"])),
		ResearcherName:    workerInfo.Name,
		ResearcherSurname: workerInfo.Surname,
		RequestSummary:    strings.Join(summaryParts, " || "),
	}

	for _, row := range rows {
		if asBool(row["ip_response"]) {
			info.HasIPResponse = true
		}
		if asBool(row["projects_response"]) {
			info.HasProjectsResponse = true
		}
		if asBool(row["acc_or_it_response"]) {
			info.HasAccOrITResponse = true
		}
	}

	return info, nil
}

func notifyProjectsInvoiceProvided(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	info, err := getInvoiceFlowInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	// Ya les había llegado si IP ya respondió o si ya estaba en etapas posteriores
	if !(info.HasIPResponse || info.HasProjectsResponse || info.HasAccOrITResponse) {
		return nil
	}

	mailData := InvoiceProvidedMailData{
		Name:              "Projects",
		Surname:           "Team",
		ResearcherName:    info.ResearcherName,
		ResearcherSurname: info.ResearcherSurname,
		PIName:            info.PIName,
		PISurname:         info.PISurname,
		ProjectName:       info.ProjectName,
		UpdatedAt:         time.Now().Format("2006-01-02 15:04"),
		RequestSummary:    info.RequestSummary,
	}

	return sendInvoiceProvidedEmail(
		projectsEmail,
		"Payment Document Updated",
		mailData,
	)
}

func notifyAccountingInvoiceProvided(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	info, err := getInvoiceFlowInfo(combinedID, apiKey, client, w)
	if err != nil {
		return err
	}

	// Accounting ya la había recibido si Projects ya aprobó o si acc_or_it ya intervino
	if !(info.HasProjectsResponse || info.HasAccOrITResponse) {
		return nil
	}

	mailData := InvoiceProvidedMailData{
		Name:              "Accounting",
		Surname:           "Team",
		ResearcherName:    info.ResearcherName,
		ResearcherSurname: info.ResearcherSurname,
		PIName:            info.PIName,
		PISurname:         info.PISurname,
		ProjectName:       info.ProjectName,
		UpdatedAt:         time.Now().Format("2006-01-02 15:04"),
		RequestSummary:    info.RequestSummary,
	}

	return sendInvoiceProvidedEmail(
		accountingEmail,
		"Payment Document Updated",
		mailData,
	)
}

// ---------------- GENERAL HELPERS ----------------

func contains(s1 []string, s2 string) bool {
	for _, v := range s1 {
		if v == s2 {
			return true
		}
	}
	return false
}

func containsString(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func cleanPath(p string) string {
	for strings.HasPrefix(p, "../") {
		p = strings.TrimPrefix(p, "../")
	}
	return p
}

func escapeSQLValue(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func escapeSQLLike(value string) string {
	value = strings.ReplaceAll(value, "'", "''")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value
}

func mapCategoriesToIDs(categories []string) []string {
	categoryMap := map[string]string{
		"travel":        "1",
		"registration":  "2",
		"accommodation": "3",
		"equipment":     "4",
		"other":         "5",
	}

	ids := []string{}
	seen := map[string]bool{}

	for _, category := range categories {
		key := strings.ToLower(strings.TrimSpace(category))
		id, ok := categoryMap[key]
		if !ok {
			continue
		}

		if seen[id] {
			continue
		}

		seen[id] = true
		ids = append(ids, id)
	}

	return ids
}

func quoteSQLStringList(values []string) []string {
	result := []string{}
	seen := map[string]bool{}

	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}

		if seen[clean] {
			continue
		}

		seen[clean] = true
		result = append(result, fmt.Sprintf("'%s'", escapeSQLValue(clean)))
	}

	return result
}

func cleanNumericList(values []string) []string {
	result := []string{}
	seen := map[string]bool{}

	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}

		if _, err := strconv.Atoi(clean); err != nil {
			continue
		}

		if seen[clean] {
			continue
		}

		seen[clean] = true
		result = append(result, clean)
	}

	return result
}

func parseBoolean(v interface{}) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if s == "1" {
		return "Yes"
	} else if s == "0" {
		return "No"
	}
	return s
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" {
		return ""
	}
	return s
}

func asInt(v interface{}) int {
	switch t := v.(type) {
	case nil:
		return 0
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func asNullableInt(v interface{}) *int {
	if v == nil {
		return nil
	}

	switch n := v.(type) {
	case int:
		x := n
		return &x
	case int32:
		x := int(n)
		return &x
	case int64:
		x := int(n)
		return &x
	case float64:
		x := int(n)
		return &x
	case string:
		if strings.TrimSpace(n) == "" {
			return nil
		}
		if parsed, err := strconv.Atoi(n); err == nil {
			x := parsed
			return &x
		}
	}

	return nil
}

func normalizeString(v string) string {
	return strings.TrimSpace(v)
}

func normalizeDateString(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

func formatDateTimeNoSeconds(value string) string {
	if value == "" {
		return ""
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}

	return t.Format("2006-01-02 15:04")
}

func stringsDifferent(a, b string) bool {
	return normalizeString(a) != normalizeString(b)
}

func intsDifferent(a, b int) bool {
	return a != b
}

func nullableIntDifferent(existing *int, incoming *int) bool {
	if existing == nil && incoming == nil {
		return false
	}
	if existing == nil || incoming == nil {
		return true
	}
	return *existing != *incoming
}

func intPtr(v int) *int {
	return &v
}

func parseTravelArea(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "spain":
		return "Inside Spanish territory"
	case "eu":
		return "European Union"
	case "non_eu":
		return "Outside the European Union (including the United Kingdom)"
	default:
		return v
	}
}

func parseLuggageType(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "none":
		return "No luggage"
	case "hand":
		return "Hand luggage only"
	case "checked":
		return "Checked luggage only"
	case "hand_checked":
		return "Hand luggage + checked luggage"
	default:
		return v
	}
}

func parseSeatPreference(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "na":
		return "Random"
	case "window":
		return "Window"
	case "middle":
		return "Middle"
	case "aisle":
		return "Aisle"
	default:
		return v
	}
}

func parseTimePreference(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "na":
		return "No preference"
	case "morning":
		return "Morning"
	case "afternoon":
		return "Afternoon"
	default:
		return v
	}
}

func parsePriceRange(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1":
		return "0 - 5k€"
	case "2":
		return "5k€ - 15k€"
	case "3":
		return "15k€ - 50k€"
	case "4":
		return "More than 50k€"
	default:
		return v
	}
}

// ---------------- ATTENDANCE CERTIFICATES ----------------
// Travel / registration requests need an attendance certificate once the end date has passed.

type attendanceCertificateCandidate struct {
	CombinedID     int      `json:"combinedId"`
	InternalID     string   `json:"internalId"`
	PeopleID       int      `json:"peopleId"`
	ProjectName    string   `json:"projectName"`
	UntilDay       string   `json:"untilDay"`
	EndDate        string   `json:"endDate"`
	Categories     []string `json:"categories"`
	Purposes       []string `json:"purposes"`
	RequestSummary string   `json:"requestSummary"`
}

func handleGetPendingAttendanceCertificates(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	pending, err := collectPendingAttendanceCertificates(userID, false, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading pending attendance certificates for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error loading pending attendance certificates", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pending": pending,
	})
}

func handleRunAttendanceCertificateReminders(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	pending, err := collectPendingAttendanceCertificates(0, true, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error running attendance certificate reminders for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error running attendance certificate reminders", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"pending": len(pending),
	})
}

func collectPendingAttendanceCertificates(userID int, sendEmails bool, apiKey string, client *http.Client, w http.ResponseWriter) ([]AttendanceCertificatePendingItem, error) {
	candidates, err := getAttendanceCertificateCandidates(userID, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	pending := make([]AttendanceCertificatePendingItem, 0)

	for _, candidate := range candidates {
		tracking, err := ensureAttendanceCertificateTracking(candidate, apiKey, client, w)
		if err != nil {
			return nil, err
		}

		if asInt(tracking["file_uploaded"]) == 1 {
			continue
		}

		item := AttendanceCertificatePendingItem{
			CombinedID:     candidate.CombinedID,
			InternalID:     candidate.InternalID,
			ProjectName:    candidate.ProjectName,
			RequestSummary: candidate.RequestSummary,
			EndDate:        candidate.EndDate,
			LastReminderAt: normalizeDateString(asString(tracking["last_reminder_at"])),
			ReminderCount:  asInt(tracking["reminder_count"]),
		}

		if sendEmails && shouldSendAttendanceReminder(item.LastReminderAt) {
			if err := notifyAttendanceCertificateMissing(candidate, apiKey, client, w); err != nil {
				createLog(fmt.Sprintf("Error sending attendance certificate reminder for request %d: %v", candidate.CombinedID, err), 1, apiKey, client, w)
			} else if err := updateAttendanceReminderState(candidate.CombinedID, item.ReminderCount+1, apiKey, client, w); err != nil {
				createLog(fmt.Sprintf("Error updating attendance reminder state for request %d: %v", candidate.CombinedID, err), 1, apiKey, client, w)
			}
		}

		pending = append(pending, item)
	}

	return pending, nil
}

func getAttendanceCertificateCandidates(userID int, apiKey string, client *http.Client, w http.ResponseWriter) ([]attendanceCertificateCandidate, error) {
	today := time.Now().Format("2006-01-02")

	conditions := []string{
		"br.canceled = 0",
		"(br.denied_comment IS NULL OR br.denied_comment = '')",
		"br.acc_or_it_response = 1",
		"(bp.category_id = 1 OR bp.category_id = 2)",
		"bp.untilDay IS NOT NULL",
		"bp.untilDay != ''",
		fmt.Sprintf("substr(bp.untilDay, 1, 10) < '%s'", today),
	}

	if userID > 0 {
		conditions = append(conditions, fmt.Sprintf("br.people_id = %d", userID))
	}

	query := map[string]interface{}{
		"table": `budget_requests br
			LEFT JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN projects p ON bp.project_id = p.id`,
		"columns": strings.Join([]string{
			"br.id AS combined_id",
			"br.id_intern",
			"br.people_id",
			"bp.category_id",
			"bp.purpose",
			"bp.untilDay",
			"p.short_name AS project_name",
		}, ", "),
		"condition": strings.Join(conditions, " AND "),
		"order":     "br.id DESC, bp.untilDay ASC",
	}

	//fmt.Printf("Attendance query: %+v\n", query)

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("empty response getting attendance certificate candidates")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("attendance candidates query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	//fmt.Printf("Certificats: %+v\n", rows)

	return groupAttendanceCertificateCandidates(rows), nil
}
func groupAttendanceCertificateCandidates(rows []map[string]interface{}) []attendanceCertificateCandidate {
	grouped := make(map[string]*attendanceCertificateCandidate)
	order := make([]string, 0)

	for _, row := range rows {
		combinedID := asString(row["combined_id"])
		if combinedID == "" {
			continue
		}

		if _, exists := grouped[combinedID]; !exists {
			endDate := normalizeDateString(asString(row["untilDay"]))

			grouped[combinedID] = &attendanceCertificateCandidate{
				CombinedID:  asInt(row["combined_id"]),
				InternalID:  asString(row["id_intern"]),
				PeopleID:    asInt(row["people_id"]),
				ProjectName: unescapeComma(asString(row["project_name"])),
				UntilDay:    endDate,
				EndDate:     endDate,
				Categories:  []string{},
				Purposes:    []string{},
			}

			order = append(order, combinedID)
		}

		candidate := grouped[combinedID]

		categoryID := asInt(row["category_id"])
		categoryLabel := ""

		switch categoryID {
		case 1:
			categoryLabel = "Travel"
		case 2:
			categoryLabel = "Registration"
		}

		if categoryLabel != "" && !containsString(candidate.Categories, categoryLabel) {
			candidate.Categories = append(candidate.Categories, categoryLabel)
		}

		purpose := strings.TrimSpace(unescapeComma(asString(row["purpose"])))
		if purpose != "" && !containsString(candidate.Purposes, purpose) {
			candidate.Purposes = append(candidate.Purposes, purpose)
		}

		rowUntilDay := normalizeDateString(asString(row["untilDay"]))
		if rowUntilDay != "" && (candidate.EndDate == "" || rowUntilDay > candidate.EndDate) {
			candidate.UntilDay = rowUntilDay
			candidate.EndDate = rowUntilDay
		}
	}

	result := make([]attendanceCertificateCandidate, 0, len(order))
	for _, combinedID := range order {
		candidate := grouped[combinedID]

		summaryParts := make([]string, 0)

		if len(candidate.Categories) > 0 {
			summaryParts = append(summaryParts, strings.Join(candidate.Categories, " + "))
		}

		if len(candidate.Purposes) > 0 {
			summaryParts = append(summaryParts, strings.Join(candidate.Purposes, " / "))
		}

		candidate.RequestSummary = strings.Join(summaryParts, " - ")

		result = append(result, *candidate)
	}

	return result
}
func ensureAttendanceCertificateTracking(candidate attendanceCertificateCandidate, apiKey string, client *http.Client, w http.ResponseWriter) (map[string]interface{}, error) {
	existing, err := getAttendanceCertificateTracking(candidate.CombinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	dueDate := candidate.EndDate
	if dueDate == "" {
		dueDate = candidate.UntilDay
	}

	now := time.Now().Format("2006-01-02 15:04")
	insert := map[string]interface{}{
		"table":   "budget_attendance_certificates",
		"columns": "id_combined, people_id, due_date, required, file_uploaded, reminder_count, created_at, updated_at",
		"value": fmt.Sprintf(
			"%d, %d, %s, 1, 0, 0, %s, %s",
			candidate.CombinedID,
			candidate.PeopleID,
			dueDate,
			now,
			now,
		),
	}

	jsonInsert, _ := json.Marshal(insert)
	resp := postReq(jsonInsert, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("empty response creating attendance certificate tracking")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create attendance certificate tracking failed: %d - %s", resp.StatusCode, string(body))
	}

	return getAttendanceCertificateTracking(candidate.CombinedID, apiKey, client, w)
}

func getAttendanceCertificateTracking(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (map[string]interface{}, error) {
	query := map[string]interface{}{
		"table":     "budget_attendance_certificates",
		"columns":   "id, id_combined, people_id, due_date, required, file_uploaded, file_path, uploaded_at, last_reminder_at, reminder_count",
		"condition": fmt.Sprintf("id_combined = %d", combinedID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("empty response getting attendance certificate tracking")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("attendance certificate tracking query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	return rows[0], nil
}

func shouldSendAttendanceReminder(lastReminderAt string) bool {
	lastReminderAt = normalizeDateString(lastReminderAt)
	if lastReminderAt == "" {
		return true
	}

	lastReminderDate, err := time.Parse("2006-01-02", lastReminderAt)
	if err != nil {
		return true
	}

	return !time.Now().Before(lastReminderDate.AddDate(0, 0, 10))
}

func updateAttendanceReminderState(combinedID int, reminderCount int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	now := time.Now().Format("2006-01-02 15:04")
	update := map[string]interface{}{
		"table":     "budget_attendance_certificates",
		"columns":   "last_reminder_at, reminder_count, updated_at",
		"value":     fmt.Sprintf("%s, %d, %s", now, reminderCount, now),
		"condition": fmt.Sprintf("id_combined = %d", combinedID),
	}

	jsonUpdate, _ := json.Marshal(update)
	resp := putReq(jsonUpdate, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("empty response updating attendance reminder state")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update attendance reminder state failed: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

func notifyAttendanceCertificateMissing(candidate attendanceCertificateCandidate, apiKey string, client *http.Client, w http.ResponseWriter) error {
	person, err := getPersonInfoByID(candidate.PeopleID, apiKey, client, w)
	if err != nil {
		return err
	}
	if strings.TrimSpace(person.Email) == "" {
		return nil
	}

	mailData := AttendanceCertificateReminderMailData{
		Name:           person.Name,
		Surname:        person.Surname,
		InternalID:     candidate.InternalID,
		ProjectName:    candidate.ProjectName,
		RequestSummary: candidate.RequestSummary,
		EndDate:        candidate.EndDate,
		Today:          time.Now().Format("2006-01-02"),
	}

	return sendAttendanceCertificateReminderEmail(person.Email, mailData)
}

func sendAttendanceCertificateReminderEmail(to string, mailData AttendanceCertificateReminderMailData) error {
	tmplContent, err := os.ReadFile("module8workers/assets/attendanceCertificateReminder.html")
	if err != nil {
		return fmt.Errorf("failed to read attendance certificate reminder template: %w", err)
	}

	tmpl, err := template.New("attendanceCertificateReminder").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse attendance certificate reminder template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mailData); err != nil {
		return fmt.Errorf("failed to execute attendance certificate reminder template: %w", err)
	}

	return sendEmail(to, "Attendance Certificate Required", buf.String())
}

func handleUploadAttendanceCertificate(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, userACCESSING, _ := getUserInfo(apiKey, client, w, r)

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		createLog(fmt.Sprintf("Error parsing attendance certificate form for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error al procesar el formulario multipart", http.StatusBadRequest)
		return
	}

	combinedIDStr := strings.TrimSpace(r.FormValue("idCombined"))
	if combinedIDStr == "" {
		http.Error(w, "idCombined es obligatorio", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(combinedIDStr)
	if err != nil {
		http.Error(w, "idCombined no es válido", http.StatusBadRequest)
		return
	}

	if ok, err := userOwnsBudgetRequest(userID, combinedID, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Error checking attendance certificate ownership for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, "Error checking request ownership", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "No tienes permisos para modificar esta solicitud", http.StatusForbidden)
		return
	}

	certificateFiles := r.MultipartForm.File["attendance_certificate"]
	if len(certificateFiles) == 0 {
		http.Error(w, "attendance_certificate es obligatorio", http.StatusBadRequest)
		return
	}
	if len(certificateFiles) > 1 {
		http.Error(w, "Solo se puede subir un certificado de asistencia", http.StatusBadRequest)
		return
	}
	certificateName := filepath.Base(certificateFiles[0].Filename)
	if strings.ToLower(filepath.Ext(certificateName)) != ".pdf" {
		http.Error(w, "El certificado de asistencia debe ser un archivo PDF", http.StatusBadRequest)
		return
	}

	projectID, err := getAttendanceCertificateProjectID(combinedID, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading project for attendance certificate %d: %v", combinedID, err), 1, apiKey, client, w)
		http.Error(w, "Error loading request project", http.StatusInternalServerError)
		return
	}

	savedFiles, err := validateAndSaveFiles("attendance_certificate", apiKey, client, w, r, certificateFiles, uploadDir, projectID)
	if err != nil {
		createLog(fmt.Sprintf("Attendance certificate upload rejected for user %s: %v", userACCESSING, err), 1, apiKey, client, w)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(savedFiles) == 0 {
		http.Error(w, "No se ha podido guardar el certificado", http.StatusInternalServerError)
		return
	}

	filePath := cleanPath(savedFiles[0].Path)
	now := time.Now().Format("2006-01-02 15:04")

	if _, err := ensureTrackingForUpload(combinedID, userID, apiKey, client, w); err != nil {
		createLog(fmt.Sprintf("Error ensuring attendance tracking for upload %d: %v", combinedID, err), 1, apiKey, client, w)
		http.Error(w, "Error preparing certificate tracking", http.StatusInternalServerError)
		return
	}

	update := map[string]interface{}{
		"table":     "budget_attendance_certificates",
		"columns":   "file_uploaded, file_path, uploaded_at, updated_at",
		"value":     fmt.Sprintf("1, %s, %s, %s", escapeComma(filePath), now, now),
		"condition": fmt.Sprintf("id_combined = %d", combinedID),
	}

	jsonUpdate, _ := json.Marshal(update)
	resp := putReq(jsonUpdate, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No response updating attendance certificate for request %d", combinedID), 1, apiKey, client, w)
		http.Error(w, "Error updating attendance certificate", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("Error updating attendance certificate for request %d. Status: %d, Body: %s", combinedID, resp.StatusCode, string(body)), 1, apiKey, client, w)
		http.Error(w, "Error updating attendance certificate", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
	})
}

func userOwnsBudgetRequest(userID int, combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (bool, error) {
	query := map[string]interface{}{
		"table":     "budget_requests",
		"columns":   "id",
		"condition": fmt.Sprintf("id = %d AND people_id = %d", combinedID, userID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return false, fmt.Errorf("empty response checking request ownership")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("ownership query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

func getAttendanceCertificateProjectID(combinedID int, apiKey string, client *http.Client, w http.ResponseWriter) (string, error) {
	query := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "project_id",
		"condition": fmt.Sprintf("id_combined = %d AND (category_id = 1 OR category_id = 2)", combinedID),
		"order":     "category_id ASC",
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return "", fmt.Errorf("empty response getting attendance certificate project")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("attendance project query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no travel or registration part found")
	}

	return asString(rows[0]["project_id"]), nil
}

func ensureTrackingForUpload(combinedID int, userID int, apiKey string, client *http.Client, w http.ResponseWriter) (map[string]interface{}, error) {
	existing, err := getAttendanceCertificateTracking(combinedID, apiKey, client, w)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	query := map[string]interface{}{
		"table":     "budget_requests br LEFT JOIN budget_parts bp ON br.id = bp.id_combined LEFT JOIN projects p ON bp.project_id = p.id",
		"columns":   "br.id AS combined_id, br.id_intern, br.people_id, bp.category_id, bp.purpose, bp.untilDay, p.short_name AS project_name",
		"condition": fmt.Sprintf("br.id = %d AND br.people_id = %d AND (bp.category_id = 1 OR bp.category_id = 2)", combinedID, userID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("empty response getting request data for attendance tracking")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request data for attendance tracking failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no travel or registration part found")
	}

	candidates := groupAttendanceCertificateCandidates(rows)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("could not build attendance tracking candidate")
	}

	return ensureAttendanceCertificateTracking(candidates[0], apiKey, client, w)
}
