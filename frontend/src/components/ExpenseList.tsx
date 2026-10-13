import { useMemo, useState } from "react";
import { categoryLabel, dayLabel, formatCOP } from "../format";
import type { Expense } from "../types";

interface Props {
  expenses: Expense[];
  loading: boolean;
  editingId: string | null;
  onEdit: (e: Expense) => void;
  onDelete: (e: Expense) => Promise<void>;
}

export default function ExpenseList({ expenses, loading, editingId, onEdit, onDelete }: Props) {
  // Borrar pide confirmación dentro de la misma fila (sin ventanas emergentes).
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

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

  async function confirmDelete(e: Expense) {
    setDeleting(true);
    try {
      await onDelete(e);
    } finally {
      setDeleting(false);
      setConfirmId(null);
    }
  }

  if (!loading && expenses.length === 0) {
    return (
      <div className="empty-state">
        <p className="empty-title">Aquí no hay gastos todavía</p>
        <p>Anota el primero con el formulario y aparecerá en la barra de arriba.</p>
      </div>
    );
  }

  return (
    <div className={`list${loading ? " loading" : ""}`} aria-busy={loading}>
      {groups.map(([date, items]) => (
        <section key={date} className="day">
          <h3>{dayLabel(date)}</h3>
          <ul>
            {items.map((e) => (
              <li key={e.id} className={`row${editingId === e.id ? " editing" : ""}`}>
                <i className={`dot cat-${e.category}`} />
                <div className="row-main">
                  <p className="row-title">{e.description || categoryLabel(e.category)}</p>
                  {e.description && <p className="row-sub">{categoryLabel(e.category)}</p>}
                </div>
                <p className="row-amount">{formatCOP(e.amount)}</p>

                <div className="row-actions">
                  {confirmId === e.id ? (
                    <>
                      <button className="btn small danger" disabled={deleting} onClick={() => confirmDelete(e)}>
                        Sí, borrar
                      </button>
                      <button className="btn small" disabled={deleting} onClick={() => setConfirmId(null)}>
                        Cancelar
                      </button>
                    </>
                  ) : (
                    <>
                      <button className="btn small" onClick={() => onEdit(e)} aria-label={`Editar gasto de ${formatCOP(e.amount)}`}>
                        Editar
                      </button>
                      <button
                        className="btn small"
                        onClick={() => setConfirmId(e.id)}
                        aria-label={`Borrar gasto de ${formatCOP(e.amount)}`}
                      >
                        Borrar
                      </button>
                    </>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
