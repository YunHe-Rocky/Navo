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

export function deriveAppState(
  dashboard: Dashboard,
  route: RouteInfo | undefined,
  failures: number,
  successes: number,
): AppState {
  const core = normalizeCoreState(dashboard.core.state);
  const capture = captureModeOf(dashboard);
  const health = deriveHealth(dashboard, failures, successes);
  const connection = deriveConnection(core, capture, dashboard.runtime.active_id, health);
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

export function riskSummary(result?: IPDetection["proxy"]) {
  if (!result || result.error || !result.ip || result.ip === "检测暂不可用") {
    return { label: "检测不可用", level: "unknown", reasons: ["基础 IP 检测未完成"] };
  }
  const reasons: string[] = [];
  if (result.hosting) reasons.push("检测为机房网络");
  if (result.proxy) reasons.push("检测到代理属性");
  if (result.mobile) reasons.push("检测为移动网络");
  if (!reasons.length) reasons.push("基础网络属性未发现明显风险");
  return {
    label: result.hosting || result.proxy ? "需关注" : "基础风险较低",
    level: result.hosting || result.proxy ? "medium" : "low",
    reasons,
  };
}

function normalizeCoreState(value: string): CoreState {
  return (["stopped", "starting", "running", "stopping", "failed"] as const).includes(value as CoreState)
    ? (value as CoreState)
    : "failed";
}

function deriveHealth(dashboard: Dashboard, failures: number, successes: number): NetworkHealthState {
  if (dashboard.ip.probe_pending) return "checking";
  if (failures >= 3) return "unavailable";
  if (dashboard.metrics.reachable && successes >= 2) return "healthy";
  if (dashboard.metrics.reachable) return "degraded";
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
