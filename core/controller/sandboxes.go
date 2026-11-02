package controller

import (
	"os"
	"strings"
	"time"

	"github.com/vanclief/ez"
)

// BuiltinKeepDone is how long a done task's stopped sandbox is kept when
// neither the config file nor the environment sets keep_done (ADR 0008
// "Garbage collection").
const BuiltinKeepDone = 24 * time.Hour

// SandboxesSettings is the sandboxes: block of the config file.
type SandboxesSettings struct {
	// KeepDone is how long after its task is done a stopped sandbox is
	// kept before agc rest deletes it, a Go duration such as 24h; 0
	// disables the collector.
	KeepDone string `mapstructure:"keep_done" json:"keep_done"`
}

// KeepDone returns the sandboxes' grace period: AGC_SANDBOX_KEEP_DONE
// overrides the config file, which overrides the built-in 24h. 0 means
// the collector is off.
func (controller *Controller) KeepDone() (time.Duration, error) {
	return ResolveKeepDone(controller.Config.Sandboxes)
}

// ResolveKeepDone applies the environment and the built-in fallback to
// configured.
func ResolveKeepDone(configured SandboxesSettings) (time.Duration, error) {
	value := firstSet(os.Getenv("AGC_SANDBOX_KEEP_DONE"), configured.KeepDone)
	if value == "" {
		return BuiltinKeepDone, nil
	}

	return ParseKeep(value)
}

// ParseKeep parses a grace period: a Go duration such as 24h or 90m, or a
// bare 0. A negative one is refused.
func ParseKeep(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "0" {
		return 0, nil
	}

	keep, err := time.ParseDuration(value)
	if err != nil {
		return 0, ez.New(ez.EINVALID, "keep "+value+" is not a duration such as 24h", err)
	}

	if keep < 0 {
		return 0, ez.New(ez.EINVALID, "keep "+value+" is negative", nil)
	}

	return keep, nil
}
