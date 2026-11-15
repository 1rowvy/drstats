import { useMemo, useState } from "react";
import type { AbilityUse, MatchView, PlayerView } from "../api";
import { clock, duration, laneName, pct, RUNES, signed, signedClock, thousands } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "../components/HeroBadge";
import { MapCanvas, MapPoint } from "../components/MapCanvas";
import { Stat } from "../components/ui";

function useFocus(view: MatchView) {
  const players = view.players ?? [];
  const me = players.find((p) => p.player_id === view.focus)!;
  const opp = players.find((p) => p.player_id === view.opponent);
  return { players, me, opp };
}

/** Difference cell, coloured by whether it is good for the focused player. */
function Diff({ v, fmt = (x: number) => signed(x), higherBetter = true }: {
  v: number | null | undefined; fmt?: (x: number) => string; higherBetter?: boolean;
}) {
  if (v == null) return <span className="dim">—</span>;
  const good = higherBetter ? v > 0 : v < 0;
  return <span className={v === 0 ? "dim" : good ? "win" : "loss"}>{fmt(v)}</span>;
}

// ---------------------------------------------------------------- items

const CONSUMABLES = new Set([
  "tango", "flask", "clarity", "ward_observer", "ward_sentry", "tpscroll", "smoke_of_deceit", "dust", "enchanted_mango",
  "faerie_fire", "blood_grenade", "tome_of_knowledge", "famango", "great_famango", "greater_famango", "branches",
]);

