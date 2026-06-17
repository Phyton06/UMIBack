// Package config proporciona la configuración de la aplicación
// basada en variables de entorno.
package config

import (
	"fmt"
	"os"
)

// Config almacena la configuración de la aplicación.
type Config struct {
	HTTPPort    string
	DatabaseURL string
	LogLevel    string
}

// NewConfig lee las variables de entorno y retorna una Config validada.
//
// Variables de entorno esperadas:
//   - PORT:      puerto HTTP (por omisión: "8080")
//   - DB_URL:    cadena de conexión a PostgreSQL (requerida)
//   - LOG_LEVEL: nivel de registro (por omisión: "info")
func NewConfig() (Config, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DB_URL es requerida")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return Config{
		HTTPPort:    port,
		DatabaseURL: dbURL,
		LogLevel:    logLevel,
	}, nil
}
