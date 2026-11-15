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
  const names = useNames();

  useEffect(() => {
    Api.GetSettings().then(setS);
    Api.Info().then(setInfo);
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
            <input value={s.map_image} onChange={(e) => set("map_image", e.target.value)} placeholder="схематичная карта" />
            <button onClick={pickMap}>Выбрать…</button>
            <button onClick={() => set("map_image", "")}>Сбросить</button>
          </div>
          <small>Квадратная картинка миникарты (png/jpg), должна покрывать весь мир карты.</small>
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

        <div className="row">
          <button className="primary" onClick={save} disabled={saving}>{saving ? "Сохраняю…" : "Сохранить"}</button>
        </div>
      </div>

      <div className="card form">
        <h2>Данные</h2>
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
