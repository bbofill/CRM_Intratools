package goworkers

// redirect from controller webserver path to module local port

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var mOdUlEcOnF Module

func handleProxy(w http.ResponseWriter, r *http.Request) {

	var internalURL *url.URL
	var err error
	var remainder string
	var transport http.RoundTripper

	targetPath := strings.TrimPrefix(r.URL.Path, "/module/proxy/")

	// validate in here

	parts := strings.SplitN(targetPath, "/", 2)
	firstSegment := parts[0]
	if len(parts) > 1 {
		remainder = "/" + parts[1]
	} else {
		remainder = "/"
	}
	if mC.Development {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	// method modules.validateConfig() called here
	mod, ok := mOdUlEcOnF[firstSegment]
	if !ok {
		http.Error(w, "error, undefined module id", http.StatusBadRequest)
		return
	}
	internalURL, err = url.Parse("https://localhost:" + mod.Port + "?key=" + mod.ApiKey)
	if err != nil {
		http.Error(w, "error parsing URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	r.URL.Path = remainder
	proxy := httputil.NewSingleHostReverseProxy(internalURL)
	proxy.Transport = transport
	proxy.ServeHTTP(w, r)
}
