<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { api } from "./api";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { CLOSE_PREFERENCE_KEY, createClosePreference, resolveClosePreference } from "./close-preference.js";
import StateGlyph from "./components/StateGlyph.vue";
import TrafficChart from "./components/TrafficChart.vue";
import { TrafficRingBuffer, captureModeOf, deriveAppState, riskSummary } from "./state";
import { generateSyntheticTraffic, seriesForCaptureMode, trafficContextForCaptureMode } from "./traffic.js";
import type {
  CaptureMode,
  CoreUpdateReport,
  Dashboard,
  HostStatus,
  IPDetection,
	LatencyResult,
	LogEntry,
	LogMetadata,
  Page,
  ProxyBenchmark,
  RouteInfo,
  RuntimeMode,
  RoutingListMode,
  SourceType,
  SubscriptionInfo,
  TrafficPoint,
  TrafficSeries,
  UpstreamRequest,
} from "./types";

const emptyDashboard: Dashboard = {
  core: { core_id: "sing-box", state: "stopped", pid: 0, uptime_seconds: 0, config_hash: "", restart_count: 0, last_error: "" },
  cores: [],
  proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
  runtime: { mode: "bypass_mainland", list_mode: "off", active_id: "", tun_enabled: false, blacklist: [], whitelist: [] },
  tun: {
    installed: false, created: false, enabled: false, name: "Navo", mtu: 1500,
    state: "missing", identifier: "", interface_index: 0, fault_id: "", last_error: "",
  },
  capture: {
    state: "stopped", phase: "stopped", desired_mode: "off", committed_mode: "off",
    transition_id: "", fault_id: "", last_error: "", can_retry_tun: false,
  },
  metrics: {
    reachable: false, available: false, unavailable_reason: "", core_name: "", latency_ms: 0,
    upload_bytes: 0, download_bytes: 0, connections: 0,
    local_available: false, local_unavailable_reason: "",
    local_upload_bps: 0, local_download_bps: 0, proxy_upload_bps: 0, proxy_download_bps: 0,
    local_upload_total: 0, local_download_total: 0, proxy_upload_total: 0, proxy_download_total: 0,
    traffic_source_state: "unavailable", traffic_sampled_at: "",
  },
  ip: { proxy_ip: "", proxy_country: "", direct_ip: "" },
};

type NavigationGroup = "核心操作" | "监测诊断" | "系统管理";
type NavigationItem = { id: Page; label: string; description: string; group: NavigationGroup; icon: string };

