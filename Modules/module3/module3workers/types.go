package module3workers

// config data obj
type Config struct {
	Development          bool   `json:"Development"`
	ServerPort           string `json:"ServerPort"`
	TlsCertPath          string `json:"TlsCertPath"`
	TlsKeyPath           string `json:"TlsKeyPath"`
	CookieEncryptionKey  string `json:"CookieEncryptionKey"`
	Module3ServerPort    string `json:"Module3ServerPort"`
	Module3ApiKey        string `json:"Module3ApiKey"`
	Module3GUIPath       string `json:"Module3GUIPath"`
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
