package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func init() { BcryptCost = 4 } // costo mínimo: los tests corren rápido

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("una-clave-bien-larga")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "una-clave-bien-larga") {
		t.Fatal("el hash no debe contener la contraseña")
	}
	if !CheckPassword(hash, "una-clave-bien-larga") {
		t.Fatal("la contraseña correcta debe validar")
	}
	if CheckPassword(hash, "otra-clave-distinta") {
		t.Fatal("una contraseña incorrecta no debe validar")
	}
	// Dos hashes de la misma contraseña son distintos (cada uno lleva su "sal").
	hash2, _ := HashPassword("una-clave-bien-larga")
	if hash == hash2 {
		t.Fatal("los hashes deberían diferir por la sal aleatoria")
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"muy corta", "corta123", false},
		{"justo 10", "1234567890", true},
		{"larga válida", "esta es una frase larga y segura", true},
		{"más de 72 bytes", strings.Repeat("a", 73), false},
		{"exactamente 72 bytes", strings.Repeat("a", 72), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidatePassword(c.pw); (err == nil) != c.ok {
				t.Fatalf("ValidatePassword(%q) err=%v, esperaba ok=%v", c.pw, err, c.ok)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	good, err := NormalizeEmail("  Ana.Perez@Example.COM ")
	if err != nil || good != "ana.perez@example.com" {
		t.Fatalf("got %q, %v", good, err)
	}
	bad := []string{"", "sin-arroba", "a@", "Nombre <a@b.com>", "a@b.com, c@d.com", strings.Repeat("a", 250) + "@b.com"}
	for _, e := range bad {
		if _, err := NormalizeEmail(e); err == nil {
			t.Errorf("%q debería ser inválido", e)
		}
	}
}

func newTM() *TokenManager {
	return NewTokenManager([]byte("secreto-de-prueba-de-al-menos-32-caracteres!"), time.Hour)
}

func TestTokenRoundTrip(t *testing.T) {
	m := newTM()
	tok, _, err := m.Issue("user-123")
	if err != nil {
		t.Fatal(err)
	}
	id, err := m.Parse(tok)
	if err != nil || id != "user-123" {
		t.Fatalf("got %q, %v", id, err)
	}
}

func TestTokenRejectsTampering(t *testing.T) {
	m := newTM()
	tok, _, _ := m.Issue("user-123")

	// 1) Cambiar un carácter de la firma.
	bad := tok[:len(tok)-2] + "xx"
	if _, err := m.Parse(bad); err == nil {
		t.Error("un token con la firma alterada debe rechazarse")
	}

	// 2) Token firmado con OTRA clave.
	other := NewTokenManager([]byte("otra-clave-totalmente-distinta-32-chars!!"), time.Hour)
	forged, _, _ := other.Issue("user-123")
	if _, err := m.Parse(forged); err == nil {
		t.Error("un token firmado con otra clave debe rechazarse")
	}

	// 3) Basura.
	for _, junk := range []string{"", "abc", "a.b.c"} {
		if _, err := m.Parse(junk); err == nil {
			t.Errorf("%q debe rechazarse", junk)
		}
	}
}

func TestTokenRejectsAlgNone(t *testing.T) {
	m := newTM()
	// Ataque clásico: un token con algoritmo "none" (sin firma) diciendo ser otro usuario.
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   "admin",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("un token alg=none debe rechazarse")
	}
}

func TestTokenRejectsWrongAlgorithm(t *testing.T) {
	m := newTM()
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   "user-123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	// Firmado con HS512 usando la clave correcta: igual debe rechazarse, porque solo aceptamos HS256.
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(m.secret)
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("solo debe aceptarse HS256")
	}
}

func TestTokenExpires(t *testing.T) {
	m := newTM()
	now := time.Now()
	m.now = func() time.Time { return now }
	tok, _, _ := m.Issue("user-123")

	m.now = func() time.Time { return now.Add(59 * time.Minute) }
	if _, err := m.Parse(tok); err != nil {
		t.Fatalf("a los 59 min debería seguir válido: %v", err)
	}
	m.now = func() time.Time { return now.Add(61 * time.Minute) }
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("pasada la hora debe estar expirado")
	}
}

func TestTokenRejectsWrongIssuer(t *testing.T) {
	m := newTM()
	claims := jwt.RegisteredClaims{
		Issuer:    "otro-sistema",
		Subject:   "user-123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("un emisor distinto debe rechazarse")
	}
}
