//go:build unix

package schedules

import (
	"os/exec"
	"syscall"
)

// killGroup runs cmd in its own process group and kills the whole group
// when its context ends, so sh -c does not leave its children running.
func killGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
