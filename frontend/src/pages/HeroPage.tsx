import { useEffect, useMemo, useState } from "react";
import type { Bucket, Dashboard, Filter, Habit } from "../api";
import { clock, LANES, METRICS, pct, plural, regionName } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "../components/HeroBadge";
import { Icon } from "../components/Icon";
import { MapCanvas } from "../components/MapCanvas";
import { RecentTable, useDashboard } from "./Dashboard";

const CONSUMABLE = /^(tango|flask|clarity|ward_|tpscroll|smoke_of_deceit|dust|enchanted_mango|faerie_fire|blood_grenade|branches|tome_of_knowledge)/;
const LOWER_BETTER = new Set(["deaths", "deaths10", "gold_lost"]);
const CARDS: { key: string; label: string }[] = [
  { key: "lh10", label: "ЛХ на 10 мин" },
  { key: "gpm", label: "GPM" },
  { key: "deaths", label: "Смертей за игру" },
  { key: "kp", label: "Участие в убийствах" },
  { key: "hero_dmg_pm", label: "Урон героям / мин" },
];

export function HeroPage({ heroID, tick, onHero, onMatch }: {
  heroID: number; tick: number; onHero: (id: number) => void; onMatch: (id: number) => void;
}) {
  const names = useNames();
  const filter: Filter = { hero_id: heroID, days: 0, last: 0, result: "", mode: 0, lane: "" };
  const { d, error } = useDashboard(filter, tick);
  const [recentN, setRecentN] = useState(10);

  // "Герои" without a hero: open the most played one.
  useEffect(() => {
    if (heroID === 0 && d?.heroes?.length) onHero(d.heroes[0].hero_id);
  }, [heroID, d, onHero]);

  if (error) return <div className="notice bad">{error}</div>;
  if (!d) return <div className="dim">Загрузка…</div>;
  if (d.unknown) return <div className="notice">Укажи свой Steam ID в настройках — страницы героев строятся по твоим матчам.</div>;
  if (heroID === 0) return <div className="card empty">Разобранных матчей с твоим участием пока нет.</div>;

  const heroes = d.heroes ?? [];
  const npc = names.raw.hero_by_id?.[heroID] ?? heroes.find((h) => h.hero_id === heroID)?.hero_name ?? "";
  const heroName = npc ? names.hero(npc) : `Герой #${heroID}`;
  const lane = (d.by_lane ?? [])[0]?.key;

  return (
    <div>
      <div className="hero-tabs">
        {heroes.slice(0, 8).map((h) => (
          <button key={h.hero_id} className={h.hero_id === heroID ? "on" : ""} onClick={() => onHero(h.hero_id)}>
            <HeroBadge npc={h.hero_name} size={22} /> {names.hero(h.hero_name)} <small>{h.games}</small>
          </button>
        ))}
        {heroes.length > 8 && (
          <select value={heroes.slice(8).some((h) => h.hero_id === heroID) ? heroID : 0} onChange={(e) => Number(e.target.value) && onHero(Number(e.target.value))}>
            <option value={0}>Ещё {heroes.length - 8}…</option>
            {heroes.slice(8).map((h) => <option key={h.hero_id} value={h.hero_id}>{names.hero(h.hero_name)} ({h.games})</option>)}
          </select>
        )}
      </div>

      <div className="page-head">
        <div className="hero-name" style={{ gap: 16 }}>
          {npc && <HeroBadge npc={npc} size={60} />}
          <div>
            <h1>{heroName}</h1>
            <div className="sub">
              {d.games} {plural(d.games, "матч", "матча", "матчей")} · винрейт {pct(d.winrate)} ({d.wins}–{d.games - d.wins})
              {lane && LANES[lane] && ` · ${LANES[lane].toLowerCase()}`}
            </div>
          </div>
        </div>
        <label className="field">
          <span>Сравнивать</span>
          <select value={recentN} onChange={(e) => setRecentN(Number(e.target.value))}>
            <option value={10}>Последние 10 vs всё время</option>
            <option value={20}>Последние 20 vs всё время</option>
          </select>
        </label>
      </div>

      {d.games === 0 ? <div className="card empty">На этом герое разобранных матчей нет.</div> : (
        <>
          <CompareCards d={d} n={recentN} />
          <div className="grid2">
            <HabitsCard habits={d.habits ?? []} hero={heroName} />
            <GpmCard d={d} />
          </div>
          <div className="grid2">
            <DayTimeCard hours={d.by_hour ?? []} />
            <DeathZonesCard d={d} />
          </div>
          <ItemsCard d={d} />
          <RecentTable d={d} onMatch={onMatch} />
        </>
      )}
    </div>
  );
}

