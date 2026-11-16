import { useEffect, useState } from "react";
import { Api, GoalProgress, PlanView } from "../api";
import { pct, plural } from "../format";
import { gameMetric } from "../gameMetrics";

/** Weekly plan: loads itself, reloads when matches change (tick). */
export function PlanCard({ tick, onMatch }: { tick: number; onMatch: (id: number) => void }) {
  const [v, setV] = useState<PlanView | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  useEffect(() => {
    Api.Plan().then(setV).catch((e) => setErr(String(e?.message ?? e)));
  }, [tick]);
  const make = () => {
    setBusy(true);
    setErr("");
    Api.NewPlan().then(setV).catch((e) => setErr(String(e?.message ?? e))).finally(() => setBusy(false));
  };
  if (!v || v.unknown) return err ? <div className="notice bad">{err}</div> : null;
  const p = v.plan;
  if (!p) {
    return (
      <div className="card plan-card empty-plan">
        <div>
          <h2>План на неделю</h2>
          <div className="dim">
            {v.enough
              ? "Возьмём 3 слабых показателя из последних 30 игр — в первую очередь те, от которых сильнее всего зависят твои победы, — поставим реалистичные цели и будем отслеживать каждую новую игру."
              : "Нужно хотя бы 10 разобранных игр с твоим участием, чтобы составить план."}
          </div>
        </div>
        <button className="primary" disabled={busy || !v.enough} onClick={make}>{busy ? "Составляю…" : "Составить план"}</button>
        {err && <div className="loss small">{err}</div>}
      </div>
    );
  }
  const goals = v.goals ?? [];
  const done = goals.filter((g) => reached(g)).length;
  const daysLeft = Math.max(0, Math.ceil((p.end * 1000 - Date.now()) / 86400000));
  return (
    <div className="card plan-card">
      <div className="card-head">
        <h2>План на неделю</h2>
        <span className="meta">
          {v.expired ? `неделя закончилась · выполнено ${done} из ${goals.length}` : `осталось ${daysLeft} ${plural(daysLeft, "день", "дня", "дней")} · ${v.games} ${plural(v.games, "игра", "игры", "игр")} сыграно`}
          {" · "}<button className="link" disabled={busy} onClick={make}>{v.expired ? "новый план" : "пересоставить"}</button>
        </span>
      </div>
      <div className="plan-goals">
        {goals.map((g) => <Goal key={g.key} g={g} onMatch={onMatch} />)}
      </div>
      {err && <div className="loss small">{err}</div>}
    </div>
  );
}

function reached(g: GoalProgress) {
  if (g.current == null) return false;
  return g.higher_better ? g.current >= g.target : g.current <= g.target;
}

function Goal({ g, onMatch }: { g: GoalProgress; onMatch: (id: number) => void }) {
  const d = gameMetric(g.key);
  const games = g.games ?? [];
  const cur = g.current;
  // Progress from the baseline to the target, 0..1.
  const span = g.target - g.base;
  const prog = cur == null || span === 0 ? 0 : Math.max(0, Math.min(1, (cur - g.base) / span));
  const ok = reached(g);
  return (
    <div className={`plan-goal ${ok ? "ok" : ""}`}>
      <div className="pg-head">
        <b>{d.label}</b>
        <span className="mono">{d.fmt(g.base)} → <span className="accent">{d.fmt(g.target)}</span></span>
      </div>
      <div className="pg-now">
        <span className={`mono big ${cur == null ? "faint" : ok ? "win" : ""}`}>{cur == null ? "—" : d.fmt(cur)}</span>
        <span className="dim small">{cur == null ? "сыграй игру — посчитаем" : `сейчас, в ${g.hits} из ${games.length} ${plural(games.length, "игры", "игр", "игр")} цель взята`}</span>
      </div>
      <div className="bar"><div style={{ width: `${prog * 100}%`, background: ok ? "var(--win)" : undefined }} /></div>
      {games.length > 0 && (
        <div className="pg-dots">
          {games.map((x) => (
            <span key={x.match_id} className={x.met ? "w" : "l"} title={d.fmt(x.value)} onClick={() => onMatch(x.match_id)} />
          ))}
        </div>
      )}
      {g.win_met != null && g.win_miss != null && (
        <div className="small" style={{ marginTop: 8 }}>
          <span className="dim">Когда цель взята — </span><b className={g.win_met > g.win_miss ? "win" : ""}>{pct(g.win_met)}</b>
          <span className="dim"> побед, когда нет — </span><b className={g.win_met > g.win_miss ? "loss" : ""}>{pct(g.win_miss)}</b>
          <span className="faint"> · {g.n_met + g.n_miss} {plural(g.n_met + g.n_miss, "игра", "игры", "игр")}</span>
        </div>
      )}
      {d.tip && <div className="note" style={{ marginTop: 8 }}>{d.tip}</div>}
    </div>
  );
}
