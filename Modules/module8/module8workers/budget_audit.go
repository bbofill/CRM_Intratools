package module8workers

import (
	"bytes"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

const budgetAuditGenesisHash = "GENESIS"

type BudgetAuditPayload struct {
	CombinedID int                    `json:"combined_id"`
	PartID     *int                   `json:"part_id,omitempty"`
	CategoryID *int                   `json:"category_id,omitempty"`
	Action     string                 `json:"action"`
	Status     string                 `json:"status"`
	Summary    string                 `json:"summary"`
	CreatedAt  string                 `json:"created_at"`
	Actor      BudgetAuditActor       `json:"actor"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

type BudgetAuditActor struct {
	PeopleID int    `json:"people_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
}

type BudgetAuditRow struct {
	ID           int    `json:"id"`
	CombinedID   int    `json:"combined_id"`
	PartID       *int   `json:"part_id,omitempty"`
	CategoryID   *int   `json:"category_id,omitempty"`
	PreviousHash string `json:"previous_hash"`
	CurrentHash  string `json:"current_hash"`
	Payload      string `json:"payload"`
	CreatedAt    string `json:"created_at"`
}

type BudgetAuditVerificationResult struct {
	OK       bool   `json:"ok"`
	BrokenAt int    `json:"brokenAt,omitempty"`
	Error    string `json:"error,omitempty"`
	LastHash string `json:"lastHash,omitempty"`
}

type BudgetAuditPartRef struct {
	PartID     int
	CategoryID int
}

type BudgetAuditCertificateMeta struct {
	RequesterName       string
	RequesterSurname    string
	RequesterDocument   string
	ProjectName         string
	ProjectCode         string
	RequestCreationDate string
	TravelArea          string
	ManagerName         string
	ManagerSurname      string
	LogoPath            string
	IDIntern            string
}

type BudgetAuditPDFBuildResult struct {
	Bytes             []byte
	SignaturePosition VisibleSignaturePosition
}

type VisibleSignaturePosition struct {
	Page         int
	Xmm          float64
	Ymm          float64
	Wmm          float64
	Hmm          float64
	PageHeightMm float64
}

type PDFSignatureRect struct {
	Page int
	Xpt  float64
	Ypt  float64
	Wpt  float64
	Hpt  float64
}

func encryptBudgetAuditPayload(payloadText string) (string, error) {
	encryptedPayload := encryptLogWithGPG(payloadText)
	if encryptedPayload == "" {
		return "", fmt.Errorf("error cifrando payload de budget audit")
	}

	encodedPayload := base64_encode(encryptedPayload)
	if encodedPayload == "" {
		return "", fmt.Errorf("error codificando payload de budget audit")
	}

	return encodedPayload, nil
}

func decryptBudgetAuditPayload(encodedEncryptedPayload string) (string, error) {
	return decryptLogWithGPG(encodedEncryptedPayload)
}

func calculateBudgetAuditHash(previousHash string, payloadText string) string {
	hashInput := fmt.Sprintf("previous_hash=%s\npayload=%s", previousHash, payloadText)

	sum := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(sum[:])
}

func base64_decode(input string) (string, error) {
	decoded, err := b64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func getBudgetCertificateManager(
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (string, string, error) {
	query := map[string]interface{}{
		"table":   "users u LEFT JOIN people p ON u.people_id = p.id",
		"columns": "p.name, p.surname, p.secondSurname",
		"condition": strings.Join([]string{
			"u.access_gerencia = '1'",
			"ORDER BY u.people_id ASC",
			"LIMIT 1",
		}, " "),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return "", "", fmt.Errorf("no response getting manager user")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf(
			"error getting manager user. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	if strings.TrimSpace(string(bodyBytes)) == "" {
		return "", "", fmt.Errorf("empty response getting manager user")
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rows); err != nil {
		return "", "", err
	}

	if len(rows) == 0 {
		return "", "", fmt.Errorf("manager user with access_gerencia = 1 not found")
	}

	surname := asString(rows[0]["surname"])
	if secondSurname := asString(rows[0]["secondSurname"]); secondSurname != "" {
		surname = unescapeComma(fmt.Sprintf("%s %s", surname, secondSurname))
	}

	name := unescapeComma(asString(rows[0]["name"]))

	return name, surname, nil
}

func getBudgetAuditCertificateMeta(
	combinedID int,
	partID int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (BudgetAuditCertificateMeta, error) {
	meta := BudgetAuditCertificateMeta{}

	query := map[string]interface{}{
		"table": strings.Join([]string{
			"budget_requests br",
			"LEFT JOIN budget_parts bp ON bp.id_combined = br.id",
			"LEFT JOIN people requester ON br.people_id = requester.id",
			"LEFT JOIN projects pr ON bp.project_id = pr.id",
		}, " "),
		"columns": strings.Join([]string{
			"requester.name AS requester_name",
			"requester.surname AS requester_surname",
			"requester.secondSurname AS requester_second_surname",
			"requester.nif AS requester_nif",
			"br.id_intern",
			"br.creation_date",
			"bp.project_id AS project_code",
			"pr.name AS project_name",
			"bp.travel_area AS travel_area",
		}, ", "),
		"condition": fmt.Sprintf("br.id = %d AND bp.id = %d LIMIT 1", combinedID, partID),
	}

	jsonQuery, _ := json.Marshal(query)
	//fmt.Println("[AUDIT PDF META] query:", string(jsonQuery))

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return meta, fmt.Errorf("no response getting budget audit certificate meta")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return meta, err
	}

	//fmt.Println("[AUDIT PDF META] status:", resp.StatusCode)
	//fmt.Println("[AUDIT PDF META] body:", string(bodyBytes))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return meta, fmt.Errorf(
			"error getting budget audit certificate meta. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rows); err != nil {
		return meta, err
	}

	if len(rows) == 0 {
		return meta, fmt.Errorf("budget audit certificate meta not found for request %d part %d", combinedID, partID)
	}

	row := rows[0]

	meta.RequesterName = unescapeComma(asString(row["requester_name"]))
	meta.RequesterSurname = unescapeComma(asString(row["requester_surname"]))
	if secondSurname := asString(row["requester_second_surname"]); secondSurname != "" {
		meta.RequesterSurname = fmt.Sprintf("%s %s", meta.RequesterSurname, unescapeComma(secondSurname))
	}
	meta.RequesterDocument = unescapeComma(asString(row["requester_nif"]))
	meta.RequestCreationDate = asString(row["creation_date"])
	meta.ProjectCode = unescapeComma(asString(row["project_code"]))
	meta.ProjectName = unescapeComma(asString(row["project_name"]))
	meta.TravelArea = translateTravelArea(asString(row["travel_area"]))
	meta.IDIntern = asString(row["id_intern"])

	if strings.TrimSpace(meta.ProjectName) == "" || meta.ProjectName == "<nil>" {
		meta.ProjectName = meta.ProjectCode
	}

	managerName, managerSurname, err := getBudgetCertificateManager(apiKey, client, w)
	if err != nil {
		fmt.Println("[AUDIT PDF META] manager not found:", err)
	} else {
		meta.ManagerName = managerName
		meta.ManagerSurname = managerSurname
	}

	return meta, nil
}

func createBudgetAuditLog(
	combinedID int,
	partID *int,
	categoryID *int,
	action string,
	status string,
	summary string,
	actorPeopleID int,
	actorUsername string,
	actorName string,
	actorSurname string,
	details map[string]interface{},
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// Caso global:
	// Si no llega partID/categoryID, se crea un log por cada parte de la petición.
	if partID == nil && categoryID == nil {
		parts, err := getBudgetAuditPartsForRequest(combinedID, apiKey, client, w)
		if err != nil {
			return err
		}

		if len(parts) == 0 {
			return fmt.Errorf("no budget parts found for combinedID %d", combinedID)
		}

		for _, part := range parts {
			partIDValue := part.PartID
			categoryIDValue := part.CategoryID

			partDetails := filterBudgetAuditDetailsForPart(details, partIDValue)

			if err := createBudgetAuditLogForSinglePart(
				combinedID,
				&partIDValue,
				&categoryIDValue,
				action,
				status,
				summary,
				actorPeopleID,
				actorUsername,
				actorName,
				actorSurname,
				partDetails,
				createdAt,
				apiKey,
				client,
				w,
			); err != nil {
				return err
			}
		}

		return nil
	}

	// Caso normal: log de una única parte.
	if partID == nil || categoryID == nil {
		return fmt.Errorf("partID and categoryID must both be nil or both have value")
	}

	return createBudgetAuditLogForSinglePart(
		combinedID,
		partID,
		categoryID,
		action,
		status,
		summary,
		actorPeopleID,
		actorUsername,
		actorName,
		actorSurname,
		details,
		createdAt,
		apiKey,
		client,
		w,
	)
}

func createBudgetAuditLogForSinglePart(
	combinedID int,
	partID *int,
	categoryID *int,
	action string,
	status string,
	summary string,
	actorPeopleID int,
	actorUsername string,
	actorName string,
	actorSurname string,
	details map[string]interface{},
	createdAt string,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	payload := BudgetAuditPayload{
		CombinedID: combinedID,
		PartID:     partID,
		CategoryID: categoryID,
		Action:     action,
		Status:     status,
		Summary:    summary,
		CreatedAt:  createdAt,
		Actor: BudgetAuditActor{
			PeopleID: actorPeopleID,
			Username: actorUsername,
			Name:     actorName,
			Surname:  actorSurname,
		},
		Details: details,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creando payload audit: %w", err)
	}

	plainPayloadText := string(payloadBytes)

	encryptedPayloadText, err := encryptBudgetAuditPayload(plainPayloadText)
	if err != nil {
		return err
	}

	previousHash, err := getPreviousBudgetAuditHash(combinedID, partID, apiKey, client, w)
	if err != nil {
		return err
	}

	// Importante:
	// El hash se calcula sobre el payload cifrado que se guarda en BD.
	currentHash := calculateBudgetAuditHash(previousHash, encryptedPayloadText)

	partIDValue := "NULL"
	if partID != nil {
		partIDValue = strconv.Itoa(*partID)
	}

	categoryIDValue := "NULL"
	if categoryID != nil {
		categoryIDValue = strconv.Itoa(*categoryID)
	}

	insertPayload := map[string]interface{}{
		"table": "budget_audit_logs",
		"columns": strings.Join([]string{
			"combined_id",
			"part_id",
			"category_id",
			"previous_hash",
			"current_hash",
			"payload",
			"created_at",
		}, ", "),
		"value": fmt.Sprintf(
			"%d, %s, %s, %s, %s, %s, %s",
			combinedID,
			partIDValue,
			categoryIDValue,
			escapeComma(previousHash),
			escapeComma(currentHash),
			escapeComma(encryptedPayloadText),
			createdAt,
		),
	}

	jsonInsertPayload, _ := json.Marshal(insertPayload)
	resp := postReq(jsonInsertPayload, apiKey, client, w)
	if resp == nil {
		return fmt.Errorf("no response inserting budget audit log")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"error inserting budget audit log. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	return nil
}

func getBudgetAuditPartsForRequest(
	combinedID int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) ([]BudgetAuditPartRef, error) {
	query := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "id, category_id",
		"condition": fmt.Sprintf("id_combined = %d ORDER BY id ASC", combinedID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("no response getting budget audit parts")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"error getting budget audit parts. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	parts := make([]BudgetAuditPartRef, 0, len(rows))

	for _, row := range rows {
		partID := asInt(row["id"])
		categoryID := asInt(row["category_id"])

		if partID == 0 || categoryID == 0 {
			continue
		}

		parts = append(parts, BudgetAuditPartRef{
			PartID:     partID,
			CategoryID: categoryID,
		})
	}

	return parts, nil
}

func filterBudgetAuditDetailsForPart(
	details map[string]interface{},
	partID int,
) map[string]interface{} {
	if details == nil {
		return nil
	}

	// Copia profunda para no modificar el map original.
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return details
	}

	var cloned map[string]interface{}
	if err := json.Unmarshal(detailsBytes, &cloned); err != nil {
		return details
	}

	rawSnapshot, ok := cloned["approved_snapshot"]
	if !ok || rawSnapshot == nil {
		cloned["affected_part_id"] = partID
		return cloned
	}

	snapshot, ok := rawSnapshot.(map[string]interface{})
	if !ok {
		cloned["affected_part_id"] = partID
		return cloned
	}

	partSnapshot := findPartInAuditSnapshot(snapshot, partID)
	if partSnapshot == nil {
		cloned["affected_part_id"] = partID
		return cloned
	}

	// Mantiene los datos globales de la solicitud, pero deja solo la parte aprobada.
	snapshot["parts"] = []interface{}{partSnapshot}
	snapshot["part_id"] = partID

	cloned["approved_snapshot"] = snapshot
	cloned["affected_part_id"] = partID

	return cloned
}

func findPartInAuditSnapshot(
	snapshot map[string]interface{},
	partID int,
) map[string]interface{} {
	rawParts, ok := snapshot["parts"]
	if !ok || rawParts == nil {
		return nil
	}

	switch parts := rawParts.(type) {
	case []interface{}:
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]interface{})
			if !ok {
				continue
			}

			if asInt(part["part_id"]) == partID {
				return part
			}
		}

	case []map[string]interface{}:
		for _, part := range parts {
			if asInt(part["part_id"]) == partID {
				return part
			}
		}
	}

	return nil
}

func getPreviousBudgetAuditHash(
	combinedID int,
	partID *int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (string, error) {
	condition := fmt.Sprintf(
		"combined_id = %d AND part_id IS NULL ORDER BY id DESC LIMIT 1",
		combinedID,
	)

	if partID != nil {
		condition = fmt.Sprintf(
			"combined_id = %d AND part_id = %d ORDER BY id DESC LIMIT 1",
			combinedID,
			*partID,
		)
	}

	query := map[string]interface{}{
		"table":     "budget_audit_logs",
		"columns":   "current_hash",
		"condition": condition,
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return "", fmt.Errorf("no response getting previous budget audit hash")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"error getting previous budget audit hash. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return "", err
	}

	if len(rows) == 0 || rows[0]["current_hash"] == nil {
		return budgetAuditGenesisHash, nil
	}

	hash := strings.TrimSpace(asString(rows[0]["current_hash"]))
	if hash == "" {
		return budgetAuditGenesisHash, nil
	}

	return unescapeComma(hash), nil
}

func getLastBudgetPartID(
	combinedID int,
	categoryID int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (int, error) {
	query := map[string]interface{}{
		"table":     "budget_parts",
		"columns":   "MAX(id) AS max_id",
		"condition": fmt.Sprintf("id_combined = %d AND category_id = %d", combinedID, categoryID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return 0, fmt.Errorf("no response getting last budget part id")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf(
			"error getting last budget part id. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return 0, err
	}

	if len(rows) == 0 || rows[0]["max_id"] == nil {
		return 0, fmt.Errorf("budget part id not found")
	}

	return asInt(rows[0]["max_id"]), nil
}

func verifyBudgetAuditChain(
	combinedID int,
	partID *int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) BudgetAuditVerificationResult {
	condition := fmt.Sprintf(
		"combined_id = %d AND part_id IS NULL ORDER BY id ASC",
		combinedID,
	)

	if partID != nil {
		condition = fmt.Sprintf(
			"combined_id = %d AND part_id = %d ORDER BY id ASC",
			combinedID,
			*partID,
		)
	}

	query := map[string]interface{}{
		"table": "budget_audit_logs",
		"columns": strings.Join([]string{
			"id",
			"combined_id",
			"part_id",
			"category_id",
			"previous_hash",
			"current_hash",
			"payload",
			"created_at",
		}, ", "),
		"condition": condition,
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return BudgetAuditVerificationResult{
			OK:    false,
			Error: "No response from database",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return BudgetAuditVerificationResult{
			OK:    false,
			Error: fmt.Sprintf("DB error: %d - %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return BudgetAuditVerificationResult{
			OK:    false,
			Error: err.Error(),
		}
	}

	expectedPreviousHash := budgetAuditGenesisHash
	lastHash := ""

	for _, row := range rows {
		id := asInt(row["id"])

		previousHash := unescapeComma(asString(row["previous_hash"]))
		currentHash := unescapeComma(asString(row["current_hash"]))
		encryptedPayloadText := unescapeComma(asString(row["payload"]))

		if previousHash == "" {
			previousHash = budgetAuditGenesisHash
		}

		if previousHash != expectedPreviousHash {
			return BudgetAuditVerificationResult{
				OK:       false,
				BrokenAt: id,
				Error: fmt.Sprintf(
					"previous_hash mismatch. Expected %s but got %s",
					expectedPreviousHash,
					previousHash,
				),
			}
		}

		calculatedHash := calculateBudgetAuditHash(previousHash, encryptedPayloadText)

		if calculatedHash != currentHash {
			return BudgetAuditVerificationResult{
				OK:       false,
				BrokenAt: id,
				Error: fmt.Sprintf(
					"current_hash mismatch. Expected %s but got %s",
					calculatedHash,
					currentHash,
				),
			}
		}

		if calculatedHash != currentHash {
			return BudgetAuditVerificationResult{
				OK:       false,
				BrokenAt: id,
				Error: fmt.Sprintf(
					"current_hash mismatch. Expected %s but got %s",
					calculatedHash,
					currentHash,
				),
			}
		}

		expectedPreviousHash = currentHash
		lastHash = currentHash
	}

	return BudgetAuditVerificationResult{
		OK:       true,
		LastHash: lastHash,
	}
}

func handleVerifyBudgetAuditChain(
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
	r *http.Request,
) {
	combinedIDRaw := r.URL.Query().Get("idCombined")
	partIDRaw := r.URL.Query().Get("partId")

	if combinedIDRaw == "" {
		http.Error(w, "idCombined is required", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(combinedIDRaw)
	if err != nil {
		http.Error(w, "idCombined inválido", http.StatusBadRequest)
		return
	}

	var partID *int
	if partIDRaw != "" {
		parsedPartID, err := strconv.Atoi(partIDRaw)
		if err != nil {
			http.Error(w, "partId inválido", http.StatusBadRequest)
			return
		}
		partID = &parsedPartID
	}

	result := verifyBudgetAuditChain(combinedID, partID, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func auditBudgetPartCreation(
	combinedID int,
	categoryID int,
	category string,
	partID int,
	ipApproved bool,
	actorID int,
	actorUsername string,
	details map[string]interface{},
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) error {
	personInfo, err := getPersonInfoByID(actorID, apiKey, client, w)
	if err != nil {
		return err
	}

	err = createBudgetAuditLog(
		combinedID,
		&partID,
		&categoryID,
		"creat",
		"creat",
		fmt.Sprintf("%s %s crea una petició de tipus %s", personInfo.Name, personInfo.Surname, category),
		actorID,
		actorUsername,
		personInfo.Name,
		personInfo.Surname,
		details,
		apiKey,
		client,
		w,
	)
	if err != nil {
		return err
	}

	if ipApproved {
		err = createBudgetAuditLog(
			combinedID,
			&partID,
			&categoryID,
			"auto_aprovat",
			"aprovat",
			fmt.Sprintf("%s %s és responsable del projecte i s'aprova automàticament la seva petició de tipus %s ", personInfo.Name, personInfo.Surname, category),
			actorID,
			actorUsername,
			personInfo.Name,
			personInfo.Surname,
			map[string]interface{}{
				"category": category,
				"stage":    "responsable",
				"reason":   "El sol·licitant és responsable del projecte",
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

func getBudgetRequestAuditSnapshot(
	combinedID int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (map[string]interface{}, error) {

	query := map[string]interface{}{
		"table": `budget_requests br
			LEFT JOIN budget_parts bp ON bp.id_combined = br.id
			LEFT JOIN people p ON p.id = br.people_id
			LEFT JOIN projects pr ON bp.project_id = pr.id
			LEFT JOIN request_prices rp ON bp.other_price = rp.id`,
		"columns": strings.Join([]string{
			"br.id AS combined_id",
			"br.id_intern",
			"br.people_id",
			"p.name AS requester_name",
			"p.surname AS requester_surname",
			"p.secondSurname AS requester_second_surname",
			"p.nif AS requester_identity_document",
			"pr.name AS project_name",
			"pr.id AS project_code",
			"br.creation_date",
			"br.projects_response",
			"br.projects_response_date",
			"br.acc_or_it_response",
			"br.acc_or_it_response_date",
			"br.denied_comment",
			"br.canceled",
			"br.cancelation_motive",
			"br.prev_request",

			"bp.id AS part_id",
			"bp.category_id",
			"bp.project_id",
			"bp.ip_id",
			"bp.ip_response",
			"bp.ip_response_date",
			"bp.purpose",
			"bp.observations",
			"bp.travel_fromPlace",
			"bp.wherePlace",
			"bp.institution",
			"bp.fromDay",
			"bp.untilDay",
			"bp.equipment_category",
			"rp.price_range AS other_price",
			"bp.travel_area",
		}, ", "),
		"condition": fmt.Sprintf("br.id = %d ORDER BY bp.id ASC", combinedID),
	}

	jsonQuery, err := json.Marshal(query)
	if err != nil {
		fmt.Println("[AUDIT SNAPSHOT] ERROR marshaling query:", err)
		return nil, err
	}
	//fmt.Println("[AUDIT SNAPSHOT] Query payload:")
	//fmt.Println(string(jsonQuery))

	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		fmt.Println("[AUDIT SNAPSHOT] ERROR: resp is nil")
		return nil, fmt.Errorf("no response getting budget request audit snapshot")
	}
	defer resp.Body.Close()

	//fmt.Println("[AUDIT SNAPSHOT] Response status:", resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[AUDIT SNAPSHOT] ERROR reading body:", err)
		return nil, err
	}

	//fmt.Println("[AUDIT SNAPSHOT] Raw response body:")
	//fmt.Println(string(bodyBytes))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("[AUDIT SNAPSHOT] ERROR non-2xx response")
		return nil, fmt.Errorf(
			"error getting budget request audit snapshot. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rows); err != nil {
		fmt.Println("[AUDIT SNAPSHOT] ERROR decoding JSON:", err)
		fmt.Println("[AUDIT SNAPSHOT] Raw body was:")
		fmt.Println(string(bodyBytes))

		return nil, fmt.Errorf(
			"error decoding budget request audit snapshot JSON: %w. raw body: %s",
			err,
			string(bodyBytes),
		)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("budget request %d not found", combinedID)
	}

	first := rows[0]

	requester_surname := asString(first["requester_surname"])
	if secondSurname, ok := first["requester_second_surname"]; ok && asString(secondSurname) != "" {
		requester_surname += " " + asString(secondSurname)
	}

	snapshot := map[string]interface{}{
		"combined_id":                 asInt(first["combined_id"]),
		"id_intern":                   unescapeComma(asString(first["id_intern"])),
		"people_id":                   asInt(first["people_id"]),
		"requester_name":              unescapeComma(asString(first["requester_name"])),
		"requester_surname":           unescapeComma(requester_surname),
		"requester_identity_document": unescapeComma(asString(first["requester_identity_document"])),
		"project_name":                unescapeComma(asString(first["project_name"])),
		"project_code":                unescapeComma(asString(first["project_code"])),
		"creation_date":               asString(first["creation_date"]),
		"projects_response":           asInt(first["projects_response"]),
		"projects_response_date":      asString(first["projects_response_date"]),
		"acc_or_it_response":          asInt(first["acc_or_it_response"]),
		"acc_or_it_response_date":     asString(first["acc_or_it_response_date"]),
		"denied_comment":              unescapeComma(asString(first["denied_comment"])),
		"canceled":                    asInt(first["canceled"]),
		"cancelation_motive":          unescapeComma(asString(first["cancelation_motive"])),
		"prev_request":                asInt(first["prev_request"]),
		"parts":                       []map[string]interface{}{},
	}

	partsByID := make(map[int]map[string]interface{})
	partOrder := []int{}

	for _, row := range rows {
		partID := asInt(row["part_id"])
		if partID == 0 {
			continue
		}

		part, exists := partsByID[partID]
		if !exists {
			categoryID := asInt(row["category_id"])

			part = map[string]interface{}{
				"part_id":            partID,
				"category_id":        categoryID,
				"category":           CATEGORY_ID_TO_KEY[categoryID],
				"project_id":         unescapeComma(asString(row["project_id"])),
				"project_name":       unescapeComma(asString(row["project_name"])),
				"ip_id":              asInt(row["ip_id"]),
				"ip_response":        asInt(row["ip_response"]),
				"ip_response_date":   asString(row["ip_response_date"]),
				"purpose":            unescapeComma(asString(row["purpose"])),
				"observations":       unescapeComma(asString(row["observations"])),
				"travel_from_place":  unescapeComma(asString(row["travel_fromPlace"])),
				"where_place":        unescapeComma(asString(row["wherePlace"])),
				"institution":        unescapeComma(asString(row["institution"])),
				"from_day":           asString(row["fromDay"]),
				"until_day":          asString(row["untilDay"]),
				"equipment_category": unescapeComma(asString(row["equipment_category"])),
				"other_price":        unescapeComma(asString(row["other_price"])),
				"travel_area":        unescapeComma(asString(row["travel_area"])),
			}

			partsByID[partID] = part
			partOrder = append(partOrder, partID)
		}
	}

	parts := []map[string]interface{}{}
	for _, partID := range partOrder {
		parts = append(parts, partsByID[partID])
	}

	snapshot["parts"] = parts

	return snapshot, nil
}

func getBudgetAuditRowsForPart(
	combinedID int,
	partID int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) ([]BudgetAuditRow, error) {
	query := map[string]interface{}{
		"table": "budget_audit_logs",
		"columns": strings.Join([]string{
			"id",
			"combined_id",
			"part_id",
			"category_id",
			"previous_hash",
			"current_hash",
			"payload",
			"created_at",
		}, ", "),
		"condition": fmt.Sprintf(
			"combined_id = %d AND part_id = %d ORDER BY id ASC",
			combinedID,
			partID,
		),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("no response getting budget audit rows")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"error getting budget audit rows. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rawRows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawRows); err != nil {
		return nil, err
	}

	rows := make([]BudgetAuditRow, 0, len(rawRows))

	for _, raw := range rawRows {
		partIDValue := asInt(raw["part_id"])
		categoryIDValue := asInt(raw["category_id"])

		rowPartID := partIDValue
		rowCategoryID := categoryIDValue

		rows = append(rows, BudgetAuditRow{
			ID:           asInt(raw["id"]),
			CombinedID:   asInt(raw["combined_id"]),
			PartID:       &rowPartID,
			CategoryID:   &rowCategoryID,
			PreviousHash: unescapeComma(asString(raw["previous_hash"])),
			CurrentHash:  unescapeComma(asString(raw["current_hash"])),
			Payload:      unescapeComma(asString(raw["payload"])),
			CreatedAt:    asString(raw["created_at"]),
		})
	}

	return rows, nil
}

// ------------------ EXPORT PDF ------------------

func decodeBudgetAuditPayload(row BudgetAuditRow) (BudgetAuditPayload, string, error) {
	payloadText := strings.TrimSpace(row.Payload)

	var payload BudgetAuditPayload

	// Compatibilidad con logs antiguos sin cifrar.
	if strings.HasPrefix(payloadText, "{") {
		if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
			return payload, "", err
		}

		return payload, payloadText, nil
	}

	plainPayloadText, err := decryptBudgetAuditPayload(payloadText)
	if err != nil {
		return payload, "", err
	}

	if err := json.Unmarshal([]byte(plainPayloadText), &payload); err != nil {
		return payload, "", err
	}

	return payload, plainPayloadText, nil
}

func handleExportBudgetAuditPDF(
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
	r *http.Request,
) {
	combinedIDRaw := r.URL.Query().Get("idCombined")
	partIDRaw := r.URL.Query().Get("partId")

	if combinedIDRaw == "" || partIDRaw == "" {
		http.Error(w, "idCombined and partId are required", http.StatusBadRequest)
		return
	}

	combinedID, err := strconv.Atoi(combinedIDRaw)
	if err != nil {
		http.Error(w, "idCombined inválido", http.StatusBadRequest)
		return
	}

	partID, err := strconv.Atoi(partIDRaw)
	if err != nil {
		http.Error(w, "partId inválido", http.StatusBadRequest)
		return
	}

	verification := verifyBudgetAuditChain(combinedID, &partID, apiKey, client, w)
	if !verification.OK {
		http.Error(
			w,
			"Audit chain is not valid: "+verification.Error,
			http.StatusConflict,
		)
		return
	}

	rows, err := getBudgetAuditRowsForPart(combinedID, partID, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf(
			"Error getting audit rows for request %d part %d: %v",
			combinedID,
			partID,
			err,
		), 1, apiKey, client, w)

		http.Error(w, "Error obteniendo logs de auditoría", http.StatusInternalServerError)
		return
	}

	if len(rows) == 0 {
		http.Error(w, "No audit logs found for this part", http.StatusNotFound)
		return
	}

	payloads := make([]BudgetAuditPayload, 0, len(rows))

	for _, row := range rows {
		payload, _, err := decodeBudgetAuditPayload(row)
		if err != nil {
			createLog(fmt.Sprintf(
				"Error decoding audit payload for request %d part %d log %d: %v",
				combinedID,
				partID,
				row.ID,
				err,
			), 1, apiKey, client, w)

			http.Error(w, "Error descifrando logs de auditoría", http.StatusInternalServerError)
			return
		}

		payloads = append(payloads, payload)
	}

	meta, err := getBudgetAuditCertificateMeta(combinedID, partID, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf(
			"Error getting audit certificate meta for request %d part %d: %v",
			combinedID,
			partID,
			err,
		), 1, apiKey, client, w)

		http.Error(w, "Error obteniendo datos del certificado", http.StatusInternalServerError)
		return
	}

	pdfResult, err := buildBudgetAuditCertificatePDF(rows, payloads, verification, meta, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf(
			"Error creating audit PDF for request %d part %d: %v",
			combinedID,
			partID,
			err,
		), 1, apiKey, client, w)

		http.Error(w, "Error generando PDF de auditoría", http.StatusInternalServerError)
		return
	}

	pdfBytes, err := signBudgetAuditPDFWithVisibleCertificate(
		pdfResult.Bytes,
		pdfResult.SignaturePosition,
	)
	if err != nil {
		createLog(fmt.Sprintf(
			"Error signing audit PDF for request %d part %d: %v",
			combinedID,
			partID,
			err,
		), 1, apiKey, client, w)

		message := "Error firmando PDF con certificado de la aplicación"
		if mC.Development {
			message = message + ": " + err.Error()
		}

		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("budget-log-certificate-request-%d-part-%d.pdf", combinedID, partID)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(pdfBytes)
}

func buildBudgetAuditCertificatePDF(
	rows []BudgetAuditRow,
	payloads []BudgetAuditPayload,
	verification BudgetAuditVerificationResult,
	meta BudgetAuditCertificateMeta,
	apiKey string, client *http.Client,
	w http.ResponseWriter,
) (BudgetAuditPDFBuildResult, error) {
	if len(rows) == 0 || len(payloads) == 0 {
		return BudgetAuditPDFBuildResult{}, fmt.Errorf("no audit data to export")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Certificat d'auditoria de petició pressupostària", false)
	pdf.SetAuthor("CRMIntratools", false)
	pdf.SetCreator("CRMIntratools - Mòdul de pressupostació", false)
	pdf.SetMargins(18, 15, 18)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AliasNbPages("")

	if strings.TrimSpace(mC.PrivateKeyPassphrase) != "" {
		pdf.SetProtection(
			fpdf.CnProtectPrint,
			"",
			mC.PrivateKeyPassphrase,
		)
	}

	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "", 8)
		pdf.CellFormat(0, 10, tr(fmt.Sprintf("Pàgina %d/{nb}", pdf.PageNo())), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()

	logoPath := "module8workers/assets/crmlogo.jfif"
	if _, err := os.Stat(logoPath); err == nil {
		pdf.ImageOptions(
			logoPath,
			18, 12,
			38, 0,
			false,
			fpdf.ImageOptions{
				ImageType: "jpg",
				ReadDpi:   true,
			},
			0,
			"",
		)
	}

	pdf.Ln(28)

	pdf.SetFont("Arial", "B", 16)
	pdf.MultiCell(0, 8, tr("CERTIFICAT D'AUDITORIA DE PETICIÓ PRESSUPOSTÀRIA"), "", "C", false)
	pdf.Ln(8)

	writeLegalIntro(pdf, tr, meta)
	writeRequestSummarySection(pdf, tr, payloads, meta, apiKey, client, w)
	writeAuditEventsSection(pdf, tr, rows, payloads)
	signaturePosition := writeSignatureBlock(pdf, tr, meta)
	writeEncryptionStatement(pdf, tr, verification)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return BudgetAuditPDFBuildResult{}, err
	}

	return BudgetAuditPDFBuildResult{
		Bytes:             buf.Bytes(),
		SignaturePosition: signaturePosition,
	}, nil
}

func writePDFKeyValue(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	key string,
	value string,
) {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		value = "-"
	}

	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(38, 5, tr(key+":"), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 5, tr(value), "", "L", false)
}

func writeWrappedText(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	text string,
) {
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			pdf.Ln(3)
			continue
		}

		pdf.MultiCell(0, 4, tr(line), "", "L", false)
	}
}

func getSnapshotFromPayload(payload BudgetAuditPayload) map[string]interface{} {
	if payload.Details == nil {
		return nil
	}

	rawSnapshot, ok := payload.Details["approved_snapshot"]
	if !ok || rawSnapshot == nil {
		return nil
	}

	snapshot, ok := rawSnapshot.(map[string]interface{})
	if !ok {
		return nil
	}

	return snapshot
}

func writeLegalIntro(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	meta BudgetAuditCertificateMeta,
) {
	requesterName := cleanPDFValue(meta.RequesterName)
	requesterSurname := cleanPDFValue(meta.RequesterSurname)
	identityDocument := cleanPDFValue(meta.RequesterDocument)
	projectName := cleanPDFValue(meta.ProjectName)
	//projectCode := cleanPDFValue(meta.ProjectCode)
	creationDate := formatCatalanDate(meta.RequestCreationDate)
	idIntern := cleanPDFValue(meta.IDIntern)

	fullName := strings.TrimSpace(requesterName + " " + requesterSurname)
	if strings.TrimSpace(fullName) == "" || fullName == "- -" {
		fullName = "la persona investigadora sol·licitant"
	}

	pdf.SetFont("Arial", "", 10)

	writeInlineText(pdf, tr, "Es fa constar que ")
	writeHighlightedInlineValue(pdf, tr, fullName)
	writeInlineText(pdf, tr, ", amb document d'identitat ")
	writeHighlightedInlineValue(pdf, tr, identityDocument)
	writeInlineText(pdf, tr, ", va formular una petició pressupostària vinculada al projecte ")
	writeHighlightedInlineValue(pdf, tr, truncatePDFInlineValue(projectName, 90))
	writeInlineText(pdf, tr, ", amb codi ")
	writeHighlightedInlineValue(pdf, tr, idIntern)
	writeInlineText(pdf, tr, ", en data ")
	writeHighlightedInlineValue(pdf, tr, creationDate)
	writeInlineText(pdf, tr, ".")

	pdf.Ln(8)

	pdf.SetFont("Arial", "", 10)
	text := "Aquest certificat recull la traçabilitat dels actes registrats en el sistema de pressupostació i acredita, d'acord amb la cadena d'integritat associada als registres d'auditoria, la successió d'esdeveniments que consten vinculats a aquesta petició."

	pdf.MultiCell(0, 5.5, tr(text), "", "J", false)
	pdf.Ln(7)
}

func writeRequestSummarySection(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	payloads []BudgetAuditPayload,
	meta BudgetAuditCertificateMeta,
	apiKey string, client *http.Client,
	w http.ResponseWriter,
) {
	pdf.SetFont("Arial", "B", 12)
	pdf.MultiCell(0, 7, tr("Resum de la petició"), "", "L", false)
	pdf.Ln(1)

	var summary map[string]interface{}

	// Preferimos el snapshot final porque contiene el estado aprobado.
	for i := len(payloads) - 1; i >= 0; i-- {
		snapshot := getSnapshotFromPayload(payloads[i])
		if snapshot != nil {
			summary = snapshot
			break
		}
	}

	// Si no hay snapshot, buscamos una parte afectada en los details.
	if summary == nil {
		for i := len(payloads) - 1; i >= 0; i-- {
			if payloads[i].Details == nil {
				continue
			}

			affectedPartID := asInt(payloads[i].Details["affected_part_id"])
			if affectedPartID == 0 && payloads[i].PartID != nil {
				affectedPartID = *payloads[i].PartID
			}

			if affectedPartID == 0 {
				continue
			}

			partSummary, err := getBudgetPartSummaryByID(affectedPartID, apiKey, client, w)
			if err == nil && partSummary != nil {
				// Conservamos también datos del rechazo.
				if motive := asString(payloads[i].Details["motive"]); motive != "" {
					partSummary["rejection_motive"] = motive
				}
				if stage := asString(payloads[i].Details["stage"]); stage != "" {
					partSummary["rejection_stage"] = stage
				}

				summary = partSummary
				break
			}
		}
	}

	// Último fallback.
	if summary == nil && len(payloads) > 0 && payloads[0].Details != nil {
		summary = payloads[0].Details
	}

	if summary == nil {
		pdf.SetFont("Arial", "", 10)
		pdf.MultiCell(0, 5, tr("No consten dades de resum de la petició."), "", "L", false)
		pdf.Ln(4)
		return
	}

	parts := extractSnapshotParts(summary)

	if len(parts) == 0 {
		writeBudgetPartSummary(pdf, tr, summary, meta, apiKey, client, w)
		pdf.Ln(3)
		return
	}

	for _, part := range parts {
		writeBudgetPartSummary(pdf, tr, part, meta, apiKey, client, w)
		pdf.Ln(3)
	}
}

func getBudgetPartSummaryByID(
	partID int,
	apiKey string,
	client *http.Client,
	w http.ResponseWriter,
) (map[string]interface{}, error) {
	query := map[string]interface{}{
		"table": `budget_parts bp
			LEFT JOIN projects pr ON bp.project_id = pr.id
			LEFT JOIN request_prices rp ON bp.other_price = rp.id`,
		"columns": strings.Join([]string{
			"bp.id AS part_id",
			"bp.category_id",
			"bp.project_id",
			"pr.name AS project_name",
			"pr.id AS project_code",
			"bp.ip_id",
			"bp.ip_response",
			"bp.ip_response_date",
			"bp.purpose",
			"bp.observations",
			"bp.travel_fromPlace",
			"bp.wherePlace",
			"bp.institution",
			"bp.fromDay",
			"bp.untilDay",
			"bp.equipment_category",
			"rp.price_range AS other_price",
			"bp.travel_area",
		}, ", "),
		"condition": fmt.Sprintf("bp.id = %d LIMIT 1", partID),
	}

	jsonQuery, _ := json.Marshal(query)
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		return nil, fmt.Errorf("no response getting budget part summary")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"error getting budget part summary. status: %d, body: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("budget part %d not found", partID)
	}

	row := rows[0]
	categoryID := asInt(row["category_id"])

	return map[string]interface{}{
		"part_id":            asInt(row["part_id"]),
		"affected_part_id":   asInt(row["part_id"]),
		"category_id":        categoryID,
		"category":           CATEGORY_ID_TO_KEY[categoryID],
		"project_id":         unescapeComma(asString(row["project_id"])),
		"project_code":       unescapeComma(asString(row["project_code"])),
		"project_name":       unescapeComma(asString(row["project_name"])),
		"ip_id":              asInt(row["ip_id"]),
		"ip_response":        asInt(row["ip_response"]),
		"ip_response_date":   asString(row["ip_response_date"]),
		"purpose":            unescapeComma(asString(row["purpose"])),
		"observations":       unescapeComma(asString(row["observations"])),
		"travel_from_place":  unescapeComma(asString(row["travel_fromPlace"])),
		"where_place":        unescapeComma(asString(row["wherePlace"])),
		"institution":        unescapeComma(asString(row["institution"])),
		"from_day":           asString(row["fromDay"]),
		"until_day":          asString(row["untilDay"]),
		"equipment_category": unescapeComma(asString(row["equipment_category"])),
		"other_price":        unescapeComma(asString(row["other_price"])),
		"travel_area":        unescapeComma(asString(row["travel_area"])),
	}, nil
}

func cleanOptionalPDFString(value string) string {
	value = strings.TrimSpace(value)

	if value == "" || value == "<nil>" || value == "-" {
		return ""
	}

	return value
}

func writeBudgetPartSummary(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
	meta BudgetAuditCertificateMeta,
	apiKey string, client *http.Client,
	w http.ResponseWriter,
) {
	categoryKey := getBudgetPartCategoryKey(part, apiKey, client, w)
	categoryLabel := translateBudgetCategory(categoryKey)

	pdf.SetFont("Arial", "B", 10)
	pdf.MultiCell(0, 6, tr(categoryLabel), "", "L", false)

	writeCommonBudgetPartFields(pdf, tr, part, meta)

	fmt.Printf("DEBUG: Budget part category key: %s\n", categoryKey)

	switch categoryKey {
	case "travel":
		writeTravelBudgetPartFields(pdf, tr, part, meta)

	case "registration":
		writeRegistrationBudgetPartFields(pdf, tr, part)

	case "accommodation":
		writeAccommodationBudgetPartFields(pdf, tr, part)

	case "equipment":
		writeEquipmentBudgetPartFields(pdf, tr, part)

	case "other":
		writeOtherBudgetPartFields(pdf, tr, part)

	default:
		writeGenericBudgetPartFields(pdf, tr, part, meta)
	}
}
func formalizeAuditSummary(payload BudgetAuditPayload) string {
	summary := strings.TrimSpace(payload.Summary)

	if payload.Details == nil {
		return formalizeSummary(summary)
	}

	motive := strings.TrimSpace(firstNonEmpty(
		asString(payload.Details["motive"]),
		asString(payload.Details["reason"]),
		asString(payload.Details["rejection_motive"]),
		asString(payload.Details["denied_comment"]),
	))

	stage := strings.TrimSpace(asString(payload.Details["stage"]))

	isRejected := payload.Status == "rebutjat" ||
		strings.HasPrefix(payload.Action, "rebutjat") ||
		strings.Contains(strings.ToLower(payload.Action), "reject")

	if isRejected {
		if summary == "" || summary == "-" {
			summary = "La petició ha estat rebutjada"
		}

		if stage != "" && stage != "-" && !strings.Contains(strings.ToLower(summary), strings.ToLower(stage)) {
			summary += fmt.Sprintf(" en la fase %s", translateAuditStage(stage))
		}

		if motive != "" && motive != "-" && !strings.Contains(strings.ToLower(summary), strings.ToLower(motive)) {
			summary += fmt.Sprintf(". Motiu del rebuig: %s", motive)
		}
	}

	return formalizeSummary(summary)
}

func translateAuditStage(stage string) string {
	stage = strings.TrimSpace(strings.ToLower(stage))

	switch stage {
	case "projects", "projectes":
		return "Projectes"
	case "responsable", "ip":
		return "Responsable del projecte"
	case "accounting", "comptabilitat":
		return "Comptabilitat"
	case "it":
		return "IT"
	case "gerencia", "gerència":
		return "Gerència"
	default:
		return stage
	}
}

func getBudgetPartCategoryKey(part map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) string {
	category := strings.TrimSpace(strings.ToLower(asString(part["category"])))

	fmt.Print("Debug raw ", category, part)
	if category == "" {
		lookupPartID := asInt(part["part_id"])
		if lookupPartID == 0 {
			lookupPartID = asInt(part["affected_part_id"])
		}
		queryCategory := map[string]interface{}{
			"table":     "budget_parts",
			"columns":   "*",
			"condition": fmt.Sprintf("id = %d", lookupPartID),
		}
		jsonQueryCategory, _ := json.Marshal(queryCategory)
		respCategory := getReq(jsonQueryCategory, apiKey, client, w)
		if respCategory != nil && respCategory.StatusCode == http.StatusOK {
			defer respCategory.Body.Close()

			var rowsCategory []map[string]interface{}
			if err := json.NewDecoder(respCategory.Body).Decode(&rowsCategory); err == nil && len(rowsCategory) > 0 {
				categoryID := asInt(rowsCategory[0]["category_id"])
				category = CATEGORY_ID_TO_KEY[categoryID]
			}
		}
	}
	if category != "" && category != "<nil>" {
		switch category {
		case "travel", "viatge":
			return "travel"
		case "registration", "inscripcio", "inscripció":
			return "registration"
		case "accommodation", "allotjament":
			return "accommodation"
		case "equipment", "equipament":
			return "equipment"
		case "other", "altres":
			return "other"
		}
	}

	categoryID := asInt(part["category_id"])

	switch categoryID {
	case 1:
		return "travel"
	case 2:
		return "registration"
	case 3:
		return "accommodation"
	case 4:
		return "equipment"
	case 5:
		return "other"
	default:
		return category
	}
}

func writeCommonBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
	meta BudgetAuditCertificateMeta,
) {
	writePDFKeyValueCA(
		pdf,
		tr,
		"Projecte",
		cleanPDFValue(firstNonEmpty(
			asString(part["project_name"]),
			meta.ProjectName,
			asString(part["project_id"]),
			meta.ProjectCode,
		)),
	)

	writePDFKeyValueCA(
		pdf,
		tr,
		"Codi del projecte",
		cleanPDFValue(firstNonEmpty(
			asString(part["project_code"]),
			asString(part["project_id"]),
			meta.ProjectCode,
		)),
	)
}

func writeTravelBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
	meta BudgetAuditCertificateMeta,
) {

	//fmt.Printf("DEBUG: part", part)
	writePDFKeyValueCA(pdf, tr, "Finalitat", cleanPDFValue(asString(part["purpose"])))
	writePDFKeyValueCA(pdf, tr, "Origen", cleanPDFValue(firstNonEmpty(
		asString(part["travel_from_place"]),
		asString(part["from_place"]),
	)))
	writePDFKeyValueCA(pdf, tr, "Destinació", cleanPDFValue(asString(part["where_place"])))
	writePDFKeyValueCA(pdf, tr, "Institució de destí", cleanPDFValue(asString(part["institution"])))
	writePDFKeyValueCA(pdf, tr, "Data d'inici", formatCatalanDate(asString(part["from_day"])))
	writePDFKeyValueCA(pdf, tr, "Data de finalització", formatCatalanDate(asString(part["until_day"])))

	writePDFKeyValueCA(
		pdf,
		tr,
		"Àmbit del viatge",
		translateTravelArea(firstNonEmpty(
			asString(part["travel_area"]),
			meta.TravelArea,
		)),
	)
}

func writeRegistrationBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
) {
	writePDFKeyValueCA(pdf, tr, "Finalitat", cleanPDFValue(asString(part["purpose"])))
	writePDFKeyValueCA(pdf, tr, "Data d'inici", formatCatalanDate(asString(part["from_day"])))
	writePDFKeyValueCA(pdf, tr, "Data de finalització", formatCatalanDate(asString(part["until_day"])))

}

func writeAccommodationBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
) {
	writePDFKeyValueCA(pdf, tr, "Finalitat", cleanPDFValue(asString(part["purpose"])))
	writePDFKeyValueCA(pdf, tr, "Lloc d'allotjament", cleanPDFValue(asString(part["where_place"])))
	writePDFKeyValueCA(pdf, tr, "Data d'entrada", formatCatalanDate(asString(part["from_day"])))
	writePDFKeyValueCA(pdf, tr, "Data de sortida", formatCatalanDate(asString(part["until_day"])))
}

func writeEquipmentBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
) {

	if asString(part["equipment_category"]) == "other" {
		writePDFKeyValueCA(pdf, tr, "Categoria de l'equipament", cleanPDFValue(asString(part["purpose"])))
	} else {
		switch asString(part["equipment_category"]) {
		case "laptop":
			writePDFKeyValueCA(pdf, tr, "Categoria de l'equipament", "Portàtil")
		case "screen":
			writePDFKeyValueCA(pdf, tr, "Categoria de l'equipament", "Monitor")
		case "PC":
			writePDFKeyValueCA(pdf, tr, "Categoria de l'equipament", "Ordinador de sobretaula")
		case "Peripheral":
			writePDFKeyValueCA(pdf, tr, "Categoria de l'equipament", "Equipament informàtic perifèric")
		default:
			writePDFKeyValueCA(pdf, tr, "Categoria de l'equipament", cleanPDFValue(asString(part["equipment_category"])))
		}
	}
	writePDFKeyValueCA(pdf, tr, "Descripció", cleanPDFValue(asString(part["observations"])))
	if asString(part["price_range"]) != "" {

		switch asString(part["price_range"]) {
		case "1", "0 - 5k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Menys de 5.000€")
		case "2", "5k - 15k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Entre 5.000€ i 15.000€")
		case "3", "15k - 50k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Entre 15.000€ i 50.000€")
		case "4", "+50k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Més de 50.000€")
		default:
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", cleanPDFValue(asString(part["other_price"])))
		}
	}
}

func writeOtherBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
) {
	writePDFKeyValueCA(pdf, tr, "Finalitat", cleanPDFValue(asString(part["observations"])))
	if strings.TrimSpace(asString(part["other_price"])) != "" {

		switch asString(part["other_price"]) {
		case "1", "0 - 5k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Menys de 5.000€")
		case "2", "5k - 15k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Entre 5.000€ i 15.000€")
		case "3", "15k - 50k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Entre 15.000€ i 50.000€")
		case "4", "+50k":
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", "Més de 50.000€")
		default:
			writePDFKeyValueCA(pdf, tr, "Referència econòmica", cleanPDFValue(asString(part["other_price"])))
		}
	}

}

func writeGenericBudgetPartFields(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	part map[string]interface{},
	meta BudgetAuditCertificateMeta,
) {
	writePDFKeyValueCA(pdf, tr, "Institució", cleanPDFValue(asString(part["institution"])))
	writePDFKeyValueCA(pdf, tr, "Lloc", cleanPDFValue(asString(part["where_place"])))
	writePDFKeyValueCA(pdf, tr, "Data d'inici", formatCatalanDate(asString(part["from_day"])))
	writePDFKeyValueCA(pdf, tr, "Data de finalització", formatCatalanDate(asString(part["until_day"])))

	if strings.TrimSpace(asString(part["travel_area"])) != "" || strings.TrimSpace(meta.TravelArea) != "" {
		writePDFKeyValueCA(
			pdf,
			tr,
			"Àmbit",
			translateTravelArea(firstNonEmpty(
				asString(part["travel_area"]),
				meta.TravelArea,
			)),
		)
	}

	if strings.TrimSpace(asString(part["equipment_category"])) != "" {
		writePDFKeyValueCA(pdf, tr, "Categoria d'equipament", cleanPDFValue(asString(part["equipment_category"])))
	}

	if strings.TrimSpace(asString(part["other_price"])) != "" {
		writePDFKeyValueCA(pdf, tr, "Import o referència econòmica", cleanPDFValue(asString(part["other_price"])))
	}
}

