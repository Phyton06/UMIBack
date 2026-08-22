# UMI Backend

[![CI](https://github.com/Phyton06/UMIBack/actions/workflows/ci.yml/badge.svg)](https://github.com/Phyton06/UMIBack/actions/workflows/ci.yml)

Backend in Go for the UMI ride-hailing platform — real-time geolocation via PostGIS, OTP/JWT authentication, and AWS deployment with Terraform.

## Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.26 |
| Database | PostgreSQL 17 + PostGIS |
| DB Driver | pgx/v5 |
| Auth | JWT (HS256) + OTP via SMS |
| SMS | AWS SNS / Twilio / LogSender (dev) |
| Migrations | golang-migrate (embedded via `go:embed`) |
| Infrastructure | Terraform (AWS ECS Fargate, RDS Multi-AZ, ALB, S3/CloudFront) |
| Container | Docker multi-stage (alpine:3.21, ~15MB) |

## Architecture

```
cmd/server/main.go          → Entry point, routes, graceful shutdown
internal/
  api/                      → HTTP handlers (35 endpoints)
  auth/                     → JWT, OTP, middleware, SMS providers
  config/                   → Environment variables
  db/                       → pgx pool, embedded migrations
  model/                    → Domain types, ride state machine
terraform/                  → Full AWS IaC
```

## API Endpoints (35)

### Health
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Database ping |

### Authentication
| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/register/rider` | Register rider |
| POST | `/auth/register/driver` | Register driver |
| POST | `/auth/register/admin` | Register admin |
| POST | `/auth/request-otp` | Request OTP (5/min/IP) |
| POST | `/auth/verify-otp` | Verify OTP → JWT pair |
| POST | `/auth/refresh` | Rotate refresh token |
| POST | `/auth/logout` | Revoke all refresh tokens |

### Rides
| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| POST | `/rides` | rider | Create ride with coordinates |
| GET | `/rides/{id}` | rider/driver | Ride details |
| GET | `/rides` | rider/driver | List rides |
| PATCH | `/rides/{id}/accept` | driver | Accept ride |
| PATCH | `/rides/{id}/en-route` | driver | En route to pickup |
| PATCH | `/rides/{id}/arrived` | driver | Arrived at pickup |
| PATCH | `/rides/{id}/start` | driver | Start ride |
| PATCH | `/rides/{id}/complete` | driver | Complete + calculate fare |
| PATCH | `/rides/{id}/cancel` | rider/driver | Cancel |
| POST | `/rides/estimate` | JWT | Estimate fare |

### Drivers
| Method | Path | Description |
|--------|------|-------------|
| PATCH | `/drivers/location` | Update GPS |
| PATCH | `/drivers/availability` | Toggle online/offline |
| GET | `/drivers/nearby` | Nearby drivers (PostGIS ST_DWithin) |
| GET | `/drivers/rides` | Driver ride history |

### Riders
| Method | Path | Description |
|--------|------|-------------|
| GET | `/rider/stats` | Aggregated stats |
| GET | `/rider/rides` | Paginated ride history |

### Admin (13 endpoints)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/dashboard/stats` | Dashboard metrics |
| GET | `/admin/rides/active` | Active rides with GPS for live map |
| GET/POST/PATCH | `/admin/drivers/*` | CRUD + suspend + membership |
| GET/POST/PATCH | `/admin/passengers/*` | CRUD + suspend + ban + history |

## Ride State Machine

```
REQUESTED → ACCEPTED → EN_ROUTE → ARRIVED → STARTED → COMPLETED
    ↓           ↓          ↓          ↓         ↓
  CANCELLED  CANCELLED  CANCELLED  CANCELLED  CANCELLED
```

Each transition uses `UPDATE ... WHERE status = $expected` to prevent race conditions. Conflict → 409.

## Configuration

```bash
# Required
DB_URL=postgres://user:pass@localhost:5432/umi?sslmode=disable
JWT_SECRET=your-secret-here

# Optional
PORT=8080
LOG_LEVEL=info
FARE_RATE_PER_KM=8.0
FARE_MINIMUM=25.0
SMS_PROVIDER=log          # log | twilio | aws-sns
DEV_MODE=true             # OTP 000000/123456 bypass verification
```

## Getting Started

```bash
# Local development
cp .env.example .env
docker compose up -d postgres    # PostGIS 17 on :5432
make run                         # go run ./cmd/server

# Build
make build                       # → bin/server

# Tests
make test                        # go test ./... -v -count=1

# Production
cd terraform && terraform init && terraform apply
```

## Tests

14 test files using `pgxmock/v3` (no real DB for unit tests):

| Package | What it tests |
|---------|--------------|
| `api` | All handlers (auth, rides, drivers, riders, admin, health, rate-limit) |
| `auth` | JWT sign/validate, OTP gen/hash, middleware, SMS normalization |
| `config` | Env var parsing, defaults |
| `db` | Migration logic |
| `model` | Valid state machine transitions |

## CI/CD

| Workflow | Trigger | Action |
|----------|---------|--------|
| `ci.yml` | Push to `main`/`desarrollo/**`, PRs | tidy → build → vet → test |
| `deploy-backend.yml` | Push to `main` (.go, Dockerfile, go.mod changes) | Docker build → ECR → ECS deploy |
| `keepalive.yml` | Cron Mon+Fri 8:00 UTC | Keep-alive endpoint |

## Infrastructure (Terraform)

- **VPC** with 2 AZs, public/private subnets
- **ECS Fargate** with auto-scaling
- **RDS PostgreSQL 17** Multi-AZ with automatic failover
- **ALB** with health checks
- **S3 + CloudFront** for static assets
- **Secrets Manager** for credentials
- **Estimated cost**: ~$48/month

## Technical Decisions

1. **Stdlib HTTP router** (`net/http.ServeMux` Go 1.22+) — no routing dependencies
2. **No ORM** — raw SQL with pgx, `Pool` interface for testability
3. **Embedded migrations** — SQL files in binary via `go:embed`
4. **Pluggable SMS** — `Sender` interface with 3 implementations
5. **Fare via PostGIS** — `ST_DDistance` on geography type for meter-accurate distance
6. **Phone normalization** — Mexican E.164 format (10-digit → +52)

---

<details>
<summary>🇪🇸 Español</summary>

## UMI Backend

Backend en Go para la plataforma de ride-hailing UMI — con geolocalización en tiempo real vía PostGIS, autenticación OTP/JWT y despliegue en AWS con Terraform.

### Stack

| Componente | Tecnología |
|------------|-----------|
| Lenguaje | Go 1.26 |
| Base de datos | PostgreSQL 17 + PostGIS |
| Driver DB | pgx/v5 |
| Auth | JWT (HS256) + OTP por SMS |
| SMS | AWS SNS / Twilio / LogSender (dev) |
| Migraciones | golang-migrate (embebidas con `go:embed`) |
| Infraestructura | Terraform (AWS ECS Fargate, RDS Multi-AZ, ALB, S3/CloudFront) |
| Contenedor | Docker multi-stage (alpine:3.21, ~15MB) |

### Arquitectura

```
cmd/server/main.go          → Entry point, rutas, graceful shutdown
internal/
  api/                      → HTTP handlers (35 endpoints)
  auth/                     → JWT, OTP, middleware, SMS providers
  config/                   → Variables de entorno
  db/                       → Pool pgx, migraciones embebidas
  model/                    → Tipos de dominio, state machine de viajes
terraform/                  → IaC completa para AWS
```

### API Endpoints (35)

#### Salud
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/health` | Ping a la base de datos |

#### Autenticación
| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/auth/register/rider` | Registrar pasajero |
| POST | `/auth/register/driver` | Registrar conductor |
| POST | `/auth/register/admin` | Registrar admin |
| POST | `/auth/request-otp` | Solicitar OTP (5/min/IP) |
| POST | `/auth/verify-otp` | Verificar OTP → JWT pair |
| POST | `/auth/refresh` | Rotar refresh token |
| POST | `/auth/logout` | Revocar todos los refresh tokens |

#### Viajes
| Método | Ruta | Auth | Descripción |
|--------|------|:----:|-------------|
| POST | `/rides` | rider | Crear viaje con coordenadas |
| GET | `/rides/{id}` | rider/driver | Detalle del viaje |
| GET | `/rides` | rider/driver | Listar viajes |
| PATCH | `/rides/{id}/accept` | driver | Aceptar viaje |
| PATCH | `/rides/{id}/en-route` | driver | En camino al punto |
| PATCH | `/rides/{id}/arrived` | driver | Llegó al punto |
| PATCH | `/rides/{id}/start` | driver | Iniciar viaje |
| PATCH | `/rides/{id}/complete` | driver | Completar + calcular tarifa |
| PATCH | `/rides/{id}/cancel` | rider/driver | Cancelar |
| POST | `/rides/estimate` | JWT | Estimar tarifa |

#### Conductores
| Método | Ruta | Descripción |
|--------|------|-------------|
| PATCH | `/drivers/location` | Actualizar GPS |
| PATCH | `/drivers/availability` | Toggle online/offline |
| GET | `/drivers/nearby` | Conductores cercanos (PostGIS ST_DWithin) |
| GET | `/drivers/rides` | Historial del conductor |

#### Pasajeros
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/rider/stats` | Estadísticas agregadas |
| GET | `/rider/rides` | Historial paginado |

#### Admin (13 endpoints)
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/admin/dashboard/stats` | Métricas del dashboard |
| GET | `/admin/rides/active` | Viajes activos con GPS para mapa |
| GET/POST/PATCH | `/admin/drivers/*` | CRUD + suspensión + membresía |
| GET/POST/PATCH | `/admin/passengers/*` | CRUD + suspensión + ban + historial |

### State Machine de Viajes

```
REQUESTED → ACCEPTED → EN_ROUTE → ARRIVED → STARTED → COMPLETED
    ↓           ↓          ↓          ↓         ↓
  CANCELLED  CANCELLED  CANCELLED  CANCELLED  CANCELLED
```

Cada transición usa `UPDATE ... WHERE status = $expected` para evitar condiciones de carrera. Conflicto → 409.

### Configuración

```bash
# Requerido
DB_URL=postgres://user:pass@localhost:5432/umi?sslmode=disable
JWT_SECRET=tu-secreto-aqui

# Opcional
PORT=8080
LOG_LEVEL=info
FARE_RATE_PER_KM=8.0
FARE_MINIMUM=25.0
SMS_PROVIDER=log          # log | twilio | aws-sns
DEV_MODE=true             # OTP 000000/123456 sin verificación
```

### Ejecutar

```bash
# Desarrollo local
cp .env.example .env
docker compose up -d postgres    # PostGIS 17 en :5432
make run                         # go run ./cmd/server

# Build
make build                       # → bin/server

# Tests
make test                        # go test ./... -v -count=1

# Producción
cd terraform && terraform init && terraform apply
```

### Tests

14 archivos de test usando `pgxmock/v3` (sin DB real para unit tests):

| Paquete | Qué testea |
|---------|-----------|
| `api` | Todos los handlers (auth, rides, drivers, riders, admin, health, rate-limit) |
| `auth` | JWT sign/validate, OTP gen/hash, middleware, normalización de SMS |
| `config` | Parsing de env vars, defaults |
| `db` | Lógica de migraciones |
| `model` | Transiciones válidas del state machine |

### CI/CD

| Workflow | Trigger | Acción |
|----------|---------|--------|
| `ci.yml` | Push a `main`/`desarrollo/**`, PRs | tidy → build → vet → test |
| `deploy-backend.yml` | Push a `main` (cambios en .go, Dockerfile, go.mod) | Docker build → ECR → ECS deploy |
| `keepalive.yml` | Cron Lun+Vie 8:00 UTC | Keep-alive del endpoint |

### Infraestructura (Terraform)

- **VPC** con 2 AZs, subnets públicas/privadas
- **ECS Fargate** con auto-scaling
- **RDS PostgreSQL 17** Multi-AZ con failover automático
- **ALB** con health checks
- **S3 + CloudFront** para assets estáticos
- **Secrets Manager** para credenciales
- **Costo estimado**: ~$48/mes

### Decisiones Técnicas

1. **Stdlib HTTP router** (`net/http.ServeMux` Go 1.22+) — sin dependencias de routing
2. **Sin ORM** — SQL crudo con pgx, interfaz `Pool` para testabilidad
3. **Migraciones embebidas** — archivos SQL en el binario vía `go:embed`
4. **SMS pluggable** — interfaz `Sender` con 3 implementaciones
5. **Tarifa vía PostGIS** — `ST_DDistance` en tipo geography para metros exactos
6. **Phone normalization** — formato E.164 mexicano (10 dígitos → +52)

</details>
