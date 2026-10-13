import { describe, expect, it } from "vitest";
import { defaultDateFor, digitsOnly, formatCOP, monthLabel, parseISODate, percent, shiftMonth, toISODate } from "./format";

describe("formatCOP", () => {
  it("usa puntos de miles y sin decimales", () => {
    expect(formatCOP(25000)).toBe("$ 25.000");
    expect(formatCOP(1250000)).toBe("$ 1.250.000");
  });
});

describe("digitsOnly", () => {
  it("quita todo lo que no sea dígito", () => {
    expect(digitsOnly("$ 12.500,50")).toBe("1250050");
    expect(digitsOnly("abc")).toBe("");
  });
  it("limita la longitud", () => {
    expect(digitsOnly("9".repeat(30))).toHaveLength(13);
  });
});

describe("fechas", () => {
  it("shiftMonth cruza años", () => {
    expect(shiftMonth("2026-12", 1)).toBe("2027-01");
    expect(shiftMonth("2026-01", -1)).toBe("2025-12");
    expect(shiftMonth("2026-10", 0)).toBe("2026-10");
  });
  it("parseISODate no se corre de día por la zona horaria", () => {
    const d = parseISODate("2026-10-11");
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 9, 11]);
    expect(toISODate(d)).toBe("2026-10-11");
  });
  it("monthLabel en español", () => {
    expect(monthLabel("2026-10")).toBe("octubre 2026");
  });
  it("defaultDateFor usa hoy en el mes actual y el día 1 en otros", () => {
    const now = new Date(2026, 9, 11);
    expect(defaultDateFor("2026-10", now)).toBe("2026-10-11");
    expect(defaultDateFor("2026-08", now)).toBe("2026-08-01");
  });
});

describe("percent", () => {
  it("calcula y evita dividir por cero", () => {
    expect(percent(25, 100)).toBe(25);
    expect(percent(1, 0)).toBe(0);
  });
});
