# Seguridad

Este documento explica **qué se protegió, cómo y dónde se comprueba**. Cada medida apunta al código que la implementa y, cuando existe, al test que falla si alguien la rompe.

## Modelo de amenazas (resumen)

**Qué protegemos:** los gastos de cada usuario (datos financieros privados) y sus credenciales.

**Contra quién:**
1. Un usuario autenticado que intenta ver o modificar datos de otro.
2. Un atacante anónimo que intenta adivinar contraseñas o descubrir qué cuentas existen.
3. Un sitio malicioso que intenta aprovechar la sesión abierta del usuario (CSRF).
4. Contenido malicioso inyectado en la página (XSS) o en las consultas (SQLi).

**Fuera de alcance:** ataques a la infraestructura donde se despliegue (el servidor, el proveedor de nube) y a la máquina del usuario.

## Mapeo al OWASP Top 10 (2021)

| Riesgo | Qué hace la app | Dónde |
|---|---|---|
| **A01 Control de acceso roto** | Toda consulta de gastos incluye `AND user_id = $N`. El dueño sale de la sesión, nunca del cliente. Un recurso ajeno responde `404`, sin confirmar que existe. Un `user_id` en el cuerpo se rechaza (`400`). | `expenses/store.go`, `expenses/handlers.go` · Test: `TestUsersCannotAccessEachOthersExpenses`, `TestClientCannotSetOwner` |
| **A02 Fallas criptográficas** | Contraseñas con bcrypt (costo 12, máx. 72 bytes, sin truncar en silencio). JWT firmado con HS256 y clave de ≥32 caracteres; si no cumple, el servidor no arranca. Cookie `Secure` en producción + HSTS. | `auth/password.go`, `auth/token.go`, `config/config.go` |
| **A03 Inyección** | Consultas parametrizadas en todo el acceso a datos. El parámetro `month` se parsea como fecha; el `id` se valida como UUID antes de consultar. Frontend con React (escapa el contenido por defecto, sin `dangerouslySetInnerHTML`). | `expenses/store.go`, `expenses/model.go` · Test: `TestSQLInjectionIsHarmless` |
| **A04 Diseño inseguro** | Restricciones `CHECK` en la base de datos como segunda línea de defensa; listas cerradas para categorías; montos como enteros con tope; tamaño máximo de cuerpo (64 KB). | `db/migrations/0001_init.sql`, `httpx/httpx.go` · Test: `TestInputValidationAndStrictBody` |
| **A05 Configuración de seguridad incorrecta** | CSP estricta (`default-src 'self'`, sin scripts inline), `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy`, `Cache-Control: no-store` en `/api`. Timeouts de servidor contra *slowloris*. Contenedor *distroless*, usuario sin privilegios, sistema de archivos de solo lectura, sin capacidades Linux. Errores internos nunca se muestran al cliente. | `middleware/middleware.go`, `cmd/api/main.go`, `Dockerfile`, `docker-compose.yml` · Test: `TestSecurityHeaders` |
| **A06 Componentes vulnerables** | Solo 3 dependencias en Go. `govulncheck` y `npm audit` en cada push; Dependabot semanal. | `.github/workflows/ci.yml`, `.github/dependabot.yml` |
| **A07 Fallas de identificación y autenticación** | Límite de intentos por IP en login y registro (`429` + `Retry-After`). El login devuelve el **mismo mensaje y tarda lo mismo** exista o no el correo (bcrypt contra un hash falso), evitando enumeración de cuentas y *timing attacks*. Expiración de sesión a las 8 h. | `auth/handlers.go`, `middleware/ratelimit.go` · Test: `TestLoginErrorsDoNotRevealWhichAccountsExist`, `TestLoginRateLimit` |
| **A08 Fallas de integridad** | El algoritmo del JWT está fijado a HS256; se valida emisor y expiración. Se rechazan tokens `alg: none`, firmados con otra clave, con otro algoritmo o con otro emisor. Build de CI reproducible con `npm ci`. | `auth/token.go` · Tests: `TestTokenRejectsAlgNone`, `TestTokenRejectsWrongAlgorithm`, `TestTokenRejectsTampering` |
| **A09 Fallas de registro y monitoreo** | Logs estructurados (JSON) de cada petición con método, ruta, estado y duración. **Nunca** se registran cuerpos, contraseñas ni cookies. Los errores internos se registran con detalle en el servidor. | `middleware/middleware.go` |
| **A10 SSRF** | No aplica: el servidor no hace peticiones a URLs que el usuario controle. | — |

## Sesiones y CSRF

La sesión vive en una cookie con tres banderas:

- `HttpOnly`: JavaScript no puede leerla, así que un XSS no puede robar la sesión (se comprobó en un navegador real que `document.cookie` queda vacío con la sesión iniciada).
- `Secure`: solo viaja por HTTPS (activada con `APP_ENV=production`).
- `SameSite=Strict`: el navegador no la envía en peticiones originadas en otros sitios.

Como **defensa adicional** contra CSRF, todo método que modifica datos exige la cabecera `X-Requested-With: plata-clara`. Un sitio ajeno no puede añadirla sin un permiso CORS previo, y la app nunca concede CORS. Además, el cuerpo debe ser `application/json`.

## Cómo se prueba

La suite de integración (`backend/internal/server/server_test.go`) corre contra PostgreSQL real y actúa como un atacante: manipula cookies, intenta leer gastos ajenos, inyecta SQL, manda cuerpos malformados y satura el login.

Para comprobar que las pruebas realmente detectan fallos, se hizo una **prueba de mutación manual**: se quitó el filtro `user_id` de la consulta `DELETE` y `TestUsersCannotAccessEachOthersExpenses` falló con `Bob borrando gasto ajeno: esperaba 404, obtuve 204`. Se restauró el código y la prueba volvió a pasar.

## Limitaciones conocidas

Se documentan a propósito: conocer los límites del diseño es parte de la seguridad.

1. **Las sesiones no se pueden revocar antes de expirar.** Un JWT sin estado es válido hasta que vence; `logout` solo borra la cookie del navegador. Mitigación: sesión de 8 horas. Mejora: lista de sesiones en base de datos.
2. **El límite de intentos vive en la memoria de un solo proceso.** Con varias instancias, cada una cuenta por separado; y limita por IP, no por cuenta, así que un ataque distribuido lo esquiva. Mejora: almacenamiento compartido (Redis) y límite también por cuenta.
3. **El registro revela si un correo ya existe** (`409`). Es el compromiso habitual entre usabilidad y enumeración; el límite de intentos lo frena. Mejora: flujo de registro con verificación por correo.
4. **No hay verificación de correo, recuperación de contraseña ni autenticación de dos factores.**
5. **Detrás de un proxy** (Render, Fly, nginx), la IP real llega en `X-Forwarded-For`. Por eso `TRUST_PROXY` está apagado por defecto: activarlo sin proxy permitiría falsificar la IP y esquivar el límite. Actívalo solo si realmente hay un proxy de confianza delante.
6. **No hay rotación de la clave JWT.** Cambiar `JWT_SECRET` cierra todas las sesiones.

## Si encuentras una vulnerabilidad

Es un proyecto de portafolio, pero si encuentras algo, abre un *issue* privado o escribe al autor antes de publicarlo.
