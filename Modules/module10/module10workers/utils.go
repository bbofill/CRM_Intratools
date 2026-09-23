package module10workers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

const (
	commissionStatusSubmitted      = "submitted"
	commissionStatusUnderReview    = "under_review"
	commissionStatusFinalReview    = "final_review"
	commissionStatusApproved       = "approved"
	commissionStatusRejected       = "rejected"
	commissionStatusExported       = "exported"
	centerExpensePurpose           = "center"
	userExpensePurpose             = "user"
	budgetTravelExpenseType        = "budget_travel"
	budgetRegistrationType         = "budget_registration"
	budgetAccommodationType        = "budget_accommodation"
	contractProgramProjectID       = "001000001CP"
	contractProgramLegacyProjectID = "CONTRACTE_PROGRAMA"
	contractProgramProjectName     = "Contracte Programa"
	projectsEmail                  = "crmprojects@crm.cat" //crmprojects
	managementEmail                = "gerencia@crm.cat"    //gerencia
	serviceCommissionSystemUserID  = 304
)

type commissionExpensePayload struct {
	Type             string
	Amount           float64
	DistanceKM       float64
	TransportMethod  string
	Description      string
	DepartureDate    string
	ReturnDate       string
	DepartureTime    string
	ReturnTime       string
	PerDiemBreakdown string
}

type fileStorageContext struct {
	Username    string
	ProjectCode string
	TravelCode  string
	Timestamp   string
}

type pricingTablePayload struct {
	ID       int                 `json:"id"`
	Title    string              `json:"title"`
	Category string              `json:"category"`
	Type     string              `json:"type"`
	Rows     []pricingRowPayload `json:"rows"`
}

type pricingRowPayload struct {
	ID        int     `json:"id"`
	Territory string  `json:"territory"`
	GroupCode string  `json:"group_code"`
	MealType  string  `json:"meal_type"`
	Amount    float64 `json:"amount"`
	SortOrder int     `json:"sort_order"`
}

type expensePricingReviewPayload struct {
	ExpenseID        int     `json:"expense_id"`
	PricingTableID   int     `json:"pricing_table_id"`
	GroupCode        string  `json:"group_code"`
	Territory        string  `json:"territory"`
	CalculatedAmount float64 `json:"calculated_amount"`
}

type mileageReviewPayload struct {
	ExpenseID        int     `json:"expense_id"`
	DistanceKM       float64 `json:"distance_km"`
	MileageRate      float64 `json:"mileage_rate"`
	CalculatedAmount float64 `json:"calculated_amount"`
}

type expenseAdvancePayload struct {
	ExpenseID               int     `json:"expense_id"`
	ResearcherAdvanceAmount float64 `json:"researcher_advance_amount"`
}

type expenseEditPayload struct {
	ExpenseID               int     `json:"expense_id"`
	Description             string  `json:"description"`
	TransportMethod         string  `json:"transport_method"`
	Amount                  float64 `json:"amount"`
	ResearcherAdvanceAmount float64 `json:"researcher_advance_amount"`
}

func handleAvailableTravels(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, _, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	today := time.Now().Format("2006-01-02")

	travelsQuery := map[string]interface{}{
		"table": `
			budget_requests br
			INNER JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN budget_attendance_certificates bac ON bac.id_combined = br.id
		`,
		"columns": `
			br.id AS id,
			br.id_intern AS code,
			bp.purpose AS purpose,
			bp.institution AS destination,
			bp.fromDay AS startDate,
			bp.untilDay AS endDate,
			bp.travel_fromPlace AS fromWhere,
			bp.wherePlace AS toWhere,
			bp.project_id AS project,
			COALESCE(bac.file_uploaded, 0) AS attendance_certificate_uploaded
		`,
		"condition": fmt.Sprintf(
			`br.people_id = %d
			AND br.acc_or_it_response = 1
			AND br.canceled = 0
			AND (br.denied_comment = '' OR br.denied_comment IS NULL)
			AND bp.category_id = 1
			AND date(bp.untilDay) <= date('%s')
			AND NOT EXISTS (
				SELECT 1
				FROM service_commisions sc
				WHERE sc.request_travel_id = br.id
				AND COALESCE(sc.status, '') <> '%s'
			)`,
			userID,
			today,
			commissionStatusRejected,
		),
	}

	proxyJSONGet(travelsQuery, apiKey, client, w)
}

func handleGetProjects(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	today := time.Now().Format("2006-01-02")

	query := map[string]interface{}{
		"table": `
			projects p
			LEFT JOIN people_projects pp ON p.id = pp.project_id
		`,
		"columns": `
			p.id AS id,
			p.short_name AS short_name
		`,
		"condition": fmt.Sprintf(
			`pp.people_id = %d
			AND (p.end_date >= '%s' OR p.end_date IS NULL)
			ORDER BY p.short_name ASC`,
			userID,
			today,
		),
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		createLog(fmt.Sprintf("Error preparing projects query for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error preparing projects query", http.StatusInternalServerError)
		return
	}

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No response fetching projects for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Could not fetch projects", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("DB error fetching projects. Status: %d Body: %s User: %s", resp.StatusCode, string(bodyBytes), username), 1, apiKey, client, w)
		http.Error(w, "Error fetching projects", http.StatusInternalServerError)
		return
	}

	var projects []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		createLog(fmt.Sprintf("Error decoding projects for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding projects", http.StatusInternalServerError)
		return
	}

	for i := range projects {
		if shortName, ok := projects[i]["short_name"].(string); ok {
			projects[i]["short_name"] = unescapeComma(shortName)
		}
	}

	projects = appendContractProgramProjectIfEligible(projects, userID, today, apiKey, client, w)

	respondJSON(w, http.StatusOK, projects)
}

func appendContractProgramProjectIfEligible(projects []map[string]interface{}, userID int, today string, apiKey string, client *http.Client, w http.ResponseWriter) []map[string]interface{} {
	if !hasActiveContractProgramContract(userID, today, apiKey, client, w) {
		return projects
	}

	for _, project := range projects {
		if isContractProgramProjectID(fmt.Sprint(project["id"])) {
			return projects
		}
	}

	return append(projects, map[string]interface{}{
		"id":                  contractProgramProjectID,
		"number":              contractProgramProjectID,
		"short_name":          contractProgramProjectName,
		"is_contract_program": true,
	})
}

func isContractProgramProjectID(projectID string) bool {
	projectID = strings.TrimSpace(projectID)
	return strings.EqualFold(projectID, contractProgramProjectID) ||
		strings.EqualFold(projectID, contractProgramLegacyProjectID)
}

func serviceCommissionProjectListNameSQL() string {
	return fmt.Sprintf(
		"CASE WHEN COALESCE(NULLIF(ct.project, ''), bp.project_id) IN ('%s', '%s') THEN '%s' ELSE COALESCE(pr.short_name, ct.project, bp.project_id, '') END",
		contractProgramProjectID,
		contractProgramLegacyProjectID,
		contractProgramProjectName,
	)
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

func handleCreateCommission(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _, _ := getUserInfoWithProjects(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04")
	travelSource := strings.TrimSpace(r.FormValue("travelSource"))
	if travelSource == "" {
		travelSource = "existing"
	}

	var requestTravelID int
	var newTravelID int
	storage := fileStorageContext{
		Username:  username,
		Timestamp: now.Format("20060102_150405"),
	}

	expenses := readExpensesFromMultipart(r)
	if len(expenses) == 0 {
		http.Error(w, "At least one expense is required", http.StatusBadRequest)
		return
	}

	if err := validateMileageProofUploads(r, expenses); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if travelSource == "existing" {
		budgetingTravelID := parseIntFormValue(r, "budgetingTravelId")
		if budgetingTravelID == 0 {
			http.Error(w, "Budgeting travel is required", http.StatusBadRequest)
			return
		}
		uploaded, err := existingTravelHasAttendanceCertificate(apiKey, client, w, budgetingTravelID, userID)
		if err != nil {
			createLog(fmt.Sprintf("Error checking attendance certificate for existing travel %d: %v", budgetingTravelID, err), 1, apiKey, client, w)
			http.Error(w, "Could not verify attendance certificate", http.StatusInternalServerError)
			return
		}
		if !uploaded {
			http.Error(w, "Attendance certificate is required before creating a service commission for this travel", http.StatusBadRequest)
			return
		}
		existingStorage, err := getExistingTravelStorageContext(apiKey, client, w, budgetingTravelID)
		if err != nil {
			createLog(fmt.Sprintf("Error loading storage context for existing travel %d: %v", budgetingTravelID, err), 1, apiKey, client, w)
			http.Error(w, "Could not prepare upload folders", http.StatusInternalServerError)
			return
		}
		storage.ProjectCode = existingStorage.ProjectCode
		storage.TravelCode = existingStorage.TravelCode
		requestTravelID = budgetingTravelID
		fmt.Printf("Creating commission for existing travel ID: %d\n", budgetingTravelID)
	} else {
		travelID, manualStorage, err := createManualTravel(apiKey, client, w, r, userID, username, now)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newTravelID = travelID
		storage.ProjectCode = manualStorage.ProjectCode
		storage.TravelCode = manualStorage.TravelCode
	}

	peopleCategory := peopleCategoryLabel(calculatePeopleCategory(userID, apiKey, client, w))
	serviceID, err := insertRowAndReturnID(apiKey, client, w, "service_commisions",
		"people_id, creation_date, people_category, request_travel_id, new_travel_id, status, updated_at",
		fmt.Sprintf("%d, %s, %s, %d, %d, %s, %s", userID, nowStr, peopleCategory, requestTravelID, newTravelID, commissionStatusSubmitted, nowStr))

	if err != nil || serviceID == 0 {
		http.Error(w, "Could not create service commission", http.StatusInternalServerError)
		return
	}

	if travelSource != "existing" && newTravelID != 0 {
		manualCode := buildManualCommissionCode(now, strings.TrimSpace(r.FormValue("travel_project")), serviceID)
		if err := updateManualTravelCode(apiKey, client, w, newTravelID, manualCode); err != nil {
			createLog(fmt.Sprintf("Error updating manual travel code for commission %d: %v", serviceID, err), 1, apiKey, client, w)
			http.Error(w, "Could not update commission code", http.StatusInternalServerError)
			return
		}
		storage.TravelCode = manualCode
	}

	for index, expense := range expenses {
		expenseAmount := expense.Amount
		mileageRate := 0.0
		if expense.Type == "mileage" {
			expenseAmount = 0
			mileageRate = 0.30
		}

		expenseID, err := insertRowAndReturnID(apiKey, client, w, "service_commission_expenses",
			"service_id, type, amount, transport_method, description, purpose, departure_date, return_date, departure_time, return_time, per_diem_breakdown, distance_km, mileage_rate",
			fmt.Sprintf("%d, %s, %f, %s, %s, %s, %s, %s, %s, %s, %s, %.2f, %.2f", serviceID, escapeComma(expense.Type), expenseAmount, escapeComma(expense.TransportMethod), escapeComma(expense.Description), userExpensePurpose, (expense.DepartureDate), (expense.ReturnDate), (expense.DepartureTime), (expense.ReturnTime), escapeComma(expense.PerDiemBreakdown), expense.DistanceKM, mileageRate))
		if err != nil || expenseID == 0 {
			http.Error(w, "Could not create expense", http.StatusInternalServerError)
			return
		}

		saveExpenseUploadFiles(apiKey, client, w, r, expense, storage, index, expenseID)
	}

	if requestTravelID != 0 {
		if err := ensureBudgetCenterExpenses(apiKey, client, w, serviceID); err != nil {
			createLog(fmt.Sprintf("Error creating budget center expenses for commission %d: %v", serviceID, err), 1, apiKey, client, w)
			http.Error(w, "Could not create budget center expenses", http.StatusInternalServerError)
			return
		}
	}

	if err := sendServiceCommissionCreatedProjectsEmail(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Service commission %d created but projects email failed: %v", serviceID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":     serviceID,
		"status": commissionStatusSubmitted,
	})
}

func ensureBudgetCenterExpenses(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commisions",
		"columns":   "request_travel_id, status",
		"condition": fmt.Sprintf("id = %d", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("commission not found")
	}

	budgetingTravelID := intFromAny(rows[0]["request_travel_id"])
	if budgetingTravelID == 0 {
		return nil
	}
	status := strings.TrimSpace(fmt.Sprint(rows[0]["status"]))

	existing, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "id",
		"condition": fmt.Sprintf("service_id = %d AND COALESCE(excluded, 0) = 0 AND purpose = '%s' AND type IN ('%s', '%s', '%s')", serviceID, centerExpensePurpose, budgetTravelExpenseType, budgetRegistrationType, budgetAccommodationType),
	}, apiKey, client, w)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return ensureBudgetCenterExpenseDefaults(apiKey, client, w, serviceID, budgetingTravelID)
	}
	if status != "" && status != commissionStatusSubmitted {
		return nil
	}

	return createBudgetCenterExpenses(apiKey, client, w, serviceID, budgetingTravelID)
}

func createBudgetCenterExpenses(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, budgetingTravelID int) error {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "category_id, purpose, institution, wherePlace, fromDay, untilDay",
		"condition": fmt.Sprintf("id_combined = %d AND category_id IN (1, 2, 3)", budgetingTravelID),
		"order":     "category_id ASC, id ASC",
	}, apiKey, client, w)
	if err != nil {
		return err
	}

	for _, row := range rows {
		expenseType := serviceCommissionBudgetExpenseType(intFromAny(row["category_id"]))
		if expenseType == "" {
			continue
		}
		description := serviceCommissionBudgetDefaultDescription(row)
		_, err := insertRowAndReturnID(apiKey, client, w, "service_commission_expenses",
			"service_id, type, amount, transport_method, description, purpose, departure_date, return_date, departure_time, return_time, per_diem_breakdown, distance_km, mileage_rate, related_task, location_label",
			fmt.Sprintf("%d, %s, %.2f, %s, %s, %s, %s, %s, %s, %s, %s, %.2f, %.2f, %s, %s",
				serviceID,
				escapeComma(expenseType),
				0.0,
				"",
				escapeComma(description),
				centerExpensePurpose,
				serviceCommissionDateString(fmt.Sprint(row["fromDay"])),
				serviceCommissionDateString(fmt.Sprint(row["untilDay"])),
				"",
				"",
				"",
				0.0,
				0.0,
				escapeComma(unescapeComma(fmt.Sprint(row["purpose"]))),
				escapeComma(serviceCommissionBudgetLocation(row)),
			))
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureBudgetCenterExpenseDefaults(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, budgetingTravelID int) error {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "category_id, purpose",
		"condition": fmt.Sprintf("id_combined = %d AND category_id IN (1, 2, 3)", budgetingTravelID),
		"order":     "id ASC",
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		return err
	}

	for _, row := range rows {
		expenseType := serviceCommissionBudgetExpenseType(intFromAny(row["category_id"]))
		if expenseType == "" {
			continue
		}
		task := strings.TrimSpace(unescapeComma(fmt.Sprint(row["purpose"])))
		if task == "" || task == "<nil>" {
			continue
		}
		if err := updateRowColumns(apiKey, client, w, "service_commission_expenses",
			"related_task",
			escapeComma(task),
			fmt.Sprintf("service_id = %d AND type = '%s' AND purpose = '%s' AND (related_task IS NULL OR related_task = '')", serviceID, expenseType, centerExpensePurpose)); err != nil {
			return err
		}
		if expenseType == budgetRegistrationType {
			if err := updateRowColumns(apiKey, client, w, "service_commission_expenses",
				"description",
				escapeComma(task),
				fmt.Sprintf("service_id = %d AND type = '%s' AND purpose = '%s' AND (description IS NULL OR description = '')", serviceID, budgetRegistrationType, centerExpensePurpose)); err != nil {
				return err
			}
		}
	}

	return nil
}

func serviceCommissionBudgetDefaultDescription(row map[string]interface{}) string {
	if intFromAny(row["category_id"]) != 2 {
		return ""
	}
	description := strings.TrimSpace(unescapeComma(fmt.Sprint(row["purpose"])))
	if description == "<nil>" {
		return ""
	}
	return description
}

func serviceCommissionBudgetExpenseType(categoryID int) string {
	switch categoryID {
	case 1:
		return budgetTravelExpenseType
	case 2:
		return budgetRegistrationType
	case 3:
		return budgetAccommodationType
	default:
		return ""
	}
}

func isBudgetCenterExpenseType(expenseType string) bool {
	switch strings.TrimSpace(strings.ToLower(expenseType)) {
	case budgetTravelExpenseType, budgetAccommodationType, budgetRegistrationType:
		return true
	default:
		return false
	}
}

func serviceCommissionBudgetLocation(row map[string]interface{}) string {
	destination := strings.TrimSpace(unescapeComma(fmt.Sprint(row["institution"])))
	toWhere := strings.TrimSpace(unescapeComma(fmt.Sprint(row["wherePlace"])))
	if destination == "<nil>" {
		destination = ""
	}
	if toWhere == "<nil>" {
		toWhere = ""
	}
	if destination == "" || strings.EqualFold(destination, toWhere) {
		return toWhere
	}
	if toWhere == "" {
		return destination
	}
	return destination + ", " + toWhere
}

func calculatePeopleCategory(userID int, apiKey string, client *http.Client, w http.ResponseWriter) int {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "people",
		"columns":   "academic_grade",
		"condition": fmt.Sprintf("id = %d", userID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		return 0
	}

	return peopleCategoryFromAcademicGrade(fmt.Sprint(rows[0]["academic_grade"]))
}

func peopleCategoryFromAcademicGrade(value string) int {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "1", "2", "6", "7":
		return 1
	case "3", "4", "5", "G":
		return 2
	default:
		return 0
	}
}

func peopleCategoryLabel(value int) string {
	if value < 0 || value > 2 {
		value = 0
	}
	return fmt.Sprintf("G%d", value+1)
}

func peopleCategoryLabelFromAny(value interface{}) string {
	text := strings.ToUpper(strings.TrimSpace(fmt.Sprint(value)))
	if text == "G1" || text == "G2" || text == "G3" {
		return text
	}
	return peopleCategoryLabel(intFromAny(value))
}

func createManualTravel(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, userID int, username string, now time.Time) (int, fileStorageContext, error) {
	fromWhere := strings.TrimSpace(r.FormValue("travel_from_where"))
	toWhere := strings.TrimSpace(r.FormValue("travel_to_where"))
	destination := strings.TrimSpace(r.FormValue("destination"))
	purpose := strings.TrimSpace(r.FormValue("travelPurpose"))
	startDate := strings.TrimSpace(r.FormValue("startDate"))
	endDate := strings.TrimSpace(r.FormValue("endDate"))
	project := strings.TrimSpace(r.FormValue("travel_project"))
	storage := fileStorageContext{
		Username:   username,
		TravelCode: fmt.Sprintf("COMM-%d-%s", userID, now.Format("20060102150405")),
		Timestamp:  now.Format("20060102_150405"),
	}

	if fromWhere == "" || toWhere == "" || destination == "" || purpose == "" || startDate == "" || endDate == "" {
		return 0, storage, fmt.Errorf("manual travel requires origin, destination, purpose and dates")
	}

	if !multipartHasFiles(r, "travel_attendance_certificate") {
		return 0, storage, fmt.Errorf("attendance certificate is required")
	}

	if isContractProgramProjectID(project) && !hasActiveContractProgramContract(userID, now.Format("2006-01-02"), apiKey, client, w) {
		return 0, storage, fmt.Errorf("Contracte Programa is only available for people with an active contract programme contract")
	}

	projectCode, err := getProjectCode(apiKey, client, w, project)
	if err != nil {
		return 0, storage, err
	}
	storage.ProjectCode = projectCode
	travelFilePath := saveTravelFiles(r, storage)

	travelID, err := insertRowAndReturnID(apiKey, client, w, "comm_travels",
		"id_intern, from_where, to_where, destination, purpose, start_date, end_date, project, file_path",
		fmt.Sprintf("%s, %s, %s, %s, %s, %s, %s, %s, %s", storage.TravelCode, escapeComma(fromWhere), escapeComma(toWhere), escapeComma(destination), escapeComma(purpose), startDate, endDate, escapeComma(project), travelFilePath))

	return travelID, storage, err
}

func buildManualCommissionCode(now time.Time, projectCode string, serviceID int) string {
	codeProject := strings.TrimSpace(projectCode)
	if codeProject == "" {
		codeProject = "UNKNOWN"
	}

	return fmt.Sprintf("COMM-%d-%s-%05d", now.Year(), codeProject, serviceID)
}

func updateManualTravelCode(apiKey string, client *http.Client, w http.ResponseWriter, travelID int, code string) error {
	return updateRowColumns(apiKey, client, w, "comm_travels",
		"id_intern",
		escapeComma(code),
		fmt.Sprintf("id = %d", travelID))
}

func existingTravelHasAttendanceCertificate(apiKey string, client *http.Client, w http.ResponseWriter, travelID int, userID int) (bool, error) {
	query := map[string]interface{}{
		"table": `budget_requests br
			LEFT JOIN budget_attendance_certificates bac ON bac.id_combined = br.id`,
		"columns":   "COALESCE(bac.file_uploaded, 0) AS file_uploaded",
		"condition": fmt.Sprintf("br.id = %d AND br.people_id = %d", travelID, userID),
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		return false, err
	}

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return false, fmt.Errorf("empty response checking attendance certificate")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("attendance certificate check failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, fmt.Errorf("travel not found")
	}

	return isUploadedValue(rows[0]["file_uploaded"]), nil
}

func isUploadedValue(value interface{}) bool {
	if asBool(value) {
		return true
	}

	return intFromAny(value) == 1
}

func getExistingTravelStorageContext(apiKey string, client *http.Client, w http.ResponseWriter, travelID int) (fileStorageContext, error) {
	query := map[string]interface{}{
		"table": `budget_requests br
			INNER JOIN budget_parts bp ON br.id = bp.id_combined
			LEFT JOIN projects p ON bp.project_id = p.id`,
		"columns":   "br.id_intern AS travel_code, COALESCE(p.short_name, bp.project_id, '') AS project_code",
		"condition": fmt.Sprintf("br.id = %d AND bp.category_id = 1", travelID),
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		return fileStorageContext{}, err
	}

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return fileStorageContext{}, fmt.Errorf("empty response getting existing travel storage context")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fileStorageContext{}, fmt.Errorf("existing travel storage context failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return fileStorageContext{}, err
	}
	if len(rows) == 0 {
		return fileStorageContext{}, fmt.Errorf("travel not found")
	}

	return fileStorageContext{
		TravelCode:  strings.TrimSpace(fmt.Sprint(rows[0]["travel_code"])),
		ProjectCode: strings.TrimSpace(unescapeComma(fmt.Sprint(rows[0]["project_code"]))),
	}, nil
}

func getProjectCode(apiKey string, client *http.Client, w http.ResponseWriter, projectID string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "NO_PROJECT", nil
	}
	if isContractProgramProjectID(projectID) {
		return contractProgramProjectID, nil
	}

	query := map[string]interface{}{
		"table":     "projects",
		"columns":   "COALESCE(short_name, id) AS project_code",
		"condition": fmt.Sprintf("id = '%s'", escapeSQL(projectID)),
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		return "", err
	}

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return "", fmt.Errorf("empty response getting project code")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("project code query failed: %d - %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return projectID, nil
	}

	projectCode := strings.TrimSpace(unescapeComma(fmt.Sprint(rows[0]["project_code"])))
	if projectCode == "" {
		return projectID, nil
	}

	return projectCode, nil
}

