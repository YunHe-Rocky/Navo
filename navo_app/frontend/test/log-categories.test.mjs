import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadCategories() {
  const source = await readFile(new URL("../src/features/logs/categories.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

test("log categories default to basic service and keep stable labels", async () => {
  const categories = await loadCategories();
  assert.equal(categories.DEFAULT_LOG_CATEGORY, "basic_service");
  assert.deepEqual(categories.defaultLogCategorySelection(), ["basic_service"]);
  assert.equal(categories.logCategoryLabel("basic_service"), "\u57fa\u7840\u670d\u52a1");
  assert.equal(categories.logCategoryLabel("network_capture"), "\u7f51\u7edc\u4e0e\u63a5\u7ba1");
  assert.equal(categories.logCategoryLabel("future_category"), "\u5176\u4ed6\u670d\u52a1");
});

test("settings log filters expose category and severity as separate controls", async () => {
  const source = await readFile(new URL("../src/features/settings/SettingsPage.vue", import.meta.url), "utf8");
  assert.match(source, /<legend>\u670d\u52a1\u5206\u7ea7<\/legend>/u);
  assert.match(source, /<legend>\u65e5\u5fd7\u7ea7\u522b<\/legend>/u);
  assert.match(source, /selectedLogCategories/);
});