func extractSnapshotParts(snapshot map[string]interface{}) []map[string]interface{} {
	rawParts, ok := snapshot["parts"]
	if !ok || rawParts == nil {
		return nil
	}

	result := []map[string]interface{}{}

	switch parts := rawParts.(type) {
	case []interface{}:
		for _, raw := range parts {
			if part, ok := raw.(map[string]interface{}); ok {
				result = append(result, part)
			}
		}
	case []map[string]interface{}:
		result = append(result, parts...)
	}

	return result
}

func writeAuditEventsSection(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	rows []BudgetAuditRow,
	payloads []BudgetAuditPayload,
) {
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 12)
	pdf.MultiCell(0, 7, tr("Relació cronològica d'actuacions registrades"), "", "L", false)

	pdf.SetDrawColor(180, 180, 180)
	pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
	pdf.Ln(5)

	for i, row := range rows {
		payload := payloads[i]

		// Salto preventivo para evitar que una actuación empiece cortada al final de página.
		if pdf.GetY() > 245 {
			pdf.AddPage()
		}

		writeAuditEventBlock(pdf, tr, i+1, row, payload)
	}
}

func writeAuditEventBlock(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	stepNumber int,
	row BudgetAuditRow,
	payload BudgetAuditPayload,
) {
	startY := pdf.GetY()
	leftX := 18.0
	numberBoxWidth := 9.0
	contentX := leftX + numberBoxWidth + 4

	// Línea vertical discreta de continuidad cronológica.
	pdf.SetDrawColor(210, 210, 210)
	pdf.Line(leftX+4.5, startY+1, leftX+4.5, startY+34)

	// Número del paso.
	pdf.SetFillColor(245, 245, 245)
	pdf.SetDrawColor(185, 185, 185)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(leftX, startY)
	pdf.CellFormat(numberBoxWidth, 7, tr(fmt.Sprintf("%02d", stepNumber)), "1", 0, "C", true, 0, "")

	// Título.
	pdf.SetXY(contentX, startY)
	pdf.SetFont("Arial", "B", 10)
	pdf.MultiCell(0, 5.5, tr(translateAuditAction(payload.Action)), "", "L", false)

	// Línea secundaria: fecha + estado.
	pdf.SetX(contentX)
	pdf.SetFont("Arial", "", 8.5)
	secondaryLine := fmt.Sprintf(
		"%s · Estat: %s",
		formatCatalanDateTime(payload.CreatedAt),
		translateAuditStatus(payload.Status),
	)
	pdf.MultiCell(0, 4.5, tr(secondaryLine), "", "L", false)

	pdf.Ln(1)

	// Contenido principal con más separación.
	pdf.SetX(contentX)
	writePDFKeyValueCAAt(pdf, tr, contentX, "Persona actuant", formatActorName(payload.Actor))

	pdf.SetX(contentX)
	writePDFKeyValueCAAt(pdf, tr, contentX, "Descripció", formalizeAuditSummary(payload))

	pdf.Ln(1.5)

	// Nota técnica discreta.
	pdf.SetX(contentX)
	pdf.SetFont("Arial", "I", 7.2)
	pdf.SetTextColor(90, 90, 90)

	technicalNote := fmt.Sprintf(
		"Registre núm. %d\nEmpremta anterior: %s\nEmpremta del registre: %s",
		row.ID,
		row.PreviousHash,
		row.CurrentHash,
	)

	pdf.MultiCell(0, 3.8, tr(technicalNote), "", "L", false)
	pdf.SetTextColor(0, 0, 0)

	pdf.Ln(6)
}

