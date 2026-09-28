package module0workers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// handle GET logs (paginated)
func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	numWorkers := 2 * runtime.NumCPU()
	semaphore := make(chan struct{}, numWorkers)

	limit := 200
	offset := 0

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	req, _ := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			"https://localhost:%s/module/api/log?limit=%d&offset=%d",
			cfg.BackendPort,
			limit,
			offset,
		),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)

	client := &http.Client{Transport: setInsecureRequest()}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "backend unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	//fmt.Println("limit =", limit, "offset =", offset)

	var logs []LogEntry
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		http.Error(w, "invalid backend response", http.StatusInternalServerError)
		return
	}

	for i := range logs {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(i int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			plain, err := decryptLogWithGPG(logs[i].Msg)
			if err == nil {
				logs[i].Msg = plain
			} else {
				logs[i].Msg = "error on decrypt or debase64ing"
			}
		}(i)
	}

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// handle post (validation)
func handlePostLogs(w http.ResponseWriter) {
	logs, err := getLogChainFromController()
	if err != nil {
		http.Error(w, "error fetching logs", http.StatusBadGateway)
		return
	}

	prev := "GENESIS"
	for _, l := range logs {
		expected := hashString(fmt.Sprintf(
			"PreviousHash:%s;Timestamp:%d;Msg:%s;Code:%d",
			l.PreviousHash,
			l.Timestamp,
			l.Msg,
			l.Code,
		))
		if l.PreviousHash != prev {
			writeJSON(w, map[string]interface{}{
				"status": "broken",
				"at":     l.ID,
				"reason": "previous hash mismatch",
			})
			return
		}
		if l.Hash != expected {
			writeJSON(w, map[string]interface{}{
				"status": "broken",
				"at":     l.ID,
				"reason": "hash mismatch",
			})
			return
		}
		prev = l.Hash
	}

	writeJSON(w, map[string]interface{}{
		"status": "valid",
		"count":  len(logs),
	})
}

func getLogChainFromController() ([]LogEntry, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://localhost:"+cfg.BackendPort+"/module/api/log?mode=verify", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.ModuleApiKey)

	client := &http.Client{Transport: setInsecureRequest()} // no if !debug
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var logs []LogEntry
	err = json.NewDecoder(resp.Body).Decode(&logs)
	return logs, err
}

// decrypt cookie to be readeable on frontend
func decryptLogWithGPG(encrypted string) (string, error) {
	privKeyFile, err := os.Open("../../" + cfg.LogCypherPrivKeyFilePath)
	if err != nil {
		return "", fmt.Errorf("could not open private key: %w", err)
	}
	defer privKeyFile.Close()

	entities, err := openpgp.ReadArmoredKeyRing(privKeyFile)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	for _, entity := range entities {
		if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
			err := entity.PrivateKey.Decrypt([]byte(cfg.LogCypherKeyPassphrase))
			if err != nil {
				return "", fmt.Errorf("cannot decrypt private key: %w", err)
			}
		}

		for _, sub := range entity.Subkeys {
			if sub.PrivateKey != nil && sub.PrivateKey.Encrypted {
				err := sub.PrivateKey.Decrypt([]byte(cfg.LogCypherKeyPassphrase))
				if err != nil {
					return "", fmt.Errorf("cannot decrypt subkey: %w", err)
				}
			}
		}
	}

	// now is readeable
	debase64d, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}
	md, err := openpgp.ReadMessage(bytes.NewReader(debase64d), entities, nil, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}
	plaintext, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}

	return string(plaintext), nil
}
