package module2workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func serveAPI(w http.ResponseWriter, r *http.Request) {

	//req api key
	apiKey := r.FormValue("key") // sanitize in here

	// verify api key is correct (compare with config value)
	if verifyAPIKey(apiKey) {

		//set client
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		if mC.Development {
			client = setInsecureRequest()
		}

		// get user's cookie
		var user string
		if c, err := r.Cookie("CRMINTRATOOLS"); err == nil {
			if claims, ok := parseAuthCookie(c.Value, nil); ok {
				user = getUsernameByHash(claims.Sub, apiKey, client, w)
			}
		}

		// ---------------------------- get user ID and role of user accessing --------------------------------------
		// get user's ID
		query := jsonUsernameID(user)
		jsonData, _ := json.Marshal(query)
		respID := getReq(jsonData, apiKey, client, w)
		if respID == nil {
			createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 130, apiKey, client, w)
			http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
			return
		}
		defer respID.Body.Close()
		bodyB, _ := io.ReadAll(respID.Body)
		var result []map[string]interface{}
		if err := json.Unmarshal(bodyB, &result); err != nil {
			http.Error(w, "Error parsing modification JSON", http.StatusBadRequest)
			createLog(fmt.Sprintf("Error parsing modification JSON for user %s: %v", user, err), 1, apiKey, client, w)
			return
		}
		if len(result) == 0 {
			http.Error(w, "User not found", http.StatusNotFound)
			createLog(fmt.Sprintf("User %s not found for cookie", user), 130, apiKey, client, w)
		}
		idRaw := result[0]["people_id"]
		var userRow int
		roleRow := result[0]["role"].(string)
		switch id := idRaw.(type) {
		case float64:
			userRow = int(id)
		default:
			http.Error(w, "Invalid user ID type", http.StatusInternalServerError)
			createLog(fmt.Sprintf("Invalid user ID type for user %s", user), 130, apiKey, client, w)
			return
		}
		// view petition
		var bodyBytes []byte
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// ------------------------------------------ MANAGE DESKS --------------------------------------

		if r.URL.Query().Get("action") == "desks" {
			// send all desks to frontend
			handleSendDesksToFE(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "people" {
			// send all people to frontend
			handleSendWorkersToFE(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "personContracts" {
			// send contracts for a selected person to frontend
			handleSendPersonContractsToFE(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "releaseDesk" {
			// creates a period of time where an assigned desk is released
			handleReleaseDesk(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "assignDesk" {
			// assigns a desk to a user
			handleAssignDesk(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "reservableDesk" {
			// makes a desk reservable at the index page
			handleReservableDesk(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "freezeDesk" {
			// freezes a desk (makes it unavailable for reservation)
			handleFreezeDesk(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "deleteRelease" {
			// deletes a release period of a desk
			handleDeleteRelease(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "deleteAssignment" {
			// deletes an assignment of a desk
			handleDeleteAssignment(apiKey, client, w, r, user)
			return
		}
		if r.URL.Query().Get("action") == "updateAssignment" {
			// updates an assignment or release of a desk
			handleUpdateAssignment(apiKey, client, w, r, user)
			return
		}

		// ------------------------------------------ MODIFY RESERVATION --------------------------------------------------------------------
		//
		// METHOD GET = get all reservations made by user

		switch r.Method {
		case http.MethodGet:
			{
				switch r.URL.Query().Get("action") {
				case "fetchMyReservations":
					{
						queryFuture := jsonFutureReservationsByUser(userRow)
						jsonData, _ = json.Marshal(queryFuture)
						resp := getReq(jsonData, apiKey, client, w)
						if resp == nil {
							createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
							http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
							return
						}
						defer resp.Body.Close()
						var reservas []map[string]interface{}
						if err := json.NewDecoder(resp.Body).Decode(&reservas); err != nil {
							createLog(fmt.Sprintf("Error decoding reservations JSON for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to decode reservations", http.StatusInternalServerError)
							return
						}
						for i := range reservas {
							rawConcept := reservas[i]["concept"]
							if rawConcept == nil {
								continue
							}
							conceptStr := rawConcept.(string)
							reservas[i]["concept"] = unescapeComma(conceptStr)
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(reservas)
						return
					}
				case "getUserEmails":
					{
						userID, _, _ := getUserInfo(apiKey, client, w, r)
						if userID == 0 {
							http.Error(w, "Invalid user session", http.StatusUnauthorized)
							return
						}

						// Preparamos la consulta en formato JSON para getReq
						query := map[string]interface{}{
							"table":     "people",
							"columns":   "user_email, crm_email",
							"condition": fmt.Sprintf("id = %d", userID),
						}

						getJSON, _ := json.Marshal(query)
						resp := getReq(getJSON, apiKey, client, w)
						if resp == nil {
							createLog("Failed to fetch user emails: getReq returned nil", 1, apiKey, client, w)
							http.Error(w, "Internal DB request failed", http.StatusInternalServerError)
							return
						}
						defer resp.Body.Close()

						var result []struct {
							CrmEmail  string `json:"crm_email"`
							UserEmail string `json:"user_email"`
						}

						if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
							createLog(fmt.Sprintf("Error decoding user emails: %v", err), 1, apiKey, client, w)
							http.Error(w, "Failed to parse response", http.StatusInternalServerError)
							return
						}

						// Si no hay datos
						if len(result) == 0 {
							json.NewEncoder(w).Encode(map[string]interface{}{
								"status":     "ok",
								"crm_email":  "",
								"user_email": "",
							})
							return
						}

						// Devolvemos los valores al frontend
						json.NewEncoder(w).Encode(map[string]interface{}{
							"status":     "ok",
							"crm_email":  result[0].CrmEmail,
							"user_email": result[0].UserEmail,
						})
						return

					}
				// CHECK RESERVATION HISTORY
				case "fetchAllMyReservations":
					{
						queryAllReservations := jsonReservationsByUser(userRow)
						jsonData, _ = json.Marshal(queryAllReservations)
						resp := getReq(jsonData, apiKey, client, w)
						if resp == nil {
							createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
							http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
							return
						}
						defer resp.Body.Close()
						var reservas []map[string]interface{}
						if err := json.NewDecoder(resp.Body).Decode(&reservas); err != nil {
							createLog(fmt.Sprintf("Error decoding reservations JSON for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to decode reservations", http.StatusInternalServerError)
							return
						}
						for i := range reservas {
							rawConcept := reservas[i]["concept"]
							if rawConcept == nil {
								continue
							}
							conceptStr := rawConcept.(string)
							reservas[i]["concept"] = unescapeComma(conceptStr)
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(reservas)
						return
					}
				case "fetchAllReservations":
					{
						queryAllReservations := jsonReservations()
						jsonData, _ = json.Marshal(queryAllReservations)
						resp := getReq(jsonData, apiKey, client, w)
						if resp == nil {
							createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
							http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
							return
						}
						defer resp.Body.Close()
						var reservas []map[string]interface{}
						if err := json.NewDecoder(resp.Body).Decode(&reservas); err != nil {
							createLog(fmt.Sprintf("Error decoding reservations JSON for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to decode reservations", http.StatusInternalServerError)
							return
						}
						for i := range reservas {
							rawConcept := reservas[i]["concept"]
							if rawConcept == nil {
								continue
							}
							conceptStr := rawConcept.(string)
							reservas[i]["concept"] = unescapeComma(conceptStr)

							rawName := reservas[i]["guest_name"]
							if rawName == nil {
								continue
							}
							nameStr := rawName.(string)
							reservas[i]["name"] = unescapeComma(nameStr)

							rawHost := reservas[i]["host"]
							if rawHost == nil {
								continue
							}
							hostStr := rawHost.(string)
							reservas[i]["host"] = unescapeComma(hostStr)

							rawProgram := reservas[i]["program"]
							if rawProgram == nil {
								continue
							}
							programStr := rawProgram.(string)
							reservas[i]["program"] = unescapeComma(programStr)

						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(reservas)
						return
					}
				// GET ROLE TO CHECK ALL RESERVATIONS
				case "checkRole":
					{
						userRole := roleRow
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(map[string]string{
							"role": userRole,
							"id":   fmt.Sprintf("%d", userRow),
						})
						return
					}
				}

			}
		// METHOD PUT = edit an existing reservation
		case http.MethodPut:
			{
				var mod ReservaModification
				if err := json.Unmarshal(bodyBytes, &mod); err != nil {
					http.Error(w, "Error parsing modification JSON", http.StatusBadRequest)
					createLog(fmt.Sprintf("Error parsing modification JSON for user %s: %v", user, err), 1, apiKey, client, w)
					return
				}
				mod.Concept = escapeComma(mod.Concept)
				queryMod := jsonUpdateReservation(mod.ReservationID, mod.Concept, mod.StartDate, mod.EndDate)
				jsonData, _ := json.Marshal(queryMod)
				resp := putReq(jsonData, apiKey, client, w)
				if resp == nil || resp.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error updating reservation for reservation %d by user %s", mod.ReservationID, user), 1, apiKey, client, w)
					http.Error(w, "Failed to update reservation", http.StatusInternalServerError)
					return
				}
			}
		// METHOD DELETE = delete an existing reservation (in both `reservation` and `reservation_user`)
		case http.MethodDelete:
			{
				var del struct {
					ReservationID int `json:"reservation_id"`
				}
				if err := json.Unmarshal(bodyBytes, &del); err != nil {
					http.Error(w, "Error parsing delete JSON", http.StatusBadRequest)
					createLog(fmt.Sprintf("Error parsing delete JSON for user %s: %v", user, err), 1, apiKey, client, w)
					return
				}
				deleteResUser := jsonDeleteReservationUser(del.ReservationID)
				jsonData, _ := json.Marshal(deleteResUser)
				resp := deleteReq(jsonData, apiKey, client, w)
				if resp == nil || resp.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error deleting reservation_user for reservation %d by user %s", del.ReservationID, user), 1, apiKey, client, w)
					http.Error(w, "Failed to delete reservation_user", http.StatusInternalServerError)
					return
				}
				deleteRes := jsonDeleteReservation(del.ReservationID)
				jsonData, _ = json.Marshal(deleteRes)
				resp = deleteReq(jsonData, apiKey, client, w)
				if resp == nil || resp.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error deleting reservation %d by user %s", del.ReservationID, user), 1, apiKey, client, w)
					http.Error(w, "Failed to delete reservation", http.StatusInternalServerError)
					return
				}
				createLog(fmt.Sprintf("User %s deleted reservation %d", user, del.ReservationID), 0, apiKey, client, w)
			}
		}
		// ------------------------------------------- POST RESERVATION ---------------------------------------------------------------------
		// case bodyBytes contains table_id --> reservation
		if strings.Contains(string(bodyBytes), "table_id") {

			// var containing reservation data
			var reserva ReservaEntry
			if err := json.Unmarshal(bodyBytes, &reserva); err != nil {
				http.Error(w, "Error parsing reservation JSON", http.StatusBadRequest)
				createLog(fmt.Sprintf("Error parsing JSON by user %s: %v", user, err), 1, apiKey, client, w)
				return
			}

			reserva.Concept = escapeComma(reserva.Concept)

			// post reservation
			query = buildInsertReservation(reserva.TableID, reserva.Concept, reserva.StartDate, reserva.EndDate)
			jsonData, _ := json.Marshal(query)
			respInsert := postReq(jsonData, apiKey, client, w)
			if respInsert == nil || respInsert.StatusCode >= 400 {
				createLog(fmt.Sprintf("Error inserting reservation for user %s", user), 1, apiKey, client, w)
				http.Error(w, "Failed to insert reservation", http.StatusInternalServerError)
				return
			}
			defer respInsert.Body.Close()

			// get reservation ID
			query = jsonReservationID(reserva.TableID, reserva.StartDate, reserva.EndDate)
			jsonData, _ = json.Marshal(query)
			respReservationID := getReq(jsonData, apiKey, client, w)
			if respReservationID == nil {
				createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
				http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
				return
			}
			defer respReservationID.Body.Close()
			var idRows []map[string]interface{}
			if err := json.NewDecoder(respReservationID.Body).Decode(&idRows); err != nil {
				http.Error(w, "Error decoding reservation ID", http.StatusInternalServerError)
				createLog(fmt.Sprintf("Error decoding reservation ID for user %s: %v", user, err), 1, apiKey, client, w)
				return
			}
			if len(idRows) == 0 {
				http.Error(w, "Reservation inserted but ID not found", http.StatusInternalServerError)
				createLog(fmt.Sprintf("Reservation inserted by %s but ID not found", user), 1, apiKey, client, w)
				return
			}

			// post reservation_user
			reservationID := int(idRows[0]["id"].(float64))

			queryResUser := buildInsertUserReservation(userRow, reservationID)
			jsonData, _ = json.Marshal(queryResUser)
			resp := postReq(jsonData, apiKey, client, w)
			if resp == nil || resp.StatusCode >= 400 {
				createLog(fmt.Sprintf("Error inserting reservation_user for user %s", user), 1, apiKey, client, w)
				http.Error(w, "Failed to insert reservation_user", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			// get table name
			queryTableName := jsonTableName(reserva.TableID)
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
				println("Error decoding JSON in slice:", err.Error())
				createLog(fmt.Sprintf("Error decoding table name for user %s: %v", user, err), 1, apiKey, client, w)
			} else {
				if len(tableNameResponses) > 0 {
					reserva.Name = tableNameResponses[0].Name
				} else {
					println("Empty answer, no table name found.")
				}
			}
			// send confirmation email
			_ = sendConfirmationEmailFromTemplate(reserva, reserva.Email)

			//w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}

		// ------------------------------------------- GET AVAILABLE TABLES ---------------------------------------------------------------------
		var reqData ReqData
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			createLog(fmt.Sprintf("Error decoding request JSON for user %s: %v", user, err), 1, apiKey, client, w)
			http.Error(w, "Failed to decode request", http.StatusInternalServerError)
			return
		}

		// different json depending on consulting mode
		var queryTables map[string]interface{}
		// click floor image
		if reqData.Floor == 4 {
			queryTables = jsonAllHotTables()
		} else { // fill form
			queryTables = jsonHotTablesByFloor(reqData.Floor)
		}

		// get query to consult each floor's tables
		jsonData, _ = json.Marshal(queryTables)
		resp := getReq(jsonData, apiKey, client, w)
		if resp == nil {
			createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
			http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// fill var with available tables
		var mesas []HotTable
		if err := json.NewDecoder(resp.Body).Decode(&mesas); err != nil {
			createLog(fmt.Sprintf("Error decoding hottables JSON for user %s: %v", user, err), 1, apiKey, client, w)
			http.Error(w, "Failed to decode hottables", http.StatusInternalServerError)
			return
		}

		for _, m := range mesas {
			fmt.Printf("  Mesa ID=%d Capacity=%d\n", m.ID, m.Capacity)
		}

		// get query to consult reservations
		// queryReservation -> consulta las mesas que tienen alguna reserva en ese rango de tiempo
		// queryNotReservation -> consulta las mesas que no tienen ninguna reserva en ese rango de tiempo

		queryReservation, _ := jsonAvailableTable(mesas, reqData.StartTime, reqData.EndTime)

		// consulting reservations
		jsonData, _ = json.Marshal(queryReservation)
		respReservations := getReq(jsonData, apiKey, client, w)
		if respReservations == nil {
			createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
			http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
			return
		}
		defer respReservations.Body.Close()

		// string containing reservations at the same timeframe
		bodyReservations, _ := io.ReadAll(respReservations.Body)
		fmt.Printf("DEBUG Raw reservation response: %s\n", string(bodyReservations))

		var reservas []ConsultaColisiones
		if err := json.Unmarshal(bodyReservations, &reservas); err != nil {
			http.Error(w, "Error parsing reservation JSON", http.StatusBadRequest)
			createLog(fmt.Sprintf("Error parsing reservation JSON for user %s: %v", user, err), 1, apiKey, client, w)
			return
		}

		resByMesa := make(map[int][]ConsultaColisiones)
		for _, r := range reservas {
			resByMesa[r.TableID] = append(resByMesa[r.TableID], r)
		}

		var effectiveReservas []ConsultaColisiones
		for _, list := range resByMesa {
			assignedCount := 0
			releasedCount := 0
			otherCount := 0

			for _, r := range list {
				switch strings.ToLower(r.Type) {
				case "assigned":
					assignedCount++
				case "released":
					releasedCount++
				default:
					otherCount++
				}
			}
			// If there is only one assigned reservation and no other type, it means the desk is assigned and not released
			if assignedCount == 1 && releasedCount >= 1 && otherCount == 0 {
				continue
			}

			// In any other case, all reservations NOT released count as active
			for _, r := range list {
				if strings.EqualFold(r.Type, "released") {
					continue
				}
				effectiveReservas = append(effectiveReservas, r)
			}
		}

		if len(effectiveReservas) == 0 {
			response := Reserva{
				StartTime: reqData.StartTime,
				EndTime:   reqData.EndTime,
				Mesas:     mesas,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		if roleRow != "admin" && roleRow != "manager" && roleRow != "support" {
			// checking if user has another active reservation at the timeframe
			if checkActiveReservations(userRow, effectiveReservas, apiKey, client, w) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "You already have an active reservation.",
				})
				return
			}
		}

		// checking users per reservation
		tableCapacity := countReservationsByMesa(effectiveReservas, apiKey, client, w)

		// verifying each table's capacity
		var mesasConEspacio []HotTable
		mesasSinEspacio := make(map[int]bool)
		for _, mesa := range mesas {
			ocupados := tableCapacity[mesa.ID]
			if mesa.Capacity >= ocupados+reqData.Capacity {
				mesasConEspacio = append(mesasConEspacio, mesa)
			} else {
				mesasSinEspacio[mesa.ID] = true
			}
		}
		response := Reserva{
			StartTime: reqData.StartTime,
			EndTime:   reqData.EndTime,
			Mesas:     mesasConEspacio,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return

	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		createLog("Invalid API key provided", 2, apiKey, nil, w)
		return
	}

}
