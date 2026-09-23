package goworkers

// types definition
// type for json controller config parse
type Config struct {
	Development             bool        `json:"Development"`
	SuperAdminPassword      string      `json:"SuperAdminPassword"`
	ServerPort              string      `json:"ServerPort"`
	CookieExpirationTime    int         `json:"CookieExpirationTime"`
	CookieEncryptionKey     string      `json:"CookieEncryptionKey"`
	TlsCert                 string      `json:"TlsCertPath"`
	TlsKey                  string      `json:"TlSKeyPath"`
	LogCypherPubKeyFilePath string      `json:"LogCypherPubKeyFilePath"`
	DBFilePath              string      `json:"DBFilePath"`
	MSSQL                   MSSQLConfig `json:"MSSQL"`
	AuthFilePath            string      `json:"AuthFilePath"`
	GUIAssetsDir            string      `json:"GUIAssetsDir"`
	MainFilePath            string      `json:"MainFilePath"`
	ModuleDisabledFilePath  string      `json:"ModuleDisabledFilePath"`
	SmtpHost                string      `json:"SmtpHost"`
	SmtpUser                string      `json:"SmtpUser"`

	ModuleConfig
}

type MSSQLConfig struct {
	Enabled  bool   `json:"Enabled"`
	Host     string `json:"Host"`
	Port     string `json:"Port"`
	User     string `json:"User"`
	Password string `json:"Password"`
	Database string `json:"Database"`
}

// type for json module config parse
type ModuleConfig struct {
	// module 1
	Module0Name          string `json:"Module0Name"`
	Module0ServerPort    string `json:"Module0ServerPort"`
	Module0ApiKey        string `json:"Module0ApiKey"`
	Module0GUIPath       string `json:"Module0GUIPath"`
	Module0StartCommand  string `json:"Module0StartCommand"`
	Module0StatusEnabled bool   `json:"Module0StatusEnabled"`

	// module 1
	Module1Name          string `json:"Module1Name"`
	Module1ServerPort    string `json:"Module1ServerPort"`
	Module1ApiKey        string `json:"Module1ApiKey"`
	Module1GUIPath       string `json:"Module1GUIPath"`
	Module1StartCommand  string `json:"Module1StartCommand"`
	Module1StatusEnabled bool   `json:"Module1StatusEnabled"`
	// module 2
	Module2Name          string `json:"Module2Name"`
	Module2ServerPort    string `json:"Module2ServerPort"`
	Module2ApiKey        string `json:"Module2ApiKey"`
	Module2GUIPath       string `json:"Module2GUIPath"`
	Module2StartCommand  string `json:"Module2StartCommand"`
	Module2StatusEnabled bool   `json:"Module2StatusEnabled"`
	// module 3
	Module3Name          string `json:"Module3Name"`
	Module3ServerPort    string `json:"Module3ServerPort"`
	Module3ApiKey        string `json:"Module3ApiKey"`
	Module3GUIPath       string `json:"Module3GUIPath"`
	Module3StartCommand  string `json:"Module3StartCommand"`
	Module3StatusEnabled bool   `json:"Module3StatusEnabled"`
	// module 4
	Module4Name          string `json:"Module4Name"`
	Module4ServerPort    string `json:"Module4ServerPort"`
	Module4ApiKey        string `json:"Module4ApiKey"`
	Module4GUIPath       string `json:"Module4GUIPath"`
	Module4StartCommand  string `json:"Module4StartCommand"`
	Module4StatusEnabled bool   `json:"Module4StatusEnabled"`
	// module 5
	Module5Name          string `json:"Module5Name"`
	Module5ServerPort    string `json:"Module5ServerPort"`
	Module5ApiKey        string `json:"Module5ApiKey"`
	Module5GUIPath       string `json:"Module5GUIPath"`
	Module5StartCommand  string `json:"Module5StartCommand"`
	Module5StatusEnabled bool   `json:"Module5StatusEnabled"`
	// module 6
	Module6Name          string `json:"Module6Name"`
	Module6ServerPort    string `json:"Module6ServerPort"`
	Module6ApiKey        string `json:"Module6ApiKey"`
	Module6GUIPath       string `json:"Module6GUIPath"`
	Module6StartCommand  string `json:"Module6StartCommand"`
	Module6StatusEnabled bool   `json:"Module6StatusEnabled"`
	// module 7
	Module7Name          string `json:"Module7Name"`
	Module7ServerPort    string `json:"Module7ServerPort"`
	Module7ApiKey        string `json:"Module7ApiKey"`
	Module7GUIPath       string `json:"Module7GUIPath"`
	Module7StartCommand  string `json:"Module7StartCommand"`
	Module7StatusEnabled bool   `json:"Module7StatusEnabled"`
	// module 8
	Module8Name          string `json:"Module8Name"`
	Module8ServerPort    string `json:"Module8ServerPort"`
	Module8ApiKey        string `json:"Module8ApiKey"`
	Module8GUIPath       string `json:"Module8GUIPath"`
	Module8StartCommand  string `json:"Module8StartCommand"`
	Module8StatusEnabled bool   `json:"Module8StatusEnabled"`
	// module 9
	Module9Name          string `json:"Module9Name"`
	Module9ServerPort    string `json:"Module9ServerPort"`
	Module9ApiKey        string `json:"Module9ApiKey"`
	Module9GUIPath       string `json:"Module9GUIPath"`
	Module9StartCommand  string `json:"Module9StartCommand"`
	Module9StatusEnabled bool   `json:"Module9StatusEnabled"`
	// module 10
	Module10Name          string `json:"Module10Name"`
	Module10ServerPort    string `json:"Module10ServerPort"`
	Module10ApiKey        string `json:"Module10ApiKey"`
	Module10GUIPath       string `json:"Module10GUIPath"`
	Module10StartCommand  string `json:"Module10StartCommand"`
	Module10StatusEnabled bool   `json:"Module10StatusEnabled"`
	// module 11
	Module11Name          string `json:"Module11Name"`
	Module11ServerPort    string `json:"Module11ServerPort"`
	Module11ApiKey        string `json:"Module11ApiKey"`
	Module11GUIPath       string `json:"Module11GUIPath"`
	Module11StartCommand  string `json:"Module11StartCommand"`
	Module11StatusEnabled bool   `json:"Module11StatusEnabled"`
}

// type for auth and role parse
type AuthCookieClaims struct {
	Sub  string `json:"sha256username"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

// type for log parsing
type Log struct {
	ID           int64  `json:"id"`
	Module       string `json:"module"`
	Timestamp    int64  `json:"timestamp"`
	Msg          string `json:"msg"`
	Code         int    `json:"code"`
	Hash         string `json:"hash"`
	PreviousHash string `json:"previousHash"`
}

// type for module obj, here we implement module methods (validateConfig, start, restart, kill)
type Module map[string]struct {
	Port         string
	ApiKey       string
	StartCommand string
	Enabled      bool
}

// type for control moudules (moduleX, {start|stop})
type ModuleAction struct {
	Name   string
	Action string
}

// type for gui templates out parse
type TemplateData map[string]interface{}

// type for API in parse
type ApiData map[string]interface{}
