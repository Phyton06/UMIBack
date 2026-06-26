package api

import (
	"bytes"
	"io"
	"net/http"
)

// MaxBodySize is the maximum request body size (1 MB).
const MaxBodySize = 1 << 20

// RequireJSON ensures the request has Content-Type: application/json
// and limits the body size to MaxBodySize.
func RequireJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "DELETE" {
			next(w, r)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		// Read body with explicit size limit (returns 413 when exceeded)
		body, err := io.ReadAll(io.LimitReader(r.Body, MaxBodySize+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(body) > MaxBodySize {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next(w, r)
	}
}

// CORS adds basic CORS headers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
