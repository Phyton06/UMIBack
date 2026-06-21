package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Phyton06/UMIBack/internal/auth"
	"github.com/Phyton06/UMIBack/internal/engine"
	"github.com/Phyton06/UMIBack/internal/model"
)

// rideCols son las columnas SELECT compartidas entre GetRide y ListRides.
// ponytail: single source of truth evitó duplicar la lista en 2 handlers
const rideCols = `id, passenger_id, driver_id, status,
	ST_AsText(pickup_location), ST_AsText(dropoff_location),
	pickup_address, dropoff_address,
	fare, cancelled_by, cancelled_at, completed_at,
	created_at, updated_at`

// --- WKT geometry helpers ---

// parsePoint convierte "POINT(lon lat)" a (lon, lat float64).
func parsePoint(wkt string) (lon, lat float64, err error) {
	s := strings.TrimPrefix(wkt, "POINT(")
	s = strings.TrimSuffix(s, ")")
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid POINT: %s", wkt)
	}
	lon, err = strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %s", parts[0])
	}
	lat, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %s", parts[1])
	}
	return lon, lat, nil
}

// formatPoint genera "POINT(lon lat)" para ST_GeomFromText.
func formatPoint(lon, lat float64) string {
	// ponytail: %v avoids extra trailing zeros from %f
	return fmt.Sprintf("POINT(%v %v)", lon, lat)
}

