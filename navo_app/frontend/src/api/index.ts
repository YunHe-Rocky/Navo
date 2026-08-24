import { createCaptureApi } from "./capture";
import { backend, runtimeBridge } from "./client";
import { createCoreApi } from "./core";
import { createDiagnosticsApi } from "./diagnostics";
import { createLogApi } from "./logs";
import { createNodeApi } from "./nodes";
import { createRoutingApi } from "./routing";
import { createRuntimeApi } from "./runtime";
import { createSubscriptionApi } from "./subscriptions";
import { createSystemApi } from "./system";

export const apis = {
  capture: createCaptureApi(backend),
  core: createCoreApi(backend),
  diagnostics: createDiagnosticsApi(backend),
  logs: createLogApi(backend),
  nodes: createNodeApi(backend),
  routing: createRoutingApi(backend),
  runtime: createRuntimeApi(backend),
  subscriptions: createSubscriptionApi(backend),
  system: createSystemApi(backend, runtimeBridge),
};
