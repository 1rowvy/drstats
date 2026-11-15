export const RADIANT = 2;
export const DIRE = 3;

/** Game time (seconds, may be negative) as m:ss. */
export function clock(sec: number): string {
  const neg = sec < 0;
  const s = Math.floor(Math.abs(sec));
  const m = Math.floor(s / 60);
  const r = String(s % 60).padStart(2, "0");
  return `${neg ? "-" : ""}${m}:${r}`;
}

/** Duration as "34:12" or "1:02:03". */
export function duration(sec: number): string {
  const s = Math.round(sec);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = String(s % 60).padStart(2, "0");
  return h ? `${h}:${String(m).padStart(2, "0")}:${r}` : `${m}:${r}`;
}

export function date(unix: number): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString("ru-RU", {
    day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

export function thousands(n: number): string {
  return n >= 10000 ? `${(n / 1000).toFixed(1)}k` : n.toLocaleString("ru-RU");
}

const GAME_MODES: Record<number, string> = {
  1: "All Pick", 2: "Captains Mode", 3: "Random Draft", 4: "Single Draft", 5: "All Random",
  12: "Least Played", 16: "Captains Draft", 18: "Ability Draft", 20: "All Random Deathmatch",
  21: "1v1 Mid", 22: "All Pick", 23: "Turbo", 24: "Mutation",
};
const LOBBY_TYPES: Record<number, string> = {
  0: "Normal", 1: "Practice", 2: "Tournament", 4: "Bots", 5: "Team", 6: "Solo", 7: "Ranked", 8: "1v1 Mid", 9: "Battle Cup",
};

export function gameMode(mode: number, lobby: number): string {
  const m = GAME_MODES[mode] ?? `Mode ${mode}`;
  const l = LOBBY_TYPES[lobby];
  return l && l !== "Normal" ? `${m} · ${l}` : m;
}

export function teamName(team: number): string {
  return team === RADIANT ? "Radiant" : team === DIRE ? "Dire" : "—";
}

/** Turns an npc/unit key into something readable: npc_dota_goodguys_tower1_mid -> "Radiant tower1 mid". */
export function prettifyUnit(key: string): string {
  if (!key) return "—";
  let s = key.replace(/^npc_dota_/, "").replace(/^hero_/, "");
  s = s.replace(/goodguys/g, "Radiant").replace(/badguys/g, "Dire").replace(/_/g, " ");
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export function baseName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}
