import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Api, AppEvents, MatchRow, Progress, Settings } from "./api";
import { NamesProvider } from "./names";
import { MatchesPage } from "./pages/Matches";
import { MatchPage } from "./pages/MatchPage";
import { SettingsPage } from "./pages/Settings";

type Route = { page: "matches" } | { page: "match"; id: number } | { page: "settings" };

export default function App() {
  return (
    <NamesProvider>
      <Shell />
    </NamesProvider>
  );
}

function Shell() {
  const [route, setRoute] = useState<Route>({ page: "matches" });
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
          <button className={route.page !== "settings" ? "on" : ""} onClick={() => setRoute({ page: "matches" })}>Матчи</button>
          <button className={route.page === "settings" ? "on" : ""} onClick={() => setRoute({ page: "settings" })}>Настройки</button>
        </nav>
        <div className="side-actions">
          <button onClick={importFiles}>Импорт…</button>
          <button onClick={scan}>Сканировать</button>
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
          </div>
        )}
        <div className="version">{version}</div>
      </aside>
      <main className="content">
        {route.page === "matches" && (
          <MatchesPage rows={rows} hasSteamID={!!settings?.steam_id} onOpen={(id) => setRoute({ page: "match", id })} />
        )}
        {route.page === "match" && <MatchPage key={route.id} matchID={route.id} onBack={() => setRoute({ page: "matches" })} />}
        {route.page === "settings" && (
          <SettingsPage toast={showToast} onSaved={(s) => { setSettings(s); reload(); }} />
        )}
      </main>
      {dragging && <div className="drop-overlay">Отпусти, чтобы импортировать реплеи</div>}
      {toast && <div className={`toast ${toast.bad ? "bad" : ""}`}>{toast.msg}</div>}
    </div>
  );
}
