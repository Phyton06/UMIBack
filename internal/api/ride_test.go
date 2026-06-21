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

// ========================================================
// AcceptRide tests
// ========================================================

func TestAcceptRide_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(true))

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, nil, "REQUESTED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(userID, rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("UPDATE drivers SET available").
		WithArgs(userID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	AcceptRide(mock)(w, req)

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
	if m["status"] != "ACCEPTED" {
		t.Errorf("status=%q, esperado ACCEPTED", m["status"])
	}
	if m["driver_id"] != userID.String() {
		t.Errorf("driver_id=%q, esperado %q", m["driver_id"], userID.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAcceptRide_DoubleAcceptRace_Returns409(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(true))

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, nil, "REQUESTED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(userID, rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	AcceptRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusConflict)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAcceptRide_DriverNotAvailable_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(false))

	// No further queries expected — fails at availability check

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	AcceptRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAcceptRide_OwnRide_Returns403(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(true))

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(userID, nil, "REQUESTED"))

	// No UPDATE expected — own-ride check fails

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	AcceptRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusForbidden)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAcceptRide_InvalidTransition_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(true))

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, nil, "COMPLETED"))

	// No UPDATE expected — CanTransition should fail first

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	AcceptRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// EnRouteRide tests
// ========================================================

func TestEnRouteRide_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &userID, "ACCEPTED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/en-route", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EnRouteRide(mock)(w, req)

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
	if m["status"] != "EN_ROUTE" {
		t.Errorf("status=%q, esperado EN_ROUTE", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnRouteRide_WrongStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &userID, "COMPLETED"))

	// No UPDATE expected — CanTransition fails first

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/en-route", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EnRouteRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnRouteRide_NotAssigned_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	otherDriverID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &otherDriverID, "ACCEPTED"))

	// No UPDATE expected — access control fails first

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/en-route", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EnRouteRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnRouteRide_RideNotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/en-route", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EnRouteRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// ArrivedRide tests
// ========================================================

func TestArrivedRide_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &userID, "EN_ROUTE"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/arrived", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	ArrivedRide(mock)(w, req)

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
	if m["status"] != "ARRIVED" {
		t.Errorf("status=%q, esperado ARRIVED", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestArrivedRide_WrongStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &userID, "COMPLETED"))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/arrived", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	ArrivedRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestArrivedRide_NotAssigned_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	otherDriverID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &otherDriverID, "EN_ROUTE"))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/arrived", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	ArrivedRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// StartRide tests
// ========================================================

func TestStartRide_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &userID, "ARRIVED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/start", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	StartRide(mock)(w, req)

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
	if m["status"] != "IN_PROGRESS" {
		t.Errorf("status=%q, esperado IN_PROGRESS", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestStartRide_WrongStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &userID, "COMPLETED"))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/start", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	StartRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestStartRide_NotAssigned_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	otherDriverID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &otherDriverID, "ARRIVED"))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/start", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	StartRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// CompleteRide tests
// ========================================================

