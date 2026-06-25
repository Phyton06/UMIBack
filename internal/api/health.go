// Package api proporciona los handlers HTTP de la API REST.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Handler retorna un http.HandlerFunc que verifica el estado de la aplicación
// y la conectividad con la base de datos.
//
// Responde con:
//   - 200 OK y {"status":"ok","database":"connected"} si la DB responde
//   - 503 Service Unavailable y {"status":"degraded","database":"disconnected"} si no
func Handler(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := pool.Ping(ctx); err != nil {
			slog.Warn("health check: base de datos no disponible",
				"error", err,
				"remote", r.RemoteAddr,
			)
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status":   "degraded",
				"database": "disconnected",
			})
			return
		}

		slog.Info("health check exitoso",
			"remote", r.RemoteAddr,
		)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	}
}
