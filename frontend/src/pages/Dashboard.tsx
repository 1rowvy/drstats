import { useEffect, useMemo, useState } from "react";
import { Browser } from "@wailsio/runtime";
import { Api, Bucket, Dashboard, Filter } from "../api";
import { clock, duration, modeShort, pct, plural, relDate, roleName, WEEKDAYS } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "../components/HeroBadge";
import { ResultPill } from "../components/ui";
import { HabitsCard } from "../components/Habits";

const ACTIVITY_WEEKS = 12;
const SOURCE_KEY = "dashboard.source";

/** Matches known only from the OpenDota history have no match page: open them on OpenDota. */
function openMatch(id: number, opendota: boolean | undefined, onMatch: (id: number) => void) {
  if (opendota) Browser.OpenURL(`https://www.opendota.com/matches/${id}`).catch(console.error);
  else onMatch(id);
}

function loadSource(): string {
  try { return localStorage.getItem(SOURCE_KEY) ?? "all"; } catch { return "all"; }
}

/** Loads the dashboard for a filter; reloads when matches change (tick). */
export function useDashboard(filter: Filter, tick: number) {
  const [d, setD] = useState<Dashboard | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    Api.Dashboard(filter)
      .then((v) => { setD(v); setError(""); })
      .catch((e) => setError(String(e?.message ?? e)));
  }, [JSON.stringify(filter), tick]);
  return { d, error };
}

export function DashboardPage({ tick, onMatch, onHero }: {
  tick: number; onMatch: (id: number) => void; onHero: (heroID: number) => void;
}) {
  const [days, setDays] = useState(ACTIVITY_WEEKS * 7);
  const [source, setSourceState] = useState(loadSource);
  const setSource = (v: string) => {
    setSourceState(v);
    try { localStorage.setItem(SOURCE_KEY, v); } catch { /* per-viewer convenience only */ }
  };
  const { d, error } = useDashboard({ hero_id: 0, days, last: 0, result: "", mode: 0, lane: "", source }, tick);
  // Nothing in the last 12 weeks but older matches exist: show all time.
  useEffect(() => {
    if (d && days && d.games === 0 && d.total > 0) setDays(0);
  }, [d, days]);
  if (error) return <div className="notice bad">{error}</div>;
  if (!d) return <div className="dim">Загрузка…</div>;
  if (d.unknown) return <div className="notice">Укажи свой Steam ID на странице «Импорт» — дашборд строится по твоим матчам.</div>;
  const bar = <SourceBar d={d} source={source} setSource={setSource} />;
  if (d.total === 0) return <>{bar}<div className="card empty">{source === "all" ? "Матчей с твоим участием пока нет — ни реплеев, ни истории OpenDota." : "Разобранных матчей с твоим участием пока нет."}</div></>;
  return <>{bar}<Body d={d} days={days} setDays={setDays} onMatch={onMatch} onHero={onHero} /></>;
}

/** Source switch (all games via OpenDota / replays only) and the history sync state. */
function SourceBar({ d, source, setSource }: { d: Dashboard; source: string; setSource: (v: string) => void }) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const h = d.history;
  const sync = () => {
    setBusy(true);
    setErr("");
    Api.SyncHistory().catch((e) => setErr(String(e?.message ?? e))).finally(() => setBusy(false));
  };
  let status = "";
  let bad = false;
  if (busy) status = h?.count ? "Обновляю историю OpenDota…" : "Загружаю историю с OpenDota, это может занять минуту…";
  else if (!h || !h.status) status = "История OpenDota ещё не загружена.";
  else if (h.status === "error") { status = `OpenDota: ${h.error}`; bad = true; }
  else if (h.count === 0) {
    status = "OpenDota не вернула ни одной игры. Включи в настройках Dota «Открыть доступ к данным матчей» (Expose Public Match Data) и обнови через пару минут.";
    bad = true;
  } else status = `OpenDota: ${h.count} ${plural(h.count, "игра", "игры", "игр")} · обновлено ${relDate(h.ok_at)}`;
  return (
    <div className="source-bar">
      <div className="seg">
        <button className={source === "all" ? "on" : ""} onClick={() => setSource("all")}>Все игры</button>
        <button className={source === "" ? "on" : ""} onClick={() => setSource("")}>Только реплеи</button>
      </div>
      {source === "all" && <>
        <span className={bad ? "loss" : "dim"}>{status}</span>
        <button className="link" disabled={busy} onClick={sync}>{busy ? "обновляю…" : "обновить"}</button>
      </>}
      {err && <span className="loss">{err}</span>}
    </div>
  );
}

