// Small stroke icons (24x24 grid, currentColor) — no icon font / network.
const PATHS: Record<string, string> = {
  pulse: "M3 12h4l3-8 4 16 3-8h4",
  list: "M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01",
  user: "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0",
  grid: "M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z",
  sliders: "M4 7h10M18 7h2M4 17h4M12 17h8M14 4v6M8 14v6",
  refresh: "M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7",
  plus: "M12 5v14M5 12h14",
  back: "M15 18l-6-6 6-6",
  check: "M5 12l5 5 9-10",
  alert: "M12 3l10 18H2L12 3zM12 10v4M12 17.5v.01",
  info: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v6M12 7.5v.01",
  eyeOff: "M3 3l18 18M10.6 6.1A10 10 0 0 1 12 6c5 0 9 6 9 6a17 17 0 0 1-2.6 3.2M6.6 6.6C4.2 8.2 3 12 3 12s4 6 9 6a9 9 0 0 0 4.4-1.1M9.9 9.9a3 3 0 0 0 4.2 4.2",
  trend: "M3 17l6-6 4 4 8-8M15 7h6v6",
  download: "M12 4v12M6 11l6 6 6-6M5 20h14",
  x: "M6 6l12 12M18 6L6 18",
};

export function Icon({ name, size = 16, className }: { name: keyof typeof PATHS | string; size?: number; className?: string }) {
  return (
    <svg className={`icon ${className ?? ""}`} width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d={PATHS[name] ?? ""} />
    </svg>
  );
}
