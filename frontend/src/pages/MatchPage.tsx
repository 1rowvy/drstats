import { useEffect, useMemo, useState } from "react";
import { Api, MatchView, PlayerView } from "../api";
import { clock, duration, GOLD_SOURCES, laneName, modeShort, num, pct, RADIANT, DIRE, regionName, relDate, roleName, teamName, thousands } from "../format";
import { useNames } from "../names";
import { HeroBadge, HeroName } from "../components/HeroBadge";
import { Icon } from "../components/Icon";
import { VersusChart } from "../components/Chart";
import { MapCanvas, MapPoint, useHasMapImage } from "../components/MapCanvas";
import { AbilitiesTab, FarmTab, FightsTab, ItemsTab, LaningTab, VisionTab } from "./MatchTabs";

type Tab = "overview" | "laning" | "farm" | "items" | "deaths" | "fights" | "abilities" | "vision";

const TABS: { id: Tab; label: string }[] = [
  { id: "overview", label: "Обзор" },
  { id: "laning", label: "Лайнинг" },
  { id: "farm", label: "Фарм" },
  { id: "items", label: "Предметы" },
  { id: "deaths", label: "Смерти" },
  { id: "fights", label: "Драки" },
  { id: "abilities", label: "Способности" },
  { id: "vision", label: "Карта" },
];

