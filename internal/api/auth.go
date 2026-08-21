package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// Pool define las operaciones de base de datos que usan los handlers de auth y rides.
// pgxpool.Pool y pgxmock.Pool satisfacen esta interfaz.
type Pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Ping(ctx context.Context) error
}

// Pool interface kept for pgxmock testability — compile-time check via interface satisfaction

const (
	defaultAccessExpiry  = 15 * time.Minute
	defaultRefreshExpiry = 168 * time.Hour // 7 days
)

// isValidRole verifica si el rol está entre los permitidos.
func isValidRole(role string) bool {
	return role == "rider" || role == "driver" || role == "admin"
}

// writeJSON escribe una respuesta JSON con el código de estado indicado.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError escribe una respuesta JSON de error.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Register ---

// RegisterRider crea un nuevo rider (users) y emite un par JWT.
//
// Request:  POST /auth/register/rider  {phone, name}
// Response: 201 {user_id, access_token, refresh_token}
// Errors:   400 si faltan campos, 409 si el teléfono ya existe
func RegisterRider(pool Pool, jwtSecret []byte) http.HandlerFunc {
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

		// Normalize phone: strip non-digits, remove +52 prefix if present
		normalized := regexp.MustCompile(`\D`).ReplaceAllString(body.Phone, "")
		if len(normalized) == 12 && normalized[:2] == "52" {
			normalized = normalized[2:]
		}
		if len(normalized) != 10 {
			writeError(w, http.StatusBadRequest, "phone must be 10 digits")
			return
		}
		body.Phone = normalized

		var userID uuid.UUID
		err := pool.QueryRow(r.Context(),
			`INSERT INTO users (phone, name) VALUES ($1, $2) RETURNING id`,
			body.Phone, body.Name,
		).Scan(&userID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, http.StatusConflict, "phone already registered")
				return
			}
			slog.Error("register rider: insert", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		accessToken, err := auth.SignAccessToken(userID, "rider", jwtSecret, defaultAccessExpiry)
		if err != nil {
			slog.Error("register rider: sign access", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		refreshToken, rtData, err := auth.SignRefreshToken(userID, "rider", jwtSecret, defaultRefreshExpiry)
		if err != nil {
			slog.Error("register rider: sign refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_, err = pool.Exec(r.Context(),
			`INSERT INTO refresh_tokens (user_id, role, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			userID, "rider", rtData.TokenHash, rtData.ExpiresAt,
		)
		if err != nil {
			slog.Error("register rider: store refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"user_id":       userID.String(),
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		})
	}
}

// RegisterDriver crea un nuevo driver y emite un par JWT.
//
// Request:  POST /auth/register/driver  {phone, name}
// Response: 201 {driver_id, access_token, refresh_token}
// Errors:   400 si faltan campos, 409 si el teléfono ya existe
func RegisterDriver(pool Pool, jwtSecret []byte) http.HandlerFunc {
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

		// Normalize phone: strip non-digits, remove +52 prefix if present
		normalized := regexp.MustCompile(`\D`).ReplaceAllString(body.Phone, "")
		if len(normalized) == 12 && normalized[:2] == "52" {
			normalized = normalized[2:]
		}
		if len(normalized) != 10 {
			writeError(w, http.StatusBadRequest, "phone must be 10 digits")
			return
		}
		body.Phone = normalized

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
			slog.Error("register driver: insert", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		accessToken, err := auth.SignAccessToken(driverID, "driver", jwtSecret, defaultAccessExpiry)
		if err != nil {
			slog.Error("register driver: sign access", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		refreshToken, rtData, err := auth.SignRefreshToken(driverID, "driver", jwtSecret, defaultRefreshExpiry)
		if err != nil {
			slog.Error("register driver: sign refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_, err = pool.Exec(r.Context(),
			`INSERT INTO refresh_tokens (user_id, role, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			driverID, "driver", rtData.TokenHash, rtData.ExpiresAt,
		)
		if err != nil {
			slog.Error("register driver: store refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"driver_id":     driverID.String(),
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		})
	}
}

// RegisterAdmin crea un nuevo admin y emite un par JWT.
//
// Request:  POST /auth/register/admin  {phone, name}
// Response: 201 {admin_id, access_token, refresh_token}
// Errors:   400 si faltan campos, 409 si el teléfono ya existe
func RegisterAdmin(pool Pool, jwtSecret []byte) http.HandlerFunc {
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

		// Normalize phone: strip non-digits, remove +52 prefix if present
		normalized := regexp.MustCompile(`\D`).ReplaceAllString(body.Phone, "")
		if len(normalized) == 12 && normalized[:2] == "52" {
			normalized = normalized[2:]
		}
		if len(normalized) != 10 {
			writeError(w, http.StatusBadRequest, "phone must be 10 digits")
			return
		}
		body.Phone = normalized

		var adminID uuid.UUID
		err := pool.QueryRow(r.Context(),
			`INSERT INTO admins (phone, name) VALUES ($1, $2) RETURNING id`,
			body.Phone, body.Name,
		).Scan(&adminID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, http.StatusConflict, "phone already registered")
				return
			}
			slog.Error("register admin: insert", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		accessToken, err := auth.SignAccessToken(adminID, "admin", jwtSecret, defaultAccessExpiry)
		if err != nil {
			slog.Error("register admin: sign access", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		refreshToken, rtData, err := auth.SignRefreshToken(adminID, "admin", jwtSecret, defaultRefreshExpiry)
		if err != nil {
			slog.Error("register admin: sign refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_, err = pool.Exec(r.Context(),
			`INSERT INTO refresh_tokens (user_id, role, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			adminID, "admin", rtData.TokenHash, rtData.ExpiresAt,
		)
		if err != nil {
			slog.Error("register admin: store refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"admin_id":      adminID.String(),
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		})
	}
}

// --- OTP ---

// RequestOTP genera un código OTP, lo almacena hasheado y lo envía (mock: log).
//
// Request:  POST /auth/request-otp  {phone, role}
// Response: 200 {message: "OTP sent"}
// Errors:   400 si role no es "rider"|"driver", 400 si falta phone
func RequestOTP(pool Pool, sender auth.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone string `json:"phone"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.Phone == "" {
			writeError(w, http.StatusBadRequest, "phone is required")
			return
		}

		// Normalize phone: strip non-digits, remove +52 prefix if present
		normalized := regexp.MustCompile(`\D`).ReplaceAllString(body.Phone, "")
		if len(normalized) == 12 && normalized[:2] == "52" {
			normalized = normalized[2:]
		}
		if len(normalized) != 10 {
			writeError(w, http.StatusBadRequest, "phone must be 10 digits")
			return
		}
		body.Phone = normalized

		if body.Role == "" {
			// Auto-detect by checking which table has this phone
			var detectedRole string
			var id uuid.UUID
			err := pool.QueryRow(r.Context(),
				`SELECT id FROM users WHERE phone = $1 OR phone = $2`,
				normalized, "+52"+normalized,
			).Scan(&id)
			if err == nil {
				detectedRole = "rider"
			} else if errors.Is(err, pgx.ErrNoRows) {
				err = pool.QueryRow(r.Context(),
					`SELECT id FROM drivers WHERE phone = $1 OR phone = $2`,
					normalized, "+52"+normalized,
				).Scan(&id)
				if err == nil {
					detectedRole = "driver"
				} else if errors.Is(err, pgx.ErrNoRows) {
					err = pool.QueryRow(r.Context(),
						`SELECT id FROM admins WHERE phone = $1 OR phone = $2`,
						normalized, "+52"+normalized,
					).Scan(&id)
					if err == nil {
						detectedRole = "admin"
					} else if errors.Is(err, pgx.ErrNoRows) {
						writeError(w, http.StatusNotFound, "phone not registered")
						return
					}
				}
			}
			if err != nil && detectedRole == "" {
				if !errors.Is(err, pgx.ErrNoRows) {
					slog.Error("request otp: auto-detect lookup", "error", err)
				}
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			body.Role = detectedRole
		}
		if !isValidRole(body.Role) {
			writeError(w, http.StatusBadRequest, "role must be 'rider', 'driver', or 'admin'")
			return
		}

		devMode := os.Getenv("DEV_MODE") == "true"
		var code string
		var err error
		if devMode {
			code = "000000"
		} else {
			code, err = auth.GenerateOTP(6)
			if err != nil {
				slog.Error("request otp: generate", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
		}

		codeHash := auth.HashOTP(code)
		expiresAt := time.Now().Add(5 * time.Minute)

		_, err = pool.Exec(r.Context(),
			`INSERT INTO otp_codes (phone, role, code_hash, attempts, expires_at, created_at)
			 VALUES ($1, $2, $3, 0, $4, now())
			 ON CONFLICT (phone, role)
			 DO UPDATE SET code_hash = $3, attempts = 0, expires_at = $4, created_at = now()`,
			body.Phone, body.Role, codeHash, expiresAt,
		)
		if err != nil {
			slog.Error("request otp: store", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := sender.Send(body.Phone, code); err != nil {
			slog.Error("request otp: send", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if devMode {
			writeJSON(w, http.StatusOK, map[string]string{"message": "OTP sent", "code": code})
		} else {
			writeJSON(w, http.StatusOK, map[string]string{"message": "OTP sent"})
		}
	}
}

// VerifyOTP verifica un código OTP y emite un par JWT.
//
// Request:  POST /auth/verify-otp  {phone, role, code}
// Response: 200 {access_token, refresh_token}
// Errors:   401 si código inválido, 410 si expiró, 429 si excedió intentos
func VerifyOTP(pool Pool, jwtSecret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone string `json:"phone"`
			Role  string `json:"role"`
			Code  string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.Phone == "" || body.Code == "" {
			writeError(w, http.StatusBadRequest, "phone and code are required")
			return
		}

		// Normalize phone: strip non-digits, remove +52 prefix if present
		normalized := regexp.MustCompile(`\D`).ReplaceAllString(body.Phone, "")
		if len(normalized) == 12 && normalized[:2] == "52" {
			normalized = normalized[2:]
		}
		if len(normalized) != 10 {
			writeError(w, http.StatusBadRequest, "phone must be 10 digits")
			return
		}
		body.Phone = normalized

		if body.Role == "" {
			// Auto-detect by checking which table has this phone
			var detectedRole string
			var id uuid.UUID
			err := pool.QueryRow(r.Context(),
				`SELECT id FROM users WHERE phone = $1 OR phone = $2`,
				normalized, "+52"+normalized,
			).Scan(&id)
			if err == nil {
				detectedRole = "rider"
			} else if errors.Is(err, pgx.ErrNoRows) {
				err = pool.QueryRow(r.Context(),
					`SELECT id FROM drivers WHERE phone = $1 OR phone = $2`,
					normalized, "+52"+normalized,
				).Scan(&id)
				if err == nil {
					detectedRole = "driver"
				} else if errors.Is(err, pgx.ErrNoRows) {
					err = pool.QueryRow(r.Context(),
						`SELECT id FROM admins WHERE phone = $1 OR phone = $2`,
						normalized, "+52"+normalized,
					).Scan(&id)
					if err == nil {
						detectedRole = "admin"
					} else if errors.Is(err, pgx.ErrNoRows) {
						writeError(w, http.StatusNotFound, "phone not registered")
						return
					}
				}
			}
			if err != nil && detectedRole == "" {
				if !errors.Is(err, pgx.ErrNoRows) {
					slog.Error("verify otp: auto-detect lookup", "error", err)
				}
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			body.Role = detectedRole
		}
		if !isValidRole(body.Role) {
			writeError(w, http.StatusBadRequest, "role must be 'rider', 'driver', or 'admin'")
			return
		}

		devMode := os.Getenv("DEV_MODE") == "true"
		var err error

		if !(devMode && (body.Code == "000000" || body.Code == "123456")) {
			var storedHash []byte
			var attempts int16
			var expiresAt time.Time
			err = pool.QueryRow(r.Context(),
				`SELECT code_hash, attempts, expires_at FROM otp_codes WHERE phone = $1 AND role = $2`,
				body.Phone, body.Role,
			).Scan(&storedHash, &attempts, &expiresAt)

			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, "invalid code")
				return
			}
			if err != nil {
				slog.Error("verify otp: select", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}

			if time.Now().After(expiresAt) {
				writeError(w, http.StatusGone, "code expired")
				return
			}

			if attempts >= 3 {
				writeError(w, http.StatusTooManyRequests, "too many attempts")
				return
			}

			inputHash := auth.HashOTP(body.Code)
			if subtle.ConstantTimeCompare(storedHash, inputHash) != 1 {
				_, incrErr := pool.Exec(r.Context(),
					`UPDATE otp_codes SET attempts = attempts + 1 WHERE phone = $1 AND role = $2`,
					body.Phone, body.Role,
				)
				if incrErr != nil {
					slog.Error("verify otp: increment attempts", "error", incrErr)
				}
				writeError(w, http.StatusUnauthorized, "invalid code")
				return
			}
		}

		// Delete OTP record (skip in dev mode with magic code)
		if !(devMode && (body.Code == "000000" || body.Code == "123456")) {
			_, err = pool.Exec(r.Context(),
				`DELETE FROM otp_codes WHERE phone = $1 AND role = $2`,
				body.Phone, body.Role,
			)
			if err != nil {
				slog.Error("verify otp: delete", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
		}

		// Look up user by phone to issue JWT for the correct identity.
		var userID uuid.UUID
		switch body.Role {
		case "rider":
			err = pool.QueryRow(r.Context(),
				`SELECT id FROM users WHERE phone = $1 OR phone = $2`,
				body.Phone, "+52"+body.Phone,
			).Scan(&userID)
		case "driver":
			err = pool.QueryRow(r.Context(),
				`SELECT id FROM drivers WHERE phone = $1 OR phone = $2`,
				body.Phone, "+52"+body.Phone,
			).Scan(&userID)
		default: // admin
			err = pool.QueryRow(r.Context(),
				`SELECT id FROM admins WHERE phone = $1 OR phone = $2`,
				body.Phone, "+52"+body.Phone,
			).Scan(&userID)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "user not found — register first")
			return
		}
		if err != nil {
			slog.Error("verify otp: lookup user", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		accessToken, err := auth.SignAccessToken(userID, body.Role, jwtSecret, defaultAccessExpiry)
		if err != nil {
			slog.Error("verify otp: sign access", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		refreshToken, rtData, err := auth.SignRefreshToken(userID, body.Role, jwtSecret, defaultRefreshExpiry)
		if err != nil {
			slog.Error("verify otp: sign refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_, err = pool.Exec(r.Context(),
			`INSERT INTO refresh_tokens (user_id, role, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			userID, body.Role, rtData.TokenHash, rtData.ExpiresAt,
		)
		if err != nil {
			slog.Error("verify otp: store refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		})
	}
}

// --- Refresh ---

// RefreshToken rota un refresh token: invalida el anterior, emite un nuevo par.
//
// Request:  POST /auth/refresh  {refresh_token}
// Response: 200 {access_token, refresh_token}
// Errors:   401 si no encontrado, 401 + breach si fue revocado (elimina todos)
func RefreshToken(pool Pool, jwtSecret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.RefreshToken == "" {
			writeError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}

		tokenHash := sha256.Sum256([]byte(body.RefreshToken))
		hashSlice := tokenHash[:]

		var id uuid.UUID
		var userID uuid.UUID
		var role string
		var revokedAt *time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT id, user_id, role, revoked_at FROM refresh_tokens WHERE token_hash = $1`,
			hashSlice,
		).Scan(&id, &userID, &role, &revokedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		if err != nil {
			slog.Error("refresh: select", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if revokedAt != nil {
			// Breach detected — revoke ALL tokens for this user
			_, err := pool.Exec(r.Context(),
				`DELETE FROM refresh_tokens WHERE user_id = $1`,
				userID,
			)
			if err != nil {
				slog.Error("refresh: breach delete all", "error", err)
			}
			writeError(w, http.StatusUnauthorized, "refresh token revoked")
			return
		}

		// Mark old token as revoked
		_, err = pool.Exec(r.Context(),
			`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`,
			id,
		)
		if err != nil {
			slog.Error("refresh: mark revoked", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Issue new pair
		accessToken, err := auth.SignAccessToken(userID, role, jwtSecret, defaultAccessExpiry)
		if err != nil {
			slog.Error("refresh: sign access", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		newRefreshToken, rtData, err := auth.SignRefreshToken(userID, role, jwtSecret, defaultRefreshExpiry)
		if err != nil {
			slog.Error("refresh: sign refresh", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_, err = pool.Exec(r.Context(),
			`INSERT INTO refresh_tokens (user_id, role, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			userID, role, rtData.TokenHash, rtData.ExpiresAt,
		)
		if err != nil {
			slog.Error("refresh: store new", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"access_token":  accessToken,
			"refresh_token": newRefreshToken,
		})
	}
}

// --- Logout ---

// Logout invalida todos los refresh tokens del usuario autenticado.
// Requiere auth middleware (Bearer token).
//
// Request:  POST /auth/logout  (Bearer token en Authorization header)
// Response: 200 {message: "logged out"}
func Logout(pool Pool) http.HandlerFunc {
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

		_, err = pool.Exec(r.Context(),
			`DELETE FROM refresh_tokens WHERE user_id = $1`,
			userID,
		)
		if err != nil {
			slog.Error("logout: delete", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
	}
}

// constant-time comparison prevents timing side-channels on OTP verification
