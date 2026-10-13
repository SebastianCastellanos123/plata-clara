// Package expenses contiene la lógica de gastos: modelo, validación,
// acceso a datos y handlers HTTP.
package expenses

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Categories es la lista cerrada de categorías permitidas (coincide con el
// CHECK de la base de datos). Una lista cerrada es más segura que texto libre.
var Categories = map[string]bool{
	"comida": true, "transporte": true, "vivienda": true, "servicios": true,
	"salud": true, "ocio": true, "educacion": true, "otros": true,
}

const (
	maxAmount      = 1_000_000_000_000 // 1 billón de pesos: un tope razonable
	maxDescription = 200
	dateLayout     = "2006-01-02"
)

// Expense es lo que enviamos al cliente.
type Expense struct {
	ID          string `json:"id"`
	Amount      int64  `json:"amount"`
	Category    string `json:"category"`
	Description string `json:"description"`
	SpentOn     string `json:"spentOn"` // YYYY-MM-DD
}

// Input es lo que RECIBIMOS del cliente. Nunca confiamos en él: se valida todo.
// Fíjate que no tiene campo user_id: el dueño sale de la sesión, jamás del cliente.
type Input struct {
	Amount      int64  `json:"amount"`
	Category    string `json:"category"`
	Description string `json:"description"`
	SpentOn     string `json:"spentOn"`
}

// Validate normaliza el input y devuelve un mapa campo→mensaje si hay errores.
func (in *Input) Validate() map[string]string {
	errs := map[string]string{}

	if in.Amount <= 0 {
		errs["amount"] = "El monto debe ser mayor a 0"
	} else if in.Amount > maxAmount {
		errs["amount"] = "El monto es demasiado grande"
	}

	in.Category = strings.ToLower(strings.TrimSpace(in.Category))
	if !Categories[in.Category] {
		errs["category"] = "Elige una categoría válida"
	}

	in.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(in.Description) > maxDescription {
		errs["description"] = "La descripción no puede pasar de 200 caracteres"
	} else if hasControlChars(in.Description) {
		errs["description"] = "La descripción tiene caracteres no permitidos"
	}

	t, err := time.Parse(dateLayout, in.SpentOn)
	if err != nil || t.Year() < 2000 || t.Year() > 2100 {
		errs["spentOn"] = "La fecha debe tener el formato AAAA-MM-DD"
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// MonthRange convierte "2026-10" en [2026-10-01, 2026-11-01) como strings de fecha.
func MonthRange(month string) (from, to string, ok bool) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", false
	}
	return t.Format(dateLayout), t.AddDate(0, 1, 0).Format(dateLayout), true
}
