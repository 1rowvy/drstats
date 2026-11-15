import { useEffect, useMemo, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Api, AppEvents, AutoState, DownloadProgress, MatchRow, Progress, Settings } from "../api";
import { baseName, mb, plural, relDate } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "../components/HeroBadge";
import { Icon } from "../components/Icon";
import { useToast } from "../components/ui";

/** Queue: everything not parsed yet, failures, then the latest parsed replays. */
const QUEUE_DONE = 6;

export function ImportPage({ rows, parsing, progress, settings, onSave, onImport, onDownload, downloading, dl, dragging, onOpen }: {
  rows: MatchRow[]; parsing: Record<number, number>; progress: Progress | null; settings: Settings | null;
  onSave: (s: Settings) => Promise<boolean>; onImport: () => void; onDownload: (input: string) => Promise<boolean>;
  downloading: boolean; dl: DownloadProgress | null; dragging: boolean; onOpen: (matchID: number) => void;
}) {
  const toast = useToast();
  const [matchInput, setMatchInput] = useState("");
  const [steam, setSteam] = useState<string | null>(null);

  const queue = useMemo(() => {
    const order: Record<string, number> = { parsing: 0, pending: 1, failed: 2, done: 3 };
    const active = rows.filter((r) => r.status !== "done");
    active.sort((a, b) => order[a.status] - order[b.status]);
    const done = rows.filter((r) => r.status === "done").sort((a, b) => b.replay_id - a.replay_id).slice(0, QUEUE_DONE);
    return [...active, ...done];
  }, [rows]);
  const counts = useMemo(() => ({
    parsing: rows.filter((r) => r.status === "parsing").length,
    pending: rows.filter((r) => r.status === "pending").length,
    failed: rows.filter((r) => r.status === "failed").length,
  }), [rows]);

  const changeDir = async () => {
    if (!settings) return;
    const d = await Api.PickDirectory("Папка с реплеями");
    if (d && (await onSave({ ...settings, replay_dir: d }))) toast("Папка сохранена, сканирую…");
  };
  const reparseAll = async () => {
    const stale = progress?.stale ?? 0;
    const n = await Api.ReparseAll(stale > 0);
    toast(n ? `Перепарсинг: ${n}` : "Нечего перепарсивать");
  };
  const saveSteam = async () => {
    if (!settings || steam === null || steam.trim() === settings.steam_id) return;
    if (await onSave({ ...settings, steam_id: steam.trim() })) toast("Steam ID сохранён");
    setSteam(null);
  };
  const doneCount = progress?.done ?? rows.filter((r) => r.status === "done").length;

  return (
    <div className="grid-side">
      <div>
        <h1 style={{ marginBottom: 18 }}>Импорт реплеев</h1>
        <div className={`dropzone ${dragging ? "drag" : ""}`}>
          <div className="up"><Icon name="upload" size={20} /></div>
          <h2>Кидай сюда .dem файлы</h2>
          <p>Можно сразу пачкой или целой папкой. Повторно уже распарсенные реплеи пропускаются.</p>
          <button className="primary" onClick={onImport}>Выбрать файлы</button>
        </div>

        <div className="card" style={{ marginTop: 14 }}>
          <h3 style={{ marginBottom: 12 }}>Скачать по match ID через OpenDota</h3>
          <div className="row" style={{ marginBottom: 8 }}>
            <input className="mono" placeholder="Например, из ссылки на Dotabuff" value={matchInput} onChange={(e) => setMatchInput(e.target.value)}
              onKeyDown={async (e) => { if (e.key === "Enter" && matchInput.trim() && (await onDownload(matchInput))) setMatchInput(""); }} />
            <button disabled={downloading || !matchInput.trim()} onClick={async () => { if (await onDownload(matchInput)) setMatchInput(""); }}>Скачать</button>
          </div>
          {dl && dl.stage !== "done" && dl.stage !== "failed" ? (
            <div className="small dim">
              Матч {dl.match_id}: {dl.stage === "lookup" ? "ищу реплей…" : `${Math.round(dl.bytes / 1048576)}${dl.total ? ` из ${Math.round(dl.total / 1048576)}` : ""} МБ`}
              <div className="bar thin" style={{ marginTop: 6 }}><div style={{ width: dl.total ? `${(dl.bytes / dl.total) * 100}%` : "10%" }} /></div>
            </div>
          ) : <div className="faint small">Valve хранит реплеи недолго, старые матчи скачать не получится.</div>}
        </div>

        <div className="card" style={{ marginTop: 14 }}>
          <div className="card-head">
            <h2>Очередь</h2>
            <span className="meta">
              {counts.parsing} в работе · {counts.pending} ждут · {counts.failed} {plural(counts.failed, "ошибка", "ошибки", "ошибок")}
            </span>
          </div>
          {queue.length === 0 ? <div className="empty">Пусто — импортируй реплеи.</div> : (
            <div className="queue">
              {queue.map((r) => <QueueItem key={r.replay_id} r={r} p={parsing[r.replay_id]} onOpen={onOpen} />)}
            </div>
          )}
        </div>
      </div>

      <div className="stack">
        <div className="card side-card">
          <h3><span className="dot" style={{ background: settings?.watch_off ? "var(--faint)" : "var(--win)" }} />
            {settings?.watch_off ? "Папка реплеев" : "Слежу за папкой"}</h3>
          <div className="path-box">{settings?.replay_dir || "не задана"}</div>
          <button className="block" onClick={changeDir}>Сменить папку</button>
          <div className="opt-row">
            <div><b>Парсить новые сразу</b><small>как только реплей появился</small></div>
            <button className={`toggle ${settings && !settings.watch_off ? "on" : ""}`} aria-label="Парсить новые сразу"
              onClick={() => settings && onSave({ ...settings, watch_off: !settings.watch_off })} />
          </div>
          <div className="opt-row">
            <div><b>Параллельно</b><small>реплеев за раз</small></div>
            <div className="stepper">
              <button disabled={!settings || settings.parallelism <= 1} onClick={() => settings && onSave({ ...settings, parallelism: settings.parallelism - 1 })}>−</button>
              <b>{settings?.parallelism ?? "—"}</b>
              <button disabled={!settings} onClick={() => settings && onSave({ ...settings, parallelism: settings.parallelism + 1 })}>+</button>
            </div>
          </div>
        </div>

        <AutoDownloadCard settings={settings} onSave={onSave} />

        <div className="card side-card">
          <h3>Твой Steam ID</h3>
          <input className="mono" placeholder="[ТВОЙ STEAM ID]" value={steam ?? settings?.steam_id ?? ""}
            onChange={(e) => setSteam(e.target.value)} onBlur={saveSteam} onKeyDown={(e) => e.key === "Enter" && saveSteam()} />
          <div className="faint small" style={{ marginTop: 8 }}>Подходит 32-битный account id и steam64. По нему находим тебя в каждом реплее.</div>
        </div>

        <div className="card side-card">
          <div className="kpis">
            <div><b>{doneCount}</b><small>в базе</small></div>
            <div><b>{progress?.avg_parse_ms ? `${(progress.avg_parse_ms / 1000).toFixed(1)} с` : "—"}</b><small>в среднем на реплей</small></div>
          </div>
          <button className="block" onClick={reparseAll}>
            {progress?.stale ? `Перепарсить устаревшие (${progress.stale})` : "Перепарсить всё"}
          </button>
          {progress?.stale ? <div className="faint small" style={{ marginTop: 8 }}>Вышла новая версия парсера — старые матчи покажут не всё.</div> : null}
        </div>
      </div>
    </div>
  );
}

