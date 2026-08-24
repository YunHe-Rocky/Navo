import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/state.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText;
const state = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

function dashboard(readinessState = "ready") {
  return {
    core: { state: "running" },
    capture: {
      state: readinessState === "failed" ? "faulted" : "running_system_proxy",
      committed_mode: "system_proxy",
      last_error: readinessState === "failed" ? "application probe failed" : "",
      readiness: {
        state: readinessState,
        sites: { "openai-api": { dns: true, tcp: true, https: true, status_code: 401 } },
        checked_at: new Date().toISOString(),
        error: readinessState === "failed" ? "openai-api failed" : "",
      },
    },
    runtime: { active_id: "route-1" },
    metrics: { reachable: true },
    ip: { probe_pending: false },
  };
}

test("ChatGPT readiness is a hard condition for healthy and connected state", () => {
  const ready = state.deriveAppState(dashboard(), { id: "route-1" });
  assert.equal(ready.networkHealth, "healthy");
  assert.equal(ready.connection, "connected");

  const failed = state.deriveAppState(dashboard("failed"), { id: "route-1" });
  assert.equal(failed.networkHealth, "unavailable");
  assert.equal(failed.connection, "failed");
});

test("active recovery overrides transient OFF state until verification finishes", () => {
  const recovering = dashboard();
  recovering.capture.committed_mode = "off";
  recovering.capture.recovery = {
    state: "failover",
    evidence: { domain: "node" },
    rounds: [],
  };
  const active = state.deriveAppState(recovering, { id: "route-1" });
  assert.equal(active.networkHealth, "checking");
  assert.equal(active.connection, "reconnecting");

  recovering.capture.recovery.state = "failed";
  const failed = state.deriveAppState(recovering, { id: "route-1" });
  assert.equal(failed.networkHealth, "unavailable");
  assert.equal(failed.connection, "failed");
});
test("connection risk distinguishes ready, stale, failed, and disabled evidence", () => {
  const readyDashboard = dashboard();
  assert.equal(state.connectionRiskSummary(readyDashboard, "healthy").level, "low");

  readyDashboard.capture.readiness.checked_at = new Date(Date.now() - 6 * 60 * 1000).toISOString();
  assert.equal(state.connectionRiskSummary(readyDashboard, "healthy").label, "ChatGPT 证据已过期");

  const failed = state.connectionRiskSummary(dashboard("failed"), "unavailable");
  assert.equal(failed.level, "high");
  assert.match(failed.reasons[0], /openai-api/);

  const disabled = dashboard();
  disabled.capture.committed_mode = "off";
  assert.equal(state.connectionRiskSummary(disabled, "unknown").level, "unknown");
});

test("IP attributes remain contextual and do not claim application availability", () => {
  const summary = state.ipAttributeSummary({ ip: "203.0.113.1", hosting: true, proxy: true, mobile: false });
  assert.equal(summary.level, "medium");
  assert.match(summary.label, /IP 属性/);
});

test("primary connection action uses full-device TUN capture", () => {
  assert.equal(state.nextPrimaryCaptureMode("off"), "tun");
  assert.equal(state.nextPrimaryCaptureMode("tun"), "off");
  assert.equal(state.nextPrimaryCaptureMode("system_proxy"), "off");
});
