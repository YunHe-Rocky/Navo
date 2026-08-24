import { computed, onMounted, ref } from "vue";
import { apis } from "../../api/index";
import { createDashboardSnapshotLoader, createEmptyDashboard } from "../../state/runtime";
import { createInitialUIState } from "../../state/ui";
import { formatBytes, formatDuration, formatRate, formatTime, formatUptime } from "./formatters";
import { useApplicationFeedback, type ApplicationFeedbackHooks } from "./useApplicationFeedback";
import { useCloseBehavior } from "./useCloseBehavior";
import { useLogs } from "../logs/useLogs";
import { useDiagnostics } from "../diagnostics/useDiagnostics";
import { useCore } from "../core/useCore";
import { useRouting } from "../routing/useRouting";
import { useSubscriptions } from "../subscriptions/useSubscriptions";
import { useTraffic } from "../traffic/useTraffic";
import { useNodes } from "../nodes/useNodes";
import { useCapture } from "../capture/useCapture";
import { useRuntimeOverview } from "../runtime/useRuntimeOverview";
import { useStartupSettings } from "../startup/useStartupSettings";
import {
  captureLabel,
  capturePhaseLabel,
  faultDomainLabel,
  recoveryStateLabel,
  repairActionLabel,
  routingListModeLabel,
  runtimeModeLabel,
  sourceLabel,
} from "../runtime/presenter";
import type {
  Dashboard,
  Page,
} from "../../types";

