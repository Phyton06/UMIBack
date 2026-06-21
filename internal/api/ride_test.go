package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// ========================================================
// CreateRide tests
// ========================================================

func TestCreateRide_HappyPath_Returns201(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("INSERT INTO rides").
		WithArgs(userID, pgxmock.AnyArg(), pgxmock.AnyArg(), "Av Reforma 123", "Insurgentes 456").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(rideID, now))

	body := mustMarshal(t, map[string]any{
		"pickup_address":  "Av Reforma 123",
		"dropoff_address": "Insurgentes 456",
		"pickup_lon":      -99.1333,
		"pickup_lat":      19.4326,
		"dropoff_lon":     -99.1678,
		"dropoff_lat":     19.4123,
	})
	req := httptest.NewRequest("POST", "/rides", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CreateRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusCreated)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if m["id"] != rideID.String() {
		t.Errorf("id=%q, esperado %q", m["id"], rideID.String())
	}
	if m["status"] != "REQUESTED" {
		t.Errorf("status=%q, esperado REQUESTED", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateRide_MissingFields_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	// Missing pickup_lon
	body := mustMarshal(t, map[string]any{
		"pickup_address":  "Av Reforma 123",
		"dropoff_address": "Insurgentes 456",
		"pickup_lat":      19.4326,
		"dropoff_lon":     -99.1678,
		"dropoff_lat":     19.4123,
	})
	req := httptest.NewRequest("POST", "/rides", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CreateRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// ponytail: role gating tested by middleware tests; handler only validates auth + fields

func TestCreateRide_NoClaims_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	body := mustMarshal(t, map[string]any{
		"pickup_address":  "Av Reforma 123",
		"dropoff_address": "Insurgentes 456",
		"pickup_lon":      -99.1333,
		"pickup_lat":      19.4326,
		"dropoff_lon":     -99.1678,
		"dropoff_lat":     19.4123,
	})
	req := httptest.NewRequest("POST", "/rides", bytes.NewReader(body))
	w := httptest.NewRecorder()
	CreateRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// ========================================================
// GetRide tests
// ========================================================

func TestGetRide_Owner_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}).AddRow(
			rideID, userID, nil, "REQUESTED",
			"POINT(-99.1333 19.4326)", "POINT(-99.1678 19.4123)",
			"Av Reforma 123", "Insurgentes 456",
			nil, nil, nil, nil,
			now, now,
		))

	req := httptest.NewRequest("GET", "/rides/"+rideID.String(), nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		Role:    "rider",
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	GetRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if m["id"] != rideID.String() {
		t.Errorf("id=%q, esperado %q", m["id"], rideID.String())
	}
	if m["status"] != "REQUESTED" {
		t.Errorf("status=%q, esperado REQUESTED", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetRide_NonOwner_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	otherID := uuid.New()
	rideID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}).AddRow(
			rideID, otherID, nil, "REQUESTED",
			"POINT(-99.1333 19.4326)", "POINT(-99.1678 19.4123)",
			"Av Reforma 123", "Insurgentes 456",
			nil, nil, nil, nil,
			now, now,
		))

	req := httptest.NewRequest("GET", "/rides/"+rideID.String(), nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		Role:    "rider",
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	GetRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetRide_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(rideID).
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest("GET", "/rides/"+rideID.String(), nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		Role:    "rider",
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	GetRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}
}

// ========================================================
// ListRides tests
// ========================================================

func TestListRides_Rider_ReturnsOwnRides(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}).AddRow(
			rideID, userID, nil, "REQUESTED",
			"POINT(-99.1333 19.4326)", "POINT(-99.1678 19.4123)",
			"Av Reforma 123", "Insurgentes 456",
			nil, nil, nil, nil,
			now, now,
		))

	req := httptest.NewRequest("GET", "/rides", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		Role:    "rider",
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	ListRides(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	rides, ok := body["rides"].([]any)
	if !ok {
		t.Fatal("rides no es un array")
	}
	if len(rides) != 1 {
		t.Fatalf("len(rides)=%d, esperado 1", len(rides))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListRides_Driver_ReturnsOpenRequests(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs("REQUESTED").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}).AddRow(
			rideID, uuid.New(), nil, "REQUESTED",
			"POINT(-99.1333 19.4326)", "POINT(-99.1678 19.4123)",
			"Colonia Roma 1", "Condesa 2",
			nil, nil, nil, nil,
			now, now,
		))

	req := httptest.NewRequest("GET", "/rides", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		Role:    "driver",
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	ListRides(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	rides, ok := body["rides"].([]any)
	if !ok {
		t.Fatal("rides no es un array")
	}
	if len(rides) != 1 {
		t.Fatalf("len(rides)=%d, esperado 1", len(rides))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListRides_Empty_ReturnsEmptyArray(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}))

	req := httptest.NewRequest("GET", "/rides", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		Role:    "rider",
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	ListRides(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	rides, ok := body["rides"].([]any)
	if !ok {
		t.Fatal("rides no es un array")
	}
	if len(rides) != 0 {
		t.Fatalf("len(rides)=%d, esperado 0", len(rides))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// CancelRide tests
// ========================================================

func TestCancelRide_RiderCancelsOwnRide_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(userID, nil, "REQUESTED"))

	mock.ExpectExec("UPDATE rides").
		WithArgs(userID.String(), rideID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/cancel", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CancelRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if m["id"] != rideID.String() {
		t.Errorf("id=%q, esperado %q", m["id"], rideID.String())
	}
	if m["status"] != "CANCELLED" {
		t.Errorf("status=%q, esperado CANCELLED", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelRide_DriverCancelsOwnRide_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	otherPassengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(otherPassengerID, &userID, "ACCEPTED"))

	mock.ExpectExec("UPDATE rides").
		WithArgs(userID.String(), rideID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/cancel", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CancelRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if m["id"] != rideID.String() {
		t.Errorf("id=%q, esperado %q", m["id"], rideID.String())
	}
	if m["status"] != "CANCELLED" {
		t.Errorf("status=%q, esperado CANCELLED", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelRide_InvalidTransition_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(userID, nil, "COMPLETED"))

	// No UPDATE expected — CanTransition should fail first

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/cancel", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CancelRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelRide_NonOwner_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	otherID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(otherID, nil, "REQUESTED"))

	// No UPDATE expected — access control fails first

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/cancel", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CancelRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelRide_RaceCondition_Returns409(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(userID, nil, "REQUESTED"))

	mock.ExpectExec("UPDATE rides").
		WithArgs(userID.String(), rideID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/cancel", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CancelRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusConflict)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// WKT helper tests
// ========================================================

func TestParsePoint_HappyPath_ReturnsCoords(t *testing.T) {
	lon, lat, err := parsePoint("POINT(-58.3816 -34.6037)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lon != -58.3816 {
		t.Errorf("lon=%v, esperado -58.3816", lon)
	}
	if lat != -34.6037 {
		t.Errorf("lat=%v, esperado -34.6037", lat)
	}
}

func TestParsePoint_EmptyString_ReturnsError(t *testing.T) {
	_, _, err := parsePoint("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParsePoint_InvalidWKT_ReturnsError(t *testing.T) {
	_, _, err := parsePoint("POINT(invalid)")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFormatPoint_ReturnsWKT(t *testing.T) {
	wkt := formatPoint(-58.3816, -34.6037)
	if wkt != "POINT(-58.3816 -34.6037)" {
		t.Errorf("wkt=%q, esperado POINT(-58.3816 -34.6037)", wkt)
	}
}
