package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

func GetStaticFS() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}

func ParseTemplates() (*template.Template, error) {
	t, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return t, nil
}
