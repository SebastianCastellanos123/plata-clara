# Plata Clara

Gestor de gastos personales, construido de punta a punta con la seguridad como criterio de diseño: autenticación con sesiones en cookie `HttpOnly`, control de acceso por usuario, validación estricta, límite de intentos de login y una suite de pruebas que ataca la propia API.

<p align="center">
  <img src="docs/screenshots/dashboard.png" alt="Panel principal de Plata Clara con el total del mes y la barra de categorías" width="780">
</p>

<p align="center">
  <img src="docs/screenshots/login.png" alt="Pantalla de acceso" width="380">
  <img src="docs/screenshots/mobile.png" alt="Vista móvil" width="190">
</p>

## Qué hace

- Registro e inicio de sesión.
- Crear, editar y borrar gastos (monto en COP, categoría, nota, fecha).
- Resumen del mes: total y barra que muestra cómo se reparte por categoría. Al hacer clic en una categoría filtra la lista.
- Navegación entre meses.
- Cada usuario ve **únicamente** sus propios datos.

## Stack

| Capa | Tecnología |
|---|---|
| Frontend | React 18 + TypeScript + Vite |
| Backend | Go (librería estándar `net/http`), `lib/pq`, `golang-jwt`, `bcrypt` |
| Base de datos | PostgreSQL 16, migraciones SQL embebidas |
| Infraestructura | Docker (imagen final *distroless*, usuario sin privilegios), Docker Compose |
| CI | GitHub Actions: `go vet`, tests con Postgres real, `govulncheck`, `npm audit`, build de Docker |

## Seguridad

El detalle completo, con el mapeo al **OWASP Top 10** y las limitaciones conocidas, está en [`SECURITY.md`](SECURITY.md). Resumen:

- Contraseñas con **bcrypt** (costo 12), nunca en texto plano.
- Sesión en **JWT firmado (HS256)** dentro de una cookie `HttpOnly` + `Secure` + `SameSite=Strict`. El algoritmo está fijado, así que `alg: none` se rechaza.
- **Control de acceso**: toda consulta filtra por `user_id`. Un gasto ajeno responde `404`.
- **Consultas parametrizadas** en todo el acceso a datos (sin inyección SQL).
- **Rate limiting** en login y registro; el login responde igual y tarda igual exista o no la cuenta.
- **Defensa CSRF** doble: `SameSite=Strict` + cabecera personalizada obligatoria.
- Cabeceras de seguridad (**CSP estricta**, `X-Frame-Options`, `nosniff`, HSTS en producción).
- Validación estricta de entrada: JSON con campos conocidos, tamaño máximo, listas cerradas, restricciones `CHECK` también en la base de datos.
- Dependencias vigiladas con `govulncheck`, `npm audit` y Dependabot.

## Probarlo en 2 minutos (Docker)

Requisitos: Docker.

```bash
git clone https://github.com/SebastianCastellanos123/plata-clara.git
cd plata-clara

cp .env.example .env
# Edita .env: cambia POSTGRES_PASSWORD y genera JWT_SECRET con:
#   openssl rand -base64 48

docker compose up --build
```

Abre http://localhost:8080.

## Desarrollo local

Requisitos: Go 1.22+, Node 22+, Docker (para la base de datos).

```bash
# 1) Base de datos
docker compose up -d db

# 2) Backend (http://localhost:8080)
cd backend
go mod tidy        # la primera vez: descarga dependencias y crea go.sum
export DATABASE_URL="postgres://plata:TU_CLAVE@localhost:5432/plata_clara?sslmode=disable"
export JWT_SECRET="$(openssl rand -base64 48)"
go run ./cmd/api

# 3) Frontend con recarga en vivo (http://localhost:5173), en otra terminal
cd frontend
npm install
npm run dev
```

En desarrollo, Vite reenvía `/api` al backend, así el navegador ve un solo origen y no se necesita CORS.

