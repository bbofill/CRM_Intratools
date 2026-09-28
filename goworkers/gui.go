package goworkers

//frontend serving templates

import (
	"fmt"
	"net/http"
)

// serve main panel (access to modules)
func serveMain(w http.ResponseWriter, r *http.Request) {
	var user, role string
	if c, err := r.Cookie(cOoKiEnAmE); err == nil {
		if claims, ok := parseAuthCookie(c.Value, nil); ok {
			user = getUsernameByHash(claims.Sub)
			role = claims.Role

		}
	}

	name, surname := getName(user)

	data := TemplateData{
		"NAME":  fmt.Sprintf("%s %s", name, surname),
		"UNAME": user,
		"ROLE":  role,
	}
	rendered := parseHTMLTemplate(mC.MainFilePath, data)
	fmt.Fprint(w, rendered)
}

// serve style dir
func serveStyleDir(w http.ResponseWriter, r *http.Request) {
	path := "/assets/"
	fs := http.FileServer(http.Dir(mC.GUIAssetsDir))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 0 gui dir
func serveModule0GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module0/"
	fs := http.FileServer(http.Dir(mC.Module0GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 1 gui dir
func serveModule1GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module1/"
	fs := http.FileServer(http.Dir(mC.Module1GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 2 gui dir
func serveModule2GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module2/"
	fs := http.FileServer(http.Dir(mC.Module2GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 3 gui dir
func serveModule3GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module3/"
	fs := http.FileServer(http.Dir(mC.Module3GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 4 gui dir
func serveModule4GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module4/"
	fs := http.FileServer(http.Dir(mC.Module4GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 5 gui dir
func serveModule5GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module5/"
	fs := http.FileServer(http.Dir(mC.Module5GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 6 gui dir
func serveModule6GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module6/"
	fs := http.FileServer(http.Dir(mC.Module6GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 7 gui dir
func serveModule7GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module7/"
	fs := http.FileServer(http.Dir(mC.Module7GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 8 gui dir
func serveModule8GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module8/"
	fs := http.FileServer(http.Dir(mC.Module8GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 9 gui dir
func serveModule9GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module9/"
	fs := http.FileServer(http.Dir(mC.Module9GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 10 gui dir
func serveModule10GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module10/"
	fs := http.FileServer(http.Dir(mC.Module10GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}

// serve module 11 gui dir
func serveModule11GUI(w http.ResponseWriter, r *http.Request) {
	path := "/module/gui/module11/"
	fs := http.FileServer(http.Dir(mC.Module11GUIPath))
	http.StripPrefix(path, fs).ServeHTTP(w, r)
}
