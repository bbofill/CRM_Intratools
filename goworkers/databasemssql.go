package goworkers

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/microsoft/go-mssqldb"
)

var (
	mssqlMutex sync.Mutex
	dbMSSQL    *sql.DB
)

// Initialize MSSQL connection
func initMSSQL(host, port, user, pass, database string) {
	mssqlMutex.Lock()
	defer mssqlMutex.Unlock()

	// Example: sqlserver://user:pass@hostname:1433?database=mydb
	connString := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
		user, pass, host, port, database)

	var err error
	dbMSSQL, err = sql.Open("sqlserver", connString)
	if err != nil {
		AddControllerLog("MSSQL OPEN ERROR: "+err.Error(), 2)
		return
	}

	if err = dbMSSQL.Ping(); err != nil {
		AddControllerLog("MSSQL CONNECTION ERROR: "+err.Error(), 2)
		return
	}

	AddControllerLog("MSSQL connected OK", 1)
}

// Close MSSQL connection
func closeMSSQL() {
	mssqlMutex.Lock()
	defer mssqlMutex.Unlock()

	if dbMSSQL != nil {
		dbMSSQL.Close()
	}
}

// READ (SELECT)
func queryMSSQL(query string) ([]map[string]interface{}, error) {
	mssqlMutex.Lock()
	rows, err := dbMSSQL.Query(query)
	mssqlMutex.Unlock()

	if err != nil {
		AddControllerLog("MSSQL SELECT ERROR: "+query, 2)
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		AddControllerLog("MSSQL PARSE COLUMNS ERROR: "+err.Error(), 2)
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			AddControllerLog("MSSQL SCAN ERROR: "+err.Error(), 2)
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		AddControllerLog("MSSQL ROW ERROR: "+err.Error(), 2)
		return nil, err
	}

	return results, nil
}

// WRITE (INSERT, UPDATE, DELETE)
func execMSSQL(query string) error {
	mssqlMutex.Lock()
	_, err := dbMSSQL.Exec(query)
	mssqlMutex.Unlock()

	if err != nil {
		AddControllerLog("MSSQL EXEC ERROR: "+query+" | "+err.Error(), 2)
		return err
	}
	return nil
}
