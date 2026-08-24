/// <reference types="vite/client" />

import type {
  Dashboard,
  NetworkEnvironmentSnapshot,
  CoreUpdateReport,
	CoreUpdateStatus,
  HostStatus,
  IPDetection,
	LogMetadata,
	LogQuery,
  LogQueryResult,
	LatencyResult,
  ProxyBenchmark,
  Routes,
  SubscriptionRequest,
  Subscriptions,
  StartupSettings,
  TestResult,
  UpstreamRequest,
} from "./types";

declare global {
  interface Window {
    runtime?: {
	  EventsOn(eventName: string, callback: (...data: unknown[]) => void): () => void;
      WindowSetBackgroundColour(red: number, green: number, blue: number, alpha: number): void;
      WindowSetLightTheme(): void;
      WindowSetDarkTheme(): void;
    };
    go?: {
      main?: {
        App?: {
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
		  GetCoreUpdateStatus(): Promise<CoreUpdateReport>;
		  InstallCoreUpdate(coreId: string): Promise<CoreUpdateStatus>;
          OpenCoreRelease(coreId: string): Promise<void>;
          ListRoutes(): Promise<Routes>;
          ListSubscriptions(): Promise<Subscriptions>;
          SetCore(coreId: string): Promise<void>;
          SetSystemProxy(enabled: boolean): Promise<void>;
          SetTUN(enabled: boolean): Promise<void>;
          SetCaptureMode(mode: string): Promise<void>;
          SetRuntimeMode(mode: string): Promise<void>;
          SetRoutingListMode(mode: string): Promise<void>;
          SetRoutingRules(blacklist: string[], whitelist: string[]): Promise<void>;
          SelectRoute(id: string): Promise<void>;
          VerifyCapture(): Promise<void>;
          TestRoute(id: string): Promise<TestResult>;
          CreateUpstream(request: UpstreamRequest): Promise<void>;
          DeleteUpstream(id: string): Promise<void>;
          AddSubscription(request: SubscriptionRequest): Promise<void>;
          RefreshSubscriptions(): Promise<void>;
          RemoveSubscription(id: string): Promise<void>;
			QueryLogs(query: LogQuery): Promise<LogQueryResult>;
			GetLogMetadata(): Promise<LogMetadata>;
			ClearPersistedLogs(): Promise<void>;
			RequestExit(): Promise<void>;
			MinimizeToTray(): Promise<void>;
        };
      };
    };
  }
}

export {};
