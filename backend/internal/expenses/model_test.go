package expenses

import (
	"strings"
	"testing"
)

func valid() Input {
	return Input{Amount: 25000, Category: "comida", Description: "Almuerzo", SpentOn: "2026-10-11"}
}

func TestValidateAcceptsGoodInput(t *testing.T) {
	in := valid()
	if errs := in.Validate(); errs != nil {
		t.Fatalf("no esperaba errores: %v", errs)
	}
}

func TestValidateNormalizes(t *testing.T) {
	in := valid()
	in.Category = "  COMIDA "
	in.Description = "  con espacios  "
	if errs := in.Validate(); errs != nil {
		t.Fatal(errs)
	}
	if in.Category != "comida" || in.Description != "con espacios" {
		t.Fatalf("no normalizó: %+v", in)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Input)
		field string
	}{
		{"monto cero", func(i *Input) { i.Amount = 0 }, "amount"},
		{"monto negativo", func(i *Input) { i.Amount = -5 }, "amount"},
		{"monto enorme", func(i *Input) { i.Amount = maxAmount + 1 }, "amount"},
		{"categoría inventada", func(i *Input) { i.Category = "hackeo" }, "category"},
		{"categoría vacía", func(i *Input) { i.Category = "" }, "category"},
		{"descripción larga", func(i *Input) { i.Description = strings.Repeat("a", 201) }, "description"},
		{"caracteres de control", func(i *Input) { i.Description = "hola\x00mundo" }, "description"},
		{"salto de línea", func(i *Input) { i.Description = "linea1\nlinea2" }, "description"},
		{"fecha mal formada", func(i *Input) { i.SpentOn = "11/10/2026" }, "spentOn"},
		{"fecha imposible", func(i *Input) { i.SpentOn = "2026-02-31" }, "spentOn"},
		{"fecha vacía", func(i *Input) { i.SpentOn = "" }, "spentOn"},
		{"año fuera de rango", func(i *Input) { i.SpentOn = "1850-01-01" }, "spentOn"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := valid()
			c.edit(&in)
			errs := in.Validate()
			if errs == nil || errs[c.field] == "" {
				t.Fatalf("esperaba error en %q, obtuve %v", c.field, errs)
			}
		})
	}
}

func TestValidateAllowsSpecialCharsInDescription(t *testing.T) {
	// Comillas, tildes y emojis son válidos: la defensa contra inyección está en
	// las consultas parametrizadas, no en prohibir caracteres.
	in := valid()
	in.Description = `Café "especial" ñandú 🍕 ' OR 1=1 --`
	if errs := in.Validate(); errs != nil {
		t.Fatalf("debería aceptarse: %v", errs)
	}
}

func TestMonthRange(t *testing.T) {
	from, to, ok := MonthRange("2026-12")
	if !ok || from != "2026-12-01" || to != "2027-01-01" {
		t.Fatalf("diciembre: %q %q %v", from, to, ok)
	}
	for _, bad := range []string{"", "2026", "2026-13", "abc", "2026-1-1", "2026-10'; DROP TABLE users;--"} {
		if _, _, ok := MonthRange(bad); ok {
			t.Errorf("%q debería ser inválido", bad)
		}
	}
}
