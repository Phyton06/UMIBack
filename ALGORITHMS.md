# UMI — Algorithms & Data Structures in Practice

> Real problems solved in the UMI ride-hailing backend, connected to algorithmic concepts. Not a LeetCode graveyard — every problem here has a reason to exist.

## 1. Spatial Indexing — Finding Nearby Drivers

**Problem**: When a rider requests a ride, find the 5 closest available drivers within 2km.

**Naive approach**: Scan all drivers, calculate distance to each, sort, take 5. O(n) per request.

**Solution**: PostGIS GIST index on `drivers.location` (GEOMETRY Point, SRID 4326).

```sql
SELECT id, name, ST_Distance(
  location::geography,
  ST_SetSRID(ST_MakePoint(-103.348, 20.659), 4326)::geography
) AS distance_meters
FROM drivers
WHERE available = true
  AND ST_DWithin(
    location::geography,
    ST_SetSRID(ST_MakePoint(-103.348, 20.659), 4326)::geography,
    2000
  )
ORDER BY distance_meters
LIMIT 5;
```

**Complexity**: O(log n) with GIST index (R-tree). Without index: O(n).

**Why it matters**: In a city with 500 drivers, this query runs in <5ms vs ~50ms without spatial indexing.

**Files**: `internal/api/driver.go`, `internal/db/migrations/m007_indexes.sql`

---

## 2. Token Bucket Rate Limiting

**Problem**: Prevent abuse — OTP endpoints limited to 5 requests/minute per IP, general endpoints to 60/minute per user.

**Solution**: Token bucket algorithm with in-memory map + mutex.

```go
type tokenBucket struct {
    tokens     float64
    lastTime   time.Time
    maxTokens  float64
    refillRate float64 // tokens per second
}

func (tb *tokenBucket) allow() bool {
    tb.refill()
    if tb.tokens >= 1 {
        tb.tokens--
        return true
    }
    return false
}
```

**Complexity**: O(1) per check. Background goroutine cleans stale entries every 5 min.

**Why it matters**: Without rate limiting, an attacker could spam OTP endpoints and exhaust SMS budgets.

**Files**: `internal/api/ratelimit.go`

---

## 3. Optimistic Concurrency — Ride State Machine

**Problem**: Two drivers accept the same ride simultaneously. Who gets it?

**Naive approach**: `SELECT ... FOR UPDATE` (pessimistic lock) — blocks other requests, reduces throughput.

**Solution**: Optimistic locking via conditional UPDATE.

```sql
UPDATE rides SET status = 'ACCEPTED', driver_id = $1
WHERE id = $2 AND status = 'REQUESTED'
RETURNING id
```

If `RowsAffected() == 0` -> HTTP 409 Conflict. The other request already changed the status.

**Complexity**: O(1) per transition. No locks, no blocking.

**Why it matters**: In高峰期 with 100 concurrent ride requests, optimistic locking avoids deadlocks and maintains throughput.

**Files**: `internal/api/ride.go` (AcceptRide, EnRouteRide, ArrivedRide, StartRide, CompleteRide, CancelRide)

---

## 4. Secure OTP Generation — Cryptographic Randomness

**Problem**: Generate 6-digit OTP codes that cannot be predicted.

**Bad approach**: `math/rand` — deterministic, predictable seed.

**Solution**: `crypto/rand` for cryptographic randomness.

```go
func GenerateOTP() (string, error) {
    b := make([]byte, 3)
    if _, err := crypto_rand.Read(b); err != nil {
        return "", err
    }
    code := fmt.Sprintf("%06d", binary.BigEndian.Uint32(b)%1000000)
    return code, nil
}
```

**Complexity**: O(1). 3 bytes from `/dev/urandom` mapped to 6 digits.

**Why it matters**: Predictable OTPs = account takeover. `crypto/rand` uses OS-level entropy.

**Files**: `internal/auth/otp.go`

---

## 5. Constant-Time Comparison — Preventing Timing Attacks

**Problem**: Comparing OTP hashes with `==` leaks information through response time variations.

**Naive approach**: `if code == expected` — an attacker can measure response time to guess characters one at a time.

**Solution**: `crypto/subtle.ConstantTimeCompare` runs in constant time regardless of where bytes differ.

```go
func VerifyOTP(input, stored []byte) bool {
    return subtle.ConstantTimeCompare(input, stored) == 1
}
```

**Complexity**: O(n) but time is independent of input content — no timing side-channel.

