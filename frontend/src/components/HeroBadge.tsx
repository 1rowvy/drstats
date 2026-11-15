import { useState } from "react";
import { useNames } from "../names";

// Muted pastel tiles; the colour is stable per hero (hash of the npc name).
const PALETTE = ["#e8a54b", "#6cc6a8", "#8f9be8", "#a9c6ea", "#e89a8f", "#c39be6", "#d8c46a", "#86cf8a", "#e6a0c4", "#7cc4d8"];

function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}

export function initials(name: string): string {
  const words = name.split(/[\s-]+/).filter(Boolean);
  const s = words.length >= 2 ? words[0][0] + words[1][0] : (words[0] ?? "").slice(0, 2);
  return s.toUpperCase();
}

/** Heroes whose portrait failed to load this session (offline, unknown hero). */
const broken = new Set<string>();

/**
 * Hero icon: Valve portrait (cached on disk by the backend, see internal/heroimg),
 * cropped to a square; initials on a pastel tile while unavailable.
 */
export function HeroBadge({ npc, size = 28 }: { npc: string; size?: number }) {
  const names = useNames();
  const name = names.hero(npc);
  const short = npc.replace(/^npc_dota_hero_/, "");
  const [failed, setFailed] = useState(() => !short || broken.has(short));
  const radius = Math.max(5, size * 0.2);
  if (!failed) {
    return (
      <img
        className="hero-badge hero-img"
        src={`/heroimg/${short}.png`}
        alt={name}
        title={name}
        loading="lazy"
        style={{ width: size, height: size, borderRadius: radius }}
        onError={() => { broken.add(short); setFailed(true); }}
      />
    );
  }
  return (
    <span
      className="hero-badge"
      title={name}
      style={{
        width: size, height: size, fontSize: Math.max(9, size * 0.34),
        borderRadius: radius,
        background: npc ? PALETTE[hash(npc) % PALETTE.length] : "#4a5266",
      }}
    >
      {initials(name) || "?"}
    </span>
  );
}

export function HeroName({ npc, badge = true, sub }: { npc: string; badge?: boolean; sub?: string }) {
  const names = useNames();
  return (
    <span className="hero-name">
      {badge && <HeroBadge npc={npc} size={sub ? 30 : 24} />}
      {sub ? (
        <span className="hero-name-text"><b>{names.hero(npc)}</b><small>{sub}</small></span>
      ) : <span>{names.hero(npc)}</span>}
    </span>
  );
}
