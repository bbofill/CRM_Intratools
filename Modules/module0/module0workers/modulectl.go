package module0workers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// handle get, req data, serve data to front
func handleGetModuleControl(w http.ResponseWriter, r *http.Request) {
	module := r.URL.Query().Get("module")
	payload := map[string]interface{}{
		"method": "get",
	}
	if module != "" {
		payload["module"] = module
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://localhost:"+cfg.BackendPort+"/module/api/ctl", bytes.NewReader(jsonPayload))
	if err != nil {
		http.Error(w, "cannot build request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Transport: setInsecureRequest()}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "backend unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handle post validate keys and forward to backend
func handlePostModuleControl(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// paerse in
	var in struct {
		Module string `json:"module"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	in.Action = strings.ToLower(strings.TrimSpace(in.Action))
	in.Module = strings.TrimSpace(in.Module)

	// required fields
	if in.Module == "" || in.Action == "" {
		http.Error(w, "missing module or action", http.StatusBadRequest)
		return
	}

	// validate only allowed actions
	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[in.Action] {
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}

	// build payload
	payload, _ := json.Marshal(map[string]string{
		"method": "post",
		"module": in.Module,
		"action": in.Action,
	})

	// create req
	req, err := http.NewRequest("POST", "https://localhost:"+cfg.BackendPort+"/module/api/ctl", bytes.NewReader(payload))
	if err != nil {
		http.Error(w, "cannot build request", http.StatusInternalServerError)
		return
	}
	// auth headers
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Transport: setInsecureRequest()}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "backend unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
