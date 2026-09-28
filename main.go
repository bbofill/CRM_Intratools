package main

import (
	"crmintratools.local/goworkers"
)

const cOnFiGfIlE string = "Config/config.json" // global config file definition

// set controller pred. settings
func init() {
	goworkers.SetConfigFile(cOnFiGfIlE) // set general config
	goworkers.SetMainCookie()           // (tmp?) set cookies name // get size and gen random name
	goworkers.SetModuleConf()           // fill module config obj
}

// starting program
func main() {
	println("Hello, crmintratools...") // if everything is ok, go
	goworkers.StartServer()            // start web server
	select {}                          // this should be a trap, for now wait to CTL+C
}

//TODO:   hashes, api, logs, module
