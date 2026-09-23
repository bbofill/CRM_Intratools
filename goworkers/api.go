package goworkers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// handle monolityc API crud route (very open)
func serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}
	var input ApiData
	if err := input.parseRequest(r); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	methodRaw, ok := input["method"]
	if !ok {
		http.Error(w, "Missing 'method' in request body", http.StatusBadRequest)
		return
	}
	methodStr, ok := methodRaw.(string)
	if !ok {
		http.Error(w, "'method' must be a string", http.StatusBadRequest)
		return
	}
	switch methodStr {
	case "GET":
		input.handleGet(w)
	case "POST":
		input.handlePost(w)
	case "PUT":
		input.handlePut(w)
	case "DELETE":
		input.handleDelete(w)
	default:
		http.Error(w, "Unsupported method in JSON", http.StatusMethodNotAllowed)
	}
}

func serveAPIMSSQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	var input ApiData
	if err := input.parseRequest(r); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	methodRaw, ok := input["method"]
	if !ok {
		http.Error(w, "Missing 'method' in request body", http.StatusBadRequest)
		return
	}

	methodStr, ok := methodRaw.(string)
	if !ok {
		http.Error(w, "'method' must be a string", http.StatusBadRequest)
		return
	}

	switch methodStr {
	case "GET":
		input.handleGetMSSQL(w)
	case "POST":
		input.handlePostMSSQL(w)
	case "PUT":
		input.handlePutMSSQL(w)
	case "DELETE":
		input.handleDeleteMSSQL(w)
	default:
		http.Error(w, "Unsupported method in JSON", http.StatusMethodNotAllowed)
	}
}

// Log API
func serveLogAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mode := strToLower(strToTrimSpace(r.URL.Query().Get("mode")))

		var query string
		switch mode {
		case "verify":
			query = `
				SELECT id,
				       id_module       AS module,
				       timestamp,
				       log_content     AS msg,
				       code,
				       hash,
				       previous_hash
				FROM logs
				ORDER BY id ASC;
			`
		default:
			limit := 200
			offset := 0

			if raw := strToTrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 200 {
					limit = v
				}
			}

			if raw := strToTrimSpace(r.URL.Query().Get("offset")); raw != "" {
				if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
					offset = v
				}
			}

			query = fmt.Sprintf(`
				SELECT id,
				       id_module       AS module,
				       timestamp,
				       log_content     AS msg,
				       code,
				       hash,
				       previous_hash
				FROM logs
				ORDER BY id DESC
				LIMIT %d OFFSET %d;
			`, limit, offset)
		}

		chain := executeQuery(query)
		writeJSON(w, chain)

	case http.MethodPost:
		var in ApiData
		module := resolveModuleFromKey(extractApiKey(r)) // sanitize here

		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		msg, okMsg := in["msg"].(string)
		codeFloat, okCode := in["code"].(float64)
		if !okMsg || !okCode || strToTrimSpace(msg) == "" {
			http.Error(w, "missing or invalid fields", http.StatusBadRequest)
			return
		}

		if err := AddModuleLog(module, msg, int(codeFloat)); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// this api is used to controll (stop, start and restart) the avaliable and enabled modules.
func serveModuleControlAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var in ApiData
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	method, ok := in["method"].(string)
	if !ok || strToTrimSpace(method) == "" {
		http.Error(w, "missing method field", http.StatusBadRequest)
		return
	}

	switch strToLower(method) {
	case "get":
		w.Header().Set("Content-Type", "application/json")
		moduleStatuses.RLock()
		defer moduleStatuses.RUnlock()

		mod, _ := in["module"].(string)
		mod = strToTrimSpace(mod)
		if mod == "" {
			_ = json.NewEncoder(w).Encode(moduleStatuses.m)
			return
		}

		status, ok := moduleStatuses.m[mod]
		if !ok {
			status = "unknown"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"module": mod,
			"status": status,
		})

	case "post":

		mod, mok := in["module"].(string)
		act, aok := in["action"].(string)

		if !mok || !aok || strToTrimSpace(mod) == "" || strToTrimSpace(act) == "" {
			http.Error(w, "missing module or action", http.StatusBadRequest)
			return
		}
		act = strToLower(strToTrimSpace(act))

		if act != "start" && act != "stop" && act != "restart" {
			http.Error(w, "invalid action", http.StatusBadRequest)
			return
		}
		if _, ok := mOdUlEcOnF[mod]; !ok {
			http.Error(w, "unknown module", http.StatusBadRequest)
			return
		}

		mOdUlEcTl <- ModuleAction{Name: mod, Action: act}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ok"}`))

	default:
		http.Error(w, "invalid method", http.StatusBadRequest)
	}
}
