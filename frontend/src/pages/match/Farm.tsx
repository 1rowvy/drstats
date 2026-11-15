import type { MatchView } from "../../api";
import { clock, duration, GOLD_SOURCES, kfmt, num, pct, regionName, upperFirst } from "../../format";
import { useNames } from "../../names";
import { VersusChart } from "../../components/Chart";
import { Icon } from "../../components/Icon";
import { Ring, Tile } from "../../components/ui";
import { ItemsTable } from "../MatchTabs";
import { keyItems, lastOf, useFocus } from "./common";

export const GOLD_COLORS: Record<string, string> = {
  creep: "var(--accent)", neutral: "var(--blue)", hero: "var(--purple)", building: "var(--teal)",
  passive: "#4a5160", bounty: "#d8c46a", roshan: "#e6a0c4", other: "#2e333d",
};
const GOLD_LABELS: Record<string, string> = { ...GOLD_SOURCES, hero: "Убийства героев", building: "Здания" };

export function FarmTab({ view, goal }: { view: MatchView; goal: number }) {
  return (
    <div>
      <NetWorthCard view={view} />
      <div className="grid2" style={{ marginTop: 14 }}>
        <LaningCard view={view} goal={goal} />
        <GoldCard view={view} />
      </div>
      <div className="grid2" style={{ marginTop: 14 }}>
        <TimingsCard view={view} />
        <HitsCard view={view} />
      </div>
      <IdleCard view={view} />
      <div className="card" style={{ marginTop: 14 }}>
        <details className="more">
          <summary><Icon name="list" size={13} /> Все покупки</summary>
          <div style={{ marginTop: 12 }}><ItemsTable view={view} /></div>
        </details>
      </div>
    </div>
  );
}

function NetWorthCard({ view }: { view: MatchView }) {
  const names = useNames();
  const { opp } = useFocus(view);
  const nw = view.farm.net_worth;
  const a = lastOf(nw.focus), b = lastOf(nw.opponent);
  const oppName = opp ? names.hero(opp.hero_name) : "";
  return (
    <div className="card">
      <div className="nw-head">
        <div>
          <h2>Нетворс</h2>
          <span className="legend-inline" style={{ marginTop: 6 }}>
            <span><i className="line" />Ты</span>
            {opp && <span><i className="dashed" />{oppName}</span>}
          </span>
        </div>
        <div className="nums">
          {a != null && <div><b>{kfmt(a)}</b><small>на {duration(view.meta.duration)}</small></div>}
          {a != null && b != null && <div><b className={a >= b ? "win" : "loss"}>{kfmt(a - b, true)}</b><small>к оппоненту</small></div>}
        </div>
      </div>
      <VersusChart t={view.farm.t ?? []} me={nw.focus ?? []} opp={opp ? nw.opponent ?? [] : undefined} oppName={oppName} height={260} unit="мин" />
    </div>
  );
}

function LaningCard({ view, goal }: { view: MatchView; goal: number }) {
  const names = useNames();
  const { opp } = useFocus(view);
  const l = view.laning;
  const at = (t: number) => (l.snaps ?? []).find((s) => s.t === t);
  const s5 = at(300), s10 = at(600);
  const lh = s10?.lh;
  const oppShort = opp ? names.hero(opp.hero_name).split(" ")[0] : "";
  const nwDiff = s10?.opp ? s10.nw - s10.opp.nw : null;
  const low = lh != null && lh < goal * 0.7;
  return (
    <div className="card">
      <div className="card-head"><h2>Лайнинг, 0–10 мин</h2></div>
      <div className="ring-row">
        <Ring value={(lh ?? 0) / goal} size={112} stroke={9} color={low ? "var(--loss)" : "var(--accent)"}>
          <b style={{ fontSize: 30 }}>{lh ?? "—"}</b><small>из {goal}</small>
        </Ring>
        <div className="tiles" style={{ flex: 1 }}>
          <Tile label="ЛХ на 5 мин" value={s5?.lh ?? "—"} />
          <Tile label="Денаи" value={s10?.dn ?? "—"} />
          <Tile label={opp ? `Нетворс vs ${oppShort}` : "Нетворс на 10:00"}
            value={nwDiff != null ? num(nwDiff, true) : s10 ? num(s10.nw) : "—"}
            tone={nwDiff == null ? "" : nwDiff >= 0 ? "win" : "loss"} />
          <Tile label="Смерти на линии" value={(l.deaths ?? []).length} tone={(l.deaths ?? []).length ? "loss" : "win"} />
        </div>
      </div>
    </div>
  );
}

