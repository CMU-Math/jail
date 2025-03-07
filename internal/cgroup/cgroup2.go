package cgroup

import (
	"fmt"
	"os"

	"github.com/CMU-Math/grader/internal/privs"
	"github.com/CMU-Math/grader/internal/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

type cgroup2 struct{}

func (c *cgroup2) MountAndSetConfig(id string, msg *nsjail.NsJailConfig) error {
	dest := rootPath + "/unified"
	if err := unix.Mount("", dest, "cgroup2", mountFlags, ""); err != nil {
		return fmt.Errorf("mount cgroup2 to %s: %w", dest, err)
	}
	jailPath := dest + "/jail"
	if err := os.Mkdir(jailPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(jailPath+"/cgroup.procs", []byte("0"), 0); err != nil {
		return err
	}
	if err := os.WriteFile(dest+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(dest+"/cgroup.procs", privs.UserId, privs.UserId); err != nil {
		return err
	}
	runPath := dest + "/run"
	if err := os.Mkdir(runPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(runPath+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(runPath, privs.UserId, privs.UserId); err != nil {
		return err
	}

	c.setConfig(msg)

	return nil
}

func (c *cgroup2) setConfig(msg *nsjail.NsJailConfig) {
	msg.UseCgroupv2 = proto.Bool(true)
	msg.Cgroupv2Mount = proto.String(rootPath + "/unified/run")
}
