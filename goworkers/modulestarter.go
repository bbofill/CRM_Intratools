package goworkers

import (
	"fmt"
	"os/exec"
	"sync"
)

var mOdUlEcTl chan ModuleAction
var moduleStatuses = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

func startEnabledModules() {
	mOdUlEcTl = make(chan ModuleAction)

	go func() {
		running := make(map[string]*exec.Cmd)
		for name, cfg := range mOdUlEcOnF {
			if !cfg.Enabled {
				continue
			}
			cmd := exec.Command("sh", "-c", cfg.StartCommand)
			if err := cmd.Start(); err != nil {
				AddControllerLog(fmt.Sprintf("%s: cannot start (%v)", name, err), 0)
				continue
			}
			running[name] = cmd

			moduleStatuses.Lock()
			moduleStatuses.m[name] = "running"
			moduleStatuses.Unlock()

			AddControllerLog(fmt.Sprintf("%s started (pid %d)", name, cmd.Process.Pid), 0)
		}

		for order := range mOdUlEcTl {
			switch order.Action {
			case "stop":
				if cmd, ok := running[order.Name]; ok {
					_ = cmd.Process.Kill()
					delete(running, order.Name)
					moduleStatuses.Lock()
					moduleStatuses.m[order.Name] = "stopped"
					moduleStatuses.Unlock()
				}
			case "start":
				if _, ok := running[order.Name]; ok {
					continue
				}
				if cfg, ok := mOdUlEcOnF[order.Name]; ok {
					cmd := exec.Command("sh", "-c", cfg.StartCommand)
					if err := cmd.Start(); err != nil {
						AddControllerLog(fmt.Sprintf("%s: cannot start (%v)", order.Name, err), 2)
						continue
					}
					running[order.Name] = cmd
					moduleStatuses.Lock()
					moduleStatuses.m[order.Name] = "running"
					moduleStatuses.Unlock()
				}
			case "restart":
				if cmd, ok := running[order.Name]; ok {
					_ = cmd.Process.Kill()
					delete(running, order.Name)
					moduleStatuses.Lock()
					moduleStatuses.m[order.Name] = "stopped"
					moduleStatuses.Unlock()
				}
				if cfg, ok := mOdUlEcOnF[order.Name]; ok {
					cmd := exec.Command("sh", "-c", cfg.StartCommand)
					if err := cmd.Start(); err != nil {
						AddControllerLog(fmt.Sprintf("%s: cannot start (%v)", order.Name, err), 2)
						break
					}
					running[order.Name] = cmd
					moduleStatuses.Lock()
					moduleStatuses.m[order.Name] = "running"
					moduleStatuses.Unlock()
				}
			}
		}
	}()
}