func writePDFKeyValueCAAt(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	x float64,
	key string,
	value string,
) {
	value = cleanPDFValue(value)

	pdf.SetX(x)
	pdf.SetFont("Arial", "B", 8.7)
	pdf.CellFormat(34, 4.8, tr(key+":"), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 8.7)
	pdf.MultiCell(0, 4.8, tr(value), "", "L", false)
}

func translateAuditAction(action string) string {
	switch action {
	case "creat":
		return "Creació de la petició"
	case "aprovat_responsable":
		return "Aprovació pel responsable"
	case "auto_aprovat":
		return "Aprovació automàtica de la persona responsable"
	case "aprovat_projectes":
		return "Aprovació per l'àrea de Projectes"
	case "aprovat_gerencia_projectes":
		return "Aprovació per Gerència en fase de Projectes"
	case "aprovat_comptabilitat":
		return "Aprovació i confirmació per Comptabilitat"
	case "aprovat_it":
		return "Aprovació i confirmació per l'àrea d'IT"
	case "rebutjat_responsable":
		return "Rebuig per la persona responsable del projecte"
	case "rebutjat_projectes":
		return "Rebuig per l'àrea de Projectes"
	case "rebutjat_comptabilitat":
		return "Rebuig per Comptabilitat"
	case "rebutjat_it":
		return "Rebuig per l'àrea d'IT"
	case "cancelat":
		return "Cancel·lació de la petició"
	default:
		return action
	}
}

