package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"
)

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

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.created_at,\n\t\t\t\tCOUNT\\(r.id\\)::int AS trips").
		WithArgs("", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "created_at", "trips"}).
			AddRow(userID, "5511111111", "Passenger1", "a@b.com", 4.5, nil, now, 3))

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

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.created_at,\n\t\t\t\tCOUNT\\(r.id\\)::int AS trips").
		WithArgs("", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "created_at", "trips"}))

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

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.created_at,\n\t\t\t\tCOUNT\\(r.id\\)::int AS trips").
		WithArgs("", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "created_at", "trips"}).
			AddRow(userID, "5511111111", "Suspended", "s@b.com", 2.0, &future, now, 0))

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

	mock.ExpectQuery("SELECT u.id, u.phone, u.name, u.email, u.rating, u.suspended_until, u.created_at,\n\t\t\t\tCOUNT\\(r.id\\)::int AS trips").
		WithArgs("maria", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "phone", "name", "email", "rating", "suspended_until", "created_at", "trips"}).
			AddRow(userID, "5511111111", "Maria", "maria@b.com", 5.0, nil, now, 5))

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
				WithArgs(interval, userID).
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
		WithArgs("1 day", userID).
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
