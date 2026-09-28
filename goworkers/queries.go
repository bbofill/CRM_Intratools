package goworkers

import (
	"database/sql"
	"fmt"
)

// obtain last log from db log table
func getPreviousLogHash(tx *sql.Tx) string {
	row := tx.QueryRow(`SELECT hash FROM logs ORDER BY id DESC LIMIT 1`)
	var hash string
	err := row.Scan(&hash)
	if err != nil {
		return "GENESIS"
	}
	return hash
}

// get hash user string return user unhashed
func getUsernameByHash(userHash string) string {
	query := buildSelectQuery("users", "username", "1=1") + " LIMIT 1000"
	results := executeQuery(query)
	for _, row := range results {
		uname, _ := row["username"].(string)
		if hashString(uname) == userHash {
			return uname
		}
	}
	return "superadmin"
}

func getName(username string) (string, string) {
	query := buildSelectQuery("people p LEFT JOIN users u ON p.id = u.people_id", "name,surname", fmt.Sprintf("u.username='%s'", username)) + " LIMIT 1"
	results := executeQuery(query)
	if len(results) > 0 {
		name, _ := results[0]["name"].(string)
		surname, _ := results[0]["surname"].(string)
		return name, surname
	}
	return "Admin", "User"
}
