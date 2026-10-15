package templates

import "embed"

// FS embeds prompt template files.
//
//go:embed *.tmpl
var FS embed.FS