const navigation: NavigationItem[] = [
  { id: "overview", label: "运行概览", description: "查看连接、出口与实时状态", group: "核心操作", icon: "M4 13h6V4H4v9Zm0 7h6v-5H4v5Zm10 0h6v-9h-6v9Zm0-16v5h6V4h-6Z" },
  { id: "connection", label: "连接管理", description: "选择线路并配置流量接管", group: "核心操作", icon: "M7 12h10M12 7l5 5-5 5M5 5v14" },
  { id: "traffic", label: "流量监控", description: "分析速率、用量与连接趋势", group: "监测诊断", icon: "M4 18V9m5 9V5m5 13v-7m5 7V7" },
  { id: "sources", label: "网络测速", description: "测试延迟、吞吐与线路质量", group: "监测诊断", icon: "M6 7h12M6 12h12M6 17h12M3 7h.01M3 12h.01M3 17h.01" },
  { id: "ip", label: "网络检测", description: "核对直连与代理出口风险", group: "监测诊断", icon: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0 0c2.5-2.4 4-5.4 4-9s-1.5-6.6-4-9c-2.5 2.4-4 5.4-4 9s1.5 6.6 4 9ZM3 12h18" },
  { id: "cores", label: "内核管理", description: "检查并升级代理内核", group: "系统管理", icon: "M8 8h8v8H8zM4 10h4m8 0h4M4 14h4m8 0h4M10 4v4m4-4v4m-4 8v4m4-4v4" },
  { id: "settings", label: "设置与日志", description: "管理运行参数与诊断日志", group: "系统管理", icon: "M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Zm7-3.5 2-1-2-3-2.2.5L15 6l.4-2.3h-3.5L11 6 8.2 8.5 6 8 4 11l2 1-2 1 2 3 2.2-.5L11 18l.9 2.3h3.5L15 18l1.8-2.5L19 16l2-3-2-1Z" },
];
const navigationGroups: NavigationGroup[] = ["核心操作", "监测诊断", "系统管理"];

type ThemeMode = "day" | "night";
type CloseAction = "minimize" | "exit";

const page = ref<Page>("overview");
const theme = ref<ThemeMode>("night");
const dashboard = ref<Dashboard>(emptyDashboard);
const routes = ref<RouteInfo[]>([]);
const subscriptions = ref<SubscriptionInfo[]>([]);
const logs = ref<LogEntry[]>([]);
const logMetadata = ref<LogMetadata>({ levels: ["DEBUG", "INFO", "WARN", "ERROR"], services: [] });
const selectedLogLevels = ref<string[]>(["INFO", "WARN", "ERROR"]);
const selectedLogServices = ref<string[]>([]);
const logFrom = ref("");
const logTo = ref("");
const logCursor = ref(0);
const logHasMore = ref(false);
const logFollow = ref(false);
const ipDetection = ref<IPDetection>();
const hostStatus = ref<HostStatus>();
const benchmark = ref<ProxyBenchmark>();
const layeredLatency = ref<LatencyResult>();
const routeBenchmarks = ref<Record<string, ProxyBenchmark>>({});
const benchmarkRunning = ref(false);
const latencyBatchRunning = ref(false);
const routeTestRunning = ref<Record<string, boolean>>({});
const coreUpdateReport = ref<CoreUpdateReport>();
const coreUpdateChecking = ref(false);
const coreUpdateInstalling = ref<Record<string, boolean>>({});
const trafficPoints = ref<TrafficPoint[]>([]);
const simulatedTrafficPoints = ref<TrafficPoint[]>([]);
const trafficSimulationSize = ref(8);
const trafficSimulationDirection = ref<"download" | "upload" | "both">("download");
const trafficTransferRunning = ref(false);
const loading = ref(false);
const metricsAvailable = ref(false);
const ipChecking = ref(false);
const notice = ref("");
const failure = ref("");
const activityVisible = ref(false);
const activityLabel = ref("");
const activityProgress = ref(0);
const dismissedFaultID = ref("");
const tunRetryButton = ref<HTMLButtonElement>();
const sourceFilter = ref<SourceType>("airport_subscription");
const latency = ref<Record<string, string>>({});
const showUpstreamForm = ref(false);
const showSubscriptionForm = ref(false);
const showAdvancedCore = ref(false);
const healthFailures = ref(0);
const healthSuccesses = ref(0);
const routingRuleDrafts = reactive({ blacklist: "", whitelist: "" });
const routingRuleDirty = reactive({ blacklist: false, whitelist: false });
const routingListEditor = ref<Exclude<RoutingListMode, "off"> | null>(null);
const routingListTextarea = ref<HTMLTextAreaElement>();
const closeDialogOpen = ref(false);
const closeAction = ref<CloseAction>("minimize");
const rememberCloseAction = ref(false);
const closeActionBusy = ref(false);
const closeActionError = ref("");
const closePrimaryButton = ref<HTMLButtonElement>();
const ring = new TrafficRingBuffer(30);

const trafficDisplayPoints = computed(() => simulatedTrafficPoints.value.length ? simulatedTrafficPoints.value : trafficPoints.value);
let metricsTimer: ReturnType<typeof setInterval> | undefined;
let captureTimer: ReturnType<typeof setInterval> | undefined;
let activityTimer: ReturnType<typeof setInterval> | undefined;
let activityHideTimer: ReturnType<typeof setTimeout> | undefined;
let logTimer: ReturnType<typeof setInterval> | undefined;
let previousRouteID = "";
let stopCloseRequestListener: (() => void) | undefined;

const upstream = ref<UpstreamRequest>({
  name: "", proto: "socks5", server: "", port: 1080,
  username: "", password: "", udp_policy: "disabled",
});
const subscription = ref({ name: "", url: "", skip_tls_verify: false });

const activeRoute = computed(() => routes.value.find((item) => item.id === dashboard.value.runtime.active_id || item.active));
const sourceRoute = computed(() => activeRoute.value?.source_type === sourceFilter.value ? activeRoute.value : undefined);
const activeCore = computed(() => dashboard.value.cores.find((item) => item.active || item.id === dashboard.value.core.core_id));
const activeCoreSupportsTUN = computed(() => activeCore.value?.tun_supported !== false);
const activeRouteLatency = computed(() => {
  const route = activeRoute.value;
  if (!route) return "无活动线路";
  return latency.value[route.id] || (dashboard.value.metrics.latency_ms ? `${dashboard.value.metrics.latency_ms} ms` : "尚未测试");
});
const captureMode = computed(() => captureModeOf(dashboard.value));
const runtimeMode = computed(() => dashboard.value.runtime.mode);
const routingListMode = computed(() => dashboard.value.runtime.list_mode ?? "off");
const routingRuleCounts = computed(() => ({
  blacklist: parseRoutingRuleDraft(routingRuleDrafts.blacklist).length,
  whitelist: parseRoutingRuleDraft(routingRuleDrafts.whitelist).length,
}));
const appState = computed(() => deriveAppState(dashboard.value, activeRoute.value, healthFailures.value, healthSuccesses.value));
const connected = computed(() => appState.value.connection === "connected");
const pageTitle = computed(() => navigation.find((item) => item.id === page.value)?.label ?? "Navo");
const currentNavigation = computed(() => navigation.find((item) => item.id === page.value));
const networkHealthLabel = computed(() => {
  switch (appState.value.networkHealth) {
    case "healthy": return "网络状态正常";
    case "degraded": return "网络状态不稳定";
    case "unavailable": return "网络暂不可用";
    case "checking": return "正在检查网络";
    default: return "网络状态未知";
  }
});
const filteredRoutes = computed(() => routes.value.filter((item) => item.source_type === sourceFilter.value));
const latestTraffic = computed(() => trafficPoints.value.at(-1));
const activeTrafficSeries = computed(() => seriesForCaptureMode(captureMode.value) as TrafficSeries[]);
const trafficContext = computed(() => trafficContextForCaptureMode(captureMode.value));
const activeTrafficUnavailable = computed(() => captureMode.value === "off"
  ? !dashboard.value.metrics.local_available
  : !dashboard.value.metrics.available);
const activeTrafficUnavailableReason = computed(() => captureMode.value === "off"
  ? dashboard.value.metrics.local_unavailable_reason
  : dashboard.value.metrics.unavailable_reason);
const activeTrafficDownload = computed(() => captureMode.value === "off"
  ? latestTraffic.value?.localDownloadBps
  : latestTraffic.value?.proxyDownloadBps);
const activeTrafficUpload = computed(() => captureMode.value === "off"
  ? latestTraffic.value?.localUploadBps
  : latestTraffic.value?.proxyUploadBps);
const activeTrafficDownloadTotal = computed(() => captureMode.value === "off"
  ? dashboard.value.metrics.local_download_total
  : dashboard.value.metrics.proxy_download_total);
const activeTrafficUploadTotal = computed(() => captureMode.value === "off"
  ? dashboard.value.metrics.local_upload_total
  : dashboard.value.metrics.proxy_upload_total);
const activeRiskResult = computed(() => captureMode.value === "off" ? ipDetection.value?.source : ipDetection.value?.proxy);
const activeRisk = computed(() => riskSummary(activeRiskResult.value));
const activeRiskLinkLabel = computed(() => captureMode.value === "off" ? "直连 IP" : "代理出口 IP");
const proxyRisk = computed(() => riskSummary(ipDetection.value?.proxy));
const coreUpdates = computed(() => Object.fromEntries(
  (coreUpdateReport.value?.items ?? []).map((item) => [item.id, item]),
));
const directAndProxySame = computed(() => {
  const source = ipDetection.value?.source.ip;
  const proxy = ipDetection.value?.proxy.ip;
  return Boolean(source && proxy && source === proxy);
});
const captureTransitioning = computed(() =>
  ["starting_system_proxy", "starting_tun", "stopping", "recovering"].includes(dashboard.value.capture.state),
);
const showTUNFault = computed(() =>
  dashboard.value.capture.state === "faulted"
  && dashboard.value.capture.can_retry_tun
  && dashboard.value.capture.fault_id !== dismissedFaultID.value,
);

watch([showTUNFault, loading], async ([shown, busy]) => {
  if (!shown || busy) return;
  await nextTick();
  tunRetryButton.value?.focus();
});

function setTheme(mode: ThemeMode) {
  theme.value = mode;
  document.documentElement.dataset.theme = mode;
  localStorage.setItem("navo-theme", mode);
  if (!window.runtime) return;
  if (mode === "day") {
    window.runtime.WindowSetLightTheme();
    window.runtime.WindowSetBackgroundColour(233, 222, 212, 255);
    return;
  }
  window.runtime.WindowSetDarkTheme();
  window.runtime.WindowSetBackgroundColour(20, 16, 39, 255);
}

const changelogText = `2026-07-30
· 窗口外框与日夜画风同步，不再固定为黑色
· 线路来源统一增加批量延迟、单线路延迟与单线路测速
· 连接管理增加线路类型约束，底部确认区明确显示来源类型
· 内核管理明确显示当前版本与官方最新版本
· 测速可按需临时启动本地核心，结束后恢复原运行状态

2026-07-29
· 日夜主题使用同一布局：日间尖锐米白橙，夜间圆润紫黑蓝
· 功能执行统一显示滚动进度，诊断日志合并到设置`;

const cardFeedbackTimers = new WeakMap<HTMLElement, number>();
const cardFeedbackSelector = [
  ".ip-card", ".risk-card", ".speed-card", ".chart-card", ".config-card",
  ".advanced-card", ".form-card", ".data-panel", ".core-grid article",
  ".diagnostic-card", ".ip-detail-grid article", ".risk-panel",
  ".host-panel", ".benchmark-panel", ".settings-log-card",
].join(",");

function showCardFeedback(event: PointerEvent) {
  const target = event.target instanceof Element ? event.target : null;
  const card = target?.closest<HTMLElement>(cardFeedbackSelector);
  if (!card) return;

  const previousTimer = cardFeedbackTimers.get(card);
  if (previousTimer) window.clearTimeout(previousTimer);
  card.classList.remove("card-feedback");
  void card.offsetWidth;
  card.classList.add("card-feedback");
  cardFeedbackTimers.set(card, window.setTimeout(() => {
    card.classList.remove("card-feedback");
    cardFeedbackTimers.delete(card);
  }, 360));
}

function beginActivity(label: string) {
  if (activityTimer) clearInterval(activityTimer);
  if (activityHideTimer) clearTimeout(activityHideTimer);
	if (logTimer) clearInterval(logTimer);
  activityLabel.value = label;
  activityProgress.value = 8;
  activityVisible.value = true;
  activityTimer = setInterval(() => {
    activityProgress.value = Math.min(88, activityProgress.value + Math.max(1, Math.round((92 - activityProgress.value) / 7)));
  }, 140);
}

function finishActivity() {
  if (activityTimer) clearInterval(activityTimer);
  activityTimer = undefined;
  activityProgress.value = 100;
  activityHideTimer = setTimeout(() => {
    activityVisible.value = false;
    activityProgress.value = 0;
  }, 520);
}

async function execute(action: () => Promise<void>, success = "", progressLabel = "") {
  loading.value = true;
  failure.value = "";
  notice.value = "";
  beginActivity(progressLabel || success || "正在处理请求");
  try {
    await action();
    notice.value = success;
	return true;
  } catch (reason) {
    failure.value = errorMessage(reason);
	return false;
  } finally {
    loading.value = false;
    finishActivity();
  }
}

async function loadDashboard() {
  const snapshot = await api.dashboard();
  dashboard.value = {
    ...emptyDashboard,
    ...snapshot,
    core: { ...emptyDashboard.core, ...snapshot?.core },
    proxy: { ...emptyDashboard.proxy, ...snapshot?.proxy },
    runtime: { ...emptyDashboard.runtime, ...snapshot?.runtime },
    tun: { ...emptyDashboard.tun, ...snapshot?.tun },
    capture: { ...emptyDashboard.capture, ...snapshot?.capture },
    metrics: { ...emptyDashboard.metrics, ...snapshot?.metrics },
    ip: { ...emptyDashboard.ip, ...snapshot?.ip },
    cores: snapshot?.cores ?? [],
  };
  syncRoutingRuleDrafts();
  metricsAvailable.value = dashboard.value.metrics.available;
  updateHealthCounters(dashboard.value.metrics.reachable);
}

async function loadRoutes() {
  routes.value = (await api.routes()).outbounds ?? [];
}

async function loadSubscriptions() {
  subscriptions.value = (await api.subscriptions()).subscriptions ?? [];
}

async function loadLogs() {
	logMetadata.value = await api.logMetadata();
	const result = await api.queryLogs({
		levels: selectedLogLevels.value,
		services: selectedLogServices.value,
		from: logFrom.value ? new Date(logFrom.value).toISOString() : "",
		to: logTo.value ? new Date(logTo.value).toISOString() : "",
		after_id: 0,
		limit: 200,
	});
	logs.value = result.entries ?? [];
	logCursor.value = result.next_cursor || 0;
	logHasMore.value = result.has_more;
}

async function refreshLogs() {
  await execute(loadLogs, "诊断日志已更新", "正在读取诊断日志");
}

async function loadMoreLogs() {
	const result = await api.queryLogs({
		levels: selectedLogLevels.value, services: selectedLogServices.value,
		from: logFrom.value ? new Date(logFrom.value).toISOString() : "",
		to: logTo.value ? new Date(logTo.value).toISOString() : "",
		after_id: logCursor.value, limit: 200,
	});
	logs.value.push(...(result.entries ?? []));
	logCursor.value = result.next_cursor || logCursor.value;
	logHasMore.value = result.has_more;
}

function toggleLogSelection(target: "level" | "service", value: string, checked: boolean) {
	const selected = target === "level" ? selectedLogLevels : selectedLogServices;
	const values = new Set(selected.value);
	checked ? values.add(value) : values.delete(value);
	selected.value = [...values];
}

function clearVisibleLogs() {
	logs.value = [];
	logCursor.value = 0;
	logHasMore.value = false;
}

async function clearPersistedLogs() {
	if (!window.confirm("仅清空 Navo 结构化文本日志；不会删除崩溃转储和诊断导出。继续？")) return;
	await execute(async () => {
		await api.clearPersistedLogs();
		clearVisibleLogs();
	}, "持久化日志已清空", "正在安全轮转日志");
}

function setLogFollow(enabled: boolean) {
	logFollow.value = enabled;
	if (logTimer) clearInterval(logTimer);
	logTimer = enabled ? setInterval(() => {
		if (page.value === "settings" && !loading.value) void loadMoreLogs();
	}, 3000) : undefined;
}

async function checkIP(showProgress = true) {
  ipChecking.value = true;
  if (showProgress) beginActivity("正在检测直连与代理出口");
  try {
    const result = await api.checkIP();
    ipDetection.value = result;
    const expected = captureMode.value === "off" ? result.source : result.proxy;
    updateHealthCounters(expected.available === true || (!expected.error && Boolean(expected.ip)));
  } catch (reason) {
    failure.value = `IP 检测失败：${errorMessage(reason)}`;
    updateHealthCounters(false);
  } finally {
    ipChecking.value = false;
    if (showProgress) finishActivity();
  }
}

async function loadHostStatus() {
  hostStatus.value = await api.hostStatus();
}

async function runBenchmark() {
  if (benchmarkRunning.value) return;
  benchmarkRunning.value = true;
  failure.value = "";
  notice.value = "";
  try {
    beginActivity("正在执行代理延迟与速度测试");
    benchmark.value = await api.runProxyBenchmark();
    notice.value = "代理链路测速完成";
  } catch (reason) {
    failure.value = `代理测速失败：${errorMessage(reason)}`;
  } finally {
    benchmarkRunning.value = false;
    finishActivity();
  }
}

async function cancelBenchmark() {
  await api.cancelProxyBenchmark();
}

async function checkCoreUpdates() {
  if (coreUpdateChecking.value) return;
  coreUpdateChecking.value = true;
  failure.value = "";
  beginActivity("正在校验内核并检查官方版本");
  try {
    coreUpdateReport.value = await api.checkCoreUpdates();
  } catch (reason) {
    failure.value = `内核更新检查失败：${errorMessage(reason)}`;
  } finally {
    coreUpdateChecking.value = false;
    finishActivity();
  }
}

async function openCoreRelease(id: string) {
  try {
    await api.openCoreRelease(id);
  } catch (reason) {
    failure.value = `无法打开官方发布页：${errorMessage(reason)}`;
  }
}

function syncRoutingRuleDrafts() {
  for (const mode of ["blacklist", "whitelist"] as const) {
    if (routingRuleDirty[mode]) continue;
    routingRuleDrafts[mode] = (dashboard.value.runtime[mode] ?? []).join("\n");
  }
}

function parseRoutingRuleDraft(value: string) {
  return [...new Set(value.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean))];
}

