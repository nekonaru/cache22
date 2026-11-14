// Package web embeds the single-page frontend into the binary.
package web

import _ "embed"

//go:embed index.html
var Page []byte
