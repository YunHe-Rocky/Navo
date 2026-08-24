import { computed, ref, type Ref } from "vue";
import { apis } from "../../api/index";
import type { CoreUpdateReport, Dashboard } from "../../types";
import { errorMessage } from "../application/formatters";
import type { ApplicationExecute } from "../application/useApplicationFeedback";

interface UseCoreOptions {
  dashboard: Ref<Dashboard>;
  loadDashboard: () => Promise<void>;
  execute: ApplicationExecute;
  failure: Ref<string>;
  beginActivity: (label: string) => void;
  finishActivity: () => void;
}

export function useCore({ dashboard, loadDashboard, execute, failure, beginActivity, finishActivity }: UseCoreOptions) {
  const coreUpdateReport = ref<CoreUpdateReport>();
  const coreUpdateChecking = ref(false);
  const coreUpdateInstalling = ref<Record<string, boolean>>({});
  const showAdvancedCore = ref(false);
  const activeCore = computed(() => dashboard.value.cores.find((item) => item.active || item.id === dashboard.value.core.core_id));
  const activeCoreSupportsTUN = computed(() => activeCore.value?.tun_supported !== false);
  const coreUpdates = computed(() => Object.fromEntries(
    (coreUpdateReport.value?.items ?? []).map((item) => [item.id, item]),
  ));

  async function checkCoreUpdates() {
    if (coreUpdateChecking.value) return;
    coreUpdateChecking.value = true;
    failure.value = "";
    beginActivity("正在校验内核并检查官方版本");
    try {
      coreUpdateReport.value = await apis.core.checkUpdates();
    } catch (reason) {
      failure.value = `内核更新检查失败：${errorMessage(reason)}`;
    } finally {
      coreUpdateChecking.value = false;
      finishActivity();
    }
  }

  async function openCoreRelease(id: string) {
    try {
      await apis.core.openRelease(id);
    } catch (reason) {
      failure.value = `无法打开官方发布页：${errorMessage(reason)}`;
    }
  }

  async function installCoreUpdate(id: string) {
    if (coreUpdateInstalling.value[id]) return;
    const update = coreUpdates.value[id];
    if (!update?.update_available || !update.install_supported) return;
    if (!window.confirm(`升级 ${update.name} 到 ${update.latest_version}？升级期间会短暂关闭网络接管，完成后自动恢复。`)) return;
    coreUpdateInstalling.value = { ...coreUpdateInstalling.value, [id]: true };
    failure.value = "";
    beginActivity(`正在验证并升级 ${update.name}`);
    try {
      const installed = await apis.core.installUpdate(id);
      coreUpdateReport.value = {
        items: (coreUpdateReport.value?.items ?? []).map((item) => item.id === id ? installed : item),
        checked_at: new Date().toISOString(),
      };
      await loadDashboard();
    } catch (reason) {
      failure.value = `内核升级失败：${errorMessage(reason)}`;
    } finally {
      coreUpdateInstalling.value = { ...coreUpdateInstalling.value, [id]: false };
      finishActivity();
    }
  }

  async function setCore(id: string) {
    await execute(async () => {
      await apis.core.select(id);
      await loadDashboard();
    }, `已切换到 ${id}`);
  }

  return {
    coreUpdateReport,
    coreUpdateChecking,
    coreUpdateInstalling,
    showAdvancedCore,
    activeCore,
    activeCoreSupportsTUN,
    coreUpdates,
    checkCoreUpdates,
    openCoreRelease,
    installCoreUpdate,
    setCore,
  };
}
