import { computed, nextTick, reactive, ref, type Ref } from "vue";
import { apis } from "../../api/index";
import type { Dashboard, RoutingListMode, RuntimeMode } from "../../types";
import type { ApplicationExecute } from "../application/useApplicationFeedback";
import { routingListModeLabel, runtimeModeLabel } from "../runtime/presenter";

interface UseRoutingOptions {
  dashboard: Ref<Dashboard>;
  loadDashboard: () => Promise<void>;
  execute: ApplicationExecute;
  failure: Ref<string>;
}

export function useRouting({ dashboard, loadDashboard, execute, failure }: UseRoutingOptions) {
  const routingRuleDrafts = reactive({ blacklist: "", whitelist: "" });
  const routingRuleDirty = reactive({ blacklist: false, whitelist: false });
  const routingListEditor = ref<Exclude<RoutingListMode, "off"> | null>(null);
  const routingListTextarea = ref<HTMLTextAreaElement>();
  const runtimeMode = computed(() => dashboard.value.runtime.mode);
  const routingListMode = computed(() => dashboard.value.runtime.list_mode ?? "off");
  const routingRuleCounts = computed(() => ({
    blacklist: parseRoutingRuleDraft(routingRuleDrafts.blacklist).length,
    whitelist: parseRoutingRuleDraft(routingRuleDrafts.whitelist).length,
  }));

  function syncRoutingRuleDrafts() {
    for (const mode of ["blacklist", "whitelist"] as const) {
      if (routingRuleDirty[mode]) continue;
      routingRuleDrafts[mode] = (dashboard.value.runtime[mode] ?? []).join("\n");
    }
  }

  function parseRoutingRuleDraft(value: string) {
    return [...new Set(value.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean))];
  }

  async function setRuntimeMode(mode: RuntimeMode) {
    await execute(async () => {
      await apis.routing.setMode(mode);
      await loadDashboard();
    }, `流量走向已切换为${runtimeModeLabel(mode)}`);
  }

  async function setRoutingListMode(mode: RoutingListMode) {
    let committed = false;
    const refreshed = await execute(async () => {
      await apis.routing.setListMode(mode);
      committed = true;
      await loadDashboard();
    }, `名单模式已切换为${routingListModeLabel(mode)}`);
    if (!committed) return;
    if (!refreshed) {
      failure.value = `名单模式已切换为${routingListModeLabel(mode)}，但状态刷新失败；界面将自动重试同步。`;
      return;
    }
    routingListEditor.value = dashboard.value.runtime.list_mode === mode && mode !== "off" ? mode : null;
    if (routingListEditor.value) {
      await nextTick();
      routingListTextarea.value?.focus();
    }
  }

  async function activateRoutingList(mode: Exclude<RoutingListMode, "off">) {
    if (routingListMode.value === mode) {
      routingListEditor.value = mode;
      await nextTick();
      routingListTextarea.value?.focus();
      return;
    }
    await setRoutingListMode(mode);
  }

  async function saveRoutingRules() {
    await execute(async () => {
      await apis.routing.setRules(
        parseRoutingRuleDraft(routingRuleDrafts.blacklist),
        parseRoutingRuleDraft(routingRuleDrafts.whitelist),
      );
      routingRuleDirty.blacklist = false;
      routingRuleDirty.whitelist = false;
      await loadDashboard();
    }, "名单规则已保存并应用", "正在验证名单规则");
  }

  function clearRoutingRules(mode: "blacklist" | "whitelist") {
    routingRuleDrafts[mode] = "";
    routingRuleDirty[mode] = true;
  }

  return {
    routingRuleDrafts,
    routingRuleDirty,
    routingListEditor,
    routingListTextarea,
    runtimeMode,
    routingListMode,
    routingRuleCounts,
    syncRoutingRuleDrafts,
    setRuntimeMode,
    setRoutingListMode,
    activateRoutingList,
    saveRoutingRules,
    clearRoutingRules,
  };
}
