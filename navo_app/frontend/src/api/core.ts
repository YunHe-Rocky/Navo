import type { BackendProvider } from "./client";

export function createCoreApi(backend: BackendProvider) {
  return {
    select: (id: string) => backend().SetCore(id),
    checkUpdates: () => backend().CheckCoreUpdates(),
    installUpdate: (id: string) => backend().InstallCoreUpdate(id),
    openRelease: (id: string) => backend().OpenCoreRelease(id),
  };
}
