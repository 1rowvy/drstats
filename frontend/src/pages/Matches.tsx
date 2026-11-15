import { useMemo, useState } from "react";
import { Api, MatchRow, Settings } from "../api";
import { baseName, duration, gradeTone, modeShort, plural, relDate, roleName, signed } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "../components/HeroBadge";
import { Icon } from "../components/Icon";
import { ResultPill, Stat } from "../components/ui";

type Result = "" | "win" | "loss";

const won = (r: MatchRow) => (r.found && r.winner !== 0 ? r.winner === r.team : null);
const avg = (xs: number[]) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null);

export function MatchesPage({ rows, settings, parsing, onOpen, onImport, onScan, onDownload, downloading, onShowImport }: {
  rows: MatchRow[]; settings: Settings | null; parsing: Record<number, number>;
  onOpen: (matchID: number) => void; onImport: () => void; onScan: () => void;
  onDownload: (input: string) => Promise<boolean>; downloading: boolean; onShowImport: () => void;
}) {
  const names = useNames();
  const [q, setQ] = useState("");
  const [hero, setHero] = useState(0);
  const [mode, setMode] = useState("");
  const [result, setResult] = useState<Result>("");

  const mine = useMemo(() => rows.filter((r) => r.found && r.status === "done"), [rows]);
  const heroes = useMemo(() => {
    const m = new Map<number, { npc: string; n: number }>();
    for (const r of mine) {
      const h = m.get(r.hero_id) ?? { npc: r.hero_name, n: 0 };
      h.n++;
      m.set(r.hero_id, h);
    }
    return [...m.entries()].sort((a, b) => b[1].n - a[1].n);
  }, [mine]);
  const modes = useMemo(() => [...new Set(rows.filter((r) => r.match_id).map((r) => modeShort(r.game_mode, r.lobby_type)))].sort(), [rows]);

  const query = q.trim().toLowerCase();
  const shown = useMemo(() => rows.filter((r) => {
    if (hero && r.hero_id !== hero) return false;
    if (mode && (!r.match_id || modeShort(r.game_mode, r.lobby_type) !== mode)) return false;
    if (result && won(r) !== (result === "win")) return false;
    if (query) {
      const hay = [String(r.match_id || ""), baseName(r.path), r.found ? names.hero(r.hero_name) : ""].join(" ").toLowerCase();
      if (!hay.includes(query)) return false;
    }
    return true;
  }), [rows, hero, mode, result, query, names]);

  const idQuery = /^\d{6,}$/.test(q.trim()) ? q.trim() : "";
  const idMissing = idQuery && !rows.some((r) => String(r.match_id) === idQuery);
  const owner = mine.find((r) => r.name)?.name;

  return (
    <div>
      <div className="page-head">
        <div>
          <div className="caps">{owner ? `${owner} · ` : ""}{rows.length} {plural(rows.length, "матч", "матча", "матчей")} в базе</div>
          <h1>Матчи</h1>
        </div>
        <div className="actions">
          <button className="ghost" onClick={onScan}><Icon name="refresh" size={14} /> Сканировать папку</button>
          <button className="primary" onClick={onImport}><Icon name="plus" size={14} /> Импорт реплея</button>
        </div>
      </div>

      {!settings?.steam_id && (
        <div className="notice">Укажи свой Steam ID на странице «Импорт» — иначе не понять, за кого ты играл.</div>
      )}
      {mine.length > 0 && <Summary mine={mine} goal={settings?.lh10_goal || 70} />}

      <div className="filter-row">
        <label className="search">
          <Icon name="search" size={14} />
          <input placeholder="Герой, предмет, match ID" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
        <select value={hero} onChange={(e) => setHero(Number(e.target.value))}>
          <option value={0}>Все герои</option>
          {heroes.map(([id, h]) => <option key={id} value={id}>{names.hero(h.npc)} ({h.n})</option>)}
        </select>
        <select value={mode} onChange={(e) => setMode(e.target.value)}>
          <option value="">Все режимы</option>
          {modes.map((m) => <option key={m} value={m}>{m}</option>)}
        </select>
        <div className="chips">
          {([["", "Все"], ["win", "Победы"], ["loss", "Поражения"]] as [Result, string][]).map(([k, l]) => (
            <button key={k} className={`chip ${result === k ? "on" : ""}`} onClick={() => setResult(k)}>{l}</button>
          ))}
        </div>
      </div>

      {idMissing && (
        <div className="card dl-hint">
          <span>Матча <span className="mono">{idQuery}</span> нет в базе.</span>
          <button className="primary" disabled={downloading}
            onClick={async () => { if (await onDownload(idQuery)) setQ(""); }}>
            <Icon name="download" size={14} /> Скачать через OpenDota
          </button>
        </div>
      )}

      {rows.length === 0 ? (
        <div className="card empty">
          Реплеев пока нет. Перетащи <code>.dem</code> файлы в окно, нажми «Импорт реплея» или проверь папку реплеев на странице «Импорт».
        </div>
      ) : (
        <div className="match-list">
          {shown.map((r) => <Row key={r.replay_id} r={r} parsing={parsing[r.replay_id]} onOpen={onOpen} onShowImport={onShowImport} />)}
          {shown.length === 0 && <div className="card empty">Под фильтр ничего не попало.</div>}
        </div>
      )}
    </div>
  );
}

