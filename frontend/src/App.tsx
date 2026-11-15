import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Api, AppEvents, DownloadProgress, MatchRow, Progress, Settings } from "./api";
import { NamesProvider } from "./names";
import { MatchesPage } from "./pages/Matches";
import { MatchPage } from "./pages/MatchPage";
import { SettingsPage } from "./pages/Settings";
import { DashboardPage } from "./pages/Dashboard";
import { HeroPage } from "./pages/HeroPage";

type Route =
  | { page: "dashboard" }
  | { page: "matches" }
  | { page: "match"; id: number }
  | { page: "hero"; id: number }
  | { page: "settings" };

export default function App() {
  return (
    <NamesProvider>
      <Shell />
    </NamesProvider>
  );
}

function Shell() {
  const [history, setHistory] = useState<Route[]>([{ page: "dashboard" }]);
  const route = history[history.length - 1];
  const go = useCallback((r: Route) => setHistory((h) => [...h, r].slice(-30)), []);
  const back = useCallback(() => setHistory((h) => (h.length > 1 ? h.slice(0, -1) : h)), []);
  const top = (r: Route) => setHistory([r]);
  const [tick, setTick] = useState(0);
  const [dl, setDl] = useState<DownloadProgress | null>(null);
  const [matchInput, setMatchInput] = useState("");
  const [rows, setRows] = useState<MatchRow[]>([]);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [version, setVersion] = useState("");
  const [toast, setToast] = useState<{ msg: string; bad: boolean } | null>(null);
  const [dragging, setDragging] = useState(false);
  const toastTimer = useRef<number>();
  const reloadTimer = useRef<number>();

  const showToast = useCallback((msg: string, bad = false) => {
    setToast({ msg, bad });
    window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setToast(null), bad ? 8000 : 3500);
  }, []);

  const reload = useCallback(() => {
    Api.ListMatches().then((r) => setRows(r ?? [])).catch(console.error);
    setTick((t) => t + 1);
  }, []);
  // Coalesce bursts of replay events (a big import emits many).
  const reloadSoon = useCallback(() => {
    window.clearTimeout(reloadTimer.current);
    reloadTimer.current = window.setTimeout(reload, 300);
  }, [reload]);

  useEffect(() => {
    reload();
    Api.Progress().then(setProgress);
    Api.GetSettings().then(setSettings);
    Api.Info().then((i) => setVersion(i.version));
    const offs = [
      Events.On(AppEvents.LibraryReplay, reloadSoon),
      Events.On(AppEvents.LibraryProgress, (e) => setProgress(e.data)),
      Events.On(AppEvents.LibraryImported, (e) => {
        const r = e.data;
        if (r.error) showToast(r.error, true);
        else if (r.queued > 0) showToast(`В очередь: ${r.queued}`);
        reloadSoon();
      }),
      Events.On(AppEvents.DownloadProgress, (e) => {
        const p: DownloadProgress = e.data;
        setDl(p);
        if (p.stage === "failed") showToast(`Матч ${p.match_id}: ${p.error}`, true);
        if (p.stage === "done") {
          showToast(`Матч ${p.match_id} скачан, разбираю…`);
          window.setTimeout(() => setDl(null), 2000);
        }
      }),
    ];
    return () => offs.forEach((off) => off());
  }, [reload, reloadSoon, showToast]);

  const importFiles = async () => {
    try {
      await Api.ImportDialog();
    } catch (e: any) {
      showToast(String(e?.message ?? e), true);
    }
  };
  const scan = async () => {
    const r = await Api.ScanReplayDir();
    if (!r.error && r.queued === 0) showToast("Новых реплеев нет");
  };

  const download = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!matchInput.trim()) return;
    try {
      const id = await Api.DownloadMatch(matchInput);
      setDl({ match_id: id, stage: "lookup", bytes: 0, total: 0, path: "", error: "" });
      setMatchInput("");
    } catch (err: any) {
      showToast(String(err?.message ?? err), true);
    }
  };
  const reparseStale = async () => {
    const n = await Api.ReparseAll(true);
    showToast(n ? `Перепарсинг: ${n}` : "Нечего перепарсивать");
  };
  const dlBusy = dl && dl.stage !== "done" && dl.stage !== "failed";

  const busy = progress ? progress.pending + progress.parsing : 0;

  return (
    <div
      className="shell"
      data-file-drop-target
      onDragEnter={() => setDragging(true)}
      onDragLeave={(e) => { if (e.currentTarget === e.target) setDragging(false); }}
      onDrop={() => setDragging(false)}
    >
      <aside className="sidebar">
        <div className="logo">drstats</div>
        <nav>
          <button className={route.page === "dashboard" || route.page === "hero" ? "on" : ""} onClick={() => top({ page: "dashboard" })}>Дашборд</button>
          <button className={route.page === "matches" || route.page === "match" ? "on" : ""} onClick={() => top({ page: "matches" })}>Матчи</button>
          <button className={route.page === "settings" ? "on" : ""} onClick={() => top({ page: "settings" })}>Настройки</button>
        </nav>
        <div className="side-actions">
          <button onClick={importFiles}>Импорт…</button>
          <button onClick={scan}>Сканировать</button>
          <form className="dl-form" onSubmit={download}>
            <input placeholder="ID матча" value={matchInput} onChange={(e) => setMatchInput(e.target.value)} disabled={!!dlBusy} />
            <button type="submit" disabled={!!dlBusy || !matchInput.trim()} title="Скачать реплей через OpenDota">↓</button>
          </form>
          {dlBusy && dl && (
            <div className="queue" style={{ padding: 0 }}>
              <div className="bar"><div style={{ width: dl.total ? `${(dl.bytes / dl.total) * 100}%` : "15%" }} /></div>
              <div>{dl.stage === "lookup" ? `Ищу матч ${dl.match_id}…` : `Скачиваю: ${(dl.bytes / 1048576).toFixed(0)}${dl.total ? ` из ${(dl.total / 1048576).toFixed(0)}` : ""} МБ`}</div>
            </div>
          )}
        </div>
        {progress && (
          <div className="queue">
            {busy > 0 ? (
              <>
                <div className="bar"><div style={{ width: `${(progress.done + progress.failed) / Math.max(1, progress.done + progress.failed + busy) * 100}%` }} /></div>
                <div>Парсинг: {progress.parsing}, в очереди {progress.pending}</div>
              </>
            ) : (
              <div>Разобрано: {progress.done}{progress.failed ? `, ошибок ${progress.failed}` : ""}</div>
            )}
            {progress.stale > 0 && busy === 0 && (
              <div className="stale">
                Устарели после обновления парсера: {progress.stale}{" "}
                <button className="link" onClick={reparseStale}>перепарсить</button>
              </div>
            )}
          </div>
        )}
        <div className="version">{version}</div>
      </aside>
      <main className="content">
        {route.page === "dashboard" && (
          <DashboardPage tick={tick} onMatch={(id) => go({ page: "match", id })} onHero={(id) => go({ page: "hero", id })} />
        )}
        {route.page === "matches" && (
          <MatchesPage rows={rows} hasSteamID={!!settings?.steam_id} onOpen={(id) => go({ page: "match", id })} />
        )}
        {route.page === "match" && (
          <MatchPage key={route.id} matchID={route.id} onBack={back} onHero={(id) => go({ page: "hero", id })} />
        )}
        {route.page === "hero" && (
          <HeroPage key={route.id} heroID={route.id} tick={tick} onBack={back} onMatch={(id) => go({ page: "match", id })} />
        )}
        {route.page === "settings" && (
          <SettingsPage toast={showToast} onSaved={(s) => { setSettings(s); reload(); }} />
        )}
      </main>
      {dragging && <div className="drop-overlay">Отпусти, чтобы импортировать реплеи</div>}
      {toast && <div className={`toast ${toast.bad ? "bad" : ""}`}>{toast.msg}</div>}
    </div>
  );
}
