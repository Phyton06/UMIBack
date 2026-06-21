package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// ========================================================
// UpdateLocation tests
// ========================================================

func TestUpdateLocation_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	mock.ExpectExec("UPDATE drivers SET location").
		WithArgs(-58.3816, -34.6037, driverID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := mustMarshal(t, map[string]any{
		"lon": -58.3816,
		"lat": -34.6037,
	})
	req := httptest.NewRequest("PATCH", "/drivers/location", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: driverID.String()},
		Role:             "driver",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	UpdateLocation(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]string
	json.NewDecoder(resp.Body).Decode(&m)
	if m["message"] != "location updated" {
		t.Errorf("message=%q, esperado 'location updated'", m["message"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateLocation_InvalidLon_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	body := mustMarshal(t, map[string]any{
		"lon": 200,
		"lat": -34.6,
	})
	req := httptest.NewRequest("PATCH", "/drivers/location", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: driverID.String()},
		Role:             "driver",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	UpdateLocation(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateLocation_InvalidLat_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	body := mustMarshal(t, map[string]any{
		"lon": -58.38,
		"lat": -100,
	})
	req := httptest.NewRequest("PATCH", "/drivers/location", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: driverID.String()},
		Role:             "driver",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	UpdateLocation(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateLocation_NotAvailable_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	// 0 rows affected (available = false) → still 200, no-op
	mock.ExpectExec("UPDATE drivers SET location").
		WithArgs(-58.3816, -34.6037, driverID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body := mustMarshal(t, map[string]any{
		"lon": -58.3816,
		"lat": -34.6037,
	})
	req := httptest.NewRequest("PATCH", "/drivers/location", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: driverID.String()},
		Role:             "driver",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	UpdateLocation(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]string
	json.NewDecoder(resp.Body).Decode(&m)
	if m["message"] != "location updated" {
		t.Errorf("message=%q, esperado 'location updated'", m["message"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateLocation_MissingFields_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()

	body := mustMarshal(t, map[string]any{
		"lon": -58.3816,
		// missing lat
	})
	req := httptest.NewRequest("PATCH", "/drivers/location", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: driverID.String()},
		Role:             "driver",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	UpdateLocation(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateLocation_NoClaims_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	body := mustMarshal(t, map[string]any{
		"lon": -58.3816,
		"lat": -34.6037,
	})
	req := httptest.NewRequest("PATCH", "/drivers/location", bytes.NewReader(body))
	w := httptest.NewRecorder()
	UpdateLocation(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// NearbyDrivers tests
// ========================================================

func TestNearbyDrivers_HappyPath_ReturnsDrivers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	riderID := uuid.New()
	driver1ID := uuid.New()
	driver2ID := uuid.New()

	mock.ExpectQuery("SELECT id, name, ST_X").
		WithArgs(-58.3816, -34.6037, 1000.0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "lon", "lat"}).
			AddRow(driver1ID, "Driver 1", -58.38, -34.6).
			AddRow(driver2ID, "Driver 2", -58.37, -34.59))

	req := httptest.NewRequest("GET", "/drivers/nearby?lon=-58.3816&lat=-34.6037&radius=1000", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: riderID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	drivers, ok := body["drivers"].([]any)
	if !ok {
		t.Fatal("drivers no es un array")
	}
	if len(drivers) != 2 {
		t.Fatalf("len(drivers)=%d, esperado 2", len(drivers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNearbyDrivers_Empty_ReturnsEmptyArray(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	riderID := uuid.New()

	mock.ExpectQuery("SELECT id, name, ST_X").
		WithArgs(-58.3816, -34.6037, 1000.0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "lon", "lat"}))

	req := httptest.NewRequest("GET", "/drivers/nearby?lon=-58.3816&lat=-34.6037&radius=1000", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: riderID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	drivers, ok := body["drivers"].([]any)
	if !ok {
		t.Fatal("drivers no es un array")
	}
	if len(drivers) != 0 {
		t.Fatalf("len(drivers)=%d, esperado 0", len(drivers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNearbyDrivers_MissingLon_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	riderID := uuid.New()

	req := httptest.NewRequest("GET", "/drivers/nearby?lat=-34.6037", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: riderID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNearbyDrivers_InvalidLatRange_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	riderID := uuid.New()

	req := httptest.NewRequest("GET", "/drivers/nearby?lon=-58.38&lat=100&radius=1000", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: riderID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNearbyDrivers_InvalidRadius_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	riderID := uuid.New()

	req := httptest.NewRequest("GET", "/drivers/nearby?lon=-58.38&lat=-34.6&radius=60000", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: riderID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNearbyDrivers_NoClaims_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	req := httptest.NewRequest("GET", "/drivers/nearby?lon=-58.3816&lat=-34.6037", nil)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNearbyDrivers_DefaultRadius_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	riderID := uuid.New()
	driverID := uuid.New()

	// Default radius 5000 when omitted
	mock.ExpectQuery("SELECT id, name, ST_X").
		WithArgs(-58.3816, -34.6037, 5000.0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "lon", "lat"}).
			AddRow(driverID, "Driver", -58.38, -34.6))

	req := httptest.NewRequest("GET", "/drivers/nearby?lon=-58.3816&lat=-34.6037", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: riderID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	NearbyDrivers(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	drivers, ok := body["drivers"].([]any)
	if !ok {
		t.Fatal("drivers no es un array")
	}
	if len(drivers) != 1 {
		t.Fatalf("len(drivers)=%d, esperado 1", len(drivers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
