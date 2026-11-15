import { createContext, ReactNode, useContext } from "react";
import { copyText } from "../format";
import { Icon } from "./Icon";

/** App-wide toast, provided by the shell. */
export const ToastCtx = createContext<(msg: string, bad?: boolean) => void>(() => {});
export const useToast = () => useContext(ToastCtx);

/** Summary card: caps label, big display value, small sub line. */
export function Stat({ label, value, sub, tone, big }: {
  label: ReactNode; value: ReactNode; sub?: ReactNode; tone?: string; big?: boolean;
}) {
  return (
    <div className="stat">
      <div className="caps">{label}</div>
      <div className={`stat-value ${big ? "big" : ""} ${tone ?? ""}`}>{value}</div>
      {sub != null && sub !== "" && <div className="stat-sub">{sub}</div>}
    </div>
  );
}

/** Small tile inside a card. */
export function Tile({ label, value, tone }: { label: ReactNode; value: ReactNode; tone?: string }) {
  return (
    <div className="tile">
      <div className="label">{label}</div>
      <div className={`val ${tone ?? ""}`}>{value}</div>
    </div>
  );
}

/** Circular progress (0..1) with content in the middle. */
export function Ring({ value, size = 120, stroke = 10, color = "var(--accent)", children }: {
  value: number; size?: number; stroke?: number; color?: string; children?: ReactNode;
}) {
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const v = Math.max(0, Math.min(1, value));
  return (
    <div className="grade-ring" style={{ width: size }}>
      <svg width={size} height={size}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--panel-3)" strokeWidth={stroke} />
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round"
          strokeDasharray={`${c * v} ${c}`} transform={`rotate(-90 ${size / 2} ${size / 2})`}
          style={{ filter: `drop-shadow(0 0 10px ${color === "var(--accent)" ? "rgba(255,173,51,.35)" : "transparent"})` }} />
      </svg>
      <div className="inner" style={{ height: size }}>{children}</div>
    </div>
  );
}

/** Copies `demo_gototick N` so the moment can be opened in the Dota replay viewer. */
export function TickButton({ tick, label = "Тик для demo_gototick" }: { tick?: number; label?: string }) {
  const toast = useToast();
  if (!tick) return null;
  const cmd = `demo_gototick ${tick}`;
  return (
    <button className="tick-btn" title={`Скопировать «${cmd}» — вставь в консоль Dota при просмотре реплея`}
      onClick={async (e) => {
        e.stopPropagation();
        toast((await copyText(cmd)) ? `Скопировано: ${cmd}` : cmd);
      }}>
      <Icon name="play" size={10} /> {label}
    </button>
  );
}

/** Victory / defeat label. */
export function ResultPill({ won }: { won: boolean | null }) {
  if (won === null) return null;
  return won ? <span className="pill win">Победа</span> : <span className="pill loss">Поражение</span>;
}
