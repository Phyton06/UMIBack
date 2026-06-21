package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
)

// GenerateOTP genera un código numérico de la longitud especificada usando
// crypto/rand. Retorna error si no se puede leer de la fuente de aleatoriedad.
func GenerateOTP(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("otp length debe ser positivo, recibido %d", length)
	}

	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generate otp: %w", err)
		}
		code[i] = byte('0' + n.Int64())
	}

	return string(code), nil
}

// HashOTP retorna el hash SHA-256 del código OTP.
func HashOTP(code string) []byte {
	h := sha256.Sum256([]byte(code))
	return h[:]
}
