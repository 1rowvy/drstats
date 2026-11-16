import { useState } from "react";
import type { Battle, MatchView } from "../../api";
import { clock, duration, num, pct, plural, RADIANT, regionName, upperFirst } from "../../format";
import { useNames } from "../../names";
import { TickButton } from "../../components/ui";
import { heroOf, useFocus } from "./common";
import { MatchSmokes } from "../../components/Smokes";

const fightName = (b: Battle) => upperFirst(regionName(b.region) || "Карта");

export function FightsTab({ view }: { view: MatchView }) {
  const battles = view.battles ?? [];
  const [sel, setSel] = useState(() => {
    // Open the biggest fight by default.
    let best = 0;
    battles.forEach((b, i) => { if (Math.abs(b.gold_swing) > Math.abs(battles[best].gold_swing)) best = i; });
    return best;
  });
  if (!battles.length) return <div><div className="card empty">Драк с твоим участием не нашлось.</div><MatchSmokes view={view} /></div>;
  const won = battles.filter((b) => b.won).length;
  const total = battles.reduce((a, b) => a + b.gold_swing, 0);
  const dur = Math.max(60, view.meta.duration);
  const b = battles[Math.min(sel, battles.length - 1)];
  let lift = 0;
  return (
    <div>
      <div className="card">
        <div className="card-head">
          <h2>{battles.length} {battles.length % 10 === 1 && battles.length % 100 !== 11 ? "драка" : [2, 3, 4].includes(battles.length % 10) && ![12, 13, 14].includes(battles.length % 100) ? "драки" : "драк"} за игру</h2>
          <span className="legend-inline" style={{ fontSize: 12 }}>
            <span><b className="win display">{won}</b> выиграно</span>
            <span><b className="loss display">{battles.length - won}</b> проиграно</span>
            <span><b className={`display ${total >= 0 ? "" : "loss"}`}>{num(total, true)}</b> итого по золоту</span>
          </span>
        </div>
        <div className="fight-track">
          {battles.map((f, i) => {
            // Markers closer than ~2.5% of the game alternate up and down.
            const near = i > 0 && (f.start - battles[i - 1].start) / dur < 0.025;
            lift = near ? -lift || -9 : 0;
            return (
            <button key={i} className={`fight-mark ${f.won ? "won" : "lost"} ${i === sel ? "on" : ""}`}
              style={{ left: `calc(14px + ${(Math.max(0, f.start) / dur)} * (100% - 28px))`, zIndex: i === sel ? 2 : 1, marginTop: lift }}
              title={`${clock(f.start)} · ${fightName(f)}`} onClick={() => setSel(i)}>{i + 1}</button>
            );
          })}
        </div>
        <div className="scale"><span>0:00</span><span>{clock(dur / 4)}</span><span>{clock(dur / 2)}</span><span>{clock((dur * 3) / 4)}</span><span>{clock(dur)}</span></div>
      </div>
      <FightPlaySummary view={view} battles={battles} onSelect={setSel} />
      <MatchSmokes view={view} />
      <div className="grid-side" style={{ gridTemplateColumns: "280px 1fr", marginTop: 14 }}>
        <div className="fight-list" style={{ maxHeight: 820, overflow: "auto", paddingRight: 2 }}>
          {battles.map((f, i) => (
            <button key={i} className={`fight-item ${i === sel ? "on" : ""}`} onClick={() => setSel(i)}>
              <span className={`num ${f.won ? "won" : "lost"}`}>{i + 1}</span>
              <span style={{ minWidth: 0 }}>
                <b>{fightName(f)}</b>
                <small>{clock(f.start)} · {duration(f.end - f.start)} · {f.allies} на {f.enemies}</small>
              </span>
              <span className={`swing ${f.gold_swing >= 0 ? "win" : "loss"}`}>{num(f.gold_swing, true)}</span>
            </button>
          ))}
        </div>
        <FightDetail view={view} b={b} n={Math.min(sel, battles.length - 1) + 1} />
      </div>
    </div>
  );
}

