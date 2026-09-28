package goworkers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func TutorialStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, _, _ := getUserInfoTutorial(r)
	module := r.URL.Query().Get("module")
	page := r.URL.Query().Get("page")

	if module == "" || page == "" {
		http.Error(w, "Missing module or page parameter", http.StatusBadRequest)
		return
	}

	query := buildSelectQuery("tutorial_progress", "1", fmt.Sprintf("user_id = '%d' AND module_name = '%s' AND page_name = '%s'", userID, module, page))
	result := executeQuery(query)

	if len(result) == 0 {
		json.NewEncoder(w).Encode(map[string]bool{"completed": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"completed": true})
}

func TutorialCompleteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, _, _ := getUserInfoTutorial(r)

	var body map[string]string
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	module, ok1 := body["module"]
	page, ok2 := body["page"]
	if !ok1 || !ok2 || module == "" || page == "" {
		http.Error(w, "Missing module or page", http.StatusBadRequest)
		return
	}

	// Insert record if not exists
	columns := []string{"user_id", "module_name", "page_name"}
	values := []string{
		fmt.Sprintf("%d", userID),
		module,
		page,
	}
	query := buildInsertQuery("tutorial_progress", columns, values)

	result := executeQuery(query)
	if len(result) == 0 {
		http.Error(w, "Error saving skip tutorial", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func TutorialResetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, _, _ := getUserInfoTutorial(r)

	query := buildDeleteQuery("tutorial_progress", fmt.Sprintf("user_id = '%d'", userID))

	_ = executeQuery(query)

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func TutorialCheckRoleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, _, role := getUserInfoTutorial(r)
	json.NewEncoder(w).Encode(map[string]string{"role": role})
}

// returns userID, username, role for tutorial system
func getUserInfoTutorial(r *http.Request) (int, string, string) {
	// get auth cookie
	c, err := r.Cookie(cOoKiEnAmE)
	if err != nil {
		return 0, "", "guest"
	}

	claims, ok := parseAuthCookie(c.Value, nil)
	if !ok {
		return 0, "", "guest"
	}

	username := getUsernameByHash(claims.Sub)

	// Fetch ID and role directly from SQLite
	query := buildSelectQuery("users", "people_id, role", fmt.Sprintf("username = '%s'", username))
	result := executeQuery(query)
	if len(result) == 0 {
		return 0, username, "guest"
	}

	row := result[0]
	userID, _ := row["people_id"].(int64)
	userRole, _ := row["role"].(string)

	return int(userID), username, userRole
}