// --- rideRow contiene los campos retornados por SELECT sobre rides. ---
type rideRow struct {
	ID              uuid.UUID
	PassengerID     uuid.UUID
	DriverID        *uuid.UUID
	Status          string
	PickupWKT       string
	DropoffWKT      string
	PickupAddress   string
	DropoffAddress  string
	Fare            *float64
	CancelledBy     *string
	CancelledAt     *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// scanRide escanea una fila de rides desde pgx.Row o pgx.Rows.
func scanRide(scanner interface {
	Scan(dest ...any) error
}) (rideRow, error) {
	var r rideRow
	err := scanner.Scan(
		&r.ID, &r.PassengerID, &r.DriverID, &r.Status,
		&r.PickupWKT, &r.DropoffWKT,
		&r.PickupAddress, &r.DropoffAddress,
		&r.Fare, &r.CancelledBy, &r.CancelledAt, &r.CompletedAt,
		&r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// rideToJSON convierte un rideRow en un map para respuesta JSON.
func rideToJSON(r rideRow) map[string]any {
	pickupLon, pickupLat, _ := parsePoint(r.PickupWKT)
	dropoffLon, dropoffLat, _ := parsePoint(r.DropoffWKT)

	// ponytail: round to 6 decimal places (~0.1m precision)
	m := map[string]any{
		"id":              r.ID.String(),
		"passenger_id":    r.PassengerID.String(),
		"status":          r.Status,
		"pickup_address":  r.PickupAddress,
		"dropoff_address": r.DropoffAddress,
		"pickup": map[string]float64{
			"lon": math.Round(pickupLon*1e6) / 1e6,
			"lat": math.Round(pickupLat*1e6) / 1e6,
		},
		"dropoff": map[string]float64{
			"lon": math.Round(dropoffLon*1e6) / 1e6,
			"lat": math.Round(dropoffLat*1e6) / 1e6,
		},
		"created_at": r.CreatedAt,
		"updated_at": r.UpdatedAt,
	}
	if r.DriverID != nil {
		m["driver_id"] = r.DriverID.String()
	}
	if r.Fare != nil {
		m["fare"] = *r.Fare
	}
	if r.CancelledBy != nil {
		m["cancelled_by"] = *r.CancelledBy
	}
	if r.CancelledAt != nil {
		m["cancelled_at"] = *r.CancelledAt
	}
	if r.CompletedAt != nil {
		m["completed_at"] = *r.CompletedAt
	}
	return m
}

// --- CalcFare ---

// CalcFare calcula MAX(distance_km * ratePerKm, minimum), redondeado a 2 decimales.
func CalcFare(distanceKm, ratePerKm, minimum float64) float64 {
	fare := math.Round(distanceKm*ratePerKm*100) / 100
	if fare < minimum {
		return minimum
	}
	return fare
}

// --- EstimateRide ---

// EstimateRide estima la tarifa de un viaje según coordenadas de origen y destino.
//
// Request:  POST /rides/estimate  {pickup_lon, pickup_lat, dropoff_lon, dropoff_lat}
// Response: 200 {estimated_fare, distance_km, rate_per_km, minimum_fare}
// Errors:   400 si faltan campos o coordenadas inválidas, 401 sin auth
func EstimateRide(pool Pool, ratePerKm, minimum float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			PickupLon  *float64 `json:"pickup_lon"`
			PickupLat  *float64 `json:"pickup_lat"`
			DropoffLon *float64 `json:"dropoff_lon"`
			DropoffLat *float64 `json:"dropoff_lat"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.PickupLon == nil || body.PickupLat == nil ||
			body.DropoffLon == nil || body.DropoffLat == nil {
			writeError(w, http.StatusBadRequest, "all fields are required")
			return
		}

		if *body.PickupLon < -180 || *body.PickupLon > 180 ||
			*body.DropoffLon < -180 || *body.DropoffLon > 180 ||
			*body.PickupLat < -90 || *body.PickupLat > 90 ||
			*body.DropoffLat < -90 || *body.DropoffLat > 90 {
			writeError(w, http.StatusBadRequest, "invalid coordinates")
			return
		}

		var distanceMeters float64
		err := pool.QueryRow(r.Context(),
			`SELECT ST_Distance(ST_SetSRID(ST_MakePoint($1,$2),4326)::geography, ST_SetSRID(ST_MakePoint($3,$4),4326)::geography)`,
			*body.PickupLon, *body.PickupLat, *body.DropoffLon, *body.DropoffLat,
		).Scan(&distanceMeters)
		if err != nil {
			slog.Error("estimate ride: distance query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		distanceKm := distanceMeters / 1000
		fare := CalcFare(distanceKm, ratePerKm, minimum)

		writeJSON(w, http.StatusOK, map[string]any{
			"estimated_fare": fare,
			"distance_km":    math.Round(distanceKm*100) / 100,
			"rate_per_km":    ratePerKm,
			"minimum_fare":   minimum,
		})
	}
}

// --- CreateRide ---

// CreateRide crea un nuevo viaje como rider.
//
// Request:  POST /rides  {pickup_address, dropoff_address, pickup_lon, pickup_lat, dropoff_lon, dropoff_lat}
// Response: 201 {id, status, pickup_address, dropoff_address, pickup, dropoff, created_at}
// Errors:   400 si faltan campos, 401 sin auth, 403 si no es rider
func CreateRide(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// ponytail: role gated by RequireRole middleware, no need to double-check

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		var body struct {
			PickupAddress  string   `json:"pickup_address"`
			DropoffAddress string   `json:"dropoff_address"`
			PickupLon      *float64 `json:"pickup_lon"`
			PickupLat      *float64 `json:"pickup_lat"`
			DropoffLon     *float64 `json:"dropoff_lon"`
			DropoffLat     *float64 `json:"dropoff_lat"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.PickupAddress == "" || body.DropoffAddress == "" ||
			body.PickupLon == nil || body.PickupLat == nil ||
			body.DropoffLon == nil || body.DropoffLat == nil {
			writeError(w, http.StatusBadRequest, "all fields are required")
			return
		}

		var rideID uuid.UUID
		var createdAt time.Time
		err = pool.QueryRow(r.Context(),
			`INSERT INTO rides (passenger_id, status, pickup_location, dropoff_location, pickup_address, dropoff_address)
			 VALUES ($1, 'REQUESTED', ST_GeomFromText($2, 4326), ST_GeomFromText($3, 4326), $4, $5)
			 RETURNING id, created_at`,
			userID,
			formatPoint(*body.PickupLon, *body.PickupLat),
			formatPoint(*body.DropoffLon, *body.DropoffLat),
			body.PickupAddress,
			body.DropoffAddress,
		).Scan(&rideID, &createdAt)
		if err != nil {
			slog.Error("create ride: insert", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":              rideID.String(),
			"status":          "REQUESTED",
			"pickup_address":  body.PickupAddress,
			"dropoff_address": body.DropoffAddress,
			"pickup": map[string]float64{
				"lon": math.Round(*body.PickupLon*1e6) / 1e6,
				"lat": math.Round(*body.PickupLat*1e6) / 1e6,
			},
			"dropoff": map[string]float64{
				"lon": math.Round(*body.DropoffLon*1e6) / 1e6,
				"lat": math.Round(*body.DropoffLat*1e6) / 1e6,
			},
			"created_at": createdAt,
		})
	}
}

// --- GetRide ---

// GetRide retorna los detalles de un viaje. Solo el pasajero o el conductor
// asignado pueden verlo (404 si no hay acceso).
//
// Request:  GET /rides/{id}
// Response: 200 full ride JSON
// Errors:   401 sin auth, 404 no encontrado o sin acceso
func GetRide(pool Pool) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		row, err := scanRide(pool.QueryRow(r.Context(),
			`SELECT `+rideCols+` FROM rides WHERE id = $1`,
			rideID,
		))
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("get ride: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if row.PassengerID != userID && (row.DriverID == nil || *row.DriverID != userID) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}

		writeJSON(w, http.StatusOK, rideToJSON(row))
	}
}

// --- ListRides ---

// ListRides lista viajes según el rol: rider ve sus propios viajes,
// driver ve solicitudes abiertas (REQUESTED sin conductor).
//
// Request:  GET /rides[?status=...]
// Response: 200 {rides: [...]}
// Errors:   401 sin auth
func ListRides(pool Pool) http.HandlerFunc {
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

		statusFilter := r.URL.Query().Get("status")

		var pgRows pgx.Rows
		if claims.Role == "rider" {
			if statusFilter != "" {
				pgRows, err = pool.Query(r.Context(),
					`SELECT `+rideCols+` FROM rides WHERE passenger_id = $1 AND status = $2 ORDER BY created_at DESC`,
					userID, statusFilter,
				)
			} else {
				pgRows, err = pool.Query(r.Context(),
					`SELECT `+rideCols+` FROM rides WHERE passenger_id = $1 ORDER BY created_at DESC`,
					userID,
				)
			}
		} else {
			query := `SELECT ` + rideCols + ` FROM rides WHERE driver_id IS NULL AND status = $1 ORDER BY created_at DESC`
			if statusFilter == "" {
				statusFilter = "REQUESTED"
			}
			pgRows, err = pool.Query(r.Context(), query, statusFilter)
		}
		if err != nil {
			slog.Error("list rides: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer pgRows.Close()

		rides := make([]map[string]any, 0)
		for pgRows.Next() {
			row, scanErr := scanRide(pgRows)
			if scanErr != nil {
				slog.Error("list rides: scan", "error", scanErr)
				writeError(w, http.StatusInternalServerError, "internal error")
				pgRows.Close()
				return
			}
			rides = append(rides, rideToJSON(row))
		}
		if pgRows.Err() != nil {
			slog.Error("list rides: rows iteration", "error", pgRows.Err())
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"rides": rides})
	}
}

// --- CancelRide ---

// CancelRide cancela un viaje. Solo el pasajero o el conductor asignado
// pueden cancelar. Valida la transición de estado y usa UPDATE condicional
// para safety ante condiciones de carrera.
//
// Request:  PATCH /rides/{id}/cancel  {reason?}
// Response: 200 {id, status: "CANCELLED"}
// Errors:   400 si la transición no es válida, 401 sin auth,
//           404 si no existe o sin acceso, 409 si otro request ya cambió el estado
func CancelRide(pool Pool) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		// Optional reason body — ignore decode errors
		var body struct {
			Reason *string `json:"reason,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// SELECT current ride for access control + state validation
		var passengerID uuid.UUID
		var driverID *uuid.UUID
		var currentStatus string
		err = pool.QueryRow(r.Context(),
			`SELECT passenger_id, driver_id, status FROM rides WHERE id = $1`,
			rideID,
		).Scan(&passengerID, &driverID, &currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("cancel ride: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Access control: only passenger or assigned driver may cancel
		if passengerID != userID && (driverID == nil || *driverID != userID) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}

		// Validate state transition
		if err := engine.CanTransition(model.RideStatus(currentStatus), model.StatusCancelled); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Conditional UPDATE for race safety: if status changed between
		// SELECT and UPDATE, zero rows match → 409.
		// ponytail: pre-cancel states duplicate state machine; update if matrix changes
		tag, err := pool.Exec(r.Context(),
			`UPDATE rides
			 SET status = 'CANCELLED', cancelled_by = $1, cancelled_at = now(), updated_at = now()
			 WHERE id = $2 AND status = ANY($3::varchar[])`,
			userID.String(), rideID,
			[]string{
				string(model.StatusRequested),
				string(model.StatusAccepted),
				string(model.StatusEnRoute),
				string(model.StatusArrived),
			},
		)
		if err != nil {
			slog.Error("cancel ride: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusConflict, "ride was already cancelled or completed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":     rideID.String(),
			"status": "CANCELLED",
		})
	}
}

// --- AcceptRide ---

// AcceptRide asigna un conductor a un viaje solicitado.
// El conductor debe estar disponible y no puede aceptar su propio viaje.
// Usa UPDATE condicional para safety ante condiciones de carrera.
//
// Request:  PATCH /rides/{id}/accept  (sin body)
// Response: 200 {id, status: "ACCEPTED", driver_id}
// Errors:   400 si el conductor no está disponible o transición inválida,
//           401 sin auth, 403 si el rol no es driver o es su propio viaje,
//           404 si no se encuentra el ride, 409 si otro conductor ya aceptó
func AcceptRide(pool Pool) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		// Check driver availability
		var available bool
		err = pool.QueryRow(r.Context(),
			`SELECT available FROM drivers WHERE id = $1`, userID,
		).Scan(&available)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "driver not found")
			return
		}
		if err != nil {
			slog.Error("accept ride: select driver", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !available {
			writeError(w, http.StatusBadRequest, "driver not available")
			return
		}

		// SELECT current ride for access control + state validation
		var passengerID uuid.UUID
		var driverID *uuid.UUID
		var currentStatus string
		err = pool.QueryRow(r.Context(),
			`SELECT passenger_id, driver_id, status FROM rides WHERE id = $1`,
			rideID,
		).Scan(&passengerID, &driverID, &currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("accept ride: select ride", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Can't accept own ride
		if passengerID == userID {
			writeError(w, http.StatusForbidden, "cannot accept your own ride")
			return
		}

		// Validate state transition
		if err := engine.CanTransition(model.RideStatus(currentStatus), model.StatusAccepted); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Conditional UPDATE for race safety: if status changed or driver
		// already assigned between SELECT and UPDATE, zero rows match → 409.
		tag, err := pool.Exec(r.Context(),
			`UPDATE rides SET status = 'ACCEPTED', driver_id = $1, updated_at = now()
			 WHERE id = $2 AND driver_id IS NULL AND status = 'REQUESTED'`,
			userID, rideID,
		)
		if err != nil {
			slog.Error("accept ride: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusConflict, "ride was already accepted")
			return
		}

		// ponytail: non-critical — ride is already assigned, log and continue on error
		_, err = pool.Exec(r.Context(),
			`UPDATE drivers SET available = false WHERE id = $1`, userID,
		)
		if err != nil {
			slog.Error("accept ride: update driver availability", "error", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":        rideID.String(),
			"status":    "ACCEPTED",
			"driver_id": userID.String(),
		})
	}
}

// --- EnRouteRide ---

// EnRouteRide marca que el conductor va en camino al punto de recogida.
// Transición: ACCEPTED → EN_ROUTE.
//
// Request:  PATCH /rides/{id}/en-route  (sin body)
// Response: 200 {id, status: "EN_ROUTE"}
// Errors:   400 si la transición no es válida, 401 sin auth,
//           404 si no existe o el conductor no es el asignado,
//           409 si otro request ya cambió el estado
func EnRouteRide(pool Pool) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		// SELECT current ride for access control + state validation
		// ponytail: passenger_id not needed here (driver-only access)
		var driverID *uuid.UUID
		var currentStatus string
		err = pool.QueryRow(r.Context(),
			`SELECT passenger_id, driver_id, status FROM rides WHERE id = $1`,
			rideID,
		).Scan(new(uuid.UUID), &driverID, &currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("en-route ride: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Access control: only the assigned driver may advance the ride
		if driverID == nil || *driverID != userID {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}

		// Validate state transition
		if err := engine.CanTransition(model.RideStatus(currentStatus), model.StatusEnRoute); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Conditional UPDATE for race safety
		tag, err := pool.Exec(r.Context(),
			`UPDATE rides SET status = 'EN_ROUTE', updated_at = now()
			 WHERE id = $1 AND status = 'ACCEPTED'`,
			rideID,
		)
		if err != nil {
			slog.Error("en-route ride: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusConflict, "ride status changed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":     rideID.String(),
			"status": "EN_ROUTE",
		})
	}
}

// --- ArrivedRide ---

// ArrivedRide marca que el conductor llegó al punto de recogida.
// Transición: EN_ROUTE → ARRIVED.
//
// Request:  PATCH /rides/{id}/arrived  (sin body)
// Response: 200 {id, status: "ARRIVED"}
// Errors:   400 si la transición no es válida, 401 sin auth,
//           404 si no existe o el conductor no es el asignado,
//           409 si otro request ya cambió el estado
func ArrivedRide(pool Pool) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		var driverID *uuid.UUID
		var currentStatus string
		err = pool.QueryRow(r.Context(),
			`SELECT passenger_id, driver_id, status FROM rides WHERE id = $1`,
			rideID,
		).Scan(new(uuid.UUID), &driverID, &currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("arrived ride: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if driverID == nil || *driverID != userID {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}

		if err := engine.CanTransition(model.RideStatus(currentStatus), model.StatusArrived); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE rides SET status = 'ARRIVED', updated_at = now()
			 WHERE id = $1 AND status = 'EN_ROUTE'`,
			rideID,
		)
		if err != nil {
			slog.Error("arrived ride: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusConflict, "ride status changed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":     rideID.String(),
			"status": "ARRIVED",
		})
	}
}

