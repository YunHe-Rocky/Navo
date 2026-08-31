const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const URL = process.env.NAVO_EXTERNAL_SMOKE_URL || "http://127.0.0.1:4194";
const EDGE = "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe";
const OUTPUT = process.env.NAVO_EXTERNAL_SMOKE_OUTPUT
  ? path.resolve(process.env.NAVO_EXTERNAL_SMOKE_OUTPUT)
  : path.resolve(__dirname, "..", ".tmp", "external-proxy-ui-smoke");

const resource = {
  known: true, coherent: true, owned_count: 0, existing_count: 0,
  missing_count: 0, conflict_count: 0, last_error: "",
};

const dashboard = {
  core: {
    core_id: "sing-box", state: "stopped", pid: 0, uptime_seconds: 0,
    config_hash: "", restart_count: 0, last_error: "",
  },
  cores: [{
    id: "sing-box", name: "sing-box", version: "1.13.14", installed: true,
    active: true, system_proxy_supported: true, tun_supported: true,
  }],
  proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
  runtime: {
    mode: "bypass_mainland", list_mode: "off", selected_id: "candidate-v2",
    active_id: "", candidate_id: "candidate-v2", tun_enabled: false,
    blacklist: [], whitelist: [],
  },
  tun: {
    installed: true, created: false, enabled: false, name: "Navo", mtu: 1500,
    state: "disabled", identifier: "", interface_index: 0, fault_id: "", last_error: "",
  },
  capture: {
    state: "stopped", phase: "stopped", desired_mode: "off", committed_mode: "off",
    transition_id: "", fault_id: "", last_error: "", can_retry_tun: false,
    readiness: {
      state: "unverified", scope: "chatgpt", sites: {}, default_proxy: false,
      checked_at: "", error: "",
    },
    recovery: {
      state: "idle", evidence: {}, rounds: [], candidates: [],
      updated_at: "", final_error: "",
    },
  },
  metrics: {
    reachable: false, available: false, unavailable_reason: "Navo core is stopped",
    core_name: "sing-box", latency_ms: 0, upload_bytes: 0, download_bytes: 0,
    connections: 0, local_available: true, local_unavailable_reason: "",
    local_upload_bps: 1536, local_download_bps: 8192,
    proxy_upload_bps: 0, proxy_download_bps: 0,
    local_upload_total: 1048576, local_download_total: 8388608,
    proxy_upload_total: 0, proxy_download_total: 0,
    traffic_source_state: "ready", traffic_sampled_at: "2026-08-31T03:00:00Z",
  },
  ip: {
    connection_kind: "external_system_proxy",
    proxy_ip: "203.0.113.20", proxy_country: "External",
    direct_ip: "198.51.100.10", proxy_error: "", direct_error: "",
    proxy_provider: "proxy-fixture", direct_provider: "direct-fixture",
    proxy_checked_at: "2026-08-31T03:00:00Z",
    direct_checked_at: "2026-08-31T03:00:00Z", probe_pending: false,
  },
  environment: {
    version: 1, collected_at: "2026-08-31T03:00:00Z",
    health: "healthy", stale: false, partial: false,
    transition: { busy: false, id: "", operation: "", phase: "", fault_domain: "" },
    capture: {
      state: "stopped", desired_mode: "off", committed_mode: "off",
      fault_id: "", readiness_state: "unverified", readiness_error: "",
    },
    physical: { known: true, available: true, active_interfaces: ["Ethernet"], last_error: "" },
    system_proxy: {
      enabled: true, proxy_server: "127.0.0.1:10808", ownership: "external",
      owned_by_navo: false, ownership_marker: false, ownership_lost: false,
      local_endpoint: true, reachable: true, reachable_known: true, last_error: "",
    },
    tun: {
      expected: false,
      navo: { present: false, enabled: false, name: "Navo", state: "absent", ownership: "none", last_error: "" },
      external_present: false, external: [],
    },
    dns: resource, routes: resource, nrpt: resource, firewall: resource,
    journal: {
      present: false, dirty: false, owned_resources: 0, preexisting_resources: 0,
      pending_actions: 0, missing_resources: 0, conflicting_resources: 0, last_error: "",
    },
    findings: [{
      code: "EXTERNAL_SYSTEM_PROXY_OBSERVED", severity: "info", domain: "system_proxy",
      summary: "检测到外部系统代理", detail: "127.0.0.1:10808",
      ownership: "external", recoverable: false, transitional: false,
    }],
    observation_errors: [],
  },
};

