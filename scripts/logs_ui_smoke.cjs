const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const URL = process.env.NAVO_LOGS_SMOKE_URL || "http://127.0.0.1:4193";
const EDGE = "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe";
const OUTPUT = path.resolve(__dirname, "..", ".tmp", "logs-ui-smoke");

const dashboard = {
  core: {
    core_id: "sing-box", state: "running", pid: 4242, uptime_seconds: 180,
    config_hash: "logs-smoke", restart_count: 0, last_error: "",
  },
  cores: [{
    id: "sing-box", name: "sing-box", version: "1.13.14", installed: true,
    active: true, system_proxy_supported: true, tun_supported: true,
  }],
  proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
  runtime: {
    mode: "bypass_mainland", list_mode: "off", selected_id: "route-1",
    active_id: "route-1", candidate_id: "", tun_enabled: false,
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
    reachable: true, available: true, unavailable_reason: "", core_name: "sing-box",
    latency_ms: 32, upload_bytes: 2048, download_bytes: 4096, connections: 4,
    local_available: true, local_unavailable_reason: "", local_upload_bps: 1024,
    local_download_bps: 4096, proxy_upload_bps: 0, proxy_download_bps: 0,
    local_upload_total: 2048, local_download_total: 8192, proxy_upload_total: 0,
    proxy_download_total: 0, traffic_source_state: "ready",
    traffic_sampled_at: "2026-08-31T01:00:00Z",
  },
  ip: {
    proxy_ip: "198.51.100.9", proxy_country: "TEST", direct_ip: "192.0.2.9",
    probe_pending: false,
  },
};

