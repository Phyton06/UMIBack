package model

import (
	"strings"
	"testing"
)

// testCase estructura para pruebas de transiciones.
type testCase struct {
	name    string
	current RideStatus
	target  RideStatus
	wantErr bool
	wantMsg string
}

func TestCanTransitionTo(t *testing.T) {
	tests := []testCase{
		// === Happy path: ciclo completo ===
		{name: "REQUESTED → ACCEPTED", current: StatusRequested, target: StatusAccepted, wantErr: false},
		{name: "ACCEPTED → EN_ROUTE", current: StatusAccepted, target: StatusEnRoute, wantErr: false},
		{name: "EN_ROUTE → ARRIVED", current: StatusEnRoute, target: StatusArrived, wantErr: false},
		{name: "ARRIVED → IN_PROGRESS", current: StatusArrived, target: StatusInProgress, wantErr: false},
		{name: "IN_PROGRESS → COMPLETED", current: StatusInProgress, target: StatusCompleted, wantErr: false},

		// === Cancelaciones desde estados no terminales ===
		{name: "REQUESTED → CANCELLED", current: StatusRequested, target: StatusCancelled, wantErr: false},
		{name: "ACCEPTED → CANCELLED", current: StatusAccepted, target: StatusCancelled, wantErr: false},
		{name: "EN_ROUTE → CANCELLED", current: StatusEnRoute, target: StatusCancelled, wantErr: false},
		{name: "ARRIVED → CANCELLED", current: StatusArrived, target: StatusCancelled, wantErr: false},
		{name: "IN_PROGRESS → CANCELLED", current: StatusInProgress, target: StatusCancelled, wantErr: false},

		// === Estados terminales: sin transiciones de salida ===
		{name: "COMPLETED → IN_PROGRESS rechazado", current: StatusCompleted, target: StatusInProgress, wantErr: true, wantMsg: "terminal"},
		{name: "COMPLETED → CANCELLED rechazado", current: StatusCompleted, target: StatusCancelled, wantErr: true, wantMsg: "terminal"},
		{name: "CANCELLED → REQUESTED rechazado", current: StatusCancelled, target: StatusRequested, wantErr: true, wantMsg: "terminal"},

		// === Transiciones inválidas ===
		{name: "REQUESTED → IN_PROGRESS rechazado (salto)", current: StatusRequested, target: StatusInProgress, wantErr: true, wantMsg: "no permitida"},
		{name: "ACCEPTED → COMPLETED rechazado (salto)", current: StatusAccepted, target: StatusCompleted, wantErr: true, wantMsg: "no permitida"},
		{name: "REQUESTED → REQUESTED autoreferencia", current: StatusRequested, target: StatusRequested, wantErr: true, wantMsg: "no permitida"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.current.CanTransitionTo(tc.target)
			if tc.wantErr && err == nil {
				t.Errorf("%s → %s: esperado error, obtenido nil", tc.current, tc.target)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("%s → %s: esperado nil, obtenido %v", tc.current, tc.target, err)
			}
			if tc.wantErr && tc.wantMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("mensaje de error '%s' no contiene '%s'", err.Error(), tc.wantMsg)
				}
			}
		})
	}
}

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		status   RideStatus
		expected []RideStatus
	}{
		{status: StatusRequested, expected: []RideStatus{StatusAccepted, StatusCancelled}},
		{status: StatusAccepted, expected: []RideStatus{StatusEnRoute, StatusCancelled}},
		{status: StatusEnRoute, expected: []RideStatus{StatusArrived, StatusCancelled}},
		{status: StatusArrived, expected: []RideStatus{StatusInProgress, StatusCancelled}},
		{status: StatusInProgress, expected: []RideStatus{StatusCompleted, StatusCancelled}},
		{status: StatusCompleted, expected: nil},
		{status: StatusCancelled, expected: nil},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			got := tc.status.ValidTransitions()
			if !sameElements(tc.expected, got) {
				t.Errorf("%s: esperado %v, obtenido %v", tc.status, tc.expected, got)
			}
		})
	}
}

// sameElements compara dos slices de RideStatus sin importar el orden.
func sameElements(a, b []RideStatus) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[RideStatus]int)
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status   RideStatus
		expected bool
	}{
		{status: StatusRequested, expected: false},
		{status: StatusAccepted, expected: false},
		{status: StatusEnRoute, expected: false},
		{status: StatusArrived, expected: false},
		{status: StatusInProgress, expected: false},
		{status: StatusCompleted, expected: true},
		{status: StatusCancelled, expected: true},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			got := tc.status.IsTerminal()
			if got != tc.expected {
				t.Errorf("%s: IsTerminal esperado %v, obtenido %v", tc.status, tc.expected, got)
			}
		})
	}
}

// TestHappyPathFullCycle verifica que el ciclo completo REQUESTED → COMPLETED
// sea aceptado paso a paso (Escenario 1 de la especificación).
func TestHappyPathFullCycle(t *testing.T) {
	path := []RideStatus{StatusRequested, StatusAccepted, StatusEnRoute, StatusArrived, StatusInProgress, StatusCompleted}
	for i := 0; i < len(path)-1; i++ {
		current := path[i]
		next := path[i+1]
		if err := current.CanTransitionTo(next); err != nil {
			t.Errorf("transición %s → %s debería ser válida, error: %v", current, next, err)
		}
	}
}
