package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// RiderStats retorna estadísticas agregadas del rider autenticado.
//
// Request:  GET /rider/stats
// Response: 200 {total_trips, completed_trips, total_spent, total_distance_km, member_since}
// Errors:   401, 403
func RiderStats(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		var memberSince time.Time
		err = pool.QueryRow(r.Context(),
			`SELECT created_at FROM users WHERE id = $1`, userID,
		).Scan(&memberSince)
		if err != nil {
			slog.Error("rider stats: member since query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		var totalTrips, completedTrips int64
		var totalSpent, totalDistanceKm float64
		err = pool.QueryRow(r.Context(),
			`SELECT
				COUNT(*)::bigint,
				COUNT(*) FILTER (WHERE status = 'COMPLETED')::bigint,
				COALESCE(SUM(fare), 0)::float8,
				COALESCE(SUM(ST_Distance(pickup_location::geography, dropoff_location::geography)) / 1000.0, 0)::float8
			 FROM rides WHERE passenger_id = $1`,
			userID,
		).Scan(&totalTrips, &completedTrips, &totalSpent, &totalDistanceKm)
		if err != nil {
			slog.Error("rider stats: aggregate query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"total_trips":       totalTrips,
			"completed_trips":   completedTrips,
			"total_spent":       totalSpent,
			"total_distance_km": totalDistanceKm,
			"member_since":      memberSince,
		})
	}
}

// RiderRides retorna el historial paginado de viajes completados del rider.
//
// Query params: limit (default 10, max 100), offset (default 0)
// Request:  GET /rider/rides?limit=10&offset=0
// Response: 200 {rides: [...], total: N}
// Errors:   400 invalid params, 401, 403
func RiderRides(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			l, pErr := strconv.Atoi(limitStr)
			if pErr != nil || l <= 0 || l > 100 {
				writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
				return
			}
			limit = l
		}

		offset := 0
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			o, pErr := strconv.Atoi(offsetStr)
			if pErr != nil || o < 0 {
				writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
				return
			}
			offset = o
		}

		// Total count for pagination metadata
		var total int64
		err = pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM rides WHERE passenger_id = $1 AND status = 'COMPLETED'`,
			userID,
		).Scan(&total)
		if err != nil {
			slog.Error("rider rides: count query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		rows, err := pool.Query(r.Context(),
			`SELECT `+rideCols+` FROM rides WHERE passenger_id = $1 AND status = 'COMPLETED' ORDER BY completed_at DESC LIMIT $2 OFFSET $3`,
			userID, limit, offset,
		)
		if err != nil {
			slog.Error("rider rides: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer rows.Close()

		rides := make([]map[string]any, 0)
		for rows.Next() {
			row, scanErr := scanRide(rows)
			if scanErr != nil {
				slog.Error("rider rides: scan", "error", scanErr)
				writeError(w, http.StatusInternalServerError, "internal error")
				rows.Close()
				return
			}
			rides = append(rides, rideToJSON(row))
		}
		if rows.Err() != nil {
			slog.Error("rider rides: rows iteration", "error", rows.Err())
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"rides": rides,
			"total": total,
		})
	}
}