func handleMyCommissions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, _, _ := getUserInfo(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	query := map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN service_commission_expenses e ON sc.id = e.service_id AND COALESCE(e.excluded, 0) = 0
LEFT JOIN service_commission_expense_pricing_reviews epr ON e.id = epr.expense_id`,
		"columns": `sc.id,
COALESCE(ct.id_intern, br.id_intern, 'SC-' || sc.id) AS code,
COALESCE(ct.purpose, bp.purpose, '') AS travel,
COALESCE(ct.destination, NULLIF(bp.institution, ''), bp.wherePlace, '') AS destination,
COALESCE(ct.project, '') AS project,
sc.creation_date AS submitted,
sc.status,
COALESCE(SUM(COALESCE(epr.calculated_amount, e.amount, 0)), 0) AS total`,
		"condition": fmt.Sprintf("sc.people_id = %d GROUP BY sc.id ORDER BY sc.creation_date DESC", userID),
	}

	rows, err := runDBGet(query, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading my commissions: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not load my commissions", http.StatusInternalServerError)
		return
	}

	for index := range rows {
		for _, key := range []string{"travel", "project", "destination"} {
			if value, ok := rows[index][key].(string); ok {
				rows[index][key] = unescapeComma(value)
			}
		}
	}

	respondJSON(w, http.StatusOK, rows)
}

func handleProjectCommissions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if userID == 0 {
		return
	}
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	canViewAttendance := userCanViewAttendanceCertificates(role, accessProjects, accessGerencia)
	attendancePathColumn := "'' AS attendance_certificate_path"
	if canViewAttendance {
		attendancePathColumn = "COALESCE(NULLIF(ct.file_path, ''), bac.file_path, '') AS attendance_certificate_path"
	}
	projectNameExpr := serviceCommissionProjectListNameSQL()

	query := map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN people p ON sc.people_id = p.id
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_attendance_certificates bac ON bac.id_combined = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN projects pr ON pr.id = COALESCE(NULLIF(ct.project, ''), bp.project_id)
LEFT JOIN service_commission_expenses e ON sc.id = e.service_id AND COALESCE(e.excluded, 0) = 0
LEFT JOIN service_commission_expense_pricing_reviews epr ON e.id = epr.expense_id`,
		"columns": `sc.id,
COALESCE(ct.id_intern, br.id_intern, 'SC-' || sc.id) AS code,
TRIM(COALESCE(p.name, '') || ' ' || COALESCE(p.surname, '') || ' ' || COALESCE(p.secondSurname, '')) AS applicant,
sc.people_id AS applicant_id,
` + projectNameExpr + ` AS project,
COALESCE(pr.type, '') AS project_type,
COALESCE(ct.destination, bp.institution, '') AS destination,
sc.status,
` + attendancePathColumn + `,
CASE WHEN COALESCE(NULLIF(ct.file_path, ''), bac.file_path, '') IS NOT NULL AND COALESCE(NULLIF(ct.file_path, ''), bac.file_path, '') <> '' THEN 1 ELSE 0 END AS attendance_certificate_available,
COALESCE(SUM(CASE WHEN e.purpose = 'center' THEN COALESCE(epr.calculated_amount, e.amount, 0) ELSE 0 END), 0) AS center_total,
COALESCE(SUM(CASE WHEN e.purpose IS NULL OR e.purpose <> 'center' THEN COALESCE(epr.calculated_amount, e.amount, 0) ELSE 0 END), 0) AS user_total`,
		"condition": "1=1 GROUP BY sc.id ORDER BY sc.updated_at DESC, sc.creation_date DESC",
	}

	rows, err := runDBGet(query, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading project commissions: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not load project commissions", http.StatusInternalServerError)
		return
	}

	for index := range rows {
		for _, key := range []string{"applicant", "project", "project_type", "destination"} {
			if value, ok := rows[index][key].(string); ok {
				rows[index][key] = unescapeComma(value)
			}
		}
	}

	respondJSON(w, http.StatusOK, rows)
}

func handleCommissionDetail(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	canViewAttendance := userCanViewAttendanceCertificates(role, accessProjects, accessGerencia)
	attendancePathColumn := "'' AS attendance_certificate_file"
	if canViewAttendance {
		attendancePathColumn = "COALESCE(NULLIF(ct.file_path, ''), bac.file_path, '') AS attendance_certificate_file"
	}

	serviceID := parseIntOrZero(r.URL.Query().Get("id"))
	if serviceID == 0 {
		http.Error(w, "Missing commission id", http.StatusBadRequest)
		return
	}
	if err := ensureBudgetCenterExpenses(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Error ensuring budget center expenses for commission detail %d: %v", serviceID, err), 1, apiKey, client, w)
		http.Error(w, "Could not load budget center expenses", http.StatusInternalServerError)
		return
	}
	projectNameExpr := serviceCommissionProjectListNameSQL()

	expensesQuery := map[string]interface{}{
		"table": `service_commission_expenses e
LEFT JOIN service_commisions sc ON e.service_id = sc.id
LEFT JOIN people p ON sc.people_id = p.id
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_attendance_certificates bac ON bac.id_combined = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN projects pr ON pr.id = COALESCE(NULLIF(ct.project, ''), bp.project_id)
LEFT JOIN service_commission_expense_pricing_reviews epr ON e.id = epr.expense_id
LEFT JOIN comm_files f ON e.id = f.expense_id`,
		"columns": `e.id, e.service_id, e.type, e.amount, e.transport_method, e.description, e.purpose, e.related_task, e.location_label, e.departure_date, e.return_date, e.departure_time, e.return_time, e.per_diem_breakdown,
e.distance_km,
e.mileage_rate,
sc.new_travel_id AS new_travel_id,
sc.request_travel_id AS request_travel_id,
sc.people_id AS applicant_id,
sc.people_category AS people_category,
TRIM(COALESCE(p.name, '') || ' ' || COALESCE(p.surname, '') || ' ' || COALESCE(p.secondSurname, '')) AS applicant,
p.academic_grade AS academic_grade,
ct.id_intern AS manual_travel_code,
ct.from_where AS manual_travel_from,
ct.to_where AS manual_travel_to,
ct.purpose AS manual_travel_purpose,
ct.start_date AS manual_travel_start,
ct.end_date AS manual_travel_end,
` + projectNameExpr + ` AS project,
COALESCE(pr.type, '') AS project_type,
COALESCE(ct.destination, bp.institution, '') AS destination,
COALESCE(ct.start_date, bp.fromDay, '') AS travel_start_date,
COALESCE(ct.end_date, bp.untilDay, '') AS travel_end_date,
COALESCE(NULLIF(e.related_task, ''), ct.purpose, bp.purpose, '') AS export_related_task,
epr.pricing_table_id AS pricing_table_id,
epr.id AS pricing_review_id,
epr.group_code AS pricing_group_code,
epr.territory AS pricing_territory,
epr.calculated_amount AS calculated_amount,
epr.reviewed_description AS reviewed_description,
COALESCE(epr.researcher_advance_amount, 0) AS researcher_advance_amount,
` + attendancePathColumn + `,
GROUP_CONCAT(CASE WHEN f.type = 'receipt' THEN f.file_path END) AS receipts,
GROUP_CONCAT(CASE WHEN f.type = 'payment_proof' THEN f.file_path END) AS payment_proofs,
GROUP_CONCAT(CASE WHEN f.type = 'mileage_proof' THEN f.file_path END) AS mileage_proofs,
GROUP_CONCAT(CASE WHEN f.type = 'travel' THEN f.file_path END) AS travel_files`,
		"condition": fmt.Sprintf("e.service_id = %d AND COALESCE(e.excluded, 0) = 0 GROUP BY e.id ORDER BY e.id", serviceID),
	}

	rows, err := runDBGet(expensesQuery, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading commission detail %d: %v", serviceID, err), 1, apiKey, client, w)
		http.Error(w, "Could not load commission detail", http.StatusInternalServerError)
		return
	}

	for index := range rows {
		for _, key := range []string{"description", "reviewed_description", "related_task", "location_label", "export_related_task", "per_diem_breakdown", "applicant", "project", "project_type", "destination", "manual_travel_from", "manual_travel_to", "manual_travel_purpose"} {
			if value, ok := rows[index][key].(string); ok {
				rows[index][key] = unescapeComma(value)
			}
		}
	}

	respondJSON(w, http.StatusOK, rows)
}

func handleApproveCommission(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		ID             int     `json:"id"`
		PeopleCategory *int    `json:"people_category"`
		RelatedTask    *string `json:"related_task"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := ensureBudgetCenterExpenses(apiKey, client, w, payload.ID); err != nil {
		createLog(fmt.Sprintf("Error ensuring budget center expenses before approval for service commission %d: %v", payload.ID, err), 1, apiKey, client, w)
		http.Error(w, "Could not verify budget center expenses", http.StatusInternalServerError)
		return
	}
	allReviewed, pending, err := serviceCommissionExpensesReviewed(apiKey, client, w, payload.ID)
	if err != nil {
		createLog(fmt.Sprintf("Error checking reviewed expenses for service commission %d: %v", payload.ID, err), 1, apiKey, client, w)
		http.Error(w, "Could not verify reviewed expenses", http.StatusInternalServerError)
		return
	}
	if !allReviewed {
		http.Error(w, fmt.Sprintf("All expenses must be reviewed before approving the commission. Pending expenses: %d", pending), http.StatusBadRequest)
		return
	}

	if payload.PeopleCategory != nil {
		if *payload.PeopleCategory < 0 || *payload.PeopleCategory > 2 {
			http.Error(w, "Invalid people category", http.StatusBadRequest)
			return
		}
		if err := updateCommissionPeopleCategory(apiKey, client, w, payload.ID, *payload.PeopleCategory); err != nil {
			createLog(fmt.Sprintf("Error updating service commission %d people category: %v", payload.ID, err), 1, apiKey, client, w)
			http.Error(w, "Could not update people category", http.StatusInternalServerError)
			return
		}
	}
	if payload.RelatedTask != nil {
		if err := updateCommissionRelatedTask(apiKey, client, w, payload.ID, *payload.RelatedTask); err != nil {
			createLog(fmt.Sprintf("Error updating service commission %d related task: %v", payload.ID, err), 1, apiKey, client, w)
			http.Error(w, "Could not update related task", http.StatusInternalServerError)
			return
		}
	}

	if err := startFinalApprovalFlow(apiKey, client, w, payload.ID, userID, role); err != nil {
		createLog(fmt.Sprintf("Error starting final approval flow for service commission %d: %v", payload.ID, err), 1, apiKey, client, w)
		http.Error(w, "Could not start final approval flow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":     payload.ID,
		"status": commissionStatusFinalReview,
	})
}

func updateCommissionRelatedTask(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, relatedTask string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := updateRowColumns(apiKey, client, w, "service_commission_expenses",
		"related_task",
		escapeComma(strings.TrimSpace(relatedTask)),
		fmt.Sprintf("service_id = %d AND (purpose IS NULL OR purpose <> '%s')", serviceID, centerExpensePurpose)); err != nil {
		return err
	}
	return updateRowColumns(apiKey, client, w, "service_commisions",
		"updated_at",
		now,
		fmt.Sprintf("id = %d", serviceID))
}

func handleSaveCommissionReviewFields(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		ID             int     `json:"id"`
		PeopleCategory *int    `json:"people_category"`
		RelatedTask    *string `json:"related_task"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !serviceCommissionReviewFieldsEditable(apiKey, client, w, payload.ID) {
		http.Error(w, "Commission cannot be edited", http.StatusBadRequest)
		return
	}

	if payload.PeopleCategory != nil {
		if *payload.PeopleCategory < 0 || *payload.PeopleCategory > 2 {
			http.Error(w, "Invalid people category", http.StatusBadRequest)
			return
		}
		if err := updateCommissionPeopleCategory(apiKey, client, w, payload.ID, *payload.PeopleCategory); err != nil {
			createLog(fmt.Sprintf("Error saving service commission %d people category: %v", payload.ID, err), 1, apiKey, client, w)
			http.Error(w, "Could not save people category", http.StatusInternalServerError)
			return
		}
	}

	if payload.RelatedTask != nil {
		if err := updateCommissionRelatedTask(apiKey, client, w, payload.ID, *payload.RelatedTask); err != nil {
			createLog(fmt.Sprintf("Error saving service commission %d related task: %v", payload.ID, err), 1, apiKey, client, w)
			http.Error(w, "Could not save related task", http.StatusInternalServerError)
			return
		}
	}

	if err := setCommissionUnderReview(apiKey, client, w, payload.ID); err != nil {
		createLog(fmt.Sprintf("Error setting commission %d under review after review field save: %v", payload.ID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":     payload.ID,
		"status": commissionStatusUnderReview,
	})
}

func serviceCommissionReviewFieldsEditable(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) bool {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commisions",
		"columns":   "status",
		"condition": fmt.Sprintf("id = %d", serviceID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(fmt.Sprint(rows[0]["status"]))) {
	case commissionStatusFinalReview, commissionStatusApproved, commissionStatusRejected, commissionStatusExported:
		return false
	default:
		return true
	}
}

func serviceCommissionExpensesReviewed(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) (bool, int, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table": `service_commission_expenses e
LEFT JOIN service_commission_expense_pricing_reviews epr ON e.id = epr.expense_id`,
		"columns":   "COUNT(e.id) AS total, SUM(CASE WHEN epr.id IS NULL THEN 1 ELSE 0 END) AS pending",
		"condition": fmt.Sprintf("e.service_id = %d AND COALESCE(e.excluded, 0) = 0", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return false, 0, err
	}
	if len(rows) == 0 {
		return false, 0, nil
	}
	total := intFromAny(rows[0]["total"])
	pending := intFromAny(rows[0]["pending"])
	return total > 0 && pending == 0, pending, nil
}

func updateCommissionPeopleCategory(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, peopleCategory int) error {
	return updateRowColumns(apiKey, client, w, "service_commisions",
		"people_category, updated_at",
		fmt.Sprintf("%s, %s", peopleCategoryLabel(peopleCategory), time.Now().Format("2006-01-02 15:04:05")),
		fmt.Sprintf("id = %d", serviceID))
}

func handleRejectCommission(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	updateCommissionStatus(apiKey, client, w, r, commissionStatusRejected)
}

func updateCommissionStatus(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, status string) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := updateRowColumns(apiKey, client, w, "service_commisions",
		"status, updated_at",
		fmt.Sprintf("%s, %s", status, time.Now().Format("2006-01-02 15:04:05")),
		fmt.Sprintf("id = %d", payload.ID)); err != nil {
		createLog(fmt.Sprintf("Error updating service commission %d status to %s: %v", payload.ID, status, err), 1, apiKey, client, w)
		http.Error(w, "Could not update commission status", http.StatusInternalServerError)
		return
	}

	if status == commissionStatusRejected {
		if err := sendServiceCommissionRejectedApplicantEmail(apiKey, client, w, payload.ID); err != nil {
			createLog(fmt.Sprintf("Service commission %d rejected but applicant email failed: %v", payload.ID, err), 1, apiKey, client, w)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":     payload.ID,
		"status": status,
	})
}

func startFinalApprovalFlow(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, actorID int, actorRole string) error {
	if err := ensureFinalApprovalSchema(apiKey, client, w); err != nil {
		return err
	}

	if err := auditServiceCommission(apiKey, client, w, serviceID, actorID, actorRole, "projects_initial_approval", map[string]interface{}{
		"message": "Projects has approved the service commission data and generated the draft PDF.",
	}); err != nil {
		return fmt.Errorf("creating initial audit log: %w", err)
	}

	draftPath, draftHash, err := generateServiceCommissionDraftPDF(apiKey, client, w, serviceID)
	if err != nil {
		return fmt.Errorf("generating draft PDF: %w", err)
	}

	if err := saveServiceCommissionDocument(apiKey, client, w, serviceID, draftPath, "", draftHash, "", "draft"); err != nil {
		return fmt.Errorf("saving draft document metadata: %w", err)
	}

	if err := ensureFinalApprovals(apiKey, client, w, serviceID); err != nil {
		return fmt.Errorf("creating final approval assignments: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	if err := updateRowColumns(apiKey, client, w, "service_commisions", "status, updated_at", fmt.Sprintf("%s, %s", commissionStatusFinalReview, now), fmt.Sprintf("id = %d", serviceID)); err != nil {
		return err
	}
	if err := notifyNextServiceCommissionApproval(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Could not notify next service commission approver for commission %d: %v", serviceID, err), 1, apiKey, client, w)
	}
	return nil
}

func ensureFinalApprovalSchema(apiKey string, client *http.Client, w http.ResponseWriter) error {
	requiredTables := []string{
		"service_commission_documents",
		"service_commission_approvals",
		"service_commission_audit_logs",
	}

	for _, table := range requiredTables {
		rows, err := runDBGet(map[string]interface{}{
			"table":     "sqlite_master",
			"columns":   "name",
			"condition": fmt.Sprintf("type = 'table' AND name = '%s'", escapeSQL(table)),
		}, apiKey, client, w)
		if err != nil {
			return fmt.Errorf("checking migration table %s: %w", table, err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("missing database table %s; apply Migrations/20260527_module10_service_commission_approvals.sql", table)
		}
	}

	return nil
}

func handleStakeholderCommissions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, _, _, _ := getUserInfoForRateManagement(apiKey, client, w, r)
	if userID == 0 {
		return
	}
	projectNameExpr := serviceCommissionProjectListNameSQL()

	rows, err := runDBGet(map[string]interface{}{
		"table": `service_commission_approvals a
INNER JOIN service_commisions sc ON sc.id = a.service_id
LEFT JOIN service_commission_documents d ON d.service_id = sc.id
LEFT JOIN people p ON sc.people_id = p.id
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN projects pr ON pr.id = COALESCE(NULLIF(ct.project, ''), bp.project_id)`,
		"columns": `a.id AS approval_id,
a.service_id,
a.approver_type,
a.status AS approval_status,
a.approved_at,
sc.status AS commission_status,
COALESCE(ct.id_intern, br.id_intern, 'SC-' || sc.id) AS code,
TRIM(COALESCE(p.name, '') || ' ' || COALESCE(p.surname, '') || ' ' || COALESCE(p.secondSurname, '')) AS applicant,
` + projectNameExpr + ` AS project,
COALESCE(ct.destination, bp.institution, '') AS destination,
d.draft_pdf_path,
d.signed_pdf_path`,
		"condition": fmt.Sprintf("a.approver_people_id = %d AND (a.status = 'approved' OR %s) ORDER BY sc.updated_at DESC, a.id DESC", userID, activeServiceCommissionApprovalCondition("a")),
	}, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading stakeholder approvals for user %d: %v", userID, err), 1, apiKey, client, w)
		http.Error(w, "Could not load pending approvals", http.StatusInternalServerError)
		return
	}

	for index := range rows {
		for _, key := range []string{"applicant", "project", "destination"} {
			if value, ok := rows[index][key].(string); ok {
				rows[index][key] = unescapeComma(value)
			}
		}
	}

	respondJSON(w, http.StatusOK, rows)
}

func handleApproveStakeholder(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, role, _, _ := getUserInfoForRateManagement(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	var payload struct {
		ApprovalID int    `json:"approval_id"`
		Message    string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ApprovalID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_approvals",
		"columns":   "id, service_id, approver_type, status",
		"condition": fmt.Sprintf("id = %d AND approver_people_id = %d", payload.ApprovalID, userID),
	}, apiKey, client, w)
	if err != nil {
		http.Error(w, "Could not validate approval", http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		http.Error(w, "Approval not found", http.StatusNotFound)
		return
	}
	if fmt.Sprint(rows[0]["status"]) == "approved" {
		respondJSON(w, http.StatusOK, map[string]interface{}{"status": "approved"})
		return
	}

	serviceID := intFromAny(rows[0]["service_id"])
	if active, err := serviceCommissionApprovalIsActive(apiKey, client, w, serviceID, payload.ApprovalID); err != nil {
		http.Error(w, "Could not validate approval order", http.StatusInternalServerError)
		return
	} else if !active {
		http.Error(w, "Previous approvals must be completed first", http.StatusBadRequest)
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = "Approved"
	}

	if err := updateRowColumns(apiKey, client, w, "service_commission_approvals",
		"status, approved_at, message, updated_at",
		fmt.Sprintf("approved, %s, %s, %s", now, escapeComma(message), now),
		fmt.Sprintf("id = %d AND approver_people_id = %d", payload.ApprovalID, userID)); err != nil {
		http.Error(w, "Could not approve", http.StatusInternalServerError)
		return
	}

	if err := auditServiceCommission(apiKey, client, w, serviceID, userID, role, "stakeholder_approval", map[string]interface{}{
		"approval_id":   payload.ApprovalID,
		"approver_type": fmt.Sprint(rows[0]["approver_type"]),
		"message":       message,
	}); err != nil {
		createLog(fmt.Sprintf("Error auditing stakeholder approval %d: %v", payload.ApprovalID, err), 1, apiKey, client, w)
		http.Error(w, "Could not audit approval", http.StatusInternalServerError)
		return
	}

	completed, err := finalApprovalsComplete(apiKey, client, w, serviceID)
	if err != nil {
		http.Error(w, "Could not verify approvals", http.StatusInternalServerError)
		return
	}
	if completed {
		if err := finalizeServiceCommissionPDF(apiKey, client, w, serviceID, userID, role); err != nil {
			createLog(fmt.Sprintf("Error finalizing service commission %d PDF: %v", serviceID, err), 1, apiKey, client, w)
			http.Error(w, fmt.Sprintf("Could not finalize signed PDF: %s", sanitizeModule10SignerOutput(err.Error())), http.StatusInternalServerError)
			return
		}
		if err := sendServiceCommissionSignedProjectsEmail(apiKey, client, w, serviceID); err != nil {
			createLog(fmt.Sprintf("Service commission %d signed but projects signed email failed: %v", serviceID, err), 1, apiKey, client, w)
		}
	} else if err := notifyNextServiceCommissionApproval(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Could not notify next service commission approver for commission %d: %v", serviceID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"service_id": serviceID,
		"completed":  completed,
	})
}

func handleDownloadFinalPDF(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if userID == 0 {
		return
	}

	serviceID := parseIntOrZero(r.URL.Query().Get("id"))
	if serviceID == 0 {
		http.Error(w, "Missing commission id", http.StatusBadRequest)
		return
	}

	if !userCanReviewProjects(role, accessProjects, accessGerencia) && !userCanAccessCommissionDocument(apiKey, client, w, serviceID, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_documents",
		"columns":   "signed_pdf_path, draft_pdf_path",
		"condition": fmt.Sprintf("service_id = %d ORDER BY id DESC LIMIT 1", serviceID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	path := strings.TrimSpace(fmt.Sprint(rows[0]["signed_pdf_path"]))
	if path == "" || path == "<nil>" {
		path = strings.TrimSpace(fmt.Sprint(rows[0]["draft_pdf_path"]))
	}
	if path == "" || path == "<nil>" {
		http.Error(w, "Document not available", http.StatusNotFound)
		return
	}

	fullPath := filepath.Clean(filepath.Join("../../", path))
	if !strings.HasPrefix(fullPath, filepath.Clean("../../Uploads")+string(os.PathSeparator)) {
		http.Error(w, "Invalid document path", http.StatusBadRequest)
		return
	}

	if strings.EqualFold(filepath.Ext(fullPath), ".zip") {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="service-commission-%d.zip"`, serviceID))
	} else {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="service-commission-%d.pdf"`, serviceID))
	}
	http.ServeFile(w, r, fullPath)
}

func handlePreviewCommissionPDF(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	//println("[module10] preview handler entered", r.URL.RawQuery)
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	//println("[module10] preview user info", "role=", role, "projects=", accessProjects, "gerencia=", accessGerencia)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		//println("[module10] preview forbidden")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	serviceID := parseIntOrZero(r.URL.Query().Get("id"))
	//println("[module10] preview service id", serviceID)
	if serviceID == 0 {
		http.Error(w, "Missing commission id", http.StatusBadRequest)
		return
	}

	var categoryOverride *int
	if categoryText := strings.TrimSpace(r.URL.Query().Get("people_category")); categoryText != "" {
		//println("[module10] preview category override raw", categoryText)
		category, err := strconv.Atoi(categoryText)
		if err != nil {
			http.Error(w, "Invalid people category", http.StatusBadRequest)
			return
		}
		if category < 0 || category > 2 {
			http.Error(w, "Invalid people category", http.StatusBadRequest)
			return
		}
		categoryOverride = &category
	}
	var relatedTaskOverride *string
	if relatedTask := strings.TrimSpace(r.URL.Query().Get("related_task")); relatedTask != "" {
		relatedTaskOverride = &relatedTask
	}

	path, _, err := generateServiceCommissionDraftPDFWithOptions(apiKey, client, w, serviceID, categoryOverride, relatedTaskOverride, true)
	if err != nil {
		//println("[module10] preview generation error", err.Error())
		createLog(fmt.Sprintf("Error generating service commission %d preview PDF: %v", serviceID, err), 1, apiKey, client, w)
		http.Error(w, "Could not generate PDF preview", http.StatusInternalServerError)
		return
	}
	//println("[module10] preview generated", path)

	fullPath := filepath.Clean(filepath.Join("../../", path))
	if !strings.HasPrefix(fullPath, filepath.Clean("../../Uploads")+string(os.PathSeparator)) {
		http.Error(w, "Invalid document path", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="service-commission-%d-preview.pdf"`, serviceID))
	http.ServeFile(w, r, fullPath)
}

