import { ref, type ComputedRef, type Ref } from "vue";
import { apis } from "../../api/index";
import type { CaptureMode, Dashboard, HostStatus, IPDetection, LatencyResult, ProxyBenchmark, RouteInfo } from "../../types";
import { errorMessage } from "../application/formatters";

interface UseDiagnosticsOptions {
  dashboard: Ref<Dashboard>;
  captureMode: ComputedRef<CaptureMode>;
  activeRoute: ComputedRef<RouteInfo | undefined>;
  loadDashboard: () => Promise<void>;
  notice: Ref<string>;
  failure: Ref<string>;
  beginActivity: (label: string) => void;
  finishActivity: () => void;
}

export function useDiagnostics(options: UseDiagnosticsOptions) {
  const {
    captureMode, activeRoute, loadDashboard, notice, failure, beginActivity, finishActivity,
  } = options;
  const ipDetection = ref<IPDetection>();
  const hostStatus = ref<HostStatus>();
  const benchmark = ref<ProxyBenchmark>();
  const layeredLatency = ref<LatencyResult>();
  const benchmarkRunning = ref(false);
  const ipChecking = ref(false);

  async function checkIP(showProgress = true) {
    ipChecking.value = true;
    if (showProgress) beginActivity("正在检测直连与代理出口");
    try {
      ipDetection.value = await apis.diagnostics.checkIP();
    } catch (reason) {
      failure.value = `IP 检测失败：${errorMessage(reason)}`;
    } finally {
      ipChecking.value = false;
      if (showProgress) finishActivity();
    }
  }

  async function checkConnection() {
    if (captureMode.value === "off") {
      await checkIP();
      notice.value = "当前未启用网络接管，已完成直连与 IP 属性检测";
      return;
    }
    ipChecking.value = true;
    failure.value = "";
    notice.value = "";
    beginActivity("正在验证 ChatGPT 应用链路");
    try {
      await apis.capture.verify();
      await Promise.all([loadDashboard(), checkIP(false)]);
      notice.value = "ChatGPT 网页、登录、API、静态资源和流式入口验证通过";
    } catch (reason) {
      failure.value = `ChatGPT 链路验证失败：${errorMessage(reason)}`;
      await loadDashboard().catch(() => undefined);
    } finally {
      ipChecking.value = false;
      finishActivity();
    }
  }

  async function loadHostStatus() {
    hostStatus.value = await apis.diagnostics.hostStatus();
  }

  async function runBenchmark() {
    if (benchmarkRunning.value) return;
    benchmarkRunning.value = true;
    failure.value = "";
    notice.value = "";
    try {
      beginActivity("正在执行代理延迟与速度测试");
      benchmark.value = await apis.diagnostics.runProxyBenchmark();
      notice.value = "代理链路测速完成";
    } catch (reason) {
      failure.value = `代理测速失败：${errorMessage(reason)}`;
    } finally {
      benchmarkRunning.value = false;
      finishActivity();
    }
  }

  async function cancelBenchmark() {
    await apis.diagnostics.cancelProxyBenchmark();
  }

  async function runLayeredLatency() {
    const route = activeRoute.value;
    if (!route || benchmarkRunning.value) {
      if (!route) failure.value = "没有可测试的当前节点";
      return;
    }
    benchmarkRunning.value = true;
    failure.value = "";
    beginActivity(`正在分层验证 ${route.name}`);
    try {
      layeredLatency.value = await apis.diagnostics.runLatencyTest(route.id);
      if (layeredLatency.value.state === "failed") {
        throw new Error(layeredLatency.value.error_message || layeredLatency.value.error_code || "分层测速失败");
      }
      notice.value = layeredLatency.value.state === "completed" ? "当前节点分层测速完成" : "链路可用，部分指标不可观测";
    } catch (reason) {
      failure.value = `分层测速失败：${errorMessage(reason)}`;
    } finally {
      benchmarkRunning.value = false;
      finishActivity();
    }
  }

  return {
    ipDetection,
    hostStatus,
    benchmark,
    layeredLatency,
    benchmarkRunning,
    ipChecking,
    checkIP,
    checkConnection,
    loadHostStatus,
    runBenchmark,
    cancelBenchmark,
    runLayeredLatency,
  };
}
