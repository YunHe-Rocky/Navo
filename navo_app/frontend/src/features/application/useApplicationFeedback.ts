import { onBeforeUnmount, ref } from "vue";
import type { UIState } from "../../state/ui";
import { errorMessage } from "./formatters";

export interface ApplicationFeedbackHooks {
  onActivityBegin?: () => void;
}

export type ApplicationExecute = (
  action: () => Promise<void>,
  success?: string,
  progressLabel?: string,
) => Promise<boolean>;

export function useApplicationFeedback(initial: UIState, hooks: ApplicationFeedbackHooks = {}) {
  const loading = ref(initial.loading);
  const notice = ref(initial.notice);
  const failure = ref(initial.failure);
  const routeRequired = ref(false);
  const activityVisible = ref(false);
  const activityLabel = ref(initial.activityLabel);
  const activityProgress = ref(0);
  let activityTimer: ReturnType<typeof setInterval> | undefined;
  let activityHideTimer: ReturnType<typeof setTimeout> | undefined;

  function beginActivity(label: string) {
    if (activityTimer) clearInterval(activityTimer);
    if (activityHideTimer) clearTimeout(activityHideTimer);
    hooks.onActivityBegin?.();
    activityLabel.value = label;
    activityProgress.value = 8;
    activityVisible.value = true;
    activityTimer = setInterval(() => {
      activityProgress.value = Math.min(88, activityProgress.value + Math.max(1, Math.round((92 - activityProgress.value) / 7)));
    }, 140);
  }

  function finishActivity() {
    if (activityTimer) clearInterval(activityTimer);
    activityTimer = undefined;
    activityProgress.value = 100;
    activityHideTimer = setTimeout(() => {
      activityVisible.value = false;
      activityProgress.value = 0;
    }, 520);
  }

  const execute: ApplicationExecute = async (action, success = "", progressLabel = "") => {
    loading.value = true;
    failure.value = "";
    notice.value = "";
    routeRequired.value = false;
    beginActivity(progressLabel || success || "正在处理请求");
    try {
      await action();
      notice.value = success;
      return true;
    } catch (reason) {
      failure.value = errorMessage(reason);
      return false;
    } finally {
      loading.value = false;
      finishActivity();
    }
  };

  function setRouteRequired(message: string) {
    notice.value = "";
    failure.value = message;
    routeRequired.value = true;
  }

  onBeforeUnmount(() => {
    if (activityTimer) clearInterval(activityTimer);
    if (activityHideTimer) clearTimeout(activityHideTimer);
  });

  return {
    loading,
    notice,
    failure,
    routeRequired,
    activityVisible,
    activityLabel,
    activityProgress,
    beginActivity,
    finishActivity,
    execute,
    setRouteRequired,
  };
}
