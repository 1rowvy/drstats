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
