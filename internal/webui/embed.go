package webui

import "embed"

// Dist contains the built Web UI static files. Build the Web UI before
// compiling the router: cd internal/webui && npm run build
// If dist/ doesn't exist, the embed will be empty and the router serves
// a "Web UI not built" message instead.
//
//go:embed all:dist
var Dist embed.FS
