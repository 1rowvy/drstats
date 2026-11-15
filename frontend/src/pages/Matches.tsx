import { useMemo, useState } from "react";
import { Api, MatchRow, Settings } from "../api";
import { baseName, duration, modeShort, plural, relDate, roleName, signed } from "../format";
import { useNames } from "../names";
import { HeroName } from "../components/HeroBadge";
import { Icon } from "../components/Icon";

type Result = "" | "win" | "loss";

const won = (r: MatchRow) => (r.found && r.winner !== 0 ? r.winner === r.team : null);
const avg = (xs: number[]) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null);

export function MatchesPage({ rows, settings, parsing, onOpen, onImport, onScan, onDownload, downloading }: {
  rows: MatchRow[]; settings: Settings | null; parsing: Record<number, number>;
  onOpen: (matchID: number) => void; onImport: () => void; onScan: () => void;
  onDownload: (input: string) => Promise<boolean>; downloading: boolean;
}) {
  const names = useNames();
  const [q, setQ] = useState("");
  const [hero, setHero] = useState(0);
  const [mode, setMode] = useState("");
  const [result, setResult] = useState<Result>("");
  const [errorRow, setErrorRow] = useState<MatchRow | null>(null);

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
  const failed = rows.filter((r) => r.status === "failed").length;

  return (
    <div>
      <div className="page-head">
        <div>
          <h1>Матчи</h1>
          <div className="sub">
            {owner ? `${owner} · ` : ""}{rows.length} {plural(rows.length, "матч", "матча", "матчей")} в базе
            {failed > 0 && <span className="loss"> · ошибок {failed}</span>}
          </div>
        </div>
        <div className="actions">
          <button onClick={onScan}><Icon name="refresh" size={14} /> Сканировать папку</button>
          <button className="primary" onClick={onImport}><Icon name="plus" size={14} /> Импорт реплея</button>
        </div>
      </div>

      {!settings?.steam_id && (
        <div className="notice">Укажи свой Steam ID в настройках — иначе не понять, за кого ты играл.</div>
      )}
      {mine.length > 0 && <Summary mine={mine} goal={settings?.lh10_goal || 70} />}

      <div className="filter-bar">
        <label className="field">
          <span>Поиск</span>
          <input placeholder="Герой, match ID или файл" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
        <label className="field">
          <span>Герой</span>
          <select value={hero} onChange={(e) => setHero(Number(e.target.value))}>
            <option value={0}>Все герои</option>
            {heroes.map(([id, h]) => <option key={id} value={id}>{names.hero(h.npc)} ({h.n})</option>)}
          </select>
        </label>
        <label className="field">
          <span>Режим</span>
          <select value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="">Все режимы</option>
            {modes.map((m) => <option key={m} value={m}>{m}</option>)}
          </select>
        </label>
        <div className="seg">
          {([["", "Все"], ["win", "Победы"], ["loss", "Поражения"]] as [Result, string][]).map(([k, l]) => (
            <button key={k} className={result === k ? "on" : ""} onClick={() => setResult(k)}>{l}</button>
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
          Реплеев пока нет. Перетащи <code>.dem</code> файлы в окно, нажми «Импорт реплея» или проверь папку реплеев в настройках.
        </div>
      ) : (
        <div className="card table-card">
          <table className="table">
            <thead>
              <tr>
                <th>Герой</th><th>Результат</th><th>K / D / A</th><th>ЛХ / ДН</th><th>GPM</th><th>Длит.</th>
                <th>Режим</th><th>Дата</th><th>Статус</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((r) => <Row key={r.replay_id} r={r} parsing={parsing[r.replay_id]} onOpen={onOpen} onError={setErrorRow} />)}
            </tbody>
          </table>
          {shown.length === 0 && <div className="empty">Под фильтр ничего не попало.</div>}
        </div>
      )}

      {errorRow && <ErrorModal r={errorRow} onClose={() => setErrorRow(null)} />}
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
  return (
    <div className="stat-grid">
      <div className="stat">
        <div className="stat-label">Винрейт, {last.length} {plural(last.length, "игра", "игры", "игр")}</div>
        <div className="stat-value">{Math.round((wins / last.length) * 100)}%</div>
        <div className="stat-sub">{wins} {plural(wins, "победа", "победы", "побед")} · {last.length - wins} {plural(last.length - wins, "поражение", "поражения", "поражений")}</div>
      </div>
      <div className="stat">
        <div className="stat-label">Средний KDA</div>
        <div className="stat-value">{k!.toFixed(1)}/{d!.toFixed(1)}/{a!.toFixed(1)}</div>
        <div className="stat-sub">по всем героям</div>
      </div>
      <div className="stat">
        <div className="stat-label">ЛХ на 10 минуте</div>
        <div className="stat-value">{lh10 == null ? "—" : Math.round(lh10)}</div>
        <div className="stat-sub">цель: {goal}</div>
      </div>
      <div className="stat">
        <div className="stat-label">Средний GPM</div>
        <div className="stat-value">{gpm == null ? "—" : Math.round(gpm)}</div>
        <div className="stat-sub">
          {gpmDiff == null ? `последние ${last.length}` : (
            <span className={gpmDiff >= 0 ? "up" : "down"}>{gpmDiff >= 0 ? "↑" : "↓"} {signed(gpmDiff)} <span className="faint">к прошлым {prev.length} играм</span></span>
          )}
        </div>
      </div>
    </div>
  );
}

function Row({ r, parsing, onOpen, onError }: {
  r: MatchRow; parsing?: number; onOpen: (id: number) => void; onError: (r: MatchRow) => void;
}) {
  const done = r.status === "done" && r.match_id !== 0;
  const w = won(r);
  const dash = <span className="faint">—</span>;
  return (
    <tr className={done ? "clickable" : ""} onClick={() => done && onOpen(r.match_id)}>
      <td>
        {r.found ? <HeroName npc={r.hero_name} sub={roleName(r.lane, r.team, r.lh10, r.position) || " "} />
          : done ? <span className="dim">не мой матч <span className="faint mono small">{r.match_id}</span></span>
          : <span className="dim" title={r.path}>{baseName(r.path)}</span>}
      </td>
      <td>{w === null ? dash : w ? <span className="pill win">Победа</span> : <span className="pill loss">Поражение</span>}</td>
      <td className="num">{r.found ? `${r.kills}/${r.deaths}/${r.assists}` : dash}</td>
      <td className="num">{r.found ? `${r.last_hits}/${r.denies}` : dash}</td>
      <td className="num">{r.found ? r.gpm : dash}</td>
      <td className="num">{done ? duration(r.duration) : dash}</td>
      <td>{done ? modeShort(r.game_mode, r.lobby_type) : dash}</td>
      <td>{relDate(done ? r.end_time : r.mtime)}</td>
      <td><Status r={r} parsing={parsing} onError={onError} /></td>
    </tr>
  );
}

function Status({ r, parsing, onError }: { r: MatchRow; parsing?: number; onError: (r: MatchRow) => void }) {
  const reparse = (e: React.MouseEvent) => {
    e.stopPropagation();
    Api.Reparse(r.replay_id).catch(console.error);
  };
  switch (r.status) {
    case "pending":
      return <span className="done-mark">В очереди</span>;
    case "parsing": {
      const p = parsing ?? 0;
      return (
        <div className="parse-cell">
          Парсинг {Math.round(p * 100)}%
          <div className="bar"><div style={{ width: `${p * 100}%` }} /></div>
        </div>
      );
    }
    case "failed":
      return (
        <span className="pill err" onClick={(e) => { e.stopPropagation(); onError(r); }} title={r.error}>
          <Icon name="alert" size={12} /> Ошибка — подробнее
        </span>
      );
    default:
      return r.stale ? (
        <span className="done-mark">
          <span className="pill warn" title="Разобран старой версией парсера">устарел</span>
          <button className="link small" onClick={reparse}>перепарсить</button>
        </span>
      ) : <span className="done-mark"><Icon name="check" size={12} /> Готово</span>;
  }
}

function ErrorModal({ r, onClose }: { r: MatchRow; onClose: () => void }) {
  return (
    <div className="modal-back" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="card-head"><h2>Не удалось разобрать реплей</h2><button className="back" onClick={onClose}><Icon name="x" /></button></div>
        <div className="dim small" style={{ marginBottom: 10 }}>{r.path}</div>
        <pre>{r.error || "Неизвестная ошибка"}</pre>
        <div className="actions" style={{ justifyContent: "flex-end", marginTop: 14 }}>
          <button onClick={onClose}>Закрыть</button>
          <button className="primary" onClick={() => { Api.Reparse(r.replay_id).catch(console.error); onClose(); }}>
            <Icon name="refresh" size={14} /> Повторить
          </button>
        </div>
      </div>
    </div>
  );
}
