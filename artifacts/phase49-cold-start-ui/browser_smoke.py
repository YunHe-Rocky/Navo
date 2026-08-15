import json
import os
import re
from pathlib import Path

from playwright.sync_api import sync_playwright


ARTIFACT_DIR = Path(os.environ.get("NAVO_UI_ARTIFACT_DIR", Path(__file__).resolve().parent)).resolve()
ARTIFACT_DIR.mkdir(parents=True, exist_ok=True)
EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
BASE_URL = os.environ.get("NAVO_UI_BASE_URL", "http://127.0.0.1:4193")


MOCK_BRIDGE = r"""
(() => {
  const dashboard = {
    core: { core_id: "sing-box", state: "stopped", pid: 0, uptime_seconds: 0, config_hash: "", restart_count: 0, last_error: "" },
    cores: [{ id: "sing-box", name: "sing-box", version: "1.13.14", installed: true, active: true, capture_modes: ["system_proxy", "tun"], system_proxy_supported: true, tun_supported: true }],
    proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
    runtime: { mode: "bypass_mainland", list_mode: "off", active_id: "route-1", tun_enabled: false, blacklist: ["chatgpt.com"], whitelist: ["example.cn"] },
    tun: { installed: true, created: false, enabled: false, name: "Navo", mtu: 1500, state: "missing", identifier: "", interface_index: 0, fault_id: "", last_error: "" },
    capture: { state: "stopped", phase: "stopped", desired_mode: "off", committed_mode: "off", transition_id: "", fault_id: "", last_error: "", can_retry_tun: true },
    metrics: { reachable: false, available: false, unavailable_reason: "", core_name: "", latency_ms: 0, upload_bytes: 0, download_bytes: 0, connections: 0, local_available: false, local_unavailable_reason: "", local_upload_bps: 0, local_download_bps: 0, proxy_upload_bps: 0, proxy_download_bps: 0, local_upload_total: 0, local_download_total: 0, proxy_upload_total: 0, proxy_download_total: 0, traffic_source_state: "unavailable", traffic_sampled_at: "" },
    ip: { proxy_ip: "", proxy_country: "", direct_ip: "", probe_pending: false }
  };
  const clone = value => JSON.parse(JSON.stringify(value));
  const app = {
    GetDashboard: async () => {
      if (window.__navoFailNextDashboard) {
        window.__navoFailNextDashboard = false;
        throw new Error("simulated dashboard refresh failure");
      }
      return clone(dashboard);
    },
    ListRoutes: async () => ({ outbounds: [{ id: "route-1", name: "测试节点", type: "socks5", server: "127.0.0.1", port: 10808, provider_id: "", source_type: "upstream_proxy", country: "", active: true }], active_id: "route-1", mode: dashboard.runtime.mode }),
    ListSubscriptions: async () => ({ subscriptions: [] }),
    GetHostStatus: async () => ({ os: "windows", arch: "amd64", app_version: "1.0.19", go_version: "go1.24", logical_cpu: 8, memory_total_bytes: 1, memory_available_bytes: 1, memory_usage_percent: 0, system_uptime_seconds: 1, process_uptime_seconds: 1 }),
    CheckIP: async () => ({ source: {}, proxy: {} }),
    SetRoutingListMode: async mode => { dashboard.runtime.list_mode = mode; },
    SetRoutingRules: async (blacklist, whitelist) => { dashboard.runtime.blacklist = blacklist; dashboard.runtime.whitelist = whitelist; },
    SetCaptureMode: async mode => { dashboard.capture.committed_mode = mode; dashboard.capture.desired_mode = mode; },
    SetRuntimeMode: async mode => { dashboard.runtime.mode = mode; }
  };
  window.go = { main: { App: new Proxy(app, { get: (target, key) => target[key] || (async () => ({})) }) } };
  window.runtime = new Proxy({ EventsOnMultiple: () => () => {} }, { get: (target, key) => target[key] || (() => {}) });
  window.__navoMockDashboard = dashboard;
})();
"""


def peer_button(page, label):
    group = page.get_by_role("group", name="系统代理、TUN 与黑白名单")
    return group.get_by_role("button").filter(has_text=re.compile(rf"^{label}"))


def open_connection(page):
    page.goto(BASE_URL)
    page.wait_for_load_state("networkidle")
    page.get_by_role("button", name=re.compile(r"^连接管理：")).click()
    page.get_by_role("heading", name="连接管理").wait_for()


def assert_default_state(page):
    group = page.get_by_role("group", name="系统代理、TUN 与黑白名单")
    labels = [button.locator("strong").inner_text() for button in group.get_by_role("button").all()]
    assert labels == ["系统代理", "TUN 代理", "黑名单", "白名单"], labels
    assert page.locator("#routing-list-editor").count() == 0
    for label in ("黑名单", "白名单"):
        button = peer_button(page, label)
        assert button.get_attribute("aria-pressed") == "false"
        assert button.get_attribute("aria-expanded") == "false"


