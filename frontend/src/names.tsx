import { createContext, ReactNode, useContext, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Api, AppEvents, Names } from "./api";
import { prettifyUnit } from "./format";

const empty: Names = { heroes: {}, hero_by_id: {}, items: {}, abilities: {}, fetched_at: 0 };
const NamesCtx = createContext<Names>(empty);

export function NamesProvider({ children }: { children: ReactNode }) {
  const [names, setNames] = useState<Names>(empty);
  useEffect(() => {
    const load = () => Api.Names().then((n) => n && setNames(n)).catch(console.error);
    load();
    return Events.On(AppEvents.ConstantsUpdated, load);
  }, []);
  return <NamesCtx.Provider value={names}>{children}</NamesCtx.Provider>;
}

export function useNames() {
  const n = useContext(NamesCtx);
  return {
    raw: n,
    /** Localized hero name by npc name (npc_dota_hero_*). */
    hero(npc: string): string {
      return n.heroes?.[npc]?.name || prettifyUnit(npc);
    },
    heroAttr(npc: string): string {
      return n.heroes?.[npc]?.attr || "";
    },
    /** Ability or item display name; "" input is an autoattack. */
    inflictor(key: string): string {
      if (!key) return "Autoattack";
      if (key.startsWith("item_")) return n.items?.[key.slice(5)]?.name || prettifyUnit(key.slice(5));
      return n.abilities?.[key] || prettifyUnit(key);
    },
    /** Any unit: hero localized name or a prettified unit key. */
    unit(key: string): string {
      if (key.startsWith("npc_dota_hero_")) return n.heroes?.[key]?.name || prettifyUnit(key);
      return prettifyUnit(key);
    },
  };
}
