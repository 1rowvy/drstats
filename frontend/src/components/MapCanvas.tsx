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
    for (const p of points.filter((p) => !p.highlight)) drawPoint(ctx, toPx(p.x, p.y), p.color, 5);
    for (const p of points.filter((p) => p.highlight)) drawPoint(ctx, toPx(p.x, p.y), p.color, 9, true);
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

function drawPoint(ctx: CanvasRenderingContext2D, [x, y]: [number, number], color: string, r: number, ring = false) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = color;
  ctx.fill();
  ctx.lineWidth = ring ? 3 : 1.5;
  ctx.strokeStyle = ring ? "#ffffff" : "rgba(0,0,0,0.7)";
  ctx.stroke();
}

type ToPx = (x: number, y: number) => [number, number];

function drawSchematic(ctx: CanvasRenderingContext2D, size: number, toPx: ToPx) {
  ctx.fillStyle = "#1b2a1f";
  ctx.fillRect(0, 0, size, size);

  // Halves: slight team tint.
  const tri = (pts: [number, number][], color: string) => {
    ctx.beginPath();
    pts.forEach(([x, y], i) => (i ? ctx.lineTo(...toPx(x, y)) : ctx.moveTo(...toPx(x, y))));
    ctx.closePath();
    ctx.fillStyle = color;
    ctx.fill();
  };
  tri([[-9000, -9000], [-9000, 9000], [9000, -9000]], "rgba(95,209,122,0.06)");
  tri([[9000, 9000], [-9000, 9000], [9000, -9000]], "rgba(255,107,91,0.06)");

  // River: band from top-left to bottom-right.
  const path = (pts: [number, number][], color: string, width: number) => {
    ctx.beginPath();
    pts.forEach(([x, y], i) => (i ? ctx.lineTo(...toPx(x, y)) : ctx.moveTo(...toPx(x, y))));
    ctx.strokeStyle = color;
    ctx.lineWidth = width;
    ctx.lineJoin = "round";
    ctx.lineCap = "round";
    ctx.stroke();
  };
  path([[-7400, 4600], [-3000, 1600], [-600, -400], [2000, -2600], [5000, -7400]], "rgba(70,130,180,0.45)", size * 0.045);

  // Lanes.
  const lane = "rgba(210,190,140,0.35)";
  const lw = size * 0.018;
  path([[-6600, -5600], [-6600, 6000], [5600, 6000]], lane, lw); // top
  path([[-5600, -6600], [6200, -6600], [6200, 5600]], lane, lw); // bot
  path([[-5200, -5000], [5000, 4800]], lane, lw); // mid

  // Bases.
  const base = (x: number, y: number, color: string) => {
    const [px, py] = toPx(x, y);
    ctx.beginPath();
    ctx.arc(px, py, size * 0.07, 0, Math.PI * 2);
    ctx.fillStyle = color;
    ctx.fill();
  };
  base(-6700, -6200, "rgba(95,209,122,0.35)");
  base(6500, 6000, "rgba(255,107,91,0.35)");
}

/** Density heatmap: additive blobs in an offscreen alpha buffer, colourized blue → yellow → red. */
function drawHeat(ctx: CanvasRenderingContext2D, size: number, pts: [number, number][]) {
  const off = document.createElement("canvas");
  off.width = size;
  off.height = size;
  const o = off.getContext("2d")!;
  const r = size * 0.035;
  const a = Math.min(0.35, Math.max(0.04, 12 / Math.sqrt(pts.length)));
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
    // 0 → blue, 0.5 → yellow, 1 → red
    const rr = v < 0.5 ? 255 * v * 2 : 255;
    const gg = v < 0.5 ? 200 * v * 2 + 40 : 240 - (v - 0.5) * 2 * 200;
    const bb = v < 0.5 ? 255 * (1 - v * 2) : 0;
    d[i] = rr; d[i + 1] = gg; d[i + 2] = bb; d[i + 3] = Math.min(220, 60 + v * 200);
  }
  o.putImageData(img, 0, 0);
  ctx.drawImage(off, 0, 0, size, size);
}
