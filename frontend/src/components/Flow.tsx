import type { Bucket, FlowStats, PhaseStats, SwingGame } from "../api";
import { clock, kfmt, pct, plural, relDate } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "./HeroBadge";

const PHASES: Record<string, { label: string; span: string; when: string }> = {
  lane: { label: "Линия", span: "0–10 мин", when: "На линии" },
  mid: { label: "Мидгейм", span: "10–25 мин", when: "В мидгейме" },
  late: { label: "Лейт", span: "25+ мин", when: "В лейте" },
};
const LENGTHS: Record<string, string> = { "0": "до 25", "25": "25–35", "35": "35–45", "45": "45+" };
const MIN_PHASE = 5; // games a phase needs before it is judged

/** «Ход игры»: phases, thrown / comeback games and winrate by game length. */
export function FlowSection({ f, onMatch }: { f: FlowStats; onMatch: (id: number) => void }) {
  return (
    <>
      <div className="section-head"><h2>Ход игры</h2><span className="dim small">{f.games} {plural(f.games, "игра", "игры", "игр")} с реплеем</span></div>
      <PhasesCard f={f} />
      <div className="grid2" style={{ marginTop: 14 }}>
        <SwingsCard f={f} onMatch={onMatch} />
        <LengthCard buckets={f.by_length ?? []} />
      </div>
    </>
  );
}

