import { ref, type Ref } from "vue";
import { apis } from "../../api";
import type { Dashboard, StartupSettings } from "../../types";
import type { ApplicationExecute } from "../application/useApplicationFeedback";
import { requiresNavoRoute } from "../runtime/routeRecovery";

interface UseStartupSettingsOptions {
  execute: ApplicationExecute;
  dashboard: Ref<Dashboard>;
  setRouteRequired: (message: string) => void;
}

export function useStartupSettings({ execute, dashboard, setRouteRequired }: UseStartupSettingsOptions) {
  const startupSettings = ref<StartupSettings>();

  async function loadStartupSettings() {
    startupSettings.value = await apis.system.startupSettings();
  }

  async function configureStartup(enabled: boolean, mode: StartupSettings["mode"]) {
    if (enabled && requiresNavoRoute(dashboard.value)) {
      setRouteRequired("开机连接需要一条由 Navo 管理的可用线路。请先前往连接管理添加并选择线路；当前外部代理仅用于只读观测。");
      return;
    }
    await execute(async () => {
      startupSettings.value = await apis.system.configureStartup(enabled, mode);
    }, enabled ? "开机连接已启用" : "开机连接已关闭", "正在更新开机连接");
  }

  return { startupSettings, loadStartupSettings, configureStartup };
}
