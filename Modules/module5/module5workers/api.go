package module5workers

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
		//fmt.Println("MODULE5 action =", action, "url =", r.URL.String())

		switch action {
		// --------- UNEIX DATA EXTRACTION ---------

		// table 46
		// returns all workers to front-end with a list of missing fields for each worker
		case "missingMandatoryFields":
			handleIncompleteUsers(apiKey, client, w, r)
			// updates database information
		case "updateFields":
			handleUpdateFields(apiKey, client, w, r)
		case "updateVisitor":
			handleUpdateVisitors(apiKey, client, w, r)

			//table 03
		case "missingMandatoryFieldsUnits":
			handleIncompleteUnits(apiKey, client, w, r)
		case "getAllUnits":
			handleGetUnits(apiKey, client, w, r)

			// table 48
			// returns all groups to frontend with a list of missing fields for each group
		case "missingMandatoryFieldsGroups":
			handleIncompleteGroups(apiKey, client, w, r)
			// returns all groups to frontend
		case "getAllGroups":
			handleGetGroups(apiKey, client, w, r)

			// table 50
		case "missingMandatoryFieldsGroupRecognition":
			handleIncompleteGroupsRecognition(apiKey, client, w, r)
		case "getAllGroupsRecognition":
			handleGetGroupsRecognition(apiKey, client, w, r)

			// table 52
		case "missingMandatoryFieldsGroupMembers":
			handleIncompleteGroupMembers(apiKey, client, w, r)
		case "getAllGroupMembers":
			handleGetGroupMembers(apiKey, client, w, r)

			// table 64
		case "exportUneixSpinOffs":
			handleExportUneixSpinOffs(apiKey, client, w)

			// --------- EMPLOYEE DATA EXTRACTION ---------
		case "employeeExport":
			handleExportEmployeeData(apiKey, client, w, r)
		case "exportCSV":
			handleExportCSV(apiKey, client, w, r)

			//ACCESS CONTROL TO WORKER DATA
		case "checkRole":
			handleCheckRole(apiKey, client, w, r)
		case "getUsersAccess":
			handleGetUsersAccess(apiKey, client, w, r)
		case "setUsersAccess":
			handleSetUsersAccess(apiKey, client, w, r)
		case "checkAccess":
			handleCheckAccess(apiKey, client, w, r)
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
