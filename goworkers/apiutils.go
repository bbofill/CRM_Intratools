package goworkers

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_,\.\[\]\s]+$`)
var validCondition = regexp.MustCompile(`^[a-zA-Z0-9_,\.\[\]\s\=\>\<\!\@\?]+$`)

// ------------------------------ SQLITE --------------------------------
// SELECT from database
func (a ApiData) handleGet(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	columns, _ := a["columns"].(string)
	condition, _ := a["condition"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) || !validIdentifier.MatchString(columns) {
		http.Error(w, "invalid identifier detected", http.StatusBadRequest)
		return
	}

	query := buildSelectQuery(table, columns, condition)
	result := executeQuery(query, args...)

	writeJSON(w, result)
}

// INSERT into database
func (a ApiData) handlePost(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	columns, _ := a["columns"].(string)
	values, _ := a["value"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) {
		http.Error(w, "invalid table identifier", http.StatusBadRequest)
		return
	}

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	if len(colList) != len(valList) || len(valList) != len(args) {
		http.Error(w, "Mismatched columns, placeholders or args", http.StatusBadRequest)
		return
	}

	query := buildInsertQuery(table, colList, valList)
	executeInsert(query, args...)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"success"}`))
}

// UPDATE in database
func (a ApiData) handlePut(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	columns, _ := a["columns"].(string)
	values, _ := a["value"].(string)
	condition, _ := a["condition"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) {
		http.Error(w, "invalid table identifier", http.StatusBadRequest)
		return
	}

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	query := buildUpdateQuery(table, colList, valList, condition)
	executeInsert(query, args...)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// DELETE from database
func (a ApiData) handleDelete(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	condition, _ := a["condition"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) || condition == "" {
		http.Error(w, "invalid table or missing condition", http.StatusBadRequest)
		return
	}

	query := buildDeleteQuery(table, condition)
	executeInsert(query, args...)

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

func (a ApiData) handleGetMSSQL(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	columns, _ := a["columns"].(string)
	condition, _ := a["condition"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) || !validIdentifier.MatchString(columns) {
		http.Error(w, "invalid identifier detected", http.StatusBadRequest)
		return
	}

	if condition != "" && !validCondition.MatchString(condition) {
		http.Error(w, "invalid characters in condition", http.StatusBadRequest)
		return
	}

	query := "SELECT " + columns + " FROM " + table
	if condition != "" {
		query += " WHERE " + condition
	}

	result, err := queryMSSQL(query, args...)
	if err != nil {
		http.Error(w, "MSSQL read error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// INSERT into MSSQL
func (a ApiData) handlePostMSSQL(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	columns, _ := a["columns"].(string)
	values, _ := a["value"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) {
		http.Error(w, "invalid table identifier", http.StatusBadRequest)
		return
	}

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	if len(colList) != len(valList) || len(valList) != len(args) {
		http.Error(w, "Mismatched columns, placeholders or args", http.StatusBadRequest)
		return
	}

	query := buildInsertQuery(table, colList, valList)
	if err := execMSSQL(query, args...); err != nil {
		http.Error(w, "MSSQL insert error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"success"}`))
}

// UPDATE MSSQL
func (a ApiData) handlePutMSSQL(w http.ResponseWriter) {
	args, _ := a["args"].([]interface{})

	if rawAny, ok := a["raw"]; ok {
		raw, _ := rawAny.(string)
		raw = strings.TrimSpace(raw)
		if raw != "" {
			// Se mantiene la funcionalidad raw intacta pero parametrizada
			if strings.Contains(raw, ";") {
				http.Error(w, "raw SQL cannot contain ';'", http.StatusBadRequest)
				return
			}

			if err := execMSSQL(raw, args...); err != nil {
				http.Error(w, "MSSQL update error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success"}`))
			return
		}
	}

	table, _ := a["table"].(string)
	columns, _ := a["columns"].(string)
	values, _ := a["value"].(string)
	condition, _ := a["condition"].(string)

	if !validIdentifier.MatchString(table) {
		http.Error(w, "invalid table identifier", http.StatusBadRequest)
		return
	}

	colList := splitCommaSeparated(columns)
	valList := splitCommaSeparated(values)

	query := buildUpdateQuery(table, colList, valList, condition)
	if err := execMSSQL(query, args...); err != nil {
		http.Error(w, "MSSQL update error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// DELETE MSSQL
func (a ApiData) handleDeleteMSSQL(w http.ResponseWriter) {
	table, _ := a["table"].(string)
	condition, _ := a["condition"].(string)
	args, _ := a["args"].([]interface{})

	if !validIdentifier.MatchString(table) || condition == "" {
		http.Error(w, "invalid table or missing condition", http.StatusBadRequest)
		return
	}

	query := buildDeleteQuery(table, condition)
	if err := execMSSQL(query, args...); err != nil {
		http.Error(w, "MSSQL delete error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