const GB = 1 << 30;

/** Auto-download of recent games without a replay and the storage cap of those files. */
function AutoDownloadCard({ settings, onSave }: { settings: Settings | null; onSave: (s: Settings) => Promise<boolean> }) {
  const [st, setSt] = useState<AutoState | null>(null);
  useEffect(() => {
    Api.AutoDownloadState().then(setSt).catch(console.error);
    return Events.On(AppEvents.AutodlState, (e) => setSt(e.data));
  }, []);
  // Settings changes (toggle, cap) also change the state: refresh.
  useEffect(() => { Api.AutoDownloadState().then(setSt).catch(console.error); }, [settings]);
  if (!settings) return null;
  const on = settings.auto_download;
  const keep = settings.replay_keep || "cap";
  const cap = settings.replay_cap_gb || 5;
  const stored = st?.stored_bytes ?? 0;
  const gb = (b: number) => `${(b / GB).toFixed(b >= 10 * GB ? 0 : 1).replace(".", ",")} ГБ`;
  const setCap = (v: number) => onSave({ ...settings, replay_cap_gb: Math.max(1, Math.min(1000, v)) });

  let status = "";
  if (on && settings.opendota_off) status = "Нужна OpenDota — она выключена в настройках.";
  else if (on && !settings.steam_id) status = "Сначала укажи Steam ID.";
  else if (st?.current) status = `Качаю матч ${st.current}…${st.queued ? ` ещё ${st.queued} в очереди` : ""}`;
  else if (on) status = st?.done ? `Скачано ${st.done} ${plural(st.done, "реплей", "реплея", "реплеев")}, новых нет.` : "Новых игр без реплея нет.";
  const extra = [
    st?.waiting ? `${st.waiting} ждут OpenDota` : "",
    st?.expired ? `${st.expired} Valve уже удалил` : "",
  ].filter(Boolean).join(" · ");

  return (
    <div className="card side-card">
      <h3><span className="dot" style={{ background: on ? (st?.current ? "var(--accent)" : "var(--win)") : "var(--faint)" }} />Автозагрузка</h3>
      <div className="opt-row">
        <div><b>Скачивать свежие игры</b><small>реплеи за 2 недели, которых нет в базе</small></div>
        <button className={`toggle ${on ? "on" : ""}`} aria-label="Скачивать свежие игры"
          onClick={() => onSave({ ...settings, auto_download: !on })} />
      </div>
      <div className="opt-row">
        <div><b>После разбора</b><small>только скачанные сами</small></div>
        <select value={keep} onChange={(e) => onSave({ ...settings, replay_keep: e.target.value })} style={{ width: "auto" }}>
          <option value="cap">хранить до лимита</option>
          <option value="delete">удалять сразу</option>
          <option value="all">хранить все</option>
        </select>
      </div>
      {keep === "cap" && (
        <div className="opt-row">
          <div><b>Лимит</b><small>старые удаляются первыми</small></div>
          <div className="stepper">
            <button disabled={cap <= 1} onClick={() => setCap(cap - 1)}>−</button>
            <b>{cap} ГБ</b>
            <button onClick={() => setCap(cap + 1)}>+</button>
          </div>
        </div>
      )}
      <div className="small dim" style={{ marginTop: 10 }}>
        Занято {gb(stored)}{keep === "cap" ? ` из ${cap} ГБ` : ""}{st?.stored_files ? ` · ${st.stored_files} ${plural(st.stored_files, "файл", "файла", "файлов")}` : ""}
      </div>
      {keep === "cap" && <div className="bar thin" style={{ marginTop: 6 }}><div style={{ width: `${Math.min(100, (stored / (cap * GB)) * 100)}%` }} /></div>}
      {status && <div className={`small ${on && (settings.opendota_off || !settings.steam_id) ? "loss" : "faint"}`} style={{ marginTop: 8 }}>{status}</div>}
      {extra && <div className="faint small" style={{ marginTop: 4 }}>{extra}</div>}
      <div className="faint small" style={{ marginTop: 8 }}>
        Папка Dota и скачанные вручную реплеи не трогаются. Статистика удалённых остаётся, но перепарсить их новой версией парсера уже нельзя.
      </div>
    </div>
  );
}

