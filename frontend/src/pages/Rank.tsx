import { useEffect, useMemo, useState } from "react";
import { Browser } from "@wailsio/runtime";
import type { EChartsOption } from "echarts";
import { Api, RankBucket, RankMark, RankPoint, RankView } from "../api";
import { EChart } from "../components/EChart";
import { HeroBadge } from "../components/HeroBadge";
import { lowerFirst, pct, plural, rankName, relDate, signed } from "../format";
import { useNames } from "../names";

const PER_GAME = 25; // stats.MMRPerGame
const PERIODS = [
  { days: 30, label: "30 дней" },
  { days: 90, label: "3 месяца" },
  { days: 365, label: "Год" },
  { days: 0, label: "Всё время" },
];
const PERIOD_KEY = "rank.days";
const LANES: Record<string, string> = { safe: "Лёгкая линия", mid: "Мид", off: "Сложная линия", jungle: "Лес", roam: "Роум" };
const PARTY: Record<string, string> = { solo: "Соло", party: "В пати" };

// Star boundaries (MMR where each star starts), mirrors stats.TierMMR.
const STARS: { mmr: number; tier: number }[] = (() => {
  const out: { mmr: number; tier: number }[] = [];
  for (let medal = 1; medal <= 6; medal++) for (let star = 1; star <= 5; star++) out.push({ mmr: ((medal - 1) * 5 + star - 1) * 154, tier: medal * 10 + star });
  for (let star = 1; star <= 5; star++) out.push({ mmr: 4620 + (star - 1) * 200, tier: 70 + star });
  out.push({ mmr: 5620, tier: 80 });
  return out;
})();

function loadDays(): number {
  try { const v = Number(localStorage.getItem(PERIOD_KEY)); return Number.isFinite(v) && localStorage.getItem(PERIOD_KEY) != null ? v : 90; } catch { return 90; }
}

type Lists = "points" | "marks" | "by_hero" | "by_lane" | "by_party" | "by_period";
/** RankView with the lists made non-null (Go nil slices arrive as null). */
type View = Omit<RankView, Lists> & {
  points: RankPoint[]; marks: RankMark[]; by_hero: RankBucket[]; by_lane: RankBucket[]; by_party: RankBucket[]; by_period: RankBucket[];
};
const norm = (r: RankView): View => ({
  ...r, points: r.points ?? [], marks: r.marks ?? [], by_hero: r.by_hero ?? [], by_lane: r.by_lane ?? [],
  by_party: r.by_party ?? [], by_period: r.by_period ?? [],
});

const mmr = (v: number) => Math.round(v).toLocaleString("ru-RU");
const games = (n: number) => `${n} ${plural(n, "игра", "игры", "игр")}`;

