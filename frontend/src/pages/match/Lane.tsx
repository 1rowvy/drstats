import type { Dashboard, LaneHero, MatchView } from "../../api";
import { clock, num, plural } from "../../format";
import { useNames } from "../../names";
import { VersusChart } from "../../components/Chart";
import { HeroBadge } from "../../components/HeroBadge";
import { heroOf, useFocus } from "./common";
import { useDashboard } from "../Dashboard";

const LANE_NAMES: Record<string, string> = { top: "Топ", mid: "Мид", bot: "Бот", jungle: "Лес", roam: "Роум" };
const VERDICT: Record<string, { text: string; tone: string }> = {
  won: { text: "Линия выиграна", tone: "win" },
  even: { text: "Линия ровная", tone: "" },
  lost: { text: "Линия проиграна", tone: "loss" },
};

type Row = { label: string; key: "lh" | "dn" | "nw" | "xp" | "level" };
const ROWS: Row[] = [
  { label: "Добивания", key: "lh" },
  { label: "Денаи", key: "dn" },
  { label: "Нетворс", key: "nw" },
  { label: "Опыт", key: "xp" },
  { label: "Уровень", key: "level" },
];

/** Lane tab: the first 10 minutes against the lane opponent. */
export function LaneTab({ view, goal }: { view: MatchView; goal: number }) {
  const names = useNames();
  const { opp } = useFocus(view);
  const l = view.laning;
  // Your usual numbers against this hero (replays only: OpenDota history has no lane data).
  const { d } = useDashboard({ hero_id: 0, days: 0, last: 0, result: "", mode: 0, lane: "", source: "" }, 0);
  if (!opp) {
    return <div className="card empty">Не получилось определить оппонента по линии{l.lane ? ` (${(LANE_NAMES[l.lane] ?? l.lane).toLowerCase()})` : ""}: рядом с тобой в первые 10 минут не стоял ни один враг.</div>;
  }
  const oppName = names.hero(opp.hero_name);
  const s5 = (l.snaps ?? []).find((s) => s.t === 300);
  const s10 = (l.snaps ?? []).find((s) => s.t === 600);
  const v = VERDICT[l.outcome];
  return (
    <div>
      <Verdict view={view} oppName={oppName} goal={goal} />

      <div className="grid2" style={{ marginTop: 14 }}>
        <div className="card">
          <div className="card-head"><h2>Ты против {oppName}</h2>{v && <span className={`meta ${v.tone}`}>{v.text.toLowerCase()}</span>}</div>
          <table className="lane-h2h">
            <thead>
              <tr><th />
                <th colSpan={3}>5:00</th>
                <th colSpan={3}>10:00</th>
              </tr>
              <tr className="sub"><th />
                <th>ты</th><th>он</th><th>±</th>
                <th>ты</th><th>он</th><th>±</th>
              </tr>
            </thead>
            <tbody>
              {ROWS.map((r) => (
                <tr key={r.key}>
                  <td>{r.label}</td>
                  {[s5, s10].map((s, i) => {
                    const a = s?.[r.key];
                    const b = s?.opp?.[r.key];
                    const diff = a != null && b != null ? a - b : null;
                    return [
                      <td key={`a${i}`} className="mono">{a ?? "—"}</td>,
                      <td key={`b${i}`} className="mono dim">{b ?? "—"}</td>,
                      <td key={`d${i}`} className={`mono ${diff == null || diff === 0 ? "faint" : diff > 0 ? "win" : "loss"}`}>{diff == null ? "" : num(diff, true)}</td>,
                    ];
                  })}
                </tr>
              ))}
              <tr>
                <td>Убийства друг друга</td>
                <td colSpan={6} className="mono">
                  <span className={l.killed_opp ? "win" : "faint"}>ты его {l.killed_opp}</span>
                  <span className="faint"> · </span>
                  <span className={l.killed_by_opp ? "loss" : "faint"}>он тебя {l.killed_by_opp}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <LaneSides view={view} />
      </div>

      <div className="grid2" style={{ marginTop: 14 }}>
        <LaneChart view={view} metric="net_worth" title="Нетворс, 0–10 мин" oppName={oppName} />
        <LaneChart view={view} metric="last_hits" title="Добивания, 0–10 мин" oppName={oppName} />
      </div>

      <div className="grid2" style={{ marginTop: 14 }}>
        <LaneDeaths view={view} />
        <Matchup d={d} oppNpc={opp.hero_name} oppName={oppName} />
      </div>
    </div>
  );
}

