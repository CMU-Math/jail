package cgroup

import (
	"fmt"
	"os"
	"path"

	"github.com/redpwn/jail/internal/config"
	"github.com/redpwn/jail/internal/privs"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

type cgroup1Entry struct {
	controllers string
	parent      string
}

type cgroup1 struct {
	pids *cgroup1Entry
	mem  *cgroup1Entry
	cpu  *cgroup1Entry
}

func (c *cgroup1) Init() error {
	if err := mountCgroup1Entry("pids", c.pids); err != nil {
		return err
	}
	if err := mountCgroup1Entry("mem", c.mem); err != nil {
		return err
	}
	if err := mountCgroup1Entry("cpu", c.cpu); err != nil {
		return err
	}
	return nil
}

func mountCgroup1Entry(name string, entry *cgroup1Entry) error {
	dest := path.Join(config.CgroupV1Root, name)
	if err := unix.Mount("", dest, "cgroup", mountFlags, entry.controllers); err != nil {
		return fmt.Errorf("mount cgroup1 %s to %s: %w", entry.controllers, dest, err)
	}
	if err := os.Chmod(dest, 0755); err != nil {
		return err
	}
	return nil
}

func createCgroup1Delegate(id string, name string, entry *cgroup1Entry) error {
	delegated := path.Join(config.CgroupV1Root, name, entry.parent, id)
	if err := os.Mkdir(delegated, 0755); err != nil {
		return err
	}
	if err := os.Chown(delegated, privs.UserId, privs.UserId); err != nil {
		return err
	}
	return nil
}

func (c *cgroup1) MountAndSetConfig(id string, msg *nsjail.NsJailConfig) error {
	if err := createCgroup1Delegate(id, "pids", c.pids); err != nil {
		return err
	}
	if err := createCgroup1Delegate(id, "mem", c.mem); err != nil {
		return err
	}
	if err := createCgroup1Delegate(id, "cpu", c.cpu); err != nil {
		return err
	}

	c.setConfig(id, msg)

	return nil
}

func (c *cgroup1) setConfig(id string, msg *nsjail.NsJailConfig) {
	msg.CgroupPidsMount = proto.String(config.CgroupV1Root + "/pids")
	tmp := path.Join(c.pids.parent, id)
	msg.CgroupPidsParent = &tmp

	msg.CgroupMemMount = proto.String(config.CgroupV1Root + "/mem")
	tmp2 := path.Join(c.mem.parent, id)
	msg.CgroupMemParent = &tmp2

	msg.CgroupCpuMount = proto.String(config.CgroupV1Root + "/cpu")
	tmp3 := path.Join(c.cpu.parent, id)
	msg.CgroupCpuParent = &tmp3
}
