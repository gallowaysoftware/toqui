package main

import (
	"embed"
	"io/fs"
)

//go:embed adminui
var adminRawFS embed.FS

func adminStaticFS() fs.FS {
	sub, _ := fs.Sub(adminRawFS, "adminui")
	return sub
}
