import type {
  CoreUpdateReport,
  CoreUpdateStatus,
  Dashboard,
  NetworkEnvironmentSnapshot,
  HostStatus,
  IPDetection,
  LatencyResult,
  LogMetadata,
  LogQuery,
  LogQueryResult,
  ProxyBenchmark,
  Routes,
  SubscriptionRequest,
  Subscriptions,
  StartupSettings,
  TestResult,
  UpstreamRequest,
} from "../types";

export interface BackendBridge {
  GetDashboard(): Promise<Dashboard>;
  RepairNetworkEnvironment(code: string): Promise<NetworkEnvironmentSnapshot>;
  GetStartupSettings(): Promise<StartupSettings>;
  SetStartupSettings(enabled: boolean, mode: string): Promise<StartupSettings>;
  CheckIP(): Promise<IPDetection>;
  GetHostStatus(): Promise<HostStatus>;
  RunProxyBenchmark(): Promise<ProxyBenchmark>;
  RunRouteBenchmark(outboundId: string): Promise<ProxyBenchmark>;
  CancelProxyBenchmark(): Promise<void>;
  RunLatencyTest(outboundId: string): Promise<LatencyResult>;
  RunTrafficTransfer(sizeMiB: number, direction: string): Promise<ProxyBenchmark>;
  CheckCoreUpdates(): Promise<CoreUpdateReport>;
  InstallCoreUpdate(coreId: string): Promise<CoreUpdateStatus>;
  OpenCoreRelease(coreId: string): Promise<void>;
  ListRoutes(): Promise<Routes>;
  ListSubscriptions(): Promise<Subscriptions>;
  QueryLogs(query: LogQuery): Promise<LogQueryResult>;
  GetLogMetadata(): Promise<LogMetadata>;
  ClearPersistedLogs(): Promise<void>;
  SetCore(coreId: string): Promise<void>;
  SetCaptureMode(mode: string): Promise<void>;
  VerifyCapture(): Promise<void>;
  SetRuntimeMode(mode: string): Promise<void>;
  SetRoutingListMode(mode: string): Promise<void>;
  SetRoutingRules(blacklist: string[], whitelist: string[]): Promise<void>;
  SelectRoute(id: string): Promise<void>;
  TestRoute(id: string): Promise<TestResult>;
  CreateUpstream(request: UpstreamRequest): Promise<void>;
  DeleteUpstream(id: string): Promise<void>;
  AddSubscription(request: SubscriptionRequest): Promise<void>;
  RefreshSubscriptions(): Promise<void>;
  RemoveSubscription(id: string): Promise<void>;
  RequestExit(): Promise<void>;
  MinimizeToTray(): Promise<void>;
}

export interface RuntimeBridge {
  EventsOn(eventName: string, callback: (...data: unknown[]) => void): () => void;
  WindowSetBackgroundColour(red: number, green: number, blue: number, alpha: number): void;
  WindowSetLightTheme(): void;
  WindowSetDarkTheme(): void;
}

export type BackendProvider = () => BackendBridge;
export type RuntimeProvider = () => RuntimeBridge | undefined;

export function backend(): BackendBridge {
  const bridge = window.go?.main?.App;
  if (!bridge) throw new Error("Navo 桌面桥接尚未就绪，请重启应用。");
  return bridge;
}

export function runtimeBridge(): RuntimeBridge | undefined {
  return window.runtime;
}