/** Big verdict with the reasons behind it. */
function Verdict({ view, oppName, goal }: { view: MatchView; oppName: string; goal: number }) {
  const { me } = useFocus(view);
  const l = view.laning;
  const s10 = (l.snaps ?? []).find((s) => s.t === 600);
  const v = VERDICT[l.outcome] ?? { text: "Линия без данных", tone: "" };
  const reasons: { text: string; good: boolean }[] = [];
  if (s10?.opp) {
    const o = s10.opp;
    const lh = s10.lh - o.lh;
    if (Math.abs(lh) >= 5) reasons.push({ good: lh > 0, text: lh > 0 ? `Добил на ${lh} крипов больше (${s10.lh} против ${o.lh})` : `Добил на ${-lh} крипов меньше (${s10.lh} против ${o.lh})` });
    const dn = s10.dn - o.dn;
    if (Math.abs(dn) >= 4) reasons.push({ good: dn > 0, text: dn > 0 ? `Денаил больше: ${s10.dn} против ${o.dn}` : `Он денаил больше: ${o.dn} против ${s10.dn}` });
    const lvl = s10.level - o.level;
    if (lvl !== 0) reasons.push({ good: lvl > 0, text: lvl > 0 ? `Обогнал по уровню: ${s10.level} против ${o.level}` : `Отстал по уровню: ${s10.level} против ${o.level}` });
    // The last-hit goal is for cores: skip it for supports and support-vs-support lanes.
    const support = (me.od?.position ?? 0) >= 4 || s10.lh + o.lh < 30;
    if (!support && s10.lh < goal * 0.7) reasons.push({ good: false, text: `${s10.lh} добиваний к 10:00 — это меньше 70% от твоей цели (${goal})` });
  }
  if (l.killed_opp) reasons.push({ good: true, text: `Убил ${oppName} ${l.killed_opp} ${plural(l.killed_opp, "раз", "раза", "раз")}` });
  if (l.killed_by_opp) reasons.push({ good: false, text: `${oppName} убил тебя ${l.killed_by_opp} ${plural(l.killed_by_opp, "раз", "раза", "раз")}` });
  const other = (l.deaths ?? []).length - l.killed_by_opp;
  if (other > 0) reasons.push({ good: false, text: `Ещё ${other} ${plural(other, "смерть", "смерти", "смертей")} на линии от других` });
  if (reasons.length === 0) reasons.push({ good: true, text: "Ни одной заметной разницы — чистый паритет." });
  return (
    <div className="card lane-verdict">
      <div>
        <div className="caps">{LANE_NAMES[l.lane] ?? "Линия"} · к 10:00</div>
        <h1 className={v.tone}>{v.text}</h1>
        {s10?.opp && <div className="dim">Нетворс {num(s10.nw - s10.opp.nw, true)} · опыт {num(s10.xp - s10.opp.xp, true)} к {oppName}</div>}
      </div>
      <ul>
        {reasons.map((r) => <li key={r.text} className={r.good ? "good" : "bad"}>{r.text}</li>)}
      </ul>
    </div>
  );
}

/** Both sides of the lane with their 10:00 numbers. */
function LaneSides({ view }: { view: MatchView }) {
  const names = useNames();
  const { me } = useFocus(view);
  const heroes = view.laning.heroes ?? [];
  const mine = heroes.filter((h) => h.team === me.team);
  const theirs = heroes.filter((h) => h.team !== me.team);
  const sum = (hs: LaneHero[]) => hs.reduce((a, h) => a + h.nw, 0);
  const diff = sum(mine) - sum(theirs);
  const side = (hs: LaneHero[], title: string) => (
    <div className="lane-side">
      <div className="caps">{title} · {num(sum(hs))}</div>
      {hs.map((h) => (
        <div key={h.pid} className={`lane-hero ${h.pid === view.focus ? "me" : ""} ${h.pid === view.opponent ? "opp" : ""}`}>
          <HeroBadge npc={h.hero_name} size={30} />
          <div>
            <b>{names.hero(h.hero_name)}</b>
            <small>{h.lh}/{h.dn} · ур. {h.level} · {h.kills}/{h.deaths}</small>
          </div>
          <span className="mono">{num(h.nw)}</span>
        </div>
      ))}
    </div>
  );
  return (
    <div className="card">
      <div className="card-head">
        <h2>Вся линия</h2>
        <span className={`meta mono ${diff >= 0 ? "win" : "loss"}`}>{num(diff, true)} нетворса</span>
      </div>
      <div className="lane-sides">
        {side(mine, "Твоя сторона")}
        {side(theirs, "Против")}
      </div>
      <div className="note">ЛХ/ДН · уровень · убийства/смерти до 10:00. Союзники — кто стоял на твоей линии, враги — на линии оппонента.</div>
    </div>
  );
}

