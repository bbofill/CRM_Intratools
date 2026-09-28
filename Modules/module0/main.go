package main

import (
	"module0.local/module0workers"
)

const cOnFiGfIlE = "../../Config/config.json"

func init() {
	module0workers.LoadConfigData(cOnFiGfIlE)
}

func main() {
	println("Starting module0...")
	module0workers.StartServer()
}

/*

ToDo:

auth routes
logs
hashes, logchain.
delete user


*/