func ensureFinalApprovals(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}
	if len(contextRows) == 0 {
		return fmt.Errorf("commission not found")
	}

	applicantID := intFromAny(contextRows[0]["applicant_id"])
	projectID := strings.TrimSpace(fmt.Sprint(contextRows[0]["project_id"]))

	approvers := []struct {
		kind string
		id   int
	}{{"applicant", applicantID}}

	if projectID != "" && projectID != "<nil>" && !isContractProgramProjectID(projectID) {
		projectPeople, err := runDBGet(map[string]interface{}{
			"table":     "people_projects",
			"columns":   "people_id",
			"condition": fmt.Sprintf("project_id = '%s' AND researcher_type = 'INVESTIGADOR PRINCIPAL'", escapeSQL(projectID)),
		}, apiKey, client, w)
		if err != nil {
			return err
		}
		if len(projectPeople) > 0 {
			row := projectPeople[0]
			if id := intFromAny(row["people_id"]); id != 0 && id != applicantID {
				approvers = append(approvers, struct {
					kind string
					id   int
				}{"project_responsible", id})
			}
		}

	}

	gerenciaRows, err := runDBGet(map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": "access_gerencia = 1",
	}, apiKey, client, w)
	if err != nil {
		return err
	}
	if len(gerenciaRows) > 0 {
		row := gerenciaRows[0]
		if id := intFromAny(row["people_id"]); id != 0 {
			approvers = append(approvers, struct {
				kind string
				id   int
			}{"management", id})
		}
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	for _, approver := range approvers {
		if approver.id == 0 {
			continue
		}
		existing, err := runDBGet(map[string]interface{}{
			"table":     "service_commission_approvals",
			"columns":   "id",
			"condition": fmt.Sprintf("service_id = %d AND approver_type = '%s' AND approver_people_id = %d", serviceID, approver.kind, approver.id),
		}, apiKey, client, w)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			continue
		}
		if _, err := insertRowAndReturnID(apiKey, client, w, "service_commission_approvals",
			"service_id, approver_type, approver_people_id, status, created_at, updated_at",
			fmt.Sprintf("%d, %s, %d, pending, %s, %s", serviceID, approver.kind, approver.id, now, now)); err != nil {
			return err
		}
	}

	return nil
}

func activeServiceCommissionApprovalCondition(alias string) string {
	return fmt.Sprintf(`COALESCE(%s.status, '') <> 'approved'
AND NOT EXISTS (
	SELECT 1
	FROM service_commission_approvals previous_approval
	WHERE previous_approval.service_id = %s.service_id
	AND COALESCE(previous_approval.status, '') <> 'approved'
	AND (
		(%s.approver_type = 'project_responsible' AND previous_approval.approver_type = 'applicant')
		OR (%s.approver_type = 'management' AND previous_approval.approver_type IN ('applicant', 'project_responsible'))
	)
)`, alias, alias, alias, alias)
}

func serviceCommissionApprovalIsActive(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, approvalID int) (bool, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_approvals a",
		"columns":   "a.id",
		"condition": fmt.Sprintf("a.service_id = %d AND a.id = %d AND %s", serviceID, approvalID, activeServiceCommissionApprovalCondition("a")),
	}, apiKey, client, w)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func nextServiceCommissionApprovalRows(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) ([]map[string]interface{}, error) {
	return runDBGet(map[string]interface{}{
		"table": `service_commission_approvals a
LEFT JOIN people approver ON a.approver_people_id = approver.id`,
		"columns": `a.id,
a.service_id,
a.approver_type,
a.approver_people_id,
TRIM(COALESCE(approver.name, '') || ' ' || COALESCE(approver.surname, '') || ' ' || COALESCE(approver.secondSurname, '')) AS approver_name,
COALESCE(approver.crm_email, '') AS approver_crm_email,
COALESCE(approver.user_email, '') AS approver_user_email`,
		"condition": fmt.Sprintf("a.service_id = %d AND %s ORDER BY CASE a.approver_type WHEN 'applicant' THEN 1 WHEN 'project_responsible' THEN 2 WHEN 'management' THEN 3 ELSE 4 END, a.id LIMIT 1", serviceID, activeServiceCommissionApprovalCondition("a")),
	}, apiKey, client, w)
}

func notifyNextServiceCommissionApproval(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	rows, err := nextServiceCommissionApprovalRows(apiKey, client, w, serviceID)
	if err != nil || len(rows) == 0 {
		return err
	}
	return sendServiceCommissionApprovalEmail(apiKey, client, w, serviceID, rows[0])
}

func sendServiceCommissionApprovalEmail(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, approvalRow map[string]interface{}) error {
	approverType := strings.TrimSpace(fmt.Sprint(approvalRow["approver_type"]))
	to := serviceCommissionApprovalEmailAddress(approvalRow)
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("email not found for approver type %s", approverType)
	}

	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}
	if len(contextRows) == 0 {
		return fmt.Errorf("commission not found")
	}
	subject := "Service Commission pending your validation"
	templateName := "serviceCommissionApproval.html"
	if approverType == "management" {
		subject = "Service Commission pending management validation"
		templateName = "serviceCommissionManagementApproval.html"
	}
	body, err := renderServiceCommissionEmailTemplate(templateName, buildServiceCommissionMailData(
		"Service commission pending validation",
		serviceCommissionApprovalIntro(approverType),
		serviceCommissionApprovalRecipientName(approvalRow),
		contextRows[0],
	))
	if err != nil {
		return err
	}
	notificationTitle := subject
	notificationContent := serviceCommissionNotificationContent(serviceCommissionApprovalIntro(approverType), contextRows[0])
	emailErr := sendEmail(to, subject, body)
	notificationErr := notifyServiceCommissionApprovalRecipient(notificationTitle, notificationContent, approvalRow, apiKey, client, w)
	return serviceCommissionDeliveryError(emailErr, notificationErr)
}

func sendServiceCommissionCreatedProjectsEmail(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}
	if len(contextRows) == 0 {
		return fmt.Errorf("commission not found")
	}
	body, err := renderServiceCommissionEmailTemplate("serviceCommissionProjectsCreated.html", buildServiceCommissionMailData(
		"New service commission created",
		"A new service commission has been created and is pending review by Projects.",
		"Projects team",
		contextRows[0],
	))
	if err != nil {
		return err
	}
	subject := "New Service Commission pending Projects review"
	emailErr := sendEmail(projectsEmail, subject, body)
	notificationErr := notifyServiceCommissionUsersByCondition(
		subject,
		serviceCommissionNotificationContent("A new service commission has been created and is pending review by Projects.", contextRows[0]),
		"access_projects = 1",
		apiKey,
		client,
		w,
	)
	return serviceCommissionDeliveryError(emailErr, notificationErr)
}

func sendServiceCommissionSignedProjectsEmail(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}
	if len(contextRows) == 0 {
		return fmt.Errorf("commission not found")
	}
	body, err := renderServiceCommissionEmailTemplate("serviceCommissionProjectsSigned.html", buildServiceCommissionMailData(
		"Service commission signed",
		"The service commission has completed all required validations and the signed PDF has been generated.",
		"Projects team",
		contextRows[0],
	))
	if err != nil {
		return err
	}
	subject := "Service Commission signed"
	emailErr := sendEmail(projectsEmail, subject, body)
	notificationErr := notifyServiceCommissionUsersByCondition(
		subject,
		serviceCommissionNotificationContent("The service commission has completed all required validations and the signed PDF has been generated.", contextRows[0]),
		"access_projects = 1",
		apiKey,
		client,
		w,
	)
	return serviceCommissionDeliveryError(emailErr, notificationErr)
}

func sendServiceCommissionRejectedApplicantEmail(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}
	if len(contextRows) == 0 {
		return fmt.Errorf("commission not found")
	}
	context := contextRows[0]
	to := serviceCommissionApplicantEmailAddress(context)
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("email not found for applicant")
	}

	intro := "Your service commission has been reviewed by Projects and has been rejected. Please check the request details in the Service Commissions module."
	body, err := renderServiceCommissionEmailTemplate("serviceCommissionRejectedApplicant.html", buildServiceCommissionMailData(
		"Service commission rejected",
		intro,
		serviceCommissionMailValue(context["applicant"]),
		context,
	))
	if err != nil {
		return err
	}

	subject := "Service Commission rejected"
	emailErr := sendEmail(to, subject, body)
	notificationErr := sendServiceCommissionNotification(
		subject,
		serviceCommissionNotificationContent(intro, context),
		intFromAny(context["applicant_id"]),
		apiKey,
		client,
		w,
	)
	return serviceCommissionDeliveryError(emailErr, notificationErr)
}

func notifyServiceCommissionApprovalRecipient(title string, content string, approvalRow map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) error {
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(approvalRow["approver_type"])), "management") {
		return notifyServiceCommissionUsersByCondition(title, content, "access_gerencia = 1", apiKey, client, w)
	}
	return sendServiceCommissionNotification(title, content, intFromAny(approvalRow["approver_people_id"]), apiKey, client, w)
}

func notifyServiceCommissionUsersByCondition(title string, content string, condition string, apiKey string, client *http.Client, w http.ResponseWriter) error {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "users",
		"columns":   "people_id",
		"condition": condition,
	}, apiKey, client, w)
	if err != nil {
		return err
	}

	var firstErr error
	for _, row := range rows {
		userID := intFromAny(row["people_id"])
		if userID == 0 {
			continue
		}
		if err := sendServiceCommissionNotification(title, content, userID, apiKey, client, w); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func sendServiceCommissionNotification(title string, content string, to int, apiKey string, client *http.Client, w http.ResponseWriter) error {
	if to == 0 {
		return fmt.Errorf("notification recipient is missing")
	}

	now := time.Now().Format("2006-01-02 15:04")
	notificationID, err := insertRowAndReturnID(apiKey, client, w, "notifications",
		"type, title, content, user_id, created_at",
		fmt.Sprintf("%s, %s, %s, %d, %s", escapeComma("Service Commissions"), escapeComma(title), escapeComma(content), serviceCommissionSystemUserID, now))
	if err != nil {
		return err
	}
	if notificationID == 0 {
		return fmt.Errorf("notification id was not returned")
	}

	_, err = insertRowAndReturnID(apiKey, client, w, "notification_user",
		"notification_id, user_id, can_read, can_download, seen",
		fmt.Sprintf("%d, %d, 1, 0, 0", notificationID, to))
	return err
}

func serviceCommissionNotificationContent(prefix string, context map[string]interface{}) string {
	return fmt.Sprintf(
		"%s Request: %s. Applicant: %s. Project: %s. Destination: %s.",
		strings.TrimSpace(prefix),
		serviceCommissionMailValue(context["code"]),
		serviceCommissionMailValue(context["applicant"]),
		serviceCommissionMailValue(context["project_name"]),
		serviceCommissionMailValue(context["destination"]),
	)
}

func serviceCommissionDeliveryError(emailErr error, notificationErr error) error {
	if emailErr != nil && notificationErr != nil {
		return fmt.Errorf("email failed: %v; notification failed: %w", emailErr, notificationErr)
	}
	if emailErr != nil {
		return emailErr
	}
	return notificationErr
}

func serviceCommissionApprovalEmailAddress(approvalRow map[string]interface{}) string {
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(approvalRow["approver_type"])), "management") {
		return managementEmail
	}
	crmEmail := strings.TrimSpace(fmt.Sprint(approvalRow["approver_crm_email"]))
	if crmEmail != "" && crmEmail != "<nil>" {
		return crmEmail
	}
	userEmail := strings.TrimSpace(fmt.Sprint(approvalRow["approver_user_email"]))
	if serviceCommissionAllowedUserDomain(userEmail) {
		return userEmail
	}
	return ""
}

func serviceCommissionApplicantEmailAddress(context map[string]interface{}) string {
	crmEmail := strings.TrimSpace(fmt.Sprint(context["applicant_crm_email"]))
	if crmEmail != "" && crmEmail != "<nil>" {
		return crmEmail
	}
	userEmail := strings.TrimSpace(fmt.Sprint(context["applicant_user_email"]))
	if serviceCommissionAllowedUserDomain(userEmail) {
		return userEmail
	}
	return ""
}

func serviceCommissionAllowedUserDomain(email string) bool {
	allowedSuffixes := []string{"crm.cat", "uab.cat", "ub.edu", "upc.edu", "icrea.cat"}
	at := strings.LastIndex(email, "@")
	if at == -1 || at == len(email)-1 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(domain, strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}

func serviceCommissionApprovalIntro(approverType string) string {
	switch strings.TrimSpace(strings.ToLower(approverType)) {
	case "applicant":
		return "Please confirm the service commission after Projects review."
	case "project_responsible":
		return "Please validate the service commission as the project responsible person."
	case "management":
		return "The service commission has completed the previous validations and is pending management validation."
	default:
		return "Please validate the service commission."
	}
}

type serviceCommissionMailData struct {
	Title       string
	Intro       string
	Recipient   string
	RequestCode string
	Applicant   string
	Project     string
	Destination string
	Purpose     string
	TravelDates string
	ModuleName  string
	LogoURL     string
	GeneratedAt string
}

func buildServiceCommissionMailData(title string, intro string, recipient string, context map[string]interface{}) serviceCommissionMailData {
	startDate := serviceCommissionMailValue(context["travel_start_date"])
	endDate := serviceCommissionMailValue(context["travel_end_date"])
	travelDates := strings.TrimSpace(startDate + " - " + endDate)
	if strings.Trim(travelDates, " -") == "" {
		travelDates = "-"
	}
	return serviceCommissionMailData{
		Title:       title,
		Intro:       intro,
		Recipient:   firstNonEmptyString(recipient, "there"),
		RequestCode: serviceCommissionMailValue(context["code"]),
		Applicant:   serviceCommissionMailValue(context["applicant"]),
		Project:     serviceCommissionMailValue(context["project_name"]),
		Destination: serviceCommissionMailValue(context["destination"]),
		Purpose:     serviceCommissionMailValue(context["purpose"]),
		TravelDates: travelDates,
		ModuleName:  "CRM Intratools Service Commissions",
		LogoURL:     "https://issues.crm.cat/assets/images/crmlogo.jfif",
		GeneratedAt: time.Now().Format("02/01/2006 15:04"),
	}
}

func serviceCommissionMailValue(value interface{}) string {
	text := strings.TrimSpace(unescapeComma(fmt.Sprint(value)))
	if text == "" || text == "<nil>" {
		return "-"
	}
	return text
}

func serviceCommissionApprovalRecipientName(approvalRow map[string]interface{}) string {
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(approvalRow["approver_type"])), "management") {
		return "Gerència"
	}
	return firstNonEmptyString(fmt.Sprint(approvalRow["approver_name"]), "there")
}

func renderServiceCommissionEmailTemplate(templateFile string, data serviceCommissionMailData) (string, error) {
	templatePath, err := resolveModule10ReadablePath(filepath.Join("module10workers", "assets", templateFile))
	if err != nil {
		return "", err
	}
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(templateFile).Parse(string(tmplContent))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func finalApprovalsComplete(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) (bool, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_approvals",
		"columns":   "COUNT(*) AS total, SUM(CASE WHEN status = 'approved' THEN 1 ELSE 0 END) AS approved",
		"condition": fmt.Sprintf("service_id = %d", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	total := intFromAny(rows[0]["total"])
	approved := intFromAny(rows[0]["approved"])
	return total > 0 && total == approved, nil
}

func userCanAccessCommissionDocument(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, userID int) bool {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commisions sc LEFT JOIN service_commission_approvals a ON a.service_id = sc.id",
		"columns":   "sc.id",
		"condition": fmt.Sprintf("sc.id = %d AND (sc.people_id = %d OR a.approver_people_id = %d)", serviceID, userID, userID),
	}, apiKey, client, w)
	return err == nil && len(rows) > 0
}

