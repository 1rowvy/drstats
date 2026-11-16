import { useEffect, useMemo, useState } from "react";
import type { Dashboard, Filter, Habit } from "../api";
import { clock, METRICS, pct, plural, regionName, roleName, upperFirst } from "../format";
import { useNames } from "../names";
import { HeroBadge, initials } from "../components/HeroBadge";
import { MapCanvas } from "../components/MapCanvas";
import { HabitsCard } from "../components/Habits";
import { LaneStatsCard } from "../components/LaneStats";
import { FlowSection } from "../components/Flow";
import { FarmCard } from "../components/Farm";
import { AvoidDeathsCard } from "../components/AvoidDeaths";
import { DayParts, useDashboard } from "./Dashboard";

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
  const filter: Filter = { hero_id: heroID, days: 0, last: 0, result: "", mode: 0, lane: "", source: "" };
  const { d, error } = useDashboard(filter, tick);
  const [recentN, setRecentN] = useState(10);

  // "Герои" without a hero: open the most played one.
  useEffect(() => {
    if (heroID === 0 && d?.heroes?.length) onHero(d.heroes[0].hero_id);
  }, [heroID, d, onHero]);

  if (error) return <div className="notice bad">{error}</div>;
  if (!d) return <div className="dim">Загрузка…</div>;
  if (d.unknown) return <div className="notice">Укажи свой Steam ID на странице «Импорт» — страницы героев строятся по твоим матчам.</div>;
  if (heroID === 0) return <div className="card empty">Разобранных матчей с твоим участием пока нет.</div>;

  const heroes = d.heroes ?? [];
  const npc = names.raw.hero_by_id?.[heroID] ?? heroes.find((h) => h.hero_id === heroID)?.hero_name ?? "";
  const heroName = npc ? names.hero(npc) : `Герой #${heroID}`;
  const lane = (d.by_lane ?? [])[0]?.key ?? "";
  const team = d.recent?.[0]?.team ?? 2;
  const lh10 = (d.averages ?? []).find((a) => a.key === "lh10");
  const role = roleName(lane, team, lh10?.n ? lh10.avg : undefined);
  const kda = (d.averages ?? []).find((a) => a.key === "kda");

  return (
    <div>
      <div className="hero-tabs">
        {heroes.slice(0, 8).map((h) => (
          <button key={h.hero_id} className={h.hero_id === heroID ? "on" : ""} onClick={() => onHero(h.hero_id)}>
            <HeroBadge npc={h.hero_name} size={26} /> {names.hero(h.hero_name)} <small>{h.games}</small>
          </button>
        ))}
        {heroes.length > 8 && (
          <select value={heroes.slice(8).some((h) => h.hero_id === heroID) ? heroID : 0} onChange={(e) => Number(e.target.value) && onHero(Number(e.target.value))}>
            <option value={0}>Ещё {heroes.length - 8}…</option>
            {heroes.slice(8).map((h) => <option key={h.hero_id} value={h.hero_id}>{names.hero(h.hero_name)} ({h.games})</option>)}
          </select>
        )}
      </div>

      <div className="card hero-hero">
        <div className="watermark">{initials(heroName)}</div>
        <div style={{ position: "relative" }}>
          <div className="caps">{[role, `${d.games} ${plural(d.games, "матч", "матча", "матчей")}`].filter(Boolean).join(" · ")}</div>
          <h1>{heroName}</h1>
          <div className="dim">{heroSummary(d.habits ?? [])}</div>
        </div>
        <div className="big-nums">
          <div><b className={d.winrate >= 0.5 ? "accent" : "loss"}>{pct(d.winrate)}</b><div className="caps">Винрейт · {d.wins}–{d.games - d.wins}</div></div>
          <div><b>{kda && kda.n ? kda.avg.toFixed(1) : "—"}</b><div className="caps">Средний KDA</div></div>
        </div>
      </div>

      {d.games === 0 ? <div className="card empty" style={{ marginTop: 14 }}>На этом герое разобранных матчей нет.</div> : (
        <>
          <div className="section-head">
            <h2>Последние {recentN} против твоего среднего</h2>
            <select value={recentN} onChange={(e) => setRecentN(Number(e.target.value))}>
              <option value={10}>Последние 10 vs всё время</option>
              <option value={20}>Последние 20 vs всё время</option>
            </select>
          </div>
          <CompareCards d={d} n={recentN} />
          <div className="grid2" style={{ marginTop: 14 }}>
            <HabitsCard habits={d.habits ?? []} empty="Нужно хотя бы 15 матчей на герое." />
            <GpmCard d={d} />
          </div>
          <div style={{ marginTop: 14 }}><LaneStatsCard s={d.lanes} /></div>
          <FlowSection f={d.flow} onMatch={onMatch} />
          <div style={{ marginTop: 14 }}><FarmCard f={d.farm} /></div>
          <div style={{ marginTop: 14 }}><AvoidDeathsCard s={d.avoid} /></div>
          <div className="grid2" style={{ marginTop: 14 }}>
            <div className="card">
              <div className="card-head"><h2>Винрейт по времени суток</h2></div>
              <DayParts hours={d.by_hour ?? []} />
            </div>
            <DeathZonesCard d={d} />
          </div>
          <ItemsCard d={d} />
        </>
      )}
    </div>
  );
}

