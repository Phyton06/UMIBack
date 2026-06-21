// Command server es el punto de entrada del servidor HTTP de UMI.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Phyton06/UMIBack/internal/api"
	"github.com/Phyton06/UMIBack/internal/auth"
	"github.com/Phyton06/UMIBack/internal/config"
	"github.com/Phyton06/UMIBack/internal/db"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		slog.Error("error al cargar configuración", "error", err)
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(parseLogLevel(cfg.LogLevel))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("error al crear pool de conexiones", "error", err)
		os.Exit(1)
	}
	// El pool se cierra explícitamente en el graceful shutdown (línea 70).
	// Sin defer para evitar doble cierre.

	if err := db.RunMigrations(pool); err != nil {
		slog.Error("error al ejecutar migraciones", "error", err)
		os.Exit(1)
	}

	jwtSecret := []byte(cfg.JWTSecret)
	sender := auth.MockSender{}

	// ponytail: two fixed tiers, no per-endpoint config
	authTier := api.RateLimit(5, time.Minute, api.ClientIP)
	generalTier := api.RateLimit(60, time.Minute, api.UserIDFromClaims)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.Handler(pool))
	mux.HandleFunc("POST /auth/register/rider", api.RegisterRider(pool, jwtSecret))
	mux.HandleFunc("POST /auth/register/driver", api.RegisterDriver(pool, jwtSecret))
	mux.Handle("POST /auth/request-otp", authTier(http.HandlerFunc(api.RequestOTP(pool, sender))))
	mux.Handle("POST /auth/verify-otp", authTier(http.HandlerFunc(api.VerifyOTP(pool, jwtSecret))))
	mux.HandleFunc("POST /auth/refresh", api.RefreshToken(pool, jwtSecret))
	mux.Handle("POST /auth/logout", auth.Auth(jwtSecret)(generalTier(api.Logout(pool))))
	mux.Handle("POST /rides", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider")(http.HandlerFunc(api.CreateRide(pool))))))
	mux.Handle("GET /rides/{id}", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider", "driver")(http.HandlerFunc(api.GetRide(pool))))))
	mux.Handle("GET /rides", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider", "driver")(http.HandlerFunc(api.ListRides(pool))))))
	mux.Handle("PATCH /rides/{id}/cancel", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider", "driver")(http.HandlerFunc(api.CancelRide(pool))))))
	mux.Handle("PATCH /rides/{id}/accept", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.AcceptRide(pool))))))
	mux.Handle("PATCH /rides/{id}/en-route", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.EnRouteRide(pool))))))
	mux.Handle("PATCH /rides/{id}/arrived", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.ArrivedRide(pool))))))
	mux.Handle("PATCH /rides/{id}/start", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.StartRide(pool))))))
	mux.Handle("PATCH /rides/{id}/complete", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.CompleteRide(pool, cfg.FareRatePerKm, cfg.FareMinimum))))))
	mux.Handle("POST /rides/estimate", auth.Auth(jwtSecret)(generalTier(http.HandlerFunc(api.EstimateRide(pool, cfg.FareRatePerKm, cfg.FareMinimum)))))
	mux.Handle("PATCH /drivers/location", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.UpdateLocation(pool))))))
	mux.Handle("PATCH /drivers/availability", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.ToggleAvailability(pool))))))
	mux.Handle("GET /drivers/rides", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("driver")(http.HandlerFunc(api.DriverRides(pool))))))
	mux.Handle("GET /drivers/nearby", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider")(http.HandlerFunc(api.NearbyDrivers(pool))))))
	// ponytail: generalTier for rider endpoints matches the existing pattern
	mux.Handle("GET /rider/stats", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider")(http.HandlerFunc(api.RiderStats(pool))))))
	mux.Handle("GET /rider/rides", auth.Auth(jwtSecret)(generalTier(auth.RequireRole("rider")(http.HandlerFunc(api.RiderRides(pool))))))

	// Sweep stale rate-limit entries every 5 minutes
	api.StartRateLimitSweeper(context.Background(), 5*time.Minute)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para la señal de cierre
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine para graceful shutdown
	go func() {
		sig := <-quit
		slog.Info("señal de cierre recibida, cerrando servidor...", "señal", sig)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("error al cerrar servidor HTTP", "error", err)
		}

		pool.Close()
		slog.Info("servidor cerrado correctamente")
	}()

	slog.Info("iniciando servidor HTTP",
		"puerto", cfg.HTTPPort,
		"nivel_log", cfg.LogLevel,
	)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("error fatal en servidor HTTP", "error", err)
		os.Exit(1)
	}

	slog.Info("servidor finalizado")
}

// parseLogLevel convierte un string a slog.Level.
// Valores válidos: debug, info, warn, error.
// Por omisión retorna slog.LevelInfo.
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
