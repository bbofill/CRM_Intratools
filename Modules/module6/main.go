package main

import (
	"module6.local/module6workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module6workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module6workers.StartServer()
}