export function ItemsTable({ view }: { view: MatchView }) {
  const names = useNames();
  const [all, setAll] = useState(false);
  const items = (view.items ?? []).filter((it) => all || !CONSUMABLES.has(it.item));
  const cost = (k: string) => names.raw.items?.[k]?.cost ?? 0;
  return (
    <div>
      <label className="dim small" style={{ display: "flex", gap: 6, alignItems: "center", marginBottom: 8 }}>
        <input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} /> показывать расходники
      </label>
      <table className="table">
        <thead><tr><th>Время</th><th>Предмет</th><th>Цена</th><th>Мой средний тайминг на герое</th><th>Разница</th></tr></thead>
        <tbody>
          {items.map((it, i) => {
            const key = cost(it.item) >= 1800;
            return (
              <tr key={i} className={key ? "" : "minor"}>
                <td>{clock(it.t)}</td>
                <td>{key ? <b>{names.inflictor("item_" + it.item)}</b> : names.inflictor("item_" + it.item)}{!it.first && <span className="dim small"> (повтор)</span>}</td>
                <td className="gold">{cost(it.item) || "—"}</td>
                <td className="dim">{it.avg != null && it.first ? `${clock(it.avg)} (матчей: ${it.n})` : "—"}</td>
                <td>{it.diff != null ? <Diff v={Math.round(it.diff)} fmt={signedClock} higherBetter={false} /> : <span className="dim">—</span>}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {items.length === 0 && <div className="empty">Покупок нет.</div>}
      <div className="faint small" style={{ marginTop: 10 }}>
        Стартовые предметы, купленные во время выбора героев, в реплей не попадают. Жирным — предметы от 1800 золота.
        Средний тайминг — по первой покупке в других моих матчах на этом герое.
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- abilities

export function AbilitiesTab({ view }: { view: MatchView }) {
  const names = useNames();
  const a = view.abilities;
  const uses = a.uses ?? [];
  const row = (u: AbilityUse) => (
    <tr key={u.ability}>
      <td>{names.inflictor(u.ability)}</td>
      <td>{u.casts}</td>
      <td>{u.hit_rate != null ? `${u.hits} / ${u.casts}` : "—"}</td>
      <td>{u.hit_rate != null ? <HitBar v={u.hit_rate} /> : <span className="dim" title="Ни разу не нанесло урон и не наложило дебафф на вражеского героя — вероятно, не атакующая способность">—</span>}</td>
      <td>{u.damage ? thousands(u.damage) : <span className="dim">—</span>}</td>
      <td className="dim small times">{(u.times ?? []).slice(0, 30).map(clock).join(" ")}{(u.times ?? []).length > 30 ? " …" : ""}</td>
    </tr>
  );
  const head = <thead><tr><th>Способность</th><th>Касты</th><th>Попадания</th><th>Hit rate*</th><th>Урон героям</th><th>Время кастов</th></tr></thead>;
  return (
    <div>
      <div className="card">
        <h2 style={{ marginBottom: 12 }}>Способности</h2>
        <table className="table">{head}<tbody>{uses.filter((u) => !u.is_item).map(row)}</tbody></table>
      </div>
      <div className="card" style={{ marginTop: 14 }}>
        <h2 style={{ marginBottom: 12 }}>Активные предметы</h2>
        <table className="table">{head}<tbody>{uses.filter((u) => u.is_item).map(row)}</tbody></table>
      </div>
      <div className="notice small" style={{ marginTop: 14 }}>
        * Эвристика: каст считается попаданием, если в течение {a.hit_window} с после него эта способность нанесла урон
        или наложила дебафф на вражеского героя. Окно настраивается в настройках. Для снарядов с долгим полётом и
        отложенных способностей занижено.
      </div>
    </div>
  );
}

function HitBar({ v }: { v: number }) {
  return (
    <span className="hitbar">
      <span style={{ width: `${v * 100}%`, background: v >= 0.6 ? "var(--win)" : v >= 0.35 ? "var(--warn)" : "var(--loss)" }} />
      <em>{pct(v)}</em>
    </span>
  );
}

// ---------------------------------------------------------------- vision & map

type VisionLayer = "heat" | "obs" | "sentry" | "dewards";

export function VisionTab({ view }: { view: MatchView }) {
  const names = useNames();
  const { me, players } = useFocus(view);
  const v = view.vision;
  const [layer, setLayer] = useState<VisionLayer>("heat");
  const [hover, setHover] = useState<number | null>(null);
  const byPID = useMemo(() => new Map(players.map((p) => [p.player_id, p])), [players]);

  const wards = layer === "obs" ? (v.placed ?? []).filter((w) => !w.sentry)
    : layer === "sentry" ? (v.placed ?? []).filter((w) => w.sentry)
    : layer === "dewards" ? v.destroyed ?? [] : [];
  const points: MapPoint[] = wards.map((w, i) => ({
    x: w.x, y: w.y, highlight: hover === i,
    color: layer === "dewards" ? "#ff6b5b" : w.sentry ? "#5b9cff" : w.destroyed ? "#ff9f43" : "#f2c14e",
  }));
  const share = v.time_share;
  const killer = (pid: number, fallback: string) => {
    const p = byPID.get(pid);
    return p ? names.hero(p.hero_name) : names.unit(fallback);
  };

  return (
    <div>
      <div className="stat-grid">
        <Stat label="Обсерверы" value={v.obs_placed} sub={v.obs_placed ? `живут в среднем ${duration(v.obs_avg_life)}` : ""} />
        <Stat label="Сентри" value={v.sen_placed} />
        <Stat label="Мои варды снесли" value={v.my_lost} />
        <Stat label="Я снёс вардов" value={(v.destroyed ?? []).length} />
        <Stat label="Стаки" value={(v.stacks ?? []).length} sub={(v.stacks ?? []).map(clock).join(", ")} />
        <Stat label="Руны" value={(v.runes ?? []).length} />
      </div>
      <div className="deaths-layout">
        <div className="card">
          <div className="seg" style={{ marginBottom: 12 }}>
            <button className={layer === "heat" ? "on" : ""} onClick={() => setLayer("heat")}>Перемещения</button>
            <button className={layer === "obs" ? "on" : ""} onClick={() => setLayer("obs")}>Обсерверы</button>
            <button className={layer === "sentry" ? "on" : ""} onClick={() => setLayer("sentry")}>Сентри</button>
            <button className={layer === "dewards" ? "on" : ""} onClick={() => setLayer("dewards")}>Снесённые мной</button>
          </div>
          <MapCanvas world={view.world} size={460} points={points} heat={layer === "heat" ? v.heat ?? [] : undefined} onHover={setHover} />
          <div style={{ marginTop: 16 }}>
            <h3 style={{ marginBottom: 10 }}>Где проводил время</h3>
            <ShareBar label="Своя база" v={share.base} color="#9aa3b5" />
            <ShareBar label={`Своя линия (${laneName(v.lane, me.team)})`} v={share.lane} color="#5b9cff" />
            <ShareBar label="Свой лес и прочее" v={share.jungle} color="#5fd17a" />
            <ShareBar label="Половина врага" v={share.enemy} color="#ff6b5b" />
            <div className="dim small">По позиции раз в 5 с, только живым; границы зон приблизительные.</div>
          </div>
        </div>
        <div className="card" style={{ maxHeight: 720, overflow: "auto" }}>
          {layer === "heat" ? (
            <>
              <h2 style={{ marginBottom: 10 }}>Руны</h2>
              <table className="table">
                <thead><tr><th>Время</th><th>Руна</th><th></th></tr></thead>
                <tbody>
                  {(v.runes ?? []).map((r, i) => (
                    <tr key={i}><td>{clock(r.t)}</td><td>{RUNES[r.type] ?? `#${r.type}`}</td><td className="dim">{r.bottled ? "в бутылку" : ""}</td></tr>
                  ))}
                </tbody>
              </table>
              {(v.runes ?? []).length === 0 && <div className="dim">Рун не брал.</div>}
            </>
          ) : (
            <table className="table">
              <thead><tr><th>Поставлен</th><th>Снят</th><th>Прожил</th><th>{layer === "dewards" ? "Чей" : "Итог"}</th></tr></thead>
              <tbody>
                {wards.map((w, i) => (
                  <tr key={i} className={hover === i ? "hover" : ""} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                    <td>{clock(w.t)}</td>
                    <td>{clock(w.end)}</td>
                    <td>{duration(w.end - w.t)}</td>
                    <td>
                      {layer === "dewards" ? <PlayerHero p={byPID.get(w.pid)} />
                        : w.destroyed ? <span className="loss">снёс {killer(w.killer, w.killer_name)}</span>
                        : <span className="dim">истёк / конец игры</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {layer !== "heat" && wards.length === 0 && <div className="empty">Ничего нет.</div>}
        </div>
      </div>
    </div>
  );
}

function PlayerHero({ p }: { p?: PlayerView }) {
  return p ? <span className="hero-name"><HeroBadge npc={p.hero_name} size={20} /><HeroNameText npc={p.hero_name} /></span> : <span className="dim">—</span>;
}

function HeroNameText({ npc }: { npc: string }) {
  return <>{useNames().hero(npc)}</>;
}

function ShareBar({ label, v, color }: { label: string; v: number; color: string }) {
  return (
    <div className="share">
      <div className="share-label"><span>{label}</span><b>{pct(v)}</b></div>
      <div className="share-bar"><div style={{ width: `${v * 100}%`, background: color }} /></div>
    </div>
  );
}
