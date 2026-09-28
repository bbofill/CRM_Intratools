// cron.go
package module4workers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"time"

	cron "github.com/robfig/cron/v3"
)

func InitContractsCron(apiKey string, client *http.Client) {
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		createLog(fmt.Sprintf("Failed to load location: %v", err), 1, apiKey, client, nil)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))

	_, err = c.AddFunc("0 8 * * *", func() {
		createLog("CRON ACTIVATED", 1, apiKey, client, nil)
		if err := deactivateExpiredContracts(apiKey, client, nil); err != nil {
			createLog(fmt.Sprintf("Error in contract cron: %v", err), 1, apiKey, client, nil)
		}
		if err := activateNewContracts(apiKey, client, nil); err != nil {
			createLog(fmt.Sprintf("Error in contract cron: %v", err), 1, apiKey, client, nil)
		}
		if err := deactivateExpiredGroups(apiKey, client, nil); err != nil {
			createLog(fmt.Sprintf("Error in groups cron: %v", err), 1, apiKey, client, nil)
		}
		if err := informEndingContracts(apiKey, client, nil); err != nil {
			createLog(fmt.Sprintf("Error in ending-contracts cron: %v", err), 1, apiKey, client, nil)
		}

	})
	if err != nil {
		createLog(fmt.Sprintf("Failed to schedule contract cron: %v", err), 1, apiKey, client, nil)
	}

	c.Start()
}

func jsonExpiredContracts() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return map[string]interface{}{
		"table":   "contract",
		"columns": "people_id, end_date",
		"condition": fmt.Sprintf(
			"people_id NOT IN (SELECT people_id FROM contract WHERE end_date IS NULL OR DATE(end_date) >= '%s') GROUP BY people_id",
			today,
		),
	}
}

func jsonNewContracts() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return map[string]interface{}{
		"table":   "contract",
		"columns": "people_id",
		"condition": fmt.Sprintf(`
      people_id IN (
        SELECT c.people_id
        FROM contract c
        JOIN people p ON p.id = c.people_id
        WHERE (DATE(c.start_date) <= '%s')
          AND (c.end_date   IS NULL OR DATE(c.end_date)   >= '%s')
      )
      GROUP BY people_id
    `, today, today),
	}
}

