package model

import (
	"time"

	"github.com/google/uuid"
)

// Admin representa un administrador del sistema.
type Admin struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