def assert_list_interaction(page):
    blacklist = peer_button(page, "黑名单")
    page.evaluate("window.__navoFailNextDashboard = true")
    blacklist.click()
    page.locator("#blacklist-rules").wait_for()
    assert blacklist.get_attribute("aria-pressed") == "true"
    assert blacklist.get_attribute("aria-expanded") == "true"
    assert page.locator("#blacklist-rules").evaluate("element => element === document.activeElement")
    assert page.evaluate("window.__navoMockDashboard.runtime.list_mode") == "blacklist"
    assert page.locator("#whitelist-rules").count() == 0

    whitelist = peer_button(page, "白名单")
    whitelist.click()
    page.locator("#whitelist-rules").wait_for()
    assert whitelist.get_attribute("aria-pressed") == "true"
    assert whitelist.get_attribute("aria-expanded") == "true"
    assert blacklist.get_attribute("aria-pressed") == "false"
    assert page.locator("#whitelist-rules").evaluate("element => element === document.activeElement")
    assert page.evaluate("window.__navoMockDashboard.runtime.list_mode") == "whitelist"
    assert page.locator("#blacklist-rules").count() == 0

    page.get_by_role("button", name="关闭名单").click()
    page.locator("#routing-list-editor").wait_for(state="detached")
    assert page.evaluate("window.__navoMockDashboard.runtime.list_mode") == "off"
    assert peer_button(page, "黑名单").get_attribute("aria-pressed") == "false"
    assert peer_button(page, "白名单").get_attribute("aria-pressed") == "false"


def verify_layout(page):
    overflow = page.evaluate("document.documentElement.scrollWidth - document.documentElement.clientWidth")
    assert overflow <= 0, overflow
    viewport_width = page.viewport_size["width"]

    intro = page.locator(".connection-page-intro").bounding_box()
    assert intro and intro["height"] < 90, intro

    setup_cards = [card.bounding_box() for card in page.locator(".connection-setup-grid > article").all()]
    assert len(setup_cards) == 2 and all(setup_cards), setup_cards
    source_card, route_card = setup_cards
    if viewport_width > 860:
        assert abs(source_card["y"] - route_card["y"]) <= 2, setup_cards
        assert source_card["x"] + source_card["width"] < route_card["x"], setup_cards
    else:
        assert route_card["y"] >= source_card["y"] + source_card["height"], setup_cards
        route_buttons = [button.bounding_box() for button in page.locator(".routing-policy-options button").all()]
        assert len(route_buttons) == 3 and max(item["y"] for item in route_buttons) - min(item["y"] for item in route_buttons) <= 2, route_buttons

    notice = page.locator(".feedback p")
    if notice.count() and notice.is_visible():
        notice_box = notice.bounding_box()
        heading_box = page.locator(".page-heading").bounding_box()
        theme_box = page.locator(".theme-switch").bounding_box()
        assert notice_box and heading_box and theme_box
        for name, target_box in (("heading", heading_box), ("theme", theme_box)):
            overlaps = not (
                notice_box["x"] + notice_box["width"] <= target_box["x"]
                or target_box["x"] + target_box["width"] <= notice_box["x"]
                or notice_box["y"] + notice_box["height"] <= target_box["y"]
                or target_box["y"] + target_box["height"] <= notice_box["y"]
            )
            assert not overlaps, {"notice": notice_box, name: target_box}


with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True, executable_path=EDGE)
    context = browser.new_context(viewport={"width": 1280, "height": 900}, device_scale_factor=1)
    context.add_init_script(script=MOCK_BRIDGE)
    page = context.new_page()
    errors = []
    page.on("console", lambda message: errors.append(message.text) if message.type == "error" else None)
    page.on("pageerror", lambda error: errors.append(str(error)))

    open_connection(page)
    page.get_by_role("button", name="夜", exact=True).click()
    page.wait_for_timeout(250)
    assert_default_state(page)
    assert_list_interaction(page)
    verify_layout(page)
    page.screenshot(path=str(ARTIFACT_DIR / "desktop-night.png"), full_page=True)

    page.get_by_role("button", name="日", exact=True).click()
    page.wait_for_timeout(250)
    peer_button(page, "黑名单").click()
    page.locator("#blacklist-rules").wait_for()
    page.screenshot(path=str(ARTIFACT_DIR / "desktop-day-blacklist.png"), full_page=True)
    verify_layout(page)

    compact = context.new_page()
    compact.set_viewport_size({"width": 760, "height": 900})
    compact.on("console", lambda message: errors.append(message.text) if message.type == "error" else None)
    compact.on("pageerror", lambda error: errors.append(str(error)))
    open_connection(compact)
    compact.get_by_role("button", name="夜", exact=True).click()
    compact.wait_for_timeout(250)
    assert_default_state(compact)
    peer_button(compact, "白名单").click()
    compact.locator("#whitelist-rules").wait_for()
    verify_layout(compact)
    compact.screenshot(path=str(ARTIFACT_DIR / "compact-night-whitelist.png"), full_page=True)

    assert not errors, json.dumps(errors, ensure_ascii=False)
    print(json.dumps({"result": "PASS", "console_errors": 0, "screenshots": 3}, ensure_ascii=False))
    browser.close()
