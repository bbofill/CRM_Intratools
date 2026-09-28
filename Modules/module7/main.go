package main

import (
	"module7.local/module7workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module7workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module7workers.StartServer()
}
