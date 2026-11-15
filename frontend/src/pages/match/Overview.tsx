import { useMemo, useState } from "react";
import type { MatchView, PlayerView } from "../../api";
import { clock, kfmt, lowerFirst, num, pct, RADIANT, regionName, upperFirst } from "../../format";
import { useNames } from "../../names";
import { TickButton } from "../../components/ui";
import { heroOf, keyItems, lastOf, objectiveLabel, tickAt, useFocus, useWidth } from "./common";

// ---------------------------------------------------------------- summary line

/** One-line story of the game: role, opponent, lane result, how it went. */
export function summaryLine(view: MatchView, role: string, names: ReturnType<typeof useNames>): string {
  const { me, opp } = focusOf(view);
  if (!me) return "";
  const parts: string[] = [];
  const who = [role, opp ? `против ${names.hero(opp.hero_name)}` : ""].filter(Boolean).join(" ");
  if (who) parts.push(upperFirst(who) + ".");
  const s10 = (view.laning.snaps ?? []).find((s) => s.t === 600);
  const story: string[] = [];
  if (s10?.opp) {
    const d = s10.nw - s10.opp.nw;
    story.push(d >= 500 ? "Выиграл линию" : d <= -500 ? "Проиграл линию" : "Линия вышла ровной");
  }
  // Team advantage from our side.
  const adv = (view.farm.advantage ?? []).map((v) => (v == null ? null : me.team === RADIANT ? v : -v));
  const ts = view.farm.t ?? [];
  const won = me.team === view.meta.winner;
  const minAdv = Math.min(...adv.filter((v): v is number => v != null), 0);
  if (won && minAdv <= -5000) {
    story.push(`отыграли отставание в ${kfmt(-minAdv)}`);
  } else {
    // Last moment the advantage crossed ±5k for good.
    const sign = won ? 1 : -1;
    let at = -1;
    for (let i = adv.length - 1; i >= 0; i--) {
      if (adv[i] == null) continue;
      if (adv[i]! * sign < 5000) break;
      at = i;
    }
    if (at >= 0 && ts[at] > 0) {
      const min = Math.max(1, Math.round(ts[at] / 60));
      story.push(won ? `к ${min}-й минуте разогнали игру` : `к ${min}-й минуте игра ушла`);
    } else story.push(won ? "дожали в равной игре" : "проиграли в равной игре");
  }
  if (story.length) parts.push(upperFirst(story.join(", ")) + ".");
  return parts.join(" ");
}

function focusOf(view: MatchView) {
  const players = view.players ?? [];
  return { me: players.find((p) => p.player_id === view.focus), opp: players.find((p) => p.player_id === view.opponent) };
}

// ---------------------------------------------------------------- overview tab

export function OverviewTab({ view }: { view: MatchView }) {
  return (
    <div>
      <FlowCard view={view} />
      <Moments view={view} />
      <Verdict view={view} />
    </div>
  );
}

// ---------------------------------------------------------------- game flow

const PAD_L = 82;
const PAD_R = 14;
const CHART_H = 210;
const LANE_H = 30;

