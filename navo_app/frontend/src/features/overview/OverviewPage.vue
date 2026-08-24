<script setup lang="ts">
import StateGlyph from "../../components/StateGlyph.vue";
import TrafficChart from "../../components/TrafficChart.vue";
import NetworkEnvironmentCard from "../environment/NetworkEnvironmentCard.vue";
import { useNavoApplicationContext } from "../application/context";
import type { CaptureMode } from "../../types";

const {
  page,
  dashboard,
  ipDetection,
  benchmark,
  benchmarkRunning,
  trafficPoints,
  loading,
  ipChecking,
  activeRoute,
  activeRouteLatency,
  captureMode,
  appState,
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
  testActiveRoute,
  captureLabel,
  recoveryStateLabel,
  faultDomainLabel,
  repairActionLabel,
  sourceLabel,
  connectionLabel,
  formatBytes,
  formatRate,
  formatUptime,
  formatTime,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page overview-page">
    <article class="hero-status" :class="`state-${appState.icon}`">
      <div class="hero-copy">
        <StateGlyph :state="appState.icon" size="lg" />
        <div>
          <span class="eyebrow">{{ networkHealthLabel }}</span>
          <h2>{{ connectionLabel() }}</h2>
          <p>{{ sourceLabel(activeRoute?.source_type) }} · {{ activeRoute?.name || "尚未选择节点" }} · {{ captureLabel(captureMode) }}</p>
        </div>
      </div>
      <div class="hero-actions">
        <button class="secondary" :disabled="loading || ipChecking" @click="checkConnection">{{ ipChecking ? "验证中" : "验证 ChatGPT 链路" }}</button>
        <button class="primary" :disabled="loading" @click="toggleConnection">{{ dashboard.core.state === "running" ? "停止代理" : "启动代理" }}</button>
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
  
    <NetworkEnvironmentCard />

    <div class="overview-grid">
      <article class="ip-card">
        <span class="card-label">直连公网 IP</span>
        <strong class="mono">{{ ipDetection?.source.ip || "等待检测" }}</strong>
        <p v-if="ipDetection?.source.error" class="inline-error">{{ ipDetection.source.error }}</p>
        <p>{{ ipDetection?.source.country || "地区未知" }} {{ ipDetection?.source.city }}</p>
        <dl>
          <div><dt>ISP</dt><dd>{{ ipDetection?.source.isp || "暂不可用" }}</dd></div>
          <div><dt>ASN</dt><dd>{{ ipDetection?.source.asn || "暂不可用" }}</dd></div>
        </dl>
        <small>{{ ipDetection?.source.provider || "无检测源" }} · {{ formatTime(ipDetection?.source.checked_at) }}</small>
      </article>
      <article class="ip-card">
        <span class="card-label">代理出口 IP</span>
        <strong class="mono">{{ ipDetection?.proxy.ip || dashboard.ip.proxy_ip || "等待检测" }}</strong>
        <p v-if="ipDetection?.proxy.error" class="inline-error">{{ ipDetection.proxy.error }}</p>
        <p>{{ ipDetection?.proxy.country || dashboard.ip.proxy_country || "地区未知" }} {{ ipDetection?.proxy.city }}</p>
        <dl>
          <div><dt>ISP</dt><dd>{{ ipDetection?.proxy.isp || "暂不可用" }}</dd></div>
          <div><dt>ASN</dt><dd>{{ ipDetection?.proxy.asn || "暂不可用" }}</dd></div>
        </dl>
        <small>{{ ipDetection?.proxy.provider || "无检测源" }} · {{ formatTime(ipDetection?.proxy.checked_at) }}</small>
      </article>
      <article class="risk-card">
        <span class="card-label">连接可用性风险</span>
        <strong :class="`risk-${activeRisk.level}`">{{ activeRisk.label }}</strong>
        <ul><li v-for="reason in activeRisk.reasons" :key="reason">{{ reason }}</li></ul>
        <p v-if="activeRisk.action" class="risk-action">建议：{{ activeRisk.action }}</p>
        <small>{{ dashboard.capture.readiness.default_proxy ? "Windows 默认代理已验证" : captureMode === "tun" ? "TUN 数据面已验证" : "尚无默认应用证据" }} · {{ formatTime(dashboard.capture.readiness.checked_at) }}</small>
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
        <TrafficChart :points="trafficPoints" :visible-series="activeTrafficSeries" :stopped="activeTrafficUnavailable" :status-label="activeTrafficUnavailableReason" compact />
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
