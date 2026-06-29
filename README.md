# UMIBack

Backend Go para la plataforma de ride-hailing UMI. Autenticación OTP/JWT, ciclo de vida de viajes con state machine, tracking GPS vía PostGIS y panel de administración.

## Quick path

```bash
cp .env.example .env    # Configurar DB_URL, JWT_SECRET, PORT
make run                 # Inicia el servidor en :8080
curl localhost:8080/health
```

## Stack

| Herramienta | Uso |
|-------------|-----|
| Go 1.26 | Lenguaje |
| pgx/v5 | Driver PostgreSQL + pool de conexiones |
| PostGIS | Consultas geoespaciales (ST_DWithin, ST_MakePoint) |
| golang-jwt/jwt/v5 | Firma y validación de JWT (HS256) |
| golang-migrate/v4 | Migraciones de esquema (embebidas vía `embed`) |
| crypto/rand + sha256 | OTP seguro + hashing |

## Requisitos

- Go 1.26+
- PostgreSQL 16+ con extensión PostGIS
- Variables de entorno: `DB_URL`, `JWT_SECRET`, `PORT` (default 8080), `LOG_LEVEL` (default info)

## Arquitectura

```
cmd/server/main.go          # Punto de entrada, wiring de middlewares y rutas
internal/
├── api/                    # Handlers HTTP
│   ├── auth.go             # Registro, OTP, JWT, refresh, logout
│   ├── ride.go             # CRUD + ciclo de vida del viaje
│   ├── driver.go           # Ubicación, disponibilidad, conductores cercanos
│   ├── rider.go            # Dashboard y historial del pasajero
│   ├── admin.go            # CRUD de conductores/pasajeros, membresías
│   ├── middleware.go        # RequireJSON, CORS
│   ├── ratelimit.go         # Token bucket rate limiter
│   └── health.go            # Health check
├── auth/                   # JWT, OTP, SMS (mock), middleware Auth + RequireRole
├── config/                 # Carga de variables de entorno
├── db/                     # Pool PostgreSQL, migraciones, health check
└── model/                  # Ride (state machine), User, Driver, Admin, RefreshToken
```

## API

| Método | Ruta | Auth | Descripción |
|--------|------|:---:|-------------|
| `GET` | `/health` | — | Health check (DB + server) |
| `POST` | `/auth/register/{rider,driver,admin}` | — | Registro |
| `POST` | `/auth/request-otp` | Rate limit | Solicitar código OTP |
| `POST` | `/auth/verify-otp` | Rate limit | Verificar OTP → JWT |
| `POST` | `/auth/refresh` | — | Refrescar access token |
| `POST` | `/auth/logout` | JWT | Revocar refresh token |
| `POST` | `/rides` | JWT (rider) | Crear viaje |
| `GET` | `/rides/{id}` | JWT | Ver viaje |
| `GET` | `/rides` | JWT | Listar viajes |
| `PATCH` | `/rides/{id}/accept` | JWT (driver) | Aceptar viaje |
| `PATCH` | `/rides/{id}/en-route` | JWT (driver) | En camino |
| `PATCH` | `/rides/{id}/arrived` | JWT (driver) | Llegó al punto |
| `PATCH` | `/rides/{id}/start` | JWT (driver) | Iniciar viaje |
| `PATCH` | `/rides/{id}/complete` | JWT (driver) | Completar viaje |
| `PATCH` | `/rides/{id}/cancel` | JWT | Cancelar viaje |
| `POST` | `/rides/estimate` | JWT | Estimar tarifa |
| `PATCH` | `/drivers/location` | JWT (driver) | Actualizar ubicación GPS |
| `PATCH` | `/drivers/availability` | JWT (driver) | Toggle online/offline |
| `GET` | `/drivers/nearby` | JWT (rider) | Conductores cercanos (PostGIS) |
| `GET` | `/drivers/rides` | JWT (driver) | Historial del conductor |
| `GET/POST` | `/admin/*` | JWT (admin) | Panel de administración |

## Makefile

```bash
make build        # go build -o bin/server ./cmd/server
make run          # go run ./cmd/server
make test         # go test ./... -v -count=1
make tidy         # go mod tidy
make migrate-up   # Inicia el servidor (ejecuta migraciones automáticamente)
make migrate-down # Requiere golang-migrate CLI
```

## Docker

```bash
docker compose up -d postgres    # PostgreSQL 17 + PostGIS en :5432
make run                         # Inicia el backend
```

## Tests

```bash
make test    # 5 paquetes, cobertura en api/auth/config/db/model
```

## Verificación

- [ ] `make build` compila sin errores
- [ ] `make test` todo verde
- [ ] `curl localhost:8080/health` → `{"status":"ok","database":"connected"}`
- [ ] `curl -X POST localhost:8080/auth/request-otp -H "Content-Type: application/json" -d '{"phone":"5512345678"}'` → 200
