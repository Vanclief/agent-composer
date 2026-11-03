package controller

import (
	"net"
	"os"
	"strings"
)

// BuiltinAppHost is where agc rest listens when neither the config file
// nor the environment sets a host: loopback only, reached from elsewhere
// over an SSH tunnel (rule rig-services-bind-localhost).
const BuiltinAppHost = "127.0.0.1"

// AppHost returns the address agc rest listens on: AGC_APP_HOST overrides
// app.host in the config file, which overrides the built-in 127.0.0.1. An
// empty app.host means every interface.
func (controller *Controller) AppHost() string {
	return ResolveAppHost(controller.Config.App)
}

// ListenAddress is the host:port agc rest listens on.
func (controller *Controller) ListenAddress() string {
	return net.JoinHostPort(controller.AppHost(), controller.Config.App.Port)
}

// ResolveAppHost applies the environment and the built-in fallback to
// configured. An empty AGC_APP_HOST counts as unset.
func ResolveAppHost(configured AppSettings) string {
	env := strings.TrimSpace(os.Getenv("AGC_APP_HOST"))
	if env != "" {
		return env
	}

	if configured.Host == nil {
		return BuiltinAppHost
	}

	return strings.TrimSpace(*configured.Host)
}