/** "Фармишь лучше, чем раньше, но чаще умираешь." from the habit deltas. */
function heroSummary(habits: Habit[]): string {
  const better: Record<string, string> = {
    gpm: "фармишь лучше", lh10: "лучше стоишь линию", kp: "чаще в драках", deaths: "реже умираешь", hero_dmg_pm: "больше урона по героям", kda: "KDA вырос",
  };
  const worse: Record<string, string> = {
    gpm: "фармишь хуже", lh10: "проседаешь на линии", kp: "реже в драках", deaths: "чаще умираешь", hero_dmg_pm: "меньше урона по героям", kda: "KDA просел",
    deaths10: "чаще умираешь на линии", wards: "меньше вардов",
  };
  if (!habits.length) return "Сыграй хотя бы 15 матчей — покажу, что меняется.";
  const up = habits.filter((h) => h.change >= 0.1 && better[h.key]).sort((a, b) => b.change - a.change).slice(0, 2).map((h) => better[h.key]);
  const down = habits.filter((h) => h.worse && worse[h.key]).slice(0, 2).map((h) => worse[h.key]);
  if (!up.length && !down.length) return "Играешь стабильно: последние 10 матчей в пределах твоего среднего.";
  if (up.length && down.length) return `${upperFirst(up.join(" и "))}, чем раньше, но ${down.join(" и ")}.`;
  if (up.length) return `${upperFirst(up.join(" и "))}, чем раньше.`;
  return `В последних матчах ${down.join(" и ")}.`;
}

function recentAvg(d: Dashboard, key: string, n: number): number | null {
  const xs = (d.trend ?? []).slice(-n).map((g) => g.values?.[key]).filter((v): v is number => v != null);
  return xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null;
}

