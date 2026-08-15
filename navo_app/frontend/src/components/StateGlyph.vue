<script setup lang="ts">
import type { IconState } from "../types";
import trayIcon from "../assets/tray_icon.ico";

defineProps<{ state: IconState; size?: "sm" | "md" | "lg"; label?: string }>();
</script>

<template>
  <span
    class="state-glyph"
    :class="[`state-${state}`, `size-${size ?? 'md'}`]"
    role="img"
    :aria-label="label ?? `Navo ${state} 状态`"
  >
    <img :src="trayIcon" alt="" aria-hidden="true" />
  </span>
</template>
