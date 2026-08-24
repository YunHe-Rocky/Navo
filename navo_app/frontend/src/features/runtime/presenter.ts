import type { CaptureMode, RoutingListMode, RuntimeMode, SourceType } from "../../types";

export function captureLabel(mode: CaptureMode) {
  return ({ off: "未接管", system_proxy: "系统代理", tun: "TUN 代理" } as const)[mode];
}

export function runtimeModeLabel(mode: RuntimeMode) {
  return ({ bypass_mainland: "绕过大陆", global: "全局代理", direct: "全部直连" } as const)[mode];
}

export function routingListModeLabel(mode: RoutingListMode) {
  return ({ off: "未启用", blacklist: "黑名单模式", whitelist: "白名单模式" } as const)[mode];
}

export function recoveryStateLabel(state: string) {
  return ({
    idle: "无恢复任务", detected: "已检测故障", repairing: "正在修复",
    verifying: "正在验证", failover: "正在切换候选", recovered: "已恢复", failed: "恢复失败",
  } as Record<string, string>)[state] || state;
}

export function faultDomainLabel(domain: string) {
  return ({
    node: "活动节点", core: "代理内核", system_proxy: "System Proxy", tun: "TUN",
    route: "Route", dns: "DNS", nrpt: "NRPT", firewall: "Firewall",
    traffic_rule: "流量规则", physical_network: "物理网络", detection: "检测链路", unknown: "未知域",
  } as Record<string, string>)[domain] || domain || "未归因";
}

export function repairActionLabel(action: string) {
  return ({
    reapply_capture: "重建接管", restart_owned_core: "重启 Navo 内核",
    reconcile_owned_capture: "校准 Navo 接管", reconcile_owned_network: "校准 Navo 网络资源",
    recover_owned_network: "恢复 Navo 网络资源", reapply_traffic_policy: "重载流量策略", none: "仅观察",
  } as Record<string, string>)[action] || action;
}

export function capturePhaseLabel(phase: string) {
  return ({
    stopped: "未运行",
    stopping_old_mode: "正在停止旧模式",
    recovering_adapter: "正在恢复虚拟网卡",
    starting_core: "正在启动内核",
    configuring_routes: "正在配置路由",
    checking_connection: "正在检测连接",
    verifying: "正在验证网络",
    running: "运行中",
    faulted: "异常停止",
    rolling_back: "正在回滚",
  } as Record<string, string>)[phase] || phase;
}

export function sourceLabel(type?: SourceType) {
  return type === "airport_subscription" ? "机场订阅" : type === "upstream_proxy" ? "独享代理" : "未选择";
}