function recentAvg(d: Dashboard, key: string, n: number): number | null {
  const xs = (d.trend ?? []).slice(-n).map((g) => g.values?.[key]).filter((v): v is number => v != null);
  return xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null;
}

function CompareCards({ d, n }: { d: Dashboard; n: number }) {
  return (
    <div className="stat-grid">
      {CARDS.map(({ key, label }) => {
        const m = METRICS[key];
        const all = (d.averages ?? []).find((a) => a.key === key);
        const r = recentAvg(d, key, n);
        if (!all || all.n === 0 || r == null) {
          return <div className="stat" key={key}><div className="stat-label">{label}</div><div className="stat-value">—</div></div>;
        }
        const delta = r - all.avg;
        const better = LOWER_BETTER.has(key) ? delta < 0 : delta > 0;
        const same = Math.abs(delta) < Math.abs(all.avg) * 0.02;
        return (
          <div className="stat" key={key}>
            <div className="stat-label">{label}</div>
            <div className="stat-value">{m.fmt(r)}</div>
            <div className="stat-sub">
              ср. {m.fmt(all.avg)}{" "}
              {same ? <span className="faint">≈ так же</span>
                : better ? <span className="up">{delta > 0 ? "↑" : "↓"} лучше</span>
                : <span className="down">{delta > 0 ? "↑" : "↓"} хуже</span>}
            </div>
          </div>
        );
      })}
    </div>
  );
}

const HABIT_TEXT: Record<string, (r: string, b: string) => string> = {
  lh10: (r, b) => `Добиваешь меньше крипов на линии: ${r} к 10:00 вместо обычных ${b}.`,
  dn10: (r, b) => `Меньше денаев на линии: ${r} против ${b}.`,
  nw10: (r, b) => `Выходишь с линии беднее: ${r} нетворса к 10:00 против ${b}.`,
  deaths: (r, b) => `Умираешь чаще: ${r} смертей за игру вместо ${b}.`,
  deaths10: (r, b) => `Чаще умираешь до 10:00: ${r} против ${b}. Линия проседает с самого начала.`,
  gpm: (r, b) => `Фармишь медленнее: ${r} GPM против обычных ${b}.`,
  xpm: (r, b) => `Медленнее качаешься: ${r} XPM против ${b}.`,
  kp: (r, b) => `Реже участвуешь в драках: ${r} против ${b}.`,
  kda: (r, b) => `KDA просел: ${r} против ${b}.`,
  kills: (r, b) => `Меньше убийств: ${r} за игру против ${b}.`,
  assists: (r, b) => `Меньше помощи команде: ${r} ассистов против ${b}.`,
  hero_dmg_pm: (r, b) => `Меньше урона по героям: ${r} в минуту против ${b}.`,
  tower_dmg: (r, b) => `Меньше давишь строения: ${r} урона против ${b}.`,
  gold_lost: (r, b) => `Больше теряешь на смертях: ${r} золота против ${b}.`,
  wards: (r, b) => `Меньше вардов: ${r} за игру против ${b}.`,
  stacks: (r, b) => `Реже стакаешь: ${r} против ${b}.`,
};

