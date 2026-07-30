package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RegisterDriver (admin) crea un nuevo driver como administrador.
//
// Request:  POST /admin/drivers  {phone, name}
// Response: 201 {driver_id}
// Errors:   400 si faltan campos, 409 si el teléfono ya existe
func AdminCreateDriver(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone string `json:"phone"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.Phone == "" || body.Name == "" {
			writeError(w, http.StatusBadRequest, "phone and name are required")
			return
		}

		var driverID uuid.UUID
		err := pool.QueryRow(r.Context(),
			`INSERT INTO drivers (phone, name, available) VALUES ($1, $2, false) RETURNING id`,
			body.Phone, body.Name,
		).Scan(&driverID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, http.StatusConflict, "phone already registered")
				return
			}
			slog.Error("admin register driver: insert", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"driver_id": driverID.String(),
		})
	}
}

// ListDrivers retorna una lista paginada de conductores.
//
// Request:  GET /admin/drivers?limit=N&offset=N&status=all|active|suspended
// Response: 200 {drivers: [...], total: N}
// Errors:   400 si los parámetros son inválidos
func ListDrivers(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := parseIntQuery(r.URL.Query().Get("limit"), 20)
		offset := parseIntQuery(r.URL.Query().Get("offset"), 0)
		status := r.URL.Query().Get("status")
		if status == "" {
			status = "all"
		}

		where := ""
		switch status {
		case "active":
			where = "WHERE suspended_until IS NULL OR suspended_until <= now()"
		case "suspended":
			where = "WHERE suspended_until > now()"
		case "all":
			// no filter
		default:
			writeError(w, http.StatusBadRequest, "status must be 'all', 'active', or 'suspended'")
			return
		}

		// Total count
		var total int
		err := pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM drivers `+where,
		).Scan(&total)
		if err != nil {
			slog.Error("list drivers: count", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		rows, err := pool.Query(r.Context(),
			`SELECT id, phone, name, available, membresia_active_until, suspended_until, created_at FROM drivers `+where+` ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset,
		)
		if err != nil {
			slog.Error("list drivers: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer rows.Close()

		type driverRow struct {
			ID                   string     `json:"id"`
			Phone                string     `json:"phone"`
			Name                 string     `json:"name"`
			Available            bool       `json:"available"`
			MembresiaActiveUntil *time.Time `json:"membresia_active_until"`
			SuspendedUntil       *time.Time `json:"suspended_until"`
			CreatedAt            time.Time  `json:"created_at"`
		}

		drivers := make([]driverRow, 0)
		for rows.Next() {
			var id uuid.UUID
			var phone, name string
			var available bool
			var membroUntil, suspendedUntil *time.Time
			var createdAt time.Time
			if err := rows.Scan(&id, &phone, &name, &available, &membroUntil, &suspendedUntil, &createdAt); err != nil {
				slog.Error("list drivers: scan", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			drivers = append(drivers, driverRow{
				ID:                   id.String(),
				Phone:                phone,
				Name:                 name,
				Available:            available,
				MembresiaActiveUntil: membroUntil,
				SuspendedUntil:       suspendedUntil,
				CreatedAt:            createdAt,
			})
		}
		if rows.Err() != nil {
			slog.Error("list drivers: rows iteration", "error", rows.Err())
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"drivers": drivers,
			"total":   total,
		})
	}
}

// ListPassengers retorna una lista paginada de pasajeros (users).
//
// Request:  GET /admin/passengers?limit=N&offset=N&search=&status=all|active|suspended
// Response: 200 {passengers: [...], total: N}
// Errors:   400 si los parámetros son inválidos
// ponytail: single SQL path with parametrized WHERE clauses, no query builder
func ListPassengers(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := parseIntQuery(r.URL.Query().Get("limit"), 20)
		offset := parseIntQuery(r.URL.Query().Get("offset"), 0)
		search := r.URL.Query().Get("search")
		status := r.URL.Query().Get("status")
		if status == "" {
			status = "all"
		}

		switch status {
		case "all", "active", "suspended":
			// valid
		default:
			writeError(w, http.StatusBadRequest, "status must be 'all', 'active', or 'suspended'")
			return
		}

		// Build status WHERE clause substring (parametrized, no string concat of values).
		statusWhere := ""
		switch status {
		case "active":
			statusWhere = "AND (u.suspended_until IS NULL OR u.suspended_until <= now())"
		case "suspended":
			statusWhere = "AND u.suspended_until > now()"
		}

		// Total count — no JOIN needed, just filter on users.
		var total int
		countSQL := `SELECT COUNT(*) FROM users u
			WHERE ($1 = '' OR u.name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR u.phone ILIKE '%'||$1||'%')
			` + statusWhere
		err := pool.QueryRow(r.Context(), countSQL, search).Scan(&total)
		if err != nil {
			slog.Error("list passengers: count", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		listSQL := `SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.created_at,
				COUNT(r.id)::int AS trips
			FROM users u LEFT JOIN rides r ON r.passenger_id = u.id
			WHERE ($1 = '' OR u.name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR u.phone ILIKE '%'||$1||'%')
			` + statusWhere + `
			GROUP BY u.id
			ORDER BY u.created_at DESC LIMIT $2 OFFSET $3`

		rows, err := pool.Query(r.Context(), listSQL, search, limit, offset)
		if err != nil {
			slog.Error("list passengers: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer rows.Close()

		type passengerRow struct {
			ID             string     `json:"id"`
			Phone          string     `json:"phone"`
			Name           string     `json:"name"`
			Email          *string    `json:"email"`
			Rating         float64    `json:"rating"`
			Trips          int        `json:"trips"`
			SuspendedUntil *time.Time `json:"suspended_until"`
			CreatedAt      time.Time  `json:"created_at"`
		}

		passengers := make([]passengerRow, 0)
		for rows.Next() {
			var id uuid.UUID
			var phone, name string
			var email *string
			var rating float64
			var suspendedUntil *time.Time
			var createdAt time.Time
			var trips int
			if err := rows.Scan(&id, &phone, &name, &email, &rating, &suspendedUntil, &createdAt, &trips); err != nil {
				slog.Error("list passengers: scan", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			passengers = append(passengers, passengerRow{
				ID:             id.String(),
				Phone:          phone,
				Name:           name,
				Email:          email,
				Rating:         rating,
				Trips:          trips,
				SuspendedUntil: suspendedUntil,
				CreatedAt:      createdAt,
			})
		}
		if rows.Err() != nil {
			slog.Error("list passengers: rows iteration", "error", rows.Err())
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"passengers": passengers,
			"total":      total,
		})
	}
}

// suspendRow sets suspended_until far in the future for the given table/id.
func suspendRow(ctx context.Context, pool Pool, table string, id uuid.UUID) (time.Time, error) {
	var suspendedUntil time.Time
	err := pool.QueryRow(ctx,
		fmt.Sprintf(`UPDATE %s SET suspended_until = now() + interval '9999 days', updated_at = now() WHERE id = $1 RETURNING suspended_until`, table),
		id,
	).Scan(&suspendedUntil)
	return suspendedUntil, err
}

// unsuspendRow clears suspended_until for the given table/id. Returns false if not found.
func unsuspendRow(ctx context.Context, pool Pool, table string, id uuid.UUID) (bool, error) {
	tag, err := pool.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET suspended_until = NULL, updated_at = now() WHERE id = $1`, table),
		id,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// SuspendDriver suspende un conductor por tiempo indefinido.
//
// Request:  PATCH /admin/drivers/{id}/suspend
// Response: 200 {suspended_until}
// Errors:   404 si no se encuentra el conductor
func SuspendDriver(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		driverID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid driver id")
			return
		}
		suspendedUntil, err := suspendRow(r.Context(), pool, "drivers", driverID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "driver not found")
			return
		}
		if err != nil {
			slog.Error("suspend driver: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"suspended_until": suspendedUntil})
	}
}

// UnsuspendDriver restaura un conductor suspendido.
//
// Request:  PATCH /admin/drivers/{id}/unsuspend
// Response: 200 {message: "driver unsuspended"}
// Errors:   404 si no se encuentra el conductor
func UnsuspendDriver(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		driverID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid driver id")
			return
		}
		found, err := unsuspendRow(r.Context(), pool, "drivers", driverID)
		if err != nil {
			slog.Error("unsuspend driver: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "driver not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "driver unsuspended"})
	}
}

