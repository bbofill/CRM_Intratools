package module4workers

// config data obj
type Config struct {
	Development          bool   `json:"Development"`
	ServerPort           string `json:"ServerPort"`
	TlsCertPath          string `json:"TlsCertPath"`
	TlsKeyPath           string `json:"TlsKeyPath"`
	CookieEncryptionKey  string `json:"CookieEncryptionKey"`
	Module4ServerPort    string `json:"Module4ServerPort"`
	Module4ApiKey        string `json:"Module4ApiKey"`
	Module4GUIPath       string `json:"Module4GUIPath"`
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

type Training struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Hours    int    `json:"hours"`
}

type Group struct {
	ResearchCode      string `json:"research_code"`
	InternCode        string `json:"intern_code"`
	Name              string `json:"name"`
	FormerName        string `json:"former_name"`
	StartDate         string `json:"start_date"`
	EndDate           string `json:"end_date"`
	Outdated          int    `json:"outdated"`
	Project           int    `json:"project"`
	Sgr               int    `json:"sgr"`
	CodiReconeixement string `json:"codi_reconeixement"`
	DataObtencio      string `json:"data_obtencio"`
	Category          string `json:"category"`
	Type              string `json:"type"`
	Cif               string `json:"cif"`
	Character         string `json:"character"`
	Typology          string `json:"typology"`
}

type deleteReqBody struct {
	ID string `json:"id"`
}

type EndingContract struct {
	ContractID      int
	PeopleID        int
	EndDate         string
	VinculationType string
	Name            string
	Surname         string
	UserEmail       string
}