async function installCoreUpdate(id: string) {
  if (coreUpdateInstalling.value[id]) return;
  const update = coreUpdates.value[id];
  if (!update?.update_available || !update.install_supported) return;
  if (!window.confirm(`升级 ${update.name} 到 ${update.latest_version}？升级期间会短暂关闭网络接管，完成后自动恢复。`)) return;
  coreUpdateInstalling.value = { ...coreUpdateInstalling.value, [id]: true };
  failure.value = "";
  beginActivity(`正在验证并升级 ${update.name}`);
  try {
    const installed = await api.installCoreUpdate(id);
    coreUpdateReport.value = {
      items: (coreUpdateReport.value?.items ?? []).map((item) => item.id === id ? installed : item),
      checked_at: new Date().toISOString(),
    };
    await loadDashboard();
  } catch (reason) {
    failure.value = `内核升级失败：${errorMessage(reason)}`;
  } finally {
    coreUpdateInstalling.value = { ...coreUpdateInstalling.value, [id]: false };
    finishActivity();
  }
}

async function loadPageData(target: Page) {
  if (target === "overview") await Promise.all([loadDashboard(), loadRoutes(), checkIP(false)]);
  if (target === "connection") {
    await Promise.all([loadDashboard(), loadRoutes(), loadSubscriptions()]);
    if (activeRoute.value?.source_type) sourceFilter.value = activeRoute.value.source_type;
  }
  if (target === "sources") await loadRoutes();
  if (target === "cores" || target === "traffic") await loadDashboard();
  if (target === "ip") await Promise.all([checkIP(false), loadHostStatus()]);
  if (target === "settings") await Promise.all([loadLogs(), loadHostStatus()]);
}

