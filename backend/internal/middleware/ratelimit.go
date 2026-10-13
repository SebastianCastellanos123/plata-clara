package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/httpx"
)

// RateLimiter implementa un "token bucket" por clave (IP):
// cada cliente tiene un balde con hasta `burst` fichas. Cada petición gasta una;
// el balde se rellena a `perMinute` fichas por minuto. Si no hay fichas → 429.
//
// Sirve para frenar ataques de fuerza bruta contra el login: probar miles de
// contraseñas por minuto deja de ser viable.
//
// Limitación: vive en memoria de UN proceso. Con varias instancias, cada una
// llevaría su propia cuenta (la solución real sería Redis).
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	rate      float64 // fichas por segundo
	burst     float64
	now       func() time.Time
	lastPrune time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// Allow dice si la clave puede hacer una petición ahora. Si no, devuelve
// cuánto tiempo falta para que tenga una ficha.
func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.prune(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	// Rellenar según el tiempo transcurrido, sin pasar del máximo.
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
	return false, wait
}

// prune borra clientes inactivos para que el mapa no crezca sin límite
// (si no, un atacante con muchas IPs podría agotar la memoria).
func (l *RateLimiter) prune(now time.Time) {
	if now.Sub(l.lastPrune) < time.Minute {
		return
	}
	l.lastPrune = now
	for k, b := range l.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.buckets, k)
		}
	}
}

// Middleware aplica el límite usando la IP del cliente como clave.
func (l *RateLimiter) Middleware(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := l.Allow(ClientIP(r, trustProxy))
			if !ok {
				secs := int(math.Ceil(wait.Seconds()))
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				httpx.WriteError(w, http.StatusTooManyRequests,
					"Demasiados intentos. Espera "+strconv.Itoa(secs)+" segundos e intenta de nuevo")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP obtiene la IP del cliente.
//
// Si NO hay proxy, usamos la IP de la conexión (no se puede falsificar).
// Detrás de un proxy (Render, Fly, nginx...) toda la conexión viene del proxy,
// así que miramos X-Forwarded-For. Tomamos la entrada MÁS A LA DERECHA porque
// es la que añadió nuestro proxy de confianza; las de la izquierda las puede
// escribir el atacante. Por eso TRUST_PROXY está apagado por defecto.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
