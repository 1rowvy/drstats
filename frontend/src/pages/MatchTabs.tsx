import { useMemo, useState } from "react";
import type { AbilityUse, Amount, MatchView, PlayerView } from "../api";
import {
  clock, duration, GOLD_SOURCES, laneName, pct, RUNES, signed, signedClock, SOURCE_KINDS, thousands, UNIT_CATEGORIES,
} from "../format";
import { useNames } from "../names";
import { HeroBadge, HeroName } from "../components/HeroBadge";
import { BarChart, SERIES_COLORS, TimeChart } from "../components/Chart";
import { MapCanvas, MapPoint } from "../components/MapCanvas";

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

// ---------------------------------------------------------------- laning

export function LaningTab({ view }: { view: MatchView }) {
  const names = useNames();
  const { me, opp } = useFocus(view);
  const l = view.laning;
  return (
    <div>
      <div className="stat-row">
        <Stat label="Линия" value={laneName(l.lane, me.team)} />
        <Stat label="Оппонент" value={opp ? <HeroName npc={opp.hero_name} /> : "не определён"} sub={opp ? laneName(l.opp_lane, opp.team) : ""} />
        <Stat label="До 10:00" value={`${l.kills} / ${(l.deaths ?? []).length} / ${l.assists}`} sub="убийства / смерти / помощь" />
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr><th>Время</th><th>LH</th><th>DN</th><th>Net worth</th><th>Опыт</th><th>Уровень</th></tr>
          </thead>
          <tbody>
            {(l.snaps ?? []).map((s) => (
              <tr key={s.t}>
                <td><b>{clock(s.t)}</b></td>
                <td>{s.lh} {s.opp && <span className="dim">vs {s.opp.lh} (<Diff v={s.lh - s.opp.lh} />)</span>}</td>
                <td>{s.dn} {s.opp && <span className="dim">vs {s.opp.dn} (<Diff v={s.dn - s.opp.dn} />)</span>}</td>
                <td className="gold">{thousands(s.nw)} {s.opp && <span className="dim">vs {thousands(s.opp.nw)} (<Diff v={s.nw - s.opp.nw} />)</span>}</td>
                <td>{thousands(s.xp)} {s.opp && <span className="dim">vs {thousands(s.opp.xp)} (<Diff v={s.xp - s.opp.xp} />)</span>}</td>
                <td>{s.level} {s.opp && <span className="dim">vs {s.opp.level}</span>}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!opp && <div className="dim small">Без оппонента сравнение недоступно.</div>}
      </div>
      <div className="card">
        <h2>Смерти на линии</h2>
        {(l.deaths ?? []).length === 0 ? <div className="dim">Не умирал до 10:00.</div> : (
          <table className="table">
            <thead><tr><th>Время</th><th>Убийца</th><th>Чем</th><th>Потеряно</th></tr></thead>
            <tbody>
              {(l.deaths ?? []).map((d, i) => {
                const killer = view.players?.find((p) => p.player_id === d.killer);
                return (
                  <tr key={i}>
                    <td>{clock(d.t)}</td>
                    <td>{killer ? <HeroName npc={killer.hero_name} /> : names.unit(d.killer_name)}</td>
                    <td className="dim">{names.inflictor(d.inflictor)}</td>
                    <td className="gold">{d.gold_lost || "—"}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
      <div className="dim small">Линия и оппонент определяются по позициям на 1–10 минуте (приблизительно).</div>
    </div>
  );
}

function Stat({ label, value, sub }: { label: string; value: React.ReactNode; sub?: React.ReactNode }) {
  return (
    <div className="stat">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
    </div>
  );
}

// ---------------------------------------------------------------- farm & gold

export function FarmTab({ view }: { view: MatchView }) {
  const names = useNames();
  const { me, opp } = useFocus(view);
  const [hover, setHover] = useState<number | null>(null);
  const t = view.farm.t ?? [];
  const lines = (m: { focus: any; team_avg: any; opponent: any }) => [
    { name: names.hero(me.hero_name), data: m.focus, color: SERIES_COLORS.focus },
    { name: "Среднее по команде", data: m.team_avg, color: SERIES_COLORS.team, dashed: true },
    ...(opp ? [{ name: `${names.hero(opp.hero_name)} (оппонент)`, data: m.opponent, color: SERIES_COLORS.opponent }] : []),
  ];
  const g = view.gold;
  const sources = (g.sources ?? []).filter((s) => s.amount > 0);
  const total = sources.reduce((a, s) => a + s.amount, 0);
  const idle = g.idle ?? [];
  const minutes = view.meta.duration / 60;
  return (
    <div>
      <div className="grid2">
        <div className="card"><TimeChart title="Net worth" t={t} lines={lines(view.farm.net_worth)} /></div>
        <div className="card"><TimeChart title="GPM" t={t} lines={lines(view.farm.gpm)} /></div>
        <div className="card"><TimeChart title="XPM" t={t} lines={lines(view.farm.xpm)} /></div>
        <div className="card"><TimeChart title="Last hits" t={t} lines={lines(view.farm.last_hits)} /></div>
      </div>
      <div className="grid2">
        <div className="card">
          <BarChart
            title={`Источники золота (всего ${thousands(total)})`} horizontal height={280}
            bars={sources.map((s) => ({ label: GOLD_SOURCES[s.key] ?? s.key, value: s.amount, tip: pct(s.amount / total) }))}
          />
          <div className="dim small">Потеряно при смертях: <span className="gold">{g.gold_lost}</span></div>
        </div>
        <div className="card">
          <TimeChart title="Фарм: крипы и нейтралы (накопительно)" t={t} height={280} lines={[
            { name: "Крипы на линии", data: g.creep, color: SERIES_COLORS.focus },
            { name: "Нейтралы", data: g.neutral, color: "#f2c14e" },
          ]} />
        </div>
      </div>
      <div className="card">
        <h2>Простои</h2>
        <div className="dim small" style={{ marginBottom: 10 }}>
          Отрезки ≥ {g.threshold} с, когда герой жив, но не добивал крипов, не бил и не получал урон (порог — в настройках).
          Всего: <b>{duration(g.idle_total)}</b> ({pct(g.idle_total / 60 / Math.max(1, minutes))} игры).
        </div>
        {idle.length === 0 ? <div className="dim">Простоев нет.</div> : (
          <div className="deaths-layout">
            <div className="map-card">
              <MapCanvas size={360} world={view.world} onHover={setHover}
                points={idle.map((p, i) => ({ x: p.x, y: p.y, color: "#f2c14e", highlight: hover === i }))} />
            </div>
            <div className="list-card" style={{ maxHeight: 380 }}>
              <table className="table">
                <thead><tr><th>С</th><th>По</th><th>Длительность</th></tr></thead>
                <tbody>
                  {idle.map((p, i) => (
                    <tr key={i} className={hover === i ? "hover" : ""} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                      <td>{clock(p.from)}</td><td>{clock(p.to)}</td><td>{duration(p.to - p.from)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
      {!opp && <div className="dim small">Лайн-оппонента определить не удалось.</div>}
    </div>
  );
}

// ---------------------------------------------------------------- items

const CONSUMABLES = new Set([
  "tango", "flask", "clarity", "ward_observer", "ward_sentry", "tpscroll", "smoke_of_deceit", "dust", "enchanted_mango",
  "faerie_fire", "blood_grenade", "tome_of_knowledge", "famango", "great_famango", "greater_famango", "branches",
]);

export function ItemsTab({ view }: { view: MatchView }) {
  const names = useNames();
  const [all, setAll] = useState(false);
  const items = (view.items ?? []).filter((it) => all || !CONSUMABLES.has(it.item));
  const cost = (k: string) => names.raw.items?.[k]?.cost ?? 0;
  return (
    <div className="card">
      <div className="page-head">
        <h2 style={{ margin: 0 }}>Покупки</h2>
        <label className="dim"><input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} /> показывать расходники</label>
      </div>
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
      <div className="dim small">
        Стартовые предметы, купленные во время выбора героев, в реплей не попадают. Жирным — предметы от 1800 золота.
        Средний тайминг — по первой покупке в других моих матчах на этом герое.
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- deaths extras used in MatchPage

export function deathExtras(d: { gold_lost: number; buyback: boolean }) {
  return (
    <>
      {d.gold_lost ? <span className="gold">−{d.gold_lost}</span> : <span className="dim">—</span>}
      {d.buyback && <span className="badge warn" style={{ marginLeft: 6 }}>байбек</span>}
    </>
  );
}

// ---------------------------------------------------------------- fights

export function FightsTab({ view }: { view: MatchView }) {
  const names = useNames();
  const { me } = useFocus(view);
  const f = view.fights;
  const heroBars = (xs: Amount[] | null) => (xs ?? []).map((a) => ({ label: names.hero(a.key), value: a.amount, tip: `${a.count} попаданий` }));
  const unitBars = (xs: Amount[] | null) => (xs ?? []).map((a) => ({ label: UNIT_CATEGORIES[a.key] ?? names.unit(a.key), value: a.amount }));
  const srcBars = (xs: Amount[] | null) => (xs ?? []).map((a) => ({ label: SOURCE_KINDS[a.key] ?? a.key, value: a.amount }));
  return (
    <div>
      <div className="stat-row">
        <Stat label="Участие в убийствах" value={pct(f.kp)} sub={`${me.kills + me.assists} из ${f.team_kills} убийств команды`} />
        <Stat label="Урон героям" value={thousands(f.hero_damage)} sub={`за минуту: ${Math.round(f.hero_damage / Math.max(1, view.meta.duration / 60))}`} />
        <Stat label="Урон строениям" value={thousands(f.tower_damage)} />
        <Stat label="Лечение" value={thousands(f.heal_total)} />
        <Stat label="Оглушения" value={`${f.stuns.toFixed(1)} с`} />
      </div>
      <div className="grid2">
        <div className="card"><BarChart title="Урон по героям" horizontal bars={heroBars(f.dmg_out_heroes)} height={220} /></div>
        <div className="card"><BarChart title="Получено от героев" horizontal bars={heroBars(f.dmg_in_heroes)} height={220} /></div>
        <div className="card"><BarChart title="Урон героям по источнику" horizontal bars={srcBars(f.dmg_out_source)} height={160} /></div>
        <div className="card"><BarChart title="Получено от героев по источнику" horizontal bars={srcBars(f.dmg_in_source)} height={160} /></div>
        <div className="card"><BarChart title="Урон по прочим целям" horizontal bars={unitBars(f.dmg_out_units)} height={200} /></div>
        <div className="card"><BarChart title="Получено от прочих" horizontal bars={unitBars(f.dmg_in_units)} height={200} /></div>
      </div>
      <div className="grid2">
        <InflictorTable title="Мой урон героям по способностям и предметам" rows={f.dmg_out_by_inflictor} />
        <InflictorTable title="Чем били меня" rows={f.dmg_in_by_inflictor} />
      </div>
      {(f.heal_targets ?? []).length > 0 && (
        <div className="card">
          <BarChart title="Лечение по целям" horizontal height={200}
            bars={(f.heal_targets ?? []).map((a) => ({ label: a.key.startsWith("npc_dota_hero_") ? names.hero(a.key) : UNIT_CATEGORIES[a.key] ?? a.key, value: a.amount }))} />
        </div>
      )}
      <div className="dim small">Урон и лечение — из комбат-лога, включая иллюзии и призванных существ.</div>
    </div>
  );
}

function InflictorTable({ title, rows }: { title: string; rows: Amount[] | null }) {
  const names = useNames();
  const xs = rows ?? [];
  const total = xs.reduce((a, r) => a + r.amount, 0);
  return (
    <div className="card list-card" style={{ maxHeight: 420 }}>
      <h2>{title}</h2>
      <table className="table">
        <thead><tr><th>Источник</th><th>Урон</th><th>Доля</th><th>Попаданий</th></tr></thead>
        <tbody>
          {xs.map((r) => (
            <tr key={r.key}><td>{names.inflictor(r.key)}</td><td>{thousands(r.amount)}</td><td className="dim">{pct(r.amount / total)}</td><td className="dim">{r.count}</td></tr>
          ))}
        </tbody>
      </table>
      {xs.length === 0 && <div className="dim">Нет данных.</div>}
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
        <h2>Способности</h2>
        <table className="table">{head}<tbody>{uses.filter((u) => !u.is_item).map(row)}</tbody></table>
      </div>
      <div className="card">
        <h2>Активные предметы</h2>
        <table className="table">{head}<tbody>{uses.filter((u) => u.is_item).map(row)}</tbody></table>
      </div>
      <div className="notice small">
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
      <div className="stat-row">
        <Stat label="Обсерверы" value={v.obs_placed} sub={v.obs_placed ? `живут в среднем ${duration(v.obs_avg_life)}` : ""} />
        <Stat label="Сентри" value={v.sen_placed} />
        <Stat label="Мои варды снесли" value={v.my_lost} />
        <Stat label="Я снёс вардов" value={(v.destroyed ?? []).length} />
        <Stat label="Стаки" value={(v.stacks ?? []).length} sub={(v.stacks ?? []).map(clock).join(", ")} />
        <Stat label="Руны" value={(v.runes ?? []).length} />
      </div>
      <div className="deaths-layout">
        <div className="card map-card">
          <div className="seg" style={{ marginBottom: 10 }}>
            <button className={layer === "heat" ? "on" : ""} onClick={() => setLayer("heat")}>Перемещения</button>
            <button className={layer === "obs" ? "on" : ""} onClick={() => setLayer("obs")}>Обсерверы</button>
            <button className={layer === "sentry" ? "on" : ""} onClick={() => setLayer("sentry")}>Сентри</button>
            <button className={layer === "dewards" ? "on" : ""} onClick={() => setLayer("dewards")}>Снесённые мной</button>
          </div>
          <MapCanvas world={view.world} points={points} heat={layer === "heat" ? v.heat ?? [] : undefined} onHover={setHover} />
          <div className="time-share">
            <h2>Где проводил время</h2>
            <ShareBar label="Своя база" v={share.base} color="#9aa3b5" />
            <ShareBar label={`Своя линия (${laneName(v.lane, me.team)})`} v={share.lane} color="#5b9cff" />
            <ShareBar label="Свой лес и прочее" v={share.jungle} color="#5fd17a" />
            <ShareBar label="Половина врага" v={share.enemy} color="#ff6b5b" />
            <div className="dim small">По позиции раз в 5 с, только живым; границы зон приблизительные.</div>
          </div>
        </div>
        <div className="card list-card">
          {layer === "heat" ? (
            <>
              <h2>Руны</h2>
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
