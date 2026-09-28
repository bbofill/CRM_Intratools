package module0workers

type Config struct {
	ModuleApiKey             string `json:"Module0ApiKey"`
	BackendPort              string `json:"ServerPort"`
	ListenPort               string `json:"Module0ServerPort"`
	Development              bool   `json:"Development"`
	TlsKeyPath               string `json:"TlsKeyPath"`
	TlsCertPath              string `json:"TlsCertPath"`
	LogCypherPrivKeyFilePath string `json:"LogCypherPrivKeyFilePath"`
	LogCypherKeyPassphrase   string `json:"LogPrivateKeyPassphrase"`
}

type LogEntry struct {
	ID           int    `json:"id"`
	Module       string `json:"module"`
	Timestamp    int64  `json:"timestamp"`
	Msg          string `json:"msg"`
	Code         int    `json:"code"`
	Hash         string `json:"hash"`
	PreviousHash string `json:"previous_hash"`
}
