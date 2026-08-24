import type {
  CaptureMode,
  ConnectionState,
  CoreState,
  Dashboard,
  IconState,
  IPDetection,
  NetworkHealthState,
  RouteInfo,
  TrafficPoint,
} from "./types";

export interface AppState {
  core: CoreState;
  connection: ConnectionState;
  networkHealth: NetworkHealthState;
  sourceType?: RouteInfo["source_type"];
  capture: CaptureMode;
  icon: IconState;
}

export class TrafficRingBuffer {
  private readonly values: TrafficPoint[] = [];

  constructor(private readonly capacity = 30) {
    if (!Number.isInteger(capacity) || capacity < 1) throw new Error("capacity must be positive");
  }

  push(point: TrafficPoint) {
    if (this.values.length === this.capacity) this.values.shift();
    this.values.push(point);
  }

  clear() {
    this.values.length = 0;
  }

  snapshot() {
    return this.values.slice();
  }
}

export function captureModeOf(dashboard: Dashboard): CaptureMode {
  return dashboard.capture.committed_mode;
}

// The primary connect action promises application-wide proxying. System Proxy
// remains available in advanced controls for proxy-aware applications only.
export function nextPrimaryCaptureMode(current: CaptureMode): CaptureMode {
  return current === "off" ? "tun" : "off";
}

export function deriveAppState(
  dashboard: Dashboard,
  route: RouteInfo | undefined,
): AppState {
  const core = normalizeCoreState(dashboard.core.state);
  const capture = captureModeOf(dashboard);
  const health = deriveHealth(dashboard);
  const recoveryActive = ["detected", "repairing", "verifying", "failover"].includes(dashboard.capture.recovery?.state ?? "idle");
  const connection = recoveryActive
    ? "reconnecting"
    : deriveConnection(core, capture, dashboard.runtime.active_id, health);
  return {
    core,
    connection,
    networkHealth: health,
    sourceType: route?.source_type,
    capture,
    icon: deriveIconState(core, connection, health, route?.source_type),
  };
}

export function deriveIconState(
  core: CoreState,
  connection: ConnectionState,
  health: NetworkHealthState,
  sourceType?: RouteInfo["source_type"],
): IconState {
  if (core === "stopped") return "default";
  if (core === "failed" || connection === "failed" || health === "unavailable") return "error";
  if (connection !== "connected" || health !== "healthy") return "default";
  return sourceType === "airport_subscription" ? "airport" : "proxy";
}

export type RiskLevel = "low" | "medium" | "high" | "unknown";

export interface ExplainableRiskSummary {
  label: string;
  level: RiskLevel;
  reasons: string[];
  action?: string;
}

export function ipAttributeSummary(result?: IPDetection["proxy"]): ExplainableRiskSummary {
  if (result?.state === "inactive") {
    return { label: "代理出口未启用", level: "unknown", reasons: ["当前没有代理出口证据"] };
  }
  if (!result || result.error || !result.ip) {
    return { label: "IP 属性未知", level: "unknown", reasons: ["IP 属性检测未完成或来源不可用"] };
  }
  const reasons: string[] = [];
  if (result.hosting) reasons.push("来源标记为机房网络");
  if (result.proxy) reasons.push("来源标记为代理网络");
  if (result.mobile) reasons.push("来源标记为移动网络");
  if (!reasons.length) reasons.push("当前来源未返回 Proxy、Hosting 或 Mobile 标记");
  return {
    label: result.hosting || result.proxy ? "IP 属性需关注" : "未发现基础属性提示",
    level: result.hosting || result.proxy ? "medium" : "low",
    reasons,
  };
}

export const riskSummary = ipAttributeSummary;

