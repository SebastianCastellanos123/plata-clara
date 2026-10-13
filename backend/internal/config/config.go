// Package config lee la configuración desde variables de entorno.
//
// Regla de oro: los secretos (contraseña de la BD, clave del JWT) NUNCA van
// en el código ni en el repositorio. Siempre entran por el entorno.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    []byte
	Production   bool
	CookieSecure bool          // la cookie solo viaja por HTTPS
	TrustProxy   bool          // confiar en X-Forwarded-For (solo detrás de un proxy propio)
	StaticDir    string        // carpeta con el build del frontend (opcional)
	TokenTTL     time.Duration // cuánto dura una sesión

	// Límite de intentos para login y registro (por IP).
	AuthRatePerMin int // fichas que se recuperan por minuto
	AuthBurst      int // intentos seguidos permitidos antes de bloquear
}

const minSecretLen = 32

func Load() (Config, error) {
	env := strings.ToLower(getenv("APP_ENV", "development"))
	prod := env == "production"

	cfg := Config{
		Port:       getenv("PORT", "8080"),
		Production: prod,
		StaticDir:  os.Getenv("STATIC_DIR"),
		TokenTTL:   8 * time.Hour,

		AuthRatePerMin: 10,
		AuthBurst:      5,
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("falta DATABASE_URL")
	}

	// Una clave corta o adivinable permite falsificar sesiones. Si no cumple,
	// el servidor NO arranca: es mejor fallar fuerte que correr inseguro.
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < minSecretLen {
		return Config{}, fmt.Errorf("JWT_SECRET debe tener al menos %d caracteres", minSecretLen)
	}
	cfg.JWTSecret = []byte(secret)

	var err error
	// En producción la cookie es Secure por defecto. En desarrollo (http://localhost)
	// el navegador no la guardaría si fuera Secure, así que default = false.
	if cfg.CookieSecure, err = getbool("COOKIE_SECURE", prod); err != nil {
		return Config{}, err
	}
	if cfg.TrustProxy, err = getbool("TRUST_PROXY", false); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s debe ser true o false", key)
	}
	return b, nil
}