function PhasesCard({ f }: { f: FlowStats }) {
  const phases = (f.phases ?? []).filter((p) => PHASES[p.key]);
  const judged = phases.filter((p) => p.games >= MIN_PHASE);
  // Weakest phase: where the team loses the most lead on average.
  const worst = judged.length > 1 ? judged.reduce((a, b) => (b.lead_delta < a.lead_delta ? b : a)) : null;
  const best = judged.length > 1 ? judged.reduce((a, b) => (b.lead_delta > a.lead_delta ? b : a)) : null;
  const notes: string[] = [];
  if (worst && worst.lead_delta < 0) {
    const ph = PHASES[worst.key];
    notes.push(`${ph.when} команда в среднем теряет ${kfmt(-worst.lead_delta)} преимущества — это твоя слабая фаза.`);
    const others = judged.filter((p) => p !== worst);
    const deathsElse = others.reduce((a, p) => a + p.deaths10, 0) / others.length;
    if (worst.deaths10 > deathsElse * 1.3) notes.push(`И умираешь там чаще: ${worst.deaths10.toFixed(1)} за 10 минут против ${deathsElse.toFixed(1)} в других фазах.`);
    const kpElse = others.reduce((a, p) => a + p.kp, 0) / others.length;
    if (worst.kp < kpElse - 0.08) notes.push(`Реже участвуешь в убийствах: ${pct(worst.kp)} против ${pct(kpElse)}.`);
  }
  if (best && best.lead_delta > 0 && best !== worst) notes.push(`Сильнее всего команда играет ${PHASES[best.key].when.toLowerCase()}: +${kfmt(best.lead_delta)} в среднем.`);
  if (judged.length < 2) notes.push(`Нужно хотя бы по ${MIN_PHASE} игр в фазе, чтобы делать выводы.`);
  const rows: { label: string; v: (p: PhaseStats) => string; tone?: (p: PhaseStats) => string }[] = [
    { label: "GPM", v: (p) => String(Math.round(p.gpm)) },
    { label: "XPM", v: (p) => String(Math.round(p.xpm)) },
    { label: "Добиваний в минуту", v: (p) => p.lhpm.toFixed(1) },
    { label: "Смертей за 10 мин", v: (p) => p.deaths10.toFixed(1), tone: (p) => (p === worst && p.deaths10 > 0 ? "loss" : "") },
    { label: "Участие в убийствах", v: (p) => pct(p.kp) },
    { label: "Преимущество команды", v: (p) => kfmt(p.lead_delta, true), tone: (p) => (p.lead_delta >= 0 ? "win" : "loss") },
    { label: "Фаза выиграна", v: (p) => (p.games ? `${pct(p.gained / p.games)} игр` : "—") },
    { label: "Победа, если фаза выиграна / проиграна", v: (p) => `${p.gained ? pct(p.win_gained) : "—"} / ${p.games - p.gained ? pct(p.win_lost) : "—"}` },
  ];
  return (
    <div className="card">
      <div className="card-head"><h2>Фазы игры</h2><span className="meta">средние за фазу</span></div>
      {phases.every((p) => p.games === 0) ? <div className="dim">Нужны разобранные реплеи.</div> : (
        <table className="phase-table">
          <thead>
            <tr><th />{phases.map((p) => (
              <th key={p.key} className={p === worst ? "loss" : ""}>{PHASES[p.key].label}<small>{PHASES[p.key].span} · {p.games} {plural(p.games, "игра", "игры", "игр")}</small></th>
            ))}</tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.label}>
                <td>{r.label}</td>
                {phases.map((p) => <td key={p.key} className={`mono ${p.games ? r.tone?.(p) ?? "" : "faint"}`}>{p.games ? r.v(p) : "—"}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
    </div>
  );
}

function SwingsCard({ f, onMatch }: { f: FlowStats; onMatch: (id: number) => void }) {
  const names = useNames();
  const thrown = f.ahead.games - f.ahead.wins;
  const back = f.behind.wins;
  const tile = (label: string, b: Bucket, good: boolean, sub: string) => (
    <div className="tile" style={{ padding: "14px 16px" }}>
      <div className="label">{label} · {b.games} {plural(b.games, "игра", "игры", "игр")}</div>
      <div className={`val ${b.games ? (good ? "win" : "loss") : "faint"}`} style={{ fontSize: 26 }}>{b.games ? pct(b.winrate) : "—"}</div>
      <div className="label">{sub}</div>
    </div>
  );
  const row = (g: SwingGame) => {
    const kp = g.kp >= 0 ? `участие ${pct(g.kp)}` : "убийств не было";
    const gpm = g.game_gpm > 0 && g.gpm > 0 ? ` · GPM ${Math.round(g.gpm)} (за игру ${Math.round(g.game_gpm)})` : "";
    return (
      <div key={g.match_id} className="swing" onClick={() => onMatch(g.match_id)}>
        <HeroBadge npc={g.hero_name} size={30} />
        <div>
          <b>{g.win ? "Вытащили" : "Слили"} {kfmt(g.peak)} на {clock(g.peak_t)}{g.flip_t >= 0 ? ` → ${g.win ? "вышли вперёд" : "ушло"} к ${clock(g.flip_t)}` : ""}</b>
          <small>{names.hero(g.hero_name)} · {relDate(g.end_time).replace(/, \d\d:\d\d$/, "")} · за это время: {g.deaths} {plural(g.deaths, "смерть", "смерти", "смертей")}, {kp}{gpm}</small>
        </div>
      </div>
    );
  };
  return (
    <div className="card">
      <div className="card-head"><h2>Слитые и вытащенные</h2><span className="meta">перевес от {kfmt(5000)}</span></div>
      <div className="tiles">
        {tile("Вели", f.ahead, f.ahead.winrate >= 0.75, thrown ? `слили ${thrown}` : "ни одной слитой")}
        {tile("Отставали", f.behind, f.behind.winrate >= 0.2, back ? `вытащили ${back}` : "ни одной вытащенной")}
      </div>
      {thrown > 0 && (
        <div className="note">
          Пока преимущество уходило, ты в среднем умирал {f.throw_deaths.toFixed(1)} {plural(Math.round(f.throw_deaths), "раз", "раза", "раз")}
          {f.throw_kp > 0 ? ` и участвовал в ${pct(f.throw_kp)} убийств команды` : ""}.
        </div>
      )}
      {(f.thrown ?? []).length > 0 && <><div className="caps" style={{ margin: "14px 0 6px" }}>Последние слитые</div>{(f.thrown ?? []).map(row)}</>}
      {(f.comebacks ?? []).length > 0 && <><div className="caps" style={{ margin: "14px 0 6px" }}>Последние камбэки</div>{(f.comebacks ?? []).map(row)}</>}
    </div>
  );
}

function LengthCard({ buckets }: { buckets: Bucket[] }) {
  const played = buckets.filter((b) => b.games >= 3);
  const best = played.length > 1 ? played.reduce((a, b) => (b.winrate > a.winrate ? b : a)) : null;
  const worst = played.length > 1 ? played.reduce((a, b) => (b.winrate < a.winrate ? b : a)) : null;
  let note = "";
  if (best && worst && best !== worst && best.winrate - worst.winrate >= 0.1) {
    const fast = Number(best.key) < Number(worst.key);
    note = fast ? `Ты сильнее в быстрых играх: ${LENGTHS[best.key]} мин — ${pct(best.winrate)}, ${LENGTHS[worst.key]} мин — ${pct(worst.winrate)}. Дави темпом и заканчивай.`
      : `Ты сильнее в долгих играх: ${LENGTHS[best.key]} мин — ${pct(best.winrate)}, ${LENGTHS[worst.key]} мин — ${pct(worst.winrate)}. Не бойся затягивать.`;
  } else if (played.length > 1) note = "Длина игры почти не влияет на результат.";
  return (
    <div className="card">
      <div className="card-head"><h2>Длина игры</h2><span className="meta">все игры</span></div>
      <div className="dayparts">
        {buckets.map((b) => (
          <div className="col" key={b.key}>
            <b className={b.games && b.winrate < 0.5 ? "loss" : ""}>{b.games ? pct(b.winrate) : "—"}</b>
            <div className="b" style={{ height: `${Math.max(4, b.winrate * 100)}%`,
              background: b.games === 0 ? "#1f232c" : b === best ? "var(--accent)" : b === worst && b.winrate < 0.5 ? "var(--loss)" : "#2c313c" }} />
          </div>
        ))}
      </div>
      <div className="dayparts-labels">
        {buckets.map((b) => <div key={b.key}><span>{LENGTHS[b.key]} мин</span><small>{b.games} {plural(b.games, "игра", "игры", "игр")}</small></div>)}
      </div>
      {note && <div className="note">{note}</div>}
    </div>
  );
}

// ---------------------------------------------------------------- one match

type PhaseRow = { label: string; key: "gpm" | "xpm" | "lhpm" | "deaths10" | "kp" | "lead_delta"; fmt: (v: number) => string; lower?: boolean; minDiff: number };
const MATCH_ROWS: PhaseRow[] = [
  { label: "GPM", key: "gpm", fmt: (v) => String(Math.round(v)), minDiff: 40 },
  { label: "XPM", key: "xpm", fmt: (v) => String(Math.round(v)), minDiff: 40 },
  { label: "Добиваний в минуту", key: "lhpm", fmt: (v) => v.toFixed(1), minDiff: 0.5 },
  { label: "Смертей за 10 мин", key: "deaths10", fmt: (v) => v.toFixed(1), lower: true, minDiff: 0.7 },
  { label: "Участие в убийствах", key: "kp", fmt: (v) => pct(v), minDiff: 0.1 },
  { label: "Преимущество команды", key: "lead_delta", fmt: (v) => kfmt(v, true), minDiff: Infinity },
];

/** Phases of one match against your usual numbers on the hero. */
export function MatchPhasesCard({ phases, avg, swing, heroName, own }: {
  phases: PhaseStats[]; avg: PhaseStats[] | null; swing: SwingGame | null; heroName: string; own: boolean;
}) {
  const shown = phases.filter((p) => PHASES[p.key]);
  if (!shown.some((p) => p.games)) return null;
  const avgOf = (key: string) => avg?.find((a) => a.key === key && a.games >= 3);
  // Notes: biggest drops against the usual numbers, the phase where the team lost most.
  const notes: { text: string; good: boolean; w: number }[] = [];
  for (const p of shown) {
    const a = avgOf(p.key);
    if (!p.games || !a) continue;
    for (const r of MATCH_ROWS) {
      if (r.key === "lead_delta" || (r.key === "kp" && p.kp < 0)) continue;
      const d = p[r.key] - a[r.key];
      if (Math.abs(d) < r.minDiff) continue;
      const better = r.lower ? d < 0 : d > 0;
      notes.push({
        good: better, w: Math.abs(d) / r.minDiff,
        text: `${PHASES[p.key].when}: ${r.label.toLowerCase()} ${r.fmt(p[r.key])} против обычных ${r.fmt(a[r.key])}`,
      });
    }
  }
  const worst = shown.filter((p) => p.games).reduce<PhaseStats | null>((a, b) => (!a || b.lead_delta < a.lead_delta ? b : a), null);
  const lines = [...notes].sort((a, b) => b.w - a.w).slice(0, 4);
  return (
    <div className="card">
      <div className="card-head"><h2>Фазы игры</h2>{avg && <span className="meta">мелким — твоё среднее на {heroName}</span>}</div>
      <table className="phase-table">
        <thead>
          <tr><th />{shown.map((p) => (
            <th key={p.key} className={p === worst && p.lead_delta < 0 ? "loss" : ""}>{PHASES[p.key].label}<small>{PHASES[p.key].span}</small></th>
          ))}</tr>
        </thead>
        <tbody>
          {MATCH_ROWS.map((r) => (
            <tr key={r.key}>
              <td>{r.label}</td>
              {shown.map((p) => {
                if (!p.games) return <td key={p.key} className="faint">—</td>;
                const v = p[r.key];
                if (r.key === "kp" && v < 0) return <td key={p.key} className="faint">убийств не было</td>;
                const a = avgOf(p.key);
                let tone = "";
                if (r.key === "lead_delta") tone = v >= 0 ? "win" : "loss";
                else if (a && Math.abs(v - a[r.key]) >= r.minDiff) tone = (r.lower ? v < a[r.key] : v > a[r.key]) ? "win" : "loss";
                return (
                  <td key={p.key} className="mono">
                    <span className={tone}>{r.fmt(v)}</span>
                    {a && r.key !== "lead_delta" && <small className="avg">{r.fmt(a[r.key])}</small>}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
      {swing && (
        <div className={`swing-note ${swing.win ? "win" : "loss"}`}>
          {swing.win ? "Камбэк" : "Слитая игра"}: {swing.win ? "отставали" : "вели"} на {kfmt(swing.peak)} к {clock(swing.peak_t)}
          {swing.flip_t >= 0 ? `, к ${clock(swing.flip_t)} ${swing.win ? "вышли вперёд" : "преимущество ушло"}` : ""}.
          {" "}За это время ты умер {swing.deaths} {plural(swing.deaths, "раз", "раза", "раз")}
          {swing.kp >= 0 ? `, участвовал в ${pct(swing.kp)} убийств` : ""}
          {swing.gpm > 0 && swing.game_gpm > 0 ? `, GPM ${Math.round(swing.gpm)} (за игру ${Math.round(swing.game_gpm)})` : ""}.
        </div>
      )}
      {lines.length > 0 && (
        <ul className="phase-notes">
          {lines.map((n) => <li key={n.text} className={n.good ? "good" : "bad"}>{n.text}</li>)}
        </ul>
      )}
      {own && !avg && <div className="note">Твоё среднее на этом герое появится, когда наберётся хотя бы 3 разобранные игры.</div>}
    </div>
  );
}
