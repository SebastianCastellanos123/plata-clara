import { categoryLabel, formatCOP, percent } from "../format";
import type { Category, Summary } from "../types";

interface Props {
  summary: Summary | null;
  month: string;
  active: Category | null;
  onSelect: (c: Category | null) => void;
}

// La pieza central de la app: el total del mes y una barra que muestra cómo se reparte.
export default function SpendBar({ summary, month, active, onSelect }: Props) {
  const total = summary?.total ?? 0;
  const items = summary?.byCategory ?? [];

  return (
    <div className="spend">
      <p className="spend-total" aria-label={`Total del mes: ${formatCOP(total)}`}>
        <span className="mark">{formatCOP(total)}</span>
      </p>

      {/* key={month}: al cambiar de mes la barra se vuelve a dibujar con su animación */}
      <div className={`bar${total === 0 ? " empty" : ""}`} key={month} aria-hidden="true">
        {items.map((c) => (
          <span
            key={c.category}
            className={`seg cat-${c.category}${active && active !== c.category ? " dim" : ""}`}
            style={{ flexGrow: c.total }}
          />
        ))}
      </div>

      {items.length > 0 && (
        <ul className="legend">
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
