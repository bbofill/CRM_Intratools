package module0workers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

var cfg Config

// forward to controller backend
func sendToBackend(w http.ResponseWriter, payload map[string]interface{}) {
	jsonPayload, _ := json.Marshal(payload)
	client := &http.Client{Transport: setInsecureRequest()}

	req, _ := http.NewRequest(http.MethodPost, "https://localhost:"+cfg.BackendPort+"/module/api/db", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		log.Printf("Error contacting backend: %v", err)
		http.Error(w, "Backend error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// trust certs if development
func setInsecureRequest() *http.Transport {
	if cfg.Development {
		return &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}}
}

// if development accept untrusted certs
func setInsecureRequestClient() (client *http.Client) {
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // best add used certs exception
		},
	}
	return client
}

// set available server routes
func handler() {
	http.HandleFunc("/api/users", handleUsers) // toDo: authorize routes
	http.HandleFunc("/api/module", handleModuleControl)
	http.HandleFunc("/api/logs", handleLogs)
}

// start web server
func StartServer() {
	handler()
	log.Println("Servidor en https://localhost:" + cfg.ListenPort)
	log.Fatal(http.ListenAndServeTLS(":"+cfg.ListenPort, "../../"+cfg.TlsCertPath, "../../"+cfg.TlsKeyPath, nil))
}
