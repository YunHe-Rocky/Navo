import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadTypeScriptModule(path) {
  const source = await readFile(new URL(path, import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

function dashboard(activeID, captureState = "running_tun") {
  return {
    runtime: { active_id: activeID },
    capture: { state: captureState },
    metrics: { traffic_sampled_at: "2026-08-25T00:00:00Z" },
  };
}

test("dashboard loader keeps one request in flight and commits only the trailing snapshot", async () => {
  const runtime = await loadTypeScriptModule("../src/state/runtime.ts");
  const requests = [];
  const committed = [];
  const load = runtime.createDashboardSnapshotLoader(
    () => {
      const request = deferred();
      requests.push(request);
      return request.promise;
    },
    (snapshot) => committed.push(snapshot.runtime.active_id),
  );

  const first = load();
  const second = load();
  assert.equal(requests.length, 1);

  requests[0].resolve(dashboard(""));
  await new Promise(setImmediate);
  assert.equal(requests.length, 2);
  assert.deepEqual(committed, []);

  requests[1].resolve(dashboard("route-stable"));
  const [firstResult, secondResult] = await Promise.all([first, second]);
  assert.equal(firstResult.runtime.active_id, "route-stable");
  assert.equal(secondResult.runtime.active_id, "route-stable");
  assert.deepEqual(committed, ["route-stable"]);
});

test("a stale dashboard failure cannot cancel a newer trailing refresh", async () => {
  const runtime = await loadTypeScriptModule("../src/state/runtime.ts");
  const requests = [];
  const committed = [];
  const load = runtime.createDashboardSnapshotLoader(
    () => {
      const request = deferred();
      requests.push(request);
      return request.promise;
    },
    (snapshot) => committed.push(snapshot.runtime.active_id),
  );

  const first = load();
  const second = load();
  requests[0].reject(new Error("transition snapshot failed"));
  await new Promise(setImmediate);
  assert.equal(requests.length, 2);

  requests[1].resolve(dashboard("route-recovered"));
  const results = await Promise.all([first, second]);
  assert.deepEqual(results.map((item) => item.runtime.active_id), ["route-recovered", "route-recovered"]);
  assert.deepEqual(committed, ["route-recovered"]);
});

test("traffic history ignores temporary route identity loss during capture transitions", async () => {
  const polling = await loadTypeScriptModule("../src/features/traffic/polling.ts");

  const transition = polling.trafficRouteUpdate("route-1", "", "starting_tun");
  assert.deepEqual(transition, { routeID: "route-1", reset: false });
  assert.equal(polling.shouldRecordTrafficSample("starting_tun"), false);

  const stable = polling.trafficRouteUpdate("route-1", "route-2", "running_tun");
  assert.deepEqual(stable, { routeID: "route-2", reset: true });
  assert.equal(polling.shouldRecordTrafficSample("running_tun"), true);
});
