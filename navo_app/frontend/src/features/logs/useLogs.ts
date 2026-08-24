import { onBeforeUnmount, ref, type Ref } from "vue";
import { apis } from "../../api/index";
import type { LogEntry, LogMetadata, Page } from "../../types";
import type { ApplicationExecute } from "../application/useApplicationFeedback";

interface UseLogsOptions {
  page: Ref<Page>;
  loading: Ref<boolean>;
  execute: ApplicationExecute;
}

export function useLogs({ page, loading, execute }: UseLogsOptions) {
  const logs = ref<LogEntry[]>([]);
  const logMetadata = ref<LogMetadata>({ levels: ["DEBUG", "INFO", "WARN", "ERROR"], services: [] });
  const selectedLogLevels = ref<string[]>(["INFO", "WARN", "ERROR"]);
  const selectedLogServices = ref<string[]>([]);
  const logFrom = ref("");
  const logTo = ref("");
  const logCursor = ref(0);
  const logHasMore = ref(false);
  const logFollow = ref(false);
  let logTimer: ReturnType<typeof setInterval> | undefined;

  function query(afterID: number) {
    return apis.logs.query({
      levels: selectedLogLevels.value,
      services: selectedLogServices.value,
      from: logFrom.value ? new Date(logFrom.value).toISOString() : "",
      to: logTo.value ? new Date(logTo.value).toISOString() : "",
      after_id: afterID,
      limit: 200,
    });
  }

  async function loadLogs() {
    logMetadata.value = await apis.logs.metadata();
    const result = await query(0);
    logs.value = result.entries ?? [];
    logCursor.value = result.next_cursor || 0;
    logHasMore.value = result.has_more;
  }

  async function refreshLogs() {
    await execute(loadLogs, "诊断日志已更新", "正在读取诊断日志");
  }

  async function loadMoreLogs() {
    const result = await query(logCursor.value);
    logs.value.push(...(result.entries ?? []));
    logCursor.value = result.next_cursor || logCursor.value;
    logHasMore.value = result.has_more;
  }

  function toggleLogSelection(target: "level" | "service", value: string, checked: boolean) {
    const selected = target === "level" ? selectedLogLevels : selectedLogServices;
    const values = new Set(selected.value);
    checked ? values.add(value) : values.delete(value);
    selected.value = [...values];
  }

  function clearVisibleLogs() {
    logs.value = [];
    logCursor.value = 0;
    logHasMore.value = false;
  }

  async function clearPersistedLogs() {
    if (!window.confirm("仅清空 Navo 结构化文本日志；不会删除崩溃转储和诊断导出。继续？")) return;
    await execute(async () => {
      await apis.logs.clear();
      clearVisibleLogs();
    }, "持久化日志已清空", "正在安全轮转日志");
  }

  function pauseFollowPolling() {
    if (logTimer) clearInterval(logTimer);
    logTimer = undefined;
  }

  function setLogFollow(enabled: boolean) {
    logFollow.value = enabled;
    pauseFollowPolling();
    logTimer = enabled ? setInterval(() => {
      if (page.value === "settings" && !loading.value) void loadMoreLogs();
    }, 3000) : undefined;
  }

  onBeforeUnmount(pauseFollowPolling);

  return {
    logs,
    logMetadata,
    selectedLogLevels,
    selectedLogServices,
    logFrom,
    logTo,
    logHasMore,
    logFollow,
    loadLogs,
    refreshLogs,
    loadMoreLogs,
    toggleLogSelection,
    clearVisibleLogs,
    clearPersistedLogs,
    setLogFollow,
    pauseFollowPolling,
  };
}
