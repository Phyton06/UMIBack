package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestSignAccessToken_Valido_RetornaClaims(t *testing.T) {
	userID := uuid.New()
	secret := []byte("mi-secreto-super-seguro")
	expiry := 15 * time.Minute

	tokenStr, err := SignAccessToken(userID, "rider", secret, expiry)
	if err != nil {
		t.Fatalf("SignAccessToken: error inesperado %v", err)
	}

	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("ValidateToken: error inesperado %v", err)
	}

	if claims.Role != "rider" {
		t.Errorf("role = %q, esperado %q", claims.Role, "rider")
	}
	if claims.Subject != userID.String() {
		t.Errorf("sub = %q, esperado %q", claims.Subject, userID.String())
	}
}

func TestSignAccessToken_Expirado_Falla(t *testing.T) {
	userID := uuid.New()
	secret := []byte("mi-secreto")

	tokenStr, err := SignAccessToken(userID, "driver", secret, -1*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: error inesperado %v", err)
	}

	_, err = ValidateToken(tokenStr, secret)
	if err == nil {
		t.Fatal("ValidateToken: esperado error por token expirado, obtenido nil")
	}
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Errorf("error = %v, esperado jwt.ErrTokenExpired", err)
	}
}

func TestSignAccessToken_SecretoIncorrecto_Falla(t *testing.T) {
	userID := uuid.New()

	tokenStr, err := SignAccessToken(userID, "rider", []byte("secreto-a"), 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken: error inesperado %v", err)
	}

	_, err = ValidateToken(tokenStr, []byte("secreto-b"))
	if err == nil {
		t.Fatal("ValidateToken: esperado error por secreto incorrecto, obtenido nil")
	}
}

func TestSignAccessToken_Malformed_Falla(t *testing.T) {
	_, err := ValidateToken("esto-no-es-un-jwt", []byte("secreto"))
	if err == nil {
		t.Fatal("ValidateToken: esperado error por token malformado, obtenido nil")
	}
}

func TestSignRefreshToken_Valido_RetornaClaimsConTokenID(t *testing.T) {
	userID := uuid.New()
	secret := []byte("mi-secreto")
	expiry := 168 * time.Hour

	tokenStr, data, err := SignRefreshToken(userID, "driver", secret, expiry)
	if err != nil {
		t.Fatalf("SignRefreshToken: error inesperado %v", err)
	}
	if data == nil {
		t.Fatal("RefreshTokenData es nil")
	}
	if data.TokenID == uuid.Nil {
		t.Error("TokenID no debería ser zero-value")
	}
	if len(data.TokenHash) == 0 {
		t.Error("TokenHash no debería estar vacío")
	}
	if data.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt debería ser futuro")
	}

	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("ValidateToken refresh: error inesperado %v", err)
	}
	if claims.Role != "driver" {
		t.Errorf("role = %q, esperado %q", claims.Role, "driver")
	}
	if claims.Subject != userID.String() {
		t.Errorf("sub = %q, esperado %q", claims.Subject, userID.String())
	}
	if claims.TokenID != data.TokenID {
		t.Errorf("token_id = %v, esperado %v", claims.TokenID, data.TokenID)
	}
}
