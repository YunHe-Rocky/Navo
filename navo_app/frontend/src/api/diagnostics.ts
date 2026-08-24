import type { BackendProvider } from "./client";

export function createDiagnosticsApi(backend: BackendProvider) {
  return {
    checkIP: () => backend().CheckIP(),
    hostStatus: () => backend().GetHostStatus(),
    runProxyBenchmark: () => backend().RunProxyBenchmark(),
    runRouteBenchmark: (id: string) => backend().RunRouteBenchmark(id),
    cancelProxyBenchmark: () => backend().CancelProxyBenchmark(),
    runLatencyTest: (id: string) => backend().RunLatencyTest(id),
    runTrafficTransfer: (sizeMiB: number, direction: string) => backend().RunTrafficTransfer(sizeMiB, direction),
  };
}