function GoldCard({ view }: { view: MatchView }) {
  const sources = (view.gold.sources ?? []).filter((s) => s.amount > 0).sort((a, b) => b.amount - a.amount);
  const sum = sources.reduce((a, s) => a + s.amount, 0);
  const total = sum || 1;
  return (
    <div className="card">
      <div className="card-head"><h2>Откуда золото</h2>{sum > 0 && <span className="meta mono">{num(sum)}</span>}</div>
      {sum === 0 && <div className="dim">Нет данных — перепарси реплей.</div>}
      <div className="stack-bar">
        {sources.map((s) => <div key={s.key} title={GOLD_LABELS[s.key]} style={{ width: `${(s.amount / total) * 100}%`, background: GOLD_COLORS[s.key] }} />)}
      </div>
      <div className="legend-list">
        {sources.map((s) => (
          <div className="li" key={s.key}>
            <span className="sw" style={{ background: GOLD_COLORS[s.key] }} />
            <span>{GOLD_LABELS[s.key] ?? s.key}</span>
            <span className="mono">{num(s.amount)}</span>
            <span className="mono">{Math.round((s.amount / total) * 100)}%</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function TimingsCard({ view }: { view: MatchView }) {
  const names = useNames();
  const items = keyItems(view, names, 5);
  const maxT = Math.max(600, ...items.map((it) => Math.max(it.t, it.avg ?? 0)));
  const scaleMax = Math.ceil(maxT / 300) * 300;
  const ticks: number[] = [];
  for (let t = 0; t <= scaleMax; t += scaleMax > 2400 ? 600 : 300) ticks.push(t);
  const x = (t: number) => `${(Math.max(0, t) / scaleMax) * 100}%`;
  return (
    <div className="card">
      <div className="card-head">
        <h2>Тайминги предметов</h2>
        <span className="meta"><span><span className="dot" style={{ background: "var(--accent)", width: 7, height: 7 }} /> эта игра</span><span>│ твоё среднее</span></span>
      </div>
      {items.length === 0 ? <div className="dim">Дорогих предметов не покупал.</div> : items.map((it, i) => {
        const d = it.diff;
        return (
          <div className="timing" key={i}>
            <div className="th">
              <b>{names.inflictor("item_" + it.item)}</b>
              <span>
                <span className="t">{clock(it.t)}</span>
                <span className={`d ${d == null ? "faint" : d <= 0 ? "win" : "loss"}`}>
                  {d == null ? "впервые" : `${clock(Math.abs(Math.round(d)))} ${d <= 0 ? "раньше" : "позже"}`}
                </span>
              </span>
            </div>
            <div className="track">
              {it.avg != null && <span className="avg" style={{ left: x(it.avg) }} title={`среднее ${clock(it.avg)} (${it.n} матчей)`} />}
              <span className="me" style={{ left: x(it.t) }} />
            </div>
          </div>
        );
      })}
      {items.length > 0 && <div className="scale">{ticks.map((t) => <span key={t}>{t / 60}</span>)}<span>мин</span></div>}
    </div>
  );
}

function HitsCard({ view }: { view: MatchView }) {
  const names = useNames();
  const uses = (view.abilities.uses ?? []).filter((u) => !u.is_item && u.hit_rate != null)
    .sort((a, b) => (b.hit_rate ?? 0) - (a.hit_rate ?? 0)).slice(0, 7);
  return (
    <div className="card">
      <div className="card-head"><h2>Попадания скиллами</h2><span className="meta">≈ урон в течение {view.abilities.hit_window} с после каста</span></div>
      {uses.length === 0 ? <div className="dim">Нет способностей, бьющих по героям.</div> : uses.map((u) => {
        const r = u.hit_rate ?? 0;
        return (
          <div className="hitrow" key={u.ability}>
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{names.inflictor(u.ability)}</span>
            <div className={`bar ${r < 0.6 ? "bad" : ""}`}><div style={{ width: `${r * 100}%` }} /></div>
            <span className="mono">{u.hits}/{u.casts}</span>
            <span className="mono">{pct(r)}</span>
          </div>
        );
      })}
    </div>
  );
}

function IdleCard({ view }: { view: MatchView }) {
  const g = view.gold;
  const idle = g.idle ?? [];
  const dur = Math.max(1, view.meta.duration);
  const longest = [...idle].sort((a, b) => (b.to - b.from) - (a.to - a.from)).slice(0, 3).sort((a, b) => a.from - b.from);
  return (
    <div className="card" style={{ marginTop: 14 }}>
      <div className="card-head">
        <h2>Простой без фарма</h2>
        <span style={{ display: "inline-flex", alignItems: "baseline", gap: 10 }}>
          <span className={`display ${g.idle_total >= 120 ? "loss" : "win"}`} style={{ fontSize: 26, fontWeight: 800 }}>{clock(g.idle_total)}</span>
          <span className="faint small">порог: жив и ≥{g.threshold} с без фарма и драк</span>
        </span>
      </div>
      <div className="idle-track">
        {idle.map((p, i) => (
          <span key={i} title={`${clock(p.from)} – ${clock(p.to)}`}
            style={{ left: `${(Math.max(0, p.from) / dur) * 100}%`, width: `${((p.to - Math.max(0, p.from)) / dur) * 100}%` }} />
        ))}
      </div>
      {idle.length === 0 ? <div className="dim">Простоев нет — так держать.</div> : (
        <div className="idle-cards">
          {longest.map((p, i) => (
            <div className="tile" key={i}>
              <span className="mono">{clock(p.from)} – {clock(p.to)}</span>
              <b>{p.region ? upperFirst(regionName(p.region)) : "—"} · {duration(p.to - p.from)}</b>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
