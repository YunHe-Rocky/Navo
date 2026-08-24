import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadPolling() {
  const source = await readFile(new URL("../src/features/traffic/polling.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

test("metrics polling keeps healthy cadence and applies bounded exponential failure delay", async () => {
  const { metricsPollDelay } = await loadPolling();
  assert.deepEqual(
    [0, 1, 2, 3, 4, 20].map(metricsPollDelay),
    [2_000, 4_000, 8_000, 16_000, 30_000, 30_000],
  );
  assert.equal(metricsPollDelay(-1), 2_000);
  assert.equal(metricsPollDelay(Number.NaN), 2_000);
});

test("traffic composable uses one rescheduled timer instead of fixed overlapping interval", async () => {
  const source = await readFile(new URL("../src/features/traffic/useTraffic.ts", import.meta.url), "utf8");
  assert.doesNotMatch(source, /setInterval\(\(\) => void sampleMetrics/);
  assert.match(source, /metricsPollInFlight/);
  assert.match(source, /scheduleMetricsPoll\(\)/);
});

test("capture transition polling reschedules after completion instead of overlapping dashboard requests", async () => {
  const source = await readFile(new URL("../src/features/capture/useCapture.ts", import.meta.url), "utf8");
  assert.doesNotMatch(source, /setInterval\(\(\) => void loadDashboard/);
  assert.match(source, /capturePollInFlight/);
  assert.match(source, /scheduleCapturePoll\(\)/);
});