async function changePage(next: Page) {
  page.value = next;
  await execute(() => loadPageData(next), "", `正在载入${navigation.find((item) => item.id === next)?.label ?? "页面"}`);
}

async function sampleMetrics() {
  try {
    const snapshot = await api.dashboard();
    dashboard.value.metrics = snapshot.metrics;
    dashboard.value.core = snapshot.core;
    dashboard.value.runtime = snapshot.runtime;
    dashboard.value.proxy = snapshot.proxy;
    dashboard.value.tun = snapshot.tun;
    updateHealthCounters(snapshot.metrics.reachable);

    if (previousRouteID && previousRouteID !== snapshot.runtime.active_id) {
      ring.clear();
      trafficPoints.value = [];
    }
    previousRouteID = snapshot.runtime.active_id;
    const point: TrafficPoint = {
      timestamp: Date.parse(snapshot.metrics.traffic_sampled_at) || Date.now(),
      localUploadBps: snapshot.metrics.local_upload_bps || 0,
      localDownloadBps: snapshot.metrics.local_download_bps || 0,
      proxyUploadBps: snapshot.metrics.proxy_upload_bps || 0,
      proxyDownloadBps: snapshot.metrics.proxy_download_bps || 0,
      localUploadTotal: snapshot.metrics.local_upload_total || 0,
      localDownloadTotal: snapshot.metrics.local_download_total || 0,
      proxyUploadTotal: snapshot.metrics.proxy_upload_total || 0,
      proxyDownloadTotal: snapshot.metrics.proxy_download_total || 0,
      routeID: snapshot.runtime.active_id,
    };
    ring.push(point);
    metricsAvailable.value = snapshot.metrics.available || snapshot.metrics.local_available;
    if (document.visibilityState === "visible") trafficPoints.value = ring.snapshot();
  } catch {
    updateHealthCounters(false);
  }
}

async function toggleConnection() {
  const disconnect = dashboard.value.capture.committed_mode !== "off";
  await setCapture(disconnect ? "off" : "system_proxy");
}

async function setCapture(mode: CaptureMode) {
  if (captureTimer) clearInterval(captureTimer);
  captureTimer = setInterval(() => void loadDashboard().catch(() => undefined), 350);
  try {
    await execute(async () => {
      await api.setCaptureMode(mode);
      await loadDashboard();
    }, `接管模式已切换为${captureLabel(mode)}`);
  } finally {
    if (captureTimer) clearInterval(captureTimer);
    captureTimer = undefined;
    await loadDashboard().catch((reason) => {
      failure.value ||= `读取接管状态失败：${errorMessage(reason)}`;
    });
  }
}

async function setRuntimeMode(mode: RuntimeMode) {
  await execute(async () => {
    await api.setMode(mode);
    await loadDashboard();
  }, `流量走向已切换为${runtimeModeLabel(mode)}`);
}

async function setRoutingListMode(mode: RoutingListMode) {
	let committed = false;
	const refreshed = await execute(async () => {
    await api.setRoutingListMode(mode);
		committed = true;
		// The mutation and the follow-up dashboard read are separate IPC calls.
		// Reflect a committed mode immediately so a transient refresh failure
		// cannot leave the editor/ARIA state behind Service state.
		dashboard.value.runtime.list_mode = mode;
    await loadDashboard();
  }, `名单模式已切换为${routingListModeLabel(mode)}`);
	if (!committed) return;
	if (!refreshed) {
		failure.value = `名单模式已切换为${routingListModeLabel(mode)}，但状态刷新失败；界面将自动重试同步。`;
	}
	routingListEditor.value = mode === "off" ? null : mode;
	if (routingListEditor.value) {
		await nextTick();
		routingListTextarea.value?.focus();
	}
}

async function activateRoutingList(mode: Exclude<RoutingListMode, "off">) {
	if (routingListMode.value === mode) {
		routingListEditor.value = mode;
		await nextTick();
		routingListTextarea.value?.focus();
		return;
	}
	await setRoutingListMode(mode);
}

async function saveRoutingRules() {
  await execute(async () => {
    await api.setRoutingRules(
      parseRoutingRuleDraft(routingRuleDrafts.blacklist),
      parseRoutingRuleDraft(routingRuleDrafts.whitelist),
    );
    routingRuleDirty.blacklist = false;
    routingRuleDirty.whitelist = false;
    await loadDashboard();
  }, "名单规则已保存并应用", "正在验证名单规则");
}

function clearRoutingRules(mode: "blacklist" | "whitelist") {
  routingRuleDrafts[mode] = "";
  routingRuleDirty[mode] = true;
}

async function setCore(id: string) {
  await execute(async () => {
    await api.setCore(id);
    await loadDashboard();
  }, `已切换到 ${id}`);
}

async function selectRoute(item: RouteInfo) {
  await execute(async () => {
    await api.selectRoute(item.id);
    ring.clear();
    trafficPoints.value = [];
		previousRouteID = "";
    await Promise.all([loadRoutes(), loadDashboard()]);
  }, `当前线路：${item.name}`);
}

async function testRoute(item: RouteInfo) {
  if (routeTestRunning.value[item.id]) return;
  routeTestRunning.value = { ...routeTestRunning.value, [item.id]: true };
  await execute(async () => {
    const result = await api.testRoute(item.id);
    latency.value[item.id] = result.reachable ? `${Math.round(result.latency_ms)} ms` : result.error || "不可达";
  }, "", `正在测试 ${item.name} 的连接延迟`);
  routeTestRunning.value = { ...routeTestRunning.value, [item.id]: false };
}