func translateAuditStatus(status string) string {
	switch status {
	case "creat":
		return "Creat"
	case "aprovat":
		return "Aprovat"
	case "rebutjat":
		return "Rebutjat"
	case "cancelat":
		return "Cancel·lat"
	default:
		return status
	}
}

func translateBudgetCategory(category string) string {
	switch category {
	case "travel", "viatge":
		return "Viatge"
	case "registration", "inscripcio", "inscripció":
		return "Inscripció"
	case "accommodation", "allotjament":
		return "Allotjament"
	case "equipment", "equipament":
		return "Equipament"
	case "other", "altres":
		return "Altres despeses"
	default:
		return category
	}
}

func writeEncryptionStatement(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	verification BudgetAuditVerificationResult,
) {
	pdf.Ln(10)

	pdf.SetDrawColor(210, 210, 210)
	pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
	pdf.Ln(4)

	pdf.SetFont("Arial", "I", 7.5)

	text := "Nota sobre la garantia d'integritat: els registres d'auditoria associats a aquesta petició han estat protegits mitjançant una cadena criptogràfica d'empremtes digitals. Cada actuació registrada a la traça d’auditoria es converteix primer en un payload JSON estructurat que conté els identificadors de la sol·licitud, l’acció realitzada, l’estat, la data i hora, la informació de l’actor i els detalls rellevants. Abans d’emmagatzemar-se, aquest payload es xifra mitjançant GPG amb la clau pública del sistema. El resultat xifrat es codifica posteriorment en Base64 per poder-se desar de manera segura com a text a la base de dades. Per garantir-ne la integritat, cada registre d’auditoria forma part d’un sistema de verificació encadenat. Cada registre desa dos hashes: el hash del registre anterior i el seu propi hash actual. El hash actual es calcula amb SHA-256 a partir del payload xifrat i del hash anterior. El primer registre parteix del valor inicial fix GENESIS. Abans de generar aquest certificat, el sistema verifica tota la cadena des del primer registre fins a l’últim. Per a cada entrada, comprova que el hash anterior desat coincideixi amb el hash del registre precedent i recalcula el hash actual per confirmar que coincideix amb el valor emmagatzemat. Això significa que qualsevol modificació del contingut xifrat, de l’ordre dels registres, d’un hash anterior o d’un hash actual trencaria la cadena i seria detectada durant la verificació. Per tant, una verificació correcta confirma que la traça d’auditoria utilitzada per generar aquest certificat no ha estat alterada des que es va registrar."

	pdf.MultiCell(0, 4, tr(text), "", "J", false)
	pdf.Ln(1.5)

	text2 := fmt.Sprintf(
		"Abans de generar aquest certificat, el sistema ha verificat la cadena vinculada a la part certificada amb resultat positiu. Empremta final de verificació: %s.",
		verification.LastHash,
	)

	pdf.MultiCell(0, 4, tr(text2), "", "J", false)
}

