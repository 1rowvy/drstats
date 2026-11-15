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

export function pct(v: number | null | undefined, digits = 0): string {
  return v == null ? "—" : `${(v * 100).toFixed(digits)}%`;
}

/** Signed number: +12 / −3. */
export function signed(v: number, digits = 0): string {
  const s = Math.abs(v).toFixed(digits);
  return v > 0 ? `+${s}` : v < 0 ? `−${s}` : s;
}

/** Signed game-time difference: +1:23 (later) / −0:45 (earlier). */
export function signedClock(sec: number): string {
  const s = clock(Math.abs(sec));
  return sec > 0 ? `+${s}` : sec < 0 ? `−${s}` : s;
}

export const LANES: Record<string, string> = {
  top: "Верх", mid: "Мид", bot: "Низ", jungle: "Лес", roam: "Роум", "": "—",
};

/** Lane name with safe/off-lane meaning for the team (Radiant safe = bot, Dire safe = top). */
export function laneName(lane: string, team?: number): string {
  const base = LANES[lane] ?? lane;
  if (!team || (lane !== "top" && lane !== "bot")) return base;
  const safe = (team === RADIANT && lane === "bot") || (team === DIRE && lane === "top");
  return `${base} (${safe ? "лёгкая" : "сложная"})`;
}

export const GOLD_SOURCES: Record<string, string> = {
  creep: "Крипы на линии", neutral: "Нейтралы", hero: "Герои", building: "Строения",
  passive: "Пассивное", bounty: "Баунти-руны", roshan: "Рошан", other: "Прочее",
};

export const RUNES: Record<number, string> = {
  0: "Двойной урон", 1: "Ускорение", 2: "Иллюзии", 3: "Невидимость", 4: "Регенерация",
  5: "Баунти", 6: "Чародейство", 7: "Вода", 8: "Мудрость", 9: "Щит",
};

export const UNIT_CATEGORIES: Record<string, string> = {
  creep: "Крипы", neutral: "Нейтралы", building: "Строения", roshan: "Рошан", illusion: "Иллюзии",
  ward: "Варды", unit: "Прочие юниты",
};

export const SOURCE_KINDS: Record<string, string> = {
  autoattack: "Автоатака", ability: "Способности", item: "Предметы",
};

/** Aggregate metrics (stats.metrics keys). */
export const METRICS: Record<string, { label: string; fmt: (v: number) => string }> = {
  kda: { label: "KDA", fmt: (v) => v.toFixed(2) },
  kills: { label: "Убийства", fmt: (v) => v.toFixed(1) },
  deaths: { label: "Смерти", fmt: (v) => v.toFixed(1) },
  assists: { label: "Помощь", fmt: (v) => v.toFixed(1) },
  gpm: { label: "GPM", fmt: (v) => Math.round(v).toString() },
  xpm: { label: "XPM", fmt: (v) => Math.round(v).toString() },
  lh10: { label: "LH @10", fmt: (v) => v.toFixed(1) },
  dn10: { label: "DN @10", fmt: (v) => v.toFixed(1) },
  nw10: { label: "NW @10", fmt: (v) => Math.round(v).toLocaleString("ru-RU") },
  deaths10: { label: "Смерти до 10:00", fmt: (v) => v.toFixed(2) },
  kp: { label: "Участие в убийствах", fmt: (v) => pct(v) },
  hero_dmg_pm: { label: "Урон героям / мин", fmt: (v) => Math.round(v).toString() },
  tower_dmg: { label: "Урон строениям", fmt: (v) => Math.round(v).toLocaleString("ru-RU") },
  gold_lost: { label: "Потеряно золота", fmt: (v) => Math.round(v).toLocaleString("ru-RU") },
  wards: { label: "Вардов поставлено", fmt: (v) => v.toFixed(1) },
  stacks: { label: "Стаков", fmt: (v) => v.toFixed(1) },
};

export const WEEKDAYS = ["", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"];

const MONTHS = ["янв.", "февр.", "мар.", "апр.", "мая", "июня", "июля", "авг.", "сент.", "окт.", "нояб.", "дек."];

/** Human date for lists: "Сегодня, 22:03", "Вчера, 23:41", older: "13 нояб." (+ year if not current). */
export function relDate(unix: number, now = new Date()): string {
  if (!unix) return "—";
  const d = new Date(unix * 1000);
  const hm = d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86400000);
  if (diff === 0) return `Сегодня, ${hm}`;
  if (diff === 1) return `Вчера, ${hm}`;
  const base = `${d.getDate()} ${MONTHS[d.getMonth()]}`;
  return d.getFullYear() === now.getFullYear() ? base : `${base} ${d.getFullYear()}`;
}

/** Short mode label for lists: Turbo / Ranked / All Pick / … */
export function modeShort(mode: number, lobby: number): string {
  if (mode === 23) return "Turbo";
  if (lobby === 7) return "Ranked";
  return GAME_MODES[mode] ?? `Mode ${mode}`;
}

/** Guessed role from the laning-stage lane and early farm (LH@10). */
export function roleName(lane: string, team: number, lh10?: number | null): string {
  if (lane === "mid") return "Мид";
  if (lane === "jungle") return "Лес";
  if (lane === "roam") return "Роумер";
  if (lane !== "top" && lane !== "bot") return "";
  const safe = (team === RADIANT && lane === "bot") || (team === DIRE && lane === "top");
  const farmed = lh10 == null || lh10 >= (safe ? 25 : 18);
  if (!farmed) return "Саппорт";
  return safe ? "Керри" : "Оффлейн";
}

const SIDE: Record<string, string> = { radiant: "Radiant", dire: "Dire" };
const LANE_LABEL: Record<string, string> = { top: "Топ", mid: "Мид", bot: "Бот" };

/** Map zone key from the backend (stats.Region) as a label: "Река, мид", "Лес Dire", "Топ, сторона Dire". */
export function regionName(key: string | undefined): string {
  if (!key) return "";
  const [zone, a, b] = key.split(":");
  switch (zone) {
    case "base": return `База ${SIDE[a] ?? a}`;
    case "river": return `Река, ${(LANE_LABEL[a] ?? a).toLowerCase()}`;
    case "lane": return `${LANE_LABEL[a] ?? a}, сторона ${SIDE[b] ?? b}`;
    case "jungle": return `Лес ${SIDE[a] ?? a}`;
  }
  return key;
}

/** Space-grouped integer: 9 420, with a sign when asked: +1 140 / −620. */
export function num(v: number, sign = false): string {
  const s = Math.round(Math.abs(v)).toLocaleString("ru-RU").replace(/ /g, " ");
  if (!sign) return v < 0 ? `−${s}` : s;
  return v > 0 ? `+${s}` : v < 0 ? `−${s}` : s;
}

/** Plural form picker for Russian: plural(5, "матч", "матча", "матчей"). */
export function plural(n: number, one: string, few: string, many: string): string {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return few;
  return many;
}
