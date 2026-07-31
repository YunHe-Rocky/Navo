"""Compatibility entrypoint for environments that launch smoke tests via Python."""

import subprocess
from pathlib import Path


DASHBOARD = {
    "core": {
        "core_id": "sing-box",
        "state": "stopped",
        "pid": 0,
        "uptime": 0,
        "config_hash": "",
        "restart_count": 0,
        "last_error": "",
    },
    "cores": [
        {
            "id": "sing-box",
            "name": "sing-box",
            "version": "1.13.14",
            "installed": True,
            "active": True,
        }
    ],
    "proxy": {"enabled": False, "server": "127.0.0.1", "port": 12080},
    "runtime": {"mode": "global", "active_id": "node-1", "tun_enabled": False},
    "tun": {
        "installed": True,
        "created": False,
        "enabled": False,
        "name": "Navo",
        "mtu": 1500,
        "state": "missing",
        "identifier": "",
        "interface_index": 0,
        "fault_id": "",
        "last_error": "",
    },
    "capture": {
        "state": "stopped",
        "phase": "stopped",
        "desired_mode": "off",
        "committed_mode": "off",
        "transition_id": "",
        "fault_id": "",
        "last_error": "",
        "can_retry_tun": False,
    },
    "metrics": {
        "reachable": False,
        "available": True,
        "unavailable_reason": "",
        "core_name": "sing-box",
        "latency_ms": 0,
        "upload_bytes": 0,
        "download_bytes": 0,
        "connections": 0,
    },
    "ip": {"proxy_ip": "", "proxy_country": "", "direct_ip": ""},
}

ROUTES = {
    "active_id": "node-1",
    "mode": "global",
    "outbounds": [
        {
            "id": "node-1",
            "name": "Smoke Route",
            "type": "vless",
            "server": "edge.example.net",
            "port": 443,
            "provider_id": "provider-1",
            "source_type": "airport_subscription",
            "country": "JP",
            "active": True,
        }
    ],
}

BRIDGE = """
({ dashboardValue, routesValue }) => {
  const clone = (value) => JSON.parse(JSON.stringify(value));
  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const setPhase = (state, phase) => {
    dashboardValue.capture.state = state;
    dashboardValue.capture.phase = phase;
  };
  window.__navoSetTunFault = () => {
    dashboardValue.core.state = "stopped";
    dashboardValue.runtime.tun_enabled = false;
    dashboardValue.tun.enabled = false;
    dashboardValue.tun.state = "disabled";
    dashboardValue.tun.fault_id = "fault-ui-smoke";
    dashboardValue.tun.last_error = "虚拟网卡被外部禁用，已安全回滚";
    Object.assign(dashboardValue.capture, {
      state: "faulted",
      phase: "faulted",
      desired_mode: "tun",
      committed_mode: "off",
      fault_id: "fault-ui-smoke",
      last_error: "虚拟网卡被外部禁用，已安全回滚",
      can_retry_tun: true,
    });
  };
  window.go = { main: { App: {
    GetDashboard: async () => clone(dashboardValue),
    ListRoutes: async () => clone(routesValue),
    ListSubscriptions: async () => ({ subscriptions: [] }),
    CheckIP: async () => ({
      source: { ip: "192.0.2.1", country: "TEST", city: "", asn: "", isp: "", network: "", provider: "smoke", mobile: false, proxy: false, hosting: false, checked_at: "", error: "" },
      proxy: { ip: "198.51.100.1", country: "TEST", city: "", asn: "", isp: "", network: "", provider: "smoke", mobile: false, proxy: true, hosting: false, checked_at: "", error: "" },
    }),
    SetCaptureMode: async (mode) => {
      if (mode !== "tun") return;
      Object.assign(dashboardValue.capture, {
        state: "starting_tun",
        phase: "stopping_old_mode",
        desired_mode: "tun",
        transition_id: "capture-ui-smoke",
      });
      await wait(450);
      setPhase("recovering", "recovering_adapter");
      await wait(450);
      setPhase("starting_tun", "starting_core");
      await wait(450);
      setPhase("starting_tun", "configuring_routes");
      await wait(450);
      setPhase("starting_tun", "checking_connection");
      await wait(450);
      dashboardValue.core.state = "running";
      dashboardValue.runtime.tun_enabled = true;
      Object.assign(dashboardValue.tun, {
        created: true,
        enabled: true,
        state: "enabled",
        identifier: "{00000000-0000-0000-0000-000000000001}",
        interface_index: 42,
      });
      Object.assign(dashboardValue.capture, {
        state: "running_tun",
        phase: "running",
        desired_mode: "tun",
        committed_mode: "tun",
        transition_id: "",
      });
    },
    SetCore: async () => undefined,
    SetCoreRunning: async () => undefined,
    SetSystemProxy: async () => undefined,
    SetTUN: async () => undefined,
    SetRuntimeMode: async () => undefined,
    SelectRoute: async () => undefined,
    TestRoute: async () => ({ id: "node-1", reachable: true, latency_ms: 20, error: "" }),
    CreateUpstream: async () => undefined,
    DeleteUpstream: async () => undefined,
    AddSubscription: async () => undefined,
    RefreshSubscriptions: async () => undefined,
    RemoveSubscription: async () => undefined,
    TailLogs: async () => ({ lines: [] }),
  } } };
}
"""


def main() -> None:
    script = Path(__file__).with_suffix(".cjs")
    subprocess.run(["node", str(script)], check=True)


if __name__ == "__main__":
    main()
