package main

import (
	"module2.local/module2workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module2workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module2workers.StartServer()
}