function FightDetail({ view, b, n }: { view: MatchView; b: Battle; n: number }) {
  const names = useNames();
  const { me, players } = useFocus(view);
  const sources = (b.my_sources ?? []).slice(0, 6);
  const maxSrc = Math.max(1, ...sources.map((s) => s.amount));
  const combo = sources.filter((s) => s.key).slice(0, 3).map((s) => names.inflictor(s.key));
  const heroes = b.heroes ?? [];
  const side = (team: number) => heroes.filter((h) => heroOf(players, h.pid)?.team === team);
  return (
    <div className="card fight-detail">
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <span className={`pill ${b.won ? "win" : "loss"}`}>{b.won ? "Выиграли" : "Проиграли"}</span>
        <span className="mono faint small">{clock(b.start)} · длилась {duration(b.end - b.start)}</span>
        <span style={{ marginLeft: "auto" }}><TickButton tick={b.tick} label="demo_gototick" /></span>
      </div>
      <h2>Драка {n}: {fightName(b)}</h2>
      <div className="dim">
        {b.allies} на {b.enemies} · {b.kills} {b.kills === 1 ? "убийство" : "убийств"}, {b.losses} {b.losses === 1 ? "потеря" : "потерь"}
        {combo.length ? <> · {combo.join(" → ")}</> : null}
        {b.my_died ? <span className="loss"> · ты умер</span> : null}
      </div>
      <div className={`swing-big ${b.gold_swing >= 0 ? "win" : "loss"}`}>{num(b.gold_swing, true)}</div>
      <div className="caps" style={{ marginTop: 8 }}>Свинг по золоту</div>
      <PlayLines view={view} b={b} />
      <div className="tiles4" style={{ marginTop: 16 }}>
        <div className="tile"><div className="label">Твой урон</div><div className="val">{num(b.my_damage)}</div></div>
        <div className="tile"><div className="label">Доля в команде</div><div className="val">{pct(b.my_share)}</div></div>
        <div className="tile"><div className="label">Убийства</div><div className="val">{b.my_kills}{b.my_assists ? <span className="faint" style={{ fontSize: 13 }}> +{b.my_assists}</span> : null}</div></div>
        <div className="tile"><div className="label">Получено урона</div><div className="val">{num(b.my_taken)}</div></div>
      </div>
      {sources.length > 0 && (
        <>
          <div className="caps" style={{ margin: "20px 0 10px" }}>Откуда урон</div>
          {sources.map((s, i) => (
            <div className={`src-row ${i === 0 ? "top" : ""}`} key={s.key}>
              <span>{names.inflictor(s.key)}</span>
              <div className="bar"><div style={{ width: `${(s.amount / maxSrc) * 100}%` }} /></div>
              <span className="mono">{num(s.amount)}</span>
            </div>
          ))}
        </>
      )}
      <div className="teams" style={{ marginTop: 18 }}>
        {[RADIANT, 3].map((team) => (
          <div key={team}>
            <h3 className={team === RADIANT ? "radiant" : "dire"}>{team === RADIANT ? "RADIANT" : "DIRE"}</h3>
            {side(team).length === 0 && <div className="faint small" style={{ padding: "8px 0" }}>—</div>}
            {side(team).map((h) => {
              const p = heroOf(players, h.pid)!;
              const isMe = h.pid === me.player_id;
              return (
                <div key={h.pid} className={`tr ${isMe ? "me" : ""}`}>
                  {isMe ? <b>{names.hero(p.hero_name)} (ты)</b> : <span>{names.hero(p.hero_name)}</span>}
                  <span className="mono">{num(h.dealt)}</span>
                </div>
              );
            })}
          </div>
        ))}
      </div>
      <div className="faint small" style={{ marginTop: 12 }}>Урон по героям за время драки; участники — кто нанёс или получил ≥150 урона.</div>
    </div>
  );
}

// ---------------------------------------------------------------- fight play

const LATE_MIN = 2; // late fights before it becomes a note
const INSTANT_DEATH = 3; // died this fast after the start: no time to press anything

