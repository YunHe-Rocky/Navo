import type { BackendProvider, RuntimeProvider } from "./client";

export function createSystemApi(backend: BackendProvider, runtime: RuntimeProvider) {
  return {
    startupSettings: () => backend().GetStartupSettings(),
    configureStartup: (enabled: boolean, mode: "system_proxy" | "tun") => backend().SetStartupSettings(enabled, mode),
    requestExit: () => backend().RequestExit(),
    minimizeToTray: () => backend().MinimizeToTray(),
    onCloseRequested: (callback: () => void) => runtime()?.EventsOn("navo:close-requested", callback) ?? (() => undefined),
    setTheme: (mode: "day" | "night") => {
      const target = runtime();
      if (!target) return;
      if (mode === "day") {
        target.WindowSetLightTheme();
        target.WindowSetBackgroundColour(233, 222, 212, 255);
        return;
      }
      target.WindowSetDarkTheme();
      target.WindowSetBackgroundColour(20, 16, 39, 255);
    },
  };
}
