package main

import (
	"fmt"
	"os"

	"github.com/redpwn/jail/internal/cgroup"
	"github.com/redpwn/jail/internal/config"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"github.com/redpwn/jail/internal/server"
)

func run() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	if len(os.Args) > 1 {
		if os.Args[1] == "proxy" {
			return server.RunProxy(cfg)
		} else if os.Args[1] == "driver" {
			if err = config.DoMounts(); err != nil {
				return err
			}

			return server.RunDriver(cfg)
		}
	}

	cgroup, err := cgroup.ReadCgroup()
	if err != nil {
		return err
	}
	msg := &nsjail.NsJailConfig{}
	if err := cgroup.MountAndSetConfig(cfg.Id, msg); err != nil {
		return fmt.Errorf("delegate cgroup: %w", err)
	}
	cfg.SetConfig(msg)
	if err := config.WriteConfig(msg); err != nil {
		return err
	}
	if err := config.MountDev(); err != nil {
		return err
	}
	if err := config.RunHook(); err != nil {
		return err
	}
	return server.ExecServer(cfg)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
