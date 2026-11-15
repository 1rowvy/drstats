import { useNames } from "../names";

const ATTR_COLORS: Record<string, string> = {
  str: "#c0392b", agi: "#27ae60", int: "#2e86de", all: "#a569bd",
};

/** Hero "icon": initials on the primary attribute colour (no network images). */
export function HeroBadge({ npc, size = 28 }: { npc: string; size?: number }) {
  const names = useNames();
  const name = names.hero(npc);
  const initials = name
    .split(/[\s-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0])
    .join("")
    .toUpperCase();
  return (
    <span
      className="hero-badge"
      title={name}
      style={{
        width: size, height: size, fontSize: size * 0.38,
        background: ATTR_COLORS[names.heroAttr(npc)] ?? "#4a5266",
      }}
    >
      {initials || "?"}
    </span>
  );
}

export function HeroName({ npc, badge = true }: { npc: string; badge?: boolean }) {
  const names = useNames();
  return (
    <span className="hero-name">
      {badge && <HeroBadge npc={npc} size={24} />}
      <span>{names.hero(npc)}</span>
    </span>
  );
}
