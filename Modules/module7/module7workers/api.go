package module7workers

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
		action := r.URL.Query().Get("action")
		switch action {
		// creates a new distribution list
		case "createNewList":
			handleCreateNewList(apiKey, client, w, r)
		// updates the fields of an existing distribution list
		case "updateList":
			handleUpdateList(apiKey, client, w, r)
		// retrieves all distribution lists
		case "getLists":
			handleGetDistributionLists(apiKey, client, w, r)
		// retrieves a specific distribution list by ID and its fields
		case "getList":
			handleGetList(apiKey, client, w, r)
		// duplicates an existing distribution list -> admin or creator
		case "duplicateList":
			handleDuplicateList(apiKey, client, w, r)
		// exports the members of a distribution list
		case "exportList":
			handleExportList(apiKey, client, w, r)
		// retrieves all groups to show in the distribution list creation / update
		case "getGroups":
			handleGetGroups(apiKey, client, w, r)
		// retrieves all fundings to show in the distribution list creation / update
		case "getFundings":
			handleGetFundings(apiKey, client, w, r)
		// deletes a distribution list -> admin or creator
		case "deleteList":
			handleDeleteList(apiKey, client, w, r)
		// sends a notification to a distribution list
		case "notificationToList":
			handleSendNotification(apiKey, client, w, r)
		// checks user information and returns it
		case "checkUserInformation":
			handleCheckUserInformation(apiKey, client, w, r)

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