export function RankPage({ tick, onMatch }: { tick: number; onMatch: (id: number) => void }) {
  const [days, setDaysState] = useState(loadDays);
  const setDays = (d: number) => {
    setDaysState(d);
    try { localStorage.setItem(PERIOD_KEY, String(d)); } catch { /* per-viewer convenience only */ }
  };
  const [v, setV] = useState<View | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    Api.Rank(days).then((r) => { setV(r ? norm(r) : null); setError(""); }).catch((e) => setError(String(e?.message ?? e)));
  }, [days, tick]);
  if (error) return <div className="notice bad">{error}</div>;
  if (!v) return <div className="dim">Загрузка…</div>;
  if (v.unknown) return <div className="notice">Укажи свой Steam ID на странице «Импорт» — ранг строится по твоим матчам.</div>;
  const seg = (
    <div className="seg">
      {PERIODS.map((p) => <button key={p.days} className={days === p.days ? "on" : ""} onClick={() => setDays(p.days)}>{p.label}</button>)}
    </div>
  );
  if (v.total === 0) {
    return <div className="card empty">Рейтинговых игр пока нет: ни в реплеях, ни в истории OpenDota. История подтягивается на «Дашборде» (режим «Все игры»).</div>;
  }
  return (
    <div>
      <Head v={v} seg={seg} />
      <div className="card" style={{ marginTop: 14 }}>
        <div className="card-head">
          <h2>Рейтинг</h2>
          <span className="meta">{v.anchored ? `оценка: ±${PER_GAME} за игру, привязана к последней медали` : `без медали: изменение от начала, ±${PER_GAME} за игру`}</span>
        </div>
        {v.points.length === 0 ? <div className="dim">За этот период рейтинговых игр нет.</div> : <Curve v={v} onMatch={onMatch} />}
        <Insights v={v} />
      </div>
      {v.by_period.length > 1 && (
        <div className="card" style={{ marginTop: 14 }}>
          <div className="card-head"><h2>{v.weekly ? "По неделям" : "По месяцам"}</h2><span className="meta">MMR за период</span></div>
          <PeriodBars v={v} />
        </div>
      )}
      <div className="grid2" style={{ marginTop: 14 }}>
        <div className="card">
          <div className="card-head"><h2>Герои</h2><span className="meta">от {3} игр · MMR за период</span></div>
          <HeroNet buckets={v.by_hero} />
        </div>
        <div className="card">
          <div className="card-head"><h2>Линия и пати</h2><span className="meta">MMR за период</span></div>
          <NetList buckets={v.by_lane} label={(k) => LANES[k] ?? k} empty="Линия известна только по реплеям и истории OpenDota — пока мало игр." />
          {v.by_party.length > 0 && <div style={{ marginTop: 18 }}><NetList buckets={v.by_party} label={(k) => PARTY[k] ?? k} empty="" /></div>}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- head

function Head({ v, seg }: { v: View; seg: React.ReactNode }) {
  const wr = v.games ? v.wins / v.games : 0;
  const toNext = v.anchored && v.next_at > 0 ? Math.max(1, Math.ceil((v.next_at - v.mmr) / PER_GAME)) : 0;
  const tile = (label: string, val: React.ReactNode, cls = "", sub = "") => (
    <div className="tile">
      <div className="label">{label}</div>
      <div className={`val ${cls}`}>{val}</div>
      {sub && <div className="faint small" style={{ marginTop: 3 }}>{sub}</div>}
    </div>
  );
  const tone = (n: number) => (n > 0 ? "win" : n < 0 ? "loss" : "");
  return (
    <div className="card dash-hero">
      <div style={{ position: "relative" }}>
        <div className="caps" style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>{seg}</div>
        <h1 style={{ marginTop: 12 }}>
          {v.tier ? rankName(v.tier) : "Ранг"}
          {v.leaderboard > 0 && <span className="dim" style={{ fontSize: "0.5em", marginLeft: 10 }}>#{v.leaderboard}</span>}
        </h1>
        <div className="dim" style={{ marginTop: -6, marginBottom: 12 }}>
          {v.tier
            ? <>медаль видна {lowerFirst(relDate(v.tier_at))}{v.peak_tier > v.tier ? ` · лучшая — ${rankName(v.peak_tier)}` : ""}
              {v.est_tier && v.est_tier !== v.tier ? ` · по результатам после — уже ${rankName(v.est_tier)}` : ""}</>
            : "Медаль неизвестна: OpenDota её не отдаёт, а в реплеях её нет. Кривая показывает только изменение."}
        </div>
        <div className="tiles4">
          {tile(v.anchored ? "Оценка MMR" : "Изменение всего", v.anchored ? `≈ ${mmr(v.mmr)}` : signed(v.mmr), "",
            toNext ? `до ${rankName(v.est_tier % 10 >= 5 ? nextTier(v.est_tier) : v.est_tier + 1)} ≈ ${toNext} ${plural(toNext, "победа", "победы", "побед")} в плюс` : "")}
          {tile("За 7 дней", signed(v.net7), tone(v.net7))}
          {tile("За 30 дней", signed(v.net30), tone(v.net30))}
          {tile("За период", signed(v.net), tone(v.net), `${games(v.games)} · ${pct(wr)} побед`)}
        </div>
      </div>
    </div>
  );
}

function nextTier(t: number): number {
  const medal = Math.floor(t / 10);
  return medal >= 7 ? 80 : (medal + 1) * 10 + 1;
}

// ---------------------------------------------------------------- curve

function Curve({ v, onMatch }: { v: View; onMatch: (id: number) => void }) {
  const names = useNames();
  const option = useMemo<EChartsOption>(() => {
    const ys = v.points.map((p) => p.mmr).concat(v.marks.map((m) => m.mmr));
    const lo = Math.min(...ys), hi = Math.max(...ys);
    // Star lines inside the visible range; only medal starts when there are many.
    let lines = v.anchored ? STARS.filter((s) => s.mmr > lo - 40 && s.mmr < hi + 40) : [];
    if (lines.length > 8) lines = lines.filter((s) => s.tier % 10 === 1 || s.tier === 80);
    const peakIdx = v.points.findIndex((p) => p.mmr === v.peak);
    const longSpan = v.points.length > 1 && v.points[v.points.length - 1].end_time - v.points[0].end_time > 300 * 86400;
    return {
      backgroundColor: "transparent",
      animation: false,
      grid: { left: 56, right: 92, top: 16, bottom: 48 },
      tooltip: {
        trigger: "axis",
        backgroundColor: "#1f232c",
        borderColor: "#2a2f3a",
        textStyle: { color: "#eceef3" },
        formatter: (ps: any) => {
          const arr = Array.isArray(ps) ? ps : [ps];
          const i = arr[0]?.dataIndex ?? 0;
          const p = v.points[i];
          if (!p) return "";
          const mark = v.marks.find((m) => m.index === i);
          return [
            `<b>${names.hero(p.hero_name)}</b> · <span style="color:${p.win ? "#3fd68a" : "#f2705f"}">${p.win ? "победа" : "поражение"}</span>`,
            new Date(p.end_time * 1000).toLocaleString("ru-RU", { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" }),
            v.anchored ? `≈ ${mmr(p.mmr)} MMR` : `${signed(p.mmr)} от начала`,
            mark ? `медаль: ${rankName(mark.tier)}${mark.profile ? " (профиль)" : ""}` : "",
            p.opendota ? `<span style="color:#5f6676">без реплея — откроется OpenDota</span>` : "",
          ].filter(Boolean).join("<br/>");
        },
      },
      xAxis: {
        type: "category",
        data: v.points.map((p) => new Date(p.end_time * 1000).toLocaleDateString("ru-RU", longSpan
          ? { month: "short", year: "2-digit" } : { day: "numeric", month: "short" })),
        boundaryGap: false,
        axisLine: { lineStyle: { color: "#2a2f3a" } },
        axisLabel: { color: "#5f6676" },
      },
      yAxis: {
        type: "value",
        scale: true,
        splitLine: { lineStyle: { color: "#1a1e26" } },
        axisLabel: { color: "#5f6676" },
      },
      dataZoom: v.points.length > 60
        ? [{ type: "inside" }, { type: "slider", height: 16, bottom: 8, borderColor: "#2a2f3a", textStyle: { color: "#5f6676" } }]
        : [],
      series: ([
        {
          name: "MMR",
          type: "line",
          data: v.points.map((p) => p.mmr),
          showSymbol: v.points.length <= 60,
          symbolSize: 5,
          itemStyle: { color: "#ffad33" },
          lineStyle: { color: "#ffad33", width: 2 },
          areaStyle: { color: "rgba(255,173,51,.07)", origin: "start" },
          markLine: lines.length ? {
            silent: true,
            symbol: "none",
            lineStyle: { color: "#2c313c", type: "dashed" },
            label: { color: "#5f6676", position: "end", formatter: (p: any) => rankName(lines[p.dataIndex]?.tier) },
            data: lines.map((s) => ({ yAxis: s.mmr })),
          } : undefined,
          markPoint: peakIdx >= 0 && v.points.length > 1 ? {
            symbol: "pin",
            symbolSize: 34,
            itemStyle: { color: "#3fd68a" },
            label: { color: "#0b0c10", fontSize: 9, formatter: "пик" },
            data: [{ coord: [peakIdx, v.peak] }],
          } : undefined,
        },
        ...(v.anchored && v.marks.length ? [{
          name: "Медаль",
          type: "scatter" as const,
          data: v.marks.map((m) => [m.index, m.mmr]),
          symbol: "diamond",
          symbolSize: 9,
          itemStyle: { color: "#4f9df8" },
          tooltip: { show: false },
        }] : []),
      ] as any),
    };
  }, [v, names]);
  const open = (i: number) => {
    const p = v.points[i];
    if (!p) return;
    if (p.opendota) Browser.OpenURL(`https://www.opendota.com/matches/${p.match_id}`).catch(console.error);
    else onMatch(p.match_id);
  };
  return (
    <>
      <EChart option={option} height={320} onClick={open} />
      {v.anchored && v.marks.length > 0 && <div className="faint small">◆ — медаль, которую видели в матче или в профиле OpenDota; линия — оценка по победам и поражениям</div>}
    </>
  );
}

// ---------------------------------------------------------------- insights

function Insights({ v }: { v: View }) {
  const names = useNames();
  const notes: string[] = [];
  if (v.games >= 5) {
    const down = v.peak - v.mmr;
    if (down >= 3 * PER_GAME) notes.push(`Ты на ${mmr(down)} ниже пика за период (${lowerFirst(relDate(v.peak_at))}) — это ${Math.round(down / PER_GAME)} ${plural(Math.round(down / PER_GAME), "победа", "победы", "побед")} в плюс.`);
    else if (v.points.length > 1 && down === 0) notes.push("Ты сейчас на пике за период.");
  }
  if (v.streak <= -3) notes.push(`${-v.streak} ${plural(-v.streak, "поражение", "поражения", "поражений")} подряд. Самое время остановиться — серии поражений чаще продолжаются, чем обрываются.`);
  else if (v.streak >= 3) notes.push(`${v.streak} ${plural(v.streak, "победа", "победы", "побед")} подряд — хороший момент.`);
  if (v.worst_run >= 4) notes.push(`Худшая серия за период — ${v.worst_run} ${plural(v.worst_run, "поражение", "поражения", "поражений")} подряд (−${v.worst_run * PER_GAME}), лучшая — ${v.best_run} ${plural(v.best_run, "победа", "победы", "побед")}.`);
  const heroes = v.by_hero.filter((b) => b.games >= 5);
  const worst = heroes.length ? heroes[heroes.length - 1] : null;
  const best = heroes.length ? heroes[0] : null;
  if (best && best.net > 0) notes.push(`Больше всего MMR приносит ${names.hero(best.key)}: ${signed(best.net)} за ${games(best.games)}.`);
  if (worst && worst.net < 0 && worst !== best) notes.push(`Больше всего теряешь на ${names.hero(worst.key)}: ${signed(worst.net)} за ${games(worst.games)}. Убери из рейтинга или разбери эти игры.`);
  const solo = v.by_party.find((b) => b.key === "solo"), party = v.by_party.find((b) => b.key === "party");
  if (solo && party && solo.games >= 5 && party.games >= 5) {
    const d = Math.round((solo.wins / solo.games - party.wins / party.games) * 100);
    if (Math.abs(d) >= 6) notes.push(`${d > 0 ? "Соло" : "В пати"} выигрываешь на ${Math.abs(d)}% чаще (${pct(solo.wins / solo.games)} соло против ${pct(party.wins / party.games)} в пати).`);
  }
  const lanes = v.by_lane.filter((b) => b.games >= 5);
  if (lanes.length > 1 && lanes[0].net > 0 && lanes[lanes.length - 1].net < 0) {
    notes.push(`Лучше всего — ${(LANES[lanes[0].key] ?? lanes[0].key).toLowerCase()} (${signed(lanes[0].net)}), хуже всего — ${(LANES[lanes[lanes.length - 1].key] ?? "").toLowerCase()} (${signed(lanes[lanes.length - 1].net)}).`);
  }
  if (notes.length === 0) return null;
  return <div style={{ marginTop: 6 }}>{notes.map((n) => <div key={n} className="note">{n}</div>)}</div>;
}

// ---------------------------------------------------------------- periods

function PeriodBars({ v }: { v: View }) {
  const option = useMemo<EChartsOption>(() => ({
    backgroundColor: "transparent",
    animation: false,
    grid: { left: 48, right: 16, top: 12, bottom: 28 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "#1f232c",
      borderColor: "#2a2f3a",
      textStyle: { color: "#eceef3" },
      formatter: (ps: any) => {
        const b = v.by_period[(Array.isArray(ps) ? ps[0] : ps).dataIndex];
        return `${periodLabel(b.key, v.weekly, true)}<br/>${signed(b.net)} MMR · ${games(b.games)} · ${pct(b.wins / b.games)} побед`;
      },
    },
    xAxis: { type: "category", data: v.by_period.map((b) => periodLabel(b.key, v.weekly, false)), axisLine: { lineStyle: { color: "#2a2f3a" } }, axisLabel: { color: "#5f6676" } },
    yAxis: { type: "value", splitLine: { lineStyle: { color: "#1a1e26" } }, axisLabel: { color: "#5f6676" } },
    series: [{
      type: "bar",
      data: v.by_period.map((b) => ({ value: b.net, itemStyle: { color: b.net >= 0 ? "#3fd68a" : "#f2705f", borderRadius: 3 } })),
      barMaxWidth: 26,
    }],
  }), [v]);
  return <EChart option={option} height={180} />;
}

function periodLabel(key: string, weekly: boolean, long: boolean): string {
  const [y, m, d] = key.split("-").map(Number);
  if (weekly) {
    const dt = new Date(y, m - 1, d);
    return (long ? "неделя с " : "") + dt.toLocaleDateString("ru-RU", { day: "numeric", month: "short" });
  }
  return new Date(y, m - 1, 1).toLocaleDateString("ru-RU", long ? { month: "long", year: "numeric" } : { month: "short", year: "2-digit" });
}

// ---------------------------------------------------------------- buckets

function HeroNet({ buckets }: { buckets: RankBucket[] }) {
  const names = useNames();
  if (buckets.length === 0) return <div className="dim">Ни на одном герое нет 3 рейтинговых игр за период.</div>;
  // Biggest gains and losses, at most 5 each.
  const top = buckets.filter((b) => b.net > 0).slice(0, 5);
  const bottom = buckets.filter((b) => b.net < 0).slice(-5);
  const rest = buckets.filter((b) => b.net === 0).slice(0, Math.max(0, 6 - top.length - bottom.length));
  const list = [...top, ...rest, ...bottom];
  const max = Math.max(1, ...list.map((b) => Math.abs(b.net)));
  return (
    <div className="pool">
      {list.map((b) => (
        <div key={b.key} className="row" style={{ cursor: "default" }}>
          <HeroBadge npc={b.key} size={28} />
          <div>
            <div className="top">
              <span>{names.hero(b.key)}</span>
              <span className="mono"><span className={b.net > 0 ? "win" : b.net < 0 ? "loss" : "dim"}>{signed(b.net)}</span> · {pct(b.wins / b.games)} · {b.games}</span>
            </div>
            <div className={`bar ${b.net < 0 ? "bad" : ""}`}><div style={{ width: `${(Math.abs(b.net) / max) * 100}%` }} /></div>
          </div>
        </div>
      ))}
    </div>
  );
}

function NetList({ buckets, label, empty }: { buckets: RankBucket[]; label: (k: string) => string; empty: string }) {
  if (buckets.length === 0) return empty ? <div className="dim">{empty}</div> : null;
  const max = Math.max(1, ...buckets.map((b) => Math.abs(b.net)));
  return (
    <div className="pool">
      {buckets.map((b) => (
        <div key={b.key} className="row" style={{ cursor: "default", gridTemplateColumns: "1fr" }}>
          <div>
            <div className="top">
              <span>{label(b.key)}</span>
              <span className="mono"><span className={b.net > 0 ? "win" : b.net < 0 ? "loss" : "dim"}>{signed(b.net)}</span> · {pct(b.wins / b.games)} · {b.games}</span>
            </div>
            <div className={`bar ${b.net < 0 ? "bad" : ""}`}><div style={{ width: `${(Math.abs(b.net) / max) * 100}%` }} /></div>
          </div>
        </div>
      ))}
    </div>
  );
}