func activateNewContracts(apiKey string, client *http.Client, w http.ResponseWriter) error {
	// SELECT para ver contratos nuevos
	selectPayload := jsonNewContracts()
	jsonSelect, _ := json.Marshal(selectPayload)

	resp := getReq(jsonSelect, apiKey, client, w)
	if resp == nil {
		errMsg := "failed to execute getReq in contract cron"
		createLog(errMsg, 1, apiKey, nil, w)
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	var contracts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&contracts); err != nil {
		errMsg := fmt.Sprintf("Error decoding new contracts: %v", err)
		createLog(errMsg, 1, apiKey, nil, w)
		return errors.New(errMsg)
	}

	if len(contracts) == 0 {
		createLog("Contract cron executed — no new contracts found.", 0, apiKey, client, w)
		return nil
	}

	for _, c := range contracts {

		idFloat, ok := c["people_id"].(float64)
		if !ok {
			msg := fmt.Sprintf("Invalid people_id value: %v", c["people_id"])
			createLog(msg, 1, apiKey, nil, w)
			continue
		}
		id := int(idFloat)

		// UPDATE individual para cada people_id
		updatePayload := map[string]interface{}{
			"table":   "people",
			"columns": "active",
			"value":   "1",
			"condition": fmt.Sprintf(
				"id = %d", id,
			),
		}

		jsonData, err := json.Marshal(updatePayload)
		if err != nil {
			msg := fmt.Sprintf("Error serializing payload for ID %d: %v", id, err)
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		respUpdate := putReq(jsonData, apiKey, client, w)
		if respUpdate == nil {
			msg := fmt.Sprintf("Failed to execute putReq for user ID %d", id)
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		body, _ := io.ReadAll(respUpdate.Body)
		respUpdate.Body.Close()

		if respUpdate.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("API error activating user ID %d: %s", id, string(body))
			createLog(msg, 1, apiKey, client, w)
		} else {
			//msg := fmt.Sprintf("User ID %d activated successfully (contract ended on %v)", id, c["end_date"])
			//createLog(msg, 0, apiKey, client, w)
		}
	}

	return nil
}

func deactivateExpiredContracts(apiKey string, client *http.Client, w http.ResponseWriter) error {
	// SELECT para ver contratos vencidos
	selectPayload := jsonExpiredContracts()
	jsonSelect, _ := json.Marshal(selectPayload)

	resp := getReq(jsonSelect, apiKey, client, w)
	if resp == nil {
		errMsg := "failed to execute getReq in contract cron"
		createLog(errMsg, 1, apiKey, client, w)
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	var contracts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&contracts); err != nil {
		errMsg := fmt.Sprintf("Error decoding expired contracts: %v", err)
		createLog(errMsg, 1, apiKey, client, w)
		return errors.New(errMsg)
	}

	if len(contracts) == 0 {
		createLog("Contract cron executed — no expired contracts found.", 0, apiKey, client, w)
		return nil
	}

	for _, c := range contracts {

		idFloat, ok := c["people_id"].(float64)
		if !ok {
			msg := fmt.Sprintf("Invalid people_id value: %v", c["people_id"])
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		id := int(idFloat)

		// UPDATE individual para cada people_id
		updatePayload := map[string]interface{}{
			"table":   "people",
			"columns": "active",
			"value":   "0",
			"condition": fmt.Sprintf(
				"id = %d", id,
			),
		}

		jsonData, err := json.Marshal(updatePayload)
		if err != nil {
			msg := fmt.Sprintf("Error serializing payload for ID %d: %v", id, err)
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		respUpdate := putReq(jsonData, apiKey, client, w)
		if respUpdate == nil {
			msg := fmt.Sprintf("Failed to execute putReq for user ID %d", id)
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		body, _ := io.ReadAll(respUpdate.Body)
		respUpdate.Body.Close()

		if respUpdate.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("API error deactivating user ID %d: %s", id, string(body))
			createLog(msg, 1, apiKey, client, w)
		}
	}

	return nil
}

func jsonExpiredGroups() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return map[string]interface{}{
		"table":   "researchGroup",
		"columns": "intern_code, end_date",
		"condition": fmt.Sprintf(
			"intern_code NOT IN (SELECT intern_code FROM researchGroup WHERE end_date IS '' OR DATE(end_date) >= '%s') GROUP BY intern_code",
			today,
		),
	}
}

func deactivateExpiredGroups(apiKey string, client *http.Client, w http.ResponseWriter) error {
	// SELECT para ver contratos vencidos
	selectPayload := jsonExpiredGroups()
	jsonSelect, _ := json.Marshal(selectPayload)

	resp := getReq(jsonSelect, apiKey, client, w)
	if resp == nil {
		errMsg := "failed to execute getReq in group cron"
		createLog(errMsg, 1, apiKey, client, w)
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	var groups []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		errMsg := fmt.Sprintf("Error decoding expired groups: %v", err)
		createLog(errMsg, 1, apiKey, client, w)
		return errors.New(errMsg)
	}

	if len(groups) == 0 {
		createLog("Group cron executed — no expired groups found.", 0, apiKey, client, w)
		return nil
	}

	for _, c := range groups {

		idStr, ok := c["intern_code"]
		if !ok {
			msg := fmt.Sprintf("Invalid intern_code value: %v", c["intern_code"])
			createLog(msg, 1, apiKey, client, w)
			continue
		}

		// UPDATE individual para cada people_id
		updatePayload := map[string]interface{}{
			"table":   "researchGroup",
			"columns": "outdated",
			"value":   "1",
			"condition": fmt.Sprintf(
				"intern_code = '%s'", idStr,
			),
		}

		jsonData, err := json.Marshal(updatePayload)
		if err != nil {
			msg := fmt.Sprintf("Error serializing payload for ID %s: %v", idStr, err)
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		respUpdate := putReq(jsonData, apiKey, client, w)
		if respUpdate == nil {
			msg := fmt.Sprintf("Failed to execute putReq for group ID %s", idStr)
			createLog(msg, 1, apiKey, client, w)
			continue
		}
		body, _ := io.ReadAll(respUpdate.Body)
		respUpdate.Body.Close()

		if respUpdate.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("API error deactivating group %s: %s", idStr, string(body))
			createLog(msg, 1, apiKey, client, w)
		}
	}

	return nil
}

// -------------------- NOTIFICATION WHEN A CONTRACT IS ENDING --------------------

var rrhhEmail = "RRHH@crm.cat"

func jsonEndingContracts() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return map[string]interface{}{
		"table":   "contract c LEFT JOIN people p ON p.id = c.people_id",
		"columns": "c.id AS contract_id, p.id AS people_id, c.end_date, c.vinculation_type, p.name, p.surname, p.crm_email AS user_email",
		"condition": fmt.Sprintf(`
  c.end_date IS NOT NULL
  AND DATE(c.end_date) BETWEEN DATE('%s') AND DATE('%s','+15 day')
  AND p.active = 1
  AND NOT EXISTS (
    SELECT 1
    FROM contract_end_notifications cen
    WHERE cen.contract_id = c.id
  )
`, today, today),
	}
}

func buildNotifyURL(baseURL, token string) string {
	return fmt.Sprintf("%s/api/contracts/notify-worker?token=%s", baseURL, url.QueryEscape(token))
}

func generateSecureToken() string {
	b := make([]byte, 32) // 256-bit token
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getActiveContractEndToken(contractID int, apiKey string, client *http.Client, w http.ResponseWriter) (string, bool) {
	now := time.Now().Unix()
	cond := fmt.Sprintf("contract_id=%d AND used_at IS NULL AND expires_at > %d", contractID, now)

	payload := map[string]interface{}{
		"table":     "contract_end_notifications",
		"columns":   "token, sent_to_hr_at",
		"condition": cond,
	}
	js, _ := json.Marshal(payload)
	resp := getReq(js, apiKey, client, w)
	if resp == nil {
		return "", false
	}
	defer resp.Body.Close()

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return "", false
	}
	if len(rows) == 0 {
		return "", false
	}
	tok := fmt.Sprint(rows[0]["token"])
	return tok, tok != ""
}

func createOrReuseContractEndToken(contractID, peopleID int, expiryUnix int64, apiKey string, client *http.Client, w http.ResponseWriter) (string, bool, error) {
	if tok, ok := getActiveContractEndToken(contractID, apiKey, client, w); ok {
		return tok, true, nil // reused
	}

	token := generateSecureToken()
	tokenPayload := map[string]interface{}{
		"table":   "contract_end_notifications",
		"columns": "contract_id, people_id, token, expires_at",
		"value":   fmt.Sprintf("%d,%d,%s,%d", contractID, peopleID, token, expiryUnix),
	}
	js, _ := json.Marshal(tokenPayload)

	resp := postReq(js, apiKey, client, w)
	if resp == nil {
		errMsg := "failed to execute postReq creating contract_end_notifications"
		createLog(errMsg, 1, apiKey, client, w)
		return "", false, errors.New(errMsg)
	}
	defer resp.Body.Close()

	return token, false, nil
}

func informEndingContracts(apiKey string, client *http.Client, w http.ResponseWriter) error {
	selectPayload := jsonEndingContracts()
	jsonSelect, _ := json.Marshal(selectPayload)

	resp := getReq(jsonSelect, apiKey, client, w)
	if resp == nil {
		errMsg := "failed to execute getReq in informEndingContracts"
		createLog(errMsg, 1, apiKey, client, w)
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	var contracts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&contracts); err != nil {
		errMsg := fmt.Sprintf("Error decoding ending contracts: %v", err)
		createLog(errMsg, 1, apiKey, client, w)
		return errors.New(errMsg)
	}

	if len(contracts) == 0 {
		return nil
	}

	baseURL := "https://crmintratools.crm.cat"
	if baseURL == "" {
		createLog("PUBLIC_BASE_URL not set", 1, apiKey, client, w)
		return nil
	}

	for _, c := range contracts {

		contractID := int(c["contract_id"].(float64))
		peopleID := int(c["people_id"].(float64))
		endDateStr := fmt.Sprint(c["end_date"])

		endDateTime, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			createLog(
				fmt.Sprintf("Invalid end_date for contract %d: %v", contractID, err),
				1, apiKey, client, w,
			)
			continue
		}
		endDate := endDateTime.Format("02/01/2006")
		vincType := unescapeComma(fmt.Sprint(c["vinculation_type"]))
		email := fmt.Sprint(c["user_email"])
		name := unescapeComma(fmt.Sprint(c["name"]))
		surname := unescapeComma(fmt.Sprint(c["surname"]))

		expiry := time.Now().Add(30 * 24 * time.Hour).Unix()

		token, reused, err := createOrReuseContractEndToken(contractID, peopleID, expiry, apiKey, client, w)
		if err != nil {
			createLog(fmt.Sprintf("Failed token for contract %d: %v", contractID, err), 1, apiKey, client, w)
			continue
		}
		if reused {
			continue
		}

		if email != "" {
			notifyURL := buildNotifyURL(baseURL, token)
			sendContractEndingEmailToHR(rrhhEmail, name, surname, vincType, endDate, notifyURL, email)
		} else {
			sendContractEndingEmailToHRWOworker(rrhhEmail, name, surname, vincType, endDate)
		}

		tokenPayload := map[string]interface{}{
			"table":   "contract_end_notifications",
			"columns": "sent_to_hr_at",
			"value":   fmt.Sprintf("%d", time.Now().Unix()),
			"condition": fmt.Sprintf(
				"token='%s'", token,
			),
		}
		js, _ := json.Marshal(tokenPayload)

		resp := putReq(js, apiKey, client, w)
		if resp == nil {
			errMsg := "failed to execute putReq updating contract_end_notifications"
			createLog(errMsg, 1, apiKey, client, w)
		}
		defer resp.Body.Close()

	}

	return nil
}

