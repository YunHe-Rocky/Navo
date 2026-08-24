import type { RoutingListMode, RuntimeMode } from "../types";
import type { BackendProvider } from "./client";

export function createRoutingApi(backend: BackendProvider) {
  return {
    setMode: (mode: RuntimeMode) => backend().SetRuntimeMode(mode),
    setListMode: (mode: RoutingListMode) => backend().SetRoutingListMode(mode),
    setRules: (blacklist: string[], whitelist: string[]) => backend().SetRoutingRules(blacklist, whitelist),
  };
}
