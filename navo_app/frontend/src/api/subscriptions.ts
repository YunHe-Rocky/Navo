import type { SubscriptionRequest, UpstreamRequest } from "../types";
import type { BackendProvider } from "./client";

export function createSubscriptionApi(backend: BackendProvider) {
  return {
    list: () => backend().ListSubscriptions(),
    add: (request: SubscriptionRequest) => backend().AddSubscription(request),
    refresh: () => backend().RefreshSubscriptions(),
    remove: (id: string) => backend().RemoveSubscription(id),
    createUpstream: (request: UpstreamRequest) => backend().CreateUpstream(request),
    deleteUpstream: (id: string) => backend().DeleteUpstream(id),
  };
}
