import { useEffect, useMemo, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Api, AppEvents, DeathCtx, MatchView, PlayerView } from "../api";
import { clock, duration, gradeTone, kfmt, modeShort, num, pct, RADIANT, DIRE, rankName, regionName, relDate, roleName, teamName, thousands } from "../format";
import { useNames } from "../names";
import { HeroBadge, HeroName } from "../components/HeroBadge";
import { Icon } from "../components/Icon";
import { MapCanvas, MapPoint, useHasMapImage } from "../components/MapCanvas";
import { ResultPill, Ring, Stat, TickButton } from "../components/ui";
import { AbilitiesTab, VisionTab } from "./MatchTabs";
import { OverviewTab, summaryLine } from "./match/Overview";
import { FarmTab } from "./match/Farm";
import { LaneTab } from "./match/Lane";
import { FightsTab } from "./match/Fights";

type Tab = "overview" | "lane" | "farm" | "fights" | "deaths" | "abilities" | "vision";

const TABS: { id: Tab; label: string }[] = [
  { id: "overview", label: "Разбор" },
  { id: "lane", label: "Линия" },
  { id: "farm", label: "Фарм" },
  { id: "fights", label: "Драки" },
  { id: "deaths", label: "Смерти" },
  { id: "abilities", label: "Способности" },
  { id: "vision", label: "Вижн" },
];

