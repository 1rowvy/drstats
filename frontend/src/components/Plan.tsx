import { useEffect, useState } from "react";
import { Api, GoalProgress, PlanView } from "../api";
import { num, pct, plural } from "../format";

const GOALS: Record<string, { label: string; fmt: (v: number) => string; tip: string }> = {
  lh10: { label: "Добивания к 10:00", fmt: (v) => String(Math.round(v)), tip: "Первые 10 минут — крипы важнее убийств: добивай каждую волну, не уходи с линии без причины." },
  deaths: { label: "Смертей за игру", fmt: (v) => v.toFixed(1), tip: "Перед каждым заходом смотри на миникарту: сколько врагов сейчас не видно." },
  deaths10: { label: "Смертей до 10:00", fmt: (v) => v.toFixed(1), tip: "На линии держись у своих крипов; пропали враги с карты — отходи." },
  kp: { label: "Участие в убийствах", fmt: (v) => pct(v), tip: "Зачистил волну — посмотри на карту: где команда, туда и иди." },
  lane_nw: { label: "Нетворс к оппоненту на 10:00", fmt: (v) => num(v, true), tip: "Не уступай волну: добивай и денай, торгуйся хп, когда оппонент тянется за крипом." },
  mid_idle: { label: "Простой в мидгейме", fmt: (v) => pct(v), tip: "Между волнами — лагерь, стак или пуш. Не стой на месте и не бегай без цели." },
  farm_gpm: { label: "Фарм в минуту", fmt: (v) => String(Math.round(v)), tip: "Строй маршрут: волна → лагерь → волна, без пустых переходов через карту." },
};

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
              ? "Возьмём 3 самых слабых показателя из последних 30 игр, поставим реалистичные цели и будем отслеживать каждую новую игру."
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
  const d = GOALS[g.key] ?? { label: g.key, fmt: (v: number) => v.toFixed(1), tip: "" };
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
      {d.tip && <div className="note" style={{ marginTop: 8 }}>{d.tip}</div>}
    </div>
  );
}
