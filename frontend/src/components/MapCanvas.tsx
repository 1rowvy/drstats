import { useEffect, useRef, useState } from "react";
import { Api } from "../api";

export interface MapPoint {
  x: number;
  y: number;
  color: string;
  label?: string;
  highlight?: boolean;
}

let mapImageCache: Promise<HTMLImageElement | null> | null = null;

/** Loads the user's map background once per session (null if none). */
function loadMapImage(): Promise<HTMLImageElement | null> {
  if (!mapImageCache) {
    mapImageCache = Api.MapImage().then(
      (url) =>
        new Promise((resolve) => {
          if (!url) return resolve(null);
          const img = new Image();
          img.onload = () => resolve(img);
          img.onerror = () => resolve(null);
          img.src = url;
        }),
    );
  }
  return mapImageCache;
}

/** Whether the user configured a map background (null while loading). */
export function useHasMapImage(): boolean | null {
  const [has, setHas] = useState<boolean | null>(null);
  useEffect(() => {
    loadMapImage().then((img) => setHas(!!img));
  }, []);
  return has;
}

/** Forget the cached background (after the setting changes). */
export function resetMapImage() {
  mapImageCache = null;
}

/**
 * Dota map with points. World coordinates are mapped linearly from
 * [world[0], world[1]] on both axes; y grows upwards (Radiant bottom-left).
 * Without a user background a schematic map (lanes, river, bases) is drawn.
 */
export function MapCanvas({ points, world, size = 520, onHover, heat, segments }: {
  points: MapPoint[]; world: number[]; size?: number; onHover?: (index: number | null) => void;
  /** World positions drawn as a density heatmap under the points. */
  heat?: number[][];
  /** Lines between world points (e.g. ward → nothing, path), drawn under points. */
  segments?: { from: number[]; to: number[]; color: string }[];
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  const [bg, setBg] = useState<HTMLImageElement | null>(null);
  const [min, max] = world.length === 2 ? world : [-8288, 8288];
  const toPx = (x: number, y: number): [number, number] => [
    ((x - min) / (max - min)) * size,
    size - ((y - min) / (max - min)) * size,
  ];

  useEffect(() => {
    loadMapImage().then(setBg);
  }, []);

  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    const dpr = window.devicePixelRatio || 1;
    c.width = size * dpr;
    c.height = size * dpr;
    const ctx = c.getContext("2d")!;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    if (bg) {
      ctx.drawImage(bg, 0, 0, size, size);
      ctx.fillStyle = "rgba(0,0,0,0.25)";
      ctx.fillRect(0, 0, size, size);
    } else {
      drawSchematic(ctx, size, toPx);
    }
    if (heat && heat.length) drawHeat(ctx, size, heat.map(([x, y]) => toPx(x, y)));
    for (const sg of segments ?? []) {
      ctx.beginPath();
      ctx.moveTo(...toPx(sg.from[0], sg.from[1]));
      ctx.lineTo(...toPx(sg.to[0], sg.to[1]));
      ctx.strokeStyle = sg.color;
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }
    for (const p of points.filter((p) => !p.highlight)) drawPoint(ctx, toPx(p.x, p.y), p.color, p.label ? 11 : 5, false, p.label);
    for (const p of points.filter((p) => p.highlight)) drawPoint(ctx, toPx(p.x, p.y), p.color, p.label ? 13 : 9, true, p.label);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [points, bg, size, min, max, heat, segments]);

  const onMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!onHover) return;
    const r = e.currentTarget.getBoundingClientRect();
    const mx = e.clientX - r.left, my = e.clientY - r.top;
    let best: number | null = null, bestD = 10;
    points.forEach((p, i) => {
      const [px, py] = toPx(p.x, p.y);
      const d = Math.hypot(px - mx, py - my);
      if (d < bestD) { best = i; bestD = d; }
    });
    onHover(best);
  };

  return (
    <canvas
      ref={ref}
      className="map-canvas"
      style={{ width: size, height: size }}
      onMouseMove={onMove}
      onMouseLeave={() => onHover?.(null)}
    />
  );
}

function drawPoint(ctx: CanvasRenderingContext2D, [x, y]: [number, number], color: string, r: number, ring = false, label?: string) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = color;
  ctx.fill();
  ctx.lineWidth = ring ? 3 : label ? 2.5 : 1.5;
  ctx.strokeStyle = ring ? "#ffffff" : "rgba(10,12,16,0.85)";
  ctx.stroke();
  if (label) {
    ctx.fillStyle = "#16100e";
    ctx.font = `700 ${Math.round(r * 1.05)}px Inter, system-ui, sans-serif`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(label, x, y + 0.5);
  }
}

