package auth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost define cuánto trabajo cuesta calcular un hash. Cada +1 duplica el
// tiempo. Con 12 tarda ~250 ms: imperceptible para un usuario legítimo, pero
// carísimo para un atacante que prueba millones de contraseñas.
// Es una variable (no constante) solo para que los tests la bajen y corran rápido.
var BcryptCost = 12

const (
	minPasswordLen = 10
	// bcrypt solo mira los primeros 72 bytes. Rechazamos contraseñas más largas
	// en lugar de truncarlas en silencio.
	maxPasswordBytes = 72
)

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(h), err
}

// CheckPassword compara en tiempo constante (lo hace bcrypt internamente).
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ValidatePassword aplica la política mínima. Seguimos la guía NIST: importa
// más el largo que obligar a mezclar símbolos.
func ValidatePassword(p string) error {
	if utf8.RuneCountInString(p) < minPasswordLen {
		return errors.New("La contraseña debe tener al menos 10 caracteres")
	}
	if len(p) > maxPasswordBytes {
		return errors.New("La contraseña es demasiado larga (máximo 72 bytes)")
	}
	return nil
}

// NormalizeEmail devuelve el correo en minúsculas y sin espacios. Normalizar
// evita que "Ana@x.com" y "ana@x.com" sean dos cuentas distintas.
func NormalizeEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" || len(e) > 254 {
		return "", errors.New("Correo inválido")
	}
	addr, err := mail.ParseAddress(e)
	// ParseAddress acepta "Nombre <a@b.com>"; exigimos que sea SOLO la dirección.
	if err != nil || addr.Address != e {
		return "", errors.New("Correo inválido")
	}
	return e, nil
}
