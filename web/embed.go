// Package web embeds the built Console (see web/README.md for the build
// step) so the CLI can serve an interactive viewer without a Node.js
// runtime dependency for end users. Contributors touching web/src must run
// `npm run build` and commit the refreshed web/dist before the Go binary
// will pick up their changes — go:embed reads whatever is on disk at Go
// build time, not at Console-source-edit time.
package web

import "embed"

//go:embed dist
var ConsoleFS embed.FS
