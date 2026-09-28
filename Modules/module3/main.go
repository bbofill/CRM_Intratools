package main

import (
	"module3.local/module3workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module3workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module3workers.StartServer()
}
