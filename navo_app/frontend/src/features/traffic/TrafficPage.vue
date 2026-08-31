<script setup lang="ts">
import TrafficChart from "../../components/TrafficChart.vue";
import { useNavoApplicationContext } from "../application/context";

const {
  page,
  dashboard,
  trafficPoints,
  simulatedTrafficPoints,
  trafficSimulationSize,
  trafficSimulationDirection,
  trafficTransferRunning,
  metricsAvailable,
  trafficDisplayPoints,
  effectiveConnection,
  activeTrafficSeries,
  trafficContext,
  activeTrafficUnavailable,
  activeTrafficUnavailableReason,
  activeTrafficDownload,
  activeTrafficUpload,
  cancelBenchmark,
  previewSyntheticTraffic,
  runControlledTraffic,
  formatRate,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page traffic-page">
    <div class="section-heading page-intro"><div><span class="eyebrow">实时流量</span><h2>流量与连接趋势</h2><p>指标自动跟随当前接管模式，真实统计与模拟预览始终分开显示。</p></div></div>
    <div class="metric-hero">
      <div><span>{{ trafficContext.label }}下载</span><strong class="download">{{ formatRate(activeTrafficDownload) }}</strong></div>
      <div><span>{{ trafficContext.label }}上传</span><strong class="upload">{{ formatRate(activeTrafficUpload) }}</strong></div>
      <div><span>采样来源</span><strong>{{ trafficContext.source }}</strong></div>
      <div><span>有效连接</span><strong>{{ effectiveConnection.modeLabel }}</strong></div>
      <div><span>活动连接</span><strong>{{ effectiveConnection.controlledByNavo && metricsAvailable ? dashboard.metrics.connections : effectiveConnection.kind === "external_system_proxy" ? "外部只读" : "—" }}</strong></div>
      <div><span>采样窗口</span><strong>{{ trafficPoints.length }} / 30</strong></div>
    </div>
    <article class="chart-card full">
      <div class="section-heading">
        <div><span class="card-label">2 秒采样 · 自动跟随接管模式</span><h2>最近 60 秒{{ trafficContext.label }}</h2></div>
        <span class="traffic-mode-badge">{{ trafficContext.source }}</span>
      </div>
      <p class="traffic-source-note">{{ trafficContext.note }}</p>
  		  <div v-if="simulatedTrafficPoints.length" class="simulation-banner"><strong>纯数据模拟预览</strong><span>不计入真实统计，不代表网络性能。</span><button class="text-button" @click="simulatedTrafficPoints = []">返回真实数据</button></div>
      <TrafficChart :points="trafficDisplayPoints" :visible-series="activeTrafficSeries" :stopped="activeTrafficUnavailable" :status-label="activeTrafficUnavailableReason" :local-metric-label="effectiveConnection.kind === 'external_system_proxy' ? '系统总量' : undefined" />
    </article>
  		<article class="traffic-simulation-card">
  		  <div><span class="card-label">受控流量</span><h3>虚拟文件流量模拟</h3><p>纯数据模式只预览曲线；真实传输模式经当前 Navo 代理发送受控数据，单方向限制 1–32 MiB。</p></div>
  		  <div class="simulation-controls">
  			<label>文件大小（MiB）<input v-model.number="trafficSimulationSize" type="number" min="1" max="32" /></label>
  			<label>方向<select v-model="trafficSimulationDirection"><option value="download">下载</option><option value="upload">上传</option><option value="both">双向</option></select></label>
  			<button class="secondary" @click="previewSyntheticTraffic">纯数据预览</button>
  			<button class="primary" :disabled="trafficTransferRunning || dashboard.core.state !== 'running'" @click="runControlledTraffic">{{ trafficTransferRunning ? "真实传输中" : "执行真实传输" }}</button>
  			<button v-if="trafficTransferRunning" class="danger" @click="cancelBenchmark">取消</button>
  		  </div>
  		</article>
    <div v-if="activeTrafficUnavailable || effectiveConnection.kind === 'external_system_proxy'" class="capability-note">
      <strong>{{ effectiveConnection.kind === "external_system_proxy" ? "外部代理流量口径" : "当前流量口径暂不可用" }}</strong>
      <p v-if="effectiveConnection.kind === 'external_system_proxy'">{{ effectiveConnection.trafficNote }}</p>
      <p v-else-if="trafficContext.metric === 'local'">{{ dashboard.metrics.local_unavailable_reason || "无法读取物理网卡计数器" }}</p>
      <p v-else>{{ dashboard.metrics.unavailable_reason || "当前内核没有启用 Metrics Adapter" }}</p>
      <p>Navo 不会以系统总流量推算外部代理专属流量，也不会使用模拟数据冒充真实统计。</p>
    </div>
  </section>
</template>
