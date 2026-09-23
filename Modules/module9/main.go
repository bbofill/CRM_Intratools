package main

import (
	"module9.local/module9workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module9workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module9workers.StartServer()
}
