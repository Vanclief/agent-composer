package controller

import (
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// BuiltinRecoveryRestarts is how often the watchdog restarts one crashed
// sandbox within an hour before it gives up (ADR 0017).
const BuiltinRecoveryRestarts = 3

// RecoverySettings is the recovery: block of the config file. Restarts is
// how many times an hour the watchdog restarts a crashed sandbox; 0 turns
// the watchdog's restarts off. Unset means the built-in.
type RecoverySettings struct {
	Restarts *int `mapstructure:"restarts" json:"restarts"`
}

// Recovery is the resolved recovery: block.
type Recovery struct {
	Restarts int
}

// Recovery returns the recovery settings: AGC_RECOVERY_RESTARTS overrides
// the config file, which overrides the built-in, like Defaults.
func (controller *Controller) Recovery() Recovery {
	return ResolveRecovery(controller.Config.Recovery)
}

// ResolveRecovery applies the environment and the built-in to configured.
// An unreadable or negative count is logged and the built-in used.
func ResolveRecovery(configured RecoverySettings) Recovery {
	restarts := BuiltinRecoveryRestarts
	if configured.Restarts != nil {
		restarts = *configured.Restarts
	}

	value := strings.TrimSpace(os.Getenv("AGC_RECOVERY_RESTARTS"))
	if value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			log.Warn().Str("value", value).Int("using", BuiltinRecoveryRestarts).Msg("unreadable AGC_RECOVERY_RESTARTS")
			return Recovery{Restarts: BuiltinRecoveryRestarts}
		}
		restarts = parsed
	}

	if restarts < 0 {
		log.Warn().Int("value", restarts).Int("using", BuiltinRecoveryRestarts).Msg("negative recovery restarts")
		restarts = BuiltinRecoveryRestarts
	}

	return Recovery{Restarts: restarts}
}