func writeSignatureBlock(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	meta BudgetAuditCertificateMeta,
) VisibleSignaturePosition {
	ensureSpaceForVisibleSignature(pdf)

	pdf.Ln(10)

	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(
		0,
		5,
		tr(fmt.Sprintf("Bellaterra, %s", formatCatalanDate(time.Now().Format("2006-01-02")))),
		"",
		"L",
		false,
	)

	pdf.Ln(8)

	_, pageHeight := pdf.GetPageSize()

	// Caja reservada para que el firmador externo coloque la apariencia visible real.
	// No escribimos datos del certificado aquí para evitar mostrar una firma "simulada".
	signatureX := 18.0
	signatureY := pdf.GetY()
	signatureW := 105.0
	signatureH := 34.0

	signaturePosition := VisibleSignaturePosition{
		Page:         pdf.PageNo(),
		Xmm:          signatureX,
		Ymm:          signatureY,
		Wmm:          signatureW,
		Hmm:          signatureH,
		PageHeightMm: pageHeight,
	}

	pdf.SetDrawColor(165, 165, 165)
	pdf.SetFillColor(250, 250, 250)
	pdf.Rect(signatureX, signatureY, signatureW, signatureH, "DF")

	pdf.SetXY(signatureX+4, signatureY+4)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(90, 90, 90)
	pdf.MultiCell(
		signatureW-8,
		4,
		tr(""),
		"",
		"L",
		false,
	)

	pdf.SetTextColor(0, 0, 0)
	pdf.SetY(signatureY + signatureH + 5)

	pdf.SetFont("Arial", "B", 10)
	pdf.MultiCell(0, 5, tr("Centre de Recerca Matemàtica (CRM)"), "", "L", false)

	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(
		0,
		4.5,
		tr("Centre emissor del certificat d'auditoria pressupostària"),
		"",
		"L",
		false,
	)

	return signaturePosition
}

