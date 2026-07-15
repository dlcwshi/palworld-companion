package web

import "embed"

// Assets contains the production frontend build. The committed placeholder keeps
// Go builds working before the first frontend build.
//
//go:embed all:dist
var Assets embed.FS
