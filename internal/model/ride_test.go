package model

import "testing"

// testCase estructura para pruebas de transiciones.
type testCase struct {
	name     string
	current  RideStatus
	target   RideStatus
	opts     []TransitionOption
	expected bool
}

func TestCanTransitionTo(t *testing.T) {
	tests := []testCase{
		// === Happy path: ciclo completo ===
		{name: "REQUESTED → ACCEPTED", current: StatusRequested, target: StatusAccepted, expected: true},
		{name: "ACCEPTED → EN_ROUTE", current: StatusAccepted, target: StatusEnRoute, expected: true},
		{name: "EN_ROUTE → ARRIVED", current: StatusEnRoute, target: StatusArrived, expected: true},
		{name: "ARRIVED → IN_PROGRESS", current: StatusArrived, target: StatusInProgress, expected: true},
		{name: "IN_PROGRESS → COMPLETED", current: StatusInProgress, target: StatusCompleted, expected: true},

		// === Cancelaciones desde estados no terminales ===
		{name: "REQUESTED → CANCELLED", current: StatusRequested, target: StatusCancelled, expected: true},
		{name: "ACCEPTED → CANCELLED", current: StatusAccepted, target: StatusCancelled, expected: true},
		{name: "EN_ROUTE → CANCELLED", current: StatusEnRoute, target: StatusCancelled, expected: true},
		{name: "ARRIVED → CANCELLED", current: StatusArrived, target: StatusCancelled, expected: true},

		// === Estados terminales: sin transiciones de salida ===
		{name: "COMPLETED → IN_PROGRESS rechazado", current: StatusCompleted, target: StatusInProgress, expected: false},
		{name: "COMPLETED → CANCELLED rechazado", current: StatusCompleted, target: StatusCancelled, expected: false},
		{name: "CANCELLED → REQUESTED rechazado", current: StatusCancelled, target: StatusRequested, expected: false},

		// === Compuerta IN_PROGRESS → CANCELLED ===
		{name: "IN_PROGRESS → CANCELLED sin SystemAction rechazado", current: StatusInProgress, target: StatusCancelled, expected: false},
		{name: "IN_PROGRESS → CANCELLED con SystemAction aceptado", current: StatusInProgress, target: StatusCancelled, opts: []TransitionOption{WithSystemAction()}, expected: true},

		// === Transiciones inválidas ===
		{name: "REQUESTED → IN_PROGRESS rechazado (salto)", current: StatusRequested, target: StatusInProgress, expected: false},
		{name: "ACCEPTED → COMPLETED rechazado (salto)", current: StatusAccepted, target: StatusCompleted, expected: false},
		{name: "REQUESTED → REQUESTED autoreferencia", current: StatusRequested, target: StatusRequested, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.current.CanTransitionTo(tc.target, tc.opts...)
			if got != tc.expected {
				t.Errorf("%s → %s: esperado %v, obtenido %v",
					tc.current, tc.target, tc.expected, got)
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
		if !current.CanTransitionTo(next) {
			t.Errorf("transición %s → %s debería ser válida", current, next)
		}
	}
}