function HabitsCard({ habits, hero }: { habits: Habit[]; hero: string }) {
  const worse = habits.filter((h) => h.worse).slice(0, 3);
  const better = habits.filter((h) => h.change >= 0.1).slice(0, 3 - Math.min(3, worse.length));
  return (
    <div className="card">
      <div className="card-head"><h2 style={{ display: "flex", gap: 8, alignItems: "center" }}><Icon name="trend" className="loss" /> Худшие привычки</h2></div>
      <p className="lead">Где последние 10 матчей хуже твоего же среднего на {hero}</p>
      {habits.length === 0 && <div className="dim">Нужно хотя бы 15 матчей на герое.</div>}
      {habits.length > 0 && worse.length === 0 && <div className="dim" style={{ marginTop: 10 }}>Последние 10 матчей не хуже среднего — так держать.</div>}
      {worse.map((h) => <HabitItem key={h.key} h={h} />)}
      {better.map((h) => <HabitItem key={h.key} h={h} />)}
    </div>
  );
}

function HabitItem({ h }: { h: Habit }) {
  const m = METRICS[h.key];
  const f = m ? m.fmt : (v: number) => v.toFixed(1);
  const text = h.worse ? (HABIT_TEXT[h.key] ?? ((r, b) => `${r} против ${b}.`))(f(h.recent), f(h.before)) : `Стало лучше: ${f(h.recent)} против ${f(h.before)}.`;
  return (
    <div className={`habit-card ${h.worse ? "" : "good"}`}>
      <div className="hc-head"><span>{m?.label ?? h.key}</span><span className="mono">{f(h.recent)} vs {f(h.before)}</span></div>
      <p>{text}</p>
    </div>
  );
}

function GpmCard({ d }: { d: Dashboard }) {
  const games = (d.trend ?? []).slice(-20);
  const vals = games.map((g) => g.values?.gpm ?? null);
  const nums = vals.filter((v): v is number => v != null);
  if (nums.length === 0) return <div className="card"><h2>GPM</h2><div className="dim">Нет данных.</div></div>;
  const max = Math.max(...nums), min = Math.min(...nums);
  const floor = Math.max(0, min - (max - min) * 0.6);
  const last10 = vals.slice(-10).filter((v): v is number => v != null);
  return (
    <div className="card">
      <div className="card-head">
        <h2>GPM, последние {games.length} {plural(games.length, "матч", "матча", "матчей")}</h2>
        <span className="mini-legend">{games.length > 10 && <span><i style={{ background: "#3a404c" }} />{games.length}–11</span>}<span><i style={{ background: "var(--accent)" }} />последние 10</span></span>
      </div>
      <div className="vbars">
        {vals.map((v, i) => (
          <div key={i} title={v == null ? "—" : String(Math.round(v))}
            style={{ height: `${v == null ? 0 : ((v - floor) / (max - floor)) * 100}%`, background: i >= vals.length - 10 ? "var(--accent)" : "#3a404c" }} />
        ))}
      </div>
      <div className="chart-foot" style={{ marginTop: 8 }}><span>{games.length} игр назад</span><span>Последний матч</span></div>
      <div className="chart-stats">
        <span>Мин: <b>{Math.round(min)}</b></span>
        <span>Макс: <b>{Math.round(max)}</b></span>
        <span>Ср. последних 10: <b>{Math.round(last10.reduce((a, b) => a + b, 0) / Math.max(1, last10.length))}</b></span>
      </div>
    </div>
  );
}

const DAY_PARTS = [
  { label: "Утро, 6–12", from: 6, to: 12, night: "Утром" },
  { label: "День, 12–18", from: 12, to: 18, night: "Днём" },
  { label: "Вечер, 18–24", from: 18, to: 24, night: "Вечером" },
  { label: "Ночь, 0–6", from: 0, to: 6, night: "Ночью" },
];

