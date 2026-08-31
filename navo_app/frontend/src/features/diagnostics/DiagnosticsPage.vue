<script setup lang="ts">
import NetworkEnvironmentCard from "../environment/NetworkEnvironmentCard.vue";
import { useNavoApplicationContext } from "../application/context";

const {
  page,
  dashboard,
  ipDetection,
  hostStatus,
  benchmark,
  benchmarkRunning,
  ipChecking,
  effectiveConnection,
  directAndProxySame,
  proxyRisk,
  checkIP,
  runBenchmark,
  cancelBenchmark,
  formatBytes,
  formatTime,
  formatDuration,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page ip-page">
    <div class="section-heading page-intro">
      <div><span class="eyebrow">网络诊断</span><h2>主机与代理链路检测</h2><p>双链路出口检测跟随当前有效连接；完整测速只使用 Navo 自有核心，所有检测均不会修改外部代理、TUN 或线路选择。</p></div>
      <button class="secondary" :disabled="ipChecking" @click="checkIP()">{{ ipChecking ? "检测中" : "检测双链路" }}</button>
    </div>

    <NetworkEnvironmentCard />
  
    <div class="diagnostics-grid">
      <article class="host-panel">
        <div class="panel-title"><span class="card-label">主机状态</span><strong>本机运行状态</strong><i aria-hidden="true"></i></div>
        <div class="host-metrics">
          <span><small>平台</small><b>{{ hostStatus ? `${hostStatus.os} / ${hostStatus.arch}` : "读取中" }}</b></span>
          <span><small>逻辑 CPU</small><b>{{ hostStatus?.logical_cpu ?? "—" }}</b></span>
          <span><small>内存占用</small><b>{{ hostStatus ? `${hostStatus.memory_usage_percent.toFixed(0)}%` : "—" }}</b></span>
          <span><small>可用内存</small><b>{{ formatBytes(hostStatus?.memory_available_bytes ?? 0) }}</b></span>
          <span><small>系统运行</small><b>{{ formatDuration(hostStatus?.system_uptime_seconds) }}</b></span>
          <span><small>Navo 运行</small><b>{{ formatDuration(hostStatus?.process_uptime_seconds) }}</b></span>
        </div>
        <div class="host-footer"><span>NAVO {{ hostStatus?.app_version || "—" }}</span><span>{{ hostStatus?.go_version || "runtime unknown" }}</span></div>
      </article>
  
      <article class="benchmark-panel" :aria-busy="benchmarkRunning">
        <div class="panel-title"><span class="card-label">代理测速</span><strong>当前代理实测</strong><i :class="{ pulsing: benchmarkRunning }" aria-hidden="true"></i></div>
        <div class="benchmark-values" aria-live="polite">
          <span><small>延迟</small><b>{{ benchmark ? benchmark.latency_ms.toFixed(1) : "—" }}<em> ms</em></b></span>
          <span><small>抖动</small><b>{{ benchmark ? benchmark.jitter_ms.toFixed(1) : "—" }}<em> ms</em></b></span>
          <span><small>下载</small><b>{{ benchmark ? benchmark.download_mbps.toFixed(2) : "—" }}<em> Mbps</em></b></span>
          <span><small>上传</small><b>{{ benchmark ? benchmark.upload_mbps.toFixed(2) : "—" }}<em> Mbps</em></b></span>
        </div>
        <div v-if="benchmarkRunning" class="benchmark-progress" role="status">
          <span></span><p>正在通过本地代理执行 3 次延迟、4 MiB 下载与 1 MiB 上传测试…</p>
        </div>
        <div v-else class="benchmark-footer">
          <div>
            <strong>{{ benchmark?.proxy_endpoint || `${dashboard.proxy.server}:${dashboard.proxy.port}` }}</strong>
            <small>{{ benchmark ? `${benchmark.test_server} · ${formatTime(benchmark.checked_at)}` : "Cloudflare Edge · 尚未测速" }}</small>
          </div>
          <button class="primary" :disabled="benchmarkRunning" @click="runBenchmark">
            {{ dashboard.core.state === "running" ? "开始完整测速" : "启动本地核心并测速" }}
          </button>
        </div>
        <button v-if="benchmarkRunning" class="danger benchmark-cancel" @click="cancelBenchmark">取消测速</button>
      </article>
    </div>
  
    <div v-if="directAndProxySame" class="warning-banner">两条链路返回相同公网 IP，代理可能未生效。</div>
    <div class="section-heading identity-heading"><div><span class="eyebrow">网络身份</span><h2>双链路 IP 检测</h2><p>直连基线与当前有效出口使用独立 HTTP Transport；外部代理仅只读检测，不会被 Navo 接管。</p></div></div>
    <div class="ip-detail-grid">
      <article v-for="(result, key) in { source: ipDetection?.source, proxy: ipDetection?.proxy }" :key="key">
        <div class="ip-result-heading"><span class="card-label">{{ key === "source" ? "直连基线 IP" : effectiveConnection.kind === "external_system_proxy" ? "外部代理出口 IP" : "Navo 代理出口 IP" }}</span><em :class="`state-${result?.state || 'unavailable'}`">{{ result?.state === "available" ? "可用" : result?.state === "inactive" ? "未启用" : "不可用" }}</em></div>
        <strong class="mono">{{ result?.ip || "尚未检测" }}</strong>
        <p v-if="result?.error" class="inline-error">{{ result.error }}</p>
        <dl>
          <div><dt>国家 / 城市</dt><dd>{{ result?.country || "暂不可用" }} {{ result?.city }}</dd></div>
          <div><dt>ISP</dt><dd>{{ result?.isp || "暂不可用" }}</dd></div>
          <div><dt>ASN</dt><dd>{{ result?.asn || "暂不可用" }}</dd></div>
          <div><dt>网络组织</dt><dd>{{ result?.network || "暂不可用" }}</dd></div>
          <div><dt>检测来源</dt><dd>{{ result?.provider || "暂不可用" }}</dd></div>
          <div><dt>检测时间</dt><dd>{{ formatTime(result?.checked_at) }}</dd></div>
        </dl>
      </article>
    </div>
    <article class="risk-panel">
      <div><span class="card-label">IP 属性提示</span><h3>{{ proxyRisk.label }}</h3><p>仅展示检测来源直接返回的网络属性；它不代表 ChatGPT 可用性，也不推断独享、Fraud 或 Abuse。</p></div>
      <div class="risk-signals">
        <span :class="{ flagged: ipDetection?.proxy.proxy }"><b>Proxy</b>{{ ipDetection ? (ipDetection.proxy.proxy ? "是" : "否") : "未知" }}</span>
        <span :class="{ flagged: ipDetection?.proxy.hosting }"><b>Hosting</b>{{ ipDetection ? (ipDetection.proxy.hosting ? "是" : "否") : "未知" }}</span>
        <span><b>Mobile</b>{{ ipDetection ? (ipDetection.proxy.mobile ? "是" : "否") : "未知" }}</span>
        <span><b>证据来源</b>{{ ipDetection?.proxy.provider || "未知" }} · {{ formatTime(ipDetection?.proxy.checked_at) }}</span>
      </div>
    </article>
  </section>
</template>
