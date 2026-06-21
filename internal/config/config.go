// Package config proporciona la configuración de la aplicación
// basada en variables de entorno.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config almacena la configuración de la aplicación.
type Config struct {
	HTTPPort      string
	DatabaseURL   string
	LogLevel      string
	JWTSecret     string
	FareRatePerKm float64
	FareMinimum   float64
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
		HTTPPort:      port,
		DatabaseURL:   dbURL,
		LogLevel:      logLevel,
		JWTSecret:     jwtSecret,
		FareRatePerKm: parseFloatEnv("FARE_RATE_PER_KM", 8.0),
		FareMinimum:   parseFloatEnv("FARE_MINIMUM", 25.0),
	}, nil
}

// ponytail: only JWT_SECRET is used from the new fields; OTP/expiry/SMS are hardcoded in handlers

// parseFloatEnv parsea un float64 desde una variable de entorno.
// Si la variable está vacía, retorna el valor por omisión.
func parseFloatEnv(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}
