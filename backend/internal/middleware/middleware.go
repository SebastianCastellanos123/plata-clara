// Package middleware contiene funciones que "envuelven" a los handlers para
// añadir comportamiento común (logs, cabeceras de seguridad, límites...).
//
// Un middleware recibe un http.Handler y devuelve otro: puede hacer algo antes,
// llamar al siguiente, y hacer algo después.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/httpx"
)

// statusRecorder guarda el código de respuesta para poder registrarlo en el log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Recover evita que un panic tumbe el servidor: lo registra y responde 500.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recuperado", "panic", rec, "path", r.URL.Path)
					httpx.WriteError(w, http.StatusInternalServerError, "Ocurrió un error. Intenta de nuevo")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logging registra cada petición. OJO: loguea el PATH, nunca el cuerpo ni las
// cabeceras, para que contraseñas y cookies no terminen en los logs.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// SecurityHeaders añade cabeceras que le piden al navegador activar defensas.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			// CSP: solo se ejecuta JS/CSS que venga de nuestro propio dominio.
			// Aunque alguien lograra inyectar un <script> en la página (XSS),
			// el navegador se negaría a ejecutarlo.
			h.Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; "+
					"font-src 'self'; connect-src 'self'; object-src 'none'; "+
					"base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
			h.Set("X-Content-Type-Options", "nosniff")         // no adivinar tipos de archivo
			h.Set("X-Frame-Options", "DENY")                   // no permitir iframes (clickjacking)
			h.Set("Referrer-Policy", "no-referrer")            // no filtrar URLs a otros sitios
			h.Set("Cross-Origin-Opener-Policy", "same-origin") // aísla la ventana
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if hsts {
				// Obliga al navegador a usar HTTPS siempre con este dominio.
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				h.Set("Cache-Control", "no-store") // datos privados: que nada los cachee
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireXHR es una defensa extra contra CSRF (que otro sitio haga peticiones
// en nombre del usuario). Para métodos que modifican datos exigimos la cabecera
// X-Requested-With. Un sitio ajeno no puede añadirla sin un permiso CORS
// previo, y nosotros nunca damos ese permiso. Se suma a SameSite=Strict.
func RequireXHR(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("X-Requested-With") != "plata-clara" {
				httpx.WriteError(w, http.StatusForbidden, "Solicitud no permitida")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
