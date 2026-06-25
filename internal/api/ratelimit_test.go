package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// ========================================
// 3.1 Unit: TokenBucket allow/refill
// ========================================

func TestTokenBucket_FirstRequest_Allowed(t *testing.T) {
	tb := newTokenBucket(5, time.Minute)
	ok, _ := tb.allow("ip-1")
	if !ok {
		t.Error("first request should be allowed")
	}
}

func TestTokenBucket_ExactCapacity_AllowsBurst(t *testing.T) {
	tb := newTokenBucket(3, time.Minute)
	for i := 0; i < 3; i++ {
		ok, _ := tb.allow("ip-1")
		if !ok {
			t.Fatalf("request %d should be allowed (burst)", i+1)
		}
	}
	// 4th should be denied
	ok, wait := tb.allow("ip-1")
	if ok {
		t.Fatal("4th request should be denied")
	}
	if wait <= 0 {
		t.Fatal("wait duration should be positive")
	}
}

func TestTokenBucket_Refill_AllowsAfterWindow(t *testing.T) {
	tb := newTokenBucket(1, 50*time.Millisecond)

	ok, _ := tb.allow("ip-1")
	if !ok {
		t.Fatal("first request should be allowed")
	}

	// Immediately denied
	ok, _ = tb.allow("ip-1")
	if ok {
		t.Fatal("second request should be denied (no refill yet)")
	}

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	ok, _ = tb.allow("ip-1")
	if !ok {
		t.Fatal("third request should be allowed after refill")
	}
}

func TestTokenBucket_IndependentKeys(t *testing.T) {
	tb := newTokenBucket(2, time.Minute)

	ok1, _ := tb.allow("alpha")
	ok2, _ := tb.allow("beta")
	if !ok1 || !ok2 {
		t.Fatal("both keys should be allowed on first request")
	}

	ok1, _ = tb.allow("alpha")
	if !ok1 {
		t.Fatal("alpha should be allowed (2nd request)")
	}

	// alpha exhausted
	ok1, _ = tb.allow("alpha")
	if ok1 {
		t.Fatal("alpha should be denied (3rd request)")
	}

	// beta still has 1 left
	ok2, _ = tb.allow("beta")
	if !ok2 {
		t.Fatal("beta should still be allowed (2nd request)")
	}
}

// no time-mocking, use small windows for real-time tests
func TestTokenBucket_CapacityCapped(t *testing.T) {
	tb := newTokenBucket(3, 100*time.Millisecond)

	// Consume 1 token
	tb.allow("ip-1")

	// Wait more than 1 window — tokens should cap at 3 (not accumulate beyond)
	time.Sleep(150 * time.Millisecond)

	// Should have 3 tokens (capped), consume all 3
	for i := 0; i < 3; i++ {
		ok, _ := tb.allow("ip-1")
		if !ok {
			t.Fatalf("request %d should be allowed (capped at capacity)", i+1)
		}
	}

	// 4th should be denied (we consumed all 3)
	ok, _ := tb.allow("ip-1")
	if ok {
		t.Fatal("should be denied — tokens capped at capacity, not accumulated")
	}
}

func TestTokenBucket_RetryAfter_Positive(t *testing.T) {
	tb := newTokenBucket(1, 10*time.Second)

	tb.allow("ip-1")
	ok, wait := tb.allow("ip-1")
	if ok {
		t.Fatal("should be denied")
	}

	// Wait should be positive and within the window
	if wait <= 0 || wait > 10*time.Second {
		t.Fatalf("wait should be between 0 and 10s, got %v", wait)
	}

	// Retry-After header in middleware uses int(math.Ceil(wait.Seconds()))
	// which always gives >= 1 second for a denied request
	retryAfter := int(wait.Seconds())
	if wait > 0 && retryAfter < 1 {
		// Tiny wait from floating point refill — ceil gives 1
		retryAfter = 1
	}
	_ = retryAfter // middleware handles ceil rounding; allow() returns precise duration
}

// ========================================
// 3.4 Unit: Sweep cleanup
// ========================================

