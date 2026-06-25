// Package auth proporciona autenticación por OTP y manejo de sesiones JWT.
package auth

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims representa los datos incluidos en un JWT de acceso o actualización.
type Claims struct {
	jwt.RegisteredClaims
	Role    string    `json:"role"`
	TokenID uuid.UUID `json:"token_id,omitempty"` // solo presente en refresh tokens
}

// RefreshTokenData contiene los metadatos de un refresh token para almacenar en DB.
type RefreshTokenData struct {
	TokenID   uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
}

// SignAccessToken firma un JWT de acceso con los claims provistos.
func SignAccessToken(userID uuid.UUID, role string, secret []byte, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
		Role: role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// SignRefreshToken firma un JWT de actualización y retorna el token firmado
// junto con los metadatos necesarios para almacenarlo en la base de datos.
// El TokenHash en RefreshTokenData es el SHA-256 del token firmado.
func SignRefreshToken(userID uuid.UUID, role string, secret []byte, expiry time.Duration) (string, *RefreshTokenData, error) {
	now := time.Now()
	tokenID := uuid.New()
	expiresAt := now.Add(expiry)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Role:    role,
		TokenID: tokenID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", nil, fmt.Errorf("sign refresh token: %w", err)
	}

	hash := sha256.Sum256([]byte(signed))
	data := &RefreshTokenData{
		TokenID:   tokenID,
		TokenHash: hash[:],
		ExpiresAt: expiresAt,
	}

	return signed, data, nil
}

// ValidateToken analiza y valida un JWT, retornando los claims si es válido.
func ValidateToken(tokenStr string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("claims inválidos")
	}

	return claims, nil
}


