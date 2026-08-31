import type { CaptureMode, Dashboard, IPDetection, IconState, RouteInfo } from "../../types";

export type EffectiveConnectionKind = "direct" | "navo" | "external_system_proxy";
export type EffectiveTrafficMetric = "local" | "proxy";

export interface EffectiveConnection {
  kind: EffectiveConnectionKind;
  active: boolean;
  controlledByNavo: boolean;
  title: string;
  summary: string;
  modeLabel: string;
  ownerLabel: string;
  endpoint: string;
  routeID: string;
  routeName: string;
  icon: IconState;
  trafficMetric: EffectiveTrafficMetric;
  trafficLabel: string;
  trafficSource: string;
  trafficNote: string;
  exitIP: string;
  exitCountry: string;
  exitError: string;
  exitProvider: string;
  exitCheckedAt: string;
  exitPending: boolean;
  directIP: string;
  directError: string;
}

function navoCaptureLabel(mode: CaptureMode) {
  return mode === "tun" ? "Navo TUN" : "Navo 系统代理";
}

function detectedProxyResult(
  ipDetection: IPDetection | undefined,
  kind: EffectiveConnectionKind,
) {
  if (!ipDetection) return undefined;
  if (kind === "external_system_proxy" && ipDetection.connection_kind !== kind) return undefined;
  if (ipDetection.connection_kind && ipDetection.connection_kind !== kind) return undefined;
  return ipDetection.proxy;
}

function detectedDirectResult(
  ipDetection: IPDetection | undefined,
  kind: EffectiveConnectionKind,
) {
  if (!ipDetection) return undefined;
  if (kind === "external_system_proxy" && ipDetection.connection_kind !== kind) return undefined;
  if (ipDetection.connection_kind && ipDetection.connection_kind !== kind) return undefined;
  return ipDetection.source;
}

export function deriveEffectiveConnection(
  dashboard: Dashboard,
  activeRoute?: RouteInfo,
  ipDetection?: IPDetection,
): EffectiveConnection {
  const captureMode = dashboard.capture.committed_mode;
  const activeID = dashboard.runtime.active_id?.trim() || "";
  const externalProxy = dashboard.environment?.system_proxy;
  const dashboardKind = dashboard.ip.connection_kind;
  const externalObserved = captureMode === "off" && (
    dashboardKind === "external_system_proxy" ||
    Boolean(externalProxy?.enabled && externalProxy.ownership === "external")
  );
  const kind: EffectiveConnectionKind = captureMode !== "off"
    ? "navo"
    : externalObserved ? "external_system_proxy" : "direct";
  const proxyDetection = detectedProxyResult(ipDetection, kind);
  const directDetection = detectedDirectResult(ipDetection, kind);
  const exitIP = proxyDetection?.ip || dashboard.ip.proxy_ip || "";
  const exitCountry = proxyDetection?.country || dashboard.ip.proxy_country || "";
  const exitError = proxyDetection?.error || dashboard.ip.proxy_error || "";
  const exitProvider = proxyDetection?.provider || dashboard.ip.proxy_provider || "";
  const exitCheckedAt = proxyDetection?.checked_at || dashboard.ip.proxy_checked_at || "";
  const directIP = directDetection?.ip || dashboard.ip.direct_ip || "";
  const directError = directDetection?.error || dashboard.ip.direct_error || "";

  if (kind === "navo") {
    const modeLabel = navoCaptureLabel(captureMode);
    const routeName = activeRoute?.id === activeID ? activeRoute.name : activeID;
    return {
      kind,
      active: Boolean(activeID),
      controlledByNavo: true,
      title: activeID ? `${modeLabel}已连接` : `${modeLabel}正在提交`,
      summary: activeID
        ? `${routeName || "活动节点"} · 由 Navo 管理`
        : "尚无通过数据面验证的活动节点",
      modeLabel,
      ownerLabel: "Navo 管理",
      endpoint: dashboard.proxy?.enabled ? `${dashboard.proxy.server}:${dashboard.proxy.port}` : "",
      routeID: activeID,
      routeName,
      icon: activeRoute?.source_type === "airport_subscription" ? "airport" : "proxy",
      trafficMetric: "proxy",
      trafficLabel: captureMode === "tun" ? "TUN 流量" : "系统代理流量",
      trafficSource: "代理内核计数",
      trafficNote: "曲线只统计当前 Navo 代理内核处理的业务流量。",
      exitIP,
      exitCountry,
      exitError,
      exitProvider,
      exitCheckedAt,
      exitPending: Boolean(dashboard.ip.probe_pending && !exitIP),
      directIP,
      directError,
    };
  }

  if (kind === "external_system_proxy") {
    const endpoint = externalProxy?.proxy_server || "";
    return {
      kind,
      active: Boolean(externalProxy?.enabled || dashboardKind === kind),
      controlledByNavo: false,
      title: exitIP ? "外部系统代理出口已验证" : "检测到外部系统代理",
      summary: endpoint ? `${endpoint} · Navo 只读观察` : "Windows System Proxy · Navo 只读观察",
      modeLabel: "外部系统代理",
      ownerLabel: "外部应用管理",
      endpoint,
      routeID: endpoint ? `external-system-proxy:${endpoint}` : "external-system-proxy",
      routeName: "外部代理",
      icon: "proxy",
      trafficMetric: "local",
      trafficLabel: "系统总流量",
      trafficSource: "物理网卡计数 · 外部代理只读",
      trafficNote: "外部代理未向 Navo 提供独立 Metrics；曲线是系统接口总量，不冒充代理专属流量。",
      exitIP,
      exitCountry,
      exitError,
      exitProvider,
      exitCheckedAt,
      exitPending: Boolean(dashboard.ip.probe_pending && !exitIP),
      directIP,
      directError,
    };
  }

  return {
    kind,
    active: true,
    controlledByNavo: false,
    title: "当前为直连网络",
    summary: "未检测到已生效的系统代理或 Navo 接管",
    modeLabel: "直连",
    ownerLabel: "系统网络",
    endpoint: "",
    routeID: "direct",
    routeName: "直连网络",
    icon: "default",
    trafficMetric: "local",
    trafficLabel: "本机总流量",
    trafficSource: "物理网卡计数",
    trafficNote: "曲线显示系统物理接口的真实总流量。",
    exitIP: directIP,
    exitCountry: directDetection?.country || "",
    exitError: directError,
    exitProvider: directDetection?.provider || dashboard.ip.direct_provider || "",
    exitCheckedAt: directDetection?.checked_at || dashboard.ip.direct_checked_at || "",
    exitPending: Boolean(dashboard.ip.probe_pending && !directIP),
    directIP,
    directError,
  };
}
