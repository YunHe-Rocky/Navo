const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const dashboard = {
  core: {
    core_id: "sing-box", state: "stopped", pid: 0, uptime: 0,
    config_hash: "", restart_count: 0, last_error: "",
  },
  cores: [{ id: "sing-box", name: "sing-box", version: "1.13.14", installed: true, active: true }],
  proxy: { enabled: false, server: "127.0.0.1", port: 12080 },
  runtime: { mode: "global", active_id: "node-1", tun_enabled: false },
  tun: {
    installed: true, created: false, enabled: false, name: "Navo", mtu: 1500,
    state: "missing", identifier: "", interface_index: 0, fault_id: "", last_error: "",
  },
  capture: {
    state: "stopped", phase: "stopped", desired_mode: "off", committed_mode: "off",
    transition_id: "", fault_id: "", last_error: "", can_retry_tun: false,
  },
  metrics: {
    reachable: false, available: true, unavailable_reason: "", core_name: "sing-box",
    latency_ms: 0, upload_bytes: 0, download_bytes: 0, connections: 0,
  },
  ip: { proxy_ip: "", proxy_country: "", direct_ip: "" },
};

const routes = {
  active_id: "node-1",
  mode: "global",
  outbounds: [{
    id: "node-1", name: "Smoke Route", type: "vless", server: "edge.example.net",
    port: 443, provider_id: "provider-1", source_type: "airport_subscription",
    country: "JP", active: true,
  }],
};

async function main() {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1180, height: 760 } });
    const errors = [];
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });
    page.on("pageerror", (error) => errors.push(error.message));
    await page.addInitScript(({ dashboardValue, routesValue }) => {
      const clone = (value) => JSON.parse(JSON.stringify(value));
      const wait = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));
      const setPhase = (state, phase) => Object.assign(dashboardValue.capture, { state, phase });
      window.__navoSetTunFault = () => {
        dashboardValue.core.state = "stopped";
        dashboardValue.runtime.tun_enabled = false;
        Object.assign(dashboardValue.tun, {
          enabled: false,
          state: "disabled",
          fault_id: "fault-ui-smoke",
          last_error: "虚拟网卡被外部禁用，已安全回滚",
        });
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
            state: "starting_tun", phase: "stopping_old_mode", desired_mode: "tun",
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
            created: true, enabled: true, state: "enabled",
            identifier: "{00000000-0000-0000-0000-000000000001}", interface_index: 42,
          });
          Object.assign(dashboardValue.capture, {
            state: "running_tun", phase: "running", desired_mode: "tun",
            committed_mode: "tun", transition_id: "",
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
    }, { dashboardValue: dashboard, routesValue: routes });

    await page.goto("http://127.0.0.1:4173", { waitUntil: "networkidle" });
    await page.getByRole("button", { name: "连接管理", exact: true }).click();
    const tunButton = page.getByRole("button", { name: "TUN 接管系统网络层流量" });
    await tunButton.click();
    await page.getByText("正在恢复虚拟网卡", { exact: true }).waitFor({ timeout: 3000 });
    const modeButtons = page.locator(".capture-options button");
    if (await modeButtons.count() !== 3) throw new Error("capture mode button count mismatch");
    for (let index = 0; index < 3; index += 1) {
      if (!await modeButtons.nth(index).isDisabled()) throw new Error("capture mode buttons were not locked");
    }
    await page.getByText("运行中", { exact: true }).waitFor({ timeout: 5000 });
    if (!(await tunButton.getAttribute("class") || "").includes("selected")) {
      throw new Error("TUN did not become the committed UI mode");
    }

    await page.evaluate(() => window.__navoSetTunFault());
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: "虚拟网卡已异常停止" });
    await dialog.waitFor();
    const retry = page.getByRole("button", { name: "重新启动 TUN" });
    if (!await retry.evaluate((element) => element === document.activeElement)) {
      throw new Error("fault dialog did not move focus to the recovery action");
    }
    await retry.press("Escape");
    await dialog.waitFor({ state: "hidden" });

    await page.setViewportSize({ width: 900, height: 700 });
    if (await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)) {
      throw new Error("desktop UI has an unexpected horizontal scrollbar");
    }
    const output = path.resolve(__dirname, "..", ".tmp", "capture-ui-smoke.png");
    fs.mkdirSync(path.dirname(output), { recursive: true });
    await page.screenshot({ path: output, fullPage: true });
    if (errors.length > 0) throw new Error(errors.join("\n"));
    process.stdout.write(`Capture UI smoke passed: phases, lockout, fault dialog, keyboard, ${output}\n`);
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exitCode = 1;
});