func ensureSpaceForVisibleSignature(pdf *fpdf.Fpdf) {
	_, pageHeight := pdf.GetPageSize()
	_, _, _, bottomMargin := pdf.GetMargins()

	requiredHeight := 80.0
	currentY := pdf.GetY()
	maxY := pageHeight - bottomMargin - 10

	if currentY+requiredHeight > maxY {
		pdf.AddPage()
	}
}

func signBudgetAuditPDFWithVisibleCertificate(
	pdfBytes []byte,
	position VisibleSignaturePosition,
) ([]byte, error) {
	if !isBudgetAuditPDFSigningConfigured() {
		return nil, fmt.Errorf("PDF signing is not configured")
	}

	resolvedSignerCommand, err := resolveExecutablePath(mC.PDFSignerCommand)
	if err != nil {
		return nil, err
	}

	resolvedCertificatePath, err := resolveReadablePath(mC.PDFCertificatePath)
	if err != nil {
		return nil, err
	}

	rect := convertToPDFSignatureRect(position)

	unsignedFile, err := os.CreateTemp("", "budget-audit-unsigned-*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(unsignedFile.Name())

	if err := unsignedFile.Chmod(0o600); err != nil {
		unsignedFile.Close()
		return nil, err
	}

	if _, err := unsignedFile.Write(pdfBytes); err != nil {
		unsignedFile.Close()
		return nil, err
	}

	if err := unsignedFile.Close(); err != nil {
		return nil, err
	}

	signedFile, err := os.CreateTemp("", "budget-audit-signed-*.pdf")
	if err != nil {
		return nil, err
	}

	signedPath := signedFile.Name()

	if err := signedFile.Chmod(0o600); err != nil {
		signedFile.Close()
		return nil, err
	}

	if err := signedFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(signedPath)

	certPassword, err := getPDFCertificatePassword()
	if err != nil {
		return nil, err
	}

	args := []string{
		"--input", unsignedFile.Name(),
		"--output", signedPath,
		"--certificate", resolvedCertificatePath,
		"--visible",
		"--page", strconv.Itoa(rect.Page),
		"--x", fmt.Sprintf("%.2f", rect.Xpt),
		"--y", fmt.Sprintf("%.2f", rect.Ypt),
		"--width", fmt.Sprintf("%.2f", rect.Wpt),
		"--height", fmt.Sprintf("%.2f", rect.Hpt),
		"--reason", firstNonEmpty(mC.PDFSignatureReason, "Segell electrònic intern del certificat d'auditoria pressupostària"),
		"--location", firstNonEmpty(mC.PDFSignatureLocation, "CRMIntratools"),
	}

	cmd := exec.Command(resolvedSignerCommand, args...)

	// No pasamos la contraseña como argumento para evitar que pueda verse en la lista de procesos.
	cmd.Stdin = strings.NewReader(certPassword + "\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error signing PDF: %w - %s", err, sanitizeSignerOutput(string(output)))
	}

	signedBytes, err := os.ReadFile(signedPath)
	if err != nil {
		return nil, err
	}

	if len(signedBytes) == 0 {
		return nil, fmt.Errorf("signed PDF is empty")
	}

	return signedBytes, nil
}