func getCommissionContext(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) ([]map[string]interface{}, error) {
	projectIDExpr := "COALESCE(NULLIF(ct.project, ''), bp.project_id)"
	return runDBGet(map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN people p ON sc.people_id = p.id
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN projects pr ON pr.id = COALESCE(NULLIF(ct.project, ''), bp.project_id)`,
		"columns": fmt.Sprintf(`sc.id,
sc.people_id AS applicant_id,
sc.request_travel_id AS request_travel_id,
TRIM(COALESCE(p.name, '') || ' ' || COALESCE(p.surname, '') || ' ' || COALESCE(p.secondSurname, '')) AS applicant,
COALESCE(p.nif, p.nif_extended, '') AS applicant_nif,
COALESCE(p.crm_email, '') AS applicant_crm_email,
COALESCE(p.user_email, '') AS applicant_user_email,
COALESCE(ct.id_intern, br.id_intern, 'SC-' || sc.id) AS code,
COALESCE(ct.project, bp.project_id, '') AS project_id,
CASE WHEN %s IN ('%s', '%s') THEN '%s' ELSE COALESCE(pr.name, pr.short_name, ct.project, bp.project_id, '') END AS project_name,
CASE WHEN %s IN ('%s', '%s') THEN '%s' ELSE COALESCE(pr.number, '') END AS project_number,
CASE WHEN %s IN ('%s', '%s') THEN '%s' ELSE COALESCE(pr.short_name, ct.project, bp.project_id, '') END AS project_code,
COALESCE(ct.destination, bp.institution, '') AS destination,
COALESCE(ct.to_where, bp.wherePlace, '') AS travel_to_where,
COALESCE(ct.purpose, bp.purpose, '') AS purpose,
COALESCE((SELECT related_task FROM service_commission_expenses WHERE service_id = sc.id AND (purpose IS NULL OR purpose <> 'center') AND related_task IS NOT NULL AND related_task <> '' LIMIT 1), '') AS related_task_override,
COALESCE(ct.start_date, bp.fromDay, '') AS travel_start_date,
COALESCE(ct.end_date, bp.untilDay, '') AS travel_end_date,
p.academic_grade AS academic_grade,
sc.people_category`,
			projectIDExpr, contractProgramProjectID, contractProgramLegacyProjectID, contractProgramProjectName,
			projectIDExpr, contractProgramProjectID, contractProgramLegacyProjectID, contractProgramProjectID,
			projectIDExpr, contractProgramProjectID, contractProgramLegacyProjectID, contractProgramProjectID),
		"condition": fmt.Sprintf("sc.id = %d", serviceID),
	}, apiKey, client, w)
}

func generateServiceCommissionDraftPDF(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) (string, string, error) {
	return generateServiceCommissionDraftPDFWithOptions(apiKey, client, w, serviceID, nil, nil, false)
}

func generateServiceCommissionDraftPDFWithOptions(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, peopleCategoryOverride *int, relatedTaskOverride *string, preview bool) (string, string, error) {
	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return "", "", err
	}
	if len(contextRows) == 0 {
		return "", "", fmt.Errorf("commission not found")
	}
	if peopleCategoryOverride != nil {
		contextRows[0]["people_category"] = peopleCategoryLabel(*peopleCategoryOverride)
	}
	if relatedTaskOverride != nil {
		contextRows[0]["related_task_override"] = strings.TrimSpace(*relatedTaskOverride)
	}
	if err := ensureBudgetCenterExpenses(apiKey, client, w, serviceID); err != nil {
		return "", "", err
	}
	expenses, err := runDBGet(map[string]interface{}{
		"table": `service_commission_expenses e
LEFT JOIN service_commission_expense_pricing_reviews epr ON e.id = epr.expense_id`,
		"columns": `e.id, e.type, e.amount, e.transport_method, e.description, epr.reviewed_description AS reviewed_description, e.purpose, e.related_task, e.location_label, e.departure_date, e.return_date, e.departure_time, e.return_time,
e.distance_km, e.mileage_rate, COALESCE(epr.calculated_amount, e.amount, 0) AS calculated_amount, COALESCE(epr.researcher_advance_amount, 0) AS researcher_advance_amount`,
		"condition": fmt.Sprintf("e.service_id = %d AND COALESCE(e.excluded, 0) = 0 ORDER BY e.id", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return "", "", err
	}

	now := time.Now()
	dir := filepath.Join("Uploads", "04_ServiceCommissions", "final_documents", fmt.Sprintf("SC-%d", serviceID))
	if err := os.MkdirAll(filepath.Join("../../", dir), 0750); err != nil {
		return "", "", err
	}

	xlsxPath := filepath.Join("../../", dir, fmt.Sprintf("service-commission-%d.xlsx", serviceID))
	pdfPath := filepath.Join("../../", dir, fmt.Sprintf("service-commission-%d-draft.pdf", serviceID))
	if preview {
		xlsxPath = filepath.Join("../../", dir, fmt.Sprintf("service-commission-%d-preview.xlsx", serviceID))
		pdfPath = filepath.Join("../../", dir, fmt.Sprintf("service-commission-%d-preview.pdf", serviceID))
	}
	if err := generateServiceCommissionPaymentPDFs(xlsxPath, pdfPath, contextRows[0], expenses, now); err != nil {
		return "", "", err
	}
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", "", err
	}

	return filepath.ToSlash(filepath.Join(dir, filepath.Base(pdfPath))), sha256Hex(pdfBytes), nil
}

type serviceCommissionPaymentExportGroup struct {
	key           string
	paymentMethod string
	expenses      []map[string]interface{}
}

func generateServiceCommissionPaymentPDFs(xlsxPath string, pdfPath string, context map[string]interface{}, expenses []map[string]interface{}, now time.Time) error {
	cleanupServiceCommissionGroupOutputs(xlsxPath)
	groups := serviceCommissionPaymentExportGroups(expenses)
	generatedPDFs := make([]string, 0, len(groups))
	for _, group := range groups {
		groupContext := cloneStringMap(context)
		groupContext["payment_method"] = group.paymentMethod
		groupXLSXPath := xlsxPath
		if len(groups) > 1 {
			ext := filepath.Ext(xlsxPath)
			groupXLSXPath = strings.TrimSuffix(xlsxPath, ext) + "-" + group.key + ext
		}
		if err := fillServiceCommissionTemplate(groupXLSXPath, groupContext, group.expenses, now); err != nil {
			return err
		}
		if err := convertSpreadsheetToPDF(groupXLSXPath, filepath.Dir(pdfPath)); err != nil {
			return err
		}
		generatedPDF := filepath.Join(filepath.Dir(pdfPath), strings.TrimSuffix(filepath.Base(groupXLSXPath), filepath.Ext(groupXLSXPath))+".pdf")
		generatedPDFs = append(generatedPDFs, generatedPDF)
	}

	if len(generatedPDFs) == 1 {
		if generatedPDFs[0] != pdfPath {
			_ = os.Rename(generatedPDFs[0], pdfPath)
		}
		return nil
	}

	return mergePDFs(generatedPDFs, pdfPath)
}

func cleanupServiceCommissionGroupOutputs(xlsxPath string) {
	ext := filepath.Ext(xlsxPath)
	base := strings.TrimSuffix(xlsxPath, ext)
	for _, key := range []string{"nomina", "transferencia"} {
		_ = os.Remove(base + "-" + key + ext)
		_ = os.Remove(base + "-" + key + ".pdf")
	}
}

func serviceCommissionPaymentExportGroups(expenses []map[string]interface{}) []serviceCommissionPaymentExportGroup {
	userExpenses, centerExpenses := splitCommissionExpenses(expenses)
	payrollExpenses := make([]map[string]interface{}, 0)
	transferExpenses := make([]map[string]interface{}, 0)
	for _, expense := range userExpenses {
		if serviceCommissionExpenseUsesTransfer(fmt.Sprint(expense["type"])) {
			transferExpenses = append(transferExpenses, expense)
		} else {
			payrollExpenses = append(payrollExpenses, expense)
		}
	}

	groups := make([]serviceCommissionPaymentExportGroup, 0, 2)
	if len(payrollExpenses) > 0 || len(transferExpenses) == 0 {
		groups = append(groups, serviceCommissionPaymentExportGroup{
			key:           "nomina",
			paymentMethod: "Nomina",
			expenses:      appendServiceCommissionExpenses(payrollExpenses, centerExpenses),
		})
	}
	if len(transferExpenses) > 0 {
		groups = append(groups, serviceCommissionPaymentExportGroup{
			key:           "transferencia",
			paymentMethod: "Transferència",
			expenses:      appendServiceCommissionExpenses(transferExpenses, centerExpenses),
		})
	}
	return groups
}

func serviceCommissionExpenseUsesTransfer(expenseType string) bool {
	switch strings.TrimSpace(strings.ToLower(expenseType)) {
	case "registration", "poster":
		return true
	default:
		return false
	}
}

func appendServiceCommissionExpenses(left []map[string]interface{}, right []map[string]interface{}) []map[string]interface{} {
	combined := make([]map[string]interface{}, 0, len(left)+len(right))
	combined = append(combined, left...)
	combined = append(combined, right...)
	return combined
}

func cloneStringMap(values map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{}, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func mergePDFs(inputPaths []string, outputPath string) error {
	args := append([]string{}, inputPaths...)
	args = append(args, outputPath)
	cmd := exec.Command("pdfunite", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pdfunite failed: %w - %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func fillServiceCommissionTemplate(outputPath string, context map[string]interface{}, expenses []map[string]interface{}, now time.Time) error {
	templatePath, err := resolveModule10ReadablePath("module10workers/assets/service_commission_template.xlsx")
	if err != nil {
		return err
	}
	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	userExpenses, centerExpenses := splitCommissionExpenses(expenses)
	if intFromAny(context["request_travel_id"]) == 0 {
		centerExpenses = nil
	}
	userRowCount := maxInt(1, len(userExpenses))
	rowOffset := userRowCount - 1
	hasCenterExpenses := len(centerExpenses) > 0
	centerExtraRows := 0
	centerDeletedRows := 0
	if hasCenterExpenses {
		centerExtraRows = maxInt(0, len(centerExpenses)-4)
		centerDeletedRows = maxInt(0, 4-len(centerExpenses))
	} else {
		centerDeletedRows = 8
	}
	summaryStartRow := 27 + rowOffset + centerExtraRows - centerDeletedRows
	replacements := buildServiceCommissionTemplateReplacements(context, userExpenses, centerExpenses, now, rowOffset, centerExtraRows, centerDeletedRows)

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			writer.Close()
			return err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			writer.Close()
			return err
		}
		if file.Name == "xl/worksheets/sheet1.xml" {
			sheetXML := expandSpreadsheetRows(string(content), 13, userRowCount)
			sheetXML = compactUserExpenseRowsIfNeeded(sheetXML, 13, context, userExpenses)
			if hasCenterExpenses {
				sheetXML = expandSpreadsheetRows(sheetXML, 23+rowOffset, centerExtraRows+1)
				if centerDeletedRows > 0 {
					centerStartRow := 20 + rowOffset
					sheetXML = deleteSpreadsheetRows(sheetXML, centerStartRow+len(centerExpenses), centerStartRow+3)
				}
				centerTotalRow := 25 + rowOffset + centerExtraRows - centerDeletedRows
				sheetXML = applyCenterAmountStyles(sheetXML, 20+rowOffset, len(centerExpenses), centerTotalRow)
			} else {
				sheetXML = deleteSpreadsheetRows(sheetXML, 18+rowOffset, 25+rowOffset)
			}
			sheetXML = ensureSpreadsheetDescriptionMerges(sheetXML, 13, userRowCount)
			if hasCenterExpenses {
				sheetXML = ensureCenterExpenseMerges(sheetXML, 20+rowOffset, len(centerExpenses))
			}
			sheetXML = applyServiceCommissionExportStyles(sheetXML, 13, userRowCount, 20+rowOffset, len(centerExpenses), summaryStartRow)
			sheetXML = restrictMainServiceCommissionSheet(sheetXML, 48+rowOffset+centerExtraRows-centerDeletedRows)
			sheetXML = replaceSpreadsheetCells(sheetXML, replacements)
			sheetXML = applyServiceCommissionExportStyles(sheetXML, 13, userRowCount, 20+rowOffset, len(centerExpenses), summaryStartRow)
			content = []byte(sheetXML)
		}
		if file.Name == "xl/sharedStrings.xml" {
			content = []byte(normalizeServiceCommissionSharedStrings(string(content)))
		}
		if file.Name == "xl/workbook.xml" {
			content = []byte(hideAuxiliaryWorkbookSheets(string(content)))
		}
		header := file.FileHeader
		out, err := writer.CreateHeader(&header)
		if err != nil {
			writer.Close()
			return err
		}
		if _, err := out.Write(content); err != nil {
			writer.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buffer.Bytes(), 0600)
}

type serviceCommissionExpenseSummary struct {
	kind        string
	transport   string
	description string
}

func splitCommissionExpenses(expenses []map[string]interface{}) ([]map[string]interface{}, []map[string]interface{}) {
	userExpenses := make([]map[string]interface{}, 0)
	centerExpenses := make([]map[string]interface{}, 0)
	for _, expense := range expenses {
		if fmt.Sprint(expense["purpose"]) == centerExpensePurpose {
			centerExpenses = append(centerExpenses, expense)
		} else {
			userExpenses = append(userExpenses, expense)
		}
	}
	return userExpenses, centerExpenses
}

func buildServiceCommissionTemplateReplacements(context map[string]interface{}, userExpenses []map[string]interface{}, centerExpenses []map[string]interface{}, now time.Time, rowOffset int, centerExtraRows int, centerDeletedRows int) map[string]string {
	userTotal := commissionExpenseRowsTotal(userExpenses)
	centerTotal := commissionExpenseRowsTotal(centerExpenses)
	researcherAdvanceTotal := commissionResearcherAdvanceRowsTotal(userExpenses)
	centerStartRow := 20 + rowOffset
	centerTotalRow := 25 + rowOffset + centerExtraRows
	summaryStartRow := 27 + rowOffset + centerExtraRows - centerDeletedRows

	replacements := map[string]string{
		"B7":                                  unescapeComma(fmt.Sprint(context["applicant"])),
		"E7":                                  firstNonEmptyString(fmt.Sprint(context["code"]), fmt.Sprintf("SC-%d", intFromAny(context["id"]))),
		"G7":                                  fmt.Sprint(context["applicant_nif"]),
		"H7":                                  peopleCategoryLabelFromAny(context["people_category"]),
		"I7":                                  now.Format("2006-01-02"),
		"B9":                                  serviceCommissionProjectTitle(context),
		"I9":                                  serviceCommissionProjectInternalID(context),
		fmt.Sprintf("K%d", 15+rowOffset):      fmt.Sprintf("%.2f", userTotal),
		fmt.Sprintf("E%d", summaryStartRow):   fmt.Sprintf("%.2f", userTotal+centerTotal),
		fmt.Sprintf("E%d", summaryStartRow+1): fmt.Sprintf("%.2f", researcherAdvanceTotal),
		fmt.Sprintf("E%d", summaryStartRow+2): fmt.Sprintf("%.2f", centerTotal),
		fmt.Sprintf("E%d", summaryStartRow+3): fmt.Sprintf("%.2f", userTotal-researcherAdvanceTotal),
		fmt.Sprintf("G%d", summaryStartRow+3): serviceCommissionPaymentMethod(context),
	}
	if len(centerExpenses) > 0 && serviceCommissionExpenseRowsHaveExportAmount(centerExpenses) {
		replacements[fmt.Sprintf("K%d", centerTotalRow)] = fmt.Sprintf("%.2f", centerTotal)
	}

	for index := 0; index < maxInt(1, len(userExpenses)); index++ {
		row := 13 + index
		if index < len(userExpenses) {
			addServiceCommissionExpenseCells(replacements, row, context, userExpenses[index], false)
		} else {
			clearServiceCommissionExpenseCells(replacements, row)
		}
	}

	if len(centerExpenses) == 0 {
		return replacements
	}

	for index := 0; index < len(centerExpenses); index++ {
		row := centerStartRow + index
		addServiceCommissionExpenseCells(replacements, row, context, centerExpenses[index], true)
	}

	return replacements
}

func addServiceCommissionExpenseCells(replacements map[string]string, row int, context map[string]interface{}, expense map[string]interface{}, center bool) {
	startDate := serviceCommissionDateString(firstNonEmptyString(expense["departure_date"], context["travel_start_date"]))
	endDate := serviceCommissionDateString(firstNonEmptyString(expense["return_date"], context["travel_end_date"]))
	amount := floatFromAny(expense["calculated_amount"])

	replacements[fmt.Sprintf("B%d", row)] = startDate
	replacements[fmt.Sprintf("C%d", row)] = endDate
	replacements[fmt.Sprintf("D%d", row)] = fmt.Sprint(daysBetween(startDate, endDate))
	replacements[fmt.Sprintf("E%d", row)] = formatServiceCommissionExpenseType(fmt.Sprint(expense["type"]))
	replacements[fmt.Sprintf("F%d", row)] = serviceCommissionTransportLabel(expense)
	replacements[fmt.Sprintf("G%d", row)] = serviceCommissionExpenseLocation(context, expense)
	replacements[fmt.Sprintf("H%d", row)] = serviceCommissionExpenseRelatedTask(context, expense)
	if center {
		replacements[fmt.Sprintf("J%d", row)] = serviceCommissionExpenseDescription(expense)
		return
	}

	replacements[fmt.Sprintf("I%d", row)] = serviceCommissionExpenseDescription(expense)
	if serviceCommissionExpenseHasExportAmount(expense) {
		replacements[fmt.Sprintf("K%d", row)] = fmt.Sprintf("%.2f", amount)
	} else {
		replacements[fmt.Sprintf("K%d", row)] = ""
	}
}

func clearServiceCommissionExpenseCells(replacements map[string]string, row int) {
	for _, col := range []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K"} {
		replacements[fmt.Sprintf("%s%d", col, row)] = ""
	}
}

func clearCenterExpenseSection(replacements map[string]string, rowOffset int) {
	for row := 18 + rowOffset; row <= 25+rowOffset; row++ {
		for _, col := range []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K"} {
			replacements[fmt.Sprintf("%s%d", col, row)] = ""
		}
	}
}

func commissionExpenseRowsTotal(expenses []map[string]interface{}) float64 {
	total := 0.0
	for _, expense := range expenses {
		if !serviceCommissionExpenseHasExportAmount(expense) {
			continue
		}
		total += floatFromAny(expense["calculated_amount"])
	}
	return total
}

func serviceCommissionExpenseRowsHaveExportAmount(expenses []map[string]interface{}) bool {
	for _, expense := range expenses {
		if serviceCommissionExpenseHasExportAmount(expense) {
			return true
		}
	}
	return false
}

func serviceCommissionExpenseHasExportAmount(expense map[string]interface{}) bool {
	return !isBudgetCenterExpenseType(fmt.Sprint(expense["type"]))
}

func commissionResearcherAdvanceRowsTotal(expenses []map[string]interface{}) float64 {
	total := 0.0
	for _, expense := range expenses {
		total += floatFromAny(expense["researcher_advance_amount"])
	}
	return total
}

func serviceCommissionExpenseDescription(expense map[string]interface{}) string {
	description := strings.TrimSpace(unescapeComma(firstNonEmptyString(expense["reviewed_description"], expense["description"])))
	if description == "" || description == "<nil>" {
		return ""
	}
	return description
}

func serviceCommissionProjectTitle(context map[string]interface{}) string {
	name := strings.TrimSpace(unescapeComma(fmt.Sprint(context["project_name"])))
	number := strings.TrimSpace(unescapeComma(fmt.Sprint(context["project_number"])))
	if name == "<nil>" {
		name = ""
	}
	if number == "" || number == "<nil>" {
		return name
	}
	if name == "" {
		return number
	}
	return fmt.Sprintf("%s (%s)", name, number)
}

func serviceCommissionProjectInternalID(context map[string]interface{}) string {
	projectNumber := strings.TrimSpace(unescapeComma(fmt.Sprint(context["project_number"])))
	if isContractProgramProjectID(projectNumber) {
		return contractProgramProjectID
	}
	if projectNumber != "" && projectNumber != "<nil>" {
		return projectNumber
	}

	projectID := strings.TrimSpace(unescapeComma(fmt.Sprint(context["project_id"])))
	if isContractProgramProjectID(projectID) {
		return contractProgramProjectID
	}
	if projectID == "" || projectID == "<nil>" {
		return ""
	}
	return projectID
}

func serviceCommissionPaymentMethod(context map[string]interface{}) string {
	method := strings.TrimSpace(unescapeComma(fmt.Sprint(context["payment_method"])))
	if method == "" || method == "<nil>" {
		return "Nomina"
	}
	return method
}

func serviceCommissionLocation(context map[string]interface{}) string {
	destination := strings.TrimSpace(unescapeComma(fmt.Sprint(context["destination"])))
	toWhere := strings.TrimSpace(unescapeComma(fmt.Sprint(context["travel_to_where"])))
	if destination == "<nil>" {
		destination = ""
	}
	if toWhere == "<nil>" {
		toWhere = ""
	}
	if destination == "" || strings.EqualFold(destination, toWhere) {
		return toWhere
	}
	if toWhere == "" {
		return destination
	}
	return destination + ", " + toWhere
}

func serviceCommissionExpenseLocation(context map[string]interface{}, expense map[string]interface{}) string {
	location := strings.TrimSpace(unescapeComma(fmt.Sprint(expense["location_label"])))
	if location != "" && location != "<nil>" {
		return location
	}
	return serviceCommissionLocation(context)
}

func serviceCommissionExpenseRelatedTask(context map[string]interface{}, expense map[string]interface{}) string {
	override := strings.TrimSpace(unescapeComma(fmt.Sprint(context["related_task_override"])))
	if override != "" && override != "<nil>" {
		return override
	}
	task := strings.TrimSpace(unescapeComma(fmt.Sprint(expense["related_task"])))
	if task != "" && task != "<nil>" {
		return task
	}
	return unescapeComma(fmt.Sprint(context["purpose"]))
}

func serviceCommissionDateString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Format("2006-01-02")
	}
	if parsed, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return parsed.Format("2006-01-02")
	}
	if len(value) >= len("2006-01-02") {
		candidate := value[:len("2006-01-02")]
		if _, err := time.Parse("2006-01-02", candidate); err == nil {
			return candidate
		}
	}
	return value
}

func formatServiceCommissionExpenseType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case budgetTravelExpenseType:
		return "Bitllet"
	case budgetAccommodationType:
		return "Allotjament"
	case budgetRegistrationType:
		return "Inscripció"
	case "per_diem":
		return "Dietes"
	case "transport":
		return "Trasllat"
	case "accommodation":
		return "Allotjament"
	case "mileage":
		return "Quilometratge"
	case "tax":
		return "Taxa turística"
	case "registration":
		return "Inscripció"
	case "poster":
		return "Impressió de pòster"
	case "management_fee":
		return "Despeses de gestió"
	case "internal_transport":
		return "Transport intern"
	case "bank_fee":
		return "Comissió bancària"
	case "other":
		return "Altres"
	}

	label := strings.ReplaceAll(strings.TrimSpace(value), "_", " ")
	if label == "" || label == "<nil>" {
		return ""
	}
	return strings.Title(label)
}

func serviceCommissionTransportLabel(expense map[string]interface{}) string {
	switch strings.TrimSpace(strings.ToLower(fmt.Sprint(expense["type"]))) {
	case "transport", budgetTravelExpenseType:
		method := strings.TrimSpace(unescapeComma(fmt.Sprint(expense["transport_method"])))
		if method == "" || method == "<nil>" {
			if strings.TrimSpace(strings.ToLower(fmt.Sprint(expense["type"]))) == "transport" {
				return "Transport"
			}
			return ""
		}
		return serviceCommissionTransportMethodCatalan(method)
	default:
		return ""
	}
}

func serviceCommissionTransportMethodCatalan(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "plane", "avió", "avio":
		return "Avió"
	case "train", "tren":
		return "Tren"
	case "bus", "autobús", "autobus":
		return "Autobús"
	case "taxi":
		return "Taxi"
	case "metro":
		return "Metro"
	case "public_transport", "public transport", "transport públic", "transport public":
		return "Transport públic"
	case "boat", "vaixell":
		return "Vaixell"
	default:
		return value
	}
}

func firstNonEmptyString(values ...interface{}) string {
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func expandSpreadsheetRows(sheetXML string, templateRow int, rowCount int) string {
	if rowCount <= 1 {
		return sheetXML
	}

	template := extractSpreadsheetRow(sheetXML, templateRow)
	if template == "" {
		return sheetXML
	}

	extraRows := rowCount - 1
	shifted := shiftSpreadsheetRows(sheetXML, templateRow+1, extraRows)
	copies := strings.Builder{}
	for index := 1; index <= extraRows; index++ {
		copies.WriteString(copySpreadsheetRow(template, templateRow, templateRow+index))
	}

	return strings.Replace(shifted, template, template+copies.String(), 1)
}

func ensureSpreadsheetDescriptionMerges(sheetXML string, startRow int, rowCount int) string {
	if rowCount <= 0 {
		return sheetXML
	}
	refs := make([]string, 0, rowCount)
	for row := startRow; row < startRow+rowCount; row++ {
		refs = append(refs, fmt.Sprintf("I%d:J%d", row, row))
	}
	return ensureSpreadsheetMergeCells(sheetXML, refs)
}

func ensureCenterExpenseMerges(sheetXML string, startRow int, rowCount int) string {
	if rowCount <= 0 {
		return sheetXML
	}
	refs := make([]string, 0, rowCount*2)
	for row := startRow; row < startRow+rowCount; row++ {
		refs = append(refs, fmt.Sprintf("H%d:I%d", row, row))
		refs = append(refs, fmt.Sprintf("J%d:K%d", row, row))
	}
	return ensureSpreadsheetMergeCells(sheetXML, refs)
}

func applyCenterAmountStyles(sheetXML string, startRow int, rowCount int, totalRow int) string {
	return sheetXML
}

func compactUserExpenseRowsIfNeeded(sheetXML string, startRow int, context map[string]interface{}, userExpenses []map[string]interface{}) string {
	if len(userExpenses) <= 1 {
		return sheetXML
	}

	for index, expense := range userExpenses {
		row := startRow + index
		height := serviceCommissionUserExpenseRowHeight(context, expense)
		sheetXML = setSpreadsheetRowHeight(sheetXML, row, height)
	}

	return sheetXML
}

func serviceCommissionUserExpenseRowHeight(context map[string]interface{}, expense map[string]interface{}) float64 {
	lines := maxInt(1, textLineEstimate(formatServiceCommissionExpenseType(fmt.Sprint(expense["type"])), 18))
	lines = maxInt(lines, textLineEstimate(serviceCommissionTransportLabel(expense), 18))
	lines = maxInt(lines, textLineEstimate(serviceCommissionExpenseLocation(context, expense), 26))
	lines = maxInt(lines, textLineEstimate(serviceCommissionExpenseRelatedTask(context, expense), 34))
	lines = maxInt(lines, textLineEstimate(serviceCommissionExpenseDescription(expense), 58))

	height := 20.0 + float64(lines*11)
	if height < 34 {
		return 34
	}
	if height > 75 {
		return 75
	}
	return height
}

func textLineEstimate(text string, charsPerLine int) int {
	text = strings.TrimSpace(text)
	if text == "" || text == "<nil>" {
		return 1
	}
	if charsPerLine <= 0 {
		charsPerLine = 30
	}
	lines := 0
	for _, part := range strings.Split(text, "\n") {
		length := len([]rune(strings.TrimSpace(part)))
		if length == 0 {
			lines++
			continue
		}
		lines += (length + charsPerLine - 1) / charsPerLine
	}
	return maxInt(1, lines)
}

func setSpreadsheetRowHeight(sheetXML string, row int, height float64) string {
	re := regexp.MustCompile(fmt.Sprintf(`(<row\b[^>]*\br="%d"[^>]*>)`, row))
	return re.ReplaceAllStringFunc(sheetXML, func(match string) string {
		heightAttr := fmt.Sprintf(` ht="%.2f"`, height)
		updated := match
		if strings.Contains(updated, ` ht="`) {
			updated = regexp.MustCompile(` ht="[^"]*"`).ReplaceAllString(updated, heightAttr)
		} else {
			updated = strings.Replace(updated, "<row ", "<row "+strings.TrimSpace(heightAttr)+" ", 1)
		}
		if strings.Contains(updated, ` customHeight="`) {
			updated = regexp.MustCompile(` customHeight="[^"]*"`).ReplaceAllString(updated, ` customHeight="true"`)
		} else {
			updated = strings.Replace(updated, "<row ", `<row customHeight="true" `, 1)
		}
		return updated
	})
}

