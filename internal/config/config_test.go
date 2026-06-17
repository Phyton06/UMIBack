package config

import (
	"os"
	"testing"
)

func TestNewConfig_UsaVariablesDeEntorno(t *testing.T) {
	os.Setenv("DB_URL", "postgres://localhost:5432/test")
	os.Setenv("PORT", "9000")
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("DB_URL")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("LOG_LEVEL")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error inesperado: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost:5432/test" {
		t.Errorf("DatabaseURL = %q, esperado %q", cfg.DatabaseURL, "postgres://localhost:5432/test")
	}
	if cfg.HTTPPort != "9000" {
		t.Errorf("HTTPPort = %q, esperado %q", cfg.HTTPPort, "9000")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, esperado %q", cfg.LogLevel, "debug")
	}
}

func TestNewConfig_ValoresPorOmision(t *testing.T) {
	os.Setenv("DB_URL", "postgres://localhost:5432/test")
	defer os.Unsetenv("DB_URL")
	os.Unsetenv("PORT")
	os.Unsetenv("LOG_LEVEL")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error inesperado: %v", err)
	}

	if cfg.HTTPPort != "8080" {
		t.Errorf("HTTPPort por omision = %q, esperado %q", cfg.HTTPPort, "8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel por omision = %q, esperado %q", cfg.LogLevel, "info")
	}
}

func TestNewConfig_DBURLRequerida(t *testing.T) {
	os.Unsetenv("DB_URL")
	_, err := NewConfig()
	if err == nil {
		t.Fatal("NewConfig() esperado error, obtenido nil")
	}
}
