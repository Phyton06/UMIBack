package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// captureLog redirects slog output to a buffer and returns a function
// to read the captured output.
func captureLog() (buf *bytes.Buffer, restore func()) {
	buf = &bytes.Buffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return buf, func() { slog.SetDefault(old) }
}

// extractOTP parses the first OTP code from captured slog output.
func extractOTP(buf *bytes.Buffer) string {
	raw := buf.String()
	// slog TextHandler output: "time=... level=INFO msg=\"mock SMS\" phone=... code=123456\n"
	_, after, _ := strings.Cut(raw, "code=")
	if after == "" {
		return ""
	}
	code, _, _ := strings.Cut(after, "\n")
	return strings.TrimSpace(code)
}

func TestAuthFullCycle(t *testing.T) {
	logBuf, restoreLog := captureLog()
	defer restoreLog()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	jwtSecret := []byte("test-secret-1234567890")
	phone := "5511111111"
	name := "Test Rider"

	// ========================================================
	// Step 1: Register Rider — POST /auth/register/rider
	// ========================================================
	userID := uuid.New()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(phone, name).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(userID))

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(userID, "rider", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := mustMarshal(t, map[string]string{"phone": phone, "name": name})
	req := httptest.NewRequest("POST", "/auth/register/rider", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RegisterRider(mock, jwtSecret)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("[1] register rider: status=%d, esperado %d", resp.StatusCode, http.StatusCreated)
	}
	regResp := mustDecodeMap(t, resp)
	resp.Body.Close()

	if regResp["user_id"] == "" {
		t.Fatal("[1] register rider: user_id vacío")
	}
	_ = regResp["access_token"].(string)
	_ = regResp["refresh_token"].(string)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[1] unmet expectations: %v", err)
	}
	t.Logf("[1] Register OK — user=%s", regResp["user_id"])

	// ========================================================
	// Step 2: Request OTP — POST /auth/request-otp
	// ========================================================
	mock.ExpectExec("INSERT INTO otp_codes").
		WithArgs(phone, "rider", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	otpBody := mustMarshal(t, map[string]string{"phone": phone, "role": "rider"})
	req = httptest.NewRequest("POST", "/auth/request-otp", bytes.NewReader(otpBody))
	w = httptest.NewRecorder()
	RequestOTP(mock, auth.LogSender{})(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("[2] request otp: status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[2] unmet expectations: %v", err)
	}

	otpCode := extractOTP(logBuf)
	if otpCode == "" {
		t.Fatal("[2] OTP code not found in slog output")
	}
	t.Logf("[2] Request OTP OK — code=%s", otpCode)

	// ========================================================
	// Step 3: Verify OTP — POST /auth/verify-otp
	// ========================================================
	otpHash := auth.HashOTP(otpCode)
	future := time.Now().Add(5 * time.Minute)

	mock.ExpectQuery("SELECT code_hash, attempts, expires_at").
		WithArgs(phone, "rider").
		WillReturnRows(pgxmock.NewRows([]string{"code_hash", "attempts", "expires_at"}).
			AddRow(otpHash, int16(0), future))

	mock.ExpectExec("DELETE FROM otp_codes").
		WithArgs(phone, "rider").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	mock.ExpectQuery("SELECT id FROM users").
		WithArgs(phone, "+52"+phone).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(userID))

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(userID, "rider", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	verifyBody := mustMarshal(t, map[string]string{"phone": phone, "role": "rider", "code": otpCode})
	req = httptest.NewRequest("POST", "/auth/verify-otp", bytes.NewReader(verifyBody))
	w = httptest.NewRecorder()
	VerifyOTP(mock, jwtSecret)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("[3] verify otp: status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}
	verifyResp := mustDecodeMap(t, resp)
	resp.Body.Close()

	_ = verifyResp["access_token"].(string)
	refreshToken2 := verifyResp["refresh_token"].(string)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[3] unmet expectations: %v", err)
	}
	t.Logf("[3] Verify OTP OK")

	// ========================================================
	// Step 4: Refresh — POST /auth/refresh
	// ========================================================
	// Hash the refresh token to match DB lookup in handler
	tokenHash := sha256.Sum256([]byte(refreshToken2))
	tokenHashSlice := tokenHash[:]
	rtID := uuid.New()

	mock.ExpectQuery("SELECT id, user_id, role, revoked_at").
		WithArgs(tokenHashSlice).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "role", "revoked_at"}).
			AddRow(rtID, userID, "rider", nil))

	mock.ExpectExec("UPDATE refresh_tokens").
		WithArgs(rtID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(userID, "rider", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	refreshBody := mustMarshal(t, map[string]string{"refresh_token": refreshToken2})
	req = httptest.NewRequest("POST", "/auth/refresh", bytes.NewReader(refreshBody))
	w = httptest.NewRecorder()
	RefreshToken(mock, jwtSecret)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("[4] refresh: status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}
	refreshResp := mustDecodeMap(t, resp)
	resp.Body.Close()

	_ = refreshResp["access_token"].(string)
	refreshToken3 := refreshResp["refresh_token"].(string)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[4] unmet expectations: %v", err)
	}
	t.Logf("[4] Refresh OK — new pair issued")

	// ========================================================
	// Step 5: Logout — POST /auth/logout
	// ========================================================
	mock.ExpectExec("DELETE FROM refresh_tokens").
		WithArgs(userID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	// Inject claims directly into context (bypass middleware since mock doesn't
	// satisfy *pgxpool.Pool which auth.Auth requires).
	claims := &auth.Claims{}
	claims.Subject = userID.String()
	claims.Role = "rider"

	req = httptest.NewRequest("POST", "/auth/logout", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, claims)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	Logout(mock)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("[5] logout: status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[5] unmet expectations: %v", err)
	}
	t.Logf("[5] Logout OK")

	// ========================================================
	// Step 6: Breach — reuse old revoked token → 401 + all deleted
	// ========================================================
	// The old refreshToken2 was marked revoked in step 4.
	// Reusing it triggers breach: all tokens for user_id are deleted.
	revokedTime := time.Now()
	mock.ExpectQuery("SELECT id, user_id, role, revoked_at").
		WithArgs(tokenHashSlice).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "role", "revoked_at"}).
			AddRow(rtID, userID, "rider", &revokedTime))

	mock.ExpectExec("DELETE FROM refresh_tokens").
		WithArgs(userID).
		WillReturnResult(pgxmock.NewResult("DELETE", 2))

	breachBody := mustMarshal(t, map[string]string{"refresh_token": refreshToken2})
	req = httptest.NewRequest("POST", "/auth/refresh", bytes.NewReader(breachBody))
	w = httptest.NewRecorder()
	RefreshToken(mock, jwtSecret)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("[6] breach: status=%d, esperado 401", resp.StatusCode)
	}
	var breachResp map[string]string
	json.NewDecoder(resp.Body).Decode(&breachResp)
	resp.Body.Close()

	if breachResp["error"] == "" {
		t.Fatal("[6] breach: esperado mensaje de error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[6] unmet expectations: %v", err)
	}
	t.Logf("[6] Breach OK — token revoked, all tokens deleted")

	// Verify the new token (from refresh) also no longer works
	rt3Hash := sha256.Sum256([]byte(refreshToken3))
	mock.ExpectQuery("SELECT id, user_id, role, revoked_at").
		WithArgs(rt3Hash[:]).
		WillReturnError(pgx.ErrNoRows) // all tokens were deleted

	nilBody := mustMarshal(t, map[string]string{"refresh_token": refreshToken3})
	req = httptest.NewRequest("POST", "/auth/refresh", bytes.NewReader(nilBody))
	w = httptest.NewRecorder()
	RefreshToken(mock, jwtSecret)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("[6b] new token after breach: status=%d, esperado 401", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("[6b] unmet expectations: %v", err)
	}
	t.Logf("[6b] New token invalid after breach OK")

}

