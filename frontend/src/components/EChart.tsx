import { useEffect, useRef } from "react";
import * as echarts from "echarts/core";
import { BarChart, LineChart, ScatterChart } from "echarts/charts";
import { DataZoomComponent, GridComponent, LegendComponent, MarkLineComponent, MarkPointComponent, TitleComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { EChartsOption } from "echarts";

echarts.use([LineChart, BarChart, ScatterChart, MarkLineComponent, GridComponent, TooltipComponent, LegendComponent, TitleComponent, DataZoomComponent, MarkPointComponent, CanvasRenderer]);

/** Minimal React wrapper around an ECharts instance (resizes with its box). */
export function EChart({ option, height, onClick }: {
  option: EChartsOption; height: number; onClick?: (dataIndex: number) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const chart = useRef<echarts.ECharts>();

  useEffect(() => {
    const el = ref.current!;
    chart.current = echarts.init(el, undefined, { renderer: "canvas" });
    const ro = new ResizeObserver(() => chart.current?.resize());
    ro.observe(el);
    return () => {
      ro.disconnect();
      chart.current?.dispose();
    };
  }, []);

  useEffect(() => {
    chart.current?.setOption(option, { notMerge: true });
  }, [option]);

  useEffect(() => {
    const c = chart.current;
    if (!c || !onClick) return;
    const h = (p: any) => onClick(p.dataIndex);
    c.on("click", h);
    return () => { c.off("click", h); };
  }, [onClick]);

  return <div ref={ref} style={{ height, width: "100%" }} />;
}
