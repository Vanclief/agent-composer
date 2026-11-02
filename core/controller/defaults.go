package controller

import (
	"os"
	"strings"
)

// Built-in task defaults, used when neither the config file nor the
// environment sets one (ADR 0009 item 1). The owner has no built-in value.
const (
	BuiltinHarness = "claude_code"
	BuiltinModel   = "claude-opus-5-5"
)

// DefaultsSettings is the defaults: block of the config file: who works on
// a task, with which model, and who owns it, when the request leaves them
// empty.
type DefaultsSettings struct {
	Harness string `mapstructure:"harness" json:"harness"`
	Model   string `mapstructure:"model" json:"model"`
	Owner   string `mapstructure:"owner" json:"owner"`
}

// Defaults returns the task defaults: AGC_DEFAULT_HARNESS, AGC_DEFAULT_MODEL
// and AGC_DEFAULT_OWNER override the config file, which overrides the
// built-in harness and model. An unset owner stays empty.
func (controller *Controller) Defaults() DefaultsSettings {
	return ResolveDefaults(controller.Config.Defaults)
}

// ResolveDefaults applies the environment and the built-in fallbacks to
// configured.
func ResolveDefaults(configured DefaultsSettings) DefaultsSettings {
	return DefaultsSettings{
		Harness: firstSet(os.Getenv("AGC_DEFAULT_HARNESS"), configured.Harness, BuiltinHarness),
		Model:   firstSet(os.Getenv("AGC_DEFAULT_MODEL"), configured.Model, BuiltinModel),
		Owner:   firstSet(os.Getenv("AGC_DEFAULT_OWNER"), configured.Owner),
	}
}

func firstSet(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}
