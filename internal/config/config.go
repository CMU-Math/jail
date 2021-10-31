package config

//go:generate mkdir -p ../proto
//go:generate protoc -I../../nsjail --go_out ../proto --go_opt Mconfig.proto=/nsjail config.proto

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v6"
	"github.com/docker/go-units"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)
type size uint64

func (s *size) UnmarshalText(t []byte) error {
	v, err := units.RAMInBytes(string(t))
	*s = size(v)
	return err
}

type Config struct {
	Time       uint32   `env:"JAIL_TIME" envDefault:"20"`
	Conns      uint32   `env:"JAIL_CONNS"`
	ConnsPerIp uint32   `env:"JAIL_CONNS_PER_IP"`
	Id         string   `env:"JAIL_ID" envDefault:"NSJAIL"`
	Pids       uint64   `env:"JAIL_PIDS" envDefault:"5"`
	Mem        size     `env:"JAIL_MEM" envDefault:"5M"`
	Cpu        uint32   `env:"JAIL_CPU" envDefault:"100"`
	Pow        uint32   `env:"JAIL_POW"`
	Port       uint32   `env:"JAIL_PORT" envDefault:"5000"`
	Syscalls   []string `env:"JAIL_SYSCALLS"`
	TmpSize    size     `env:"JAIL_TMP_SIZE"`
}

const NsjailConfigPath = "/tmp/nsjail.cfg"
const CgroupV1Root = "/jail/cgroup/v1"

func (c *Config) SetConfig(msg *nsjail.NsJailConfig) {
	msg.Mode = nsjail.Mode_EXECVE.Enum()
	msg.TimeLimit = &c.Time
	msg.RlimitAsType = nsjail.RLimit_HARD.Enum()
	msg.RlimitCpuType = nsjail.RLimit_HARD.Enum()
	msg.RlimitFsizeType = nsjail.RLimit_HARD.Enum()
	msg.RlimitNofileType = nsjail.RLimit_HARD.Enum()
	msg.CgroupPidsMax = &c.Pids
	msg.CgroupMemMax = proto.Uint64(uint64(c.Mem))
	msg.CgroupCpuMsPerSec = &c.Cpu
	msg.Mount = []*nsjail.MountPt{{
		Src:    proto.String("/srv"),
		Dst:    proto.String("/"),
		IsBind: proto.Bool(true),
		Nodev:  proto.Bool(true),
		Nosuid: proto.Bool(true),
	}}
	msg.Hostname = proto.String("app")
	msg.Cwd = proto.String("/app")
	msg.ExecBin = &nsjail.Exe{
		Path: proto.String("/app/run"),
	}
	if c.Pow > 0 {
		msg.Bindhost = proto.String("127.0.0.1")
		msg.Port = proto.Uint32(c.Port + 1)
	} else {
		msg.Port = &c.Port
		msg.MaxConns = &c.Conns
		msg.MaxConnsPerIp = &c.ConnsPerIp
	}
	if c.TmpSize > 0 {
		msg.Mount = append(msg.Mount, &nsjail.MountPt{
			Dst:     proto.String("/tmp"),
			Fstype:  proto.String("tmpfs"),
			Rw:      proto.Bool(true),
			Options: proto.String(fmt.Sprintf("size=%d", c.TmpSize)),
			Nodev:   proto.Bool(true),
			Nosuid:  proto.Bool(true),
		})
	}
}

const tmpMountFlags = uintptr(unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME)

func DoMounts() error {
	if err := mountTmpFS("/tmp"); err != nil {
		return err
	}
	return nil
}

func mountTmpFS(path string) error {
	if err := unix.Mount("", path, "tmpfs", tmpMountFlags, ""); err != nil {
		return fmt.Errorf("mount tmpfs: %w", err)
	}
	return nil
}

func WriteConfig(msg *nsjail.NsJailConfig) error {
	content, err := prototext.Marshal(msg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(NsjailConfigPath, content, 0644); err != nil {
		return err
	}
	return nil
}

func GetConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env config: %w", err)
	}
	return cfg, nil
}
