<script setup lang="ts">
import { computed } from "vue";
import { useNavoApplicationContext } from "../application/context";
import type { NetworkEnvironmentFinding, NetworkResourceSnapshot } from "../../types";

const {
  dashboard,
  loading,
  repairNetworkEnvironment,
  formatTime,
} = useNavoApplicationContext();

const environment = computed(() => dashboard.value.environment);
const healthLabel = computed(() => {
  switch (environment.value?.health) {
    case "healthy": return "环境健康";
    case "degraded": return "环境降级";
    case "unavailable": return "环境不可用";
    case "checking": return "正在检查";
    default: return "等待环境证据";
  }
});

function resourceLabel(resource?: NetworkResourceSnapshot) {
  if (!resource?.known) return "未确认";
  if (resource.coherent) return "一致";
  if (resource.conflict_count > 0) return "存在冲突";
  if (resource.missing_count > 0) return "资源缺失";
  return "不一致";
}

const statusItems = computed(() => {
  const value = environment.value;
  if (!value) return [];
  const proxy = value.system_proxy;
  const tun = value.tun;
  return [
    { label: "物理网络", value: !value.physical.known ? "未确认" : value.physical.available ? "可用" : "不可用" },
    { label: "系统代理", value: proxy.owned_by_navo ? (proxy.reachable_known && !proxy.reachable ? "Navo 端点不可达" : "Navo 管理") : proxy.enabled ? "外部管理" : "未启用" },
    { label: "TUN", value: tun.navo.present ? (tun.navo.enabled ? "Navo 已启用" : "Navo 未启用") : tun.external_present ? "仅检测到外部网卡" : "未启用" },
    { label: "DNS / NRPT", value: resourceLabel(value.nrpt.known ? value.nrpt : value.dns) },
    { label: "路由", value: resourceLabel(value.routes) },
    { label: "防火墙", value: resourceLabel(value.firewall) },
  ];
});

function canRepair(finding: NetworkEnvironmentFinding) {
  const value = environment.value;
  return Boolean(
    value &&
    finding.recoverable &&
    finding.ownership === "navo" &&
    !finding.transitional &&
    !value.stale &&
    !value.transition.busy,
  );
}

function repairLabel(finding: NetworkEnvironmentFinding) {
  if (finding.transitional || environment.value?.transition.busy) return "等待当前操作完成";
  if (environment.value?.stale) return "等待新快照";
  if (finding.ownership !== "navo") return "外部资源仅观察";
  return finding.recoverable ? "修复并重新验证" : "仅观察";
}
</script>

<template>
  <article
    class="environment-card"
    :data-health="environment?.health || 'unknown'"
    aria-labelledby="environment-card-title"
    aria-live="polite"
  >
    <header class="environment-card-heading">
      <div>
        <span class="card-label">网络环境 · 只读聚合</span>
        <h3 id="environment-card-title">{{ healthLabel }}</h3>
        <p v-if="environment">
          {{ environment.partial ? "部分状态暂不可读" : "所有观察源已汇总" }}
          <span v-if="environment.stale"> · 快照已过期</span>
        </p>
        <p v-else>当前 Agent 尚未提供环境快照，连接功能仍按兼容模式运行。</p>
      </div>
      <div class="environment-meta">
        <strong>{{ environment?.findings.length || 0 }} 项发现</strong>
        <small>{{ formatTime(environment?.collected_at) }}</small>
      </div>
    </header>

    <ul v-if="environment" class="environment-status-grid" aria-label="网络环境分项状态">
      <li v-for="item in statusItems" :key="item.label">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
      </li>
    </ul>

    <div v-if="environment?.findings.length" class="environment-findings">
      <h4>环境发现</h4>
      <ul>
        <li
          v-for="finding in environment.findings"
          :key="finding.code"
          class="environment-finding"
          :data-severity="finding.severity"
        >
          <div>
            <span class="finding-code mono">{{ finding.code }}</span>
            <strong>{{ finding.summary }}</strong>
            <p v-if="finding.detail">{{ finding.detail }}</p>
            <small>
              所有权：{{ finding.ownership === "navo" ? "Navo" : finding.ownership === "external" ? "外部" : "未确认" }}
              <span v-if="finding.transitional"> · 当前状态正在变化</span>
            </small>
          </div>
          <button
            v-if="finding.recoverable && finding.ownership === 'navo'"
            class="secondary environment-repair"
            :disabled="loading || !canRepair(finding)"
            :aria-label="`${finding.summary}：${repairLabel(finding)}`"
            @click="repairNetworkEnvironment(finding.code)"
          >
            {{ loading ? "处理中" : repairLabel(finding) }}
          </button>
          <span v-else class="finding-observe-only">{{ repairLabel(finding) }}</span>
        </li>
      </ul>
    </div>
    <p v-else-if="environment" class="environment-empty">未发现需要处理的网络环境异常。</p>
  </article>
</template>