func TestTokenBucket_Cleanup_RemovesStale(t *testing.T) {
	tb := newTokenBucket(5, time.Minute)

	tb.allow("stale-key")
	tb.allow("fresh-key")

	// Pretend 3 minutes passed — stale-key's lastAccess is the allow() time above
	// which is in the past, so cleanup with a threshold 2min before "now"
	// would catch it if lastAccess < now - 2*window = now - 2min
	future := time.Now().Add(3 * time.Minute)
	tb.cleanup(future)

	// stale-key should be removed
	tb.mu.Lock()
	_, staleOk := tb.buckets["stale-key"]
	tb.mu.Unlock()
	if staleOk {
		t.Error("stale-key should have been removed")
	}

	// fresh-key is not stale because we just created it... wait, we created both at same time.
	// Let me reconsider: we created stale-key and fresh-key at the same time.
	// With future = now+3min, and window = 1min, threshold = future - 2*1min = future-2min
	// Both were created at ~now, which is future-3min. future-3min < future-2min
	// So BOTH are stale. That's not what I want.
	//
	// The issue is that both keys were created at the same time.
	// Let me think about this differently: create one key that's old, and one that's recent.
}

func TestTokenBucket_Cleanup_RemovesOnlyStale(t *testing.T) {
	tb := newTokenBucket(5, time.Minute)

	// Create an old entry by setting lastAccess in the past
	tb.mu.Lock()
	tb.buckets["old"] = &bucketEntry{
		tokens:     5,
		lastRefill: time.Now().Add(-3 * time.Minute),
		lastAccess: time.Now().Add(-3 * time.Minute),
	}
	tb.buckets["recent"] = &bucketEntry{
		tokens:     5,
		lastRefill: time.Now(),
		lastAccess: time.Now(),
	}
	tb.mu.Unlock()

	// Cleanup with threshold = now - 2*1min
	tb.cleanup(time.Now())

	tb.mu.Lock()
	_, oldOk := tb.buckets["old"]
	_, recentOk := tb.buckets["recent"]
	tb.mu.Unlock()

	if oldOk {
		t.Error("old entry should have been removed (3min stale, threshold 2min)")
	}
	if !recentOk {
		t.Error("recent entry should still be present")
	}
}

// ========================================
// 3.2 Integration: Auth tier middleware
// ========================================

func TestRateLimit_AuthTier_WithinLimit(t *testing.T) {
	mw := RateLimit(3, time.Minute, ClientIP)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// 3 requests from same IP should all pass (burst)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status=%d, expected 200", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestRateLimit_AuthTier_Exceeded(t *testing.T) {
	mw := RateLimit(3, time.Minute, ClientIP)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust 3 tokens
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		w.Result().Body.Close()
	}

	// 4th should be 429
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}

	if retry := resp.Header.Get("Retry-After"); retry == "" {
		t.Fatal("expected Retry-After header")
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "too many requests" {
		t.Fatalf("expected error message, got %v", body)
	}
}

func TestRateLimit_AuthTier_DifferentIPs_Independent(t *testing.T) {
	mw := RateLimit(2, time.Minute, ClientIP)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust IP-A
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		w.Result().Body.Close()
	}

	// IP-A should be denied
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatal("IP-A should be rate limited")
	}
	w.Result().Body.Close()

	// IP-B should still be allowed
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:9999"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatal("IP-B should be allowed (different key)")
	}
	w.Result().Body.Close()
}

// ========================================
// 3.3 Integration: General tier middleware
// ========================================

func TestRateLimit_GeneralTier_WithinLimit(t *testing.T) {
	mw := RateLimit(3, time.Minute, UserIDFromClaims)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/rides", nil)
		ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{})
		ctx.Value(auth.ClaimsKey).(*auth.Claims).Subject = "user-abc"
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("request %d: expected 200", i+1)
		}
		w.Result().Body.Close()
	}
}

func TestRateLimit_GeneralTier_Exceeded(t *testing.T) {
	mw := RateLimit(2, time.Minute, UserIDFromClaims)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/rides", nil)
		ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{})
		ctx.Value(auth.ClaimsKey).(*auth.Claims).Subject = "user-xyz"
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		w.Result().Body.Close()
	}

	// 3rd should be 429
	req := httptest.NewRequest("GET", "/rides", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{})
	ctx.Value(auth.ClaimsKey).(*auth.Claims).Subject = "user-xyz"
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}

