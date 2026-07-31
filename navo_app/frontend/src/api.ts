import type { SubscriptionRequest, UpstreamRequest } from "./types";

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
  checkCoreUpdates: () => app().CheckCoreUpdates(),
  openCoreRelease: (id: string) => app().OpenCoreRelease(id),
  routes: () => app().ListRoutes(),
  subscriptions: () => app().ListSubscriptions(),
  logs: () => app().TailLogs(),
  setCore: (id: string) => app().SetCore(id),
  setCoreRunning: (running: boolean) => app().SetCoreRunning(running),
  setSystemProxy: (enabled: boolean) => app().SetSystemProxy(enabled),
  setTUN: (enabled: boolean) => app().SetTUN(enabled),
  setCaptureMode: (mode: string) => app().SetCaptureMode(mode),
  setMode: (mode: string) => app().SetRuntimeMode(mode),
  selectRoute: (id: string) => app().SelectRoute(id),
  testRoute: (id: string) => app().TestRoute(id),
  createUpstream: (request: UpstreamRequest) => app().CreateUpstream(request),
  deleteUpstream: (id: string) => app().DeleteUpstream(id),
  addSubscription: (request: SubscriptionRequest) => app().AddSubscription(request),
  refreshSubscriptions: () => app().RefreshSubscriptions(),
  removeSubscription: (id: string) => app().RemoveSubscription(id),
};
