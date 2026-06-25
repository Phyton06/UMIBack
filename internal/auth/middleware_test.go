package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAuth_TokenValido_PasaClaims(t *testing.T) {
	secret := []byte("test-secret")
	userID := uuid.New()
	tokenStr, err := SignAccessToken(userID, "rider", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	var gotClaims *Claims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = r.Context().Value(ClaimsKey).(*Claims)
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth(secret)(handler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, esperado %d", w.Code, http.StatusOK)
	}
	if gotClaims == nil {
		t.Fatal("claims no deberían ser nil")
	}
	if gotClaims.Role != "rider" {
		t.Errorf("role = %q, esperado %q", gotClaims.Role, "rider")
	}
	if gotClaims.Subject != userID.String() {
		t.Errorf("sub = %q, esperado %q", gotClaims.Subject, userID.String())
	}
}

func TestAuth_SinHeader_Retorna401(t *testing.T) {
	secret := []byte("test-secret")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler no debería ejecutarse")
	})

	middleware := Auth(secret)(handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "missing authorization header" {
		t.Errorf("error = %q, esperado %q", body["error"], "missing authorization header")
	}
}

func TestAuth_TokenInvalido_Retorna401(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler no debería ejecutarse")
	})

	middleware := Auth([]byte("secret"))(handler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token-invalido")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "invalid token" {
		t.Errorf("error = %q, esperado %q", body["error"], "invalid token")
	}
}

func TestAuth_TokenExpirado_Retorna401(t *testing.T) {
	secret := []byte("test-secret")
	userID := uuid.New()
	tokenStr, err := SignAccessToken(userID, "rider", secret, -5*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler no debería ejecutarse")
	})

	middleware := Auth(secret)(handler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "token expired" {
		t.Errorf("error = %q, esperado %q", body["error"], "token expired")
	}
}

func TestRequireRole_RolCoincide_Pasa(t *testing.T) {
	secret := []byte("test-secret")
	userID := uuid.New()
	tokenStr, err := SignAccessToken(userID, "driver", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	stack := Auth(secret)(RequireRole("driver")(handler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	stack.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, esperado %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler debería haber sido llamado")
	}
}

func TestRequireRole_RolNoCoincide_Retorna403(t *testing.T) {
	secret := []byte("test-secret")
	userID := uuid.New()
	tokenStr, err := SignAccessToken(userID, "rider", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler no debería ejecutarse con rol incorrecto")
	})

	stack := Auth(secret)(RequireRole("driver")(handler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	stack.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, esperado %d", resp.StatusCode, http.StatusForbidden)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "forbidden" {
		t.Errorf("error = %q, esperado %q", body["error"], "forbidden")
	}
}
