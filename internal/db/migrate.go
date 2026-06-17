package db

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations ejecuta las migraciones pendientes usando golang-migrate.
// Las migraciones están embebidas en el binario via go:embed.
func RunMigrations(pool *pgxpool.Pool) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("error al crear fuente de migraciones: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	dbDriver, err := migratepgx.WithInstance(sqlDB, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("error al crear driver de base de datos: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx_v5", dbDriver)
	if err != nil {
		return fmt.Errorf("error al crear instancia de migración: %w", err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("no hay migraciones pendientes")
			return nil
		}
		return fmt.Errorf("error al ejecutar migraciones: %w", err)
	}

	slog.Info("migraciones ejecutadas exitosamente")
	return nil
}
