package web

import "embed"

// DistFS contains the production SPA bundle served by the REST interface.
//
//go:embed all:dist
var DistFS embed.FS