function Body({ d, days, setDays, onMatch, onHero }: {
  d: Dashboard; days: number; setDays: (n: number) => void; onMatch: (id: number) => void; onHero: (heroID: number) => void;
}) {
  const names = useNames();
  const gpm = (d.averages ?? []).find((a) => a.key === "gpm");
  const form = (d.trend ?? []).slice(-20);
  const x = d.extras;
  return (
    <div>
      <div className="card dash-hero">
        <div className="watermark">{d.games}</div>
        <div style={{ position: "relative" }}>
          <div className="caps" style={{ display: "flex", gap: 10, alignItems: "center" }}>
            {days ? `Последние ${ACTIVITY_WEEKS} недель` : "За всё время"}
            <button className="link caps" style={{ color: "var(--accent)" }} onClick={() => setDays(days ? 0 : ACTIVITY_WEEKS * 7)}>
              {days ? "всё время" : `${ACTIVITY_WEEKS} недель`}
            </button>
          </div>
          <h1>Твоя дота в цифрах</h1>
          {d.filter.source === "all" && d.replays < d.games && (
            <div className="dim" style={{ marginTop: -6, marginBottom: 6 }}>
              {d.replays} из {d.games} — с разобранным реплеем; самый быстрый предмет считается только по ним
            </div>
          )}
          <div className="big-row">
            <div><b>{d.games}</b><div className="caps">{plural(d.games, "матч", "матча", "матчей")}</div></div>
            <div><b className={d.winrate >= 0.5 ? "accent" : "loss"}>{pct(d.winrate)}</b><div className="caps">побед</div></div>
            <div><b>{Math.round(x.hours)} ч</b><div className="caps">в игре</div></div>
            <div><b>{gpm?.n ? Math.round(gpm.avg) : "—"}</b><div className="caps">средний GPM</div></div>
          </div>
          {form.length > 0 && (
            <div style={{ marginTop: 22 }}>
              <div className="caps">Форма, последние {form.length} · слева старые</div>
              <div className="form-row">
                {form.map((g) => (
                  <span key={g.match_id} className={g.win ? "w" : "l"} title={`${names.hero(g.hero_name)} · ${relDate(g.end_time)}${g.opendota ? " · без реплея, откроется OpenDota" : ""}`}
                    onClick={() => openMatch(g.match_id, g.opendota, onMatch)}>
                    {g.win ? "В" : "П"}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="grid-side" style={{ gridTemplateColumns: "1fr 330px", marginTop: 14 }}>
        <Activity d={d} />
        <div className="card">
          <div className="card-head"><h2>Пул героев</h2></div>
          <HeroPool d={d} onHero={onHero} />
        </div>
      </div>

      <div className="grid2" style={{ marginTop: 14 }}>
        <TiltCard d={d} />
        <HabitsCard habits={d.habits ?? []} empty="Нужно хотя бы 15 матчей." />
      </div>

      {(d.recent ?? []).length > 0 && <>
        <div className="section-head"><h2>Последние игры</h2></div>
        <RecentGames d={d} onMatch={onMatch} />
      </>}

      <div className="section-head"><h2>Рекорды</h2></div>
      <Records d={d} onMatch={onMatch} />

      <div className="grid2" style={{ marginTop: 14 }}>
        <PartyCard d={d} />
        <div className="card">
          <div className="card-head"><h2>Когда лучше не играть</h2></div>
          <DayParts hours={d.by_hour ?? []} />
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- activity

function Activity({ d }: { d: Dashboard }) {
  const byDate = new Map((d.extras.days ?? []).map((b) => [b.date, b]));
  // Columns are weeks (oldest left), rows Mon..Sun.
  const today = new Date();
  const monday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - ((today.getDay() + 6) % 7));
  const weeks: Date[] = [];
  for (let w = ACTIVITY_WEEKS - 1; w >= 0; w--) weeks.push(new Date(monday.getFullYear(), monday.getMonth(), monday.getDate() - 7 * w));
  const key = (dt: Date) => `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, "0")}-${String(dt.getDate()).padStart(2, "0")}`;
  const color = (n: number) => n === 0 ? "#171a21" : ["rgba(255,173,51,.25)", "rgba(255,173,51,.42)", "rgba(255,173,51,.6)", "rgba(255,173,51,.8)", "#ffad33"][Math.min(5, n) - 1];
  // Peak: best weekday + day part by number of games.
  const wd = [...(d.by_weekday ?? [])].sort((a, b) => b.games - a.games).filter((b) => b.games > 0).slice(0, 2);
  const parts = dayParts(d.by_hour ?? []).sort((a, b) => b.games - a.games)[0];
  const WD_LONG = ["", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота", "воскресенье"];
  const peak = wd.length && parts?.games ? `Пик — ${wd.map((b) => WD_LONG[Number(b.key)]).join(" и ")} ${parts.when.toLowerCase()}` : "";
  return (
    <div className="card">
      <div className="card-head"><h2>Активность</h2><span className="meta">{peak}</span></div>
      <div className="heat-grid" style={{ gridTemplateColumns: `22px repeat(${ACTIVITY_WEEKS}, 1fr)` }}>
        {[1, 2, 3, 4, 5, 6, 7].map((row) => (
          <Row key={row} label={row % 2 === 1 ? WEEKDAYS[row] : ""}>
            {weeks.map((w, i) => {
              const day = new Date(w.getFullYear(), w.getMonth(), w.getDate() + row - 1);
              const b = byDate.get(key(day));
              const future = day > today;
              return (
                <div key={i} className="cell" style={{ background: future ? "transparent" : color(b?.games ?? 0) }}
                  title={future ? "" : `${day.toLocaleDateString("ru-RU", { day: "numeric", month: "long" })}: ${b ? `${b.games} ${plural(b.games, "игра", "игры", "игр")}, ${b.wins} ${plural(b.wins, "победа", "победы", "побед")}` : "не играл"}`} />
              );
            })}
          </Row>
        ))}
      </div>
      <div className="heat-legend">0 {[1, 2, 3, 4, 5].map((n) => <i key={n} style={{ background: color(n) }} />)} 5+ игр</div>
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return <><span className="lbl">{label}</span>{children}</>;
}

// ---------------------------------------------------------------- heroes

function HeroPool({ d, onHero }: { d: Dashboard; onHero: (id: number) => void }) {
  const names = useNames();
  const top = (d.by_hero ?? []).slice(0, 5);
  const max = Math.max(1, ...top.map((b) => b.games));
  const heroID = (npc: string) => (d.heroes ?? []).find((h) => h.hero_name === npc)?.hero_id ?? 0;
  return (
    <div className="pool">
      {top.map((b) => (
        <div key={b.key} className="row" onClick={() => onHero(heroID(b.key))}>
          <HeroBadge npc={b.key} size={28} />
          <div>
            <div className="top"><span>{names.hero(b.key)}</span><span className="mono">{pct(b.winrate)} · {b.games}</span></div>
            <div className={`bar ${b.winrate < 0.45 ? "bad" : ""}`}><div style={{ width: `${(b.games / max) * 100}%` }} /></div>
          </div>
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------- tilt

const TILT_MIN = 5; // games a bucket needs before it is compared

/** Winrate after a win / a loss and by the game's number in a session. */
function TiltCard({ d }: { d: Dashboard }) {
  const t = d.extras.tilt;
  const { after_win: aw, after_loss: al, current: cur } = t;
  const session = t.session ?? [];
  const notes: string[] = [];
  if (aw.games >= TILT_MIN && al.games >= TILT_MIN) {
    const diff = Math.round((aw.winrate - al.winrate) * 100);
    if (diff >= 8) notes.push(`После поражения выигрываешь на ${diff}% реже, чем после победы. Слил — сделай паузу.`);
    else if (diff <= -8) notes.push(`После поражения играешь даже лучше — на ${-diff}%. Злость тебе на пользу.`);
    else notes.push("После поражения играешь так же, как после победы — тильт не мешает.");
  }
  const first = session[0];
  if (first && first.games >= TILT_MIN) {
    const drop = session.slice(1).find((b) => b.games >= TILT_MIN && first.winrate - b.winrate >= 0.1);
    if (drop) {
      const n = Number(drop.key);
      notes.push(`${n === 4 ? "С 4-й" : `На ${n}-й`} игре за сессию винрейт падает до ${pct(drop.winrate)}. Лимит — ${n - 1} ${plural(n - 1, "игра", "игры", "игр")} подряд.`);
    }
  }
  if (notes.length === 0) notes.push("Мало игр подряд, чтобы искать тильт.");
  const streak = cur.len >= 2 ? `сейчас ${cur.len} ${cur.win ? plural(cur.len, "победа", "победы", "побед") : plural(cur.len, "поражение", "поражения", "поражений")} подряд` : "";
  // Colour the tiles only when the gap is big enough to mean something.
  const gap = aw.games >= TILT_MIN && al.games >= TILT_MIN && Math.abs(aw.winrate - al.winrate) >= 0.08;
  const tile = (label: string, b: Bucket, other: Bucket) => (
    <div className="tile" style={{ padding: "14px 16px" }}>
      <div className="label">{label} · {b.games} {plural(b.games, "игра", "игры", "игр")}</div>
      <div className={`val ${!b.games ? "faint" : gap ? (b.winrate < other.winrate ? "loss" : "win") : ""}`} style={{ fontSize: 30 }}>{b.games ? pct(b.winrate) : "—"}</div>
    </div>
  );
  const max = Math.max(...session.map((b) => (b.games ? b.winrate : 0)));
  return (
    <div className="card">
      <div className="card-head">
        <h2>Тильт</h2>
        {streak && <span className={`meta ${!cur.win && cur.len >= 3 ? "loss" : ""}`}>{streak}</span>}
      </div>
      <div className="tiles">
        {tile("После победы", aw, al)}
        {tile("После поражения", al, aw)}
      </div>
      <div className="caps" style={{ margin: "16px 0 8px" }}>Игра по счёту за сессию</div>
      <div className="dayparts" style={{ height: 90 }}>
        {session.map((b) => (
          <div className="col" key={b.key}>
            <b className={b.games && b.winrate < 0.5 ? "loss" : ""}>{b.games ? pct(b.winrate) : "—"}</b>
            <div className="b" style={{ height: `${Math.max(4, b.winrate * 100)}%`,
              background: b.games === 0 ? "#1f232c" : b.winrate === max ? "var(--accent)" : b.winrate < 0.5 ? "var(--loss)" : "#2c313c" }} />
          </div>
        ))}
      </div>
      <div className="dayparts-labels">
        {session.map((b) => <div key={b.key}><span>{b.key === "4" ? "4-я и дальше" : `${b.key}-я`}</span><small>{b.games} {plural(b.games, "игра", "игры", "игр")}</small></div>)}
      </div>
      {notes.map((n) => <div key={n} className="note">{n}</div>)}
    </div>
  );
}

// ---------------------------------------------------------------- recent games

/** The last matches under the dashboard filter, newest first. */
function RecentGames({ d, onMatch }: { d: Dashboard; onMatch: (id: number) => void }) {
  const names = useNames();
  return (
    <div className="match-list">
      {(d.recent ?? []).map((g) => {
        const sub = [roleName(g.lane, g.team, g.lh10), modeShort(g.game_mode, g.lobby_type), relDate(g.end_time)].filter(Boolean).join(" · ");
        const gpm = g.duration >= 60 ? Math.round(g.total_gold / (g.duration / 60)) : null;
        return (
          <div key={g.match_id} className="mrow recent click" title={g.opendota ? "Без реплея — откроется на OpenDota" : undefined}
            onClick={() => openMatch(g.match_id, g.opendota, onMatch)}>
            <span className="hero-name">
              <HeroBadge npc={g.hero_name} size={40} />
              <span className="hero-name-text"><b>{names.hero(g.hero_name)}</b><small>{sub}</small></span>
            </span>
            <div><ResultPill won={g.win} />{g.opendota && <span className="pill" style={{ marginLeft: 6 }}>OpenDota</span>}</div>
            <div className="cell"><span className="mono">{g.kills}/{g.deaths}/{g.assists}</span><small>K/D/A</small></div>
            <div className="cell"><span className="mono">{g.last_hits}/{g.denies}</span><small>ЛХ/ДН</small></div>
            <div className="cell"><span className="mono">{gpm ?? "—"}</span><small>GPM</small></div>
            <div className="cell"><span className="mono">{duration(g.duration)}</span><small>длит.</small></div>
          </div>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------- records

function Records({ d, onMatch }: { d: Dashboard; onMatch: (id: number) => void }) {
  const names = useNames();
  const rec = (k: string) => (d.extras.records ?? []).find((r) => r.key === k);
  const sub = (r: { hero_name: string; end_time: number }, extra?: string) =>
    [names.hero(r.hero_name), relDate(r.end_time).replace(/, \d\d:\d\d$/, ""), extra].filter(Boolean).join(" · ");
  // Fastest purchase of the most often bought key item.
  const fast = useMemo(() => {
    const cost = (k: string) => names.raw.items?.[k]?.cost ?? 0;
    return [...(d.extras.fastest ?? [])].filter((f) => cost(f.item) >= 1800 && f.n >= 2 && !/^(aegis|cheese|refresher_shard)$/.test(f.item))
      .sort((a, b) => b.n - a.n)[0];
  }, [d, names]);
  const cards: { label: string; value: string; sub: string; tone?: string; id?: number; od?: boolean }[] = [];
  const g = rec("max_gpm"), k = rec("max_kills"), dmg = rec("max_hero_dmg"), long = rec("longest");
  if (g) cards.push({ label: "Макс. GPM", value: String(Math.round(g.value)), sub: sub(g), tone: "accent", id: g.match_id, od: g.opendota });
  if (k) cards.push({ label: "Больше всего убийств", value: String(k.value), sub: sub(k), id: k.match_id, od: k.opendota });
  if (fast) cards.push({ label: `Самый быстрый ${names.inflictor("item_" + fast.item)}`, value: clock(fast.t), sub: sub(fast), id: fast.match_id });
  if (dmg) cards.push({ label: "Урон по героям", value: `${(dmg.value / 1000).toFixed(1)}k`, sub: sub(dmg), id: dmg.match_id, od: dmg.opendota });
  if (long) cards.push({ label: "Самая длинная игра", value: duration(long.value), sub: sub(long, long.win ? "победа" : "поражение"), id: long.match_id, od: long.opendota });
  const st = d.extras.streak;
  if (st.len > 0) cards.push({
    label: "Серия побед", value: String(st.len), tone: "win",
    sub: st.len > 1 ? `подряд · ${relDate(st.from).replace(/, \d\d:\d\d$/, "")} – ${relDate(st.to).replace(/, \d\d:\d\d$/, "")}` : "пока одна",
  });
  return (
    <div className="grid3">
      {cards.map((c) => (
        <div key={c.label} className={`stat ${c.id ? "record" : ""}`} title={c.od ? "Без реплея — откроется на OpenDota" : undefined}
          onClick={() => c.id && openMatch(c.id, c.od, onMatch)}>
          <div className="caps">{c.label}</div>
          <div className={`stat-value ${c.tone ?? ""}`}>{c.value}</div>
          <div className="stat-sub">{c.sub}</div>
        </div>
      ))}
    </div>
  );
}

function PartyCard({ d }: { d: Dashboard }) {
  const { party, solo } = d.extras;
  let note = "Нет данных о пати — они приходят с OpenDota.";
  if (party.games >= 3 && solo.games >= 3) {
    const diff = Math.round((party.winrate - solo.winrate) * 100);
    note = diff >= 5 ? `С пати выигрываешь на ${diff}% чаще. Зови кентов.`
      : diff <= -5 ? `Соло выигрываешь на ${-diff}% чаще. Кенты тянут вниз?`
      : "С пати и соло играешь примерно одинаково.";
  } else if (party.games + solo.games > 0) note = "Мало игр, чтобы сравнивать.";
  const tile = (label: string, b: Bucket, good: boolean) => (
    <div className="tile" style={{ padding: "14px 16px" }}>
      <div className="label">{label} · {b.games} {plural(b.games, "игра", "игры", "игр")}</div>
      <div className={`val ${b.games ? (good ? "win" : "") : "faint"}`} style={{ fontSize: 30 }}>{b.games ? pct(b.winrate) : "—"}</div>
    </div>
  );
  return (
    <div className="card">
      <div className="card-head"><h2>С пати или соло</h2></div>
      <div className="tiles">
        {tile("С кентами", party, party.winrate >= solo.winrate)}
        {tile("Соло", solo, solo.winrate > party.winrate)}
      </div>
      <div className="note">{note}</div>
    </div>
  );
}

// ---------------------------------------------------------------- day parts

const DAY_PARTS = [
  { label: "Утро", from: 6, to: 12, when: "Утром" },
  { label: "День", from: 12, to: 18, when: "Днём" },
  { label: "Вечер", from: 18, to: 24, when: "Вечером" },
  { label: "Ночь", from: 0, to: 6, when: "Ночью" },
];

function dayParts(hours: Bucket[]) {
  return DAY_PARTS.map((p) => {
    const bs = hours.filter((b) => Number(b.key) >= p.from && Number(b.key) < p.to);
    const games = bs.reduce((a, b) => a + b.games, 0);
    const wins = bs.reduce((a, b) => a + b.wins, 0);
    return { ...p, games, wins, wr: games ? wins / games : 0 };
  });
}

/** Winrate by time of day as four bars; worst part in red, best in accent. */
export function DayParts({ hours }: { hours: Bucket[] }) {
  const parts = dayParts(hours);
  const played = parts.filter((p) => p.games >= 3);
  const best = played.length > 1 ? played.reduce((a, b) => (b.wr > a.wr ? b : a)) : null;
  const worst = played.length > 1 ? played.reduce((a, b) => (b.wr < a.wr ? b : a)) : null;
  let note = "";
  if (worst) {
    const rest = parts.filter((p) => p !== worst);
    const rg = rest.reduce((a, p) => a + p.games, 0), rw = rest.reduce((a, p) => a + p.wins, 0);
    const lossW = 1 - worst.wr, lossR = rg ? 1 - rw / rg : 0;
    if (lossR > 0 && lossW / lossR >= 1.3) {
      const k = lossW / lossR;
      note = `${worst.when} сливаешь в ${k >= 1.95 ? `${Math.round(k)} раза` : `${k.toFixed(1).replace(".", ",")} раза`} чаще.${worst.from === 0 ? " Иди спать." : ""}`;
    }
  }
  return (
    <div>
      <div className="dayparts">
        {parts.map((p) => {
          const color = p.games === 0 ? "#1f232c" : p === worst && worst !== best && p.wr < 0.5 ? "var(--loss)" : p === best ? "var(--accent)" : "#2c313c";
          return (
            <div className="col" key={p.label}>
              <b className={p === worst && p.wr < 0.5 ? "loss" : ""}>{p.games ? pct(p.wr) : "—"}</b>
              <div className="b" style={{ height: `${Math.max(4, p.wr * 100)}%`, background: color }} />
            </div>
          );
        })}
      </div>
      <div className="dayparts-labels">
        {parts.map((p) => <div key={p.label}><span>{p.label}</span><small>{p.games} {plural(p.games, "игра", "игры", "игр")}</small></div>)}
      </div>
      {note && <div className="note">{note}</div>}
    </div>
  );
}

