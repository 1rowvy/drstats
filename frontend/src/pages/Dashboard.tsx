import { useEffect, useMemo, useState } from "react";
import { Api, Dashboard, Filter, Habit } from "../api";
import { date, duration, gameMode, LANES, laneName, METRICS, pct, WEEKDAYS } from "../format";
import { useNames } from "../names";
import { HeroName } from "../components/HeroBadge";
import { BarChart, TrendChart } from "../components/Chart";
import { MapCanvas } from "../components/MapCanvas";

const EMPTY_FILTER: Filter = { hero_id: 0, days: 0, last: 0, result: "", mode: 0, lane: "" };

/** Loads the dashboard for a filter; reloads when matches change (tick). */
export function useDashboard(filter: Filter, tick: number) {
  const [d, setD] = useState<Dashboard | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    Api.Dashboard(filter)
      .then((v) => { setD(v); setError(""); })
      .catch((e) => setError(String(e?.message ?? e)));
  }, [JSON.stringify(filter), tick]);
  return { d, error };
}

export function RecentTable({ d, onMatch }: { d: Dashboard; onMatch: (id: number) => void }) {
  return (
    <div className="card">
      <h2>Последние матчи</h2>
      <table className="table">
        <thead><tr><th>Герой</th><th>Результат</th><th>K / D / A</th><th>Линия</th><th>Длительность</th><th>Дата</th></tr></thead>
        <tbody>
          {(d.recent ?? []).map((g) => (
            <tr key={g.match_id} className="clickable" onClick={() => onMatch(g.match_id)}>
              <td><HeroName npc={g.hero_name} /></td>
              <td>{g.win ? <span className="win">Победа</span> : <span className="loss">Поражение</span>}</td>
              <td>{g.kills} / <span className="loss">{g.deaths}</span> / {g.assists}</td>
              <td className="dim">{laneName(g.lane, g.team)}</td>
              <td>{duration(g.duration)}</td>
              <td className="dim">{date(g.end_time)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function DashboardPage({ tick, onMatch, onHero }: {
  tick: number; onMatch: (id: number) => void; onHero: (heroID: number) => void;
}) {
  const [filter, setFilter] = useState<Filter>(EMPTY_FILTER);
  const { d, error } = useDashboard(filter, tick);
  if (error) return <div className="notice bad">{error}</div>;
  if (!d) return <div className="dim">Загрузка…</div>;
  return (
    <div>
      <div className="page-head"><h1>Дашборд</h1></div>
      <Filters d={d} filter={filter} onChange={setFilter} />
      <Body d={d} onMatch={onMatch} onHero={onHero} />
    </div>
  );
}

export function Filters({ d, filter, onChange, hideHero }: {
  d: Dashboard; filter: Filter; onChange: (f: Filter) => void; hideHero?: boolean;
}) {
  const names = useNames();
  const set = <K extends keyof Filter>(k: K, v: Filter[K]) => onChange({ ...filter, [k]: v });
  return (
    <div className="filters">
      {!hideHero && (
        <select value={filter.hero_id} onChange={(e) => set("hero_id", Number(e.target.value))}>
          <option value={0}>Все герои</option>
          {(d.heroes ?? []).map((h) => <option key={h.hero_id} value={h.hero_id}>{names.hero(h.hero_name)} ({h.games})</option>)}
        </select>
      )}
      <select value={filter.days} onChange={(e) => set("days", Number(e.target.value))}>
        <option value={0}>За всё время</option>
        <option value={7}>7 дней</option>
        <option value={30}>30 дней</option>
        <option value={90}>90 дней</option>
        <option value={365}>Год</option>
      </select>
      <select value={filter.last} onChange={(e) => set("last", Number(e.target.value))}>
        <option value={0}>Все матчи</option>
        <option value={20}>Последние 20</option>
        <option value={50}>Последние 50</option>
        <option value={100}>Последние 100</option>
      </select>
      <div className="seg">
        {[["", "Все"], ["win", "Победы"], ["loss", "Поражения"]].map(([k, l]) => (
          <button key={k} className={filter.result === k ? "on" : ""} onClick={() => set("result", k)}>{l}</button>
        ))}
      </div>
      <select value={filter.mode} onChange={(e) => set("mode", Number(e.target.value))}>
        <option value={0}>Все режимы</option>
        {(d.modes ?? []).map((m) => <option key={m} value={m}>{gameMode(m, 0)}</option>)}
      </select>
      <select value={filter.lane} onChange={(e) => set("lane", e.target.value)}>
        <option value="">Все линии</option>
        {["top", "mid", "bot", "jungle", "roam"].map((l) => <option key={l} value={l}>{LANES[l]}</option>)}
      </select>
      {JSON.stringify(filter) !== JSON.stringify({ ...EMPTY_FILTER, hero_id: filter.hero_id }) && (
        <button className="link" onClick={() => onChange({ ...EMPTY_FILTER, hero_id: hideHero ? filter.hero_id : 0 })}>сбросить</button>
      )}
    </div>
  );
}

function Body({ d, onMatch, onHero }: { d: Dashboard; onMatch: (id: number) => void; onHero: (heroID: number) => void }) {
  if (d.unknown) return <div className="notice">Укажи свой Steam ID в настройках — агрегаты считаются по твоим матчам.</div>;
  if (d.total === 0) return <div className="empty">Разобранных матчей с твоим участием пока нет.</div>;
  if (d.games === 0) return <div className="empty">Под фильтр не попал ни один матч.</div>;
  const byHero = (d.by_hero ?? []).slice(0, 25);
  const heroID = (npc: string) => (d.heroes ?? []).find((h) => h.hero_name === npc)?.hero_id ?? 0;
  return (
    <>
      <Summary d={d} />
      <HabitsCard habits={d.habits ?? []} />
      <TrendCard d={d} />
      <div className="grid2">
        <div className="card">
          <BarChart title="Винрейт по дням недели" fmt={(v) => pct(v)} max={1} height={220}
            bars={(d.by_weekday ?? []).map((b) => wrBar(WEEKDAYS[Number(b.key)], b))} />
        </div>
        <div className="card">
          <BarChart title="Винрейт по часу начала" fmt={(v) => pct(v)} max={1} height={220}
            bars={(d.by_hour ?? []).map((b) => wrBar(b.key, b))} />
        </div>
      </div>
      <div className="grid2">
        <div className="card list-card" style={{ maxHeight: 520 }}>
          <h2>Герои</h2>
          <table className="table">
            <thead><tr><th>Герой</th><th>Матчи</th><th>Винрейт</th><th>KDA</th><th>GPM</th></tr></thead>
            <tbody>
              {byHero.map((b) => (
                <tr key={b.key} className="clickable" onClick={() => onHero(heroID(b.key))}>
                  <td><HeroName npc={b.key} /></td>
                  <td>{b.games}</td>
                  <td><Winrate b={b} /></td>
                  <td>{b.kda.toFixed(2)}</td>
                  <td>{Math.round(b.gpm)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="card">
          <h2>Линии</h2>
          <table className="table">
            <thead><tr><th>Линия</th><th>Матчи</th><th>Винрейт</th><th>KDA</th><th>GPM</th></tr></thead>
            <tbody>
              {(d.by_lane ?? []).map((b) => (
                <tr key={b.key}><td>{LANES[b.key] ?? b.key}</td><td>{b.games}</td><td><Winrate b={b} /></td><td>{b.kda.toFixed(2)}</td><td>{Math.round(b.gpm)}</td></tr>
              ))}
            </tbody>
          </table>
          <div className="dim small">Линия определяется по позициям на 1–10 минуте.</div>
        </div>
      </div>
      <DeathMap d={d} />
      <RecentTable d={d} onMatch={onMatch} />
    </>
  );
}

/** Winrate bar; played buckets with 0% still get a visible stub. */
function wrBar(label: string, b: { games: number; wins: number; winrate: number }) {
  return {
    label, value: b.games ? Math.max(b.winrate, 0.02) : 0, color: wrColor(b),
    text: b.games ? pct(b.winrate) : "—", tip: `${b.wins} из ${b.games}`,
  };
}

function wrColor(b: { games: number; winrate: number }) {
  if (b.games === 0) return "#3a4150";
  return b.winrate >= 0.5 ? "#5fd17a" : "#ff6b5b";
}

function Winrate({ b }: { b: { winrate: number; wins: number; games: number } }) {
  return <span className={b.winrate >= 0.5 ? "win" : "loss"} title={`${b.wins} из ${b.games}`}>{pct(b.winrate)}</span>;
}

export function Summary({ d }: { d: Dashboard }) {
  const avg = (k: string) => (d.averages ?? []).find((a) => a.key === k);
  const show = ["kda", "gpm", "xpm", "lh10", "deaths10", "kp", "hero_dmg_pm", "wards"];
  return (
    <div className="stat-row">
      <div className="stat">
        <div className="stat-label">Матчей</div>
        <div className="stat-value">{d.games}</div>
        <div className="stat-sub">{d.wins} побед · <span className={d.winrate >= 0.5 ? "win" : "loss"}>{pct(d.winrate, 1)}</span></div>
      </div>
      {show.map((k) => {
        const a = avg(k);
        if (!a || a.n === 0) return null;
        return (
          <div className="stat" key={k}>
            <div className="stat-label">{METRICS[k].label}</div>
            <div className="stat-value">{METRICS[k].fmt(a.avg)}</div>
            <div className="stat-sub">в среднем</div>
          </div>
        );
      })}
    </div>
  );
}

export function HabitsCard({ habits }: { habits: Habit[] }) {
  const worse = habits.filter((h) => h.worse);
  const better = habits.filter((h) => h.change >= 0.1);
  return (
    <div className="card">
      <h2>Худшие привычки <span className="dim small">последние 10 матчей против предыдущих</span></h2>
      {habits.length === 0 ? (
        <div className="dim">Нужно хотя бы 15 матчей под фильтром.</div>
      ) : worse.length === 0 ? (
        <div className="dim">Последние 10 матчей не хуже среднего — так держать.</div>
      ) : (
        <div className="habits">
          {worse.map((h) => <HabitChip key={h.key} h={h} />)}
        </div>
      )}
      {better.length > 0 && (
        <div className="habits" style={{ marginTop: 10 }}>
          <span className="dim small">Стало лучше:</span>
          {better.map((h) => <HabitChip key={h.key} h={h} />)}
        </div>
      )}
    </div>
  );
}

function HabitChip({ h }: { h: Habit }) {
  const m = METRICS[h.key];
  return (
    <div className={`habit ${h.worse ? "bad" : "good"}`}>
      <b>{m?.label ?? h.key}</b>
      <span>{m ? m.fmt(h.recent) : h.recent.toFixed(1)} <span className="dim">vs {m ? m.fmt(h.before) : h.before.toFixed(1)}</span></span>
      <span className={h.worse ? "loss" : "win"}>{h.change > 0 ? "+" : ""}{pct(h.change)}</span>
    </div>
  );
}

export function TrendCard({ d }: { d: Dashboard }) {
  const [metric, setMetric] = useState("kda");
  const trend = d.trend ?? [];
  const m = METRICS[metric];
  const data = useMemo(() => ({
    labels: trend.map((g, i) => String(i + 1)),
    values: trend.map((g) => g.values?.[metric] ?? null),
    wins: trend.map((g) => g.win),
  }), [trend, metric]);
  if (trend.length < 2) return null;
  return (
    <div className="card">
      <div className="page-head" style={{ marginBottom: 0 }}>
        <h2 style={{ margin: 0 }}>Тренд за последние {trend.length} матчей</h2>
        <select value={metric} onChange={(e) => setMetric(e.target.value)}>
          {Object.entries(METRICS).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
        </select>
      </div>
      <TrendChart title="" labels={data.labels} values={data.values} wins={data.wins} fmt={m.fmt} />
    </div>
  );
}

export function DeathMap({ d, title = "Где я умираю" }: { d: Dashboard; title?: string }) {
  const [mode, setMode] = useState<"heat" | "points">("heat");
  const deaths = d.deaths ?? [];
  if (deaths.length === 0) return null;
  return (
    <div className="card">
      <div className="page-head">
        <h2 style={{ margin: 0 }}>{title} <span className="dim small">{deaths.length} смертей</span></h2>
        <div className="seg">
          <button className={mode === "heat" ? "on" : ""} onClick={() => setMode("heat")}>Тепловая карта</button>
          <button className={mode === "points" ? "on" : ""} onClick={() => setMode("points")}>Точки</button>
        </div>
      </div>
      <MapCanvas world={d.world} size={560}
        heat={mode === "heat" ? deaths.map((p) => [p.x, p.y]) : undefined}
        points={mode === "points" ? deaths.map((p) => ({ x: p.x, y: p.y, color: p.t < 600 ? "#f2c14e" : "#ff6b5b" })) : []} />
      {mode === "points" && <div className="legend"><span><span className="dot" style={{ background: "#f2c14e" }} /> до 10:00</span><span><span className="dot" style={{ background: "#ff6b5b" }} /> позже</span></div>}
    </div>
  );
}
