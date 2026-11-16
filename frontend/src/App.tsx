import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { WindowControls } from "./components/WindowControls";
import { Api, AppEvents, DownloadProgress, MatchRow, ParseProgress, Progress, ReplayEvent, Settings } from "./api";
import { Icon } from "./components/Icon";
import { ToastCtx } from "./components/ui";
import { plural } from "./format";
import { NamesProvider } from "./names";
import { MatchesPage } from "./pages/Matches";
import { MatchPage } from "./pages/MatchPage";
import { SettingsPage } from "./pages/Settings";
import { DashboardPage } from "./pages/Dashboard";
import { HeroPage } from "./pages/HeroPage";
import { ImportPage } from "./pages/Import";
import { RankPage } from "./pages/Rank";

type Route =
  | { page: "dashboard" }
  | { page: "matches" }
  | { page: "match"; id: number }
  | { page: "hero"; id: number }
  | { page: "rank" }
  | { page: "import" }
  | { page: "settings" };

export default function App() {
  return (
    <NamesProvider>
      <Shell />
    </NamesProvider>
  );
}

function Shell() {
  const [history, setHistory] = useState<Route[]>([{ page: "matches" }]);
  const route = history[history.length - 1];
  const go = useCallback((r: Route) => setHistory((h) => [...h, r].slice(-30)), []);
  const back = useCallback(() => setHistory((h) => (h.length > 1 ? h.slice(0, -1) : [{ page: "matches" }])), []);
  const top = (r: Route) => setHistory([r]);
  const [tick, setTick] = useState(0);
  const [dl, setDl] = useState<DownloadProgress | null>(null);
  const [rows, setRows] = useState<MatchRow[]>([]);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [parsing, setParsing] = useState<Record<number, number>>({});
  const [settings, setSettings] = useState<Settings | null>(null);
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
    const offs = [
      Events.On(AppEvents.LibraryReplay, (e) => {
        const ev: ReplayEvent = e.data;
        if (ev.status !== "parsing") {
          setParsing((p) => {
            if (!(ev.replay_id in p)) return p;
            const { [ev.replay_id]: _, ...rest } = p;
            return rest;
          });
        }
        reloadSoon();
      }),
      Events.On(AppEvents.LibraryParsing, (e) => {
        const ev: ParseProgress = e.data;
        setParsing((p) => ({ ...p, [ev.replay_id]: ev.fraction }));
      }),
      Events.On(AppEvents.LibraryProgress, (e) => setProgress(e.data)),
      Events.On(AppEvents.OpendotaUpdated, reloadSoon),
      Events.On(AppEvents.OpendotaHistory, reloadSoon),
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
  const download = async (input: string) => {
    try {
      const id = await Api.DownloadMatch(input);
      setDl({ match_id: id, stage: "lookup", bytes: 0, total: 0, path: "", error: "" });
      return true;
    } catch (err: any) {
      showToast(String(err?.message ?? err), true);
      return false;
    }
  };
  const saveSettings = async (s: Settings) => {
    try {
      const saved = await Api.SaveSettings(s);
      setSettings(saved);
      reload();
      return true;
    } catch (e: any) {
      showToast(String(e?.message ?? e), true);
      return false;
    }
  };
  const dlBusy = !!dl && dl.stage !== "done" && dl.stage !== "failed";

  const nav: { id: string; label: string; on: boolean; to: Route }[] = [
    { id: "matches", label: "Матчи", on: route.page === "matches" || route.page === "match", to: { page: "matches" } },
    { id: "dashboard", label: "Дашборд", on: route.page === "dashboard", to: { page: "dashboard" } },
    { id: "rank", label: "Ранг", on: route.page === "rank", to: { page: "rank" } },
    { id: "heroes", label: "Герои", on: route.page === "hero", to: { page: "hero", id: 0 } },
    { id: "import", label: "Импорт", on: route.page === "import", to: { page: "import" } },
  ];

  return (
    <ToastCtx.Provider value={showToast}>
      <div
        className="shell"
        data-file-drop-target
        onDragEnter={() => setDragging(true)}
        onDragLeave={(e) => { if (e.currentTarget === e.target) setDragging(false); }}
        onDrop={() => setDragging(false)}
      >
        <header className="topbar">
          <div className="logo"><Icon name="pulse" size={18} /> drstats</div>
          <nav className="topnav">
            {nav.map((n) => (
              <button key={n.id} className={n.on ? "on" : ""} onClick={() => top(n.to)}>{n.label}</button>
            ))}
          </nav>
          <div className="topbar-right">
            <QueuePill progress={progress} parsing={parsing} dl={dlBusy ? dl : null} onClick={() => top({ page: "import" })} />
            <button className={`icon-btn ${route.page === "settings" ? "on" : ""}`} title="Настройки" onClick={() => top({ page: "settings" })}>
              <Icon name="settings" size={17} />
            </button>
          </div>
          <WindowControls />
        </header>
        <main className="content">
          <div className="page">
            {route.page === "dashboard" && (
              <DashboardPage tick={tick} onMatch={(id) => go({ page: "match", id })} onHero={(id) => go({ page: "hero", id })} />
            )}
            {route.page === "rank" && <RankPage tick={tick} onMatch={(id) => go({ page: "match", id })} />}
            {route.page === "matches" && (
              <MatchesPage rows={rows} settings={settings} parsing={parsing} onOpen={(id) => go({ page: "match", id })}
                onImport={importFiles} onScan={scan} onDownload={download} downloading={dlBusy}
                onShowImport={() => top({ page: "import" })} />
            )}
            {route.page === "match" && (
              <MatchPage key={route.id} matchID={route.id} lh10Goal={settings?.lh10_goal || 70}
                onBack={back} onHero={(id) => go({ page: "hero", id })} />
            )}
            {route.page === "hero" && (
              <HeroPage key={route.id} heroID={route.id} tick={tick} onHero={(id) => setHistory((h) => [...h.slice(0, -1), { page: "hero", id }])}
                onMatch={(id) => go({ page: "match", id })} />
            )}
            {route.page === "import" && (
              <ImportPage rows={rows} parsing={parsing} progress={progress} settings={settings} onSave={saveSettings}
                onImport={importFiles} onDownload={download} downloading={dlBusy} dl={dl} dragging={dragging}
                onOpen={(id) => go({ page: "match", id })} />
            )}
            {route.page === "settings" && (
              <SettingsPage toast={showToast} onSaved={(s) => { setSettings(s); reload(); }} />
            )}
          </div>
        </main>
        {dragging && route.page !== "import" && <div className="drop-overlay">Отпусти, чтобы импортировать реплеи</div>}
        {toast && <div className={`toast ${toast.bad ? "bad" : ""}`}>{toast.msg}</div>}
      </div>
    </ToastCtx.Provider>
  );
}

/** Top-right status: download, parse queue or "all parsed". */
function QueuePill({ progress, parsing, dl, onClick }: {
  progress: Progress | null; parsing: Record<number, number>; dl: DownloadProgress | null; onClick: () => void;
}) {
  if (!progress) return null;
  const busy = progress.pending + progress.parsing;
  const running = Object.values(parsing);
  const pct = running.length ? Math.round((running.reduce((a, b) => a + b, 0) / running.length) * 100) : null;
  let dot = "var(--win)";
  let text = "Всё разобрано";
  if (dl) {
    dot = "var(--blue)";
    text = dl.stage === "lookup" ? `Ищу матч ${dl.match_id}…` : `Скачивание · ${Math.round(dl.bytes / 1048576)}${dl.total ? ` из ${Math.round(dl.total / 1048576)}` : ""} МБ`;
  } else if (busy > 0) {
    dot = "var(--accent)";
    text = `Парсинг · ${busy} ${plural(busy, "реплей", "реплея", "реплеев")}${pct != null ? ` · ${pct}%` : ""}`;
  } else if (progress.failed > 0) {
    dot = "var(--loss)";
    text = `Ошибок: ${progress.failed}`;
  } else if (progress.stale > 0) {
    dot = "var(--warn)";
    text = `Устарели: ${progress.stale}`;
  }
  return (
    <button className="status-pill" onClick={onClick} title="Импорт и очередь">
      <span className="dot" style={{ background: dot, boxShadow: `0 0 8px ${dot}` }} /> {text}
    </button>
  );
}
