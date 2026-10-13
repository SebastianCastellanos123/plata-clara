import { useCallback, useEffect, useMemo, useState } from "react";
import { api, ApiError } from "../api";
import { monthLabel, monthOf, shiftMonth } from "../format";
import type { Category, Expense, Summary, User } from "../types";
import ExpenseForm from "./ExpenseForm";
import ExpenseList from "./ExpenseList";
import SpendBar from "./SpendBar";

interface Props {
  user: User;
  onLogout: () => void;
}

export default function Dashboard({ user, onLogout }: Props) {
  const [month, setMonth] = useState(() => monthOf(new Date()));
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<Expense | null>(null);
  const [filter, setFilter] = useState<Category | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  // Carga la lista y el resumen del mes. El flag `cancelled` evita que una
  // respuesta lenta de un mes anterior pise a la del mes actual.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    Promise.all([api.listExpenses(month), api.summary(month)])
      .then(([list, sum]) => {
        if (cancelled) return;
        setExpenses(list);
        setSummary(sum);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof ApiError ? err.message : "No pudimos cargar tus gastos");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [month, reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  function changeMonth(delta: number) {
    setMonth((m) => shiftMonth(m, delta));
    setEditing(null);
    setFilter(null);
  }

  function handleSaved(saved: Expense) {
    setEditing(null);
    const savedMonth = saved.spentOn.slice(0, 7);
    // Si el gasto cae en otro mes, saltamos a ese mes para que lo veas.
    if (savedMonth !== month) setMonth(savedMonth);
    else reload();
  }

  async function handleDelete(e: Expense) {
    try {
      await api.deleteExpense(e.id);
      if (editing?.id === e.id) setEditing(null);
      reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No pudimos borrar el gasto");
    }
  }

  const visible = useMemo(() => (filter ? expenses.filter((e) => e.category === filter) : expenses), [expenses, filter]);
  const isCurrentMonth = month === monthOf(new Date());

  return (
    <div className="app">
      <header className="topbar">
        <p className="brand">Plata Clara</p>
        <div className="topbar-user">
          <span className="user-email">{user.email}</span>
          <button className="btn small" onClick={onLogout}>
            Salir
          </button>
        </div>
      </header>

      <main className="wrap">
        <section className="hero" aria-label="Resumen del mes">
          <div className="month-nav">
            <button className="icon-btn" onClick={() => changeMonth(-1)} aria-label="Mes anterior">
              ‹
            </button>
            <h1 aria-live="polite">{monthLabel(month)}</h1>
            <button className="icon-btn" onClick={() => changeMonth(1)} aria-label="Mes siguiente" disabled={isCurrentMonth}>
              ›
            </button>
          </div>

          <SpendBar summary={summary} month={month} active={filter} onSelect={setFilter} />
        </section>

        {error && (
          <p className="banner" role="alert">
            {error}{" "}
            <button className="link" onClick={reload}>
              Reintentar
            </button>
          </p>
        )}

        <div className="grid">
          <aside className="grid-form">
            <ExpenseForm month={month} editing={editing} onSaved={handleSaved} onCancelEdit={() => setEditing(null)} />
          </aside>

          <section className="grid-list" aria-label="Gastos del mes">
            {filter && (
              <p className="filter-note">
                Mostrando solo esta categoría.{" "}
                <button className="link" onClick={() => setFilter(null)}>
                  Ver todas
                </button>
              </p>
            )}
            <ExpenseList
              expenses={visible}
              loading={loading}
              editingId={editing?.id ?? null}
              onEdit={setEditing}
              onDelete={handleDelete}
            />
          </section>
        </div>
      </main>
    </div>
  );
}
