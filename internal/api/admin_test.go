package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// strPtr retorna un puntero a s para valores *string en filas mock.
func strPtr(s string) *string { return &s }

// --- AdminCreateDriver ---

func TestAdminCreateDriver_Success_Returns201(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()
	mock.ExpectQuery("INSERT INTO drivers").
		WithArgs("5511111111", "Test Driver").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(driverID))

	body := mustMarshal(t, map[string]string{"phone": "5511111111", "name": "Test Driver"})
	req := httptest.NewRequest("POST", "/admin/drivers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	AdminCreateDriver(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d, esperado 201", resp.StatusCode)
	}
	m := mustDecodeMap(t, resp)
	resp.Body.Close()

	if m["driver_id"] != driverID.String() {
		t.Fatalf("driver_id=%s, esperado %s", m["driver_id"], driverID.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAdminCreateDriver_DuplicatePhone_Returns409(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO drivers").
		WithArgs("5511111111", "Dup").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	body := mustMarshal(t, map[string]string{"phone": "5511111111", "name": "Dup"})
	req := httptest.NewRequest("POST", "/admin/drivers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	AdminCreateDriver(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, esperado 409", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAdminCreateDriver_MissingFields_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	tests := []struct {
		name string
		body map[string]string
	}{
		{"empty phone", map[string]string{"phone": "", "name": "X"}},
		{"empty name", map[string]string{"phone": "5511111111", "name": ""}},
		{"missing phone", map[string]string{"name": "X"}},
		{"missing name", map[string]string{"phone": "5511111111"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := mustMarshal(t, tc.body)
			req := httptest.NewRequest("POST", "/admin/drivers", bytes.NewReader(body))
			w := httptest.NewRecorder()
			AdminCreateDriver(mock)(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status=%d, esperado 400", resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}

// --- ListDrivers ---

func TestListDrivers_Returns200WithDriversAndTotal(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	now := time.Now()
	driverID := uuid.New()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM drivers").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(1)))

	mock.ExpectQuery("SELECT id, phone, name, available, membresia_active_until, suspended_until, created_at FROM drivers").
		WithArgs(20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "available", "membresia_active_until", "suspended_until", "created_at"}).
			AddRow(driverID, "5511111111", "Driver1", true, nil, nil, now))

	req := httptest.NewRequest("GET", "/admin/drivers?limit=20&offset=0", nil)
	w := httptest.NewRecorder()
	ListDrivers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 1 {
		t.Fatalf("total=%v, esperado 1", result["total"])
	}

	drivers, ok := result["drivers"].([]any)
	if !ok || len(drivers) != 1 {
		t.Fatalf("drivers length=%d, esperado 1", len(drivers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListDrivers_EmptyList_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM drivers").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(0)))

	mock.ExpectQuery("SELECT id, phone, name, available, membresia_active_until, suspended_until, created_at FROM drivers").
		WithArgs(20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "available", "membresia_active_until", "suspended_until", "created_at"}))

	req := httptest.NewRequest("GET", "/admin/drivers", nil)
	w := httptest.NewRecorder()
	ListDrivers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 0 {
		t.Fatalf("total=%v, esperado 0", result["total"])
	}

	drivers, ok := result["drivers"].([]any)
	if !ok || len(drivers) != 0 {
		t.Fatalf("drivers length=%d, esperado 0", len(drivers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListDrivers_StatusFilterSuspended_ReturnsFiltered(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()
	now := time.Now()
	future := now.Add(24 * time.Hour)

	// With suspended status filter, WHERE clause includes suspended_until > now()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM drivers WHERE suspended_until > now\\(\\)").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(1)))

	mock.ExpectQuery("SELECT id, phone, name, available, membresia_active_until, suspended_until, created_at FROM drivers WHERE suspended_until > now\\(\\) ORDER BY created_at DESC LIMIT \\$1 OFFSET \\$2").
		WithArgs(20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "available", "membresia_active_until", "suspended_until", "created_at"}).
			AddRow(driverID, "5511111111", "Suspended", false, nil, &future, now))

	req := httptest.NewRequest("GET", "/admin/drivers?status=suspended", nil)
	w := httptest.NewRecorder()
	ListDrivers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 1 {
		t.Fatalf("total=%v, esperado 1", result["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListDrivers_InvalidStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	req := httptest.NewRequest("GET", "/admin/drivers?status=invalid", nil)
	w := httptest.NewRecorder()
	ListDrivers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado 400", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- ListPassengers ---

func TestListPassengers_Returns200WithPassengersAndTotal(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	now := time.Now()
	userID := uuid.New()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users u").
		WithArgs("").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(1)))

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.suspension_reason, u.created_at,\\s+COUNT\\(r.id\\)::int AS trips").
		WithArgs("", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "suspension_reason", "created_at", "trips"}).
			AddRow(userID, "5511111111", "Passenger1", strPtr("a@b.com"), 4.5, nil, nil, now, 3))

	req := httptest.NewRequest("GET", "/admin/passengers?limit=20&offset=0", nil)
	w := httptest.NewRecorder()
	ListPassengers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 1 {
		t.Fatalf("total=%v, esperado 1", result["total"])
	}

	passengers, ok := result["passengers"].([]any)
	if !ok || len(passengers) != 1 {
		t.Fatalf("passengers length=%d, esperado 1", len(passengers))
	}

	// Verify new fields are present on the first passenger
	p := passengers[0].(map[string]any)
	if p["email"] != "a@b.com" {
		t.Fatalf("email=%v, esperado a@b.com", p["email"])
	}
	if p["rating"] != 4.5 {
		t.Fatalf("rating=%v, esperado 4.5", p["rating"])
	}
	if trips, _ := p["trips"].(float64); trips != 3 {
		t.Fatalf("trips=%v, esperado 3", p["trips"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListPassengers_EmptyList_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users u").
		WithArgs("").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(0)))

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.suspension_reason, u.created_at,\\s+COUNT\\(r.id\\)::int AS trips").
		WithArgs("", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "suspension_reason", "created_at", "trips"}))

	req := httptest.NewRequest("GET", "/admin/passengers", nil)
	w := httptest.NewRecorder()
	ListPassengers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 0 {
		t.Fatalf("total=%v, esperado 0", result["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListPassengers_StatusFilterSuspended_ReturnsFiltered(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	now := time.Now()
	future := now.Add(24 * time.Hour)
	userID := uuid.New()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users u").
		WithArgs("").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(1)))

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.suspension_reason, u.created_at,\\s+COUNT\\(r.id\\)::int AS trips").
		WithArgs("", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "suspension_reason", "created_at", "trips"}).
			AddRow(userID, "5511111111", "Suspended", strPtr("s@b.com"), 2.0, &future, nil, now, 0))

	req := httptest.NewRequest("GET", "/admin/passengers?status=suspended", nil)
	w := httptest.NewRecorder()
	ListPassengers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 1 {
		t.Fatalf("total=%v, esperado 1", result["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListPassengers_SearchFilter_ReturnsFiltered(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	now := time.Now()
	userID := uuid.New()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users u").
		WithArgs("maria").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(1)))

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.suspension_reason, u.created_at,\\s+COUNT\\(r.id\\)::int AS trips").
		WithArgs("maria", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "suspension_reason", "created_at", "trips"}).
			AddRow(userID, "5511111111", "Maria", strPtr("maria@b.com"), 5.0, nil, nil, now, 5))

	req := httptest.NewRequest("GET", "/admin/passengers?search=maria", nil)
	w := httptest.NewRecorder()
	ListPassengers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	total, ok := result["total"].(float64)
	if !ok || int(total) != 1 {
		t.Fatalf("total=%v, esperado 1", result["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListPassengers_InvalidStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	req := httptest.NewRequest("GET", "/admin/passengers?status=invalid", nil)
	w := httptest.NewRecorder()
	ListPassengers(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado 400", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- PassengerStats ---

func TestPassengerStats_Happy_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnRows(pgxmock.NewRows([]string{"total", "banned", "avg_rating"}).
			AddRow(int(4), int(1), float64(4.2)))

	req := httptest.NewRequest("GET", "/admin/passengers/stats", nil)
	w := httptest.NewRecorder()
	PassengerStats(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	if total, _ := result["total"].(float64); int(total) != 4 {
		t.Fatalf("total=%v, esperado 4", total)
	}
	if banned, _ := result["banned"].(float64); int(banned) != 1 {
		t.Fatalf("banned=%v, esperado 1", banned)
	}
	if avg, _ := result["avg_rating"].(float64); avg != 4.2 {
		t.Fatalf("avg_rating=%v, esperado 4.2", avg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPassengerStats_EmptyDB_ReturnsZeros(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnRows(pgxmock.NewRows([]string{"total", "banned", "avg_rating"}).
			AddRow(int(0), int(0), float64(0)))

	req := httptest.NewRequest("GET", "/admin/passengers/stats", nil)
	w := httptest.NewRecorder()
	PassengerStats(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	if total, _ := result["total"].(float64); int(total) != 0 {
		t.Fatalf("total=%v, esperado 0", total)
	}
	if banned, _ := result["banned"].(float64); int(banned) != 0 {
		t.Fatalf("banned=%v, esperado 0", banned)
	}
	if avg, _ := result["avg_rating"].(float64); avg != 0 {
		t.Fatalf("avg_rating=%v, esperado 0", avg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- BanPassenger ---

func TestBanPassenger_AllUnits_Returns200(t *testing.T) {
	units := []string{"day", "week", "month", "year"}
	for _, unit := range units {
		t.Run(unit, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("pgxmock.NewPool() error: %v", err)
			}
			defer mock.Close()

			userID := uuid.New()
			interval := "1 " + unit
			expectedTime := time.Now().Add(24 * time.Hour)

			mock.ExpectQuery("UPDATE users SET suspended_until").
				WithArgs(interval, "", userID).
				WillReturnRows(pgxmock.NewRows([]string{"suspended_until"}).AddRow(expectedTime))

			body := mustMarshal(t, map[string]any{"duration": 1, "unit": unit})
			req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/ban", bytes.NewReader(body))
			req.SetPathValue("id", userID.String())
			w := httptest.NewRecorder()
			BanPassenger(mock)(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("[%s] status=%d, esperado 200", unit, resp.StatusCode)
			}
			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("json decode: %v", err)
			}
			resp.Body.Close()

			if result["suspended_until"] == nil {
				t.Fatalf("[%s] suspended_until is nil", unit)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestBanPassenger_InvalidUnit_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	body := mustMarshal(t, map[string]any{"duration": 1, "unit": "decade"})
	req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/ban", bytes.NewReader(body))
	req.SetPathValue("id", userID.String())
	w := httptest.NewRecorder()
	BanPassenger(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestBanPassenger_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectQuery("UPDATE users SET suspended_until").
		WithArgs("1 day", "", userID).
		WillReturnError(pgx.ErrNoRows)

	body := mustMarshal(t, map[string]any{"duration": 1, "unit": "day"})
	req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/ban", bytes.NewReader(body))
	req.SetPathValue("id", userID.String())
	w := httptest.NewRecorder()
	BanPassenger(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- SuspendDriver ---

func TestSuspendDriver_Success_Returns200WithSuspendedUntil(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()
	expectedTime := time.Now().Add(9999 * 24 * time.Hour)

	mock.ExpectQuery("UPDATE drivers SET suspended_until").
		WithArgs(driverID).
		WillReturnRows(pgxmock.NewRows([]string{"suspended_until"}).AddRow(expectedTime))

	req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/suspend", nil)
	req.SetPathValue("id", driverID.String())
	w := httptest.NewRecorder()
	SuspendDriver(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	if result["suspended_until"] == nil {
		t.Fatal("suspended_until is nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSuspendDriver_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	mock.ExpectQuery("UPDATE drivers SET suspended_until").
		WithArgs(driverID).
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/suspend", nil)
	req.SetPathValue("id", driverID.String())
	w := httptest.NewRecorder()
	SuspendDriver(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- UnsuspendDriver ---

func TestUnsuspendDriver_Success_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	mock.ExpectExec("UPDATE drivers SET suspended_until").
		WithArgs(driverID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/unsuspend", nil)
	req.SetPathValue("id", driverID.String())
	w := httptest.NewRecorder()
	UnsuspendDriver(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUnsuspendDriver_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	mock.ExpectExec("UPDATE drivers SET suspended_until").
		WithArgs(driverID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/unsuspend", nil)
	req.SetPathValue("id", driverID.String())
	w := httptest.NewRecorder()
	UnsuspendDriver(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- SuspendPassenger ---

func TestSuspendPassenger_Success_Returns200WithSuspendedUntil(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	expectedTime := time.Now().Add(9999 * 24 * time.Hour)

	mock.ExpectQuery("UPDATE users SET suspended_until").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"suspended_until"}).AddRow(expectedTime))

	req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/suspend", nil)
	req.SetPathValue("id", userID.String())
	w := httptest.NewRecorder()
	SuspendPassenger(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	if result["suspended_until"] == nil {
		t.Fatal("suspended_until is nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSuspendPassenger_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectQuery("UPDATE users SET suspended_until").
		WithArgs(userID).
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/suspend", nil)
	req.SetPathValue("id", userID.String())
	w := httptest.NewRecorder()
	SuspendPassenger(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- UnsuspendPassenger ---

func TestUnsuspendPassenger_Success_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectExec("UPDATE users SET suspended_until").
		WithArgs(userID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/unsuspend", nil)
	req.SetPathValue("id", userID.String())
	w := httptest.NewRecorder()
	UnsuspendPassenger(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUnsuspendPassenger_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectExec("UPDATE users SET suspended_until").
		WithArgs(userID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	req := httptest.NewRequest("PATCH", "/admin/passengers/"+userID.String()+"/unsuspend", nil)
	req.SetPathValue("id", userID.String())
	w := httptest.NewRecorder()
	UnsuspendPassenger(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- SetMembership ---

func TestSetMembership_AllUnits_Returns200(t *testing.T) {
	units := []string{"day", "week", "month", "year"}
	for _, unit := range units {
		t.Run(unit, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("pgxmock.NewPool() error: %v", err)
			}
			defer mock.Close()

			driverID := uuid.New()
			interval := "1 " + unit
			expectedTime := time.Now().Add(24 * time.Hour)

			mock.ExpectQuery("UPDATE drivers SET membresia_active_until").
				WithArgs(interval, driverID).
				WillReturnRows(pgxmock.NewRows([]string{"membresia_active_until"}).AddRow(expectedTime))

			body := mustMarshal(t, map[string]any{"duration": 1, "unit": unit})
			req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/membership", bytes.NewReader(body))
			req.SetPathValue("id", driverID.String())
			w := httptest.NewRecorder()
			SetMembership(mock)(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("[%s] status=%d, esperado 200", unit, resp.StatusCode)
			}
			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("json decode: %v", err)
			}
			resp.Body.Close()

			if result["membresia_active_until"] == nil {
				t.Fatalf("[%s] membresia_active_until is nil", unit)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestSetMembership_InvalidUnit_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()
	body := mustMarshal(t, map[string]any{"duration": 1, "unit": "decade"})
	req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/membership", bytes.NewReader(body))
	req.SetPathValue("id", driverID.String())
	w := httptest.NewRecorder()
	SetMembership(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSetMembership_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	mock.ExpectQuery("UPDATE drivers SET membresia_active_until").
		WithArgs("1 day", driverID).
		WillReturnError(pgx.ErrNoRows)

	body := mustMarshal(t, map[string]any{"duration": 1, "unit": "day"})
	req := httptest.NewRequest("PATCH", "/admin/drivers/"+driverID.String()+"/membership", bytes.NewReader(body))
	req.SetPathValue("id", driverID.String())
	w := httptest.NewRecorder()
	SetMembership(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- DashboardStats ---

func TestDashboardStats_Success_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnRows(pgxmock.NewRows([]string{"total_drivers", "active_drivers", "pending_rides", "rides_today", "revenue_today"}).
			AddRow(int(45), int(12), int(3), int(28), float64(1250.50)))

	req := httptest.NewRequest("GET", "/admin/dashboard/stats", nil)
	w := httptest.NewRecorder()
	DashboardStats(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	want := map[string]any{"total_drivers": 45.0, "active_drivers": 12.0, "pending_rides": 3.0, "rides_today": 28.0, "revenue_today": 1250.5}
	for key, val := range want {
		if got, ok := result[key].(float64); !ok || got != val {
			t.Fatalf("%s=%v, esperado %v", key, result[key], val)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDashboardStats_EmptyDB_ReturnsZeros(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnRows(pgxmock.NewRows([]string{"total_drivers", "active_drivers", "pending_rides", "rides_today", "revenue_today"}).
			AddRow(int(0), int(0), int(0), int(0), float64(0)))

	req := httptest.NewRequest("GET", "/admin/dashboard/stats", nil)
	w := httptest.NewRecorder()
	DashboardStats(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	for _, key := range []string{"total_drivers", "active_drivers", "pending_rides", "rides_today", "revenue_today"} {
		if got, ok := result[key].(float64); !ok || got != 0 {
			t.Fatalf("%s=%v, esperado 0", key, result[key])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDashboardStats_DBError_Returns500(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("db down"))

	req := httptest.NewRequest("GET", "/admin/dashboard/stats", nil)
	w := httptest.NewRecorder()
	DashboardStats(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d, esperado 500", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- ActiveRides ---

func TestActiveRides_Success_Returns200WithParsedLocations(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	rideID := uuid.New()
	driverID := uuid.New()
	now := time.Now()

	cols := []string{"ride_id", "status", "passenger_name", "driver_id", "driver_name",
		"driver_loc", "pickup_loc", "dropoff_loc", "created_at", "updated_at"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(rideID, "IN_PROGRESS", "Juan", driverID, "Carlos",
				"POINT(-58.3816 -34.6037)", "POINT(-58.38 -34.6)", "POINT(-58.39 -34.61)", now, now).
			AddRow(uuid.New(), "ACCEPTED", "Maria", uuid.New(), "Luis",
				"POINT(-58.4 -34.6)", nil, nil, now, now))

	req := httptest.NewRequest("GET", "/admin/rides/active", nil)
	w := httptest.NewRecorder()
	ActiveRides(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var rides []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rides); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	if len(rides) != 2 {
		t.Fatalf("rides length=%d, esperado 2", len(rides))
	}

	first := rides[0]
	if first["ride_id"] != rideID.String() {
		t.Fatalf("ride_id=%v, esperado %s", first["ride_id"], rideID.String())
	}
	if first["status"] != "IN_PROGRESS" || first["passenger_name"] != "Juan" || first["driver_name"] != "Carlos" {
		t.Fatalf("row fields incorrectos: %v", first)
	}
	if first["driver_id"] != driverID.String() {
		t.Fatalf("driver_id=%v, esperado %s", first["driver_id"], driverID.String())
	}
	if first["driver_lat"] != -34.6037 || first["driver_lng"] != -58.3816 {
		t.Fatalf("driver location=%v,%v, esperado -34.6037,-58.3816", first["driver_lat"], first["driver_lng"])
	}
	if first["pickup_lat"] != -34.6 || first["pickup_lng"] != -58.38 {
		t.Fatalf("pickup location=%v,%v, esperado -34.6,-58.38", first["pickup_lat"], first["pickup_lng"])
	}
	if first["dropoff_lat"] != -34.61 || first["dropoff_lng"] != -58.39 {
		t.Fatalf("dropoff location=%v,%v, esperado -34.61,-58.39", first["dropoff_lat"], first["dropoff_lng"])
	}
	if started, ok := first["started_at"].(string); !ok || started == "" {
		t.Fatalf("started_at=%v, esperado timestamp no vacío", first["started_at"])
	}

	second := rides[1]
	if second["pickup_lat"] != nil || second["pickup_lng"] != nil || second["dropoff_lat"] != nil || second["dropoff_lng"] != nil {
		t.Fatalf("pickup/dropoff deberían ser null, got: %v", second)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestActiveRides_EmptyList_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnRows(pgxmock.NewRows([]string{"ride_id", "status", "passenger_name", "driver_id", "driver_name",
			"driver_loc", "pickup_loc", "dropoff_loc", "created_at", "updated_at"}))

	req := httptest.NewRequest("GET", "/admin/rides/active", nil)
	w := httptest.NewRecorder()
	ActiveRides(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", resp.StatusCode)
	}
	var rides []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rides); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	resp.Body.Close()

	if len(rides) != 0 {
		t.Fatalf("rides length=%d, esperado 0", len(rides))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestActiveRides_DBError_Returns500(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("db down"))

	req := httptest.NewRequest("GET", "/admin/rides/active", nil)
	w := httptest.NewRecorder()
	ActiveRides(mock)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d, esperado 500", resp.StatusCode)
	}
	resp.Body.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- Admin chain auth guard (401/403) ---

func TestAdminEndpoints_UnauthorizedOrForbidden(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	secret := []byte("test-secret")
	riderToken, err := auth.SignAccessToken(uuid.New(), "rider", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	chains := map[string]http.Handler{
		"/admin/dashboard/stats": auth.Auth(secret)(auth.RequireRole("admin")(http.HandlerFunc(DashboardStats(mock)))),
		"/admin/rides/active":    auth.Auth(secret)(auth.RequireRole("admin")(http.HandlerFunc(ActiveRides(mock)))),
	}

	tests := []struct {
		name  string
		path  string
		token string
		want  int
	}{
		{name: "stats sin token", path: "/admin/dashboard/stats", token: "", want: http.StatusUnauthorized},
		{name: "stats rol no admin", path: "/admin/dashboard/stats", token: riderToken, want: http.StatusForbidden},
		{name: "rides sin token", path: "/admin/rides/active", token: "", want: http.StatusUnauthorized},
		{name: "rides rol no admin", path: "/admin/rides/active", token: riderToken, want: http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			chains[tc.path].ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d, esperado %d", w.Code, tc.want)
			}
		})
	}
}
