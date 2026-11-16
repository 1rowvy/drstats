import type { Habit } from "../api";
import { METRICS } from "../format";

const HABIT_TEXT: Record<string, (r: string, b: string) => string> = {
  lh10: (r, b) => `Добиваешь меньше крипов на линии: ${r} к 10:00 вместо обычных ${b}.`,
  dn10: (r, b) => `Меньше денаев на линии: ${r} против ${b}.`,
  nw10: (r, b) => `Выходишь с линии беднее: ${r} нетворса к 10:00 против ${b}.`,
  deaths: (r, b) => `Умираешь чаще: ${r} смертей за игру вместо ${b}.`,
  deaths10: (r, b) => `Чаще умираешь до 10:00: ${r} против ${b}. Линия проседает с самого начала.`,
  gpm: (r, b) => `Фармишь медленнее: ${r} GPM против обычных ${b}.`,
  xpm: (r, b) => `Медленнее качаешься: ${r} XPM против ${b}.`,
  kp: (r, b) => `Реже участвуешь в драках: ${r} против ${b}.`,
  kda: (r, b) => `KDA просел: ${r} против ${b}.`,
  kills: (r, b) => `Меньше убийств: ${r} за игру против ${b}.`,
  assists: (r, b) => `Меньше помощи команде: ${r} ассистов против ${b}.`,
  hero_dmg_pm: (r, b) => `Меньше урона по героям: ${r} в минуту против ${b}.`,
  tower_dmg: (r, b) => `Меньше давишь строения: ${r} урона против ${b}.`,
  gold_lost: (r, b) => `Больше теряешь на смертях: ${r} золота против ${b}.`,
  wards: (r, b) => `Меньше вардов: ${r} за игру против ${b}.`,
  stacks: (r, b) => `Реже стакаешь: ${r} против ${b}.`,
};

/** Worst (and improved) habits: the last 10 matches against the earlier ones. */
export function HabitsCard({ habits, empty }: { habits: Habit[]; empty: string }) {
  const worse = habits.filter((h) => h.worse).slice(0, 3);
  const better = habits.filter((h) => h.change >= 0.1).slice(0, 3 - Math.min(3, worse.length));
  return (
    <div className="card">
      <div className="card-head"><h2 className="loss">Худшие привычки</h2><span className="meta">последние 10 vs раньше</span></div>
      {habits.length === 0 && <div className="dim">{empty}</div>}
      {habits.length > 0 && worse.length === 0 && <div className="dim" style={{ marginBottom: 10 }}>Последние 10 матчей не хуже среднего — так держать.</div>}
      {[...worse, ...better].map((h, i) => <HabitItem key={h.key} h={h} n={i + 1} />)}
    </div>
  );
}

function HabitItem({ h, n }: { h: Habit; n: number }) {
  const m = METRICS[h.key];
  const f = m ? m.fmt : (v: number) => v.toFixed(1);
  const text = h.worse ? (HABIT_TEXT[h.key] ?? ((r, b) => `${r} против ${b}.`))(f(h.recent), f(h.before)) : `Стало лучше: ${f(h.recent)} против ${f(h.before)}.`;
  return (
    <div className={`habit-card ${h.worse ? "" : "good"}`}>
      <span className="no">{String(n).padStart(2, "0")}</span>
      <div>
        <div className="hc-head"><span>{m?.label ?? h.key}</span><span className="mono">{f(h.recent)} vs {f(h.before)}</span></div>
        <p>{text}</p>
      </div>
    </div>
  );
}
