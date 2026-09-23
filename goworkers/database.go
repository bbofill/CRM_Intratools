package goworkers

// database access and settings

import (
	"database/sql"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// global vars for db interacion
var (
	mutex sync.Mutex
	db    *sql.DB
)

// start db connection
func initDB(dbFile string) {
	var err error
	mutex.Lock()
	defer mutex.Unlock()

	db, err = sql.Open("sqlite", dbFile)
	if err != nil {
		AddControllerLog("Error openning db:", 2)
	}
	_, err = db.Exec("PRAGMA busy_timeout=3000; PRAGMA journal_mode=WAL; PRAGMA foreign_keys = ON;")
	if err != nil {
		AddControllerLog("Error on init db:", 2)
	}
}

// close db connection
func closeDB() {
	mutex.Lock()
	defer mutex.Unlock()

	if db != nil {
		db.Close()
	}
}

// execute write query
func executeInsert(query string, args ...interface{}) {
	mutex.Lock()
	defer mutex.Unlock()

	_, err := db.Exec(query, args...)
	if err != nil {
		AddControllerLog("Error running INSERT: "+query, 2)
	}
}

// execute read-only query
func executeQuery(query string, args ...interface{}) (results []map[string]interface{}) {
	rows, err := db.Query(query, args...)
	if err != nil {
		AddControllerLog("Error running SELECT: "+query, 2)
		return []map[string]interface{}{}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		AddControllerLog("Error parsing query db data: "+query, 2)
		return []map[string]interface{}{}
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			AddControllerLog("Error scanning db rows: "+query, 2)
			return []map[string]interface{}{}
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		AddControllerLog("Error parsing rows: "+query, 2)
	}

	if len(results) == 0 {
		results = []map[string]interface{}{}
	}

	return results
}

// create database tables
func createDBTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS people (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			picture_path TEXT,
			name TEXT CHECK(LENGTH(name) <= 20),
			prefered_name TEXT CHECK(LENGTH(prefered_name) <= 20),
			surname TEXT CHECK(LENGTH(surname) <= 20), 
			secondSurname TEXT CHECK(LENGTH(secondSurname) <= 20),
			gender TEXT CHECK(LENGTH(gender) = 1 AND gender IN ('H', 'D')),
			birth_date DATE,
			birth_country TEXT, 
			birth_province TEXT,
			birth_city TEXT,
			nif TEXT UNIQUE CHECK(LENGTH(nif) <= 10),
			nif_extended TEXT UNIQUE CHECK(LENGTH(nif_extended) <= 20),
			user_phone INTEGER,
			emergencyContact_name TEXT CHECK(LENGTH(emergencyContact_name) <= 20),
			emergencyContact_phone INTEGER,
			user_email TEXT CHECK(LENGTH(user_email) <= 50),
			people_idExternal TEXT UNIQUE CHECK(LENGTH(people_idExternal) <= 20),
			webUser_idExternal TEXT UNIQUE CHECK(LENGTH(webUser_idExternal) <= 20),
			crm_email TEXT UNIQUE CHECK(LENGTH(crm_email) <= 50),
			observations TEXT CHECK(LENGTH(observations) <= 1000),
			academic_grade TEXT CHECK((LENGTH(academic_grade) = 1 AND academic_grade IN ('1','2','3','4','5','6','7','G'))), -- old codi_grau
			research_interests TEXT CHECK(LENGTH(research_interests) <= 100),
			orcid TEXT,
			certificat_I3 BOOL,
			personal_webPage TEXT CHECK(LENGTH(personal_webPage) <= 100),
			agreesToUneix BOOL,
			active BOOL,
			FOREIGN KEY(birth_country) REFERENCES country(code)
		);`,
		`CREATE TABLE IF NOT EXISTS residence (
			people_id INTEGER,
			residence_country TEXT, 
			residence_province TEXT,
			residence_city TEXT,
			postal_code TEXT,
			address TEXT CHECK(LENGTH(address) <= 200),
			actual BOOL,
			FOREIGN KEY(people_id) REFERENCES people(id),
			FOREIGN KEY(residence_country) REFERENCES country(code)
		);`,
		`CREATE TABLE IF NOT EXISTS responsible (
			people_id INTEGER,
			name TEXT TEXT CHECK(LENGTH(name) <= 100),
			tesis_id INTEGER,
			ip_or_tutor TEXT,
			PRIMARY KEY (tesis_id, name),
			FOREIGN KEY(people_id) REFERENCES people(id),
			FOREIGN KEY(tesis_id) REFERENCES people_phd(id)
		);`,
		`CREATE TABLE IF NOT EXISTS people_phd (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			people_id INTEGER,
			phd_program TEXT CHECK(LENGTH(phd_program) <= 500),
			phd_startYear INTEGER,
			phd_tesisDirector TEXT CHECK(LENGTH(phd_tesisDirector) <= 500),
			phd_university TEXT CHECK(LENGTH(phd_university) <= 100),
			phd_tesisTitle TEXT CHECK(LENGTH(phd_tesisTitle) <= 500),
			phd_plannedPresentationDate DATE,
			phd_presentationDate DATE,
			phd_link TEXT,
			FOREIGN KEY(people_id) REFERENCES people(id)
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			people_id INTEGER,
			username TEXT PRIMARY KEY,
			role TEXT NOT NULL,
			status TEXT,
			password TEXT NOT NULL,
			FOREIGN KEY(people_id) REFERENCES people(id)
		);`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			id_module TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			log_content TEXT NOT NULL,
			code INTEGER NOT NULL,
    		hash TEXT NOT NULL,
    		previous_hash TEXT NOT NULL
		);`, // module 1 tables:
		`CREATE TABLE IF NOT EXISTS room (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			uab_code TEXT,
			category TEXT,
			reservable BOOL
		);`,
		`CREATE TABLE IF NOT EXISTS roomBooking (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id INTEGER,
			concept TEXT, 
			start_date DATETIME,
			end_date DATETIME,
			email TEXT, 
			user_id INTEGER,
			FOREIGN KEY(room_id) REFERENCES room(id),
			FOREIGN KEY(user_id) REFERENCES people(id)
		);`, // module 2 tables:
		`CREATE TABLE IF NOT EXISTS reservation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_id INTEGER,
			start_date DATETIME,
			end_date DATETIME,
			FOREIGN KEY(table_id) REFERENCES hottable(id)
		);`,
		`CREATE TABLE IF NOT EXISTS hottable (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			is_hot BOOL,
			capacity INTEGER,
			floor INT,
			position TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS reservation_user (
			reservation_id INTEGER,
			user_id INTEGER,
			contract_id INTEGER,
			FOREIGN KEY (reservation_id) REFERENCES reservation(id),
			FOREIGN KEY (user_id) REFERENCES people(id),
			FOREIGN KEY (contract_id) REFERENCES contract(id)
		);`, // module 3, 4 and 5 tables:
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,           -- 'admin-message', 'profile-request', 'document'
			title TEXT NOT NULL,
			content TEXT,        -- puede ser texto, JSON o ruta de archivo
			file_path TEXT,      -- si hay un PDF o similar
			user_id INTEGER,     -- who created the notification
			created_at DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES people(id)
		);`,
		`CREATE TABLE IF NOT EXISTS notification_user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			notification_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,         -- who receives the notification
			can_read INTEGER DEFAULT 1,       -- 1 = puede ver la notificación
			can_download INTEGER DEFAULT 0,   -- 1 = puede acceder a file_path
			seen INTEGER DEFAULT 0,           -- 0 = no leída, 1 = leída
			profile_applied INTEGER DEFAULT 0,
			FOREIGN KEY (notification_id) REFERENCES notifications(id),
			FOREIGN KEY (user_id) REFERENCES people(id)
		);`,
		`CREATE TABLE IF NOT EXISTS researchGroup (          -- old uneix_Group
			intern_code TEXT PRIMARY KEY,    -- old codi_grupIntern
			research_code TEXT UNIQUE,               -- old codi_grupRecerca
			category TEXT,
			name TEXT,                               
			start_date DATE,                         -- old data_creacio
			end_date DATE
		);`,
		`CREATE TABLE IF NOT EXISTS researchArea (          
			id INTEGER PRIMARY KEY,   
			name TEXT UNIQUE,             
			categories TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS people_grade ( 
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			people_id INTEGER,
			grade_master_doctorate TEXT,
			code INTEGER,
			gradeName TEXT,
			universityName TEXT CHECK(LENGTH(universityName) <= 100),  -- only if university is other (98||99)
			graduation_university INTEGER,
			graduation_country INTEGER,
			graduation_year INTEGER,
			UNIQUE (people_id, grade_master_doctorate, code),
			FOREIGN KEY (people_id) REFERENCES people(id),
			FOREIGN KEY (code) REFERENCES studies(code),
			FOREIGN KEY (graduation_country) REFERENCES country(code)
		);`,
		`CREATE TABLE IF NOT EXISTS contract ( 
			id INTEGER PRIMARY KEY AUTOINCREMENT, 
			people_id INTEGER,
			vinculation_type TEXT CHECK(LENGTH(vinculation_type) <= 20), -- old codi_vinculacio
			trainee_type TEXT,
			internship BOOL, 
			trainee_studies INTEGER,
			start_date DATE,
			end_date DATE,
			job_category TEXT,  -- old codi_carrec
			type TEXT CHECK(LENGTH(type) <= 20),
			position TEXT,
			file_path TEXT,
			totalDedication_hours TEXT CHECK(LENGTH(totalDedication_hours) = 4 AND totalDedication_hours GLOB '[0-9][0-9][0-9][0-9]'),
			supervisor INTEGER,
			contracting_institution TEXT CHECK(LENGTH(contracting_institution) <= 20),
			belongs_to_contract_program BOOL NOT NULL DEFAULT 0,
			office_location INTEGER,
			research_area INTEGER,
			funding TEXT, -- old codi_subvencio
			FOREIGN KEY(research_area) REFERENCES researchArea(id),
			FOREIGN KEY(trainee_studies) REFERENCES people_grade(id),
			FOREIGN KEY(supervisor) REFERENCES people(id),
			FOREIGN KEY(office_location) REFERENCES room(id),
			FOREIGN KEY (people_id) REFERENCES people(id)
		);`,
		`CREATE TABLE IF NOT EXISTS training ( 
			id INTEGER PRIMARY KEY AUTOINCREMENT, 
			category TEXT,
			name TEXT CHECK(LENGTH(name) <= 200), 
			hours INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS people_training ( 
			training_id INTEGER, 
			people_id INTEGER,
			enrolled BOOL,
			completed BOOL,
			date DATE,
			diploma_path TEXT, 
			FOREIGN KEY (training_id) REFERENCES training(id),
			FOREIGN KEY (people_id) REFERENCES people(id)
		);`,
		`CREATE TABLE IF NOT EXISTS country ( 
			code TEXT PRIMARY KEY, 
			name_EN TEXT,
			name_CAT TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS people_nationality ( 
			nationality_code TEXT, 
			people_id INTEGER,
			PRIMARY KEY (people_id, nationality_code),
			FOREIGN KEY (people_id) REFERENCES people(id),
			FOREIGN KEY (nationality_code) REFERENCES country(code)
		);`,
		`CREATE TABLE IF NOT EXISTS studies ( 
			code INTEGER PRIMARY KEY, 
			name TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS people_group (
			people_id INTEGER,
			group_intern_code TEXT,
			start_date DATE,
			end_date DATE,
			ip BOOL, 
			PRIMARY KEY (people_id, group_intern_code),
			FOREIGN KEY (people_id) REFERENCES people(id),
			FOREIGN KEY (group_intern_code) REFERENCES researchGroup(intern_code)
		);`,
		`CREATE TABLE IF NOT EXISTS uneix_groupReconeixement (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			codi_entitat TEXT,
			codi_grupRecerca TEXT,
			codi_reconeixement TEXT,
			data_obtencio INTEGER,
			FOREIGN KEY (codi_grupRecerca) REFERENCES researchGroup(research_code)
		);`,
		`CREATE TABLE IF NOT EXISTS uneix_spinoffs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			codi_entitat TEXT,
			codi_ens TEXT,
			cif TEXT,
			nom TEXT,
			nif TEXT,
			nif_ampliat TEXT,
			codi_conacit TEXT,
			data_creacio TEXT,
			data_cessio TEXT,
			perc_participacio TEXT,
			data_extincio TEXT,
			area_cnae TEXT,
			ext_o_fin TEXT
		);`,
	}
	for _, query := range queries {
		executeInsert(query)
	}
}

func dbConnect() func() {
	initDB(mC.DBFilePath) // set db connection
	createDBTables()      // create main tables
	return closeDB        // return for closing on server() end
}

// constructs a basic SELECT query
func buildSelectQuery(table string, columns string, condition string) string {
	query := "SELECT " + columns + " FROM " + table
	if condition != "" {
		query += " WHERE " + condition
	}
	return query
}

// constructs a basic INSERT SQL query
func buildInsertQuery(table string, columns []string, placeholders []string) string {
	return "INSERT INTO " + table + " (" + strings.Join(columns, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ")"
}

// constructs a basic UPDATE SQL query
func buildUpdateQuery(table string, columns []string, placeholders []string, condition string) string {
	assignments := make([]string, len(columns))
	for i := range columns {
		assignments[i] = columns[i] + "=" + placeholders[i]
	}
	query := "UPDATE " + table + " SET " + strings.Join(assignments, ", ")
	if condition != "" {
		query += " WHERE " + condition
	}
	return query
}

// constructs a basic DELETE SQL query
func buildDeleteQuery(table string, condition string) string {
	return "DELETE FROM " + table + " WHERE " + condition
}

// ToDo: modules should have a way to recreate his own tables on delete db.
