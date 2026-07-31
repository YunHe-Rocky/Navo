const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const dashboard = {
  core: {
    core_id: "sing-box",
    state: "running",
    pid: 12024,
    uptime: 96,
    config_hash: "smoke-test",
    restart_count: 0,
    last_error: "",
  },
  cores: [
    { id: "sing-box", name: "sing-box", version: "1.13.14", installed: true, active: true },
    { id: "mihomo", name: "Mihomo", version: "1.19.29", installed: true, active: false },
    { id: "xray", name: "Xray", version: "26.3.27", installed: true, active: false },
  ],
  proxy: { enabled: true, server: "127.0.0.1", port: 12080 },
  runtime: { mode: "global", active_id: "airport-node", tun_enabled: false },
  tun: { installed: false, enabled: false, name: "Navo", mtu: 1500 },
  metrics: {
    reachable: true,
    latency_ms: 42,
    upload_bytes: 1048576,
    download_bytes: 8388608,
    connections: 7,
  },
  ip: { proxy_ip: "203.0.113.8", proxy_country: "TEST", direct_ip: "192.0.2.8" },
};

const routes = {
  active_id: "airport-node",
  mode: "global",
  outbounds: [
    {
      id: "airport-node",
      name: "Airport · Tokyo",
      type: "vless",
      server: "edge.example.net",
      port: 443,
      provider_id: "airport-one",
      source_type: "airport_subscription",
      country: "JP",
      active: true,
    },
    {
      id: "upstream-node",
      name: "Office SOCKS5",
      type: "socks5",
      server: "proxy.example.net",
      port: 1080,
      provider_id: "",
      source_type: "upstream_proxy",
      country: "",
      active: false,
    },
  ],
};

const subscriptions = {
  subscriptions: [
    {
      id: "airport-one",
      name: "Airport One",
      configured: true,
      enabled: true,
      node_count: 18,
      last_error: "",
      skip_tls_verify: false,
    },
  ],
};

async function main() {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1180, height: 760 } });
    const errors = [];
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(`console: ${message.text()}`);
    });
    page.on("pageerror", (error) => errors.push(`page: ${error.message}`));

    await page.addInitScript(
    ({ dashboardValue, routesValue, subscriptionsValue }) => {
      const clone = (value) => JSON.parse(JSON.stringify(value));
      window.go = {
        main: {
          App: {
            GetDashboard: async () => clone(dashboardValue),
            ListRoutes: async () => clone(routesValue),
            ListSubscriptions: async () => clone(subscriptionsValue),
            SetCore: async () => undefined,
            SetCoreRunning: async () => undefined,
            SetSystemProxy: async () => undefined,
            SetTUN: async () => undefined,
            SetRuntimeMode: async () => undefined,
            SelectRoute: async () => undefined,
            TestRoute: async () => ({ id: "airport-node", reachable: true, latency_ms: 42, error: "" }),
            CreateUpstream: async () => undefined,
            DeleteUpstream: async () => undefined,
            AddSubscription: async () => undefined,
            RefreshSubscriptions: async () => undefined,
            RemoveSubscription: async () => undefined,
            TailLogs: async () => ({ lines: ["[navo] smoke test"] }),
          },
        },
      };
    },
    { dashboardValue: dashboard, routesValue: routes, subscriptionsValue: subscriptions },
    );

    await page.goto("http://127.0.0.1:4173", { waitUntil: "networkidle" });
    await page.getByText("代理链路已就绪", { exact: true }).waitFor();
    for (const core of ["sing-box", "Mihomo", "Xray"]) {
      await page.getByText(core, { exact: true }).first().waitFor();
    }

    const outputDir = path.resolve(__dirname, "..", ".tmp");
    fs.mkdirSync(outputDir, { recursive: true });
    await page.screenshot({ path: path.join(outputDir, "ui-overview.png"), fullPage: true });

    await page.getByRole("button", { name: "线路来源", exact: true }).click();
    await page.getByRole("button", { name: "上游代理", exact: true }).click();
    await page.getByRole("button", { name: "添加上游代理" }).click();
    await page.getByText("这里只接受 HTTP、HTTPS 或 SOCKS5。").waitFor();
    await page.getByLabel("名称").fill("Smoke Proxy");
    await page.getByLabel("服务器").fill("proxy.example.net");
    await page.screenshot({ path: path.join(outputDir, "ui-routes.png"), fullPage: true });

    await page.getByRole("button", { name: "机场订阅", exact: true }).click();
    await page.getByText("URL 已加密保存", { exact: true }).waitFor();
    if ((await page.locator("body").innerText()).includes("https://secret.example")) {
      throw new Error("subscription secret leaked into the rendered UI");
    }

    await page.setViewportSize({ width: 900, height: 620 });
    if (await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)) {
      throw new Error("desktop UI has an unexpected horizontal scrollbar at minimum size");
    }
    if (errors.length > 0) throw new Error(errors.join("\n"));
    process.stdout.write("UI smoke passed: desktop view, routes, secure subscriptions, minimum window.\n");
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exitCode = 1;
});
