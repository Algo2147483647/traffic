package frontend

import "embed"

// Files contains the UI only; simulation state stays in the backend.
//
//go:embed *.html *.css *.js
var Files embed.FS
