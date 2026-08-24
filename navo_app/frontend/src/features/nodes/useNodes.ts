import { computed, ref, type Ref } from "vue";
import { apis } from "../../api/index";
import type { Dashboard, ProxyBenchmark, RouteInfo, SourceType } from "../../types";
import { errorMessage } from "../application/formatters";
import type { ApplicationExecute } from "../application/useApplicationFeedback";
import { sourceLabel } from "../runtime/presenter";

interface UseNodesOptions {
  dashboard: Ref<Dashboard>;
  loadDashboard: () => Promise<void>;
  execute: ApplicationExecute;
  notice: Ref<string>;
  failure: Ref<string>;
  beginActivity: (label: string) => void;
  finishActivity: () => void;
  benchmarkState: () => { benchmark: Ref<ProxyBenchmark | undefined>; running: Ref<boolean> };
  resetTrafficHistory: () => void;
}

export function useNodes(options: UseNodesOptions) {
  const {
    dashboard, loadDashboard, execute, notice, failure, beginActivity, finishActivity,
    benchmarkState, resetTrafficHistory,
  } = options;
  const routes = ref<RouteInfo[]>([]);
  const routeBenchmarks = ref<Record<string, ProxyBenchmark>>({});
  const latencyBatchRunning = ref(false);
  const routeTestRunning = ref<Record<string, boolean>>({});
  const sourceFilter = ref<SourceType>("airport_subscription");
  const latency = ref<Record<string, string>>({});
  const activeRoute = computed(() => routes.value.find((item) => item.id === dashboard.value.runtime.active_id || item.active));
  const selectedRoute = computed(() =>
    routes.value.find((item) => item.id === dashboard.value.runtime.selected_id)
    ?? routes.value.find((item) => item.selected || item.candidate)
    ?? activeRoute.value,
  );
  const sourceRoute = computed(() => selectedRoute.value?.source_type === sourceFilter.value ? selectedRoute.value : undefined);
  const activeRouteLatency = computed(() => {
    const route = activeRoute.value;
    if (!route) return "无活动线路";
    return latency.value[route.id] || (dashboard.value.metrics.latency_ms ? `${dashboard.value.metrics.latency_ms} ms` : "尚未测试");
  });
  const filteredRoutes = computed(() => routes.value.filter((item) => item.source_type === sourceFilter.value));

  async function loadRoutes() {
    routes.value = (await apis.nodes.list()).outbounds ?? [];
  }

  async function selectRoute(item: RouteInfo) {
    await execute(async () => {
      await apis.nodes.select(item.id);
      resetTrafficHistory();
      await Promise.all([loadRoutes(), loadDashboard()]);
    }, `当前线路：${item.name}`);
  }

  async function testRoute(item: RouteInfo) {
    if (routeTestRunning.value[item.id]) return;
    routeTestRunning.value = { ...routeTestRunning.value, [item.id]: true };
    await execute(async () => {
      const result = await apis.nodes.test(item.id);
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
          const result = await apis.nodes.test(item.id);
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
    const state = benchmarkState();
    if (state.running.value) return;
    state.running.value = true;
    failure.value = "";
    notice.value = "";
    try {
      beginActivity(`正在测试 ${item.name} 的延迟与速度`);
      const result = await apis.diagnostics.runRouteBenchmark(item.id);
      state.benchmark.value = result;
      routeBenchmarks.value = { ...routeBenchmarks.value, [item.id]: result };
      latency.value[item.id] = `${Math.round(result.latency_ms)} ms`;
      notice.value = `${item.name} 测速完成`;
    } catch (reason) {
      failure.value = `${item.name} 测速失败：${errorMessage(reason)}`;
    } finally {
      await Promise.all([loadRoutes(), loadDashboard()]).catch((reason) => {
        failure.value ||= `测速后读取线路状态失败：${errorMessage(reason)}`;
      });
      state.running.value = false;
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
      const result = await apis.nodes.test(route.id);
      latency.value[route.id] = result.reachable ? `${Math.round(result.latency_ms)} ms` : result.error || "不可达";
      if (!result.reachable) throw new Error(result.error || "当前线路不可达");
    }, "延迟测试完成", `正在测试 ${route.name} 的连接延迟`);
  }

  return {
    routes,
    routeBenchmarks,
    latencyBatchRunning,
    routeTestRunning,
    sourceFilter,
    latency,
    activeRoute,
    selectedRoute,
    sourceRoute,
    activeRouteLatency,
    filteredRoutes,
    loadRoutes,
    selectRoute,
    testRoute,
    testFilteredRoutes,
    benchmarkRoute,
    testActiveRoute,
  };
}
