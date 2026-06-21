package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// UpdateLocation actualiza la ubicación GPS del conductor autenticado.
//
// La ubicación se almacena como GEOMETRY(Point, 4326) en drivers.location.
// Solo conductores con available = true persisten el cambio; los no disponibles
// reciben 200 igual (no-op) para no filtrar estado interno.
//
// Request:  PATCH /drivers/location  {lon, lat}
// Response: 200 {"message": "location updated"}
// Errors:   400 invalid/missing coords, 401, 403
func UpdateLocation(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		driverID, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		var body struct {
			Lon *float64 `json:"lon"`
			Lat *float64 `json:"lat"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Lon == nil || body.Lat == nil {
			writeError(w, http.StatusBadRequest, "lon and lat are required")
			return
		}

		if *body.Lon < -180 || *body.Lon > 180 {
			writeError(w, http.StatusBadRequest, "lon must be between -180 and 180")
			return
		}
		if *body.Lat < -90 || *body.Lat > 90 {
			writeError(w, http.StatusBadRequest, "lat must be between -90 and 90")
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE drivers SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326), updated_at = now() WHERE id = $3 AND available = true`,
			*body.Lon, *body.Lat, driverID,
		)
		if err != nil {
			slog.Error("update location: exec", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// ponytail: RowsAffected ignorado a propósito — 200 siempre evita
		// filtrar el estado de available al conductor.
		_ = tag

		writeJSON(w, http.StatusOK, map[string]string{"message": "location updated"})
	}
}

// NearbyDrivers busca conductores disponibles dentro de un radio desde un punto.
//
// Los resultados incluyen solo conductores con available = true.
// El radio por omisión es 5000 metros, máximo 50000.
//
// Request:  GET /drivers/nearby?lon=X&lat=Y&radius=Z
// Response: 200 {"drivers": [{"id": "...", "name": "...", "lon": ..., "lat": ...}]}
// Errors:   400 missing/invalid params, 401, 403
func NearbyDrivers(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		_, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		lonStr := r.URL.Query().Get("lon")
		latStr := r.URL.Query().Get("lat")
		if lonStr == "" || latStr == "" {
			writeError(w, http.StatusBadRequest, "lon and lat are required")
			return
		}

		lon, err := strconv.ParseFloat(lonStr, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lon")
			return
		}
		lat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lat")
			return
		}
		if lon < -180 || lon > 180 {
			writeError(w, http.StatusBadRequest, "lon must be between -180 and 180")
			return
		}
		if lat < -90 || lat > 90 {
			writeError(w, http.StatusBadRequest, "lat must be between -90 and 90")
			return
		}

		radius := 5000.0
		if radiusStr := r.URL.Query().Get("radius"); radiusStr != "" {
			r, pErr := strconv.ParseFloat(radiusStr, 64)
			if pErr != nil {
				writeError(w, http.StatusBadRequest, "invalid radius")
				return
			}
			if r <= 0 || r > 50000 {
				writeError(w, http.StatusBadRequest, "radius must be between 1 and 50000")
				return
			}
			radius = r
		}

		rows, err := pool.Query(r.Context(),
			`SELECT id, name, ST_X(location)::float8 AS lon, ST_Y(location)::float8 AS lat
			 FROM drivers
			 WHERE available = true
			   AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326), $3)`,
			lon, lat, radius,
		)
		if err != nil {
			slog.Error("nearby drivers: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer rows.Close()

		type driverJSON struct {
			ID   string  `json:"id"`
			Name string  `json:"name"`
			Lon  float64 `json:"lon"`
			Lat  float64 `json:"lat"`
		}
		drivers := make([]driverJSON, 0)
		for rows.Next() {
			var id uuid.UUID
			var name string
			var dLon, dLat float64
			if err := rows.Scan(&id, &name, &dLon, &dLat); err != nil {
				slog.Error("nearby drivers: scan", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				rows.Close()
				return
			}
			// ponytail: round to 6 decimal places (~0.1m precision)
			drivers = append(drivers, driverJSON{
				ID:   id.String(),
				Name: name,
				Lon:  math.Round(dLon*1e6) / 1e6,
				Lat:  math.Round(dLat*1e6) / 1e6,
			})
		}
		if rows.Err() != nil {
			slog.Error("nearby drivers: rows iteration", "error", rows.Err())
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"drivers": drivers})
	}
}
