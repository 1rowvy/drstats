import { useState } from "react";
import type { Battle, MatchView } from "../../api";
import { clock, duration, num, pct, RADIANT, regionName, upperFirst } from "../../format";
import { useNames } from "../../names";
import { TickButton } from "../../components/ui";
import { heroOf, useFocus } from "./common";

const fightName = (b: Battle) => upperFirst(regionName(b.region) || "Карта");

export function FightsTab({ view }: { view: MatchView }) {
  const battles = view.battles ?? [];
  const [sel, setSel] = useState(() => {
    // Open the biggest fight by default.
    let best = 0;
    battles.forEach((b, i) => { if (Math.abs(b.gold_swing) > Math.abs(battles[best].gold_swing)) best = i; });
    return best;
  });
  if (!battles.length) return <div className="card empty">Драк с твоим участием не нашлось.</div>;
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