## Pruebas

```bash
# Unitarias + integración (las de integración necesitan una BD de PRUEBAS)
docker compose exec db createdb -U plata plata_clara_test

cd backend
TEST_DATABASE_URL="postgres://plata:TU_CLAVE@localhost:5432/plata_clara_test?sslmode=disable" \
  go test ./... -race -count=1

cd ../frontend
npm test
```

> Los tests de integración **borran las tablas** de la base indicada en `TEST_DATABASE_URL`. Usa siempre una base de pruebas. Si la variable no está definida, esos tests se omiten.

Lo que cubren las pruebas de integración (contra PostgreSQL real): flujo de registro/login/logout, que los errores de login no revelen qué cuentas existen, rutas protegidas sin sesión, cookies manipuladas, **un usuario intentando leer/editar/borrar gastos de otro**, inyección SQL, validación y cuerpos malformados, protección CSRF y rate limiting.

## API

Todas las rutas viven bajo `/api`. Las que modifican datos exigen `Content-Type: application/json` y la cabecera `X-Requested-With: plata-clara`.

| Método | Ruta | Auth | Descripción |
|---|---|---|---|
| POST | `/api/auth/register` | no | Crea la cuenta e inicia sesión |
| POST | `/api/auth/login` | no | Inicia sesión |
| POST | `/api/auth/logout` | no | Cierra sesión |
| GET | `/api/me` | sí | Usuario actual |
| GET | `/api/expenses?month=AAAA-MM` | sí | Gastos del mes |
| GET | `/api/expenses/summary?month=AAAA-MM` | sí | Total y desglose por categoría |
| POST | `/api/expenses` | sí | Crea un gasto |
| PUT | `/api/expenses/{id}` | sí | Actualiza un gasto propio |
| DELETE | `/api/expenses/{id}` | sí | Borra un gasto propio |
| GET | `/healthz` | no | Estado del servicio |

## Estructura

```
plata-clara/
├── backend/
│   ├── cmd/api/              # punto de entrada (main)
│   └── internal/
│       ├── auth/             # bcrypt, JWT, login/registro, RequireAuth
│       ├── expenses/         # modelo, validación, consultas SQL, handlers
│       ├── users/            # consultas de usuarios
│       ├── middleware/       # rate limit, cabeceras de seguridad, CSRF, logs
│       ├── httpx/            # lectura estricta de JSON y respuestas
│       ├── db/               # conexión y migraciones (SQL embebido)
│       ├── config/           # variables de entorno
│       └── server/           # rutas y ensamblado
├── frontend/src/             # React: componentes, cliente de API, utilidades
├── Dockerfile                # build multi-etapa, imagen final distroless
├── docker-compose.yml
└── .github/workflows/ci.yml
```

## Decisiones de diseño

- **Go con `net/http` y pocas dependencias** (solo 3). Menos código de terceros es menos superficie de ataque y menos cosas que actualizar. El enrutador de Go 1.22 ya soporta métodos y parámetros (`PUT /api/expenses/{id}`).
- **Una sola imagen y un solo origen.** El backend sirve el frontend compilado; la cookie `SameSite=Strict` funciona sin configurar CORS, y se despliega como un único contenedor más la base de datos.
- **El monto es un entero (pesos), no un decimal.** Los flotantes no representan bien el dinero.
- **Migraciones embebidas en el binario** con bloqueo (`advisory lock`) para que dos instancias no migren a la vez.
- **El dueño del gasto sale de la sesión, nunca del cuerpo de la petición.** Enviar un `user_id` falso da error `400`.

## Limitaciones conocidas

Están explicadas en [`SECURITY.md`](SECURITY.md): no hay revocación de sesión antes de que expire (8 horas), el límite de intentos vive en memoria de un solo proceso, no hay verificación de correo ni recuperación de contraseña, y el registro revela si un correo ya existe.

## Licencia

MIT
