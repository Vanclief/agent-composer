package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vanclief/ez"
)

// The Go SDK can only hand secret values to the runtime inline, which stores
// them in the sandbox config. The msb CLI reads them from its own environment
// and stores a reference, so create and start go through it
// (docs/decisions/0004-secrets-are-placeholders-bound-to-hosts.md).

// createArgs is the msb argv that creates spec. It holds secret names and
// hosts, never values.
func createArgs(spec Spec) []string {
	args := []string{
		"create", spec.Image,
		"--name", spec.Name,
		"-c", fmt.Sprint(spec.CPUs),
		"-m", fmt.Sprintf("%dM", spec.MemoryMiB),
		"-u", spec.User,
	}
	for _, k := range sortedKeys(spec.Env) {
		args = append(args, "-e", k+"="+spec.Env[k])
	}
	for _, k := range sortedKeys(spec.Labels) {
		args = append(args, "--label", k+"="+spec.Labels[k])
	}
	for _, secret := range spec.Secrets {
		args = append(args, "--secret", secret.Name+"@"+strings.Join(secret.Hosts, ","))
	}
	for _, v := range spec.Volumes {
		flag := "--mount-named"
		if v.HostPath {
			flag = "--mount-dir"
		}
		options := []string{}
		if !v.ReadWrite {
			options = append(options, "ro")
		}
		if v.QuotaMiB != 0 {
			options = append(options, "quota="+FormatSize(v.QuotaMiB))
		}
		mount := v.Source + ":" + v.Dest
		if len(options) > 0 {
			mount += ":" + strings.Join(options, ",")
		}
		args = append(args, flag, mount)
	}
	if spec.DiskDest != "" {
		args = append(args, "--mount-owned", spec.DiskDest+":kind=disk,size="+FormatSize(spec.DiskMiB))
	}
	if spec.VsockPath != "" {
		args = append(args, "--vsock", fmt.Sprintf("%s:%d", spec.VsockPath, spec.VsockPort))
	}
	return args
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// msbPath finds the msb binary: $MSB_PATH, then PATH, then the two install
// locations microsandbox uses.
func msbPath() (string, error) {
	var candidates []string
	envPath := os.Getenv("MSB_PATH")
	if envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates, "msb")
	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "msb"), filepath.Join(home, ".microsandbox", "bin", "msb"))
	}
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", ez.New(ez.EUNAVAILABLE, "msb is not installed: tried "+strings.Join(candidates, ", "), nil)
}

// runMsb runs msb with args. The secret values are added to the child's
// environment only. A failed command's stderr becomes the error message.
func runMsb(ctx context.Context, secrets []Secret, args ...string) error {
	path, err := msbPath()
	if err != nil {
		return ez.Wrap(err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = os.Environ()
	for _, secret := range secrets {
		cmd.Env = append(cmd.Env, secret.Name+"="+secret.Value)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = "msb " + args[0] + " failed: " + exitErr.Error()
		}
		return ez.New(ez.EINTERNAL, message, nil)
	}
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot run msb "+args[0], err)
	}
	return nil
}
