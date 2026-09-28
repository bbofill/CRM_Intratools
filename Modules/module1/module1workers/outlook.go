package module1workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// only works when accessing directly to the endpoint /api/outlook/getAllEvents
//
//export calendar events for all rooms as SQL insert statements
func GetAllCalendarEvents(apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request) ([][]byte, error) {

	emails := reqGetCalendarEmail(apiKey, client, w)
	if len(emails) == 0 {
		return nil, fmt.Errorf("no calendar emails found in room table")
	}

	start := time.Now().UTC()
	end := start.AddDate(2, 0, 0) // EWS max allowed

	var allResults [][]byte

	for _, email := range emails {
		soap := buildFindItemsSOAP(email, start, end)
		resp, err := EWSRequest(soap)
		if err != nil {
			allResults = append(allResults, []byte(
				fmt.Sprintf("<error email='%s'>%s</error>", email, err),
			))
			continue
		}
		allResults = append(allResults, resp)
	}

	return allResults, nil
}

var tokenCache = struct {
	sync.Mutex
	token *oauth2.Token
}{}

// gets and caches EWS OAuth2 token
func getEWSToken() (*oauth2.Token, error) {
	tokenCache.Lock()
	defer tokenCache.Unlock()

	conf := &clientcredentials.Config{
		ClientID:     mC.OutlookClientID,
		ClientSecret: mC.OutlookClientSecret,
		TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", mC.OutlookTenantID),
		Scopes:       []string{"https://outlook.office365.com/.default"},
	}

	if tokenCache.token == nil || tokenCache.token.Expiry.Before(time.Now().Add(-1*time.Minute)) {
		tok, err := conf.Token(oauth2.NoContext)
		if err != nil {
			return nil, err
		}
		tokenCache.token = tok
	}

	return tokenCache.token, nil
}

// makes EWS request with given SOAP XML
func EWSRequest(soapXML string) ([]byte, error) {
	token, err := getEWSToken()
	if err != nil {
		return nil, err
	}

	client := oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token))

	req, err := http.NewRequest("POST", mC.EWSURL, bytes.NewBuffer([]byte(soapXML)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// returns email addresses of all rooms with calendar
func reqGetCalendarEmail(apiKey string, client *http.Client, w http.ResponseWriter) []string {
	query := map[string]interface{}{
		"table":     "room",
		"columns":   "email",
		"condition": "email IS NOT NULL AND email <> ''",
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, nil)
	defer resp.Body.Close()

	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)

	var emails []string
	for _, row := range rows {
		if email, ok := row["email"].(string); ok {
			emails = append(emails, email)
		}
	}

	return emails
}

// builds SOAP request to get all calendar items between start and end for given email
func buildFindItemsSOAP(email string, start, end time.Time) string {

	return fmt.Sprintf(`
<soap:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" 
	xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
	xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"
	xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
	<soap:Header>
		<t:RequestServerVersion Version="Exchange2016" />
		<t:ExchangeImpersonation>
			<t:ConnectingSID>
				<t:PrimarySmtpAddress>%s</t:PrimarySmtpAddress>
			</t:ConnectingSID>
		</t:ExchangeImpersonation>
	</soap:Header>

	<soap:Body>
		<m:FindItem Traversal="Shallow">
			<m:ItemShape>
				<t:BaseShape>Default</t:BaseShape>
			</m:ItemShape>

			<m:CalendarView 
				StartDate="%s"
				EndDate="%s"/>

			<m:ParentFolderIds>
				<t:DistinguishedFolderId Id="calendar"/>
			</m:ParentFolderIds>
		</m:FindItem>
	</soap:Body>
</soap:Envelope>`,
		email,
		start.Format("2006-01-02T15:04:05Z"),
		end.Format("2006-01-02T15:04:05Z"),
	)
}

// gets room ID by email address to build SQL insert statements
func GetRoomIDByEmail(email string, apiKey string, client *http.Client, w http.ResponseWriter) (int, error) {
	query := map[string]interface{}{
		"table":     "room",
		"columns":   "id",
		"condition": fmt.Sprintf("email = '%s'", email),
	}

	jsonData, _ := json.Marshal(query)
	resp := getReq(jsonData, apiKey, client, w)
	if resp == nil {
		return 0, fmt.Errorf("db error")
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)

	if len(rows) == 0 {
		return 0, fmt.Errorf("room not found")
	}

	return int(rows[0]["id"].(float64)), nil
}

// parses EWS response and extracts calendar events
func ParseEventsFromEWS(body []byte) []CalendarEvent {
	var events []CalendarEvent

	str := string(body)

	items := strings.Split(str, "<t:CalendarItem>")
	for _, block := range items[1:] {

		subject := extract(block, "<t:Subject>", "</t:Subject>")
		start := extract(block, "<t:Start>", "</t:Start>")
		end := extract(block, "<t:End>", "</t:End>")

		events = append(events, CalendarEvent{
			Subject: subject,
			Start:   start,
			End:     end,
		})
	}

	return events
}

func extract(src, begin, end string) string {
	b := strings.Index(src, begin)
	if b == -1 {
		return ""
	}
	b += len(begin)

	e := strings.Index(src[b:], end)
	if e == -1 {
		return ""
	}

	return src[b : b+e]
}

// converts outlook time format to local time format
func convertOutlookTime(t string) string {
	parsedUTC, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return t
	}

	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		loc = time.UTC
	}

	local := parsedUTC.In(loc)
	return local.Format("2006-01-02 15:04")
}

