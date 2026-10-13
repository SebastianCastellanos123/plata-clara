// Package httpx agrupa utilidades pequeñas para responder y leer JSON.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// maxBody limita el tamaño del cuerpo que aceptamos (64 KB). Sin límite, alguien
// podría enviar gigabytes y agotar la memoria del servidor.
const maxBody = 64 << 10

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

// WriteError responde con un mensaje legible. Nunca incluyas aquí detalles
// internos (errores SQL, rutas de archivos): le dan pistas a un atacante.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, errorBody{Error: msg})
}

// WriteFieldErrors responde 422 indicando qué campo falló y por qué.
func WriteFieldErrors(w http.ResponseWriter, fields map[string]string) {
	WriteJSON(w, http.StatusUnprocessableEntity, errorBody{
		Error:  "Revisa los datos enviados",
		Fields: fields,
	})
}

var ErrBadBody = errors.New("cuerpo de la solicitud inválido")

// DecodeJSON lee el cuerpo como JSON de forma estricta:
//   - exige Content-Type: application/json (un <form> de otro sitio no puede
//     enviar ese tipo sin pasar por CORS, lo que ayuda contra CSRF),
//   - limita el tamaño,
//   - rechaza campos desconocidos (evita "mass assignment"),
//   - rechaza contenido extra después del JSON.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return ErrBadBody
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrBadBody
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ErrBadBody
	}
	return nil
}
