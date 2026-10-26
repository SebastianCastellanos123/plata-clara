import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, ApiError } from "../api";
import { categoryLabel, formatCOP, monthLabel, monthOf, shiftMonth } from "../format";
import type { Category, Expense, Summary, User } from "../types";
import ExpenseForm from "./ExpenseForm";
import ExpenseList from "./ExpenseList";
import { ChevronLeft, ChevronRight, Plus } from "./Icons";
import SpendBar from "./SpendBar";

interface Props {
  user: User;
  onLogout: () => void;
}

interface Toast {
  id: number;
  text: string;
  undo?: () => Promise<void>;
}

const TOAST_MS = 6000;

export default function Dashboard({ user, onLogout }: Props) {
  const [month, setMonth] = useState(() => monthOf(new Date()));
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [previousTotal, setPreviousTotal] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<Expense | null>(null);
  const [filter, setFilter] = useState<Category | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [toast, setToast] = useState<Toast | null>(null);
  const [formInView, setFormInView] = useState(true);
  const toastSeq = useRef(0);

  // Carga la lista y el resumen del mes. El flag `cancelled` evita que una
  // respuesta lenta de un mes anterior pise a la del mes actual.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    Promise.all([
      api.listExpenses(month),
      api.summary(month),
      // El mes anterior es solo una referencia: si falla, no rompe la pantalla.
      api.summary(shiftMonth(month, -1)).catch(() => null),
    ])
      .then(([list, sum, prev]) => {
        if (cancelled) return;
        setExpenses(list);
        setSummary(sum);
        setPreviousTotal(prev?.total ?? null);
        // Si la categoría filtrada se quedó sin gastos, quitamos el filtro.
        setFilter((f) => (f && sum.byCategory.some((c) => c.category === f) ? f : null));
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

  // El aviso se va solo a los pocos segundos.
  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), TOAST_MS);
    return () => clearTimeout(t);
  }, [toast]);

  function notify(text: string, undo?: () => Promise<void>) {
    setToast({ id: ++toastSeq.current, text, undo });
  }

  // En pantallas angostas el formulario queda debajo de la lista: el botón
  // flotante "Anotar gasto" solo aparece mientras el formulario no se ve.
  useEffect(() => {
    const form = document.getElementById("expense-form");
    if (!form || typeof IntersectionObserver === "undefined") return;
    const io = new IntersectionObserver(([entry]) => setFormInView(entry.isIntersecting), { threshold: 0.15 });
    io.observe(form);
    return () => io.disconnect();
  }, []);

  function changeMonth(delta: number) {
    setMonth((m) => shiftMonth(m, delta));
    setEditing(null);
    setFilter(null);
  }

  function goToForm() {
    const form = document.getElementById("expense-form");
    if (!form) return;
    // Primero el foco y después el desplazamiento: enfocar cancela un scroll suave en curso.
    document.getElementById("amount")?.focus({ preventScroll: true });
    const r = form.getBoundingClientRect();
    if (r.top < 0 || r.bottom > window.innerHeight) {
      const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      form.scrollIntoView({ block: "start", behavior: reduce ? "auto" : "smooth" });
    }
  }

  function startEdit(e: Expense) {
    setEditing(e);
    // Esperamos a que el formulario se llene con el gasto antes de llevar el foco.
    requestAnimationFrame(goToForm);
  }

  function handleSaved(saved: Expense, wasEditing: boolean) {
    setEditing(null);
    notify(wasEditing ? "Cambios guardados" : `Anotado: ${formatCOP(saved.amount)} en ${categoryLabel(saved.category)}`);
    const savedMonth = saved.spentOn.slice(0, 7);
    // Si el gasto cae en otro mes, saltamos a ese mes para que lo veas.
    if (savedMonth !== month) setMonth(savedMonth);
    else reload();
  }

  async function handleDelete(e: Expense) {
    try {
      await api.deleteExpense(e.id);
      setEditing(null);
      reload();
      // Deshacer vuelve a crear el gasto con los mismos datos.
      notify(`Borrado: ${formatCOP(e.amount)} en ${categoryLabel(e.category)}`, async () => {
        const { amount, category, description, spentOn } = e;
        await api.createExpense({ amount, category, description, spentOn });
        notify("Gasto recuperado");
        reload();
      });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No pudimos borrar el gasto");
    }
  }

  async function undo() {
    const action = toast?.undo;
    setToast(null);
    if (!action) return;
    try {
      await action();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No pudimos recuperar el gasto");
    }
  }

  const visible = useMemo(() => (filter ? expenses.filter((e) => e.category === filter) : expenses), [expenses, filter]);
  const isCurrentMonth = month === monthOf(new Date());

  return (
    <div className="app">
      <div className="masthead">
        <header className="topbar">
          <p className="brand">Plata Clara</p>
          <div className="topbar-user">
            <span className="user-email">{user.email}</span>
            <button className="btn small on-ink" onClick={onLogout}>
              Salir
            </button>
          </div>
        </header>

        <section className="hero" aria-label="Resumen del mes">
          <div className="month-nav">
            <button className="icon-btn" onClick={() => changeMonth(-1)} aria-label="Mes anterior">
              <ChevronLeft />
            </button>
            <h1>{monthLabel(month)}</h1>
            <button
              className="icon-btn"
              onClick={() => changeMonth(1)}
              aria-label="Mes siguiente"
              title={isCurrentMonth ? "Ya estás en el mes actual" : undefined}
              disabled={isCurrentMonth}
            >
              <ChevronRight />
            </button>
          </div>

          <SpendBar
            summary={summary}
            month={month}
            previous={previousTotal === null ? null : { month: shiftMonth(month, -1), total: previousTotal }}
            active={filter}
            onSelect={setFilter}
          />
        </section>
      </div>

      <main className="wrap">
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
            <ExpenseForm
              month={month}
              editing={editing}
              preferredCategory={filter}
              onSaved={handleSaved}
              onCancelEdit={() => setEditing(null)}
              onDelete={handleDelete}
            />
          </aside>

          <section className="grid-list" aria-label="Gastos del mes">
            {filter && (
              <p className="filter-note">
                Solo <strong>{categoryLabel(filter)}</strong> ·{" "}
                <button className="link" onClick={() => setFilter(null)}>
                  Ver todas
                </button>
              </p>
            )}
            <ExpenseList expenses={visible} loading={loading} editingId={editing?.id ?? null} onEdit={startEdit} />
          </section>
        </div>
      </main>

      <div className="dock">
        <div className="toast-region" aria-live="polite">
          {toast && (
            <div className="toast" key={toast.id}>
              <span>{toast.text}</span>
              {toast.undo && (
                <button type="button" className="toast-action" onClick={undo}>
                  Deshacer
                </button>
              )}
            </div>
          )}
        </div>
        {!formInView && (
          <button type="button" className="fab" onClick={goToForm}>
            <Plus />
            Anotar gasto
          </button>
        )}
      </div>
    </div>
  );
}
