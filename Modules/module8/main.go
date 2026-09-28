package main

import (
	"module8.local/module8workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module8workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module8workers.StartServer()
}
