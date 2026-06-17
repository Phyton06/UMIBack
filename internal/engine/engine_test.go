package engine

import (
	"testing"

	"github.com/Phyton06/UMIBack/internal/model"
)

func TestCanTransition_DelegaAlModelo(t *testing.T) {
	tests := []struct {
		name    string
		current model.RideStatus
		target  model.RideStatus
		wantErr bool
	}{
		{name: "valida: REQUESTED → ACCEPTED", current: model.StatusRequested, target: model.StatusAccepted, wantErr: false},
		{name: "invalida: COMPLETED → IN_PROGRESS", current: model.StatusCompleted, target: model.StatusInProgress, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CanTransition(tc.current, tc.target)
			if tc.wantErr && err == nil {
				t.Errorf("%s → %s: esperado error, obtenido nil", tc.current, tc.target)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("%s → %s: esperado nil, obtenido %v", tc.current, tc.target, err)
			}
		})
	}
}

func TestValidTransitions_DelegaAlModelo(t *testing.T) {
	got := ValidTransitions(model.StatusRequested)
	expected := []model.RideStatus{model.StatusAccepted, model.StatusCancelled}

	if len(got) != len(expected) {
		t.Fatalf("ValidTransitions(Requested) = %v, esperado %v", got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("ValidTransitions(Requested)[%d] = %s, esperado %s", i, got[i], expected[i])
		}
	}
}

func TestTransitionMatrix_Expuesta(t *testing.T) {
	if _, ok := TransitionMatrix[model.StatusRequested]; !ok {
		t.Error("TransitionMatrix no contiene StatusRequested")
	}
}
