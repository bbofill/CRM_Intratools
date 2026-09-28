package module10workers

// config data obj
type Config struct {
	Development            bool   `json:"Development"`
	ServerPort             string `json:"ServerPort"`
	TlsCertPath            string `json:"TlsCertPath"`
	TlsKeyPath             string `json:"TlsKeyPath"`
	CookieEncryptionKey    string `json:"CookieEncryptionKey"`
	Module10ServerPort     string `json:"Module10ServerPort"`
	Module10ApiKey         string `json:"Module10ApiKey"`
	Module10GUIPath        string `json:"Module10GUIPath"`
	SmtpHost               string `json:"SmtpHost"`
	SmtpUser               string `json:"SmtpUser"`
	PrivateKeyPassphrase   string `json:"PrivateKeyPassphrase"`
	PDFSignerCommand       string `json:"PDFSignerCommand"`
	PDFCertificatePath     string `json:"PDFCertificatePath"`
	PDFCertificatePassword string `json:"PDFCertificatePassword"`
	PDFSignatureReason     string `json:"PDFSignatureReason"`
	PDFSignatureLocation   string `json:"PDFSignatureLocation"`
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
