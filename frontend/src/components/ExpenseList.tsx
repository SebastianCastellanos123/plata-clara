import { useMemo } from "react";
import { categoryLabel, dayLabel, formatCOP } from "../format";
import type { Expense } from "../types";
import { ChevronRight } from "./Icons";

interface Props {
  expenses: Expense[];
  loading: boolean;
  editingId: string | null;
  onEdit: (e: Expense) => void;
}

export default function ExpenseList({ expenses, loading, editingId, onEdit }: Props) {
  // Agrupamos por día (la API ya los entrega ordenados del más reciente al más antiguo).
  const groups = useMemo(() => {
    const map = new Map<string, Expense[]>();
    for (const e of expenses) {
      const list = map.get(e.spentOn);
      if (list) list.push(e);
      else map.set(e.spentOn, [e]);
    }
    return [...map.entries()];
  }, [expenses]);

  if (!loading && expenses.length === 0) {
    return (
      <div className="empty-state">
        <p className="empty-title">Aquí no hay gastos todavía</p>
        <p>Anota el primero y aparecerá en la barra de arriba.</p>
      </div>
    );
  }

  return (
    <div className={`list${loading ? " loading" : ""}`} aria-busy={loading}>
      {groups.map(([date, items]) => (
        <section key={date} className="day">
          <h3>
            <span className="day-name">{dayLabel(date)}</span>
            <span className="day-total">{formatCOP(items.reduce((s, e) => s + e.amount, 0))}</span>
          </h3>
          <ul>
            {items.map((e) => (
              <li key={e.id}>
                {/* Toda la fila es el botón: tocarla abre el gasto para editarlo o borrarlo. */}
                <button
                  type="button"
                  className={`row${editingId === e.id ? " editing" : ""}`}
                  aria-current={editingId === e.id ? "true" : undefined}
                  onClick={() => onEdit(e)}
                >
                  <i className={`dot cat-${e.category}`} />
                  <span className="row-main">
                    <span className="sr-only">Editar: </span>
                    <span className="row-title">{e.description || categoryLabel(e.category)}</span>
                    {e.description && <span className="row-sub">{categoryLabel(e.category)}</span>}
                  </span>
                  <span className="row-amount">{formatCOP(e.amount)}</span>
                  <span className="row-go">
                    <ChevronRight />
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
