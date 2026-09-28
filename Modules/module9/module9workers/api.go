package module9workers

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
		case "checkUserInformation":
			handleCheckUserInformation(apiKey, client, w, r)
		case "fetchModalities":
			handleFetchModalities(apiKey, client, w, r)
		case "createModality":
			handleCreateModality(apiKey, client, w, r)
		case "fetchDocumentsByModality":
			handleFetchDocumentsByModality(apiKey, client, w, r)
		case "getPath":
			handleGetPath(apiKey, client, w, r)
		case "uploadDocument":
			handleUploadDocument(apiKey, client, w, r)
		case "updateDocument":
			handleUpdateDocument(apiKey, client, w, r)
		case "deleteDocument":
			handleDeleteDocument(apiKey, client, w, r)
		case "updateModality":
			handleUpdateModality(apiKey, client, w, r)
		case "deleteModality":
			handleDeleteModality(apiKey, client, w, r)
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
