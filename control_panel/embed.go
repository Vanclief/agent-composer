// Package controlpanel embeds the static control panel served at /panel/.
package controlpanel

import "embed"

// FS holds every file in this directory, index.html at its root.
//
//go:embed *
var FS embed.FS
