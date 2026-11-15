import { EChart } from "./EChart";
import type { EChartsOption } from "echarts";
import { clock } from "../format";
import type { Line } from "../api";

export const SERIES_COLORS = { focus: "#f2a93b", team: "#9aa1af", opponent: "#4ea1f5" };

export interface ChartLine {
  name: string;
  data: Line;
  color: string;
  dashed?: boolean;
}

/** Time-series line chart with the app's dark styling; x axis is game time. */
export function TimeChart({ title, t, lines, height = 260 }: {
  title: string; t: number[]; lines: ChartLine[]; height?: number;
}) {
  const option: EChartsOption = {
    backgroundColor: "transparent",
    animation: false,
    title: { text: title, left: 8, top: 4, textStyle: { color: "#e8eaf0", fontSize: 13, fontWeight: 600 } },
    grid: { left: 56, right: 16, top: 40, bottom: 48 },
    legend: { top: 4, right: 8, textStyle: { color: "#9aa1af" }, itemWidth: 16, itemHeight: 8 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "#191d25",
      borderColor: "#2d3340",
      textStyle: { color: "#e8eaf0" },
      valueFormatter: (v) => (v == null ? "—" : Math.round(Number(v)).toLocaleString("ru-RU")),
    },
    xAxis: {
      type: "category",
      data: t.map(clock),
      boundaryGap: false,
      axisLine: { lineStyle: { color: "#2d3340" } },
      axisLabel: { color: "#697080" },
    },
    yAxis: {
      type: "value",
      splitLine: { lineStyle: { color: "#1f232c" } },
      axisLabel: { color: "#697080" },
    },
    dataZoom: [{ type: "inside" }, { type: "slider", height: 16, bottom: 8, borderColor: "#2d3340", textStyle: { color: "#697080" } }],
    series: lines
      .filter((l) => l.data && l.data.length > 0)
      .map((l) => ({
        name: l.name,
        type: "line",
        data: l.data as (number | null)[],
        showSymbol: false,
        connectNulls: false,
        lineStyle: { color: l.color, width: 2, type: l.dashed ? "dashed" : "solid" },
        itemStyle: { color: l.color },
      })),
  };
  return <EChart option={option} height={height} />;
}

const tooltipStyle = {
  backgroundColor: "#191d25",
  borderColor: "#2d3340",
  textStyle: { color: "#e8eaf0" },
};

export interface Bar {
  label: string;
  value: number;
  color?: string;
  tip?: string; // extra tooltip line
  text?: string; // tooltip value text instead of fmt(value)
}

/** Simple bar chart; horizontal bars suit long labels (gold sources, heroes). */
export function BarChart({ title, bars, height = 260, horizontal = false, fmt, max, onClick }: {
  title?: string; bars: Bar[]; height?: number; horizontal?: boolean; fmt?: (v: number) => string; max?: number;
  onClick?: (index: number) => void;
}) {
  const f = fmt ?? ((v: number) => Math.round(v).toLocaleString("ru-RU"));
  const cat = {
    type: "category" as const,
    data: bars.map((b) => b.label),
    axisLine: { lineStyle: { color: "#2d3340" } },
    axisLabel: { color: "#9aa1af", interval: 0 },
    inverse: horizontal,
  };
  const val = {
    type: "value" as const,
    max,
    splitLine: { lineStyle: { color: "#1f232c" } },
    axisLabel: { color: "#697080", formatter: (v: number) => f(v) },
  };
  const option: EChartsOption = {
    backgroundColor: "transparent",
    animation: false,
    title: title ? { text: title, left: 8, top: 4, textStyle: { color: "#e8eaf0", fontSize: 13, fontWeight: 600 } } : undefined,
    grid: { left: horizontal ? 130 : 48, right: 24, top: title ? 36 : 12, bottom: 28 },
    tooltip: {
      trigger: "item", ...tooltipStyle,
      formatter: (p: any) => `${bars[p.dataIndex].label}: <b>${bars[p.dataIndex].text ?? f(bars[p.dataIndex].value)}</b>${bars[p.dataIndex].tip ? "<br/>" + bars[p.dataIndex].tip : ""}`,
    },
    xAxis: horizontal ? val : cat,
    yAxis: horizontal ? cat : val,
    series: [{
      type: "bar",
      data: bars.map((b) => ({ value: b.value, itemStyle: { color: b.color ?? SERIES_COLORS.focus } })),
      barMaxWidth: 28,
      cursor: onClick ? "pointer" : "default",
    }],
  };
  return <EChart option={option} height={height} onClick={onClick} />;
}

