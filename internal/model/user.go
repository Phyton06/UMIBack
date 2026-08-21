// Package model define las estructuras de dominio del sistema UMI.
package model

import (
	"time"

	"github.com/google/uuid"
)

// User representa un usuario del sistema (rider o driver).
type User struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	Phone            string     `json:"phone"`
	SuspendedUntil   *time.Time `json:"suspended_until,omitempty"`
	SuspensionReason *string    `json:"suspension_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
