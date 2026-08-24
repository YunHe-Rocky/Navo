import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function loadPresenter() {
  const source = await readFile(new URL("../src/features/logs/presenter.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
}

test("log evidence exposes only bounded actionable fields", async () => {
  const { formatLogEvidence } = await loadPresenter();
  const evidence = formatLogEvidence({ fields: {
    method: " runtime.verify ",
    error_code: "APPLICATION_READINESS_FAILED",
    reason: " default   path stayed direct ",
    request_id: "request-123",
    credential: "must-not-render",
  } });
  assert.equal(
    evidence,
    "\u65b9\u6cd5=runtime.verify \u00b7 \u9519\u8bef\u7801=APPLICATION_READINESS_FAILED \u00b7 \u539f\u56e0=default path stayed direct \u00b7 \u8bf7\u6c42=request-123",
  );
  assert.doesNotMatch(evidence, /credential|must-not-render/);
});

test("log evidence clamps arbitrary backend text", async () => {
  const { formatLogEvidence } = await loadPresenter();
  const evidence = formatLogEvidence({ fields: { reason: "x".repeat(900) } });
  assert.ok(evidence.length <= 480);
  assert.match(evidence, /\u2026$/);
  assert.equal(formatLogEvidence({ fields: { secret: "hidden" } }), "");
});
