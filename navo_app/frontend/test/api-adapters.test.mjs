import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadFactory(file, name) {
  const source = await readFile(new URL(`../src/api/${file}`, import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const module = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
  return module[name];
}

test("feature adapters delegate one intent to the matching Wails method", async () => {
  const calls = [];
  const bridge = new Proxy({}, {
    get: (_, method) => (...args) => {
      calls.push([method, ...args]);
      return Promise.resolve({ method });
    },
  });
  const backend = () => bridge;

  const capture = (await loadFactory("capture.ts", "createCaptureApi"))(backend);
  const routing = (await loadFactory("routing.ts", "createRoutingApi"))(backend);
  const core = (await loadFactory("core.ts", "createCoreApi"))(backend);
  const nodes = (await loadFactory("nodes.ts", "createNodeApi"))(backend);
  const subscriptions = (await loadFactory("subscriptions.ts", "createSubscriptionApi"))(backend);
  const diagnostics = (await loadFactory("diagnostics.ts", "createDiagnosticsApi"))(backend);
  const logs = (await loadFactory("logs.ts", "createLogApi"))(backend);
  const runtime = (await loadFactory("runtime.ts", "createRuntimeApi"))(backend);

  await capture.switchMode("tun");
  await capture.verify();
  await routing.setMode("global");
  await routing.setListMode("blacklist");
  await routing.setRules(["openai.com"], ["example.cn"]);
  await core.select("sing-box");
  await core.checkUpdates();
  await core.installUpdate("mihomo");
  await core.openRelease("xray");
  await nodes.list();
  await nodes.select("route-1");
  await nodes.test("route-2");
  await subscriptions.list();
  await subscriptions.add({ name: "airport", url: "https://example.com/sub", skip_tls_verify: false });
  await subscriptions.refresh();
  await subscriptions.remove("subscription-1");
  await subscriptions.createUpstream({ name: "proxy", proto: "socks5", server: "127.0.0.1", port: 1080, username: "", password: "", udp_policy: "disabled" });
  await subscriptions.deleteUpstream("upstream-1");
  await diagnostics.checkIP();
  await diagnostics.hostStatus();
  await diagnostics.runProxyBenchmark();
  await diagnostics.runRouteBenchmark("route-2");
  await diagnostics.cancelProxyBenchmark();
  await diagnostics.runLatencyTest("route-1");
  await diagnostics.runTrafficTransfer(8, "download");
  await logs.query({ levels: ["ERROR"], services: [], from: "", to: "", after_id: 0, limit: 20 });
  await logs.metadata();
  await logs.clear();
  await runtime.dashboard();
  await runtime.repairEnvironment("ENV_NAVO_ROUTE_RESIDUAL");

  assert.deepEqual(calls.map(([method]) => method), [
    "SetCaptureMode", "VerifyCapture",
    "SetRuntimeMode", "SetRoutingListMode", "SetRoutingRules",
    "SetCore", "CheckCoreUpdates", "InstallCoreUpdate", "OpenCoreRelease",
    "ListRoutes", "SelectRoute", "TestRoute",
    "ListSubscriptions", "AddSubscription", "RefreshSubscriptions", "RemoveSubscription", "CreateUpstream", "DeleteUpstream",
    "CheckIP", "GetHostStatus", "RunProxyBenchmark", "RunRouteBenchmark", "CancelProxyBenchmark", "RunLatencyTest", "RunTrafficTransfer",
    "QueryLogs", "GetLogMetadata", "ClearPersistedLogs", "GetDashboard", "RepairNetworkEnvironment",
  ]);
  assert.deepEqual(calls[0], ["SetCaptureMode", "tun"]);
  assert.deepEqual(calls.at(-1), ["RepairNetworkEnvironment", "ENV_NAVO_ROUTE_RESIDUAL"]);
  assert.deepEqual(calls[4], ["SetRoutingRules", ["openai.com"], ["example.cn"]]);
});

test("system adapter owns Wails runtime events and window theme calls", async () => {
  const backendCalls = [];
  const runtimeCalls = [];
  const bridge = new Proxy({}, {
    get: (_, method) => (...args) => {
      backendCalls.push([method, ...args]);
      return Promise.resolve();
    },
  });
  const runtime = {
    EventsOn: (...args) => { runtimeCalls.push(["EventsOn", ...args]); return () => runtimeCalls.push(["stop"]); },
    WindowSetBackgroundColour: (...args) => runtimeCalls.push(["WindowSetBackgroundColour", ...args]),
    WindowSetLightTheme: () => runtimeCalls.push(["WindowSetLightTheme"]),
    WindowSetDarkTheme: () => runtimeCalls.push(["WindowSetDarkTheme"]),
  };
  const system = (await loadFactory("system.ts", "createSystemApi"))(() => bridge, () => runtime);

  await system.startupSettings();
  await system.configureStartup(true, "system_proxy");
  const stop = system.onCloseRequested(() => undefined);
  system.setTheme("day");
  system.setTheme("night");
  await system.minimizeToTray();
  await system.requestExit();
  stop();

  assert.equal(runtimeCalls[0][0], "EventsOn");
  assert.equal(runtimeCalls[0][1], "navo:close-requested");
  assert.deepEqual(runtimeCalls.slice(1, 3), [
    ["WindowSetLightTheme"],
    ["WindowSetBackgroundColour", 233, 222, 212, 255],
  ]);
  assert.deepEqual(backendCalls.map(([method]) => method), [
    "GetStartupSettings", "SetStartupSettings", "MinimizeToTray", "RequestExit",
  ]);
  assert.deepEqual(backendCalls[1], ["SetStartupSettings", true, "system_proxy"]);
  assert.deepEqual(runtimeCalls.at(-1), ["stop"]);
});