// --- StartRide ---

// StartRide inicia el viaje (conductor y pasajero a bordo).
// Transición: ARRIVED → IN_PROGRESS.
//
// Request:  PATCH /rides/{id}/start  (sin body)
// Response: 200 {id, status: "IN_PROGRESS"}
// Errors:   400 si la transición no es válida, 401 sin auth,
//           404 si no existe o el conductor no es el asignado,
//           409 si otro request ya cambió el estado
func StartRide(pool Pool) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		var driverID *uuid.UUID
		var currentStatus string
		err = pool.QueryRow(r.Context(),
			`SELECT passenger_id, driver_id, status FROM rides WHERE id = $1`,
			rideID,
		).Scan(new(uuid.UUID), &driverID, &currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("start ride: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if driverID == nil || *driverID != userID {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}

		if err := engine.CanTransition(model.RideStatus(currentStatus), model.StatusInProgress); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE rides SET status = 'IN_PROGRESS', updated_at = now()
			 WHERE id = $1 AND status = 'ARRIVED'`,
			rideID,
		)
		if err != nil {
			slog.Error("start ride: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusConflict, "ride status changed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":     rideID.String(),
			"status": "IN_PROGRESS",
		})
	}
}

// --- CompleteRide ---

// CompleteRide finaliza el viaje y calcula la tarifa.
// Transición: IN_PROGRESS → COMPLETED. Asigna completed_at y fare.
//
// Request:  PATCH /rides/{id}/complete  (sin body)
// Response: 200 {id, status: "COMPLETED", completed_at, fare}
// Errors:   400 si la transición no es válida, 401 sin auth,
//           404 si no existe o el conductor no es el asignado,
//           409 si otro request ya cambió el estado
func CompleteRide(pool Pool, ratePerKm, minimum float64) http.HandlerFunc {
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

		rideID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ride id")
			return
		}

		var driverID *uuid.UUID
		var currentStatus string
		var distanceMeters float64
		err = pool.QueryRow(r.Context(),
			`SELECT passenger_id, driver_id, status,
			        ST_Distance(pickup_location::geography, dropoff_location::geography)
			 FROM rides WHERE id = $1`,
			rideID,
		).Scan(new(uuid.UUID), &driverID, &currentStatus, &distanceMeters)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}
		if err != nil {
			slog.Error("complete ride: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if driverID == nil || *driverID != userID {
			writeError(w, http.StatusNotFound, "ride not found")
			return
		}

		if err := engine.CanTransition(model.RideStatus(currentStatus), model.StatusCompleted); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		fare := CalcFare(distanceMeters/1000, ratePerKm, minimum)

		tag, err := pool.Exec(r.Context(),
			`UPDATE rides SET status = 'COMPLETED', completed_at = now(), updated_at = now(), fare = $2
			 WHERE id = $1 AND status = 'IN_PROGRESS'`,
			rideID, fare,
		)
		if err != nil {
			slog.Error("complete ride: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusConflict, "ride status changed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":     rideID.String(),
			"status": "COMPLETED",
			"fare":   fare,
		})
	}
}
