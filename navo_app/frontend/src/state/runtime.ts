import type { Dashboard } from "../types";

export function createEmptyDashboard(): Dashboard {
  return {
    core: { core_id: "sing-box", state: "stopped", pid: 0, uptime_seconds: 0, config_hash: "", restart_count: 0, last_error: "" },
    cores: [],
    proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
    runtime: { mode: "bypass_mainland", list_mode: "off", selected_id: "", active_id: "", candidate_id: "", tun_enabled: false, blacklist: [], whitelist: [] },
    tun: {
      installed: false, created: false, enabled: false, name: "Navo", mtu: 1500,
      state: "missing", identifier: "", interface_index: 0, fault_id: "", last_error: "",
    },
    capture: {
      state: "stopped", phase: "stopped", desired_mode: "off", committed_mode: "off",
      transition_id: "", fault_id: "", last_error: "", can_retry_tun: false,
      updated_at: "", readiness: { state: "unverified", scope: "chatgpt", sites: {}, default_proxy: false, checked_at: "" },
    },
    metrics: {
      reachable: false, available: false, unavailable_reason: "", core_name: "", latency_ms: 0,
      upload_bytes: 0, download_bytes: 0, connections: 0,
      local_available: false, local_unavailable_reason: "",
      local_upload_bps: 0, local_download_bps: 0, proxy_upload_bps: 0, proxy_download_bps: 0,
      local_upload_total: 0, local_download_total: 0, proxy_upload_total: 0, proxy_download_total: 0,
      traffic_source_state: "unavailable", traffic_sampled_at: "",
    },
    ip: {
      connection_kind: "direct",
      proxy_ip: "", proxy_country: "", direct_ip: "",
      proxy_error: "", direct_error: "",
      proxy_provider: "", direct_provider: "",
      proxy_checked_at: "", direct_checked_at: "",
      probe_pending: false,
    },
  };
}

export function normalizeDashboard(snapshot?: Partial<Dashboard>): Dashboard {
  const empty = createEmptyDashboard();
  return {
    ...empty,
    ...snapshot,
    core: { ...empty.core, ...snapshot?.core },
    proxy: { ...empty.proxy, ...snapshot?.proxy },
    runtime: { ...empty.runtime, ...snapshot?.runtime },
    tun: { ...empty.tun, ...snapshot?.tun },
    capture: {
      ...empty.capture,
      ...snapshot?.capture,
      readiness: { ...empty.capture.readiness, ...snapshot?.capture?.readiness },
    },
    metrics: { ...empty.metrics, ...snapshot?.metrics },
    ip: { ...empty.ip, ...snapshot?.ip },
    environment: snapshot?.environment ? {
      ...snapshot.environment,
      findings: snapshot.environment.findings ?? [],
      observation_errors: snapshot.environment.observation_errors ?? [],
    } : undefined,
    cores: snapshot?.cores ?? [],
  };
}

export function createDashboardSnapshotLoader(
  fetchSnapshot: () => Promise<Partial<Dashboard>>,
  commit: (snapshot: Dashboard) => void,
): () => Promise<Dashboard> {
  let requestedGeneration = 0;
  let inFlight: Promise<Dashboard> | undefined;

  async function drain(): Promise<Dashboard> {
    for (;;) {
      const generation = requestedGeneration;
      try {
        const snapshot = normalizeDashboard(await fetchSnapshot());
        if (generation !== requestedGeneration) continue;
        commit(snapshot);
        return snapshot;
      } catch (error) {
        if (generation === requestedGeneration) throw error;
      }
    }
  }

  return function loadDashboardSnapshot(): Promise<Dashboard> {
    requestedGeneration += 1;
    if (!inFlight) {
      inFlight = drain().finally(() => {
        inFlight = undefined;
      });
    }
    return inFlight;
  };
}
