package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/httpx"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/users"
)

const cookieName = "session"

type ctxKey struct{}

// UserID devuelve el id del usuario autenticado puesto por RequireAuth.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

type Handler struct {
	users        *users.Store
	tokens       *TokenManager
	cookieSecure bool
	log          *slog.Logger
	// dummyHash se usa cuando el correo no existe, para que el login tarde lo
	// mismo exista o no la cuenta (ver Login).
	dummyHash string
}

func NewHandler(u *users.Store, t *TokenManager, cookieSecure bool, log *slog.Logger) (*Handler, error) {
	dummy, err := HashPassword("contraseña-falsa-para-igualar-tiempos")
	if err != nil {
		return nil, err
	}
	return &Handler{users: u, tokens: t, cookieSecure: cookieSecure, log: log, dummyHash: dummy}, nil
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Register crea la cuenta e inicia sesión de una vez.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := NormalizeEmail(in.Email)
	if err != nil {
		httpx.WriteFieldErrors(w, map[string]string{"email": err.Error()})
		return
	}
	if err := ValidatePassword(in.Password); err != nil {
		httpx.WriteFieldErrors(w, map[string]string{"password": err.Error()})
		return
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	u, err := h.users.Create(r.Context(), email, hash)
	if errors.Is(err, users.ErrEmailTaken) {
		httpx.WriteError(w, http.StatusConflict, "Ese correo ya está registrado")
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.startSession(w, u.ID); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, userView{ID: u.ID, Email: u.Email})
}

// Login valida credenciales.
//
// Dos detalles de seguridad:
//  1. El mensaje de error es SIEMPRE el mismo ("correo o contraseña incorrectos"),
//     sin decir cuál de los dos falló. Así no revelamos qué correos existen.
//  2. Si el correo no existe, igual calculamos un bcrypt contra un hash falso.
//     Sin esto, "correo inexistente" respondería mucho más rápido que
//     "contraseña errónea" y un atacante podría medir el tiempo para descubrir
//     qué cuentas existen (timing attack).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	const invalid = "Correo o contraseña incorrectos"

	email, err := NormalizeEmail(in.Email)
	if err != nil {
		CheckPassword(h.dummyHash, in.Password) // mismo costo de tiempo
		httpx.WriteError(w, http.StatusUnauthorized, invalid)
		return
	}

	u, err := h.users.ByEmail(r.Context(), email)
	hash := h.dummyHash
	if err == nil {
		hash = u.PasswordHash
	} else if !errors.Is(err, users.ErrNotFound) {
		h.fail(w, r, err)
		return
	}

	ok := CheckPassword(hash, in.Password)
	if err != nil || !ok {
		httpx.WriteError(w, http.StatusUnauthorized, invalid)
		return
	}
	if err := h.startSession(w, u.ID); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, userView{ID: u.ID, Email: u.Email})
}

// Logout borra la cookie. Limitación conocida: un JWT sin estado no se puede
// "revocar" antes de que expire; por eso la sesión dura solo 8 horas.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me devuelve el usuario de la sesión actual (el frontend lo usa al cargar).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.ByID(r.Context(), UserID(r.Context()))
	if errors.Is(err, users.ErrNotFound) {
		h.clearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "Sesión inválida")
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, userView{ID: u.ID, Email: u.Email})
}

// RequireAuth protege rutas: exige una cookie con un JWT válido y guarda el id
// del usuario en el contexto de la petición.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "Necesitas iniciar sesión")
			return
		}
		id, err := h.tokens.Parse(c.Value)
		if err != nil {
			h.clearCookie(w)
			httpx.WriteError(w, http.StatusUnauthorized, "Necesitas iniciar sesión")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

func (h *Handler) startSession(w http.ResponseWriter, userID string) error {
	token, exp, err := h.tokens.Issue(userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,                    // JavaScript NO puede leerla: un XSS no roba la sesión
		Secure:   h.cookieSecure,          // solo viaja por HTTPS
		SameSite: http.SameSiteStrictMode, // no se envía en peticiones desde otros sitios (anti-CSRF)
	})
	return nil
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

// fail registra el error real en el log del servidor, pero al cliente solo le
// decimos "error interno": los detalles podrían ayudar a un atacante.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("error interno", "err", err, "path", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "Ocurrió un error. Intenta de nuevo")
}
