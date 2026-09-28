package module9workers

import (
	"bytes"
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

func handleCheckUserInformation(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, userRole := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"role":     userRole,
		"username": username,
		"userID":   userID,
	})
}

func handleFetchModalities(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, userRole := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":   "modalities",
		"columns": "*",
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getModalities: getReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Error fetching modalities", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getModalities: error reading response for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		createLog(fmt.Sprintf("getModalities: invalid JSON from DB for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}

	for _, row := range rows {
		row["title"] = unescapeComma(row["title"].(string))
		row["category"] = unescapeComma(row["category"].(string))
		row["description"] = unescapeComma(row["description"].(string))
		row["icon"] = unescapeComma(row["icon"].(string))
	}
	asString := func(v interface{}) string {
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		default:
			return ""
		}
	}
	isPrivileged := userRole == "admin" || userRole == "manager" || userRole == "support"
	if !isPrivileged {
		filtered := make([]map[string]interface{}, 0, len(rows))
		for _, row := range rows {
			if asString(row["category"]) == "Administration" {
				continue
			}
			filtered = append(filtered, row)
		}
		rows = filtered
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   rows,
	})
}

func handleCreateModality(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	var payload struct {
		Title       string `json:"title"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		createLog(fmt.Sprintf("createModality: invalid JSON from user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":   "modalities",
		"columns": "title, category, description, icon, creator",
		"value":   fmt.Sprintf("%s, %s, %s, %s, %d", escapeComma(payload.Title), escapeComma(payload.Category), escapeComma(payload.Description), escapeComma(payload.Icon), userID),
	}

	body, _ := json.Marshal(query)
	resp := postReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("createModality: postReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Error creating modality", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("createModality: DB error for user %s: %s", username, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error: "+string(respBody), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func handleFetchDocumentsByModality(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, userRole := getUserInfo(apiKey, client, w, r)

	modalityID := r.URL.Query().Get("modalityId")
	if modalityID == "" {
		http.Error(w, "Missing modalityId", http.StatusBadRequest)
		return
	}

	isPrivileged := userRole == "admin" || userRole == "manager" || userRole == "support"

	asString := func(v interface{}) string {
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		default:
			return ""
		}
	}

	checkQuery := map[string]interface{}{
		"table":     "modalities",
		"columns":   "id, category",
		"condition": fmt.Sprintf("id = %s", modalityID),
		"limit":     1,
	}
	checkBody, _ := json.Marshal(checkQuery)
	checkResp := getReq(checkBody, apiKey, client, w)
	if checkResp == nil {
		createLog(fmt.Sprintf("fetchDocumentsByModality: getReq nil checking modality %s for user %s", modalityID, username), 1, apiKey, client, w)
		http.Error(w, "Error checking modality", http.StatusInternalServerError)
		return
	}
	defer checkResp.Body.Close()

	checkRespBody, err := io.ReadAll(checkResp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchDocumentsByModality: error reading modality check for %s user %s: %v", modalityID, username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var modalityRows []map[string]interface{}
	if err := json.Unmarshal(checkRespBody, &modalityRows); err != nil {
		createLog(fmt.Sprintf("fetchDocumentsByModality: invalid JSON modality check for %s user %s: %v", modalityID, username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}
	if len(modalityRows) == 0 {
		http.Error(w, "Modality not found", http.StatusNotFound)
		return
	}

	category := unescapeComma(asString(modalityRows[0]["category"]))
	if !isPrivileged && category == "Administration" {
		createLog(fmt.Sprintf("fetchDocumentsByModality: forbidden modality %s (Administration) for user %s role %s", modalityID, username, userRole), 1, apiKey, client, w)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	query := map[string]interface{}{
		"table":     "modality_docs",
		"columns":   "id, title, updatedAt, modality_id",
		"condition": fmt.Sprintf("modality_id = %s", modalityID),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("fetchDocumentsByModality: getReq returned nil for modality %s", modalityID), 1, apiKey, client, w)
		http.Error(w, "Error fetching documents", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchDocumentsByModality: error reading response for modality %s: %v", modalityID, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		createLog(fmt.Sprintf("fetchDocumentsByModality: invalid JSON from DB for modality %s: %v", modalityID, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}

	for _, row := range rows {
		row["title"] = unescapeComma(row["title"].(string))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   rows,
	})
}

func handleGetPath(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, userRole := getUserInfo(apiKey, client, w, r)

	docID := r.URL.Query().Get("docId")
	if docID == "" {
		http.Error(w, "Missing docId", http.StatusBadRequest)
		return
	}

	isPrivileged := userRole == "admin" || userRole == "manager" || userRole == "support"

	asString := func(v interface{}) string {
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		default:
			return ""
		}
	}

	query := map[string]interface{}{
		"table":     "modality_docs",
		"columns":   "filePath, title, modality_id",
		"condition": fmt.Sprintf("id = %s", docID),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getPath: getReq returned nil for docId %s", docID), 1, apiKey, client, w)
		http.Error(w, "Error fetching document path", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getPath: error reading response for docId %s: %v", docID, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		createLog(fmt.Sprintf("getPath: invalid JSON from DB for docId %s: %v", docID, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}

	if len(rows) == 0 {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	var modalityID int
	switch v := rows[0]["modality_id"].(type) {
	case float64:
		modalityID = int(v)
	case int:
		modalityID = v
	case string:
		modalityID, _ = strconv.Atoi(v)
	default:
		modalityID = 0
	}
	if modalityID <= 0 {
		createLog(fmt.Sprintf("getPath: invalid modality_id for docId %d user %s", docID, username), 1, apiKey, client, w)
		http.Error(w, "Invalid document data", http.StatusInternalServerError)
		return
	}

	modQuery := map[string]interface{}{
		"table":     "modalities",
		"columns":   "id, category",
		"condition": fmt.Sprintf("id = %d", modalityID),
		"limit":     1,
	}
	modBody, _ := json.Marshal(modQuery)
	modResp := getReq(modBody, apiKey, client, w)
	if modResp == nil {
		createLog(fmt.Sprintf("getPath: getReq nil checking modality %d for docId %d user %s", modalityID, docID, username), 1, apiKey, client, w)
		http.Error(w, "Error checking modality", http.StatusInternalServerError)
		return
	}
	defer modResp.Body.Close()

	modRespBody, err := io.ReadAll(modResp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getPath: error reading modality response modality %d docId %d user %s: %v", modalityID, docID, username, err), 1, apiKey, client, w)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var modRows []map[string]interface{}
	if err := json.Unmarshal(modRespBody, &modRows); err != nil {
		createLog(fmt.Sprintf("getPath: invalid JSON modality check modality %d docId %d user %s: %v", modalityID, docID, username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid JSON from DB", http.StatusInternalServerError)
		return
	}
	if len(modRows) == 0 {
		http.Error(w, "Modality not found", http.StatusNotFound)
		return
	}

	category := unescapeComma(asString(modRows[0]["category"]))
	if !isPrivileged && category == "Administration" {
		createLog(fmt.Sprintf("getPath: forbidden docId %d (modality %d Administration) for user %s role %s", docID, modalityID, username, userRole), 1, apiKey, client, w)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"filePath": rows[0]["filePath"],
		"title":    unescapeComma(rows[0]["title"].(string)),
	})
}

func handleUploadDocument(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	// Parse multipart form with a reasonable max memory
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	userID, username, _ := getUserInfo(apiKey, client, w, r)

	title := r.FormValue("title")
	modalityID := r.FormValue("modalityId")
	file, handler, err := r.FormFile("file")
	if err != nil {
		createLog(fmt.Sprintf("uploadDocument: error getting file from form for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Error getting file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if title == "" || modalityID == "" {
		http.Error(w, "Missing title or modalityId", http.StatusBadRequest)
		return
	}

	// Validate file type (basic check)
	head := make([]byte, 5)
	n, err := io.ReadFull(file, head)
	if err != nil || n < 5 {
		http.Error(w, "Invalid PDF file", http.StatusBadRequest)
		return
	}
	if string(head) != "%PDF-" {
		http.Error(w, "Only PDF files are allowed", http.StatusBadRequest)
		return
	}

	// Obtener el título de la modalidad para el path <id>_<titulo_modalidad>
	modTitle, err := getModalityTitle(apiKey, client, w, modalityID)
	if err != nil {
		createLog(fmt.Sprintf("uploadDocument: could not resolve modality title for modalityID=%s user=%s: %v", modalityID, username, err),
			1, apiKey, client, w)
		modTitle = "modality"
	}

	folderName := fmt.Sprintf("%s_%s", modalityID, slugifyFS(modTitle))
	// Path en disco
	dirOnDisk := path.Join("../../Uploads", "02_Modalities", folderName)

	if err := os.MkdirAll(dirOnDisk, os.ModePerm); err != nil {
		createLog(fmt.Sprintf("uploadDocument: mkdir failed user %s dir %s: %v", username, dirOnDisk, err), 1, apiKey, client, w)
		http.Error(w, "Error creating folder", http.StatusInternalServerError)
		return
	}

	ext := filepath.Ext(handler.Filename)
	if ext == "" || len(ext) > 5 {
		ext = ".bin"
	}

	// filename YYYYMMDD_HHMMSS_ZZZZ.pdf
	// ZZZZ = userID con ceros delante (4 dígitos)
	userPadded := fmt.Sprintf("%04d", userID)
	ts := time.Now().Format("20060102_150405") // YYYYMMDD_HHMMSS
	filename := fmt.Sprintf("%s_%s%s", ts, userPadded, ext)

	fullPathOnDisk := path.Join(dirOnDisk, filename)

	out, err := os.Create(fullPathOnDisk)
	if err != nil {
		createLog(fmt.Sprintf("uploadDocument: create file failed user %s path %s: %v", username, fullPathOnDisk, err), 1, apiKey, client, w)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	// reconstruimos el reader con los bytes ya leídos + resto del file
	reader := io.MultiReader(bytes.NewReader(head), file)

	if _, err := io.Copy(out, reader); err != nil {
		createLog(fmt.Sprintf("uploadDocument: write file failed user %s path %s: %v", username, fullPathOnDisk, err), 1, apiKey, client, w)
		http.Error(w, "Error writing file", http.StatusInternalServerError)
		return
	}

	// filePath que guardas en DB
	// "/Uploads/02_Modalities/<folderName>/<filename>"
	filePathForDB := path.Join("/Uploads", "02_Modalities", folderName, filename)
	updatedAt := time.Now().Format("02-01-2006 15:04")

	// Insert document record into DB
	query := map[string]interface{}{
		"table":   "modality_docs",
		"columns": "title, modality_id, filePath, updatedAt, updatedBy",
		"value":   fmt.Sprintf("%s, %s, %s, %s, %d", escapeComma(title), modalityID, filePathForDB, updatedAt, userID),
	}

	body, _ := json.Marshal(query)
	resp := postReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("uploadDocument: postReq returned nil for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Error saving document info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("uploadDocument: DB error for user %s: %s", username, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error: "+string(respBody), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func slugifyFS(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "untitled"
	}

	var b strings.Builder
	b.Grow(len(s))

	prevDash := false
	for _, r := range s {
		isAZ := (r >= 'a' && r <= 'z')
		is09 := (r >= '0' && r <= '9')
		if isAZ || is09 {
			b.WriteRune(r)
			prevDash = false
			continue
		}

		if r == '_' {
			b.WriteRune(r)
			prevDash = false
			continue
		}

		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	if len(out) > 60 {
		out = out[:60]
		out = strings.Trim(out, "-")
	}
	return out
}
func getModalityTitle(apiKey string, client *http.Client, w http.ResponseWriter, modalityID string) (string, error) {
	query := map[string]interface{}{
		"table":     "modalities",
		"columns":   "title",
		"condition": fmt.Sprintf("id = '%s'", modalityID),
	}

	body, _ := json.Marshal(query)
	resp := getReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("getModalityTitle: getReq returned nil for modalityID=%s", modalityID), 1, apiKey, client, w)
		return "", fmt.Errorf("getReq returned nil for modalityID=%s", modalityID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("getModalityTitle: error reading response for modalityID=%s: %v", modalityID, err), 1, apiKey, client, w)
		return "", fmt.Errorf("error reading response for modalityID=%s: %v", modalityID, err)
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		createLog(fmt.Sprintf("getModalityTitle: invalid JSON from DB for modalityID=%s: %v", modalityID, err), 1, apiKey, client, w)
		return "", fmt.Errorf("invalid JSON from DB for modalityID=%s: %v", modalityID, err)
	}

	if len(rows) == 0 {
		createLog(fmt.Sprintf("getModalityTitle: modality not found for modalityID=%s", modalityID), 1, apiKey, client, w)
		return "", fmt.Errorf("modality not found for modalityID=%s", modalityID)
	}

	title := pathUnescape(rows[0]["title"].(string))
	return title, nil
}

func handleUpdateDocument(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		DocID      string `json:"docId"`
		Title      string `json:"title"`
		ModalityID string `json:"modalityId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "modality_docs",
		"columns":   "modality_id, title",
		"value":     fmt.Sprintf("%s, %s", payload.ModalityID, escapeComma(payload.Title)),
		"condition": fmt.Sprintf("id = '%s'", payload.DocID),
	}

	body, _ := json.Marshal(query)
	resp := putReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("updateDocument: putReq returned nil for docId %s", payload.DocID), 1, apiKey, client, w)
		http.Error(w, "Error updating document", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("updateDocument: DB error for docId %s: %s", payload.DocID, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error: "+string(respBody), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func handleDeleteDocument(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		DocID      string `json:"docId"`
		Title      string `json:"title"`
		ModalityID string `json:"modalityId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.DocID == "" {
		http.Error(w, "Missing docId", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "modality_docs",
		"condition": fmt.Sprintf("id = '%s'", payload.DocID),
	}

	body, _ := json.Marshal(query)
	resp := deleteReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("deleteDocument: deleteReq returned nil for docId %s", payload.DocID), 1, apiKey, client, w)
		http.Error(w, "Error deleting document", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("deleteDocument: DB error for docId %s: %s", payload.DocID, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error: "+string(respBody), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func handleUpdateModality(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ModalityID  string `json:"id"`
		Title       string `json:"title"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "modalities",
		"columns":   "title, category, description, icon",
		"value":     fmt.Sprintf("%s, %s, %s, %s", escapeComma(payload.Title), escapeComma(payload.Category), escapeComma(payload.Description), escapeComma(payload.Icon)),
		"condition": fmt.Sprintf("id = '%s'", payload.ModalityID),
	}

	body, _ := json.Marshal(query)
	resp := putReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("updateModality: putReq returned nil for modalityId %s", payload.ModalityID), 1, apiKey, client, w)
		http.Error(w, "Error updating modality", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("updateModality: DB error for modalityId %s: %s", payload.ModalityID, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error: "+string(respBody), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func handleDeleteModality(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ModalityID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.ModalityID == "" {
		http.Error(w, "Missing modalityId", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "modality_docs",
		"condition": fmt.Sprintf("modality_id = '%s'", payload.ModalityID),
	}

	body, _ := json.Marshal(query)
	resp := deleteReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("deleteModality: deleteReq for modality_docs returned nil for modalityId %s", payload.ModalityID), 1, apiKey, client, w)
		http.Error(w, "Error deleting modality documents", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("deleteModality: DB error deleting modality_docs for modalityId %s: %s", payload.ModalityID, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error deleting modality documents: "+string(respBody), http.StatusInternalServerError)
		return
	}

	query = map[string]interface{}{
		"table":     "modalities",
		"condition": fmt.Sprintf("id = '%s'", payload.ModalityID),
	}

	body, _ = json.Marshal(query)
	resp = deleteReq(body, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("deleteModality: deleteReq returned nil for modalityId %s", payload.ModalityID), 1, apiKey, client, w)
		http.Error(w, "Error deleting modality", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		createLog(fmt.Sprintf("deleteModality: DB error for modalityId %s: %s", payload.ModalityID, string(respBody)), 1, apiKey, client, w)
		http.Error(w, "DB error: "+string(respBody), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}
