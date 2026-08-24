import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadRuntimeState() {
  const source = await readFile(new URL("../src/state/runtime.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

test("dashboard normalization remains compatible without environment", async () => {
  const state = await loadRuntimeState();
  const dashboard = state.normalizeDashboard({ core: { state: "running" } });
  assert.equal(dashboard.core.state, "running");
  assert.equal(dashboard.environment, undefined);
});

test("dashboard normalization preserves environment evidence with array fallbacks", async () => {
  const state = await loadRuntimeState();
  const dashboard = state.normalizeDashboard({
    environment: {
      version: 1,
      health: "degraded",
      stale: false,
      partial: true,
      findings: undefined,
      observation_errors: undefined,
    },
  });
  assert.equal(dashboard.environment.health, "degraded");
  assert.deepEqual(dashboard.environment.findings, []);
  assert.deepEqual(dashboard.environment.observation_errors, []);
});

test("environment card only offers repair for Navo-owned recoverable findings", async () => {
  const source = await readFile(
    new URL("../src/features/environment/NetworkEnvironmentCard.vue", import.meta.url),
    "utf8",
  );
  assert.match(source, /finding\.recoverable && finding\.ownership === 'navo'/);
  assert.match(source, /!value\.stale/);
  assert.match(source, /!value\.transition\.busy/);
  assert.match(source, /finding\.transitional/);
  assert.match(source, /aria-live="polite"/);
});