func sendContractEndingEmailToHR(to, name, surname, vincType, endDate, notifyURL, workerEmail string) {
	subject := "CRMIntratools - Contracte finalitza aviat"

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; background-color:#f7f7f7; padding:20px;">

  <div style="max-width:650px; margin:0 auto; background:white; padding:30px; border-radius:8px;
              box-shadow:0 2px 8px rgba(0,0,0,0.15); color:#333;">

    <!-- AVÍS A RRHH -->
    <h2 style="color:#8b0000; text-align:center; margin:0 0 16px 0;">
      Contracte finalitza en 15 dies
    </h2>

    <p style="margin:6px 0;"><b>Empleat:</b> %s %s</p>
    <p style="margin:6px 0;"><b>Tipus de contracte:</b> %s</p>
    <p style="margin:6px 0;"><b>Data de finalització:</b> %s</p>
    <p style="margin:6px 0;"><b>Correu electrònic del treballador:</b> %s</p>

    <p style="margin-top:16px;">Si vols notificar al treballador, fes clic:</p>
    <div style="text-align:center; margin:18px 0 10px 0;">
      <a href="%s" style="padding:12px 20px; background:#b30000; color:white;
                          text-decoration:none; font-weight:bold; border-radius:6px; display:inline-block;">
        Notificar treballador
      </a>
    </div>

    <p style="font-size:12px; color:#777; margin:0 0 18px 0;">
      Aquest enllaç caduca en 30 dies per motius de seguretat.
    </p>

    <!-- PREVIEW CORREU TREBALLADOR -->
    <hr style="margin:24px 0; border:none; border-top:1px solid #ddd;">

    <div style="background:#f9f9f9; border:1px solid #e8e8e8; border-radius:8px; padding:16px;">
      <h3 style="margin:0 0 12px 0; color:#333; font-size:16px;">
        Correu que rebrà el treballador (vista prèvia)
      </h3>

      <p style="margin:0 0 10px 0; color:#555; font-size:13px;">
        <b>Destinatari:</b> %s
      </p>

      <!-- Català -->
      <div style="background:#fff7f7; border:1px solid #f0d6d6; border-radius:8px; padding:16px; margin:12px 0;">
        <p style="margin-top:0;">Bon dia, <b>%s %s</b>,</p>

        <p>
          El proper <b>%s</b> finalitza la teva vinculació amb el nostre centre.
        </p>

        <p><b>Abans de la teva sortida, et demanem, si us plau, que tinguis en compte els següents aspectes:</b></p>
        <ul style="margin-top:8px; margin-bottom:8px;">
          <li>Retornar la clau/targeta del despatx.</li>
          <li>Retornar l’equipament informàtic facilitat pel centre (si escau).</li>
          <li>Notificar si cal una extensió temporal addicional als tres mesos concedits inicialment des del correu electrònic del centre.</li>
          <li>Deixar el despatx endreçat, amb els calaixos i els armaris buits.</li>
        </ul>

        <p>
          Aprofitem també per recordar-te que, si necessites qualsevol documentació administrativa
          (com ara el certificat de serveis prestats, una carta de constància de treball o còpies de nòmines),
          només cal que ens ho facis saber i t’ho farem arribar al més aviat possible.
        </p>

        <p style="margin-bottom:0;">
          Volem agrair-te sincerament la teva contribució, implicació i col·laboració durant la teva etapa al nostre centre.
          Ha estat un plaer comptar amb tu, i et desitgem molta sort i èxits en els teus propers reptes professionals i personals.
        </p>

        <p style="margin-bottom:0;"><b>Una cordial salutació,</b></p>
      </div>

      <!-- English -->
      <div style="background:#f7fbff; border:1px solid #d6e6f5; border-radius:8px; padding:16px; margin:12px 0 0 0;">
        <p style="margin-top:0;">Hello, <b>%s %s</b>,</p>

        <p>
          On <b>%s</b>, your affiliation with our centre will come to an end.
        </p>

        <p><b>Before your departure, we kindly ask you to take care of the following:</b></p>
        <ul style="margin-top:8px; margin-bottom:8px;">
          <li>Return the office key/card.</li>
          <li>Return any IT equipment provided by the centre (if applicable).</li>
          <li>Notify if an additional temporary extension beyond the three months initially granted from the centre’s email account is required.</li>
          <li>Leave the office tidy, with all drawers and cabinets emptied.</li>
        </ul>

        <p>
          We would also like to remind you that, should you need any administrative documentation
          (such as a certificate of services rendered, an employment confirmation letter, or copies of payslips),
          please do not hesitate to let us know and we will be happy to provide it as soon as possible.
        </p>

        <p style="margin-bottom:0;">
          We would like to sincerely thank you for your contribution, commitment, and collaboration during your time at our centre.
          It has been a pleasure working with you, and we wish you every success in your next professional and personal step.
        </p>

        <p style="margin-bottom:0;"><b>Kind regards,</b></p>
      </div>
    </div>

    <!-- FOOTER -->
    <hr style="margin-top:24px; border:none; border-top:1px solid #ddd;">
    <p style="font-size:12px; color:#aaa; text-align:center; margin:0;">
      Centre de Recerca Matemàtica - IT
    </p>
  </div>

</body>
</html>
`,
		// Avis RRHH
		name, surname, vincType, endDate, workerEmail, notifyURL,
		// Preview
		workerEmail,
		// Català
		name, surname, endDate,
		// English
		name, surname, endDate,
	)

	_ = sendEmail(to, subject, body)
}

func sendEmail(to string, subject string, body string) error {
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

func sendContractEndingEmailToHRWOworker(to, name, surname, vincType, endDate string) {
	subject := "CRMIntratools - Contracte finalitza aviat"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; background-color:#f7f7f7; padding:20px;">
  <div style="max-width:650px; margin:0 auto; background:white; padding:30px; border-radius:8px;
              box-shadow:0 2px 8px rgba(0,0,0,0.15); color:#333;">

    <h2 style="color:#8b0000; text-align:center; margin-bottom:10px;">
      Contracte finalitza en 15 dies
    </h2>

    <p><b>Empleat:</b> %s %s</p>
    <p><b>Tipus de contracte:</b> %s</p>
    <p><b>Data de finalització:</b> %s</p>

    <p style="font-size:12px; color:#777;">
      Aquest treballador no té un correu electrònic professional associat. Per tant, no se li pot enviar una notificació automàtica.
    </p>

    <hr style="margin-top:30px; border:none; border-top:1px solid #ddd;">
    <p style="font-size:12px; color:#aaa; text-align:center;">
      Centre de Recerca Matemàtica - IT
    </p>
  </div>
</body>
</html>
`, name, surname, vincType, endDate)

	_ = sendEmail(to, subject, body)
}
