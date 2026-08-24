"""Browser-visible smoke test for the thin-layer Vue shell using a mock Wails bridge."""

from pathlib import Path

from playwright.sync_api import sync_playwright


URL = "http://127.0.0.1:4187"
EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
OUTPUT = Path(__file__).resolve().parents[1] / ".cache" / "ui-thin-layer-smoke"

BRIDGE = r"""
(() => {
  const clone = (value) => JSON.parse(JSON.stringify(value));
  const dashboard = {
    core: { core_id: "sing-box", state: "running", pid: 4242, uptime_seconds: 180, config_hash: "smoke-revision", restart_count: 0, last_error: "" },
    cores: [
      { id: "sing-box", name: "sing-box", version: "1.13.14", installed: true, active: true, system_proxy_supported: true, tun_supported: true },
      { id: "mihomo", name: "Mihomo", version: "1.19.0", installed: true, active: false, system_proxy_supported: true, tun_supported: true },
      { id: "xray", name: "Xray", version: "25.8.3", installed: true, active: false, system_proxy_supported: true, tun_supported: false },
    ],
    proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
    runtime: { mode: "bypass_mainland", list_mode: "off", selected_id: "route-1", active_id: "route-1", candidate_id: "", tun_enabled: false, blacklist: [], whitelist: [] },
    tun: { installed: true, created: false, enabled: false, name: "Navo", mtu: 1500, state: "disabled", identifier: "", interface_index: 0, fault_id: "", last_error: "" },
    capture: {
      state: "stopped", phase: "stopped", desired_mode: "off", committed_mode: "off", transition_id: "", fault_id: "", last_error: "", can_retry_tun: false,
      readiness: { state: "unverified", scope: "chatgpt", sites: {}, default_proxy: false, checked_at: "", error: "" },
      recovery: { state: "idle", evidence: {}, rounds: [], candidates: [], updated_at: "", final_error: "" },
    },
    metrics: {
      reachable: true, available: true, unavailable_reason: "", core_name: "sing-box", latency_ms: 32, upload_bytes: 2048, download_bytes: 4096, connections: 4,
      local_available: true, local_unavailable_reason: "", local_upload_bps: 1024, local_download_bps: 4096, proxy_upload_bps: 0, proxy_download_bps: 0,
      local_upload_total: 2048, local_download_total: 8192, proxy_upload_total: 0, proxy_download_total: 0, traffic_source_state: "ready", traffic_sampled_at: new Date().toISOString(),
    },
    ip: { proxy_ip: "198.51.100.9", proxy_country: "TEST", direct_ip: "192.0.2.9", probe_pending: false },
  };
  const routes = {
    selected_id: "route-1", active_id: "route-1", candidate_id: "", mode: "bypass_mainland",
    outbounds: [
      { id: "route-1", name: "Smoke Tokyo", type: "vless", server: "edge.example.net", port: 443, source_type: "airport_subscription", country: "JP", selected: true, active: true },
      { id: "route-2", name: "Smoke Osaka", type: "trojan", server: "edge2.example.net", port: 443, source_type: "airport_subscription", country: "JP" },
    ],
  };
  const ipResult = () => ({
    source: { state: "available", available: true, ip: "192.0.2.9", country: "TEST", city: "Direct", asn: "AS64500", isp: "Direct ISP", network: "TEST-NET-1", provider: "smoke", mobile: false, proxy: false, hosting: false, checked_at: new Date().toISOString(), error: "" },
    proxy: { state: dashboard.capture.committed_mode === "off" ? "inactive" : "available", available: dashboard.capture.committed_mode !== "off", ip: "198.51.100.9", country: "TEST", city: "Proxy", asn: "AS64501", isp: "Proxy ISP", network: "TEST-NET-2", provider: "smoke", mobile: false, proxy: true, hosting: false, checked_at: new Date().toISOString(), error: "" },
  });
  window.runtime = {
    EventsOn: (name, callback) => { window.__navoEvents = { name, callback }; return () => undefined; },
    WindowSetBackgroundColour: () => undefined,
    WindowSetLightTheme: () => undefined,
    WindowSetDarkTheme: () => undefined,
  };
  window.go = { main: { App: {
    GetDashboard: async () => clone(dashboard),
    ListRoutes: async () => clone(routes),
    ListSubscriptions: async () => ({ subscriptions: [{ id: "sub-1", name: "Smoke Airport", enabled: true, node_count: 2, last_error: "", skip_tls_verify: false }] }),
    CheckIP: async () => clone(ipResult()),
    GetHostStatus: async () => ({ os: "windows", arch: "amd64", logical_cpu: 8, memory_usage_percent: 36, memory_available_bytes: 8589934592, system_uptime_seconds: 7200, process_uptime_seconds: 180, app_version: "1.0.smoke", go_version: "go1.25" }),
    QueryLogs: async () => ({ entries: [{ id: 1, timestamp: new Date().toISOString(), level: "INFO", service: "ui-smoke", component: "bridge", message: "mock ready" }], next_cursor: 1, has_more: false }),
    GetLogMetadata: async () => ({ levels: ["DEBUG", "INFO", "WARN", "ERROR"], services: ["ui-smoke"] }),
    ClearPersistedLogs: async () => undefined,
    SetCaptureMode: async (mode) => {
      dashboard.capture.desired_mode = mode;
      dashboard.capture.committed_mode = mode;
      dashboard.capture.state = mode === "off" ? "stopped" : mode === "tun" ? "running_tun" : "running_system_proxy";
      dashboard.capture.phase = mode === "off" ? "stopped" : "running";
      dashboard.proxy.enabled = mode === "system_proxy";
      dashboard.runtime.tun_enabled = mode === "tun";
      dashboard.tun.enabled = mode === "tun";
      dashboard.metrics.proxy_upload_bps = mode === "off" ? 0 : 512;
      dashboard.metrics.proxy_download_bps = mode === "off" ? 0 : 2048;
    },
    VerifyCapture: async () => undefined,
    SetRuntimeMode: async (mode) => { dashboard.runtime.mode = mode; routes.mode = mode; },
    SetRoutingListMode: async (mode) => { dashboard.runtime.list_mode = mode; },
    SetRoutingRules: async (blacklist, whitelist) => { dashboard.runtime.blacklist = blacklist; dashboard.runtime.whitelist = whitelist; },
    SetCore: async (id) => { dashboard.core.core_id = id; dashboard.cores.forEach((item) => { item.active = item.id === id; }); },
    SelectRoute: async (id) => { dashboard.runtime.selected_id = id; dashboard.runtime.active_id = id; routes.selected_id = id; routes.active_id = id; routes.outbounds.forEach((item) => { item.selected = item.id === id; item.active = item.id === id; }); },
    TestRoute: async (id) => ({ id, reachable: true, latency_ms: 24, error: "" }),
    RunProxyBenchmark: async () => ({ latency_ms: 24, jitter_ms: 2, download_mbps: 64, upload_mbps: 16, download_bytes: 4194304, upload_bytes: 1048576, proxy_endpoint: "127.0.0.1:12080", test_server: "smoke", checked_at: new Date().toISOString() }),
    RunRouteBenchmark: async (id) => ({ latency_ms: id === "route-1" ? 24 : 31, jitter_ms: 2, download_mbps: 64, upload_mbps: 16, download_bytes: 4194304, upload_bytes: 1048576, proxy_endpoint: "127.0.0.1:12080", test_server: "smoke", checked_at: new Date().toISOString() }),
    CancelProxyBenchmark: async () => undefined,
    RunLatencyTest: async (id) => ({ outbound_id: id, state: "completed", tcp_connect_ms: 8, proxy_handshake_ms: 12, dns_observable: false, dns_ms: 0, tls_ms: 16, ttfb_ms: 22, total_ms: 30, exit_ip: "198.51.100.9", checked_at: new Date().toISOString(), error_code: "", error_message: "" }),
    RunTrafficTransfer: async () => ({ latency_ms: 24, jitter_ms: 2, download_mbps: 64, upload_mbps: 16, download_bytes: 1048576, upload_bytes: 0, proxy_endpoint: "127.0.0.1:12080", test_server: "smoke", checked_at: new Date().toISOString() }),
    CheckCoreUpdates: async () => ({ checked_at: new Date().toISOString(), items: [] }),
    InstallCoreUpdate: async (id) => ({ id, name: id, update_available: false, install_supported: true, integrity_ok: true }),
    OpenCoreRelease: async () => undefined,
    CreateUpstream: async () => undefined,
    DeleteUpstream: async () => undefined,
    AddSubscription: async () => undefined,
    RefreshSubscriptions: async () => undefined,
    RemoveSubscription: async () => undefined,
    RequestExit: async () => undefined,
    MinimizeToTray: async () => undefined,
  } } };
})();
"""


