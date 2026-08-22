# UMI — System Architecture

## System Design

```
┌──────────────────────────────────────────────────┐
│              Mobile Apps (React Native)           │
│              ┌────────┐     ┌────────┐           │
│              │ Rider  │     │ Driver │           │
│              └───┬────┘     └───┬────┘           │
└──────────────────┼──────────────┼────────────────┘
                   │    HTTPS     │
                   ▼              ▼
┌──────────────────────────────────────────────────┐
│               AWS ECS Fargate                     │
│  ┌────────────────────────────────────────────┐  │
│  │          Go API Server (:8080)              │  │
│  │                                             │  │
│  │  CORS → RequireJSON → RateLimit →          │  │
│  │  Auth(JWT) → RequireRole → Handler         │  │
│  └───────┬──────────┬────────────┬────────────┘  │
│          │          │            │                 │
│  ┌───────▼──┐ ┌─────▼──────┐ ┌──▼──────────┐    │
│  │SMS Sender│ │ pgxpool    │ │  Embedded   │    │
│  │(pluggable│ │(10 conns)  │ │  Migrations │    │
│  └──────────┘ └─────┬──────┘ └─────────────┘    │
└─────────────────────┼───────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────┐
│              AWS RDS PostgreSQL 17 + PostGIS     │
│  ┌───────┐ ┌────────┐ ┌───────┐ ┌───────────┐  │
│  │ users │ │drivers │ │ rides │ │ otp_codes  │  │
│  └───────┘ └────────┘ └───────┘ └───────────┘  │
│  ┌────────────────┐ ┌──────────────────────┐     │
│  │refresh_tokens  │ │suspension_history    │     │
│  └────────────────┘ └──────────────────────┘     │
└─────────────────────────────────────────────────┘
```

## Database Schema

### `users` (riders/passengers)

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, gen_random_uuid() |
| phone | VARCHAR(20) | UNIQUE NOT NULL |
| name | VARCHAR(100) | NOT NULL |
| email | VARCHAR(255) | |
| rating | NUMERIC(2,1) | DEFAULT 0 |
| suspended_until | TIMESTAMPTZ | |
| suspension_reason | TEXT | |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

### `drivers`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, gen_random_uuid() |
| phone | VARCHAR(20) | UNIQUE NOT NULL |
| name | VARCHAR(100) | NOT NULL |
| location | GEOMETRY(Point, 4326) | PostGIS |
| available | BOOLEAN | DEFAULT false |
| membresia_active_until | TIMESTAMPTZ | |
| suspended_until | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

### `rides`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, gen_random_uuid() |
| passenger_id | UUID | FK -> users(id) |
| driver_id | UUID | FK -> drivers(id) |
| status | VARCHAR(20) | DEFAULT 'REQUESTED' |
| pickup_location | GEOMETRY(Point, 4326) | NOT NULL |
| dropoff_location | GEOMETRY(Point, 4326) | NOT NULL |
| pickup_address | TEXT | NOT NULL |
| dropoff_address | TEXT | |
| fare | NUMERIC(10,2) | |
| cancelled_by | VARCHAR(20) | |
| cancelled_at | TIMESTAMPTZ | |
| completed_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

### `otp_codes`

| Column | Type | Constraints |
|--------|------|-------------|
| phone | VARCHAR(20) | PK (composite) |
| role | VARCHAR(10) | PK (composite) |
| code_hash | BYTEA | NOT NULL |
| attempts | SMALLINT | DEFAULT 0 |
| expires_at | TIMESTAMPTZ | NOT NULL |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |

### `refresh_tokens`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, gen_random_uuid() |
| user_id | UUID | NOT NULL |
| role | VARCHAR(10) | NOT NULL |
| token_hash | BYTEA | NOT NULL |
| expires_at | TIMESTAMPTZ | NOT NULL |
| revoked_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |

### `admins`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, gen_random_uuid() |
| phone | VARCHAR(20) | UNIQUE NOT NULL |
| name | VARCHAR(100) | NOT NULL |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

