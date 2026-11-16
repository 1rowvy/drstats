import type { MatchView, SmokeStats, SmokeView } from "../api";
import { clock, pct, plural, regionName, upperFirst } from "../format";
import { useNames } from "../names";
import { TickButton } from "./ui";
import { objectiveLabel } from "../pages/match/common";

const objKind = (key: string) => (key.includes("roshan") ? "roshan" : key.includes("_rax_") ? "barracks" : "tower");

const RESULT: Record<string, { label: string; tone: string }> = {
  kill: { label: "убили", tone: "win" },
  objective: { label: "взяли объект", tone: "win" },
  trade: { label: "размен", tone: "" },
  lost: { label: "потеряли", tone: "loss" },
  nothing: { label: "впустую", tone: "dim" },
};

/** Smokes of one match: yours with their outcome, and enemy smokes that caught you. */
export function MatchSmokes({ view }: { view: MatchView }) {
  const names = useNames();
  const smokes = view.smokes ?? [];
  const own = smokes.filter((s) => !s.enemy);
  const enemy = smokes.filter((s) => s.enemy);
  if (!smokes.length) return null;
  const hero = (pid: number) => {
    if (pid === view.focus) return "ты";
    const p = view.players?.find((x) => x.player_id === pid);
    return p ? names.hero(p.hero_name) : "?";
  };
  const mine = own.filter((s) => s.mine);
  const good = mine.filter((s) => s.result === "kill" || s.result === "objective").length;
  const caught = enemy.filter((s) => s.died);
  const notes: string[] = [];
  const empty = mine.filter((s) => s.result === "nothing").length;
  if (empty >= 2) notes.push(`${empty} ${plural(empty, "смок", "смока", "смоков")} из ${mine.length} прошли впустую. Смок — под цель: заранее решите, кого ловите, и где он сейчас.`);
  if (caught.length) notes.push(`Вражеский смок ${caught.length === 1 ? "поймал" : `${caught.length} раза ловил`} тебя. Пропали трое-четверо с карты — уходи к своим или под вижн.`);
  const row = (s: SmokeView, i: number) => {
    const members = s.members ?? [], kills = s.kills ?? [], lost = s.lost ?? [];
    const r = RESULT[s.result] ?? RESULT.nothing;
    const where = s.region ? upperFirst(regionName(s.region)) : "";
    const who = members.map(hero).join(", ");
    const dead = lost.length ? ` · ${s.enemy ? "их" : "наших"} погибло ${lost.length}` : "";
    return (
      <div key={i} className={`smoke-row ${s.mine ? "me" : ""}`}>
        <span className="mono faint">{clock(s.t)}</span>
        <span style={{ minWidth: 0 }}>
          <b>{s.enemy ? "Вражеский смок" : s.user === view.focus ? "Твой смок" : `Смок ${hero(s.user)}`}</b>
          <small> · {members.length} {plural(members.length, "герой", "героя", "героев")}{where ? ` · раскрылись: ${where}` : ""}</small>
          <div className="faint small">
            {who}
            {kills.length ? ` · убили ${kills.map(hero).join(", ")}` : ""}
            {dead}
            {s.objective ? ` · затем ${objectiveLabel(s.objective, objKind(s.objective))}` : ""}
          </div>
        </span>
        <span className="smoke-tags">
          {s.enemy
            ? (s.died ? <span className="tag warn">поймали тебя</span> : <span className="faint small">{r.label}</span>)
            : <span className={`pill ${r.tone}`}>{upperFirst(r.label)}</span>}
          {!s.enemy && s.mine && s.involved && <span className="tag win-tag">ты в убийстве</span>}
          {!s.enemy && s.died && <span className="tag warn">ты умер</span>}
          {!s.enemy && !s.mine && <span className="faint small">без тебя</span>}
        </span>
        <TickButton tick={s.tick} label="demo_gototick" />
      </div>
    );
  };
  return (
    <div className="card" style={{ marginTop: 14 }}>
      <div className="card-head">
        <h2>Смоки</h2>
        <span className="legend-inline" style={{ fontSize: 12 }}>
          <span><b className="display">{mine.length}</b> с тобой</span>
          <span><b className="win display">{good}</b> удачных</span>
          <span><b className={`display ${caught.length ? "loss" : ""}`}>{enemy.length}</b> вражеских{caught.length ? `, поймали тебя ${caught.length}` : ""}</span>
        </span>
      </div>
      <div className="smoke-list">{smokes.map(row)}</div>
      <div className="faint small" style={{ marginTop: 10 }}>Убийства и потери — героев под смоком, пока он действовал и 15 с после того, как спал.</div>
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
    </div>
  );
}

