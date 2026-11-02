//go:build !unix

package schedules

import (
	"os/exec"
)

// killGroup leaves the default: the context kills sh only.
func killGroup(cmd *exec.Cmd) {
}
