import type { LogQuery } from "../types";
import type { BackendProvider } from "./client";

export function createLogApi(backend: BackendProvider) {
  return {
    query: (query: LogQuery) => backend().QueryLogs(query),
    metadata: () => backend().GetLogMetadata(),
    clear: () => backend().ClearPersistedLogs(),
  };
}
