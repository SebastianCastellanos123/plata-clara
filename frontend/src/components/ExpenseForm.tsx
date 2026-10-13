import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError } from "../api";
import { CATEGORIES, defaultDateFor, digitsOnly, formatThousands } from "../format";
import type { Category, Expense } from "../types";

interface Props {
  month: string;
  editing: Expense | null;
  onSaved: (e: Expense) => void;
  onCancelEdit: () => void;
}

export default function ExpenseForm({ month, editing, onSaved, onCancelEdit }: Props) {
  const [amount, setAmount] = useState(""); // solo dígitos
  const [category, setCategory] = useState<Category>("comida");
  const [description, setDescription] = useState("");
  const [spentOn, setSpentOn] = useState(defaultDateFor(month));
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

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
      setCategory("comida");
      setDescription("");
      setSpentOn(defaultDateFor(month));
    }
    setFields({});
    setError("");
  }, [editing, month]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setFields({});
    setError("");

    const value = Number(amount);
    if (!amount || value <= 0) {
      setFields({ amount: "Escribe cuánto gastaste" });
      return;
    }

    setBusy(true);
    try {
      const input = { amount: value, category, description, spentOn };
      const saved = editing ? await api.updateExpense(editing.id, input) : await api.createExpense(input);
      onSaved(saved);
      if (!editing) {
        // Tras guardar, dejamos listo el siguiente: se limpia monto y descripción.
        setAmount("");
        setDescription("");
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

  return (
    <form className="panel expense-form" onSubmit={submit} noValidate>
      <h2>{editing ? "Editar gasto" : "Nuevo gasto"}</h2>

      <div className="field">
        <label htmlFor="amount">Monto</label>
        <div className="money">
          <span aria-hidden="true">$</span>
          <input
            id="amount"
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

      <div className="field">
        <label htmlFor="category">Categoría</label>
        <select
          id="category"
          value={category}
          onChange={(e) => setCategory(e.target.value as Category)}
          aria-invalid={Boolean(fields.category)}
        >
          {CATEGORIES.map((c) => (
            <option key={c.id} value={c.id}>
              {c.label}
            </option>
          ))}
        </select>
        {fields.category && <p className="field-error">{fields.category}</p>}
      </div>

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
        />
        {fields.description && <p className="field-error">{fields.description}</p>}
      </div>

      <div className="field">
        <label htmlFor="spentOn">Fecha</label>
        <input
          id="spentOn"
          type="date"
          value={spentOn}
          onChange={(e) => setSpentOn(e.target.value)}
          aria-invalid={Boolean(fields.spentOn)}
        />
        {fields.spentOn && <p className="field-error">{fields.spentOn}</p>}
      </div>

      <p className="form-error" role="alert">
        {error}
      </p>

      <div className="form-actions">
        <button className="btn primary" type="submit" disabled={busy}>
          {busy ? "Guardando…" : editing ? "Guardar cambios" : "Guardar gasto"}
        </button>
        {editing && (
          <button className="btn" type="button" onClick={onCancelEdit}>
            Cancelar
          </button>
        )}
      </div>
    </form>
  );
}