function FlowCard({ view }: { view: MatchView }) {
  const names = useNames();
  const { me, players } = useFocus(view);
  const [ref, W] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const dur = Math.max(60, view.meta.duration);
  const ts = view.farm.t ?? [];
  const adv = view.farm.advantage ?? [];
  const pts = ts.map((t, i) => [t, adv[i]] as [number, number | null]).filter((p): p is [number, number] => p[0] >= 0 && p[1] != null);
  const innerW = Math.max(100, W - PAD_L - PAD_R);
  const x = (t: number) => PAD_L + (Math.max(0, Math.min(dur, t)) / dur) * innerW;
  const maxAbs = Math.max(5000, ...pts.map((p) => Math.abs(p[1])));
  const yMax = Math.ceil(maxAbs / 5000) * 5000;
  const top = 18, h = CHART_H - top - 10;
  const y = (v: number) => top + (h / 2) * (1 - v / yMax);
  const y0 = y(0);
  const line = pts.map((p, i) => `${i ? "L" : "M"}${x(p[0]).toFixed(1)},${y(p[1]).toFixed(1)}`).join("");
  const area = pts.length ? `${line}L${x(pts[pts.length - 1][0]).toFixed(1)},${y0}L${x(pts[0][0]).toFixed(1)},${y0}Z` : "";
  const yTicks: number[] = [];
  for (let v = -yMax; v <= yMax; v += yMax >= 20000 ? 10000 : 5000) yTicks.push(v);
  const myRadiant = me.team === RADIANT;

  // Lanes.
  const deaths = view.deaths ?? [];
  const kills = deaths.filter((d) => d.killer === view.focus).map((d) => d.t);
  const myDeaths = deaths.filter((d) => d.victim === view.focus).map((d) => d.t);
  const items = keyItems(view, names, 6);
  const objectives = view.objectives ?? [];
  const roshans = objectives.filter((o) => o.kind === "roshan");
  // Barracks fall in bursts: one marker per burst. Lost buildings are drawn
  // unlabelled; labels go to our objectives and Roshan.
  const objMarks = useMemo(() => {
    const out: { t: number; label: string; color: string; ours: boolean; tick: number }[] = [];
    for (const o of objectives) {
      const ours = o.kind === "roshan" ? (o.killer >= 0 && heroOf(players, o.killer)?.team === me.team) : o.team !== me.team;
      const color = o.kind === "roshan" ? "var(--accent)" : ours ? "var(--blue)" : "var(--loss)";
      const prev = out[out.length - 1];
      if (o.kind === "barracks" && prev && prev.label === "Бараки" && prev.ours === ours && o.t - prev.t < 60) continue;
      out.push({ t: o.t, label: objectiveLabel(o.key, o.kind), color, ours, tick: o.tick });
    }
    return out;
  }, [objectives, players, me.team]);

  const labelled = new Set(spacedPick(objMarks.filter((o) => o.ours || o.label === "Рошан")));
  const lanesTop = CHART_H + 26;
  const laneY = (i: number) => lanesTop + i * LANE_H + LANE_H / 2;
  const axisY = lanesTop + 3 * LANE_H + 8;
  const H = axisY + 14;
  const minutes: number[] = [];
  for (let m = 0; m * 60 <= dur - 120; m += 10) minutes.push(m);

  // Stack markers of the "Ты" lane that fall within ~12 s.
  const stacked = (times: number[]) => {
    const out: { t: number; k: number }[] = [];
    times.forEach((t) => {
      const k = out.filter((o) => Math.abs(o.t - t) < 12).length;
      out.push({ t, k });
    });
    return out;
  };
  // Labels that would overlap the previous one are dropped.
  function spacedPick<T extends { t: number }>(xs: T[], gap = 70): T[] {
    let last = -1e9;
    return xs.filter((m) => {
      const show = x(m.t) - last >= gap;
      if (show) last = x(m.t);
      return show;
    });
  }

  const onMove = (e: React.MouseEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    const px = e.clientX - r.left;
    if (px < PAD_L || px > W - PAD_R || !pts.length || e.clientY - r.top > CHART_H) return setHover(null);
    const t = ((px - PAD_L) / innerW) * dur;
    let best = 0;
    for (let i = 1; i < pts.length; i++) if (Math.abs(pts[i][0] - t) < Math.abs(pts[best][0] - t)) best = i;
    setHover(best);
  };
  const hp = hover != null ? pts[hover] : null;

  return (
    <div className="card flow">
      <div className="card-head">
        <h2>Ход игры</h2>
        <span className="legend-inline">
          <span><i style={{ background: "var(--blue)" }} />Преимущество Radiant{myRadiant ? " (твоя команда)" : ""}</span>
          <span><i style={{ background: "var(--loss)" }} />Преимущество Dire{!myRadiant ? " (твоя команда)" : ""}</span>
        </span>
      </div>
      <div ref={ref} style={{ position: "relative" }}>
        {W > 0 && (
          <svg width={W} height={H} onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
            <defs>
              <clipPath id="above"><rect x={0} y={0} width={W} height={y0} /></clipPath>
              <clipPath id="below"><rect x={0} y={y0} width={W} height={CHART_H} /></clipPath>
              <linearGradient id="gBlue" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#4f9df8" stopOpacity=".38" /><stop offset="1" stopColor="#4f9df8" stopOpacity=".05" /></linearGradient>
              <linearGradient id="gRed" x1="0" y1="1" x2="0" y2="0"><stop offset="0" stopColor="#f2705f" stopOpacity=".38" /><stop offset="1" stopColor="#f2705f" stopOpacity=".05" /></linearGradient>
            </defs>
            {yTicks.map((v) => (
              <g key={v}>
                <line x1={PAD_L} x2={W - PAD_R} y1={y(v)} y2={y(v)} stroke={v === 0 ? "#2a2f3a" : "#171a21"} />
                <text className="axis" x={PAD_L - 10} y={y(v) + 3} textAnchor="end">{v === 0 ? "0" : `${v > 0 ? "+" : "−"}${Math.abs(v) / 1000}k`}</text>
              </g>
            ))}
            <path d={area} fill="url(#gBlue)" clipPath="url(#above)" />
            <path d={area} fill="url(#gRed)" clipPath="url(#below)" />
            <path d={line} fill="none" stroke="#4f9df8" strokeWidth={2} clipPath="url(#above)" />
            <path d={line} fill="none" stroke="#f2705f" strokeWidth={2} clipPath="url(#below)" />
            {roshans.map((o, i) => (
              <g key={i}>
                <line x1={x(o.t)} x2={x(o.t)} y1={top - 4} y2={axisY - 12} stroke="var(--accent)" strokeDasharray="3 3" opacity={0.75} />
                <text x={x(o.t) + 6} y={top + 4} fill="var(--accent)" fontSize={11} fontWeight={600}>Рошан</text>
              </g>
            ))}
            {hp && (
              <g>
                <line x1={x(hp[0])} x2={x(hp[0])} y1={top} y2={top + h} stroke="#3a4150" />
                <circle cx={x(hp[0])} cy={y(hp[1])} r={4} fill={hp[1] >= 0 ? "#4f9df8" : "#f2705f"} />
              </g>
            )}

            {["Ты", "Предметы", "Объекты"].map((l, i) => (
              <g key={l}>
                <text x={0} y={laneY(i) + 4} fill="var(--dim)" fontSize={12}>{l}</text>
                <line x1={PAD_L} x2={W - PAD_R} y1={laneY(i)} y2={laneY(i)} stroke="#1c2028" strokeDasharray="2 4" />
              </g>
            ))}
            {stacked(kills).map((k, i) => {
              const cx = x(k.t), cy = laneY(0) - k.k * 9;
              return <path key={`k${i}`} d={`M${cx},${cy - 5}L${cx + 5},${cy + 4}L${cx - 5},${cy + 4}Z`} fill="var(--win)"><title>Убийство {clock(k.t)}</title></path>;
            })}
            {myDeaths.map((t, i) => {
              const cx = x(t), cy = laneY(0);
              return (
                <g key={`d${i}`} stroke="var(--loss)" strokeWidth={2.2} strokeLinecap="round">
                  <title>Смерть {clock(t)}</title>
                  <line x1={cx - 4.5} y1={cy - 4.5} x2={cx + 4.5} y2={cy + 4.5} />
                  <line x1={cx + 4.5} y1={cy - 4.5} x2={cx - 4.5} y2={cy + 4.5} />
                </g>
              );
            })}
            {items.map((it, i) => {
              const cx = x(it.t), cy = laneY(1);
              const up = i % 2 === 0;
              const name = names.inflictor("item_" + it.item).replace(/^Hand of /, "").replace(/ Scepter$/, "");
              return (
                <g key={`i${i}`}>
                  <circle cx={cx} cy={cy} r={4.5} fill="var(--accent)" />
                  <text x={cx + 7} y={up ? cy - 6 : cy + 14} fontSize={11} fontWeight={600} fill="var(--text)">{name} {clock(it.t)}</text>
                </g>
              );
            })}
            {objMarks.map((o) => ({ ...o, show: labelled.has(o) })).map((o, i) => {
              const cx = x(o.t), cy = laneY(2);
              return (
                <g key={`o${i}`}>
                  <title>{o.label} {clock(o.t)}{o.label !== "Рошан" ? (o.ours ? " — снесли мы" : " — потеряли") : ""}</title>
                  <rect x={cx - 4} y={cy - 4} width={8} height={8} rx={1.5} fill={o.color} opacity={o.ours || o.label === "Рошан" ? 1 : 0.55} />
                </g>
              );
            })}
            {objMarks.filter((o) => labelled.has(o)).map((o, i) => (
              <text key={`ol${i}`} x={x(o.t) + 8} y={laneY(2) + 4} fontSize={10.5} fill="var(--dim)" stroke="#111318" strokeWidth={4} paintOrder="stroke">{o.label}</text>
            ))}
            {minutes.map((m) => <text key={m} className="axis" x={x(m * 60)} y={axisY + 6} textAnchor="middle">{m}</text>)}
            <text className="axis" x={x(dur)} y={axisY + 6} textAnchor="end">{Math.round(dur / 60)}</text>
          </svg>
        )}
        {hp && (
          <div className="flow-tip" style={{ left: x(hp[0]), top: y(hp[1]) - 6 }}>
            <span className="mono">{clock(hp[0])}</span> · <b className={hp[1] >= 0 ? "radiant" : "dire"}>{hp[1] >= 0 ? "Radiant" : "Dire"} +{kfmt(Math.abs(hp[1]))}</b>
          </div>
        )}
      </div>
      <div className="legend-inline" style={{ marginTop: 10 }}>
        <span><span className="win">▲</span> Убийство ({kills.length})</span>
        <span><span className="loss">✕</span> Смерть ({myDeaths.length})</span>
        <span><i style={{ background: "var(--blue)" }} />Наши объекты</span>
        <span><i style={{ background: "var(--loss)", opacity: .55 }} />Потерянные</span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- moments

type Moment = { kind: "best" | "worst" | "turn"; t: number; title: string; text: string; tick?: number };

function useMoments(view: MatchView): Moment[] {
  const names = useNames();
  const { me, players } = useFocus(view);
  return useMemo(() => {
    const out: Moment[] = [];
    const heroName = (pid: number) => { const p = heroOf(players, pid); return p ? names.hero(p.hero_name) : ""; };
    const battles = view.battles ?? [];
    // Best: a won fight where I did the most.
    const scored = battles.filter((b) => b.won && (b.my_kills + b.my_assists > 0 || b.my_damage > 0))
      .map((b) => ({ b, s: b.my_kills * 2 + b.my_assists + b.my_damage / 1000 + Math.max(0, b.gold_swing) / 1500 - (b.my_died ? 2 : 0) }))
      .sort((a, b) => b.s - a.s);
    if (scored.length) {
      const b = scored[0].b;
      const combo = (b.my_sources ?? []).filter((s) => s.key).slice(0, 3).map((s) => names.inflictor(s.key));
      const len = Math.max(1, Math.round(b.end - b.start));
      let title = combo.length >= 2 ? combo.join(" → ") : `Драка: ${lowerFirst(regionName(b.region) || "карта")}`;
      if (b.my_kills >= 2) title += `, ${b.my_kills} ${b.my_kills < 5 ? "фрага" : "фрагов"} за ${len} секунд`;
      const text = [
        `${b.allies} на ${b.enemies}, ${b.kills} ${b.kills === 1 ? "убийство" : b.kills < 5 ? "убийства" : "убийств"} у команды.`,
        `Твои ${num(b.my_damage)} урона — ${pct(b.my_share)} от команды`,
        b.gold_swing > 0 ? `и ${num(b.gold_swing, true)} к преимуществу.` : ".",
      ].join(" ").replace(" .", ".");
      out.push({ kind: "best", t: b.start, title, text, tick: b.tick });
    }
    // Worst: the most expensive death of mine.
    const deaths = (view.deaths ?? []).map((d, i) => ({ d, ctx: view.death_ctx?.[i] })).filter(({ d }) => d.victim === view.focus);
    const objectives = view.objectives ?? [];
    const worst = deaths.map(({ d, ctx }) => {
      const lostAfter = objectives.filter((o) => o.kind !== "roshan" && o.team === me.team && o.t >= d.t && o.t - d.t <= 60);
      const s = d.gold_lost + (ctx && !ctx.vision && d.has_pos ? 400 : 0) + lostAfter.length * 500 + (d.buyback ? 0 : 0);
      return { d, ctx, lostAfter, s };
    }).sort((a, b) => b.s - a.s)[0];
    if (worst) {
      const { d, ctx, lostAfter } = worst;
      const where = ctx?.region ? lowerFirst(regionName(ctx.region)) : "";
      const blind = d.has_pos && ctx && !ctx.vision;
      const title = `Умер${where ? ` — ${where}` : ""}${blind ? " без вижна" : ""}`;
      const killers = [d.killer, ...(d.assists ?? [])].filter((p, j, a) => p >= 0 && a.indexOf(p) === j).map(heroName).filter(Boolean);
      const text = [
        killers.length ? `Убили ${killers.join(", ")}.` : `Убил ${names.unit(d.killer_name)}.`,
        d.gold_lost ? `Минус ${num(d.gold_lost)} золота` : "",
        lostAfter.length ? `${d.gold_lost ? " и" : "Следом"} ${lostAfter.length === 1 ? "потерянная " + objectiveLabel(lostAfter[0].key, lostAfter[0].kind) : `${lostAfter.length} потерянных строения`}.` : d.gold_lost ? "." : "",
      ].join(" ").replace(" .", ".").trim();
      out.push({ kind: "worst", t: d.t, title, text, tick: view.death_ctx?.[view.deaths!.indexOf(d)]?.tick });
    }
    // Turning point: the biggest 2-minute swing of our advantage.
    const ts = view.farm.t ?? [];
    const sign = me.team === RADIANT ? 1 : -1;
    const adv = (view.farm.advantage ?? []).map((v) => (v == null ? null : v * sign));
    const won = me.team === view.meta.winner;
    let best = { i: -1, j: -1, d: 0 };
    for (let i = 0; i < ts.length; i++) {
      if (adv[i] == null || ts[i] < 300) continue;
      for (let j = i + 1; j < ts.length && ts[j] - ts[i] <= 120; j++) {
        if (adv[j] == null) continue;
        const d = (adv[j]! - adv[i]!) * (won ? 1 : -1);
        if (d > best.d) best = { i, j, d };
      }
    }
    if (best.i >= 0 && best.d >= 2000) {
      const from = adv[best.i]!, to = adv[best.j]!;
      const t0 = ts[best.i], t1 = ts[best.j];
      const rosh = objectives.find((o) => o.kind === "roshan" && o.t >= t0 - 30 && o.t <= t1 + 10);
      const roshOurs = rosh && rosh.killer >= 0 && heroOf(players, rosh.killer)?.team === me.team;
      const fight = (view.battles ?? []).find((b) => b.start <= t1 && b.end >= t0);
      let title: string;
      if (rosh) title = roshOurs ? "Забрали Рошана, игра поехала" : "Отдали Рошана — игра качнулась";
      else if (fight) title = won ? `Выиграли драку: ${lowerFirst(regionName(fight.region) || "карта")}` : `Проиграли драку: ${lowerFirst(regionName(fight.region) || "карта")}`;
      else title = won ? "Перелом в нашу пользу" : "Здесь игра ушла";
      const mins = Math.max(1, Math.round((t1 - t0) / 60));
      const text = `За ${mins} ${mins === 1 ? "минуту" : "минуты"} преимущество команды ${to > from ? "выросло" : "упало"} с ${kfmt(from, true)} до ${kfmt(to, true)}.`;
      out.push({ kind: "turn", t: t0, title, text, tick: tickAt(view, Math.max(-90, t0 - 10)) });
    }
    return out;
  }, [view, names, me, players]);
}

const MOMENT_LABEL = { best: ["Лучший момент", "win"], worst: ["Худший момент", "loss"], turn: ["Поворот", "accent"] } as const;

function Moments({ view }: { view: MatchView }) {
  const moments = useMoments(view);
  if (!moments.length) return null;
  return (
    <div className="grid3" style={{ marginTop: 14 }}>
      {moments.map((m) => (
        <div key={m.kind} className="card moment">
          <div className="mh"><span className={`caps ${MOMENT_LABEL[m.kind][1]}`}>{MOMENT_LABEL[m.kind][0]}</span><span className="mono">{clock(m.t)}</span></div>
          <h3>{m.title}</h3>
          <p>{m.text}</p>
          <TickButton tick={m.tick} />
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------- what went well / to improve

type Point = { title: string; sub: string; w: number };

function Verdict({ view }: { view: MatchView }) {
  const names = useNames();
  const { me, opp } = useFocus(view);
  const { good, bad } = useMemo(() => verdict(view, me, opp, names), [view, me, opp, names]);
  if (!good.length && !bad.length) return null;
  return (
    <div className="grid2" style={{ marginTop: 14 }}>
      <div className="card verdict good">
        <h2 className="win">Что зашло</h2>
        {good.length ? <ul>{good.map((p, i) => <li key={i}><span className="mk">+</span><div><b>{p.title}</b><span>{p.sub}</span></div></li>)}</ul>
          : <div className="dim small">Ничего выдающегося — средняя игра.</div>}
      </div>
      <div className="card verdict bad">
        <h2 className="loss">Что подтянуть</h2>
        {bad.length ? <ul>{bad.map((p, i) => <li key={i}><span className="mk">−</span><div><b>{p.title}</b><span>{p.sub}</span></div></li>)}</ul>
          : <div className="dim small">Явных провалов нет.</div>}
      </div>
    </div>
  );
}

function verdict(view: MatchView, me: PlayerView, opp: PlayerView | undefined, names: ReturnType<typeof useNames>) {
  const good: Point[] = [], bad: Point[] = [];
  const heroName = names.hero(me.hero_name);
  // Item timings against my average on the hero.
  for (const it of keyItems(view, names, 8)) {
    if (it.diff == null || it.avg == null) continue;
    const name = names.inflictor("item_" + it.item);
    const d = Math.round(Math.abs(it.diff));
    const dd = d >= 60 ? clock(d) : `${d} секунд`;
    if (it.diff <= -30) good.push({ title: `${name} на ${dd} раньше среднего`, sub: `${clock(it.t)} против твоих обычных ${clock(it.avg)}`, w: d / 30 });
    if (it.diff >= 45) bad.push({ title: `${name} на ${dd} позже среднего`, sub: `${clock(it.t)} против твоих обычных ${clock(it.avg)}`, w: d / 30 });
  }
  // Ability hit rates.
  const uses = (view.abilities.uses ?? []).filter((u) => !u.is_item && u.hit_rate != null && u.casts >= 5);
  const bestHit = [...uses].sort((a, b) => b.hit_rate! - a.hit_rate!)[0];
  if (bestHit && bestHit.hit_rate! >= 0.7) good.push({ title: `${names.inflictor(bestHit.ability)} попал ${bestHit.hits} из ${bestHit.casts}`, sub: `${pct(bestHit.hit_rate!)} попаданий по героям`, w: 2 });
  const worstHit = [...uses].sort((a, b) => a.hit_rate! - b.hit_rate!)[0];
  if (worstHit && worstHit.hit_rate! < 0.5 && worstHit !== bestHit)
    bad.push({ title: `${names.inflictor(worstHit.ability)}: ${worstHit.hits} попаданий из ${worstHit.casts}`, sub: `${pct(worstHit.hit_rate!)} — каст без урона или дебаффа по героям`, w: 2.5 });
  // Laning.
  const s10 = (view.laning.snaps ?? []).find((s) => s.t === 600);
  const laneDeaths = (view.laning.deaths ?? []).length;
  const nwDiff = s10?.opp ? s10.nw - s10.opp.nw : null;
  if (laneDeaths === 0 && s10) good.push({ title: "0 смертей на линии", sub: nwDiff != null && opp ? `и ${num(nwDiff, true)} нетворса к ${names.hero(opp.hero_name)} на 10-й минуте` : "чистая линия", w: nwDiff != null && nwDiff > 0 ? 3 : 1.5 });
  if (laneDeaths >= 2) bad.push({ title: `${laneDeaths} смерти на линии`, sub: nwDiff != null ? `${num(nwDiff, true)} нетворса к оппоненту на 10-й минуте` : "линия проседает с самого начала", w: laneDeaths });
  else if (nwDiff != null && nwDiff <= -800 && opp) bad.push({ title: `Отстал от ${names.hero(opp.hero_name)} на линии`, sub: `${num(nwDiff, true)} нетворса к 10-й минуте`, w: 2 });
  // Fights.
  if (view.fights.kp >= 0.7) good.push({ title: `Участие в ${pct(view.fights.kp)} убийств команды`, sub: `${me.kills + me.assists} из ${view.fights.team_kills}`, w: 1.5 });
  // Idle.
  const idle = view.gold.idle ?? [];
  if (view.gold.idle_total >= 120) {
    const longest = [...idle].sort((a, b) => (b.to - b.from) - (a.to - a.from))[0];
    bad.push({
      title: `${clock(view.gold.idle_total)} без фарма`,
      sub: longest ? `Самый долгий отрезок: ${clock(longest.from)}–${clock(longest.to)}${longest.region ? `, ${lowerFirst(regionName(longest.region))}` : ""}` : "",
      w: view.gold.idle_total / 90,
    });
  }
  // Deaths without vision.
  const deaths = (view.deaths ?? []).map((d, i) => ({ d, ctx: view.death_ctx?.[i] })).filter(({ d }) => d.victim === view.focus && d.has_pos);
  const blind = deaths.filter(({ ctx }) => ctx && !ctx.vision);
  if (blind.length >= 2) {
    const zones = [...new Set(blind.map(({ ctx }) => lowerFirst(regionName(ctx!.region))).filter(Boolean))].slice(0, 2);
    bad.push({ title: `${blind.length} из ${deaths.length} смертей без вижна`, sub: zones.length ? `${upperFirst(zones.join(" и "))}` : "", w: blind.length * 1.2 });
  }
  // OpenDota benchmark.
  const gpmB = me.od?.benchmarks?.gold_per_min;
  if (gpmB && gpmB.pct >= 0.8) good.push({ title: `GPM лучше, чем у ${Math.round(gpmB.pct * 100)}% игроков на ${heroName}`, sub: `${me.gpm} GPM, по данным OpenDota`, w: 1.8 });
  const nw = lastOf(view.farm.net_worth.focus);
  const oppNw = opp ? lastOf(view.farm.net_worth.opponent) : null;
  if (nw != null && oppNw != null && nw - oppNw >= 4000 && opp)
    good.push({ title: `Обогнал ${names.hero(opp.hero_name)} на ${kfmt(nw - oppNw)} нетворса`, sub: `к концу игры, ${clock(view.meta.duration)}`, w: 1.6 });
  const top = (xs: Point[]) => xs.sort((a, b) => b.w - a.w).slice(0, 3);
  return { good: top(good), bad: top(bad) };
}
