import type { TrafficChartPreferences, TrafficPoint } from "./types";

export function parseTrafficPreferences(serialized: string | null): TrafficChartPreferences;
export function seriesForCaptureMode(mode: string): TrafficChartPreferences["visibleSeries"];
export function trafficContextForCaptureMode(mode: string): { id: string; label: string; source: string };
export function generateSyntheticTraffic(
  sizeMiB: number,
  direction: "download" | "upload" | "both",
  now?: number,
): { size: number; points: TrafficPoint[] };