const entries = [
  {
    id: 1, timestamp: "2026-08-31T01:01:00Z", level: "INFO",
    category: "basic_service", service: "Service", component: "IPC",
    message: "basic service ready", fields: { method: "runtime.status" },
  },
  {
    id: 2, timestamp: "2026-08-31T01:02:00Z", level: "WARN",
    category: "network_capture", service: "TUN", component: "Readiness",
    message: "capture route retry", fields: { error_code: "ROUTE_RETRY" },
  },
  {
    id: 3, timestamp: "2026-08-31T01:03:00Z", level: "ERROR",
    category: "core_runtime", service: "Supervisor", component: "Process",
    message: "core process stopped", fields: { error_code: "CORE_STOPPED" },
  },
];

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
    await page.addInitScript(({ dashboardValue, logEntries }) => {
      const clone = (value) => JSON.parse(JSON.stringify(value));
      const routes = {
        selected_id: "route-1", active_id: "route-1", candidate_id: "",
        mode: "bypass_mainland",
        outbounds: [{
          id: "route-1", name: "Smoke Tokyo", type: "vless",
          server: "edge.example.net", port: 443,
          source_type: "airport_subscription", country: "JP",
          selected: true, active: true,
        }],
      };
      const ipResult = {
        source: {
          state: "available", available: true, ip: "192.0.2.9", country: "TEST",
          city: "Direct", asn: "AS64500", isp: "Direct ISP", network: "TEST-NET-1",
          provider: "smoke", mobile: false, proxy: false, hosting: false,
          checked_at: "2026-08-31T01:00:00Z", error: "",
        },
        proxy: {
          state: "inactive", available: false, ip: "", country: "", city: "",
          asn: "", isp: "", network: "", provider: "smoke", mobile: false,
          proxy: false, hosting: false, checked_at: "2026-08-31T01:00:00Z", error: "",
        },
      };
      window.__navoLogQueries = [];
      window.runtime = {
        EventsOn: () => () => undefined,
        WindowSetBackgroundColour: () => undefined,
        WindowSetLightTheme: () => undefined,
        WindowSetDarkTheme: () => undefined,
      };
      window.go = { main: { App: {
        GetDashboard: async () => clone(dashboardValue),
        ListRoutes: async () => clone(routes),
        ListSubscriptions: async () => ({ subscriptions: [] }),
        CheckIP: async () => clone(ipResult),
        GetHostStatus: async () => ({
          os: "windows", arch: "amd64", logical_cpu: 8, memory_usage_percent: 36,
          memory_available_bytes: 8589934592, system_uptime_seconds: 7200,
          process_uptime_seconds: 180, app_version: "1.0.smoke", go_version: "go1.25",
        }),
        GetStartupSettings: async () => ({
          supported: true, enabled: false, registered: false,
          mode: "system_proxy", last_error: "",
        }),
        QueryLogs: async (query) => {
          window.__navoLogQueries.push(clone(query));
          const matches = logEntries.filter((entry) =>
            (!query.levels?.length || query.levels.includes(entry.level)) &&
            (!query.categories?.length || query.categories.includes(entry.category)) &&
            (!query.services?.length || query.services.includes(entry.service))
          );
          return {
            entries: clone(matches),
            next_cursor: matches.length ? matches[matches.length - 1].id : 0,
            has_more: false,
          };
        },
        GetLogMetadata: async () => ({
          levels: ["DEBUG", "INFO", "WARN", "ERROR"],
          categories: [
            "basic_service", "network_capture", "core_runtime",
            "subscription_update", "other",
          ],
          services: ["Agent", "Service", "Supervisor", "TUN"],
        }),
        ClearPersistedLogs: async () => undefined,
      } } };
    }, { dashboardValue: dashboard, logEntries: entries });

    await page.goto(URL, { waitUntil: "networkidle" });
    await page.getByRole("button", { name: /^设置与日志：/ }).click();
    await page.locator(".settings-log-card").waitFor({ state: "visible" });

    const categories = page.getByRole("group", { name: "服务分级" });
    const levels = page.getByRole("group", { name: "日志级别" });
    const services = page.getByRole("group", { name: "具体服务" });
    const basic = categories.getByLabel("基础服务", { exact: true });
    const network = categories.getByLabel("网络与接管", { exact: true });
    assert(await basic.isChecked(), "Basic Service was not selected by default");
    assert(!await network.isChecked(), "Network category should not be selected by default");
    assert(await levels.getByLabel("INFO", { exact: true }).isChecked(), "INFO should be selected");
    assert(await levels.getByLabel("WARN", { exact: true }).isChecked(), "WARN should be selected");
    assert(!await levels.getByLabel("DEBUG", { exact: true }).isChecked(), "DEBUG should be excluded");
    await page.getByText("basic service ready", { exact: true }).waitFor();
    assert(await page.getByText("capture route retry", { exact: true }).count() === 0, "default query leaked network logs");

    const initialQuery = await page.evaluate(() => window.__navoLogQueries[0]);
    assert(
      JSON.stringify(initialQuery.categories) === JSON.stringify(["basic_service"]),
      `initial categories = ${JSON.stringify(initialQuery.categories)}`,
    );
    assert(
      JSON.stringify(initialQuery.levels) === JSON.stringify(["INFO", "WARN", "ERROR"]),
      `initial levels = ${JSON.stringify(initialQuery.levels)}`,
    );
    await page.getByRole("button", { name: "夜", exact: true }).click();
    const defaultPath = path.join(OUTPUT, "logs-default-night.png");
    await page.screenshot({ path: defaultPath, fullPage: true });

    await basic.uncheck();
    await network.check();
    await services.getByLabel("TUN", { exact: true }).check();
    await page.getByRole("button", { name: "查询", exact: true }).click();
    await page.getByText("capture route retry", { exact: true }).waitFor();
    assert(await page.getByText("basic service ready", { exact: true }).count() === 0, "category switch kept basic logs");

    await levels.getByLabel("WARN", { exact: true }).uncheck();
    await page.getByRole("button", { name: "查询", exact: true }).click();
    await page.getByText("暂无日志", { exact: true }).waitFor();
    const filteredQuery = await page.evaluate(() => window.__navoLogQueries.at(-1));
    assert(
      JSON.stringify(filteredQuery.categories) === JSON.stringify(["network_capture"]),
      `filtered categories = ${JSON.stringify(filteredQuery.categories)}`,
    );
    assert(
      JSON.stringify(filteredQuery.services) === JSON.stringify(["TUN"]),
      `filtered services = ${JSON.stringify(filteredQuery.services)}`,
    );
    assert(!filteredQuery.levels.includes("WARN"), "WARN remained in the severity filter");

    await levels.getByLabel("WARN", { exact: true }).check();
    await page.getByRole("button", { name: "查询", exact: true }).click();
    await page.getByText("capture route retry", { exact: true }).waitFor();

    await page.getByRole("button", { name: "夜", exact: true }).click();
    assert(await page.locator("html").getAttribute("data-theme") === "night", "night theme did not apply");
    const nightPath = path.join(OUTPUT, "logs-night.png");
    await page.screenshot({ path: nightPath, fullPage: true });

    await page.getByRole("button", { name: "日", exact: true }).click();
    assert(await page.locator("html").getAttribute("data-theme") === "day", "day theme did not apply");
    const dayPath = path.join(OUTPUT, "logs-day.png");
    await page.screenshot({ path: dayPath, fullPage: true });

    await page.setViewportSize({ width: 760, height: 900 });
    await page.waitForTimeout(100);
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    assert(overflow <= 1, `compact layout overflows horizontally by ${overflow}px`);
    const compactPath = path.join(OUTPUT, "logs-compact-day.png");
    await page.screenshot({ path: compactPath, fullPage: true });

    assert(pageErrors.length === 0, `page errors: ${pageErrors.join("\n")}`);
    assert(consoleErrors.length === 0, `console errors: ${consoleErrors.join("\n")}`);
    process.stdout.write("LOGS_UI_SMOKE=PASS\n");
    process.stdout.write("DEFAULT_CATEGORY=basic_service LEVELS=INFO,WARN,ERROR\n");
    process.stdout.write("CATEGORY_SWITCH=network_capture SERVICE=TUN SEVERITY_FILTER=PASS\n");
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
