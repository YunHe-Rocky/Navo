<script setup lang="ts">
import { useNavoApplicationContext } from "../application/context";
import { formatLogEvidence } from "../logs/presenter";
import { logCategoryLabel } from "../logs/categories";

const {
  page,
  logs,
  logMetadata,
  selectedLogLevels,
  selectedLogCategories,
  selectedLogServices,
  logFrom,
  logTo,
  logHasMore,
  logFollow,
  hostStatus,
  startupSettings,
  captureRouteMissing,
  loading,
  changelogText,
  refreshLogs,
  loadMoreLogs,
  toggleLogSelection,
  clearVisibleLogs,
  clearPersistedLogs,
  setLogFollow,
  configureStartup,
  goToRouteSelection,
  formatDuration,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page settings-page">
    <div class="section-heading page-intro"><div><span class="eyebrow">系统与诊断</span><h2>设置与诊断</h2><p>运行环境、更新日志与诊断日志集中在这里；界面形态由顶部状态栏切换。</p></div></div>
  
    <article class="settings-runtime">
      <div><span class="card-label">运行环境</span><strong>Navo {{ hostStatus?.app_version || "—" }}</strong><small>{{ hostStatus ? `${hostStatus.os} / ${hostStatus.arch} · ${hostStatus.go_version}` : "正在读取运行环境" }}</small></div>
      <div><span>系统运行</span><b>{{ formatDuration(hostStatus?.system_uptime_seconds) }}</b></div>
      <div><span>Navo 运行</span><b>{{ formatDuration(hostStatus?.process_uptime_seconds) }}</b></div>
      <div><span>内存占用</span><b>{{ hostStatus ? `${hostStatus.memory_usage_percent.toFixed(0)}%` : "—" }}</b></div>
    </article>

    <article class="config-card startup-settings-card">
      <div class="startup-settings-copy">
        <span class="card-label">登录启动</span>
        <strong>开机连接</strong>
        <small>登录 Windows 后延迟 15 秒静默启动，并在健康验证通过后才接管流量。</small>
      </div>
      <div class="startup-settings-controls">
        <label class="checkbox startup-toggle">
          <input
            type="checkbox"
            :checked="startupSettings?.enabled || false"
            :disabled="loading || !startupSettings?.supported || (captureRouteMissing && !startupSettings?.enabled)"
            aria-describedby="startup-route-status"
            @change="configureStartup(($event.target as HTMLInputElement).checked, startupSettings?.mode || 'system_proxy')"
          />
          启用开机连接
        </label>
        <label class="startup-mode-field">
          接管方式
          <select
            :value="startupSettings?.mode || 'system_proxy'"
            :disabled="loading || !startupSettings?.supported || (captureRouteMissing && startupSettings?.enabled)"
            @change="configureStartup(startupSettings?.enabled || false, ($event.target as HTMLSelectElement).value as 'system_proxy' | 'tun')"
          >
            <option value="system_proxy">系统代理</option>
            <option value="tun">TUN</option>
          </select>
        </label>
      </div>
      <div v-if="captureRouteMissing" id="startup-route-status" class="route-required-callout" role="status">
        <div><strong>需要先配置 Navo 线路</strong><small>V2 外部代理只用于状态观测，不会被 Navo 当成开机连接线路。</small></div>
        <button class="secondary" @click="goToRouteSelection">前往连接管理</button>
      </div>
      <small v-if="!startupSettings" class="startup-settings-status">正在读取开机连接状态…</small>
      <small v-else-if="!startupSettings.supported" class="startup-settings-status faulted">当前环境不支持登录启动。</small>
      <small v-else-if="startupSettings.last_error" class="startup-settings-status faulted">登录任务异常：{{ startupSettings.last_error }}</small>
      <small v-else class="startup-settings-status">{{ startupSettings.enabled && startupSettings.registered ? "登录任务已注册" : "当前不会在登录后自动连接" }}</small>
    </article>
  
    <article class="changelog-card">
      <div class="section-heading"><div><span class="eyebrow">版本记录</span><h2>更新日志</h2><p>按版本日期记录实际功能调整。</p></div></div>
      <pre class="changelog-text">{{ changelogText }}</pre>
    </article>
  
    <article class="settings-log-card">
      <div class="section-heading">
        <div><span class="eyebrow">诊断日志</span><h2>结构化诊断日志</h2><p>日志按严重级别和服务分级保存；打开时默认展示基础服务，敏感字段写入前脱敏。</p></div>
  			<div class="log-actions">
  				<button class="secondary" :disabled="loading" @click="refreshLogs">查询</button>
  				<button class="secondary" @click="clearVisibleLogs">清空当前显示</button>
  				<button class="danger" @click="clearPersistedLogs">清空持久化日志</button>
  			</div>
      </div>
  			<div class="log-filters">
				<fieldset><legend>日志级别</legend><label v-for="level in logMetadata.levels" :key="level"><input type="checkbox" :checked="selectedLogLevels.includes(level)" @change="toggleLogSelection('level', level, ($event.target as HTMLInputElement).checked)" />{{ level }}</label></fieldset>
				<fieldset><legend>服务分级</legend><label v-for="category in logMetadata.categories" :key="category"><input type="checkbox" :checked="selectedLogCategories.includes(category)" @change="toggleLogSelection('category', category, ($event.target as HTMLInputElement).checked)" />{{ logCategoryLabel(category) }}</label><small>默认基础服务；全部取消表示不限分级</small></fieldset>
				<fieldset><legend>具体服务</legend><label v-for="service in logMetadata.services" :key="service"><input type="checkbox" :checked="selectedLogServices.includes(service)" @change="toggleLogSelection('service', service, ($event.target as HTMLInputElement).checked)" />{{ service }}</label><small v-if="!logMetadata.services.length">尚无结构化事件</small></fieldset>
  				<label>起始时间<input v-model="logFrom" type="datetime-local" /></label>
  				<label>截止时间<input v-model="logTo" type="datetime-local" /></label>
  				<label class="log-follow"><input type="checkbox" :checked="logFollow" @change="setLogFollow(($event.target as HTMLInputElement).checked)" />实时跟随</label>
  			</div>
  			<div class="log-view structured">
  				<div v-if="!logs.length" class="empty-state"><strong>暂无日志</strong><p>当前筛选条件没有结构化事件。</p></div>
				<div v-for="entry in logs" :key="entry.id" v-memo="[entry.id, entry.category]" :class="`level-${entry.level.toLowerCase()}`">
					<span>{{ new Date(entry.timestamp).toLocaleString() }}</span><b>{{ entry.level }}</b><i :title="`${logCategoryLabel(entry.category)} · ${entry.service}`">{{ logCategoryLabel(entry.category) }} · {{ entry.service }}<template v-if="entry.component"> / {{ entry.component }}</template></i><code>{{ entry.message }}</code>
					<small v-if="formatLogEvidence(entry)" class="log-evidence">{{ formatLogEvidence(entry) }}</small>
  				</div>
  			</div>
  			<button v-if="logHasMore" class="secondary log-more" @click="loadMoreLogs">加载下一页</button>
    </article>
  </section>
</template>
