package web

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:assets
var distFS embed.FS

// Handler serves the embedded Angular build under basePath.
func Handler(basePath string) (http.Handler, error) {
	sub, err := fs.Sub(distFS, "assets/dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	return newHandler(sub, basePath)
}

func newHandler(files fs.FS, basePath string) (http.Handler, error) {
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read embedded frontend (run npm run build first): %w", err)
	}
	const base = `<base href="/">`
	if !bytes.Contains(index, []byte(base)) {
		return nil, fmt.Errorf("embedded index.html is missing %s", base)
	}
	index = bytes.Replace(index, []byte(base), []byte(`<base href="`+html.EscapeString(basePath)+`/">`), 1)
	fileServer := http.FileServer(http.FS(files))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "." && name != "index.html" {
			if info, err := fs.Stat(files, name); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}), nil
}
