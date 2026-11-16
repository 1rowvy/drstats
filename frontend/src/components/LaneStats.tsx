import type { LaneStats } from "../api";
import { num, pct, plural } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "./HeroBadge";

const MIN_MATCHUP = 2; // games against a hero before it is listed

/** How often the lane is won against the lane opponent and what it means for the game. */
export function LaneStatsCard({ s }: { s: LaneStats }) {
  const names = useNames();
  const parts = [
    { key: "won", label: "Выиграл", b: s.won, color: "var(--win)" },
    { key: "even", label: "Ровно", b: s.even, color: "#4a5160" },
    { key: "lost", label: "Проиграл", b: s.lost, color: "var(--loss)" },
  ];
  const hard = (s.matchups ?? []).filter((m) => m.games >= MIN_MATCHUP && m.avg_score < 0).slice(0, 3);
  const easy = [...(s.matchups ?? [])].reverse().filter((m) => m.games >= MIN_MATCHUP && m.avg_score > 0).slice(0, 3);
  let note = "";
  if (s.won.games >= 3 && s.lost.games >= 3) {
    const d = Math.round((s.won.winrate - s.lost.winrate) * 100);
    note = d >= 10 ? `Выигранная линия даёт +${d}% к победе. Лайнинг решает — вкладывайся в него.`
      : d <= -10 ? `После проигранной линии ты выигрываешь чаще (на ${-d}%) — вытаскиваешь игры позже.`
      : "Исход линии почти не влияет на исход игры.";
  }
  const row = (m: (typeof hard)[number]) => (
    <div key={m.hero_name} className="lane-mu">
      <HeroBadge npc={m.hero_name} size={26} />
      <span>{names.hero(m.hero_name)}</span>
      <span className="mono faint">{m.won}/{m.lost} из {m.games}</span>
      <span className={`mono ${m.avg_nw >= 0 ? "win" : "loss"}`}>{num(m.avg_nw, true)}</span>
    </div>
  );
  return (
    <div className="card">
      <div className="card-head"><h2>Линия против оппонента</h2><span className="meta">к 10:00 · {s.games} {plural(s.games, "игра", "игры", "игр")} с реплеем</span></div>
      {s.games === 0 ? <div className="dim">Нужны разобранные реплеи — по истории OpenDota линию не посчитать.</div> : <>
        <div className="stack-bar" style={{ height: 10 }}>
          {parts.map((p) => p.b.games > 0 && <div key={p.key} style={{ width: `${(p.b.games / s.games) * 100}%`, background: p.color }} />)}
        </div>
        <div className="tiles4" style={{ gridTemplateColumns: "repeat(3, 1fr)", marginTop: 12 }}>
          {parts.map((p) => (
            <div className="tile" key={p.key}>
              <div className="label">{p.label} · {pct(p.b.games / s.games)}</div>
              <div className="val">{p.b.games ? pct(p.b.winrate) : "—"}</div>
              <div className="label">побед в игре</div>
            </div>
          ))}
        </div>
        <div className="dim small" style={{ marginTop: 10 }}>Нетворс к оппоненту на 10:00 в среднем: <b className={s.avg_nw >= 0 ? "win" : "loss"}>{num(s.avg_nw, true)}</b></div>
        {(hard.length > 0 || easy.length > 0) && (
          <div className="grid2" style={{ marginTop: 12, gap: 14 }}>
            <div><div className="caps" style={{ marginBottom: 6 }}>Тяжелее всего</div>{hard.length ? hard.map(row) : <div className="faint small">нет</div>}</div>
            <div><div className="caps" style={{ marginBottom: 6 }}>Легче всего</div>{easy.length ? easy.map(row) : <div className="faint small">нет</div>}</div>
          </div>
        )}
        {note && <div className="note">{note}</div>}
      </>}
    </div>
  );
}
