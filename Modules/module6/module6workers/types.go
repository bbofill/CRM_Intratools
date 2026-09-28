package module6workers

// config data obj
type Config struct {
	Development          bool   `json:"Development"`
	ServerPort           string `json:"ServerPort"`
	TlsCertPath          string `json:"TlsCertPath"`
	TlsKeyPath           string `json:"TlsKeyPath"`
	CookieEncryptionKey  string `json:"CookieEncryptionKey"`
	Module6ServerPort    string `json:"Module6ServerPort"`
	Module6ApiKey        string `json:"Module6ApiKey"`
	Module6GUIPath       string `json:"Module6GUIPath"`
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

type Department struct {
	Name      string `json:"name"`
	ManagerID int    `json:"manager_id"`
}

type TicketUpdateData struct {
	TicketID    int
	Status      string
	StatusClass string
	UpdatedAt   string
	Message     string
}
