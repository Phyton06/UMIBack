package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockPool simula una base de datos para las pruebas de health.
type mockPool struct {
	pingErr error
}

func (m *mockPool) Ping(_ context.Context) error {
	return m.pingErr
}
func (m *mockPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row    { return nil }
func (m *mockPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func TestHandler_DBConectado_Retorna200(t *testing.T) {
	mock := &mockPool{}
	handler := Handler(mock)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("status = %q, esperado %q", body["status"], "ok")
	}
	if body["database"] != "connected" {
		t.Errorf("database = %q, esperado %q", body["database"], "connected")
	}
}

func TestHandler_DBDesconectado_Retorna503(t *testing.T) {
	mock := &mockPool{pingErr: errors.New("conexion rechazada")}
	handler := Handler(mock)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, esperado %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)

	if body["status"] != "degraded" {
		t.Errorf("status = %q, esperado %q", body["status"], "degraded")
	}
	if body["database"] != "disconnected" {
		t.Errorf("database = %q, esperado %q", body["database"], "disconnected")
	}
}

func TestHandler_ContentTypeJSON(t *testing.T) {
	mock := &mockPool{}
	handler := Handler(mock)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, esperado %q", ct, "application/json")
	}
}