func applyServiceCommissionExportStyles(sheetXML string, userStartRow int, userRowCount int, centerStartRow int, centerRowCount int, summaryStartRow int) string {
	if centerRowCount > 0 {
		for _, row := range []int{centerStartRow - 4, centerStartRow - 3} {
			for _, col := range []string{"B", "C", "D", "E", "F", "G", "H"} {
				sheetXML = setSpreadsheetCellStyle(sheetXML, fmt.Sprintf("%s%d", col, row), "0")
			}
		}
	}

	return applyServiceCommissionSummaryStyles(sheetXML, summaryStartRow)
}

func applyServiceCommissionSummaryStyles(sheetXML string, summaryStartRow int) string {
	labelStyles := []string{"43", "45", "46", "48"}
	amountStyles := []string{"44", "44", "44", "49"}
	rowHeights := []float64{20.25, 20.25, 20.25, 48.75}

	for offset := 0; offset < 4; offset++ {
		row := summaryStartRow + offset
		sheetXML = setSpreadsheetRowHeight(sheetXML, row, rowHeights[offset])
		for _, col := range []string{"B", "C", "D"} {
			sheetXML = setSpreadsheetCellStyle(sheetXML, fmt.Sprintf("%s%d", col, row), labelStyles[offset])
		}
		sheetXML = setSpreadsheetCellStyle(sheetXML, fmt.Sprintf("E%d", row), amountStyles[offset])
	}

	paymentMethodRow := summaryStartRow + 3
	sheetXML = setSpreadsheetCellStyle(sheetXML, fmt.Sprintf("F%d", paymentMethodRow), "50")
	sheetXML = setSpreadsheetCellStyle(sheetXML, fmt.Sprintf("G%d", paymentMethodRow), "51")
	return sheetXML
}

func normalizeServiceCommissionSharedStrings(sharedStringsXML string) string {
	sharedStringsXML = strings.ReplaceAll(sharedStringsXML, "Data Final                   Final Date", "Data Final\nFinal Date")
	return strings.ReplaceAll(sharedStringsXML, "Data Final&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;&#32;Final Date", "Data Final\nFinal Date")
}

func setSpreadsheetCellStyle(sheetXML string, cell string, style string) string {
	re := regexp.MustCompile(`<c r="` + regexp.QuoteMeta(cell) + `"[^>]*>`)
	return re.ReplaceAllStringFunc(sheetXML, func(match string) string {
		if strings.Contains(match, ` s="`) {
			return regexp.MustCompile(` s="[^"]*"`).ReplaceAllString(match, ` s="`+style+`"`)
		}
		return strings.Replace(match, ` r="`+cell+`"`, ` r="`+cell+`" s="`+style+`"`, 1)
	})
}

func ensureSpreadsheetMergeCells(sheetXML string, refs []string) string {
	added := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.Contains(sheetXML, `ref="`+ref+`"`) {
			continue
		}
		added = append(added, fmt.Sprintf(`<mergeCell ref="%s"/>`, ref))
	}
	if len(added) == 0 {
		return sheetXML
	}

	if strings.Contains(sheetXML, "</mergeCells>") {
		sheetXML = strings.Replace(sheetXML, "</mergeCells>", strings.Join(added, "")+"</mergeCells>", 1)
		mergeCountRe := regexp.MustCompile(`<mergeCells count="\d+"`)
		return mergeCountRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
			count := len(regexp.MustCompile(`<mergeCell ref="[^"]+"\s*/>`).FindAllString(sheetXML, -1))
			return fmt.Sprintf(`<mergeCells count="%d"`, count)
		})
	}

	insert := fmt.Sprintf(`<mergeCells count="%d">%s</mergeCells>`, len(added), strings.Join(added, ""))
	return strings.Replace(sheetXML, "<printOptions ", insert+"<printOptions ", 1)
}

func extractSpreadsheetRow(sheetXML string, row int) string {
	re := regexp.MustCompile(fmt.Sprintf(`<row[^>]* r="%d"[^>]*>.*?</row>`, row))
	return re.FindString(sheetXML)
}

func shiftSpreadsheetRows(sheetXML string, startRow int, delta int) string {
	if delta == 0 {
		return sheetXML
	}

	rowTagRe := regexp.MustCompile(`(<row[^>]* r=")(\d+)(")`)
	sheetXML = rowTagRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
		parts := rowTagRe.FindStringSubmatch(match)
		row, _ := strconv.Atoi(parts[2])
		if row < startRow {
			return match
		}
		return fmt.Sprintf(`%s%d%s`, parts[1], row+delta, parts[3])
	})

	cellRefRe := regexp.MustCompile(`\b([A-Z]{1,3})(\d+)\b`)
	return cellRefRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
		parts := cellRefRe.FindStringSubmatch(match)
		row, _ := strconv.Atoi(parts[2])
		if row < startRow {
			return match
		}
		return fmt.Sprintf("%s%d", parts[1], row+delta)
	})
}

func copySpreadsheetRow(rowXML string, sourceRow int, targetRow int) string {
	copied := regexp.MustCompile(fmt.Sprintf(`(<row[^>]* r=")%d(")`, sourceRow)).
		ReplaceAllString(rowXML, fmt.Sprintf(`${1}%d${2}`, targetRow))
	cellRefRe := regexp.MustCompile(fmt.Sprintf(`\b([A-Z]{1,3})%d\b`, sourceRow))
	return cellRefRe.ReplaceAllString(copied, fmt.Sprintf(`${1}%d`, targetRow))
}

func hideSpreadsheetRows(sheetXML string, startRow int, endRow int) string {
	rowRe := regexp.MustCompile(`<row[^>]* r="(\d+)"[^>]*>.*?</row>`)
	return rowRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
		parts := rowRe.FindStringSubmatch(match)
		row, _ := strconv.Atoi(parts[1])
		if row < startRow || row > endRow {
			return match
		}

		hidden := match
		if strings.Contains(hidden, ` hidden="`) {
			hidden = regexp.MustCompile(` hidden="[^"]*"`).ReplaceAllString(hidden, ` hidden="true"`)
		} else {
			hidden = strings.Replace(hidden, "<row ", `<row hidden="true" `, 1)
		}
		if strings.Contains(hidden, ` ht="`) {
			hidden = regexp.MustCompile(` ht="[^"]*"`).ReplaceAllString(hidden, ` ht="0"`)
		} else {
			hidden = strings.Replace(hidden, "<row ", `<row ht="0" `, 1)
		}
		if strings.Contains(hidden, ` customHeight="`) {
			hidden = regexp.MustCompile(` customHeight="[^"]*"`).ReplaceAllString(hidden, ` customHeight="true"`)
		} else {
			hidden = strings.Replace(hidden, "<row ", `<row customHeight="true" `, 1)
		}
		return hidden
	})
}

func deleteSpreadsheetRows(sheetXML string, startRow int, endRow int) string {
	if endRow < startRow {
		return sheetXML
	}

	rowRe := regexp.MustCompile(`<row[^>]* r="(\d+)"[^>]*>.*?</row>`)
	sheetXML = rowRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
		parts := rowRe.FindStringSubmatch(match)
		row, _ := strconv.Atoi(parts[1])
		if row >= startRow && row <= endRow {
			return ""
		}
		return match
	})

	sheetXML = removeSpreadsheetMergesInRows(sheetXML, startRow, endRow)
	return shiftSpreadsheetRows(sheetXML, endRow+1, -(endRow - startRow + 1))
}

func removeSpreadsheetMergesInRows(sheetXML string, startRow int, endRow int) string {
	mergeRe := regexp.MustCompile(`<mergeCell ref="([^"]+)"\s*/>`)
	sheetXML = mergeRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
		parts := mergeRe.FindStringSubmatch(match)
		if spreadsheetRangeTouchesRows(parts[1], startRow, endRow) {
			return ""
		}
		return match
	})

	mergeCountRe := regexp.MustCompile(`<mergeCells count="\d+"`)
	return mergeCountRe.ReplaceAllStringFunc(sheetXML, func(match string) string {
		count := len(mergeRe.FindAllString(sheetXML, -1))
		return fmt.Sprintf(`<mergeCells count="%d"`, count)
	})
}

func spreadsheetRangeTouchesRows(ref string, startRow int, endRow int) bool {
	rowRe := regexp.MustCompile(`[A-Z]{1,3}(\d+)`)
	matches := rowRe.FindAllStringSubmatch(ref, -1)
	for _, match := range matches {
		row, _ := strconv.Atoi(match[1])
		if row >= startRow && row <= endRow {
			return true
		}
	}
	return false
}

func restrictMainServiceCommissionSheet(sheetXML string, lastRow int) string {
	if lastRow < 1 {
		lastRow = 1
	}
	sheetXML = regexp.MustCompile(`<dimension ref="[^"]*"`).ReplaceAllString(sheetXML, fmt.Sprintf(`<dimension ref="B1:K%d"`, lastRow))
	sheetXML = regexp.MustCompile(`<col[^>]*max="257"[^>]*min="12"[^>]*/>`).ReplaceAllString(sheetXML, `<col collapsed="false" customWidth="true" hidden="true" outlineLevel="0" max="257" min="12" style="1" width="0"/>`)
	return sheetXML
}

