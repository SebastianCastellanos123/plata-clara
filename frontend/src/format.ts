import type { Category } from "./types";

export const CATEGORIES: { id: Category; label: string }[] = [
  { id: "comida", label: "Comida" },
  { id: "transporte", label: "Transporte" },
  { id: "vivienda", label: "Vivienda" },
  { id: "servicios", label: "Servicios" },
  { id: "salud", label: "Salud" },
  { id: "ocio", label: "Ocio" },
  { id: "educacion", label: "Educación" },
  { id: "otros", label: "Otros" },
];

export function categoryLabel(id: Category): string {
  return CATEGORIES.find((c) => c.id === id)?.label ?? id;
}

const cop = new Intl.NumberFormat("es-CO", {
  style: "currency",
  currency: "COP",
  maximumFractionDigits: 0,
});

/** 25000 → "$ 25.000" */
export function formatCOP(n: number): string {
  return cop.format(n).replace(/\u00a0/g, " ");
}

/** Número con puntos de miles, sin símbolo: 1250000 → "1.250.000" */
export function formatThousands(n: number): string {
  return new Intl.NumberFormat("es-CO", { maximumFractionDigits: 0 }).format(n);
}

/** Deja solo dígitos (para el campo de monto). Límite de 13 cifras. */
export function digitsOnly(s: string): string {
  return s.replace(/\D/g, "").slice(0, 13);
}

// --- Fechas -----------------------------------------------------------------
// Trabajamos con strings AAAA-MM-DD y los convertimos a Date con componentes
// LOCALES. Si usáramos new Date("2026-10-11") JavaScript lo interpreta como UTC y,
// en Colombia (UTC-5), mostraría el día anterior.

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

export function toISODate(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function monthOf(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}

export function parseISODate(iso: string): Date {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
}

/** Suma (o resta) meses a un "AAAA-MM". */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split("-").map(Number);
  return monthOf(new Date(y, m - 1 + delta, 1));
}

export function monthLabel(month: string): string {
  const [y, m] = month.split("-").map(Number);
  const name = new Intl.DateTimeFormat("es-CO", { month: "long" }).format(new Date(y, m - 1, 1));
  return `${name} ${y}`;
}

export function dayLabel(iso: string): string {
  return new Intl.DateTimeFormat("es-CO", { weekday: "long", day: "numeric", month: "long" }).format(parseISODate(iso));
}

/** Fecha por defecto del formulario: hoy si estamos en el mes actual; si no, el día 1 de ese mes. */
export function defaultDateFor(month: string, now = new Date()): string {
  return monthOf(now) === month ? toISODate(now) : `${month}-01`;
}

export function percent(part: number, total: number): number {
  return total === 0 ? 0 : Math.round((part / total) * 100);
}
