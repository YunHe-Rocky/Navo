<script setup lang="ts">
import { useNavoApplicationContext } from "../application/context";

const {
  page,
  dashboard,
  coreUpdateReport,
  coreUpdateChecking,
  coreUpdateInstalling,
  loading,
  coreUpdates,
  checkCoreUpdates,
  openCoreRelease,
  installCoreUpdate,
  setCore,
  formatTime,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page cores-page">
    <div class="section-heading page-intro">
      <div><span class="eyebrow">内核升级</span><h2>管理代理内核</h2><p>分别检查三个内核版本与当前文件完整性；没有受信 SHA-256 的远程资产不会安装。</p></div>
      <button class="primary" :disabled="coreUpdateChecking" @click="checkCoreUpdates">
        {{ coreUpdateChecking ? "正在检查" : "检查内核升级" }}
      </button>
    </div>
    <div class="core-grid">
      <article v-for="item in dashboard.cores" :key="item.id" :class="{ active: item.active }">
        <div class="core-heading">
          <span class="core-symbol">{{ item.id.slice(0, 2).toUpperCase() }}</span>
          <span
            :class="['availability', {
              healthy: item.installed && (!coreUpdates[item.id] || coreUpdates[item.id].integrity_ok),
              update: coreUpdates[item.id]?.update_available,
            }]"
          >
            {{ !item.installed ? "未安装" : coreUpdates[item.id]?.update_available ? "发现升级" : coreUpdates[item.id] && !coreUpdates[item.id].integrity_ok ? "校验异常" : "已安装" }}
          </span>
        </div>
        <h3>{{ item.name || item.id }}</h3>
        <dl>
          <div class="core-current-version"><dt>当前版本</dt><dd class="mono">{{ item.version || "未识别" }}</dd></div>
          <div><dt>官方最新版本</dt><dd class="mono">{{ coreUpdates[item.id]?.latest_version || "尚未检查" }}</dd></div>
            <div><dt>接管能力</dt><dd>系统代理 {{ item.system_proxy_supported === false ? "不支持" : "支持" }} · TUN {{ item.tun_supported === false ? "不支持" : "支持" }}</dd></div>
          <div><dt>运行状态</dt><dd>{{ item.active ? dashboard.core.state : "未启用" }}</dd></div>
          <div><dt>文件完整性</dt><dd>{{ coreUpdates[item.id] ? (coreUpdates[item.id].integrity_ok ? "SHA-256 通过" : "校验失败") : "等待检查" }}</dd></div>
          <div><dt>实时指标</dt><dd>{{ item.id === "xray" ? "Stats API 未启用" : "真实流量与连接数" }}</dd></div>
        </dl>
        <p v-if="coreUpdates[item.id]?.error" class="inline-error">{{ coreUpdates[item.id].error }}</p>
  			<p v-if="coreUpdates[item.id]?.update_available && !coreUpdates[item.id]?.install_supported" class="capability-note">{{ coreUpdates[item.id].install_blocked_reason }}</p>
        <div class="core-actions">
          <button class="secondary" :disabled="!item.installed || item.active || loading" @click="setCore(item.id)">{{ item.active ? "当前内核" : "设为当前" }}</button>
  			  <button
  				v-if="coreUpdates[item.id]?.update_available && coreUpdates[item.id]?.install_supported"
  				class="primary"
  				:disabled="coreUpdateInstalling[item.id]"
  				@click="installCoreUpdate(item.id)"
  			  >{{ coreUpdateInstalling[item.id] ? "正在升级" : "立即升级" }}</button>
  			  <button class="text-button" @click="openCoreRelease(item.id)">官方发布页</button>
        </div>
      </article>
    </div>
    <p v-if="coreUpdateReport" class="check-stamp">最后检查：{{ formatTime(coreUpdateReport.checked_at) }} · 安装包签名边界保持不变，Navo 不执行后台静默升级。</p>
    <article v-if="dashboard.core.last_error" class="diagnostic-card"><strong>最近启动错误</strong><code>{{ dashboard.core.last_error }}</code></article>
  </section>
</template>
