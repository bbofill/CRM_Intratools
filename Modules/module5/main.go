package main

import (
	"module5.local/module5workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module5workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module5workers.StartServer()
}
