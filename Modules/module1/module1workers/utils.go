package module1workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// ------------------------------ json queries ----------------------------------------
// GET reservation info
func jsonReservations() map[string]interface{} {
	return map[string]interface{}{
		"table":   "roomBooking JOIN room ON room.id = roomBooking.room_id LEFT JOIN people ON roomBooking.user_id = people.id",
		"columns": "room.name, roomBooking.id, roomBooking.concept, roomBooking.start_date, roomBooking.end_date, roomBooking.email, people.name AS user_name, people.surname",
	}
}

// DELETE reservation
func jsonDeleteReservation(id int) map[string]interface{} {
	query := map[string]interface{}{
		"table":     "roomBooking",
		"condition": fmt.Sprintf("id = %d", id),
	}

	return query
}

// GET reservations
func jsonReservationsByID(ID int) map[string]interface{} {
	return map[string]interface{}{
		"table":     "roomBooking",
		"columns":   "id, room_id, start_date,end_date, email, outlook_event_id",
		"condition": fmt.Sprintf("id = %d", ID),
	}
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

// GET incoming reservations
func jsonFutureReservationsByUser(userID int) map[string]interface{} {
	now := time.Now().Format("2006-01-02 15:04:05")

	return map[string]interface{}{
		"table": "room r JOIN roomBooking b ON r.id = b.room_id LEFT JOIN people p ON b.email = p.crm_email",
		"columns": `
			r.name,
			b.id,
			b.room_id,
			b.start_date,
			b.end_date,
			b.email,
			b.concept
		`,
		"condition": fmt.Sprintf(`
			(
				b.user_id = %d
				OR p.id = %d
			)
			AND b.start_date >= '%s'
		`, userID, userID, now),
	}
}

// ------------------------------ module functions -----------------------------------
// checks available rooms and returns them
func handleAvailable(req ReqData, w http.ResponseWriter, client *http.Client) {
	queryRooms := map[string]interface{}{
		"table":     "room",
		"columns":   "id, name, uab_code, category",
		"condition": "category IN ('classroom', 'office', 'auditorium') AND reservable = 1",
	}
	jsonData, _ := json.Marshal(queryRooms)
	respRooms := getReq(jsonData, mC.Module1ApiKey, client, w)
	defer respRooms.Body.Close()
	var rooms []map[string]interface{}
	if err := json.NewDecoder(respRooms.Body).Decode(&rooms); err != nil {
		http.Error(w, "Error decoding rooms", http.StatusInternalServerError)
		return
	}
	req.StartTime = strings.Replace(req.StartTime, "T", " ", 1)
	req.EndTime = strings.Replace(req.EndTime, "T", " ", 1)

	//from all rooms, find their reservations and checks timeframe collisions
	var availableRooms []map[string]interface{}
	//also send occupied rooms to frontend to add information
	var occupiedRooms []map[string]interface{}
	for _, room := range rooms {
		roomID := int(room["id"].(float64))
		queryReservation := map[string]interface{}{
			"table":   "roomBooking",
			"columns": "id, concept, start_date, end_date, email",
			"condition": fmt.Sprintf(
				"room_id = %d AND start_date < '%s' AND end_date > '%s'",
				roomID,
				req.EndTime,
				req.StartTime,
			),
		}
		jsonData, _ = json.Marshal(queryReservation)
		resp := getReq(jsonData, mC.Module1ApiKey, client, w)
		defer resp.Body.Close()
		var reservations []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&reservations); err != nil {
			http.Error(w, "Error decoding reservations", http.StatusInternalServerError)
			return
		}
		//if no reservations found, room is available
		if len(reservations) == 0 {
			availableRooms = append(availableRooms, room)
		} else {
			// we send occupied rooms to frontend to add information
			occupiedRooms = append(occupiedRooms, map[string]interface{}{
				"room":         room,
				"reservations": reservations,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"rooms":         availableRooms,
		"occupiedRooms": occupiedRooms,
		"startTime":     req.StartTime,
		"endTime":       req.EndTime,
	}
	json.NewEncoder(w).Encode(response)
}

// checks available rooms for repeated reservations and returns them
func handleAvailableRepeated(req ReqData, w http.ResponseWriter, client *http.Client) {
	occurrences := generateRepeatedDates(req)

	if len(occurrences) == 0 {
		createLog("No valid repetition dates generated", 1, mC.Module1ApiKey, client, w)
		http.Error(w, "No valid repetition dates generated", http.StatusBadRequest)
		return
	}

	queryRooms := map[string]interface{}{
		"table":     "room",
		"columns":   "id, name, uab_code, category",
		"condition": "category IN ('classroom', 'office', 'auditorium') AND reservable = 1",
	}
	jsonData, _ := json.Marshal(queryRooms)

	respRooms := getReq(jsonData, mC.Module1ApiKey, client, w)
	defer respRooms.Body.Close()

	var rooms []map[string]interface{}
	if err := json.NewDecoder(respRooms.Body).Decode(&rooms); err != nil {
		createLog("Error decoding rooms", 1, mC.Module1ApiKey, client, w)
		http.Error(w, "Error decoding rooms", http.StatusInternalServerError)
		return
	}

	var availableRooms []map[string]interface{}
	var occupiedRooms []map[string]interface{}

	for _, room := range rooms {
		roomID := int(room["id"].(float64))
		isAvailable := true
		var conflicts []map[string]interface{}

		for _, occ := range occurrences {
			queryReservation := map[string]interface{}{
				"table":   "roomBooking",
				"columns": "id, concept, start_date, end_date, email",
				"condition": fmt.Sprintf(
					"room_id = %d AND start_date < '%s' AND end_date > '%s'",
					roomID,
					occ.End,
					occ.Start,
				),
			}

			jsonData, _ = json.Marshal(queryReservation)
			resp := getReq(jsonData, mC.Module1ApiKey, client, w)
			defer resp.Body.Close()

			var reservations []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&reservations); err != nil {
				http.Error(w, "Error decoding reservations", http.StatusInternalServerError)
				return
			}
			//conflict found, room not available
			if len(reservations) > 0 {
				isAvailable = false
				conflicts = append(conflicts, reservations...)
				break
			}
		}

		if isAvailable {
			availableRooms = append(availableRooms, room)
		} else {
			occupiedRooms = append(occupiedRooms, map[string]interface{}{
				"room":         room,
				"reservations": conflicts,
			})
		}
	}
	if len(availableRooms) == 0 {
		createLog("No available rooms found for requested time slot", 0, mC.Module1ApiKey, client, w)
	}
	response := map[string]interface{}{
		"rooms":         availableRooms,
		"occupiedRooms": occupiedRooms,
		"occurrences":   occurrences,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generates all occurrence dates based on repetition rules
func generateRepeatedDates(req ReqData) []Occurrence {
	var result []Occurrence

	layout := "2006-01-02T15:04"
	start, _ := time.Parse(layout, req.StartTime)
	end, _ := time.Parse(layout, req.EndTime)

	// End repeat (date only)
	endRepeatDate, _ := time.Parse("2006-01-02", req.EndRepeat)
	endRepeatDate = endRepeatDate.AddDate(0, 0, 1)

	switch req.Frequency {

	// --- WEEKLY WITH SELECTED DAYS ---
	case "weekly":
		weekdayMap := map[int]time.Weekday{
			1: time.Monday,
			2: time.Tuesday,
			3: time.Wednesday,
			4: time.Thursday,
			5: time.Friday,
		}

		selectedDays := map[time.Weekday]bool{}
		for _, d := range req.Weekdays {
			if weekday, exists := weekdayMap[d]; exists {
				selectedDays[weekday] = true
			}
		}

		currentStart := start
		currentEnd := end

		// Scan day by day until limit
		for !currentStart.After(endRepeatDate) {
			if selectedDays[currentStart.Weekday()] {
				result = append(result, Occurrence{
					Start: currentStart.Format("2006-01-02 15:04"),
					End:   currentEnd.Format("2006-01-02 15:04"),
				})
			}
			currentStart = currentStart.AddDate(0, 0, 1)
			currentEnd = currentEnd.AddDate(0, 0, 1)
		}

	// --- MONTHLY ---
	case "monthly":
		currentStart := start
		duration := end.Sub(start)

		if req.RepeatBy == "month_weekday" {
			// Determine the base pattern
			targetWeekday := start.Weekday()
			targetWeek := (start.Day()-1)/7 + 1 // week number (1–4)

			for !currentStart.After(endRepeatDate) {
				var occurrenceStart time.Time

				// First day of month with original time
				firstDayOfMonth := time.Date(currentStart.Year(), currentStart.Month(), 1, start.Hour(), start.Minute(), 0, 0, currentStart.Location())

				count := 0
				for i := 0; i < 31; i++ {
					candidate := firstDayOfMonth.AddDate(0, 0, i)

					if candidate.Month() != firstDayOfMonth.Month() {
						break
					}

					if candidate.Weekday() == targetWeekday {
						count++
						if count == targetWeek {
							occurrenceStart = candidate
							break
						}
					}
				}

				if !occurrenceStart.IsZero() && !occurrenceStart.After(endRepeatDate) {
					result = append(result, Occurrence{
						Start: occurrenceStart.Format("2006-01-02 15:04"),
						End:   occurrenceStart.Add(duration).Format("2006-01-02 15:04"),
					})
				}

				// Move to next month
				currentStart = currentStart.AddDate(0, 1, 0)
			}

		} else { // month_day
			for !currentStart.After(endRepeatDate) {
				result = append(result, Occurrence{
					Start: currentStart.Format("2006-01-02 15:04"),
					End:   currentStart.Add(duration).Format("2006-01-02 15:04"),
				})

				currentStart = currentStart.AddDate(0, 1, 0)
			}
		}

	// --- YEARLY ---
	case "yearly":

		duration := end.Sub(start) // maintain the original duration
		currentStart := start

		if req.RepeatBy == "year_weekday" {

			targetWeekday := start.Weekday()    // Monday, Tuesday...
			targetWeek := (start.Day()-1)/7 + 1 // week number (1–4)
			targetMonth := start.Month()        // repeat always on same month

			for !currentStart.After(endRepeatDate) {

				// First day of target month, same hour/minute
				firstDayOfMonth := time.Date(currentStart.Year(), targetMonth, 1,
					start.Hour(), start.Minute(), 0, 0, currentStart.Location())

				var occurrenceStart time.Time
				count := 0

				for i := 0; i < 31; i++ {
					candidate := firstDayOfMonth.AddDate(0, 0, i)
					if candidate.Month() != firstDayOfMonth.Month() {
						break
					}

					if candidate.Weekday() == targetWeekday {
						count++
						if count == targetWeek {
							occurrenceStart = candidate
							break
						}
					}
				}

				if !occurrenceStart.IsZero() && !occurrenceStart.After(endRepeatDate) {
					result = append(result, Occurrence{
						Start: occurrenceStart.Format("2006-01-02 15:04"),
						End:   occurrenceStart.Add(duration).Format("2006-01-02 15:04"),
					})
				}

				currentStart = currentStart.AddDate(1, 0, 0) // Next year
			}

		} else { // year_day (exact date repeat)
			for !currentStart.After(endRepeatDate) {
				result = append(result, Occurrence{
					Start: currentStart.Format("2006-01-02 15:04"),
					End:   currentStart.Add(duration).Format("2006-01-02 15:04"),
				})

				currentStart = currentStart.AddDate(1, 0, 0)
			}
		}
	}

	return result
}

// saves new reservations in database
func handlePost(req ReservaEntry, apiKey string, user string, userID int, w http.ResponseWriter, client *http.Client) {
	//to avoid insert problems, "," are replaced with "§"
	parsedConcept := escapeComma(req.Concept)
	query := map[string]interface{}{
		"table":   "roomBooking",
		"columns": "room_id, concept, start_date, end_date, email, user_id",
		"value":   fmt.Sprintf("%d, %s, %s, %s, %s, %d", req.RoomID, parsedConcept, req.StartDate, req.EndDate, req.Email, userID),
	}
	jsonData, _ := json.Marshal(query)
	respInsert := postReq(jsonData, apiKey, client, w)
	if respInsert == nil || respInsert.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error inserting reservation for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to insert reservation", http.StatusInternalServerError)
		return
	}
	defer respInsert.Body.Close()

	// --- Create Outlook event ---
	roomEmail := getRoomEmailByID(req.RoomID, apiKey, client, w)
	if roomEmail != "" {
		eventID, err := CreateOutlookEvent(roomEmail, req)
		if err == nil {
			saveOutlookEventID(req, eventID, apiKey, client, w)
		} else {
			fmt.Println("ERROR creating Outlook event:", err)
		}
	}

	// get room name
	queryTableName := map[string]interface{}{
		"table":     "room",
		"columns":   "name",
		"condition": fmt.Sprintf("id='%d'", req.RoomID),
	}
	jsonData, _ = json.Marshal(queryTableName)
	respTableNames := getReq(jsonData, apiKey, client, w)
	if respTableNames == nil {
		createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
		return
	}
	defer respTableNames.Body.Close()
	bodyBytes, _ := io.ReadAll(respTableNames.Body)
	var tableNameResponses []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &tableNameResponses); err != nil {
		createLog(fmt.Sprintf("Error decoding room name for user %s: %v", user, err), 1, apiKey, client, w)
	} else {
		if len(tableNameResponses) > 0 {
			req.Name = tableNameResponses[0].Name
		} else {
			println("Empty answer, no room name found.")
		}
	}
	// send confirmation email
	_ = sendConfirmationEmailFromTemplate("emailTemplate.html", req, req.Email)
	createLog(fmt.Sprintf("Reservation inserted and confirmation sent to %s by user %s", req.Email, user), 0, apiKey, client, w)
	w.Write([]byte(`{"status":"ok"}`))
}

// returns room email by its ID to create Outlook events
func getRoomEmailByID(roomID int, apiKey string, client *http.Client, w http.ResponseWriter) string {
	query := map[string]interface{}{
		"table":     "room",
		"columns":   "email",
		"condition": fmt.Sprintf("id = '%d'", roomID),
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return ""
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		return ""
	}

	if email, ok := rows[0]["email"].(string); ok {
		return email
	}
	return ""
}

// updates reservation with newly created outlook event ID
func saveOutlookEventID(r ReservaEntry, eventID string, apiKey string, client *http.Client, w http.ResponseWriter) {
	query := map[string]interface{}{
		"table":   "roomBooking",
		"columns": "outlook_event_id",
		"value":   eventID,
		"condition": fmt.Sprintf("room_id='%d' AND start_date='%s' AND end_date='%s'",
			r.RoomID, r.StartDate, r.EndDate,
		),
	}

	jsonData, _ := json.Marshal(query)
	_ = putReq(jsonData, apiKey, client, w)
}

// saves new reservations in database
func individualPost(req ReservaEntry, apiKey string, user string, userID int, w http.ResponseWriter, client *http.Client) {
	//to avoid insert problems, "," are replaced with "§"
	parsedConcept := escapeComma(req.Concept)
	query := map[string]interface{}{
		"table":   "roomBooking",
		"columns": "room_id, concept, start_date, end_date, email, user_id",
		"value":   fmt.Sprintf("%d, %s, %s, %s, %s, %d", req.RoomID, parsedConcept, req.StartDate, req.EndDate, req.Email, userID),
	}
	jsonData, _ := json.Marshal(query)
	respInsert := postReq(jsonData, apiKey, client, w)
	if respInsert == nil || respInsert.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error inserting reservation for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to insert reservation", http.StatusInternalServerError)
		return
	}
	defer respInsert.Body.Close()
	// --- Create Outlook event ---
	roomEmail := getRoomEmailByID(req.RoomID, apiKey, client, w)
	if roomEmail != "" {
		eventID, err := CreateOutlookEvent(roomEmail, req)
		if err == nil {
			saveOutlookEventID(req, eventID, apiKey, client, w)
		} else {
			fmt.Println("ERROR creating Outlook event:", err)
		}
	}
}

// saves new repeated reservations in database
func handlePostRepeated(req ReqRepeated, apiKey string, user string, userID int, w http.ResponseWriter, client *http.Client) {

	if req.RoomID == 0 || len(req.Occurrences) == 0 {
		http.Error(w, "Missing room_id or occurrences", http.StatusBadRequest)
		return
	}

	// check for collisions before inserting any reservation
	var collisions []Collision
	for i, occ := range req.Occurrences {
		hasOverlap, err := existsOverlappingReservation(req.RoomID, occ.Start, occ.End, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Collision check failed (repeated) user %s: %v", user, err), 1, apiKey, client, w)
			http.Error(w, "Failed to validate repeated reservation availability", http.StatusInternalServerError)
			return
		}
		if hasOverlap {
			collisions = append(collisions, Collision{
				Index:  i,
				Start:  occ.Start,
				End:    occ.End,
				Reason: "overlaps_existing_reservation",
			})
		}
	}

	// if any collision found, return conflict response
	if len(collisions) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "conflict",
			"message":    "Some occurrences are no longer available",
			"collisions": collisions,
		})
		return
	}

	// no collisions, proceed to insert all occurrences
	for _, occ := range req.Occurrences {
		reserva := ReservaEntry{
			RoomID:    req.RoomID,
			Concept:   req.Concept,
			StartDate: occ.Start,
			EndDate:   occ.End,
			Email:     req.Email,
		}

		// reuse individualPost function to insert each occurrence
		individualPost(reserva, apiKey, user, userID, w, client)
	}

	// get information to send email
	queryTableName := map[string]interface{}{
		"table":     "room",
		"columns":   "name",
		"condition": fmt.Sprintf("id='%d'", req.RoomID),
	}

	emailData := struct {
		RoomID           int
		Frequency        string
		Occurrences      []Occurrence
		TotalOccurrences int
		Name             string
		Concept          string
	}{
		RoomID:           req.RoomID,
		Frequency:        req.Frequency,
		Occurrences:      req.Occurrences,
		TotalOccurrences: len(req.Occurrences),
		Concept:          req.Concept,
	}

	jsonData, _ := json.Marshal(queryTableName)
	respTableNames := getReq(jsonData, apiKey, client, w)
	if respTableNames == nil {
		createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
		return
	}
	defer respTableNames.Body.Close()
	bodyBytes, _ := io.ReadAll(respTableNames.Body)
	var tableNameResponses []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &tableNameResponses); err != nil {
		createLog(fmt.Sprintf("Error decoding room name for user %s: %v", user, err), 1, apiKey, client, w)
	} else {
		if len(tableNameResponses) > 0 {
			emailData.Name = tableNameResponses[0].Name
		} else {
			createLog(fmt.Sprintf("Empty answer, no room name found for user %s", user), 1, apiKey, client, w)
		}
	}

	// send confirmation email
	err := sendConfirmationEmailFromTemplate("emailConfirmation.html", emailData, req.Email)
	if err != nil {
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	// log success
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","message":"Repeated reservation inserted and email sent"}`))
}

