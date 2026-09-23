package module6workers

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

		// control de sidebar i pestanyes a les que es pot accedir
		case "checkRole":
			handleSendUserRole(apiKey, client, w, r)
		case "getUserInfo":
			handleGetUserInfo(apiKey, client, w, r)
		case "getManagedDepartments":
			handleGetManagedDepartments(apiKey, client, w, r)
		case "getUserDepartments":
			handleGetUserDepartments(apiKey, client, w, r)

		//administració de departaments
		case "getDepartments":
			handleGetDepartments(apiKey, client, w, r)
		case "getDepartmentWorkers":
			handleGetDepartmentWorkers(apiKey, client, w, r)
		case "getAllWorkers":
			handleGetAllWorkers(apiKey, client, w, r)
		case "createDepartment":
			handleCreateDepartment(apiKey, client, w, r)
		case "deleteDepartment":
			handleDeleteDepartment(apiKey, client, w, r)
		case "addWorkerToDepartment":
			handleAddWorkerToDepartment(apiKey, client, w, r)
		case "updateWorkerManagerStatus":
			handleUpdateWorkerManagerStatus(apiKey, client, w, r)
		case "removeWorkerFromDepartment":
			handleRemoveWorkerFromDepartment(apiKey, client, w, r)
		case "downloadDeptTickets":
			handleDownloadDepartmentTickets(apiKey, client, w, r)
		case "downloadDeptHistory":
			handleDownloadDepartmentTicketHistory(apiKey, client, w, r)

		//creacio de tickets
		case "createTicket":
			handleCreateTicket(apiKey, client, w, r)
		case "getUserEmails":
			handleSendEmail(apiKey, client, w, r)

		//assignacio de tickets
		case "getDepartmentTicketsAndWorkers":
			handleGetDepartmentTicketsAndWorkers(apiKey, client, w, r)
		case "assignDepartmentTicket":
			handleAssignDepartmentTicket(apiKey, client, w, r)
		case "getDepartmentTicketsAndWorkersAssigned":
			handleGetDepartmentTicketsAndWorkersAssigned(apiKey, client, w, r)

			//work area
		case "getAssignedTickets":
			handleGetAssignedTickets(apiKey, client, w, r)
		case "updateTicketStatus":
			handleUpdateTicketStatus(apiKey, client, w, r)

			//visualize your tickets
		case "getMyTickets":
			handleGetMyTickets(apiKey, client, w, r)

			//tancament de tickets
		case "confirmCloseTicket":
			handleConfirmCloseTicket(apiKey, client, w, r)
		case "reopenTicket":
			handleReopenTicket(apiKey, client, w, r)
		case "replyTicket":
			handleReplyTicket(apiKey, client, w, r)

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
