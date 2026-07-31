/// <reference types="vite/client" />

import type {
  Dashboard,
  CoreUpdateReport,
  HostStatus,
  IPDetection,
  Logs,
  ProxyBenchmark,
  Routes,
  SubscriptionRequest,
  Subscriptions,
  TestResult,
  UpstreamRequest,
} from "./types";

declare global {
  interface Window {
    runtime?: {
      WindowSetBackgroundColour(red: number, green: number, blue: number, alpha: number): void;
      WindowSetLightTheme(): void;
      WindowSetDarkTheme(): void;
    };
    go?: {
      main?: {
        App?: {
          GetDashboard(): Promise<Dashboard>;
          CheckIP(): Promise<IPDetection>;
          GetHostStatus(): Promise<HostStatus>;
          RunProxyBenchmark(): Promise<ProxyBenchmark>;
          CancelProxyBenchmark(): Promise<void>;
          CheckCoreUpdates(): Promise<CoreUpdateReport>;
          OpenCoreRelease(coreId: string): Promise<void>;
          ListRoutes(): Promise<Routes>;
          ListSubscriptions(): Promise<Subscriptions>;
          SetCore(coreId: string): Promise<void>;
          SetCoreRunning(running: boolean): Promise<void>;
          SetSystemProxy(enabled: boolean): Promise<void>;
          SetTUN(enabled: boolean): Promise<void>;
          SetCaptureMode(mode: string): Promise<void>;
          SetRuntimeMode(mode: string): Promise<void>;
          SelectRoute(id: string): Promise<void>;
          TestRoute(id: string): Promise<TestResult>;
          CreateUpstream(request: UpstreamRequest): Promise<void>;
          DeleteUpstream(id: string): Promise<void>;
          AddSubscription(request: SubscriptionRequest): Promise<void>;
          RefreshSubscriptions(): Promise<void>;
          RemoveSubscription(id: string): Promise<void>;
          TailLogs(): Promise<Logs>;
        };
      };
    };
  }
}

export {};
