// Package db proporciona la conexión a PostgreSQL y utilerías de base de datos.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool crea un pool de conexiones PostgreSQL usando pgx/v5.
// La cadena dsn debe ser una URL de conexión válida de PostgreSQL.
//
// Configuración del pool:
//   - MaxConns: 10
//   - MinConns:  2
//   - MaxConnLifetime: 30 minutos
//   - HealthCheckPeriod: 5 minutos
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("error al analizar DSN: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error al crear pool: %w", err)
	}

	slog.Info("pool de conexiones creado",
		"max_conns", cfg.MaxConns,
		"min_conns", cfg.MinConns,
		"max_conn_lifetime", cfg.MaxConnLifetime,
	)
	return pool, nil
}

// HealthCheck verifica que la base de datos esté accesible.
// Retorna un error si no se puede alcanzar.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	var lastErr error
	for i := range maxRetries {
		if err := pool.Ping(ctx); err != nil {
			lastErr = fmt.Errorf("intento %d: %w", i+1, err)
			slog.Warn("health check falló", "intento", i+1, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				continue
			}
		}
		slog.Info("health check exitoso")
		return nil
	}

	return fmt.Errorf("health check falló después de %d intentos: %w", maxRetries, lastErr)
}