export function useNavoApplication() {
  const emptyDashboard = createEmptyDashboard();
  const initialUI = createInitialUIState();
  const feedbackHooks: ApplicationFeedbackHooks = {};
  const {
    loading, notice, failure, activityVisible, activityLabel, activityProgress,
    beginActivity, finishActivity, execute,
  } = useApplicationFeedback(initialUI, feedbackHooks);
  
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
  
  const page = ref<Page>(initialUI.page);
  const theme = ref<ThemeMode>(initialUI.theme);
  const dashboard = ref<Dashboard>(emptyDashboard);
  const dashboardSnapshotLoader = createDashboardSnapshotLoader(
    () => apis.runtime.dashboard(),
    (snapshot) => {
      dashboard.value = snapshot;
    },
  );
  
  const logsFeature = useLogs({ page, loading, execute });
  const {
    logs, logMetadata, selectedLogLevels, selectedLogServices, logFrom, logTo,
    logHasMore, logFollow, loadLogs, refreshLogs, loadMoreLogs, toggleLogSelection,
    clearVisibleLogs, clearPersistedLogs, setLogFollow,
  } = logsFeature;
  feedbackHooks.onActivityBegin = logsFeature.pauseFollowPolling;

  const nodes = useNodes({
    dashboard,
    loadDashboard,
    execute,
    notice,
    failure,
    beginActivity,
    finishActivity,
    benchmarkState: () => ({ benchmark, running: benchmarkRunning }),
    resetTrafficHistory: () => resetTrafficHistory(),
  });
  const {
    routes, routeBenchmarks, latencyBatchRunning, routeTestRunning, sourceFilter, latency,
    activeRoute, selectedRoute, sourceRoute, activeRouteLatency, filteredRoutes, loadRoutes,
    selectRoute, testRoute, testFilteredRoutes, benchmarkRoute, testActiveRoute,
  } = nodes;
  const capture = useCapture({ dashboard, loading, failure, loadDashboard, execute });
  const {
    dismissedFaultID, tunRetryButton, captureMode, captureRouteMissing, captureTransitioning,
    showTUNFault, toggleConnection, setCapture,
  } = capture;
  const routing = useRouting({ dashboard, loadDashboard, execute, failure });
  const {
    routingRuleDrafts, routingRuleDirty, routingListEditor, routingListTextarea,
    runtimeMode, routingListMode, routingRuleCounts, syncRoutingRuleDrafts,
    setRuntimeMode, setRoutingListMode, activateRoutingList, saveRoutingRules, clearRoutingRules,
  } = routing;
  const pageTitle = computed(() => navigation.find((item) => item.id === page.value)?.label ?? "Navo");
  const currentNavigation = computed(() => navigation.find((item) => item.id === page.value));
  const diagnostics = useDiagnostics({
    dashboard, captureMode, activeRoute, loadDashboard, notice, failure, beginActivity, finishActivity,
  });
  const {
    ipDetection, hostStatus, benchmark, layeredLatency, benchmarkRunning, ipChecking,
    checkIP, checkConnection, loadHostStatus, runBenchmark, cancelBenchmark, runLayeredLatency,
  } = diagnostics;

  const traffic = useTraffic({ dashboard, captureMode, benchmark, notice, failure, beginActivity, finishActivity, loadDashboard: loadDashboardSnapshot });
  const {
    trafficPoints, simulatedTrafficPoints, trafficSimulationSize, trafficSimulationDirection,
    trafficTransferRunning, metricsAvailable, trafficDisplayPoints, activeTrafficSeries,
    trafficContext, activeTrafficUnavailable, activeTrafficUnavailableReason,
    activeTrafficDownload, activeTrafficUpload, activeTrafficDownloadTotal,
    activeTrafficUploadTotal, resetTrafficHistory, previewSyntheticTraffic, runControlledTraffic,
  } = traffic;

  const core = useCore({ dashboard, loadDashboard, execute, failure, beginActivity, finishActivity });
  const {
    coreUpdateReport, coreUpdateChecking, coreUpdateInstalling, showAdvancedCore,
    activeCore, activeCoreSupportsTUN, coreUpdates, checkCoreUpdates, openCoreRelease,
    installCoreUpdate, setCore,
  } = core;

  const closeBehavior = useCloseBehavior(hostStatus);
  const {
    closeDialogOpen, closeAction, rememberCloseAction, closeActionBusy,
    closeActionError, closePrimaryButton, performCloseAction, dismissCloseChoice,
  } = closeBehavior;

  const subscriptionsFeature = useSubscriptions({ sourceFilter, loadRoutes, execute });
  const {
    subscriptions, showUpstreamForm, showSubscriptionForm, upstream, subscription,
    loadSubscriptions, createUpstream, deleteUpstream, addSubscription,
    removeSubscription, refreshAllSubscriptions,
  } = subscriptionsFeature;

  const runtimeOverview = useRuntimeOverview({ dashboard, activeRoute, captureMode, ipDetection });
  const { appState, networkHealthLabel, directAndProxySame, activeRisk, proxyRisk, connectionLabel } = runtimeOverview;
  const { startupSettings, loadStartupSettings, configureStartup } = useStartupSettings(execute);
  function setTheme(mode: ThemeMode) {
    theme.value = mode;
    document.documentElement.dataset.theme = mode;
    localStorage.setItem("navo-theme", mode);
    apis.system.setTheme(mode);
  }
  
  const changelogText = `2026-08-21
  · 系统代理失败不再误报为 TUN 故障
  · 无可用线路时阻止接管并给出明确提示
  · 设置页增加需用户明确启用的开机连接与接管方式

  2026-08-17
  · 启用前强制验证 ChatGPT 网页、登录、API、静态资源与流式入口
  · 连接可用性风险显示证据时间、失败原因与处置建议
  · IP 属性提示与 ChatGPT 可用性分离，移除无数据来源的推断项
  
  2026-07-30
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
  
  function loadDashboardSnapshot(): Promise<Dashboard> {
    return dashboardSnapshotLoader();
  }

  async function loadDashboard() {
    await loadDashboardSnapshot();
    syncRoutingRuleDrafts();
  }
  
  async function repairNetworkEnvironment(code: string) {
    await execute(async () => {
      await apis.runtime.repairEnvironment(code);
      await loadDashboard();
    }, "网络环境已重新验证", "正在修复 Navo 管理的网络状态");
  }

  async function loadPageData(target: Page) {
    if (target === "overview") await Promise.all([loadDashboard(), loadRoutes(), checkIP(false)]);
    if (target === "connection") {
      await Promise.all([loadDashboard(), loadRoutes(), loadSubscriptions()]);
      if (selectedRoute.value?.source_type) sourceFilter.value = selectedRoute.value.source_type;
    }
    if (target === "sources") await loadRoutes();
    if (target === "cores" || target === "traffic") await loadDashboard();
    if (target === "ip") await Promise.all([checkIP(false), loadHostStatus()]);
    if (target === "settings") await Promise.all([loadLogs(), loadHostStatus(), loadStartupSettings()]);
  }
  
  async function changePage(next: Page) {
    page.value = next;
    await execute(() => loadPageData(next), "", `正在载入${navigation.find((item) => item.id === next)?.label ?? "页面"}`);
  }
  
  onMounted(async () => {
    const preferred = localStorage.getItem("navo-theme");
    setTheme(preferred === "day" || preferred === "night"
      ? preferred
      : window.matchMedia("(prefers-color-scheme: light)").matches ? "day" : "night");
    await Promise.all([loadDashboard(), loadRoutes(), loadHostStatus()]);
    void checkIP(false);
  });
  return {
    navigation,
    navigationGroups,
    page,
    theme,
    dashboard,
    subscriptions,
    logs,
    logMetadata,
    selectedLogLevels,
    selectedLogServices,
    logFrom,
    logTo,
    logHasMore,
    logFollow,
    ipDetection,
    hostStatus,
    startupSettings,
    benchmark,
    layeredLatency,
    routeBenchmarks,
    benchmarkRunning,
    latencyBatchRunning,
    routeTestRunning,
    coreUpdateReport,
    coreUpdateChecking,
    coreUpdateInstalling,
    trafficPoints,
    simulatedTrafficPoints,
    trafficSimulationSize,
    trafficSimulationDirection,
    trafficTransferRunning,
    loading,
    metricsAvailable,
    ipChecking,
    notice,
    failure,
    activityVisible,
    activityLabel,
    activityProgress,
    dismissedFaultID,
    tunRetryButton,
    sourceFilter,
    latency,
    showUpstreamForm,
    showSubscriptionForm,
    showAdvancedCore,
    routingRuleDrafts,
    routingRuleDirty,
    routingListEditor,
    routingListTextarea,
    closeDialogOpen,
    closeAction,
    rememberCloseAction,
    closeActionBusy,
    closeActionError,
    closePrimaryButton,
    trafficDisplayPoints,
    upstream,
    subscription,
    activeRoute,
    sourceRoute,
    activeCore,
    activeCoreSupportsTUN,
    activeRouteLatency,
    captureMode,
    runtimeMode,
    routingListMode,
    routingRuleCounts,
    appState,
    pageTitle,
    currentNavigation,
    networkHealthLabel,
    filteredRoutes,
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
    proxyRisk,
    coreUpdates,
    captureRouteMissing,
    captureTransitioning,
    showTUNFault,
    setTheme,
    changelogText,
    showCardFeedback,
    refreshLogs,
    loadMoreLogs,
    toggleLogSelection,
    clearVisibleLogs,
    clearPersistedLogs,
    setLogFollow,
    configureStartup,
    checkIP,
    checkConnection,
    runBenchmark,
    cancelBenchmark,
    checkCoreUpdates,
    openCoreRelease,
    installCoreUpdate,
    changePage,
    toggleConnection,
    setCapture,
    repairNetworkEnvironment,
    setRuntimeMode,
    setRoutingListMode,
    activateRoutingList,
    saveRoutingRules,
    clearRoutingRules,
    setCore,
    selectRoute,
    testRoute,
    testFilteredRoutes,
    benchmarkRoute,
    testActiveRoute,
    runLayeredLatency,
    previewSyntheticTraffic,
    runControlledTraffic,
    createUpstream,
    deleteUpstream,
    addSubscription,
    removeSubscription,
    refreshAllSubscriptions,
    captureLabel,
    runtimeModeLabel,
    routingListModeLabel,
    recoveryStateLabel,
    faultDomainLabel,
    repairActionLabel,
    capturePhaseLabel,
    sourceLabel,
    connectionLabel,
    formatBytes,
    formatRate,
    formatUptime,
    formatTime,
    formatDuration,
    performCloseAction,
    dismissCloseChoice,
  };
}
