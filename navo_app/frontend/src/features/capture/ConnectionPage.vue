<script setup lang="ts">
import { useNavoApplicationContext } from "../application/context";
import type { CaptureMode, RoutingListMode, RuntimeMode } from "../../types";

const {
  page,
  dashboard,
  subscriptions,
  loading,
  sourceFilter,
  showUpstreamForm,
  showSubscriptionForm,
  showAdvancedCore,
  routingRuleDrafts,
  routingRuleDirty,
  routingListEditor,
  routingListTextarea,
  upstream,
  subscription,
  sourceRoute,
  activeCore,
  activeCoreSupportsTUN,
  captureMode,
  captureRouteMissing,
  runtimeMode,
  routingListMode,
  routingRuleCounts,
  filteredRoutes,
  captureTransitioning,
  toggleConnection,
  setCapture,
  setRuntimeMode,
  setRoutingListMode,
  activateRoutingList,
  saveRoutingRules,
  clearRoutingRules,
  setCore,
  selectRoute,
  createUpstream,
  addSubscription,
  removeSubscription,
  refreshAllSubscriptions,
  captureLabel,
  runtimeModeLabel,
  routingListModeLabel,
  capturePhaseLabel,
  sourceLabel,
} = useNavoApplicationContext();
</script>

<template>
  <section class="page-content task-page connection-page">
    <div class="section-heading page-intro connection-page-intro"><div><span class="eyebrow">连接方案</span><h2>配置本次连接</h2><p>先选择线路，再分别设置接管方式与流量走向；两者相互独立。</p></div></div>
    <div class="connection-setup-grid">
    <article class="connection-source-scope">
      <div>
        <span class="card-label">全局线路类型</span>
        <strong>{{ sourceLabel(sourceFilter) }}</strong>
        <small>节点选择、来源管理和连接确认均只显示当前线路类型。</small>
      </div>
      <div class="source-tabs" role="tablist" aria-label="连接管理全局线路类型">
        <button :class="{ active: sourceFilter === 'airport_subscription' }" @click="sourceFilter = 'airport_subscription'">机场订阅</button>
        <button :class="{ active: sourceFilter === 'upstream_proxy' }" @click="sourceFilter = 'upstream_proxy'">独享代理</button>
      </div>
    </article>
      <article class="config-card route-config-card">
        <span class="card-label">{{ sourceLabel(sourceFilter) }}节点</span>
        <label class="field-label" for="route-select">节点选择</label>
        <select id="route-select" :value="sourceRoute?.id || ''" @change="selectRoute(filteredRoutes.find((item) => item.id === ($event.target as HTMLSelectElement).value)!)">
          <option value="" disabled>请选择可用节点</option>
          <option v-for="item in filteredRoutes" :key="item.id" :value="item.id">{{ item.name }} · {{ item.type }}</option>
        </select>
        <p v-if="!filteredRoutes.length" class="capability-note">当前没有{{ sourceLabel(sourceFilter) }}节点，请先在下方添加来源。</p>
      </article>
    </div>
      <article class="config-card traffic-control-card" :aria-busy="loading || captureTransitioning">
        <div class="traffic-control-heading">
          <div>
            <span class="card-label">流量控制</span>
            <strong>{{ captureLabel(captureMode) }} · {{ runtimeModeLabel(runtimeMode) }} · {{ routingListModeLabel(routingListMode) }}</strong>
            <small>接管方式与流量策略相互独立。黑名单内走代理、白名单内走直连、私网始终直连；其他流量服从下方基础策略。</small>
          </div>
          <div class="traffic-control-actions">
            <button v-if="routingListMode !== 'off'" class="secondary compact" :disabled="loading || captureTransitioning" @click="setRoutingListMode('off')">关闭名单</button>
            <button v-if="captureMode !== 'off'" class="secondary compact" :disabled="loading || captureTransitioning" @click="setCapture('off')">停止接管</button>
          </div>
        </div>
  
        <section class="control-axis mode-entry-axis" aria-labelledby="capture-axis-title">
          <div class="control-axis-copy">
            <span class="axis-index">01</span>
            <div><strong id="capture-axis-title">运行入口</strong><small :class="{ faulted: dashboard.capture.state === 'faulted' }">{{ capturePhaseLabel(dashboard.capture.phase) }} · {{ routingListModeLabel(routingListMode) }}</small></div>
          </div>
          <div class="capture-options mode-entry-options" role="group" aria-label="系统代理、TUN 与黑白名单">
            <button v-for="mode in (['system_proxy', 'tun'] as CaptureMode[])" :key="mode" :class="{ selected: captureMode === mode }" :aria-pressed="captureMode === mode" :disabled="loading || captureTransitioning || captureRouteMissing || (mode === 'tun' && (!dashboard.tun.installed || !activeCoreSupportsTUN))" @click="setCapture(mode)">
              <strong>{{ captureLabel(mode) }}</strong>
              <small>{{ mode === "system_proxy" ? "覆盖浏览器和遵循 Windows 代理的应用" : !dashboard.tun.installed ? "TUN 组件不可用" : !activeCoreSupportsTUN ? `${activeCore?.name || dashboard.core.core_id} 不支持 TUN` : "覆盖浏览器及不读取系统代理的应用流量" }}</small>
            </button>
            <button v-for="mode in (['blacklist', 'whitelist'] as Exclude<RoutingListMode, 'off'>[])" :key="mode" :class="{ selected: routingListMode === mode }" :aria-pressed="routingListMode === mode" :aria-expanded="routingListEditor === mode" aria-controls="routing-list-editor" :disabled="loading || captureTransitioning" @click="activateRoutingList(mode)">
              <strong>{{ mode === "blacklist" ? "黑名单" : "白名单" }}</strong>
              <small>{{ mode === "blacklist" ? "点击启用并打开代理规则编辑器" : "点击启用并打开直连规则编辑器" }}</small>
            </button>
          </div>
          <p v-if="captureRouteMissing" class="capability-note">当前流量策略需要代理线路。请先添加并选择可用节点；Navo 不会借用其他代理软件的端口。</p>
        </section>
  
        <section class="control-axis" aria-labelledby="routing-axis-title">
          <div class="control-axis-copy">
            <span class="axis-index">02</span>
            <div><strong id="routing-axis-title">流量走向</strong><small>{{ runtimeModeLabel(runtimeMode) }}</small></div>
          </div>
          <div class="capture-options routing-policy-options" role="group" aria-label="流量走向">
            <button v-for="mode in (['bypass_mainland', 'global', 'direct'] as RuntimeMode[])" :key="mode" :class="{ selected: runtimeMode === mode }" :aria-pressed="runtimeMode === mode" :disabled="loading || captureTransitioning" @click="setRuntimeMode(mode)">
              <strong>{{ runtimeModeLabel(mode) }}</strong>
              <small>{{ mode === "bypass_mainland" ? "大陆与局域网直连，其余走节点" : mode === "global" ? "必要旁路外全部走节点" : "全部流量直连" }}</small>
            </button>
          </div>
        </section>
  
        <section v-if="routingListEditor" id="routing-list-editor" class="routing-rule-editor" aria-labelledby="routing-list-title">
          <div class="routing-rule-heading">
            <div>
              <span class="axis-index">03</span>
              <div><strong id="routing-list-title">{{ routingListEditor === "blacklist" ? "代理黑名单" : "直连白名单" }}</strong><small>已启用并自动打开 · 支持域名后缀和 CIDR</small></div>
            </div>
          </div>
          <div class="routing-list-grid single">
            <div v-if="routingListEditor === 'blacklist'">
              <div class="routing-list-label"><label class="field-label" for="blacklist-rules">代理黑名单 · {{ routingRuleCounts.blacklist }} 条</label><button class="text-button" type="button" :disabled="loading || !routingRuleCounts.blacklist" @click="clearRoutingRules('blacklist')">清空</button></div>
              <textarea id="blacklist-rules" ref="routingListTextarea" v-model="routingRuleDrafts.blacklist" rows="5" spellcheck="false" placeholder="chatgpt.com&#10;openai.com" @input="routingRuleDirty.blacklist = true" />
              <small>命中后强制走当前节点。</small>
            </div>
            <div v-else>
              <div class="routing-list-label"><label class="field-label" for="whitelist-rules">直连白名单 · {{ routingRuleCounts.whitelist }} 条</label><button class="text-button" type="button" :disabled="loading || !routingRuleCounts.whitelist" @click="clearRoutingRules('whitelist')">清空</button></div>
              <textarea id="whitelist-rules" ref="routingListTextarea" v-model="routingRuleDrafts.whitelist" rows="5" spellcheck="false" placeholder="example.cn&#10;192.168.0.0/16" @input="routingRuleDirty.whitelist = true" />
              <small>命中目标强制直连。</small>
            </div>
          </div>
          <div class="routing-rule-actions">
            <small>当前：{{ routingListModeLabel(routingListMode) }}。关闭名单后内容仍会保留，但规则不会参与路由。</small>
            <button class="primary compact" :disabled="loading || captureTransitioning || (!routingRuleDirty.blacklist && !routingRuleDirty.whitelist)" @click="saveRoutingRules">保存名单</button>
          </div>
        </section>
      </article>
    <article class="source-management-card">
      <div>
        <span class="card-label">线路来源管理</span>
        <strong>{{ sourceFilter === "airport_subscription" ? "机场订阅" : "独享代理" }}</strong>
        <small>{{ sourceFilter === "airport_subscription" ? `${subscriptions.length} 个订阅 · ${filteredRoutes.length} 个节点` : `${filteredRoutes.length} 条独享线路` }}</small>
      </div>
      <div class="source-toolbar-actions">
        <button v-if="sourceFilter === 'airport_subscription' && subscriptions.length" class="secondary" :disabled="loading" @click="refreshAllSubscriptions">同步全部订阅</button>
        <button class="primary" @click="sourceFilter === 'upstream_proxy' ? showUpstreamForm = true : showSubscriptionForm = true">{{ sourceFilter === "upstream_proxy" ? "添加独享代理" : "添加机场订阅" }}</button>
      </div>
    </article>
  
    <form v-if="showUpstreamForm" class="form-card" @submit.prevent="createUpstream">
      <div class="form-heading"><div><span class="eyebrow">独享代理</span><h2>添加独享代理</h2></div><button type="button" class="icon-button" aria-label="关闭" @click="showUpstreamForm = false">×</button></div>
      <div class="form-grid">
        <label>名称<span>*</span><input v-model.trim="upstream.name" required /></label>
        <label>协议<span>*</span><select v-model="upstream.proto"><option value="http">HTTP</option><option value="https">HTTPS</option><option value="socks5">SOCKS5</option></select></label>
        <label class="wide">服务器<span>*</span><input v-model.trim="upstream.server" required placeholder="proxy.example.com" /></label>
        <label>端口<span>*</span><input v-model.number="upstream.port" required type="number" min="1" max="65535" /></label>
        <label>UDP 策略<select v-model="upstream.udp_policy" :disabled="upstream.proto !== 'socks5'"><option value="disabled">禁用</option><option value="prefer">优先</option><option value="require">必须支持</option></select></label>
        <label>用户名<input v-model="upstream.username" autocomplete="username" /></label>
        <label>密码<input v-model="upstream.password" type="password" autocomplete="new-password" /></label>
      </div>
      <div class="form-actions"><button type="button" class="secondary" @click="showUpstreamForm = false">取消</button><button class="primary" :disabled="loading">保存并启用</button></div>
    </form>
  
    <form v-if="showSubscriptionForm" class="form-card" @submit.prevent="addSubscription">
      <div class="form-heading"><div><span class="eyebrow">机场订阅</span><h2>添加机场订阅</h2></div><button type="button" class="icon-button" aria-label="关闭" @click="showSubscriptionForm = false">×</button></div>
      <div class="form-grid">
        <label>订阅名称<span>*</span><input v-model.trim="subscription.name" required /></label>
        <label class="wide">订阅地址<span>*</span><input v-model.trim="subscription.url" required type="url" autocomplete="off" /></label>
        <label class="checkbox"><input v-model="subscription.skip_tls_verify" type="checkbox" />仅在证书错误且确认来源可信时跳过 TLS 校验</label>
      </div>
      <div class="form-actions"><button type="button" class="secondary" @click="showSubscriptionForm = false">取消</button><button class="primary" :disabled="loading">保存并刷新</button></div>
    </form>
  
    <div v-if="sourceFilter === 'airport_subscription' && subscriptions.length" class="subscription-summary">
      <article v-for="item in subscriptions" :key="item.id">
        <div><strong>{{ item.name }}</strong><span :class="{ healthy: !item.last_error }">{{ item.last_error ? "同步异常" : "可用" }}</span></div>
        <p>{{ item.node_count }} 个节点 · {{ item.enabled ? "已启用" : "已停用" }}</p>
        <small v-if="item.last_error">{{ item.last_error }}</small>
        <button class="danger compact" @click="removeSubscription(item)">删除</button>
      </article>
    </div>
    <article class="advanced-card">
      <button class="advanced-toggle" :aria-expanded="showAdvancedCore" @click="showAdvancedCore = !showAdvancedCore">
        <span><strong>高级设置</strong><small>默认自动使用当前兼容内核；仅在排障时手动指定。</small></span>
        <span>{{ showAdvancedCore ? "收起" : "展开" }}</span>
      </button>
      <div v-if="showAdvancedCore" class="core-options">
        <button v-for="item in dashboard.cores" :key="item.id" :class="{ selected: item.active }" :disabled="!item.installed || loading" @click="setCore(item.id)">
          <strong>{{ item.name || item.id }}</strong><small>{{ item.version || "版本未知" }} · {{ item.installed ? "可用" : "未安装" }}</small>
        </button>
      </div>
    </article>
    <div class="connection-cta">
      <div class="connection-summary">
        <span class="source-constraint">{{ sourceLabel(sourceFilter) }}</span>
        <strong>{{ sourceRoute?.name || `尚未选择${sourceLabel(sourceFilter)}节点` }}</strong>
        <span>{{ captureLabel(captureMode) }} · {{ dashboard.core.core_id }} · 仅使用当前来源类型</span>
      </div>
      <button class="primary" :disabled="loading || captureTransitioning || (captureMode === 'off' && !sourceRoute)" @click="toggleConnection">{{ captureMode !== "off" ? "断开连接" : "连接" }}</button>
    </div>
  </section>
</template>
