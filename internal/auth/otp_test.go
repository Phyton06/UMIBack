package auth

import (
	"bytes"
	"testing"
)

func TestGenerateOTP_LongitudCorrecta(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{name: "longitud 4", length: 4},
		{name: "longitud 6", length: 6},
		{name: "longitud 8", length: 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, err := GenerateOTP(tc.length)
			if err != nil {
				t.Fatalf("GenerateOTP(%d): error inesperado %v", tc.length, err)
			}
			if len(code) != tc.length {
				t.Errorf("longitud = %d, esperado %d", len(code), tc.length)
			}
			for i, ch := range code {
				if ch < '0' || ch > '9' {
					t.Errorf("carácter[%d] = %c, esperado dígito", i, ch)
				}
			}
		})
	}
}

func TestGenerateOTP_LengthCero_Error(t *testing.T) {
	_, err := GenerateOTP(0)
	if err == nil {
		t.Fatal("GenerateOTP(0): esperado error, obtenido nil")
	}
}

func TestGenerateOTP_LengthNegativo_Error(t *testing.T) {
	_, err := GenerateOTP(-1)
	if err == nil {
		t.Fatal("GenerateOTP(-1): esperado error, obtenido nil")
	}
}

func TestHashOTP_MismoCodigo_MismoHash(t *testing.T) {
	code := "123456"
	h1 := HashOTP(code)
	h2 := HashOTP(code)

	if !bytes.Equal(h1, h2) {
		t.Error("mismo código debería producir mismo hash")
	}
}

func TestHashOTP_DiferentesCodigos_DiferentesHashes(t *testing.T) {
	h1 := HashOTP("123456")
	h2 := HashOTP("654321")

	if bytes.Equal(h1, h2) {
		t.Error("códigos diferentes deberían producir hashes diferentes")
	}
}

func TestHashOTP_LongitudCorrecta(t *testing.T) {
	h := HashOTP("000000")
	if len(h) != 32 {
		t.Errorf("longitud del hash = %d, esperado 32 (SHA-256)", len(h))
	}
}
