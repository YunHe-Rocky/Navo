<script setup lang="ts">
import { computed, ref } from "vue";
import type { TrafficPoint } from "../types";

const props = defineProps<{ points: TrafficPoint[]; stopped?: boolean; compact?: boolean }>();
const hovered = ref<number | null>(null);
const width = 720;
const height = 220;
const pad = 22;

const peak = computed(() => Math.max(1, ...props.points.flatMap((point) => [point.downloadBps, point.uploadBps])));
const downloadPath = computed(() => linePath("downloadBps"));
const uploadPath = computed(() => linePath("uploadBps"));
const average = computed(() => {
  if (!props.points.length) return { upload: 0, download: 0 };
  return {
    upload: props.points.reduce((sum, point) => sum + point.uploadBps, 0) / props.points.length,
    download: props.points.reduce((sum, point) => sum + point.downloadBps, 0) / props.points.length,
  };
});
const hoverPoint = computed(() => hovered.value === null ? undefined : props.points[hovered.value]);

function linePath(key: "uploadBps" | "downloadBps") {
  if (!props.points.length) return "";
  const usableWidth = width - pad * 2;
  const usableHeight = height - pad * 2;
  return props.points
    .map((point, index) => {
      const x = pad + (index / Math.max(1, props.points.length - 1)) * usableWidth;
      const y = height - pad - (point[key] / peak.value) * usableHeight;
      return `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
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

function setHover(event: MouseEvent) {
  if (!props.points.length) return;
  const box = (event.currentTarget as SVGElement).getBoundingClientRect();
  const ratio = Math.min(1, Math.max(0, (event.clientX - box.left) / box.width));
  hovered.value = Math.round(ratio * (props.points.length - 1));
}
</script>

<template>
  <div class="traffic-chart" :class="{ compact, stopped }">
    <div v-if="points.length" class="chart-summary">
      <span><i class="legend download"></i>下载均值 {{ formatRate(average.download) }}</span>
      <span><i class="legend upload"></i>上传均值 {{ formatRate(average.upload) }}</span>
      <span>峰值 {{ formatRate(peak) }}</span>
      <em v-if="stopped">采样已停止</em>
    </div>
    <div v-if="!points.length" class="chart-empty">
      <strong>暂无真实流量数据</strong>
      <span>连接后每 2 秒采样，窗口固定为最近 30 个点。</span>
    </div>
    <svg
      v-else
      viewBox="0 0 720 220"
      preserveAspectRatio="none"
      role="img"
      aria-label="最近 60 秒上传和下载速度折线图"
      @mousemove="setHover"
      @mouseleave="hovered = null"
    >
      <g class="grid-lines">
        <line v-for="y in [44, 88, 132, 176]" :key="y" x1="22" :y1="y" x2="698" :y2="y" />
      </g>
      <path class="area download" :d="`${downloadPath} L698,198 L22,198 Z`" />
      <path class="line download" :d="downloadPath" />
      <path class="line upload" :d="uploadPath" />
    </svg>
    <div v-if="hoverPoint" class="chart-tooltip">
      <strong>{{ new Date(hoverPoint.timestamp).toLocaleTimeString() }}</strong>
      <span>↓ {{ formatRate(hoverPoint.downloadBps) }}</span>
      <span>↑ {{ formatRate(hoverPoint.uploadBps) }}</span>
    </div>
  </div>
</template>