async function testFilteredRoutes() {
  if (latencyBatchRunning.value || !filteredRoutes.value.length) return;
  latencyBatchRunning.value = true;
  failure.value = "";
  notice.value = "";
  beginActivity(`正在批量测试${sourceLabel(sourceFilter.value)}延迟`);
  const queue = [...filteredRoutes.value];
  const workers = Array.from({ length: Math.min(4, queue.length) }, async () => {
    while (queue.length) {
      const item = queue.shift();
      if (!item) return;
      try {
        const result = await api.testRoute(item.id);
        latency.value[item.id] = result.reachable ? `${Math.round(result.latency_ms)} ms` : result.error || "不可达";
      } catch (reason) {
        latency.value[item.id] = errorMessage(reason);
      }
    }
  });
  await Promise.all(workers);
  notice.value = `${sourceLabel(sourceFilter.value)}批量延迟测试完成`;
  latencyBatchRunning.value = false;
  finishActivity();
}

async function benchmarkRoute(item: RouteInfo) {
  if (benchmarkRunning.value) return;
  const previous = activeRoute.value;
  const targetChanged = previous?.id !== item.id;
  if (targetChanged && captureMode.value !== "off") {
    failure.value = "系统代理或 TUN 接管期间只能测速当前线路；请先断开连接，避免临时切线影响正在进行的流量";
    return;
  }

  benchmarkRunning.value = true;
  failure.value = "";
  notice.value = "";
  const wasRunning = dashboard.value.core.state === "running";
  try {
    if (targetChanged) {
      beginActivity(`正在切换到 ${item.name}`);
      await api.selectRoute(item.id);
      await Promise.all([loadRoutes(), loadDashboard()]);
    }
    if (!wasRunning) throw new Error("代理内核尚未运行，请先连接节点");
    beginActivity(`正在测试 ${item.name} 的延迟与速度`);
    const result = await api.runProxyBenchmark();
    benchmark.value = result;
    routeBenchmarks.value = { ...routeBenchmarks.value, [item.id]: result };
    latency.value[item.id] = `${Math.round(result.latency_ms)} ms`;
    notice.value = `${item.name} 测速完成`;
  } catch (reason) {
    failure.value = `${item.name} 测速失败：${errorMessage(reason)}`;
  } finally {
    try {
      if (targetChanged && previous) await api.selectRoute(previous.id);
      await Promise.all([loadRoutes(), loadDashboard()]);
    } catch (reason) {
      failure.value ||= `测速后恢复线路失败：${errorMessage(reason)}`;
    }
    benchmarkRunning.value = false;
    finishActivity();
  }
}

async function testActiveRoute() {
  const route = activeRoute.value;
  if (!route) {
    failure.value = "没有可测试的活动线路，请先在线路来源中添加并选择节点";
    return;
  }
  await execute(async () => {
    const result = await api.testRoute(route.id);
    latency.value[route.id] = result.reachable ? `${Math.round(result.latency_ms)} ms` : result.error || "不可达";
    if (!result.reachable) throw new Error(result.error || "当前线路不可达");
  }, "延迟测试完成", `正在测试 ${route.name} 的连接延迟`);
}

async function runLayeredLatency() {
	const route = activeRoute.value;
	if (!route || benchmarkRunning.value) {
		if (!route) failure.value = "没有可测试的当前节点";
		return;
	}
	benchmarkRunning.value = true;
	failure.value = "";
	beginActivity(`正在分层验证 ${route.name}`);
	try {
		layeredLatency.value = await api.runLatencyTest(route.id);
		if (layeredLatency.value.state === "failed") {
			throw new Error(layeredLatency.value.error_message || layeredLatency.value.error_code || "分层测速失败");
		}
		notice.value = layeredLatency.value.state === "completed" ? "当前节点分层测速完成" : "链路可用，部分指标不可观测";
	} catch (reason) {
		failure.value = `分层测速失败：${errorMessage(reason)}`;
	} finally {
		benchmarkRunning.value = false;
		finishActivity();
	}
}

function previewSyntheticTraffic() {
	const preview = generateSyntheticTraffic(trafficSimulationSize.value, trafficSimulationDirection.value);
	trafficSimulationSize.value = preview.size;
	simulatedTrafficPoints.value = preview.points;
	notice.value = "已生成纯数据预览；未发起网络请求，也未写入真实流量历史";
}

async function runControlledTraffic() {
	if (trafficTransferRunning.value) return;
	trafficTransferRunning.value = true;
	simulatedTrafficPoints.value = [];
	failure.value = "";
	beginActivity("正在执行受控真实传输");
	try {
		const result = await api.runTrafficTransfer(trafficSimulationSize.value, trafficSimulationDirection.value);
		benchmark.value = result;
		notice.value = `真实传输完成：下载 ${formatBytes(result.download_bytes)}，上传 ${formatBytes(result.upload_bytes)}`;
	} catch (reason) {
		failure.value = `真实传输失败：${errorMessage(reason)}`;
	} finally {
		trafficTransferRunning.value = false;
		finishActivity();
	}
}

async function createUpstream() {
  await execute(async () => {
    await api.createUpstream(upstream.value);
    upstream.value = { name: "", proto: "socks5", server: "", port: 1080, username: "", password: "", udp_policy: "disabled" };
    showUpstreamForm.value = false;
    sourceFilter.value = "upstream_proxy";
    await loadRoutes();
  }, "独享代理已保存并启用");
}

async function deleteUpstream(item: RouteInfo) {
  if (!window.confirm(`删除独享代理“${item.name}”？此操作会同时移除加密凭据。`)) return;
  await execute(async () => {
    await api.deleteUpstream(item.id);
    await loadRoutes();
  }, "独享代理已删除");
}

async function addSubscription() {
  await execute(async () => {
    await api.addSubscription(subscription.value);
    subscription.value = { name: "", url: "", skip_tls_verify: false };
    showSubscriptionForm.value = false;
    await Promise.all([loadSubscriptions(), loadRoutes()]);
  }, "订阅已保存并同步");
}

async function removeSubscription(item: SubscriptionInfo) {
  if (!window.confirm(`删除机场订阅“${item.name}”及其节点？`)) return;
  await execute(async () => {
    await api.removeSubscription(item.id);
    await Promise.all([loadSubscriptions(), loadRoutes()]);
  }, "订阅已删除");
}

async function refreshAllSubscriptions() {
  await execute(async () => {
    await api.refreshSubscriptions();
    await Promise.all([loadSubscriptions(), loadRoutes()]);
  }, "订阅同步完成");
}

function updateHealthCounters(success: boolean) {
  if (success) {
    healthSuccesses.value = Math.min(2, healthSuccesses.value + 1);
    healthFailures.value = 0;
  } else {
    healthFailures.value = Math.min(3, healthFailures.value + 1);
    healthSuccesses.value = 0;
  }
}

function captureLabel(mode: CaptureMode) {
	return ({ off: "未接管", system_proxy: "系统代理", tun: "TUN 代理" } as const)[mode];
}

function runtimeModeLabel(mode: RuntimeMode) {
  return ({ bypass_mainland: "绕过大陆", global: "全局代理", direct: "全部直连" } as const)[mode];
}

