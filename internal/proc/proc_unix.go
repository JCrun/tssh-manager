//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

func hide(*exec.Cmd) {}

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
