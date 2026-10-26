import { categoryLabel, formatCOP, monthName, percent } from "../format";
import type { Category, Summary } from "../types";

interface Props {
  summary: Summary | null;
  month: string;
  /** Total del mes anterior, como punto de referencia. null si no hay datos. */
  previous: { month: string; total: number } | null;
  active: Category | null;
  onSelect: (c: Category | null) => void;
}

// La pieza central de la app: el total del mes y una barra que muestra cómo se reparte.
export default function SpendBar({ summary, month, previous, active, onSelect }: Props) {
  const total = summary?.total ?? 0;
  const items = summary?.byCategory ?? [];
  const count = items.reduce((n, c) => n + c.count, 0);
  const activeItem = active ? items.find((c) => c.category === active) : undefined;

  // Con una categoría elegida, el número grande es el de esa categoría y la
  // línea de abajo dice qué parte del mes representa.
  let context = "";
  if (activeItem) {
    context = `${percent(activeItem.total, total)}% de los ${unbroken(total)} de ${monthName(month)}`;
  } else if (summary) {
    context = count === 0 ? "Sin gastos anotados" : `${count} ${count === 1 ? "gasto anotado" : "gastos anotados"}`;
    if (previous && previous.total > 0) {
      context += ` · ${capitalize(monthName(previous.month))} cerró en ${unbroken(previous.total)}`;
    }
  }

  return (
    <div className="spend">
      <div className="spend-head" aria-live="polite" aria-atomic="true">
        {summary ? (
          <p className="spend-total">
            <span className="sr-only">{activeItem ? `Total en ${categoryLabel(activeItem.category)}:` : "Total del mes:"} </span>
            <span className="mark">{formatCOP(activeItem ? activeItem.total : total)}</span>
          </p>
        ) : (
          <p className="spend-total" aria-busy="true">
            <span className="sr-only">Cargando el total del mes</span>
            <span className="mark skeleton" aria-hidden="true">
              $ 0.000.000
            </span>
          </p>
        )}
        <p className="spend-context">{context || " "}</p>
      </div>

      {/* key={month}: al cambiar de mes la barra se vuelve a dibujar con su animación */}
      <div className="tray" aria-hidden="true">
        <div className={`bar${summary && total === 0 ? " empty" : ""}`} key={month}>
          {items.map((c) => (
            <span
              key={c.category}
              className={`seg cat-${c.category}${active && active !== c.category ? " dim" : ""}`}
              style={{ flexGrow: c.total }}
            />
          ))}
        </div>
      </div>

      {items.length > 0 && (
        <ul className="legend" aria-label="Filtrar por categoría">
          {items.map((c) => {
            const isActive = active === c.category;
            return (
              <li key={c.category}>
                <button
                  type="button"
                  className={`legend-item${isActive ? " on" : ""}`}
                  aria-pressed={isActive}
                  onClick={() => onSelect(isActive ? null : c.category)}
                >
                  <i className={`dot cat-${c.category}`} />
                  <span className="legend-name">{categoryLabel(c.category)}</span>
                  <span className="legend-amount">{formatCOP(c.total)}</span>
                  <span className="legend-pct">{percent(c.total, total)}%</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/** El monto dentro de una frase: que el "$" no quede solo al final de una línea. */
function unbroken(n: number): string {
  return formatCOP(n).replace(" ", " ");
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}
