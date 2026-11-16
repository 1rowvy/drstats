import { num, pct } from "./format";

/** Per-game metrics of the plan and the match reasons (stats.planMetrics keys). */
export const GAME_METRICS: Record<string, { label: string; fmt: (v: number) => string; tip: string }> = {
  lh10: { label: "Добивания к 10:00", fmt: (v) => String(Math.round(v)), tip: "Первые 10 минут — крипы важнее убийств: добивай каждую волну, не уходи с линии без причины." },
  deaths: { label: "Смертей за игру", fmt: (v) => v.toFixed(1).replace(/\.0$/, ""), tip: "Перед каждым заходом смотри на миникарту: сколько врагов сейчас не видно." },
  deaths10: { label: "Смертей до 10:00", fmt: (v) => v.toFixed(1).replace(/\.0$/, ""), tip: "На линии держись у своих крипов; пропали враги с карты — отходи." },
  avoid: { label: "Глупых смертей", fmt: (v) => v.toFixed(1).replace(/\.0$/, ""), tip: "Один, без вижна или на чужой половине — не ходи. Фарми там, где видно, или рядом со своими." },
  kp: { label: "Участие в убийствах", fmt: (v) => pct(v), tip: "Зачистил волну — посмотри на карту: где команда, туда и иди." },
  lane_nw: { label: "Нетворс к оппоненту на 10:00", fmt: (v) => num(v, true), tip: "Не уступай волну: добивай и денай, торгуйся хп, когда оппонент тянется за крипом." },
  mid_idle: { label: "Простой в мидгейме", fmt: (v) => pct(v), tip: "Между волнами — лагерь, стак или пуш. Не стой на месте и не бегай без цели." },
  farm_gpm: { label: "Фарм в минуту", fmt: (v) => String(Math.round(v)), tip: "Строй маршрут: волна → лагерь → волна, без пустых переходов через карту." },
  gpm: { label: "GPM", fmt: (v) => String(Math.round(v)), tip: "Меньше золота — позже предметы. Ищи, где терялся фарм: смерти, простой, линия." },
  xpm: { label: "XPM", fmt: (v) => String(Math.round(v)), tip: "Опыт уходит со смертями и простоем: держись рядом с волнами и драками." },
  hero_dmg_pm: { label: "Урон героям в минуту", fmt: (v) => String(Math.round(v)), tip: "Мало урона — мало участия в драках или неудачная позиция в них." },
};

export function gameMetric(key: string) {
  return GAME_METRICS[key] ?? { label: key, fmt: (v: number) => v.toFixed(1), tip: "" };
}