function CompareCards({ d, n }: { d: Dashboard; n: number }) {
  return (
    <div className="cmp-grid">
      {CARDS.map(({ key, label }) => {
        const m = METRICS[key];
        const all = (d.averages ?? []).find((a) => a.key === key);
        const r = recentAvg(d, key, n);
        if (!all || all.n === 0 || r == null) {
          return <div className="stat" key={key}><div className="label">{label}</div><div className="stat-value">—</div></div>;
        }
        const delta = r - all.avg;
        const better = LOWER_BETTER.has(key) ? delta < 0 : delta > 0;
        const same = Math.abs(delta) < Math.abs(all.avg) * 0.02;
        return (
          <div className="stat" key={key} style={!same && !better ? { borderColor: "rgba(242,112,95,.25)" } : undefined}>
            <div className="label">{label}</div>
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


function GpmCard({ d }: { d: Dashboard }) {
  const games = (d.trend ?? []).slice(-20);
  const vals = games.map((g) => g.values?.gpm ?? null);
  const nums = vals.filter((v): v is number => v != null);
  if (nums.length === 0) return <div className="card"><h2>GPM</h2><div className="dim">Нет данных.</div></div>;
  const max = Math.max(...nums), min = Math.min(...nums);
  const floor = max === min ? min * 0.5 : Math.max(0, min - (max - min) * 0.6);
  const h = (v: number) => ((v - floor) / Math.max(1, max - floor)) * 100;
  const last10 = vals.slice(-10).filter((v): v is number => v != null);
  const avg10 = last10.reduce((a, b) => a + b, 0) / Math.max(1, last10.length);
  const avgAll = nums.reduce((a, b) => a + b, 0) / nums.length;
  return (
    <div className="card">
      <div className="card-head">
        <h2>GPM, {games.length} {plural(games.length, "матч", "матча", "матчей")}</h2>
        <span><b className="display accent" style={{ fontSize: 22 }}>{Math.round(avg10)}</b> <span className="faint small">ср. последних 10</span></span>
      </div>
      <div className="vbars">
        <div className="avg-line" style={{ bottom: `${h(avgAll)}%` }} />
        {vals.map((v, i) => (
          <div key={i} title={v == null ? "—" : String(Math.round(v))}
            style={{ height: `${v == null ? 0 : h(v)}%`, background: i >= vals.length - 10 ? "var(--accent)" : "#2c313c" }} />
        ))}
      </div>
      <div className="chart-foot"><span>{games.length} игр назад</span><span>пунктир — {Math.round(avgAll)}, твоё среднее</span><span>сейчас</span></div>
    </div>
  );
}

function DeathZonesCard({ d }: { d: Dashboard }) {
  const deaths = d.deaths ?? [];
  const top = useMemo(() => {
    const m = new Map<string, number>();
    for (const p of deaths) if (p.region) m.set(p.region, (m.get(p.region) ?? 0) + 1);
    return [...m.entries()].sort((a, b) => b[1] - a[1]).slice(0, 3);
  }, [deaths]);
  return (
    <div className="card">
      <div className="card-head"><h2>Где ты умираешь</h2><span className="meta">{deaths.length} {plural(deaths.length, "смерть", "смерти", "смертей")}</span></div>
      {deaths.length === 0 ? <div className="dim">Смертей с позицией нет.</div> : (
        <div style={{ display: "flex", gap: 18, alignItems: "center" }}>
          <MapCanvas world={d.world} size={230} points={[]} heat={deaths.map((p) => [p.x, p.y])} />
          <div className="zones">
            {top.map(([k, n], i) => (
              <div key={k}><b className={i === 0 ? "top" : ""}>{Math.round((n / deaths.length) * 100)}%</b><small>{regionName(k)}</small></div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function ItemsCard({ d }: { d: Dashboard }) {
  const names = useNames();
  const heroGames = d.games;
  const items = (d.hero?.items ?? [])
    .filter((it) => !CONSUMABLE.test(it.item) && (names.raw.items?.[it.item]?.cost ?? 0) >= 1800 && it.n >= Math.max(2, heroGames * 0.15))
    .slice(0, 4);
  return (
    <div className="card" style={{ marginTop: 14 }}>
      <div className="card-head"><h2>Средние тайминги ключевых предметов</h2><span className="meta">первая покупка</span></div>
      {items.length === 0 ? <div className="dim">Мало данных.</div> : (
        <div className="item-grid">
          {items.map((it) => (
            <div className="tile" key={it.item}>
              <b>{names.inflictor("item_" + it.item)}</b>
              <div className="val">{clock(it.avg)}</div>
              <small>лучший <span className="accent">{clock(it.min)}</span> · собран в {it.n} из {heroGames}</small>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
