import { useEffect, useMemo, useState } from "react";
import { Api, Death, MatchView, PlayerView } from "../api";
import { clock, date, DIRE, duration, gameMode, RADIANT, teamName, thousands } from "../format";
import { useNames } from "../names";
import { HeroBadge, HeroName } from "../components/HeroBadge";
import { SERIES_COLORS, TimeChart } from "../components/Chart";
import { MapCanvas, MapPoint } from "../components/MapCanvas";

type Tab = "overview" | "farm" | "deaths";

export function MatchPage({ matchID, onBack }: { matchID: number; onBack: () => void }) {
  const [focus, setFocus] = useState(-1);
  const [view, setView] = useState<MatchView | null>(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<Tab>("overview");

  useEffect(() => {
    Api.Match(matchID, focus)
      .then((v) => { setView(v); setError(""); })
      .catch((e) => setError(String(e?.message ?? e)));
  }, [matchID, focus]);

  if (error) return <div><button className="link" onClick={onBack}>← к списку</button><div className="notice bad">{error}</div></div>;
  if (!view) return <div className="dim">Загрузка…</div>;

  const players = view.players ?? [];
  const me = players.find((p) => p.player_id === view.focus);
  const won = me ? me.team === view.meta.winner : null;

  return (
    <div>
      <button className="link" onClick={onBack}>← к списку</button>
      <div className="match-head">
        {me && <HeroBadge npc={me.hero_name} size={48} />}
        <div>
          <h1>
            {me ? <HeroTitle npc={me.hero_name} /> : "Матч"}{" "}
            {won !== null && (won ? <span className="win">Победа</span> : <span className="loss">Поражение</span>)}
          </h1>
          <div className="dim">
            Матч {view.meta.match_id > 0 ? view.meta.match_id : "локальный"} · {gameMode(view.meta.game_mode, view.meta.lobby_type)} ·{" "}
            {duration(view.meta.duration)} · {date(view.meta.end_time)} · победа {teamName(view.meta.winner)}
          </div>
          {me && !view.is_me && <div className="dim">Смотришь за другого игрока ({me.name || "без имени"}).</div>}
        </div>
      </div>

      <div className="tabs">
        <button className={tab === "overview" ? "on" : ""} onClick={() => setTab("overview")}>Обзор</button>
        <button className={tab === "farm" ? "on" : ""} onClick={() => setTab("farm")} disabled={!me}>Фарм</button>
        <button className={tab === "deaths" ? "on" : ""} onClick={() => setTab("deaths")} disabled={!me}>Смерти</button>
      </div>

      {tab === "overview" && <Overview view={view} onPick={setFocus} />}
      {tab === "farm" && me && <FarmTab view={view} />}
      {tab === "deaths" && me && <DeathsTab view={view} />}
    </div>
  );
}

function HeroTitle({ npc }: { npc: string }) {
  return <>{useNames().hero(npc)}</>;
}

function Overview({ view, onPick }: { view: MatchView; onPick: (pid: number) => void }) {
  const players = view.players ?? [];
  return (
    <div>
      {view.focus < 0 && (
        <div className="notice">В этом матче нет игрока с твоим Steam ID — кликни по строке, чтобы разобрать любого игрока.</div>
      )}
      {[RADIANT, DIRE].map((team) => (
        <div key={team} className="card">
          <h2 className={team === RADIANT ? "radiant" : "dire"}>
            {teamName(team)} {view.meta.winner === team && <span className="badge ok">победа</span>}
          </h2>
          <table className="table scoreboard">
            <thead>
              <tr>
                <th>Герой</th><th>Игрок</th><th>Ур.</th><th>K</th><th>D</th><th>A</th><th>LH / DN</th>
                <th>NW</th><th>GPM</th><th>XPM</th><th title="Урон героям">Урон гер.</th><th title="Урон строениям">Урон стр.</th><th>Лечение</th>
              </tr>
            </thead>
            <tbody>
              {players.filter((p) => p.team === team).map((p) => (
                <ScoreRow key={p.player_id} p={p} focus={p.player_id === view.focus} opponent={p.player_id === view.opponent} onPick={onPick} />
              ))}
            </tbody>
          </table>
        </div>
      ))}
      <div className="dim small">Лайн-оппонент (отмечен ⚔) определяется по позициям на 1–10 минуте.</div>
    </div>
  );
}

function ScoreRow({ p, focus, opponent, onPick }: { p: PlayerView; focus: boolean; opponent: boolean; onPick: (pid: number) => void }) {
  return (
    <tr className={`clickable ${focus ? "focus" : ""}`} onClick={() => onPick(p.player_id)}>
      <td><HeroName npc={p.hero_name} /> {opponent && <span title="лайн-оппонент">⚔</span>}</td>
      <td className="dim">{p.name || "—"}</td>
      <td>{p.level}</td>
      <td>{p.kills}</td>
      <td>{p.deaths}</td>
      <td>{p.assists}</td>
      <td>{p.last_hits} / {p.denies}</td>
      <td className="gold">{thousands(p.net_worth)}</td>
      <td>{p.gpm}</td>
      <td>{p.xpm}</td>
      <td>{thousands(p.hero_damage)}</td>
      <td>{thousands(p.tower_damage)}</td>
      <td>{thousands(p.healing)}</td>
    </tr>
  );
}

function FarmTab({ view }: { view: MatchView }) {
  const names = useNames();
  const players = view.players ?? [];
  const me = players.find((p) => p.player_id === view.focus)!;
  const opp = players.find((p) => p.player_id === view.opponent);
  const t = view.farm.t ?? [];
  const lines = (m: { focus: any; team_avg: any; opponent: any }) => [
    { name: names.hero(me.hero_name), data: m.focus, color: SERIES_COLORS.focus },
    { name: "Среднее по команде", data: m.team_avg, color: SERIES_COLORS.team, dashed: true },
    ...(opp ? [{ name: `${names.hero(opp.hero_name)} (оппонент)`, data: m.opponent, color: SERIES_COLORS.opponent }] : []),
  ];
  return (
    <div className="grid2">
      <div className="card"><TimeChart title="Net worth" t={t} lines={lines(view.farm.net_worth)} /></div>
      <div className="card"><TimeChart title="GPM" t={t} lines={lines(view.farm.gpm)} /></div>
      <div className="card"><TimeChart title="XPM" t={t} lines={lines(view.farm.xpm)} /></div>
      <div className="card"><TimeChart title="Last hits" t={t} lines={lines(view.farm.last_hits)} /></div>
      {!opp && <div className="dim small">Лайн-оппонента определить не удалось.</div>}
    </div>
  );
}

type DeathKind = "death" | "kill";

function DeathsTab({ view }: { view: MatchView }) {
  const names = useNames();
  const [show, setShow] = useState<{ death: boolean; kill: boolean }>({ death: true, kill: true });
  const [hover, setHover] = useState<number | null>(null);
  const players = view.players ?? [];
  const byPID = useMemo(() => new Map(players.map((p) => [p.player_id, p])), [players]);

  const items = useMemo(() => {
    const out: { d: Death; kind: DeathKind }[] = [];
    for (const d of view.deaths ?? []) {
      if (d.victim === view.focus && show.death) out.push({ d, kind: "death" });
      else if (d.killer === view.focus && show.kill) out.push({ d, kind: "kill" });
    }
    return out;
  }, [view, show]);

  const points: MapPoint[] = items
    .map((it, i) => ({ it, i }))
    .filter(({ it }) => it.d.has_pos)
    .map(({ it, i }) => ({
      x: it.d.x, y: it.d.y,
      color: it.kind === "death" ? "#ff6b5b" : "#5fd17a",
      highlight: hover === i,
    }));
  const pointToItem = items.map((it, i) => ({ it, i })).filter(({ it }) => it.d.has_pos).map(({ i }) => i);

  const who = (pid: number, fallback: string) => {
    const p = byPID.get(pid);
    return p ? names.hero(p.hero_name) : names.unit(fallback);
  };
  const deaths = items.filter((i) => i.kind === "death").length;
  const kills = items.filter((i) => i.kind === "kill").length;

  return (
    <div className="deaths-layout">
      <div className="card map-card">
        <MapCanvas
          points={points}
          world={view.world}
          onHover={(pi) => setHover(pi === null ? null : pointToItem[pi])}
        />
        <div className="legend">
          <label><input type="checkbox" checked={show.death} onChange={(e) => setShow({ ...show, death: e.target.checked })} /> <span className="dot" style={{ background: "#ff6b5b" }} /> смерти ({deaths})</label>
          <label><input type="checkbox" checked={show.kill} onChange={(e) => setShow({ ...show, kill: e.target.checked })} /> <span className="dot" style={{ background: "#5fd17a" }} /> убийства ({kills})</label>
        </div>
      </div>
      <div className="card list-card">
        <table className="table">
          <thead><tr><th>Время</th><th></th><th>Кто</th><th>Чем</th><th>Помогали</th></tr></thead>
          <tbody>
            {items.map((it, i) => {
              const other = it.kind === "death" ? it.d.killer : it.d.victim;
              const otherName = it.kind === "death" ? who(it.d.killer, it.d.killer_name) : who(it.d.victim, "");
              const otherHero = byPID.get(other)?.hero_name;
              return (
                <tr key={i} className={hover === i ? "hover" : ""} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                  <td>{clock(it.d.t)}</td>
                  <td>{it.kind === "death" ? <span className="loss">☠ убит</span> : <span className="win">⚔ убил</span>}</td>
                  <td><span className="hero-name">{otherHero && <HeroBadge npc={otherHero} size={20} />}{otherName}</span></td>
                  <td className="dim">{names.inflictor(it.d.inflictor)}</td>
                  <td className="dim">{(it.d.assists ?? []).map((a) => who(a, "")).join(", ") || "—"}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {items.length === 0 && <div className="empty">Ничего нет.</div>}
      </div>
    </div>
  );
}
