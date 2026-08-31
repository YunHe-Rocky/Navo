const fs = require("fs");
const path = require("path");
let chromium;
try {
  ({ chromium } = require("playwright"));
} catch {
  ({ chromium } = require("C:/Users/28484/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright"));
}

const URL = process.env.NAVO_PHASE75_SMOKE_URL || "http://127.0.0.1:4195";
const EDGE = "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe";
const OUTPUT = process.env.NAVO_PHASE75_SMOKE_OUTPUT
  ? path.resolve(process.env.NAVO_PHASE75_SMOKE_OUTPUT)
  : path.resolve(__dirname, "..", ".tmp", "phase75-ui-smoke");

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
    mode: "bypass_mainland", list_mode: "off", selected_id: "",
    active_id: "", candidate_id: "", tun_enabled: false,
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
    traffic_source_state: "ready", traffic_sampled_at: "2026-08-31T05:00:00Z",
  },
  ip: {
    connection_kind: "external_system_proxy",
    proxy_ip: "203.0.113.20", proxy_country: "External",
    direct_ip: "198.51.100.10", proxy_error: "", direct_error: "",
    proxy_provider: "proxy-fixture", direct_provider: "direct-fixture",
    proxy_checked_at: "2026-08-31T05:00:00Z",
    direct_checked_at: "2026-08-31T05:00:00Z", probe_pending: false,
  },
  environment: {
    version: 1, collected_at: "2026-08-31T05:00:00Z",
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

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function maximumDurationSeconds(value) {
  return Math.max(...value.split(",").map((part) => {
    const token = part.trim();
    const amount = Number.parseFloat(token);
    return Number.isFinite(amount) ? amount * (token.endsWith("ms") ? 0.001 : 1) : Number.POSITIVE_INFINITY;
  }));
}

async function layoutEvidence(page, name) {
  const result = await page.evaluate(() => {
    const selectors = ["html", "body", "#app", ".app-shell", "main", ".page-content"];
    const evidence = selectors.map((selector) => {
      const element = document.querySelector(selector);
      return {
        selector,
        clientWidth: element?.clientWidth || 0,
        scrollWidth: element?.scrollWidth || 0,
        overflow: Math.max(0, (element?.scrollWidth || 0) - (element?.clientWidth || 0)),
      };
    });
    const main = document.querySelector("main");
    const mainRect = main?.getBoundingClientRect();
    const allMainElements = [...document.querySelectorAll("main *")];
    const offenders = mainRect ? allMainElements
      .map((element) => {
        const rect = element.getBoundingClientRect();
        const style = getComputedStyle(element);
        return {
          element: `${element.tagName.toLowerCase()}${element.id ? `#${element.id}` : ""}${[...element.classList].map((name) => `.${name}`).join("")}`,
          left: Math.round(rect.left), right: Math.round(rect.right), width: Math.round(rect.width),
          minWidth: style.minWidth, overflowX: style.overflowX,
        };
      })
      .filter((entry) => entry.right > Math.ceil(mainRect.right) + 1)
      .sort((a, b) => b.right - a.right)
      .slice(0, 12) : [];
    const internalOverflow = allMainElements.map((element) => ({
      element: `${element.tagName.toLowerCase()}${element.id ? `#${element.id}` : ""}${[...element.classList].map((name) => `.${name}`).join("")}`,
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
      overflow: Math.max(0, element.scrollWidth - element.clientWidth),
      overflowX: getComputedStyle(element).overflowX,
    })).filter((entry) => entry.overflow > 1).sort((a, b) => b.overflow - a.overflow).slice(0, 12);
    return { evidence, offenders, internalOverflow };
  });
  const { evidence, offenders, internalOverflow } = result;
  const maxOverflow = Math.max(...evidence.map((entry) => entry.overflow));
  assert(maxOverflow <= 1, `${name} overflows horizontally by ${maxOverflow}px: ${JSON.stringify(evidence)} offenders=${JSON.stringify(offenders)} internal=${JSON.stringify(internalOverflow)}`);
  return { name, maxOverflow, evidence, offenders, internalOverflow };
}

async function screenshot(page, name) {
  const target = path.join(OUTPUT, `${name}.png`);
  await page.screenshot({ path: target });
  return target;
}

async function settle(page) {
  const progress = page.locator(".activity-progress");
  if (await progress.count()) {
    await progress.waitFor({ state: "hidden", timeout: 3000 });
  }
  await page.waitForTimeout(60);
}

async function main() {
  fs.mkdirSync(OUTPUT, { recursive: true });
  const browser = await chromium.launch({ executablePath: EDGE, headless: true });
  const consoleErrors = [];
  const pageErrors = [];
  const layouts = [];
  const screenshots = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    page.on("console", (message) => {
      if (message.type() === "error") consoleErrors.push(message.text());
    });
    page.on("pageerror", (error) => pageErrors.push(error.message));
    await page.addInitScript(({ dashboardValue }) => {
      const clone = (value) => JSON.parse(JSON.stringify(value));
      window.__captureCalls = [];
      window.__startupCalls = [];
      window.runtime = {
        EventsOn: () => () => undefined,
        WindowSetBackgroundColour: () => undefined,
        WindowSetLightTheme: () => undefined,
        WindowSetDarkTheme: () => undefined,
      };
      window.go = { main: { App: {
        GetDashboard: async () => clone(dashboardValue),
        ListRoutes: async () => ({
          selected_id: "", active_id: "", candidate_id: "",
          mode: "bypass_mainland", outbounds: [],
        }),
        ListSubscriptions: async () => ({ subscriptions: [] }),
        SetCaptureMode: async (mode) => {
          window.__captureCalls.push(mode);
          throw new Error("missing-route capture request reached bridge");
        },
        GetStartupSettings: async () => ({
          supported: true, enabled: false, registered: false,
          mode: "system_proxy", last_error: "", checked_at: "2026-08-31T05:00:00Z",
        }),
        SetStartupSettings: async (enabled, mode) => {
          window.__startupCalls.push({ enabled, mode });
          throw new Error("missing-route startup request reached bridge");
        },
        GetHostStatus: async () => ({
          os: "windows", arch: "amd64", logical_cpu: 8, memory_usage_percent: 36,
          memory_available_bytes: 8589934592, system_uptime_seconds: 7200,
          process_uptime_seconds: 0, app_version: "1.0.phase75", go_version: "go1.25",
        }),
        QueryLogs: async () => ({ entries: [], next_cursor: 0, has_more: false }),
        GetLogMetadata: async () => ({
          levels: ["DEBUG", "INFO", "WARN", "ERROR"], categories: [], services: [],
        }),
        ClearPersistedLogs: async () => undefined,
        CheckIP: async () => ({ source: {}, proxy: {} }),
      } } };
    }, { dashboardValue: dashboard });

    await page.goto(URL, { waitUntil: "networkidle" });
    await page.locator(".connection-evidence-card").waitFor({ state: "visible" });
    await page.getByRole("button", { name: "夜", exact: true }).click();
    assert((await page.locator("html").getAttribute("data-theme")) === "night", "night theme was not explicitly applied");
    await settle(page);
    assert(await page.locator(".connection-evidence-card").count() === 1, "overview must retain one canonical connection card");
    assert(await page.getByRole("button", { name: "选择 Navo 线路", exact: true }).count() === 1, "missing-route overview CTA is not state-aware");
    assert(await page.getByText("外部系统代理出口已验证", { exact: true }).count() >= 1, "external proxy observation was lost");
    layouts.push(await layoutEvidence(page, "overview-1440x900-night"));
    screenshots.push(await screenshot(page, "overview-1440x900-night"));

    await page.getByRole("button", { name: "选择 Navo 线路", exact: true }).click();
    await page.getByRole("heading", { name: "连接管理", level: 1 }).waitFor();
    await settle(page);
    assert(await page.getByText(/Navo 不会借用其他代理软件的端口/).count() === 1, "connection page did not explain external-proxy isolation");
    assert((await page.evaluate(() => window.__captureCalls.length)) === 0, "route-selection CTA issued a capture bridge request");

    await page.getByRole("button", { name: /^运行概览：/ }).click();
    await settle(page);
    const overviewSizes = [
      [1024, 768, "overview-1024x768-night"],
      [760, 900, "overview-760x900-night"],
      [375, 812, "overview-375x812-night"],
      [667, 375, "overview-667x375-night-landscape"],
    ];
    for (const [width, height, name] of overviewSizes) {
      await page.setViewportSize({ width, height });
      await page.waitForTimeout(80);
      layouts.push(await layoutEvidence(page, name));
      screenshots.push(await screenshot(page, name));
    }

    await page.setViewportSize({ width: 1024, height: 768 });
    await page.getByRole("button", { name: "日", exact: true }).click();
    assert((await page.locator("html").getAttribute("data-theme")) === "day", "day theme was not explicitly applied");
    layouts.push(await layoutEvidence(page, "overview-1024x768-day"));
    screenshots.push(await screenshot(page, "overview-1024x768-day"));

    await page.getByRole("button", { name: /^设置与日志：/ }).click();
    await page.locator(".startup-settings-card").waitFor({ state: "visible" });
    await settle(page);
    const checkbox = page.getByRole("checkbox", { name: "启用开机连接" });
    assert(await checkbox.isDisabled(), "startup enable control must be disabled while the Navo route is missing");
    assert(await page.getByText("需要先配置 Navo 线路", { exact: true }).count() === 1, "settings route callout is missing");
    assert(await page.getByText(/外部代理只用于状态观测/).count() === 1, "settings does not disclose external proxy isolation");
    layouts.push(await layoutEvidence(page, "settings-1024x768-day"));
    screenshots.push(await screenshot(page, "settings-1024x768-day"));

    await page.setViewportSize({ width: 375, height: 812 });
    await page.waitForTimeout(80);
    layouts.push(await layoutEvidence(page, "settings-375x812-day"));
    const startupBoundsValid = await page.locator(".startup-settings-card").evaluate((card) => {
      const outer = card.getBoundingClientRect();
      return [...card.children].every((child) => {
        const rect = child.getBoundingClientRect();
        return rect.left >= outer.left - 1 && rect.right <= outer.right + 1;
      });
    });
    assert(startupBoundsValid, "startup controls escaped their settings card at 375px");
    screenshots.push(await screenshot(page, "settings-375x812-day"));

    await checkbox.evaluate((element) => {
      element.checked = true;
      element.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const alert = page.getByRole("alert");
    await alert.getByText("操作未完成", { exact: true }).waitFor();
    assert(await alert.getByRole("button", { name: "前往连接管理", exact: true }).count() === 1, "route failure is not actionable");
    assert((await page.evaluate(() => window.__startupCalls.length)) === 0, "startup preflight still issued a bridge request");
    await alert.getByRole("button", { name: "前往连接管理", exact: true }).click();
    await page.getByRole("heading", { name: "连接管理", level: 1 }).waitFor();
    await settle(page);

    await page.getByRole("button", { name: /^运行概览：/ }).click();
    await settle(page);
    await page.setViewportSize({ width: 760, height: 900 });
    await page.evaluate(() => document.activeElement?.blur());
    await page.keyboard.press("Tab");
    const focus = await page.evaluate(() => {
      const element = document.activeElement;
      const style = element ? getComputedStyle(element) : null;
      return { tag: element?.tagName || "", outlineStyle: style?.outlineStyle || "", outlineWidth: style?.outlineWidth || "" };
    });
    assert(focus.tag === "BUTTON" && focus.outlineStyle !== "none" && focus.outlineWidth !== "0px", `keyboard focus is not visible: ${JSON.stringify(focus)}`);

    await page.emulateMedia({ reducedMotion: "reduce" });
    const reducedMotion = await page.locator(".sidebar nav button.active").evaluate((element) => ({
      transitionDuration: getComputedStyle(element).transitionDuration,
      animationDuration: getComputedStyle(element, "::after").animationDuration,
    }));
    const durations = `${reducedMotion.transitionDuration},${reducedMotion.animationDuration}`;
    assert(maximumDurationSeconds(durations) <= 0.001, `reduced motion override is not bounded: ${durations}`);

    assert(pageErrors.length === 0, `page errors: ${pageErrors.join("\n")}`);
    assert(consoleErrors.length === 0, `console errors: ${consoleErrors.join("\n")}`);
    process.stdout.write("PHASE75_UI_SMOKE=PASS\n");
    process.stdout.write("ROUTE_CTA=select_route CAPTURE_BRIDGE_CALLS=0 STARTUP_BRIDGE_CALLS=0\n");
    process.stdout.write(`LAYOUTS=${layouts.length} MAX_OVERFLOW=${Math.max(...layouts.map((entry) => entry.maxOverflow))}\n`);
    process.stdout.write(`FOCUS=${JSON.stringify(focus)} REDUCED_MOTION=${JSON.stringify(reducedMotion)}\n`);
    process.stdout.write(`THEMES=2 VIEWPORTS=1440x900,1024x768,760x900,375x812,667x375 SCREENSHOTS=${screenshots.length}\n`);
    process.stdout.write(`OUTPUT=${OUTPUT}\n`);
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exitCode = 1;
});