const ipResult = {
  connection_kind: "external_system_proxy",
  source: {
    outbound_id: "source", state: "available", available: true,
    ip: "198.51.100.10", country: "Direct", city: "Baseline",
    asn: "AS64500", isp: "Direct ISP", network: "TEST-NET-2",
    provider: "direct-fixture", mobile: false, proxy: false, hosting: false,
    checked_at: "2026-08-31T03:00:00Z", error: "",
  },
  proxy: {
    outbound_id: "external-system-proxy", state: "available", available: true,
    ip: "203.0.113.20", country: "External", city: "Exit",
    asn: "AS64501", isp: "External ISP", network: "TEST-NET-3",
    provider: "proxy-fixture", mobile: false, proxy: true, hosting: false,
    checked_at: "2026-08-31T03:00:00Z", error: "",
  },
};

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

async function main() {
  fs.mkdirSync(OUTPUT, { recursive: true });
  const browser = await chromium.launch({ executablePath: EDGE, headless: true });
  const consoleErrors = [];
  const pageErrors = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    page.on("console", (message) => {
      if (message.type() === "error") consoleErrors.push(message.text());
    });
    page.on("pageerror", (error) => pageErrors.push(error.message));
    await page.addInitScript(({ dashboardValue, ipValue }) => {
      const clone = (value) => JSON.parse(JSON.stringify(value));
      window.__externalIPChecks = 0;
      window.runtime = {
        EventsOn: () => () => undefined,
        WindowSetBackgroundColour: () => undefined,
        WindowSetLightTheme: () => undefined,
        WindowSetDarkTheme: () => undefined,
      };
      window.go = { main: { App: {
        GetDashboard: async () => clone(dashboardValue),
        ListRoutes: async () => ({
          selected_id: "candidate-v2", active_id: "", candidate_id: "candidate-v2",
          mode: "bypass_mainland",
          outbounds: [{
            id: "candidate-v2", name: "Uncommitted Navo Candidate", type: "vless",
            server: "candidate.invalid", port: 443, source_type: "manual",
            country: "", selected: true, active: false,
          }],
        }),
        ListSubscriptions: async () => ({ subscriptions: [] }),
        CheckIP: async () => {
          window.__externalIPChecks += 1;
          return clone(ipValue);
        },
        GetHostStatus: async () => ({
          os: "windows", arch: "amd64", logical_cpu: 8, memory_usage_percent: 36,
          memory_available_bytes: 8589934592, system_uptime_seconds: 7200,
          process_uptime_seconds: 0, app_version: "1.0.external-smoke", go_version: "go1.25",
        }),
        GetStartupSettings: async () => ({
          supported: true, enabled: false, registered: false,
          mode: "system_proxy", last_error: "",
        }),
        QueryLogs: async () => ({ entries: [], next_cursor: 0, has_more: false }),
        GetLogMetadata: async () => ({
          levels: ["DEBUG", "INFO", "WARN", "ERROR"], categories: [], services: [],
        }),
        ClearPersistedLogs: async () => undefined,
      } } };
    }, { dashboardValue: dashboard, ipValue: ipResult });

    await page.goto(URL, { waitUntil: "networkidle" });
    await page.locator(".connection-evidence-card").waitFor({ state: "visible" });
    assert(await page.locator(".connection-evidence-card").count() === 1, "overview did not render exactly one effective connection card");
    assert(await page.locator(".ip-card").count() === 0, "legacy duplicate IP cards are still present");
    assert(await page.locator(".environment-card").count() === 0, "environment detail duplicated the overview connection surface");
    assert(await page.getByText("外部系统代理出口已验证", { exact: true }).count() >= 1, "external proxy was not the overview connection");
    assert(await page.getByText("127.0.0.1:10808", { exact: true }).count() >= 1, "external proxy endpoint is missing");
    assert(await page.getByText("203.0.113.20", { exact: true }).count() >= 1, "external exit IP is missing");
    assert(await page.getByText("198.51.100.10", { exact: true }).count() >= 1, "direct baseline is missing");
    assert(await page.getByRole("button", { name: "检测外部代理出口", exact: true }).count() === 1, "external exit action is mislabeled");
    assert(await page.getByText("系统总流量", { exact: true }).count() >= 1, "overview traffic is not labeled as system total");
    assert(await page.getByText(/物理网卡计数 · 外部代理只读/).count() >= 1, "overview traffic source does not disclose external read-only attribution");
    assert(await page.getByText(/系统总量上传均值/).count() >= 1, "chart upload legend is not system total");
    assert(await page.getByText(/系统总量下载均值/).count() >= 1, "chart download legend is not system total");

    await page.getByRole("button", { name: "检测外部代理出口", exact: true }).click();
    await page.getByText(/已完成外部系统代理出口与直连基线检测/).waitFor();
    assert(await page.evaluate(() => window.__externalIPChecks) >= 2, "manual external IP check did not call the bridge");

    await page.getByRole("button", { name: "夜", exact: true }).click();
    const overviewNight = path.join(OUTPUT, "external-overview-night.png");
    await page.screenshot({ path: overviewNight, fullPage: true });

    await page.getByRole("button", { name: /^流量监控：/ }).click();
    await page.getByText("外部代理流量口径", { exact: true }).waitFor();
    assert(await page.getByText("物理网卡计数 · 外部代理只读", { exact: true }).count() >= 1, "traffic page source is not external read-only");
    assert(await page.getByText(/不会以系统总流量推算外部代理专属流量/).count() >= 1, "traffic page limitation is missing");

    await page.getByRole("button", { name: /^网络检测：/ }).click();
    await page.locator(".environment-card").waitFor({ state: "visible" });
    assert(await page.getByText("网络环境 · 只读聚合", { exact: true }).count() === 1, "environment detail is missing from diagnostics");
    assert(await page.getByText("外部管理", { exact: true }).count() >= 1, "diagnostics did not identify external ownership");
    assert(await page.getByText("外部代理出口 IP", { exact: true }).count() === 1, "diagnostics exit card is mislabeled");
    const diagnosticsNight = path.join(OUTPUT, "external-diagnostics-night.png");
    await page.screenshot({ path: diagnosticsNight, fullPage: true });

    await page.getByRole("button", { name: /^运行概览：/ }).click();
    await page.getByRole("button", { name: "日", exact: true }).click();
    await page.setViewportSize({ width: 760, height: 900 });
    await page.waitForTimeout(100);
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    assert(overflow <= 1, `compact layout overflows horizontally by ${overflow}px`);
    const compactDay = path.join(OUTPUT, "external-overview-compact-day.png");
    await page.screenshot({ path: compactDay, fullPage: true });

    assert(pageErrors.length === 0, `page errors: ${pageErrors.join("\n")}`);
    assert(consoleErrors.length === 0, `console errors: ${consoleErrors.join("\n")}`);
    process.stdout.write("EXTERNAL_PROXY_UI_SMOKE=PASS\n");
    process.stdout.write("OVERVIEW_CONNECTION_CARDS=1 LEGACY_IP_CARDS=0 ENVIRONMENT_CARDS=0\n");
    process.stdout.write("EXIT=203.0.113.20 DIRECT=198.51.100.10 ENDPOINT=127.0.0.1:10808\n");
    process.stdout.write("TRAFFIC=system_total SOURCE=physical_interface_external_read_only\n");
    process.stdout.write(`THEMES=2 COMPACT=760x900 HORIZONTAL_OVERFLOW=${overflow}\n`);
    process.stdout.write(`SCREENSHOTS=${OUTPUT}\n`);
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exitCode = 1;
});
