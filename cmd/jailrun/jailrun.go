package main

import (
	"fmt"
	"os"

	"github.com/CMU-Math/grader/internal/cgroup"
	"github.com/CMU-Math/grader/internal/config"
	"github.com/CMU-Math/grader/internal/proto/nsjail"
	"github.com/CMU-Math/grader/internal/server"
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

			return server.RunGRPC()
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
	if err := server.ExecServer(cfg); err != nil {
		return err
	}
	return nil
	//return cgroup.Cleanup()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
