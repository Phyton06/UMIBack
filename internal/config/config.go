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
	JWTSecret   string
}

// NewConfig lee las variables de entorno y retorna una Config validada.
//
// Variables de entorno esperadas:
//   - PORT:                puerto HTTP (por omisión: "8080")
//   - DB_URL:              cadena de conexión a PostgreSQL (requerida)
//   - LOG_LEVEL:           nivel de registro (por omisión: "info")
//   - JWT_SECRET:          clave secreta para firmar JWT (requerida)
//   - JWT_EXPIRY_ACCESS:   duración del access token (por omisión: "15m")
//   - JWT_EXPIRY_REFRESH:  duración del refresh token (por omisión: "168h")
//   - OTP_LENGTH:          cantidad de dígitos del OTP (por omisión: 6)
//   - OTP_TTL:             tiempo de vida del OTP (por omisión: "5m")
//   - SMS_MOCK:            usa MockSender en lugar de SMS real (por omisión: true)
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET es requerida")
	}

	return Config{
		HTTPPort:    port,
		DatabaseURL: dbURL,
		LogLevel:    logLevel,
		JWTSecret:   jwtSecret,
	}, nil
}

// ponytail: only JWT_SECRET is used from the new fields; OTP/expiry/SMS are hardcoded in handlers
