import { computed, type ComputedRef, type Ref } from "vue";
import { connectionRiskSummary, deriveAppState, ipAttributeSummary } from "../../state";
import type { CaptureMode, Dashboard, IPDetection, RouteInfo } from "../../types";

interface UseRuntimeOverviewOptions {
  dashboard: Ref<Dashboard>;
  activeRoute: ComputedRef<RouteInfo | undefined>;
  captureMode: ComputedRef<CaptureMode>;
  ipDetection: Ref<IPDetection | undefined>;
}

export function useRuntimeOverview({ dashboard, activeRoute, captureMode, ipDetection }: UseRuntimeOverviewOptions) {
  const appState = computed(() => deriveAppState(dashboard.value, activeRoute.value));
  const networkHealthLabel = computed(() => {
    switch (appState.value.networkHealth) {
      case "healthy": return "网络状态正常";
      case "degraded": return "网络状态不稳定";
      case "unavailable": return "网络暂不可用";
      case "checking": return "正在检查网络";
      default: return "网络状态未知";
    }
  });
  const activeRiskResult = computed(() => captureMode.value === "off" ? ipDetection.value?.source : ipDetection.value?.proxy);
  const directAndProxySame = computed(() => {
    const source = ipDetection.value?.source.ip;
    const proxy = ipDetection.value?.proxy.ip;
    return Boolean(source && proxy && source === proxy);
  });
  const activeRisk = computed(() => connectionRiskSummary(
    dashboard.value,
    appState.value.networkHealth,
    activeRiskResult.value,
    directAndProxySame.value,
  ));
  const proxyRisk = computed(() => ipAttributeSummary(ipDetection.value?.proxy));

  function connectionLabel() {
    return ({
      disconnected: "未连接", connecting: "正在连接", connected: "连接正常",
      reconnecting: "正在确认链路", failed: "代理异常",
    } as const)[appState.value.connection];
  }

  return {
    appState,
    networkHealthLabel,
    directAndProxySame,
    activeRisk,
    proxyRisk,
    connectionLabel,
  };
}