### `suspension_history`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, gen_random_uuid() |
| user_id | UUID | FK -> users(id) |
| suspended_at | TIMESTAMPTZ | DEFAULT NOW() |
| suspended_until | TIMESTAMPTZ | NOT NULL |
| reason | TEXT | |
| unsuspended_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |

### Indexes

| Index | Type | Purpose |
|-------|------|---------|
| idx_drivers_location | GIST | Spatial: nearby drivers via ST_DWithin |
| idx_rides_pickup_location | GIST | Spatial on pickup coordinates |
| idx_rides_dropoff_location | GIST | Spatial on dropoff coordinates |
| idx_refresh_tokens_hash | B-tree | Fast token lookup |
| idx_refresh_tokens_user | B-tree | Token cleanup per user |
| idx_rides_status_created | B-tree | Dashboard active rides |
| idx_drivers_available | B-tree | Availability toggle |
| idx_rides_driver_status | B-tree | Driver ride history |

### Relationships

```
users --1:N-- rides (passenger_id)
drivers --1:N-- rides (driver_id)
users --1:N-- suspension_history
rides --GEOMETRY-- PostGIS spatial indexes
drivers --GEOMETRY-- PostGIS spatial index (ST_DWithin)
```

## Ride State Machine

```
REQUESTED -> ACCEPTED -> EN_ROUTE -> ARRIVED -> IN_PROGRESS -> COMPLETED
    |            |           |           |            |
    +-> CANCELLED <----------<-----------<------------<
```

Each transition uses optimistic locking:

```sql
UPDATE rides SET status = $new_status, ...
WHERE id = $id AND status = $expected_status
RETURNING id
```

If RowsAffected() == 0 -> HTTP 409 Conflict.

### Transition Matrix

| From | To |
|------|-----|
| REQUESTED | ACCEPTED, CANCELLED |
| ACCEPTED | EN_ROUTE, CANCELLED |
| EN_ROUTE | ARRIVED, CANCELLED |
| ARRIVED | IN_PROGRESS, CANCELLED |
| IN_PROGRESS | COMPLETED, CANCELLED |
| COMPLETED | (terminal) |
| CANCELLED | (terminal) |

### Concurrency Safety

- No SELECT ... FOR UPDATE -- purely optimistic concurrency
- Application-level validation (CanTransitionTo()) runs before SQL
- SQL-level guard (WHERE status = $expected) catches race conditions
- Two concurrent requests: one wins, one gets 409

## Concurrency Model

| Component | Pattern | Details |
|-----------|---------|---------|
| HTTP server | goroutine-per-request | Standard Go net/http |
| Connection pool | pgxpool.Pool | MaxConns=10, MinConns=2, MaxConnLifetime=30m |
| Rate limiter | sync.Mutex + token bucket | Global mutex on bucket map |
| Rate limiter cleanup | Background goroutine | Ticker-based, cleans every 5 min |
| Graceful shutdown | os.Signal channel | SIGINT/SIGTERM -> srv.Shutdown(ctx) 15s timeout |
| OTP generation | crypto/rand | Cryptographically secure 6-digit codes |
| OTP verification | Constant-time compare | crypto/subtle.ConstantTimeCompare |
| Refresh token rotation | Breach detection | Revoked token reused -> all tokens deleted |

## Middleware Chain

```
Request
  -> CORS (wraps entire mux)
  -> RequireJSON (Content-Type, 1MB body limit)
  -> RateLimit (5/min auth, 60/min general)
  -> Auth (JWT extraction + validation)
  -> RequireRole (role check)
  -> Handler
```

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Stdlib HTTP router (Go 1.22+) | Zero dependencies, sufficient for 35 endpoints |
| No ORM | Raw SQL with pgx, Pool interface for testability |
| Embedded migrations | SQL files in binary via go:embed, no external files |
| Pluggable SMS | Sender interface with 3 implementations (Log/Twilio/AWS) |
| Optimistic concurrency | No row locks, UPDATE WHERE status = expected |
| OTP constant-time compare | Prevents timing attacks on verification |
| Refresh token breach detection | Revoked reuse -> delete all tokens for user |
| Dev mode shortcuts | OTP 000000/123456 bypass, code in response body |