func hideAuxiliaryWorkbookSheets(workbookXML string) string {
	re := regexp.MustCompile(`<sheet name="Desplegable"([^>]*)state="[^"]*"([^>]*)/>`)
	if re.MatchString(workbookXML) {
		return re.ReplaceAllString(workbookXML, `<sheet name="Desplegable"${1}state="hidden"${2}/>`)
	}

	return strings.Replace(workbookXML, `<sheet name="Desplegable"`, `<sheet name="Desplegable" state="hidden"`, 1)
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func splitCommissionTotals(expenses []map[string]interface{}) (float64, float64) {
	var userTotal float64
	var centerTotal float64
	for _, expense := range expenses {
		amount := floatFromAny(expense["calculated_amount"])
		if fmt.Sprint(expense["purpose"]) == centerExpensePurpose {
			centerTotal += amount
		} else {
			userTotal += amount
		}
	}
	return userTotal, centerTotal
}

func expenseSummary(expenses []map[string]interface{}, center bool) serviceCommissionExpenseSummary {
	kinds := make([]string, 0)
	descriptions := make([]string, 0)
	transport := ""
	for _, expense := range expenses {
		isCenter := fmt.Sprint(expense["purpose"]) == centerExpensePurpose
		if isCenter != center {
			continue
		}
		kind := strings.TrimSpace(fmt.Sprint(expense["type"]))
		if kind != "" {
			kinds = append(kinds, kind)
		}
		if kind == "transport" || kind == "mileage" {
			transport = kind
		}
		description := strings.TrimSpace(unescapeComma(fmt.Sprint(expense["description"])))
		if description != "" && description != "<nil>" {
			descriptions = append(descriptions, description)
		}
	}
	return serviceCommissionExpenseSummary{
		kind:        strings.Join(uniqueStrings(kinds), ", "),
		transport:   transport,
		description: strings.Join(descriptions, " | "),
	}
}

func replaceSpreadsheetCells(sheetXML string, values map[string]string) string {
	for cell, value := range values {
		escaped := xmlEscape(value)
		re := regexp.MustCompile(`<c r="` + regexp.QuoteMeta(cell) + `"[^>]*/>|<c r="` + regexp.QuoteMeta(cell) + `"[^>]*>(?s:.*?)</c>`)
		sheetXML = re.ReplaceAllStringFunc(sheetXML, func(match string) string {
			style := ""
			if styleMatch := regexp.MustCompile(` s="[^"]*"`).FindString(match); styleMatch != "" {
				style = styleMatch
			}
			return fmt.Sprintf(`<c r="%s"%s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, cell, style, escaped)
		})
	}
	return sheetXML
}

func convertSpreadsheetToPDF(xlsxPath string, outputDir string) error {
	cmd := exec.Command("libreoffice", "-env:UserInstallation=file:///tmp/libreoffice-module10-runtime", "--headless", "--convert-to", "pdf", "--outdir", outputDir, xlsxPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("libreoffice conversion failed: %w - %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func saveServiceCommissionDocument(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, draftPath string, signedPath string, draftHash string, signedHash string, status string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	existing, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_documents",
		"columns":   "id",
		"condition": fmt.Sprintf("service_id = %d", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return updateRowColumns(apiKey, client, w, "service_commission_documents",
			"draft_pdf_path, signed_pdf_path, document_hash, signed_document_hash, status, updated_at",
			fmt.Sprintf("%s, %s, %s, %s, %s, %s", escapeComma(draftPath), escapeComma(signedPath), draftHash, signedHash, status, now),
			fmt.Sprintf("service_id = %d", serviceID))
	}
	_, err = insertRowAndReturnID(apiKey, client, w, "service_commission_documents",
		"service_id, draft_pdf_path, signed_pdf_path, document_hash, signed_document_hash, status, created_at, updated_at",
		fmt.Sprintf("%d, %s, %s, %s, %s, %s, %s, %s", serviceID, escapeComma(draftPath), escapeComma(signedPath), draftHash, signedHash, status, now, now))
	return err
}

func finalizeServiceCommissionPDF(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, actorID int, actorRole string) error {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_documents",
		"columns":   "draft_pdf_path, document_hash",
		"condition": fmt.Sprintf("service_id = %d ORDER BY id DESC LIMIT 1", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("draft document not found")
	}

	draftPath := strings.TrimSpace(fmt.Sprint(rows[0]["draft_pdf_path"]))
	unsignedBytes, err := os.ReadFile(filepath.Join("../../", draftPath))
	if err != nil {
		return err
	}
	if hash := sha256Hex(unsignedBytes); hash != fmt.Sprint(rows[0]["document_hash"]) {
		return fmt.Errorf("draft document hash mismatch")
	}

	if ok, err := verifyServiceCommissionAuditChain(apiKey, client, w, serviceID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("audit chain integrity check failed")
	}
	auditHash, err := getPreviousServiceCommissionAuditHash(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}

	signingInputs := serviceCommissionSigningInputs(draftPath, serviceID)
	signedRel, signedBytes, err := signServiceCommissionDocuments(apiKey, client, w, signingInputs, serviceID, serviceCommissionIntegrityHashes{
		DraftDocumentHash: fmt.Sprint(rows[0]["document_hash"]),
		AuditChainHash:    auditHash,
	})
	if err != nil {
		return err
	}

	signedHash := sha256Hex(signedBytes)
	if err := saveServiceCommissionDocument(apiKey, client, w, serviceID, draftPath, signedRel, fmt.Sprint(rows[0]["document_hash"]), signedHash, "signed"); err != nil {
		return err
	}
	if err := auditServiceCommission(apiKey, client, w, serviceID, actorID, actorRole, "final_pdf_signed", map[string]interface{}{
		"message":              "All required approvals completed. Signed final service commission PDF generated.",
		"signed_document_hash": signedHash,
	}); err != nil {
		return err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return updateRowColumns(apiKey, client, w, "service_commisions", "status, updated_at", fmt.Sprintf("%s, %s", commissionStatusApproved, now), fmt.Sprintf("id = %d", serviceID))
}

func auditServiceCommission(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, actorID int, actorRole string, action string, details map[string]interface{}) error {
	now := time.Now().Format(time.RFC3339)
	payload := map[string]interface{}{
		"service_id":      serviceID,
		"timestamp":       now,
		"action":          action,
		"actor_people_id": actorID,
		"actor_role":      actorRole,
		"details":         details,
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	encrypted := encryptLogWithGPG(string(rawPayload))
	if encrypted == "" {
		return fmt.Errorf("could not encrypt service commission audit payload")
	}
	encoded := base64_encode(encrypted)
	previousHash, err := getPreviousServiceCommissionAuditHash(apiKey, client, w, serviceID)
	if err != nil {
		return err
	}
	currentHash := sha256String(previousHash + "|" + encoded)

	_, err = insertRowAndReturnID(apiKey, client, w, "service_commission_audit_logs",
		"service_id, timestamp, action, actor_people_id, actor_role, payload, hash, previous_hash",
		fmt.Sprintf("%d, %s, %s, %d, %s, %s, %s, %s", serviceID, now, action, actorID, actorRole, encoded, currentHash, previousHash))
	return err
}

func getPreviousServiceCommissionAuditHash(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) (string, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_audit_logs",
		"columns":   "hash",
		"condition": fmt.Sprintf("service_id = %d ORDER BY id DESC LIMIT 1", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "GENESIS", nil
	}
	return fmt.Sprint(rows[0]["hash"]), nil
}

func verifyServiceCommissionAuditChain(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) (bool, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_audit_logs",
		"columns":   "payload, hash, previous_hash",
		"condition": fmt.Sprintf("service_id = %d ORDER BY id ASC", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return false, err
	}
	previous := "GENESIS"
	for _, row := range rows {
		if fmt.Sprint(row["previous_hash"]) != previous {
			return false, nil
		}
		expected := sha256String(previous + "|" + fmt.Sprint(row["payload"]))
		if expected != fmt.Sprint(row["hash"]) {
			return false, nil
		}
		previous = expected
	}
	return len(rows) > 0, nil
}

func signServiceCommissionPDF(pdfBytes []byte) ([]byte, error) {
	if strings.TrimSpace(mC.PDFSignerCommand) == "" || strings.TrimSpace(mC.PDFCertificatePath) == "" || strings.TrimSpace(mC.PDFCertificatePassword) == "" {
		return nil, fmt.Errorf("PDF signing is not configured")
	}

	signer, err := resolveModule10ExecutablePath(mC.PDFSignerCommand)
	if err != nil {
		return nil, err
	}
	signer, cleanupSigner, err := serviceCommissionNonStrictSignerScript(signer)
	if err != nil {
		return nil, err
	}
	defer cleanupSigner()
	certificate, err := resolveModule10ReadablePath(mC.PDFCertificatePath)
	if err != nil {
		return nil, err
	}

	unsigned, err := os.CreateTemp("", "service-commission-unsigned-*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(unsigned.Name())
	if _, err := unsigned.Write(pdfBytes); err != nil {
		unsigned.Close()
		return nil, err
	}
	if err := unsigned.Close(); err != nil {
		return nil, err
	}
	signerInputPath, cleanupSignerInput, err := normalizeServiceCommissionPDFForSigning(unsigned.Name())
	if err != nil {
		return nil, err
	}
	defer cleanupSignerInput()

	signed, err := os.CreateTemp("", "service-commission-signed-*.pdf")
	if err != nil {
		return nil, err
	}
	signedPath := signed.Name()
	_ = signed.Close()
	defer os.Remove(signedPath)

	args := []string{
		"--input", signerInputPath,
		"--output", signedPath,
		"--certificate", certificate,
		"--visible",
		"--page", "1",
		"--x", "455",
		"--y", "30",
		"--width", "180",
		"--height", "70",
		"--reason", firstNonEmptyModule10(mC.PDFSignatureReason, "Segell electrònic intern de comissió de servei"),
		"--location", firstNonEmptyModule10(mC.PDFSignatureLocation, "CRMIntratools"),
	}
	cmd := exec.Command(signer, args...)
	cmd.Stdin = strings.NewReader(mC.PDFCertificatePassword + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error signing PDF: %w - %s", err, sanitizeModule10SignerOutput(string(output)))
	}
	return os.ReadFile(signedPath)
}

func serviceCommissionNonStrictSignerScript(signer string) (string, func(), error) {
	raw, err := os.ReadFile(signer)
	if err != nil {
		return "", func() {}, err
	}

	content := string(raw)
	if !strings.Contains(content, "--no-strict-syntax") {
		const styleArg = "  --style-name crm-logo \\\n"
		if !strings.Contains(content, styleArg) {
			return "", func() {}, fmt.Errorf("PDF signer does not support non-strict syntax signing: style argument not found")
		}
		content = strings.Replace(content, styleArg, styleArg+"  --no-strict-syntax \\\n", 1)
	}

	temp, err := os.CreateTemp("", "service-commission-nonstrict-signer-*.sh")
	if err != nil {
		return "", func() {}, err
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = os.Remove(tempPath)
	}
	if _, err := temp.WriteString(content); err != nil {
		temp.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := os.Chmod(tempPath, 0700); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return tempPath, cleanup, nil
}

func normalizeServiceCommissionPDFForSigning(inputPath string) (string, func(), error) {
	if _, err := exec.LookPath("mutool"); err != nil {
		return inputPath, func() {}, nil
	}

	cleaned, err := os.CreateTemp("", "service-commission-normalized-*.pdf")
	if err != nil {
		return "", func() {}, err
	}
	cleanedPath := cleaned.Name()
	_ = cleaned.Close()
	cleanup := func() {
		_ = os.Remove(cleanedPath)
	}

	output, err := exec.Command("mutool", "clean", inputPath, cleanedPath).CombinedOutput()
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("normalizing PDF before signing failed: %w - %s", err, strings.TrimSpace(string(output)))
	}
	return cleanedPath, cleanup, nil
}

type serviceCommissionSigningInput struct {
	Key        string
	SourceRel  string
	SourceFull string
	SignedRel  string
	SignedFull string
}

func serviceCommissionSigningInputs(draftPath string, serviceID int) []serviceCommissionSigningInput {
	dirRel := filepath.ToSlash(filepath.Dir(draftPath))
	dirFull := filepath.Join("../../", dirRel)
	groupInputs := make([]serviceCommissionSigningInput, 0, 2)
	for _, key := range []string{"nomina", "transferencia"} {
		sourceRel := filepath.ToSlash(filepath.Join(dirRel, fmt.Sprintf("service-commission-%d-%s.pdf", serviceID, key)))
		sourceFull := filepath.Join(dirFull, filepath.Base(sourceRel))
		if _, err := os.Stat(sourceFull); err != nil {
			continue
		}
		signedRel := filepath.ToSlash(filepath.Join(dirRel, fmt.Sprintf("service-commission-%d-signed-%s.pdf", serviceID, key)))
		groupInputs = append(groupInputs, serviceCommissionSigningInput{
			Key:        key,
			SourceRel:  sourceRel,
			SourceFull: sourceFull,
			SignedRel:  signedRel,
			SignedFull: filepath.Join("../../", signedRel),
		})
	}
	if len(groupInputs) > 1 {
		return groupInputs
	}

	signedRel := filepath.ToSlash(filepath.Join(dirRel, fmt.Sprintf("service-commission-%d-signed.pdf", serviceID)))
	return []serviceCommissionSigningInput{{
		Key:        "document",
		SourceRel:  draftPath,
		SourceFull: filepath.Join("../../", draftPath),
		SignedRel:  signedRel,
		SignedFull: filepath.Join("../../", signedRel),
	}}
}

func signServiceCommissionDocuments(apiKey string, client *http.Client, w http.ResponseWriter, inputs []serviceCommissionSigningInput, serviceID int, hashes serviceCommissionIntegrityHashes) (string, []byte, error) {
	if len(inputs) == 0 {
		return "", nil, fmt.Errorf("no service commission PDFs available for signing")
	}

	auditPage, err := generateServiceCommissionIntegrityPage(apiKey, client, w, serviceID, hashes)
	if err != nil {
		return "", nil, err
	}
	defer os.Remove(auditPage)

	signedFiles := make([]serviceCommissionSigningInput, 0, len(inputs))
	for _, input := range inputs {
		unsignedFull, cleanup, err := appendServiceCommissionIntegrityPage(input.SourceFull, auditPage, serviceID, input.Key)
		if err != nil {
			return "", nil, err
		}
		defer cleanup()

		unsignedBytes, err := os.ReadFile(unsignedFull)
		if err != nil {
			return "", nil, err
		}
		signedBytes, err := signServiceCommissionPDF(unsignedBytes)
		if err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(input.SignedFull, signedBytes, 0600); err != nil {
			return "", nil, err
		}
		signedFiles = append(signedFiles, input)
	}

	if len(signedFiles) == 1 {
		signedBytes, err := os.ReadFile(signedFiles[0].SignedFull)
		return signedFiles[0].SignedRel, signedBytes, err
	}

	dirRel := filepath.ToSlash(filepath.Dir(signedFiles[0].SignedRel))
	zipRel := filepath.ToSlash(filepath.Join(dirRel, fmt.Sprintf("service-commission-%d-signed.zip", serviceID)))
	zipFull := filepath.Join("../../", zipRel)
	if err := createServiceCommissionSignedZip(zipFull, signedFiles); err != nil {
		return "", nil, err
	}
	zipBytes, err := os.ReadFile(zipFull)
	return zipRel, zipBytes, err
}

func createServiceCommissionSignedZip(zipFull string, files []serviceCommissionSigningInput) error {
	zipFile, err := os.OpenFile(zipFull, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	seen := make(map[string]int)
	for _, fileInfo := range files {
		filename := uniqueZipFilename(filepath.Base(fileInfo.SignedRel), seen)
		fileWriter, err := zipWriter.Create(filename)
		if err != nil {
			return err
		}
		file, err := os.Open(fileInfo.SignedFull)
		if err != nil {
			return err
		}
		if _, err := io.Copy(fileWriter, file); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

type serviceCommissionIntegrityActor struct {
	Role       string
	Name       string
	DocumentID string
	ApprovedAt string
}

type serviceCommissionIntegrityHashes struct {
	DraftDocumentHash string
	AuditChainHash    string
}

func generateServiceCommissionIntegrityPage(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int, hashes serviceCommissionIntegrityHashes) (string, error) {
	actors, err := serviceCommissionIntegrityActors(apiKey, client, w, serviceID)
	if err != nil {
		return "", err
	}
	contextRows, err := getCommissionContext(apiKey, client, w, serviceID)
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("SC-%d", serviceID)
	if len(contextRows) > 0 {
		code = firstNonEmptyString(fmt.Sprint(contextRows[0]["code"]), code)
	}

	pdfFile, err := os.CreateTemp("", "service-commission-integrity-*.pdf")
	if err != nil {
		return "", err
	}
	pdfPath := pdfFile.Name()
	if err := pdfFile.Close(); err != nil {
		return "", err
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetTitle("Informació d'integritat del document", false)
	pdf.SetAuthor("CRMIntratools", false)
	pdf.SetCreator("CRMIntratools - Mòdul de comissions de servei", false)
	pdf.SetMargins(18, 15, 18)
	pdf.SetAutoPageBreak(true, 15)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.AddPage()
	pdf.SetFont("Arial", "B", 18)
	pdf.MultiCell(0, 8, tr("Informació d'integritat del document"), "", "L", false)
	pdf.Ln(3)

	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(0, 6, tr(fmt.Sprintf("Aquesta pàgina forma part de la comissió de servei %s i resumeix el recorregut de validació del document abans de la seva signatura electrònica.", code)), "", "L", false)
	pdf.Ln(4)

	pdf.SetFont("Arial", "B", 13)
	pdf.MultiCell(0, 6, tr("Persones que han intervingut en la validació"), "", "L", false)
	pdf.Ln(1)
	writeServiceCommissionIntegrityActorsTable(pdf, tr, actors)

	pdf.Ln(6)
	pdf.SetFont("Arial", "B", 13)
	pdf.MultiCell(0, 6, tr("Verificació d'integritat"), "", "L", false)
	pdf.SetFont("Arial", "", 10)
	for _, paragraph := range serviceCommissionIntegrityParagraphs() {
		pdf.MultiCell(0, 5.2, tr(paragraph), "", "L", false)
		pdf.Ln(1.5)
	}
	writeServiceCommissionIntegrityHashes(pdf, tr, hashes)
	pdf.Ln(2)
	pdf.SetFont("Arial", "I", 8.5)
	pdf.MultiCell(0, 4.5, tr(fmt.Sprintf("Pàgina generada automàticament el %s.", time.Now().Format("02/01/2006 15:04"))), "", "L", false)

	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return "", err
	}
	return pdfPath, nil
}

func writeServiceCommissionIntegrityActorsTable(pdf *fpdf.Fpdf, tr func(string) string, actors []serviceCommissionIntegrityActor) {
	widths := []float64{70, 85, 45, 55}
	headers := []string{"Rol", "Persona", "DNI/NIF", "Data"}
	pdf.SetFont("Arial", "B", 9.5)
	pdf.SetFillColor(238, 238, 238)
	for index, header := range headers {
		pdf.CellFormat(widths[index], 7, tr(header), "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for _, actor := range actors {
		values := []string{actor.Role, actor.Name, actor.DocumentID, actor.ApprovedAt}
		rowHeight := serviceCommissionIntegrityRowHeight(pdf, tr, values, widths)
		x := pdf.GetX()
		y := pdf.GetY()
		for index, value := range values {
			pdf.Rect(x, y, widths[index], rowHeight, "")
			pdf.SetXY(x+1.5, y+1.5)
			pdf.MultiCell(widths[index]-3, 4.2, tr(value), "", "L", false)
			x += widths[index]
			pdf.SetXY(x, y)
		}
		pdf.SetXY(18, y+rowHeight)
	}
}

func serviceCommissionIntegrityRowHeight(pdf *fpdf.Fpdf, tr func(string) string, values []string, widths []float64) float64 {
	maxLines := 1
	for index, value := range values {
		lines := pdf.SplitLines([]byte(tr(value)), widths[index]-3)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	height := float64(maxLines)*4.2 + 3
	if height < 7 {
		return 7
	}
	return height
}

func serviceCommissionIntegrityParagraphs() []string {
	return []string{
		"El sistema conserva una traça d'auditoria associada a aquesta comissió de servei. Cada actuació registrada es transforma en un payload estructurat, es protegeix criptogràficament i queda vinculada a l'actuació anterior mitjançant una cadena de hashes SHA-256.",
		"Abans de generar el document final signat, l'aplicació d'eines internes del centre verifica que la cadena d'auditoria sigui coherent: comprova que cada hash anterior coincideixi amb el registre precedent i recalcula el hash de cada entrada. Si algun registre s'hagués modificat, eliminat o reordenat, aquesta verificació fallaria i el PDF signat no es generaria.",
		"El document resultant es signa amb el segell electrònic intern del Centre de Recerca Matemàtica. Qualsevol canvi posterior al PDF signat invalidaria la signatura digital.",
	}
}

func writeServiceCommissionIntegrityHashes(pdf *fpdf.Fpdf, tr func(string) string, hashes serviceCommissionIntegrityHashes) {
	pdf.SetFont("Arial", "B", 8)
	pdf.MultiCell(0, 4, tr("Empremtes d'integritat"), "", "L", false)
	pdf.SetFont("Arial", "", 7.2)
	if strings.TrimSpace(hashes.DraftDocumentHash) != "" {
		pdf.MultiCell(0, 3.8, tr("Hash SHA-256 del document base: "+hashes.DraftDocumentHash), "", "L", false)
	}
	if strings.TrimSpace(hashes.AuditChainHash) != "" {
		pdf.MultiCell(0, 3.8, tr("Hash final de la cadena d'auditoria abans de la signatura: "+hashes.AuditChainHash), "", "L", false)
	}
	pdf.MultiCell(0, 3.8, tr("El hash del document signat es desa al registre intern un cop completada la signatura, ja que no es pot incloure dins del mateix PDF sense alterar-lo."), "", "L", false)
}

func appendServiceCommissionIntegrityPage(sourceFull string, auditPageFull string, serviceID int, key string) (string, func(), error) {
	outputFull := filepath.Join(os.TempDir(), fmt.Sprintf("service-commission-%d-%s-with-integrity-%d.pdf", serviceID, sanitizeDownloadFilename(key), time.Now().UnixNano()))
	if err := mergePDFs([]string{sourceFull, auditPageFull}, outputFull); err != nil {
		return "", func() {}, err
	}
	return outputFull, func() { _ = os.Remove(outputFull) }, nil
}

func serviceCommissionIntegrityActors(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) ([]serviceCommissionIntegrityActor, error) {
	applicantRows, err := runDBGet(map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN people applicant ON sc.people_id = applicant.id`,
		"columns": `TRIM(COALESCE(applicant.name, '') || ' ' || COALESCE(applicant.surname, '') || ' ' || COALESCE(applicant.secondSurname, '')) AS applicant_name,
COALESCE(applicant.nif, applicant.nif_extended, '') AS applicant_nif,
sc.creation_date AS initial_requested_at`,
		"condition": fmt.Sprintf("sc.id = %d", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	projectRows, err := runDBGet(map[string]interface{}{
		"table": "service_commission_documents d",
		"columns": `d.created_at AS projects_approved_at,
(SELECT TRIM(COALESCE(pp.name, '') || ' ' || COALESCE(pp.surname, '') || ' ' || COALESCE(pp.secondSurname, '')) FROM users pu LEFT JOIN people pp ON pu.people_id = pp.id WHERE pu.access_projects = 1 AND pp.id IS NOT NULL ORDER BY pu.people_id LIMIT 1) AS project_person_name,
(SELECT COALESCE(pp.nif, pp.nif_extended, '') FROM users pu LEFT JOIN people pp ON pu.people_id = pp.id WHERE pu.access_projects = 1 AND pp.id IS NOT NULL ORDER BY pu.people_id LIMIT 1) AS project_person_nif`,
		"condition": fmt.Sprintf("d.service_id = %d ORDER BY d.created_at ASC LIMIT 1", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	approvalRows, err := runDBGet(map[string]interface{}{
		"table": `service_commission_approvals a
LEFT JOIN people approver ON a.approver_people_id = approver.id`,
		"columns": `a.approver_type,
a.approved_at,
TRIM(COALESCE(approver.name, '') || ' ' || COALESCE(approver.surname, '') || ' ' || COALESCE(approver.secondSurname, '')) AS approver_name,
COALESCE(approver.nif, approver.nif_extended, '') AS approver_nif`,
		"condition": fmt.Sprintf("a.service_id = %d AND COALESCE(a.approved_at, '') <> '' ORDER BY CASE a.approver_type WHEN 'applicant' THEN 1 WHEN 'project_responsible' THEN 2 WHEN 'management' THEN 3 ELSE 4 END, a.id", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	actors := make([]serviceCommissionIntegrityActor, 0, len(approvalRows)+2)
	if len(applicantRows) > 0 {
		actors = append(actors, serviceCommissionIntegrityActor{
			Role:       "Persona sol·licitant",
			Name:       firstNonEmptyString(fmt.Sprint(applicantRows[0]["applicant_name"]), "-"),
			DocumentID: firstNonEmptyString(fmt.Sprint(applicantRows[0]["applicant_nif"]), "-"),
			ApprovedAt: formatServiceCommissionIntegrityDate(fmt.Sprint(applicantRows[0]["initial_requested_at"])),
		})
	}
	if len(projectRows) > 0 {
		actors = append(actors, serviceCommissionIntegrityActor{
			Role:       "Aprovació de projectes",
			Name:       firstNonEmptyString(fmt.Sprint(projectRows[0]["project_person_name"]), "Equip de projectes"),
			DocumentID: firstNonEmptyString(fmt.Sprint(projectRows[0]["project_person_nif"]), "-"),
			ApprovedAt: formatServiceCommissionIntegrityDate(fmt.Sprint(projectRows[0]["projects_approved_at"])),
		})
	}
	for _, row := range approvalRows {
		name := strings.TrimSpace(fmt.Sprint(row["approver_name"]))
		if name == "" || name == "<nil>" {
			continue
		}
		actors = append(actors, serviceCommissionIntegrityActor{
			Role:       serviceCommissionIntegrityRoleLabel(fmt.Sprint(row["approver_type"])),
			Name:       name,
			DocumentID: firstNonEmptyString(fmt.Sprint(row["approver_nif"]), "-"),
			ApprovedAt: formatServiceCommissionIntegrityDate(fmt.Sprint(row["approved_at"])),
		})
	}
	if len(actors) == 0 {
		actors = append(actors, serviceCommissionIntegrityActor{
			Role:       "Document",
			Name:       "-",
			DocumentID: "-",
			ApprovedAt: "-",
		})
	}
	return actors, nil
}

func serviceCommissionIntegrityRoleLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "applicant":
		return "Confirmació de la persona sol·licitant"
	case "project_responsible":
		return "Validació de la persona responsable del projecte"
	case "management":
		return "Validació de gerència"
	default:
		return firstNonEmptyString(value, "Validació")
	}
}

func formatServiceCommissionIntegrityDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return "-"
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05Z"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("02/01/2006 15:04")
		}
	}
	return value
}

func handleAddCenterExpense(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		CommissionID int     `json:"commission_id"`
		Type         string  `json:"type"`
		Amount       float64 `json:"amount"`
		Description  string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.CommissionID == 0 || payload.Type == "" {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	expenseID, err := insertRowAndReturnID(apiKey, client, w, "service_commission_expenses",
		"service_id, type, amount, description, purpose",
		fmt.Sprintf("%d, %s, %f, %s, %s", payload.CommissionID, escapeComma(payload.Type), payload.Amount, escapeComma(payload.Description), centerExpensePurpose))

	if err != nil || expenseID == 0 {
		http.Error(w, "Could not add center expense", http.StatusInternalServerError)
		return
	}

	if err := setCommissionUnderReview(apiKey, client, w, payload.CommissionID); err != nil {
		createLog(fmt.Sprintf("Error setting commission %d under review after center expense: %v", payload.CommissionID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":     expenseID,
		"status": commissionStatusUnderReview,
	})
}

func handleDeleteCenterExpense(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	handleExcludeExpense(apiKey, client, w, r)
}

func handleExcludeExpense(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		ExpenseID int `json:"expense_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ExpenseID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "service_id, purpose",
		"condition": fmt.Sprintf("id = %d AND COALESCE(excluded, 0) = 0", payload.ExpenseID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		http.Error(w, "Expense not found", http.StatusNotFound)
		return
	}

	serviceID := intFromAny(rows[0]["service_id"])
	activeCount, err := activeServiceCommissionExpenseCount(apiKey, client, w, serviceID)
	if err != nil {
		createLog(fmt.Sprintf("Error counting active expenses for commission %d: %v", serviceID, err), 1, apiKey, client, w)
		http.Error(w, "Could not verify commission expenses", http.StatusInternalServerError)
		return
	}
	if activeCount <= 1 {
		http.Error(w, "Cannot delete the only expense in the commission. Reject the commission instead.", http.StatusBadRequest)
		return
	}

	if err := updateRowColumns(apiKey, client, w, "service_commission_expenses",
		"excluded",
		"1",
		fmt.Sprintf("id = %d", payload.ExpenseID)); err != nil {
		createLog(fmt.Sprintf("Error excluding expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
		http.Error(w, "Could not delete expense", http.StatusInternalServerError)
		return
	}

	if serviceID != 0 {
		if err := setCommissionUnderReview(apiKey, client, w, serviceID); err != nil {
			createLog(fmt.Sprintf("Error setting commission %d under review after excluding expense: %v", serviceID, err), 1, apiKey, client, w)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"expense_id": payload.ExpenseID,
		"service_id": serviceID,
		"status":     commissionStatusUnderReview,
	})
}

func activeServiceCommissionExpenseCount(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) (int, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "COUNT(*) AS total",
		"condition": fmt.Sprintf("service_id = %d AND COALESCE(excluded, 0) = 0", serviceID),
	}, apiKey, client, w)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return intFromAny(rows[0]["total"]), nil
}

func deleteRows(apiKey string, client *http.Client, w http.ResponseWriter, table string, condition string) error {
	payload := map[string]interface{}{
		"table":     table,
		"condition": condition,
	}
	jsonData, _ := json.Marshal(payload)
	resp := deleteReq(jsonData, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("delete failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func handleGetPricingTables(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanManageRates(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tables, err := getPricingTables(apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Error loading pricing tables: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not load pricing tables", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"tables": tables})
}

func handleSavePricingTable(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if !requestCanManageRates(apiKey, client, w, r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload pricingTablePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	payload.Title = strings.TrimSpace(payload.Title)
	payload.Category = strings.TrimSpace(payload.Category)
	payload.Type = strings.TrimSpace(payload.Type)

	if payload.Title == "" || payload.Category == "" || !isValidPricingTableType(payload.Type) {
		http.Error(w, "Title, category and valid table type are required", http.StatusBadRequest)
		return
	}

	tableID, err := savePricingTable(apiKey, client, w, payload)
	if err != nil {
		createLog(fmt.Sprintf("Error saving pricing table: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not save pricing table", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": tableID})
}

func handleDeletePricingTable(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	if !requestCanManageRates(apiKey, client, w, r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := deletePricingTable(apiKey, client, w, payload.ID); err != nil {
		createLog(fmt.Sprintf("Error deleting pricing table %d: %v", payload.ID, err), 1, apiKey, client, w)
		http.Error(w, "Could not delete pricing table", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"deleted": payload.ID})
}

func requestCanManageRates(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) bool {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	return userCanManageRates(role, accessProjects, accessGerencia)
}

func isValidPricingTableType(tableType string) bool {
	return tableType == "accommodation" || tableType == "per_diem"
}

func getPricingTables(apiKey string, client *http.Client, w http.ResponseWriter) ([]map[string]interface{}, error) {
	tableQuery := map[string]interface{}{
		"table":     "service_commission_pricing_tables",
		"columns":   "id, title, category, table_type, active, created_at, updated_at",
		"condition": "(active IS NULL OR active = 1 OR active = '1' OR LOWER(active) = 'true') ORDER BY table_type ASC, category ASC, title ASC",
	}

	tableResp, err := runDBGet(tableQuery, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	rowQuery := map[string]interface{}{
		"table":     "service_commission_pricing_rows",
		"columns":   "id, table_id, territory, group_code, meal_type, amount, sort_order, active",
		"condition": "(active IS NULL OR active = 1 OR active = '1' OR LOWER(active) = 'true') AND COALESCE(meal_type, '') <> 'HALF_BOARD' ORDER BY table_id ASC, sort_order ASC, territory ASC, group_code ASC, meal_type ASC",
	}

	rowResp, err := runDBGet(rowQuery, apiKey, client, w)
	if err != nil {
		return nil, err
	}

	rowsByTable := make(map[int][]map[string]interface{})
	for _, row := range rowResp {
		tableID := intFromAny(row["table_id"])
		if tableID == 0 {
			continue
		}
		row["territory"] = unescapeComma(fmt.Sprint(row["territory"]))
		rowsByTable[tableID] = append(rowsByTable[tableID], row)
	}

	for index := range tableResp {
		tableID := intFromAny(tableResp[index]["id"])
		tableResp[index]["title"] = unescapeComma(fmt.Sprint(tableResp[index]["title"]))
		tableResp[index]["category"] = unescapeComma(fmt.Sprint(tableResp[index]["category"]))
		tableResp[index]["rows"] = rowsByTable[tableID]
	}

	return tableResp, nil
}

func savePricingTable(apiKey string, client *http.Client, w http.ResponseWriter, payload pricingTablePayload) (int, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	tableID := payload.ID

	if tableID == 0 {
		insertedID, err := insertRowAndReturnID(apiKey, client, w, "service_commission_pricing_tables",
			"title, category, table_type, active, created_at, updated_at",
			fmt.Sprintf("%s, %s, %s, 1, %s, %s", escapeComma(payload.Title), escapeComma(payload.Category), payload.Type, now, now))
		if err != nil {
			return 0, err
		}
		tableID = insertedID
	} else {
		if err := updateRowColumns(apiKey, client, w, "service_commission_pricing_tables",
			"title, category, table_type, updated_at",
			fmt.Sprintf("%s, %s, %s, %s", escapeComma(payload.Title), escapeComma(payload.Category), payload.Type, now),
			fmt.Sprintf("id = %d", tableID)); err != nil {
			return 0, err
		}

		if err := deletePricingRows(apiKey, client, w, tableID); err != nil {
			return 0, err
		}
	}

	for index, row := range payload.Rows {
		territory := strings.TrimSpace(row.Territory)
		groupCode := strings.ToUpper(strings.TrimSpace(row.GroupCode))
		mealType := strings.ToUpper(strings.TrimSpace(row.MealType))

		if territory == "" || !isValidGroupCode(groupCode) || row.Amount < 0 {
			continue
		}
		if payload.Type == "accommodation" {
			mealType = ""
		} else if !isValidMealType(mealType) {
			continue
		}

		sortOrder := row.SortOrder
		if sortOrder == 0 {
			sortOrder = index + 1
		}

		_, err := insertRowAndReturnID(apiKey, client, w, "service_commission_pricing_rows",
			"table_id, territory, group_code, meal_type, amount, sort_order, active",
			fmt.Sprintf("%d, %s, %s, %s, %.2f, %d, 1", tableID, escapeComma(territory), groupCode, mealType, row.Amount, sortOrder))
		if err != nil {
			return 0, err
		}
	}

	return tableID, nil
}

func deletePricingTable(apiKey string, client *http.Client, w http.ResponseWriter, tableID int) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := updateRowColumns(apiKey, client, w, "service_commission_pricing_tables", "active, updated_at", fmt.Sprintf("0, %s", now), fmt.Sprintf("id = %d", tableID)); err != nil {
		return err
	}
	return updateRowColumns(apiKey, client, w, "service_commission_pricing_rows", "active", "0", fmt.Sprintf("table_id = %d", tableID))
}

func deletePricingRows(apiKey string, client *http.Client, w http.ResponseWriter, tableID int) error {
	return updateRowColumns(apiKey, client, w, "service_commission_pricing_rows", "active", "0", fmt.Sprintf("table_id = %d", tableID))
}

func handleSaveExpensePricingReview(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload expensePricingReviewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ExpenseID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	groupCode := strings.ToUpper(strings.TrimSpace(payload.GroupCode))
	territory := strings.TrimSpace(payload.Territory)
	if !isValidGroupCode(groupCode) || territory == "" || payload.PricingTableID == 0 {
		http.Error(w, "Invalid pricing review", http.StatusBadRequest)
		return
	}

	if err := saveExpensePricingReview(apiKey, client, w, payload, groupCode, territory); err != nil {
		createLog(fmt.Sprintf("Error saving expense pricing review for expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
		http.Error(w, "Could not save pricing review", http.StatusInternalServerError)
		return
	}

	serviceID, err := getServiceIDForExpense(apiKey, client, w, payload.ExpenseID)
	if err != nil {
		createLog(fmt.Sprintf("Error resolving commission for pricing review expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
	} else if err := setCommissionUnderReview(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Error setting commission %d under review after pricing review: %v", serviceID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"expense_id":        payload.ExpenseID,
		"pricing_table_id":  payload.PricingTableID,
		"group_code":        groupCode,
		"territory":         territory,
		"calculated_amount": payload.CalculatedAmount,
		"service_id":        serviceID,
		"status":            commissionStatusUnderReview,
	})
}

func saveExpensePricingReview(apiKey string, client *http.Client, w http.ResponseWriter, payload expensePricingReviewPayload, groupCode string, territory string) error {
	existing, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expense_pricing_reviews",
		"columns":   "id",
		"condition": fmt.Sprintf("expense_id = %d", payload.ExpenseID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	values := fmt.Sprintf("%d, %s, %s, %.2f, %s", payload.PricingTableID, groupCode, escapeComma(territory), payload.CalculatedAmount, now)
	if len(existing) > 0 {
		return updateRowColumns(apiKey, client, w, "service_commission_expense_pricing_reviews",
			"pricing_table_id, group_code, territory, calculated_amount, updated_at",
			values,
			fmt.Sprintf("expense_id = %d", payload.ExpenseID))
	}

	_, err = insertRowAndReturnID(apiKey, client, w, "service_commission_expense_pricing_reviews",
		"expense_id, pricing_table_id, group_code, territory, calculated_amount, updated_at",
		fmt.Sprintf("%d, %s", payload.ExpenseID, values))
	return err
}

func handleSaveMileageReview(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload mileageReviewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ExpenseID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.DistanceKM <= 0 || payload.MileageRate <= 0 || payload.CalculatedAmount < 0 {
		http.Error(w, "Invalid mileage review", http.StatusBadRequest)
		return
	}

	if err := updateRowColumns(apiKey, client, w, "service_commission_expenses",
		"distance_km, mileage_rate",
		fmt.Sprintf("%.2f, %.4f", payload.DistanceKM, payload.MileageRate),
		fmt.Sprintf("id = %d AND type = 'mileage'", payload.ExpenseID)); err != nil {
		createLog(fmt.Sprintf("Error saving mileage review for expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
		http.Error(w, "Could not save mileage review", http.StatusInternalServerError)
		return
	}
	if err := saveMileageReviewAmount(apiKey, client, w, payload); err != nil {
		createLog(fmt.Sprintf("Error saving mileage reviewed amount for expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
		http.Error(w, "Could not save mileage review", http.StatusInternalServerError)
		return
	}

	serviceID, err := getServiceIDForExpense(apiKey, client, w, payload.ExpenseID)
	if err != nil {
		createLog(fmt.Sprintf("Error resolving commission for mileage review expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
	} else if err := setCommissionUnderReview(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Error setting commission %d under review after mileage review: %v", serviceID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"expense_id":        payload.ExpenseID,
		"distance_km":       payload.DistanceKM,
		"mileage_rate":      payload.MileageRate,
		"calculated_amount": payload.CalculatedAmount,
		"service_id":        serviceID,
		"status":            commissionStatusUnderReview,
	})
}

func saveMileageReviewAmount(apiKey string, client *http.Client, w http.ResponseWriter, payload mileageReviewPayload) error {
	existing, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expense_pricing_reviews",
		"columns":   "id",
		"condition": fmt.Sprintf("expense_id = %d", payload.ExpenseID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	values := fmt.Sprintf("G1, %s, %.2f, %s", escapeComma("Mileage"), payload.CalculatedAmount, now)
	if len(existing) > 0 {
		return updateRowColumns(apiKey, client, w, "service_commission_expense_pricing_reviews",
			"group_code, territory, calculated_amount, updated_at",
			values,
			fmt.Sprintf("expense_id = %d", payload.ExpenseID))
	}

	_, err = insertRowAndReturnID(apiKey, client, w, "service_commission_expense_pricing_reviews",
		"expense_id, group_code, territory, calculated_amount, updated_at",
		fmt.Sprintf("%d, %s", payload.ExpenseID, values))
	return err
}

func handleSaveExpenseAdvance(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload expenseAdvancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ExpenseID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.ResearcherAdvanceAmount < 0 {
		http.Error(w, "Invalid advance amount", http.StatusBadRequest)
		return
	}

	if err := saveExpenseAdvance(apiKey, client, w, payload); err != nil {
		createLog(fmt.Sprintf("Error saving researcher advance for expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
		http.Error(w, "Could not save advance amount", http.StatusInternalServerError)
		return
	}

	serviceID, err := getServiceIDForExpense(apiKey, client, w, payload.ExpenseID)
	if err != nil {
		createLog(fmt.Sprintf("Error resolving commission for advance expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
	} else if err := setCommissionUnderReview(apiKey, client, w, serviceID); err != nil {
		createLog(fmt.Sprintf("Error setting commission %d under review after advance review: %v", serviceID, err), 1, apiKey, client, w)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"expense_id":                payload.ExpenseID,
		"researcher_advance_amount": payload.ResearcherAdvanceAmount,
		"service_id":                serviceID,
		"status":                    commissionStatusUnderReview,
	})
}

func saveExpenseAdvance(apiKey string, client *http.Client, w http.ResponseWriter, payload expenseAdvancePayload) error {
	existing, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expense_pricing_reviews",
		"columns":   "id",
		"condition": fmt.Sprintf("expense_id = %d", payload.ExpenseID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	values := fmt.Sprintf("%.2f, %s", payload.ResearcherAdvanceAmount, now)
	if len(existing) > 0 {
		return updateRowColumns(apiKey, client, w, "service_commission_expense_pricing_reviews",
			"researcher_advance_amount, updated_at",
			values,
			fmt.Sprintf("expense_id = %d", payload.ExpenseID))
	}

	_, err = insertRowAndReturnID(apiKey, client, w, "service_commission_expense_pricing_reviews",
		"expense_id, researcher_advance_amount, updated_at",
		fmt.Sprintf("%d, %s", payload.ExpenseID, values))
	return err
}

func handleSaveExpenseEdit(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload expenseEditPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ExpenseID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.ResearcherAdvanceAmount < 0 {
		http.Error(w, "Invalid advance amount", http.StatusBadRequest)
		return
	}
	if payload.Amount < 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	serviceID, err := getServiceIDForExpense(apiKey, client, w, payload.ExpenseID)
	if err != nil {
		createLog(fmt.Sprintf("Error resolving commission for expense edit %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
	}

	description := strings.TrimSpace(payload.Description)
	if err := saveExpenseEditReview(apiKey, client, w, payload, description); err != nil {
		createLog(fmt.Sprintf("Error saving reviewed expense edit for expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
		http.Error(w, "Could not save expense edit", http.StatusInternalServerError)
		return
	}
	if expenseCanEditTransportMethod(apiKey, client, w, payload.ExpenseID) {
		if err := updateRowColumns(apiKey, client, w, "service_commission_expenses",
			"transport_method",
			escapeComma(strings.TrimSpace(payload.TransportMethod)),
			fmt.Sprintf("id = %d", payload.ExpenseID)); err != nil {
			createLog(fmt.Sprintf("Error saving transport method for expense %d: %v", payload.ExpenseID, err), 1, apiKey, client, w)
			http.Error(w, "Could not save expense transport method", http.StatusInternalServerError)
			return
		}
	}

	if serviceID != 0 {
		if err := setCommissionUnderReview(apiKey, client, w, serviceID); err != nil {
			createLog(fmt.Sprintf("Error setting commission %d under review after expense edit: %v", serviceID, err), 1, apiKey, client, w)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"expense_id":                payload.ExpenseID,
		"description":               description,
		"transport_method":          strings.TrimSpace(payload.TransportMethod),
		"amount":                    payload.Amount,
		"researcher_advance_amount": payload.ResearcherAdvanceAmount,
		"service_id":                serviceID,
		"status":                    commissionStatusUnderReview,
	})
}

func expenseCanEditTransportMethod(apiKey string, client *http.Client, w http.ResponseWriter, expenseID int) bool {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "type, purpose",
		"condition": fmt.Sprintf("id = %d", expenseID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		return false
	}
	return fmt.Sprint(rows[0]["purpose"]) == centerExpensePurpose && strings.TrimSpace(fmt.Sprint(rows[0]["type"])) == budgetTravelExpenseType
}

func saveExpenseEditReview(apiKey string, client *http.Client, w http.ResponseWriter, payload expenseEditPayload, description string) error {
	existing, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expense_pricing_reviews",
		"columns":   "id",
		"condition": fmt.Sprintf("expense_id = %d", payload.ExpenseID),
	}, apiKey, client, w)
	if err != nil {
		return err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	columns := []string{"reviewed_description", "updated_at"}
	values := []string{escapeComma(description), now}

	if expenseUsesDirectAmount(apiKey, client, w, payload.ExpenseID) {
		columns = append(columns, "calculated_amount")
		values = append(values, fmt.Sprintf("%.2f", payload.Amount))
	}

	if !expenseIsCenterExpense(apiKey, client, w, payload.ExpenseID) {
		columns = append(columns, "researcher_advance_amount")
		values = append(values, fmt.Sprintf("%.2f", payload.ResearcherAdvanceAmount))
	}

	if len(existing) > 0 {
		return updateRowColumns(apiKey, client, w, "service_commission_expense_pricing_reviews",
			strings.Join(columns, ", "),
			strings.Join(values, ", "),
			fmt.Sprintf("expense_id = %d", payload.ExpenseID))
	}

	insertColumns := append([]string{"expense_id"}, columns...)
	insertValues := append([]string{fmt.Sprint(payload.ExpenseID)}, values...)
	_, err = insertRowAndReturnID(apiKey, client, w, "service_commission_expense_pricing_reviews",
		strings.Join(insertColumns, ", "),
		strings.Join(insertValues, ", "))
	return err
}

func expenseUsesDirectAmount(apiKey string, client *http.Client, w http.ResponseWriter, expenseID int) bool {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "type, purpose",
		"condition": fmt.Sprintf("id = %d", expenseID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		return false
	}
	expenseType := strings.TrimSpace(fmt.Sprint(rows[0]["type"]))
	if isBudgetCenterExpenseType(expenseType) {
		return false
	}
	if fmt.Sprint(rows[0]["purpose"]) == centerExpensePurpose {
		return true
	}
	return !isReviewedAmountExpenseType(expenseType)
}

func isReviewedAmountExpenseType(expenseType string) bool {
	switch strings.TrimSpace(strings.ToLower(expenseType)) {
	case "per_diem", "accommodation", "mileage":
		return true
	default:
		return false
	}
}

func expenseIsCenterExpense(apiKey string, client *http.Client, w http.ResponseWriter, expenseID int) bool {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "purpose",
		"condition": fmt.Sprintf("id = %d", expenseID),
	}, apiKey, client, w)
	if err != nil || len(rows) == 0 {
		return false
	}
	return fmt.Sprint(rows[0]["purpose"]) == centerExpensePurpose
}

func getServiceIDForExpense(apiKey string, client *http.Client, w http.ResponseWriter, expenseID int) (int, error) {
	rows, err := runDBGet(map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "service_id",
		"condition": fmt.Sprintf("id = %d", expenseID),
	}, apiKey, client, w)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("expense not found")
	}
	return intFromAny(rows[0]["service_id"]), nil
}

func setCommissionUnderReview(apiKey string, client *http.Client, w http.ResponseWriter, serviceID int) error {
	if serviceID == 0 {
		return fmt.Errorf("missing service id")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return updateRowColumns(
		apiKey,
		client,
		w,
		"service_commisions",
		"status, updated_at",
		fmt.Sprintf("%s, %s", commissionStatusUnderReview, now),
		fmt.Sprintf(
			"id = %d AND (status IS NULL OR status NOT IN ('%s', '%s', '%s'))",
			serviceID,
			commissionStatusApproved,
			commissionStatusRejected,
			commissionStatusExported,
		),
	)
}

func isValidGroupCode(groupCode string) bool {
	return groupCode == "G1" || groupCode == "G2" || groupCode == "G3"
}

func isValidMealType(mealType string) bool {
	return mealType == "A" ||
		mealType == "B" ||
		mealType == "C" ||
		mealType == "FULL_BOARD"
}

func handleApproveExpense(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	// The current table service_commission_expenses does not include status/reviewer columns.
	// Keep this endpoint compatible by approving the parent commission when an expense id is sent.
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		ExpenseID int `json:"expense_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ExpenseID == 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "service_commission_expenses",
		"columns":   "service_id",
		"condition": fmt.Sprintf("id = %d", payload.ExpenseID),
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Could not fetch expense", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		http.Error(w, "Expense not found", http.StatusNotFound)
		return
	}

	serviceID := intFromAny(rows[0]["service_id"])
	if serviceID == 0 {
		http.Error(w, "Invalid expense", http.StatusBadRequest)
		return
	}

	update := map[string]interface{}{
		"table": "service_commisions",
		"updates": map[string]interface{}{
			"status":     commissionStatusApproved,
			"updated_at": time.Now().Format("2006-01-02 15:04:05"),
		},
		"condition": fmt.Sprintf("id = %d", serviceID),
	}
	proxyJSONPut(update, apiKey, client, w)
}

func handleExportCommissions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	ids := r.URL.Query().Get("ids")
	condition := fmt.Sprintf("sc.status = '%s'", commissionStatusApproved)
	if strings.TrimSpace(ids) != "" {
		cleanIDs := sanitizeIDList(ids)
		if cleanIDs == "" {
			http.Error(w, "Invalid ids", http.StatusBadRequest)
			return
		}
		condition = fmt.Sprintf("sc.status IN ('%s', '%s') AND sc.id IN (%s)", commissionStatusApproved, commissionStatusExported, cleanIDs)
	}
	condition += " AND d.signed_pdf_path IS NOT NULL AND d.signed_pdf_path <> ''"

	query := map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN service_commission_documents d ON d.service_id = sc.id`,
		"columns": `sc.id,
COALESCE(ct.id_intern, br.id_intern, 'SC-' || sc.id) AS code,
sc.status,
d.signed_pdf_path`,
		"condition": condition + " GROUP BY sc.id ORDER BY sc.id",
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Could not load signed PDFs", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		http.Error(w, "Could not parse signed PDF data", http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		http.Error(w, "No signed PDFs found for approved commissions", http.StatusNotFound)
		return
	}

	exportRows := exportableSignedServiceCommissionRows(rows)
	if len(exportRows) == 0 {
		http.Error(w, "No signed PDFs found for approved commissions", http.StatusNotFound)
		return
	}
	if err := markServiceCommissionsExported(apiKey, client, w, exportRows); err != nil {
		createLog(fmt.Sprintf("Error marking service commissions as exported: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not update exported status", http.StatusInternalServerError)
		return
	}

	if len(exportRows) == 1 {
		serveSignedServiceCommissionPDF(w, r, exportRows[0])
		return
	}

	serveSignedServiceCommissionPDFZip(w, exportRows)
}

func handleServiceCommissionDataExportOptions(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	projectExpr := serviceCommissionProjectListNameSQL()
	projectIDExpr := "COALESCE(NULLIF(ct.project, ''), bp.project_id)"
	projectRows, err := runDBGet(map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN projects pr ON pr.id = COALESCE(NULLIF(ct.project, ''), bp.project_id)
LEFT JOIN service_commission_documents d ON d.service_id = sc.id`,
		"columns":   fmt.Sprintf("%s AS id, %s AS name", projectIDExpr, projectExpr),
		"condition": "d.signed_pdf_path IS NOT NULL AND d.signed_pdf_path <> '' GROUP BY " + projectIDExpr + " ORDER BY name",
	}, apiKey, client, w)
	if err != nil {
		http.Error(w, "Could not load projects", http.StatusInternalServerError)
		return
	}

	personRows, err := runDBGet(map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN people p ON sc.people_id = p.id
LEFT JOIN service_commission_documents d ON d.service_id = sc.id`,
		"columns": `sc.people_id AS id,
TRIM(COALESCE(p.name, '') || ' ' || COALESCE(p.surname, '') || ' ' || COALESCE(p.secondSurname, '')) AS name`,
		"condition": "d.signed_pdf_path IS NOT NULL AND d.signed_pdf_path <> '' GROUP BY sc.people_id ORDER BY name",
	}, apiKey, client, w)
	if err != nil {
		http.Error(w, "Could not load people", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"projects": normalizeServiceCommissionExportOptions(projectRows),
		"people":   normalizeServiceCommissionExportOptions(personRows),
	})
}

func normalizeServiceCommissionExportOptions(rows []map[string]interface{}) []map[string]interface{} {
	options := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(fmt.Sprint(row["id"]))
		name := strings.TrimSpace(unescapeComma(fmt.Sprint(row["name"])))
		if id == "" || id == "<nil>" {
			continue
		}
		if name == "" || name == "<nil>" {
			name = id
		}
		options = append(options, map[string]interface{}{"id": id, "name": name})
	}
	return options
}

type serviceCommissionDataExportPayload struct {
	DateFrom  string   `json:"date_from"`
	DateTo    string   `json:"date_to"`
	Projects  []string `json:"projects"`
	People    []int    `json:"people"`
	Documents []string `json:"documents"`
}

func handleServiceCommissionDataExport(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, role, accessProjects, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)
	if !userCanReviewProjects(role, accessProjects, accessGerencia) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload serviceCommissionDataExportPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	documents := serviceCommissionSelectedExportDocuments(payload.Documents)
	if len(documents) == 0 {
		http.Error(w, "Select at least one document type", http.StatusBadRequest)
		return
	}

	rows, err := serviceCommissionDataExportRows(apiKey, client, w, payload)
	if err != nil {
		createLog(fmt.Sprintf("Error loading service commission data export rows: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not load export data", http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		http.Error(w, "No signed commissions found for these filters", http.StatusNotFound)
		return
	}

	files, err := serviceCommissionDataExportFiles(apiKey, client, w, rows, documents)
	if err != nil {
		createLog(fmt.Sprintf("Error loading service commission data export files: %v", err), 1, apiKey, client, w)
		http.Error(w, "Could not load export files", http.StatusInternalServerError)
		return
	}
	if len(files) == 0 {
		http.Error(w, "No files found for these filters", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="service_commission_data_export.zip"`)
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()
	writeServiceCommissionDataExportZip(zipWriter, files)
}

func serviceCommissionSelectedExportDocuments(values []string) map[string]bool {
	if len(values) == 0 {
		values = []string{"tickets", "payment_proofs", "mileage", "commission_pdf", "attendance_certificate"}
	}
	allowed := map[string]bool{
		"tickets":                false,
		"payment_proofs":         false,
		"mileage":                false,
		"commission_pdf":         false,
		"attendance_certificate": false,
	}
	for _, value := range values {
		key := strings.TrimSpace(strings.ToLower(value))
		if _, ok := allowed[key]; ok {
			allowed[key] = true
		}
	}
	selected := make(map[string]bool)
	for key, ok := range allowed {
		if ok {
			selected[key] = true
		}
	}
	return selected
}

func serviceCommissionDataExportRows(apiKey string, client *http.Client, w http.ResponseWriter, payload serviceCommissionDataExportPayload) ([]map[string]interface{}, error) {
	projectIDExpr := "COALESCE(NULLIF(ct.project, ''), bp.project_id)"
	projectNameExpr := serviceCommissionProjectListNameSQL()
	conditions := []string{"d.signed_pdf_path IS NOT NULL", "d.signed_pdf_path <> ''"}
	if date := strings.TrimSpace(payload.DateFrom); date != "" {
		conditions = append(conditions, fmt.Sprintf("date(d.updated_at) >= date('%s')", escapeSQL(date)))
	}
	if date := strings.TrimSpace(payload.DateTo); date != "" {
		conditions = append(conditions, fmt.Sprintf("date(d.updated_at) <= date('%s')", escapeSQL(date)))
	}
	if len(payload.Projects) > 0 {
		values := make([]string, 0, len(payload.Projects))
		for _, project := range payload.Projects {
			project = strings.TrimSpace(project)
			if project != "" {
				values = append(values, "'"+escapeSQL(project)+"'")
			}
		}
		if len(values) > 0 {
			conditions = append(conditions, projectIDExpr+" IN ("+strings.Join(values, ",")+")")
		}
	}
	if len(payload.People) > 0 {
		values := make([]string, 0, len(payload.People))
		for _, person := range payload.People {
			if person > 0 {
				values = append(values, fmt.Sprint(person))
			}
		}
		if len(values) > 0 {
			conditions = append(conditions, "sc.people_id IN ("+strings.Join(values, ",")+")")
		}
	}

	return runDBGet(map[string]interface{}{
		"table": `service_commisions sc
LEFT JOIN people p ON sc.people_id = p.id
LEFT JOIN comm_travels ct ON sc.new_travel_id = ct.id
LEFT JOIN budget_requests br ON sc.request_travel_id = br.id
LEFT JOIN budget_parts bp ON br.id = bp.id_combined AND bp.category_id = 1
LEFT JOIN budget_attendance_certificates bac ON bac.id_combined = br.id
LEFT JOIN projects pr ON pr.id = COALESCE(NULLIF(ct.project, ''), bp.project_id)
LEFT JOIN service_commission_documents d ON d.service_id = sc.id`,
		"columns": fmt.Sprintf(`sc.id,
COALESCE(ct.id_intern, br.id_intern, 'SC-' || sc.id) AS code,
sc.people_id AS person_id,
TRIM(COALESCE(p.name, '') || ' ' || COALESCE(p.surname, '') || ' ' || COALESCE(p.secondSurname, '')) AS person_name,
%s AS project_id,
%s AS project_name,
d.signed_pdf_path,
COALESCE(NULLIF(ct.file_path, ''), bac.file_path, '') AS attendance_certificate_path,
d.updated_at AS signed_at`, projectIDExpr, projectNameExpr),
		"condition": strings.Join(conditions, " AND ") + " GROUP BY sc.id ORDER BY project_name, person_name, code",
	}, apiKey, client, w)
}

type serviceCommissionDataExportFile struct {
	SourcePath string
	ZipPath    string
}

func serviceCommissionDataExportFiles(apiKey string, client *http.Client, w http.ResponseWriter, rows []map[string]interface{}, documents map[string]bool) ([]serviceCommissionDataExportFile, error) {
	files := make([]serviceCommissionDataExportFile, 0)
	rowByID := make(map[int]map[string]interface{}, len(rows))
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		id := intFromAny(row["id"])
		if id == 0 {
			continue
		}
		rowByID[id] = row
		ids = append(ids, fmt.Sprint(id))
		if documents["commission_pdf"] {
			path := strings.TrimSpace(fmt.Sprint(row["signed_pdf_path"]))
			if path != "" && path != "<nil>" {
				files = append(files, serviceCommissionDataExportFile{
					SourcePath: path,
					ZipPath:    serviceCommissionDataExportZipPath(row, "commission_pdf", filepath.Base(path)),
				})
			}
		}
		if documents["attendance_certificate"] {
			path := strings.TrimSpace(fmt.Sprint(row["attendance_certificate_path"]))
			if path != "" && path != "<nil>" {
				files = append(files, serviceCommissionDataExportFile{
					SourcePath: path,
					ZipPath:    serviceCommissionDataExportZipPath(row, "attendance_certificate", filepath.Base(path)),
				})
			}
		}
	}
	if len(ids) == 0 {
		return files, nil
	}

	fileTypes := make([]string, 0)
	if documents["tickets"] {
		fileTypes = append(fileTypes, "'receipt'")
	}
	if documents["payment_proofs"] {
		fileTypes = append(fileTypes, "'payment_proof'")
	}
	if documents["mileage"] {
		fileTypes = append(fileTypes, "'mileage_proof'")
	}
	if len(fileTypes) == 0 {
		return files, nil
	}

	fileRows, err := runDBGet(map[string]interface{}{
		"table": `comm_files f
INNER JOIN service_commission_expenses e ON f.expense_id = e.id`,
		"columns":   "e.service_id, f.file_path, f.type",
		"condition": "e.service_id IN (" + strings.Join(ids, ",") + ") AND f.type IN (" + strings.Join(fileTypes, ",") + ") ORDER BY e.service_id, f.type, f.id",
	}, apiKey, client, w)
	if err != nil {
		return nil, err
	}
	for _, fileRow := range fileRows {
		serviceID := intFromAny(fileRow["service_id"])
		parent := rowByID[serviceID]
		if parent == nil {
			continue
		}
		docFolder := serviceCommissionDataExportDocFolder(fmt.Sprint(fileRow["type"]))
		if docFolder == "" {
			continue
		}
		path := strings.TrimSpace(fmt.Sprint(fileRow["file_path"]))
		files = append(files, serviceCommissionDataExportFile{
			SourcePath: path,
			ZipPath:    serviceCommissionDataExportZipPath(parent, docFolder, filepath.Base(path)),
		})
	}
	return files, nil
}

func serviceCommissionDataExportDocFolder(fileType string) string {
	switch strings.TrimSpace(fileType) {
	case "receipt":
		return "tickets"
	case "payment_proof":
		return "comprobantes"
	case "mileage_proof":
		return "mileage"
	default:
		return ""
	}
}
func serviceCommissionDataExportZipPath(row map[string]interface{}, docFolder string, filename string) string {
	project := sanitizeDownloadFilename(firstNonEmptyString(row["project_name"], row["project_id"], "project"))
	person := sanitizeDownloadFilename(firstNonEmptyString(row["person_name"], fmt.Sprintf("person-%d", intFromAny(row["person_id"]))))
	code := sanitizeDownloadFilename(firstNonEmptyString(row["code"], fmt.Sprintf("SC-%d", intFromAny(row["id"]))))
	file := sanitizeDownloadFilename(filename)

	if file == "" {
		file = "document"
	}
	_ = docFolder

	return filepath.ToSlash(filepath.Join(project, person, code, file))
}

func writeServiceCommissionDataExportZip(zipWriter *zip.Writer, files []serviceCommissionDataExportFile) {
	seen := make(map[string]int)
	for _, file := range files {
		fullPath, err := serviceCommissionUploadFullPath(file.SourcePath)
		if err != nil {
			continue
		}
		zipPath := uniqueZipFilename(file.ZipPath, seen)
		writer, err := zipWriter.Create(zipPath)
		if err != nil {
			continue
		}
		handle, err := os.Open(fullPath)
		if err != nil {
			continue
		}
		_, _ = io.Copy(writer, handle)
		_ = handle.Close()
	}
}

func serviceCommissionUploadFullPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "<nil>" {
		return "", fmt.Errorf("empty file path")
	}
	fullPath := filepath.Clean(filepath.Join("../../", path))
	if !strings.HasPrefix(fullPath, filepath.Clean("../../Uploads")+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid file path")
	}
	if _, err := os.Stat(fullPath); err != nil {
		return "", err
	}
	return fullPath, nil
}

func exportableSignedServiceCommissionRows(rows []map[string]interface{}) []map[string]interface{} {
	exportRows := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		if _, _, err := signedServiceCommissionPDFPath(row); err == nil {
			exportRows = append(exportRows, row)
		}
	}
	return exportRows
}

func markServiceCommissionsExported(apiKey string, client *http.Client, w http.ResponseWriter, rows []map[string]interface{}) error {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		id := intFromAny(row["id"])
		if id != 0 {
			ids = append(ids, fmt.Sprint(id))
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("missing service commission ids")
	}
	return updateRowColumns(apiKey, client, w, "service_commisions",
		"status, updated_at",
		fmt.Sprintf("%s, %s", commissionStatusExported, time.Now().Format("2006-01-02 15:04:05")),
		fmt.Sprintf("id IN (%s) AND status = '%s'", strings.Join(ids, ","), commissionStatusApproved))
}

func serveSignedServiceCommissionPDF(w http.ResponseWriter, r *http.Request, row map[string]interface{}) {
	fullPath, filename, err := signedServiceCommissionPDFPath(row)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.EqualFold(filepath.Ext(fullPath), ".zip") {
		w.Header().Set("Content-Type", "application/zip")
	} else {
		w.Header().Set("Content-Type", "application/pdf")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, fullPath)
}

func serveSignedServiceCommissionPDFZip(w http.ResponseWriter, rows []map[string]interface{}) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="signed_service_commissions.zip"`)

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	seen := make(map[string]int)

	for _, row := range rows {
		fullPath, filename, err := signedServiceCommissionPDFPath(row)
		if err != nil {
			continue
		}

		commissionFolder := sanitizeDownloadFilename(firstNonEmptyString(
			row["code"],
			fmt.Sprintf("SC-%s", fmt.Sprint(row["id"])),
		))
		if commissionFolder == "" {
			commissionFolder = "service-commission"
		}

		// Si el documento firmado es un ZIP interno, metemos sus PDFs
		// directamente dentro de la carpeta de la comisión.
		if addInnerZipFilesToExportZip(zipWriter, fullPath, commissionFolder, seen) {
			continue
		}

		// Caso normal: la comisión tiene un único PDF.
		outName := uniqueZipFilename(
			filepath.ToSlash(filepath.Join(commissionFolder, sanitizeDownloadFilename(filename))),
			seen,
		)

		fileWriter, err := zipWriter.Create(outName)
		if err != nil {
			continue
		}

		file, err := os.Open(fullPath)
		if err != nil {
			continue
		}

		_, _ = io.Copy(fileWriter, file)
		_ = file.Close()
	}
}
func addInnerZipFilesToExportZip(zipWriter *zip.Writer, zipPath string, commissionFolder string, seen map[string]int) bool {
	file, err := os.Open(zipPath)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false
	}

	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return false
	}

	addedAny := false

	for _, innerFile := range reader.File {
		if innerFile.FileInfo().IsDir() {
			continue
		}

		// Esto elimina carpetas internas del ZIP original.
		innerFilename := sanitizeDownloadFilename(filepath.Base(innerFile.Name))
		if innerFilename == "" {
			innerFilename = "document.pdf"
		}

		if !strings.EqualFold(filepath.Ext(innerFilename), ".pdf") {
			continue
		}

		outName := uniqueZipFilename(
			filepath.ToSlash(filepath.Join(commissionFolder, innerFilename)),
			seen,
		)

		src, err := innerFile.Open()
		if err != nil {
			continue
		}

		dst, err := zipWriter.Create(outName)
		if err != nil {
			_ = src.Close()
			continue
		}

		_, _ = io.Copy(dst, src)
		_ = src.Close()

		addedAny = true
	}

	return addedAny
}
func signedServiceCommissionPDFPath(row map[string]interface{}) (string, string, error) {
	path := strings.TrimSpace(fmt.Sprint(row["signed_pdf_path"]))
	if path == "" || path == "<nil>" {
		return "", "", fmt.Errorf("signed PDF not available")
	}
	fullPath := filepath.Clean(filepath.Join("../../", path))
	if !strings.HasPrefix(fullPath, filepath.Clean("../../Uploads")+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("invalid signed PDF path")
	}
	if _, err := os.Stat(fullPath); err != nil {
		return "", "", fmt.Errorf("signed PDF file not found")
	}
	code := sanitizeDownloadFilename(firstNonEmptyString(row["code"], fmt.Sprintf("SC-%s", fmt.Sprint(row["id"]))))
	if code == "" {
		code = "service-commission"
	}
	if strings.EqualFold(filepath.Ext(fullPath), ".zip") {
		return fullPath, code + ".zip", nil
	}
	return fullPath, code + ".pdf", nil
}

func uniqueZipFilename(filename string, seen map[string]int) string {
	count := seen[filename]
	seen[filename] = count + 1
	if count == 0 {
		return filename
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return fmt.Sprintf("%s-%d%s", base, count+1, ext)
}

func sanitizeDownloadFilename(value string) string {
	value = strings.TrimSpace(unescapeComma(value))
	if value == "" || value == "<nil>" {
		return ""
	}
	value = regexp.MustCompile(`[^A-Za-z0-9._-]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func getUserInfoWithProjects(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) (int, string, string, bool) {
	var user string
	if c, err := r.Cookie("CRMINTRATOOLS"); err == nil {
		if claims, ok := parseAuthCookie(c.Value, nil); ok {
			user = getUsernameByHash(claims.Sub, apiKey, client, w)
		}
	}

	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id, role, access_projects",
		"condition": fmt.Sprintf("username = '%s'", user),
	}

	jsonData, _ := json.Marshal(query)
	respID := getReq(jsonData, apiKey, client, w)
	if respID == nil {
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return 0, "", "", false
	}
	defer respID.Body.Close()

	bodyB, _ := io.ReadAll(respID.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(bodyB, &result); err != nil || len(result) == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return 0, "", "", false
	}

	userID := intFromAny(result[0]["people_id"])
	role, _ := result[0]["role"].(string)
	accessProjects := asBool(result[0]["access_projects"])

	return userID, user, role, accessProjects
}

func getUserInfoForRateManagement(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) (int, string, bool, bool) {
	var user string
	if c, err := r.Cookie("CRMINTRATOOLS"); err == nil {
		if claims, ok := parseAuthCookie(c.Value, nil); ok {
			user = getUsernameByHash(claims.Sub, apiKey, client, w)
		}
	}

	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id, role, access_projects, access_gerencia",
		"condition": fmt.Sprintf("username = '%s'", user),
	}

	jsonData, _ := json.Marshal(query)
	respID := getReq(jsonData, apiKey, client, w)
	if respID == nil {
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return 0, "", false, false
	}
	defer respID.Body.Close()

	bodyB, _ := io.ReadAll(respID.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(bodyB, &result); err != nil || len(result) == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return 0, "", false, false
	}

	role, _ := result[0]["role"].(string)
	return intFromAny(result[0]["people_id"]),
		role,
		asBool(result[0]["access_projects"]),
		asBool(result[0]["access_gerencia"])
}

func asBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return int(v) == 1
	case int:
		return v == 1
	case string:
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	default:
		return false
	}
}

func userCanReviewProjects(role string, accessProjects bool, accessGerencia bool) bool {
	return role == "admin" || role == "superadmin" || accessProjects || accessGerencia
}

func userCanViewAttendanceCertificates(role string, accessProjects bool, accessGerencia bool) bool {
	return role == "admin" || role == "superadmin" || (accessProjects && accessGerencia)
}

func userCanManageRates(role string, accessProjects bool, accessGerencia bool) bool {
	return role == "admin" || role == "superadmin" || accessProjects || accessGerencia
}

func handleServiceCommissionSession(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, role, accessProjects := getUserInfoWithProjects(apiKey, client, w, r)
	if userID == 0 && username == "" {
		return
	}
	_, _, _, accessGerencia := getUserInfoForRateManagement(apiKey, client, w, r)

	payload := map[string]interface{}{
		"user_id":                          userID,
		"username":                         username,
		"role":                             role,
		"access_projects":                  accessProjects,
		"access_gerencia":                  accessGerencia,
		"can_review":                       userCanReviewProjects(role, accessProjects, accessGerencia),
		"can_view_attendance_certificates": userCanViewAttendanceCertificates(role, accessProjects, accessGerencia),
		"can_manage_rates":                 userCanManageRates(role, accessProjects, accessGerencia),
	}

	respondJSON(w, http.StatusOK, payload)
}

