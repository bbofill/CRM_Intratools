package main

import (
	"module1.local/module1workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module1workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module1workers.StartServer()
}
