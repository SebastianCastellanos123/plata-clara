package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimiterBlocksAndRefills(t *testing.T) {
	l := NewRateLimiter(60, 3) // 1 ficha por segundo, ráfaga de 3
	now := time.Now()
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("1.1.1.1"); !ok {
			t.Fatalf("la petición %d debería pasar", i+1)
		}
	}
	ok, wait := l.Allow("1.1.1.1")
	if ok {
		t.Fatal("la 4.ª petición seguida debe bloquearse")
	}
	if wait <= 0 || wait > 2*time.Second {
		t.Fatalf("espera inesperada: %v", wait)
	}

	// Otra IP tiene su propio balde.
	if ok, _ := l.Allow("2.2.2.2"); !ok {
		t.Fatal("otra IP no debe verse afectada")
	}

	// Pasan 2 segundos → se recuperan 2 fichas.
	now = now.Add(2 * time.Second)
	if ok, _ := l.Allow("1.1.1.1"); !ok {
		t.Fatal("tras esperar debería poder pasar de nuevo")
	}
}

func TestRateLimiterMiddleware429(t *testing.T) {
	l := NewRateLimiter(1, 1)
	h := l.Middleware(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	do := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/auth/login", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	if rr := do(); rr.Code != 200 {
		t.Fatalf("primera: %d", rr.Code)
	}
	rr := do()
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("segunda: %d, esperaba 429", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("falta la cabecera Retry-After")
	}
}

func TestPruneRemovesIdleEntries(t *testing.T) {
	l := NewRateLimiter(60, 1)
	now := time.Now()
	l.now = func() time.Time { return now }
	l.Allow("viejo")
	now = now.Add(30 * time.Minute)
	l.Allow("nuevo") // dispara la limpieza
	if _, exists := l.buckets["viejo"]; exists {
		t.Fatal("las entradas inactivas deberían borrarse")
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:5555"
	// El atacante intenta falsificar su IP escribiendo la izquierda del header;
	// el proxy de confianza agrega la IP real a la derecha.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.7")

	if got := ClientIP(req, false); got != "10.0.0.5" {
		t.Errorf("sin proxy de confianza debe ignorar el header: %q", got)
	}
	if got := ClientIP(req, true); got != "203.0.113.7" {
		t.Errorf("con proxy debe usar la entrada de la derecha: %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/me", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}
	for k, v := range want {
		if got := rr.Header().Get(k); got != v {
			t.Errorf("%s = %q, esperaba %q", k, got, v)
		}
	}
	if csp := rr.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") ||
		!strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP inesperada: %q", csp)
	}
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("falta HSTS cuando está activado")
	}

	// Sin HSTS (desarrollo en http) no debe enviarse.
	h2 := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rr2 := httptest.NewRecorder()
	h2.ServeHTTP(rr2, httptest.NewRequest("GET", "/", nil))
	if rr2.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS no debe enviarse en desarrollo")
	}
}

func TestRequireXHR(t *testing.T) {
	h := RequireXHR(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	run := func(method, header string) int {
		req := httptest.NewRequest(method, "/api/expenses", nil)
		if header != "" {
			req.Header.Set("X-Requested-With", header)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	if c := run("GET", ""); c != 200 {
		t.Errorf("GET sin header debe pasar: %d", c)
	}
	if c := run("POST", ""); c != 403 {
		t.Errorf("POST sin header debe dar 403: %d", c)
	}
	if c := run("DELETE", "otra-cosa"); c != 403 {
		t.Errorf("DELETE con header incorrecto debe dar 403: %d", c)
	}
	if c := run("POST", "plata-clara"); c != 200 {
		t.Errorf("POST con header correcto debe pasar: %d", c)
	}
}
