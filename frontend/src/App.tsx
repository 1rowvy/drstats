import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Api, AppEvents, DownloadProgress, MatchRow, ParseProgress, Progress, ReplayEvent, Settings } from "./api";
import { Icon } from "./components/Icon";
import { plural } from "./format";
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

/** Last two path components: "…/dota/replays". */
function shortPath(p: string): string {
  const parts = p.split(/[\\/]/).filter(Boolean);
  return parts.length > 2 ? `…/${parts.slice(-2).join("/")}` : p;
}

function Shell() {
  const [history, setHistory] = useState<Route[]>([{ page: "matches" }]);
  const route = history[history.length - 1];
  const go = useCallback((r: Route) => setHistory((h) => [...h, r].slice(-30)), []);
  const back = useCallback(() => setHistory((h) => (h.length > 1 ? h.slice(0, -1) : h)), []);
  const top = (r: Route) => setHistory([r]);
  const [tick, setTick] = useState(0);
  const [dl, setDl] = useState<DownloadProgress | null>(null);
  const [rows, setRows] = useState<MatchRow[]>([]);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [parsing, setParsing] = useState<Record<number, number>>({});
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
  const reparseStale = async () => {
    const n = await Api.ReparseAll(true);
    showToast(n ? `Перепарсинг: ${n}` : "Нечего перепарсивать");
  };
  const dlBusy = !!dl && dl.stage !== "done" && dl.stage !== "failed";

  const busy = progress ? progress.pending + progress.parsing : 0;
  const running = Object.values(parsing);
  const runningPct = running.length ? running.reduce((a, b) => a + b, 0) / running.length : 0;

  const nav: { id: string; label: string; icon: string; on: boolean; to: Route }[] = [
    { id: "matches", label: "Матчи", icon: "list", on: route.page === "matches" || route.page === "match", to: { page: "matches" } },
    { id: "heroes", label: "Герои", icon: "user", on: route.page === "hero", to: { page: "hero", id: 0 } },
    { id: "dashboard", label: "Дашборд", icon: "grid", on: route.page === "dashboard", to: { page: "dashboard" } },
    { id: "settings", label: "Настройки", icon: "sliders", on: route.page === "settings", to: { page: "settings" } },
  ];

  return (
    <div
      className="shell"
      data-file-drop-target
      onDragEnter={() => setDragging(true)}
      onDragLeave={(e) => { if (e.currentTarget === e.target) setDragging(false); }}
      onDrop={() => setDragging(false)}
    >
      <aside className="sidebar">
        <div className="logo"><Icon name="pulse" size={20} /> drstats</div>
        <nav>
          {nav.map((n) => (
            <button key={n.id} className={n.on ? "on" : ""} onClick={() => top(n.to)}>
              <Icon name={n.icon} /> {n.label}
            </button>
          ))}
        </nav>
        <div className="side-bottom">
          {dlBusy && dl && (
            <div className="queue-card">
              <div className="q-title">Скачивание</div>
              <div className="q-main">
                {dl.stage === "lookup" ? `Ищу матч ${dl.match_id}…` : `${(dl.bytes / 1048576).toFixed(0)}${dl.total ? ` из ${(dl.total / 1048576).toFixed(0)}` : ""} МБ`}
              </div>
              <div className="bar thin"><div style={{ width: dl.total ? `${(dl.bytes / dl.total) * 100}%` : "15%" }} /></div>
            </div>
          )}
          {progress && (
            <div className="queue-card">
              <div className="q-title">Очередь парсинга</div>
              {busy > 0 ? (
                <>
                  <div className="q-main">
                    {busy} {plural(busy, "реплей", "реплея", "реплеев")}{running.length > 0 && ` · ${Math.round(runningPct * 100)}%`}
                  </div>
                  <div className="bar thin"><div style={{ width: `${(running.length ? runningPct : 0.05) * 100}%` }} /></div>
                </>
              ) : (
                <div className="q-main" style={{ marginBottom: 0 }}>
                  Всё разобрано{progress.failed ? <span className="loss"> · ошибок {progress.failed}</span> : ""}
                </div>
              )}
              {progress.stale > 0 && busy === 0 && (
                <div className="stale">
                  Устарели: {progress.stale}{" "}
                  <button className="link" onClick={reparseStale}>перепарсить</button>
                </div>
              )}
              {settings?.replay_dir && <div className="q-path" title={settings.replay_dir}>{shortPath(settings.replay_dir)}</div>}
            </div>
          )}
          <div className="version">{version}</div>
        </div>
      </aside>
      <main className="content">
        <div className="page">
          {route.page === "dashboard" && (
            <DashboardPage tick={tick} onMatch={(id) => go({ page: "match", id })} onHero={(id) => go({ page: "hero", id })} />
          )}
          {route.page === "matches" && (
            <MatchesPage rows={rows} settings={settings} parsing={parsing} onOpen={(id) => go({ page: "match", id })}
              onImport={importFiles} onScan={scan} onDownload={download} downloading={dlBusy} />
          )}
          {route.page === "match" && (
            <MatchPage key={route.id} matchID={route.id} lh10Goal={settings?.lh10_goal || 70} idleThreshold={settings?.idle_threshold ?? 20}
              onBack={back} onHero={(id) => go({ page: "hero", id })} />
          )}
          {route.page === "hero" && (
            <HeroPage key={route.id} heroID={route.id} tick={tick} onHero={(id) => setHistory((h) => [...h.slice(0, -1), { page: "hero", id }])}
              onMatch={(id) => go({ page: "match", id })} />
          )}
          {route.page === "settings" && (
            <SettingsPage toast={showToast} onSaved={(s) => { setSettings(s); reload(); }} />
          )}
        </div>
      </main>
      {dragging && <div className="drop-overlay">Отпусти, чтобы импортировать реплеи</div>}
      {toast && <div className={`toast ${toast.bad ? "bad" : ""}`}>{toast.msg}</div>}
    </div>
  );
}