/** Per-match values with a rolling average, x axis = match order. */
export function TrendChart({ title, labels, values, wins, fmt, height = 260, window = 5 }: {
  title: string; labels: string[]; values: (number | null)[]; wins: boolean[]; fmt: (v: number) => string;
  height?: number; window?: number;
}) {
  const rolling = values.map((_, i) => {
    const xs = values.slice(Math.max(0, i - window + 1), i + 1).filter((v): v is number => v != null);
    return xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null;
  });
  const option: EChartsOption = {
    backgroundColor: "transparent",
    animation: false,
    title: { text: title, left: 8, top: 4, textStyle: { color: "#e8eaf0", fontSize: 13, fontWeight: 600 } },
    grid: { left: 56, right: 16, top: 40, bottom: 28 },
    legend: { top: 4, right: 8, textStyle: { color: "#9aa1af" }, itemWidth: 16, itemHeight: 8 },
    tooltip: { trigger: "axis", ...tooltipStyle, valueFormatter: (v) => (v == null ? "—" : fmt(Number(v))) },
    xAxis: { type: "category", data: labels, axisLine: { lineStyle: { color: "#2d3340" } }, axisLabel: { color: "#697080" } },
    yAxis: { type: "value", splitLine: { lineStyle: { color: "#1f232c" } }, axisLabel: { color: "#697080", formatter: (v: number) => fmt(v) } },
    series: [
      {
        name: "Матч", type: "bar", barMaxWidth: 14,
        data: values.map((v, i) => ({ value: v, itemStyle: { color: wins[i] ? "rgba(95,209,122,.55)" : "rgba(255,107,91,.55)" } })),
      },
      {
        name: `Среднее за ${window}`, type: "line", data: rolling, showSymbol: false,
        lineStyle: { color: SERIES_COLORS.focus, width: 2 }, itemStyle: { color: SERIES_COLORS.focus },
      },
    ],
  };
  return <EChart option={option} height={height} />;
}

/** Me vs lane opponent over game time (minutes on a value axis), mockup style. */
export function VersusChart({ t, me, opp, oppName = "Оппонент", height = 220 }: {
  t: number[]; me: Line; opp?: Line; oppName?: string; height?: number;
}) {
  const pts = (l: Line | undefined) =>
    (l ?? []).map((v, i) => [t[i] / 60, v] as [number, number | null]).filter((p) => p[0] >= 0 && p[1] != null);
  const mine = pts(me);
  const last = mine[mine.length - 1];
  const option: EChartsOption = {
    backgroundColor: "transparent",
    animation: false,
    grid: { left: 40, right: 12, top: 12, bottom: 24 },
    tooltip: {
      trigger: "axis", ...tooltipStyle,
      axisPointer: { lineStyle: { color: "#2d3340" } },
      formatter: (ps: any) => {
        const arr = Array.isArray(ps) ? ps : [ps];
        const head = arr.length ? clock(arr[0].value[0] * 60) : "";
        return head + arr.map((p: any) => `<br/>${p.marker}${p.seriesName}: <b>${Math.round(p.value[1]).toLocaleString("ru-RU")}</b>`).join("");
      },
    },
    xAxis: {
      type: "value", min: 0, max: (v) => Math.ceil(v.max),
      axisLine: { show: false }, axisTick: { show: false }, splitLine: { show: false },
      axisLabel: { color: "#697080", fontFamily: "monospace", fontSize: 10, formatter: (v: number) => (v % 10 === 0 ? String(v) : "") },
      interval: 10,
    },
    yAxis: {
      type: "value", splitNumber: 4,
      splitLine: { lineStyle: { color: "#1f232c" } },
      axisLabel: { color: "#697080", fontFamily: "monospace", fontSize: 10, formatter: (v: number) => (v >= 1000 ? `${v / 1000}k` : String(v)) },
    },
    series: [
      {
        name: "Ты", type: "line", data: mine, showSymbol: false, smooth: 0.2,
        lineStyle: { color: SERIES_COLORS.focus, width: 2 }, itemStyle: { color: SERIES_COLORS.focus },
        markPoint: last ? { symbol: "circle", symbolSize: 7, data: [{ name: "end", coord: last as any }], itemStyle: { color: SERIES_COLORS.focus }, label: { show: false } } : undefined,
      },
      ...(opp ? [{
        name: oppName, type: "line" as const, data: pts(opp), showSymbol: false, smooth: 0.2,
        lineStyle: { color: "#9aa1af", width: 1.5, type: "dashed" as const }, itemStyle: { color: "#9aa1af" },
      }] : []),
    ],
  };
  return <EChart option={option} height={height} />;
}
