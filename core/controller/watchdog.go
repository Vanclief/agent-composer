package controller

import (
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// Built-in watchdog thresholds (ADR 0013).
const (
	BuiltinWatchdogSilence = 15 * time.Minute
	BuiltinWatchdogReview  = 2 * time.Hour
)

// WatchdogSettings is the watchdog: block of the config file, durations
// such as "15m": how long an agent may be silent, and a task wait in
// review, before agc tells the owner. "0" turns that check off.
type WatchdogSettings struct {
	Silence string `mapstructure:"silence" json:"silence"`
	Review  string `mapstructure:"review" json:"review"`
}

// Watchdog is the resolved watchdog: block.
type Watchdog struct {
	Silence time.Duration
	Review  time.Duration
}

// Watchdog returns the watchdog thresholds: AGC_WATCHDOG_SILENCE and
// AGC_WATCHDOG_REVIEW override the config file, which overrides the
// built-ins, like Defaults.
func (controller *Controller) Watchdog() Watchdog {
	return ResolveWatchdog(controller.Config.Watchdog)
}

// ResolveWatchdog applies the environment and the built-ins to
// configured. An unreadable duration is logged and the built-in used.
func ResolveWatchdog(configured WatchdogSettings) Watchdog {
	return Watchdog{
		Silence: duration("silence", firstSet(os.Getenv("AGC_WATCHDOG_SILENCE"), configured.Silence), BuiltinWatchdogSilence),
		Review:  duration("review", firstSet(os.Getenv("AGC_WATCHDOG_REVIEW"), configured.Review), BuiltinWatchdogReview),
	}
}

func duration(key string, value string, builtin time.Duration) time.Duration {
	if value == "" {
		return builtin
	}

	if value == "0" {
		return 0
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		log.Warn().Str("watchdog", key).Str("value", value).Dur("using", builtin).Msg("unreadable watchdog duration")
		return builtin
	}

	return parsed
}
