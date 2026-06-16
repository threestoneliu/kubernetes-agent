package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web_dist
var webDistFS embed.FS

// staticHandler serves the embedded SPA. Any request that resolves
// to an existing file under web_dist/ is served with a long-lived
// cache header (Vite emits content-hashed filenames, so they are
// safe to cache forever). Missing files fall back to index.html for
// client-side routing, and index.html itself is served with
// no-cache so deploys take effect immediately.
func staticHandler() http.Handler {
	sub, err := fs.Sub(webDistFS, "web_dist")
	if err != nil {
		// Should never happen: web_dist is the embed root.
		panic(err)
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(r.URL.Path, "/")
		if upath == "" {
			upath = "index.html"
		} else {
			if _, err := fs.Stat(sub, upath); err != nil {
				// SPA fallback
				upath = "index.html"
			}
		}
		setCacheHeaders(w, upath)
		r2 := r.Clone(r.Context())
		// When the target is index.html, point the FileServer at
		// the directory root so it resolves the index without
		// redirecting (FileServer 301s /index.html → /index.html/
		// because it treats it as an index candidate).
		if upath == "index.html" {
			r2.URL.Path = "/"
		} else {
			r2.URL.Path = "/" + upath
		}
		files.ServeHTTP(w, r2)
	})
}

func setCacheHeaders(w http.ResponseWriter, filePath string) {
	base := path.Base(filePath)
	if filePath == "index.html" || base == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}