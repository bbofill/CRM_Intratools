package module9workers

// config data obj
type Config struct {
	Development          bool   `json:"Development"`
	ServerPort           string `json:"ServerPort"`
	TlsCertPath          string `json:"TlsCertPath"`
	TlsKeyPath           string `json:"TlsKeyPath"`
	CookieEncryptionKey  string `json:"CookieEncryptionKey"`
	Module9ServerPort    string `json:"Module9ServerPort"`
	Module9ApiKey        string `json:"Module9ApiKey"`
	Module9GUIPath       string `json:"Module9GUIPath"`
	SmtpHost             string `json:"SmtpHost"`
	SmtpUser             string `json:"SmtpUser"`
	PrivateKeyPassphrase string `json:"PrivateKeyPassphrase"`
}

// type for auth and role parse
type AuthCookieClaims struct {
	Sub  string `json:"sha256username"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

// type for log parsing
type Log struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}
