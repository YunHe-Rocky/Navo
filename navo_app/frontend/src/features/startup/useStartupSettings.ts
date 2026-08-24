import { ref } from "vue";
import { apis } from "../../api";
import type { StartupSettings } from "../../types";
import type { ApplicationExecute } from "../application/useApplicationFeedback";

export function useStartupSettings(execute: ApplicationExecute) {
  const startupSettings = ref<StartupSettings>();

  async function loadStartupSettings() {
    startupSettings.value = await apis.system.startupSettings();
  }

  async function configureStartup(enabled: boolean, mode: StartupSettings["mode"]) {
    await execute(async () => {
      startupSettings.value = await apis.system.configureStartup(enabled, mode);
    }, enabled ? "开机连接已启用" : "开机连接已关闭", "正在更新开机连接");
  }

  return { startupSettings, loadStartupSettings, configureStartup };
}
