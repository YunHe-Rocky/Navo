import test from "node:test";
import assert from "node:assert/strict";
import {
  CLOSE_PREFERENCE_TTL_MS,
  createClosePreference,
  resolveClosePreference,
} from "../src/close-preference.js";

test("close preference remains valid during the same boot session", () => {
  const now = 1_800_000_000_000;
  const stored = createClosePreference("minimize", now, 3600);
  assert.equal(resolveClosePreference(stored, now + 30_000, 3630), "minimize");
});

test("close preference expires after one day", () => {
  const now = 1_800_000_000_000;
  const stored = createClosePreference("exit", now, 3600);
  assert.equal(resolveClosePreference(stored, now + CLOSE_PREFERENCE_TTL_MS, 90000), undefined);
});

test("close preference does not cross a reboot or malformed storage", () => {
  const now = 1_800_000_000_000;
  const stored = createClosePreference("exit", now, 3600);
  assert.equal(resolveClosePreference(stored, now + 60_000, 30), undefined);
  assert.equal(resolveClosePreference("not-json", now, 3600), undefined);
});
