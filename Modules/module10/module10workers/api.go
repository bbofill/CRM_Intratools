package module10workers

import (
	"fmt"
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
		//fmt.Println("[module10] API request", r.Method, r.URL.Path, r.URL.RawQuery, "action=", action)
		switch action {
		case "session":
			handleServiceCommissionSession(apiKey, client, w, r)
		case "projects":
			handleGetProjects(apiKey, client, w, r)
		case "available-travels":
			handleAvailableTravels(apiKey, client, w, r)
		case "create-commission":
			handleCreateCommission(apiKey, client, w, r)
		case "my-commissions":
			handleMyCommissions(apiKey, client, w, r)
		case "project-commissions":
			handleProjectCommissions(apiKey, client, w, r)
		case "commission-detail":
			handleCommissionDetail(apiKey, client, w, r)
		case "approve-expense":
			handleApproveExpense(apiKey, client, w, r)
		case "approve-commission":
			handleApproveCommission(apiKey, client, w, r)
		case "save-commission-review-fields":
			handleSaveCommissionReviewFields(apiKey, client, w, r)
		case "reject-commission":
			handleRejectCommission(apiKey, client, w, r)
		case "stakeholder-commissions", "final-approvals":
			handleStakeholderCommissions(apiKey, client, w, r)
		case "approve-stakeholder", "approve-final-approval":
			handleApproveStakeholder(apiKey, client, w, r)
		case "download-final-pdf", "commission-pdf":
			handleDownloadFinalPDF(apiKey, client, w, r)
		case "preview-commission-pdf", "previewCommissionPdf", "preview-commission", "preview-pdf":
			fmt.Println("[module10] handling PDF preview action", action)
			handlePreviewCommissionPDF(apiKey, client, w, r)
		case "add-center-expense":
			handleAddCenterExpense(apiKey, client, w, r)
		case "delete-center-expense":
			handleDeleteCenterExpense(apiKey, client, w, r)
		case "exclude-expense":
			handleExcludeExpense(apiKey, client, w, r)
		case "export-commissions":
			handleExportCommissions(apiKey, client, w, r)
		case "data-export-options":
			handleServiceCommissionDataExportOptions(apiKey, client, w, r)
		case "data-export":
			handleServiceCommissionDataExport(apiKey, client, w, r)
		case "pricing-tables":
			handleGetPricingTables(apiKey, client, w, r)
		case "save-pricing-table":
			handleSavePricingTable(apiKey, client, w, r)
		case "delete-pricing-table":
			handleDeletePricingTable(apiKey, client, w, r)
		case "save-expense-pricing-review":
			handleSaveExpensePricingReview(apiKey, client, w, r)
		case "save-mileage-review":
			handleSaveMileageReview(apiKey, client, w, r)
		case "save-expense-advance":
			handleSaveExpenseAdvance(apiKey, client, w, r)
		case "save-expense-edit":
			handleSaveExpenseEdit(apiKey, client, w, r)
		default:
			fmt.Println("[module10] invalid action received", action, "rawQuery=", r.URL.RawQuery)
			http.Error(w, "Invalid action: "+action, http.StatusBadRequest)
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
