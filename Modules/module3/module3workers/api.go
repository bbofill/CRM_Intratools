package module3workers

import (
	"io"
	"net/http"
	"time"
)

func serveAPI(w http.ResponseWriter, r *http.Request) {

	//req api key
	apiKey := r.FormValue("key")

	if verifyAPIKey(apiKey) {
		//set client

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		if mC.Development {
			client = setInsecureRequest()
		}

		switch r.Method {
		case http.MethodGet:
			switch r.URL.Query().Get("action") {

			// Sends all user info to frontend
			case "profile":
				handleGetUserProfile(apiKey, client, w, r)
			// DEPRECATED - Users can no longer suggest changes to their data
			case "sendProfileSuggestion":
				handleSendProfileSuggestion(apiKey, client, w, r)
			case "notifications":
				handleGetNotifications(apiKey, client, w, r)
			case "getSentNotifications":
				handleGetSentNotifications(apiKey, client, w, r)
			case "get-options":
				//returns all form options to front-end
				handleGetOptions(apiKey, client, w, r)
			}
		case http.MethodPut:
			switch r.URL.Query().Get("action") {
			// Changes db field notification_user.seen from 0 to 1 (changes visualization)
			case "markSeen":
				handleMarkNotificationSeen(apiKey, client, w, r)
			// DEPRECATED - Data is no longer modified in module 3
			case "applyProfileChanges":
				handleApplyProfileChanges(apiKey, client, w, r)

			}
		case http.MethodPost:
			switch r.URL.Query().Get("action") {
			// DEPRECATED - Users can no longer suggest changes to their data
			case "sendProfileSuggestion":
				handleSendProfileSuggestion(apiKey, client, w, r)
			// Changes users.password
			case "changePassword":
				handleChangePassword(apiKey, client, w, r)
			// DEPRECATED - Profile picture is no longer modified in module 3
			case "uploadPhoto":
				handleUploadPhoto(apiKey, client, w, r)
			}
		case http.MethodDelete:
			switch r.URL.Query().Get("action") {
			// Deletes notification_user entry
			case "deleteNotification":
				handleDeleteNotification(apiKey, client, w, r)
			}
		}

	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		createLog("Invalid API key provided", 2, apiKey, nil, w)
		return
	}

}
func serveUpload(w http.ResponseWriter, r *http.Request) {
	apiKey := r.FormValue("key")

	if !verifyAPIKey(apiKey) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		createLog("Invalid API key provided", 2, apiKey, nil, w)
		return
	}

	if verifyAPIKey(apiKey) {
		req, _ := http.NewRequest(http.MethodGet, "https://localhost:"+mC.ServerPort+"/"+r.URL.Query().Get("file"), nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		if mC.Development {
			client = setInsecureRequest()
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Error haciendo forwarding", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
