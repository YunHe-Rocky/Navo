import type { BackendProvider } from "./client";

export function createRuntimeApi(backend: BackendProvider) {
  return {
    dashboard: () => backend().GetDashboard(),
    repairEnvironment: (code: string) => backend().RepairNetworkEnvironment(code),
  };
}
