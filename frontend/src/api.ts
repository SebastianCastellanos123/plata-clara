import type { Expense, ExpenseInput, Summary, User } from "./types";

// Error con el mensaje que mandó el backend y, si aplica, qué campo falló.
export class ApiError extends Error {
  status: number;
  fields?: Record<string, string>;
  constructor(status: number, message: string, fields?: Record<string, string>) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}

// Si el backend responde 401 en una ruta protegida, la sesión venció: avisamos a
// la app para que muestre el login.
let onUnauthorized: (() => void) | null = null;
export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    credentials: "same-origin", // envía la cookie de sesión (HttpOnly, no la vemos desde JS)
    headers: {
      "Content-Type": "application/json",
      // Cabecera personalizada: el backend la exige en POST/PUT/DELETE (defensa CSRF).
      "X-Requested-With": "plata-clara",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (res.status === 204) return undefined as T;

  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const isAuthRoute = path.startsWith("/auth/");
    if (res.status === 401 && !isAuthRoute) onUnauthorized?.();
    throw new ApiError(res.status, data?.error ?? "Ocurrió un error. Intenta de nuevo", data?.fields);
  }
  return data as T;
}

export const api = {
  me: () => request<User>("GET", "/me"),
  register: (email: string, password: string) => request<User>("POST", "/auth/register", { email, password }),
  login: (email: string, password: string) => request<User>("POST", "/auth/login", { email, password }),
  logout: () => request<void>("POST", "/auth/logout"),

  listExpenses: (month: string) =>
    request<{ expenses: Expense[] }>("GET", `/expenses?month=${encodeURIComponent(month)}`).then((r) => r.expenses),
  summary: (month: string) => request<Summary>("GET", `/expenses/summary?month=${encodeURIComponent(month)}`),
  createExpense: (input: ExpenseInput) => request<Expense>("POST", "/expenses", input),
  updateExpense: (id: string, input: ExpenseInput) => request<Expense>("PUT", `/expenses/${encodeURIComponent(id)}`, input),
  deleteExpense: (id: string) => request<void>("DELETE", `/expenses/${encodeURIComponent(id)}`),
};
