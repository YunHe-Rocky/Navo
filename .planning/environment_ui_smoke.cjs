const { chromium } = require("playwright");

const screenshot = "C:\\Users\\28484\\.codex\\visualizations\\2026\\08\\23\\01a02e6b-e4fd-7900-9a3b-ee0707a55c76\\network-environment-card.png";
const resource = {
  known: true, coherent: false, owned_count: 1, existing_count: 0,
  missing_count: 1, conflict_count: 0,
};
const environment = {
  version: 1, collected_at: "2026-08-23T01:00:00Z",
  health: "degraded", stale: false, partial: false,
  transition: { busy: false },
  capture: { state: "stopped", desired_mode: "off", committed_mode: "off" },
  physical: { known: true, available: true, active_interfaces: ["Ethernet"] },
  system_proxy: {
    enabled: true, ownership: "external", owned_by_navo: false,
    ownership_marker: false, ownership_lost: false, local_endpoint: false,
    reachable: false, reachable_known: false,
  },
  tun: {
    expected: false,
    navo: { present: false, enabled: false, ownership: "none" },
    external_present: true, external: [{ name: "WireGuard", state: "up" }],
  },
  dns: resource, routes: resource, nrpt: resource, firewall: resource,
  journal: {
    present: true, dirty: true, owned_resources: 1, preexisting_resources: 0,
    pending_actions: 1, missing_resources: 1, conflicting_resources: 0,
  },
  findings: [{
    code: "ENV_NAVO_ROUTE_RESIDUAL", severity: "error", domain: "route",
    summary: "Navo route state is inconsistent", detail: "One owned route is missing",
    ownership: "navo", recoverable: true, transitional: false,
  }, {
    code: "ENV_EXTERNAL_TUN_PRESENT", severity: "info", domain: "tun",
    summary: "External virtual adapter detected", detail: "Observation only",
    ownership: "external", recoverable: false, transitional: false,
  }],
  observation_errors: [],
};

(async () => {
  const browser = await chromium.launch({
    headless: true,
    executablePath: "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
  });
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  const errors = [];
  page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });
  await page.addInitScript(({ environment }) => {
    window.__repairCode = "";
    const emptyIP = { ip:"", country:"", city:"", asn:"", isp:"", network:"", provider:"", mobile:false, proxy:false, hosting:false, checked_at:"", error:"" };
    const methods = {
      GetDashboard: async () => ({ environment }),
      ListRoutes: async () => ({ outbounds:[], selected_id:"", active_id:"", candidate_id:"", mode:"" }),
      GetHostStatus: async () => ({}),
      CheckIP: async () => ({ source:emptyIP, proxy:emptyIP }),
      RepairNetworkEnvironment: async (code) => { window.__repairCode = code; return environment; },
    };
    window.go = { main: { App: new Proxy(methods, { get: (target, key) => target[key] || (async () => ({})) }) } };
    window.runtime = {
      EventsOn: () => () => {}, WindowSetBackgroundColour: () => {},
      WindowSetLightTheme: () => {}, WindowSetDarkTheme: () => {},
    };
  }, { environment });
  await page.goto("http://127.0.0.1:4173");
  await page.waitForLoadState("networkidle");
  const card = page.locator(".environment-card");
  await card.waitFor({ state: "visible" });
  if (!(await card.innerText()).includes("ENV_NAVO_ROUTE_RESIDUAL")) throw new Error("finding not rendered");
  const repair = card.getByRole("button", { name: /Navo route state is inconsistent/ });
  if (!(await repair.isEnabled())) throw new Error("repair button disabled");
  await repair.click();
  await page.waitForFunction(() => window.__repairCode === "ENV_NAVO_ROUTE_RESIDUAL");
  await page.screenshot({ path: screenshot, fullPage: true });
  await page.setViewportSize({ width: 820, height: 900 });
  await page.waitForTimeout(200);
  const fits = await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);
  if (!fits) throw new Error("horizontal overflow at 820px");
  if (errors.length) throw new Error(`console errors: ${errors.join(" | ")}`);
  await browser.close();
  console.log(`PASS screenshot=${screenshot}`);
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
