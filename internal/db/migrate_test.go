package db

import (
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestMigrationsFS_SePuedenLeer(t *testing.T) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("error al abrir migraciones embebidas: %v", err)
	}
	defer src.Close()

	version, err := src.First()
	if err != nil {
		t.Fatalf("no se encontraron migraciones: %v", err)
	}

	t.Logf("primera migración: versión %d", version)
}
