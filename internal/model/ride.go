package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RideStatus representa el estado de un viaje.
type RideStatus string

const (
	StatusRequested  RideStatus = "REQUESTED"
	StatusAccepted   RideStatus = "ACCEPTED"
	StatusEnRoute    RideStatus = "EN_ROUTE"
	StatusArrived    RideStatus = "ARRIVED"
	StatusInProgress RideStatus = "IN_PROGRESS"
	StatusCompleted  RideStatus = "COMPLETED"
	StatusCancelled  RideStatus = "CANCELLED"
)

// ErrInvalidTransition se retorna cuando una transición entre estados
// de viaje no es válida.
type ErrInvalidTransition struct {
	Current RideStatus
	Target  RideStatus
	Reason  string
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("transicion invalida: de %s a %s: %s", e.Current, e.Target, e.Reason)
}

// transitionMatrix define las transiciones válidas para cada estado.
// Los estados terminales (COMPLETED, CANCELLED) no aparecen como origen
// porque no tienen transiciones de salida.
var transitionMatrix = map[RideStatus][]RideStatus{
	StatusRequested:  {StatusAccepted, StatusCancelled},
	StatusAccepted:   {StatusEnRoute, StatusCancelled},
	StatusEnRoute:    {StatusArrived, StatusCancelled},
	StatusArrived:    {StatusInProgress, StatusCancelled},
	StatusInProgress: {StatusCompleted, StatusCancelled},
}

// Ride representa un viaje de una plataforma de ride-hailing.
type Ride struct {
	ID                 uuid.UUID  `json:"id"`
	RiderID            uuid.UUID  `json:"rider_id"`
	DriverID           *uuid.UUID `json:"driver_id,omitempty"`
	Status             RideStatus `json:"status"`
	CancellationReason *string    `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// IsTerminal retorna true si el estado es terminal
// (COMPLETED o CANCELLED).
func (s RideStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusCancelled
}

// CanTransitionTo valida si se puede transicionar del estado actual
// al estado destino. Retorna nil si es válida, o un error descriptivo si no.
func (s RideStatus) CanTransitionTo(target RideStatus) error {
	if s.IsTerminal() {
		return &ErrInvalidTransition{
			Current: s, Target: target,
			Reason: "el estado es terminal y no admite transiciones de salida",
		}
	}

	if s == StatusInProgress && target == StatusCancelled {
		return nil
	}

	for _, allowed := range transitionMatrix[s] {
		if allowed == target {
			return nil
		}
	}

	return &ErrInvalidTransition{
		Current: s, Target: target,
		Reason: fmt.Sprintf("transicion no permitida; las validas son: %v", transitionMatrix[s]),
	}
}

// ValidTransitions retorna la lista de estados destino válidos
// desde el estado actual. Retorna nil para estados terminales.
func (s RideStatus) ValidTransitions() []RideStatus {
	if s.IsTerminal() {
		return nil
	}
	return transitionMatrix[s]
}

// CancelableStates retorna los estados desde los cuales se puede
// cancelar un viaje, derivado de transitionMatrix.
func CancelableStates() []RideStatus {
	var states []RideStatus
	for from, tos := range transitionMatrix {
		for _, to := range tos {
			if to == StatusCancelled {
				states = append(states, from)
				break
			}
		}
	}
	return states
}
