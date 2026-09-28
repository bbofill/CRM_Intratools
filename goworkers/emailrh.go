package goworkers

import (
	"fmt"
	"net/http"
	"regexp"
	"time"
)

func handleInformationContracts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	tokenRegex := regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	if !tokenRegex.MatchString(token) {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	q := buildSelectQuery(
		"contract_end_notifications",
		"contract_id, people_id, expires_at, used_at",
		"token='"+SafeInput(token)+"'",
	) + " LIMIT 1"

	res := executeQuery(q)
	if len(res) == 0 {
		renderNotifyResultPage(w, "Invalid or expired token", "This link is invalid or has expired.", false)
		return

	}

	contractID64, ok := res[0]["contract_id"].(int64)
	if !ok {
		if v, ok2 := res[0]["contract_id"].(int); ok2 {
			contractID64 = int64(v)
		} else {
			http.Error(w, "Bad contract_id type", http.StatusInternalServerError)
			return
		}
	}

	peopleID64, ok := res[0]["people_id"].(int64)
	if !ok {
		if v, ok2 := res[0]["people_id"].(int); ok2 {
			peopleID64 = int64(v)
		} else {
			http.Error(w, "Bad people_id type", http.StatusInternalServerError)
			return
		}
	}

	expiresAt, ok := res[0]["expires_at"].(int64)
	if !ok {
		if v, ok2 := res[0]["expires_at"].(int); ok2 {
			expiresAt = int64(v)
		} else {
			http.Error(w, "Bad expires_at type", http.StatusInternalServerError)
			return
		}
	}

	contractID := int(contractID64)
	peopleID := int(peopleID64)
	usedAt := res[0]["used_at"]

	if time.Now().Unix() > expiresAt {
		http.Error(w, "Token expired", http.StatusUnauthorized)
		return
	}
	if usedAt != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderNotifyResultPage(w, "Already sent", "This notification link was already used previously.", true)

		return
	}

	now := time.Now().Unix()
	fmt.Println("new query", fmt.Sprintf(
		"UPDATE contract_end_notifications SET used_at=%d WHERE token='%s' AND used_at IS NULL AND expires_at > %d",
		now, SafeInput(token), now,
	))
	executeQuery(fmt.Sprintf(
		"UPDATE contract_end_notifications SET used_at=%d WHERE token='%s' AND used_at IS NULL AND expires_at > %d",
		now, SafeInput(token), now,
	))

	recheck := executeQuery(buildSelectQuery(
		"contract_end_notifications", "used_at", "token='"+SafeInput(token)+"'",
	) + " LIMIT 1")

	if len(recheck) == 0 || recheck[0]["used_at"] == nil {
		http.Error(w, "Token already used or expired", http.StatusUnauthorized)
		return
	}
	contract, err := loadContractAndPerson(contractID, peopleID)
	if err != nil || !IsValidEmail(contract.UserEmail) {
		http.Error(w, "Cannot load worker data", http.StatusInternalServerError)
		return
	}

	if err := sendEndingContractEmailToWorker(contract.UserEmail, contract.Name, contract.Surname, contract.VincType, contract.EndDate); err != nil {
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	executeQuery(fmt.Sprintf(
		"UPDATE contract_end_notifications SET sent_to_worker_at=%d WHERE token='%s'",
		time.Now().Unix(), SafeInput(token),
	))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderNotifyResultPage(w, "Worker notified", "The worker has been notified successfully by email.", true)

}

// estructura para cargar datos
type ContractPerson struct {
	UserEmail string
	Name      string
	Surname   string
	VincType  string
	EndDate   string
}

func loadContractAndPerson(contractID int, peopleID int) (ContractPerson, error) {
	q := fmt.Sprintf(`
SELECT 
  p.crm_email AS user_email,
  p.name AS name,
  p.surname AS surname,
  c.vinculation_type AS vinculation_type,
  c.end_date AS end_date
FROM contract c
JOIN people p ON p.id = c.people_id
WHERE c.id = %d
LIMIT 1
`, contractID)

	res := executeQuery(q)
	if len(res) == 0 {
		return ContractPerson{}, fmt.Errorf("contract not found")
	}

	cp := ContractPerson{
		UserEmail: fmt.Sprint(res[0]["user_email"]),
		Name:      fmt.Sprint(res[0]["name"]),
		Surname:   fmt.Sprint(res[0]["surname"]),
		VincType:  fmt.Sprint(res[0]["vinculation_type"]),
		EndDate:   formatDateLongEN(res[0]["end_date"]),
	}
	return cp, nil
}

