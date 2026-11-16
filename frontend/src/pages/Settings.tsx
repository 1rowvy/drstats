import { useEffect, useState } from "react";
import { Api, AppInfo, Settings } from "../api";
import { useNames } from "../names";
import { date } from "../format";
import { resetMapImage } from "../components/MapCanvas";

export function SettingsPage({ onSaved, toast }: { onSaved: (s: Settings) => void; toast: (msg: string, bad?: boolean) => void }) {
  const [s, setS] = useState<Settings | null>(null);
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [autostart, setAutostart] = useState<boolean | null>(null);
  const names = useNames();

  useEffect(() => {
    Api.GetSettings().then(setS);
    Api.Info().then(setInfo);
    Api.Autostart().then(setAutostart).catch(() => setAutostart(false));
  }, []);
  if (!s) return <div className="dim">Загрузка…</div>;

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setS({ ...s, [k]: v });
  const save = async () => {
    setSaving(true);
    try {
      const saved = await Api.SaveSettings(s);
      setS(saved);
      resetMapImage();
      onSaved(saved);
      toast("Настройки сохранены");
    } catch (e: any) {
      toast(String(e?.message ?? e), true);
    } finally {
      setSaving(false);
    }
  };
  const pickDir = async () => {
    const d = await Api.PickDirectory("Папка с реплеями");
    if (d) set("replay_dir", d);
  };
  const pickMap = async () => {
    const f = await Api.PickMapImage();
    if (f) set("map_image", f);
  };
  const reparse = async (onlyStale: boolean) => {
    try {
      const n = await Api.ReparseAll(onlyStale);
      toast(n ? `В очередь на перепарсинг: ${n}` : "Нечего перепарсивать");
    } catch (e: any) {
      toast(String(e?.message ?? e), true);
    }
  };

  const fetchOD = async () => {
    try {
      const n = await Api.FetchOpenDota();
      toast(n ? `В очередь OpenDota: ${n}` : "Все матчи уже подтянуты");
    } catch (e: any) {
      toast(String(e?.message ?? e), true);
    }
  };

  const refresh = async () => {
    setRefreshing(true);
    try {
      await Api.RefreshConstants();
      toast("Константы обновлены");
    } catch (e: any) {
      toast(String(e?.message ?? e), true);
    } finally {
      setRefreshing(false);
    }
  };
  // Applied right away: it is an OS registration, not part of the saved settings.
  const toggleAutostart = async (on: boolean) => {
    try {
      await Api.SetAutostart(on);
      setAutostart(on);
      toast(on ? "drstats будет запускаться вместе с системой" : "Автозапуск выключен");
    } catch (e: any) {
      toast(String(e?.message ?? e), true);
    }
  };
  // Applied right away like autostart; other unsaved edits on the page stay as they are.
  const toggleTray = async (on: boolean) => {
    try {
      const cur = await Api.GetSettings();
      const saved = await Api.SaveSettings({ ...cur, tray_off: !on });
      setS({ ...s, tray_off: saved.tray_off });
      onSaved(saved);
      toast(on ? "Закрытие окна сворачивает в трей" : "Закрытие окна завершает drstats");
    } catch (e: any) {
      toast(String(e?.message ?? e), true);
    }
  };
  const update = () => Api.CheckForUpdates().catch((e) => toast(String(e?.message ?? e), true));

  return (
    <div className="settings">
      <h1>Настройки</h1>

      <div className="card form">
        <label>
          <span>Steam ID</span>
          <input value={s.steam_id} onChange={(e) => set("steam_id", e.target.value)} placeholder="account id, steam64 или ссылка на профиль" />
          <small>Нужен, чтобы находить тебя в матче. Например 914467708 или 76561198874733436.</small>
        </label>

        <label>
          <span>Папка реплеев</span>
          <div className="row">
            <input value={s.replay_dir} onChange={(e) => set("replay_dir", e.target.value)} />
            <button onClick={pickDir}>Выбрать…</button>
            <button onClick={async () => set("replay_dir", await Api.DefaultReplayDir())}>По умолчанию</button>
          </div>
          <small>Сканируется при запуске и по кнопке «Сканировать».</small>
        </label>

        <label>
          <span>Параллельный парсинг</span>
          <input type="number" min={1} max={16} value={s.parallelism} onChange={(e) => set("parallelism", Number(e.target.value))} />
          <small>Сколько реплеев разбирать одновременно (каждый ~300–500 МБ памяти).</small>
        </label>

        <label>
          <span>Фон карты</span>
          <div className="row">
            <input value={s.map_image} onChange={(e) => set("map_image", e.target.value)} placeholder="игровая миникарта (7.41)" />
            <button onClick={pickMap}>Выбрать…</button>
            <button onClick={() => set("map_image", "")}>Сбросить</button>
          </div>
          <small>Необязательно: своя квадратная картинка (png/jpg), покрывающая весь мир карты. Пусто — встроенная миникарта.</small>
        </label>

        <h2>Эвристики</h2>
        <label>
          <span>Окно попадания способности, с</span>
          <input type="number" step={0.1} min={0.1} value={s.hit_window_sec} onChange={(e) => set("hit_window_sec", Number(e.target.value))} />
          <small>Каст считается попаданием, если в течение N секунд был урон/дебафф (используется в фазе 3).</small>
        </label>
        <label>
          <span>Порог простоя, с</span>
          <input type="number" min={1} value={s.idle_threshold} onChange={(e) => set("idle_threshold", Number(e.target.value))} />
          <small>Сколько секунд без фарма и драк считается простоем.</small>
        </label>
        <label>
          <span>Цель по ЛХ на 10 минуте</span>
          <input type="number" min={1} value={s.lh10_goal} onChange={(e) => set("lh10_goal", Number(e.target.value))} />
          <small>Показывается в списке матчей и на вкладке «Обзор».</small>
        </label>

        <h2>OpenDota</h2>
        <label className="row" style={{ flexDirection: "row", alignItems: "center" }}>
          <input type="checkbox" checked={!s.opendota_off} onChange={(e) => set("opendota_off", !e.target.checked)} />
          <span>Подтягивать данные матча с OpenDota</span>
        </label>
        <small style={{ display: "block", marginTop: -8, marginBottom: 16 }}>
          Позиция 1–5, ранги, пати и перцентили по герою. Реплей остаётся основным источником; без сети всё считается по нему.
        </small>

        <div className="row">
          <button className="primary" onClick={save} disabled={saving}>{saving ? "Сохраняю…" : "Сохранить"}</button>
        </div>
      </div>

      <div className="card form">
        <h2>Система</h2>
        <label className="row" style={{ flexDirection: "row", alignItems: "center" }}>
          <input type="checkbox" checked={!!autostart} disabled={autostart === null} onChange={(e) => toggleAutostart(e.target.checked)} />
          <span>Запускать вместе с системой</span>
        </label>
        <small style={{ display: "block", marginTop: -8, marginBottom: 16 }}>
          Стартует в трее: следит за папкой реплеев, подтягивает OpenDota и докачивает свежие игры в фоне.
        </small>
        <label className="row" style={{ flexDirection: "row", alignItems: "center" }}>
          <input type="checkbox" checked={!s.tray_off} onChange={(e) => toggleTray(e.target.checked)} />
          <span>Сворачивать в трей при закрытии</span>
        </label>
        <small style={{ display: "block", marginTop: -8 }}>
          Крестик прячет окно в трей, приложение продолжает работать. Открыть — клик по иконке в трее, выйти — «Выход» в её меню.
        </small>
      </div>

      <div className="card form">
        <h2>Данные</h2>
        <div className="row">
          <button onClick={fetchOD} disabled={!!s.opendota_off}>Подтянуть матчи с OpenDota</button>
        </div>
        <div className="row">
          <button onClick={refresh} disabled={refreshing}>{refreshing ? "Загружаю…" : "Обновить константы OpenDota"}</button>
          <span className="dim">
            {names.raw.fetched_at ? `загружены ${date(names.raw.fetched_at)}` : "ещё не загружены — имена героев берутся из реплея"}
          </span>
        </div>
        <div className="row">
          <button onClick={() => reparse(true)}>Перепарсить устаревшие</button>
          <button onClick={() => { if (confirm("Перепарсить все реплеи заново? Это займёт время.")) reparse(false); }}>Перепарсить всё</button>
          <span className="dim">после обновления парсера или если что-то выглядит неправильно</span>
        </div>
        <div className="row">
          <button onClick={update}>Проверить обновления</button>
          <span className="dim">версия {info?.version}</span>
        </div>
        <div className="dim small">База данных: {info?.data_dir}</div>
      </div>
    </div>
  );
}
