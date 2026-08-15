import type { LogQuery, SubscriptionRequest, UpstreamRequest } from "./types";

function app() {
  const bridge = window.go?.main?.App;
  if (!bridge) throw new Error("Navo 桌面桥接尚未就绪，请重启应用。");
  return bridge;
}

export const api = {
  dashboard: () => app().GetDashboard(),
  checkIP: () => app().CheckIP(),
  hostStatus: () => app().GetHostStatus(),
  runProxyBenchmark: () => app().RunProxyBenchmark(),
  cancelProxyBenchmark: () => app().CancelProxyBenchmark(),
	runLatencyTest: (id: string) => app().RunLatencyTest(id),
	runTrafficTransfer: (sizeMiB: number, direction: string) => app().RunTrafficTransfer(sizeMiB, direction),
  checkCoreUpdates: () => app().CheckCoreUpdates(),
	installCoreUpdate: (id: string) => app().InstallCoreUpdate(id),
  openCoreRelease: (id: string) => app().OpenCoreRelease(id),
  routes: () => app().ListRoutes(),
  subscriptions: () => app().ListSubscriptions(),
	queryLogs: (query: LogQuery) => app().QueryLogs(query),
	logMetadata: () => app().GetLogMetadata(),
	clearPersistedLogs: () => app().ClearPersistedLogs(),
  setCore: (id: string) => app().SetCore(id),
  setSystemProxy: (enabled: boolean) => app().SetSystemProxy(enabled),
  setTUN: (enabled: boolean) => app().SetTUN(enabled),
  setCaptureMode: (mode: string) => app().SetCaptureMode(mode),
  setMode: (mode: string) => app().SetRuntimeMode(mode),
  setRoutingListMode: (mode: string) => app().SetRoutingListMode(mode),
  setRoutingRules: (blacklist: string[], whitelist: string[]) => app().SetRoutingRules(blacklist, whitelist),
  selectRoute: (id: string) => app().SelectRoute(id),
  testRoute: (id: string) => app().TestRoute(id),
  createUpstream: (request: UpstreamRequest) => app().CreateUpstream(request),
  deleteUpstream: (id: string) => app().DeleteUpstream(id),
  addSubscription: (request: SubscriptionRequest) => app().AddSubscription(request),
  refreshSubscriptions: () => app().RefreshSubscriptions(),
  removeSubscription: (id: string) => app().RemoveSubscription(id),
	requestExit: () => app().RequestExit(),
	minimizeToTray: () => app().MinimizeToTray(),
};