function Summary({ mine, goal }: { mine: MatchRow[]; goal: number }) {
  const last = mine.slice(0, 20);
  const prev = mine.slice(20, 40);
  const wins = last.filter((r) => won(r)).length;
  const k = avg(mine.map((r) => r.kills)), d = avg(mine.map((r) => r.deaths)), a = avg(mine.map((r) => r.assists));
  const lh10 = avg(last.filter((r) => r.lh10 != null).map((r) => r.lh10!));
  const gpm = avg(last.map((r) => r.gpm));
  const gpmPrev = avg(prev.map((r) => r.gpm));
  const gpmDiff = gpm != null && gpmPrev != null && prev.length >= 5 ? gpm - gpmPrev : null;
  const wr = Math.round((wins / last.length) * 100);
  return (
    <div className="stat-grid">
      <Stat label={`Винрейт, ${last.length} ${plural(last.length, "игра", "игры", "игр")}`} big value={`${wr}%`} tone={wr >= 50 ? "accent" : "loss"}
        sub={`${wins} ${plural(wins, "победа", "победы", "побед")} · ${last.length - wins} ${plural(last.length - wins, "поражение", "поражения", "поражений")}`} />
      <Stat label="Средний KDA" big value={`${k!.toFixed(1)}/${d!.toFixed(1)}/${a!.toFixed(1)}`} sub="по всем героям" />
      <Stat label="ЛХ на 10 мин" big value={lh10 == null ? "—" : Math.round(lh10)} sub={`цель — ${goal}`} />
      <Stat label="Средний GPM" big value={gpm == null ? "—" : Math.round(gpm)}
        sub={gpmDiff == null ? `последние ${last.length}` : (
          <span className={gpmDiff >= 0 ? "up" : "down"}>{gpmDiff >= 0 ? "↑" : "↓"} {signed(gpmDiff)} к прошлым {prev.length}</span>
        )} />
    </div>
  );
}

function Row({ r, parsing, onOpen, onShowImport }: {
  r: MatchRow; parsing?: number; onOpen: (id: number) => void; onShowImport: () => void;
}) {
  const names = useNames();
  const done = r.status === "done" && r.match_id !== 0;
  const role = r.found ? roleName(r.lane, r.team, r.lh10, r.position) : "";
  const sub = [role, done ? modeShort(r.game_mode, r.lobby_type) : "", relDate(done ? r.end_time : r.mtime)].filter(Boolean).join(" · ");
  const hero = r.found ? (
    <span className="hero-name">
      <HeroBadge npc={r.hero_name} size={40} />
      <span className="hero-name-text"><b>{names.hero(r.hero_name)}</b><small>{sub}</small></span>
    </span>
  ) : (
    <span className="hero-name">
      <span className="file-ico" style={{ width: 40, height: 40 }}><Icon name="file" size={16} /></span>
      <span className="hero-name-text">
        <b title={r.path}>{done ? `Матч ${r.match_id}` : baseName(r.path)}</b>
        <small>{done ? `не мой матч · ${sub}` : relDate(r.mtime)}</small>
      </span>
    </span>
  );

  if (r.status === "parsing" || r.status === "pending") {
    const p = parsing ?? 0;
    return (
      <div className="mrow">
        {hero}
        <div className="wide progress-line">
          {r.status === "pending" ? <span className="dim">В очереди на парсинг</span> : (
            <>
              <span>Парсинг {Math.round(p * 100)}%</span>
              <div className="bar"><div style={{ width: `${p * 100}%` }} /></div>
            </>
          )}
        </div>
      </div>
    );
  }
  if (r.status === "failed") {
    return (
      <div className="mrow">
        {hero}
        <div className="wide">
          <span className="err-line" onClick={onShowImport} title={r.error}>
            <Icon name="alert" size={13} /> Не распарсился — открыть в импорте
          </span>
        </div>
      </div>
    );
  }
  const w = won(r);
  return (
    <div className="mrow click" onClick={() => done && onOpen(r.match_id)}>
      {hero}
      <div><ResultPill won={w} />{r.stale && <span className="pill warn" style={{ marginLeft: 6 }} title="Разобран старой версией парсера — перепарси на странице «Импорт»">старый</span>}</div>
      {r.found ? (
        <>
          <div className="cell"><span className="mono">{r.kills}/{r.deaths}/{r.assists}</span><small>K/D/A</small></div>
          <div className="cell"><span className="mono">{r.last_hits}/{r.denies}</span><small>ЛХ/ДН</small></div>
          <div className="cell"><span className="mono">{r.gpm}</span><small>GPM</small></div>
          <div className="cell"><span className="mono">{duration(r.duration)}</span><small>длит.</small></div>
          <div className="grade" title={r.grade ? `${r.grade_score} / 100 — по сравнению с твоими же матчами` : "Нужно больше разобранных матчей"}>
            <b className={gradeTone(r.grade)}>{r.grade || "—"}</b><small>оценка</small>
          </div>
        </>
      ) : (
        <div className="wide dim small">Тебя нет в этом матче — можно разобрать за любого игрока.</div>
      )}
    </div>
  );
}

// Kept for the import page: re-parse a single replay.
export const reparse = (id: number) => Api.Reparse(id).catch(console.error);
