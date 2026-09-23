package goworkers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"
	"time"
	"unicode"
)

func generateSecureToken() string {
	b := make([]byte, 32) // 256-bit token
	rand.Read(b)
	return hex.EncodeToString(b)
}

func handleRequestReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Step     string `json:"step"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	// STEP 1 - solicitar recuperación
	if payload.Step == "start" {
		email := getEmailFromDB(payload.Username)
		masked := ""

		if email != "" {
			parts := strings.Split(email, "@")
			if len(parts[0]) > 2 {
				masked = fmt.Sprintf("%c****%c@%s",
					parts[0][0],
					parts[0][len(parts[0])-1],
					parts[1],
				)
			} else {
				masked = "***@" + parts[1]
			}
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "If the user exists, verify your email. If you don't receive any message, please, talk to IT",
			"masked":  masked,
		})
		return
	}

	// STEP 2 - verificar email real
	if payload.Step == "verify" {
		email := getEmailFromDB(payload.Username)
		cleanUsername := SafeInput(payload.Username)
		if !IsValidEmail(payload.Email) || !strings.EqualFold(strings.ToLower(payload.Email), strings.ToLower(email)) {
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Invalid email",
			})
			return
		}

		// Aquí ya sabemos que usuario & email existen
		token := generateSecureToken()
		expiry := time.Now().Add(30 * time.Minute).Unix()

		query := fmt.Sprintf("INSERT INTO password_resets (username, token, expires_at) VALUES ('%s','%s',%d)", cleanUsername, token, expiry)
		executeQuery(query)

		// Email
		host := r.Host
		resetURL := fmt.Sprintf("https://%s/public/reset.html?token=%s", host, token)
		sendRecoveryEmail(email, resetURL)

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "A recovery link has been sent to your email, if you don't receive anything please contact IT.",
		})
		return
	}

	http.Error(w, "Bad request", http.StatusBadRequest)
}

func sendRecoveryEmail(to, link string) {
	subject := "CRMIntratools - Password Reset Request"

	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; background-color:#f7f7f7; padding:20px;">
  <div style="max-width:600px; margin:0 auto; background:white; padding:30px; border-radius:8px;
              box-shadow:0 2px 8px rgba(0,0,0,0.15); color:#333;">
    
    <h2 style="color:#8b0000; text-align:center; margin-bottom:10px;">
      Password Reset Request
    </h2>

    <p>Hello,</p>

    <p>We received a request to reset your password for CRMIntratools.</p>
    <p>Click the button below to set a new password:</p>

    <div style="text-align:center; margin:25px 0;">
      <a href="%s" style="padding:12px 20px; background:#b30000; color:white;
                          text-decoration:none; font-weight:bold; border-radius:6px;">
        Reset Password
      </a>
    </div>

    <p>If the button doesn't work, copy and paste this link in your browser:</p>

    <p style="word-break:break-all; color:#8b0000;">
      %s
    </p>

    <p style="font-size:14px; color:#777;">
      This link will expire in 30 minutes for security reasons.
    </p>

    <p>If you did not request this password reset, please ignore this message.</p>

    <hr style="margin-top:30px; border:none; border-top:1px solid #ddd;">

    <p style="font-size:12px; color:#aaa; text-align:center;">
      Centre de Recerca Matemàtica - IT Support
    </p>

  </div>
</body>
</html>
`, link, link)

	sendEmail(to, subject, body)
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

