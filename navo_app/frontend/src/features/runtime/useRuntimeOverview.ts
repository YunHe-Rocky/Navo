import { computed, type ComputedRef, type Ref } from "vue";
import { connectionRiskSummary, deriveAppState, ipAttributeSummary } from "../../state";
import type { CaptureMode, Dashboard, IPDetection, RouteInfo } from "../../types";
import { deriveEffectiveConnection } from "./effectiveConnection";

interface UseRuntimeOverviewOptions {
  dashboard: Ref<Dashboard>;
  activeRoute: ComputedRef<RouteInfo | undefined>;
  captureMode: ComputedRef<CaptureMode>;
  ipDetection: Ref<IPDetection | undefined>;
}

export function useRuntimeOverview({ dashboard, activeRoute, captureMode, ipDetection }: UseRuntimeOverviewOptions) {
  const appState = computed(() => deriveAppState(dashboard.value, activeRoute.value));
  const effectiveConnection = computed(() => deriveEffectiveConnection(
    dashboard.value,
    activeRoute.value,
    ipDetection.value,
  ));
  const networkHealthLabel = computed(() => {
    if (effectiveConnection.value.kind === "external_system_proxy") {
      if (effectiveConnection.value.exitPending) return "正在检测外部代理出口";
      if (effectiveConnection.value.exitError) return "外部代理出口未验证";
      if (effectiveConnection.value.exitIP) return "外部代理出口已验证";
      return "检测到外部系统代理";
    }
    switch (appState.value.networkHealth) {
      case "healthy": return "网络状态正常";
      case "degraded": return "网络状态不稳定";
      case "unavailable": return "网络暂不可用";
      case "checking": return "正在检查网络";
      default: return "网络状态未知";
    }
  });
  const activeRiskResult = computed(() => effectiveConnection.value.kind === "direct"
    ? ipDetection.value?.source
    : ipDetection.value?.proxy);
  const directAndProxySame = computed(() => {
    const source = effectiveConnection.value.directIP;
    const proxy = effectiveConnection.value.exitIP;
    return Boolean(source && proxy && source === proxy);
  });
  const activeRisk = computed(() => {
    const connection = effectiveConnection.value;
    if (connection.kind === "external_system_proxy") {
      if (connection.exitPending) {
        return {
          label: "正在验证外部代理出口", level: "medium" as const,
          reasons: ["正在分别检测直连基线和当前 WinINet 代理出口"],
        };
      }
      if (connection.exitError || !connection.exitIP) {
        return {
          label: "外部代理出口未验证", level: "high" as const,
          reasons: [connection.exitError || "尚未获得外部代理出口 IP"],
          action: "确认外部代理端点可用后重新检测双链路",
        };
      }
      if (directAndProxySame.value) {
        return {
          label: "外部代理可能未生效", level: "high" as const,
          reasons: ["直连基线与外部代理出口相同"],
          action: "检查 v2rayN 的系统代理与路由模式",
        };
      }
      return {
        label: "外部代理出口已验证", level: "low" as const,
        reasons: ["当前 WinINet 代理路径返回了独立出口；Navo 仅观察，不接管其配置"],
      };
    }
    return connectionRiskSummary(
      dashboard.value,
      appState.value.networkHealth,
      activeRiskResult.value,
      directAndProxySame.value,
    );
  });
  const proxyRisk = computed(() => ipAttributeSummary(ipDetection.value?.proxy));

  function connectionLabel() {
    if (effectiveConnection.value.kind === "external_system_proxy") {
      return effectiveConnection.value.title;
    }
    return ({
      disconnected: "未连接", connecting: "正在连接", connected: "连接正常",
      reconnecting: "正在确认链路", failed: "代理异常",
    } as const)[appState.value.connection];
  }

  return {
    appState,
    effectiveConnection,
    connectionIcon: computed(() => effectiveConnection.value.kind === "external_system_proxy"
      ? effectiveConnection.value.icon
      : appState.value.icon),
    networkHealthLabel,
    directAndProxySame,
    activeRisk,
    proxyRisk,
    connectionLabel,
  };
}
