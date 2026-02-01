package embed

import "embed"

// FrontendFS holds the embedded frontend files
// This will be empty during development builds
//
//go:embed all:frontend
var FrontendFS embed.FS
