package module2workers

import (
	"bytes"
	"html/template"
	"strconv"

	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// -------------------------------- json encoding --------------------------------------------
//
// GET hot tables by floor
func jsonHotTablesByFloor(floor int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "hottable",
		"columns":   "id,is_hot,capacity,floor, name, position",
		"condition": fmt.Sprintf("is_hot='true' AND floor='%d'", floor),
	}
	return query
}

// GET hot tables
func jsonAllHotTables() map[string]interface{} {
	query := map[string]interface{}{
		"table":     "hottable",
		"columns":   "id,is_hot,capacity,floor, name, position",
		"condition": "is_hot='true'",
	}
	return query
}

// GET table name with ID
func jsonTableName(id int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "hottable",
		"columns":   "name",
		"condition": fmt.Sprintf("id='%d'", id),
	}
	return query
}

// GET query1 -> tables with reservations, query2 -> tables without reservations
func jsonAvailableTable(mesas []HotTable, startDate string, endDate string) (map[string]interface{}, map[string]interface{}) {

	var idStrings []string
	for _, m := range mesas {
		idStrings = append(idStrings, fmt.Sprintf("%d", m.ID))
	}
	idList := strings.Join(idStrings, ",")

	query1 := map[string]interface{}{
		"table":   "reservation",
		"columns": "id,table_id, type, start_date, end_date",
		"condition": fmt.Sprintf(
			"table_id IN (%s) AND NOT (end_date <= '%s' OR start_date >= '%s')",
			idList, startDate, endDate,
		)}

	query2 := map[string]interface{}{
		"table":   "reservation",
		"columns": "id,table_id, type, start_date, end_date",
		"condition": fmt.Sprintf(
			"table_id IN (%s) AND (end_date <= '%s' OR start_date >= '%s')",
			idList, startDate, endDate,
		)}

	return query1, query2

}

// GET user_id by username
func jsonUsernameID(user string) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "users",
		"columns":   "people_id, role",
		"condition": fmt.Sprintf("username = '%s'", user),
	}
	return query
}

// GET reservation_id by table_id and timeframe
func jsonReservationID(tableID int, startDate string, endDate string) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "reservation",
		"columns":   "id",
		"condition": fmt.Sprintf("table_id = %d AND start_date = '%s' AND end_date = '%s'", tableID, startDate, endDate),
	}
	return query
}

// GET quantity of users per reservation
func jsonCountUsersInReservation(reservaID int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "reservation_user",
		"columns":   "count(user_id)",
		"condition": fmt.Sprintf("reservation_id = %d", reservaID),
	}
	return query
}

// GET reservations by user's id
func jsonActiveReservations(reservaID int, userID int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "reservation_user",
		"columns":   "reservation_id",
		"condition": fmt.Sprintf("reservation_id = %d AND user_id = %d", reservaID, userID),
	}
	return query
}

// GET incoming reservations
func jsonFutureReservationsByUser(userID int) map[string]interface{} {
	now := time.Now().Format("2006-01-02 15:04:05")
	return map[string]interface{}{
		"table":   "reservation r JOIN hottable h ON r.table_id = h.id",
		"columns": "r.id, h.name AS table_name, r.concept, r.start_date, r.end_date, h.floor, h.position",
		"condition": fmt.Sprintf(
			"r.id IN (SELECT reservation_id FROM reservation_user WHERE user_id = %d) AND r.start_date >= '%s'",
			userID, now,
		),
	}
}

// GET reservations
func jsonReservationsByUser(userID int) map[string]interface{} {
	return map[string]interface{}{
		"table":   "reservation r JOIN hottable h ON r.table_id = h.id",
		"columns": "h.name AS table_name, r.concept, r.start_date, r.end_date",
		"condition": fmt.Sprintf(
			"r.id IN (SELECT reservation_id FROM reservation_user WHERE user_id = %d)",
			userID,
		),
	}
}

func jsonReservations() map[string]interface{} {
	return map[string]interface{}{
		"table":   "reservation JOIN hottable h ON reservation.table_id = h.id JOIN reservation_user ON reservation.id = reservation_user.reservation_id LEFT JOIN people ON reservation_user.user_id = people.id",
		"columns": "h.name AS table_name, CONCAT(people.name, ' ', people.surname) AS worker, reservation.concept, reservation_user.host, reservation_user.role, reservation_user.program, reservation_user.name AS guest_name, reservation.start_date, reservation.end_date",
	}
}

