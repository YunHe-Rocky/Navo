<script setup lang="ts">
import type { IconState } from "../types";

defineProps<{ state: IconState; size?: "sm" | "md" | "lg"; label?: string }>();
</script>

<template>
  <span
    class="state-glyph"
    :class="[`state-${state}`, `size-${size ?? 'md'}`]"
    role="img"
    :aria-label="label ?? `Navo ${state} 状态`"
  >
    <svg viewBox="0 0 32 32" aria-hidden="true">
      <path d="M8 24V8l16 16V8" />
      <circle cx="24" cy="8" r="3.25" />
    </svg>
  </span>
</template>
