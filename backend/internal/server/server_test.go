package server_test

// Tests de INTEGRACIÓN: levantan el servidor completo con una base de datos
// PostgreSQL real y hacen peticiones HTTP como lo haría el frontend.
//
// Se activan definiendo TEST_DATABASE_URL, por ejemplo:
//   TEST_DATABASE_URL="postgres://plata:plata@localhost:5432/plata_clara_test?sslmode=disable" go test ./...
// Sin esa variable, estos tests se omiten (los unitarios siguen corriendo).
//
// ¡CUIDADO! Estos tests BORRAN las tablas de esa base de datos. Usa una BD de pruebas.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/auth"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/config"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/db"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/server"
)

const testSecret = "secreto-de-pruebas-de-mas-de-32-caracteres-ok"

func init() { auth.BcryptCost = 4 }

func newServer(t *testing.T, mutate func(*config.Config)) (*httptest.Server, *sql.DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no está definida; se omiten los tests de integración")
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	conn, err := db.Open(ctx, url, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Empezar de cero en cada test.
	if _, err := conn.Exec(`DROP TABLE IF EXISTS expenses, users, schema_migrations CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, conn, log); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		JWTSecret:      []byte(testSecret),
		TokenTTL:       time.Hour,
		AuthRatePerMin: 1000, // alto, salvo en el test del límite
		AuthBurst:      1000,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	h, err := server.New(cfg, conn, log)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, conn
}

// client simula un navegador: guarda las cookies entre peticiones.
type client struct {
	t    *testing.T
	base string
	hc   *http.Client
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, hc: &http.Client{Jar: jar}}
}

type resp struct {
	status int
	body   string
	header http.Header
}

func (r resp) json(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal([]byte(r.body), dst); err != nil {
		t.Fatalf("JSON inválido %q: %v", r.body, err)
	}
}

// do envía una petición con las cabeceras que enviaría el frontend.
func (c *client) do(method, path string, body any) resp {
	return c.doWith(method, path, body, map[string]string{
		"X-Requested-With": "plata-clara",
		"Content-Type":     "application/json",
	})
}

func (c *client) doWith(method, path string, body any, headers map[string]string) resp {
	c.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return resp{status: res.StatusCode, body: string(data), header: res.Header}
}

func (c *client) register(email string) resp {
	return c.do("POST", "/api/auth/register", map[string]string{"email": email, "password": "clave-segura-123"})
}

func (c *client) addExpense(amount int64, cat, desc, date string) resp {
	return c.do("POST", "/api/expenses", map[string]any{
		"amount": amount, "category": cat, "description": desc, "spentOn": date,
	})
}

type expenseResp struct {
	ID          string `json:"id"`
	Amount      int64  `json:"amount"`
	Category    string `json:"category"`
	Description string `json:"description"`
	SpentOn     string `json:"spentOn"`
}

func TestAuthFlow(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)

	// Sin sesión → 401
	if r := c.do("GET", "/api/me", nil); r.status != 401 {
		t.Fatalf("/api/me sin sesión: %d", r.status)
	}

	// Registro OK + cookie segura
	r := c.register("Ana@Example.com")
	if r.status != 201 {
		t.Fatalf("registro: %d %s", r.status, r.body)
	}
	setCookie := r.header.Get("Set-Cookie")
	for _, flag := range []string{"HttpOnly", "SameSite=Strict", "session="} {
		if !strings.Contains(setCookie, flag) {
			t.Errorf("la cookie debe incluir %q: %s", flag, setCookie)
		}
	}
	if strings.Contains(r.body, "password") {
		t.Error("la respuesta no debe incluir datos de contraseña")
	}

	// /me funciona con la cookie, y el correo quedó normalizado
	r = c.do("GET", "/api/me", nil)
	var me struct{ Email string }
	r.json(t, &me)
	if r.status != 200 || me.Email != "ana@example.com" {
		t.Fatalf("/api/me: %d %s", r.status, r.body)
	}

	// Correo duplicado (aunque cambie la capitalización) → 409
	if r := newClient(t, ts.URL).register("ANA@example.com"); r.status != 409 {
		t.Fatalf("duplicado: %d", r.status)
	}

	// Contraseña débil / correo inválido → 422
	r = newClient(t, ts.URL).do("POST", "/api/auth/register", map[string]string{"email": "b@b.com", "password": "corta"})
	if r.status != 422 {
		t.Fatalf("contraseña corta: %d", r.status)
	}
	r = newClient(t, ts.URL).do("POST", "/api/auth/register", map[string]string{"email": "no-es-correo", "password": "clave-segura-123"})
	if r.status != 422 {
		t.Fatalf("correo inválido: %d", r.status)
	}

	// Logout borra la sesión
	if r := c.do("POST", "/api/auth/logout", nil); r.status != 204 {
		t.Fatalf("logout: %d", r.status)
	}
	if r := c.do("GET", "/api/me", nil); r.status != 401 {
		t.Fatalf("/api/me tras logout: %d", r.status)
	}

	// Login correcto
	r = c.do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "clave-segura-123"})
	if r.status != 200 {
		t.Fatalf("login: %d %s", r.status, r.body)
	}
}

func TestLoginErrorsDoNotRevealWhichAccountsExist(t *testing.T) {
	ts, _ := newServer(t, nil)
	newClient(t, ts.URL).register("real@example.com")

	c := newClient(t, ts.URL)
	wrongPass := c.do("POST", "/api/auth/login", map[string]string{"email": "real@example.com", "password": "clave-equivocada!"})
	noUser := c.do("POST", "/api/auth/login", map[string]string{"email": "fantasma@example.com", "password": "clave-equivocada!"})
	badEmail := c.do("POST", "/api/auth/login", map[string]string{"email": "###", "password": "x"})

	for name, r := range map[string]resp{"contraseña errónea": wrongPass, "correo inexistente": noUser, "correo malformado": badEmail} {
		if r.status != 401 {
			t.Errorf("%s: esperaba 401, obtuve %d", name, r.status)
		}
	}
	if wrongPass.body != noUser.body || noUser.body != badEmail.body {
		t.Errorf("los mensajes deben ser idénticos:\n%q\n%q\n%q", wrongPass.body, noUser.body, badEmail.body)
	}
}

func TestProtectedRoutesRequireSession(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)
	id := "00000000-0000-0000-0000-000000000000"
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/me"},
		{"GET", "/api/expenses"},
		{"GET", "/api/expenses/summary"},
		{"POST", "/api/expenses"},
		{"PUT", "/api/expenses/" + id},
		{"DELETE", "/api/expenses/" + id},
	} {
		if r := c.do(tc.method, tc.path, nil); r.status != 401 {
			t.Errorf("%s %s sin sesión: esperaba 401, obtuve %d", tc.method, tc.path, r.status)
		}
	}
}

func TestTamperedCookieIsRejected(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)

	for _, bad := range []string{"basura", "a.b.c", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJhZG1pbiJ9."} {
		r := c.doWith("GET", "/api/me", nil, map[string]string{"Cookie": "session=" + bad})
		if r.status != 401 {
			t.Errorf("cookie %q: esperaba 401, obtuve %d", bad, r.status)
		}
	}
}

func TestExpenseCRUD(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)
	c.register("ana@example.com")

	r := c.addExpense(25000, "comida", "Almuerzo", "2026-10-11")
	if r.status != 201 {
		t.Fatalf("crear: %d %s", r.status, r.body)
	}
	var created expenseResp
	r.json(t, &created)
	c.addExpense(80000, "transporte", "Gasolina", "2026-10-05")
	c.addExpense(15000, "comida", "Cena", "2026-10-12")
	c.addExpense(99999, "ocio", "Otro mes", "2026-09-30") // no debe aparecer en octubre

	// Listar octubre
	var list struct{ Expenses []expenseResp }
	r = c.do("GET", "/api/expenses?month=2026-10", nil)
	r.json(t, &list)
	if r.status != 200 || len(list.Expenses) != 3 {
		t.Fatalf("listar: %d, %d gastos (esperaba 3): %s", r.status, len(list.Expenses), r.body)
	}
	if list.Expenses[0].SpentOn != "2026-10-12" {
		t.Errorf("deberían venir del más reciente al más antiguo: %+v", list.Expenses)
	}

	// Resumen
	var sum struct {
		Total      int64
		ByCategory []struct {
			Category string
			Total    int64
			Count    int64
		}
	}
	r = c.do("GET", "/api/expenses/summary?month=2026-10", nil)
	r.json(t, &sum)
	if sum.Total != 120000 || len(sum.ByCategory) != 2 || sum.ByCategory[0].Category != "transporte" || sum.ByCategory[1].Total != 40000 {
		t.Fatalf("resumen incorrecto: %s", r.body)
	}

	// Mes sin gastos → lista vacía "[]" (no null)
	r = c.do("GET", "/api/expenses?month=2020-01", nil)
	if !strings.Contains(r.body, `"expenses":[]`) {
		t.Errorf("esperaba lista vacía, obtuve %s", r.body)
	}

	// Actualizar
	r = c.do("PUT", "/api/expenses/"+created.ID, map[string]any{
		"amount": 30000, "category": "comida", "description": "Almuerzo grande", "spentOn": "2026-10-11",
	})
	var updated expenseResp
	r.json(t, &updated)
	if r.status != 200 || updated.Amount != 30000 || updated.Description != "Almuerzo grande" {
		t.Fatalf("actualizar: %d %s", r.status, r.body)
	}

	// Borrar
	if r := c.do("DELETE", "/api/expenses/"+created.ID, nil); r.status != 204 {
		t.Fatalf("borrar: %d", r.status)
	}
	if r := c.do("DELETE", "/api/expenses/"+created.ID, nil); r.status != 404 {
		t.Fatalf("borrar de nuevo debería dar 404, dio %d", r.status)
	}
}

// El test MÁS importante: un usuario no puede tocar los datos de otro (IDOR).
func TestUsersCannotAccessEachOthersExpenses(t *testing.T) {
	ts, _ := newServer(t, nil)
	alice := newClient(t, ts.URL)
	bob := newClient(t, ts.URL)
	alice.register("alice@example.com")
	bob.register("bob@example.com")

	var secret expenseResp
	alice.addExpense(500000, "salud", "Cita médica privada", "2026-10-10").json(t, &secret)

	// Bob no la ve en su lista ni en su resumen
	r := bob.do("GET", "/api/expenses?month=2026-10", nil)
	if strings.Contains(r.body, secret.ID) || strings.Contains(r.body, "Cita médica") {
		t.Fatalf("¡Bob ve datos de Alice! %s", r.body)
	}
	r = bob.do("GET", "/api/expenses/summary?month=2026-10", nil)
	if !strings.Contains(r.body, `"total":0`) {
		t.Fatalf("el resumen de Bob debería estar en 0: %s", r.body)
	}

	// Bob no puede editarla ni borrarla, aun sabiendo su id → 404
	r = bob.do("PUT", "/api/expenses/"+secret.ID, map[string]any{
		"amount": 1, "category": "otros", "description": "hackeado", "spentOn": "2026-10-10",
	})
	if r.status != 404 {
		t.Fatalf("Bob editando gasto ajeno: esperaba 404, obtuve %d", r.status)
	}
	if r := bob.do("DELETE", "/api/expenses/"+secret.ID, nil); r.status != 404 {
		t.Fatalf("Bob borrando gasto ajeno: esperaba 404, obtuve %d", r.status)
	}

	// El gasto de Alice sigue intacto
	r = alice.do("GET", "/api/expenses?month=2026-10", nil)
	if !strings.Contains(r.body, "Cita médica privada") || !strings.Contains(r.body, `"amount":500000`) {
		t.Fatalf("el gasto de Alice cambió: %s", r.body)
	}
}

func TestClientCannotSetOwner(t *testing.T) {
	ts, conn := newServer(t, nil)
	alice := newClient(t, ts.URL)
	bob := newClient(t, ts.URL)
	alice.register("alice@example.com")
	bob.register("bob@example.com")

	var bobID string
	_ = conn.QueryRow(`SELECT id FROM users WHERE email='bob@example.com'`).Scan(&bobID)

	// Alice intenta crear un gasto "a nombre de" Bob mandando user_id: el campo no existe → 400.
	r := alice.do("POST", "/api/expenses", map[string]any{
		"amount": 1000, "category": "otros", "description": "x", "spentOn": "2026-10-10", "user_id": bobID,
	})
	if r.status != 400 {
		t.Fatalf("un campo desconocido debe rechazarse: %d %s", r.status, r.body)
	}
	var n int
	_ = conn.QueryRow(`SELECT count(*) FROM expenses WHERE user_id=$1`, bobID).Scan(&n)
	if n != 0 {
		t.Fatal("no debió crearse ningún gasto para Bob")
	}
}

func TestSQLInjectionIsHarmless(t *testing.T) {
	ts, conn := newServer(t, nil)
	c := newClient(t, ts.URL)
	c.register("ana@example.com")

	// Se guarda como TEXTO literal; no se ejecuta.
	evil := `'); DROP TABLE expenses; --`
	r := c.addExpense(1000, "otros", evil, "2026-10-10")
	if r.status != 201 {
		t.Fatalf("crear: %d %s", r.status, r.body)
	}
	var list struct{ Expenses []expenseResp }
	c.do("GET", "/api/expenses?month=2026-10", nil).json(t, &list)
	if len(list.Expenses) != 1 || list.Expenses[0].Description != evil {
		t.Fatalf("la descripción debe guardarse tal cual: %+v", list.Expenses)
	}
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM expenses`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("la tabla expenses debe seguir existiendo: n=%d err=%v", n, err)
	}

	// Intentos en el parámetro de mes y en el id → rechazados sin tocar la BD.
	if r := c.do("GET", "/api/expenses?month="+"2026-10'%20OR%20'1'='1", nil); r.status != 400 {
		t.Errorf("mes malicioso: esperaba 400, obtuve %d", r.status)
	}
	if r := c.do("DELETE", "/api/expenses/1%27%20OR%20%271%27=%271", nil); r.status != 404 {
		t.Errorf("id malicioso: esperaba 404, obtuve %d", r.status)
	}

	// Login con inyección clásica → simplemente falla.
	r = newClient(t, ts.URL).do("POST", "/api/auth/login", map[string]string{
		"email": "ana@example.com' OR '1'='1", "password": "' OR '1'='1",
	})
	if r.status != 401 {
		t.Errorf("login con inyección: esperaba 401, obtuve %d", r.status)
	}
}

func TestInputValidationAndStrictBody(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)
	c.register("ana@example.com")

	bad := []map[string]any{
		{"amount": -100, "category": "comida", "description": "", "spentOn": "2026-10-10"},
		{"amount": 100, "category": "inventada", "description": "", "spentOn": "2026-10-10"},
		{"amount": 100, "category": "comida", "description": "", "spentOn": "ayer"},
	}
	for i, body := range bad {
		if r := c.do("POST", "/api/expenses", body); r.status != 422 {
			t.Errorf("caso %d: esperaba 422, obtuve %d (%s)", i, r.status, r.body)
		}
	}

	// Decimal donde se espera entero, JSON roto y contenido extra → 400.
	for name, raw := range map[string]string{
		"decimal":         `{"amount": 10.5, "category":"comida","description":"","spentOn":"2026-10-10"}`,
		"json roto":       `{"amount": `,
		"contenido extra": `{"amount": 10, "category":"comida","description":"","spentOn":"2026-10-10"} {"x":1}`,
	} {
		if r := c.do("POST", "/api/expenses", raw); r.status != 400 {
			t.Errorf("%s: esperaba 400, obtuve %d", name, r.status)
		}
	}

	// Content-Type incorrecto → 400.
	r := c.doWith("POST", "/api/expenses", `{"amount":1}`, map[string]string{
		"X-Requested-With": "plata-clara", "Content-Type": "text/plain",
	})
	if r.status != 400 {
		t.Errorf("content-type text/plain: esperaba 400, obtuve %d", r.status)
	}

	// Cuerpo gigante (>64 KB) → rechazado.
	huge := `{"amount":1,"category":"otros","description":"` + strings.Repeat("a", 100_000) + `","spentOn":"2026-10-10"}`
	if r := c.do("POST", "/api/expenses", huge); r.status != 400 {
		t.Errorf("cuerpo gigante: esperaba 400, obtuve %d", r.status)
	}
}

func TestCSRFHeaderRequired(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)
	c.register("ana@example.com")

	// Simula una petición "de otro sitio": trae la cookie pero no la cabecera personalizada.
	r := c.doWith("POST", "/api/expenses",
		map[string]any{"amount": 1000, "category": "otros", "description": "", "spentOn": "2026-10-10"},
		map[string]string{"Content-Type": "application/json"})
	if r.status != 403 {
		t.Fatalf("sin X-Requested-With debería ser 403, fue %d", r.status)
	}
}

func TestLoginRateLimit(t *testing.T) {
	ts, _ := newServer(t, func(c *config.Config) {
		c.AuthRatePerMin = 1
		c.AuthBurst = 3
	})
	c := newClient(t, ts.URL)
	body := map[string]string{"email": "x@example.com", "password": "clave-equivocada"}

	for i := 1; i <= 3; i++ {
		if r := c.do("POST", "/api/auth/login", body); r.status != 401 {
			t.Fatalf("intento %d: esperaba 401, obtuve %d", i, r.status)
		}
	}
	r := c.do("POST", "/api/auth/login", body)
	if r.status != 429 {
		t.Fatalf("el 4.º intento seguido debe dar 429, dio %d", r.status)
	}
	if r.header.Get("Retry-After") == "" {
		t.Error("falta Retry-After")
	}
}

func TestSecurityHeadersAndErrorShapes(t *testing.T) {
	ts, _ := newServer(t, nil)
	c := newClient(t, ts.URL)

	r := c.do("GET", "/healthz", nil)
	if r.status != 200 || r.header.Get("X-Content-Type-Options") != "nosniff" || r.header.Get("Content-Security-Policy") == "" {
		t.Fatalf("healthz o cabeceras: %d %v", r.status, r.header)
	}
	// Una ruta /api inexistente responde JSON 404.
	r = c.do("GET", "/api/no-existe", nil)
	if r.status != 404 || !strings.Contains(r.header.Get("Content-Type"), "application/json") {
		t.Fatalf("404 de api: %d %v", r.status, r.header)
	}
	// Un método no permitido da 405.
	if r := c.do("PATCH", "/api/me", nil); r.status != 405 && r.status != 404 {
		t.Fatalf("método no permitido: %d", r.status)
	}
}