function QueueItem({ r, p, onOpen }: { r: MatchRow; p?: number; onOpen: (id: number) => void }) {
  const names = useNames();
  const title = r.found ? `${names.hero(r.hero_name)} · ${relDate(r.end_time)}` : r.match_id ? `Матч ${r.match_id}` : baseName(r.path);
  const file = `${baseName(r.path)}${r.size && !r.pruned ? ` · ${mb(r.size)}` : ""}`;
  let status: React.ReactNode;
  switch (r.status) {
    case "parsing": status = <span className="qs busy">{Math.round((p ?? 0) * 100)}%</span>; break;
    case "pending": status = <span className="qs">В очереди</span>; break;
    case "failed": status = <span className="qs err">Ошибка</span>; break;
    default: status = r.stale
      ? <span className="qs" style={{ color: "var(--warn)" }}>Устарел</span>
      : <span className="qs ok" title={r.pruned ? "Файл удалён по лимиту места, статистика сохранена" : undefined}>
          ✓ Готово{r.parse_ms ? ` · ${(r.parse_ms / 1000).toFixed(1)} с` : ""}{r.pruned ? " · файл удалён" : ""}
        </span>;
  }
  const open = r.status === "done" && r.match_id ? () => onOpen(r.match_id) : undefined;
  return (
    <div className={`qitem ${r.status === "parsing" ? "active" : ""} ${r.status === "failed" ? "failed" : ""}`}
      style={open ? { cursor: "pointer" } : undefined} onClick={open}>
      <div className="qh">
        {r.found ? <HeroBadge npc={r.hero_name} size={28} /> : <span className="file-ico"><Icon name="file" size={14} /></span>}
        <div style={{ minWidth: 0 }}>
          <b>{title}</b>
          <small title={r.path}>{file}</small>
        </div>
        {status}
      </div>
      {r.status === "parsing" && <div className="bar" style={{ marginTop: 10 }}><div style={{ width: `${(p ?? 0) * 100}%` }} /></div>}
      {r.status === "failed" && (
        <>
          <div className="err-text">{r.error || "неизвестная ошибка"}</div>
          <button className="danger" style={{ marginTop: 10, padding: "5px 11px", fontSize: 12 }}
            onClick={(e) => { e.stopPropagation(); Api.Reparse(r.replay_id).catch(console.error); }}>Повторить</button>
        </>
      )}
      {r.status === "done" && r.stale && (
        <button className="link small" style={{ marginTop: 6 }} onClick={(e) => { e.stopPropagation(); Api.Reparse(r.replay_id).catch(console.error); }}>
          перепарсить
        </button>
      )}
    </div>
  );
}