func TestCompleteRide_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &userID, "IN_PROGRESS", 10000.0))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID, 80.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

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
	if m["status"] != "COMPLETED" {
		t.Errorf("status=%q, esperado COMPLETED", m["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompleteRide_WrongStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &userID, "REQUESTED", 5000.0))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompleteRide_NotAssigned_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, nil, "IN_PROGRESS", 5000.0))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompleteRide_ChecksCompletedAt(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &userID, "IN_PROGRESS", 10000.0))

	// The UPDATE must set completed_at = now() and fare
	mock.ExpectExec("UPDATE rides SET status = 'COMPLETED', completed_at").
		WithArgs(rideID, 80.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// E2E: accept → en-route → arrived → start → complete
// ========================================================

func TestRideProgress_E2E(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	driverID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	// --- accept ---
	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(driverID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(true))

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, nil, "REQUESTED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(driverID, rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("UPDATE drivers SET available").
		WithArgs(driverID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// --- en-route ---
	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &driverID, "ACCEPTED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// --- arrived ---
	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &driverID, "EN_ROUTE"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// --- start ---
	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status"}).
			AddRow(passengerID, &driverID, "ARRIVED"))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// --- complete ---
	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &driverID, "IN_PROGRESS", 10000.0))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID, 80.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: driverID.String()},
	}
	ctx := context.WithValue(context.Background(), auth.ClaimsKey, claims)

	// accept
	req1 := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req1.SetPathValue("id", rideID.String())
	req1 = req1.WithContext(ctx)
	w1 := httptest.NewRecorder()
	AcceptRide(mock)(w1, req1)
	if w1.Result().StatusCode != http.StatusOK {
		t.Fatal("accept failed")
	}

	// en-route
	req2 := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/en-route", nil)
	req2.SetPathValue("id", rideID.String())
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	EnRouteRide(mock)(w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatal("en-route failed")
	}

	// arrived
	req3 := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/arrived", nil)
	req3.SetPathValue("id", rideID.String())
	req3 = req3.WithContext(ctx)
	w3 := httptest.NewRecorder()
	ArrivedRide(mock)(w3, req3)
	if w3.Result().StatusCode != http.StatusOK {
		t.Fatal("arrived failed")
	}

	// start
	req4 := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/start", nil)
	req4.SetPathValue("id", rideID.String())
	req4 = req4.WithContext(ctx)
	w4 := httptest.NewRecorder()
	StartRide(mock)(w4, req4)
	if w4.Result().StatusCode != http.StatusOK {
		t.Fatal("start failed")
	}

	// complete
	req5 := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req5.SetPathValue("id", rideID.String())
	req5 = req5.WithContext(ctx)
	w5 := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w5, req5)
	if w5.Result().StatusCode != http.StatusOK {
		t.Fatal("complete failed")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ========================================================
// CalcFare tests
// ========================================================

func TestCalcFare_LongTrip_ReturnsDistanceBased(t *testing.T) {
	fare := CalcFare(10, 8.0, 25.0)
	if fare != 80.0 {
		t.Errorf("CalcFare(10, 8, 25) = %v, esperado 80.0", fare)
	}
}

func TestCalcFare_ShortTrip_ReturnsMinimum(t *testing.T) {
	fare := CalcFare(2, 8.0, 25.0)
	if fare != 25.0 {
		t.Errorf("CalcFare(2, 8, 25) = %v, esperado 25.0", fare)
	}
}

func TestCalcFare_ZeroDistance_ReturnsMinimum(t *testing.T) {
	fare := CalcFare(0, 8.0, 25.0)
	if fare != 25.0 {
		t.Errorf("CalcFare(0, 8, 25) = %v, esperado 25.0", fare)
	}
}

func TestCalcFare_Rounding_TwoDecimals(t *testing.T) {
	fare := CalcFare(3.333, 8.0, 25.0)
	if fare != 26.66 {
		t.Errorf("CalcFare(3.333, 8, 25) = %v, esperado 26.66", fare)
	}
}

func TestCalcFare_ExactMinimum_ReturnsFare(t *testing.T) {
	fare := CalcFare(3.125, 8.0, 25.0)
	if fare != 25.0 {
		t.Errorf("CalcFare(3.125, 8, 25) = %v, esperado 25.0", fare)
	}
}

func TestCalcFare_RoundingEdge_Upward(t *testing.T) {
	fare := CalcFare(3.1251, 8.0, 25.0)
	if fare != 25.0 {
		t.Errorf("CalcFare(3.1251, 8, 25) = %v, esperado 25.0", fare)
	}
}

// ========================================================
// EstimateRide tests
// ========================================================

func TestEstimateRide_HappyPath_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	pickupLon := -99.1333
	pickupLat := 19.4326
	dropoffLon := -99.1678
	dropoffLat := 19.4123

	mock.ExpectQuery("SELECT ST_Distance").
		WithArgs(pickupLon, pickupLat, dropoffLon, dropoffLat).
		WillReturnRows(pgxmock.NewRows([]string{"st_distance"}).AddRow(5000.0))

	body := mustMarshal(t, map[string]any{
		"pickup_lon":  pickupLon,
		"pickup_lat":  pickupLat,
		"dropoff_lon": dropoffLon,
		"dropoff_lat": dropoffLat,
	})
	req := httptest.NewRequest("POST", "/rides/estimate", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EstimateRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	fare, ok := m["estimated_fare"].(float64)
	if !ok {
		t.Fatal("estimated_fare no es float64")
	}
	if fare != 40.0 {
		t.Errorf("estimated_fare=%v, esperado 40.0", fare)
	}
	if m["rate_per_km"] != 8.0 {
		t.Errorf("rate_per_km=%v, esperado 8.0", m["rate_per_km"])
	}
	if m["minimum_fare"] != 25.0 {
		t.Errorf("minimum_fare=%v, esperado 25.0", m["minimum_fare"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEstimateRide_MinimumFare_ReturnsMinimum(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectQuery("SELECT ST_Distance").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"st_distance"}).AddRow(500.0))

	body := mustMarshal(t, map[string]any{
		"pickup_lon":  -99.1333,
		"pickup_lat":  19.4326,
		"dropoff_lon": -99.1678,
		"dropoff_lat": 19.4123,
	})
	req := httptest.NewRequest("POST", "/rides/estimate", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EstimateRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	fare, ok := m["estimated_fare"].(float64)
	if !ok {
		t.Fatal("estimated_fare no es float64")
	}
	if fare != 25.0 {
		t.Errorf("estimated_fare=%v, esperado 25.0", fare)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEstimateRide_MissingFields_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	body := mustMarshal(t, map[string]any{
		"pickup_lat":  19.4326,
		"dropoff_lon": -99.1678,
		"dropoff_lat": 19.4123,
	})
	req := httptest.NewRequest("POST", "/rides/estimate", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EstimateRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestEstimateRide_InvalidLon_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	body := mustMarshal(t, map[string]any{
		"pickup_lon":  200.0,
		"pickup_lat":  19.4326,
		"dropoff_lon": -99.1678,
		"dropoff_lat": 19.4123,
	})
	req := httptest.NewRequest("POST", "/rides/estimate", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	EstimateRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestEstimateRide_NoAuth_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	body := mustMarshal(t, map[string]any{
		"pickup_lon":  -99.1333,
		"pickup_lat":  19.4326,
		"dropoff_lon": -99.1678,
		"dropoff_lat": 19.4123,
	})
	req := httptest.NewRequest("POST", "/rides/estimate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	EstimateRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// ========================================================
// CompleteRide fare tests
// ========================================================

func TestCompleteRide_WithFare_HappyDistance_Returns200(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &userID, "IN_PROGRESS", 10000.0))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID, 80.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

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
	if m["status"] != "COMPLETED" {
		t.Errorf("status=%q, esperado COMPLETED", m["status"])
	}
	fare, ok := m["fare"].(float64)
	if !ok {
		t.Fatal("fare no es float64")
	}
	if fare != 80.0 {
		t.Errorf("fare=%v, esperado 80.0", fare)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompleteRide_WithFare_Minimum_ReturnsMinimum(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &userID, "IN_PROGRESS", 500.0))

	mock.ExpectExec("UPDATE rides SET status").
		WithArgs(rideID, 25.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	fare, ok := m["fare"].(float64)
	if !ok {
		t.Fatal("fare no es float64")
	}
	if fare != 25.0 {
		t.Errorf("fare=%v, esperado 25.0", fare)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompleteRide_WithFare_WrongStatus_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, &userID, "REQUESTED", 5000.0))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompleteRide_WithFare_NotAssigned_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	passengerID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT passenger_id, driver_id, status, ST_Distance").
		WithArgs(rideID).
		WillReturnRows(pgxmock.NewRows([]string{"passenger_id", "driver_id", "status", "st_distance"}).
			AddRow(passengerID, nil, "IN_PROGRESS", 5000.0))

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/complete", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	CompleteRide(mock, 8.0, 25.0)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAcceptRide_RideNotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID := uuid.New()

	mock.ExpectQuery("SELECT available FROM drivers WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"available"}).AddRow(true))

	mock.ExpectQuery("SELECT passenger_id, driver_id, status FROM rides WHERE id").
		WithArgs(rideID).
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest("PATCH", "/rides/"+rideID.String()+"/accept", nil)
	req.SetPathValue("id", rideID.String())
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	AcceptRide(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
