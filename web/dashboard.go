package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed all:dist
var distEmbed embed.FS

// RegisterDashboardRoutes mounts the embedded SPA dashboard to chi router.
func RegisterDashboardRoutes(r chi.Router) {
	distFS, err := fs.Sub(distEmbed, "dist")
	if err != nil {
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	// Serve static files and fallback to index.html for SPA routing
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path == "" {
			req.URL.Path = "/"
			fileServer.ServeHTTP(w, req)
			return
		}

		// If file exists in dist, serve it
		if f, err := distFS.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, req)
			return
		}

		// Fallback to index.html for SPA client-side routes
		req.URL.Path = "/"
		fileServer.ServeHTTP(w, req)
	})
}