func readExpensesFromMultipart(r *http.Request) []commissionExpensePayload {
	expenses := make([]commissionExpensePayload, 0)
	for i := 0; ; i++ {
		prefix := fmt.Sprintf("expenses[%d]", i)
		expenseType := strings.TrimSpace(r.FormValue(prefix + "[type]"))
		if expenseType == "" {
			if i == 0 {
				break
			}
			// Stop at first gap because indexes are generated sequentially in the frontend.
			break
		}

		expenses = append(expenses, commissionExpensePayload{
			Type:             expenseType,
			Amount:           parseFloatOrZero(r.FormValue(prefix + "[amount]")),
			DistanceKM:       mileageDistanceFromAmount(expenseType, parseFloatOrZero(r.FormValue(prefix+"[amount]"))),
			TransportMethod:  strings.TrimSpace(r.FormValue(prefix + "[transport_method]")),
			Description:      strings.TrimSpace(r.FormValue(prefix + "[description]")),
			DepartureDate:    strings.TrimSpace(r.FormValue(prefix + "[departure_date]")),
			ReturnDate:       strings.TrimSpace(r.FormValue(prefix + "[return_date]")),
			DepartureTime:    strings.TrimSpace(r.FormValue(prefix + "[departure_time]")),
			ReturnTime:       strings.TrimSpace(r.FormValue(prefix + "[return_time]")),
			PerDiemBreakdown: strings.TrimSpace(r.FormValue(prefix + "[per_diem_breakdown]")),
		})
	}
	return expenses
}

func mileageDistanceFromAmount(expenseType string, value float64) float64 {
	if expenseType == "mileage" {
		return value
	}

	return 0
}

func validateMileageProofUploads(r *http.Request, expenses []commissionExpensePayload) error {
	for index, expense := range expenses {
		if expense.Type != "mileage" {
			continue
		}

		formKey := fmt.Sprintf("expenses[%d][mileage_proof]", index)
		if !multipartHasFiles(r, formKey) {
			return fmt.Errorf("mileage screenshot is required for expense %d", index+1)
		}
	}

	return nil
}

func saveExpenseUploadFiles(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, expense commissionExpensePayload, storage fileStorageContext, expenseIndex int, expenseID int) {
	keys := map[string]string{
		fmt.Sprintf("expenses[%d][receipt]", expenseIndex):       "receipt",
		fmt.Sprintf("expenses[%d][payment_proof]", expenseIndex): "payment_proof",
		fmt.Sprintf("expenses[%d][mileage_proof]", expenseIndex): "mileage_proof",
	}
	for formKey, fileType := range keys {
		paths := saveExpenseFiles(r, formKey, storage, expense.Type, fileType)
		for _, filePath := range paths {
			_, _ = insertRowAndReturnID(apiKey, client, w, "comm_files",
				"expense_id, file_path, type",
				fmt.Sprintf("%d, %s, %s", expenseID, escapeComma(filePath), fileType))
		}
	}
}

func saveTravelFiles(r *http.Request, storage fileStorageContext) string {
	paths := saveMultipartFiles(r, "travel_attendance_certificate", filepath.Join("Uploads", "03_BudgetReq", safePathPart(storage.ProjectCode)), func(index int, original string) string {
		return buildUploadFilename("attendance_certificate", storage.Username, "", storage.Timestamp, original, index)
	})
	return strings.Join(paths, ";")
}

func multipartHasFiles(r *http.Request, formKey string) bool {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return false
	}

	files := r.MultipartForm.File[formKey]
	return len(files) > 0
}

func saveExpenseFiles(r *http.Request, formKey string, storage fileStorageContext, expenseType string, fileType string) []string {
	prefix := "comprobante"
	if fileType == "receipt" {
		prefix = "ticket"
	}

	uploadDir := filepath.Join("Uploads", "04_ServiceCommissions", safePathPart(storage.ProjectCode), safePathPart(expenseType))

	return saveMultipartFiles(r, formKey, uploadDir, func(index int, original string) string {
		return buildUploadFilename(prefix, storage.Username, storage.TravelCode, storage.Timestamp, original, index)
	})
}

func saveMultipartFiles(r *http.Request, formKey string, uploadDir string, filenameBuilder func(index int, original string) string) []string {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}

	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Join("../../", uploadDir), 0750); err != nil {
		return nil
	}

	paths := make([]string, 0, len(files))
	for index, header := range files {
		in, err := header.Open()
		if err != nil {
			continue
		}
		defer in.Close()

		filename := filenameBuilder(index, header.Filename)
		relativePath := filepath.ToSlash(filepath.Join(uploadDir, filename))
		out, err := os.Create(filepath.Join("../../", relativePath))
		if err != nil {
			continue
		}
		_, _ = io.Copy(out, in)
		_ = out.Close()
		paths = append(paths, relativePath)
	}
	return paths
}

func insertRowAndReturnID(apiKey string, client *http.Client, w http.ResponseWriter, table string, columns, values string) (int, error) {
	payload := map[string]interface{}{
		"table":   table,
		"columns": columns,
		"value":   values,
	}
	jsonData, _ := json.Marshal(payload)

	//fmt.Printf("Inserting into %s: %v\n", table, values)
	//fmt.Printf("Payload: %s\n", string(jsonData))
	resp := postReq(jsonData, apiKey, client, w)
	if resp == nil {
		return 0, fmt.Errorf("insert failed")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("insert failed with status %d: %s", resp.StatusCode, string(body))
	}

	id := extractIDFromResponse(body)
	if id != 0 {
		return id, nil
	}

	// Fallback for DB APIs that insert successfully but do not return the new id.
	return fetchLastID(apiKey, client, w, table)
}

func fetchLastID(apiKey string, client *http.Client, w http.ResponseWriter, table string) (int, error) {
	query := map[string]interface{}{
		"table":     table,
		"columns":   "id",
		"condition": "1=1 ORDER BY id DESC LIMIT 1",
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return 0, fmt.Errorf("could not fetch inserted id")
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil || len(rows) == 0 {
		return 0, fmt.Errorf("could not parse inserted id")
	}
	return intFromAny(rows[0]["id"]), nil
}

func updateRow(apiKey string, client *http.Client, w http.ResponseWriter, table string, updates map[string]interface{}, condition string) error {
	payload := map[string]interface{}{
		"table":     table,
		"updates":   updates,
		"condition": condition,
	}
	jsonData, _ := json.Marshal(payload)
	resp := putReq(jsonData, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("update failed")
	}
	defer resp.Body.Close()
	return nil
}

func updateRowColumns(apiKey string, client *http.Client, w http.ResponseWriter, table string, columns string, values string, condition string) error {
	payload := map[string]interface{}{
		"table":     table,
		"columns":   columns,
		"value":     values,
		"condition": condition,
	}

	jsonData, _ := json.Marshal(payload)
	resp := putReq(jsonData, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("update failed")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("update failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func proxyJSONGet(query map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) {
	jsonData, _ := json.Marshal(query)

	//fmt.Println("Proxying GET with payload:", string(jsonData))

	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Could not fetch data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	//fmt.Println("Proxy GET status:", resp.StatusCode)
	//fmt.Println("Proxy GET body:", string(body))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func runDBGet(query map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) ([]map[string]interface{}, error) {
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("database request failed")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("database request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}

	return rows, nil
}

func proxyJSONPut(query map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) {
	jsonData, _ := json.Marshal(query)
	resp := putReq(jsonData, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Database request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func extractIDFromResponse(body []byte) int {
	var object map[string]interface{}
	if err := json.Unmarshal(body, &object); err == nil {
		for _, key := range []string{"id", "last_insert_id", "lastInsertId", "insert_id"} {
			if id := intFromAny(object[key]); id != 0 {
				return id
			}
		}
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err == nil && len(rows) > 0 {
		for _, key := range []string{"id", "last_insert_id", "lastInsertId", "insert_id"} {
			if id := intFromAny(rows[0][key]); id != 0 {
				return id
			}
		}
	}
	return 0
}

func parseIntFormValue(r *http.Request, key string) int {
	return parseIntOrZero(r.FormValue(key))
}

func parseIntOrZero(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func parseFloatOrZero(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func intFromAny(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		parsed, _ := v.Int64()
		return int(parsed)
	case string:
		return parseIntOrZero(v)
	default:
		return 0
	}
}

func nullableString(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, " ", "_")
	filename = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, filename)
	if filename == "" || filename == "." {
		return fmt.Sprintf("file_%d", time.Now().UnixNano())
	}
	return filename
}

func safePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "NO_PROJECT"
	}

	return strings.Trim(sanitizeFilename(value), ".")
}

func buildUploadFilename(prefix string, username string, travelCode string, timestamp string, original string, index int) string {
	parts := []string{
		safePathPart(prefix),
		safePathPart(username),
	}

	if strings.TrimSpace(travelCode) != "" {
		parts = append(parts, safePathPart(travelCode))
	}

	parts = append(parts, safePathPart(timestamp))

	if index > 0 {
		parts = append(parts, fmt.Sprintf("%02d", index+1))
	}

	extension := strings.ToLower(filepath.Ext(filepath.Base(original)))
	if extension == "" {
		extension = ".dat"
	}

	return strings.Join(parts, "_") + extension
}

func escapeSQL(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func floatFromAny(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		parsed, _ := v.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed
	default:
		return 0
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func daysBetween(start string, end string) int {
	startTime, errStart := time.Parse("2006-01-02", strings.TrimSpace(start))
	endTime, errEnd := time.Parse("2006-01-02", strings.TrimSpace(end))
	if errStart != nil || errEnd != nil || endTime.Before(startTime) {
		return 0
	}
	return int(endTime.Sub(startTime).Hours()/24) + 1
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sha256String(value string) string {
	return sha256Hex([]byte(value))
}

func resolveModule10ReadablePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if module10FileExists(path) {
		return path, nil
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("%s not found", path)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, path)
		if module10FileExists(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("%s not found from %s", path, wd)
}

func resolveModule10ExecutablePath(path string) (string, error) {
	resolved, err := resolveModule10ReadablePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable", resolved)
	}
	return resolved, nil
}

func module10FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstNonEmptyModule10(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return "-"
}

func sanitizeModule10SignerOutput(output string) string {
	output = strings.ReplaceAll(output, "\n", " ")
	output = strings.ReplaceAll(output, "\r", " ")
	output = strings.ReplaceAll(output, strings.TrimSpace(mC.PDFCertificatePassword), "[hidden]")
	if len(output) > 500 {
		return output[:500] + "..."
	}
	return output
}

func sanitizeIDList(ids string) string {
	parts := strings.Split(ids, ",")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		id := parseIntOrZero(part)
		if id > 0 {
			clean = append(clean, strconv.Itoa(id))
		}
	}
	return strings.Join(clean, ",")
}