// builds SQL insert statement for given event
func BuildInsertSQL(roomID int, event CalendarEvent, userID int, email string) string {
	return fmt.Sprintf(
		"INSERT INTO roomBooking (room_id, concept, start_date, end_date, email, user_id) VALUES (%d, '%s', '%s', '%s', '%s', %d);\n",
		roomID,
		strings.ReplaceAll(event.Subject, "'", "''"),
		convertOutlookTime(event.Start),
		convertOutlookTime(event.End),
		email,
		userID,
	)
}

// handler for /api/outlook/getAllEvents endpoint, builds SQL file with all events from all rooms
func handleGetAllOutlookEvents(w http.ResponseWriter, r *http.Request) {

	apiKey := r.FormValue("key")
	var client *http.Client
	if mC.Development {
		client = setInsecureRequest()
	}

	//get all room emails
	emails := reqGetCalendarEmail(apiKey, client, w)

	var sqlText strings.Builder

	//email associated to the created reservations
	responsibleEmail := "crmactivitats@crm.cat"

	//initial reservations hardcoded to member of activities team
	userID := 235

	//for each room, get all events and build insert statements
	for _, roomEmail := range emails {

		//get room ID by email
		roomID, _ := GetRoomIDByEmail(roomEmail, apiKey, client, w)

		start := time.Now().UTC()
		end := start.AddDate(2, 0, 0)
		soap := buildFindItemsSOAP(roomEmail, start, end)

		body, _ := EWSRequest(soap)

		events := ParseEventsFromEWS(body)

		for _, ev := range events {
			sqlText.WriteString(BuildInsertSQL(roomID, ev, userID, responsibleEmail))
		}
	}

	//send SQL file as response
	w.Header().Set("Content-Disposition", "attachment; filename=\"events.sql.txt\"")
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(sqlText.String()))
}

// formats outlook event in EWS and creates it
func CreateOutlookEvent(roomEmail string, r ReservaEntry) (string, error) {

	soap := fmt.Sprintf(`
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
               xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
               xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Header>
    <t:RequestServerVersion Version="Exchange2016"/>
    <t:ExchangeImpersonation>
      <t:ConnectingSID>
        <t:PrimarySmtpAddress>%s</t:PrimarySmtpAddress>
      </t:ConnectingSID>
    </t:ExchangeImpersonation>
  </soap:Header>

  <soap:Body>
    <m:CreateItem SendMeetingInvitations="SendToNone">
      <m:SavedItemFolderId>
        <t:DistinguishedFolderId Id="calendar"/>
      </m:SavedItemFolderId>

      <m:Items>
        <t:CalendarItem>
          <t:Subject>%s</t:Subject>

          <t:Start>%s</t:Start>
          <t:End>%s</t:End>

          <t:StartTimeZone Id="W. Europe Standard Time"/>
  		  <t:EndTimeZone Id="W. Europe Standard Time"/>

        </t:CalendarItem>
      </m:Items>
    </m:CreateItem>
  </soap:Body>
</soap:Envelope>`,
		roomEmail,
		r.Concept,
		formatAsLocal(r.StartDate),
		formatAsLocal(r.EndDate),
	)

	resp, err := EWSRequest(soap)
	if err != nil {
		return "", err
	}

	eventID := extract(string(resp), "<t:ItemId Id=\"", "\"")
	return eventID, nil
}

// formats local time to outlook time format
func formatAsLocal(local string) string {
	t, _ := time.Parse("2006-01-02 15:04", local)

	loc, _ := time.LoadLocation("Europe/Madrid")
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)

	return t.Format("2006-01-02T15:04:05")
}

// deletes outlook event by room email and event ID
func DeleteOutlookEvent(roomEmail, eventID string) error {

	soap := fmt.Sprintf(`
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
               xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
               xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Header>
    <t:RequestServerVersion Version="Exchange2016"/>
    <t:ExchangeImpersonation>
      <t:ConnectingSID>
        <t:PrimarySmtpAddress>%s</t:PrimarySmtpAddress>
      </t:ConnectingSID>
    </t:ExchangeImpersonation>
  </soap:Header>

  <soap:Body>
    <m:DeleteItem DeleteType="HardDelete" SendMeetingCancellations="SendToNone">
      <m:ItemIds>
        <t:ItemId Id="%s"/>
      </m:ItemIds>
    </m:DeleteItem>
  </soap:Body>
</soap:Envelope>`,
		roomEmail, eventID,
	)

	_, err := EWSRequest(soap)
	return err
}
