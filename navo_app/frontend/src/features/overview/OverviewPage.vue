<script setup lang="ts">
import StateGlyph from "../../components/StateGlyph.vue";
import TrafficChart from "../../components/TrafficChart.vue";
import { useNavoApplicationContext } from "../application/context";
import type { CaptureMode } from "../../types";

const {
  page,
  dashboard,
  benchmark,
  benchmarkRunning,
  trafficPoints,
  loading,
  ipChecking,
  activeRoute,
  activeRouteLatency,
  captureMode,
  primaryConnectionAction,
  effectiveConnection,
  connectionIcon,
  networkHealthLabel,
  activeTrafficSeries,
  trafficContext,
  activeTrafficUnavailable,
  activeTrafficUnavailableReason,
  activeTrafficDownload,
  activeTrafficUpload,
  activeTrafficDownloadTotal,
  activeTrafficUploadTotal,
  directAndProxySame,
  activeRisk,
  checkConnection,
  runBenchmark,
  changePage,
  toggleConnection,
  goToRouteSelection,
  testActiveRoute,
  captureLabel,
  recoveryStateLabel,
  faultDomainLabel,
  repairActionLabel,
  connectionLabel,
  formatBytes,
  formatRate,
  formatUptime,
  formatTime,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page overview-page">
    <article class="hero-status" :class="`state-${connectionIcon}`">
      <div class="hero-copy">
        <StateGlyph :state="connectionIcon" size="lg" />
        <div>
          <span class="eyebrow">{{ networkHealthLabel }}</span>
          <h2>{{ connectionLabel() }}</h2>
          <p>{{ effectiveConnection.summary }} · {{ effectiveConnection.modeLabel }}</p>
        </div>
      </div>
      <div class="hero-actions">
        <button class="secondary" :disabled="loading || ipChecking" @click="checkConnection">{{ ipChecking ? "验证中" : effectiveConnection.kind === "external_system_proxy" ? "检测外部代理出口" : effectiveConnection.kind === "direct" ? "检测公网出口" : "验证 ChatGPT 链路" }}</button>
        <button
          class="primary"
          :disabled="loading"
          @click="primaryConnectionAction.kind === 'select_route' ? goToRouteSelection() : toggleConnection()"
        >{{ primaryConnectionAction.label }}</button>
      </div>
    </article>
  
    <div v-if="directAndProxySame" class="warning-banner" role="status">直连公网 IP 与代理出口 IP 相同，代理可能未生效。</div>
  
    <article
      v-if="dashboard.capture.recovery && dashboard.capture.recovery.state !== 'idle'"
      class="recovery-card"
      :data-state="dashboard.capture.recovery.state"
      role="status"
      aria-live="polite"
    >
      <div class="recovery-heading">
        <div>
          <span class="card-label">自动恢复报告 · {{ dashboard.capture.recovery.evidence.code || "等待证据" }}</span>
          <h3>{{ dashboard.capture.recovery.evidence.summary || "正在归因连接故障" }}</h3>
        </div>
        <strong class="recovery-state">{{ recoveryStateLabel(dashboard.capture.recovery.state) }}</strong>
      </div>
      <div class="recovery-evidence-grid">
        <span><small>故障域</small><strong>{{ faultDomainLabel(dashboard.capture.recovery.evidence.domain) }}</strong></span>
        <span><small>影响</small><strong>{{ dashboard.capture.recovery.final_impact || dashboard.capture.recovery.evidence.impact || "正在评估" }}</strong></span>
        <span><small>活动节点</small><strong class="mono">{{ dashboard.capture.recovery.evidence.outbound_id || "未获取" }}</strong></span>
        <span><small>接管方式</small><strong>{{ dashboard.capture.recovery.evidence.capture_mode ? captureLabel(dashboard.capture.recovery.evidence.capture_mode as CaptureMode) : "未获取" }}</strong></span>
      </div>
      <div class="recovery-progress-grid">
        <div v-if="dashboard.capture.recovery.rounds.length">
          <span class="card-label">两轮最小修复</span>
          <ol class="recovery-list">
            <li v-for="round in dashboard.capture.recovery.rounds" :key="round.round">
              <span>第 {{ round.round }} 轮 · {{ repairActionLabel(round.action) }}</span>
              <strong :class="round.recovered ? 'result-ok' : 'result-failed'">{{ round.recovered ? "验证通过" : round.error ? "失败" : "执行中" }}</strong>
              <small v-if="round.error">{{ round.error }}</small>
              <small v-else-if="round.evidence">{{ round.evidence }}</small>
            </li>
          </ol>
        </div>
        <div v-if="dashboard.capture.recovery.candidates?.length">
          <span class="card-label">同通道候选验证</span>
          <ol class="recovery-list">
            <li v-for="candidate in dashboard.capture.recovery.candidates" :key="candidate.outbound_id">
              <span class="mono">{{ candidate.outbound_id }} · {{ candidate.latency_ms ?? "—" }} ms</span>
              <strong :class="candidate.verified ? 'result-ok' : 'result-failed'">{{ candidate.verified ? "验证通过" : candidate.error ? "失败" : candidate.selected ? "验证中" : "已排除" }}</strong>
              <small v-if="candidate.error">{{ candidate.error }}</small>
            </li>
          </ol>
        </div>
      </div>
      <p v-if="dashboard.capture.recovery.final_error" class="inline-error">{{ dashboard.capture.recovery.final_error }}</p>
      <small class="recovery-stamp">最后更新：{{ formatTime(dashboard.capture.recovery.updated_at) }}</small>
    </article>
  
    <div class="overview-grid">
      <article class="connection-evidence-card" :data-kind="effectiveConnection.kind" aria-labelledby="effective-connection-title">
        <div class="connection-evidence-heading">
          <div>
            <span class="card-label">当前有效连接 · {{ effectiveConnection.ownerLabel }}</span>
            <h3 id="effective-connection-title">{{ effectiveConnection.modeLabel }}</h3>
          </div>
          <span class="connection-owner-badge">{{ effectiveConnection.controlledByNavo ? "Navo 管理" : "只读观察" }}</span>
        </div>
        <div class="connection-primary-evidence">
          <small>{{ effectiveConnection.kind === "direct" ? "当前公网 IP" : "当前出口 IP" }}</small>
          <strong class="mono">{{ effectiveConnection.exitIP || (effectiveConnection.exitPending ? "正在检测" : "等待检测") }}</strong>
          <p v-if="effectiveConnection.exitError" class="inline-error">{{ effectiveConnection.exitError }}</p>
          <p v-else>{{ effectiveConnection.exitCountry || "地区未知" }}</p>
        </div>
        <dl class="connection-evidence-grid">
          <div><dt>连接归属</dt><dd>{{ effectiveConnection.ownerLabel }}</dd></div>
          <div><dt>连接对象</dt><dd>{{ effectiveConnection.routeName || "未确认" }}</dd></div>
          <div><dt>代理端点</dt><dd class="mono">{{ effectiveConnection.endpoint || "不适用" }}</dd></div>
          <div><dt>直连基线</dt><dd class="mono">{{ effectiveConnection.directIP || "等待检测" }}</dd></div>
          <div><dt>出口检测源</dt><dd>{{ effectiveConnection.exitProvider || "等待检测" }}</dd></div>
          <div><dt>检测时间</dt><dd>{{ formatTime(effectiveConnection.exitCheckedAt) }}</dd></div>
        </dl>
        <small class="connection-evidence-note">{{ effectiveConnection.trafficNote }}</small>
      </article>
      <article class="risk-card">
        <span class="card-label">当前连接证据</span>
        <strong :class="`risk-${activeRisk.level}`">{{ activeRisk.label }}</strong>
        <ul><li v-for="reason in activeRisk.reasons" :key="reason">{{ reason }}</li></ul>
        <p v-if="activeRisk.action" class="risk-action">建议：{{ activeRisk.action }}</p>
        <small>{{ effectiveConnection.kind === "external_system_proxy" ? "外部出口已独立检测；ChatGPT 应用链路未由 Navo 接管验证" : dashboard.capture.readiness.default_proxy ? "Windows 默认代理已验证" : captureMode === "tun" ? "TUN 数据面已验证" : "尚无默认应用证据" }} · {{ formatTime(effectiveConnection.exitCheckedAt || dashboard.capture.readiness.checked_at) }}</small>
      </article>
    </div>
  
    <div class="monitor-grid">
      <article class="speed-card">
        <span class="card-label">{{ trafficContext.label }} · {{ trafficContext.source }}</span>
  			<div class="speed-value download"><small>实时下载</small><strong>{{ formatRate(activeTrafficDownload) }}</strong></div>
  			<div class="speed-value upload"><small>实时上传</small><strong>{{ formatRate(activeTrafficUpload) }}</strong></div>
        <dl>
          <div><dt>累计下载</dt><dd>{{ formatBytes(activeTrafficDownloadTotal) }}</dd></div>
          <div><dt>累计上传</dt><dd>{{ formatBytes(activeTrafficUploadTotal) }}</dd></div>
          <div><dt>采样来源</dt><dd>{{ trafficContext.source }}</dd></div>
          <div><dt>节点延迟</dt><dd>{{ activeRouteLatency }}</dd></div>
        </dl>
        <div class="speed-card-actions">
          <button class="secondary compact" :disabled="loading || !activeRoute" @click="testActiveRoute">测试延迟</button>
          <button class="primary compact" :disabled="benchmarkRunning" @click="runBenchmark">完整测速</button>
        </div>
        <small v-if="benchmark" class="speed-benchmark-summary">
          下载 {{ benchmark.download_mbps.toFixed(2) }} Mbps · 上传 {{ benchmark.upload_mbps.toFixed(2) }} Mbps
        </small>
      </article>
      <article class="chart-card">
        <div class="section-heading"><div><span class="card-label">最近 60 秒 · 自动跟随接管模式</span><h3>{{ trafficContext.label }}</h3></div><button class="text-button" @click="changePage('traffic')">查看详情</button></div>
        <TrafficChart :points="trafficPoints" :visible-series="activeTrafficSeries" :stopped="activeTrafficUnavailable" :status-label="activeTrafficUnavailableReason" :local-metric-label="effectiveConnection.kind === 'external_system_proxy' ? '系统总量' : undefined" compact />
      </article>
    </div>
  
    <article class="runtime-strip">
      <span><b>{{ dashboard.core.core_id }}</b><small>当前内核</small></span>
      <span><b>{{ captureLabel(captureMode) }}</b><small>接管模式</small></span>
      <span><b>{{ formatUptime(dashboard.core.uptime_seconds) }}</b><small>运行时长</small></span>
      <span><b class="mono">{{ dashboard.core.config_hash || "未激活" }}</b><small>配置 Revision</small></span>
      <span><b>{{ dashboard.core.pid || "—" }}</b><small>PID</small></span>
    </article>
  </section>
</template>