// ------------------------------------ email functions -----------------------------------
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

// sends email using an HTML template and data
func sendConfirmationEmailFromTemplate(url string, data interface{}, email string) error {
	// read the template file
	tmplContent, err := os.ReadFile(fmt.Sprintf("module1workers/assets/%s", url))
	if err != nil {
		return fmt.Errorf("error reading template: %w", err)
	}

	// parse the template
	tmpl, err := template.New("email").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	// execute the template with data
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	// send the email
	subject := "Reservation Confirmed"
	return sendConfirmationEmail(email, subject, body.String())
}

// sends cancellation email using an HTML template and reservation data
func sendCancellationEmailFromTemplate(ID int, StartDate string, EndDate string, email string, apiKey string, user string, w http.ResponseWriter, client *http.Client) error {
	// get reservation room name
	queryTableName := map[string]interface{}{
		"table":     "room",
		"columns":   "name",
		"condition": fmt.Sprintf("id='%d'", ID),
	}
	jsonData, _ := json.Marshal(queryTableName)
	respTableNames := getReq(jsonData, apiKey, client, w)
	if respTableNames == nil {
		createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
		return fmt.Errorf("failed to fetch reservations")
	}
	defer respTableNames.Body.Close()
	bodyBytes, _ := io.ReadAll(respTableNames.Body)
	var tableNameResponses []struct {
		Name string `json:"name"`
	}
	var ReservationName string
	if err := json.Unmarshal(bodyBytes, &tableNameResponses); err != nil {
		createLog(fmt.Sprintf("Error decoding table name for user %s: %v", user, err), 1, apiKey, client, w)
	} else {
		if len(tableNameResponses) > 0 {
			ReservationName = tableNameResponses[0].Name
		} else {
			createLog(fmt.Sprintf("Empty answer, no table name found for user %s", user), 1, apiKey, client, w)
		}
	}
	// read the template file
	tmplContent, err := os.ReadFile("module1workers/assets/cancellationTemplate.html")
	if err != nil {
		return fmt.Errorf("error reading template: %w", err)
	}

	tmpl, err := template.New("email").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	// format email information
	const layoutIn = "2006-01-02T15:04:05Z"
	const layoutOut = "02-01-2006 15:04"
	parsedStart, err := time.Parse(layoutIn, StartDate)
	if err != nil {
		fmt.Println("Error parsing StartDate:", err)
		parsedStart = time.Now()
	}
	parsedEnd, err := time.Parse(layoutIn, EndDate)
	if err != nil {
		fmt.Println("Error parsing EndDate:", err)
		parsedEnd = time.Now()
	}
	formattedStart := parsedStart.Format(layoutOut)
	formattedEnd := parsedEnd.Format(layoutOut)
	data := map[string]string{
		"Name":      ReservationName,
		"StartDate": formattedStart,
		"EndDate":   formattedEnd,
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}
	subject := "Room Reservation Cancelled"
	return sendConfirmationEmail(email, subject, body.String())
}

