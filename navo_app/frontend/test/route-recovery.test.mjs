import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadRouteRecovery() {
  const source = await readFile(new URL("../src/features/runtime/routeRecovery.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

function dashboard({ selectedID = "", committedMode = "off", runtimeMode = "rule" } = {}) {
  return {
    capture: { committed_mode: committedMode },
    runtime: { mode: runtimeMode, selected_id: selectedID },
  };
}

test("missing Navo route becomes an explicit selection action rather than a doomed start", async () => {
  const { derivePrimaryConnectionAction, requiresNavoRoute } = await loadRouteRecovery();
  const missing = dashboard();
  assert.equal(requiresNavoRoute(missing), true);
  assert.deepEqual(derivePrimaryConnectionAction(missing), {
    kind: "select_route",
    label: "选择 Navo 线路",
  });

  assert.deepEqual(derivePrimaryConnectionAction(dashboard({ selectedID: "route-a" })), {
    kind: "start",
    label: "启动 Navo 接管",
  });
  assert.deepEqual(derivePrimaryConnectionAction(dashboard({ committedMode: "system_proxy" })), {
    kind: "stop",
    label: "停止 Navo 接管",
  });
  assert.equal(requiresNavoRoute(dashboard({ runtimeMode: "direct" })), false);
});

test("login startup preflights route admission before invoking Agent", async () => {
  const source = await readFile(new URL("../src/features/startup/useStartupSettings.ts", import.meta.url), "utf8");
  const guard = source.indexOf("requiresNavoRoute");
  const request = source.indexOf("apis.system.configureStartup");
  assert.ok(guard >= 0, "startup composable must share the route requirement guard");
  assert.ok(request > guard, "the route guard must run before configureStartup IPC");
  assert.match(source, /setRouteRequired/);
});

test("overview, settings, and global feedback expose one route-selection recovery path", async () => {
  const [overview, settings, app] = await Promise.all([
    readFile(new URL("../src/features/overview/OverviewPage.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/features/settings/SettingsPage.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/App.vue", import.meta.url), "utf8"),
  ]);
  assert.match(overview, /primaryConnectionAction/);
  assert.match(overview, /goToRouteSelection/);
  assert.doesNotMatch(overview, /dashboard\.core\.state === "running" \? "停止 Navo 代理"/);
  assert.match(settings, /route-required-callout/);
  assert.match(settings, /goToRouteSelection/);
  assert.match(app, /role="alert"/);
  assert.match(app, /goToRouteSelection/);
});