type ToPx = (x: number, y: number) => [number, number];

function drawSchematic(ctx: CanvasRenderingContext2D, size: number, toPx: ToPx) {
  ctx.fillStyle = "#12151b";
  ctx.fillRect(0, 0, size, size);

  // Team corners: soft glows.
  const glow = (x: number, y: number, color: string) => {
    const [px, py] = toPx(x, y);
    const g = ctx.createRadialGradient(px, py, 0, px, py, size * 0.3);
    g.addColorStop(0, color);
    g.addColorStop(1, "rgba(0,0,0,0)");
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, size, size);
  };
  glow(-7400, -7000, "rgba(79,209,197,0.16)");
  glow(7400, 7000, "rgba(248,119,108,0.16)");

  const path = (pts: [number, number][], color: string, width: number, curve = false) => {
    ctx.beginPath();
    const px = pts.map(([x, y]) => toPx(x, y));
    ctx.moveTo(...px[0]);
    if (curve && px.length >= 3) {
      for (let i = 1; i < px.length - 1; i++) {
        const [mx, my] = [(px[i][0] + px[i + 1][0]) / 2, (px[i][1] + px[i + 1][1]) / 2];
        ctx.quadraticCurveTo(px[i][0], px[i][1], mx, my);
      }
      ctx.lineTo(...px[px.length - 1]);
    } else {
      px.slice(1).forEach((p) => ctx.lineTo(...p));
    }
    ctx.strokeStyle = color;
    ctx.lineWidth = width;
    ctx.lineJoin = "round";
    ctx.lineCap = "butt";
    ctx.stroke();
  };
  // River: same polyline as the backend region geometry (stats/regions.go).
  path([[-8200, 5200], [-3000, 1600], [-600, -400], [2000, -2600], [4400, -8200]], "rgba(46,96,160,0.55)", size * 0.05, true);

  // Lanes: outer square (top + bot) and the mid diagonal.
  const lane = "rgba(120,128,145,0.55)";
  const lw = size * 0.012;
  path([[-6600, -5600], [-6600, 6000], [5600, 6000]], lane, lw);
  path([[-5600, -6600], [6200, -6600], [6200, 5600]], lane, lw);
  path([[-6000, -5600], [5600, 5400]], lane, lw);

  // Ancients and labels.
  const base = (x: number, y: number, color: string, label: string, dx: number, dy: number) => {
    const [px, py] = toPx(x, y);
    ctx.beginPath();
    ctx.arc(px, py, size * 0.042, 0, Math.PI * 2);
    ctx.fillStyle = color;
    ctx.fill();
    ctx.fillStyle = "rgba(200,205,215,0.55)";
    ctx.font = `500 ${Math.round(size * 0.026)}px Inter, system-ui, sans-serif`;
    ctx.textAlign = "left";
    ctx.textBaseline = "middle";
    ctx.fillText(label, px + dx * size, py + dy * size);
  };
  base(-6200, -5800, "#4fd1c5", "Radiant", 0.06, 0.04);
  base(5900, 5700, "#f8776c", "Dire", -0.14, -0.045);
}

/** Density heatmap: additive blobs in an offscreen alpha buffer, drawn as soft warm-red glows. */
function drawHeat(ctx: CanvasRenderingContext2D, size: number, pts: [number, number][]) {
  const off = document.createElement("canvas");
  off.width = size;
  off.height = size;
  const o = off.getContext("2d")!;
  const r = size * 0.055;
  const a = Math.min(0.5, Math.max(0.05, 14 / Math.sqrt(pts.length)));
  for (const [x, y] of pts) {
    const g = o.createRadialGradient(x, y, 0, x, y, r);
    g.addColorStop(0, `rgba(0,0,0,${a})`);
    g.addColorStop(1, "rgba(0,0,0,0)");
    o.fillStyle = g;
    o.fillRect(x - r, y - r, r * 2, r * 2);
  }
  const img = o.getImageData(0, 0, size, size);
  const d = img.data;
  let peak = 1;
  for (let i = 3; i < d.length; i += 4) peak = Math.max(peak, d[i]);
  for (let i = 0; i < d.length; i += 4) {
    const v = d[i + 3] / peak;
    if (v <= 0.02) { d[i + 3] = 0; continue; }
    // low density: muted red; peaks: brighter, slightly orange
    d[i] = 248; d[i + 1] = 110 + 50 * v; d[i + 2] = 100 - 40 * v; d[i + 3] = Math.min(230, 40 + v * 190);
  }
  o.putImageData(img, 0, 0);
  ctx.drawImage(off, 0, 0, size, size);
}