// SuspendPassenger suspende un pasajero por tiempo indefinido.
//
// Request:  PATCH /admin/passengers/{id}/suspend
// Response: 200 {suspended_until}
// Errors:   404 si no se encuentra el pasajero
func SuspendPassenger(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid passenger id")
			return
		}
		suspendedUntil, err := suspendRow(r.Context(), pool, "users", userID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "passenger not found")
			return
		}
		if err != nil {
			slog.Error("suspend passenger: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"suspended_until": suspendedUntil})
	}
}

// UnsuspendPassenger restaura un pasajero suspendido.
//
// Request:  PATCH /admin/passengers/{id}/unsuspend
// Response: 200 {message: "passenger unsuspended"}
// Errors:   404 si no se encuentra el pasajero
func UnsuspendPassenger(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid passenger id")
			return
		}
		found, err := unsuspendRow(r.Context(), pool, "users", userID)
		if err != nil {
			slog.Error("unsuspend passenger: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "passenger not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "passenger unsuspended"})
	}
}

// PassengerStats retorna estadísticas agregadas de pasajeros.
//
// Request:  GET /admin/passengers/stats
// Response: 200 {total, banned, avg_rating}
func PassengerStats(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var total, banned int
		var avgRating float64
		err := pool.QueryRow(r.Context(),
			`SELECT
				COUNT(*) AS total,
				COUNT(*) FILTER (WHERE suspended_until > now()) AS banned,
				COALESCE(AVG(rating), 0) AS avg_rating
			FROM users`,
		).Scan(&total, &banned, &avgRating)
		if err != nil {
			slog.Error("passenger stats: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"total":      total,
			"banned":     banned,
			"avg_rating": avgRating,
		})
	}
}

