import { useMemo, useState } from "react";
import { Api, MatchRow } from "../api";
import { baseName, date, duration, gameMode } from "../format";
import { HeroName } from "../components/HeroBadge";

type Filter = "all" | "mine" | "failed";

export function MatchesPage({ rows, hasSteamID, onOpen }: {
  rows: MatchRow[]; hasSteamID: boolean; onOpen: (matchID: number) => void;
}) {
  const [filter, setFilter] = useState<Filter>("all");
  const shown = useMemo(() => rows.filter((r) => {
    if (filter === "mine") return r.found;
    if (filter === "failed") return r.status === "failed";
    return true;
  }), [rows, filter]);
  const failed = rows.filter((r) => r.status === "failed").length;

  return (
    <div>
      <div className="page-head">
        <h1>Матчи</h1>
        <div className="seg">
          <button className={filter === "all" ? "on" : ""} onClick={() => setFilter("all")}>Все ({rows.length})</button>
          <button className={filter === "mine" ? "on" : ""} onClick={() => setFilter("mine")}>Мои</button>
          <button className={filter === "failed" ? "on" : ""} onClick={() => setFilter("failed")}>Ошибки ({failed})</button>
        </div>
      </div>
      {!hasSteamID && (
        <div className="notice">Укажи свой Steam ID в настройках — иначе не понять, за кого ты играл.</div>
      )}
      {rows.length === 0 ? (
        <div className="empty">
          Реплеев пока нет. Перетащи <code>.dem</code> файлы в окно, нажми «Импорт» или проверь папку реплеев в настройках.
        </div>
      ) : (
        <table className="table matches">
          <thead>
            <tr>
              <th>Герой</th><th>Результат</th><th>K / D / A</th><th>Длительность</th><th>Дата</th><th>Режим</th><th>Статус</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((r) => <Row key={r.replay_id} r={r} onOpen={onOpen} />)}
          </tbody>
        </table>
      )}
    </div>
  );
}

function Row({ r, onOpen }: { r: MatchRow; onOpen: (id: number) => void }) {
  const done = r.status === "done" && r.match_id !== 0;
  const won = r.found && r.winner !== 0 ? r.winner === r.team : null;
  return (
    <tr className={done ? "clickable" : ""} onClick={() => done && onOpen(r.match_id)}>
      <td>
        {r.found ? <HeroName npc={r.hero_name} /> : done ? <span className="dim">не мой матч</span> : <span className="dim" title={r.path}>{baseName(r.path)}</span>}
      </td>
      <td>{won === null ? <span className="dim">—</span> : won ? <span className="win">Победа</span> : <span className="loss">Поражение</span>}</td>
      <td>{r.found ? <span className="kda">{r.kills} / <span className="loss">{r.deaths}</span> / {r.assists}</span> : "—"}</td>
      <td>{done ? duration(r.duration) : "—"}</td>
      <td>{date(done ? r.end_time : r.mtime)}</td>
      <td>{done ? gameMode(r.game_mode, r.lobby_type) : "—"}</td>
      <td><Status r={r} /></td>
    </tr>
  );
}

function Status({ r }: { r: MatchRow }) {
  const reparse = (e: React.MouseEvent) => {
    e.stopPropagation();
    Api.Reparse(r.replay_id).catch(console.error);
  };
  switch (r.status) {
    case "pending":
      return <span className="badge">в очереди</span>;
    case "parsing":
      return <span className="badge busy">парсинг…</span>;
    case "failed":
      return (
        <span className="status-failed">
          <span className="badge bad" title={r.error}>ошибка</span>
          <span className="err-text" title={r.error}>{r.error}</span>
          <button className="link" onClick={reparse}>повторить</button>
        </span>
      );
    default:
      return r.stale ? (
        <span>
          <span className="badge warn" title="Разобран старой версией парсера">устарел</span>{" "}
          <button className="link" onClick={reparse}>перепарсить</button>
        </span>
      ) : <span className="badge ok">готов</span>;
  }
}
