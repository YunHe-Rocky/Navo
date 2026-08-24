import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/state/ui.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText;
const ui = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

test("UI page and dialog state changes without manufacturing runtime state", () => {
  const initial = ui.createInitialUIState();
  const navigated = ui.selectPage(initial, "connection");
  const opened = ui.openDialog(navigated, "close");
  const closed = ui.closeDialog(opened);

  assert.equal(initial.page, "overview");
  assert.equal(navigated.page, "connection");
  assert.equal(opened.dialog, "close");
  assert.equal(closed.dialog, "none");
  assert.equal("captureMode" in closed, false);
});

test("UI loading and feedback transitions remain presentation-only", () => {
  const initial = ui.createInitialUIState();
  const pending = ui.beginOperation(initial, "正在验证网络");
  const failed = ui.failOperation(pending, "验证失败");
  const completed = ui.finishOperation(pending, "验证完成");

  assert.deepEqual([pending.loading, pending.activityLabel, pending.notice, pending.failure], [true, "正在验证网络", "", ""]);
  assert.deepEqual([failed.loading, failed.notice, failed.failure], [false, "", "验证失败"]);
  assert.deepEqual([completed.loading, completed.notice, completed.failure], [false, "验证完成", ""]);
});