// BanPassenger suspende un pasajero por una duración determinada.
//
// Request:  PATCH /admin/passengers/{id}/ban  {duration, unit}
// Response: 200 {suspended_until}
// Errors:   400 si unidad inválida, 404 si no se encuentra el pasajero
// ponytail: follows SetMembership pattern — same unit validation, same interval logic
func BanPassenger(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid passenger id")
			return
		}

		var body struct {
			Duration int    `json:"duration"`
			Unit     string `json:"unit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Duration <= 0 {
			writeError(w, http.StatusBadRequest, "duration must be a positive integer")
			return
		}

		switch body.Unit {
		case "day", "week", "month", "year":
			// valid
		default:
			writeError(w, http.StatusBadRequest, "unit must be 'day', 'week', 'month', or 'year'")
			return
		}

		interval := fmt.Sprintf("%d %s", body.Duration, body.Unit)
		var suspendedUntil time.Time
		err = pool.QueryRow(r.Context(),
			`UPDATE users SET suspended_until = now() + $1::interval, updated_at = now()
			 WHERE id = $2 RETURNING suspended_until`,
			interval, userID,
		).Scan(&suspendedUntil)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "passenger not found")
			return
		}
		if err != nil {
			slog.Error("ban passenger: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"suspended_until": suspendedUntil,
		})
	}
}

// SetMembership activa la membresía de un conductor por una duración determinada.
//
// Request:  PATCH /admin/drivers/{id}/membership  {duration, unit}
// Response: 200 {membresia_active_until}
// Errors:   400 si la unidad es inválida, 404 si no se encuentra el conductor
func SetMembership(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		driverID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid driver id")
			return
		}

		var body struct {
			Duration int    `json:"duration"`
			Unit     string `json:"unit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Duration <= 0 {
			writeError(w, http.StatusBadRequest, "duration must be a positive integer")
			return
		}

		switch body.Unit {
		case "day", "week", "month", "year":
			// valid
		default:
			writeError(w, http.StatusBadRequest, "unit must be 'day', 'week', 'month', or 'year'")
			return
		}

		interval := fmt.Sprintf("%d %s", body.Duration, body.Unit)
		var membresiaActiveUntil time.Time
		err = pool.QueryRow(r.Context(),
			`UPDATE drivers SET membresia_active_until = now() + $1::interval, updated_at = now()
			 WHERE id = $2 RETURNING membresia_active_until`,
			interval, driverID,
		).Scan(&membresiaActiveUntil)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "driver not found")
			return
		}
		if err != nil {
			slog.Error("set membership: update", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"membresia_active_until": membresiaActiveUntil,
		})
	}
}

// parseIntQuery parsea un string a entero, retornando defaultValue si está vacío o es inválido.
func parseIntQuery(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
