// Package api proporciona los handlers HTTP de la API REST.
package api

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// tokenBucket implementa un rate limiter token-bucket por clave.
// Ponte en corto: mapa global con un solo mutex, sharding si hay contención.
type tokenBucket struct {
	mu         sync.Mutex
	buckets    map[string]*bucketEntry
	capacity   int
	refillRate float64 // tokens por segundo
	window     time.Duration
}

type bucketEntry struct {
	tokens     float64
	lastRefill time.Time
	lastAccess time.Time
}

// newTokenBucket crea un bucket que permite `capacity` requests por `window`.
func newTokenBucket(capacity int, window time.Duration) *tokenBucket {
	return &tokenBucket{
		buckets:    make(map[string]*bucketEntry),
		capacity:   capacity,
		refillRate: float64(capacity) / window.Seconds(),
		window:     window,
	}
}

// allow verifica si la clave puede hacer un request. Recalcula el refill según
// el tiempo transcurrido y luego consume un token si hay disponibles.
//
// Devuelve true si se permite el request, o false + la duración de espera
// para el header Retry-After.
func (tb *tokenBucket) allow(key string) (bool, time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	b, ok := tb.buckets[key]
	if !ok {
		// Primer request — arranca con capacity-1 (consume el primer token)
		b = &bucketEntry{
			tokens:     float64(tb.capacity - 1),
			lastRefill: now,
			lastAccess: now,
		}
		tb.buckets[key] = b
		return true, 0
	}

	// Refill según tiempo transcurrido
	elapsed := now.Sub(b.lastRefill)
	b.tokens += elapsed.Seconds() * tb.refillRate
	if b.tokens > float64(tb.capacity) {
		b.tokens = float64(tb.capacity)
	}
	b.lastRefill = now
	b.lastAccess = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	// Calcular tiempo hasta el próximo token (ceil para ser conservadores)
	need := 1 - b.tokens
	waitSec := need / tb.refillRate
	wait := time.Duration(math.Ceil(waitSec * float64(time.Second)))
	return false, wait
}

// cleanup remueve entradas que no se accedieron en más de 2× la ventana.
func (tb *tokenBucket) cleanup(now time.Time) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	threshold := now.Add(-2 * tb.window)
	for key, b := range tb.buckets {
		if b.lastAccess.Before(threshold) {
			delete(tb.buckets, key)
		}
	}
}

// --- registro global de limiters para el sweeper ---

var (
	limitersMu sync.Mutex
	limiters   []*tokenBucket
)

// RateLimit retorna un middleware HTTP que limita requests según una clave
// extraída por keyFn. Ejemplo: RateLimit(5, time.Minute, ClientIP).
func RateLimit(rate int, window time.Duration, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	tb := newTokenBucket(rate, window)

	limitersMu.Lock()
	limiters = append(limiters, tb)
	limitersMu.Unlock()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			ok, wait := tb.allow(key)
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{"error": "too many requests"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// StartRateLimitSweeper lanza una goroutine que cada `interval` limpia las
// entradas stale de todos los limiters registrados vía RateLimit().
func StartRateLimitSweeper(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				limitersMu.Lock()
				for _, tb := range limiters {
					tb.cleanup(now)
				}
				limitersMu.Unlock()
			}
		}
	}()
}

// ClientIP extrae la IP del cliente desde r.RemoteAddr, sin el puerto.
func ClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// UserIDFromClaims extrae el user ID desde los claims JWT en el contexto.
// Devuelve string vacío si no hay claims (no debería pasar porque se usa
// después de auth.Auth, pero manejamos el caso).
func UserIDFromClaims(r *http.Request) string {
	claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !ok || claims == nil {
		return ""
	}
	return claims.Subject
}
