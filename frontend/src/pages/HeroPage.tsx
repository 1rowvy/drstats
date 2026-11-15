import { useState } from "react";
import type { Filter } from "../api";
import { clock, pct } from "../format";
import { useNames } from "../names";
import { HeroBadge } from "../components/HeroBadge";
import { DeathMap, Filters, HabitsCard, RecentTable, Summary, TrendCard, useDashboard } from "./Dashboard";

const CONSUMABLE = /^(tango|flask|clarity|ward_|tpscroll|smoke_of_deceit|dust|enchanted_mango|faerie_fire|blood_grenade|branches|tome_of_knowledge)/;

export function HeroPage({ heroID, tick, onBack, onMatch }: {
  heroID: number; tick: number; onBack: () => void; onMatch: (id: number) => void;
}) {
  const names = useNames();
  const [filter, setFilter] = useState<Filter>({ hero_id: heroID, days: 0, last: 0, result: "", mode: 0, lane: "" });
  const { d, error } = useDashboard(filter, tick);
  const npc = names.raw.hero_by_id?.[heroID] ?? d?.heroes?.find((h) => h.hero_id === heroID)?.hero_name ?? "";

  if (error) return <div><button className="link" onClick={onBack}>← назад</button><div className="notice bad">{error}</div></div>;
  if (!d) return <div className="dim">Загрузка…</div>;
  const items = (d.hero?.items ?? []).filter((it) => !CONSUMABLE.test(it.item) && (names.raw.items?.[it.item]?.cost ?? 0) >= 1000);
  const abilities = d.hero?.abilities ?? [];
  const heroGames = d.heroes?.find((h) => h.hero_id === heroID)?.games ?? d.games;
  return (
    <div>
      <button className="link" onClick={onBack}>← назад</button>
      <div className="match-head">
        {npc && <HeroBadge npc={npc} size={48} />}
        <h1>{npc ? names.hero(npc) : `Герой #${heroID}`}</h1>
      </div>
      <Filters d={d} filter={filter} onChange={setFilter} hideHero />
      {d.games === 0 ? <div className="empty">Под фильтр не попал ни один матч на этом герое.</div> : (
        <>
          <Summary d={d} />
          <HabitsCard habits={d.habits ?? []} />
          <TrendCard d={d} />
          <div className="grid2">
            <div className="card list-card" style={{ maxHeight: 560 }}>
              <h2>Тайминги предметов <span className="dim small">первая покупка, все матчи на герое</span></h2>
              <table className="table">
                <thead><tr><th>Предмет</th><th>Средний тайминг</th><th>Покупал в матчах</th></tr></thead>
                <tbody>
                  {items.map((it) => (
                    <tr key={it.item}>
                      <td>{names.inflictor("item_" + it.item)}</td>
                      <td>{clock(it.avg)}</td>
                      <td className="dim">{it.n} ({pct(it.n / Math.max(1, heroGames))})</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {items.length === 0 && <div className="dim">Нет данных.</div>}
            </div>
            <div className="card list-card" style={{ maxHeight: 560 }}>
              <h2>Касты <span className="dim small">все матчи на герое</span></h2>
              <table className="table">
                <thead><tr><th>Способность / предмет</th><th>Всего</th><th>За матч</th></tr></thead>
                <tbody>
                  {abilities.map((a) => (
                    <tr key={a.ability}>
                      <td>{names.inflictor(a.ability)}</td>
                      <td>{a.casts}</td>
                      <td className="dim">{(a.casts / Math.max(1, a.matches)).toFixed(1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          <DeathMap d={d} title="Где я умираю на этом герое" />
          <RecentTable d={d} onMatch={onMatch} />
        </>
      )}
    </div>
  );
}
