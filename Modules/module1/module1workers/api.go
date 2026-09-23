package module1workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func serveAPI(w http.ResponseWriter, r *http.Request) {

	//req api key
	apiKey := r.FormValue("key")

	if verifyAPIKey(apiKey) {
		//set client
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		if mC.Development {
			client = setInsecureRequest()
		}
		//consult user ID, username and user role
		userRow, user, userRole := getUserInfo(apiKey, client, w, r)
		switch r.Method {
		case http.MethodPost:
			{
				switch r.URL.Query().Get("action") {
				case "checkAvailability":
					{
						var reqData ReqDataModify
						if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
							createLog(fmt.Sprintf("Error decoding request JSON for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to decode request", http.StatusInternalServerError)
							return
						}
						handleCheckAvailability(reqData, apiKey, client, w, r)
						return
					}
				//consult available rooms
				case "getAvailable":
					{
						var reqData ReqData
						if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
							createLog(fmt.Sprintf("Error decoding request JSON for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to decode request", http.StatusInternalServerError)
							return
						}
						handleAvailable(reqData, w, client)
						return
					}
				//consult available rooms
				case "getAvailableRepeated":
					{
						var reqData ReqData
						if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
							createLog(fmt.Sprintf("Error decoding request JSON for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to decode request", http.StatusInternalServerError)
							return
						}
						handleAvailableRepeated(reqData, w, client)
						return
					}
				//save new reservation in database
				case "postReservation":
					{
						var reserva ReservaEntry
						if err := json.NewDecoder(r.Body).Decode(&reserva); err != nil {
							http.Error(w, "Error parsing reservation JSON", http.StatusBadRequest)
							createLog(fmt.Sprintf("Error parsing JSON by user %s: %v", user, err), 1, apiKey, client, w)
							return
						}
						//check for overlapping reservations before saving
						hasOverlap, err := existsOverlappingReservation(
							reserva.RoomID,
							reserva.StartDate,
							reserva.EndDate,
							apiKey, client, w,
						)
						if err != nil {
							createLog(fmt.Sprintf("Collision check failed for user %s: %v", user, err), 1, apiKey, client, w)
							http.Error(w, "Failed to validate reservation availability", http.StatusInternalServerError)
							return
						}
						if hasOverlap {
							http.Error(w, "Room already reserved for that time slot", http.StatusConflict)
							return
						}
						handlePost(reserva, apiKey, user, userRow, w, client)
						return
					}
				case "postReservationRepeated":
					{
						var req ReqRepeated

						if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
							http.Error(w, "Error parsing JSON", http.StatusBadRequest)
							return
						}

						handlePostRepeated(req, apiKey, user, userRow, w, client)
						return
					}
					//allows monthly-calendar visualization
				case "getReservationsCalendar":
					{
						handleGetMonthlyCalendar(apiKey, client, w, r)
					}
				case "updateReservation":
					{
						var reqData ReqDataModify
						if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
							http.Error(w, "Error parsing reservation JSON", http.StatusBadRequest)
							createLog(fmt.Sprintf("Error parsing JSON by user %s: %v", user, err), 1, apiKey, client, w)
							return
						}

						handleUpdateReservation(reqData, apiKey, user, userRow, w, client)
						return
					}
				}
			}
		case http.MethodGet:
			{
				switch r.URL.Query().Get("action") {
				//admin/manager/support function, shows all reservations made by all users
				case "fetchAllReservations":
					{
						queryAllReservations := jsonReservations()
						jsonData, _ := json.Marshal(queryAllReservations)
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
							if c, ok := reservas[i]["concept"].(string); ok {
								reservas[i]["concept"] = unescapeComma(c)
							}
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(reservas)
						return
					}
				//returns user role to frontend
				case "checkRole":
					{
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(map[string]string{
							"role": userRole,
						})
						return
					}

				//returns user email handle to frontend
				case "getUserEmails":
					{
						userID, _, _ := getUserInfo(apiKey, client, w, r)
						if userID == 0 {
							http.Error(w, "Invalid user session", http.StatusUnauthorized)
							return
						}

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

						//sends both institutional and personal if saved
						var result []struct {
							CrmEmail  string `json:"crm_email"`
							UserEmail string `json:"user_email"`
						}

						if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
							createLog(fmt.Sprintf("Error decoding user emails: %v", err), 1, apiKey, client, w)
							http.Error(w, "Failed to parse response", http.StatusInternalServerError)
							return
						}

						//incase no emails are found, return empty strings
						if len(result) == 0 {
							json.NewEncoder(w).Encode(map[string]interface{}{
								"status":     "ok",
								"crm_email":  "",
								"user_email": "",
							})
							return
						}

						json.NewEncoder(w).Encode(map[string]interface{}{
							"status":     "ok",
							"crm_email":  result[0].CrmEmail,
							"user_email": result[0].UserEmail,
						})

					}
				//general user function, shows their incoming reservations
				case "fetchMyReservations":
					{
						queryFuture := jsonFutureReservationsByUser(userRow)
						jsonData, _ := json.Marshal(queryFuture)
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
							if c, ok := reservas[i]["concept"].(string); ok {
								reservas[i]["concept"] = unescapeComma(c)
							}

							rawRoomID, ok := reservas[i]["room_id"]
							if !ok || rawRoomID == nil {
								continue
							}

							roomID, ok := rawRoomID.(string)
							if !ok {
								roomID = fmt.Sprint(rawRoomID)
							}

							queryRoom := map[string]interface{}{
								"table":     "room",
								"columns":   "uab_code",
								"condition": fmt.Sprintf("id = '%s'", roomID),
							}

							roomJSON, err := json.Marshal(queryRoom)
							if err != nil {
								createLog(fmt.Sprintf("Error marshaling room query for user %s, room_id %s: %v", user, roomID, err), 1, apiKey, client, w)
								http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
								return
							}

							roomResp := getReq(roomJSON, apiKey, client, w)
							if roomResp == nil {
								createLog(fmt.Sprintf("Error: getReq returned nil when fetching room for user %s, room_id %s", user, roomID), 1, apiKey, client, w)
								http.Error(w, "Failed to fetch reservations", http.StatusInternalServerError)
								return
							}

							func() {
								defer roomResp.Body.Close()

								var aula []map[string]interface{}
								if err := json.NewDecoder(roomResp.Body).Decode(&aula); err != nil {
									createLog(fmt.Sprintf("Error decoding room JSON for user %s, room_id %s: %v", user, roomID, err), 1, apiKey, client, w)
									http.Error(w, "Failed to decode room information", http.StatusInternalServerError)
									return
								}

								if len(aula) > 0 {
									reservas[i]["room_id"] = aula[0]["uab_code"]
								}
							}()
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(reservas)
						return
					}
				}
			}
		// delete an existing reservation
		case http.MethodDelete:
			{
				var bodyBytes []byte
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				var del struct {
					ID int `json:"id"`
				}
				if err := json.Unmarshal(bodyBytes, &del); err != nil {
					http.Error(w, "Error parsing delete JSON", http.StatusBadRequest)
					createLog(fmt.Sprintf("Error parsing delete JSON for user %s: %v", user, err), 1, apiKey, client, w)
					return
				}
				//get reservation before deleting to save email
				queryAllReservations := jsonReservationsByID(del.ID)
				jsonData, _ := json.Marshal(queryAllReservations)
				resp := getReq(jsonData, apiKey, client, w)
				if resp == nil {
					createLog(fmt.Sprintf("Error: getReq returned nil for user %s", user), 1, apiKey, client, w)
					http.Error(w, "Failed to fetch reservation", http.StatusInternalServerError)
					return
				}
				defer resp.Body.Close()
				bodyBytes, _ = io.ReadAll(resp.Body)
				var reservas []ReservaEntry
				if err := json.Unmarshal(bodyBytes, &reservas); err != nil {
					fmt.Println("Error decoding JSON:", err)
					http.Error(w, "Failed to decode reservation", http.StatusInternalServerError)
					return
				}
				if len(reservas) == 0 {
					fmt.Println("No reservation found with ID", del.ID)
					http.Error(w, "Reservation not found", http.StatusNotFound)
					return
				}
				//deleting first reservation (should be the only one)
				reserva := reservas[0]

				if reserva.OutlookEventID != "" {
					roomEmail := getRoomEmailByID(reserva.RoomID, apiKey, client, w)
					if roomEmail != "" {
						DeleteOutlookEvent(roomEmail, reserva.OutlookEventID)
					}
				}

				deleteRes := jsonDeleteReservation(del.ID)
				jsonData, _ = json.Marshal(deleteRes)
				resp = deleteReq(jsonData, apiKey, client, w)
				if resp == nil || resp.StatusCode >= 400 {
					createLog(fmt.Sprintf("Error deleting reservation %d by user %s", del.ID, user), 1, apiKey, client, w)
					http.Error(w, "Failed to delete reservation", http.StatusInternalServerError)
					return
				}
				_ = sendCancellationEmailFromTemplate(reserva.RoomID, reserva.StartDate, reserva.EndDate, reserva.Email, apiKey, user, w, client)
				createLog(fmt.Sprintf("User %s deleted reservation %d", user, del.ID), 0, apiKey, client, w)
				w.WriteHeader(http.StatusOK)
			}

		}
	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		createLog("Invalid API key provided", 2, apiKey, nil, w)
		return
	}

}
