import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadConnectionProjection() {
  const source = await readFile(new URL("../src/features/runtime/effectiveConnection.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

function externalDashboard() {
  return {
    core: { state: "stopped" },
    capture: { state: "stopped", committed_mode: "off", desired_mode: "off" },
    runtime: { selected_id: "candidate-v2", candidate_id: "candidate-v2", active_id: "" },
    environment: {
      system_proxy: {
        enabled: true,
        ownership: "external",
        owned_by_navo: false,
        proxy_server: "127.0.0.1:10808",
      },
    },
    ip: {
      connection_kind: "external_system_proxy",
      proxy_ip: "203.0.113.20",
      proxy_country: "External",
      direct_ip: "198.51.100.10",
      proxy_provider: "proxy-fixture",
      proxy_checked_at: "2026-08-31T04:00:00Z",
      probe_pending: false,
    },
  };
}

test("external System Proxy is the single effective connection while Navo only has a candidate", async () => {
  const { deriveEffectiveConnection } = await loadConnectionProjection();
  const connection = deriveEffectiveConnection(externalDashboard());
  assert.equal(connection.kind, "external_system_proxy");
  assert.equal(connection.controlledByNavo, false);
  assert.equal(connection.routeID, "external-system-proxy:127.0.0.1:10808");
  assert.equal(connection.exitIP, "203.0.113.20");
  assert.equal(connection.directIP, "198.51.100.10");
  assert.equal(connection.trafficMetric, "local");
  assert.equal(connection.trafficLabel, "系统总流量");
  assert.match(connection.trafficSource, /外部代理只读/);
});

test("committed Navo capture overrides an observed external proxy but an uncommitted candidate does not", async () => {
  const { deriveEffectiveConnection } = await loadConnectionProjection();
  const dashboard = externalDashboard();
  const candidateOnly = deriveEffectiveConnection(dashboard);
  assert.equal(candidateOnly.kind, "external_system_proxy");

  dashboard.capture.state = "running_system_proxy";
  dashboard.capture.committed_mode = "system_proxy";
  dashboard.runtime.active_id = "navo-active";
  dashboard.ip.connection_kind = "navo";
  const committed = deriveEffectiveConnection(dashboard, { id: "navo-active", name: "Navo A" });
  assert.equal(committed.kind, "navo");
  assert.equal(committed.controlledByNavo, true);
  assert.equal(committed.routeID, "navo-active");
  assert.equal(committed.trafficMetric, "proxy");
});

test("overview renders one canonical connection evidence card and keeps environment details off the primary surface", async () => {
  const source = await readFile(new URL("../src/features/overview/OverviewPage.vue", import.meta.url), "utf8");
  assert.equal((source.match(/class="connection-evidence-card"/g) || []).length, 1);
  assert.doesNotMatch(source, /class="ip-card"/);
  assert.doesNotMatch(source, /<NetworkEnvironmentCard/);
  assert.match(source, /直连基线/);
  assert.match(source, /effectiveConnection/);
});

test("traffic projection is driven by the same effective connection instead of raw capture mode", async () => {
  const source = await readFile(new URL("../src/features/traffic/useTraffic.ts", import.meta.url), "utf8");
  assert.match(source, /effectiveConnection/);
  assert.doesNotMatch(source, /captureMode\.value === "off"/);
});
