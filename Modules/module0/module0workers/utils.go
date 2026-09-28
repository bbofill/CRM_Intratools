package module0workers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// fill config obj with json data
func LoadConfigData(path string) {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}
	if err := json.Unmarshal(file, &cfg); err != nil {
		log.Fatal("Error parsing config: ", err)
	}
}

// verifyAPIKey
func verifyAPIKey(inkey string) (isthesame bool) {
	if cfg.ModuleApiKey == inkey {
		isthesame = true
	}
	return isthesame
}

// hash string
func hashString(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

// write json response (base64?)
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	beautifiedJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Error Generating Response JSON", http.StatusInternalServerError)
		return
	}
	w.Write(beautifiedJSON)
}

// safe access to db

// -----------------------------------------------------------------------------------------------------------------
func sendReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	url := "https://localhost:" + cfg.BackendPort + "/module/api/db"

	reqInsert, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creando la solicitud HTTP:", err)
		http.Error(w, "Internal request creation error", http.StatusInternalServerError)
		return nil
	}

	reqInsert.Header.Set("Content-Type", "application/json")
	reqInsert.Header.Set("Authorization", "Bearer "+apiKey)

	respInsert, err := client.Do(reqInsert)
	if err != nil {
		fmt.Printf("Error al enviar la petición POST a %s: %v\n", url, err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return nil
	}
	return respInsert
}

// ------------------------------- http requests ------------------------------------------
func getReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "GET"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReq(modifiedJSON, apiKey, client, w)
}
func postReq(jsonData []byte, apiKey string, client *http.Client, w http.ResponseWriter) *http.Response {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("Error unmarshaling jsonData: %v\n", err)
		return nil
	}
	data["method"] = "POST"
	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling modified JSON: %v\n", err)
		return nil
	}
	return sendReq(modifiedJSON, apiKey, client, w)
}
