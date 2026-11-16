import type { DeathStats } from "../api";
import { num, pct, plural, regionName, upperFirst } from "../format";

const MIN_BUCKET = 5; // games a winrate side needs before it is compared

/** Avoidable deaths over parsed replays: how many, why, where and what they cost. */
export function AvoidDeathsCard({ s }: { s: DeathStats }) {
  if (s.games === 0) {
    return <div className="card"><div className="card-head"><h2>Глупые смерти</h2></div><div className="dim">Нужны разобранные реплеи.</div></div>;
  }
  const share = s.deaths ? s.avoidable / s.deaths : 0;
  const notes: string[] = [];
  if (s.clean.games >= MIN_BUCKET && s.messy.games >= MIN_BUCKET) {
    const d = Math.round((s.clean.winrate - s.messy.winrate) * 100);
    if (d >= 5) notes.push(`С 0–1 глупой смертью выигрываешь ${pct(s.clean.winrate)}, с двумя и больше — ${pct(s.messy.winrate)}. Это ${d}% винрейта.`);
  }
  if (s.recent > 0 || s.before > 0) {
    const d = s.recent - s.before;
    if (Math.abs(d) >= 0.3) notes.push(`Последние 10 игр — ${s.recent.toFixed(1)} за игру против ${s.before.toFixed(1)} раньше: ${d < 0 ? "стало лучше" : "стало хуже"}.`);
  }
  if (s.rich >= 3) notes.push(`${s.rich} ${plural(s.rich, "раз", "раза", "раз")} умирал с 1 500+ непотраченного золота — сначала закупись, потом иди.`);
  const reasons = [
    { label: "один без вижна", n: s.blind },
    { label: "один на чужой половине", n: s.deep },
    { label: "один против двоих и больше", n: s.outnumbered },
  ].filter((r) => r.n > 0).sort((a, b) => b.n - a.n);
  const max = Math.max(1, ...reasons.map((r) => r.n));
  const zmax = Math.max(1, ...(s.zones ?? []).map((z) => z.sec));
  return (
    <div className="card">
      <div className="card-head">
        <h2>Глупые смерти</h2>
        <span className="meta">один и без вижна, на чужой половине или против нескольких · {s.games} {plural(s.games, "игра", "игры", "игр")} с реплеем</span>
      </div>
      <div className="grid-side" style={{ gridTemplateColumns: "minmax(0, 1fr) minmax(0, 1fr)", gap: 22 }}>
        <div>
          <div className="tiles">
            <div className="tile"><div className="label">За игру</div><div className={`val ${s.per_game >= 1.5 ? "loss" : ""}`}>{s.per_game.toFixed(1)}</div></div>
            <div className="tile"><div className="label">Из всех смертей</div><div className="val">{pct(share)}</div></div>
            <div className="tile"><div className="label">Золота теряешь на них</div><div className="val">{num(-Math.round(s.gold_lost))}</div><div className="faint small">за игру</div></div>
            <div className="tile"><div className="label">Всего</div><div className="val">{s.avoidable} <span className="dim" style={{ fontSize: 12 }}>из {s.deaths}</span></div></div>
          </div>
        </div>
        <div>
          <div className="caps" style={{ marginBottom: 6 }}>Почему</div>
          {reasons.length === 0 && <div className="faint small">глупых смертей нет</div>}
          {reasons.map((r) => (
            <div key={r.label} className="zone-row">
              <span>{upperFirst(r.label)}</span>
              <div className="bar bad"><div style={{ width: `${(r.n / max) * 100}%` }} /></div>
              <span className="mono faint">{r.n}</span>
            </div>
          ))}
          {(s.zones ?? []).length > 0 && <div className="caps" style={{ margin: "14px 0 6px" }}>Где</div>}
          {(s.zones ?? []).map((z) => (
            <div key={z.region} className="zone-row">
              <span>{upperFirst(regionName(z.region))}</span>
              <div className="bar bad"><div style={{ width: `${(z.sec / zmax) * 100}%` }} /></div>
              <span className="mono faint">{z.sec}</span>
            </div>
          ))}
        </div>
      </div>
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
    </div>
  );
}