**Why it matters**: Timing attacks on OTP verification could allow brute-force bypass.

**Files**: `internal/auth/otp.go`

---

## 6. Pluggable Strategy Pattern — SMS Providers

**Problem**: Different environments need different SMS providers (Log in dev, Twilio in staging, AWS SNS in prod). Hardcoding = rewrite on every switch.

**Solution**: Strategy interface with runtime selection.

```go
type Sender interface {
    Send(ctx context.Context, phone, body string) error
}

func NewSender(cfg *config.Config) auth.Sender {
    switch cfg.SMSService {
    case "twilio":
        return auth.NewTwilioSender(cfg)
    case "aws":
        return auth.NewAWSSender(cfg)
    default:
        return auth.NewLogSender()
    }
}
```

**Complexity**: O(1) dispatch. Each strategy is independently testable.

**Why it matters**: Added AWS SNS without touching any existing code — just a new file + config switch.

**Files**: `internal/auth/sms_sender.go`, `internal/auth/sms_twilio.go`, `internal/auth/sms_aws.go`, `internal/auth/sms_log.go`

---

## 7. Honeypot Pattern — Refresh Token Breach Detection

**Problem**: If a stolen refresh token is used after the user rotated it, how do you detect and respond?

**Solution**: When a revoked token is presented, delete ALL tokens for that user.

```go
if existing.RevokedAt != nil {
    tx.Exec(ctx, "DELETE FROM refresh_tokens WHERE user_id = $1", userID)
    return http.StatusUnauthorized, fmt.Errorf("breach detected")
}
```

**Complexity**: O(1) check + O(k) cleanup where k = number of active tokens per user (typically 1-3).

**Why it matters**: Token theft is undetectable without honeypot behavior. One revoked-reuse triggers full session invalidation.

**Files**: `internal/auth/jwt.go` (ValidateRefreshToken)

---

## 8. Embedded File System — Database Migrations

**Problem**: How to ship SQL migration files in a single Go binary without external file dependencies?

**Solution**: `go:embed` directive embeds SQL files at compile time.

```go
//go:embed migrations/*.sql
var Migrations embed.FS
```

Migrations run automatically on server start, in order. Each is wrapped in a transaction.

**Complexity**: O(m) where m = number of migrations. Idempotent — skips already-applied migrations.

**Why it matters**: Single binary deployment. No volume mounts, no init containers, no file copying in Docker.

**Files**: `internal/db/migrations/` (7 SQL files), `internal/db/migrate.go`

---

## 9. Graceful Shutdown — Connection Pool Drain

**Problem**: When deploying a new version, in-flight requests get dropped. Users see errors.

**Solution**: Signal handler drains connections before exit.

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
srv.Shutdown(ctx)
```

**Complexity**: O(1) signal setup. Shutdown completes in-flight requests within timeout.

**Why it matters**: Zero-downtime deployments in ECS. Old task stops accepting new requests, finishes existing ones, then exits.

**Files**: `cmd/server/main.go`

---

## 10. Haversine Distance — Fare Estimation

**Problem**: Estimate ride fare before requesting. Need straight-line distance between pickup and dropoff.

**Solution**: PostGIS handles this natively with geography cast.

```sql
SELECT ST_Distance(
  ST_SetSRID(ST_MakePoint($pickup_lng, $pickup_lat), 4326)::geography,
  ST_SetSRID(ST_MakePoint($dropoff_lng, $dropoff_lat), 4326)::geography
) AS distance_meters
```

Behind the scenes, PostGIS uses the Haversine formula (or more accurate WGS84 ellipsoid).

**Complexity**: O(1). Single calculation, no index involved.

**Why it matters**: Accurate fare estimation before ride confirmation. No third-party API needed.

**Files**: `internal/api/ride.go` (EstimateRide handler)

---

## 11. E.164 Phone Normalization — Input Sanitization

**Problem**: Users enter phone numbers in dozens of formats: `+52 444 123 4567`, `(444)123-4567`, `4441234567`. All must map to one canonical form.

**Solution**: Strip all non-digits, validate length, prepend country code if missing.

```go
func NormalizePhone(phone string) string {
    digits := regexp.MustCompile(`\D`).ReplaceAllString(phone, "")
    if len(digits) == 10 {
        digits = "52" + digits
    }
    return "+" + digits
}
```

**Complexity**: O(n) where n = input string length. Single regex pass.

**Why it matters**: Without normalization, `+524441234567` and `4441234567` are different users. OTP delivery fails.

**Files**: `internal/auth/phone.go`

---

## 12. Connection Pooling — pgxpool

**Problem**: Opening a new DB connection per request is expensive (~5ms handshake + TLS). With 60 requests/second, that is 300ms/second wasted on connection setup.

**Solution**: Pre-allocated pool with health checks.

```go
pool, err := pgxpool.NewWithConfig(ctx, cfg)
// MaxConns=10, MinConns=2, MaxConnLifetime=30m, HealthCheck=5m
```

**Complexity**: O(1) per request (pool hit). O(c) for cold start where c = MinConns.

**Why it matters**: Reduces p99 latency by ~5ms per request. Pool handles broken connections automatically.

**Files**: `internal/db/db.go`

---

## 13. SQL Injection Prevention — Parameterized Queries

**Problem**: User input directly in SQL = game over. Classic SQL injection.

**Solution**: All queries use pgx parameterized queries (`$1`, `$2`, etc.).

```go
// SAFE: parameterized
pool.QueryRow(ctx, "SELECT id FROM users WHERE phone = $1", phone)

