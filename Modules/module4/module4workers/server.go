package module4workers

import (
	"net/http"
	"time"
)

// config data
var mC Config
var httpClient *http.Client

// handle webserver paths
func handleServerRoutes() {
	//set up http route
	http.HandleFunc("/api", serveAPI)
	http.HandleFunc("/Uploads", serveUpload)
}

// start web server
func server() {
	handleServerRoutes()
	//start http server with key
	err := http.ListenAndServeTLS(":"+mC.Module4ServerPort, "../../"+mC.TlsCertPath, "../../"+mC.TlsKeyPath, nil)
	if err != nil {
		println(err)
	}
}

func SetConfig(path string) {
	// read and load config
	mC = readConfigFile(path)
}

// main func, program start here
func StartServer() {
	println("starting module 4...")
	httpClient = &http.Client{Timeout: 30 * time.Second}
	if mC.Development {
		httpClient = setInsecureRequest()
	}

	// Arrancar cron aquí, no en serveAPI
	InitContractsCron(mC.Module4ApiKey, httpClient)
	server()
	select {}
}
