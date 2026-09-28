package module7workers

import (
	"net/http"
)

// config data
var mC Config

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
	err := http.ListenAndServeTLS(":"+mC.Module7ServerPort, "../../"+mC.TlsCertPath, "../../"+mC.TlsKeyPath, nil)
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
	println("starting module 7...")
	server()
	select {}
}