func sendEndingContractEmailToWorker(to, name, surname, vincType, endDate string) error {
	to = SafeInput(to)
	name = SafeInput(name)
	surname = SafeInput(surname)
	endDate = SafeInput(endDate)

	if !IsValidEmail(to) {
		return fmt.Errorf("invalid email: %s", to)
	}

	subject := "CRMIntratools - Affiliation ending notice"

	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; background-color:#f7f7f7; padding:20px;">
  <div style="max-width:650px; margin:0 auto; background:white; padding:30px; border-radius:8px;
              box-shadow:0 2px 8px rgba(0,0,0,0.15); color:#333;">

    <h2 style="color:#8b0000; text-align:center; margin-bottom:10px;">
      Affiliation ending notice
    </h2>


<p style="margin-top:0;">Bon dia, %s %s,</p>

    <!-- Català -->
    <div style="background:#fff7f7; border:1px solid #f0d6d6; border-radius:8px; padding:16px; margin:18px 0;">
      <p style="margin-top:0;">
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

      <p style="margin-bottom:0;">
        <b>Una cordial salutació,</b>
      </p>
    </div>

	<p style="margin-top:0;">Hello, %s %s,</p>
    <!-- English -->
    <div style="background:#f7fbff; border:1px solid #d6e6f5; border-radius:8px; padding:16px; margin:18px 0;">
      <p style="margin-top:0;">
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

      <p style="margin-bottom:0;">
        <b>Kind regards,</b>
      </p>
    </div>


    <hr style="margin-top:30px; border:none; border-top:1px solid #ddd;">

    <p style="font-size:12px; color:#aaa; text-align:center;">
      Centre de Recerca Matemàtica<br>
      Human Resources Department
    </p>

	<p style="font-size:12px; color:#777;">
		This is an automated message. Please do not reply to this email.<br>
		For any questions, please contact Human Resources at RRHH@crm.cat.
		</p>

  </div>
</body>
</html>
`, name, surname, endDate, name, surname, endDate)

	return sendEmail(to, subject, body)
}

func formatDateLongEN(v interface{}) string {
	var tt time.Time
	switch t := v.(type) {
	case time.Time:
		tt = t
	case string:
		// intenta parsear YYYY-MM-DD
		parsed, err := time.Parse("2006-01-02", t[:10])
		if err == nil {
			tt = parsed
		} else {
			return t
		}
	case []byte:
		s := string(t)
		parsed, err := time.Parse("2006-01-02", s[:10])
		if err == nil {
			tt = parsed
		} else {
			return s
		}
	default:
		return fmt.Sprint(v)
	}
	return tt.Format("02 Jan 2006") // "14 Feb 2026"
}

func renderNotifyResultPage(w http.ResponseWriter, title, message string, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	statusColor := "#2e7d32" // verde
	headerBg := "#e8f5e9"
	icon := "✅"

	if !ok {
		statusColor = "#b71c1c" // rojo
		headerBg = "#ffebee"
		icon = "❌"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>%s</title>
</head>
<body style="font-family: Arial, sans-serif; background-color:#f7f7f7; padding:24px; color:#333;">
  <div style="max-width:720px; margin:0 auto; background:white; padding:30px; border-radius:10px;
              box-shadow:0 2px 10px rgba(0,0,0,0.12);">

    <div style="background:%s; border-radius:8px; padding:16px 18px; margin-bottom:18px;">
      <h2 style="margin:0; color:%s;">%s %s</h2>
    </div>

    <p style="font-size:16px; margin-top:0;">%s</p>

    <p style="font-size:13px; color:#777; margin-top:18px;">
      This is an automated confirmation page. You can close this tab safely.
    </p>

    <hr style="margin-top:26px; border:none; border-top:1px solid #e5e5e5;" />

    <p style="font-size:12px; color:#aaa; text-align:center; margin:0;">
      Centre de Recerca Matemàtica - CRMIntratools
    </p>

  </div>
</body>
</html>`, title, headerBg, statusColor, icon, title, message)

	_, _ = w.Write([]byte(html))
}
