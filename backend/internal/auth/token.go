package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "plata-clara"

// TokenManager firma y verifica los JWT de sesión.
//
// Un JWT es un texto con 3 partes (cabecera.datos.firma). La firma se calcula
// con nuestra clave secreta; si alguien cambia los datos (por ejemplo, el id
// de usuario), la firma deja de coincidir y lo rechazamos.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // inyectable para probar expiración sin esperar
}

func NewTokenManager(secret []byte, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: secret, ttl: ttl, now: time.Now}
}

// Issue crea un token para el usuario y devuelve también su fecha de expiración.
func (m *TokenManager) Issue(userID string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return signed, exp, err
}

var ErrInvalidToken = errors.New("token inválido")

// Parse valida firma, algoritmo, emisor y expiración, y devuelve el id de usuario.
func (m *TokenManager) Parse(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Fijar el algoritmo permitido bloquea el clásico ataque "alg: none"
		// (token sin firma) y el de cambiar HS256 por otro algoritmo.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
