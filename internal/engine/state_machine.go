// Package engine proporciona la lógica de negocio del sistema UMI.
package engine

import "github.com/Phyton06/UMIBack/internal/model"

// TransitionMatrix expone el mapa de transiciones válidas para
// consulta externa. Cada entrada mapea un estado de origen a los
// estados destino permitidos.
//
// Los estados terminales (COMPLETED, CANCELLED) no aparecen como
// origen porque no admiten transiciones de salida.
var TransitionMatrix = map[model.RideStatus][]model.RideStatus{
	model.StatusRequested:  {model.StatusAccepted, model.StatusCancelled},
	model.StatusAccepted:   {model.StatusEnRoute, model.StatusCancelled},
	model.StatusEnRoute:    {model.StatusArrived, model.StatusCancelled},
	model.StatusArrived:    {model.StatusInProgress, model.StatusCancelled},
	model.StatusInProgress: {model.StatusCompleted, model.StatusCancelled},
}

// ponytail: esta función y ValidTransitions delegan al modelo.
// El mapa TransitionMatrix se mantiene aquí para que consumidores
// externos (middleware, handlers) puedan inspeccionar la máquina
// sin importar model. Si ningún consumidor lo necesita, unificar
// todo en model.

// CanTransition valida si la transición de current a target es
// válida, aplicando las opciones proporcionadas.
func CanTransition(current, target model.RideStatus, opts ...model.TransitionOption) bool {
	return current.CanTransitionTo(target, opts...)
}

// ValidTransitions retorna los destinos válidos desde el estado s.
func ValidTransitions(s model.RideStatus) []model.RideStatus {
	return s.ValidTransitions()
}
