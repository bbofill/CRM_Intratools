package goworkers

// webserver things

import (
	"log"
	"net/http"
)

// global vars for auth and server interaction
var (
	mC         Config
	cOoKiEnAmE string
)

// server ping test func
func getConnPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<html><body><p>Pong!<p></body></html>"))
}

// set and handle handle web server paths
func handlePaths() http.Handler {
	mux := http.NewServeMux()

	// recover password
	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("GUI/public"))))
	// API endpoints related to reset that must be public
	mux.HandleFunc("/api/requestReset", handleRequestReset)
	mux.HandleFunc("/api/resetPassword", handleResetPassword)
	// API endpoints for email information
	mux.HandleFunc("/api/contracts/notify-worker", handleInformationContracts)
	// auth images
	mux.Handle("/public-assets/", http.StripPrefix("/public-assets/", http.FileServer(http.Dir("GUI/assets/public"))))
	// auth
	mux.HandleFunc("/", serveAuth)
	mux.HandleFunc("/logout", logoutHandler)
	//server
	mux.HandleFunc("/ping!", authMiddleware(getConnPing, "admin")) // modules can validate controller connection here
	// gui
	mux.HandleFunc("/assets/", authMiddleware(serveStyleDir))                                                                      // scripts and styles
	mux.Handle("/Uploads/", moduleAuthMiddleware(http.StripPrefix("/Uploads/", http.FileServer(http.Dir("./Uploads"))).ServeHTTP)) //images and documents
	mux.HandleFunc("/main", authMiddleware(serveMain))                                                                             // main panel
	// module proxy
	mux.HandleFunc("/module/proxy/", authMiddleware(handleProxy)) // redirect to local module port
	// documentation
	mux.Handle("/documentation/", authMiddleware(http.StripPrefix("/documentation/", http.FileServer(http.Dir("./site"))).ServeHTTP)) // documentation (public)
	// module gui
	mux.HandleFunc("/module/gui/module0/", authMiddlewareWithModuleStatus(serveModule0GUI, mC.Module0Name, "admin"))                                        // module0
	mux.HandleFunc("/module/gui/module1/", authMiddlewareWithModuleStatus(serveModule1GUI, mC.Module1Name, "admin", "user", "manager", "support"))          // module1 (func) (module name) (accepted_roles...)
	mux.HandleFunc("/module/gui/module2/", authMiddlewareWithModuleStatus(serveModule2GUI, mC.Module2Name, "admin", "user", "manager", "guest", "support")) // module2
	mux.HandleFunc("/module/gui/module3/", authMiddlewareWithModuleStatus(serveModule3GUI, mC.Module3Name, "admin", "user", "manager", "guest", "support")) // module3
	mux.HandleFunc("/module/gui/module4/", authMiddlewareWithModuleStatus(serveModule4GUI, mC.Module4Name, "admin", "manager"))                             // module4
	mux.HandleFunc("/module/gui/module5/", authMiddlewareWithModuleStatus(serveModule5GUI, mC.Module5Name, "admin", "manager"))                             // module5
	mux.HandleFunc("/module/gui/module6/", authMiddlewareWithModuleStatus(serveModule6GUI, mC.Module6Name, "admin", "user", "manager", "support"))          // module6
	mux.HandleFunc("/module/gui/module7/", authMiddlewareWithModuleStatus(serveModule7GUI, mC.Module7Name, "admin", "manager", "support"))                  // module7
	mux.HandleFunc("/module/gui/module8/", authMiddlewareWithModuleStatus(serveModule8GUI, mC.Module8Name, "admin", "user", "manager", "support"))          // module8
	mux.HandleFunc("/module/gui/module9/", authMiddlewareWithModuleStatus(serveModule9GUI, mC.Module9Name, "admin", "user", "manager", "support"))          // module9
	mux.HandleFunc("/module/gui/module10/", authMiddlewareWithModuleStatus(serveModule10GUI, mC.Module10Name, "admin", "user", "manager", "support"))       // module10
	mux.HandleFunc("/module/gui/module11/", authMiddlewareWithModuleStatus(serveModule11GUI, mC.Module11Name, "admin"))                                     // module11
	// module api
	//mux.HandleFunc("/Uploads/", moduleAuthMiddleware(serveAPI))
	mux.HandleFunc("/module/api/db", moduleAuthMiddleware(serveAPI)) // crud i/o
	mux.HandleFunc("/module/api/dbmssql", moduleAuthMiddleware(serveAPIMSSQL))
	mux.HandleFunc("/module/api/log", moduleAuthMiddleware(serveLogAPI))           // module log i/o
	mux.HandleFunc("/module/api/ctl", moduleAuthMiddleware(serveModuleControlAPI)) // moudle control
	// tutorial api
	mux.HandleFunc("/api/tutorial/status", authMiddleware(TutorialStatusHandler))
	mux.HandleFunc("/api/tutorial/complete", authMiddleware(TutorialCompleteHandler))
	mux.HandleFunc("/api/tutorial/reset", authMiddleware(TutorialResetHandler))
	mux.HandleFunc("/api/tutorial/checkRole", TutorialCheckRoleHandler)

	// return handlers
	return mux
}

// start server
func StartServer() {
	defer dbConnect()()   // connect and start db
	startEnabledModules() // start modules where statusEnabled=1
	// go startModuleControl() <-chan [string]int // moduleName,signal (start,stop,restart)

	if mC.MSSQL.Enabled {
		// init MSSQL connection
		initMSSQL(
			mC.MSSQL.Host,
			mC.MSSQL.Port,
			mC.MSSQL.User,
			mC.MSSQL.Password,
			mC.MSSQL.Database,
		)
		defer closeMSSQL()
	}

	println("Starting webserver on: https://localhost:" + mC.ServerPort)                   // print access url
	err := http.ListenAndServeTLS(":"+mC.ServerPort, mC.TlsCert, mC.TlsKey, handlePaths()) // start https server
	if err != nil {
		log.Fatalln("Error starting webserver: " + err.Error()) // if error starting server, call log
	}
}