function DayTimeCard({ hours }: { hours: Bucket[] }) {
  const parts = DAY_PARTS.map((p) => {
    const bs = hours.filter((b) => Number(b.key) >= p.from && Number(b.key) < p.to);
    const games = bs.reduce((a, b) => a + b.games, 0);
    const wins = bs.reduce((a, b) => a + b.wins, 0);
    return { ...p, games, wins, wr: games ? wins / games : 0 };
  });
  // Loss-rate comparison of the worst part (≥3 games) against everything else.
  const played = parts.filter((p) => p.games >= 3);
  const worst = played.length > 1 ? played.reduce((a, b) => (b.wr < a.wr ? b : a)) : null;
  let note = "";
  if (worst) {
    const rest = parts.filter((p) => p !== worst);
    const rg = rest.reduce((a, p) => a + p.games, 0), rw = rest.reduce((a, p) => a + p.wins, 0);
    const lossW = 1 - worst.wr, lossR = rg ? 1 - rw / rg : 0;
    if (lossR > 0 && lossW / lossR >= 1.3) {
      const k = lossW / lossR;
      note = `${worst.night} ты проигрываешь в ${k >= 1.95 ? `${Math.round(k)} раза` : `${k.toFixed(1).replace(".", ",")} раза`} чаще.${worst.from === 0 ? " Иди спать." : ""}`;
    }
  }
  return (
    <div className="card">
      <div className="card-head"><h2 style={{ fontSize: 18 }}>Винрейт по времени суток</h2></div>
      {parts.map((p) => (
        <div className="meter-row" key={p.label}>
          <div className="meter-label">
            <span style={{ fontSize: 15 }}>{p.label}</span>
            <span className="mono">{p.games} {plural(p.games, "игра", "игры", "игр")}<b>{p.games ? pct(p.wr) : "—"}</b></span>
          </div>
          <div className={`bar ${p.games && p.wr < 0.45 ? "bad" : ""}`}><div style={{ width: `${p.wr * 100}%` }} /></div>
        </div>
      ))}
      {note && <div className="big-note" style={{ fontSize: 15 }}>{note}</div>}
    </div>
  );
}

const lowerFirst = (s: string) => s.charAt(0).toLowerCase() + s.slice(1);

function DeathZonesCard({ d }: { d: Dashboard }) {
  const deaths = d.deaths ?? [];
  const top = useMemo(() => {
    const m = new Map<string, number>();
    for (const p of deaths) if (p.region) m.set(p.region, (m.get(p.region) ?? 0) + 1);
    return [...m.entries()].sort((a, b) => b[1] - a[1]).slice(0, 3);
  }, [deaths]);
  return (
    <div className="card">
      <div className="card-head"><h2>Где ты умираешь</h2><span className="meta">{deaths.length} {plural(deaths.length, "смерть", "смерти", "смертей")}, все матчи</span></div>
      {deaths.length === 0 ? <div className="dim">Смертей с позицией нет.</div> : (
        <>
          <div style={{ display: "flex", justifyContent: "center" }}>
            <MapCanvas world={d.world} size={300} points={[]} heat={deaths.map((p) => [p.x, p.y])} />
          </div>
          <div className="big-note" style={{ fontSize: 14 }}>
            Топ-зоны: {top.map(([k, n], i) => `${i ? lowerFirst(regionName(k)) : regionName(k)} — ${Math.round((n / deaths.length) * 100)}%`).join(", ")}
          </div>
        </>
      )}
    </div>
  );
}

function ItemsCard({ d }: { d: Dashboard }) {
  const names = useNames();
  const heroGames = d.games;
  const items = (d.hero?.items ?? [])
    .filter((it) => !CONSUMABLE.test(it.item) && (names.raw.items?.[it.item]?.cost ?? 0) >= 1800 && it.n >= Math.max(2, heroGames * 0.15))
    .slice(0, 8);
  return (
    <div className="card">
      <div className="card-head"><h2 style={{ fontSize: 18 }}>Средние тайминги ключевых предметов</h2><span className="meta">первая покупка</span></div>
      {items.length === 0 ? <div className="dim">Мало данных.</div> : (
        <div className="item-grid">
          {items.map((it) => (
            <div className="tile" key={it.item}>
              <b>{names.inflictor("item_" + it.item)}</b>
              <div className="stat-value">{clock(it.avg)}</div>
              <div className="stat-sub">лучший {clock(it.min)} · собран в {it.n} из {heroGames} игр</div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
