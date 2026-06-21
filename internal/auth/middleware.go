package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey es el tipo para claves de contexto a fin de evitar colisiones.
type contextKey string

// ClaimsKey es la clave para almacenar/recuperar los claims JWT del contexto.
const ClaimsKey contextKey = "auth:claims"

// Auth retorna un middleware que extrae el token Bearer del header Authorization,
// valida el JWT con la clave secreta, e inyecta los claims en el contexto.
//
// En caso de error retorna 401 con JSON {"error": "<motivo>"}.
func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "missing authorization header"})
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid authorization header"})
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := ValidateToken(tokenStr, secret)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{"error": "token expired"})
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole retorna un middleware que verifica que el rol del usuario
// (extraído de los claims en el contexto) esté entre los roles permitidos.
// Retorna 403 Forbidden si el rol no coincide.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			claims, ok := r.Context().Value(ClaimsKey).(*Claims)
			if !ok || claims == nil {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		})
	}
}
