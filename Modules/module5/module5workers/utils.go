package module5workers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tealeg/xlsx"
)

// --------------------------------------------------------------------------------------------------------
// --------------------------------------------------------------------------------------------------------
// --------------------------------- EXPLOTACIÓ DE DADES UNEIX ---- ---------------------------------------
// --------------------------------------------------------------------------------------------------------
// --------------------------------------------------------------------------------------------------------

// sends all users with missing fields
func handleIncompleteUsers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	// Año de referencia (por defecto, el año actual)
	targetYear := time.Now().Year() - 1

	// Leer ?year=YYYY si viene del front
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil && y > 1900 && y < 3000 {
			targetYear = y
		}
	}

	// Fechas de inicio y fin de ese año
	startOfYearStr := fmt.Sprintf("%04d-01-01", targetYear)
	endOfYearStr := fmt.Sprintf("%04d-12-31", targetYear)

	// ignorem id crmadmin (304)
	condition := fmt.Sprintf(`
    (
        p.active = 1
        OR EXISTS (
            SELECT 1
            FROM contract c2
            WHERE c2.people_id = p.id
            AND (
                c2.start_date <= date('%s')
                AND (c2.end_date IS NULL OR c2.end_date >= date('%s'))
            )
        )
    )
    AND p.agreesToUneix = 1 AND p.id != 304
    `, endOfYearStr, startOfYearStr)
	query := map[string]interface{}{
		"table": `people p
				  LEFT JOIN contract c ON p.id = c.people_id
				  LEFT JOIN (
						SELECT g1.*
						FROM people_grade g1
						INNER JOIN (
							SELECT
								people_id,
								grade_master_doctorate,
								MAX(CAST(graduation_year AS INTEGER)) AS latest_year
							FROM people_grade
							WHERE grade_master_doctorate IS NOT NULL
							GROUP BY people_id, grade_master_doctorate
						) g2 ON g1.people_id = g2.people_id
							AND g1.grade_master_doctorate = g2.grade_master_doctorate
							AND CAST(g1.graduation_year AS INTEGER) = g2.latest_year
					) pg ON p.id = pg.people_id
				  LEFT JOIN people_nationality n ON p.id = n.people_id
				  LEFT JOIN people_group pg_group ON p.id = pg_group.people_id
		  		  LEFT JOIN researchGroup g ON pg_group.group_intern_code = g.intern_code`,
		"columns": `
			-- Campos de people
			p.id AS people_id,
			p.name AS people_name,
			p.surname,
			p.secondSurname,
			p.gender,
			p.birth_date,
			p.birth_country,
			p.birth_province,
			p.birth_city,
			p.nif,
			p.nif_extended,
			p.academic_grade,
			p.orcid,
			p.certificat_I3,
			p.agreesToUneix,
			p.active,
			p.picture_path,

			-- Campos de contract
			c.id AS contract_id,
			c.vinculation_type,
			c.start_date,
			c.end_date,
			c.type,
			c.job_category,
			c.totalDedication_hours,
			c.contracting_institution,
			c.funding,

			-- Campos de people_grade
			pg.id AS grade_id,
			pg.grade_master_doctorate,
			pg.graduation_university,
			pg.graduation_country,
			pg.graduation_year,

			-- Campos de people_nationality
			n.nationality_code,

			-- Campos de people_group / groups
			g.type AS group_type
		`,
		"condition": condition,
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch incomplete users: getReq returned nil", 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response for incomplete users: %v", err), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(body, &users); err != nil {
		createLog(fmt.Sprintf("Error parsing DB JSON for incomplete users: %v", err), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}

	grouped := make(map[int]map[string]interface{})

	for _, u := range users {
		idFloat, ok := u["people_id"].(float64)
		if !ok {
			continue
		}
		id := int(idFloat)

		if _, exists := grouped[id]; !exists {
			grouped[id] = map[string]interface{}{
				"people_id":     u["people_id"],
				"people_name":   u["people_name"],
				"surname":       u["surname"],
				"secondSurname": u["secondSurname"],
				"contracts":     []map[string]interface{}{},
				"grades":        []map[string]interface{}{},
				"nationalities": []map[string]interface{}{},
				"groups":        []map[string]interface{}{},
				"general": map[string]interface{}{
					"gender":         u["gender"],
					"birth_date":     u["birth_date"],
					"birth_country":  u["birth_country"],
					"birth_province": u["birth_province"],
					"birth_city":     u["birth_city"],
					"nif":            u["nif"],
					"nif_extended":   u["nif_extended"],
					"academic_grade": u["academic_grade"],
					"orcid":          u["orcid"],
					"certificat_I3":  u["certificat_I3"],
					"agreesToUneix":  u["agreesToUneix"],
					"active":         u["active"],
					"picture_path":   u["picture_path"],
				},
			}
		}

		person := grouped[id]

		contract := map[string]interface{}{
			"contract_id":             u["contract_id"],
			"vinculation_type":        u["vinculation_type"],
			"start_date":              u["start_date"],
			"end_date":                u["end_date"],
			"type":                    u["type"],
			"job_category":            u["job_category"],
			"totalDedication_hours":   u["totalDedication_hours"],
			"contracting_institution": u["contracting_institution"],
			"funding":                 u["funding"],
		}
		person["contracts"] = append(person["contracts"].([]map[string]interface{}), contract)

		grade := map[string]interface{}{
			"grade_id":               u["grade_id"],
			"grade_master_doctorate": u["grade_master_doctorate"],
			"graduation_university":  u["graduation_university"],
			"graduation_country":     u["graduation_country"],
			"graduation_year":        u["graduation_year"],
		}
		person["grades"] = append(person["grades"].([]map[string]interface{}), grade)

		nationality := map[string]interface{}{
			"nationality_code": u["nationality_code"],
		}
		person["nationalities"] = append(person["nationalities"].([]map[string]interface{}), nationality)

		group := map[string]interface{}{
			"group_type": u["group_type"],
		}
		person["groups"] = append(person["groups"].([]map[string]interface{}), group)
	}

	// marquem camps obligatoris que falten
	for _, person := range grouped {

		missing := []string{}
		g := person["general"].(map[string]interface{})

		//tabla people
		if isEmpty(g["gender"]) {
			missing = append(missing, "people.gender")
		}
		if isEmpty(g["nif"]) {
			missing = append(missing, "people.nif")
		}
		if isEmpty(g["nif_extended"]) {
			if val, ok := g["nif"].(string); ok {
				g["nif_extended"] = val
			}
		}
		if isEmpty(g["birth_date"]) {
			missing = append(missing, "people.birth_date")
		}
		if isEmpty(g["academic_grade"]) {
			missing = append(missing, "people.academic_grade")
		}

		if isEmpty(g["certificat_I3"]) {
			missing = append(missing, "people.certificat_I3")
		}

		nationalities := person["nationalities"].([]map[string]interface{})
		var nationality string
		if len(nationalities) > 0 {
			nationality, _ = nationalities[0]["nationality_code"].(string)
		}
		if nationality == "" {
			missing = append(missing, "people_nationality.nationality_code")
		}

		if nat, ok := g["nationality_code"].(string); ok && nat != "" {
			if nat == "724" { // España
				if isEmpty(g["birth_province"]) {
					missing = append(missing, "people.birth_province")
				}
				if province, ok := g["birth_province"].(string); ok {
					switch province {
					case "08", "17", "25", "43": // provincias de Cataluña
						if isEmpty(g["birth_city"]) {
							missing = append(missing, "people.birth_city")
						}
					}
				}
			} else { // Nacionalidad distinta de España
				if isEmpty(g["birth_country"]) {
					missing = append(missing, "people.birth_country")
				}
			}
		}

		contractsRaw := person["contracts"].([]map[string]interface{})

		// --- DEDUPLICAR contratos por contract_id ---
		seen := make(map[int]bool)
		contracts := []map[string]interface{}{}
		for _, c := range contractsRaw {

			if cid, ok := c["contract_id"].(float64); ok {
				id := int(cid)
				if !seen[id] {
					seen[id] = true
					contracts = append(contracts, c)
				}
			}
		}

		// Determinar cuál es el contrato más reciente (por fecha de inicio)
		var latestContractIndex int
		var latestStartDate, earliestStartDate time.Time
		for i, c := range contracts {
			if startStr, ok := c["start_date"].(string); ok && startStr != "" {
				layouts := []string{
					time.RFC3339,          // 2025-08-01T00:00:00Z
					"2006-01-02",          // 2025-08-01
					"2006-01-02 15:04:05", // 2025-08-01 00:00:00
				}
				var t time.Time
				var parsed bool
				for _, layout := range layouts {
					if tt, err := time.Parse(layout, startStr); err == nil {
						t = tt
						parsed = true
						break
					}
				}
				if parsed {
					if t.After(latestStartDate) {
						latestStartDate = t
						latestContractIndex = i
					}
					if earliestStartDate.IsZero() || t.Before(earliestStartDate) {
						earliestStartDate = t
					}
				}
			}
		}

		if !earliestStartDate.IsZero() {
			g["first_incorporation_year"] = earliestStartDate.Year()
		} else {
			g["first_incorporation_year"] = nil
		}

		// --- FILTRAR CONTRATOS ACTIVOS EN EL AÑO DE REFERENCIA ---
		startOfYear := time.Date(targetYear, time.January, 1, 0, 0, 0, 0, time.UTC)
		endOfYear := time.Date(targetYear, time.December, 31, 23, 59, 59, 0, time.UTC)

		// Parsear fechas
		parseDate := func(s string) (time.Time, bool) {
			s = strings.TrimSpace(s)
			if s == "" {
				return time.Time{}, false
			}
			layouts := []string{
				time.RFC3339,          // 2025-08-01T00:00:00Z
				"2006-01-02",          // 2025-08-01
				"2006-01-02 15:04:05", // 2025-08-01 00:00:00
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, s); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		}

		filteredContracts := []map[string]interface{}{}

		for _, c := range contracts {
			var startDate, endDate time.Time
			var hasStart, hasEnd bool

			if s, ok := c["start_date"].(string); ok {
				startDate, hasStart = parseDate(s)
			}
			if e, ok := c["end_date"].(string); ok {
				endDate, hasEnd = parseDate(e)
			}

			activeThisYear := false
			if hasStart {
				if (!hasEnd && !startDate.After(endOfYear)) ||
					(hasEnd && !endDate.Before(startOfYear) && !startDate.After(endOfYear)) {
					activeThisYear = true
				}
			}

			if activeThisYear {
				filteredContracts = append(filteredContracts, c)
			}
		}

		// Sustituir la lista original por la filtrada
		contracts = filteredContracts
		person["contracts"] = contracts

		// marcar campos que falten y son obligatorios para uneix
		for i, c := range contracts {
			cid, _ := c["contract_id"].(float64)
			contractID := int(cid)

			if isEmpty(c["vinculation_type"]) {
				missing = append(missing, fmt.Sprintf("contract[%d].vinculation_type", contractID))
			}
			vStr, _ := c["vinculation_type"].(string)
			if strings.TrimSpace(vStr) == "Contracted worker" && isEmpty(c["job_category"]) {
				missing = append(missing, fmt.Sprintf("contract[%d].job_category", contractID))
			}
			if strings.TrimSpace(vStr) == "Contracted worker" && isEmpty(c["type"]) {
				missing = append(missing, fmt.Sprintf("contract[%d].type", contractID))
			}
			if isEmpty(c["start_date"]) {
				missing = append(missing, fmt.Sprintf("contract[%d].start_date", contractID))
			}
			if isEmpty(c["totalDedication_hours"]) {
				missing = append(missing, fmt.Sprintf("contract[%d].totalDedication_hours", contractID))
			}
			if isEmpty(c["contracting_institution"]) {
				missing = append(missing, fmt.Sprintf("contract[%d].contracting_institution", contractID))
			}
			if isEmpty(c["end_date"]) {
				if len(contracts) != 1 && i != latestContractIndex {

					missing = append(missing, fmt.Sprintf("contract[%d].end_date", contractID))
				}
			}
		}

		// taula people_grade
		gradesRaw := person["grades"].([]map[string]interface{})

		// Determinar el último grado NO doctoral y el último doctorado
		var latestNonDoctorate, latestDoctorate map[string]interface{}
		var maxYearNonDoctorate, maxYearDoctorate int

		for _, g := range gradesRaw {
			val, _ := g["grade_master_doctorate"].(string)
			val = strings.TrimSpace(val)

			// Extraer y normalizar el año de graduación
			var year int
			switch y := g["graduation_year"].(type) {
			case string:
				y = strings.TrimSpace(y)
				if y != "" {
					if parsed, err := strconv.Atoi(y); err == nil {
						year = parsed
					}
				}
			case float64:
				year = int(y)
			case int:
				year = y
			}

			if strings.EqualFold(val, "Doctorate") {
				if year > maxYearDoctorate {
					maxYearDoctorate = year
					latestDoctorate = g
				}
			} else if val != "" {
				if year > maxYearNonDoctorate {
					maxYearNonDoctorate = year
					latestNonDoctorate = g
				}
			}
		}

		// Construir lista final de grados seleccionados
		filteredGrades := []map[string]interface{}{}
		if latestNonDoctorate != nil {
			filteredGrades = append(filteredGrades, latestNonDoctorate)
		}
		if latestDoctorate != nil {
			filteredGrades = append(filteredGrades, latestDoctorate)
		}
		person["grades"] = filteredGrades

		grades := person["grades"].([]map[string]interface{})
		academicGrade := g["academic_grade"]

		hasValidDoctorate := false

		for _, grade := range grades {
			val, _ := grade["grade_master_doctorate"].(string)
			if strings.TrimSpace(val) == "Doctorate" &&
				!isEmpty(grade["graduation_university"]) &&
				!isEmpty(grade["graduation_country"]) &&
				!isEmpty(grade["graduation_year"]) {
				hasValidDoctorate = true
				break
			}
		}

		for _, grade := range grades {
			gid, _ := grade["grade_id"].(float64)
			gradeID := int(gid)

			// Pasamos el grade y el academic_grade
			pid, _ := person["people_id"].(float64)
			peopleID := int(pid)
			missingFields := validateAcademicGrade(grade, academicGrade, peopleID, hasValidDoctorate)
			for _, f := range missingFields {
				missing = append(missing, fmt.Sprintf("people_grade[%d].%s", gradeID, f))
			}
		}

		// --- DEDUP ---
		uniq := map[string]struct{}{}
		for _, f := range missing {
			uniq[f] = struct{}{}
		}
		uniqueMissing := []string{}
		for f := range uniq {
			uniqueMissing = append(uniqueMissing, f)
		}
		person["missing_fields"] = uniqueMissing
	}

	var result []map[string]interface{}
	// --- ELIMINAR DUPLICADOS DE CONTRATOS Y GRADOS ---
	for _, person := range grouped {
		// Deduplicar contratos
		seenContracts := make(map[int]bool)
		uniqueContracts := []map[string]interface{}{}
		for _, c := range person["contracts"].([]map[string]interface{}) {
			if idf, ok := c["contract_id"].(float64); ok {
				id := int(idf)
				if !seenContracts[id] {
					seenContracts[id] = true
					uniqueContracts = append(uniqueContracts, c)
				}
			}
		}
		person["contracts"] = uniqueContracts

		// Deduplicar grados
		seenGrades := make(map[int]bool)
		uniqueGrades := []map[string]interface{}{}
		for _, g := range person["grades"].([]map[string]interface{}) {
			if idf, ok := g["grade_id"].(float64); ok {
				id := int(idf)
				if !seenGrades[id] {
					seenGrades[id] = true
					uniqueGrades = append(uniqueGrades, g)
				}
			}
		}
		person["grades"] = uniqueGrades

	}

	for _, v := range grouped {
		result = append(result, v)
	}

	// VISITANTS DE WEBUSERS MSSQL
	visitors, err := fetchVisitors(apiKey, client, w, r, startOfYearStr, endOfYearStr)
	if err != nil {
		createLog(fmt.Sprintf("Failed to fetch visitors for incomplete users: %v", err), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch visitors", http.StatusInternalServerError)
		return
	}

	// Añadir info de visitantes
	result = append(result, visitors...)

	w.Header().Set("Content-Type", "application/json")

	_, username, _ := getUserInfo(apiKey, client, w, r)
	createLog(fmt.Sprintf("Successfully processed incomplete users: %d users analyzed by user %s", len(result), username), 0, apiKey, client, w)

	json.NewEncoder(w).Encode(result)
}

func fetchVisitors(apiKey string, client *http.Client, w http.ResponseWriter, req *http.Request, startOfYearStr, endOfYearStr string) ([]map[string]interface{}, error) {

	_, username, _ := getUserInfo(apiKey, client, w, req)

	// actualizar tabla visits de MSSQL
	query := map[string]interface{}{
		"raw": `
    UPDATE v
    SET v.web_user_id = wu.id
    FROM visits v
    LEFT JOIN web_users wu ON wu.email = v.email
    WHERE v.web_user_id = -1
      AND wu.id IS NOT NULL
  `,
	}

	updateJSON, _ := json.Marshal(query)
	resp := putReqMSSQL(updateJSON, apiKey, client, w, req)

	if resp == nil {
		createLog(fmt.Sprintf("fetchVisitors: putReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
		return nil, fmt.Errorf("fetchVisitors: putReqMSSQL returned nil")
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchVisitors: read body error for user %s: %v", username, err), 1, apiKey, client, w)
		return nil, fmt.Errorf("fetchVisitors: read body: %w", err)
	}

	query = map[string]interface{}{
		"table": `visits v
		          LEFT JOIN web_users wu ON wu.id = v.web_user_id
				  LEFT JOIN countries c ON wu.country_of_birth_id = c.id
				  LEFT JOIN nationalities n ON wu.nationality_id = n.id
				  LEFT JOIN provinces p ON wu.province_of_birth = p.name
				  LEFT JOIN cities ci ON wu.city_of_birth = ci.name
				  LEFT JOIN countries cDegree ON wu.country_degree = cDegree.id
				  LEFT JOIN countries cDoctorate ON wu.country_doctorate = cDoctorate.id`,
		"columns": `
			v.id AS visit_id,
			v.web_user_id,
			v.start_date,
			v.finish_date,
			wu.institution_id AS webuser_institution_id,
			v.institution_id AS visit_institution_id,
			v.no_webuser,
			v.no_webuser_last_name,
			n.Pais AS nationality,
			ci.Codi AS city_code,
			wu.first_name,
			wu.surname,
			wu.gender,
			v.id_number,
			c.Codi AS country_code,
			p.Codi AS province_code,
			wu.birth_date,
			wu.cif,
			wu.BirthYear,
			wu.status,
			wu.created,
			wu.university_of_degree_id,
			wu.year_degree,
			cDegree.Codi AS country_degree_code,
			wu.university_of_doctorate_id,
			wu.year_doctorate,
			cDoctorate.Codi AS country_doctorate_code,
			wu.orcid
		`,
		"condition": fmt.Sprintf(`
			DATEDIFF(DAY, v.start_date, COALESCE(v.finish_date, GETDATE())) >= 14
			AND v.start_date <= '%s'
			AND COALESCE(v.finish_date, GETDATE()) >= '%s'
			AND v.web_user_id != '-1'
			AND v.associated_entity != 'call'
		`, endOfYearStr, startOfYearStr),
	}

	getJSON, _ := json.Marshal(query)
	resp = getReqMSSQL(getJSON, apiKey, client, w, req)
	if resp == nil {
		createLog(fmt.Sprintf("fetchVisitors: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
		return nil, fmt.Errorf("fetchVisitors: getReqMSSQL returned nil")
	}
	defer resp.Body.Close()

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("fetchVisitors: read body error for user %s: %v", username, err), 1, apiKey, client, w)
		return nil, fmt.Errorf("fetchVisitors: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		createLog(fmt.Sprintf("fetchVisitors: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
		return nil, fmt.Errorf("fetchVisitors: MSSQL API error %d: %s", resp.StatusCode, string(b))
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(b, &rows); err != nil {
		createLog(fmt.Sprintf("fetchVisitors: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
		return nil, fmt.Errorf("fetchVisitors: unmarshal: %w", err)
	}

	// helpers
	asString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t)
		default:
			return strings.TrimSpace(fmt.Sprintf("%v", v))
		}
	}
	asInt := func(v interface{}) int {
		if v == nil {
			return 0
		}
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			i, _ := strconv.Atoi(strings.TrimSpace(t))
			return i
		default:
			i, _ := strconv.Atoi(fmt.Sprintf("%v", v))
			return i
		}
	}

	parseAnyDate := func(s string) (time.Time, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return time.Time{}, false
		}
		layouts := []string{
			time.RFC3339,
			"2006-01-02",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05.000",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t, true
			}
		}
		if len(s) >= 10 {
			if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}

	/*
		// rellena BirthYear si falta usando birth_date
		normalizeBirthYear := func(birthYearStr, birthDateStr string) string {
			by := strings.TrimSpace(birthYearStr)
			if by != "" && by != "0" && by != "<nil>" {
				return by
			}
			if t, ok := parseAnyDate(birthDateStr); ok {
				return fmt.Sprintf("%d", t.Year())
			}
			// si birth_date viene como "YYYY-..." pero no parsea, intenta cortar
			bd := strings.TrimSpace(birthDateStr)
			if len(bd) >= 4 {
				if _, err := strconv.Atoi(bd[:4]); err == nil {
					return bd[:4]
				}
			}
			return ""
		}*/

	out := make([]map[string]interface{}, 0, len(rows))

	for _, r := range rows {
		visitID := asInt(r["visit_id"])
		firstName, surname1, surname2 := pickVisitorIdentity(r)

		// fechas visita
		startStr := asString(r["start_date"])
		endStr := asString(r["finish_date"])

		if t, ok := parseAnyDate(startStr); ok {
			startStr = t.Format("2006-01-02")
		} else {
			startStr = ""
		}
		if endStr != "" {
			if t, ok := parseAnyDate(endStr); ok {
				endStr = t.Format("2006-01-02")
			} else {
				endStr = ""
			}
		}

		birthDate := asString(r["birth_date"])
		//birthYear := normalizeBirthYear(asString(r["BirthYear"]), birthDate)

		var incorporationYear string
		if asString(r["created"]) != "" {
			incorporationYear = asString(r["created"])
		} else {
			incorporationYear = asString(r["start_date"])
		}

		status := MapToUneix(Mappers.Status, r["status"])

		var nifNumber string
		if asString(r["id_number"]) != "" {
			nifNumber = asString(r["id_number"])
		} else if asString(r["cif"]) != "" {
			nifNumber = asString(r["cif"])
		}

		general := map[string]interface{}{
			// diferenciar de trabajadores contratados
			"visitor": true,

			"gender":         MapToUneix(Mappers.Gender, r["gender"]),
			"birth_date":     birthDate,
			"birth_city":     asString(r["city_code"]),
			"birth_province": asString(r["province_code"]),
			"birth_country":  asString(r["country_code"]),
			"nif":            nifNumber,
			"nif_extended":   nifNumber,
			"academic_grade": status,
			"status":         asString(r["status"]),

			"first_incorporation_year": incorporationYear,
			"orcid":                    sanitizeORCID(asString(r["orcid"])),
			"certificat_I3":            0,
			"picture_path":             "", // visitantes no tienen foto
		}

		var institutionID string
		if asString(r["visit_institution_id"]) != "" {
			institutionID = asString(r["visit_institution_id"])
		} else {
			institutionID = asString(r["webuser_institution_id"])
		}

		if institutionID != "-1" && institutionID != "" {
			query := map[string]interface{}{
				"table":     "institutions",
				"columns":   "Codi, name",
				"condition": fmt.Sprintf("id = %s", institutionID),
			}

			getJSON, _ := json.Marshal(query)
			resp := getReqMSSQL(getJSON, apiKey, client, w, req)
			if resp == nil {
				createLog(fmt.Sprintf("fetchVisitors: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: getReqMSSQL returned nil")
			}
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				createLog(fmt.Sprintf("fetchVisitors: read body error for user %s: %v", username, err), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: read body: %w", err)
			}
			if resp.StatusCode >= 400 {
				createLog(fmt.Sprintf("fetchVisitors: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: MSSQL API error %d: %s", resp.StatusCode, string(b))
			}

			var degUniv []map[string]interface{}
			if err := json.Unmarshal(b, &degUniv); err != nil {
				createLog(fmt.Sprintf("fetchVisitors: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: unmarshal: %w", err)
			}
			if len(degUniv) > 0 {
				institutionID = asString(degUniv[0]["Codi"])
			}
		} else {
			institutionID = "99"
		}

		if len(asString(r["year_degree"])) != 4 {
			r["year_degree"] = ""
		}
		if len(asString(r["year_doctorate"])) != 4 {
			r["year_doctorate"] = ""
		}

		contracts := []map[string]interface{}{
			{
				"visit_id": visitID,

				"vinculation_type":        "B",
				"start_date":              startStr,
				"end_date":                endStr,
				"contracting_institution": institutionID,
				"totalDedication_hours":   "3750",
				"type":                    "00",
				"job_category":            "",
				"funding":                 "",
			},
		}

		grades := []map[string]interface{}{}

		// ---------- DEGREE ----------

		if asString(r["university_of_degree_id"]) != "-1" && asString(r["university_of_degree_id"]) != "" {
			query := map[string]interface{}{
				"table":     "institutions",
				"columns":   "Codi",
				"condition": fmt.Sprintf("id = %s", asString(r["university_of_degree_id"])),
			}

			getJSON, _ := json.Marshal(query)
			resp := getReqMSSQL(getJSON, apiKey, client, w, req)
			if resp == nil {
				createLog(fmt.Sprintf("fetchVisitors: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: getReqMSSQL returned nil")
			}
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				createLog(fmt.Sprintf("fetchVisitors: read body error for user %s: %v", username, err), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: read body: %w", err)
			}
			if resp.StatusCode >= 400 {
				createLog(fmt.Sprintf("fetchVisitors: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: MSSQL API error %d: %s", resp.StatusCode, string(b))
			}

			var degUniv []map[string]interface{}
			if err := json.Unmarshal(b, &degUniv); err != nil {
				createLog(fmt.Sprintf("fetchVisitors: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: unmarshal: %w", err)
			}
			if len(degUniv) > 0 {
				r["university_of_degree_id"] = degUniv[0]["Codi"]
			}
		} else {
			r["university_of_degree_id"] = "99"
		}

		grades = append(grades, map[string]interface{}{
			"grade_master_doctorate": "Bachelor¤s degree",
			"graduation_university":  asString(r["university_of_degree_id"]),
			"graduation_country":     asString(r["country_degree_code"]),
			"graduation_year":        asString(r["year_degree"]),
		})

		// ---------- DOCTORATE ----------

		if asString(r["university_of_doctorate_id"]) != "-1" && asString(r["university_of_doctorate_id"]) != "" {
			query := map[string]interface{}{
				"table":     "institutions",
				"columns":   "Codi",
				"condition": fmt.Sprintf("id = %s", asString(r["university_of_doctorate_id"])),
			}

			getJSON, _ := json.Marshal(query)
			resp := getReqMSSQL(getJSON, apiKey, client, w, req)
			if resp == nil {
				createLog(fmt.Sprintf("fetchVisitors: getReqMSSQL returned nil for user %s", username), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: getReqMSSQL returned nil")
			}
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				createLog(fmt.Sprintf("fetchVisitors: read body error for user %s: %v", username, err), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: read body: %w", err)
			}
			if resp.StatusCode >= 400 {
				createLog(fmt.Sprintf("fetchVisitors: MSSQL API error %d: %s for user %s", resp.StatusCode, string(b), username), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: MSSQL API error %d: %s", resp.StatusCode, string(b))
			}

			var degUniv []map[string]interface{}
			if err := json.Unmarshal(b, &degUniv); err != nil {
				createLog(fmt.Sprintf("fetchVisitors: unmarshal error for user %s: %v", username, err), 1, apiKey, client, w)
				return nil, fmt.Errorf("fetchVisitors: unmarshal: %w", err)
			}
			if len(degUniv) > 0 {
				r["university_of_doctorate_id"] = degUniv[0]["Codi"]
			}
		} else {
			r["university_of_doctorate_id"] = "99"
		}

		grades = append(grades, map[string]interface{}{
			"grade_master_doctorate": "Doctorate",
			"graduation_university":  asString(r["university_of_doctorate_id"]),
			"graduation_country":     asString(r["country_doctorate_code"]),
			"graduation_year":        asString(r["year_doctorate"]),
		})

		nationalities := []map[string]interface{}{}

		nationalities = append(nationalities, map[string]interface{}{
			"nationality_code": strings.TrimSpace(asString(r["nationality"])),
		})

		groups := []map[string]interface{}{}

		// missing fields
		missing := []string{}
		if general["nif"] == "" {
			missing = append(missing, "people.nif")
		}
		if general["gender"] == "" {
			missing = append(missing, "people.gender")
		}
		if asString(r["nationality"]) == "" {
			missing = append(missing, "people_nationality.nationality_code")
		}
		if birthDate == "" {
			missing = append(missing, "people.birth_date")
		}
		if asString(general["birth_country"]) == "" {
			missing = append(missing, "people.birth_country")
		} else if asString(r["country_code"]) == "724" { // España
			if asString(r["province_code"]) == "" {
				missing = append(missing, "people.birth_province")
			} else {
				prov := asString(r["province_code"])
				if prov == "08" || prov == "17" || prov == "25" || prov == "43" {
					if asString(r["city_code"]) == "" {
						missing = append(missing, "people.birth_city")
					}
				}
			}
		}
		if general["academic_grade"] == "" {
			missing = append(missing, "people.academic_grade")
		}
		if status == "1" {
			if asString(r["university_of_degree_id"]) == "" {
				missing = append(missing, "people_grade[degree].graduation_university")
			}
			if asString(r["country_degree_code"]) == "" {
				missing = append(missing, "people_grade[degree].graduation_country")
			}
			if asString(r["university_of_doctorate_id"]) == "" {
				missing = append(missing, "people_grade[doctorate].graduation_university")
			}
			if asString(r["country_doctorate_code"]) == "" {
				missing = append(missing, "people_grade[doctorate].graduation_country")
			}
		} else if status == "7" || status == "6" {
			if asString(r["university_of_degree_id"]) == "" {
				missing = append(missing, "people_grade[degree].graduation_university")
			}
			if asString(r["country_degree_code"]) == "" {
				missing = append(missing, "people_grade[degree].graduation_country")
			}
		}

		if institutionID == "" {
			missing = append(missing, "Institution ID")
		}
		if general["certificat_I3"] == "" {
			missing = append(missing, "people.certificat_I3")
		}

		entry := map[string]interface{}{
			"web_user_id": r["web_user_id"],
			"visitor":     true,
			"visit_id":    visitID,

			"people_name":    firstName,
			"surname":        surname1,
			"secondSurname":  surname2,
			"general":        general,
			"contracts":      contracts,
			"grades":         grades,
			"nationalities":  nationalities,
			"groups":         groups,
			"missing_fields": missing,
		}

		out = append(out, entry)
	}

	return out, nil
}

// Not implemented atm
func handleUpdateVisitors(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

}

func isEmpty(v interface{}) bool {
	switch val := v.(type) {
	case nil:
		return true // valor nulo real
	case string:
		// Normalizar el texto
		s := strings.TrimSpace(strings.ToLower(val))
		// Consideramos vacío si está literalmente vacío o es "null"
		return s == "" || s == "null"
	case fmt.Stringer:
		return val.String() == ""
	case []byte:
		return len(val) == 0
	default:
		return false // Cualquier otro valor (0, false, etc.) NO es vacío
	}
}

// Unused: handleUpdateFields updates only the specified fields for a given person.
func handleUpdateFields(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Leer body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read request body in handleUpdateFields: %v", err), 1, apiKey, client, w)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parsear JSON
	var req struct {
		PersonID   int                    `json:"personId"`
		ContractID int                    `json:"contractId"`
		GradeID    int                    `json:"gradeId"`
		FormData   map[string]interface{} `json:"formData"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		createLog(fmt.Sprintf("Invalid JSON in handleUpdateFields: %v", err), 1, apiKey, client, w)
		return
	}

	if req.PersonID == 0 || len(req.FormData) == 0 {
		createLog(fmt.Sprintf("Missing personId or formData in handleUpdateFields (personId=%d)", req.PersonID), 1, apiKey, client, w)
		http.Error(w, "Missing personId or formData", http.StatusBadRequest)
		return
	}

	// Clasificar los campos por tabla destino
	peopleFields := map[string]interface{}{}
	contractFields := map[string]interface{}{}
	educationFields := map[string]interface{}{}

	for key, value := range req.FormData {
		switch key {
		// Tabla people
		case "gender", "birth_date", "birth_country", "birth_province", "birth_city",
			"nif", "academic_grade", "orcid", "certificat_I3":
			peopleFields[key] = value

		// Tabla people_nationality
		case "nationality_code":
			// En este caso, actualiza o inserta según exista
			updateNationality(req.PersonID, value, apiKey, client, w)

		// Tabla contract
		case "vinculation_type", "start_date", "end_date", "job_category", "type",
			"totalDedication_hours", "contracting_institution", "funding":
			contractFields[key] = value

		// Tabla people_grade
		case "grade_master_doctorate", "graduation_university", "graduation_country", "graduation_year":
			educationFields[key] = value
		}
	}

	// Ejecutar actualizaciones dinámicas
	if len(peopleFields) > 0 {
		updateTable("people", "id", req.PersonID, peopleFields, apiKey, client, w)
	}
	if len(contractFields) > 0 {
		if req.ContractID > 0 {
			updateActiveContract(req.ContractID, contractFields, apiKey, client, w)
		}
	}
	if len(educationFields) > 0 {
		if req.GradeID > 0 {
			updateEducationRecord(req.GradeID, educationFields, apiKey, client, w)
		}
	}
	createLog(fmt.Sprintf("Fields updated successfully for personId=%d", req.PersonID), 0, apiKey, client, w)

	resp := map[string]string{"status": "ok", "message": "Fields updated successfully"}
	json.NewEncoder(w).Encode(resp)
}

// unused: updateTable builds and executes a simple UPDATE for one table.
func updateTable(table, idField string, id int, fields map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) {

	fields = filterEmptyFields(fields)
	if len(fields) == 0 {
		createLog(fmt.Sprintf("No fields to update for %s (id=%d)", table, id), 1, apiKey, client, w)
		return
	}
	columns := []string{}
	values := []string{}
	for k, v := range fields {
		columns = append(columns, k)

		// Escapar comillas en strings
		switch val := v.(type) {
		case string:
			values = append(values, val)
		default:
			values = append(values, fmt.Sprintf("%v", val))
		}
	}

	query := map[string]interface{}{
		"table":     table,
		"columns":   strings.Join(columns, ", "),
		"value":     strings.Join(values, ", "),
		"condition": fmt.Sprintf("%s = %d", idField, id),
	}

	jsonData, _ := json.Marshal(query)
	createLog(fmt.Sprintf("Updating table '%s' for id=%d with data: %s", table, id, string(jsonData)), 0, apiKey, client, w)
	putReq(jsonData, apiKey, client, w)
}

// unused: updateActiveContract updates only the specified fields for a given contract.
func updateActiveContract(contractID int, fields map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) {
	fields = filterEmptyFields(fields)
	if len(fields) == 0 {
		createLog(fmt.Sprintf("No fields to update for contract %d", contractID), 1, apiKey, client, w)
		return
	}

	columns := []string{}
	values := []string{}
	for k, v := range fields {
		columns = append(columns, k)
		switch val := v.(type) {
		case string:
			values = append(values, val)
		default:
			values = append(values, fmt.Sprintf("%v", val))
		}
	}

	query := map[string]interface{}{
		"table":     "contract",
		"columns":   strings.Join(columns, ", "),
		"value":     strings.Join(values, ", "),
		"condition": fmt.Sprintf("id = %d", contractID),
	}

	jsonData, _ := json.Marshal(query)
	createLog(fmt.Sprintf("Updating contract id=%d with data: %s", contractID, string(jsonData)), 0, apiKey, client, w)
	putReq(jsonData, apiKey, client, w)
}

// unused: updateEducationRecord updates only the specified fields for a given education record.
func updateEducationRecord(gradeID int, fields map[string]interface{}, apiKey string, client *http.Client, w http.ResponseWriter) {
	fields = filterEmptyFields(fields)
	if len(fields) == 0 {
		createLog(fmt.Sprintf("No education fields to update for grade_id=%d", gradeID), 1, apiKey, client, w)
		return
	}

	columns := []string{}
	values := []string{}
	for k, v := range fields {
		columns = append(columns, k)
		switch val := v.(type) {
		case string:
			values = append(values, val)
		default:
			values = append(values, fmt.Sprintf("%v", val))
		}
	}

	query := map[string]interface{}{
		"table":     "people_grade",
		"columns":   strings.Join(columns, ", "),
		"value":     strings.Join(values, ", "),
		"condition": fmt.Sprintf("id = %d", gradeID),
	}

	jsonData, _ := json.Marshal(query)
	createLog(fmt.Sprintf("Updating education record (people_grade) id=%d with data: %s", gradeID, string(jsonData)), 0, apiKey, client, w)
	putReq(jsonData, apiKey, client, w)
}

// unused: updateNationality updates nationality for a given person.
func updateNationality(personID int, value interface{}, apiKey string, client *http.Client, w http.ResponseWriter) {
	valStr := fmt.Sprintf("%v", value)
	if strings.TrimSpace(valStr) == "" {
		createLog(fmt.Sprintf("Skipped nationality update for person %d (empty value)", personID), 1, apiKey, client, w)
		return
	}
	query := map[string]interface{}{
		"table":   "people_nationality",
		"columns": "nationality_code, people_id",
		"value":   fmt.Sprintf("%s, %d", valStr, personID),
	}

	jsonData, _ := json.Marshal(query)
	createLog(fmt.Sprintf("Updating nationality for person_id=%d with data: %s", personID, string(jsonData)), 0, apiKey, client, w)
	postReq(jsonData, apiKey, client, w)
}

func filterEmptyFields(fields map[string]interface{}) map[string]interface{} {
	clean := make(map[string]interface{})
	for k, v := range fields {
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		clean[k] = v
	}
	return clean
}

// ------------------------- UNITS ------------------------------
func handleIncompleteUnits(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":     `researchGroup`,
		"columns":   `research_code, name, outdated, cif, character, typology`,
		"condition": `outdated = '0' AND category = 'unit'`,
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch research groups: getReq returned nil for user "+username, 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response for research groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var groups []map[string]interface{}
	if err := json.Unmarshal(body, &groups); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for research groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}

	var missing []map[string]interface{}
	for _, g := range groups {
		var mFields []string
		if isEmpty(g["name"]) {
			mFields = append(mFields, "Name")
		}
		if isEmpty(g["character"]) {
			mFields = append(mFields, "Character")
		}
		if isEmpty(g["typology"]) {
			mFields = append(mFields, "Typology")
		}

		if len(mFields) > 0 {
			g["missing_fields"] = mFields
			missing = append(missing, g)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missing)
}

func handleGetUnits(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":     `researchGroup`,
		"columns":   `research_code, name, outdated, cif, character, typology`,
		"condition": `category = 'unit' AND outdated = 0`,
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch research groups: getReq returned nil for user "+username, 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response while fetching groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var groups []map[string]interface{}
	if err := json.Unmarshal(body, &groups); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for research groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}
	createLog(fmt.Sprintf("Successfully retrieved %d units for user %s", len(groups), username), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// ------------------------- GROUPS --------------------------------

// sends all groups with missing fields
func handleIncompleteGroups(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":     `researchGroup`,
		"columns":   `research_code, intern_code, name, start_date`,
		"condition": `category != unit`,
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch research groups: getReq returned nil for user "+username, 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response for research groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var groups []map[string]interface{}
	if err := json.Unmarshal(body, &groups); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for research groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}

	var missing []map[string]interface{}
	for _, g := range groups {
		var mFields []string
		if isEmpty(g["research_code"]) {
			mFields = append(mFields, "researchGroup.research_code")
		}
		if isEmpty(g["intern_code"]) {
			mFields = append(mFields, "researchGroup.intern_code")
		}
		if isEmpty(g["name"]) {
			mFields = append(mFields, "researchGroup.name")
		}
		if isEmpty(g["start_date"]) {
			mFields = append(mFields, "researchGroup.start_date")
		}

		if len(mFields) > 0 {
			g["missing_fields"] = mFields
			missing = append(missing, g)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missing)
}

// sends all groups
func handleGetGroups(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)

	// Año de referencia (por defecto, el año actual)
	targetYear := time.Now().Year() - 1

	// Leer ?year=YYYY si viene del front
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil && y > 1900 && y < 3000 {
			targetYear = y
		}
	}

	// Fechas de inicio y fin de ese año
	startOfYearStr := fmt.Sprintf("%04d-01-01", targetYear)
	endOfYearStr := fmt.Sprintf("%04d-12-31", targetYear)

	query := map[string]interface{}{
		"table":     `researchGroup`,
		"columns":   `research_code, intern_code, name, start_date`,
		"condition": fmt.Sprintf("start_date <= '%s' AND (end_date >= '%s' OR end_date = '') AND category != 'unit'", endOfYearStr, startOfYearStr),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch research groups: getReq returned nil for user "+username, 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response while fetching groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var groups []map[string]interface{}
	if err := json.Unmarshal(body, &groups); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for research groups: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}
	createLog(fmt.Sprintf("Successfully retrieved %d groups for user %s", len(groups), username), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// ------------------------- GROUPS RECOGNITION --------------------------------

// Sends all recognitions with missing mandatory fields
func handleIncompleteGroupsRecognition(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	query := map[string]interface{}{
		"table":     "researchGroup AS r LEFT JOIN uneix_groupReconeixement AS g ON r.research_code = g.codi_grupRecerca",
		"columns":   "r.research_code, g.codi_grupRecerca, g.codi_reconeixement, g.data_obtencio",
		"condition": "r.category != 'unit'",
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch group recognitions: getReq returned nil for user "+username, 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed (no response)", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response for group recognitions: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var recognitions []map[string]interface{}
	if err := json.Unmarshal(body, &recognitions); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for group recognitions: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}

	var missing []map[string]interface{}

	for _, rec := range recognitions {
		var mFields []string

		// Si el grupo no tiene entrada en uneix_groupReconeixement,
		// asignamos manualmente su research_code como codi_grupRecerca
		if isEmpty(rec["codi_grupRecerca"]) && !isEmpty(rec["research_code"]) {
			rec["codi_grupRecerca"] = rec["research_code"]
		}

		// Detectar campos faltantes
		if isEmpty(rec["codi_grupRecerca"]) {
			mFields = append(mFields, "uneix_groupReconeixement.codi_grupRecerca")
		}
		if isEmpty(rec["codi_reconeixement"]) {
			mFields = append(mFields, "uneix_groupReconeixement.codi_reconeixement")
		}
		if isEmpty(rec["data_obtencio"]) {
			mFields = append(mFields, "uneix_groupReconeixement.data_obtencio")
		}

		// Si falta algo → añadir a la lista
		if len(mFields) > 0 {
			rec["missing_fields"] = mFields
			missing = append(missing, rec)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missing)
}

// sends all groups
func handleGetGroupsRecognition(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	// Año de referencia (por defecto, el año actual)
	targetYear := time.Now().Year() - 1

	// Leer ?year=YYYY si viene del front
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil && y > 1900 && y < 3000 {
			targetYear = y
		}
	}

	// Fechas de inicio y fin de ese año
	startOfYearStr := fmt.Sprintf("%04d-01-01", targetYear)
	endOfYearStr := fmt.Sprintf("%04d-12-31", targetYear)

	query := map[string]interface{}{
		"table":     "researchGroup AS r LEFT JOIN uneix_groupReconeixement AS g ON r.research_code = g.codi_grupRecerca",
		"columns":   "r.research_code, g.codi_grupRecerca, g.codi_reconeixement, g.data_obtencio",
		"condition": fmt.Sprintf("start_date <= '%s' AND (end_date >= '%s' OR end_date = '') AND category != 'unit'", endOfYearStr, startOfYearStr),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	if resp == nil {
		createLog("Failed to fetch group recognitions: getReq returned nil for user "+username, 1, apiKey, client, w)
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		createLog(fmt.Sprintf("Failed to read DB response while fetching group recognitions: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		return
	}

	var groups []map[string]interface{}
	if err := json.Unmarshal(body, &groups); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for group recognitions: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}
	createLog(fmt.Sprintf("Successfully retrieved %d group recognitions for user %s", len(groups), username), 0, apiKey, client, w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// ------------------------- GROUP MEMBERS --------------------------------

func handleIncompleteGroupMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	step1 := map[string]interface{}{
		"table": `people_group g LEFT JOIN researchGroup r ON r.intern_code = g.group_intern_code`,
		"columns": `
			g.people_id,
			g.ip,
			r.research_code
		`,
	}

	json1, _ := json.Marshal(step1)
	resp1 := getReq(json1, apiKey, client, w)
	if resp1 == nil {
		createLog("Failed to get groups and research code in handleIncompleteGroupMembers for user "+username, 1, apiKey, client, w)
		http.Error(w, "DB step1 failed", http.StatusInternalServerError)
		return
	}
	defer resp1.Body.Close()

	body1, _ := io.ReadAll(resp1.Body)
	var groupData []map[string]interface{}
	if err := json.Unmarshal(body1, &groupData); err != nil {
		createLog(fmt.Sprintf("Error parsing response in handleIncompleteGroupMembers: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed parsing DB step1", http.StatusInternalServerError)
		return
	}

	// recolectar los IDs de persona únicos
	idSet := make(map[int]struct{})
	for _, g := range groupData {
		if f, ok := g["people_id"].(float64); ok {
			idSet[int(f)] = struct{}{}
		}
	}
	var ids []int
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	// pedir info de people
	peopleMap := make(map[int]map[string]interface{})
	for i := 0; i < len(ids); i += 100 {
		end := i + 100
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		idsStr := make([]string, len(chunk))
		for j, id := range chunk {
			idsStr[j] = fmt.Sprintf("%d", id)
		}

		step2 := map[string]interface{}{
			"table":     "people p",
			"columns":   "p.id AS people_id, p.name, p.surname, p.nif, p.nif_extended",
			"condition": fmt.Sprintf("p.id IN (%s)", strings.Join(idsStr, ",")),
		}

		json2, _ := json.Marshal(step2)
		resp2 := getReq(json2, apiKey, client, w)
		if resp2 == nil {
			createLog("Skipped null response for chunk in get people info in handleIncompleteGroupMembers for user "+username, 1, apiKey, client, w)
			continue
		}
		body2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()

		var people []map[string]interface{}
		if err := json.Unmarshal(body2, &people); err != nil {
			createLog(fmt.Sprintf("Failed parsing chunk in handleIncompleteGroupMembers: %v by user %s", err, username), 1, apiKey, client, w)
			continue
		}

		for _, p := range people {
			if f, ok := p["people_id"].(float64); ok {
				peopleMap[int(f)] = p
			}
		}
	}

	// merge manual en Go
	var members []map[string]interface{}
	for _, g := range groupData {
		row := map[string]interface{}{
			"people_id":     g["people_id"],
			"research_code": g["research_code"],
			"ip":            g["ip"],
		}

		if f, ok := g["people_id"].(float64); ok {
			id := int(f)
			if p, exists := peopleMap[id]; exists {
				row["people_name"] = unescapeComma(fmt.Sprintf("%v", p["name"]))
				row["surname"] = unescapeComma(fmt.Sprintf("%v", p["surname"]))
				row["nif"] = p["nif"]
				row["nif_extended"] = p["nif_extended"]
			}
		}
		members = append(members, row)
	}

	// marcar los faltantes
	var incomplete []map[string]interface{}
	for _, m := range members {
		var missing []string
		nif := strings.TrimSpace(fmt.Sprintf("%v", m["nif"]))
		nifExt := strings.TrimSpace(fmt.Sprintf("%v", m["nif_extended"]))
		name := strings.TrimSpace(fmt.Sprintf("%v", m["people_name"]))
		surname := strings.TrimSpace(fmt.Sprintf("%v", m["surname"]))

		if (nif == "" || nif == "<nil>") && (nifExt == "" || nifExt == "<nil>") {
			missing = append(missing, "people.nif")
		}
		if name == "" {
			missing = append(missing, "people.people_name")
		}
		if surname == "" {
			missing = append(missing, "people.surname")
		}

		if len(missing) > 0 {
			m["missing_fields"] = missing
			incomplete = append(incomplete, m)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(incomplete)
}

func dedupGroupMembers(members []map[string]interface{}) []map[string]interface{} {
	seen := make(map[string]bool)
	deduped := make([]map[string]interface{}, 0, len(members))

	for _, member := range members {
		peopleID := fmt.Sprintf("%v", member["people_id"])
		researchCode := fmt.Sprintf("%v", member["research_code"])
		ip := fmt.Sprintf("%v", member["ip"])

		key := peopleID + "|" + researchCode + "|" + ip

		if seen[key] {
			continue
		}

		seen[key] = true
		deduped = append(deduped, member)
	}

	return deduped
}

// handleGetGroupMembers retrieves all group members and their linked people info.
func handleGetGroupMembers(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	// Año de referencia (por defecto, el año actual)
	targetYear := time.Now().Year() - 1

	// Leer ?year=YYYY si viene del front
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil && y > 1900 && y < 3000 {
			targetYear = y
		}
	}

	// Fechas de inicio y fin de ese año
	startOfYearStr := fmt.Sprintf("%04d-01-01", targetYear)
	endOfYearStr := fmt.Sprintf("%04d-12-31", targetYear)

	query := map[string]interface{}{
		"table": `
			people_group AS g
			LEFT JOIN researchGroup AS r ON r.intern_code = g.group_intern_code
			LEFT JOIN people AS p ON g.people_id = p.id
			LEFT JOIN contract c ON c.people_id = p.id
		`,
		"columns": `
			p.id AS people_id,
			p.name,
			p.surname,
			r.research_code,
			p.nif,
			p.nif_extended,
			g.ip
		`,
		"condition": fmt.Sprintf("r.start_date <= '%s' AND (r.end_date >= '%s' OR r.end_date = '' OR r.end_date IS NULL) AND r.category != 'unit' AND c.start_date <= '%s' AND (c.end_date >= '%s' OR c.end_date = '' OR c.end_date IS NULL)", endOfYearStr, startOfYearStr, endOfYearStr, startOfYearStr),
	}

	getJSON, _ := json.Marshal(query)
	resp := getReq(getJSON, apiKey, client, w)
	fmt.Printf("Debug: handleGetGroupMembers query: %s\n", string(getJSON))
	if resp == nil {
		http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
		createLog("Failed to fetch group members: getReq returned nil for user "+username, 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read DB response", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Failed to read DB response while fetching group members: %v by user %s", err, username), 1, apiKey, client, w)
		return
	}

	if len(body) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		createLog("Empty response body while fetching group members — returned empty list for user "+username, 1, apiKey, client, w)

		return
	}

	var members []map[string]interface{}
	if err := json.Unmarshal(body, &members); err != nil {
		createLog(fmt.Sprintf("Error parsing JSON for group members: %v by user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Failed to parse DB response", http.StatusInternalServerError)
		return
	}
	members = dedupGroupMembers(members)

	w.Header().Set("Content-Type", "application/json")
	if members == nil {
		members = []map[string]interface{}{}
	}
	createLog(fmt.Sprintf("Successfully retrieved %d group members for user %s", len(members), username), 0, apiKey, client, w)

	json.NewEncoder(w).Encode(members)
}

func handleExportUneixSpinOffs(apiKey string, client *http.Client, w http.ResponseWriter) {

	query := map[string]interface{}{
		"table": "uneix_spinoffs",
		"columns": `
			codi_entitat,
			codi_ens,
			cif,
			nom,
			nif,
			nif_ampliat,
			codi_conacit,
			data_creacio,
			data_cessio,
			perc_participacio,
			data_extincio,
			area_cnae,
			ext_o_fin
		`,
	}

	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Proxy response is nil", http.StatusBadGateway)
		createLog("Failed to fetch spinoffs for export: proxy returned nil", 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading proxy response", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Failed to read proxy response in handleExportUneixSpinOffs: %v", err), 1, apiKey, client, w)
		return
	}

	if resp.StatusCode >= 400 {
		http.Error(w, "Error from proxy", resp.StatusCode)
		createLog(fmt.Sprintf("Proxy returned HTTP %d error while exporting spinoffs", resp.StatusCode), 1, apiKey, client, w)
		return
	}

	var records []map[string]interface{}
	if err := json.Unmarshal(body, &records); err != nil {
		http.Error(w, "Invalid JSON from proxy", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Invalid JSON received from proxy while exporting spinoffs: %v", err), 1, apiKey, client, w)
		return
	}

	if len(records) == 0 {
		http.Error(w, "No hay datos para exportar", http.StatusNoContent)
		createLog("No spinoff data available for export", 1, apiKey, client, w)

		return
	}

	var buf bytes.Buffer
	currentYear := time.Now().Year() - 1

	for _, rec := range records {
		line := fmt.Sprintf("%d|%v|%v|%v|%v|%v|%v|%v|%v|%v|%v|%v|%v|%v",
			currentYear,
			rec["codi_entitat"],
			rec["codi_ens"],
			rec["cif"],
			rec["nom"],
			rec["nif"],
			rec["nif_ampliat"],
			rec["codi_conacit"],
			rec["data_creacio"],
			rec["data_cessio"],
			rec["perc_participacio"],
			rec["data_extincio"],
			rec["area_cnae"],
			rec["ext_o_fin"],
		)
		buf.WriteString(line + "\n")
	}

	filename := fmt.Sprintf("RM%d64.csv", currentYear%100)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-cache")

	if _, err := w.Write(buf.Bytes()); err != nil {
		createLog(fmt.Sprintf("Error sending CSV export for spinoffs: %v", err), 1, apiKey, client, w)
		http.Error(w, "Error enviando CSV", http.StatusInternalServerError)
		return
	}

	createLog(fmt.Sprintf("Successfully exported %d spinoff records to CSV", len(records)), 0, apiKey, client, w)

}

// --------------------------------------------------------------------------------------------------------
// --------------------------------------------------------------------------------------------------------
// --------------------------------- EXPLOTACIÓ DE DADES DEL CENTRE ---------------------------------------
// --------------------------------------------------------------------------------------------------------
// --------------------------------------------------------------------------------------------------------

// Funció de retorn dinàmica de les dades demanades al front-end
func handleExportEmployeeData(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	var req struct {
		Fields []string `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		createLog(fmt.Sprintf("Invalid JSON in handleExportEmployeeData: %v for user %s", err, username), 1, apiKey, client, w)
		return
	}

	if len(req.Fields) == 0 {
		http.Error(w, "No fields selected", http.StatusBadRequest)
		createLog(fmt.Sprintf("No fields selected in handleExportEmployeeData for user %s", username), 1, apiKey, client, w)
		return
	}

	// Cargar los mapas de traducción
	countryMap := loadCountryMap(apiKey, client)
	provinceMap := loadProvinceMap(apiKey, client)
	cityMap := loadCityMap(apiKey, client)
	academicMap := loadAcademicMap(apiKey, client)
	institutionMap := loadInstitutionsMap(apiKey, client)
	studiesMap := loadStudiesMap(apiKey, client)
	workersMap := loadWorkersMap(apiKey, client)
	officeMap := loadOfficeMap(apiKey, client)
	trainingMap := loadTrainingsMap(apiKey, client)

	// Mapa camps front-end -> Data base
	allColumns := map[string]string{
		//taula people
		"people_id":         "p.id AS people_id",
		"people_name":       "p.name AS people_name",
		"surname":           "p.surname",
		"secondSurname":     "p.secondSurname",
		"gender":            "p.gender",
		"birth_date":        "p.birth_date",
		"nationality":       "n.nationality_code AS nationality",
		"birth_country":     "p.birth_country",
		"birth_province":    "p.birth_province",
		"birth_city":        "p.birth_city",
		"nif":               "p.nif",
		"nif_extended":      "p.nif_extended",
		"academic_grade":    "p.academic_grade",
		"user_phone":        "p.user_phone",
		"emergency_phone":   "p.emergencyContact_phone  AS emergency_phone",
		"user_email":        "p.user_email",
		"employee_email":    "p.crm_email AS employee_email",
		"orcid":             "p.orcid",
		"i3":                "p.certificat_I3 AS i3",
		"webUser_ID":        "p.webUser_idExternal AS webUser_ID",
		"people_idExternal": "p.people_idExternal AS people_idExternal",
		"active":            "p.active",
		"agreesToUneix":     "p.agreesToUneix",

		//taula residence
		"residence_country":  "r.residence_country",
		"residence_province": "r.residence_province",
		"residence_city":     "r.residence_city",
		"address":            "r.address",
		"postal_code":        "r.postal_code",
		"actual":             "r.actual",

		//taula contract
		"vinculation_type":        "c.vinculation_type",
		"contracting_institution": "c.contracting_institution",
		"position":                "c.position",
		"trainee_type":            "c.trainee_type",
		"trainee_studies":         "c.trainee_studies",
		"internship":              "c.internship",
		"job_category":            "c.job_category",
		"contract_type":           "c.type AS contract_type",
		"contract_start_date":     "c.start_date AS contract_start_date",
		"contract_end_date":       "c.end_date AS contract_end_date",
		"totalDedication_hours":   "c.totalDedication_hours",
		"office":                  "c.office_location AS office",
		"funding":                 "c.funding",

		// taula people_supervisor
		"supervisor": "s.supervisor_id AS supervisor",

		//taula grade
		"grade_master_doctorate": "pg.grade_master_doctorate",
		"grade_code":             "pg.code AS grade_code",
		"gradeName":              "pg.gradeName",
		"graduation_university":  "pg.graduation_university",
		"universityName":         "pg.universityName",
		"graduation_country":     "pg.graduation_country",
		"graduation_year":        "pg.graduation_year",

		//taula group
		"group_intern_code": "gr.group_intern_code",
		"group_start_date":  "gr.start_date AS group_start_date",
		"group_end_date":    "gr.end_date AS group_end_date",
		"ip":                "gr.ip",

		//taula phd
		"phd_program":                 "phd.phd_program",
		"phd_tesisTitle":              "phd.phd_tesisTitle",
		"phd_startYear":               "phd.phd_startYear",
		"phd_tesisDirector":           "phd.phd_tesisDirector",
		"phd_university":              "phd.phd_university",
		"phd_plannedPresentationDate": "phd.phd_plannedPresentationDate",
		"phd_presentationDate":        "phd.phd_presentationDate",
		"phd_link":                    "phd.phd_link",

		//taula responsible
		"phd_centerResponsible":   "resp.people_id AS phd_centerResponsible",
		"phd_externalResponsible": "resp.name AS phd_externalResponsible",
		"phd_IP":                  "resp.ip_or_tutor AS phd_IP",

		//taula training
		"training_name": "t.training_id AS training_name",
		"training_date": "t.date AS training_date",
	}

	// Mapa camps back-end -> excel
	displayNames := map[string]string{
		"people_id":         "User ID",
		"people_name":       "Name",
		"surname":           "Surname",
		"secondSurname":     "Second Surname",
		"gender":            "Gender",
		"birth_date":        "Birth Date",
		"nationality":       "Nationality",
		"birth_country":     "Birth Country",
		"birth_province":    "Birth Province",
		"birth_city":        "Birth City",
		"nif":               "NIF",
		"nif_extended":      "NIF Extended",
		"academic_grade":    "Academic Grade",
		"user_phone":        "Phone",
		"emergency_phone":   "Emergency Phone",
		"user_email":        "Personal Email",
		"employee_email":    "Institutional Email",
		"orcid":             "ORCID",
		"i3":                "Certificat I3?",
		"webUser_ID":        "Web User ID",
		"people_idExternal": "External ID",
		"active":            "Currently employed at CRM?",
		"agreesToUneix":     "Agrees to UNEIX?",

		"residence_country":  "Residence Country",
		"residence_province": "Residence Province",
		"residence_city":     "Residence City",
		"address":            "Address",
		"postal_code":        "Postal Code",
		"actual":             "Current Residence?",

		"vinculation_type":        "Vinculation Type",
		"contracting_institution": "Contracting Institution",
		"position":                "Position",
		"trainee_type":            "Trainee Type",
		"trainee_studies":         "Trainee Studies",
		"internship":              "Internship",
		"job_category":            "Job Category",
		"contract_type":           "Contract Type",
		"contract_start_date":     "Contract Start Date",
		"contract_end_date":       "Contract End Date",
		"totalDedication_hours":   "Working hours per week",
		"supervisor":              "Supervisor",
		"office":                  "Office Location",
		"funding":                 "Funding",

		"grade_master_doctorate": "Grade Type",
		"grade_code":             "Grade Name",
		"gradeName":              "Grade (other)",
		"graduation_university":  "Graduation University",
		"universityName":         "University (other)",
		"graduation_country":     "Graduation Country",
		"graduation_year":        "Graduation Year",

		"group_intern_code": "Group Code",
		"group_start_date":  "Group Start Date",
		"group_end_date":    "Group End Date",
		"ip":                "Is IP?",

		"phd_program":                 "PhD Program",
		"phd_tesisTitle":              "Thesis Title",
		"phd_startYear":               "PhD Start Year",
		"phd_tesisDirector":           "Thesis Director",
		"phd_university":              "PhD University",
		"phd_plannedPresentationDate": "Planned Presentation Date",
		"phd_presentationDate":        "Presentation Date",
		"phd_link":                    "PhD Link",
		"phd_centerResponsible":       "Center Responsible",
		"phd_externalResponsible":     "External Responsible",
		"phd_IP":                      "IP / Tutor",

		"training_name": "Training Name",
		"training_date": "Training Date",
	}

	// Format camps que són dates
	var dateFields = map[string]bool{
		"birth_date":                  true,
		"contract_start_date":         true,
		"contract_end_date":           true,
		"group_start_date":            true,
		"group_end_date":              true,
		"phd_plannedPresentationDate": true,
		"phd_presentationDate":        true,
		"training_date":               true,
	}
	// Format camps que són hores (dividir entre 100)
	var hourFields = map[string]bool{
		"totalDedication_hours": true,
	}
	// Format camps booleans
	var boolFields = map[string]bool{
		"i3":            true,
		"active":        true,
		"agreesToUneix": true,
		"actual":        true,
		"internship":    true,
		"ip":            true,
	}
	// Traducció beques BD -> nom
	var fundingMap = map[string]string{
		"0000000175": "BP: Ajuts postdoctorals Beatriu de Pinós",
		"0000000251": "FPU: Contratos o becas predoctorales para la formación de doctores (FPU)",
		"0000000332": "FPI: Contratos o becas predoctorales para la formación de doctores (FPI)",
		"0000000375": "FI: Ajuts Joan Oró per a la contractació de personal investigador predoctoral en formació",
		"0000000469": "ICREA-SR: Contractes a Investigadors ICREA",
		"0000000533": "RYC: Ramon y Cajal",
		"0000002204": "HORIZON-MSCA-PF: Marie Skłodowska-Curie Postdoctoral Fellowship (PF)",
		"0000002325": "JDC: Contratos Juan de la Cierva",
		"0000002650": "INPHINIT: PhD Fellowship La Caixa INPHINIT",
	}

	// Construir lista dinámica de columnas seleccionadas
	var selectedCols []string
	for _, field := range req.Fields {
		if sqlCol, ok := allColumns[field]; ok {
			selectedCols = append(selectedCols, sqlCol)
		}
	}

	if len(selectedCols) == 0 {
		http.Error(w, "No valid fields selected", http.StatusBadRequest)
		createLog(fmt.Sprintf("No valid SQL columns found in handleExportEmployeeData for user %s", username), 1, apiKey, client, w)
		return
	}

	// Construir query dinámica
	query := map[string]interface{}{
		"table": `people p
				  LEFT JOIN residence r ON p.id = r.people_id
				  LEFT JOIN people_nationality n ON p.id = n.people_id
				  LEFT JOIN contract c ON p.id = c.people_id
				  LEFT JOIN people_grade pg ON p.id = pg.people_id
				  LEFT JOIN people_phd phd ON p.id = phd.people_id
				  LEFT JOIN responsible resp ON phd.id = resp.tesis_id
				  LEFT JOIN people_training t ON p.id = t.people_id
				  LEFT JOIN people_group gr ON p.id = gr.people_id
				  LEFT JOIN people_supervisor s ON c.id = s.contract_id`,
		"columns": strings.Join(selectedCols, ", "),
		// descartem id administratiu
		"condition": "p.id != 304",
	}

	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Proxy response is nil", http.StatusBadGateway)
		createLog(fmt.Sprintf("Database proxy returned nil in handleExportEmployeeData for user %s", username), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var rows []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rows); err != nil {
		createLog(fmt.Sprintf("Error decoding DB response in handleExportEmployeeData: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}
	// Eliminar filas duplicadas basadas en los campos seleccionados
	uniqueRows := make([]map[string]interface{}, 0, len(rows))
	seen := make(map[string]bool)

	for _, row := range rows {
		var keyParts []string
		for _, f := range req.Fields {
			if val, ok := row[f]; ok && val != nil {
				keyParts = append(keyParts, fmt.Sprintf("%v", val))
			} else {
				keyParts = append(keyParts, "")
			}
		}
		key := strings.Join(keyParts, "|") // hash simple de los valores de la fila

		if !seen[key] {
			seen[key] = true
			uniqueRows = append(uniqueRows, row)
		}
	}

	rows = uniqueRows

	// Crear Excel
	file := xlsx.NewFile()
	sheet, _ := file.AddSheet("Employees")

	if len(rows) == 0 {
		createLog(fmt.Sprintf("No employee data found for export for user %s", username), 1, apiKey, client, w)
	}

	// Cabeceras
	header := sheet.AddRow()
	for _, f := range req.Fields {
		label := f
		if nice, ok := displayNames[f]; ok {
			label = nice
		}
		ch := header.AddCell()
		ch.Value = label
	}

	// Filas
	for _, rowData := range rows {
		row := sheet.AddRow()
		for _, f := range req.Fields {
			c := row.AddCell()

			raw, ok := rowData[f]
			if !ok || raw == nil {
				c.SetString("")
				continue
			}

			// Fechas
			if dateFields[f] {
				if s, ok := raw.(string); ok {
					if t, ok := parseFlexibleTime(s); ok {
						setDateCell(c, t, "dd-mm-yyyy")
						continue
					}
				}
			}
			// Horas
			if hourFields[f] {
				switch v := raw.(type) {
				case float64:
					c.SetFloat(v / 100)
					continue
				case int:
					c.SetFloat(float64(v) / 100)
					continue
				case string:
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						c.SetFloat(num / 100)
						continue
					}
				}
			}
			// Booleanos
			if boolFields[f] {
				switch v := raw.(type) {
				case bool:
					if v {
						c.SetString("Sí")
					} else {
						c.SetString("No")
					}
					continue
				case float64:
					if v == 1 {
						c.SetString("Sí")
					} else {
						c.SetString("No")
					}
					continue
				case int:
					if v == 1 {
						c.SetString("Sí")
					} else {
						c.SetString("No")
					}
					continue
				case string:
					if v == "1" || strings.ToLower(v) == "true" {
						c.SetString("Sí")
					} else {
						c.SetString("No")
					}
					continue
				}
			}

			// Traducciones de códigos a nombres
			if f == "nationality" || f == "birth_country" || f == "residence_country" || f == "graduation_country" {
				if code, ok := raw.(string); ok {
					if name, exists := countryMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}
			if f == "birth_province" || f == "residence_province" {
				if code, ok := raw.(string); ok {
					if name, exists := provinceMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}
			if f == "birth_city" || f == "residence_city" {
				if code, ok := raw.(string); ok {
					if name, exists := cityMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}
			if f == "academic_grade" {
				if code, ok := raw.(string); ok {
					if name, exists := academicMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}
			if f == "contracting_institution" || f == "graduation_university" || f == "phd_university" {
				if code, ok := raw.(string); ok {
					if name, exists := institutionMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}

			if f == "trainee_studies" || f == "grade_code" || f == "phd_university" {
				if code, ok := raw.(string); ok {
					if name, exists := studiesMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}

			if f == "supervisor" || f == "phd_centerResponsible" {
				var key string
				switch v := raw.(type) {
				case string:
					key = v
				case float64:
					key = fmt.Sprintf("%.0f", v)
				case int:
					key = strconv.Itoa(v)
				default:
					key = fmt.Sprintf("%v", v)
				}

				if name, exists := workersMap[key]; exists {
					c.SetString(name)
					continue
				}
			}

			if f == "office" {
				var key string
				switch v := raw.(type) {
				case string:
					key = v
				case float64:
					key = fmt.Sprintf("%.0f", v)
				case int:
					key = strconv.Itoa(v)
				default:
					key = fmt.Sprintf("%v", v)
				}

				if name, exists := officeMap[key]; exists {
					c.SetString(name)
					continue
				}
			}
			// Funding -> nombre descriptivo
			if f == "funding" {
				if code, ok := raw.(string); ok {
					if name, exists := fundingMap[code]; exists {
						c.SetString(name)
						continue
					}
				}
			}
			if f == "training_name" {
				var key string
				switch v := raw.(type) {
				case string:
					key = v
				case float64:
					key = fmt.Sprintf("%.0f", v)
				case int:
					key = strconv.Itoa(v)
				default:
					key = fmt.Sprintf("%v", v)
				}

				if name, exists := trainingMap[key]; exists {
					c.SetString(name)
					continue
				}
			}
			// camps de text lliure que poden contenir caràcters parsejats
			if f == "vinculation_type" || f == "address" || f == "name" || f == "surname" || f == "secondSurname" || f == "position" || f == "grade_master_doctorate" || f == "gradeName" || f == "universityName" || f == "phd_program" || f == "phd_tesisTitle" || f == "phd_tesisDirector" || f == "phd_externalResponsible" || f == "training_name" {
				if vinc, ok := raw.(string); ok {
					c.SetString(unescapeComma(vinc))
					continue

				}
			}

			c.SetString(fmt.Sprintf("%v", raw))
		}
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=employee_data.xlsx")
	createLog(fmt.Sprintf("Successfully exported employee data (%d records, %d selected fields) for user %s", len(rows), len(req.Fields), username), 0, apiKey, client, w)

	if err := file.Write(w); err != nil {
		http.Error(w, "Error writing Excel file", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Failed to write Excel file in handleExportEmployeeData: %v for user %s", err, username), 1, apiKey, client, w)
		return
	}

}

// Export CSV
func handleExportCSV(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	_, username, _ := getUserInfo(apiKey, client, w, r)
	var req struct {
		Fields []string `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		createLog(fmt.Sprintf("Invalid JSON in handleExportCSV: %v for user %s", err, username), 1, apiKey, client, w)
		return
	}

	if len(req.Fields) == 0 {
		http.Error(w, "No fields selected", http.StatusBadRequest)
		createLog(fmt.Sprintf("No fields selected in handleExportCSV for user %s", username), 1, apiKey, client, w)
		return
	}

	// Cargar los mapas de traducción
	countryMap := loadCountryMap(apiKey, client)
	provinceMap := loadProvinceMap(apiKey, client)
	cityMap := loadCityMap(apiKey, client)
	academicMap := loadAcademicMap(apiKey, client)
	institutionMap := loadInstitutionsMap(apiKey, client)
	studiesMap := loadStudiesMap(apiKey, client)
	workersMap := loadWorkersMap(apiKey, client)
	officeMap := loadOfficeMap(apiKey, client)
	trainingMap := loadTrainingsMap(apiKey, client)

	// Mapa camps front-end -> Data base
	allColumns := map[string]string{
		//taula people
		"people_id":         "p.id AS people_id",
		"people_name":       "p.name AS people_name",
		"surname":           "p.surname",
		"secondSurname":     "p.secondSurname",
		"gender":            "p.gender",
		"birth_date":        "p.birth_date",
		"nationality":       "n.nationality_code AS nationality",
		"birth_country":     "p.birth_country",
		"birth_province":    "p.birth_province",
		"birth_city":        "p.birth_city",
		"nif":               "p.nif",
		"nif_extended":      "p.nif_extended",
		"academic_grade":    "p.academic_grade",
		"user_phone":        "p.user_phone",
		"emergency_phone":   "p.emergencyContact_phone  AS emergency_phone",
		"user_email":        "p.user_email",
		"employee_email":    "p.crm_email AS employee_email",
		"orcid":             "p.orcid",
		"i3":                "p.certificat_I3 AS i3",
		"webUser_ID":        "p.webUser_idExternal AS webUser_ID",
		"people_idExternal": "p.people_idExternal AS people_idExternal",
		"active":            "p.active",
		"agreesToUneix":     "p.agreesToUneix",

		//taula residence
		"residence_country":  "r.residence_country",
		"residence_province": "r.residence_province",
		"residence_city":     "r.residence_city",
		"address":            "r.address",
		"postal_code":        "r.postal_code",
		"actual":             "r.actual",

		//taula contract
		"vinculation_type":        "c.vinculation_type",
		"contracting_institution": "c.contracting_institution",
		"position":                "c.position",
		"trainee_type":            "c.trainee_type",
		"trainee_studies":         "c.trainee_studies",
		"internship":              "c.internship",
		"job_category":            "c.job_category",
		"contract_type":           "c.type AS contract_type",
		"contract_start_date":     "c.start_date AS contract_start_date",
		"contract_end_date":       "c.end_date AS contract_end_date",
		"totalDedication_hours":   "c.totalDedication_hours",
		"supervisor":              "s.supervisor_id AS supervisor",
		"office":                  "c.office_location AS office",
		"funding":                 "c.funding",

		//taula grade
		"grade_master_doctorate": "pg.grade_master_doctorate",
		"grade_code":             "pg.code AS grade_code",
		"gradeName":              "pg.gradeName",
		"graduation_university":  "pg.graduation_university",
		"universityName":         "pg.universityName",
		"graduation_country":     "pg.graduation_country",
		"graduation_year":        "pg.graduation_year",

		//taula group
		"group_intern_code": "gr.group_intern_code",
		"group_start_date":  "gr.start_date AS group_start_date",
		"group_end_date":    "gr.end_date AS group_end_date",
		"ip":                "gr.ip",

		//taula phd
		"phd_program":                 "phd.phd_program",
		"phd_tesisTitle":              "phd.phd_tesisTitle",
		"phd_startYear":               "phd.phd_startYear",
		"phd_tesisDirector":           "phd.phd_tesisDirector",
		"phd_university":              "phd.phd_university",
		"phd_plannedPresentationDate": "phd.phd_plannedPresentationDate",
		"phd_presentationDate":        "phd.phd_presentationDate",
		"phd_link":                    "phd.phd_link",

		//taula responsible
		"phd_centerResponsible":   "resp.people_id AS phd_centerResponsible",
		"phd_externalResponsible": "resp.name AS phd_externalResponsible",
		"phd_IP":                  "resp.ip_or_tutor AS phd_IP",

		//taula training
		"training_name": "t.training_id AS training_name",
		"training_date": "t.date AS training_date",
	}

	// Mapa camps back-end -> excel
	displayNames := map[string]string{
		"people_id":         "User ID",
		"people_name":       "Name",
		"surname":           "Surname",
		"secondSurname":     "Second Surname",
		"gender":            "Gender",
		"birth_date":        "Birth Date",
		"nationality":       "Nationality",
		"birth_country":     "Birth Country",
		"birth_province":    "Birth Province",
		"birth_city":        "Birth City",
		"nif":               "NIF",
		"nif_extended":      "NIF Extended",
		"academic_grade":    "Academic Grade",
		"user_phone":        "Phone",
		"emergency_phone":   "Emergency Phone",
		"user_email":        "Personal Email",
		"employee_email":    "Institutional Email",
		"orcid":             "ORCID",
		"i3":                "Certificat I3?",
		"webUser_ID":        "Web User ID",
		"people_idExternal": "External ID",
		"active":            "Currently employed at CRM?",
		"agreesToUneix":     "Agrees to UNEIX?",

		"residence_country":  "Residence Country",
		"residence_province": "Residence Province",
		"residence_city":     "Residence City",
		"address":            "Address",
		"postal_code":        "Postal Code",
		"actual":             "Current Residence?",

		"vinculation_type":        "Vinculation Type",
		"contracting_institution": "Contracting Institution",
		"position":                "Position",
		"trainee_type":            "Trainee Type",
		"trainee_studies":         "Trainee Studies",
		"internship":              "Internship",
		"job_category":            "Job Category",
		"contract_type":           "Contract Type",
		"contract_start_date":     "Contract Start Date",
		"contract_end_date":       "Contract End Date",
		"totalDedication_hours":   "Working hours per week",
		"supervisor":              "Supervisor",
		"office":                  "Office Location",
		"funding":                 "Funding",

		"grade_master_doctorate": "Grade Type",
		"grade_code":             "Grade Name",
		"gradeName":              "Grade (other)",
		"graduation_university":  "Graduation University",
		"universityName":         "University (other)",
		"graduation_country":     "Graduation Country",
		"graduation_year":        "Graduation Year",

		"group_intern_code": "Group Code",
		"group_start_date":  "Group Start Date",
		"group_end_date":    "Group End Date",
		"ip":                "Is IP?",

		"phd_program":                 "PhD Program",
		"phd_tesisTitle":              "Thesis Title",
		"phd_startYear":               "PhD Start Year",
		"phd_tesisDirector":           "Thesis Director",
		"phd_university":              "PhD University",
		"phd_plannedPresentationDate": "Planned Presentation Date",
		"phd_presentationDate":        "Presentation Date",
		"phd_link":                    "PhD Link",
		"phd_centerResponsible":       "Center Responsible",
		"phd_externalResponsible":     "External Responsible",
		"phd_IP":                      "IP / Tutor",

		"training_name": "Training Name",
		"training_date": "Training Date",
	}

	// Format camps que són dates
	var dateFields = map[string]bool{
		"birth_date":                  true,
		"contract_start_date":         true,
		"contract_end_date":           true,
		"group_start_date":            true,
		"group_end_date":              true,
		"phd_plannedPresentationDate": true,
		"phd_presentationDate":        true,
		"training_date":               true,
	}
	// Format camps que són hores (dividir entre 100)
	var hourFields = map[string]bool{
		"totalDedication_hours": true,
	}
	// Format camps booleans
	var boolFields = map[string]bool{
		"i3":            true,
		"active":        true,
		"agreesToUneix": true,
		"actual":        true,
		"internship":    true,
		"ip":            true,
	}
	// Traducció beques BD -> nom
	var fundingMap = map[string]string{
		"0000000175": "BP: Ajuts postdoctorals Beatriu de Pinós",
		"0000000251": "FPU: Contratos o becas predoctorales para la formación de doctores (FPU)",
		"0000000332": "FPI: Contratos o becas predoctorales para la formación de doctores (FPI)",
		"0000000375": "FI: Ajuts Joan Oró per a la contractació de personal investigador predoctoral en formació",
		"0000000469": "ICREA-SR: Contractes a Investigadors ICREA",
		"0000000533": "RYC: Ramon y Cajal",
		"0000002204": "HORIZON-MSCA-PF: Marie Skłodowska-Curie Postdoctoral Fellowship (PF)",
		"0000002325": "JDC: Contratos Juan de la Cierva",
		"0000002650": "INPHINIT: PhD Fellowship La Caixa INPHINIT",
	}

	// --------- construir columnas SQL ---------
	var selectedCols []string
	for _, f := range req.Fields {
		if sqlCol, ok := allColumns[f]; ok {
			selectedCols = append(selectedCols, sqlCol)
		}
	}

	if len(selectedCols) == 0 {
		http.Error(w, "No valid fields selected", http.StatusBadRequest)
		createLog(fmt.Sprintf("No valid SQL columns found in handleExportCSV for user %s", username), 1, apiKey, client, w)
		return
	}

	// --------- query dinámica---------
	query := map[string]interface{}{
		"table": `people p
                  LEFT JOIN residence r ON p.id = r.people_id
                  LEFT JOIN people_nationality n ON p.id = n.people_id
                  LEFT JOIN contract c ON p.id = c.people_id
                  LEFT JOIN people_grade pg ON p.id = pg.people_id
                  LEFT JOIN people_phd phd ON p.id = phd.people_id
                  LEFT JOIN responsible resp ON phd.id = resp.tesis_id
                  LEFT JOIN people_training t ON p.id = t.people_id
                  LEFT JOIN people_group gr ON p.id = gr.people_id
				  LEFT JOIN people_supervisor s ON c.id = s.contract_id`,
		"columns": strings.Join(selectedCols, ", "),
		// descartem id administratiu
		"condition": "p.id != 304",
	}

	queryJSON, _ := json.Marshal(query)
	resp := getReq(queryJSON, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Proxy response is nil", http.StatusBadGateway)
		createLog(fmt.Sprintf("Database proxy returned nil in handleExportCSV for user %s", username), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var rows []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rows); err != nil {
		createLog(fmt.Sprintf("Error decoding DB response in handleExportCSV: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error decoding response", http.StatusInternalServerError)
		return
	}

	seen := make(map[string]bool)
	unique := make([]map[string]interface{}, 0, len(rows))

	for _, row := range rows {
		var parts []string
		for _, f := range req.Fields {
			if v, ok := row[f]; ok {
				parts = append(parts, fmt.Sprintf("%v", v))
			} else {
				parts = append(parts, "")
			}
		}
		key := strings.Join(parts, "|")
		if !seen[key] {
			seen[key] = true
			unique = append(unique, row)
		}
	}

	rows = unique

	// --------- preparar CSV ---------
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=employee_data.csv")

	writer := csv.NewWriter(w)
	writer.Comma = ','

	// --------- escribir cabeceras ---------
	header := make([]string, len(req.Fields))
	for i, f := range req.Fields {
		if nice, ok := displayNames[f]; ok {
			header[i] = nice
		} else {
			header[i] = f
		}
	}
	writer.Write(header)

	// --------- escribir filas ---------
	for _, row := range rows {
		line := make([]string, len(req.Fields))

		for i, f := range req.Fields {
			raw := row[f]

			if raw == nil {
				line[i] = ""
				continue
			}

			val := fmt.Sprintf("%v", raw)

			// fechas
			if dateFields[f] {
				if t, ok := parseFlexibleTime(val); ok {
					line[i] = t.Format("02-01-2006")
					continue
				}
			}

			// horas
			if hourFields[f] {
				if num, err := strconv.ParseFloat(val, 64); err == nil {
					line[i] = fmt.Sprintf("%.2f", num/100)
					continue
				}
			}

			// booleanos
			if boolFields[f] {
				switch strings.ToLower(val) {
				case "1", "true":
					line[i] = "Sí"
				default:
					line[i] = "No"
				}
				continue
			}

			// traducción de códigos
			if f == "nationality" || f == "birth_country" || f == "residence_country" || f == "graduation_country" {
				if name, ok := countryMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "birth_province" || f == "residence_province" {
				if name, ok := provinceMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "birth_city" || f == "residence_city" {
				if name, ok := cityMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "academic_grade" {
				if name, ok := academicMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "contracting_institution" || f == "graduation_university" || f == "phd_university" {
				if name, ok := institutionMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "trainee_studies" || f == "grade_code" {
				if name, ok := studiesMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "supervisor" || f == "phd_centerResponsible" {
				if name, ok := workersMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "office" {
				if name, ok := officeMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "funding" {
				if name, ok := fundingMap[val]; ok {
					line[i] = name
					continue
				}
			}
			if f == "training_name" {
				if name, ok := trainingMap[val]; ok {
					line[i] = name
					continue
				}
			}

			// quitar escapes de coma
			line[i] = unescapeComma(val)
		}

		writer.Write(line)
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		http.Error(w, "Error writing CSV file", http.StatusInternalServerError)
		createLog(fmt.Sprintf("Failed writing CSV in handleExportCSV: %v for user %s", err, username), 1, apiKey, client, w)
		return
	}

	createLog(fmt.Sprintf("CSV export successful (%d rows, %d fields) for user %s", len(rows), len(req.Fields), username), 0, apiKey, client, w)
}

// Parseig dates a format correcte
func parseFlexibleTime(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339, // 2003-03-31T00:00:00Z
		"2006-01-02", // 2003-03-31
		"02/01/2006", // 31/03/2003
		"2006/01/02", // 2003/03/31
		"02-01-2006", // 31-03-2003
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func setDateCell(cell *xlsx.Cell, t time.Time, numFmt string) {
	if numFmt == "" {
		numFmt = "yyyy-mm-dd"
	}
	serial := xlsx.TimeToExcelTime(t, false)
	cell.SetFloatWithFormat(serial, numFmt)
}

// ------------------ Funciones para traducir de código a nombre -----------------
// Carga los países
func loadCountryMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "country",
		"columns": "code, name_EN",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		code, _ := r["code"].(string)
		name, _ := r["name_EN"].(string)
		if code != "" && name != "" {
			m[code] = name
		}
	}
	return m
}

// Carga las provincias
func loadProvinceMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "provinces",
		"columns": "code, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		code, _ := r["code"].(string)
		name, _ := r["name"].(string)
		if code != "" && name != "" {
			m[code] = name
		}
	}
	return m
}

// Carga las ciudades
func loadCityMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "cities",
		"columns": "code, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		code, _ := r["code"].(string)
		name, _ := r["name"].(string)
		if code != "" && name != "" {
			m[code] = name
		}
	}
	return m
}

// Carga nivell acadèmic
func loadAcademicMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "educationGrade",
		"columns": "code, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		code, _ := r["code"].(string)
		name, _ := r["name"].(string)
		if code != "" && name != "" {
			m[code] = name
		}
	}
	return m
}

// Carga centres contractants
func loadInstitutionsMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "universities",
		"columns": "code, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		code, _ := r["code"].(string)
		name, _ := r["name"].(string)
		if code != "" && name != "" {
			m[code] = name
		}
	}
	m["0000001672"] = "CRM"
	return m
}

// Carga estudis
func loadStudiesMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "studies",
		"columns": "code, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		code, _ := r["code"].(string)
		name, _ := r["name"].(string)
		if code != "" && name != "" {
			m[code] = name
		}
	}
	return m
}

// Carga treballadors
func loadWorkersMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "people",
		"columns": "id, name, surname",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		var idStr string
		switch v := r["id"].(type) {
		case string:
			idStr = v
		case float64:
			idStr = fmt.Sprintf("%.0f", v)
		case int:
			idStr = strconv.Itoa(v)
		}

		name, _ := r["name"].(string)
		surname, _ := r["surname"].(string)

		if idStr != "" && name != "" {
			m[idStr] = strings.TrimSpace(name + " " + surname)
		}
	}
	return m
}

// Carga oficines
func loadOfficeMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "room",
		"columns": "id, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		var idStr string
		switch v := r["id"].(type) {
		case string:
			idStr = v
		case float64:
			idStr = fmt.Sprintf("%.0f", v)
		case int:
			idStr = strconv.Itoa(v)
		}

		name, _ := r["name"].(string)
		if idStr != "" && name != "" {
			m[idStr] = name
		}
	}
	return m
}

// Carga formacions
func loadTrainingsMap(apiKey string, client *http.Client) map[string]string {
	query := map[string]interface{}{
		"table":   "training",
		"columns": "id, name",
	}
	queryJSON, _ := json.Marshal(query)

	resp := getReq(queryJSON, apiKey, client, nil)
	if resp == nil {
		return map[string]string{}
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &rows)

	m := make(map[string]string)
	for _, r := range rows {
		var idStr string
		switch v := r["id"].(type) {
		case string:
			idStr = v
		case float64:
			idStr = fmt.Sprintf("%.0f", v)
		case int:
			idStr = strconv.Itoa(v)
		}

		name, _ := r["name"].(string)
		if idStr != "" && name != "" {
			m[idStr] = name
		}
	}
	return m
}

func handleCheckRole(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, _, userRole := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"role": userRole,
	})
}

func handleGetUsersAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {

	query := map[string]interface{}{
		"table":   "users",
		"columns": "username, access_exportusers",
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		return
	}
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		log.Printf("No se obtuvo respuesta para la tabla users")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Error desde el servicio de BD en getUsersAccess. Status: %d, Body: %s",
			resp.StatusCode, string(bodyBytes))
		http.Error(w, "Error al consultar la base de datos", http.StatusInternalServerError)
		return
	}
	type dbUser struct {
		Username      string `json:"username"`
		AccessUserhub int    `json:"access_exportusers"`
	}

	var dbUsers []dbUser
	if err := json.NewDecoder(resp.Body).Decode(&dbUsers); err != nil {
		log.Printf("Error al decodificar respuesta de BD en getUsersAccess: %v", err)
		http.Error(w, "Error al procesar datos de usuarios", http.StatusInternalServerError)
		return
	}

	users := make([]map[string]string, 0, len(dbUsers))
	allowed := make([]string, 0)

	for _, u := range dbUsers {
		users = append(users, map[string]string{
			"username": u.Username,
		})

		if u.AccessUserhub == 1 {
			allowed = append(allowed, u.Username)
		}
	}

	response := map[string]interface{}{
		"users":   users,
		"allowed": allowed,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error al codificar respuesta JSON en getUsersAccess: %v", err)
		http.Error(w, "Error interno al enviar la respuesta", http.StatusInternalServerError)
		return
	}
}

func handleSetUsersAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	w.Header().Set("Content-Type", "application/json")

	// Leer body enviado desde frontend
	var requestData struct {
		Users []string `json:"users"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		createLog(fmt.Sprintf("Error processing JSON in handleSetUsersAccess: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error al procesar la solicitud", http.StatusBadRequest)
		return
	}

	// Resetear accessos a tothom menys a crmAdmin
	resetQuery := map[string]interface{}{
		"table":     "users",
		"columns":   "access_exportusers",
		"value":     "0",
		"condition": "people_id != '304'",
	}

	resetJson, _ := json.Marshal(resetQuery)

	resp := putReq(resetJson, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error reseting access in handleSetUsersAccess for user %s", username), 1, apiKey, client, w)
		http.Error(w, "Error reseteando accesos", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Para cada usuario enviado, activar acceso
	for _, u := range requestData.Users {
		updateQuery := map[string]interface{}{
			"table":     "users",
			"columns":   "access_exportusers",
			"value":     "1",
			"condition": fmt.Sprintf("username = '%s'", u),
		}

		updateJson, _ := json.Marshal(updateQuery)
		resp := putReq(updateJson, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error asignando acceso a usuario: %s for user %s", u, username), 1, apiKey, client, w)
		}
	}

	response := map[string]string{
		"message": "Access permissions updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func handleCheckAccess(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	userID, username, _ := getUserInfo(apiKey, client, w, r)

	query := map[string]interface{}{
		"table":     "users",
		"columns":   "access_exportusers",
		"condition": fmt.Sprintf("people_id = '%d'", userID),
	}
	jsonQuery, err := json.Marshal(query)
	if err != nil {
		createLog(fmt.Sprintf("Error marshaling JSON in handleCheckAccess: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error interno al preparar la consulta", http.StatusInternalServerError)
		return
	}
	resp := getReq(jsonQuery, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("No response received for users table in handleCheckAccess for user %s", username), 1, apiKey, client, w)
		return
	}
	defer resp.Body.Close()

	type accessResult struct {
		AccessUserhub int `json:"access_exportusers"`
	}

	var results []accessResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		createLog(fmt.Sprintf("Error decoding DB response in handleCheckAccess: %v for user %s", err, username), 1, apiKey, client, w)
		http.Error(w, "Error al procesar datos de acceso", http.StatusInternalServerError)
		return
	}

	if len(results) == 0 {
		json.NewEncoder(w).Encode(map[string]bool{
			"can_access_userhub": false,
		})
		return
	}

	canAccess := results[0].AccessUserhub == 1

	json.NewEncoder(w).Encode(map[string]bool{
		"can_access_userhub": canAccess,
	})

}