function routingListModeLabel(mode: RoutingListMode) {
  return ({ off: "未启用", blacklist: "黑名单模式", whitelist: "白名单模式" } as const)[mode];
}

function capturePhaseLabel(phase: string) {
  return ({
    stopped: "未运行",
    stopping_old_mode: "正在停止旧模式",
    recovering_adapter: "正在恢复虚拟网卡",
    starting_core: "正在启动内核",
    configuring_routes: "正在配置路由",
    checking_connection: "正在检测连接",
    running: "运行中",
    faulted: "异常停止",
    rolling_back: "正在回滚",
  } as Record<string, string>)[phase] || phase;
}

function sourceLabel(type?: SourceType) {
  return type === "airport_subscription" ? "机场订阅" : type === "upstream_proxy" ? "独享代理" : "未选择";
}

function connectionLabel() {
  return ({
    disconnected: "未连接", connecting: "正在连接", connected: "连接正常",
    reconnecting: "正在确认链路", failed: "代理异常",
  } as const)[appState.value.connection];
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatRate(value = 0) {
  return `${formatBytes(value)}/s`;
}

function formatUptime(seconds: number) {
  if (!seconds) return "—";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = Math.floor(seconds % 60);
  return [hours, minutes, rest].map((part) => String(part).padStart(2, "0")).join(":");
}

function formatTime(value?: string) {
  if (!value) return "尚未检测";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "尚未检测" : date.toLocaleString();
}

function formatDuration(seconds = 0) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return days ? `${days} 天 ${hours} 小时` : `${hours} 小时 ${minutes} 分`;
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : String(reason);
}

function onVisibilityChange() {
  if (document.visibilityState === "visible") trafficPoints.value = ring.snapshot();
}

function storedCloseAction(): CloseAction | undefined {
  const uptime = hostStatus.value?.system_uptime_seconds;
  if (!Number.isFinite(uptime)) return undefined;
  return resolveClosePreference(localStorage.getItem(CLOSE_PREFERENCE_KEY), Date.now(), uptime!);
}

async function performCloseAction(action: CloseAction, persist: boolean, preserveExisting = false) {
  if (closeActionBusy.value) return;
  closeActionError.value = "";
  if (persist && Number.isFinite(hostStatus.value?.system_uptime_seconds)) {
    localStorage.setItem(CLOSE_PREFERENCE_KEY, createClosePreference(
      action,
      Date.now(),
      hostStatus.value!.system_uptime_seconds,
    ));
  } else if (!persist && !preserveExisting) {
    localStorage.removeItem(CLOSE_PREFERENCE_KEY);
  }

  if (action === "minimize") {
		closeActionBusy.value = true;
		try {
			await api.minimizeToTray();
			closeDialogOpen.value = false;
			closeActionBusy.value = false;
		} catch (reason) {
			closeActionBusy.value = false;
			closeActionError.value = errorMessage(reason);
		}
		return;
  }

  closeActionBusy.value = true;
  try {
    await api.requestExit();
  } catch (reason) {
    closeActionBusy.value = false;
    closeActionError.value = errorMessage(reason);
  }
}

function requestCloseChoice() {
  const remembered = storedCloseAction();
  if (remembered) {
    void performCloseAction(remembered, false, true);
    return;
  }
  closeAction.value = "minimize";
  rememberCloseAction.value = false;
  closeActionError.value = "";
  closeDialogOpen.value = true;
  void nextTick(() => closePrimaryButton.value?.focus());
}

function dismissCloseChoice() {
  if (!closeActionBusy.value) closeDialogOpen.value = false;
}

onMounted(async () => {
  const preferred = localStorage.getItem("navo-theme");
  setTheme(preferred === "day" || preferred === "night"
    ? preferred
    : window.matchMedia("(prefers-color-scheme: light)").matches ? "day" : "night");
	stopCloseRequestListener = EventsOn("navo:close-requested", requestCloseChoice);
  await Promise.all([loadDashboard(), loadRoutes(), loadHostStatus()]);
  void checkIP(false);
  metricsTimer = setInterval(() => void sampleMetrics(), 2000);
  document.addEventListener("visibilitychange", onVisibilityChange);
});
onBeforeUnmount(() => {
  if (metricsTimer) clearInterval(metricsTimer);
  if (captureTimer) clearInterval(captureTimer);
  if (activityTimer) clearInterval(activityTimer);
  if (activityHideTimer) clearTimeout(activityHideTimer);
	stopCloseRequestListener?.();
  document.removeEventListener("visibilitychange", onVisibilityChange);
});
</script>

