package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// StaticFS returns the embedded static file system rooted at static/.
func StaticFS() (fs.FS, error) {
	return fs.Sub(staticFiles, "static")
}

// MountStatic registers static assets on mux at /static/.
func MountStatic(mux *http.ServeMux) error {
	sub, err := StaticFS()
	if err != nil {
		return err
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	return nil
}