// retrieves monthly calendar events
func handleGetMonthlyCalendar(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	type Request struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	monthQuery := map[string]interface{}{
		"table": "roomBooking b JOIN room r ON b.room_id = r.id",
		"columns": `
			b.concept,
			b.start_date,
			b.end_date,
			b.email,
			r.name,
			r.uab_code,
			r.category
		`,
		"condition": fmt.Sprintf(
			"b.start_date <= '%s' AND b.end_date > '%s'",
			req.End,
			req.Start,
		),
	}

	jsonData, _ := json.Marshal(monthQuery)

	resp := getReq(jsonData, apiKey, client, w)
	defer resp.Body.Close()

	var rawData []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		http.Error(w, "Error decoding DB response", http.StatusInternalServerError)
		return
	}

	// structure for monthly events
	type MonthlyEvent struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Concept   string `json:"concept"`
		Email     string `json:"email"`
		RoomName  string `json:"name"`
		RoomCode  string `json:"uab_code"`
		Category  string `json:"category"`
	}

	events := []MonthlyEvent{}

	for _, row := range rawData {
		event := MonthlyEvent{
			Concept:   unescapeComma(row["concept"].(string)),
			StartDate: row["start_date"].(string),
			EndDate:   row["end_date"].(string),
			Email:     row["email"].(string),
			RoomName:  row["name"].(string),
			RoomCode:  row["uab_code"].(string),
			Category:  row["category"].(string),
		}
		events = append(events, event)

	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// checks if there is any overlapping reservation for given room and timeframe
func existsOverlappingReservation(roomID int, start, end string, apiKey string, client *http.Client, w http.ResponseWriter) (bool, error) {
	step := map[string]interface{}{
		"table":   "roomBooking r",
		"columns": "COUNT(*) AS cnt",
		"condition": fmt.Sprintf(
			"r.room_id = %d AND r.start_date < '%s' AND r.end_date > '%s'",
			roomID, end, start,
		),
	}

	jsonData, _ := json.Marshal(step)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return false, fmt.Errorf("getReq nil")
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	cntVal, ok := rows[0]["cnt"]
	if !ok || cntVal == nil {
		return false, nil
	}
	if f, ok := cntVal.(float64); ok {
		return int(f) > 0, nil
	}
	if s, ok := cntVal.(string); ok {
		n, _ := strconv.Atoi(s)
		return n > 0, nil
	}

	return false, nil
}

// checks if there is any overlapping reservation for given room and timeframe
func existsOverlappingReservationMinusReservation(reservationId int, roomID int, start, end string, apiKey string, client *http.Client, w http.ResponseWriter) (bool, error) {
	step := map[string]interface{}{
		"table":   "roomBooking r",
		"columns": "COUNT(*) AS cnt",
		"condition": fmt.Sprintf(
			"r.room_id = %d AND r.start_date < '%s' AND r.end_date > '%s' AND r.id != %d",
			roomID, end, start, reservationId,
		),
	}

	jsonData, _ := json.Marshal(step)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return false, fmt.Errorf("getReq nil")
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	cntVal, ok := rows[0]["cnt"]
	if !ok || cntVal == nil {
		return false, nil
	}
	if f, ok := cntVal.(float64); ok {
		return int(f) > 0, nil
	}
	if s, ok := cntVal.(string); ok {
		n, _ := strconv.Atoi(s)
		return n > 0, nil
	}

	return false, nil
}

func handleCheckAvailability(req ReqDataModify, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) {
	_, username, _ := getUserInfo(apiKey, client, w, r)
	roomID, err := strconv.Atoi(req.Room)
	if err != nil {
		createLog(fmt.Sprintf("Invalid room ID for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}
	hasOverlap, err := existsOverlappingReservationMinusReservation(req.ReservationId, roomID, req.StartTime, req.EndTime, apiKey, client, w)
	if err != nil {
		createLog(fmt.Sprintf("Collision check failed for user %s: %v", username, err), 1, apiKey, client, w)
		http.Error(w, "Failed to validate reservation availability", http.StatusInternalServerError)
		return
	}

	response := map[string]bool{"available": !hasOverlap}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleUpdateReservation(req ReqDataModify, apiKey string, user string, userID int, w http.ResponseWriter, client *http.Client) {
	parsedConcept := escapeComma(req.Concept)
	roomID, err := strconv.Atoi(req.Room)

	// get to check if it's linked to an Outlook event
	query := map[string]interface{}{
		"table":     "roomBooking",
		"columns":   "outlook_event_id, room_id",
		"condition": fmt.Sprintf("id = '%d'", req.ReservationId),
	}
	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to fetch reservation details", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		createLog(fmt.Sprintf("Error decoding reservation details for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "Error decoding reservation details", http.StatusInternalServerError)
		return
	}
	var outlookEventID string
	var old_roomID int
	if len(rows) > 0 {
		if val, ok := rows[0]["outlook_event_id"].(string); ok {
			outlookEventID = val
		}
		if val, ok := rows[0]["room_id"].(float64); ok {
			old_roomID = int(val)
		}

	}

	if outlookEventID != "" {
		roomEmail := getRoomEmailByID(old_roomID, apiKey, client, w)
		if roomEmail != "" {
			DeleteOutlookEvent(roomEmail, outlookEventID)

			query := map[string]interface{}{
				"table":     "roomBooking",
				"columns":   "outlook_event_id",
				"value":     "",
				"condition": fmt.Sprintf("id = '%d'", req.ReservationId),
			}
			jsonData, _ := json.Marshal(query)
			resp := putReq(jsonData, apiKey, client, w)
			if resp == nil {
				createLog(fmt.Sprintf("Error: putReq returned nil for user %s", user), 1, apiKey, client, w)
				http.Error(w, "Failed to update reservation details", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()
		}

	}

	if err != nil {
		createLog(fmt.Sprintf("Invalid room ID for user %s: %v", user, err), 1, apiKey, client, w)
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}
	query = map[string]interface{}{
		"table":   "roomBooking",
		"columns": "room_id, concept, start_date, end_date",
		"value":   fmt.Sprintf("%d, %s, %s, %s", roomID, parsedConcept, req.StartTime, req.EndTime),
		"condition": fmt.Sprintf(
			"id = '%d'",
			req.ReservationId,
		),
	}
	jsonData, _ = json.Marshal(query)
	respUpdate := putReq(jsonData, apiKey, client, w)
	if respUpdate == nil || respUpdate.StatusCode >= 400 {
		createLog(fmt.Sprintf("Error updating reservation for user %s", user), 1, apiKey, client, w)
		http.Error(w, "Failed to update reservation", http.StatusInternalServerError)
		return
	}
	defer respUpdate.Body.Close()

	var newReservation ReservaEntry

	// --- Create Outlook event ---
	roomEmail := getRoomEmailByID(roomID, apiKey, client, w)
	if roomEmail != "" {
		newReservation.RoomID = roomID
		newReservation.Concept = parsedConcept
		newReservation.StartDate = req.StartTime
		newReservation.EndDate = req.EndTime
		newReservation.Email = roomEmail
		eventID, err := CreateOutlookEvent(roomEmail, newReservation)
		if err == nil {
			saveOutlookEventID(newReservation, eventID, apiKey, client, w)
		} else {
			fmt.Println("ERROR creating Outlook event:", err)
		}
	}

}
