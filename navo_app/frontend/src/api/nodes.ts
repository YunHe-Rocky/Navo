import type { BackendProvider } from "./client";

export function createNodeApi(backend: BackendProvider) {
  return {
    list: () => backend().ListRoutes(),
    select: (id: string) => backend().SelectRoute(id),
    test: (id: string) => backend().TestRoute(id),
  };
}
