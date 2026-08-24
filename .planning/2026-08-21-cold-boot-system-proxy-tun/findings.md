# Findings

## Restored Context

- Existing uncommitted work is a completed UI thin-layer refactor; do not overwrite or redesign it.
- Prior product history changed the generic primary connection action to TUN, while architecture rules require explicit mutually exclusive `system_proxy` and `tun` intents.
- Prior cold-start work added startup recovery/deferred activation, but the currently executed package and actual registration still need live confirmation.

## Current Boot Baseline

- Windows boot time: 2026-08-21 13:52:09 +08:00.
- At inspection, no Navo process and no Navo listener existed.
- v2rayN PID 3348 and its `D:\v2rayN-windows-64\bin\sing_box\sing-box.exe` PID 8164 own `127.0.0.1:10808`; preserve them.
- No Navo Run registry entry, scheduled task, Windows service, or Startup-folder shortcut exists.
- The repository portable launcher is version 1.0.36.0, SHA-256 `3265A0158C32998DF81B5DC571C7109CE6CCB31870CEB2AB2D59E2226C33A12C`.
- `%LOCALAPPDATA%\Navo\structured.log.jsonl`, `sing-box.log`, `runtime_state.json`, and `agent\capture_transition.json` were updated around 13:58, proving a Navo launch occurred after boot and exited.

## Source Baseline

- `cmd/navo/tray_menu.go` exposes distinct System Proxy and TUN menu actions.
- `internal/agent/agent.go` maps `tun.enable` to TUN and `proxy.enable` to System Proxy; `capture.set` is the typed mode boundary.
- No source match currently implements Windows auto-start registration via Run, scheduled task, service, or Startup shortcut.
- The structured log schema is `id,timestamp,level,service,component,message,fields`; diagnostic details are nested under `fields`.

## Reproduction Evidence

- At 13:57:17 and 13:57:41 the UI sent `capture.set` with target `system_proxy`; the Agent journal also records `from=off`, `to=system_proxy`. The user did not accidentally select TUN.
- Both Service `capture.prepare` requests failed after about 19 seconds with `TUN_HTTPS_VERIFY_FAILED` while verifying Google through `127.0.0.1:12080`.
- The core log shows Google, ChatGPT, OpenAI API, and WebSocket destinations were routed to `outbound/direct[direct]` and timed out; the local listener then forcibly closed the client connection.
- The third System Proxy attempt was interrupted by `ui.exit`, left an uncommitted `starting_core` journal, and launcher shutdown timed out before the Job Object cleanup completed.
- Current `runtime_state.json` has `selected_outbound=""`, mode `bypass_mainland`, and candidate revision status. `runtimeRoutingPolicy` therefore substitutes `selectedTag="direct"`, making the default route direct.

## First Incorrect Boundaries

- `verifySelectedOutboundReachable` returns success when the selected outbound ID is empty, so non-direct System Proxy activation proceeds with no proxy route.
- `verifyActiveRuntimeRouting` reuses TUN-prefixed data-plane errors for System Proxy activation, producing the misleading TUN failure reported by the user.
- `handleCapturePrepare` writes every capture failure into the global TUN fault slot even when the requested mode is System Proxy.
- No Windows auto-start registration exists, so post-logon direct invocation cannot occur automatically regardless of runtime recovery correctness.

## Startup-Connection Requirement

- The Windows acceptance design explicitly says: disabled `开机连接` must leave the machine direct; enabled `开机连接` may report connected only after end-to-end verification.
- The architecture assigns login auto-start to the current-user Agent. In the current combined portable architecture the elevated launcher hosts that Agent, so a user-enabled highest-privilege logon task is required; an HKCU Run entry cannot reliably launch the `requireAdministrator` executable.
- The existing launcher already supports `--silent`, and Agent startup already exposes the UI pipe before ownership-aware recovery. Add a distinct `--startup` intent and route post-recovery activation through the same Connection Coordinator with origin `startup`.
- Default must remain disabled. Enabling stores an explicit desired capture mode (`system_proxy` or `tun`); it must never infer permission to autoconnect from a previous interactive click.
- The current profile contains neither `subscriptions.json` nor `upstream_proxies.json`; no selected outbound can be restored. Navo must not silently adopt the unrelated v2rayN listener.
