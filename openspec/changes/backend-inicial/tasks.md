# Tasks: backend-inicial

## Phase 1: Configuración y Modelos de Dominio (Work Unit 1) — COMPLETED

- [x] 1.1 Crear `internal/config/config.go` con Config y NewConfig
- [x] 1.2 Crear modelo `User` en `internal/model/user.go`
- [x] 1.3 Crear modelo `Driver` en `internal/model/driver.go`
- [x] 1.4 Crear modelo `Ride` con máquina de estados en `internal/model/ride.go`
- [x] 1.5 Crear `internal/engine/state_machine.go` con delegación a model
- [x] 1.6 Escribir pruebas de transiciones en `internal/model/ride_test.go`

## Phase 2: Base de Datos y Pool de Conexiones (Work Unit 2) — COMPLETED

- [x] 2.1 Crear `internal/db/migrations/000001_init.up.sql` con tablas users, drivers, rides + PostGIS
- [x] 2.2 Crear `internal/db/migrations/000001_init.down.sql` con DROP TABLE
- [x] 2.3 Crear `internal/db/postgres.go` con NewPool y HealthCheck usando pgx/v5

## Phase 3: Servidor HTTP y Arranque (Work Unit 3) — PENDING

- [ ] 3.1 Crear `cmd/server/main.go` con inicialización de pool, servidor HTTP y graceful shutdown
- [ ] 3.2 Crear `Makefile` con objetivos build, test, run, migrate
- [ ] 3.3 Crear `internal/server/server.go` con router y registro de rutas

## Phase 4: Handlers de API REST (Work Unit 4) — PENDING

- [ ] 4.1 Implementar handler POST /api/users (registro de usuario)
- [ ] 4.2 Implementar handler POST /api/drivers (registro de conductor)
- [ ] 4.3 Implementar handler POST /api/rides (solicitar viaje)
- [ ] 4.4 Implementar handler PATCH /api/rides/{id}/status (transición de estado)