// PUT  reservation
func jsonUpdateReservation(id int, concept string, startDate string, endDate string) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "reservation",
		"columns":   "concept, start_date, end_date",
		"value":     fmt.Sprintf("%s, %s, %s", concept, startDate, endDate),
		"condition": fmt.Sprintf("id = %d", id),
	}

	return query
}

// DELETE reservation
func jsonDeleteReservation(id int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "reservation",
		"condition": fmt.Sprintf("id = %d", id),
	}

	return query
}

// DELETE reservation_user
func jsonDeleteReservationUser(id int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "reservation_user",
		"condition": fmt.Sprintf("reservation_id = %d", id),
	}

	return query
}

// POST a new reservation
func buildInsertReservation(tableID int, concept string, startDate string, endDate string) map[string]interface{} {
	query := map[string]interface{}{
		"table":   "reservation",
		"columns": "table_id, concept, start_date, end_date",
		"value":   fmt.Sprintf("%d, %s, %s, %s", tableID, concept, startDate, endDate),
	}
	return query
}

// POST a new reservation_user entry
func buildInsertUserReservation(userID int, reservationID int) map[string]interface{} {
	query := map[string]interface{}{
		"table":   "reservation_user",
		"columns": "reservation_id, user_id",
		"value":   fmt.Sprintf("%d, %d", reservationID, userID),
	}
	return query
}

// ------------------------------------------- consults --------------------------------------
//
// calculates how many users there are per reservation (outdated but used)
func countReservationsByMesa(reserves []ConsultaColisiones, apiKey string, client *http.Client, w http.ResponseWriter) map[int]int {
	conteo := make(map[int]int)
	processed := make(map[int]bool)
	for _, reserva := range reserves {
		if processed[reserva.ID] {
			continue
		}
		processed[reserva.ID] = true
		query := jsonCountUsersInReservation(reserva.ID)
		jsonData, _ := json.Marshal(query)
		resp := getReq(jsonData, apiKey, client, w)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var result []map[string]interface{}
		_ = json.Unmarshal(body, &result)
		if len(result) > 0 {
			if count, ok := result[0]["count(user_id)"].(float64); ok {
				conteo[reserva.TableID] += int(count)
			}
		}
	}
	return conteo
}