/** Smokes over the filtered games with a replay (dashboard, hero page). */
export function SmokeStatsCard({ s }: { s: SmokeStats }) {
  if (s.games === 0) {
    return <div className="card"><div className="card-head"><h2>Смоки</h2></div><div className="dim">Нужны реплеи, разобранные новой версией (переразбери старые в настройках).</div></div>;
  }
  const good = s.kill + s.objective;
  const rate = s.smokes ? good / s.smokes : 0;
  const results = [
    { label: "убили", n: s.kill, cls: "" },
    { label: "взяли объект", n: s.objective, cls: "" },
    { label: "размен", n: s.trade, cls: "" },
    { label: "потеряли", n: s.lost, cls: "bad" },
    { label: "впустую", n: s.nothing, cls: "bad" },
  ];
  const max = Math.max(1, ...results.map((r) => r.n));
  const notes: string[] = [];
  if (s.win_good != null && s.win_bad != null) {
    const d = Math.round((s.win_good - s.win_bad) * 100);
    if (d >= 10) notes.push(`Игры с удачным смоком выигрываешь ${pct(s.win_good)}, со смоками впустую — ${pct(s.win_bad)}.`);
  }
  if (s.smokes >= 5 && s.nothing / s.smokes >= 0.4) notes.push(`${pct(s.nothing / s.smokes)} смоков ничего не дают. Идите под цель: враг на фарме без вижна, а не «погулять».`);
  if (s.games >= 5 && s.per_game < 0.5) notes.push("Смоков мало — меньше одного на две игры. Смок за 50 золота — самый дешёвый способ начать драку первыми.");
  if (s.caught_games >= 3 && s.caught_games / s.games >= 0.25) notes.push(`В ${pct(s.caught_games / s.games)} игр тебя ловил вражеский смок. Когда враги пропадают с карты толпой — не фарми один на их половине.`);
  return (
    <div className="card">
      <div className="card-head">
        <h2>Смоки</h2>
        <span className="meta">{s.games} {plural(s.games, "игра", "игры", "игр")} с реплеем</span>
      </div>
      <div className="grid-side" style={{ gridTemplateColumns: "minmax(0, 1fr) minmax(0, 1fr)", gap: 22 }}>
        <div className="tiles">
          <div className="tile"><div className="label">За игру</div><div className="val">{s.per_game.toFixed(1)}</div><div className="faint small">свои: {s.used}</div></div>
          <div className="tile"><div className="label">Удачных</div><div className={`val ${s.smokes >= 5 && rate < 0.3 ? "loss" : ""}`}>{s.smokes ? pct(rate) : "—"}</div><div className="faint small">{good} из {s.smokes}</div></div>
          <div className="tile"><div className="label">Ты в убийстве</div><div className="val">{s.involved}</div><div className="faint small">умер в смоке: {s.died}</div></div>
          <div className="tile"><div className="label">Поймал вражеский</div><div className={`val ${s.caught ? "loss" : ""}`}>{s.caught}</div><div className="faint small">из {s.enemy} их смоков</div></div>
        </div>
        <div>
          <div className="caps" style={{ marginBottom: 6 }}>Чем кончались</div>
          {s.smokes === 0 && <div className="faint small">ты не был ни в одном смоке</div>}
          {s.smokes > 0 && results.map((r) => (
            <div key={r.label} className="zone-row">
              <span>{upperFirst(r.label)}</span>
              <div className={`bar ${r.cls}`}><div style={{ width: `${(r.n / max) * 100}%` }} /></div>
              <span className="mono faint">{r.n}</span>
            </div>
          ))}
        </div>
      </div>
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
    </div>
  );
}
