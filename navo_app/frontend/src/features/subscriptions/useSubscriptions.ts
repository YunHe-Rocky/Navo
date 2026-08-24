import { ref, type Ref } from "vue";
import { apis } from "../../api/index";
import type { RouteInfo, SourceType, SubscriptionInfo, UpstreamRequest } from "../../types";
import type { ApplicationExecute } from "../application/useApplicationFeedback";

interface UseSubscriptionsOptions {
  sourceFilter: Ref<SourceType>;
  loadRoutes: () => Promise<void>;
  execute: ApplicationExecute;
}

export function useSubscriptions({ sourceFilter, loadRoutes, execute }: UseSubscriptionsOptions) {
  const subscriptions = ref<SubscriptionInfo[]>([]);
  const showUpstreamForm = ref(false);
  const showSubscriptionForm = ref(false);
  const upstream = ref<UpstreamRequest>({
    name: "", proto: "socks5", server: "", port: 1080,
    username: "", password: "", udp_policy: "disabled",
  });
  const subscription = ref({ name: "", url: "", skip_tls_verify: false });

  async function loadSubscriptions() {
    subscriptions.value = (await apis.subscriptions.list()).subscriptions ?? [];
  }

  async function createUpstream() {
    await execute(async () => {
      await apis.subscriptions.createUpstream(upstream.value);
      upstream.value = { name: "", proto: "socks5", server: "", port: 1080, username: "", password: "", udp_policy: "disabled" };
      showUpstreamForm.value = false;
      sourceFilter.value = "upstream_proxy";
      await loadRoutes();
    }, "独享代理已保存并启用");
  }

  async function deleteUpstream(item: RouteInfo) {
    if (!window.confirm(`删除独享代理“${item.name}”？此操作会同时移除加密凭据。`)) return;
    await execute(async () => {
      await apis.subscriptions.deleteUpstream(item.id);
      await loadRoutes();
    }, "独享代理已删除");
  }

  async function addSubscription() {
    await execute(async () => {
      await apis.subscriptions.add(subscription.value);
      subscription.value = { name: "", url: "", skip_tls_verify: false };
      showSubscriptionForm.value = false;
      await Promise.all([loadSubscriptions(), loadRoutes()]);
    }, "订阅已保存并同步");
  }

  async function removeSubscription(item: SubscriptionInfo) {
    if (!window.confirm(`删除机场订阅“${item.name}”及其节点？`)) return;
    await execute(async () => {
      await apis.subscriptions.remove(item.id);
      await Promise.all([loadSubscriptions(), loadRoutes()]);
    }, "订阅已删除");
  }

  async function refreshAllSubscriptions() {
    await execute(async () => {
      await apis.subscriptions.refresh();
      await Promise.all([loadSubscriptions(), loadRoutes()]);
    }, "订阅同步完成");
  }

  return {
    subscriptions,
    showUpstreamForm,
    showSubscriptionForm,
    upstream,
    subscription,
    loadSubscriptions,
    createUpstream,
    deleteUpstream,
    addSubscription,
    removeSubscription,
    refreshAllSubscriptions,
  };
}
