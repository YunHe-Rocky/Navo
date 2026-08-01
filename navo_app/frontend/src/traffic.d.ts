import type { TrafficChartPreferences, TrafficPoint } from "./types";

export function parseTrafficPreferences(serialized: string | null): TrafficChartPreferences;
export function generateSyntheticTraffic(
  sizeMiB: number,
  direction: "download" | "upload" | "both",
  now?: number,
): { size: number; points: TrafficPoint[] };
