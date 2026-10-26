// Íconos dibujados a mano (trazo de 2px, esquinas redondeadas). Son decorativos:
// el texto o el aria-label del botón dice qué hacen.

const base = {
  width: 20,
  height: 20,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  "aria-hidden": true,
  focusable: false,
};

export function ChevronLeft() {
  return (
    <svg {...base}>
      <path d="M15 18l-6-6 6-6" />
    </svg>
  );
}

export function ChevronRight() {
  return (
    <svg {...base}>
      <path d="M9 18l6-6-6-6" />
    </svg>
  );
}

export function Plus() {
  return (
    <svg {...base}>
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}
