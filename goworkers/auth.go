package goworkers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// USER AUTH
// login
func serveAuth(w http.ResponseWriter, r *http.Request) {
	hashedUser := r.URL.Query().Get("hashedUsername")
	hashedPwd := r.URL.Query().Get("hashedPwd")

	hashedUser = SafeInput(hashedUser)
	hashedPwd = SafeInput(hashedPwd)
	if hashedUser == "" || hashedPwd == "" {
		parsed := parseHTMLTemplate(mC.AuthFilePath, nil)
		fmt.Fprintf(w, "%s", parsed)
		return
	}
	role, ok := validateHashedCredentials(hashedUser, hashedPwd)
	if !ok {
		http.Redirect(w, r, "/?error=invalid_credentials", http.StatusSeeOther)
		return
	}
	expTime := time.Now().Add(time.Duration(mC.CookieExpirationTime) * time.Hour)
	claims := AuthCookieClaims{Sub: hashedUser, Role: role, Exp: expTime.Unix()}
	jsonBytes, _ := json.Marshal(claims)
	cipher, err := encryptCookie(string(jsonBytes))
	if err != nil {
		AddControllerLog("Error creating cipher cookie", 1)
	}
	setAuthCookie(w, r, cipher, expTime)
	AddControllerLog("Session started for user "+getUsernameByHash(claims.Sub)+" from IP "+r.RemoteAddr, 0)
	http.Redirect(w, r, "/main", http.StatusFound)
}

// logout
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie(cOoKiEnAmE)
	if err == nil {
		clearAuthCookie(w, r)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// auth proxy func
func authMiddleware(handler http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cOoKiEnAmE)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if _, ok := parseAuthCookie(c.Value, allowedRoles); !ok {
			clearAuthCookie(w, r)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		handler(w, r)
	}
}

func authMiddlewareWithModuleStatus(handler http.HandlerFunc, moduleName string, allowedRoles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cOoKiEnAmE)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		_, ok := parseAuthCookie(c.Value, allowedRoles)
		if !ok {
			clearAuthCookie(w, r)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if !isModuleActive(moduleName) {
			clearAuthCookie(w, r)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, parseHTMLTemplate(mC.ModuleDisabledFilePath, nil))
			return
		}
		handler(w, r)
	}
}

// MODULE AUTH
// module auth proxy func
func moduleAuthMiddleware(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractApiKey(r)
		if !isValidModuleApiKey(apiKey) {
			http.Error(w, "Unauthorized module request", http.StatusUnauthorized)
			return
		}
		handlerFunc(w, r)
	}
}
