import type { FarmStats } from "../api";
import { clock, pct, plural, regionName, upperFirst } from "../format";

const PHASE_LABEL: Record<string, string> = { lane: "Линия", mid: "Мидгейм", late: "Лейт" };
const PHASE_SPAN: Record<string, string> = { lane: "0–10 мин", mid: "10–25 мин", late: "25+ мин" };

/** Farm efficiency: gold from creeps and camps, idle and dead time by phase. */
export function FarmCard({ f }: { f: FarmStats }) {
  const phases = (f.phases ?? []).filter((p) => PHASE_LABEL[p.key]);
  if (f.games === 0) {
    return <div className="card"><div className="card-head"><h2>Фарм</h2></div><div className="dim">Нужны разобранные реплеи.</div></div>;
  }
  const worst = phases.filter((p) => p.games >= 5).reduce<(typeof phases)[number] | null>((a, b) => (!a || b.idle_pct > a.idle_pct ? b : a), null);
  const notes: string[] = [];
  if (worst && worst.idle_pct >= 0.1) notes.push(`${PHASE_LABEL[worst.key]} — больше всего простоя: ${pct(worst.idle_pct)} живого времени без фарма и драк.`);
  if (f.low_idle.games >= 5 && f.high_idle.games >= 5) {
    const d = Math.round((f.low_idle.winrate - f.high_idle.winrate) * 100);
    if (d >= 8) notes.push(`В играх с меньшим простоем выигрываешь на ${d}% чаще (${pct(f.low_idle.winrate)} против ${pct(f.high_idle.winrate)}).`);
  }
  if (f.recent_farm_gpm > 0 && f.before_farm_gpm > 0) {
    const d = f.recent_farm_gpm - f.before_farm_gpm;
    if (Math.abs(d) >= 25) notes.push(`Последние 10 игр фармишь ${d > 0 ? "быстрее" : "медленнее"}: ${Math.round(f.recent_farm_gpm)} в минуту против ${Math.round(f.before_farm_gpm)} раньше.`);
  }
  const rows: { label: string; v: (p: (typeof phases)[number]) => string; bad?: (p: (typeof phases)[number]) => boolean }[] = [
    { label: "Фарм в минуту", v: (p) => String(Math.round(p.farm_gpm)) },
    { label: "с линии", v: (p) => String(Math.round(p.creep_gpm)) },
    { label: "с леса", v: (p) => String(Math.round(p.neutral_gpm)) },
    { label: "Простой", v: (p) => pct(p.idle_pct), bad: (p) => p === worst && p.idle_pct >= 0.1 },
    { label: "Мёртвым", v: (p) => pct(p.dead_pct) },
  ];
  return (
    <div className="card">
      <div className="card-head"><h2>Фарм</h2><span className="meta">золото с крипов и лагерей · простой — от {f.threshold} с без фарма и драк</span></div>
      <div className="grid-side" style={{ gridTemplateColumns: "minmax(0, 1.4fr) minmax(0, 1fr)", gap: 22 }}>
        <table className="phase-table">
          <thead>
            <tr><th />{phases.map((p) => <th key={p.key}>{PHASE_LABEL[p.key]}<small>{PHASE_SPAN[p.key]} · {p.games} {plural(p.games, "игра", "игры", "игр")}</small></th>)}</tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.label}>
                <td style={r.label.startsWith("с ") ? { paddingLeft: 18 } : undefined}>{r.label}</td>
                {phases.map((p) => <td key={p.key} className={`mono ${!p.games ? "faint" : r.bad?.(p) ? "loss" : ""}`}>{p.games ? r.v(p) : "—"}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
        <div>
          <div className="caps">Простой за игру</div>
          <div className="stat-value" style={{ margin: "4px 0 12px" }}>{clock(f.idle_per_game)}</div>
          <div className="caps" style={{ marginBottom: 6 }}>Где простаиваешь</div>
          {(f.zones ?? []).length === 0 && <div className="faint small">нет данных</div>}
          {(f.zones ?? []).map((z) => {
            const max = f.zones![0].sec || 1;
            return (
              <div key={z.region} className="zone-row">
                <span>{upperFirst(regionName(z.region))}</span>
                <div className="bar"><div style={{ width: `${(z.sec / max) * 100}%` }} /></div>
                <span className="mono faint">{clock(z.sec)}</span>
              </div>
            );
          })}
        </div>
      </div>
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
    </div>
  );
}
