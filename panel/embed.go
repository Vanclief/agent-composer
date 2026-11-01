// Package panel embeds the control panel build served at /panel/. The
// React source lives next to this file; `make panel` writes dist/. Until
// then dist holds only .keep and agc answers /panel/ with 503.
package panel

import "embed"

// Dist holds the Vite build, with index.html at dist/index.html.
//
//go:embed all:dist
var Dist embed.FS