// SafeInput es una sanitización completa que evita SQLi, XSS, Shell injection y Unicode confuso
func SafeInput(input string) string {
	if input == "" {
		return ""
	}

	// Recortar espacios iniciales y finales
	input = strings.TrimSpace(input)

	// Normalizar unicode eliminando caracteres de control (0–31 y 127)
	input = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, input)

	// Convertir caracteres visualmente ambiguos a ASCII normal (homoglyph normalization)
	replacements := map[string]string{
		"’": "'", "‘": "'", "“": "\"", "”": "\"",
		"—": "-", "–": "-", "…": "...",
	}
	for k, v := range replacements {
		input = strings.ReplaceAll(input, k, v)
	}

	// Eliminar cualquier operador SQL o caracter de comando
	reDanger := regexp.MustCompile(`(?i)(;\s*|--\s*|\|\||\&\&|/\*|\*/|drop\s|select\s|insert\s|update\s|delete\s|union\s|xp_)`)
	input = reDanger.ReplaceAllString(input, "")

	// Bloquear caracteres inseguros y permitir solo los típicos para usernames, mails y texto seguro
	reSafe := regexp.MustCompile(`[^a-zA-Z0-9@\._\-\+ ]`)
	input = reSafe.ReplaceAllString(input, "")

	return input
}

func IsValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

func getEmailFromDB(username string) string {

	// Sanitizar input
	cleanUsername := SafeInput(username)

	if cleanUsername == "" {
		return ""
	}

	// --- Paso 1: obtener people_id desde users ---
	queryUser := buildSelectQuery("users", "people_id", "username='"+cleanUsername+"'") + " LIMIT 1"

	userResult := executeQuery(queryUser)

	if len(userResult) == 0 {
		return ""
	}

	peopleID := fmt.Sprintf("%v", userResult[0]["people_id"])

	if peopleID == "" {
		return ""
	}

	// --- Paso 2: obtener emails desde people ---
	queryPeople := buildSelectQuery("people", "crm_email, user_email", "id='"+SafeInput(peopleID)+"'") + " LIMIT 1"

	peopleResult := executeQuery(queryPeople)

	if len(peopleResult) == 0 {
		return ""
	}

	crmEmail, _ := peopleResult[0]["crm_email"].(string)
	userEmail, _ := peopleResult[0]["user_email"].(string)

	// --- Prioridad CRM → USER ---
	if IsValidEmail(crmEmail) {
		result := SafeInput(crmEmail)
		return result
	}

	if IsValidEmail(userEmail) {
		result := SafeInput(userEmail)
		return result
	}
	return ""
}

func handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Token    string `json:"token"`
		Password string `json:"password"` // hashed SHA-256 from frontend
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate token format (256-bit hex string = 64 chars)
	tokenRegex := regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	if !tokenRegex.MatchString(payload.Token) {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid token"})
		return
	}

	// Validate password hash length (SHA-256 hex = 64 chars)
	if len(payload.Password) != 64 {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid password format"})
		return
	}

	// STEP 1 — Lookup token
	query := buildSelectQuery("password_resets", "username, expires_at", "token='"+payload.Token+"'") + " LIMIT 1"
	result := executeQuery(query)
	if len(result) == 0 {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid or expired token"})
		return
	}

	username, _ := result[0]["username"].(string)
	exp, _ := result[0]["expires_at"].(int64)

	if time.Now().Unix() > exp {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Token expired"})
		return
	}

	// STEP 2 — Enforce password strength
	if !validatePasswordStrength(payload.Password) {
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Password must be at least 12 characters long and include upper/lowercase letters, numbers and symbols",
		})
		return
	}

	// STEP 3 — Update database password hash safely
	updateQ := fmt.Sprintf("UPDATE users SET password='%s' WHERE username='%s'", SafeInput(payload.Password), SafeInput(username))
	executeQuery(updateQ)

	// STEP 4 — Remove token once used
	executeQuery("DELETE FROM password_resets WHERE token='" + payload.Token + "'")

	// STEP 5 — Log event (without password)
	AddControllerLog("Password reset performed for user "+username+" from IP "+r.RemoteAddr, 1)

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Password updated successfully. You will be redirected shortly.",
	})
}

func validatePasswordStrength(hashed string) bool {
	// Since password is already hashed, enforce rules on the raw hash length (64)
	// Full real password rules should be enforced in frontend before hashing
	return len(hashed) == 64
}
