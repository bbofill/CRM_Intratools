package module0workers

import (
	"net/http"
)

// handle user frontend api
func handleUsers(w http.ResponseWriter, r *http.Request) {
	//req api key
	apiKey := r.FormValue("key")

	if verifyAPIKey(apiKey) {
		var client *http.Client
		if cfg.Development {
			client = setInsecureRequestClient()
		}
		println("method", r.Method)
		println("action", r.URL.Query().Get("action"))
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("action") == "available-people" {
				handleGetAvailablePeople(apiKey, client, w, r)
			} else if r.URL.Query().Get("action") == "generate-username" {
				handleGenerateUsername(apiKey, client, w, r)
			} else {
				handleGetUsers(w)
			}
		case http.MethodPost:
			if r.URL.Query().Get("action") == "updateUsers" {
				handleUpdateRole(w, r)
			}
			if r.URL.Query().Get("action") == "changePasswordAdmin" {
				handleUpdatePassword(w, r)

			} else {
				handlePostUser(apiKey, client, w, r)
			}
		case http.MethodDelete:
			handleDeleteUser(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
}

// handle module ctl frontend api
func handleModuleControl(w http.ResponseWriter, r *http.Request) {
	apiKey := r.FormValue("key")

	if verifyAPIKey(apiKey) {
		switch r.Method {
		case http.MethodGet:
			handleGetModuleControl(w, r) //r.HandleFunc("/api/users", handleDeleteUser).Methods("DELETE")
		case http.MethodPost:
			handlePostModuleControl(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
}

// handle log frontend api
func handleLogs(w http.ResponseWriter, r *http.Request) {
	apiKey := r.FormValue("key")

	if verifyAPIKey(apiKey) {
		switch r.Method {
		case http.MethodGet:
			handleGetLogs(w, r)
		case http.MethodPost:
			handlePostLogs(w)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
}