function LaneChart({ view, metric, title, oppName }: {
  view: MatchView; metric: "net_worth" | "last_hits"; title: string; oppName: string;
}) {
  const ts = view.farm.t ?? [];
  const end = ts.findIndex((t) => t > 600);
  const cut = end < 0 ? ts.length : end;
  const m = view.farm[metric];
  return (
    <div className="card">
      <div className="card-head"><h2>{title}</h2></div>
      <VersusChart t={ts.slice(0, cut)} me={(m.focus ?? []).slice(0, cut)} opp={(m.opponent ?? []).slice(0, cut)} oppName={oppName} height={200} unit="мин" />
    </div>
  );
}

function LaneDeaths({ view }: { view: MatchView }) {
  const names = useNames();
  const deaths = view.laning.deaths ?? [];
  return (
    <div className="card">
      <div className="card-head"><h2>Смерти на линии</h2><span className="meta">{deaths.length || "ни одной"}</span></div>
      {deaths.length === 0 && <div className="dim">Ни разу не умер до 10:00.</div>}
      {deaths.map((d, i) => {
        const killer = d.killer >= 0 ? names.hero(heroOf(view.players ?? [], d.killer)?.hero_name ?? "") : names.unit(d.killer_name);
        const helpers = (d.assists ?? []).map((pid) => names.hero(heroOf(view.players ?? [], pid)?.hero_name ?? "")).filter(Boolean);
        return (
          <div key={i} className="lane-death">
            <span className="mono">{clock(d.t)}</span>
            <span>
              <b className={d.killer === view.opponent ? "loss" : ""}>{killer}</b>
              {d.inflictor && <span className="dim"> · {names.inflictor(d.inflictor)}</span>}
              {helpers.length > 0 && <span className="faint"> + {helpers.join(", ")}</span>}
            </span>
            {d.gold_lost > 0 && <span className="mono faint">−{d.gold_lost}</span>}
          </div>
        );
      })}
    </div>
  );
}

/** Your record on the lane against this opponent hero over all parsed games. */
function Matchup({ d, oppNpc, oppName }: { d: Dashboard | null; oppNpc: string; oppName: string }) {
  const m = d?.lanes.matchups?.find((x) => x.hero_name === oppNpc);
  const all = d?.lanes;
  return (
    <div className="card">
      <div className="card-head"><h2>Против {oppName} обычно</h2><span className="meta">все разобранные игры</span></div>
      {!d && <div className="dim">Загрузка…</div>}
      {d && (!m || m.games < 2) && <div className="dim">Против {oppName} на линии стоял {m ? "только в этой игре" : "впервые"} — сравнивать пока не с чем.</div>}
      {m && m.games >= 2 && (
        <div className="tiles">
          <div className="tile" style={{ padding: "14px 16px" }}>
            <div className="label">{m.games} {plural(m.games, "игра", "игры", "игр")} · выиграл / проиграл</div>
            <div className="val" style={{ fontSize: 26 }}><span className="win">{m.won}</span> / <span className="loss">{m.lost}</span></div>
          </div>
          <div className="tile" style={{ padding: "14px 16px" }}>
            <div className="label">Нетворс к 10:00 в среднем</div>
            <div className={`val ${m.avg_nw >= 0 ? "win" : "loss"}`} style={{ fontSize: 26 }}>{num(m.avg_nw, true)}</div>
          </div>
        </div>
      )}
      {all && all.games > 0 && (
        <div className="note">
          Всего линий с оппонентом: {all.games}. Выиграл {all.won.games}, ровно {all.even.games}, проиграл {all.lost.games}.
        </div>
      )}
    </div>
  );
}
