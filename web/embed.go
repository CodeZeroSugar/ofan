package web

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var staticFS embed.FS

func GetStaticFS() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}