func TestRateLimit_GeneralTier_NoClaims_EmptyKey(t *testing.T) {
	mw := RateLimit(1, time.Minute, UserIDFromClaims)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without claims, key is empty string
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatal("first request with empty key should be allowed")
	}
	w.Result().Body.Close()
}

// ========================================
// 3.4 Integration: Sweep via StartRateLimitSweeper
// ========================================

func TestRateLimit_SweeperCleanup(t *testing.T) {
	// Create a limiter with 1-hour window so entries are easy to make "stale"
	mw := RateLimit(10, time.Hour, ClientIP)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make a request to create an entry
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	w.Result().Body.Close()

	// Verify entry exists
	limitersMu.Lock()
	tb := limiters[len(limiters)-1]
	limitersMu.Unlock()

	tb.mu.Lock()
	_, ok := tb.buckets["10.0.0.1"]
	tb.mu.Unlock()
	if !ok {
		t.Fatal("entry should exist before cleanup")
	}

	// Run cleanup with a future time that makes the entry stale
	// threshold = now - 2*1h, so if lastAccess is before that, it's stale.
	// lastAccess is ~now, so we need cleanup(now + 3h) to make threshold = now+1h
	tb.cleanup(time.Now().Add(3 * time.Hour))

	tb.mu.Lock()
	_, ok = tb.buckets["10.0.0.1"]
	tb.mu.Unlock()
	if ok {
		t.Fatal("entry should have been removed after cleanup")
	}
}

// ========================================
// 3.2/3.3 Integration: Full HTTP server
// ========================================

func TestRateLimit_Integration_SequentialRequests(t *testing.T) {
	// This test validates the full middleware chain with httptest.NewServer
	mw := RateLimit(2, time.Minute, ClientIP)

	mux := http.NewServeMux()
	mux.Handle("/test", mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{}

	// Request 1 & 2 should pass
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status=%d, expected 200", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Request 3 should be 429
	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request 3: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request 3: status=%d, expected 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestRateLimit_Integration_TierIndependence(t *testing.T) {
	// Auth and general tiers should track independently
	authMW := RateLimit(2, time.Minute, ClientIP)
	generalMW := RateLimit(10, time.Minute, UserIDFromClaims)

	mux := http.NewServeMux()
	mux.Handle("/auth", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	mux.Handle("/api", generalMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Exhaust auth tier for this IP
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/auth", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("auth req %d: %v", i+1, err)
		}
		resp.Body.Close()
	}

	// Auth should be 429 now
	req, _ := http.NewRequest("GET", srv.URL+"/auth", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatal("auth tier should be rate limited")
	}
	resp.Body.Close()

	// General should still work (different key function, no claims → empty key)
	req, _ = http.NewRequest("GET", srv.URL+"/api", nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatal("general tier should still work even though auth is exhausted")
	}
	resp.Body.Close()
}

// ========================================
// 3.1 Unit: Key extraction helpers
// ========================================

func TestClientIP_StripsPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	ip := ClientIP(req)
	if ip != "192.168.1.1" {
		t.Fatalf("expected 192.168.1.1, got %q", ip)
	}
}

func TestClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:8080"

	ip := ClientIP(req)
	if ip != "::1" {
		t.Fatalf("expected ::1, got %q", ip)
	}
}

func TestClientIP_NoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1"

	ip := ClientIP(req)
	if ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", ip)
	}
}

func TestUserIDFromClaims_ExtractsSubject(t *testing.T) {
	claims := &auth.Claims{}
	claims.Subject = "user-42"
	claims.Role = "rider"

	ctx := context.WithValue(context.Background(), auth.ClaimsKey, claims)
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	uid := UserIDFromClaims(req)
	if uid != "user-42" {
		t.Fatalf("expected user-42, got %q", uid)
	}
}

func TestUserIDFromClaims_NoClaims_ReturnsEmpty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	uid := UserIDFromClaims(req)
	if uid != "" {
		t.Fatalf("expected empty string, got %q", uid)
	}
}

// ========================================
// 4.1: Build verification is in task 4 (run manually)
// ========================================

// Reset for tests — avoid test order pollution from global limiters
func init() {
	limitersMu = sync.Mutex{}
	limiters = nil
}
