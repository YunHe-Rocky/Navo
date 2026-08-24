import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/features/runtime/presenter.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText;
const presenter = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

test("backend capture phases map to user-facing status without changing truth", () => {
  assert.equal(presenter.capturePhaseLabel("verifying"), "正在验证网络");
  assert.equal(presenter.capturePhaseLabel("rolling_back"), "正在回滚");
  assert.equal(presenter.capturePhaseLabel("future_phase"), "future_phase");
});

test("runtime and recovery values are presentation mappings only", () => {
  assert.equal(presenter.captureLabel("tun"), "TUN 代理");
  assert.equal(presenter.runtimeModeLabel("global"), "全局代理");
  assert.equal(presenter.recoveryStateLabel("verifying"), "正在验证");
  assert.equal(presenter.faultDomainLabel("dns"), "DNS");
});