def main() -> None:
    OUTPUT.mkdir(parents=True, exist_ok=True)
    console_errors: list[str] = []
    page_errors: list[str] = []
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(executable_path=EDGE, headless=True)
        page = browser.new_page(viewport={"width": 1440, "height": 1000}, device_scale_factor=1)
        page.on("console", lambda message: console_errors.append(message.text) if message.type == "error" else None)
        page.on("pageerror", lambda error: page_errors.append(str(error)))
        page.add_init_script(BRIDGE)
        page.goto(URL)
        page.wait_for_load_state("networkidle")

        nav = page.locator(".sidebar nav button")
        assert nav.count() == 7, f"expected seven navigation entries, got {nav.count()}"
        pages = [
            ("运行概览", None),
            ("连接管理", "配置本次连接"),
            ("流量监控", "流量与连接趋势"),
            ("网络测速", "当前节点一键测速"),
            ("网络检测", "主机与代理链路检测"),
            ("内核管理", "管理代理内核"),
            ("设置与日志", "设置与诊断"),
        ]
        for navigation_name, heading in pages:
            page.get_by_role("button", name=navigation_name).click()
            page.locator(".task-page").wait_for(state="visible")
            assert page.locator(".page-heading h1").inner_text() == navigation_name
            if heading:
                assert heading in page.locator(".task-page").inner_text(), f"missing heading on {navigation_name}"

        page.get_by_role("button", name="连接管理").click()
        page.locator(".mode-entry-options button", has=page.get_by_text("系统代理", exact=True)).click()
        page.wait_for_function("() => document.body.innerText.includes('系统代理 · 绕过大陆')")
        assert page.locator(".traffic-control-card").get_by_text("系统代理", exact=False).count() > 0

        page.get_by_role("button", name="日", exact=True).click()
        assert page.locator("html").get_attribute("data-theme") == "day"
        page.screenshot(path=str(OUTPUT / "desktop-day.png"), full_page=True)
        page.get_by_role("button", name="夜", exact=True).click()
        assert page.locator("html").get_attribute("data-theme") == "night"
        page.screenshot(path=str(OUTPUT / "desktop-night.png"), full_page=True)

        page.set_viewport_size({"width": 760, "height": 900})
        page.wait_for_timeout(100)
        overflow = page.evaluate("() => document.documentElement.scrollWidth - document.documentElement.clientWidth")
        assert overflow <= 1, f"compact layout overflows horizontally by {overflow}px"
        for navigation_name, _ in pages:
            assert page.get_by_role("button", name=navigation_name).count() == 1
        page.screenshot(path=str(OUTPUT / "compact-night.png"), full_page=True)

        assert not page_errors, f"page errors: {page_errors}"
        assert not console_errors, f"console errors: {console_errors}"
        browser.close()

    print("UI_THIN_LAYER_SMOKE=PASS")
    print("PAGES=7 THEMES=2 COMPACT=760x900 HORIZONTAL_OVERFLOW=0")
    print(f"SCREENSHOTS={OUTPUT}")


if __name__ == "__main__":
    main()