// UNSAFE: string interpolation (NEVER done in this codebase)
pool.QueryRow(ctx, fmt.Sprintf("SELECT id FROM users WHERE phone = '%s'", phone))
```

**Complexity**: O(1) overhead for parameter binding. pgx handles escaping.

**Why it matters**: SQL injection is OWASP #3. Parameterized queries are the only reliable defense.

**Files**: Every file in `internal/api/` and `internal/auth/`

---

## 14. AbortController Pattern — OTP Request Cleanup

**Problem**: User requests OTP, then navigates away. The timer keeps counting. Next request might hit rate limit.

**Solution**: AbortController in the React Native client cancels pending requests.

```javascript
const controller = new AbortController();
const timeoutId = setTimeout(() => controller.abort(), 30000);

const response = await fetch(url, { signal: controller.signal });
clearTimeout(timeoutId);
```

**Complexity**: O(1) setup + O(1) cleanup.

**Why it matters**: Prevents stale requests from consuming server resources or causing duplicate OTP sends.

**Files**: `src/screens/LoginScreen.tsx`, `src/screens/RegisterScreen.tsx`

---

## 15. Database Transaction — Atomic Multi-Table Writes

**Problem**: Creating a user + generating OTP + inserting into multiple tables must be atomic. Partial writes = corrupted state.

**Solution**: pgx transactions with automatic rollback on error.

```go
tx, _ := pool.Begin(ctx)
defer tx.Rollback(ctx)

tx.Exec(ctx, "INSERT INTO users ...")
tx.Exec(ctx, "INSERT INTO otp_codes ...")

tx.Commit(ctx)
```

**Complexity**: O(1) overhead for transaction framing. All or nothing.

**Why it matters**: Without transactions, a crash between INSERTs leaves orphaned records.

**Files**: `internal/auth/otp.go`, `internal/api/ride.go`

---

## Summary Table

| # | Algorithm/Pattern | Data Structure | Complexity | Real Use Case |
|---|-------------------|----------------|------------|---------------|
| 1 | Spatial indexing (R-tree) | GIST index | O(log n) | Nearby drivers |
| 2 | Token bucket | Map + Mutex | O(1) | Rate limiting |
| 3 | Optimistic locking | SQL WHERE | O(1) | Ride state transitions |
| 4 | Crypto RNG | Byte buffer | O(1) | OTP generation |
| 5 | Constant-time compare | Byte slice | O(n) | OTP verification |
| 6 | Strategy pattern | Interface | O(1) | SMS providers |
| 7 | Honeypot detection | DB row check | O(1)+O(k) | Token breach |
| 8 | Embedded FS | go:embed | O(m) | Migrations |
| 9 | Signal handler | Channel | O(1) | Graceful shutdown |
| 10 | Haversine (via PostGIS) | Geography cast | O(1) | Fare estimation |
| 11 | Regex normalization | String | O(n) | Phone formatting |
| 12 | Connection pooling | pgxpool.Pool | O(1) | DB connections |
| 13 | Parameterized queries | pgx $1,$2 | O(1) | SQL injection prevention |
| 14 | AbortController | AbortSignal | O(1) | Request cleanup |
| 15 | ACID transactions | pgx Tx | O(1) | Atomic writes |
