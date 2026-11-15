import { useEffect, useRef, useState } from "react";
import type { MatchView, PlayerView } from "../../api";
import type { useNames } from "../../names";

export function useFocus(view: MatchView) {
  const players = view.players ?? [];
  const me = players.find((p) => p.player_id === view.focus)!;
  const opp = players.find((p) => p.player_id === view.opponent);
  return { players, me, opp };
}

/** Width of an element, following resizes. */
export function useWidth<T extends HTMLElement>(): [React.RefObject<T>, number] {
  const ref = useRef<T>(null);
  const [w, setW] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setW(el.clientWidth);
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, w];
}

/** Replay tick at game time t (nearest series sample). */
export function tickAt(view: MatchView, t: number): number | undefined {
  const ts = view.farm.t ?? [];
  const ticks = view.farm.ticks ?? [];
  if (!ts.length || !ticks.length) return undefined;
  let best = 0;
  for (let i = 1; i < ts.length; i++) if (Math.abs(ts[i] - t) < Math.abs(ts[best] - t)) best = i;
  return ticks[best];
}

const CONSUMABLE = /^(tango|flask|clarity|ward_|tpscroll|smoke_of_deceit|dust|enchanted_mango|faerie_fire|blood_grenade|branches|tome_of_knowledge|famango|great_famango|greater_famango|cheese|aegis|refresher_shard)/;

/** First purchases of "key" items (≥1800 gold, not consumables). */
export function keyItems(view: MatchView, names: ReturnType<typeof useNames>, limit = 6) {
  const cost = (k: string) => names.raw.items?.[k]?.cost ?? 0;
  return (view.items ?? []).filter((it) => it.first && !CONSUMABLE.test(it.item) && cost(it.item) >= 1800).slice(0, limit);
}

/** Lane short label from an objective npc name: npc_dota_badguys_tower1_mid -> "T1 мид". */
export function objectiveLabel(key: string, kind: string): string {
  if (kind === "roshan") return "Рошан";
  if (kind === "barracks") return "Бараки";
  if (kind === "fort") return "Трон";
  const m = key.match(/tower(\d)(?:_(top|mid|bot))?/);
  if (!m) return "Башня";
  const lane: Record<string, string> = { top: "топ", mid: "мид", bot: "бот" };
  return m[2] ? `T${m[1]} ${lane[m[2]]}` : `T${m[1]}`;
}

export function heroOf(players: PlayerView[], pid: number) {
  return players.find((p) => p.player_id === pid);
}

/** Last non-null value of a series. */
export function lastOf(l: (number | null)[] | null | undefined): number | null {
  for (let i = (l ?? []).length - 1; i >= 0; i--) if (l![i] != null) return l![i]!;
  return null;
}

/** Value of a series at game time t (nearest sample). */
export function valueAt(view: MatchView, l: (number | null)[] | null | undefined, t: number): number | null {
  const ts = view.farm.t ?? [];
  if (!l || !ts.length) return null;
  let best = 0;
  for (let i = 1; i < ts.length; i++) if (Math.abs(ts[i] - t) < Math.abs(ts[best] - t)) best = i;
  return l[best] ?? null;
}
