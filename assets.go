package assets

import "embed"

// FS contains the files required by the server at runtime.
//
//go:embed static sql/migrations
var FS embed.FS