func isBudgetAuditPDFSigningConfigured() bool {
	return strings.TrimSpace(mC.PDFSignerCommand) != "" &&
		strings.TrimSpace(mC.PDFCertificatePath) != "" &&
		(strings.TrimSpace(mC.PDFCertificatePassword) != "")
}

func resolveExecutablePath(path string) (string, error) {
	resolvedPath, err := resolveReadablePath(path)
	if err != nil {
		return "", fmt.Errorf("PDF signer command not found or not accessible: %w", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("PDF signer command not found or not accessible: %s: %w", resolvedPath, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("PDF signer command points to a directory: %s", resolvedPath)
	}

	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("PDF signer command is not executable: %s", resolvedPath)
	}

	return resolvedPath, nil
}

func resolveReadablePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	if fileExists(path) {
		return path, nil
	}

	if filepath.IsAbs(path) {
		return "", fmt.Errorf("%s", path)
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get working directory while resolving %s: %w", path, err)
	}

	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, path)
		if fileExists(candidate) {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("%s not found from working directory %s or its parent directories", path, wd)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func convertToPDFSignatureRect(pos VisibleSignaturePosition) PDFSignatureRect {
	pageHeight := pos.PageHeightMm
	if pageHeight <= 0 {
		pageHeight = 297.0
	}

	return PDFSignatureRect{
		Page: pos.Page,
		Xpt:  mmToPt(pos.Xmm),
		Ypt:  mmToPt(pageHeight - pos.Ymm - pos.Hmm),
		Wpt:  mmToPt(pos.Wmm),
		Hpt:  mmToPt(pos.Hmm),
	}
}

func mmToPt(mm float64) float64 {
	return mm * 72.0 / 25.4
}

func sanitizeSignerOutput(output string) string {
	output = strings.ReplaceAll(output, "\n", " ")
	output = strings.ReplaceAll(output, "\r", " ")
	output = strings.ReplaceAll(output, strings.TrimSpace(mC.PDFCertificatePassword), "[hidden]")

	if len(output) > 500 {
		return output[:500] + "..."
	}

	return output
}

func writePDFKeyValueCA(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	key string,
	value string,
) {
	value = cleanPDFValue(value)

	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(42, 5, tr(key+":"), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 5, tr(value), "", "L", false)
}

func cleanPDFValue(value string) string {
	value = strings.TrimSpace(value)

	if value == "" || value == "<nil>" || value == "0001-01-01T00:00:00Z" {
		return "-"
	}

	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return "-"
}

func shortHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "-"
	}
	if len(hash) <= 18 {
		return hash
	}
	return hash[:18] + "..."
}

func formatActorName(actor BudgetAuditActor) string {
	fullName := strings.TrimSpace(actor.Name + " " + actor.Surname)

	if fullName == "" {
		fullName = actor.Username
	}

	/*
		if actor.Username != "" {
			return fmt.Sprintf("%s (%s)", fullName, actor.Username)
		}
	*/
	return fullName
}

func formalizeSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return "-"
	}
	return summary
}

func formatCatalanDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return "-"
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t.Format("02/01/2006")
		}
	}

	return value
}

func formatCatalanDateTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return "-"
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t.Format("02/01/2006 15:04")
		}
	}

	return value
}

func writeInlineText(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	text string,
) {
	text = tr(text)

	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	maxX := pageWidth - right

	pdf.SetFont("Arial", "", 10)

	words := strings.Split(text, " ")

	for i, word := range words {
		if word == "" {
			continue
		}

		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}

		width := pdf.GetStringWidth(chunk)

		if pdf.GetX()+width > maxX {
			pdf.Ln(5.5)
			pdf.SetX(left)
		}

		pdf.Write(5.5, chunk)
	}
}

func writeHighlightedInlineValue(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	value string,
) {
	value = cleanPDFValue(value)
	value = strings.TrimSpace(value)
	value = tr(value)

	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	maxX := pageWidth - right

	pdf.SetFont("Arial", "B", 10)

	width := pdf.GetStringWidth(value)

	if pdf.GetX()+width > maxX {
		pdf.Ln(5.8)
		pdf.SetX(left)
	}

	pdf.CellFormat(width, 5.5, value, "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 10)
}

func truncatePDFInlineValue(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)

	if len(runes) <= maxRunes {
		return value
	}

	return string(runes[:maxRunes]) + "..."
}
func translateTravelArea(area string) string {
	area = strings.TrimSpace(strings.ToLower(area))

	switch area {
	case "spain", "espanya":
		return "Nacional (Espanya)"
	case "eu":
		return "Dins de la Unió Europea"
	case "non_eu":
		return "Fora de la Unió Europea"
	default:
		return cleanPDFValue(area)
	}
}
