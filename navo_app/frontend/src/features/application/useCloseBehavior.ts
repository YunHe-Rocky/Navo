import { nextTick, onBeforeUnmount, onMounted, ref, type Ref } from "vue";
import { apis } from "../../api/index";
import { CLOSE_PREFERENCE_KEY, createClosePreference, resolveClosePreference } from "../../close-preference.js";
import type { HostStatus } from "../../types";
import { errorMessage } from "./formatters";

export type CloseAction = "minimize" | "exit";

export function useCloseBehavior(hostStatus: Ref<HostStatus | undefined>) {
  const closeDialogOpen = ref(false);
  const closeAction = ref<CloseAction>("minimize");
  const rememberCloseAction = ref(false);
  const closeActionBusy = ref(false);
  const closeActionError = ref("");
  const closePrimaryButton = ref<HTMLButtonElement>();
  let stopCloseRequestListener: (() => void) | undefined;

  function storedCloseAction(): CloseAction | undefined {
    const uptime = hostStatus.value?.system_uptime_seconds;
    if (!Number.isFinite(uptime)) return undefined;
    return resolveClosePreference(localStorage.getItem(CLOSE_PREFERENCE_KEY), Date.now(), uptime!);
  }

  async function performCloseAction(action: CloseAction, persist: boolean, preserveExisting = false) {
    if (closeActionBusy.value) return;
    closeActionError.value = "";
    if (persist && Number.isFinite(hostStatus.value?.system_uptime_seconds)) {
      localStorage.setItem(CLOSE_PREFERENCE_KEY, createClosePreference(
        action,
        Date.now(),
        hostStatus.value!.system_uptime_seconds,
      ));
    } else if (!persist && !preserveExisting) {
      localStorage.removeItem(CLOSE_PREFERENCE_KEY);
    }

    if (action === "minimize") {
      closeActionBusy.value = true;
      try {
        await apis.system.minimizeToTray();
        closeDialogOpen.value = false;
        closeActionBusy.value = false;
      } catch (reason) {
        closeActionBusy.value = false;
        closeActionError.value = errorMessage(reason);
      }
      return;
    }

    closeActionBusy.value = true;
    try {
      await apis.system.requestExit();
    } catch (reason) {
      closeActionBusy.value = false;
      closeActionError.value = errorMessage(reason);
    }
  }

  function requestCloseChoice() {
    const remembered = storedCloseAction();
    if (remembered) {
      void performCloseAction(remembered, false, true);
      return;
    }
    closeAction.value = "minimize";
    rememberCloseAction.value = false;
    closeActionError.value = "";
    closeDialogOpen.value = true;
    void nextTick(() => closePrimaryButton.value?.focus());
  }

  function dismissCloseChoice() {
    if (!closeActionBusy.value) closeDialogOpen.value = false;
  }

  onMounted(() => {
    stopCloseRequestListener = apis.system.onCloseRequested(requestCloseChoice);
  });
  onBeforeUnmount(() => stopCloseRequestListener?.());

  return {
    closeDialogOpen,
    closeAction,
    rememberCloseAction,
    closeActionBusy,
    closeActionError,
    closePrimaryButton,
    performCloseAction,
    dismissCloseChoice,
  };
}
