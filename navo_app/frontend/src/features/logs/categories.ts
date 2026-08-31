import type { LogMetadata } from "../../types";

export const DEFAULT_LOG_CATEGORY = "basic_service";

export const LOG_CATEGORIES = [
  { id: DEFAULT_LOG_CATEGORY, label: "基础服务" },
  { id: "network_capture", label: "网络与接管" },
  { id: "core_runtime", label: "核心运行" },
  { id: "subscription_update", label: "订阅与更新" },
  { id: "other", label: "其他服务" },
] as const;

export function defaultLogCategorySelection(): string[] {
  return [DEFAULT_LOG_CATEGORY];
}

export function logCategoryLabel(category: string): string {
  return LOG_CATEGORIES.find((item) => item.id === category)?.label ?? "其他服务";
}

export function normalizeLogMetadata(metadata: LogMetadata): LogMetadata {
  return {
    levels: metadata.levels?.length ? metadata.levels : ["DEBUG", "INFO", "WARN", "ERROR"],
    categories: metadata.categories?.length
      ? metadata.categories
      : LOG_CATEGORIES.map((item) => item.id),
    services: metadata.services ?? [],
  };
}