export function MatchPage({ matchID, lh10Goal, onBack, onHero }: {
  matchID: number; lh10Goal: number; idleThreshold?: number; onBack: () => void; onHero?: (heroID: number) => void;
}) {
  const names = useNames();
  const [focus, setFocus] = useState(-1);
  const [view, setView] = useState<MatchView | null>(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<Tab>("overview");

  useEffect(() => {
    Api.Match(matchID, focus)
      .then((v) => { setView(v); setError(""); })
      .catch((e) => setError(String(e?.message ?? e)));
  }, [matchID, focus]);

  const backBtn = <button className="back" onClick={onBack}><Icon name="back" size={14} /> Все матчи</button>;
  if (error) return <div>{backBtn}<div className="notice bad" style={{ marginTop: 14 }}>{error}</div></div>;
  if (!view) return <div className="dim">Загрузка…</div>;

  const players = view.players ?? [];
  const me = players.find((p) => p.player_id === view.focus);
  const opp = players.find((p) => p.player_id === view.opponent);
  const won = me ? me.team === view.meta.winner : null;
  const lh10 = view.laning?.snaps?.find((s) => s.t === 600)?.lh;
  const role = me ? roleName(view.laning.lane, me.team, lh10) : "";
  const sub = [
    modeShort(view.meta.game_mode, view.meta.lobby_type),
    me && (role || opp) ? [role, opp ? `против ${names.hero(opp.hero_name)}` : ""].filter(Boolean).join(" ") : "",
    duration(view.meta.duration),
    relDate(view.meta.end_time),
  ].filter(Boolean).join(" · ");

  return (
    <div>
      {backBtn}
      <div className="match-head">
        {me ? <HeroBadge npc={me.hero_name} size={60} /> : null}
        <div>
          <h1>
            {me ? (onHero && view.is_me
              ? <button className="link title-link" onClick={() => onHero(me.hero_id)} title="Страница героя">{names.hero(me.hero_name)}</button>
              : names.hero(me.hero_name)) : `Матч ${view.meta.match_id}`}
            {won !== null && (won ? <span className="pill win">Победа</span> : <span className="pill loss">Поражение</span>)}
          </h1>
          <div className="sub">{sub}</div>
          {me && !view.is_me && <div className="dim small">Смотришь за другого игрока ({me.name || "без имени"}).</div>}
        </div>
      </div>

      {me && tab === "overview" && <HeadStats view={view} me={me} />}

      <div className="tabs">
        {TABS.map((t) => (
          <button key={t.id} className={tab === t.id ? "on" : ""} onClick={() => setTab(t.id)} disabled={!me && t.id !== "overview"}>{t.label}</button>
        ))}
      </div>

      {tab === "overview" && <Overview view={view} me={me} opp={opp} goal={lh10Goal} onPick={setFocus} />}
      {me && tab === "laning" && <LaningTab view={view} />}
      {me && tab === "farm" && <FarmTab view={view} />}
      {me && tab === "items" && <ItemsTab view={view} />}
      {me && tab === "deaths" && <DeathsTab view={view} />}
      {me && tab === "fights" && <FightsTab view={view} />}
      {me && tab === "abilities" && <AbilitiesTab view={view} />}
      {me && tab === "vision" && <VisionTab view={view} />}
    </div>
  );
}

function HeadStats({ view, me }: { view: MatchView; me: PlayerView }) {
  return (
    <div className="stat-grid">
      <Stat label="K / D / A" value={`${me.kills}/${me.deaths}/${me.assists}`} />
      <Stat label="Нетворс" value={thousands(me.net_worth)} />
      <Stat label="GPM" value={me.gpm} />
      <Stat label="XPM" value={me.xpm} />
      <Stat label="Участие" value={pct(view.fights.kp)} />
      <Stat label="Урон по героям" value={thousands(me.hero_damage)} />
    </div>
  );
}

function Stat({ label, value, sub, tile }: { label: string; value: React.ReactNode; sub?: React.ReactNode; tile?: boolean }) {
  return (
    <div className={tile ? "tile" : "stat"}>
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
    </div>
  );
}

// ---------------------------------------------------------------- overview

function Overview({ view, me, opp, goal, onPick }: {
  view: MatchView; me?: PlayerView; opp?: PlayerView; goal: number; onPick: (pid: number) => void;
}) {
  return (
    <div>
      {view.focus < 0 && (
        <div className="notice">В этом матче нет игрока с твоим Steam ID — кликни по строке состава, чтобы разобрать любого игрока.</div>
      )}
      {me && (
        <>
          <div className="grid2">
            <NetWorthCard view={view} opp={opp} />
            <LaningCard view={view} opp={opp} goal={goal} />
          </div>
          <div className="grid2">
            <GoldCard view={view} />
            <TimingsCard view={view} me={me} />
          </div>
          <div className="grid2">
            <AbilitiesCard view={view} />
            <IdleCard view={view} />
          </div>
        </>
      )}
      <Scoreboard view={view} onPick={onPick} />
    </div>
  );
}

function NetWorthCard({ view, opp }: { view: MatchView; opp?: PlayerView }) {
  const names = useNames();
  const nw = view.farm.net_worth;
  const lastOf = (l: (number | null)[] | null) => {
    for (let i = (l ?? []).length - 1; i >= 0; i--) if (l![i] != null) return l![i]!;
    return null;
  };
  const a = lastOf(nw.focus), b = lastOf(nw.opponent);
  const oppName = opp ? names.hero(opp.hero_name) : "";
  return (
    <div className="card">
      <div className="card-head">
        <h2>Нетворс по времени</h2>
        <span className="legend-inline"><span><i />Ты</span>{opp && <span><i className="dashed" />{oppName}</span>}</span>
      </div>
      <VersusChart t={view.farm.t ?? []} me={nw.focus ?? []} opp={opp ? nw.opponent ?? [] : undefined} oppName={oppName} />
      {a != null && b != null && (
        <div className="big-note">Разница на {duration(view.meta.duration)}: <span className={`mono small ${a >= b ? "win" : "loss"}`}>{num(a - b, true)}</span></div>
      )}
    </div>
  );
}

function LaningCard({ view, opp, goal }: { view: MatchView; opp?: PlayerView; goal: number }) {
  const names = useNames();
  const l = view.laning;
  const at = (t: number) => (l.snaps ?? []).find((s) => s.t === t);
  const s5 = at(300), s10 = at(600);
  const runes = (view.vision.runes ?? []).filter((r) => r.t <= 600).length;
  const oppShort = opp ? names.hero(opp.hero_name).split(" ")[0] : "";
  const diff = (v: number | undefined) => (v == null ? "—" : num(v, true));
  return (
    <div className="card">
      <div className="card-head"><h2 style={{ fontSize: 18 }}>Лайнинг, 0–10 мин</h2><span className="meta">{laneName(l.lane, view.players?.find((p) => p.player_id === view.focus)?.team)}</span></div>
      <div className="meter-row">
        <div className="meter-label"><span className="dim small">ЛХ на 10 минуте</span><span className="mono small"><b style={{ marginLeft: 0 }}>{s10?.lh ?? "—"}</b> / цель {goal}</span></div>
        <div className={`bar ${s10 && s10.lh < goal * 0.7 ? "bad" : ""}`}><div style={{ width: `${Math.min(1, (s10?.lh ?? 0) / goal) * 100}%` }} /></div>
      </div>
      <div className="tiles">
        <Stat tile label="ЛХ на 5 мин" value={s5?.lh ?? "—"} />
        <Stat tile label="Денаи" value={s10?.dn ?? "—"} />
        <Stat tile label="Смерти" value={(l.deaths ?? []).length} />
        <Stat tile label={opp ? `Нетворс vs ${oppShort}` : "Нетворс"} value={opp ? diff(s10?.opp ? s10.nw - s10.opp.nw : undefined) : s10 ? num(s10.nw) : "—"} />
        <Stat tile label={opp ? `Опыт vs ${oppShort}` : "Опыт"} value={opp ? diff(s10?.opp ? s10.xp - s10.opp.xp : undefined) : s10 ? num(s10.xp) : "—"} />
        <Stat tile label="Руны" value={runes} />
      </div>
    </div>
  );
}

export const GOLD_COLORS: Record<string, string> = {
  creep: "var(--accent)", neutral: "var(--blue)", hero: "var(--purple)", building: "var(--teal)",
  passive: "#5b6272", bounty: "#d8c46a", roshan: "#e6a0c4", other: "#3a404c",
};
const GOLD_LABELS: Record<string, string> = { ...GOLD_SOURCES, hero: "Убийства героев", building: "Здания" };

function GoldCard({ view }: { view: MatchView }) {
  const sources = (view.gold.sources ?? []).filter((s) => s.amount > 0).sort((a, b) => b.amount - a.amount);
  const sum = sources.reduce((a, s) => a + s.amount, 0);
  const total = sum || 1;
  return (
    <div className="card">
      <div className="card-head"><h2 style={{ fontSize: 18 }}>Откуда золото</h2>{sum > 0 && <span className="meta mono">всего {num(sum)}</span>}</div>
      {sum === 0 && <div className="dim">Нет данных — перепарси реплей.</div>}
      <div className="stack-bar">
        {sources.map((s) => <div key={s.key} title={GOLD_LABELS[s.key]} style={{ width: `${(s.amount / total) * 100}%`, background: GOLD_COLORS[s.key] }} />)}
      </div>
      <div className="legend-list">
        {sources.map((s) => (
          <div className="li" key={s.key}>
            <span className="sw" style={{ background: GOLD_COLORS[s.key] }} />
            <span>{GOLD_LABELS[s.key] ?? s.key}</span>
            <span className="mono">{num(s.amount)}</span>
            <span className="mono">{Math.round((s.amount / total) * 100)}%</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function TimingsCard({ view, me }: { view: MatchView; me: PlayerView }) {
  const names = useNames();
  const cost = (k: string) => names.raw.items?.[k]?.cost ?? 0;
  const items = (view.items ?? []).filter((it) => it.first && cost(it.item) >= 1800).slice(0, 6);
  return (
    <div className="card">
      <div className="card-head"><h2>Тайминги предметов</h2><span className="meta">vs твоё среднее на {names.hero(me.hero_name)}</span></div>
      {items.length === 0 ? <div className="dim">Дорогих предметов не покупал.</div> : items.map((it, i) => {
        const d = it.diff;
        return (
          <div className="timing-row" key={i}>
            <span>{names.inflictor("item_" + it.item)}</span>
            <span className="t">{clock(it.t)}</span>
            <span className="avg">{it.avg != null ? <>ср.<br />{clock(it.avg)}</> : "—"}</span>
            <span className={`d ${d == null ? "faint" : d <= 0 ? "win" : "loss"}`}>
              {d == null ? "впервые" : `${clock(Math.abs(Math.round(d)))} ${d <= 0 ? "раньше" : "позже"}`}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function AbilitiesCard({ view }: { view: MatchView }) {
  const names = useNames();
  const uses = (view.abilities.uses ?? []).filter((u) => !u.is_item && u.hit_rate != null).slice(0, 8);
  return (
    <div className="card">
      <div className="card-head">
        <h2>Способности</h2>
        <span className="meta"><Icon name="info" size={12} /> попадание ≈ урон или дебафф в течение {view.abilities.hit_window} с после каста</span>
      </div>
      {uses.length === 0 ? <div className="dim">Нет способностей, бьющих по героям.</div> : uses.map((u) => {
        const r = u.hit_rate ?? 0;
        return (
          <div className="meter-row" key={u.ability}>
            <div className="meter-label">
              <span style={{ fontSize: 15 }}>{names.inflictor(u.ability)}</span>
              <span className="mono">{u.hits} из {u.casts}<b>{pct(r)}</b></span>
            </div>
            <div className={`bar ${r < 0.6 ? "bad" : ""}`}><div style={{ width: `${r * 100}%` }} /></div>
          </div>
        );
      })}
    </div>
  );
}

function IdleCard({ view }: { view: MatchView }) {
  const g = view.gold;
  const idle = g.idle ?? [];
  const dur = Math.max(1, view.meta.duration);
  const ticks = [0, 600, 1200, 1800].filter((t) => t < dur - 240);
  return (
    <div className="card">
      <div className="card-head"><h2>Простой без фарма</h2><span className="mono" style={{ fontSize: 18 }}>{duration(g.idle_total)}</span></div>
      <div className="timeline">
        {idle.map((p, i) => (
          <span key={i} title={`${clock(p.from)} – ${clock(p.to)}`}
            style={{ left: `${(Math.max(0, p.from) / dur) * 100}%`, width: `${((p.to - Math.max(0, p.from)) / dur) * 100}%` }} />
        ))}
      </div>
      <div className="timeline-axis">{ticks.map((t) => <span key={t}>{clock(t)}</span>)}<span>{duration(dur)}</span></div>
      {idle.length === 0 ? <div className="dim">Простоев нет — так держать.</div> : (
        <div className="idle-list">
          {idle.slice(0, 6).map((p, i) => (
            <div key={i}>
              <span>{clock(p.from)} – {clock(p.to)}{p.region && <span className="dim"> · {regionName(p.region)}</span>}</span>
              <span className="mono">{duration(p.to - p.from)}</span>
            </div>
          ))}
          {idle.length > 6 && <div className="faint small">и ещё {idle.length - 6}</div>}
        </div>
      )}
      <div className="big-note" style={{ fontSize: 14 }}>Порог: жив, но не фармил и не дрался дольше {g.threshold} с. Меняется в настройках.</div>
    </div>
  );
}

function Scoreboard({ view, onPick }: { view: MatchView; onPick: (pid: number) => void }) {
  const players = view.players ?? [];
  return (
    <>
      {[RADIANT, DIRE].map((team) => (
        <div key={team} className="card">
          <div className="card-head">
            <h2 className={team === RADIANT ? "radiant" : "dire"}>{teamName(team)}</h2>
            {view.meta.winner === team && <span className="pill win">победа</span>}
          </div>
          <table className="table scoreboard">
            <thead>
              <tr>
                <th>Герой</th><th>Игрок</th><th>Ур.</th><th>K</th><th>D</th><th>A</th><th>ЛХ / ДН</th>
                <th>NW</th><th>GPM</th><th>XPM</th><th title="Урон героям">Урон гер.</th><th title="Урон строениям">Урон стр.</th><th>Лечение</th>
              </tr>
            </thead>
            <tbody>
              {players.filter((p) => p.team === team).map((p) => (
                <tr key={p.player_id} className={`clickable ${p.player_id === view.focus ? "focus" : ""}`} onClick={() => onPick(p.player_id)}>
                  <td><HeroName npc={p.hero_name} /> {p.player_id === view.opponent && <span title="лайн-оппонент" className="faint">⚔</span>}</td>
                  <td className="dim">{p.name || "—"}</td>
                  <td className="num">{p.level}</td>
                  <td className="num">{p.kills}</td>
                  <td className="num">{p.deaths}</td>
                  <td className="num">{p.assists}</td>
                  <td className="num">{p.last_hits} / {p.denies}</td>
                  <td className="num gold">{thousands(p.net_worth)}</td>
                  <td className="num">{p.gpm}</td>
                  <td className="num">{p.xpm}</td>
                  <td className="num">{thousands(p.hero_damage)}</td>
                  <td className="num">{thousands(p.tower_damage)}</td>
                  <td className="num">{thousands(p.healing)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}
      <div className="faint small">Кликни по строке, чтобы разобрать матч за другого игрока. ⚔ — лайн-оппонент (по позициям на 1–10 минуте).</div>
    </>
  );
}

// ---------------------------------------------------------------- deaths

function DeathsTab({ view }: { view: MatchView }) {
  const names = useNames();
  const hasBg = useHasMapImage();
  const [layer, setLayer] = useState<"deaths" | "move">("deaths");
  const [hover, setHover] = useState<number | null>(null);
  const players = view.players ?? [];
  const byPID = useMemo(() => new Map(players.map((p) => [p.player_id, p])), [players]);
  const deaths = useMemo(
    () => (view.deaths ?? []).map((d, i) => ({ d, ctx: view.death_ctx?.[i] })).filter(({ d }) => d.victim === view.focus),
    [view],
  );
  const heroOf = (pid: number) => {
    const p = byPID.get(pid);
    return p ? names.hero(p.hero_name) : "";
  };
  const goldLost = deaths.reduce((a, { d }) => a + d.gold_lost, 0);
  const positioned = deaths.filter(({ d }) => d.has_pos);
  const noVision = positioned.filter(({ ctx }) => ctx && !ctx.vision).length;
  const buybacks = deaths.filter(({ d }) => d.buyback).length;

  const points: MapPoint[] = layer === "deaths"
    ? deaths.map(({ d }, i) => ({ d, i })).filter(({ d }) => d.has_pos)
      .map(({ d, i }) => ({ x: d.x, y: d.y, color: "#f8776c", label: String(i + 1), highlight: hover === i }))
    : [];
  const pointToDeath = deaths.map(({ d }, i) => ({ d, i })).filter(({ d }) => d.has_pos).map(({ i }) => i);

  return (
    <div>
      <div className="stat-grid">
        <Stat label="Смертей" value={deaths.length} />
        <Stat label="Потеряно золота" value={num(goldLost)} />
        <Stat label="Без вижна" value={`${noVision} из ${positioned.length}`} />
        <Stat label="Байбеков" value={buybacks} />
      </div>
      <div className="deaths-layout">
        <div className="card map-card">
          <div className="card-head">
            <h2>Карта</h2>
            <div className="seg">
              <button className={layer === "deaths" ? "on" : ""} onClick={() => setLayer("deaths")}>Смерти</button>
              <button className={layer === "move" ? "on" : ""} onClick={() => setLayer("move")}>Передвижение</button>
            </div>
          </div>
          <MapCanvas size={480} points={points} world={view.world}
            heat={layer === "move" ? view.vision.heat ?? [] : undefined}
            onHover={(pi) => setHover(pi === null ? null : pointToDeath[pi])} />
          {hasBg === false && <div className="map-foot">Схема без игровой миникарты. Свою подложку можно указать в настройках.</div>}
        </div>
        <div className="death-list">
          {deaths.length === 0 && <div className="card empty">Ни одной смерти. Красиво.</div>}
          {deaths.map(({ d, ctx }, i) => {
            const killers = [d.killer, ...(d.assists ?? [])].filter((pid, j, a) => pid >= 0 && a.indexOf(pid) === j).map(heroOf).filter(Boolean);
            const killerHero = heroOf(d.killer);
            const blow = d.killer >= 0
              ? `${d.inflictor ? names.inflictor(d.inflictor) : "автоатака"}${killerHero ? ` (${killerHero})` : ""}`
              : names.unit(d.killer_name);
            return (
              <div key={i} className={`death-card ${hover === i ? "hover" : ""}`} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                <div className="dc-head">
                  <span className="dc-num">{i + 1}</span>
                  <span className="dc-time">{clock(d.t)}</span>
                  {ctx?.region && <span className="dc-where">{regionName(ctx.region)}</span>}
                  {d.gold_lost > 0 && <span className="dc-gold">−{num(d.gold_lost)}</span>}
                </div>
                <div className="dc-line"><span>Убили: </span>{killers.length ? killers.join(", ") : names.unit(d.killer_name)}</div>
                <div className="dc-line"><span>Добил: </span>{blow}</div>
                <div className="dc-tags">
                  {d.has_pos && ctx && (ctx.vision
                    ? <span className="tag">Вижн был</span>
                    : <span className="tag warn"><Icon name="eyeOff" size={11} /> Без вижна</span>)}
                  <span className="tag">{d.buyback ? "Байбек" : "Без байбека"}</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
