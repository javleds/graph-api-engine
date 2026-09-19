package playground

import (
	"embed"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /playground/assets/{file}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		contentType := "text/javascript; charset=utf-8"
		switch name {
		case "playground.css":
			contentType = "text/css; charset=utf-8"
		case "playground.js", "json-viewer.js":
		default:
			http.NotFound(w, r)
			return
		}
		body, err := assets.ReadFile("assets/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		securityHeaders(w)
		w.Header().Set("Content-Type", contentType)
		w.Write(body)
	})
	mux.HandleFunc("GET /playground", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playground" {
			http.NotFound(w, r)
			return
		}
		securityHeaders(w)
		body, _ := assets.ReadFile("assets/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(body)
	})
}

func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
