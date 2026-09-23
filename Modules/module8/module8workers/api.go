package module8workers

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
		case "checkRole":
			handleCheckRole(apiKey, client, w, r)
		case "getUsersAccess":
			handleGetUsersAccess(apiKey, client, w, r)
		case "setUsersAccess":
			handleSetUsersAccess(apiKey, client, w, r)
		case "getProjects":
			handleGetProjects(apiKey, client, w, r)
		case "getAllProjects":
			handleGetAllProjects(apiKey, client, w, r)
		case "submitRequest":
			handleSubmitRequest(apiKey, client, w, r)
		case "getSentRequests":
			handleGetSentRequests(apiKey, client, w, r)
		case "getBudgetPermissions":
			handleGetBudgetPermissions(apiKey, client, w, r)
		case "getManagementRequests":
			handleGetManagementRequests(apiKey, client, w, r)
		case "getAllRequests":
			handleGetAllRequests(apiKey, client, w, r)
		case "exportAllRequestsExcel":
			handleExportAllRequestsExcel(apiKey, client, w, r)
		case "cancelRequest":
			handleCancelRequest(apiKey, client, w, r)
		case "addInvoice":
			handleAddInvoice(apiKey, client, w, r)
		case "modifyRequest":
			handleModifyRequest(apiKey, client, w, r)
		case "rejectRequest":
			handleRejectRequest(apiKey, client, w, r)
		case "acceptRequest":
			handleAcceptRequest(apiKey, client, w, r)
		case "getTravelContactInfo":
			handleGetTravelContactInfo(apiKey, client, w, r)
		case "exportFilteredRequestsExcel":
			handleExportFilteredRequestsExcel(apiKey, client, w, r)
		case "getPendingAttendanceCertificates":
			handleGetPendingAttendanceCertificates(apiKey, client, w, r)
		case "uploadAttendanceCertificate":
			handleUploadAttendanceCertificate(apiKey, client, w, r)
		case "markAttendanceCertificateViewed":
			handleMarkAttendanceCertificateViewed(apiKey, client, w, r)
		case "runAttendanceCertificateReminders":
			handleRunAttendanceCertificateReminders(apiKey, client, w, r)
		case "verifyBudgetAuditChain":
			handleVerifyBudgetAuditChain(apiKey, client, w, r)
		case "exportBudgetAuditPDF":
			handleExportBudgetAuditPDF(apiKey, client, w, r)
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
			createLog("Invalid action requested: "+action, 2, apiKey, nil, w)
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
