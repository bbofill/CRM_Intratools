package goworkers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Set authentication cookie
func setAuthCookie(w http.ResponseWriter, r *http.Request, value string, expiration time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cOoKiEnAmE,
		Value:    value,
		Path:     "/",
		Expires:  expiration,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

// Clear authentication cookie
func clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cOoKiEnAmE,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

// extracts the API key from request headers
func extractApiKey(r *http.Request) string {
	authHeader := r.Header.Get("Authorization") // sanitize in here
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

// checks if the provided key matches any known module API key
func isValidModuleApiKey(key string) bool {
	for _, cfg := range mOdUlEcOnF {
		if cfg.ApiKey == key {
			return true
		}
	}
	return false
}

// extract module name from module api key
func resolveModuleFromKey(key string) string {
	for name, cfg := range mOdUlEcOnF {
		if cfg.ApiKey == key {
			return name
		}
	}
	return "unknown module api key"
}

// check if user role is allowed to acces route
func roleAllowed(role string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	return false
}

// check if Module is enabled before allow
func isModuleActive(name string) bool {
	moduleStatuses.RLock()
	defer moduleStatuses.RUnlock()
	return moduleStatuses.m[name] == "running"
}

// extract content from cypher cookie
func parseAuthCookie(cipherText string, allowedRoles []string) (*AuthCookieClaims, bool) {
	var claims AuthCookieClaims
	plain, err := decryptCookie(cipherText)
	if err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(plain), &claims); err != nil {
		return nil, false
	}
	if claims.Exp < getUnixTimestamp() {
		return nil, false
	}
	if !roleAllowed(claims.Role, allowedRoles) {
		return nil, false
	}
	return &claims, true
}

// extract possible auth from db
func validateHashedCredentials(userHash, pwdHash string) (string, bool) {
	cond := "password='" + pwdHash + "'" // sanitize input here
	query := buildSelectQuery("users", "username, role", cond) + " LIMIT 50"

	results := executeQuery(query)
	for _, row := range results {
		inuname, _ := row["username"].(string)
		role, _ := row["role"].(string)
		if hashString(inuname) == userHash {
			return role, true
		}
	}

	if userHash == hashString("superadmin") && pwdHash == mC.SuperAdminPassword {
		return "admin", true
	}
	return "", false
}
