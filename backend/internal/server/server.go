// Package server arma todas las piezas: rutas, middlewares y archivos estáticos.
package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/auth"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/config"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/expenses"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/httpx"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/middleware"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/users"
)

func New(cfg config.Config, conn *sql.DB, log *slog.Logger) (http.Handler, error) {
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.TokenTTL)
	authH, err := auth.NewHandler(users.NewStore(conn), tokens, cfg.CookieSecure, log)
	if err != nil {
		return nil, err
	}
	expH := expenses.NewHandler(expenses.NewStore(conn), log)

	// Límite para login/registro por IP (por defecto: ráfaga de 5 y 10 por minuto).
	limit := middleware.NewRateLimiter(cfg.AuthRatePerMin, cfg.AuthBurst).Middleware(cfg.TrustProxy)
	protect := authH.RequireAuth

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// --- Autenticación (públicas, con límite de intentos) ---
	mux.Handle("POST /api/auth/register", limit(http.HandlerFunc(authH.Register)))
	mux.Handle("POST /api/auth/login", limit(http.HandlerFunc(authH.Login)))
	mux.HandleFunc("POST /api/auth/logout", authH.Logout)

	// --- Rutas protegidas: sin sesión válida responden 401 ---
	mux.Handle("GET /api/me", protect(http.HandlerFunc(authH.Me)))
	mux.Handle("GET /api/expenses", protect(http.HandlerFunc(expH.List)))
	mux.Handle("GET /api/expenses/summary", protect(http.HandlerFunc(expH.Summary)))
	mux.Handle("POST /api/expenses", protect(http.HandlerFunc(expH.Create)))
	mux.Handle("PUT /api/expenses/{id}", protect(http.HandlerFunc(expH.Update)))
	mux.Handle("DELETE /api/expenses/{id}", protect(http.HandlerFunc(expH.Delete)))

	// Cualquier otra ruta /api/... → 404 en JSON (no el HTML del frontend).
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "Ruta no encontrada")
	})

	if cfg.StaticDir != "" {
		mux.Handle("/", spa(cfg.StaticDir))
	}

	// El orden importa: el primero de la lista es el más externo.
	// Recover → Logging → SecurityHeaders → RequireXHR → rutas
	var h http.Handler = mux
	h = middleware.RequireXHR(h)
	h = middleware.SecurityHeaders(cfg.CookieSecure)(h)
	h = middleware.Logging(log)(h)
	h = middleware.Recover(log)(h)
	return h, nil
}

// spa sirve el frontend compilado. Si la ruta no es un archivo real (por
// ejemplo /login), devolvemos index.html y React Router decide qué mostrar.
func spa(dir string) http.Handler {
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		// path.Clean("/"+...) elimina ".." : evita "path traversal" (que alguien
		// pida /../../etc/passwd y salga de la carpeta del frontend).
		clean := path.Clean("/" + r.URL.Path)
		full := filepath.Join(dir, filepath.FromSlash(clean))

		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				// Archivos con hash en el nombre: se pueden cachear para siempre.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFile(w, r, full)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
