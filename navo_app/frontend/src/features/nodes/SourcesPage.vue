<script setup lang="ts">
import { useNavoApplicationContext } from "../application/context";

const {
  page,
  layeredLatency,
  routeBenchmarks,
  benchmarkRunning,
  latencyBatchRunning,
  routeTestRunning,
  sourceFilter,
  latency,
  activeRoute,
  sourceRoute,
  filteredRoutes,
  cancelBenchmark,
  selectRoute,
  testRoute,
  testFilteredRoutes,
  benchmarkRoute,
  runLayeredLatency,
  deleteUpstream,
  sourceLabel,
  formatTime,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page sources-page">
  		<article class="latency-card" :aria-busy="benchmarkRunning">
  		  <div class="section-heading page-intro">
  			<div><span class="eyebrow">分层延迟</span><h2>当前节点一键测速</h2><p>不切换节点、不修改系统代理或 TUN；远端 DNS 位于核心内部，无法独立观测时明确显示。</p></div>
  			<div class="source-toolbar-actions">
  			  <button v-if="benchmarkRunning" class="danger" @click="cancelBenchmark">停止测速</button>
  			  <button v-else class="primary" :disabled="!activeRoute" @click="runLayeredLatency">{{ layeredLatency ? "重新测速" : "开始测速" }}</button>
  			</div>
  		  </div>
  		  <div class="latency-metrics" aria-live="polite">
  			<span><small>当前节点</small><b>{{ activeRoute?.name || "未选择" }}</b></span>
  			<span><small>TCP 连接</small><b>{{ layeredLatency ? `${layeredLatency.tcp_connect_ms} ms` : "—" }}</b></span>
  			<span><small>代理握手</small><b>{{ layeredLatency ? `${layeredLatency.proxy_handshake_ms} ms` : "—" }}</b></span>
  			<span><small>远端 DNS</small><b>{{ layeredLatency?.dns_observable ? `${layeredLatency.dns_ms} ms` : "核心内不可观测" }}</b></span>
  			<span><small>TLS</small><b>{{ layeredLatency ? `${layeredLatency.tls_ms} ms` : "—" }}</b></span>
  			<span><small>首包 TTFB</small><b>{{ layeredLatency ? `${layeredLatency.ttfb_ms} ms` : "—" }}</b></span>
  			<span><small>完整请求</small><b>{{ layeredLatency ? `${layeredLatency.total_ms} ms` : "—" }}</b></span>
  			<span><small>实际出口</small><b class="mono">{{ layeredLatency?.exit_ip || "—" }}</b></span>
  		  </div>
  		  <p v-if="layeredLatency?.error_message" class="inline-error">{{ layeredLatency.error_code }} · {{ layeredLatency.error_message }}</p>
  		  <small v-if="layeredLatency" class="check-stamp">状态：{{ layeredLatency.state }} · {{ formatTime(layeredLatency.checked_at) }}</small>
  		</article>
    <div class="page-toolbar">
      <div class="source-tabs" role="tablist" aria-label="线路来源">
        <button :class="{ active: sourceFilter === 'airport_subscription' }" @click="sourceFilter = 'airport_subscription'">机场订阅</button>
        <button :class="{ active: sourceFilter === 'upstream_proxy' }" @click="sourceFilter = 'upstream_proxy'">独享代理</button>
      </div>
      <div class="source-toolbar-actions">
        <button class="secondary" :disabled="latencyBatchRunning || !filteredRoutes.length" @click="testFilteredRoutes">{{ latencyBatchRunning ? "批量测试中" : "批量测延迟" }}</button>
        <button class="secondary" :disabled="benchmarkRunning || !sourceRoute" @click="sourceRoute && benchmarkRoute(sourceRoute)">测速当前线路</button>
      </div>
    </div>
  
    <div class="data-panel">
      <div class="table-head"><span>线路</span><span>协议</span><span>地址</span><span>延迟 / 速度</span><span>操作</span></div>
      <div v-if="filteredRoutes.length" class="route-list">
        <div v-for="item in filteredRoutes" :key="item.id" class="route-row" :class="{ active: item.active, candidate: item.candidate }">
          <div><strong>{{ item.name }}</strong><small>{{ item.country || sourceLabel(item.source_type) }}</small></div>
          <span class="protocol">{{ item.type }}</span>
          <span class="mono address">{{ item.server }}:{{ item.port }}</span>
          <span class="route-diagnostics">
            <b>{{ latency[item.id] || "未测试" }}</b>
            <small v-if="routeBenchmarks[item.id]">↓ {{ routeBenchmarks[item.id].download_mbps.toFixed(1) }} / ↑ {{ routeBenchmarks[item.id].upload_mbps.toFixed(1) }} Mbps</small>
          </span>
          <div class="row-actions">
            <button class="secondary compact" :disabled="routeTestRunning[item.id]" @click="testRoute(item)">{{ routeTestRunning[item.id] ? "测试中" : "延迟" }}</button>
            <button class="secondary compact" :disabled="benchmarkRunning" @click="benchmarkRoute(item)">测速</button>
            <button class="primary compact" :disabled="item.active || item.candidate" @click="selectRoute(item)">{{ item.active ? "当前" : item.candidate ? "待验证" : "使用" }}</button>
            <button v-if="item.source_type === 'upstream_proxy'" class="danger compact" @click="deleteUpstream(item)">删除</button>
          </div>
        </div>
      </div>
      <div v-else class="empty-state"><strong>暂无{{ sourceLabel(sourceFilter) }}线路</strong><p>添加来源后，节点会显示在这里。</p></div>
    </div>
  </section>
</template>
