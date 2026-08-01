import assert from "node:assert/strict";
import test from "node:test";
import { generateSyntheticTraffic, parseTrafficPreferences } from "../src/traffic.js";

test("traffic preferences reject corrupt and unknown series", () => {
  assert.deepEqual(parseTrafficPreferences("{"), {
    visibleSeries: ["localDownloadBps", "proxyDownloadBps"], windowSeconds: 60,
  });
  assert.deepEqual(parseTrafficPreferences(JSON.stringify({ visibleSeries: ["proxyUploadBps", "unknown"] })), {
    visibleSeries: ["proxyUploadBps"], windowSeconds: 60,
  });
});

test("synthetic preview is bounded, labeled, deterministic, and separated by direction", () => {
  const first = generateSyntheticTraffic(999, "download", 1_000_000);
  const second = generateSyntheticTraffic(999, "download", 1_000_000);
  assert.equal(first.size, 32);
  assert.deepEqual(first, second);
  assert.equal(first.points.length, 30);
  assert.ok(first.points.every((point) => point.simulated && point.routeID === "synthetic-preview"));
  assert.ok(first.points.every((point) => point.proxyUploadBps === 0 && point.proxyDownloadBps >= 0));
  assert.ok(first.points.every((point, index) => index === 0 || point.timestamp - first.points[index - 1].timestamp === 2000));
});
