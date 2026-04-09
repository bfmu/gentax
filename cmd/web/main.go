// cmd/web serves the React owner panel as embedded static files.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

//go:embed dist
var distFS embed.FS

func main() {
	port := 3000
	if v := os.Getenv("WEB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		fmt.Fprintf(os.Stderr, "web: embed dist: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(sub))

	// Serve static assets directly; fall back to index.html for SPA routing.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file; if not found, return index.html.
		f, err := sub.Open(r.URL.Path[1:])
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", port)
	slog.Info("gentax web starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "web: server error: %v\n", err)
		os.Exit(1)
	}
}