// --- Tests for validation errors and edge cases ---

func TestRegisterRider_DuplicatePhone_Returns409(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("5511111111", "Dup").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	body := mustMarshal(t, map[string]string{"phone": "5511111111", "name": "Dup"})
	req := httptest.NewRequest("POST", "/auth/register/rider", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RegisterRider(mock, []byte("secret"))(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, esperado 409", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_InvalidRole_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	body := mustMarshal(t, map[string]string{"phone": "5511111111", "role": "superadmin"})
	req := httptest.NewRequest("POST", "/auth/request-otp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RequestOTP(mock, auth.LogSender{})(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestVerifyOTP_ExpiredCode_Returns410(t *testing.T) {
	_, restoreLog := captureLog()
	defer restoreLog()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	expired := time.Now().Add(-1 * time.Minute)
	mock.ExpectQuery("SELECT code_hash, attempts, expires_at").
		WithArgs("5511111111", "rider").
		WillReturnRows(pgxmock.NewRows([]string{"code_hash", "attempts", "expires_at"}).
			AddRow([]byte("hash"), int16(0), expired))

	body := mustMarshal(t, map[string]string{"phone": "5511111111", "role": "rider", "code": "123456"})
	req := httptest.NewRequest("POST", "/auth/verify-otp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	VerifyOTP(mock, []byte("secret"))(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("status=%d, esperado 410", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_MaxAttempts_Returns429(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	future := time.Now().Add(5 * time.Minute)
	mock.ExpectQuery("SELECT code_hash, attempts, expires_at").
		WithArgs("5511111111", "rider").
		WillReturnRows(pgxmock.NewRows([]string{"code_hash", "attempts", "expires_at"}).
			AddRow([]byte("hash"), int16(3), future))

	body := mustMarshal(t, map[string]string{"phone": "5511111111", "role": "rider", "code": "123456"})
	req := httptest.NewRequest("POST", "/auth/verify-otp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	VerifyOTP(mock, []byte("secret"))(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status=%d, esperado 429", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- Admin OTP flow tests ---

func TestRegisterAdmin_CreatesAdminAndReturnsJWT(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	adminID := uuid.New()

	mock.ExpectQuery("INSERT INTO admins").
		WithArgs("5500000001", "Admin Test").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(adminID))

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(adminID, "admin", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := mustMarshal(t, map[string]string{"phone": "5500000001", "name": "Admin Test"})
	req := httptest.NewRequest("POST", "/auth/register/admin", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RegisterAdmin(mock, []byte("test-secret"))(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d, esperado 201", resp.StatusCode)
	}
	m := mustDecodeMap(t, resp)
	resp.Body.Close()

	if m["admin_id"] == "" {
		t.Fatal("admin_id vacío")
	}
	if m["access_token"] == "" {
		t.Fatal("access_token vacío")
	}
	if m["refresh_token"] == "" {
		t.Fatal("refresh_token vacío")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_AdminRole_Returns200(t *testing.T) {
	logBuf, restoreLog := captureLog()
	defer restoreLog()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO otp_codes").
		WithArgs("5500000001", "admin", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := mustMarshal(t, map[string]string{"phone": "5500000001", "role": "admin"})
	req := httptest.NewRequest("POST", "/auth/request-otp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RequestOTP(mock, auth.LogSender{})(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}

	otpCode := extractOTP(logBuf)
	if otpCode == "" {
		t.Fatal("OTP code not found in slog output")
	}
	t.Logf("OTP for admin: %s", otpCode)
}

func TestVerifyOTP_AdminRole_QueriesAdminsTable(t *testing.T) {
	_, restoreLog := captureLog()
	defer restoreLog()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	adminID := uuid.New()
	phone := "5500000001"
	otpCode := "123456"
	otpHash := auth.HashOTP(otpCode)
	future := time.Now().Add(5 * time.Minute)

	mock.ExpectQuery("SELECT code_hash, attempts, expires_at").
		WithArgs(phone, "admin").
		WillReturnRows(pgxmock.NewRows([]string{"code_hash", "attempts", "expires_at"}).
			AddRow(otpHash, int16(0), future))

	mock.ExpectExec("DELETE FROM otp_codes").
		WithArgs(phone, "admin").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	mock.ExpectQuery("SELECT id FROM admins").
		WithArgs(phone, "+52"+phone).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(adminID))

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(adminID, "admin", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := mustMarshal(t, map[string]string{"phone": phone, "role": "admin", "code": otpCode})
	req := httptest.NewRequest("POST", "/auth/verify-otp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	VerifyOTP(mock, []byte("test-secret"))(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	m := mustDecodeMap(t, resp)
	resp.Body.Close()

	if m["access_token"] == "" {
		t.Fatal("access_token vacío")
	}
	if m["refresh_token"] == "" {
		t.Fatal("refresh_token vacío")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRegisterAdmin_DuplicatePhone_Returns409(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO admins").
		WithArgs("5500000001", "Dup Admin").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	body := mustMarshal(t, map[string]string{"phone": "5500000001", "name": "Dup Admin"})
	req := httptest.NewRequest("POST", "/auth/register/admin", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RegisterAdmin(mock, []byte("secret"))(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, esperado 409", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_InvalidRoleAdminNowValid_Returns200(t *testing.T) {
	_, restoreLog := captureLog()
	defer restoreLog()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO otp_codes").
		WithArgs("5500000001", "admin", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := mustMarshal(t, map[string]string{"phone": "5500000001", "role": "admin"})
	req := httptest.NewRequest("POST", "/auth/request-otp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RequestOTP(mock, auth.LogSender{})(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200 (admin role is now valid)", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- helpers ---

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func mustDecodeMap(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("json.Decode: %v", err)
	}
	return m
}
