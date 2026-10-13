package expenses

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/auth"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/httpx"
)

type Handler struct {
	store *Store
	log   *slog.Logger
}

func NewHandler(s *Store, log *slog.Logger) *Handler { return &Handler{store: s, log: log} }

// monthParam lee ?month=2026-10 (si no viene, usa el mes actual).
func monthParam(r *http.Request) (from, to string, ok bool) {
	m := r.URL.Query().Get("month")
	if m == "" {
		m = time.Now().UTC().Format("2006-01")
	}
	return MonthRange(m)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	from, to, ok := monthParam(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "El mes debe tener el formato AAAA-MM")
		return
	}
	// El id del usuario sale de la SESIÓN (auth.UserID), no de la petición.
	items, err := h.store.List(r.Context(), auth.UserID(r.Context()), from, to)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"expenses": items})
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	from, to, ok := monthParam(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "El mes debe tener el formato AAAA-MM")
		return
	}
	sum, err := h.store.Summary(r.Context(), auth.UserID(r.Context()), from, to)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sum)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readInput(w, r)
	if !ok {
		return
	}
	e, err := h.store.Create(r.Context(), auth.UserID(r.Context()), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, e)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readInput(w, r)
	if !ok {
		return
	}
	e, err := h.store.Update(r.Context(), auth.UserID(r.Context()), r.PathValue("id"), in)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "No encontramos ese gasto")
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, e)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	err := h.store.Delete(r.Context(), auth.UserID(r.Context()), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "No encontramos ese gasto")
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readInput decodifica y valida; si algo falla, ya respondió y devuelve ok=false.
func (h *Handler) readInput(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return in, false
	}
	if errs := in.Validate(); errs != nil {
		httpx.WriteFieldErrors(w, errs)
		return in, false
	}
	return in, true
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("error interno", "err", err, "path", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "Ocurrió un error. Intenta de nuevo")
}
