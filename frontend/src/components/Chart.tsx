import { EChart } from "./EChart";
import type { EChartsOption } from "echarts";
import { clock } from "../format";
import type { Line } from "../api";

export const SERIES_COLORS = { focus: "#5b9cff", team: "#9aa3b5", opponent: "#ff6b5b" };

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
    title: { text: title, left: 8, top: 4, textStyle: { color: "#e6e8ee", fontSize: 13, fontWeight: 600 } },
    grid: { left: 56, right: 16, top: 40, bottom: 48 },
    legend: { top: 4, right: 8, textStyle: { color: "#9aa3b5" }, itemWidth: 16, itemHeight: 8 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "#1d212a",
      borderColor: "#2c3240",
      textStyle: { color: "#e6e8ee" },
      valueFormatter: (v) => (v == null ? "—" : Math.round(Number(v)).toLocaleString("ru-RU")),
    },
    xAxis: {
      type: "category",
      data: t.map(clock),
      boundaryGap: false,
      axisLine: { lineStyle: { color: "#2c3240" } },
      axisLabel: { color: "#6b7385" },
    },
    yAxis: {
      type: "value",
      splitLine: { lineStyle: { color: "#232833" } },
      axisLabel: { color: "#6b7385" },
    },
    dataZoom: [{ type: "inside" }, { type: "slider", height: 16, bottom: 8, borderColor: "#2c3240", textStyle: { color: "#6b7385" } }],
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
  backgroundColor: "#1d212a",
  borderColor: "#2c3240",
  textStyle: { color: "#e6e8ee" },
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
    axisLine: { lineStyle: { color: "#2c3240" } },
    axisLabel: { color: "#9aa3b5", interval: 0 },
    inverse: horizontal,
  };
  const val = {
    type: "value" as const,
    max,
    splitLine: { lineStyle: { color: "#232833" } },
    axisLabel: { color: "#6b7385", formatter: (v: number) => f(v) },
  };
  const option: EChartsOption = {
    backgroundColor: "transparent",
    animation: false,
    title: title ? { text: title, left: 8, top: 4, textStyle: { color: "#e6e8ee", fontSize: 13, fontWeight: 600 } } : undefined,
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
    title: { text: title, left: 8, top: 4, textStyle: { color: "#e6e8ee", fontSize: 13, fontWeight: 600 } },
    grid: { left: 56, right: 16, top: 40, bottom: 28 },
    legend: { top: 4, right: 8, textStyle: { color: "#9aa3b5" }, itemWidth: 16, itemHeight: 8 },
    tooltip: { trigger: "axis", ...tooltipStyle, valueFormatter: (v) => (v == null ? "—" : fmt(Number(v))) },
    xAxis: { type: "category", data: labels, axisLine: { lineStyle: { color: "#2c3240" } }, axisLabel: { color: "#6b7385" } },
    yAxis: { type: "value", splitLine: { lineStyle: { color: "#232833" } }, axisLabel: { color: "#6b7385", formatter: (v: number) => fmt(v) } },
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
