import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from "vue";
import { apis } from "../../api/index";
import { captureModeOf, nextPrimaryCaptureMode } from "../../state";
import type { CaptureMode, Dashboard } from "../../types";
import { errorMessage } from "../application/formatters";
import type { ApplicationExecute } from "../application/useApplicationFeedback";
import { captureLabel } from "../runtime/presenter";
import { derivePrimaryConnectionAction, requiresNavoRoute } from "../runtime/routeRecovery";

interface UseCaptureOptions {
  dashboard: Ref<Dashboard>;
  loading: Ref<boolean>;
  failure: Ref<string>;
  loadDashboard: () => Promise<void>;
  execute: ApplicationExecute;
  setRouteRequired: (message: string) => void;
}

export function useCapture({ dashboard, loading, failure, loadDashboard, execute, setRouteRequired }: UseCaptureOptions) {
  const dismissedFaultID = ref("");
  const tunRetryButton = ref<HTMLButtonElement>();
  const captureMode = computed(() => captureModeOf(dashboard.value));
  const captureRouteMissing = computed(() => requiresNavoRoute(dashboard.value));
  const primaryConnectionAction = computed(() => derivePrimaryConnectionAction(dashboard.value));
  const captureTransitioning = computed(() =>
    ["starting_system_proxy", "starting_tun", "stopping", "recovering"].includes(dashboard.value.capture.state),
  );
  const showTUNFault = computed(() =>
    dashboard.value.capture.state === "faulted"
    && dashboard.value.capture.can_retry_tun
    && dashboard.value.capture.fault_id !== dismissedFaultID.value,
  );
  let captureTimer: ReturnType<typeof setTimeout> | undefined;
  let capturePollStopped = true;
  let capturePollInFlight = false;

  function stopCapturePoll() {
    capturePollStopped = true;
    if (captureTimer !== undefined) clearTimeout(captureTimer);
    captureTimer = undefined;
  }

  function scheduleCapturePoll() {
    if (capturePollStopped || capturePollInFlight || captureTimer !== undefined) return;
    captureTimer = setTimeout(() => {
      captureTimer = undefined;
      void pollCapture();
    }, 350);
  }

  async function pollCapture() {
    if (capturePollStopped || capturePollInFlight) return;
    capturePollInFlight = true;
    try {
      await loadDashboard();
    } catch {
      // The final refresh below owns user-visible failure reporting.
    } finally {
      capturePollInFlight = false;
      scheduleCapturePoll();
    }
  }

  function startCapturePoll() {
    stopCapturePoll();
    capturePollStopped = false;
    scheduleCapturePoll();
  }

  watch([showTUNFault, loading], async ([shown, busy]) => {
    if (!shown || busy) return;
    await nextTick();
    tunRetryButton.value?.focus();
  });

  async function toggleConnection() {
    await setCapture(nextPrimaryCaptureMode(dashboard.value.capture.committed_mode));
  }

  async function setCapture(mode: CaptureMode) {
    if (mode !== "off" && captureRouteMissing.value) {
      setRouteRequired("请先前往连接管理添加并选择一条由 Navo 管理的可用线路，再开启系统代理或 TUN；当前外部代理不会被 Navo 借用。");
      return;
    }
    startCapturePoll();
    try {
      await execute(async () => {
        await apis.capture.switchMode(mode);
        await loadDashboard();
      }, `接管模式已切换为${captureLabel(mode)}`);
    } finally {
      stopCapturePoll();
      await loadDashboard().catch((reason) => {
        failure.value ||= `读取接管状态失败：${errorMessage(reason)}`;
      });
    }
  }

  onBeforeUnmount(() => {
    stopCapturePoll();
  });

  return {
    dismissedFaultID,
    tunRetryButton,
    captureMode,
    captureRouteMissing,
    primaryConnectionAction,
    captureTransitioning,
    showTUNFault,
    toggleConnection,
    setCapture,
  };
}