export function MatchPage({ matchID, lh10Goal, onBack, onHero }: {
  matchID: number; lh10Goal: number; onBack: () => void; onHero?: (heroID: number) => void;
}) {
  const names = useNames();
  const [focus, setFocus] = useState(-1);
  const [view, setView] = useState<MatchView | null>(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<Tab>("overview");
  const [odTick, setOdTick] = useState(0);

  useEffect(() => {
    Api.Match(matchID, focus)
      .then((v) => { setView(v); setError(""); })
      .catch((e) => setError(String(e?.message ?? e)));
  }, [matchID, focus, odTick]);
  // OpenDota extras arrive in the background: reload when they do.
  useEffect(() => Events.On(AppEvents.OpendotaUpdated, (e) => {
    if (e.data?.match_id === matchID) setOdTick((t) => t + 1);
  }), [matchID]);

  const backBtn = <button className="back" onClick={onBack}><Icon name="back" size={13} /> Все матчи</button>;
  if (error) return <div>{backBtn}<div className="notice bad">{error}</div></div>;
  if (!view) return <div className="dim">Загрузка…</div>;

  const players = view.players ?? [];
  const me = players.find((p) => p.player_id === view.focus);
  const opp = players.find((p) => p.player_id === view.opponent);
  const won = me ? me.team === view.meta.winner : null;
  const lh10 = view.laning?.snaps?.find((s) => s.t === 600)?.lh;
  const role = me ? roleName(view.laning.lane, me.team, lh10, me.od?.position) : "";
  const heroTitle = me ? (onHero && view.is_me
    ? <button className="link title-link" onClick={() => onHero(me.hero_id)} title="Страница героя">{names.hero(me.hero_name)}</button>
    : names.hero(me.hero_name)) : `Матч ${view.meta.match_id}`;
  const metaBits = [
    modeShort(view.meta.game_mode, view.meta.lobby_type), relDate(view.meta.end_time), duration(view.meta.duration),
    me?.od?.rank_tier ? rankName(me.od.rank_tier) : "", me?.od && me.od.party_size > 1 ? `пати из ${me.od.party_size}` : "",
  ].filter(Boolean).join(" · ");
  const tabs = (
    <div className="tabs">
      {TABS.map((t) => (
        <button key={t.id} className={tab === t.id ? "on" : ""} onClick={() => setTab(t.id)} disabled={!me && t.id !== "overview"}>{t.label}</button>
      ))}
    </div>
  );

  return (
    <div>
      {backBtn}
      {me && tab === "overview" ? (
        <>
          {tabs}
          <HeroCard view={view} me={me} won={won} title={heroTitle} meta={metaBits} summary={summaryLine(view, role, names)} />
          {!view.is_me && <div className="notice" style={{ marginTop: 14 }}>Смотришь за другого игрока ({me.name || "без имени"}).</div>}
        </>
      ) : (
        <>
          <div className="compact-head">
            {me && <HeroBadge npc={me.hero_name} size={46} />}
            <div>
              <h1>{heroTitle}<ResultPill won={won} /></h1>
              <div className="sub">
                {[modeShort(view.meta.game_mode, view.meta.lobby_type), [role, opp ? `против ${names.hero(opp.hero_name)}` : ""].filter(Boolean).join(" "), duration(view.meta.duration), relDate(view.meta.end_time)].filter(Boolean).join(" · ")}
                <ODMark status={view.od_status} />
              </div>
            </div>
          </div>
          {tabs}
        </>
      )}

      {tab === "overview" && (
        <div style={{ marginTop: me ? 14 : 0 }}>
          {view.focus < 0 && <div className="notice">В этом матче нет игрока с твоим Steam ID — кликни по строке состава, чтобы разобрать любого игрока.</div>}
          {me && <OverviewTab view={view} />}
          <Scoreboard view={view} onPick={setFocus} />
        </div>
      )}
      {me && tab === "lane" && <LaneTab view={view} goal={lh10Goal} />}
      {me && tab === "farm" && <FarmTab view={view} goal={lh10Goal} />}
      {me && tab === "fights" && <FightsTab view={view} />}
      {me && tab === "deaths" && <DeathsTab view={view} />}
      {me && tab === "abilities" && <AbilitiesTab view={view} />}
      {me && tab === "vision" && <VisionTab view={view} />}
    </div>
  );
}

function ODMark({ status }: { status: string }) {
  const text: Record<string, string> = {
    "": "OpenDota: ещё не запрашивали", ok: "Роль и перцентили — с OpenDota", unparsed: "OpenDota ещё не разобрала матч — повторю позже",
    notfound: "OpenDota не знает этот матч", error: "OpenDota недоступна — повторю позже",
  };
  return <span className={`od-mark ${status === "ok" ? "ok" : ""}`} title={text[status] ?? status}>OD</span>;
}

function HeroCard({ view, me, won, title, meta, summary }: {
  view: MatchView; me: PlayerView; won: boolean | null; title: React.ReactNode; meta: string; summary: string;
}) {
  const g = view.grade;
  // Full-body hero render (cached by the backend, internal/heroimg); rings when unavailable.
  const short = me.hero_name.replace(/^npc_dota_hero_/, "");
  const [artFailed, setArtFailed] = useState(false);
  const art = short && !artFailed ? `/heroimg/render/${short}.png` : "";
  const ringColor = !g ? "var(--panel-3)" : g.letter.startsWith("A") ? "var(--accent)" : g.letter === "D" ? "var(--loss)" : "var(--text)";
  return (
    <div className={`card hero-card ${art ? "with-art" : ""}`}>
      {art ? (
        <div className="hero-art" aria-hidden>
          <img className="glow" src={art} alt="" />
          <img className="render" src={art} alt="" onError={() => setArtFailed(true)} />
        </div>
      ) : (
        <svg className="rings" viewBox="0 0 520 520" aria-hidden>
          {[250, 200, 150].map((r) => <circle key={r} cx={260} cy={260} r={r} fill="none" stroke="rgba(255,255,255,.035)" />)}
        </svg>
      )}
      <div className="main">
        <div className="meta-line"><ResultPill won={won} /><span>{meta}</span><ODMark status={view.od_status} /></div>
        <h1>{title}</h1>
        {summary && <div className="summary">{summary}</div>}
        <div className="head-stats">
          <div><div className="stat-value">{me.kills}/{me.deaths}/{me.assists}</div><div className="caps">K / D / A</div></div>
          <div><div className="stat-value">{kfmt(me.net_worth)}</div><div className="caps">Нетворс</div></div>
          <div><div className="stat-value">{me.gpm}</div><div className="caps">GPM</div></div>
          <div><div className="stat-value">{pct(view.fights.kp)}</div><div className="caps">Участие</div></div>
        </div>
      </div>
      <div className="grade-box"
        title={g ? `По сравнению с ${g.basis} твоими матчами${g.on_hero ? " на этом герое" : ""}: KDA, GPM, XPM, урон, участие, смерти, ЛХ, урон по строениям + бонус за победу` : "Нужно хотя бы 6 разобранных матчей"}>
        <Ring value={g ? g.score / 100 : 0} size={132} stroke={11} color={ringColor}>
          <b className={g ? gradeTone(g.letter) : "faint"} style={{ fontSize: 40, color: g && !gradeTone(g.letter) ? "var(--text)" : undefined }}>{g?.letter ?? "—"}</b>
          <small>{g ? `${g.score} / 100` : "мало игр"}</small>
        </Ring>
        <span className="caps">Оценка за игру</span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- scoreboard

function Scoreboard({ view, onPick }: { view: MatchView; onPick: (pid: number) => void }) {
  const players = view.players ?? [];
  return (
    <div className="card" style={{ marginTop: 14 }}>
      <div className="card-head"><h2>Составы</h2><span className="meta">клик по строке — разбор за этого игрока · ⚔ лайн-оппонент</span></div>
      {[RADIANT, DIRE].map((team) => (
        <div key={team} style={{ marginBottom: team === RADIANT ? 16 : 0 }}>
          <div style={{ display: "flex", gap: 10, alignItems: "center", marginBottom: 4 }}>
            <h3 className={team === RADIANT ? "radiant" : "dire"}>{teamName(team)}</h3>
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
                  <td className="dim" title={[rankName(p.od?.rank_tier), p.od && p.od.party_size > 1 ? `пати из ${p.od.party_size}` : ""].filter(Boolean).join(" · ")}>
                    {p.name || "—"}
                    {p.od?.position ? <span className="faint small"> · поз. {p.od.position}</span> : null}
                  </td>
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
    </div>
  );
}

// ---------------------------------------------------------------- deaths

/** Why a death counts as avoidable, from its context flags. */
function avoidWhy(c: DeathCtx): string {
  const parts = ["один"];
  if (c.deep) parts.push("на чужой половине");
  if (!c.vision) parts.push("без вижна");
  if (c.enemies >= 2) parts.push(`против ${c.enemies}`);
  return parts.join(", ");
}

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
  const avoidable = deaths.filter(({ ctx }) => ctx?.avoidable).length;

  const points: MapPoint[] = layer === "deaths"
    ? deaths.map(({ d }, i) => ({ d, i })).filter(({ d }) => d.has_pos)
      .map(({ d, i }) => ({ x: d.x, y: d.y, color: "#f2705f", label: String(i + 1), highlight: hover === i }))
    : [];
  const pointToDeath = deaths.map(({ d }, i) => ({ d, i })).filter(({ d }) => d.has_pos).map(({ i }) => i);

  return (
    <div>
      <div className="stat-grid">
        <Stat label="Смертей" value={deaths.length} big />
        <Stat label="Потеряно золота" value={goldLost ? num(-goldLost) : "0"} tone={goldLost ? "loss" : ""} big />
        <Stat label="Глупых" value={`${avoidable} из ${deaths.length}`} tone={avoidable ? "loss" : ""} big />
        <Stat label="Без вижна" value={`${noVision} из ${positioned.length}`} tone={noVision ? "accent" : ""} big />
        <Stat label="Байбеков" value={buybacks} big />
      </div>
      <div className="deaths-layout">
        <div className="card">
          <div className="card-head">
            <h2>Карта</h2>
            <div className="seg">
              <button className={layer === "deaths" ? "on" : ""} onClick={() => setLayer("deaths")}>Смерти</button>
              <button className={layer === "move" ? "on" : ""} onClick={() => setLayer("move")}>Передвижение</button>
            </div>
          </div>
          <MapCanvas size={460} points={points} world={view.world}
            heat={layer === "move" ? view.vision.heat ?? [] : undefined}
            onHover={(pi) => setHover(pi === null ? null : pointToDeath[pi])} />
          {hasBg === false && <div className="map-foot">Не удалось загрузить миникарту — показана схема.</div>}
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
              <div key={i} className={`death-card ${hover === i ? "hover" : ""} ${ctx?.avoidable ? "avoidable" : ""}`} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                <div className="dc-head">
                  <span className="dc-num">{i + 1}</span>
                  <span className="dc-time">{clock(d.t)}</span>
                  {ctx?.region && <span className="dc-where">{regionName(ctx.region)}</span>}
                  {d.gold_lost > 0 && <span className="dc-gold">−{num(d.gold_lost)}</span>}
                </div>
                {ctx?.avoidable && <div className="dc-verdict">Глупая смерть: {avoidWhy(ctx)}</div>}
                <div className="dc-line"><span>Убили: </span>{killers.length ? killers.join(", ") : names.unit(d.killer_name)}</div>
                <div className="dc-line"><span>Добил: </span>{blow}</div>
                <div className="dc-tags">
                  {d.has_pos && ctx && (ctx.vision
                    ? <span className="tag">Вижн был</span>
                    : <span className="tag warn"><Icon name="eyeOff" size={11} /> Без вижна</span>)}
                  {ctx?.alone && <span className="tag warn">Один</span>}
                  {ctx && !ctx.alone && ctx.allies > 0 && <span className="tag">{ctx.allies === 1 ? "Союзник рядом" : `Союзников рядом: ${ctx.allies}`}</span>}
                  {ctx?.deep && <span className="tag warn">Чужая половина</span>}
                  {ctx && ctx.enemies >= 2 && <span className="tag">Против {ctx.enemies}</span>}
                  {ctx && ctx.gold >= 1500 && <span className="tag warn" title="Непотраченное золото в момент смерти">{num(ctx.gold)} золота в кармане</span>}
                  <span className="tag">{d.buyback ? "Байбек" : "Без байбека"}</span>
                  <span style={{ marginLeft: "auto" }}><TickButton tick={ctx?.tick} label="тик" /></span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