export function connectionRiskSummary(
  dashboard: Dashboard,
  health: NetworkHealthState,
  result?: IPDetection["proxy"],
  sameExit = false,
  now = Date.now(),
): ExplainableRiskSummary {
  const capture = captureModeOf(dashboard);
  const readiness = dashboard.capture.readiness;
  const contextualReasons: string[] = [];
  if (result?.hosting) contextualReasons.push("出口属性为机房网络；这是背景信息，不等于应用不可用");
  if (result?.mobile) contextualReasons.push("出口属性为移动网络；这是背景信息，不等于应用不可用");

  if (capture === "off") {
    return {
      label: "未启用网络接管",
      level: "unknown",
      reasons: ["Navo 当前不负责 ChatGPT 的应用流量", ...contextualReasons],
      action: "启用系统代理或 TUN 后执行 ChatGPT 链路验证",
    };
  }
  if (dashboard.capture.state === "faulted" || readiness?.state === "failed") {
    return {
      label: "ChatGPT 当前不可用",
      level: "high",
      reasons: [readiness?.error || dashboard.capture.last_error || "应用链路验证失败", ...contextualReasons],
      action: "修复线路后重新验证；不要把“已启用”当作可用",
    };
  }
  if (readiness?.state === "checking") {
    return {
      label: "正在验证 ChatGPT 链路",
      level: "medium",
      reasons: ["正在检查网页、登录、API、静态资源和流式入口", ...contextualReasons],
    };
  }
  if (readiness?.state !== "ready") {
    return {
      label: "尚未证明 ChatGPT 可用",
      level: "high",
      reasons: ["缺少完整应用链路证据", ...contextualReasons],
      action: "执行 ChatGPT 链路验证",
    };
  }
  const checkedAt = Date.parse(readiness.checked_at);
  if (!Number.isFinite(checkedAt) || now - checkedAt > 5 * 60 * 1000) {
    return {
      label: "ChatGPT 证据已过期",
      level: "medium",
      reasons: ["最近一次应用链路证据超过 5 分钟", ...contextualReasons],
      action: "重新验证 ChatGPT 链路",
    };
  }
  if (sameExit) {
    return {
      label: "代理可能未生效",
      level: "high",
      reasons: ["直连公网 IP 与代理出口 IP 相同", ...contextualReasons],
      action: "检查路由模式和所选节点后重新验证",
    };
  }
  if (health === "unavailable") {
    return {
      label: "ChatGPT 当前不可用",
      level: "high",
      reasons: ["实时健康状态已转为不可用", ...contextualReasons],
      action: "重新验证或切换节点",
    };
  }
  if (health === "checking" || health === "degraded") {
    return {
      label: "ChatGPT 链路需复验",
      level: "medium",
      reasons: ["应用证据已通过，但实时健康状态尚未稳定", ...contextualReasons],
      action: "重新验证并观察连接稳定性",
    };
  }
  return {
    label: "ChatGPT 链路已验证",
    level: "low",
    reasons: [`${Object.keys(readiness.sites ?? {}).length} 个应用入口通过 DNS、TCP、TLS 和 HTTP 状态验证`, ...contextualReasons],
  };
}

function normalizeCoreState(value: string): CoreState {
  return (["stopped", "starting", "running", "stopping", "failed"] as const).includes(value as CoreState)
    ? (value as CoreState)
    : "failed";
}

function deriveHealth(dashboard: Dashboard): NetworkHealthState {
  const capture = captureModeOf(dashboard);
  const readiness = dashboard.capture.readiness;
  const recoveryState = dashboard.capture.recovery?.state ?? "idle";
  if (["detected", "repairing", "verifying", "failover"].includes(recoveryState)) return "checking";
  if (recoveryState === "failed") return "unavailable";
  if (dashboard.ip.probe_pending || readiness?.state === "checking") return "checking";
  if (capture !== "off") {
    if (dashboard.capture.state === "faulted" || readiness?.state === "failed") return "unavailable";
    if (readiness?.state !== "ready") return "degraded";
    if (dashboard.metrics.reachable) return "healthy";
    return "degraded";
  }
  if (dashboard.metrics.reachable) return "healthy";
  return dashboard.core.state === "running" ? "degraded" : "unknown";
}

function deriveConnection(
  core: CoreState,
  capture: CaptureMode,
  routeID: string,
  health: NetworkHealthState,
): ConnectionState {
  if (core === "starting") return "connecting";
  if (core === "stopping") return "disconnected";
  if (core === "failed" || health === "unavailable") return "failed";
  if (core !== "running" || capture === "off" || !routeID) return "disconnected";
  return health === "healthy" ? "connected" : "reconnecting";
}
