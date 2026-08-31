import { computed, onBeforeUnmount, onMounted, ref, type ComputedRef, type Ref } from "vue";
import { apis } from "../../api/index";

import { TrafficRingBuffer } from "../../state";

import { generateSyntheticTraffic } from "../../traffic.js";
import type { Dashboard, ProxyBenchmark, TrafficPoint, TrafficSeries } from "../../types";
import { errorMessage, formatBytes } from "../application/formatters";
import type { EffectiveConnection } from "../runtime/effectiveConnection";
import { metricsPollDelay, shouldRecordTrafficSample, trafficRouteUpdate } from "./polling";

interface UseTrafficOptions {
  dashboard: Ref<Dashboard>;
  effectiveConnection: ComputedRef<EffectiveConnection>;
  benchmark: Ref<ProxyBenchmark | undefined>;
  notice: Ref<string>;
  failure: Ref<string>;
  beginActivity: (label: string) => void;
  finishActivity: () => void;
  loadDashboard: () => Promise<Dashboard>;
}

export function useTraffic({ dashboard, effectiveConnection, benchmark, notice, failure, beginActivity, finishActivity, loadDashboard }: UseTrafficOptions) {
  const trafficPoints = ref<TrafficPoint[]>([]);
  const simulatedTrafficPoints = ref<TrafficPoint[]>([]);
  const trafficSimulationSize = ref(8);
  const trafficSimulationDirection = ref<"download" | "upload" | "both">("download");
  const trafficTransferRunning = ref(false);
  const ring = new TrafficRingBuffer(30);
  let metricsTimer: ReturnType<typeof setTimeout> | undefined;
  let previousRouteID = "";
  let metricsPollStopped = true;
  let metricsPollInFlight = false;
  let metricsPollFailures = 0;

  const proxyTrafficActive = computed(() => effectiveConnection.value.trafficMetric === "proxy");
  const metricsAvailable = computed(() => proxyTrafficActive.value
    ? dashboard.value.metrics.available
    : dashboard.value.metrics.local_available);
  const trafficDisplayPoints = computed(() => simulatedTrafficPoints.value.length ? simulatedTrafficPoints.value : trafficPoints.value);
  const latestTraffic = computed(() => trafficPoints.value.at(-1));
  const activeTrafficSeries = computed<TrafficSeries[]>(() => proxyTrafficActive.value
    ? ["proxyDownloadBps", "proxyUploadBps"]
    : ["localDownloadBps", "localUploadBps"]);
  const trafficContext = computed(() => ({
    id: effectiveConnection.value.kind,
    label: effectiveConnection.value.trafficLabel,
    source: effectiveConnection.value.trafficSource,
    note: effectiveConnection.value.trafficNote,
    metric: effectiveConnection.value.trafficMetric,
  }));
  const activeTrafficUnavailable = computed(() => proxyTrafficActive.value
    ? !dashboard.value.metrics.available
    : !dashboard.value.metrics.local_available);
  const activeTrafficUnavailableReason = computed(() => proxyTrafficActive.value
    ? dashboard.value.metrics.unavailable_reason
    : dashboard.value.metrics.local_unavailable_reason);
  const activeTrafficDownload = computed(() => proxyTrafficActive.value
    ? latestTraffic.value?.proxyDownloadBps
    : latestTraffic.value?.localDownloadBps);
  const activeTrafficUpload = computed(() => proxyTrafficActive.value
    ? latestTraffic.value?.proxyUploadBps
    : latestTraffic.value?.localUploadBps);
  const activeTrafficDownloadTotal = computed(() => proxyTrafficActive.value
    ? dashboard.value.metrics.proxy_download_total
    : dashboard.value.metrics.local_download_total);
  const activeTrafficUploadTotal = computed(() => proxyTrafficActive.value
    ? dashboard.value.metrics.proxy_upload_total
    : dashboard.value.metrics.local_upload_total);

  function scheduleMetricsPoll() {
    if (metricsPollStopped) return;
    if (metricsTimer) clearTimeout(metricsTimer);
    metricsTimer = setTimeout(() => {
      metricsTimer = undefined;
      void sampleMetrics();
    }, metricsPollDelay(metricsPollFailures));
  }

  async function sampleMetrics() {
    if (metricsPollStopped || metricsPollInFlight) return;
    metricsPollInFlight = true;
    try {
      const snapshot = await loadDashboard();
      const routeUpdate = trafficRouteUpdate(
        previousRouteID,
        effectiveConnection.value.routeID,
        snapshot.capture.state,
      );
      if (routeUpdate.reset) resetTrafficHistory();
      previousRouteID = routeUpdate.routeID;
      if (shouldRecordTrafficSample(snapshot.capture.state)) {
        ring.push({
          timestamp: Date.parse(snapshot.metrics.traffic_sampled_at) || Date.now(),
          localUploadBps: snapshot.metrics.local_upload_bps || 0,
          localDownloadBps: snapshot.metrics.local_download_bps || 0,
          proxyUploadBps: snapshot.metrics.proxy_upload_bps || 0,
          proxyDownloadBps: snapshot.metrics.proxy_download_bps || 0,
          localUploadTotal: snapshot.metrics.local_upload_total || 0,
          localDownloadTotal: snapshot.metrics.local_download_total || 0,
          proxyUploadTotal: snapshot.metrics.proxy_upload_total || 0,
          proxyDownloadTotal: snapshot.metrics.proxy_download_total || 0,
          routeID: routeUpdate.routeID,
        });
        if (document.visibilityState === "visible") trafficPoints.value = ring.snapshot();
      }
      metricsPollFailures = 0;
    } catch {
      metricsPollFailures = Math.min(metricsPollFailures + 1, 4);
    } finally {
      metricsPollInFlight = false;
      scheduleMetricsPoll();
    }
  }

  function resetTrafficHistory() {
    ring.clear();
    trafficPoints.value = [];
    previousRouteID = "";
  }

  function previewSyntheticTraffic() {
    const preview = generateSyntheticTraffic(trafficSimulationSize.value, trafficSimulationDirection.value);
    trafficSimulationSize.value = preview.size;
    simulatedTrafficPoints.value = preview.points;
    notice.value = "已生成纯数据预览；未发起网络请求，也未写入真实流量历史";
  }

  async function runControlledTraffic() {
    if (trafficTransferRunning.value) return;
    trafficTransferRunning.value = true;
    simulatedTrafficPoints.value = [];
    failure.value = "";
    beginActivity("正在执行受控真实传输");
    try {
      const result = await apis.diagnostics.runTrafficTransfer(trafficSimulationSize.value, trafficSimulationDirection.value);
      benchmark.value = result;
      notice.value = `真实传输完成：下载 ${formatBytes(result.download_bytes)}，上传 ${formatBytes(result.upload_bytes)}`;
    } catch (reason) {
      failure.value = `真实传输失败：${errorMessage(reason)}`;
    } finally {
      trafficTransferRunning.value = false;
      finishActivity();
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === "visible") trafficPoints.value = ring.snapshot();
  }

  onMounted(() => {
    metricsPollStopped = false;
    void sampleMetrics();
    document.addEventListener("visibilitychange", onVisibilityChange);
  });
  onBeforeUnmount(() => {
    metricsPollStopped = true;
    if (metricsTimer) clearTimeout(metricsTimer);
    document.removeEventListener("visibilitychange", onVisibilityChange);
  });

  return {
    trafficPoints,
    simulatedTrafficPoints,
    trafficSimulationSize,
    trafficSimulationDirection,
    trafficTransferRunning,
    metricsAvailable,
    trafficDisplayPoints,
    activeTrafficSeries,
    trafficContext,
    activeTrafficUnavailable,
    activeTrafficUnavailableReason,
    activeTrafficDownload,
    activeTrafficUpload,
    activeTrafficDownloadTotal,
    activeTrafficUploadTotal,
    resetTrafficHistory,
    previewSyntheticTraffic,
    runControlledTraffic,
  };
}
