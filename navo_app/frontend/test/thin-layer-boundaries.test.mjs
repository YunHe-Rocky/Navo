import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { readFile, readdir } from "node:fs/promises";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const sourceRoot = fileURLToPath(new URL("../src", import.meta.url));

async function sourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map((entry) => {
    const target = join(directory, entry.name);
    return entry.isDirectory() ? sourceFiles(target) : [target];
  }));
  return nested.flat().filter((file) => [".ts", ".vue"].includes(extname(file)));
}

test("application shell and controller stay thin and feature-composed", async () => {
  const app = await readFile(join(sourceRoot, "App.vue"), "utf8");
  const controller = await readFile(join(sourceRoot, "features", "application", "useNavoApplication.ts"), "utf8");
  const requiredComposables = [
    "application/useApplicationFeedback.ts",
    "application/useCloseBehavior.ts",
    "capture/useCapture.ts",
    "core/useCore.ts",
    "diagnostics/useDiagnostics.ts",
    "logs/useLogs.ts",
    "nodes/useNodes.ts",
    "routing/useRouting.ts",
    "runtime/useRuntimeOverview.ts",
    "subscriptions/useSubscriptions.ts",
    "traffic/useTraffic.ts",
  ];

  assert.ok(app.split(/\r?\n/).length <= 250, "App.vue must remain an application shell");
  assert.ok(controller.split(/\r?\n/).length <= 400, "application controller must remain composition-oriented");
  for (const file of requiredComposables) {
    assert.equal(existsSync(join(sourceRoot, "features", file)), true, `missing feature composable: ${file}`);
  }
});

test("Wails bridge access stays inside typed adapters and flat facade stays retired", async () => {
  const files = await sourceFiles(sourceRoot);
  const bridgeOwners = [];
  const pageApiImports = [];
  for (const file of files) {
    const source = await readFile(file, "utf8");
    const local = relative(sourceRoot, file).replaceAll("\\", "/");
    if (/window\.(?:go|runtime)/.test(source)) bridgeOwners.push(local);
    if (local.startsWith("features/") && local.endsWith("Page.vue") && /\/api(?:\/index)?["']/.test(source)) {
      pageApiImports.push(local);
    }
  }

  assert.deepEqual(bridgeOwners, ["api/client.ts"]);
  assert.deepEqual(pageApiImports, []);
  assert.equal(existsSync(join(sourceRoot, "api.ts")), false, "flat API compatibility facade must not return");
});
