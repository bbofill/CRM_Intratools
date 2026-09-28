package main

import (
	"module10.local/module10workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module10workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module10workers.StartServer()
}
