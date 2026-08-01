<script setup lang="ts">
import { computed, ref } from "vue";
import type { TrafficPoint, TrafficSeries } from "../types";

const props = defineProps<{
  points: TrafficPoint[];
  visibleSeries: TrafficSeries[];
  stopped?: boolean;
  compact?: boolean;
}>();

const hovered = ref<number | null>(null);
const width = 720;
const height = 220;
const pad = 22;

const series = [
  { id: "localUploadBps", label: "本机出口上传", symbol: "↑" },
  { id: "localDownloadBps", label: "本机入口下载", symbol: "↓" },
  { id: "proxyUploadBps", label: "代理业务上传", symbol: "⇧" },
  { id: "proxyDownloadBps", label: "代理业务下载", symbol: "⇩" },
] as const;

const visible = computed(() => series.filter((item) => props.visibleSeries.includes(item.id)));
const peak = computed(() => Math.max(
  1,
  ...props.points.flatMap((point) => visible.value.map((item) => point[item.id])),
));
const paths = computed(() => Object.fromEntries(
  visible.value.map((item) => [item.id, linePath(item.id)]),
) as Partial<Record<TrafficSeries, string>>);
const averages = computed(() => Object.fromEntries(series.map((item) => [
  item.id,
  props.points.length
    ? props.points.reduce((sum, point) => sum + point[item.id], 0) / props.points.length
    : 0,
])) as Record<TrafficSeries, number>);
// Keep the newest sample expanded by default; pointer/keyboard interaction can
// inspect an older point without requiring hover-only access.
const activeIndex = computed(() => hovered.value ?? (props.points.length ? props.points.length - 1 : null));
const hoverPoint = computed(() => activeIndex.value === null ? undefined : props.points[activeIndex.value]);

function linePath(key: TrafficSeries) {
  if (!props.points.length) return "";
  const usableWidth = width - pad * 2;
  const usableHeight = height - pad * 2;
  return props.points.map((point, index) => {
    const x = pad + (index / Math.max(1, props.points.length - 1)) * usableWidth;
    const y = height - pad - (point[key] / peak.value) * usableHeight;
    return `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(" ");
}

function formatRate(value: number) {
  const units = ["B/s", "KB/s", "MB/s", "GB/s"];
  let current = Math.max(0, value);
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index++;
  }
  return `${current.toFixed(index ? 1 : 0)} ${units[index]}`;
}

function setHover(event: PointerEvent) {
  if (!props.points.length) return;
  const box = (event.currentTarget as SVGElement).getBoundingClientRect();
  const ratio = Math.min(1, Math.max(0, (event.clientX - box.left) / box.width));
  hovered.value = Math.round(ratio * (props.points.length - 1));
}

function setKeyboardPoint(event: KeyboardEvent) {
  if (!props.points.length || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
  event.preventDefault();
  const current = activeIndex.value ?? props.points.length - 1;
  if (event.key === "Home") hovered.value = 0;
  else if (event.key === "End") hovered.value = props.points.length - 1;
  else hovered.value = Math.min(props.points.length - 1, Math.max(0, current + (event.key === "ArrowLeft" ? -1 : 1)));
}
</script>

<template>
  <div class="traffic-chart" :class="{ compact, stopped }">
    <div v-if="points.length" class="chart-summary">
      <span v-for="item in visible" :key="item.id" :class="`series-${item.id}`">
        <i class="legend"></i>{{ item.symbol }} {{ item.label }}均值 {{ formatRate(averages[item.id]) }}
      </span>
      <span>峰值 {{ formatRate(peak) }}</span>
      <em v-if="stopped">代理采样已停止，本机采样仍可继续</em>
    </div>
    <div v-if="!points.length || !visible.length" class="chart-empty">
      <strong>{{ visible.length ? "暂无真实流量数据" : "未选择显示曲线" }}</strong>
      <span>后端每 2 秒采样，窗口固定为最近 30 个点。</span>
    </div>
    <svg
      v-else
      viewBox="0 0 720 220"
      preserveAspectRatio="none"
      role="img"
	  tabindex="0"
      aria-label="最近 60 秒本机与代理上传下载速度折线图"
	  @pointermove="setHover"
	  @pointerdown="setHover"
      @mouseleave="hovered = null"
	  @keydown="setKeyboardPoint"
    >
      <g class="grid-lines">
        <line v-for="y in [44, 88, 132, 176]" :key="y" x1="22" :y1="y" x2="698" :y2="y" />
      </g>
      <path
        v-for="item in visible"
        :key="item.id"
        :class="['line', `series-${item.id}`]"
        :d="paths[item.id]"
      />
    </svg>
    <div v-if="hoverPoint" class="chart-tooltip" role="status" aria-live="polite">
      <strong>{{ new Date(hoverPoint.timestamp).toLocaleTimeString() }}</strong>
      <span v-for="item in visible" :key="item.id" :class="`series-${item.id}`">
        {{ item.symbol }} {{ item.label }} {{ formatRate(hoverPoint[item.id]) }}
      </span>
    </div>
  </div>
</template>
