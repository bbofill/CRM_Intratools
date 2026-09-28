package goworkers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// ------------------------------ SQLITE --------------------------------
// SELECT from database
func (a ApiData) handleGet(w http.ResponseWriter) {
	table := a["table"].(string)
	columns := a["columns"].(string)
	condition, _ := a["condition"].(string)

	query := buildSelectQuery(table, columns, condition)
	result := executeQuery(query)

	writeJSON(w, result)
}

// INSERT into database
func (a ApiData) handlePost(w http.ResponseWriter) {
	table := a["table"].(string)
	columns := a["columns"].(string)
	values := a["value"].(string)

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	if len(colList) != len(valList) {
		http.Error(w, "Mismatched number of columns and values", http.StatusBadRequest)
		return
	}

	query := buildInsertQuery(table, colList, valList)
	executeInsert(query)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"success"}`))
}

// UPDATE in database
func (a ApiData) handlePut(w http.ResponseWriter) {
	table := a["table"].(string)
	columns := a["columns"].(string)
	values := a["value"].(string)
	condition := a["condition"].(string)

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	if len(colList) != len(valList) {
		http.Error(w, "Mismatched number of columns and values", http.StatusBadRequest)
		return
	}

	query := buildUpdateQuery(table, colList, valList, condition)
	executeInsert(query)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// DELETE from database
func (a ApiData) handleDelete(w http.ResponseWriter) {
	table := a["table"].(string)
	condition := a["condition"].(string)

	if condition == "" {
		http.Error(w, "DELETE requires a condition", http.StatusBadRequest)
		return
	}

	query := buildDeleteQuery(table, condition)
	executeInsert(query)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// Parse incoming JSON body into ApiData (fill obj with in data)
func (a *ApiData) parseRequest(r *http.Request) error {
	return parseApiInJSON(r.Body, a)
}

// parseJSON from io.reader to ApiData
func parseApiInJSON(body io.Reader, out *ApiData) error {
	return json.NewDecoder(body).Decode(out)
}

// write json response (base64?)
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	beautifiedJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Error Generating Response JSON", http.StatusInternalServerError)
		return
	}
	w.Write(beautifiedJSON)
}

// ------------------------------ MSSQL --------------------------------
// SELECT from MSSQL DB
func (a ApiData) handleGetMSSQL(w http.ResponseWriter) {
	table := a["table"].(string)
	columns := a["columns"].(string)
	condition, _ := a["condition"].(string)

	query := buildSelectQuery(table, columns, condition)
	result, err := queryMSSQL(query)
	if err != nil {
		http.Error(w, "MSSQL query error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// INSERT into MSSQL
func (a ApiData) handlePostMSSQL(w http.ResponseWriter) {
	table := a["table"].(string)
	columns := a["columns"].(string)
	values := a["value"].(string)

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	if len(colList) != len(valList) {
		http.Error(w, "Mismatched number of columns and values", http.StatusBadRequest)
		return
	}

	query := buildInsertQuery(table, colList, valList)
	if err := execMSSQL(query); err != nil {
		http.Error(w, "MSSQL insert error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"success"}`))
}

// UPDATE MSSQL
func (a ApiData) handlePutMSSQL(w http.ResponseWriter) {

	if rawAny, ok := a["raw"]; ok {
		raw, _ := rawAny.(string)
		raw = strings.TrimSpace(raw)
		if raw != "" {

			// Seguridad mínima: evita múltiples sentencias
			// (si quieres permitir ';' dentro de strings, habría que parsear mejor)
			if strings.Contains(raw, ";") {
				http.Error(w, "raw SQL cannot contain ';'", http.StatusBadRequest)
				return
			}

			if err := execMSSQL(raw); err != nil {
				http.Error(w, "MSSQL update error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success"}`))
			return
		}
	}

	table := a["table"].(string)
	columns := a["columns"].(string)
	values := a["value"].(string)
	condition := a["condition"].(string)

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	if len(colList) != len(valList) {
		http.Error(w, "Mismatched number of columns and values", http.StatusBadRequest)
		return
	}

	query := buildUpdateQuery(table, colList, valList, condition)
	if err := execMSSQL(query); err != nil {
		http.Error(w, "MSSQL update error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// DELETE MSSQL
func (a ApiData) handleDeleteMSSQL(w http.ResponseWriter) {
	table := a["table"].(string)
	condition := a["condition"].(string)

	if condition == "" {
		http.Error(w, "DELETE requires a condition", http.StatusBadRequest)
		return
	}

	query := buildDeleteQuery(table, condition)
	if err := execMSSQL(query); err != nil {
		http.Error(w, "MSSQL delete error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
