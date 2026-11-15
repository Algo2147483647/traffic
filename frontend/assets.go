package frontend

import "embed"

// Files is the compiled React application. Build with npm run frontend:build.
//go:embed dist
var Files embed.FS