<template>
  <div class="app-shell" @pointerdown="showCardFeedback">
    <aside class="sidebar">
      <div class="brand">
        <StateGlyph :state="appState.icon" size="lg" label="Navo 当前连接状态" />
        <div><strong>Navo</strong><span>网络控制台</span></div>
      </div>
      <nav aria-label="主导航">
        <section v-for="group in navigationGroups" :key="group" class="nav-group" :aria-label="group">
          <h2>{{ group }}</h2>
          <button
            v-for="item in navigation.filter((entry) => entry.group === group)"
            :key="item.id"
            :class="{ active: page === item.id }"
            :aria-current="page === item.id ? 'page' : undefined"
            :aria-label="`${item.label}：${item.description}`"
            :title="item.description"
            @click="changePage(item.id)"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path :d="item.icon" /></svg>
            <span><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
          </button>
        </section>
      </nav>
      <div class="service-state">
        <StateGlyph :state="appState.icon" size="sm" />
        <div><strong>{{ connectionLabel() }}</strong><small>{{ activeRoute?.name || dashboard.core.core_id }}</small></div>
      </div>
    </aside>

    <main tabindex="0" aria-label="Navo 主内容，可使用方向键和滚轮滚动">
      <header>
        <div class="page-heading"><span>{{ currentNavigation?.group }}</span><h1>{{ pageTitle }}</h1><p>{{ currentNavigation?.description }}</p></div>
        <div class="theme-switch" role="group" aria-label="界面形态">
          <button :class="{ active: theme === 'day' }" :aria-pressed="theme === 'day'" @click="setTheme('day')">日</button>
          <button :class="{ active: theme === 'night' }" :aria-pressed="theme === 'night'" @click="setTheme('night')">夜</button>
        </div>
      </header>

      <div v-if="activityVisible" class="activity-progress" role="progressbar" aria-live="polite" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="activityProgress">
        <div><span>{{ activityLabel }}</span><b>{{ activityProgress }}%</b></div>
        <i><em :style="{ width: `${activityProgress}%` }"></em></i>
      </div>

      <div class="feedback" aria-live="polite">
        <p v-if="failure" class="error">{{ failure }}</p>
        <p v-else-if="notice" class="success">{{ notice }}</p>
      </div>

      <div
        v-if="showTUNFault"
        class="capture-modal-backdrop"
        role="presentation"
        @keydown.esc="dismissedFaultID = dashboard.capture.fault_id"
      >
        <section
          class="capture-modal"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="tun-fault-title"
          aria-describedby="tun-fault-description"
        >
          <span class="eyebrow">TUN 运行异常</span>
          <h2 id="tun-fault-title">虚拟网卡已异常停止</h2>
          <p id="tun-fault-description">{{ dashboard.capture.last_error || "Navo 已停止内核并回滚网络配置，避免留下无效路由。" }}</p>
          <div class="capture-modal-actions">
            <button class="secondary" @click="dismissedFaultID = dashboard.capture.fault_id">关闭</button>
            <button ref="tunRetryButton" class="primary" :disabled="loading" @click="setCapture('tun')">重新启动 TUN</button>
          </div>
        </section>
      </div>

      <section v-if="page === 'overview'" class="page-content task-page overview-page">
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
            <button class="secondary" :disabled="loading" @click="checkIP()">检测链路</button>
            <button class="primary" :disabled="loading" @click="toggleConnection">{{ dashboard.core.state === "running" ? "停止代理" : "启动代理" }}</button>
          </div>
        </article>

        <div v-if="directAndProxySame" class="warning-banner" role="status">直连公网 IP 与代理出口 IP 相同，代理可能未生效。</div>

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
            <span class="card-label">IP 风险摘要</span>
            <strong :class="`risk-${activeRisk.level}`">{{ activeRisk.label }}</strong>
            <ul><li v-for="reason in activeRisk.reasons" :key="reason">{{ reason }}</li></ul>
            <small>{{ activeRiskLinkLabel }} · 仅基于公开网络属性，不声称判断 IP 是否独享。</small>
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

      <section v-else-if="page === 'connection'" class="page-content task-page connection-page">
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
                <small>系统代理、TUN、黑名单、白名单是同级入口；只有点击的功能才会启用。</small>
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
                <button v-for="mode in (['system_proxy', 'tun'] as CaptureMode[])" :key="mode" :class="{ selected: captureMode === mode }" :aria-pressed="captureMode === mode" :disabled="loading || captureTransitioning || (mode === 'tun' && (!dashboard.tun.installed || !activeCoreSupportsTUN))" @click="setCapture(mode)">
                  <strong>{{ captureLabel(mode) }}</strong>
                  <small>{{ mode === "system_proxy" ? "覆盖浏览器和遵循 Windows 代理的应用" : !dashboard.tun.installed ? "TUN 组件不可用" : !activeCoreSupportsTUN ? `${activeCore?.name || dashboard.core.core_id} 不支持 TUN` : "覆盖浏览器及不读取系统代理的应用流量" }}</small>
                </button>
                <button v-for="mode in (['blacklist', 'whitelist'] as Exclude<RoutingListMode, 'off'>[])" :key="mode" :class="{ selected: routingListMode === mode }" :aria-pressed="routingListMode === mode" :aria-expanded="routingListEditor === mode" aria-controls="routing-list-editor" :disabled="loading || captureTransitioning" @click="activateRoutingList(mode)">
                  <strong>{{ mode === "blacklist" ? "黑名单" : "白名单" }}</strong>
                  <small>{{ mode === "blacklist" ? "点击启用并打开代理规则编辑器" : "点击启用并打开直连规则编辑器" }}</small>
                </button>
              </div>
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

      <section v-else-if="page === 'sources'" class="page-content task-page sources-page">
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
            <div v-for="item in filteredRoutes" :key="item.id" class="route-row" :class="{ active: item.active }">
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
                <button class="primary compact" :disabled="item.active" @click="selectRoute(item)">{{ item.active ? "当前" : "使用" }}</button>
                <button v-if="item.source_type === 'upstream_proxy'" class="danger compact" @click="deleteUpstream(item)">删除</button>
              </div>
            </div>
          </div>
          <div v-else class="empty-state"><strong>暂无{{ sourceLabel(sourceFilter) }}线路</strong><p>添加来源后，节点会显示在这里。</p></div>
        </div>
      </section>

      <section v-else-if="page === 'cores'" class="page-content task-page cores-page">
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

      <section v-else-if="page === 'traffic'" class="page-content task-page traffic-page">
        <div class="section-heading page-intro"><div><span class="eyebrow">实时流量</span><h2>流量与连接趋势</h2><p>指标自动跟随当前接管模式，真实统计与模拟预览始终分开显示。</p></div></div>
        <div class="metric-hero">
          <div><span>{{ trafficContext.label }}下载</span><strong class="download">{{ formatRate(activeTrafficDownload) }}</strong></div>
          <div><span>{{ trafficContext.label }}上传</span><strong class="upload">{{ formatRate(activeTrafficUpload) }}</strong></div>
          <div><span>采样来源</span><strong>{{ trafficContext.source }}</strong></div>
          <div><span>接管模式</span><strong>{{ captureLabel(captureMode) }}</strong></div>
          <div><span>活动连接</span><strong>{{ captureMode !== "off" && metricsAvailable ? dashboard.metrics.connections : "—" }}</strong></div>
          <div><span>采样窗口</span><strong>{{ trafficPoints.length }} / 30</strong></div>
        </div>
        <article class="chart-card full">
          <div class="section-heading">
            <div><span class="card-label">2 秒采样 · 自动跟随接管模式</span><h2>最近 60 秒{{ trafficContext.label }}</h2></div>
            <span class="traffic-mode-badge">{{ trafficContext.source }}</span>
          </div>
		  <div v-if="simulatedTrafficPoints.length" class="simulation-banner"><strong>纯数据模拟预览</strong><span>不计入真实统计，不代表网络性能。</span><button class="text-button" @click="simulatedTrafficPoints = []">返回真实数据</button></div>
          <TrafficChart :points="trafficDisplayPoints" :visible-series="activeTrafficSeries" :stopped="activeTrafficUnavailable" :status-label="activeTrafficUnavailableReason" />
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
        <div v-if="dashboard.metrics.traffic_source_state !== 'ready'" class="capability-note">
          <strong>部分流量口径暂不可用</strong>
          <p v-if="!dashboard.metrics.local_available">本机接口：{{ dashboard.metrics.local_unavailable_reason || "无法读取物理网卡计数器" }}</p>
          <p v-if="!dashboard.metrics.available">代理业务：{{ dashboard.metrics.unavailable_reason || "当前内核没有启用 Metrics Adapter" }}</p>
          <p>Navo 不会以本机总流量推算代理流量，也不会使用模拟数据冒充真实统计。</p>
        </div>
      </section>

      <section v-else-if="page === 'ip'" class="page-content task-page ip-page">
        <div class="section-heading page-intro">
          <div><span class="eyebrow">网络诊断</span><h2>主机与代理链路检测</h2><p>测速请求固定经过 Navo 本地代理，不会修改当前系统代理、TUN 或线路选择。</p></div>
          <button class="secondary" :disabled="ipChecking" @click="checkIP()">{{ ipChecking ? "检测中" : "检测双链路" }}</button>
        </div>

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
        <div class="section-heading identity-heading"><div><span class="eyebrow">网络身份</span><h2>双链路 IP 检测</h2><p>直连请求和代理请求使用独立 HTTP Transport，任一风险服务失败都不会影响代理连接。</p></div></div>
        <div class="ip-detail-grid">
          <article v-for="(result, key) in { source: ipDetection?.source, proxy: ipDetection?.proxy }" :key="key">
            <div class="ip-result-heading"><span class="card-label">{{ key === "source" ? "直连公网 IP" : "代理出口 IP" }}</span><em :class="`state-${result?.state || 'unavailable'}`">{{ result?.state === "available" ? "可用" : result?.state === "inactive" ? "未启用" : "不可用" }}</em></div>
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
          <div><span class="card-label">可解释风险指标</span><h3>{{ proxyRisk.label }}</h3><p>不静默平均多个来源，不声称能够判断真实共享人数。</p></div>
          <div class="risk-signals">
            <span :class="{ flagged: ipDetection?.proxy.proxy }"><b>Proxy</b>{{ ipDetection ? (ipDetection.proxy.proxy ? "是" : "否") : "未知" }}</span>
            <span :class="{ flagged: ipDetection?.proxy.hosting }"><b>Hosting</b>{{ ipDetection ? (ipDetection.proxy.hosting ? "是" : "否") : "未知" }}</span>
            <span><b>Mobile</b>{{ ipDetection ? (ipDetection.proxy.mobile ? "是" : "否") : "未知" }}</span>
            <span><b>Fraud / Abuse</b>未配置第三方服务</span>
            <span><b>Blacklist</b>未配置第三方服务</span>
            <span><b>VPN / Tor</b>当前来源不提供</span>
          </div>
        </article>
      </section>

      <section v-else class="page-content task-page settings-page">
        <div class="section-heading page-intro"><div><span class="eyebrow">系统与诊断</span><h2>设置与诊断</h2><p>运行环境、更新日志与诊断日志集中在这里；界面形态由顶部状态栏切换。</p></div></div>

        <article class="settings-runtime">
          <div><span class="card-label">运行环境</span><strong>Navo {{ hostStatus?.app_version || "—" }}</strong><small>{{ hostStatus ? `${hostStatus.os} / ${hostStatus.arch} · ${hostStatus.go_version}` : "正在读取运行环境" }}</small></div>
          <div><span>系统运行</span><b>{{ formatDuration(hostStatus?.system_uptime_seconds) }}</b></div>
          <div><span>Navo 运行</span><b>{{ formatDuration(hostStatus?.process_uptime_seconds) }}</b></div>
          <div><span>内存占用</span><b>{{ hostStatus ? `${hostStatus.memory_usage_percent.toFixed(0)}%` : "—" }}</b></div>
        </article>

        <article class="changelog-card">
          <div class="section-heading"><div><span class="eyebrow">版本记录</span><h2>更新日志</h2><p>按版本日期记录实际功能调整。</p></div></div>
          <pre class="changelog-text">{{ changelogText }}</pre>
        </article>

        <article class="settings-log-card">
          <div class="section-heading">
            <div><span class="eyebrow">诊断日志</span><h2>结构化诊断日志</h2><p>后端按级别、服务、时间与游标查询；敏感字段写入前脱敏。</p></div>
			<div class="log-actions">
				<button class="secondary" :disabled="loading" @click="refreshLogs">查询</button>
				<button class="secondary" @click="clearVisibleLogs">清空当前显示</button>
				<button class="danger" @click="clearPersistedLogs">清空持久化日志</button>
			</div>
          </div>
			<div class="log-filters">
				<fieldset><legend>级别</legend><label v-for="level in logMetadata.levels" :key="level"><input type="checkbox" :checked="selectedLogLevels.includes(level)" @change="toggleLogSelection('level', level, ($event.target as HTMLInputElement).checked)" />{{ level }}</label></fieldset>
				<fieldset><legend>服务</legend><label v-for="service in logMetadata.services" :key="service"><input type="checkbox" :checked="selectedLogServices.includes(service)" @change="toggleLogSelection('service', service, ($event.target as HTMLInputElement).checked)" />{{ service }}</label><small v-if="!logMetadata.services.length">尚无结构化事件</small></fieldset>
				<label>起始时间<input v-model="logFrom" type="datetime-local" /></label>
				<label>截止时间<input v-model="logTo" type="datetime-local" /></label>
				<label class="log-follow"><input type="checkbox" :checked="logFollow" @change="setLogFollow(($event.target as HTMLInputElement).checked)" />实时跟随</label>
			</div>
			<div class="log-view structured">
				<div v-if="!logs.length" class="empty-state"><strong>暂无日志</strong><p>当前筛选条件没有结构化事件。</p></div>
				<div v-for="entry in logs" :key="entry.id" :class="`level-${entry.level.toLowerCase()}`">
					<span>{{ new Date(entry.timestamp).toLocaleString() }}</span><b>{{ entry.level }}</b><i>{{ entry.service }}<template v-if="entry.component"> / {{ entry.component }}</template></i><code>{{ entry.message }}</code>
				</div>
			</div>
			<button v-if="logHasMore" class="secondary log-more" @click="loadMoreLogs">加载下一页</button>
        </article>
      </section>
    </main>
  </div>

	<Teleport to="body">
		<div
			v-if="closeDialogOpen"
			class="close-dialog-backdrop"
			role="presentation"
			@keydown.esc="dismissCloseChoice"
		>
			<section
				class="close-dialog"
				role="dialog"
				aria-modal="true"
				aria-labelledby="close-dialog-title"
				aria-describedby="close-dialog-description"
			>
				<div class="close-dialog-heading">
					<StateGlyph state="default" size="md" label="Navo 关闭选项" />
					<div>
						<span class="eyebrow">窗口操作</span>
						<h2 id="close-dialog-title">关闭 Navo 窗口</h2>
					</div>
				</div>
				<p id="close-dialog-description">请选择最小化到系统托盘，或安全停止代理、恢复网络设置后彻底退出。</p>

				<fieldset class="close-action-options" :disabled="closeActionBusy">
					<legend>本次关闭操作</legend>
					<label :class="{ selected: closeAction === 'minimize' }">
						<input v-model="closeAction" type="radio" value="minimize" />
						<span><strong>最小化到托盘</strong><small>代理继续运行；图标可能位于任务栏右下角“^”折叠区。</small></span>
					</label>
					<label :class="{ selected: closeAction === 'exit' }">
						<input v-model="closeAction" type="radio" value="exit" />
						<span><strong>彻底退出程序</strong><small>执行安全关闭流程，并恢复 Navo 管理的网络状态。</small></span>
					</label>
				</fieldset>

				<label class="remember-close-action">
					<input v-model="rememberCloseAction" type="checkbox" :disabled="closeActionBusy" />
					<span><strong>记住本次选择</strong><small>仅在本次开机期间有效，最长 24 小时。</small></span>
				</label>
				<p v-if="closeActionError" class="close-dialog-error" role="alert">{{ closeActionError }}</p>
				<div class="close-dialog-actions">
					<button class="secondary" :disabled="closeActionBusy" @click="dismissCloseChoice">取消</button>
					<button
						ref="closePrimaryButton"
						:class="closeAction === 'exit' ? 'danger' : 'primary'"
						:disabled="closeActionBusy"
						@click="performCloseAction(closeAction, rememberCloseAction)"
					>
						{{ closeActionBusy ? (closeAction === "exit" ? "正在安全退出…" : "正在最小化…") : closeAction === "exit" ? "安全退出" : "最小化到托盘" }}
					</button>
				</div>
			</section>
		</div>
	</Teleport>
</template>
