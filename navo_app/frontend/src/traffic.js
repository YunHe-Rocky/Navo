const trafficSeries = new Set([
  "localUploadBps", "localDownloadBps", "proxyUploadBps", "proxyDownloadBps",
]);

export function seriesForCaptureMode(mode) {
  return mode === "system_proxy" || mode === "tun"
    ? ["proxyDownloadBps", "proxyUploadBps"]
    : ["localDownloadBps", "localUploadBps"];
}

export function trafficContextForCaptureMode(mode) {
  if (mode === "tun") return { id: "tun", label: "TUN 流量", source: "代理内核计数" };
  if (mode === "system_proxy") return { id: "system_proxy", label: "系统代理流量", source: "代理内核计数" };
  return { id: "off", label: "本地流量", source: "物理网卡计数" };
}

export function parseTrafficPreferences(serialized) {
  const defaults = { visibleSeries: ["localDownloadBps", "proxyDownloadBps"], windowSeconds: 60 };
  try {
    const parsed = JSON.parse(serialized || "null");
    const visibleSeries = Array.isArray(parsed?.visibleSeries)
      ? parsed.visibleSeries.filter((item) => trafficSeries.has(item))
      : defaults.visibleSeries;
    return { visibleSeries: visibleSeries.length ? visibleSeries : defaults.visibleSeries, windowSeconds: 60 };
  } catch {
    return defaults;
  }
}

export function generateSyntheticTraffic(sizeMiB, direction, now = Date.now()) {
  const size = Math.min(32, Math.max(1, Number(sizeMiB) || 1));
  const safeDirection = ["download", "upload", "both"].includes(direction) ? direction : "download";
  const totalBytes = size * 1024 * 1024;
  const weights = Array.from({ length: 30 }, (_, index) =>
    0.25 + Math.sin((index + 1) * 0.73) ** 2 + Math.sin((index + 4) * 0.19) * 0.2);
  const weightTotal = weights.reduce((sum, value) => sum + value, 0);
  let localUp = 0, localDown = 0, proxyUp = 0, proxyDown = 0;
  const points = weights.map((weight, index) => {
    const rate = Math.round((totalBytes * weight / weightTotal) / 2);
    const upload = safeDirection !== "download" ? rate : 0;
    const download = safeDirection !== "upload" ? rate : 0;
    localUp += upload * 2; localDown += download * 2;
    proxyUp += upload * 2; proxyDown += download * 2;
    return {
      timestamp: now - (29 - index) * 2000,
      localUploadBps: Math.round(upload * 1.08), localDownloadBps: Math.round(download * 1.08),
      proxyUploadBps: upload, proxyDownloadBps: download,
      localUploadTotal: localUp, localDownloadTotal: localDown,
      proxyUploadTotal: proxyUp, proxyDownloadTotal: proxyDown,
      routeID: "synthetic-preview", simulated: true,
    };
  });
  return { size, points };
}
