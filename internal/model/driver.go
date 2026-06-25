package model

import (
	"time"

	"github.com/google/uuid"
)

// Driver representa un conductor registrado en la plataforma.
type Driver struct {
	ID              uuid.UUID  `json:"id"`
	Phone           string     `json:"phone"`
	Name            string     `json:"name"`
	Location        *string    `json:"location,omitempty"` // WKT Point, ej. "POINT(-99.13 19.43)"
	IsAvailable     bool       `json:"is_available"`
	SuspendedUntil  *time.Time `json:"suspended_until,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
