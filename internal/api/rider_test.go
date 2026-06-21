package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"

	"github.com/Phyton06/UMIBack/internal/auth"
)

// ========================================================
// RiderStats tests
// ========================================================

func TestRiderStats_HasTrips_ReturnsAggregates(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	memberSince := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT created_at FROM users WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"created_at"}).AddRow(memberSince))

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count", "count", "coalesce", "coalesce"}).
			AddRow(int64(5), int64(5), 150.00, 12.3))

	req := httptest.NewRequest("GET", "/rider/stats", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	RiderStats(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)

	if m["total_trips"].(float64) != 5 {
		t.Errorf("total_trips=%v, esperado 5", m["total_trips"])
	}
	if m["completed_trips"].(float64) != 5 {
		t.Errorf("completed_trips=%v, esperado 5", m["completed_trips"])
	}
	if m["total_spent"].(float64) != 150.00 {
		t.Errorf("total_spent=%v, esperado 150.00", m["total_spent"])
	}
	if m["total_distance_km"].(float64) != 12.3 {
		t.Errorf("total_distance_km=%v, esperado 12.3", m["total_distance_km"])
	}
	if m["member_since"] != "2026-01-15T00:00:00Z" {
		t.Errorf("member_since=%v, esperado 2026-01-15T00:00:00Z", m["member_since"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRiderStats_NewRider_ReturnsZeros(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	memberSince := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT created_at FROM users WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"created_at"}).AddRow(memberSince))

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count", "count", "coalesce", "coalesce"}).
			AddRow(int64(0), int64(0), 0.0, 0.0))

	req := httptest.NewRequest("GET", "/rider/stats", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	RiderStats(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)

	if m["total_trips"].(float64) != 0 {
		t.Errorf("total_trips=%v, esperado 0", m["total_trips"])
	}
	if m["completed_trips"].(float64) != 0 {
		t.Errorf("completed_trips=%v, esperado 0", m["completed_trips"])
	}
	if m["total_spent"].(float64) != 0 {
		t.Errorf("total_spent=%v, esperado 0", m["total_spent"])
	}
	if m["total_distance_km"].(float64) != 0 {
		t.Errorf("total_distance_km=%v, esperado 0", m["total_distance_km"])
	}
	if m["member_since"] != "2026-03-10T00:00:00Z" {
		t.Errorf("member_since=%v, esperado 2026-03-10T00:00:00Z", m["member_since"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRiderStats_NoAuth_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	req := httptest.NewRequest("GET", "/rider/stats", nil)
	w := httptest.NewRecorder()
	RiderStats(mock)(w, req)

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
// RiderRides tests
// ========================================================

func TestRiderRides_HasRides_ReturnsPaginated(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	rideID1 := uuid.New()
	rideID2 := uuid.New()
	rideID3 := uuid.New()
	now := time.Now()
	fare1 := 50.0
	fare2 := 30.0
	fare3 := 20.0

	// Total count
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(3)))

	// Rides query
	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(userID, 10, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}).AddRow(
			rideID1, userID, nil, "COMPLETED",
			"POINT(-99.1333 19.4326)", "POINT(-99.1678 19.4123)",
			"Pickup 1", "Dropoff 1",
			&fare1, nil, nil, &now,
			now, now,
		).AddRow(
			rideID2, userID, nil, "COMPLETED",
			"POINT(-99.1433 19.4426)", "POINT(-99.1778 19.4223)",
			"Pickup 2", "Dropoff 2",
			&fare2, nil, nil, &now,
			now, now,
		).AddRow(
			rideID3, userID, nil, "COMPLETED",
			"POINT(-99.1533 19.4526)", "POINT(-99.1878 19.4323)",
			"Pickup 3", "Dropoff 3",
			&fare3, nil, nil, &now,
			now, now,
		))

	req := httptest.NewRequest("GET", "/rider/rides?limit=10&offset=0", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	RiderRides(mock)(w, req)

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
	if len(rides) != 3 {
		t.Fatalf("len(rides)=%d, esperado 3", len(rides))
	}
	if body["total"].(float64) != 3 {
		t.Errorf("total=%v, esperado 3", body["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRiderRides_Empty_ReturnsEmptyArray(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(userID, 10, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "passenger_id", "driver_id", "status",
			"pickup_location", "dropoff_location",
			"pickup_address", "dropoff_address",
			"fare", "cancelled_by", "cancelled_at", "completed_at",
			"created_at", "updated_at",
		}))

	req := httptest.NewRequest("GET", "/rider/rides", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	RiderRides(mock)(w, req)

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
	if body["total"].(float64) != 0 {
		t.Errorf("total=%v, esperado 0", body["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRiderRides_Pagination_SecondPage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(25)))

	// Build 10 ride rows for offset 10
	rows := pgxmock.NewRows([]string{
		"id", "passenger_id", "driver_id", "status",
		"pickup_location", "dropoff_location",
		"pickup_address", "dropoff_address",
		"fare", "cancelled_by", "cancelled_at", "completed_at",
		"created_at", "updated_at",
	})
	for range 10 {
		rows.AddRow(
			uuid.New(), userID, nil, "COMPLETED",
			"POINT(-99.1333 19.4326)", "POINT(-99.1678 19.4123)",
			"Addr", "Addr",
			nil, nil, nil, &now,
			now, now,
		)
	}

	mock.ExpectQuery("SELECT id, passenger_id, driver_id, status").
		WithArgs(userID, 10, 10).
		WillReturnRows(rows)

	req := httptest.NewRequest("GET", "/rider/rides?limit=10&offset=10", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	RiderRides(mock)(w, req)

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
	if len(rides) != 10 {
		t.Fatalf("len(rides)=%d, esperado 10", len(rides))
	}
	total, ok := body["total"].(float64)
	if !ok || total != 25 {
		t.Errorf("total=%v, esperado 25", body["total"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRiderRides_InvalidLimit_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	userID := uuid.New()

	req := httptest.NewRequest("GET", "/rider/rides?limit=200", nil)
	ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
		Role:             "rider",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	RiderRides(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRiderRides_NoAuth_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	req := httptest.NewRequest("GET", "/rider/rides", nil)
	w := httptest.NewRecorder()
	RiderRides(mock)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
