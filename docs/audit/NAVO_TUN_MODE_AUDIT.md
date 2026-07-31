# Navo TUN virtual adapter and mode-transition audit

Date: 2026-07-29

Scope: current implementation before applying
`docs/design/Navo_TUN虚拟网卡与模式切换_Codex作业指导书.md`.

## Root Cause

Capture mode is not an owned domain transaction. The User Agent serializes only
the immediate UI request, the Service stores an in-memory `TUNEnabled` flag,
WinINet state is managed separately in the user session, sing-box creates the
adapter as a side effect of core startup, and Supervisor independently monitors
and restarts the core. No component owns the complete invariant:

`desired mode = core state = adapter state = routes = DNS = system proxy`.

## 1. Current mode state storage

- `internal/service/runtime.go`
  - `runtimeState.TUNEnabled` is the Service-side TUN flag.
  - It is explicitly reset to `false` on Service startup and is excluded from
    JSON persistence.
  - `setTUNRuntime` changes the flag, regenerates/swaps core config, and restores
    only the previous in-memory runtime value when that call fails.
- `internal/agent/agent.go`
  - `captureMu` serializes calls entering `setCaptureMode`.
  - The current system-proxy state is queried from the WinINet manager.
- `internal/supervisor`
  - Supervisor owns another state machine for core process lifecycle.
- There is no authoritative `DesiredMode`, capture transition phase, transition
  ID, committed mode, or durable fault state.

## 2. Core start/stop entry points

- Agent `capture.set` calls Service `tun.enable`/`tun.disable`.
- Service `handleTUNEnable` and `handleTUNDisable` call
  `setTUNRuntime`.
- `setTUNRuntime` reaches `applyRuntimeConfig`, which compiles and swaps the
  active core config.
- `Supervisor.Restart`/`SwapConfig` stop and start the core.
- Supervisor's one-second crash monitor can independently reconcile and restart
  the core with 3/10/30-second fixed backoff.

Risk: adapter loss and a core crash are observed by different owners. The
Supervisor can restart a TUN-configured core while capture cleanup or a user
transition is occurring.

## 3. Adapter create/delete entry points

- `internal/network/tun/tun_windows.go` exposes Wintun `Create`, `Configure`,
  `Destroy`, `Status`, and `Cleanup`.
- The Service constructs this manager only for `network.Reconciler`.
- Normal `tun.enable` does not call `tun.Manager`; the selected core creates the
  Windows adapter as a startup side effect.
- `tun.Manager.Status` reports process-local fields, not Windows device state.
- `handleTUNStatus.created` mirrors `TUNEnabled`; it is not an adapter probe.
- The manager stores only an open handle and display name. There is no persisted
  GUID/LUID/interface index.
- `Destroy` closes the handle and then calls `WintunDeleteDriver`, coupling
  adapter teardown to driver removal.

Residual risk: a disabled, renamed, externally deleted, or orphaned adapter
cannot be identified reliably after process restart.

## 4. Route, DNS, and system-proxy mutations

- `internal/network/tun/route_windows.go`
  - Adds/removes routes with `netsh`.
  - Cleanup parses localized command output and searches for the display name
    `Navo`.
- `internal/network/tun/dns_windows.go`
  - Sets/resets adapter DNS with `netsh` using the display name.
- sing-box TUN config also uses automatic route/DNS behavior, so those changes
  are not represented in Navo's lower-level network undo journal.
- `internal/agent/agent.go`
  - `EnableProxy` probes the local proxy before changing WinINet.
  - `DisableProxy` restores/disables WinINet through the user-session manager.
- Service cannot own WinINet because it runs outside the interactive user
  session; the User Agent must remain the high-level transaction boundary.

## 5. UI call chain

`navo_app/frontend/src/App.vue:setCapture`

→ `navo_app/frontend/src/api.ts:setCaptureMode`

→ `navo_app/app.go:App.SetCaptureMode`

→ User Agent `capture.set`

→ Service `tun.enable` / `tun.disable`

The UI currently exposes only a global busy flag and a completion toast. It
does not expose transition phase, desired mode, rollback, recovery, or a
once-only adapter-failure action.

Legacy Wails methods `SetTUN` and `SetSystemProxy` still bypass the unified
`capture.set` entry and are public bypass paths.

## 6. Concurrency risks

1. Agent `captureMu` does not serialize Service-side core selection, manual core
   start/stop, Supervisor crash recovery, or tray actions that use low-level
   methods.
2. Service `runtimeMu` protects fields but is released before the entire
   `setTUNRuntime` operation commits.
3. Supervisor uses fixed sleeps in crash recovery and has no capture-transition
   cancellation/suppression token.
4. `context.Background()` in TUN handlers prevents UI cancellation and bounded
   transition deadlines.
5. Rollback attempts to restore the previous TUN mode by issuing another
   partial operation; it does not restore a journaled resource set.

## 7. Residual-adapter and empty-core risks

- Core startup may fail after creating the Wintun adapter but before the local
  proxy port becomes ready, leaving the adapter and automatic network changes.
- `TUNEnabled` may be restored to `false` while an OS adapter still exists.
- `TUNEnabled=true` may be reported while the adapter is missing or disabled.
- Switching to System Proxy disables TUN before enabling WinINet; failure to
  enable WinINet tries to recreate TUN without a durable recovery record.
- Switching to Unmanaged does not guarantee that core is stopped; "off" means
  capture off, not the guide's required no-core unmanaged state.
- Supervisor may restart a core after TUN failure, producing an empty or
  repeatedly failing core without a valid adapter/data plane.
- Startup recovery runs only for a dirty-shutdown marker and does not reverse
  every incomplete capture transition.

## Required implementation boundary

The User Agent remains the only public capture coordinator because it owns the
interactive WinINet state. It must call a serialized Service transition API for
core/TUN work, persist a transition journal, expose a single capture status, and
remove all public low-level bypasses from UI/tray. The Service must expose real
adapter state, suppress Supervisor restart during capture recovery, monitor the
adapter while TUN is starting/running, and clean only resources owned by the
current transition.