// checks if any activeReservations (reservations within the timeframe) is from user
func checkActiveReservations(userID int, reserves []ConsultaColisiones, apiKey string, client *http.Client, w http.ResponseWriter) bool {
	processed := make(map[int]bool)
	for _, reserva := range reserves {
		if processed[reserva.ID] {
			continue
		}
		processed[reserva.ID] = true

		query := jsonActiveReservations(reserva.ID, userID)
		jsonData, _ := json.Marshal(query)

		resp := getReq(jsonData, apiKey, client, w)
		if resp == nil {
			fmt.Printf("Warning: nil response for reservation_id=%d, user_id=%d\n", reserva.ID, userID)
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var result []map[string]interface{}
		_ = json.Unmarshal(body, &result)

		if len(result) > 0 {
			return true
		}
	}
	return false
}

// connects to host and sends email
func sendConfirmationEmail(to string, subject string, body string) error {
	from := mC.SmtpUser
	headers := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + subject + "\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\n\n"

	msg := headers + body
	conn, _ := net.Dial("tcp", mC.SmtpHost)
	client, _ := smtp.NewClient(conn, "crm.cat")
	defer client.Quit()
	_ = client.Mail(from)
	_ = client.Rcpt(to)
	wc, _ := client.Data()
	_, _ = wc.Write([]byte(msg))
	_ = wc.Close()
	return nil
}

// reads email template from file, fills it and sends email
func sendConfirmationEmailFromTemplate(reserva ReservaEntry, email string) error {

	tmplContent, err := os.ReadFile("module2workers/assets/emailTemplate.html")
	if err != nil {
		println("Error leyendo plantilla:", err.Error())
		return fmt.Errorf("error reading template: %w", err)
	}

	tmpl, err := template.New("email").Parse(string(tmplContent))
	if err != nil {
		println("Error parseando plantilla:", err.Error())
		return fmt.Errorf("error parsing template: %w", err)
	}
	data := map[string]string{
		"Name":      reserva.Name,
		"StartDate": reserva.StartDate,
		"EndDate":   reserva.EndDate,
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		println("Error ejecutando plantilla:", err.Error())
		return fmt.Errorf("error executing template: %w", err)
	}
	subject := "Table Reservation Confirmation"
	return sendConfirmationEmail(email, subject, body.String())
}

// helper to return today if empty
func dayOrToday(s string) string {
	if s == "" {
		return time.Now().Format("2006-01-02")
	}
	return s
}

// ------------------------------------------- handlers --------------------------------------
// handles the request to send desks to frontend
func handleSendDesksToFE(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {
	floorStr := r.URL.Query().Get("floor")
	var condition string
	if floorStr != "" {
		floor, err := strconv.Atoi(floorStr)
		if err != nil {
			http.Error(w, "invalid floor", http.StatusBadRequest)
			return
		}
		condition = fmt.Sprintf("h.floor = %d", floor)
	} else {
		// sin floor => todas las plantas
		condition = "1=1"
	}

	start := dayOrToday(r.URL.Query().Get("start_date"))
	end := dayOrToday(r.URL.Query().Get("end_date"))

	startDT := start + " 00:00:00"
	endDT := end + " 23:59:59"

	// Bringing desks with their assignments, reservations and releases
	// Filtering by date range

	query := map[string]interface{}{
		"table": fmt.Sprintf(`
			hottable h
			LEFT JOIN reservation r
				ON r.table_id = h.id
				AND r.start_date <= '%s'
				AND COALESCE(r.end_date, '2099-12-31 23:59:59') >= '%s'
			LEFT JOIN reservation_user ru
				ON r.id = ru.reservation_id
			LEFT JOIN people p
				ON ru.user_id = p.id
			LEFT JOIN contract c
				ON ru.contract_id = c.id
			LEFT JOIN room rm
				ON h.room_id = rm.id
		`, endDT, startDT),

		"columns": `
			h.id        AS desk_id,
			h.name      AS desk_name,
			h.floor     AS desk_floor,
			h.is_hot    AS desk_is_hot,
			rm.uab_code AS desk_room_uab_code,

			r.id        AS occ_id,
			DATE(r.start_date) AS occ_start,
			CASE WHEN r.end_date IS NULL THEN NULL ELSE DATE(r.end_date) END AS occ_end,
			r.type      AS occ_type,

			ru.user_id,
			ru.role,
			ru.host,
			ru.program,
			ru.name,

			(p.name || ' ' || p.surname) AS person_name,

			c.id AS contract_id,
			c.vinculation_type AS contract_vinculation_type,
			c.type AS contract_type,
			c.position AS contract_position,
			DATE(c.start_date) AS contract_start_date,
			CASE WHEN c.end_date IS NULL THEN NULL ELSE DATE(c.end_date) END AS contract_end_date
		`,
		"condition": condition,
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Failed to fetch desks", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	type row struct {
		DeskID    int    `json:"desk_id"`
		DeskName  string `json:"desk_name"`
		DeskFloor int    `json:"desk_floor"`
		DeskIsHot any    `json:"desk_is_hot"`
		RoomUab   string `json:"desk_room_uab_code"`

		UserID       *int    `json:"user_id"`
		GuestRole    *string `json:"role"`
		GuestHost    *string `json:"host"`
		GuestProgram *string `json:"program"`
		GuestName    *string `json:"name"`

		OccID    *int    `json:"occ_id"`
		OccStart *string `json:"occ_start"`
		OccEnd   *string `json:"occ_end"`
		OccType  *string `json:"occ_type"`
		Person   *string `json:"person_name"`

		ContractID              *int    `json:"contract_id"`
		ContractVinculationType *string `json:"contract_vinculation_type"`
		ContractType            *string `json:"contract_type"`
		ContractPosition        *string `json:"contract_position"`
		ContractStartDate       *string `json:"contract_start_date"`
		ContractEndDate         *string `json:"contract_end_date"`
	}

	var rows []row
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		http.Error(w, "Failed to decode desks", http.StatusInternalServerError)
		return
	}

	deskMap := map[int]*DeskDTO{}

	toBool := func(v any) bool {
		switch t := v.(type) {
		case bool:
			return t
		case float64:
			return t != 0
		case string:
			return t == "true" || t == "1"
		default:
			return false
		}
	}

	for _, rw := range rows {
		d, ok := deskMap[rw.DeskID]
		if !ok {
			d = &DeskDTO{
				ID: rw.DeskID, Name: rw.DeskName, Floor: rw.DeskFloor, Room: rw.RoomUab,
				IsHot:        toBool(rw.DeskIsHot),
				Assignments:  []OccDTO{},
				Reservations: []OccDTO{},
				Releases:     []OccDTO{},
			}
			deskMap[rw.DeskID] = d
		}

		if rw.OccID == nil || rw.OccType == nil || rw.OccStart == nil {
			continue
		}
		person := ""
		if rw.Person != nil {
			//fmt.Printf("Found person for reservation: %s\n", *rw.Person)
			person = unescapeComma(*rw.Person)
		}
		unescapePtr := func(value *string) *string {
			if value == nil {
				return nil
			}
			out := unescapeComma(*value)
			return &out
		}

		// append occupation to proper slice
		occ := OccDTO{
			ID:                      *rw.OccID,
			StartDate:               *rw.OccStart,
			EndDate:                 rw.OccEnd,
			Person:                  person,
			UserID:                  rw.UserID,
			GuestName:               rw.GuestName,
			GuestHost:               rw.GuestHost,
			GuestProgram:            rw.GuestProgram,
			GuestRole:               rw.GuestRole,
			ContractID:              rw.ContractID,
			ContractVinculationType: unescapePtr(rw.ContractVinculationType),
			ContractType:            unescapePtr(rw.ContractType),
			ContractPosition:        unescapePtr(rw.ContractPosition),
			ContractStartDate:       rw.ContractStartDate,
			ContractEndDate:         rw.ContractEndDate,
		}

		t := strings.ToLower(strings.TrimSpace(*rw.OccType))

		switch t {
		case "assigned":
			d.Assignments = append(d.Assignments, occ)
		case "released":
			d.Releases = append(d.Releases, occ)
		default:
			d.Reservations = append(d.Reservations, occ)
		}
	}

	// a slice
	out := make([]DeskDTO, 0, len(deskMap))
	for _, d := range deskMap {
		out = append(out, *d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handles the request to send people to frontend
func handleSendWorkersToFE(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {

	now := time.Now().Format("2006-01-02 15:04:05")

	// Bringing people with active contracts
	query := map[string]interface{}{
		"table":     "people p LEFT JOIN contract c ON p.id = c.people_id",
		"columns":   "p.id, p.name, p.surname",
		"condition": fmt.Sprintf("c.end_date >= '%s' OR c.end_date IS NULL ", now),
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(
			fmt.Sprintf("Error: getReq returned nil for user %s", user),
			1, apiKey, client, w,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to fetch people",
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	//println(fmt.Sprintf("People raw body for user %s: %s", user, string(body)))

	var people []PersonDTO
	if err := json.NewDecoder(resp.Body).Decode(&people); err != nil {
		//log.Println("DECODE ERROR:", err)
		createLog(
			fmt.Sprintf("Error decoding people JSON for user %s: %v", user, err),
			1, apiKey, client, w,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to decode people",
		})
		return
	}

	for _, person := range people {
		person.Name = unescapeComma(person.Name)
		person.Surname = unescapeComma(person.Surname)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(people)
}

// handles the request to send contracts for a person to frontend
func handleSendPersonContractsToFE(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {
	peopleID, err := strconv.Atoi(r.URL.Query().Get("people_id"))
	if err != nil || peopleID == 0 {
		http.Error(w, "people_id is required", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "contract",
		"columns":   "id, people_id, vinculation_type, type, position, start_date, end_date",
		"condition": fmt.Sprintf("people_id = %d ORDER BY start_date DESC", peopleID),
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: getReq contracts returned nil for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch contracts", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var contracts []ContractDTO
	if err := json.NewDecoder(resp.Body).Decode(&contracts); err != nil {
		createLog(fmt.Sprintf("Error decoding contracts JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "Failed to decode contracts", http.StatusInternalServerError)
		return
	}

	for i := range contracts {
		if contracts[i].VinculationType != nil {
			value := unescapeComma(*contracts[i].VinculationType)
			contracts[i].VinculationType = &value
		}
		if contracts[i].Type != nil {
			value := unescapeComma(*contracts[i].Type)
			contracts[i].Type = &value
		}
		if contracts[i].Position != nil {
			value := unescapeComma(*contracts[i].Position)
			contracts[i].Position = &value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contracts)
}

// handles the request to release a desk
func handleReleaseDesk(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req ReleaseDeskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		createLog(fmt.Sprintf("Error decoding release desk JSON for user %s: %v", user, err), 1, apiKey, client, w)
		return
	}

	// validations
	if req.DeskID == 0 || req.StartDate == "" || req.EndDate == "" {
		http.Error(w, "desk_id, start_date, end_date are required", http.StatusBadRequest)
		return
	}

	if req.EndDate < req.StartDate {
		http.Error(w, "end_date must be >= start_date", http.StatusBadRequest)
		return
	}

	// table assignments fill full days -> adjust time to cover full days
	startDT := req.StartDate + " 00:00"
	endDT := req.EndDate + " 23:59"

	query := map[string]interface{}{
		"table":   "reservation",
		"columns": "table_id, start_date, end_date, type",
		"value": fmt.Sprintf("%d, %s, %s, %s",
			req.DeskID, startDT, endDT, "released",
		),
	}

	jsonData, _ := json.Marshal(query)

	resp := postReq(jsonData, apiKey, client, w)
	if resp == nil {
		http.Error(w, "Failed to create release", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handles the request to assign a desk
func handleAssignDesk(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req AssignDeskReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("Error decoding assign desk JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	dateOnly := func(s string) string {
		if len(s) >= 10 {
			return s[:10]
		}
		return s
	}

	req.StartDate = dateOnly(req.StartDate)
	req.EndDate = dateOnly(req.EndDate)

	// validations
	if req.DeskID == 0 || req.StartDate == "" {
		http.Error(w, "desk_id, start_date, people_id are required", http.StatusBadRequest)
		return
	}

	if req.EndDate != "" && req.EndDate < req.StartDate {
		http.Error(w, "end_date must be >= start_date", http.StatusBadRequest)
		return
	}
	if !req.Guest && req.PersonID == 0 {
		http.Error(w, "person_id is required for non-guest assignments", http.StatusBadRequest)
		return
	}

	if !req.Guest && req.ContractID != 0 {
		query := map[string]interface{}{
			"table":     "contract",
			"columns":   "id, people_id, start_date, end_date",
			"condition": fmt.Sprintf("id = %d AND people_id = %d", req.ContractID, req.PersonID),
		}
		jsonData, _ := json.Marshal(query)
		resp := getReq(jsonData, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error: getReq selected contract returned nil for user %s", user), 1, apiKey, client, w)
			http.Error(w, "Failed to fetch selected contract", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var contracts []ContractDTO
		if err := json.NewDecoder(resp.Body).Decode(&contracts); err != nil {
			createLog(fmt.Sprintf("Error decoding selected contract JSON for user %s: %v", user, err), 1, apiKey, client, w)
			http.Error(w, "Failed to decode selected contract", http.StatusInternalServerError)
			return
		}
		if len(contracts) == 0 || contracts[0].StartDate == "" {
			http.Error(w, "selected contract not found", http.StatusBadRequest)
			return
		}

		req.StartDate = dateOnly(contracts[0].StartDate)
		req.EndDate = ""
		if contracts[0].EndDate != nil {
			req.EndDate = dateOnly(*contracts[0].EndDate)
		}
		if req.EndDate != "" && req.EndDate < req.StartDate {
			http.Error(w, "contract end_date must be >= start_date", http.StatusBadRequest)
			return
		}
	}

	// table assignments fill full days -> adjust time to cover full days
	var endDT string
	startDT := req.StartDate + " 08:00"
	if req.EndDate != "" {
		endDT = req.EndDate + " 17:00"
	} else {
		endDT = "2099-12-31 17:00"
	}

	query := map[string]interface{}{
		"table":   "reservation",
		"columns": "table_id, start_date, end_date, type",
		"value": fmt.Sprintf("%d, %s, %s, %s",
			req.DeskID, startDT, endDT, "assigned",
		),
	}

	jsonData, _ := json.Marshal(query)

	resp := postReq(jsonData, apiKey, client, w)

	if resp == nil {
		http.Error(w, "Failed to create release", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	query = map[string]interface{}{
		"table":   "reservation",
		"columns": "id",
		"condition": fmt.Sprintf("table_id = %d AND start_date = '%s' AND end_date = '%s'",
			req.DeskID, startDT, endDT,
		),
	}
	jsonData, _ = json.Marshal(query)
	resp = getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(
			fmt.Sprintf("Error: getReq returned nil for user %s", user),
			1, apiKey, client, w,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to fetch reservation",
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	var rows []struct {
		Id int `json:"id"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		createLog(
			fmt.Sprintf("Error decoding reservation JSON for user %s: %v", user, err),
			1, apiKey, client, w,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to decode people",
		})
		return
	}
	reservationID := rows[0].Id

	if !req.Guest {
		columns := "reservation_id, user_id"
		value := fmt.Sprintf("%d, %d", reservationID, req.PersonID)
		if req.ContractID != 0 {
			columns = "reservation_id, user_id, contract_id"
			value = fmt.Sprintf("%d, %d, %d", reservationID, req.PersonID, req.ContractID)
		}

		query = map[string]interface{}{
			"table":   "reservation_user",
			"columns": columns,
			"value":   value,
		}
	} else {
		req.GuestName = escapeComma(req.GuestName)
		req.GuestHost = escapeComma(req.GuestHost)
		req.GuestProgram = escapeComma(req.GuestProgram)

		query = map[string]interface{}{
			"table":   "reservation_user",
			"columns": "reservation_id, role, host, program, name",
			"value":   fmt.Sprintf("%d, %s, %s, %s, %s", reservationID, req.GuestRole, req.GuestHost, req.GuestProgram, req.GuestName),
		}
	}
	jsonData, _ = json.Marshal(query)

	resp = postReq(jsonData, apiKey, client, w)

	if resp == nil {
		http.Error(w, "Failed to assign desk", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	// afegir oficina al treballador si no és guest
	if !req.Guest {

		// get del room id
		query = map[string]interface{}{
			"table":     "hottable",
			"columns":   "room_id",
			"condition": fmt.Sprintf("id = %d", req.DeskID),
		}
		jsonData, _ = json.Marshal(query)
		resp = getReq(jsonData, apiKey, client, w)
		var rows []struct {
			RoomID int `json:"room_id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
			http.Error(w, "Failed to decode desks", http.StatusInternalServerError)
			return
		}
		room := rows[0].RoomID
		if room != 0 {
			// ---- helper: parse YYYY-MM-DD safely
			parseDate := func(s string) (string, bool) {
				if s == "" {
					return "", false
				}
				t, err := time.Parse("2006-01-02", s)
				if err != nil {
					return "", false
				}
				tStr := t.Format("2006-01-02")
				return tStr, true
			}

			// Fechas de la reserva
			resStart, ok := parseDate(req.StartDate)
			if !ok {
				http.Error(w, "invalid start_date format (expected YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			condition := fmt.Sprintf("people_id = %d AND start_date = '%s'", req.PersonID, resStart)
			if req.ContractID != 0 {
				condition = fmt.Sprintf("id = %d AND people_id = %d", req.ContractID, req.PersonID)
			}
			query = map[string]interface{}{
				"table":     "contract",
				"columns":   "office_location",
				"value":     fmt.Sprintf("%d", room),
				"condition": condition,
			}
			jsonData, _ = json.Marshal(query)
			resp = putReq(jsonData, apiKey, client, w)
			if resp == nil {
				createLog(fmt.Sprintf("Error: putReq contracts returned nil for user %s", user), 1, apiKey, client, w)
				return
			}
			defer resp.Body.Close()
		}

	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})

}

// handles the request to make a desk reservable
func handleReservableDesk(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req ReservableDeskReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("Error decoding reservable desk JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.DeskID == 0 {
		createLog(fmt.Sprintf("Error: missing desk_id in reservable desk request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "desk_id is required", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "hottable",
		"columns":   "is_hot",
		"value":     "true",
		"condition": fmt.Sprintf("id = %d", req.DeskID),
	}

	jsonData, _ := json.Marshal(query)

	resp := putReq(jsonData, apiKey, client, w)

	if resp == nil {
		createLog(fmt.Sprintf("Failed to make desk reservable for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to make desk reservable", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})

}

// handles the request to freeze a desk
func handleFreezeDesk(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req ReservableDeskReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("Error decoding freeze desk JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.DeskID == 0 {
		createLog(fmt.Sprintf("Error: missing desk_id in freeze desk request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "desk_id is required", http.StatusBadRequest)
		return
	}

	query := map[string]interface{}{
		"table":     "hottable",
		"columns":   "is_hot",
		"value":     "false",
		"condition": fmt.Sprintf("id = %d", req.DeskID),
	}

	jsonData, _ := json.Marshal(query)

	resp := putReq(jsonData, apiKey, client, w)

	if resp == nil {
		createLog(fmt.Sprintf("Failed to freeze desk for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to freeze desk", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})

}

// handles the request to delete a release
func handleDeleteRelease(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {

	if r.Method != http.MethodPost {
		createLog(fmt.Sprintf("Method not allowed in delete release request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("Error decoding delete release JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ID == 0 {
		createLog(fmt.Sprintf("Error: missing id in delete release request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	// delete from reservation_user
	query := map[string]interface{}{
		"table":     "reservation",
		"condition": fmt.Sprintf("id = %d", req.ID),
	}
	jsonData, _ := json.Marshal(query)
	resp := deleteReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Failed to delete release for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to delete release", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})

}

// handles the request to delete an assignment
func handleDeleteAssignment(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {
	if r.Method != http.MethodPost {
		createLog(fmt.Sprintf("Method not allowed in delete assignment request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("Error decoding delete assignment JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ID == 0 {
		createLog(fmt.Sprintf("Error: missing id in delete assignment request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	// delete from reservation_user
	query := map[string]interface{}{
		"table":     "reservation_user",
		"condition": fmt.Sprintf("reservation_id = %d", req.ID),
	}

	jsonData, _ := json.Marshal(query)
	resp := deleteReq(jsonData, apiKey, client, w)

	if resp == nil {
		createLog(fmt.Sprintf("Failed to delete assignment_user for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to delete assignment_user", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	query = map[string]interface{}{
		"table":     "reservation",
		"condition": fmt.Sprintf("id = %d", req.ID),
	}
	jsonData, _ = json.Marshal(query)

	resp = deleteReq(jsonData, apiKey, client, w)

	if resp == nil {
		createLog(fmt.Sprintf("Failed to delete assignment for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to freeze desk", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})

}

func handleUpdateAssignment(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, user string) {
	if r.Method != http.MethodPost {
		createLog(fmt.Sprintf("Method not allowed in update assignment request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req struct {
		ID        int    `json:"id"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`

		GuestName    string `json:"guest_name"`
		GuestRole    string `json:"guest_role"`
		GuestHost    string `json:"guest_host"`
		GuestProgram string `json:"guest_program"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		createLog(fmt.Sprintf("Error decoding update assignment JSON for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ID == 0 {
		createLog(fmt.Sprintf("Error: missing id in update assignment request for user %s", user), 1, apiKey, client, w)
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	startDT := req.StartDate + " 08:00"
	endDT := req.EndDate + " 17:00"

	query := map[string]interface{}{
		"table":     "reservation",
		"columns":   "start_date, end_date",
		"value":     fmt.Sprintf("%s, %s", startDT, endDT),
		"condition": fmt.Sprintf("id = %d", req.ID),
	}

	jsonData, _ := json.Marshal(query)
	resp := putReq(jsonData, apiKey, client, w)

	if resp == nil {
		createLog(fmt.Sprintf("Failed to update assignment for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to freeze desk", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	if req.GuestName != "" || req.GuestRole != "" || req.GuestHost != "" || req.GuestProgram != "" {
		// update guest data
		gName := escapeComma(req.GuestName)
		gRole := escapeComma(req.GuestRole)
		gHost := escapeComma(req.GuestHost)
		gProgram := escapeComma(req.GuestProgram)

		queryRU := map[string]interface{}{
			"table":     "reservation_user",
			"columns":   "role, host, program, name",
			"value":     fmt.Sprintf("%s, %s, %s, %s", gRole, gHost, gProgram, gName),
			"condition": fmt.Sprintf("reservation_id = %d", req.ID),
		}
		jsonDataRU, _ := json.Marshal(queryRU)
		respRU := putReq(jsonDataRU, apiKey, client, w)
		if respRU == nil {
			createLog(fmt.Sprintf("Failed to update guest data for user %s", user), 1, apiKey, client, w)
			http.Error(w, "Failed to update guest data", http.StatusInternalServerError)
			return
		}
		defer respRU.Body.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})

}
