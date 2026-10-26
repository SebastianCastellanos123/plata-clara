import { useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError } from "../api";
import { CATEGORIES, defaultDateFor, digitsOnly, formatThousands } from "../format";
import type { Category, Expense } from "../types";

interface Props {
  month: string;
  editing: Expense | null;
  /** Categoría a proponer en un gasto nuevo (la del filtro activo, si hay). */
  preferredCategory: Category | null;
  onSaved: (e: Expense, wasEditing: boolean) => void;
  onCancelEdit: () => void;
  onDelete: (e: Expense) => Promise<void>;
}

export default function ExpenseForm({ month, editing, preferredCategory, onSaved, onCancelEdit, onDelete }: Props) {
  const [amount, setAmount] = useState(""); // solo dígitos
  const [category, setCategory] = useState<Category>(preferredCategory ?? "comida");
  const [description, setDescription] = useState("");
  const [spentOn, setSpentOn] = useState(defaultDateFor(month));
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const amountRef = useRef<HTMLInputElement>(null);
  // La última categoría guardada: quien anota varios gastos seguidos suele repetirla.
  const lastCategory = useRef<Category>("comida");

  // Si empiezas a editar un gasto, el formulario se llena con sus datos.
  // Si no, queda en blanco y con la fecha por defecto del mes que estás viendo.
  useEffect(() => {
    if (editing) {
      setAmount(String(editing.amount));
      setCategory(editing.category);
      setDescription(editing.description);
      setSpentOn(editing.spentOn);
    } else {
      setAmount("");
      setCategory(preferredCategory ?? lastCategory.current);
      setDescription("");
      setSpentOn(defaultDateFor(month));
    }
    setFields({});
    setError("");
    // preferredCategory va en su propio efecto: cambiar el filtro no debe borrar lo que escribiste.
  }, [editing, month]);

  useEffect(() => {
    if (!editing && preferredCategory) setCategory(preferredCategory);
  }, [preferredCategory, editing]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setFields({});
    setError("");

    const value = Number(amount);
    if (!amount || value <= 0) {
      setFields({ amount: "Escribe cuánto gastaste" });
      amountRef.current?.focus();
      return;
    }

    setBusy(true);
    try {
      const input = { amount: value, category, description, spentOn };
      const saved = editing ? await api.updateExpense(editing.id, input) : await api.createExpense(input);
      lastCategory.current = category;
      onSaved(saved, Boolean(editing));
      if (!editing) {
        // Tras guardar, dejamos listo el siguiente: se limpia monto y descripción.
        setAmount("");
        setDescription("");
        amountRef.current?.focus();
      }
    } catch (err) {
      if (err instanceof ApiError) {
        setFields(err.fields ?? {});
        setError(err.fields ? "" : err.message);
      } else {
        setError("No pudimos conectar con el servidor. Intenta de nuevo");
      }
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!editing) return;
    setBusy(true);
    try {
      await onDelete(editing);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form id="expense-form" className={`panel expense-form${editing ? " is-editing" : ""}`} onSubmit={submit} noValidate>
      <h2>{editing ? "Editar gasto" : "Nuevo gasto"}</h2>

      <div className="field">
        <label htmlFor="amount">Monto</label>
        <div className="money">
          <span aria-hidden="true">$</span>
          <input
            id="amount"
            ref={amountRef}
            inputMode="numeric"
            autoComplete="off"
            placeholder="0"
            value={amount ? formatThousands(Number(amount)) : ""}
            onChange={(e) => setAmount(digitsOnly(e.target.value))}
            aria-invalid={Boolean(fields.amount)}
            aria-describedby={fields.amount ? "amount-err" : undefined}
          />
        </div>
        {fields.amount && (
          <p id="amount-err" className="field-error">
            {fields.amount}
          </p>
        )}
      </div>

      <fieldset className="field chips" aria-invalid={Boolean(fields.category)} aria-describedby={fields.category ? "category-err" : undefined}>
        <legend>Categoría</legend>
        <div className="chip-grid">
          {CATEGORIES.map((c) => (
            <label key={c.id} className={`chip cat-${c.id}`}>
              <input
                type="radio"
                name="category"
                value={c.id}
                checked={category === c.id}
                onChange={() => setCategory(c.id)}
              />
              <i className="dot" />
              {c.label}
            </label>
          ))}
        </div>
        {fields.category && (
          <p id="category-err" className="field-error">
            {fields.category}
          </p>
        )}
      </fieldset>

      <div className="field">
        <label htmlFor="description">
          Nota <span className="optional">(opcional)</span>
        </label>
        <input
          id="description"
          maxLength={200}
          autoComplete="off"
          placeholder="Almuerzo con Laura"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          aria-invalid={Boolean(fields.description)}
          aria-describedby={fields.description ? "description-err" : undefined}
        />
        {fields.description && (
          <p id="description-err" className="field-error">
            {fields.description}
          </p>
        )}
      </div>

      <div className="field">
        <label htmlFor="spentOn">Fecha</label>
        <input
          id="spentOn"
          type="date"
          value={spentOn}
          onChange={(e) => setSpentOn(e.target.value)}
          aria-invalid={Boolean(fields.spentOn)}
          aria-describedby={fields.spentOn ? "spentOn-err" : undefined}
        />
        {fields.spentOn && (
          <p id="spentOn-err" className="field-error">
            {fields.spentOn}
          </p>
        )}
      </div>

      <p className="form-error" role="alert">
        {error}
      </p>

      <div className="form-actions">
        <button className="btn primary" type="submit" disabled={busy}>
          {busy ? "Guardando…" : editing ? "Guardar cambios" : "Guardar gasto"}
        </button>
        {editing && (
          <button className="btn" type="button" onClick={onCancelEdit} disabled={busy}>
            Cancelar
          </button>
        )}
      </div>

      {editing && (
        <button className="btn danger delete" type="button" onClick={remove} disabled={busy}>
          Borrar este gasto
        </button>
      )}
    </form>
  );
}
