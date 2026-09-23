package main

import (
	"module4.local/module4workers"
)

var cOnFiGfIlE string = "../../Config/config.json"

// in go this func (init) is executed before main, usually used to fill env vars
func init() {
	module4workers.SetConfig(cOnFiGfIlE)
}

// start server
func main() {
	module4workers.StartServer()
}