/** How you fight over the whole game: who starts, being caught first, joining late, wasted key spells. */
function FightPlaySummary({ view, battles, onSelect }: { view: MatchView; battles: Battle[]; onSelect: (i: number) => void }) {
  const names = useNames();
  const { me } = useFocus(view);
  const ours = battles.filter((b) => b.play.initiator >= 0 && b.play.we_started);
  const theirs = battles.filter((b) => b.play.initiator >= 0 && !b.play.we_started);
  const wr = (xs: Battle[]) => (xs.length ? xs.filter((b) => b.won).length / xs.length : 0);
  const caught = battles.filter((b) => b.play.first_target === me.player_id && !b.play.we_started);
  const late = battles.filter((b) => b.play.late);
  const silent = battles.filter((b) => b.play.my_entry == null);
  // Deaths with a key spell ready and not pressed (instant deaths aside).
  const wasted = battles.map((b, i) => ({ b, i }))
    .filter(({ b }) => b.my_died && (b.play.key_unused ?? []).length > 0 && (b.play.death_after ?? 99) >= INSTANT_DEATH);
  const byKey = new Map<string, number>();
  wasted.forEach(({ b }) => (b.play.key_unused ?? []).forEach((k) => byKey.set(k, (byKey.get(k) ?? 0) + 1)));
  const wastedList = [...byKey.entries()].sort((a, b) => b[1] - a[1]);
  const notes: string[] = [];
  if (ours.length >= 2 && theirs.length >= 2) {
    const d = Math.round((wr(ours) - wr(theirs)) * 100);
    if (d >= 20) notes.push(`Когда драку начинаете вы — выигрываете ${pct(wr(ours))}, когда они — ${pct(wr(theirs))}. Ищите инициативу: смоки, вижн, ловля на фарме.`);
  }
  if (caught.length >= 2) notes.push(`${caught.length} ${plural(caught.length, "раз", "раза", "раз")} драку начинали с тебя — тебя ловили первым. Смотри, где стоишь, когда врагов нет на карте.`);
  if (late.length >= LATE_MIN) notes.push(`В ${late.length} ${plural(late.length, "драке", "драках", "драках")} ты вступил через 10+ секунд после начала. Держись ближе к команде, когда она собирается.`);
  if (wastedList.length) notes.push(`Умер, не нажав ${wastedList.map(([k, n]) => `${names.inflictor(k)}${n > 1 ? ` ×${n}` : ""}`).join(", ")}. Сейв нажимай до того, как тебя законтролят, ульту — пока жив.`);
  return (
    <div className="card" style={{ marginTop: 14 }}>
      <div className="card-head"><h2>Как ты дерёшься</h2><span className="meta">по {battles.length} {plural(battles.length, "драке", "дракам", "дракам")} этой игры</span></div>
      <div className="tiles4">
        <div className="tile"><div className="label">Начинали мы</div><div className="val">{ours.length}</div><div className="faint small">{ours.length ? `выиграли ${pct(wr(ours))}` : "—"}</div></div>
        <div className="tile"><div className="label">Начинали они</div><div className="val">{theirs.length}</div><div className="faint small">{theirs.length ? `выиграли ${pct(wr(theirs))}` : "—"}</div></div>
        <div className="tile"><div className="label">Поймали тебя первым</div><div className={`val ${caught.length >= 2 ? "loss" : ""}`}>{caught.length}</div></div>
        <div className="tile"><div className="label">Опоздал</div><div className={`val ${late.length >= LATE_MIN ? "loss" : ""}`}>{late.length}</div>{silent.length > 0 && <div className="faint small">без урона: {silent.length}</div>}</div>
      </div>
      {wasted.length > 0 && (
        <div style={{ marginTop: 14 }}>
          <div className="caps" style={{ marginBottom: 8 }}>Умер, не нажав</div>
          <div className="dc-tags">
            {wasted.map(({ b, i }) => (
              <button key={i} className="tag warn" onClick={() => onSelect(i)} title="Открыть драку">
                {clock(b.start)} · {(b.play.key_unused ?? []).map((k) => names.inflictor(k)).join(", ")}
              </button>
            ))}
          </div>
        </div>
      )}
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
      {notes.length === 0 && <div className="note">Явных ошибок в драках не видно.</div>}
    </div>
  );
}

/** Opening, your entry and key spells of one fight. */
function PlayLines({ view, b }: { view: MatchView; b: Battle }) {
  const names = useNames();
  const { me, players } = useFocus(view);
  const p = b.play;
  const hero = (pid: number) => (pid === me.player_id ? "ты" : names.hero(heroOf(players, pid)?.hero_name ?? ""));
  const lines: { label: string; text: string; tone?: string }[] = [];
  if (p.initiator >= 0) {
    const who = p.initiator === me.player_id ? "ты" : `${hero(p.initiator)} (${p.we_started ? "наши" : "они"})`;
    lines.push({ label: "Начал", text: `${upperFirst(who)}${p.init_ability ? ` — ${names.inflictor(p.init_ability)}` : ""}`, tone: p.we_started ? "win" : "" });
  }
  if (p.first_target >= 0) {
    const mine = p.first_target === me.player_id;
    lines.push({ label: "Первым досталось", text: upperFirst(hero(p.first_target)), tone: mine && !p.we_started ? "loss" : "" });
  }
  const far = p.my_dist >= 2500 ? ` · в начале был в ${num(Math.round(p.my_dist / 100) * 100)} юнитах` : "";
  if (p.my_entry == null) lines.push({ label: "Ты", text: `урона по героям не нанёс${far}`, tone: "loss" });
  else lines.push({ label: "Ты вступил", text: (p.my_entry < 3 ? "с первых секунд" : `через ${Math.round(p.my_entry)} с`) + far, tone: p.late ? "loss" : "" });
  if (p.death_after != null) {
    lines.push({ label: "Умер", text: p.death_after < INSTANT_DEATH ? `через ${Math.round(p.death_after)} с — убили сразу` : `через ${Math.round(p.death_after)} с после начала`, tone: "loss" });
  }
  const used = p.key_used ?? [], unused = p.key_unused ?? [];
  return (
    <div style={{ marginTop: 16 }}>
      <div className="play-lines">
        {lines.map((l) => (
          <div key={l.label}><span className="dim">{l.label}</span><b className={l.tone ?? ""}>{l.text}</b></div>
        ))}
      </div>
      {(used.length > 0 || unused.length > 0) && (
        <div className="dc-tags" style={{ marginTop: 10 }}>
          {used.map((k) => <span key={k} className="tag win-tag">✓ {names.inflictor(k)}</span>)}
          {unused.map((k) => (
            <span key={k} className="tag warn" title="Было готово, но не нажато">
              ✗ {names.inflictor(k)} — {b.my_died ? "не нажал до смерти" : "не нажал"}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
