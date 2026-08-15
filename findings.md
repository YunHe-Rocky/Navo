# Findings

## 2026-08-15 Phase 57: crash-safe DNS and boot proxy recovery

- User reports two production symptoms: an abrupt system crash can leave Windows DNS corrupted/unusable, and proxy activation fails after boot.
- Existing project history already treats Service as the owner of TUN DNS/route/firewall rollback and requires recovery to finish before capture transitions; the new audit must verify whether boot ordering and journal ownership actually enforce that invariant.
- The current worktree contains extensive intentional connection-chain changes through Phase 56. Phase 57 must preserve them and use focused diffs/tests rather than resetting or reconstructing the repository.
- Current boot time is 2026-08-15 10:07:20. No Navo process or listener on the known local ports is active; the only matching core is the unrelated v2rayN-owned `D:\v2rayN-windows-64\bin\sing_box\sing-box.exe`, which remains untouched.
- Both physical interfaces currently use `223.5.5.5`, but registry evidence proves it is their DHCP-provided `DhcpNameServer` with an empty static `NameServer`; current physical DNS is not a stranded Navo mutation. WinINet remains owned by v2rayN at `127.0.0.1:10808`.
- No Navo HKCU/HKLM `Run` value, scheduled task, or Windows service is registered. The reported post-boot failure therefore concerns Navo's first launch/activation path, not a currently registered OS autostart entry.
- `runtime_state.json` was last written before the current reboot and remains `revision_status=candidate`; `recovery_state.json` claims a clean exit on 2026-08-14. The Navo root profile contains no visible capture journal, so any DNS recovery must also be checked in the elevated Service profile and Journal V2 location.
- Structured logs end with a graceful `ui.exit` at 08:47:12 before the 10:07 reboot; there is no Navo launch after this boot in the normal-user profile.
- Current TUN flow does not rewrite physical-adapter DNS. It creates an owned `.` NRPT rule pointing to `172.19.0.2`; the older `internal/network/tun` DNS manager is not in the Service activation path.
- Journal V2 is written and flushed before every route/NRPT/firewall mutation, records pending operations before apply, and uses ownership-scoped idempotent undo. The journal file itself is power-loss hardened through `Sync` plus Windows `MOVEFILE_WRITE_THROUGH`.
- Confirmed startup race: Service recovery can legitimately consume the Manager's 30-second rollback window, but Launcher waits only 10 seconds for `svc.Ready()` and then calls `fatal`. A crash-recovery boot can therefore terminate the owning process while its recovery is still running, leaving DNS/NRPT recovery incomplete and making the first post-boot proxy activation unavailable.
- Confirmed System Proxy recovery bug: `Agent.Run` logs and ignores `RecoverOwned()` failure, exposes the UI pipe, and permits activation. `Manager.Enable` then deterministically returns `stale proxy ownership must be recovered before enabling`, producing a post-boot proxy error instead of keeping the capture state faulted with a retryable recovery action.
- Supervisor has a second fail-open boundary: it records a Reconciler error but still transitions to Ready and starts the core. Reconciliation failure must block readiness/start instead of allowing proxy/TUN activation on dirty network state.
- Existing unit tests cover rollback after `after-nrpt` failure injection, but that path rolls back in the same process. It does not simulate process death after the NRPT mutation followed by a fresh Manager recovering the durable journal.
- Add a panic-boundary crash test that leaves the V2 journal intact, creates a fresh Manager, verifies owned NRPT undo, and requires journal deletion only after recovery succeeds.

## 2026-08-11 Phase 41 network privilege boundary

- Root cause: `runtime_state.json`, `tun.enable`, and `tun.config` could supply an arbitrary syntactically valid `TUNName`. That value reached adapter inspection, cleanup, core config generation, and route binding without proving the target was Navo-owned.
- Production code contains no `Disable-NetAdapter` call. The only such command is in the elevated acceptance script and is scoped to the captured TUN adapter object.
- Runtime DNS/address/MTU mutation exists only in the legacy `internal/network/tun` manager; the current Service flow uses a core-created Wintun adapter plus exact Journal V2 routes, NRPT, and firewall resources.
- New safety invariant: the only mutable adapter identity is canonical `Navo`; persisted or IPC-supplied names such as `Ethernet`, `以太网`, and `Wi-Fi` are rejected or migrated before privileged action.
- Real installed acceptance artifacts identify the owned adapter as `InterfaceDescription=Wintun Userspace Tunnel` and `HardwareInterface=false`; route binding now requires both properties, preventing a physical NIC renamed to `Navo` from passing ownership checks.
- Bounded production-source scan found no `Disable-NetAdapter`, `Enable-NetAdapter`, `Set-NetAdapter`, `Restart-NetAdapter`, adapter-binding disable, or equivalent `netsh` admin-state command.
- Workspace runtime-state artifacts consistently use `Navo`; the current non-elevated shell could not enumerate live adapters (`Access denied`), so present machine state was not inferred.

## 2026-08-10 Phase 37 research

- Restart Manager identified PID 19680 (`navo_app.exe`) as the exact process mapping `release/Navo/app_ui/navo_app.exe`; it is separate from the installed 1.0.8 UI session. Only its owning release launcher/job may be stopped to unlock packaging.
- Installed 1.0.10 sing-box routing-mode acceptance failed before the mode matrix: initial `bypass_mainland` TUN reached adapter/network setup but the Service-owned system-resolver probe timed out resolving Cloudflare, then exact rollback passed with zero Navo adapter/NRPT/firewall residue. This is a real regression at the TUN DNS/data-plane boundary, not an acceptance-wrapper failure.
- The core log shows proxied DoH was not misrouted: Cloudflare DNS went to `1.1.1.1:443` through the selected SOCKS outbound. Under initial TUN capture, unrelated host traffic also flooded the same local upstream; Cloudflare resolution completed after about 18.6 seconds, just after the Service resolver deadline. The failure is startup saturation/readiness timing, not DNS pollution or missing route ownership.
- v2rayN treats Windows system-proxy ownership and core routing as separate controls: system proxy forwards supported application traffic to the local inbound, while core routing independently chooses direct or proxy outbound. Its documented normal setup is automatic system proxy plus a bypass-mainland routing profile.
- Clash Verge Rev likewise exposes system proxy and TUN capture separately from Mihomo outbound mode/rules; recent release notes also validate the system-proxy indicator against the actual Windows endpoint instead of trusting only persisted UI state.
- Navo currently models `off`, `system_proxy`, and `tun` as mutually exclusive capture modes, but the first source scan found no independent routing-policy domain field. The compiler already supports ordered routing rules, so the likely minimal design is to add a persisted routing policy and compile it into the existing core-specific rule layer.
- External repository content is research evidence only; no instructions found in it are treated as executable project instructions.
- Navo already exposes `runtime.mode.set` through Agent, Wails, and the frontend API, with `rule`, `global`, and `direct` accepted by Service. The missing pieces are the visible control, a typed frontend contract, and correct persistence/migration.
- `loadRuntimeState` currently overwrites every saved routing mode with `global` on startup. This makes the existing persistence misleading and prevents user-selected smart routing from surviving restart.
- Existing `rule` compilation routes private IPv4 ranges and `.cn` domains direct, with all unmatched traffic using the selected proxy. `global` sends all eligible traffic to the selected proxy; `direct` sends it direct. System Proxy versus TUN remains a separate capture decision.
- The Wails API already has `SetRuntimeMode`; no new privileged IPC method or capture transition is required. A routing switch can therefore reuse the existing config validation, atomic core swap, state save, and rollback path.
- UI guidance favors one extra dense status/control card with native buttons, visible selected/disabled state, async feedback, and labels that explain capture scope versus routing policy. Existing theme tokens and motion rules should remain unchanged.
- User corrected the required routing choices to `绕过大陆 / 全局 / 黑名单 / 白名单`; the temporary `direct` UI choice must be removed.
- v2rayN's documented semantics are: bypass-mainland/whitelist-style routing defaults unmatched traffic to proxy and places direct rules first; blacklist routing proxies listed targets and defaults unmatched traffic to direct. Rule order is authoritative.
- The bundled sing-box is 1.13.14 and has no packaged `.srs` rule sets; Mihomo is 1.19.29, while only Xray ships `geoip.dat` and `geosite.dat`. Therefore a cross-core implementation cannot assume Geo files for every core. Navo needs core-neutral ordered domain/IP rules for its built-in presets, with explicit defaults.
- Creating an upstream proxy previously forced runtime mode to `global`; this would silently overwrite any of the new four choices. Selection/creation must preserve the configured routing mode and only change the selected outbound.

## 2026-08-08 delayed TUN restart incident

- Installed Navo is still 1.0.3 (`23ED...789`), while the built workspace release is 1.0.4 (`CFFA...B4D`). The serialized capture/health-owner fixes are therefore not present in the binary the user launches.
- No Navo process, split route, NRPT policy, WinINet proxy, or visible `172.19.0.x` adapter residue was present at the initial snapshot.
- `%LOCALAPPDATA%\Navo` denies the current interactive user even for ACL inspection. Earlier `fsatomic.restrictPath` derives the process token SID; under over-the-shoulder elevation this can protect the original user's profile for the administrator credential identity, breaking cross-identity launcher/UI/state/log access.
- The installed package log ends at the 2026-08-06 acceptance run; current incident evidence is inside the protected profile and requires an elevated, sanitized export.
- The accessible installed log contains the same delayed failure signature: TUN reaches `DATA_PLANE_VERIFIED` and Agent reports `running`; about seven seconds later Supervisor transitions `running -> stopping -> stopped`, then Agent reports `TUN adapter is unavailable (state=)` and marks capture retryable. This proves the restart prompt is a secondary control-plane race after a successful data plane, not initial TUN establishment failure.
- On explicit TUN-to-Off transitions the Agent health loop also wakes during Service teardown and logs `capture transition is already in progress` repeatedly. The Agent loop observes intermediate teardown state without a transition-generation/ownership guard.
- The Vue modal is driven solely by Agent `capture.state=faulted` plus `can_retry_tun`; it faithfully displays the false fault as “虚拟网卡已异常停止 / 重新启动 TUN”. UI wording is not the cause.
- Service already owns `monitorTUNAdapter`, suppresses Supervisor restart around rollback, and SelfHeal observes Supervisor crash events. Agent simultaneously owns `monitorCaptureHealth` and performs another capture rollback, creating duplicate health and recovery authorities.
- `Service.monitorTUNAdapter` fails closed on one non-`enabled` `GetAdaptersAddresses` sample every three seconds. It neither debounces consecutive observations nor re-inspects after acquiring `captureMu`; a transient/unknown/stale adapter state therefore stops a fully verified core within seconds.
- The monitor only rechecks `runtime.TUNEnabled` under the lock. It does not verify the activation generation, committed health stage, adapter GUID/index, current Supervisor state, or a second adapter observation before destructive rollback.
- `handleTUNStatus` performs two independent adapter reads (`tun.InspectAdapter` and `network.InspectAdapterSnapshot`) without a shared observation/generation, so the returned enabled/state/identity fields can describe different instants during transition or recovery.

## 2026-08-06 installed TUN failure

- A stale Navo-owned NRPT rule for namespace `.` and DNS `172.19.0.2` caused host-wide DNS timeouts before Navo startup; deleting the exact `Navo:TUN:*` rule restored resolution.
- Real installed golden TUN is healthy against the user's remote profile; a direct local helper cannot satisfy exit-IP separation and must bind its own egress to the physical interface to avoid recursive capture.
- The remaining repeat/switch failures came from two capture owners racing: Service already owns TUN adapter/core monitoring, while Agent also interpreted transient Supervisor state and began rollback. User transitions then saw `CAPTURE_BUSY`.
- Capture requests must serialize and wait within their request context; TUN runtime health must have one fail-closed owner. Agent retains system-proxy core monitoring while Service owns TUN monitoring and rollback.

## 2026-07-29 TUN virtual adapter and mode-transition audit

- The public capture path is `Vue App.vue -> api.setCaptureMode -> Wails
  App.SetCaptureMode -> User Agent capture.set -> Service tun.*`.
- Capture state is split: the Service persists a runtime selection plus the
  in-memory `TUNEnabled` flag, the User Agent owns WinINet system-proxy state,
  and Supervisor owns an independent core lifecycle/restart state machine.
- `setTUNRuntime` mutates `TUNEnabled` before applying a generated config and
  restores only that in-memory snapshot on error; it is not a durable
  cross-resource transaction.
- The existing recovery state records dirty shutdown details, while the
  lower-level network journal records command undo actions. Neither represents
  the requested capture transition fields (`from`, `to`, `currentStep`,
  adapter/routes/DNS/proxy backup, core PID, committed).
- The Windows Wintun manager keeps adapter identity only in process memory
  (`uintptr` handle plus display name). Its current `Destroy` path also calls
  `WintunDeleteDriver`, coupling adapter teardown to driver removal.
- Route and DNS cleanup discover resources by adapter display name/output text,
  so renamed, disabled, or missing adapters cannot be reconciled reliably.
- Supervisor polls core liveness every second and may independently restart it.
  A TUN adapter failure therefore has no atomic path that first suppresses core
  restart, stops core, restores network state, and exposes a single fault.
- The repository also has a newer transactional `internal/network.Manager`
  with an undo journal for routes/NRPT/firewall, but it deliberately waits for
  sing-box to create the adapter and is not wired into `Service.New`; the older
  `network.Reconciler` and display-name Wintun manager are wired instead.
- The coordinator boundary will remain in User Agent because Windows Service
  cannot safely mutate the interactive user's WinINet settings. Service will
  expose one serialized `capture.prepare` operation and real TUN health data.

## 2026-07-29 Windows installer

- The canonical application payload remains `release\Navo`; the installer must
  package that directory without changing the local Named Pipe architecture.
- The launcher requires Administrator privileges for TUN. Installer install and
  uninstall operations must also use an elevated per-machine scope.
- The existing portable packaging workflow must remain independently usable.
- The 2 GB workspace is primarily development output, not the distributable:
  `.cache` is 1,651.60 MiB, `release` is 174.07 MiB, `third_party` is
  152.27 MiB, and `navo_app` is 131.29 MiB.
- `.cache/go-build` alone is 1,026.31 MiB. Additional duplicated caches are
  `.cache/go-path` 282.63 MiB, `.cache/npm` 133.95 MiB,
  `.cache/gocache` 113.13 MiB, and `.cache/go-mod` 33.01 MiB.
- The actual release payload is dominated by `release/Navo/third_party` at
  152.27 MiB: Mihomo 45.24 MiB, sing-box 43.30 MiB, Xray 33.96 MiB, plus
  Xray GeoIP/geosite data. The Wails UI executable is only 10.89 MiB.
- Installer input must be restricted to the release payload and must exclude
  repository caches, node_modules, Wails build caches, smoke profiles, logs,
  runtime data, and crash dumps.
- The Vue frontend does not currently run a periodic timer; dashboard refreshes
  occur on mount and explicit actions. Event/polling load is therefore already
  low and should not be optimized speculatively.
- The launcher always starts `navo_app.exe --start-hidden`, while Wails uses
  `HideWindowOnClose: true`. This keeps WebView2 resident even when the user
  only uses the tray. On-demand UI creation is the highest-value runtime-memory
  optimization.
- The launcher currently treats UI process exit as application shutdown.
  On-demand UI requires an owned UI process manager so UI exit no longer stops
  Agent/Service/core, tray open can restart UI, and shutdown still terminates
  any current UI child.
- In the combined launcher, Agent to Service calls use `SendToServiceFn` and
  therefore execute in-process. Consolidating Dashboard primarily removes
  repeated Wails-to-Agent Named Pipe connections and creates one consistent
  response boundary.
- `GetDashboard` currently performs six cheap status calls plus synchronous
  `ip.check`. The latter has a 15-second context and is the only expensive
  first-screen operation. `runtime.status` already exposes cached `exit_ip` and
  `exit_country`, so Dashboard can be made network-free.
- `scripts/package.ps1` copies the developer's real `.env` into the release.
  Installer preparation must remove this behavior; only a non-secret template
  may be shipped, with runtime state remaining under the user data directory.
- The release includes two Wintun DLL copies and developer integration headers,
  but this duplication is less than 0.5 MiB. Core executables and Xray data,
  not Wintun metadata, dominate the 174 MiB payload.
- After the Phase 19 package rebuild, `release/Navo` is 173.73 MiB across 25
  files and contains no `.env`. The release launcher SHA-256 is
  `605F047D1A52B609C67B60F39AFD34AC0D8557F70CB3625CC95B9064DDC89C12`.
- No Inno Setup, NSIS, or WiX compiler is installed system-wide. WiX 7.0.0
  requires accepting the OSMF EULA at execution time, so it cannot be silently
  accepted on the user's behalf. The build pins WiX 5.0.2 into
  `.cache/tools/wix`; it provides the required MSI feature set without that
  license gate or a machine-wide installation.
- MSI is the selected installer format: per-machine installation under Program
  Files, MajorUpgrade handling, Windows Apps uninstall registration, embedded
  compressed payload, and Start Menu/Desktop shortcuts.
- Installer staging must be regenerated from `release/Navo` while excluding
  `log`, `data`, `.env`, and test artifacts, because runtime smoke can add files
  to the portable release directory after packaging.

## 2026-07-29 TUN DNS and ICMP data plane

- Real sing-box log evidence shows every Windows DNS packet for
  `172.19.0.2:53` was sent to the selected SOCKS outbound. No internal DNS
  module or `hijack-dns` route rule existed, so TUN created a DNS black hole.
- sing-box 1.13.14 supports `network: icmp` routing only to Direct, WireGuard,
  or Tailscale outbounds. HTTP, SOCKS, Shadowsocks and typical airport proxy
  protocols cannot transport actual ICMP echo traffic.
- Bare-network verification with Navo capture disabled passed both IPv4
  `1.1.1.1` and IPv6 Baidu ping with zero loss. System proxy therefore already
  leaves ICMP on the direct physical route; pinging a host that blocks direct
  ICMP cannot be made to traverse WinINet.
- Required repair: add an explicit TUN DNS server, a `hijack-dns` rule, a
  default domain resolver for outbound server hostnames, and route ICMP direct
  before the selected proxy final.
- sing-box's modern UDP DNS server already uses a direct dialer by default.
  Setting `detour` to an otherwise empty direct outbound passes `sing-box check`
  but fails when the DNS service starts. Runtime-start liveness must accompany
  native static validation.

## 2026-07-29 TUN privilege regression

- The active `Agent.setCaptureMode("tun")` path calls Service `tun.enable`
  without consulting `Config.IsElevatedFn`.
- `Config.IsElevatedFn` and the Windows `processIsElevated` helper still exist,
  but `Agent.New` does not install a default and the dispatch path also forwards
  raw `tun.enable` directly to Service. The previously documented privilege
  preflight is therefore absent from the current runtime path.
- The reported `CORE_004` is only a wrapper. The actionable core output is
  sing-box `configure tun interface: Access is denied`.
- The acceptance architecture requires a LocalSystem `Privileged Service` for
  TUN/route/DNS/core. The shipped `cmd/navo` instead constructs Service
  in-process, and `cmd/navo-svc install` is only a message-printing stub.
- Therefore the current "delegate privilege decision to Service" test is a
  false architectural assumption: the delegated Service has exactly the same
  non-elevated token as Agent.
- The fixed release manifest did elevate successfully: PID 16836 launched with
  UI PID 11888 and release sing-box PID 9300; listener 127.0.0.1:12080 accepted
  TCP connections.
- The non-elevated acceptance client was denied by the elevated UI Named Pipe.
  The existing `OW` ACE resolves unreliably for an elevated creator. The source
  now derives the token user's concrete SID and grants only that SID, SYSTEM,
  and Administrators.
- The elevated runtime has not received a `capture.set(tun)` request yet, so
  adapter/route/DNS/egress acceptance remains unproven.

## 2026-07-29 Windows proxy/TUN acceptance

- Root cause: `Agent.setCaptureMode(system_proxy)` enables WinINet immediately
  after `tun.disable`; it performs no HTTP protocol or end-to-end proxy probe.
  Core readiness currently proves only process/listener state.
- `Service.dispatch` still exposes `proxy.enable`, `proxy.disable`, and
  `proxy.status` and directly constructs `systemproxy.Manager`, violating the
  required session boundary. WinINet ownership must remain in User Agent.
- `handleTUNStatus` derives `created`, `enabled`, and a hard-coded
  `route_count=1` solely from `runtime.TUNEnabled`; it does not inspect the
  adapter, routes, DNS, or end-to-end data plane.
- The transactional `internal/network.Manager` implements route/DNS journaling
  but is not wired into Service runtime. Current TUN behavior relies on each
  core's generated auto-route configuration.
- Mihomo TUN generation omits `auto-detect-interface`, `dns-hijack`, and the
  required DNS module. Xray config generation contains no TUN inbound at all.
- `scripts/tun_smoke.ps1` used the real `%LOCALAPPDATA%`, which is not writable
  in the managed validation environment and also made the acceptance run depend
  on user state. It now uses an isolated repository-local profile like the full
  smoke test.

## 2026-07-28 System tray

- The stable boundary is one on-demand `tray.snapshot` composed by Agent from
  Service APIs. This avoids polling and prevents the native menu from owning or
  guessing runtime state.
- Dynamic endpoint health belongs in Service because compatibility and network
  probes are core-dependent. Tray only renders Service-provided color, reason,
  timestamp and latency.
- Win32 popup menus can be rendered recursively with checked and disabled flags;
  operational commands must leave the message-loop thread before performing
  Agent calls.
- Real UI automation must target the native popup-menu window class `#32768`;
  generic SendKeys is captured by the foreground Codex window in this desktop
  environment.

- The user has removed or is removing Flutter by design.
- `scripts/package.ps1` still hard-codes `D:\Flutter\flutter` and is the direct
  source of the reported error.
- The Go Service, Agent, Named Pipe protocol and three core integrations must
  remain intact; only the desktop UI implementation and build pipeline change.
- Wails v2 is the selected stable Windows UI host, using WebView2 and Vue 3.
- The existing launcher contract is intentionally simple: it starts
  `app_ui\navo_app.exe --start-hidden`, finds the window by title, and keeps
  Service/Agent in the launcher process.
- The existing Flutter MethodChannel only forwards a framed JSON request to
  `Navo.UI.Agent.v1`; the Go `internal/pipe` package already provides the same
  client transport, so the Wails host can reuse it directly.
- A single Wails method accepting and returning JSON preserves all current IPC
  methods without coupling the UI host to Service domain types.
- The workstation has Go 1.26.4 and Node 26.2.0. PowerShell blocks `npm.ps1`,
  so repository scripts must call `npm.cmd` explicitly.
- The launcher currently finds the UI by Flutter's
  `FLUTTER_RUNNER_WIN32_WINDOW` class and loads its tray icon from
  `flutter_assets`; both paths must become UI-framework-neutral.
- Wails v2.12.0 resolves successfully with Go 1.26.4 when all Go caches are
  redirected into the repository; the managed environment blocks the default
  user-wide Go module and checksum caches.
- `vue-tsc` 3.3.8 is incompatible with the new TypeScript 7 package export
  boundary; TypeScript 6.0.3 is compatible and passes the full Vue typecheck.
- The new execution document has 2,122 UTF-8 lines and 36 top-level sections.
- Section 35 is the immediate controlling task: create
  `docs/audit/NAVO_CURRENT_STATE.md`, audit all current call chains and gaps,
  then stop before domain-model or adapter implementation.
- The document still describes Flutter as mandatory, while the current
  repository was explicitly migrated to Wails by the user. The audit must
  record this current-state divergence rather than silently reintroducing
  Flutter during Phase 0.
- The target chain is `Windows UI -> Agent -> Service -> Core Host -> one
  runtime core`; sing-box, Mihomo, and Xray must be mutually exclusive at
  runtime.
- `AirportSubscription` and `UpstreamProxy` are mutually exclusive source
  modes. Automatic mixing and implicit proxy chaining are prohibited.
- The target domain must preserve protocol-specific semantics such as Reality,
  TLS, and transport. The current audit must identify any lossy generic node
  model and plaintext credential handling.
- Core Host is intended to be a strict privilege boundary: callers may select a
  known core and controlled operation, but may not supply arbitrary executable
  paths, arguments, scripts, or untrusted configuration locations.
- Third-party delivery metadata must eventually include
  `THIRD_PARTY_NOTICES.md`, `CORE_MANIFEST.json`, and `LICENSES/`, with exact
  core source, version, hash, and license data.
- Subscription ingestion is specified as a constrained pipeline:
  fetch/security limits -> format detection -> dedicated parser -> normalized
  endpoint -> domain validation -> deduplication -> transactional persistence.
  Remote Clash configuration may contribute node data only, never local TUN,
  DNS, controller, script, path, UI, rule-provider, or download behavior.
- Upstream proxies are a first-class source model supporting HTTP, HTTPS, and
  SOCKS5. Ambiguous text imports require explicit protocol selection; HTTP
  must not advertise UDP support.
- Compatibility must be resolved before compilation with supported,
  supported-with-limitations, or unsupported results and explicit reasons.
- Each core requires an independent adapter and native compiler/validator;
  configurations must never be translated through another core's format.
- Connection and switching are revisioned transactions with native config
  validation, active health probes, capture-mode application only after
  readiness, and rollback to Last Known Good on failure.
- Ports must come from one `PortPlan`, and the Agent must obtain actual local
  endpoints from the Service rather than guessing them.
- IPC selection requests must carry typed IDs and enums only, never arbitrary
  executable paths, raw passwords, whole configs, or launch arguments.
- UI success is defined by actual process, listener, revision, egress probe, and
  capture state—not by a click or IPC acknowledgement.
- The target persistence model is SQLite with singleton `active_selection`,
  revision/LKG records, compatibility data, recovery snapshots, and migration
  from legacy sing-box and proxy data. User data must never be cleared; an
  unreliable migration must remain disconnected.
- Diagnostics require stable error codes, structured `AppError`, operation and
  revision identifiers, validator/probe details, and centralized redaction of
  credentials, URLs, full configs, headers, and visited domains.
- TUN support must be version- and adapter-tested per core. An unsupported core
  must be disabled explicitly; running a second core to fake TUN is forbidden.
- Required verification spans unit, golden, six source/core combinations,
  native config validators, integration, and real Windows 11 VM E2E. A running
  process alone is not a passing connection.
- Implementation order is deliberately staged: audit, domain freeze,
  sing-box adapter extraction, Mihomo, Xray, compatibility, subscriptions,
  upstream proxies, transactional selection, IPC/UI, and finally TUN/recovery.
- The full execution document has now been read through line 2,122. Section 35
  supersedes implementation work for this turn and requires stopping after the
  audit is written.
- The current repository is compact but already split into launcher, Agent,
  Service, host, compiler, subscription, storage, supervisor, recovery, network,
  TUN, pipe, and Wails UI packages. There is no `docs/audit/` yet.
- Core files currently live at `third_party/{sing-box,mihomo,xray}/` rather than
  the document's proposed versioned `third_party/cores/...` layout.
- The current config fixtures are only `configs/test_local.json` and
  `configs/test_tun.json`; the required per-core golden configuration hierarchy
  is absent.
- Actual UI entry is Wails (`navo_app/main.go`) with a generic
  `App.Request(method, payload)` bridge over `Navo.UI.Agent.v1`; `CLAUDE.md`
  remains stale and still documents Flutter/MethodChannel and sing-box-only
  product positioning.
- Published launcher is `cmd/navo`, while `cmd/navo-svc` and `cmd/navo-agent`
  are standalone development entries. The launcher embeds Service and Agent in
  one process and launches the Wails UI as a child.
- `cmd/navo-svc` exposes caller-supplied core executable/config paths through
  flags. This is acceptable as a development CLI today but conflicts with the
  target Core Host trust boundary if retained in production.
- Storage search found no SQLite or `database/sql` usage. Current persistence is
  JSON-file based through `internal/storage.Store` and subscription-specific
  JSON persistence.
- The runtime hot path is centralized in
  `service.applyRuntimeConfig -> compiler.GenerateForCore`, with create/select/
  subscription handlers all converging there. This is the leading candidate
  for the shared airport/upstream failure point and requires exact inspection.
- `compiler.Outbound` is a flat, protocol-agnostic struct containing plaintext
  credentials and partial protocol fields. It lacks typed source identity,
  credential references, plugin options, VMess alterId, HTTP upgrade, early
  data, packet encoding, WireGuard keys/peers, and several modern transport/
  TLS details.
- `compiler.Config` is explicitly documented and implemented as a sing-box
  source model. `DefaultCompiler` always calls sing-box `Generate` and
  `sing-box check`; revisions exist only in memory and config files, not in a
  durable store.
- Multi-core support is a single `GenerateForCore` switch, not three adapters.
  `Compatible` declares all Mihomo outbounds compatible without field-level or
  version/capture checks, and its Xray list claims WireGuard support although
  `xrayOutbound` has no WireGuard case.
- Mihomo and Xray generators are incomplete: Mihomo lacks controller secret and
  full DNS/control configuration; Xray derives SOCKS as `HTTPPort + 1`, ignores
  TUN/DNS/policies/API, flattens routing to one final rule, and returns early
  for HTTP/SOCKS/SS before applying transport/TLS logic.
- `DefaultCompiler.Apply` writes sensitive native config with mode `0644`,
  marks a revision active immediately after native syntax validation, and has no
  process/listener/egress/capture health commit or durable Last Known Good state.
- Service still imports and invokes the user-scope system proxy package directly
  (`proxy.enable/disable`) even though the target boundary assigns this to
  Agent. It always uses the single configured `ProxyPort` without checking the
  selected core's actual listener.
- `runtimeState` has `CoreID`, `SelectedOutbound`, rule/global/direct mode and
  transient TUN fields, but no `SourceType`, `CaptureMode`, subscription ID, or
  typed upstream ID. Airport nodes and user-created HTTP/SOCKS entries coexist
  in one `subMgr.Outbounds()` collection and can be selected interchangeably.
- Subscription add/refresh and outbound selection often return success before a
  background config apply finishes. This creates false UI success and hides
  failures in logs.
- `applyRuntimeConfigLocked` validates native syntax, but if a running-core swap
  fails and restoring the previous config does not fail, it deliberately keeps
  the new candidate and returns success (“manual restart required”). It then
  persists the new selection, so desired state can diverge from the running
  process.
- Runtime config names and log output remain sing-box-centric for every core
  (`runtime.*.json`, `sing-box.log`). Mihomo YAML is written with a `.json`
  extension, and only Xray's extension behavior is explicitly considered.
- The current “transaction” has no post-start listener, controller, DNS, HTTP,
  egress IP, or system proxy/TUN commit probes. Rollback is ad hoc and no
  durable LKG revision is recorded.
- `handleOutboundCreate` defaults missing protocol to SOCKS and persists
  plaintext credentials into the same subscription store. It applies the new
  outbound immediately in global mode, so “save” and “activate” are not
  separate operations.
- Subscription persistence stores full subscription URLs and every outbound
  credential in plaintext `subscriptions.json` (`0600` requested, but no DPAPI
  and no Windows ACL verification). `subscription.list` also returns the full
  URL to UI.
- Fetcher has time/size/redirect controls and removes request URLs from nested
  transport errors, but it only permits HTTPS despite the execution document
  requiring HTTP+HTTPS. DNS-name SSRF rebinding is not blocked because private
  address checks are applied only when the original hostname is already an IP.
  ETag, Last-Modified, Content-Type/status metadata, and encoding handling are
  absent.
- Parser dispatch is hard-coded rather than a registry with confidence and
  metadata. Parser-level rejected items are logged but discarded from the
  Service response, so users cannot see accepted/rejected detail.
- Clash YAML parsing is a hand-written line scanner, not a YAML parser. It
  flattens nested structures and cannot safely preserve lists/maps/plugins or
  modern transport options. Unknown protocols are silently dropped.
- Normalization deduplicates solely by `server:port:type`, which can collapse
  distinct endpoints with different credentials, SNI, Reality keys, transport,
  or provider. `Merge` then replaces the entire collection with the normalized
  refresh result, mixing manual and subscription ownership semantics.
- `outboundsForProvider` includes every manual outbound (`ProviderID == ""`) for
  every failed provider. This reinforces that subscriptions and upstream
  proxies are not independent aggregates.
- All three cores reuse `host.SingBoxHost`; only argument-builder functions
  differ. The interface/comments, log messages, binary discovery/integrity
  validation, reload error, recovery behavior, and config port extraction
  remain sing-box-specific.
- The host extracts readiness only by string-searching JSON
  `"listen_port"`. Mihomo's native `mixed-port` is never found, so port `0`
  skips readiness entirely. Health then marks `PortOK=true` when no port was
  detected. This permits false “running/healthy” status for Mihomo.
- Xray emits separate HTTP and SOCKS ports, but host tracks only the first
  string-matched `port` equivalent—currently none, because it also looks only
  for `"listen_port"`. Its readiness and health are therefore skipped too.
- Health currently covers cached running state plus one TCP listener only. It
  has no controller, proxy handshake, DNS, HTTP egress, selected-exit, revision,
  capture-state, or multi-port verification.
- Core binaries are supplied to the in-process Service and directly launched by
  the generic host. There is no separate `navo-core-host` executable or narrow
  privileged IPC boundary.
- Supervisor state lacks resolving/compiling/validating/probing/applying-capture/
  rollback phases. Config swap stops the old core before starting the candidate
  and has no native transaction object or LKG commit.
- Agent correctly owns a system-proxy manager for direct UI proxy methods, but
  Service duplicates the same user-session registry operations. Both hard-code
  `127.0.0.1:12080` via config rather than consuming verified endpoints from the
  active core.
- System proxy backup is a temporary JSON file written as `0644`; `Enable`
  snapshots before setting, but ordinary `Disable` clears proxy settings instead
  of restoring the snapshot. Agent shutdown calls `Disable`, not `Restore`.
- No call site constructs or applies `internal/network.Manager`; it is currently
  test-only/dead orchestration code. Service TUN handlers merely recompile the
  selected core config, while the separate Wintun/route/DNS managers are used
  only during reconciliation.
- `setTUNRuntime` enables TUN for any selected core without a per-core/version
  capability check. The Wintun availability check is relative to the sing-box
  directory even for Mihomo/Xray.
- Windows TUN code manually creates a Wintun adapter while the core config also
  enables its own TUN inbound/auto-route, creating unclear ownership and risk of
  duplicate adapters/routes. DNS and route helpers use locale-sensitive
  `netsh/route` output parsing and contain incomplete IPv6 handling.
- Two independent recovery packages exist (`internal/network.Reconciler` and
  `internal/recovery.Reconciler`); only the former is wired to Service. Repair
  uses yet another path/schema, so recovery state is fragmented.
- `internal/ipc` defines typed envelopes/messages but production UI/Agent/
  Service code does not import it; all runtime dispatch uses untyped
  `map[string]interface{}`. The type definitions and method constants have
  drifted from actual methods (`core.select`, `outbound.select`, `testAll`,
  runtime, AI config, etc.).
- IPC currently allows `CoreStartRequest.ConfigPath` and flexible arbitrary
  config objects, directly violating the target request boundary. It has no
  typed `CoreType`, `SourceType`, `CaptureMode`, `ActiveSelection`, revision, or
  compatibility contracts.
- The framed Named Pipe transport itself is reusable: fixed magic, bounded
  10 MiB payload, deadlines, Windows ACL implementation, and concurrent server
  instances are already isolated from domain semantics.
- DPAPI machine-scope support exists and is successfully used for the AI API
  key, so credential protection can be reused. Subscription URLs, proxy
  credentials, node UUIDs/passwords, and runtime configs do not use it.
- `internal/storage.Store` explicitly says SQLite is future work. It is a
  generic JSON key-value file with no schema version, migration transaction,
  relationships, uniqueness constraints, or active-selection integrity.
- Wails UI is an untyped single-file Vue screen using `Record<string, unknown>`.
  It presents core, proxy, TUN, subscription, and outbound actions separately;
  there is no mutually exclusive SourceType selector, compatibility details, or
  atomic ApplySelection request.
- UI treats `core.state == running` as connected and displays success after many
  handlers that only queued background work. It does not require revision,
  verified ports, egress, or capture state.
- Service accepts arbitrary `config_path` from `core.start` and `core.restart`.
  Core list checks existence only; it reports no versions, hashes, capabilities,
  or compatibility.
- Core switch stops the previous core before compiling/validating the candidate.
  It has ad hoc rollback, no durable transaction/revision, and permits Xray TUN
  only via one hard-coded special case instead of an adapter capability result.
- TUN enable immediately responds `enabled` before the async config application;
  `tun.status` synthesizes `route_count=1` from a Boolean without inspecting the
  adapter or route table. These are explicit false-state paths.
- Published launcher allocates one free mixed port, creates a sing-box bootstrap
  config, starts Service+Agent in-process, and launches Wails separately. Agent
  to Service is a direct function call in packaged mode; only UI to Agent uses
  Named Pipe.
- `.cache/go-build` is now the canonical repository-local Go cache. Legacy
  `gocache*` directories are ignored and removed by `scripts/clean.ps1`; they
  are temporary compiler artifacts, not application modules. Their prior
  proliferation came from ad hoc per-phase cache paths.
- Package smoke checks startup, read-only IPC, direct HTTP data flow, core
  switching, stop/restart/shutdown, and residual processes. It uses only a
  direct outbound and therefore does not test airport parsing, authenticated
  upstreams, source exclusivity, protocol fidelity, native controller health,
  system proxy mutation, or TUN.
- Build/package scripts are Wails-based and no longer require Flutter. The
  execution document's mandatory Flutter commands are inapplicable to current
  state; equivalent frontend typecheck/build must be used unless the user
  explicitly reverses the Wails migration.
- Current pinned binaries exist:
  sing-box 1.13.14 SHA-256
  `db0d779948214cf761011d154c3a5da36df20394fa01a9fc798f1dc39fe9d183`;
  Mihomo v1.19.29
  `4316ff91fecec2fca9acb5612d7400ba228c069ffd325b1f17f46f1d4ef7e0cd`;
  Xray 26.3.27
  `15c2d007954ac53ba69b80ec91242786b3c0b71d52649165b4ca1d5cc96ef8f1`.
- `CORE_MANIFEST.json`, `THIRD_PARTY_NOTICES.md`, root `LICENSES/`,
  `configs/golden/`, and `docs/audit/` are absent. Mihomo and Xray include local
  licenses; sing-box currently has only executable/version/hash/Wintun files.
- Test volume is substantial for legacy units, but there are no tests for Wails
  UI, system proxy, securestore ACL/scope, standalone Agent/Service entries, or
  real Mihomo/Xray host integration.
- Missing mandatory coverage includes typed ActiveSelection/source exclusivity,
  versioned compatibility reasons, three native compilers' golden output,
  native validation for all cores, six source/core combinations, authenticated
  local HTTP CONNECT/SOCKS fixtures, durable database migration/revision/LKG,
  redaction, and Windows 11 system-proxy/TUN/recovery E2E.
- Current baseline verification passes: `scripts/test.ps1` completed
  `go test ./...` and `go vet ./...`; packages without tests are standalone
  Agent, standalone Service, systemproxy, securestore, and Wails host.
- Current Wails frontend verification passes:
  `npm.cmd run build` completed `vue-tsc --noEmit` and Vite production build.
  Flutter analysis/tests were not run because Flutter was intentionally removed
  from the current repository.
- User has now made the target explicit: Wails v2 + Vue 3 is permanent and the
  retired Flutter/Dart/CMake/Ninja build chain must not return. The architecture
  source document was corrected accordingly.
- Persistence target is the user's existing cloud MySQL; Codex must not install
  MySQL. Only Service infrastructure may connect, with TLS, protected
  credentials, bounded pooling, schema migrations, and explicit offline
  behavior. Current local JSON data must be migrated without deletion.
## 2026-07-28 implementation authorization

- The user authorized implementation after the architecture audit.
- Wails v2 + Vue 3 remains the permanent desktop stack; Flutter, Dart, CMake
  and Ninja must not return.
- Persistence must target the user's existing cloud MySQL. Navo only owns the
  client integration, schema migrations and failure handling; it must not
  install or administer MySQL.
- A real local `.env` must be left in the workspace, ignored by Git, with a
  committed-safe `.env.example`. Empty credentials are preferable to invented
  secrets.
- Implementation follows the audited phase order so domain invariants exist
  before persistence, adapters, IPC and UI are rewired.
- The repository had no `.env`, `.env.example`, environment loader, SQL driver
  or database configuration package before this implementation phase.
- Phase 8 now has isolated packages for core/source/capture identity,
  ActiveSelection invariants, endpoint protocol specs, upstream proxies and
  structured compatibility results. Legacy compiler types remain untouched
  until adapters are introduced.
- The official Go MySQL driver v1.10.0 supports the required `database/sql`
  pooling, context cancellation, DSN timeouts and explicit TLS configuration.
  It requires Go 1.24+, while Navo declares Go 1.26.4.
- MySQL is opt-in by default. `NAVO_MYSQL_REQUIRED=true` implies enabled mode;
  missing cloud credentials then fail configuration immediately. Disabled or
  optional offline mode must preserve local LastKnownGood operation.
# 2026-07-28 TUN 切换回退与 CORE_004

- `App.vue` 的 TUN 切换顺序是先执行 `proxy.disable`，再执行 `tun.enable`，随后立即刷新 Dashboard。
- Service 当前异步处理 `tun.enable`，IPC 在内核换配置完成前就返回成功，因此 UI 会读取到尚未生效的状态。
- 实际日志显示候选配置启动后 10 秒内未监听 `12080`，`SwapConfig` 随后成功恢复旧配置；但系统代理已被前端提前关闭，所以 UI 最终显示“不接管”。
- 这不是动画本身的问题，而是跨两次 IPC 的非原子状态切换，加上 TUN handler 过早返回共同造成的竞态。
- 下一步需从 sing-box 运行日志确认候选进程未监听 `12080` 的原始错误，并将 TUN handler 改为同步、将接管模式切换收敛为单次事务。

## Resolution

- 修复后的真实启动测试捕获到原始错误：`start inbound/tun[tun-in]: configure tun interface: Access is denied.`；`12080` 超时只是 Host 未及时回收已退出进程造成的错误包装。
- Host 现在从进程启动时立即执行 `Wait()`，readiness 同时监听进程退出，并使用 TCP connect 探测，不再使用 Windows 上不可靠的二次 bind 判断。
- 新增 `capture.set`：Agent 串行协调 TUN 与系统代理，失败时不会先关闭系统代理；Service 只在候选内核健康并提交后返回 TUN 成功。
- 非管理员进程在重启内核前返回 `TUN_REQUIRES_ADMIN`，UI 保留原接管状态并显示“以管理员身份运行”的明确操作。
- System proxy 的关闭/恢复增加所有权校验；没有 Navo owner record 时不会关闭 v2rayN 等其他软件的代理。
- 管理员 TUN 自动化因 Windows UAC 无法在当前自动化会话中完成确认；脚本超时后确认没有 Navo、TUN 或内核残留。非管理员事务、失败回滚和三内核数据面均已实测。

# 2026-07-28 Full Tray implementation

- Current `tray_windows.go` owns the correct native lifecycle but only has two
  command IDs: Show and Exit. It has no backend interface, dynamic state model,
  nested menus, endpoint/core/capture/route actions, diagnostics or refresh.
- Existing Service/Agent methods already cover most operations under current
  names: `core.*`, `outbound.*`, `runtime.*`, `metrics.current`, `ip.check`,
  `capture.set`, `proxy.status` and `tun.status`.
- The combined launcher can preserve the required boundary by exposing Agent's
  dispatch as a public application method and passing only that method to Tray;
  Tray must never receive Service, Supervisor, CoreHost, system-proxy Manager,
  configuration paths or database handles.
- Menu state should be rebuilt on each right-click from an Agent-composed
  snapshot. This avoids polling and prevents Tray from treating a prior click
  as current state.
- `ip.check` has a 15-second network timeout and must not run while opening the
  tray. Runtime snapshot needs cached/cheap fields; explicit “test exit IP”
  remains a user-triggered diagnostic action.
- `outbound.list` already carries source/provider identity and active ID but
  currently lacks explicit availability and compatibility reason fields.
## Installer validation environment

- MSI 编译成功且 hash sidecar 一致。
- 本机受管会话无权写入 `C:\Windows\Installer\inprogressinstallinfo.ipi`，所以 Windows Installer 的真实安装/管理提取与 ICE 无法在该会话完成；这不是 WiX schema 或 payload 编译错误。

## 2026-07-29 Phase 22 TUN lifecycle

- Audit confirmed capture truth was split across Agent, Service, Supervisor,
  WinINet and sing-box side effects; `tun.status` could report intent rather
  than the actual adapter.
- Capture now has one authoritative state machine and one cross-layer
  transaction. Concurrent requests return `CAPTURE_BUSY` instead of queuing a
  stale target.
- The Wintun adapter uses a deterministic GUID compatible with sing-tun,
  create-or-open semantics, actual Windows adapter status and no per-mode
  driver deletion.
- Service does not install owned routes/DNS until the core and adapter are
  ready. Failure or external adapter disable stops the core, removes only
  journal-owned network state and commits safe `off`.
- Agent persists transition steps and performs startup rollback before
  accepting UI requests.
- UI exposes concrete phases, locks duplicate mode actions, announces progress,
  and presents one fault dialog per fault ID with keyboard recovery.
- Full Go tests/vet, Vue typecheck/build and Edge interaction smoke pass.
- Latest setup EXE contains an icon resource identical to the source ICO and
  has SHA-256 `1ee2b0ba3109e574da278712fc4c6ef668c8dc9e4a63289ee38453b2e23b8454`.
- Elevated real TUN data-plane acceptance remains pending. The attempted smoke
  elevated only Navo while leaving its IPC client non-elevated; the script now
  elevates the whole test process but was not rerun to avoid more user-visible
  windows.
- A repeatedly respawning sing-box was proven by the Windows `Creating Process
  ID` counter to belong to `v2rayN.exe`, not Navo. It must not be terminated by
  Navo cleanup or acceptance automation.

## 2026-07-29 Phase 23 startup flash diagnosis

- `internal/network/tun/adapter_state_windows.go` runs PowerShell
  `Get-NetAdapter` from the three-second TUN adapter monitor without either
  `HideWindow` or `CREATE_NO_WINDOW`. This is the direct cause of continuous
  console-window flashes while Navo is open.
- Startup and capture paths also contain core validation, `sing-box check`,
  PowerShell, `netsh`, `route` and `taskkill` child processes with inconsistent
  window flags. `HideWindow` alone is weaker than `CREATE_NO_WINDOW`.
- The latest launcher log proves `navo_app.exe`, WebView2 and dashboard IPC all
  started successfully. The UI process later exited normally with status 0;
  there is no frontend build or WebView initialization failure in that run.
- Service currently starts the persisted core before Agent startup recovery
  immediately switches capture to `off`, causing avoidable core churn during
  every application launch.
- The Wails `Dashboard` DTO omitted Agent's `capture` object and the expanded
  TUN fields. Go JSON decoding silently discarded them, while Vue immediately
  dereferenced `dashboard.capture.state`; this schema mismatch caused the real
  blank/failed interface after WebView startup.
- An old disabled `Navo` adapter can delay startup recovery for 15 seconds
  before the Agent UI pipe begins listening. Adapter recovery must actively
  open/remove the stale adapter or accept a safe disabled adapter without
  blocking the desktop bridge.

## 2026-07-30 Phase 23 resolution

- The adapter monitor now calls native `GetAdaptersAddresses`; it no longer
  creates a PowerShell process every three seconds.
- Every remaining runtime child command uses one shared Windows policy with
  both `HideWindow` and `CREATE_NO_WINDOW`. The OS-level regression test
  confirms child processes have no console window.
- Agent exposes the UI pipe before startup capture recovery. A stale disabled
  adapter is handled in a bounded 1.5-second cleanup window instead of blocking
  the desktop bridge for 15 seconds.
- The Wails DTO now carries `capture` and complete TUN state. Vue defensively
  normalizes nested dashboard fields before render, removing the blank-window
  failure caused by schema mismatch.
- Combined desktop startup defers the core until a capture mode is activated,
  eliminating the persisted-core start/stop churn during launch.
- The final packaged stability smoke held one visible UI process for two
  seconds, found no duplicate/replacement process, returned dashboard data in
  17.1 ms and exited cleanly with launcher code 0.
- The refreshed setup EXE contains an icon resource whose SHA-256 exactly
  matches `winres/#1_0000.ico`.

## 2026-07-30 Phase 24 design and acceptance direction

- The recovered task scope is host/runtime status, core update inspection,
  proxy latency and throughput testing, plus a pixel-technology UI refinement.
- The UI design query recommends a dense dark operations dashboard with
  high-contrast green status accents, restrained motion, monospaced numeric
  readouts, explicit loading feedback and visible focus states.
- Network tests must show current numeric values and textual status, disable
  duplicate actions while running, expose bounded timeout/error recovery and
  respect reduced-motion. Charts cannot be the only representation of results.
- Core update behavior must not silently replace executable payloads. The safe
  boundary is version/status inspection plus an explicit user-controlled update
  action with provenance and checksum verification before installation.
- The implementation should use semantic Vue controls and dynamically
  synchronized ARIA state; no structural emoji icons or hover-only actions.
- Cloudflare's official `cloudflare/speedtest` repository identifies
  `https://speed.cloudflare.com/__down` and `/__up` as its download and upload
  APIs. Navo uses small bounded samples through its loopback proxy instead of
  embedding a browser speed-test dependency.
- Official release checks use fixed GitHub API/release URLs for SagerNet
  sing-box, MetaCubeX Mihomo and XTLS Xray-core. Numeric version comparison
  never treats an older stable release as an upgrade, which matters because the
  packaged Mihomo version may be ahead of the latest stable GitHub tag.
- Automatic binary replacement is deliberately excluded: replacing a signed
  installed payload also requires service elevation, asset-specific checksum
  provenance, core stop/swap/restart rollback and installer ownership. This
  phase exposes integrity-aware upgrade availability and an explicit official
  release action without weakening that boundary.

## 2026-07-30 Phase 24 user-corrected UI contract

- The cyan single-night operations dashboard is rejected and must not be used
  as the visual baseline.
- Day form: sharp/no-radius industrial pixel geometry, white/black/orange as
  the dominant palette, hard borders and offset shadows.
- Night form: rounded geometry, purple/black/blue as the dominant palette,
  smoother layered depth and restrained glow.
- Remove the global refresh action. A feature that performs work must show one
  unified scrolling progress track, numerical progress and a task-specific
  label while preventing duplicate submission.
- Diagnostic logs are settings content, not a top-level navigation
  destination. Settings also owns the persistent day/night selector.

## 2026-07-30 Phase 24 theme geometry refinement

- Day is the canonical layout template. Theme switching may change palette,
  border color, shadow character and corner treatment, but not DOM structure,
  grid tracks, spacing, component size or border width.
- Irregularity is expressed through shared asymmetric shape tokens:
  day uses small angular-looking corner values, while night uses larger
  asymmetric round values. Since border radius does not affect box geometry,
  both forms remain spatially identical.
- Browser acceptance compares `x`, `y`, `width` and `height` for the shell,
  sidebar, header, page content, hero, overview grid and monitor grid before
  accepting the switch. All measured deltas are zero at 1180x760.

## 2026-07-30 Phase 24 paired background and shadow refinement

- The remaining visual split came from two independent depth systems: nearly
  white day surroundings with opaque black offset shadows versus flat dark
  night surfaces with blur-only purple shadows.
- Both forms now share layered shell/panel gradients and a two-part shadow:
  a low-opacity directional offset plus a soft ambient falloff. Theme tokens
  change hue and intensity without changing elevation or geometry.
- Day uses warm gray/beige foundations with muted orange-brown depth and a
  restrained blue-gray contrast surface. Night uses purple-black/blue
  foundations with a very low-opacity warm purple-red countertone.

## 2026-07-30 Phase 24 selected-state color correction

- A selected container must remain in its theme's dominant hue family; status
  semantics must not replace the container palette. Day selections use
  beige/orange-brown, night selections use purple/blue, and green remains a
  compact health indicator only.
- Sharing layout does not require sharing the same elevation treatment. Day
  retains sharp directional offset shadows; night retains rounded blur-only
  purple depth. The commonality is hierarchy and placement, not visual texture.
- Theme screenshots must be taken after the short color transition settles;
  capturing the intermediate frame can falsely show day surfaces with night
  typography and controls.
- The proxy-IP card is informational and must not own a `featured` appearance.
  Selection is an interaction state, not a content type. Its reusable feedback
  is the original surface plus a 2-9% same-hue overlay, muted border and narrow
  marker; strong solid fills are reserved for primary actions.

## 2026-07-30 Phase 24 regional motion and card feedback

- Treat the shell as three visual regions: top status bar, left menu and main
  content. Status/menu selection owns directional edge motion; the main content
  retains its established layout and only changes feedback/elevation details.
- The selected-edge path is upper-left -> left edge -> lower-left -> bottom
  edge -> lower-right. Its inherited radius preserves sharp day geometry and
  rounded night geometry.
- Motion is theme-specific: day uses bright beige foundations with orange
  active motion; night uses dark purple foundations with blue active motion.
  Card elevation is black for day and blue for night.
- Card color response is transient UI feedback, not a domain selection or
  feature mutation. A delegated pointer handler applies a 360ms class and
  leaves proxy/core/network handlers unchanged.

## 2026-07-30 Phase 24 real latency and speed availability

- The missing-function symptom was not a missing binding. `TestRoute` and
  `RunProxyBenchmark` were wired, but full speed was disabled whenever the core
  was stopped. Real logs show capture startup rolled back after an end-to-end
  proxy failure, so the button could never be used.
- The selected MiYaIp upstream is currently unhealthy: its SOCKS route reports
  DNS timeouts and connection resets. This is external route state and must be
  shown to the user instead of hiding diagnostic controls.
- Endpoint latency is independent of the local core and can always use
  `outbound.test`. Throughput requires the local proxy listener, so diagnostics
  may temporarily start the core without enabling WinINet or TUN and must stop
  it afterward if diagnostics started it.
- A disabled diagnostic action hides the recovery path. Keep it clickable and
  return the real core-start, DNS, connection or timeout error.

## 2026-07-30 source-level diagnostics and window adaptation

- The black outer-window mismatch came from two native constants in
  `navo_app/main.go`: a dark RGB background and `windows.Dark`. CSS theme tokens
  cannot affect the Windows title bar or transparent native surface.
- Wails v2 exposes `WindowSetLightTheme`, `WindowSetDarkTheme`, and
  `WindowSetBackgroundColour`; synchronizing these in `setTheme` keeps the native
  window aligned with the unchanged Vue layout.
- A route speed test is stateful because throughput runs through the selected
  loopback proxy. Testing a non-active route is safe only while capture is off:
  snapshot the active route/core state, switch, test, stop any diagnostic-only
  core, and restore the prior route. During system-proxy/TUN capture, reject a
  non-active route speed test to avoid redirecting live traffic.
- Source filtering is now a connection invariant rather than visual filtering
  alone: the bottom confirmation and connect action require the selected route's
  `source_type` to match the current source filter.

## 2026-07-30 Phase 27 recovery matrix

- Recovery is a resource transaction, not a core restart. Its owned resources
  are core process/ports, WinINet snapshot, TUN adapter, IPv4/IPv6 routes, DNS,
  firewall rules and the durable journal.
- Every undo must be idempotent, tolerate already-missing resources and verify
  the resulting state. Old journal actions must be normalized to current safe
  undo commands before execution.
- Multi-core recovery must use adapter capabilities. Unsupported TUN must fail
  before any system mutation; supported cores must use their own generated
  configuration and readiness/data-plane probe.
- Cleanup may remove only Navo-owned state. Existing v2rayN processes, foreign
  WinINet settings, adapters and firewall rules must be preserved.
- The latest elevated smoke correctly rejected a false system-proxy success:
  sing-box listened on 12080 but reset the real HTTP proxy request. The next
  implementation pass must retain this data-plane gate while making failure
  recovery complete.
- The proxy reset was caused by Supervisor binding the long-running core to the
  short `capture.prepare` request context. Separating startup cancellation from
  the lifecycle context made sing-box system proxy and TUN both pass real data
  plane checks.
- Windows route removal can still return exit 1 with `-ErrorAction
  SilentlyContinue`; every journal undo now explicitly exits 0 after an
  idempotent removal attempt.
- A core-created Wintun adapter remains disabled after process exit. Recovery
  must open that existing adapter only after core stop, then destroy the handle.
- Direct/proxy IP detectors previously shared Go's global HTTP transport. A
  concurrent background IP check crashed inside HTTP/2 while routes changed;
  detectors now own separate HTTP/1.1-only transports.
# 2026-08-01 Four-Guide Remediation Findings

- The repository already has a long-running remediation plan through Phase 27; this request is being added as Phases 28-33 rather than replacing prior evidence.
- The four new guides are untracked user files under `docs/design/`; they are inputs, not cleanup targets.
- Declared ordering from the cleanup/privacy guide: build reachability inventory; delete AI; delete MySQL-specific code while preserving local business models; add initialization/device privacy cleanup; execute Full Remediation P0/P1; execute feature optimization; perform a second dead-code cleanup.
- The self-healing guide is bounded automation: structured detection plus live-state confirmation, known-error policies only, transaction-coordinator-only mutation, mandatory post-repair verification, visible failure, budgets/backoff/circuit breaking, correlation/deduplication, and fault-injection testing.
- Historical Navo acceptance evidence is not sufficient for this request. Current binaries, tests, Windows elevation, network data plane, and rollback/recovery must be revalidated.
- Current cleanup targets are concrete: `internal/ai/`, `internal/service/ai_settings.go`, AI dispatch cases in `internal/agent/agent.go`, MySQL config/store packages, and MySQL bootstrap in `cmd/navo/main.go`.
- The repository already contains local storage, selection/revision domain models, DPAPI helpers, recovery/reconciliation, monitoring, proxy benchmarks, core-update inspection, IP-risk UI, and traffic chart code. These must be audited and integrated rather than discarded.
- The feature Guide explicitly protects currently incomplete-but-planned core update, four-channel traffic, dual-path monitoring, IP risk, log filtering, and icon work from dead-code deletion.
- Cleanup Guide hard constraints: do not replace MySQL with SQLite without necessity analysis; preserve revision/selection semantics via atomic local persistence; foreign-context detection must use a random install secret protected by DPAPI Current User; foreign contexts must never execute copied network journals; privacy cleanup failure must stop normal startup.
- The required first deliverable is `docs/CODE_REACHABILITY_AND_REMOVAL_PLAN.md`, including production/dynamic entry points, Wails and string-dispatched IPC, goroutines, repositories, build tags, tests, and a removal-evidence classification.
- Full Remediation P0/P1 invariants include: pre-resolve endpoint egress before Navo routes; one runtime transition lock/coordinator for capture/core/node mutations; rollback uses an independent context and only restores committed state after real data-plane validation; dirty/faulted state survives incomplete rollback; Host never self-restarts; single-process production disables the external Service pipe; public IPC accepts no arbitrary `config_path`; WinINet backup/ownership is complete and two-phase; verified core paths cannot be rediscovered; Job Object failures are fatal; Named Mutex replaces lock-file mutual exclusion.
- Additional P1 gates include strict multi-core capability/compiler parity, native config validation, protocol/rule validation, HTTPS-only SSRF-safe subscriptions, real YAML parsing, canonical credential-aware node fingerprints, parser-normalizer-validator flow, corrupt-state quarantine, transaction-first destructive updates, one atomic file-replacement utility with Windows ACL/DPAPI, complete Named Pipe writes and bounded requests, and contract/data-plane tests.
- Release cannot be considered valid unless package generation is gated by Go tests/vet plus frontend typecheck/tests/build, followed by smoke; startup with deferred core must begin with capture off and no system proxy/TUN.
- Feature Optimization gap summary: current core page only inspects versions/releases and explicitly avoids installation; the Guide requires cancellable verified download plus staging/atomic replacement/health-check/rollback for all three cores. Current route tests/benchmarks do not yet expose the required layered TCP, protocol handshake, DNS/TLS/TTFB/total/exit-IP result without changing capture state. Current traffic model/UI has only generic upload/download lines and a 30-point frontend ring; the Guide requires backend timestamped local up/down plus proxy up/down samples, real counter sources, preferences, fixed colors, unavailable reasons, reset handling, and synthetic/controlled transfer modes. Current log tail is plain/simple and lacks structured service/level/date query, pagination, persisted clear, and redaction guarantees. Current risk UI derives a local summary from IP metadata; it lacks provider evidence aggregation, conflicts, opt-out, explicit stale/error states, and multi-label types.
- Feature UI acceptance also requires renaming pages (`升级内核`, `一键测速`), explicit async union states rather than global booleans, keyboard/touch-safe hidden scrollbars, full chart tooltip interaction, centralized branding resources, multi-size ICO/state icons, and Windows DPI/theme/taskbar/start-menu verification.
- Self-healing is an observer/policy layer, not a second mutation owner. It consumes structured WARN/ERROR/FATAL events, re-snapshots real runtime state, matches only registered stable error codes, acquires fixed-order resource locks and retry budget, delegates all network/core/capture mutation to the runtime coordinator or Supervisor, performs a real verification, and rolls back/backs off/circuit-breaks visibly.
- Safety boundaries: no text-derived commands; no direct route/netsh/registry/raw core operations; no killing unknown processes; no automatic credential/protocol/TLS changes; no third-party route overwrite; no bypass of DPAPI/hash checks; privacy cleanup failures remain startup-blocking; unknown errors only aggregate and surface as diagnostic/faulted state.
- The Guide supplies stable `NAVO_<DOMAIN>_<ERROR>` codes across core/runtime/TUN/route/DNS/system proxy/outbound/subscription/monitor/log/init domains, centralized definitions with repairability/admin/transition/retry/budget metadata, standardized repair actions, per-resource deduplication, bounded queues, correlation/transition IDs, jittered backoff, circuit breaker half-open validation, and explicit security-error zero-retry defaults.
- Self-healing state is a small versioned, atomic, ACL-protected local file containing only error code, hashed resource identity, attempts, timestamps, circuit deadline, and last result; foreign-context initialization clears it. Startup order places SelfHeal after initialization/logging/repositories/coordinator/Supervisor and before Agent/UI; shutdown stops new repair work before capture shutdown.
## 2026-08-01 Phase 30 entry audit

- Single-process launcher still creates an unnecessary Service pipe because `Service.Run` always listens; standalone `navo-svc` must opt in explicitly.
- `Service.New` discards the launcher-verified core path when `host.FindBinary` succeeds, weakening the verified-binary boundary.
- Service raw lifecycle handlers accept caller-provided `config_path`; Agent and Wails still expose raw `core.start/stop/restart` operations.
- Launcher treats Job Object initialization failure as a warning and uses a PID lock file as its primary single-instance primitive; both violate the remediation guide.
- Launcher also performs unconditional image-name `taskkill` against standalone Navo binaries; this can terminate an unrelated development/test instance and must be removed instead of called zombie recovery.
- `SetCoreRunning` is used only to temporarily start the core for route benchmarks. Replacing it with general `connection.enable` would mutate system-proxy state, so the correct replacement is a bounded benchmark session operation owned by the coordinator, not a raw lifecycle alias.
- Feature guide explicitly requires isolated benchmark ports and no Capture mutation. Until that Phase 31 implementation exists, disconnected benchmarks now fail explicitly instead of using the production core through a raw UI lifecycle command.
- Host still implemented autonomous crash restart while Supervisor also owned restart policy. Supervisor monitor additionally used the startup request context, so it could stop observing a healthy long-lived core after IPC completion.
- Endpoint bypass commands execute before split routes, but consume the unbounded `Find-NetRoute` result directly through `$r.InterfaceIndex/$r.NextHop`; multiple adapters therefore produce arrays and invalid `New-NetRoute` arguments.
- PowerShell command execution does not establish UTF-8 output and returns raw local-code-page bytes in errors, making Chinese Windows diagnostics unreliable.

## 2026-08-01 Phase 30 recovery audit

- `network.Reconciler` treated every state-read error as a clean first run, advertised `READY` despite cleanup failures, and contained a no-op stale-file cleanup. Recovery must fail closed and preserve `DIRTY_SHUTDOWN` until all required cleanup succeeds.
- Capture rollback must retain the previous committed mode and live manager reference when rollback is incomplete; reporting `off` would hide residual WinINet/TUN state.
- Dashboard endpoint rendering leaked an unrelated, unowned WinINet proxy address when Navo did not own system proxy state. The public snapshot must fall back to Navo's configured local endpoint unless ownership is active.
- Phase 30 storage/IPC acceptance is explicit: Windows replacement must use write-through semantics without deleting the old file; pipe header and payload require full writes; timed-out overlapped operations must be cancelled and reaped before releasing their event/memory; the 10MB frame boundary needs direct coverage.
- Current IPC violates that contract in three concrete places: `WriteFrame` trusts short writes; timed-out `Read`/`Write` frees the event after a bounded wait without a final `GetOverlappedResult`; listener shutdown closes a pending `ConnectNamedPipe` event/handle without `CancelIoEx` and completion reaping.
- Current `fsatomic.WriteFile` flushes temporary contents but delegates replacement to generic `os.Rename`; it lacks the required Windows `MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH` path and a replacement-failure test proving old data survives.
- IPC handlers are sequential per connection (effective max concurrency 1) and switch-dispatched (method whitelist), but they reuse the pre-read deadline for the response, ignore `SetDeadline`/`Send` errors, and expose `service.shutdown` through ordinary UI forwarding. Long dispatch can therefore make a valid response silently miss its stale deadline, while any UI client can stop the privileged service.
- Feature Optimization is partially present from earlier work: Wails/Vue already has per-route latency, cancellable proxy benchmark, a 30-point traffic ring, risk summaries, dual-theme/icon work, and dashboard views. The new guide still requires contract-level comparison: safe core upgrade transaction, four independently selectable traffic series, explicit simulation modes, structured/filterable logs, and multi-source IP risk semantics.
- `fsatomic` now supplies the required write-through replacement but Windows `0600/0700` still does not establish a DACL. `x/sys/windows` already exposes SDDL conversion and `SetNamedSecurityInfo`, so the unified writer can protect the destination directory and temporary candidate before replacement; this preserves the old file if ACL setup fails.
- Existing core update UI is inspection-only: it verifies the installed binary against `CORE_MANIFEST.json`, queries GitHub semantic versions, and opens the official release page. The manifest contains only installed artifacts, not trusted hashes/assets for future releases; a safe automatic installer cannot accept arbitrary latest-release assets without extending the trusted update manifest. The current behavior must not be relabeled as installation.
- Traffic UI currently derives only two series from core proxy counters and labels them generically as upload/download. Windows `x/sys` exposes `MibIfRow2`/`GetIfEntry2Ex`, enabling independent host-interface cumulative counters; adding those beside core counters supports the required four-series model without double-counting one source as another.
- `TrafficChart.vue` contains visible mojibake in labels and ARIA text; this is a source defect, not terminal rendering, because neighboring UTF-8 Vue content reads correctly. It must be rewritten while extending the chart contract.
- Follow-up encoding scan found no mojibake byte patterns in Vue/TypeScript sources; the earlier mixed rendering was a PowerShell output-codepage artifact. The rewritten chart text remains valid UTF-8 and the compiler accepts it.
- Logs remain the largest Phase 31 contract gap: Service reads up to the entire text file before tailing 200 lines, has no level/service/time query, no cursor, and no safe clear operation. Text-prefix parsing would violate the guide; a structured append store with explicit producer fields is required and can also become the event substrate for Phase 32 self-healing.
- SelfHeal integration must preserve the existing lifecycle owner: Supervisor already performs bounded crash recovery, so `NAVO_CORE_CRASHED` is an observer policy. Marking it auto-repairable in a second engine would recreate the exact dual-restart race prohibited by the guide.
- Observe-only must run before budget acquisition. Persisting half-open/attempt state for a dry observation would mutate production recovery history despite the documented no-action contract.
- The current release manifest authenticates only the bundled binaries; it does not pin future release asset names, URLs and SHA-256 values. Automatic core installation must remain blocked until a Navo-trusted update manifest exists. Opening the official release page is safer than treating GitHub metadata/download success as installation authority.
- `repair.exe` was a separate fake recovery owner: it marked a dirty journal NORMAL without adapter/route/DNS/WinINet verification. Retaining its mutating commands would invalidate fail-closed recovery even if the main runtime is correct.
# 2026-08-02 TUN one-pass closure

- Existing Phase 27 work reports ownership-aware, idempotent recovery, but its final elevated three-core matrix was not executed.
- Historical acceptance rules require real proxied HTTP/TUN egress plus adapter, route, DNS, rollback, and recovery evidence; process/listener/UI state alone is insufficient.
- Previous failures included non-admin `Access is denied`, cross-integrity Named Pipe ACL mismatch, and an elevated smoke timeout before a new launcher log.
- The supplied guide requires six implementation phases: transaction safety; deterministic activation planning; Journal V2; idempotent resource operations; control/data-plane verification; elevated Windows acceptance for sing-box and Mihomo.
- Non-negotiable ordering is core start -> adapter readiness -> Navo-owned network apply -> control-plane verification -> direct (no explicit proxy) DNS/TCP/HTTPS/exit-IP/UDP verification -> health commit.
- Xray must continue to reject TUN explicitly; sing-box and Mihomo share the same Windows network transaction owner, with `auto_route` and `strict_route` remaining disabled.
- The current worktree already contains user-owned planning changes and the supplied untracked guide; preserve both while editing only TUN scope.
- Initial source scan confirms the documented gaps still exist: `network.Manager` exposes only name-based `WaitForAdapter`; endpoint physical routes are inferred inside `manager.go` with `Find-NetRoute`; `prepareTUNLocked` assigns `s.networkManager` after adapter wait/network activation; and TUN currently commits healthy runtime directly from `startCoreForCapture`.
- The required Activation Plan, structured AdapterSnapshot, Windows verification module, Service TUN data-plane verifier, elevated integration test, one-pass acceptance script, and acceptance report do not yet exist.
- Existing `internal/network/tun` adapter inspection can be extended, but current status modeling is too shallow for GUID/index/address/MTU uniqueness acceptance.
- `internal/network/journal.go` is strictly V1 and persists `Undo Command`; recovery currently reconstructs only by action name, but V2 resource identity and exact per-resource ownership are absent.
- `Manager.Activate` uses the caller `ctx` for rollback on intent/apply/journal failures. This reproduces the guide's canceled-context rollback failure.
- Current operations create split routes by `InterfaceAlias`, infer endpoint routes during apply, and remove routes by broad prefix/alias or metric filters. They cannot distinguish pre-existing exact resources from Navo-created resources.
- The legacy Wintun manager configures address/MTU/DNS with `netsh` and reports cached configured addresses/MTU rather than observed values; adapter readiness therefore must be based on observed Windows state.
- `internal/network/tun/route_windows.go` violates the new recovery boundary by parsing localized `route print` text and deleting any route line containing `Navo`; it must be removed from crash recovery ownership or replaced with exact journal resources.
- Existing tests assert command counts and reconstructed V1 undo behavior but do not cover exact ownership, conflict preservation, or canceled-context rollback.
- Exact-resource Manager tests now prove: V2 contains no executable undo authority; pre-existing exact resources are not mutated or deleted; all apply failures roll back; cancellation does not poison rollback; failed undo remains journaled; unknown/malicious and unprovable legacy endpoint actions never execute.
- Service endpoint pinning changes only the selected runtime copy's `Server`; SNI, Reality identity, WebSocket/HTTP Host, path, and gRPC service name stay unchanged, and the persisted outbound is not mutated.
- The old Reconciler no longer invokes broad route or DNS cleanup. Exact V2 Manager recovery runs before core compilation/start; without a journal, Reconciler does not guess resource ownership.
- Non-admin `Get-NetAdapter` is denied in this managed process, while the same generated PowerShell passes parser checks. Live adapter/route facts remain an elevated acceptance gate.

## 2026-08-03 User-reported TUN unusable follow-up

- The user reports TUN is still unusable despite the Phase 34 implementation and existing elevated artifacts. Acceptance must reproduce the exact packaged/UI path and prove ordinary no-proxy traffic traverses the active TUN, not rely on prior script success alone.
- Existing `artifacts/tun-acceptance` contains elevated golden, failure-injection, crash-recovery, and lifecycle runs from 2026-08-03. Their current results, package identity, and profile inputs must be correlated before deciding whether the remaining failure is packaging, persisted configuration, UI dispatch, core selection, or data plane.
- The live residue audit found no Navo adapter, split routes, NRPT rules, or Navo firewall rules; only an unrelated `D:\v2rayN-windows-64` sing-box process is active. The expected current acceptance package `release\Navo` is absent, so prior artifacts do not prove the binary the user is launching contains the Phase 34 code.
- The actual current package is `release\Navo-phase34`. Latest isolated-clone golden results pass for both sing-box and Mihomo with `HEALTH_COMMITTED`, exit IP `165.254.151.219`, and exact rollback. One repeated TUN activation failed with a forcibly closed local proxy connection, while later TUN/proxy/TUN, TUN/off/TUN, adapter-disable, and core-crash lifecycle cases passed. This points to either an intermittent core restart/readiness race or the user launching an older installed build, not a universal route/DNS failure.
- Installed `C:\Program Files\Navo\navo.exe` is the 2026-07-29 build (`605F...9C12`, 8,811,008 bytes); current `release\Navo-phase34\navo.exe` is the 2026-08-03 build (`5352...DE94`, 8,935,424 bytes). The installed Wails UI is likewise stale.
- Current-package logs show successful TUN activation normally takes 35-40 seconds, within the new 120-second Wails `capture.set` timeout. During the adapter-disable lifecycle case, the selected upstream credential lookup consumed the transition deadline; cleanup then could not reopen the Wintun adapter and health recovery timed out. Credential resolution must not share an already-near-expired transition context.
- The `fresh-direct` golden case intentionally has `DirectMode=true`, so equal direct/TUN exit IP is valid only for disconnected/direct configuration. Isolated proxy-backed golden cases change exit IP and match local-proxy exit. User-facing acceptance must use a real selected outbound.
- `rollbackCaptureLocked` spends one 20-second context on route rollback, core stop, adapter destruction, adapter wait, and finally `compileCaptureConfig(false)`. That final compile resolves every upstream credential and validates a new config even though the core is already stopped; after earlier cleanup consumes the budget it fails with `context deadline exceeded`, leaves runtime TUN state stale, and complicates health recovery.
- A safe rollback can persist `TUNEnabled=false` without compiling credentials or starting a core. The next requested system-proxy/TUN transition already compiles a fresh config before start. This preserves fail-closed behavior and removes credential dependency from emergency cleanup.
- The same-mode restart failure is eliminated at the Agent transaction boundary: a healthy committed request for the current mode now returns idempotently, while faulted or mismatched snapshots still execute recovery/retry.
- Navo 1.0.1 is now installed per-machine. Installed launcher/UI hashes exactly match `release\Navo`, and installed `repair.exe check` reports zero issues.
- The current user Navo profile contains no persisted outbound/subscription state. The live local v2rayN proxy at `127.0.0.1:10808` passes both HTTP and SOCKS5 Microsoft connect-test probes, so it can provide a credential-free real proxy-backed acceptance path without reading or inventing user secrets.
- Installed proxy-backed acceptance proved the plan builder rejected explicit IPv4 loopback upstreams before core/network mutation. Loopback upstreams need no endpoint bypass, but TUN still needs a frozen physical egress for public split-route verification; the corrected plan models exactly that distinction.
## 2026-08-08 TUN restart ownership audit

- `SelfHeal` is observer-only for `NAVO_CORE_CRASHED`; it cannot trigger this rollback or restart loop.
- `Supervisor` restarts only a host whose state is explicitly `failed`, and capture coordination can suppress that restart. It is not the first actor in the delayed false-fault sequence.
- The remaining duplicate owner is Agent: after three `tun.status` failures it still calls `capture.prepare off`, clears the journal, and publishes its own retryable TUN fault. Service must be the sole TUN rollback/fault authority; Agent should only mirror Service state.
- Service receives Supervisor crash events only for SelfHeal diagnostics, so removing Agent TUN recovery does not remove core crash ownership.
- `monitorTUNAdapter` currently rolls back on the first 3-second sample that is anything other than `enabled`; it does not re-observe after acquiring `captureMu` and does not distinguish `starting`, `unknown`, inspection errors, disabled, or missing.
- `handleTUNStatus` combines `tun.InspectAdapter` with a second `network.InspectAdapterSnapshot` call, so state and GUID/index/address fields can represent different instants. Health must be based on one observation.
- Native `GetAdaptersAddresses` already provides state, GUID, and interface index in one call without PowerShell. The Service status path should make that native observation authoritative and use the activation snapshot only as committed metadata after validating matching GUID/index.

## 2026-08-08 profile ACL audit

- `fsatomic.restrictPath` builds a protected DACL containing only SYSTEM, Administrators, and the current process token SID. Under over-the-shoulder UAC, that SID can be the administrator credential rather than the owner of the `%LOCALAPPDATA%` profile path.
- Because `WriteFile` applies this DACL to the entire destination directory, one elevated write can permanently remove the interactive profile owner from `%LOCALAPPDATA%\Navo`, explaining the current access-denied evidence.
- The repair must preserve the path/profile owner SID as well as the process SID, SYSTEM, and Administrators; granting Users or Everyone would weaken credential/log privacy and is rejected.
- `golang.org/x/sys/windows` already exposes `GetNamedSecurityInfo(... OWNER_SECURITY_INFORMATION)` and `SecurityDescriptor.Owner()`, so the owner can be obtained without localized command output or shelling out to `icacls`.
- The managed Codex shell runs as `YunHe\CodexSandboxOnline` (SID ending `-1009`), while `%LOCALAPPDATA%` is owned by the actual desktop user `YunHe\28484` (SID ending `-1001`). Therefore the sandbox continuing to receive access denied after migration is expected and is not evidence that the desktop owner remains locked out; elevated ACL export must verify the exact `-1001` ACE.
- Elevated export after launching 1.0.5 confirms the exact `-1001` owner ACE and no broad user grants. It also exposed one orphaned Navo IPv6 firewall rule while capture was off; no split routes or NRPT remained. The disconnected Wintun adapter is named `本地连接`, not Navo, and cannot be claimed because it may belong to v2rayN.
- Manager currently recovers only resources recorded in its V2 journal. A rule under the reserved `Navo TUN IPv6 Block <session>` namespace can safely be classified as Navo-owned even after its journal was lost; startup should remove exact outbound/block rules in that namespace while refusing conflicting shapes.

## 2026-08-09 TUN external-site usability follow-up

- The user confirms Windows now recognizes the TUN as a proxy, but actual external access still fails; Google and GitHub are the explicit acceptance targets.
- Adapter creation remains control-plane evidence only. This follow-up must correlate the currently running binary, selected core, generated config, routes, NRPT/DNS and sanitized core/runtime logs with proxy-disabled DNS/TCP/HTTPS requests.
- Historical Cloudflare/ipify success is not evidence for the current installed process or for Google/GitHub reachability. Current live-state checks and rollback/re-enable evidence are required.
- Current read-only inventory shows Navo 1.0.6 is installed and no Navo launcher/UI/Service/Agent/core process is running. The only live `sing-box.exe` belongs to `D:\v2rayN-windows-64` and must not be touched.
- Current-user WinINet proxy is disabled. The managed shell is denied CIM adapter/route/DNS reads, so live TUN state and data-plane reproduction require the existing elevated acceptance path; empty non-elevated results must not be interpreted as clean network state.
- The current production verifier and elevated script probe only Cloudflare for DNS/TCP/HTTPS and use ipify/icanhazip for public IP. They do not prove Google or GitHub usability.
- Historical acceptance is intermittent even against Cloudflare: two golden attempts failed with HTTPS EOF or no exit-IP change before a later run passed. This makes the current user's failure credible as a data-plane stability/routing issue rather than a UI-only issue.
- Strong code-level root-cause hypothesis: sing-box TUN defaults all intercepted DNS to direct UDP `223.5.5.5`, while Mihomo uses the host DHCP resolver directly. Cloudflare can resolve correctly through these resolvers, but Google/GitHub can receive polluted or unsuitable answers before their TCP traffic is sent through the selected proxy. The current health check therefore systematically misses DNS correctness for the requested sites.
- The latest passing historical run used an isolated clone of the user's real profile, not fresh direct mode, and validated only Cloudflare/ipify. This proves the selected proxy could carry one target at that time, not that current domain resolution is safe.
- Official sing-box 1.13 documentation confirms modern `https` DNS servers support `server`, `server_port`, `/dns-query`, TLS, and dial fields including `detour`. This allows DoH over the selected outbound without using the poisoned system resolver when the DoH endpoint is an IP.
- Official Mihomo documentation supports `https://8.8.8.8/dns-query#proxy`-style DNS routing through a named proxy/group, and warns that node-domain bootstrap requires `proxy-server-nameserver`. Navo can use `#NAVO` for target DNS while keeping DHCP only for bootstrap/direct resolution.
- The selected outbound endpoint is already resolved and pinned before TUN core startup. Therefore proxied DoH can safely use an IP-literal endpoint without creating a bootstrap loop, while target-domain DNS is protected from local resolver pollution.
- Current release golden acceptance passes for both supported TUN cores: sing-box 1.13.14 and Mihomo 1.19.29 each reached `HEALTH_COMMITTED`, returned Google 204 and GitHub 200 with DNS/TCP/HTTPS true, matched TUN exit IP to local-proxy exit IP, and passed exact rollback.
- A separate sing-box `tun-off-tun` run remained healthy after 30 seconds, then disabled and re-enabled TUN; delayed and re-enabled Google/GitHub probes both passed. Final routes, NRPT rules, Navo firewall rules and Navo adapters were zero, with direct IP restored.
- Installed Navo 1.0.8 is the final tested payload. Its 24 release-manifest files match exactly and installed `repair.exe check` reports zero issues.
- Final installed lifecycle acceptance passed for both cores. sing-box and Mihomo each remained `HEALTH_COMMITTED` initially, after 30 seconds, and after disable/re-enable; Service and independent probes returned Google 204 and GitHub 200, while TUN and explicit-proxy exit IPs both resolved to `165.254.151.219` instead of the direct `211.90.237.75`.
- Both final rollbacks restored direct exit IP `211.90.237.75` and left zero Navo split routes, NRPT rules, firewall rules, adapters, and package-owned processes. WinINet remained disabled.
- The first final Mihomo wrapper failure was a verifier artifact: its shared PowerShell `HttpClient` retained pooled sockets across route teardown/recreation even though all three Service-owned product checks passed. Creating an isolated no-proxy transport per retry removed stale-connection coupling; the exact rerun passed.
- The final Mihomo re-enable independent Google probe needed three attempts while the Service-owned commit probe already passed Google and GitHub. This is retained in the JSON evidence rather than hidden; bounded retries apply only to read-only probes, never network mutation.

## 2026-08-09 Phase 36 dashboard and diagnostics adjustment

- The existing backend already emits independent physical-interface and core-proxy counters. The UI defeats the requested behavior by always exposing four series instead of selecting local counters for `off` and proxy counters for `system_proxy`/`tun`.
- `TrafficChart.vue` maps pointer X to a sample and renders a detached tooltip below the SVG. It has no horizontal/vertical crosshair, point markers, or pointer-relative overlay.
- `App.vue` mixes subscription creation/removal, upstream creation, route selection, batch latency, active-node throughput and layered latency inside the One-click Test page. Connection Management has only a local line-type card, so source type does not govern the whole page.
- Overview already has direct IP, proxy IP and a basic proxy risk card, but it relies on a manual `IPDetection` object and does not expose per-link partial failures cleanly.
- Service `ip.check` always starts both direct and local-proxy detectors. When capture is off/core is stopped, the proxy half is expected to fail, yet the UI treats that as overall health failure. Detector cache keys are static (`source`, `current`) for five minutes and are not invalidated on capture/route changes.
- Core update inspection fetches only a GitHub tag and release URL. `InstallSupported` is hardcoded false; release assets, checksums, staging, process coordination, atomic replacement, rollback and post-install validation do not exist.
- Historical work already proved four-source traffic, tooltip interaction and update trust blocking. This phase extends those owners instead of introducing parallel monitoring or core lifecycle paths.

## 2026-08-10 Phase 37 final routing acceptance

- Capture scope and routing policy are independent: System Proxy/TUN controls interception, while `bypass_mainland`, `global`, `blacklist`, and `whitelist` control ordered outbound selection.
- Runtime policy changes are transactional. When the core is active, each change recompiles, restarts, probes Google and GitHub through `127.0.0.1:12080`, and restores the previous policy if either probe fails.
- TUN policy recompilation must reuse the active Activation Plan. Re-reading the persisted outbound restored its hostname and caused proxied DNS to resolve its own upstream recursively; retaining the pinned endpoint IP removed the loop.
- A policy request can coincide with Supervisor crash recovery. Waiting for a stable `running` state before restart suppression prevents an active capture from accepting a disk-only, unverified policy update.
- Final packaged sing-box artifact `20260810T114227486Z-...routing-modes.json` passed 8/8 policy/capture combinations. Final Mihomo artifact `20260810T114815016Z-...routing-modes.json` also passed 8/8.
- Every combination returned Google 204 and GitHub 200. Both cores reported TUN and explicit-proxy exit `165.254.151.219`, restored direct exit `211.90.237.75`, and left zero split routes, NRPT rules, Navo firewall rules, Navo adapters, or package-owned processes.
- Final `release/Navo` passes `repair.exe check`; all 24 SHA-256 manifest entries match. The frontend dependency audit still reports one pre-existing high-severity advisory and was not auto-fixed because that would expand dependency scope.

## 2026-08-10 Installed 1.0.10 TUN restart regression

- The later installed 1.0.10 run invalidates packaged-only acceptance: initial `bypass_mainland` TUN failed with `TUN_DNS_VERIFY_FAILED`.
- Proxy-routed DoH completed after about 18.6 seconds, but Service had only one 5-second system-resolver deadline. Cold-start host traffic saturated the selected upstream before readiness was established.
- The fix retries only the read-only resolver readiness probe, bounded to three fresh 8-second attempts with 500 ms spacing. It preserves fail-closed rollback and never retries a network mutation.
- Regression tests cover timeout recovery and bounded exhaustion; targeted Service tests pass.
- The local `127.0.0.1:10808` acceptance topology is recursively captured (Navo -> v2rayN -> Navo) and is not a valid final TUN upstream. Real-profile acceptance uses the configured remote SOCKS endpoint, which receives an exact endpoint bypass route.
- Mihomo reliably exposed a second root cause on disable/re-enable: API and DNS were ready, but Wintun reattachment delayed the mixed listener beyond the shared 10-second host readiness deadline. The Supervisor then killed a live core and surfaced `CORE_004`.
- Core listener readiness is now bounded at 30 seconds. Installed 1.0.12 Mihomo twice reached `HEALTH_COMMITTED` after re-enable with Service-owned Google 204/GitHub 200 and matching TUN/proxy exits. Installed 1.0.12 sing-box completed the full independent lifecycle acceptance.

## 2026-08-11 Phase 38 capture and policy correction

- The current code already stores capture scope and routing mode separately, but the UI labels TUN as `全局代理`, which conflates interception with routing and explains the user's mental-model conflict.
- The four routing modes are implemented, but blacklist/whitelist entries are fixed package variables in `internal/service/runtime.go`; customers cannot own or edit the rules.
- Correct product axes: capture scope is `off/system_proxy/tun`; routing policy is `bypass_mainland/global/blacklist/whitelist`. Every policy must compile for both supported capture modes.
- System Proxy covers browsers and applications that honor Windows proxy settings. TUN covers browser and arbitrary application IP traffic, subject to protocol/core support.
- Phase 37 packaged evidence covered all 8 capture/policy combinations, but the user now reports System Proxy unusable; current runtime state and real WinINet plus local-proxy data plane must be re-verified before relying on historical evidence.
- UI correction must preserve existing tokens and typography, use a denser operations-console hierarchy, restore the name `TUN 代理`, and give blacklist/whitelist a visible user-editing surface.
- System Proxy enable currently writes only `ProxyEnable` and `ProxyServer`. It leaves an existing `AutoConfigURL` and `AutoDetect` active, so Windows applications may continue resolving an older PAC/WPAD policy while Navo reports ownership; Navo must suppress both only during its ownership window and restore the exact snapshot afterward.
- `CaptureProbeFn` is not wired by production startup. System Proxy still has a pre-mutation local HTTP proxy probe, but the cross-layer transaction performs no post-WinINet probe. Add a post-mutation ownership/data-plane check so a successful registry write is not treated as a usable application proxy.
- User-editable routing rules should remain core-neutral. Persist normalized domain suffix and CIDR entries in runtime state, expose typed get/set DTOs, and compile them into ordered domain/IP rules for both System Proxy and TUN.
- The Phase 37 desktop screenshot confirms the proportion defect: three equal-height columns force the sparse route selector and capture selector to match the much denser four-policy card, leaving large dead areas. The narrow view serializes the same oversized cards and pushes policy/rules far below the fold.
- Replace the equal three-column composition with one compact route row plus one full-width traffic-control panel. Inside that panel, show the 2 capture choices as one axis and the 4 policies as a separate axis; reveal the active blacklist/whitelist editor directly below the policy instead of creating another top-level card.
- `runtime.status` already flows unchanged through Agent dashboard composition, so blacklist/whitelist data can be added once at the Service boundary and decoded by the existing Wails `RuntimeStatus` DTO without a parallel state owner.
- Treat `off` as connection state, not a third capture method. The UI should present only System Proxy and TUN as the two method choices, with a separate stop/disconnect action.
- Final browser composition uses a full-width compact route row followed by one full-width traffic-control panel. This removes the desktop dead column while retaining 2 capture choices, 4 policies, and an inline rule editor; 1280x900 and 760x900 both have zero horizontal overflow.
- Runtime rule mutation uses the existing capture mutex, active TUN endpoint pin, Supervisor restart suppression, Google/GitHub verification, and rollback path. UI/Agent IPC timeouts were extended to the same two-minute hard-verification budget as capture changes.
- The earlier routing matrix only proved explicit requests through `127.0.0.1:12080`; it did not prove that Windows applications consumed WinINet or that TUN captured applications bypassing system proxy APIs. Phase 38 acceptance now performs both independent application paths for every routing policy.
- Live TUN policy reload has an additional ownership boundary: `SwapConfig` can recreate the same named Wintun adapter with a new interface index/GUID. Explicit proxy traffic still works in that state, but ordinary application traffic follows stale split routes and blackholes. Every live TUN reload must recover the previous journal, bind owned routes/NRPT/firewall state to the observed adapter, verify the control plane, and then run TUN-native DNS/TCP/HTTPS/exit-IP checks before commit.
- System Proxy real evidence now covers WinINet consumers rather than only explicit proxy clients. Customer lists were accepted and verified, all four modes passed Google/GitHub, whitelist-listed public-IP domains proved direct routing, all other tested modes proved proxy routing, and the pre-existing v2rayN WinINet/listener state survived unchanged.
# 2026-08-11 Phase 39 icon unification

- The user requested `release/Navo-1.0.12-portable-x64/app_ui/navo_app.exe` and all in-product N brand marks to use the `tray_icon.ico` design.
- Scope rule: replace only executable/window/tray/UI brand marks, not ordinary text containing the letter N.
- The release packaging chain must be updated at its source so future builds do not restore the old icon.
- Visual audit: `tray_icon.ico` is a blue angular ribbon mark on transparency; the current portable exe embeds the black Wails `W` icon from `build/appicon.png`.
- The Wails `build/windows/icon.ico` diagnostic extraction failed because its ICO directory cannot be decoded by the .NET icon reader at the requested size; use the known PNG source and final executable extraction instead.
- Canonical icon inventory found only one in-product N-shaped brand component: `StateGlyph.vue`, rendered in sidebar brand, sidebar service state, and overview hero. All three now use the tray asset while retaining state semantics in the surrounding border/background.
- The canonical ICO contains 16, 32, 48, and 256px frames. The 256px frame was converted to the required 1024px Wails `appicon.png`; visual inspection confirms transparent background and the same blue ribbon geometry.
- Final browser-visible QA rendered three canonical tray glyphs, zero legacy N SVGs, and no page errors. The rebuilt executable resource extraction shows the same blue mark instead of the old Wails W.
- The target portable executable is SHA-256 `fae6ee9c1bfe0626a45c36d77b2c1eadcb7e224c6dd7693d8edc611057b0b422`; its manifest line was updated and independent verification reports zero mismatches.

# 2026-08-11 Phase 40 core-update download timeout

- The reported sing-box 1.13.18 asset URL is structurally valid and redirects from `github.com` to `release-assets.githubusercontent.com`, already covered by the updater's trusted GitHub host allowlist.
- `trustedCoreUpdateClient` currently uses the implicit Go default transport. Go environment proxy variables are absent in the observed process context, and Go does not inherit the Windows WinINet proxy, so the archive request connects directly and times out.
- The updater downloads and validates the complete archive before disabling capture or stopping the active core. A transport failure therefore does not replace the executable, manifest, or release checksums.
- Preserve exact asset selection, official HTTPS redirect restrictions, release-provided SHA-256 verification, bounded archive size, executable extraction/version validation, atomic replacement, and rollback.
- The transport repair derives validated loopback HTTP endpoints from the running Navo dashboard and current WinINet state, then tries environment proxy and direct fallback. It deduplicates endpoints and creates a fresh transport for each read-only attempt.
- The updater's own live Go path downloaded `sing-box-1.13.18-windows-amd64.zip` through `127.0.0.1:12080` (21,042,668 bytes) and passed official redirect, release digest, size, ZIP, and executable extraction validation without replacing the installed core.

# 2026-08-11 Phase 42 installed updater closure

- The repeated error still showed a direct GitHub TCP connection because the running installed UI was not the Phase 40 build.
- Installed Navo is MSI version 1.0.12. Its `app_ui/navo_app.exe` SHA-256 is `7194cad4d9ac58946bc3f784e958b901d0f5ab6742fc15ddee143ebe4a6587c1`; Phase 40 is `a097275e12e794370e2503f835359f88d2af72e84b0a98b1b78b8a2c1713d968`.
- Installed `CORE_MANIFEST.json` still records sing-box 1.13.14. The new installer must be built from the latest source so it includes both the download transport repair and the later physical-adapter ownership hardening.
- Navo 1.0.13 installed successfully. Installed and release UI hashes both equal `a097275e12e794370e2503f835359f88d2af72e84b0a98b1b78b8a2c1713d968`; installed repair reports zero issues.
- The old launcher/UI remained mapped after MSI replacement and was the exact stale-runtime risk seen in earlier releases. Only launcher PID 21576 and its child job were terminated through UAC; new launcher/UI PIDs are 5996 and 9220 with post-install start times.
- After restart, port 12080 is not listening until the customer connects a node. The fixed updater can use Navo's local proxy only when that listener is available; with WinINet also disabled, a disconnected update necessarily reaches the direct fallback.

# 2026-08-11 Phase 43 close action and brand alignment

- Wails previously terminated the UI child on title-bar close and had no UI-to-launcher exit intent; a safe full exit must reach the launcher's existing bounded shutdown coordinator.
- The top-left icon was measurably 7.05px low because `.brand span { display:block }` matched the `StateGlyph` root span and overrode its grid centering behavior.
- A remembered close choice is bounded by both the current Windows boot (derived from system uptime) and a 24-hour TTL, so it cannot silently cross a reboot or persist indefinitely.

# 2026-08-11 Phase 45 minimize-to-tray visibility

- Installed logs prove the currently exercised 1.0.13 UI exits directly on title close; installed UI hash `a097...` differs from portable 1.0.14 hash `fae6...` that contains Phase 43.
- The Launcher nevertheless logged `native tray ready`, so initial icon creation succeeded. The remaining product gap is that frontend minimize calls `WindowHide` directly and never asks the Launcher to refresh/re-register its tray icon or provide confirmation.
- Directly launching `app_ui/navo_app.exe` cannot provide a tray because the root `navo.exe` Launcher is the sole tray owner.
- Current session still has installed 1.0.13 launcher PID 5996 and UI PID 24076. Because Navo is single-instance, launching portable 1.0.15 while these remain will reopen the old UI instead of exercising the new binary.

# 2026-08-11 Phase 44 dual-link IP detection

- Dual-link detection is implemented as an explicit `CheckIP` action and renders direct/proxy results independently.
- Prior design requires mode-sensitive cache invalidation and partial availability; optional IP providers must not block dashboard or proxy operation.
- Root cause: `detectorTransport` set an empty `TLSNextProto` map and disabled `ForceAttemptHTTP2`; on the current network, providers negotiated/sent HTTP/2 and Go attempted to parse the binary frames as HTTP/1.x (`malformed HTTP response`).
- Dedicated direct/proxy transports already provide state isolation. Keeping HTTP/2 enabled and closing idle connections before `CheckFresh` preserves isolation without corrupting negotiated responses or reusing sockets from a previous route.
- A real post-fix probe returned distinct direct and proxy exits, proving the repaired detector path rather than only its unit tests.
# 2026-08-12 Phase 46: UI hierarchy and typography

- Root cause: the sidebar presents seven pages at identical visual weight, while Chinese headings/labels inherit monospace typography, forced uppercase, very small sizes, and low-contrast gray-purple text.
- Preserve all current product features and Phase 43/45 lifecycle behavior. Group navigation as core operations, monitoring/diagnostics, and system management without changing page IDs or actions.
- Use proportional Segoe UI Variable for Chinese interface copy; reserve Cascadia Code for measurements, identifiers, and logs. Remove global uppercase transformation and raise secondary-text contrast.
- Browser-visible result confirms the grouped sidebar remains readable in both themes and collapses cleanly to icons at 760px without horizontal overflow. The day/night theme switch and all seven original page targets remain intact.
# Phase 47 findings: ChatGPT and Codex usability

- Current Navo blacklist defaults contain only `openai.com` and `chatgpt.com`. This covers core API and `ws.chatgpt.com` by suffix, but omits official authentication, asset, upload, telemetry, support, Cloudflare challenge, WorkOS, Stripe, and SendGrid dependencies.
- Existing persisted user rules can predate future default additions because `RoutingRulesConfigured=true` preserves the stored list; simply extending the default constant would not repair existing installations.
- `bypass_mainland`, `global`, and `whitelist` already use the selected node as the final outbound. `blacklist` uses `direct` as final and therefore needs an invariant OpenAI service rule before the editable user blacklist to guarantee usability.
- TUN uses protocol sniffing plus DNS hijacking. In blacklist mode Navo already sends DNS through proxy-detoured DoH, so a complete domain-suffix route can keep OpenAI DNS and application flows on the selected node without hard-coded IPs.
- OpenAI's current official network guidance lists OpenAI/ChatGPT first-party domains plus authentication/static/upload and third-party dependencies. It explicitly requires WebSocket over TCP 443 to `ws.chatgpt.com` for ChatGPT and `chatgpt.com` for Codex, with proxies preserving the standard WebSocket upgrade and long-lived connection.
- Navo is a transport proxy and performs no TLS interception, so certificate-pinning exceptions are not required; the implementation must avoid adding MITM behavior.
- The canonical `compiler.Config` feeds sing-box, Mihomo, and Xray, and every generator preserves routing-rule order. A Service-layer invariant therefore covers System Proxy on all supported cores and TUN on sing-box/Mihomo without core-specific duplicated lists.
- Domain suffixes for shared third-party services must stay as narrow as OpenAI's published hostnames. Broad rules such as `cloudflare.com`, `apple.com`, or `stripe.com` would incorrectly proxy unrelated traffic.
- User correction supersedes the Phase 38 four-choice model: blacklist/whitelist are not route modes. The independent composition is private-network direct first, whitelist direct override second, blacklist selected-node override third, then the chosen base route (`direct`, `global`, or `bypass_mainland`). Overlapping lists are invalid because rule order must not silently decide contradictory user intent.
- Legacy migration can preserve behavior deterministically: old `blacklist` mode becomes base `direct`; old `whitelist` mode becomes base `global`; their stored lists remain unchanged.
- Browser-visible isolation passed at 1280x900 and 760x900: exactly two capture choices, three base-route choices, and two simultaneously visible list editors. Switching to `direct` preserved `system_proxy`; saving both lists preserved both capture and route state; no horizontal overflow or page/console errors occurred.
- Product-health boundary correction: ChatGPT/Codex must be release-acceptance probes, not mandatory activation gates. Otherwise a service WAF, account restriction, or node reputation issue would prevent all proxy use, contradicting the transport-only responsibility of System Proxy/TUN.
- Current v2rayN exit baseline reaches OpenAI API correctly (`401` without credentials) but receives Cloudflare `403` and a browser `请稍候…` challenge for ChatGPT. This proves transport to OpenAI but not interactive ChatGPT usability; the final Navo-node acceptance must report this separately rather than masking it.
- The final product activation gate remains application-neutral: only generic Google/GitHub transport checks can affect TUN health; ChatGPT/OpenAI probes stay in release acceptance so WAF, account, region, or node reputation cannot disable the proxy itself.
- `Navo-1.0.16-portable-x64` contains 25 files, passes package `repair.exe check` with zero issues, and its ZIP contains the same 25 leaf entries including `.env.example`. Real TUN/System Proxy application acceptance remains open because the corrected elevated run was canceled at UAC before execution.
- User's second correction supersedes the three-axis override interpretation: blacklist and whitelist are explicit, independent modes at the same product level as System Proxy and TUN. System Proxy/TUN activation must never implicitly enable list routing.
- Final state model uses three isolated axes: capture (`off/system_proxy/tun`), default route (`bypass_mainland/global/direct`), and explicit list mode (`off/blacklist/whitelist`). Stored list content is inert while list mode is `off`; only the selected list is compiled, so blacklist and whitelist never run together.
- Legacy persisted `mode=blacklist/whitelist` retains its prior behavior by migrating to the matching explicit list mode. States written by the superseded 1.0.16 interpretation have no `routing_list_mode` and therefore safely load as `off` instead of silently activating a list.
- Browser interaction passed at 1280px and 760px: TUN activation preserved list mode `off`; explicit blacklist activation preserved TUN; route change and list save preserved both. No horizontal overflow or page/console errors occurred.
- Theme text audit found day mode still used bright accent and hard-coded pale text across ordinary labels, while night selected controls could transiently report interpolated dark text during the 180ms theme transition. Theme-specific text overrides now produce stable dark day text and light night text after the intended transition; semantic status/chart colors remain unchanged.
# 2026-08-12 Phase 48: Full-page UI normalization

- Phase 46 corrected navigation and the app header, but page interiors still violate the same hierarchy: benchmark/IP metadata at 8-9px, core explanations at 10px, forced-uppercase table labels, and competing cards with identical borders/elevation.
- All seven pages need the same task-first sequence: page intro, primary action or live state, result/content sections, then secondary diagnostics. Keep existing DOM behavior and backend calls intact.
- Use a stable 12/14/16/20/24 type scale for interface copy; retain 10-11px only for technical timestamps/log metadata and Cascadia Code only for measurements, addresses, hashes, and logs.
- Visual inspection of the rendered connection and network-detection pages confirms the task intro, primary controls, result panels, and secondary metadata now scan in that order. The captured 100% loading bar is the intentional transient page-load state, not a persistent layout element.
- Additional visual inspection passed for core management, traffic monitoring, network speed testing, and settings/logs. Core cards now expose version/capability/status as one coherent block; traffic metrics use a 3x2 hierarchy; speed testing separates layered results from the route table; settings separates runtime, changelog, and logs.
- Final browser audit traversed all direct visible text nodes on every page: no visible page-interior text remains below 10px, no English all-uppercase eyebrow remains, and compact navigation retains accessible names.

# 2026-08-13 Phase 49: Cold-start capture and peer list interaction

- User reports the first System Proxy or TUN activation after Windows startup fails and is unusable. Acceptance must cover the cold-start path, not only an already-warm Service/core.
- Blacklist and whitelist must be peer entry points beside System Proxy and TUN. Their editors are closed and routing is disabled until clicked; clicking performs both actions atomically from the user's perspective: select the list mode, then open its editor.
- The existing persisted three-axis model remains valid internally. The requested change is the activation/reconciliation behavior plus top-level interaction hierarchy, not a return to coupled routing semantics.
- `ui-ux-pro-max` guidance reinforces explicit selected state, keyboard-accessible peer controls, synchronized ARIA state, visible async feedback, and progressive disclosure of list editors.
- The 1.0.18 portable log from 2026-08-13 proves startup recovery completed normally. The first System Proxy activation on Mihomo succeeded in 3.675 seconds; after `core.select` switched to sing-box, the core was accepted and persisted without a data-plane recheck.
- On the next launch, System Proxy started sing-box but its upstream hostname `link.miyaip.online` repeatedly failed system DNS resolution. The real HTTP proxy probe timed out after 20 seconds and correctly rolled back. This is a cold-start endpoint-DNS readiness failure plus an incomplete active-capture core-switch transaction, not a UI-only state error.
- The protected `runtime_state.json` confirmed `core_id=sing-box`, `selected_outbound=MiYaIp-...`, and `routing_list_mode=off`; `sing-box.log` showed the local mixed listener ready while upstream DNS exchanges timed out. Process/listener readiness therefore cannot replace endpoint resolution and data-plane verification.
- The repair pins a freshly resolved upstream IP before System Proxy core startup, with four bounded cold-start DNS attempts and TLS SNI preservation. TUN now uses the same retrying resolver before route planning.
- Core selection is promoted to an Agent-owned atomic stop/switch/re-enable transaction. If the new core fails capture verification, Navo restores the previous core and previously committed capture mode.
- Routing list activation is now session-scoped: list content persists, while every application launch forces list mode off until the user explicitly clicks Blacklist or Whitelist.
- Edge verification confirms the requested progressive disclosure: the connection page initially renders four peer entries with no editor; clicking one list activates only that mode, opens only its editor, and transfers keyboard focus; closing the list returns the mode to `off` and removes the editor.
- The 1.0.19 portable source/package gates and integrity checks pass, but system-level usability is not yet an accepted fact: the elevated isolated TUN/System Proxy run that mutates host networking and probes Google/GitHub requires separate explicit authorization in this environment.

# 2026-08-13 Phase 50: Proactive capture hardening review

- Confirmed a distributed-transaction gap in Agent core switching: after capture is stopped, a `core.select` transport failure is treated as if Service definitely stayed on the previous core. Named Pipe failure is outcome-ambiguous; Service may already have committed the new core before the response is lost. The current path then re-enables the old capture mode on the actually active new core and returns an error, violating rollback intent and leaving UI/runtime state divergent.
- The safe correction is to reconcile `core.status` after every failed core-select response. If the observed core differs from the previous core, explicitly select the previous core before restoring capture; if status or restore cannot be proven, leave capture off and surface rollback failure.
- Confirmed a second state-split edge case in the list entry: `runtime.list_mode.set` can commit successfully, then the follow-up dashboard read can fail. The current `execute` result is false in that case, so the editor stays closed and local `aria-pressed` stays stale even though Service has enabled the list. The UI must distinguish mutation failure from refresh failure and immediately reflect a committed mode before best-effort refresh.
- Confirmed the Service has no request replay protection while Agent retries the exact same `request_id` up to three times after a lost response. This can execute `capture.prepare`, runtime mutations, subscription changes, or outbound creation more than once. A bounded Service-side replay cache keyed by request ID plus request fingerprint is preferable to disabling retries: reconnects can safely recover the original response without repeating network mutations, and conflicting reuse is rejected.
- Added a zero-value-safe, process-bounded Service replay cache at the unified IPC dispatch boundary. The first request owns execution; concurrent or reconnecting duplicates with the same canonical JSON fingerprint wait for and receive the original response, while reuse of the ID for different content fails with `REQUEST_ID_REUSE`. Completed responses are capped at 256 entries so the fix does not create unbounded Service memory growth.
- The cache keeps empty request IDs on the legacy uncached path for compatibility, but all Agent-generated requests already carry unique IDs. This narrows the behavior change to requests that opt into request identity.
- The package audit exposed transitive `nanoid 3.3.16` through Vite/PostCSS. A root override to `3.3.17` removes the published zero-size custom-generator DoS advisory without widening other dependency versions; the resulting npm audit reports zero vulnerabilities.

# 2026-08-13 Phase 52: Full connection-chain stability audit

- Restored the persistent plan and the Phase 49-51 connection, transaction, package, and UI evidence before starting a new audit.
- The current worktree is intentionally large and dirty across connection, network ownership, IPC replay, Wails, UI, packaging, and planning files. All existing modifications are treated as user-owned and must be preserved.
- Prior green automated/package evidence is not current real data-plane proof. The newest `Navo-1.0.21-portable-x64` still requires direct verification of cold-start System Proxy/TUN behavior, delayed stability, recovery, rollback, and owned-residue cleanup.
- The audit follows the existing Navo acceptance boundary: prove real application traffic and exact rollback; do not infer usability from an open listener, running core, adapter presence, UI state, or package integrity.
- Current read-only ownership baseline: no Navo launcher/UI/core process is running, WinINet proxy is disabled with no PAC/WPAD value, and no Navo listener is present on the known local ports. The only matching live processes are `v2rayN.exe` and its `D:\v2rayN-windows-64\bin\sing_box\sing-box.exe`; both are unrelated and must remain untouched.
- The newest portable artifact is `release/Navo-1.0.21-portable-x64.zip`. It has automated integrity evidence from Phase 51, but no Phase 52 runtime traffic evidence yet.
- Re-read the Windows proxy/TUN acceptance and one-pass closure guides. Their required order remains deterministic plan/ownership, core and adapter readiness, exact network transaction, control-plane verification, no-explicit-proxy data-plane verification, delayed health commit, and exact rollback/recovery.
- The full-chain guide additionally requires native config validation against the exact started file/hash, local-port PID ownership plus real HTTP/SOCKS handshakes, explicit-proxy proof before capture, user-session WinINet verification, and ownership-aware restoration that refuses to overwrite an external proxy change.
- Crash behavior is part of connection correctness: a core crash must clear/restore WinINet within a bounded interval and the UI must leave Running; a User Agent restart must reconcile actual WinINet and listener state instead of trusting persisted capture state.
- TUN acceptance must bind one observed adapter identity (name, description, GUID, index, address, MTU), exact endpoint bypass routes, exact `/1` routes, NRPT, and IPv6 policy to one transaction. Network-change, sleep/resume, rapid switching, core/Service crash, and reboot recovery are distinct required stability cases.
- System Proxy and TUN both require an application-flow test, not only an explicit local-proxy request. TUN probes must disable environment/WinINet proxies; System Proxy probes must consume the current-user default proxy. Unsupported core/protocol/capture combinations must fail explicitly rather than time out or retry forever.
- Release blockers include a Running UI with failed data plane, stale WinINet after core exit, route/DNS residue after TUN stop, endpoint recursion into TUN, starting an unvalidated config, unowned port acceptance, credential logging, or silent capability fallback.
- The one-pass guide fixes Navo's TUN ownership model as an invariant: cores own packet processing; Navo alone freezes physical egress and owns exact endpoint route, split-route, NRPT, IPv6, verification, journal, rollback, and crash recovery. Xray TUN remains explicitly unsupported.
- The production state transition must be linear through preflight, baseline, compile, core, adapter, network, control-plane, data-plane, and health commit. `LastKnownGood`, active revision, and Running cannot be persisted until real no-proxy traffic and exit identity pass.
- Endpoint pinning must preserve TLS/Reality/HTTP/WebSocket/gRPC identity while making the core use the same selected IP as the frozen physical bypass. If a core/protocol cannot pin safely, capability rejection is required.
- Completed the one-pass guide. Recovery may delete only exact `CreatedByNavo=true` resources from Journal V2; missing resources are idempotent success, conflicts retain DIRTY evidence, and Reconciler must never infer ownership from localized route text or name substrings.
- Initial source map confirms the intended single owners exist: Agent serializes capture and owns current-user WinINet; Service owns core/TUN transactions and health; Network Manager owns exact resource journal/rollback; Supervisor owns core lifetime. Phase 52 will test that no later helper or background goroutine violates these boundaries.
- High-risk review targets identified from the source map are background `context.Background()` mutations, Service core-select rollback, active-runtime policy `SwapConfig`/TUN `Rebind`, startup recovery, Agent/Service duplicate health monitors, request replay eviction semantics, and shutdown ordering.
- Phase 52 baseline reproduced the historical SelfHeal state-file sharing violation inside `TestVerificationFailureRollsBackAndOpensCircuit`. Because the persisted circuit/rollback state participates in connection recovery, this is now in scope as a real concurrency/persistence defect, not an ignorable test-only event.
- All other Go packages in that baseline passed, including Agent, System Proxy, Compiler, Core Adapter, Network, Service, Supervisor, and Wails. The test script correctly stopped before `go vet`, so vet is not yet a Phase 52 pass.
- SelfHeal's test readiness condition is not synchronized with persistence completion: rollback increments its counter before `budgets.complete()` atomically writes the final open-circuit record. A concurrent test reader can observe the final path while the production writer is still completing replacement/ACL work on Windows, causing the reproduced sharing violation or a stale partial assertion boundary.
- The production `budgetStore` does serialize its own begin/complete operations, but it exposes no read/snapshot or completion signal. The repair should make persistence completion observable without weakening atomic writes or adding retry loops around a race.
- `fsatomic.WriteFile` closes and flushes the temp file before `MoveFileExW`, but the destination can become visible to another goroutine while Windows is still completing the replacement syscall. The test's `os.Stat` predicate is therefore not a valid completion barrier; Engine's pending-set removal is the deterministic barrier.
- A second production SelfHeal bug exists: when a circuit enters `half_open`, `begin()` persists `HalfOpen=true`; if Engine cancellation interrupts backoff, `process()` returns without clearing it, so later attempts remain `half_open_busy` indefinitely. A third lifecycle bug leaves queued events/pending dedupe state alive across `Stop()` and can execute stale repairs after restarting the same Engine.
- Red regression confirmed the stale-queue defect: after Stop/Start on the same Engine, a pre-stop queued repair executed (`calls=1`). The initial half-open fixture needed the production `begin -> complete` sequence before it could test the canceled half-open state; corrected the fixture without changing production code.
- Confirmed a more fundamental circuit bug: the normal allowed branch of `budgetStore.begin()` did not store the initialized record. `complete()` therefore saw a zero `WindowStart`, and every later `begin()` reset attempts/open state. Existing JSON could contain `open_until`, but enforcement was effectively bypassed.
- Fixed all three SelfHeal boundaries: persist the in-memory active window, clear a canceled half-open lease, and discard pending/queued/resource-lock state after Stop so no stale repair crosses a lifecycle restart. The file-read test now waits for Engine completion rather than destination visibility.
- Focused `internal/selfheal` validation passed 20 consecutive repetitions after the repair.
- Agent capture review found a likely late-mutation race: `prepareServiceCaptureRecovery(ctx)` enforces its 20-second timeout by abandoning a goroutine that calls the non-contextual `prepareServiceCapture(off)`. After the Agent releases `captureMu`, that orphan request can still reach Service and turn capture off after a new user retry has started.
- The same Agent path can skip its post-capture application probe entirely when `CaptureProbeFn` is nil. TUN still has Service-owned no-proxy verification, but System Proxy then proves only explicit local-proxy traffic plus registry ownership, not a current-user default-proxy application request; production wiring must be confirmed.
- Agent correctly avoids duplicate TUN recovery ownership: it mirrors Service TUN faults. System Proxy health remains Agent-owned and uses three observations before ownership-aware recovery.
- Production launcher uses the synchronous in-process `SendToServiceFn`, and no production code wires `CaptureProbeFn`. Therefore the recovery timeout wrapper cannot cancel the actual Service dispatch, and the System Proxy post-apply application probe is definitely absent rather than merely unconfirmed.
- Named Pipe mode has another reliability defect: its request deadline is set only after `Send`. A deadline from the prior request remains on the connection, so the first request after a long idle can fail before the new deadline is installed. Error paths also nil the channel without closing its handle.
- The production-safe direction is one context-aware Service dispatch API used by the launcher and capture recovery, plus per-attempt deadlines installed before send and explicit channel close on reconnect. Legacy test injection can remain as a compatibility fallback.
- Service capture sequencing is otherwise conservative: it owns one capture mutex, suppresses Supervisor restart during transitions, saves Manager ownership before TUN mutation, verifies no-proxy data plane before TUN health commit, and keeps failed Manager ownership when rollback is incomplete.
- A cross-layer System Proxy commit gap remains: Service commits the runtime revision before Agent applies/verifies current-user WinINet, because `capture.prepare(system_proxy)` has no later commit acknowledgement. Failure does trigger Agent -> Service off rollback, but the revision/LKG semantics need review before deciding whether this is a product bug or only a core-config health record.
- TUN core-crash recovery currently relies on Supervisor plus Service adapter monitoring. The monitor deliberately waits for stable, identity-mismatched/missing observations and then fails closed; it does not accept a restarted core merely because its process is running.
- Service IPC replay correctly deduplicates same-ID requests and rejects conflicting fingerprints, but an owner handler panic leaves an entry permanently in-flight because `ready` is never closed. Duplicate callers then time out forever and the poisoned entry is never eligible for completed-entry eviction.
- Runtime review found a confirmed serialization gap: `handleOutboundSelect` applies/restarts the core without acquiring `captureMu`, while capture transitions and routing/list changes do acquire it. Node selection can therefore interleave with TUN/System Proxy teardown, compile, core swap, or verification.
- Runtime rollback uses unbounded `context.Background()` in several core swap/rebind recovery paths. If rollback itself stalls, the IPC request and capture mutex can hang indefinitely. Some failure returns occur before restoring `s.cfg.ConfigPath`/`s.runtime`, leaving in-memory state divergent from whichever core config actually survived.
- Runtime config is marked active/LastKnownGood inside `applyRuntimeConfigLocked` before the higher-level active-routing verification runs. Verification failure later applies the previous mode/rules, but there is a window where an unverified candidate is persisted as active.
- Supervisor `SwapConfig` ignores a failed host stop and immediately starts the next core. That can leave the previous core alive while a replacement claims the same listener/TUN resources; callers currently cannot distinguish a clean swap from this split-brain attempt.
- The first transport inventory used a stale filename (`internal/agent/ipc_windows.go`); Agent Named Pipe transport actually lives in `internal/agent/agent.go`. The bounded re-scan completed without relying on the missing path.
- Agent transport has a valid `Channel.Close` API, so reconnect failures can close the exact stale pipe handle instead of only clearing the pointer. This is necessary to avoid handle leaks and expired-deadline reuse.
- The serialization gap is wider than node selection: upstream-proxy create/delete and subscription add/remove/refresh can restart the core without `captureMu`. Two delete paths launch unbounded background config applies and return success before activation is known, so capture transitions, shutdown, and source deletion can observe different runtime snapshots.
- `handleOutboundDelete` removes stored credentials before its asynchronous config apply. If activation fails, the currently running core may still reference credentials that the repository has already destroyed, while the UI has already received success.
- Service mutation handlers commonly replace the caller context with `context.Background()`. A canceled UI/Agent request can therefore continue waiting for the capture mutex, downloading subscriptions, compiling, restarting, or rolling back after the caller has moved on.
- The repair boundary must include one context-aware Service capture lock used by capture, core, node, routing, and source mutations; background refreshes must be bounded and acquire that same lock before changing active config.
- Service Named Pipe assigned only 30 seconds to `core.select` even though Agent allows four minutes for stop, compile, native validation, restart, and capture restoration. Standalone-pipe mode could therefore cancel a valid core switch long before the Agent deadline; Service method deadlines must match the transaction class.
- Node selection while TUN was active reloaded the core with the new hostname but retained the old TUN endpoint bypass plan. The new endpoint could therefore recurse into the TUN it was meant to feed. Agent must stop capture, select, then rebuild and verify TUN; Service independently rejects a live-TUN endpoint change.
- Runtime apply marked candidate revisions active/LastKnownGood and persisted the new selection before the caller performed its real routing verification. The active marker and generated-config cleanup are now deferred to `commitHealthyRuntime` after verification; the previous config is retained until that commit.
- Production System Proxy had no post-commit application-path probe. Added a WinINet `PRECONFIG` HTTPS probe so the same current-user settings consumed by normal Windows applications are tested before Agent reports Running.
- The default System Proxy ownership files used a shared temporary directory even though the launcher already owns a stable profile directory. Production now stores proxy backup/owner state and the capture journal under the selected Navo data profile, avoiding cross-install/temp-cleanup recovery loss.
- Legacy `tun.config` could reload a running System Proxy/TUN core without the capture transaction or a data-plane check merely to persist MTU/name preferences. It now serializes with capture, rejects real live-TUN changes, and persists inactive preferences without touching the running core.
- Subscription removal deleted its URL credential inside the repository before Service knew whether the replacement runtime config could activate. Added reversible detach/restore semantics; the credential is finalized only after config activation succeeds.
- A broad recursive call-site scan crossed protected historical `.cache/tun-acceptance` profiles and produced access errors. The useful source hits were recovered, and subsequent searches remain bounded to `internal` source trees.
- Final full `scripts/test.ps1` execution passed every Go package and `go vet ./...` after the last production and regression-test changes. Frontend tests passed 6/6; typecheck, production build, npm audit, and `git diff --check` also passed.
- The refreshed portable artifact is `release/Navo-1.0.22-portable-x64.zip`. Repair reports zero issues, all 24 manifest hashes match, package/ZIP contents match at 25 files, both executables are amd64 PE32+, and ZIP SHA-256 is `7B0AEC97C4D74FC71B0F73454886D9700087C0E1316CB2AE3E5E6DD082EB5ECD`.
- The attempted elevated data-plane acceptance did not cross the UAC boundary: Windows returned `The operation was canceled by the user` before product launch. The post-attempt baseline still has WinINet disabled, no Navo process or known listener, and no new acceptance artifacts. Adapter state returned `Access denied`, so no adapter-cleanliness claim is made from the non-elevated observation.
- Real System Proxy application flow and TUN no-proxy DNS/TCP/HTTPS, Google/GitHub, exit-IP equivalence, delayed stability, crash/recovery, and exact rollback remain the sole Phase 52 acceptance gap. Automated/package evidence cannot substitute for this elevated host test.

# 2026-08-13 Phase 53: UI request identity collision

- Confirmed root cause: Agent forwards the UI parent `request_id` directly to Service and reuses it for every Service subrequest in a composite operation. `dashboard.snapshot` alone sends five different methods with one ID; core/outbound/capture transactions similarly use one ID for status, mutation, verification, and rollback calls.
- Service replay behavior is correct: identical IDs are at-most-once keys, and a different fingerprint must return `REQUEST_ID_REUSE`. Weakening or clearing the cache would reintroduce duplicated mutations after a lost response.
- Added red regressions using the reported ID `ui-1786631794855-13`. They prove both direct `core.status`/`core.select` calls and the dashboard fan-out leak/reuse the UI ID at the Service boundary.
- Required fix boundary: Agent must mint a process-session-unique ID for each logical Agent-to-Service call, keep that same child ID across the internal Named Pipe reconnect retries, and rewrite only the returned correlation ID to the original UI parent ID.
- Implemented the boundary in `Agent.SendToServiceContext`: input maps are shallow-cloned, each logical Service call receives `agent-<random-session>-<sequence>`, all reconnect attempts reuse that prepared map, and a cloned response restores the UI parent ID without mutating Service replay state or caller-owned input.
- The reported direct `core.status`/`core.select` case and the five-call dashboard fan-out now pass 10/10 focused repetitions. Service conflict detection remains unchanged.
- Broader Agent/Service suites pass 3/3 and the full repository `scripts/test.ps1` gate passes all Go packages plus vet. Frontend tests pass 6/6; typecheck/build and npm audit (0 vulnerabilities) pass.
- Refreshed portable release `Navo-1.0.23-portable-x64`: repair reports 0 issues, 24/24 manifest hashes match, package/ZIP each contain 25 files under one root, launcher/UI are amd64 `0x8664`, and ZIP SHA-256 is `5B5DCEBFAD61F29BD03794C75D2CC8587E3F40E4722C923F0380B14067AE4DD4`.

# 2026-08-13 Phase 51: Layout refinement

- The current desktop connection page spends excessive vertical space on a second hero banner and three heavy raised panels before the primary controls. Thick borders, offset shadows, and the grid texture compete equally, weakening task hierarchy.
- The four peer controls are functionally correct but desktop cards are too narrow for their descriptions, while compact mode places them in a mechanically equal 2x2 grid with little selected-state emphasis beyond border color.
- Compact mode correctly collapses the sidebar to icons, but content begins close to the fixed rail and the top header/toast consume scarce vertical space. The first useful action appears below a large header/progress region.
- The left-number/right-content step layout works at desktop width but wastes horizontal space in compact mode. On narrow layouts the step number and label should become a compact eyebrow above a full-width control grid.
- Existing typography and warm day/night tokens should remain: the generated design-system suggestions for Fira fonts and OLED-only color are rejected because they conflict with the established Segoe UI Variable/Cascadia Code roles and dual-theme product direction. Useful guidance retained: 8px rhythm, consistent content width, visible focus, subtle 150-250ms state motion, and reduced-motion support.
- First post-change screenshots confirm a materially shorter path to the controls: the compact title becomes one information band, source type and node selection share the first desktop row, and traffic control is visually dominant. Compact routing choices now form one balanced three-column row instead of 2+1.
- The selected-state dot improves scanning without changing card geometry. However, desktop visual inspection found the success notification still overlaps the left edge of the theme switch by several pixels; the geometry gate checked only the page heading and must also assert separation from header controls.
- Final desktop night/day inspection confirms the adjusted toast clears both the page heading and theme switch. Source/node setup aligns on one row, the primary traffic panel begins substantially higher, peer cards retain equal weight, and the selected route is visible through border, inset marker, background, and a compact status dot.
- List-editor screenshots intentionally begin lower in the document because activation transfers focus to the textarea; this is required focus management rather than a layout jump. Sticky header/progress remain visible and the source-management/advanced sections continue in the normal scroll flow.
- Explicit night-mode inspection confirms the same hierarchy and spacing work against the purple dark surfaces: selected borders/dot remain distinct, secondary text stays readable, and the repositioned success notice clears both heading and theme switch.
# 2026-08-14 Phase 54: System Proxy readiness HTTP 502

- The exact failure is raised before WinINet mutation: `transitionCaptureMode -> EnableProxy -> proxyHTTPProbe` receives a real HTTP response with status 502 from the local proxy path.
- `probeHTTPProxy` sequentially tests `connectivitycheck.gstatic.com`, `cp.cloudflare.com`, and `www.msftconnecttest.com`, but returns only the last error. It discards a bounded response body and does not identify the failing endpoint, which hides whether the core rejected routing/DNS/upstream handshake.
- A 502 proves more than listener readiness but still fails the required data-plane gate. The repair must not reinterpret it as success; it must identify and correct the selected outbound/config/readiness path or provide actionable diagnostics if the upstream is genuinely unavailable.
- The 09:04:23 and 09:04:47 activations both routed all three readiness destinations through `outbound/socks[MiYaIp-1785509919105435700]`; each failed at the same direct dial boundary `119.147.134.162:8001` after 5 seconds. This is conclusive selected-upstream TCP unreachability, not local HTTP parsing, DNS, or probe-site behavior.
- The user profile contains one direct upstream and a subscription with 12 nodes. Live node selection/acceptance must run in the same elevated Windows identity because Navo's UI Named Pipe ACL correctly denied the lower-privilege test client.
- Production repair now performs selected-outbound TCP preflight before System Proxy tears down old capture or starts the core. It returns `OUTBOUND_UNREACHABLE`/`OUTBOUND_UNAVAILABLE` with the selected ID/address/reason. The later HTTP proxy gate remains mandatory and aggregates every endpoint-specific failure under `PROXY_DATAPLANE_UNAVAILABLE`.
- The package gate uncovered a Named Pipe startup race independent of the reported upstream failure: `WaitNamedPipe` returns immediately for `ERROR_FILE_NOT_FOUND`, so a fixed attempt loop could exhaust before listener creation. `DialPath` now uses a real 4-second deadline, waits for busy instances, and backs off 25 ms while no instance exists.
- The final elevated routing-mode acceptance command was canceled before the new PowerShell process started. It produced no acceptance artifact and is not evidence for or against the new portable's data plane.
# 2026-08-14 Phase 55: UIIPC request failures

- User-visible events at 09:04:41, 09:05:05, and 10:53:59 report only `Agent / UIIPC request failed`; privacy initialization completed separately at 09:14:09 and 10:53:22.
- The timestamps separate privacy initialization from the failing UI-to-Agent request path. Root cause still requires exact launcher/Agent/pipe correlation.
- Preserve the local-only Named Pipe architecture and existing replay protection; repair readiness, reconnect, or error propagation at the actual failure boundary.
- Exact request correlation: all four failures were `capture.set`; 09:04:41 and 09:05:05 were `CAPTURE_TRANSITION_FAILED`, 10:53:59 was `PROXY_DATAPLANE_UNAVAILABLE`, and 10:54:45 was `TUN_ADAPTER_NOT_READY`.
- The 10:53 System Proxy attempt started sing-box successfully, but all three real HTTP probes returned 502 through the selected node. Navo correctly left WinINet disabled and stopped the core; the selected upstream was not carrying traffic.
- The 10:54 TUN attempt started sing-box at 10:54:21 and sing-box reported `inbound/tun[tun-in]: started at Navo` at 10:54:22. Navo nevertheless timed out at 10:54:42 because readiness used repeated `Get-NetAdapter`/`Get-NetIPAddress` PowerShell processes and the final process consumed the deadline.
- Replaced adapter readiness inspection with `GetAdaptersAddresses` plus `GetIfEntry2Ex`, retaining exact name, Wintun description, hardware flag, GUID/LUID, operational status, MTU, and CIDR checks without process startup or localized output parsing.
- UIIPC error events now include the stable code in the visible message and a redacted `reason` field; future failures no longer collapse to bare `request failed`.
- Packaging exposed an unrelated stale `nanoid` 3.3.17 override. Updated only the override/lock to 3.3.18; `npm audit` now reports zero vulnerabilities.
- `release/Navo-1.0.25-portable-x64` passes repair with 0 issues and independent manifest verification for 24/24 entries; it contains 25 files.

# 2026-08-14 Phase 56: all-mode acceptance

- The elevated TUN golden request was rejected by the execution safety reviewer because explicit authorization for temporary adapter/route/DNS mutation was not present in the user-visible request. No test process was created and no network state changed.
- Added a `system-proxy-routing-modes` acceptance scenario that skips every TUN mutation, verifies current-user WinINet ownership, bypass_mainland/global/direct application traffic, explicit blacklist/whitelist/off persistence, and exact WinINet rollback.
- The first sandboxed System Proxy run never reached Navo IPC: DPAPI initialization failed under the sandbox identity with `CryptProtectData: The system cannot find the file specified`. WinINet remained byte-for-byte unchanged. This is an execution identity limitation, not a product System Proxy failure.
- A cloned profile solved DPAPI initialization. The next startup failure was test-only: Go defaulted to `386`, producing PE machine `0x014C`; `JOBOBJECT_EXTENDED_LIMIT_INFORMATION` then had the wrong ABI size. The portable launcher is correctly amd64, and the acceptance launcher was rebuilt with explicit `GOARCH=amd64`.
- `runtime.rules.set` returns `verified:false` by design while the core is stopped. The acceptance scenario now activates System Proxy before applying rules so a real running-core verification is mandatory.
- Windows PowerShell 5 requires explicitly loading `System.Net.Http`; application probes now do so. ChatGPT 403 from Cloudflare is accepted only for the unauthenticated homepage edge, while Google/GitHub and OpenAI API retain exact 204/200/401 expectations.
- Confirmed product root cause for unusable direct mode: `verifyRuntimeRouting` always selected `requiredExternalSites` (Google/GitHub), independent of the effective final route. A blocked Google direct path therefore rejected a correctly compiled all-direct config.
- The same semantic error existed in TUN twice: initial `buildTUNActivationPlan` and later `verifyActiveRuntimeRouting` treated any retained selected outbound as proxy mode even when `runtime.Mode == direct`, forcing endpoint/exit-IP proxy invariants onto direct traffic.
- The repair makes both System Proxy and TUN verification mode-aware. Proxy-bearing modes retain Google/GitHub gates; explicit direct mode uses independently reachable Baidu/Xiaomi HTTPS gates and direct TUN exit semantics.
- Mode-aware targets exposed a second direct-mode product failure: sing-box logged `outbound/direct[direct]` correctly, but `route.auto_detect_interface` chose VMware VMnet8 instead of physical WLAN 2. DNS packets to the configured Windows resolver then originated on `192.168.33.1` and timed out.
- Windows `Find-NetRoute -RemoteIPAddress 1.1.1.1` returned both a property-empty synthetic record and the real `0.0.0.0/0` route. Sorting without filtering selected the empty record and failed the completeness check; require non-empty `NextHop` before ranking candidates.
- Correct direct semantics require two independent invariants: route traffic to the direct outbound and bind core-originated sockets/DNS to the Windows-selected physical interface. The compiler now carries the latter explicitly for sing-box and Mihomo instead of trusting virtual-adapter-sensitive auto-detection.
- Final real traffic proves the route split: host/direct `183.158.168.94`, selected outbound `165.254.151.219`, blacklist match `165.254.151.219`, whitelist match `183.158.168.94`. This is stronger than status-only list-mode evidence.
- System Proxy is now fully accepted on the current source and package pipeline. Elevated TUN remains the only all-mode acceptance gap because it requires explicit authorization for temporary adapter, split-route, NRPT/DNS, IPv6, and firewall mutations.
- The final package advertises System Proxy for sing-box, Mihomo, and Xray; only sing-box/Mihomo advertise TUN. Current 1.0.26 evidence now covers all three System Proxy cores, while the runner rejects unsupported Xray TUN explicitly.
- All three System Proxy cores produced the same semantic evidence: direct/whitelist `183.158.168.94`, global/blacklist `165.254.151.219`, exact WinINet rollback, and capture-off. This rules out a core-specific generator success hidden by sing-box-only testing.
- The prior TUN routing acceptance would have produced false failures for direct mode and weak passes for list modes. It now chooses mode-aware sites and validates list routing by public IP in both TUN and the subsequent System Proxy transition.
- The old elevated `full` suite omitted routing modes entirely and did not pass `StabilitySeconds`; a green result could not prove the user objective. The matrix now includes two core-specific routing cases and sustained-health timing.
- A green network rollback check was still weaker than its label: it ignored ordinary default routes, IPv6 bindings, all non-Navo NRPT rules, TUN-scenario WinINet state, and launcher child-process residue. The runner now snapshots/asserts those surfaces and records package-owned residue as a hard failure while preserving unrelated processes by executable-path ownership.
- First authorized elevated 1.0.26 TUN routing case failed at `ADAPTER_READY`: sing-box logged `inbound/tun[tun-in]: started at Navo`, but the Service returned `TUN_ADAPTER_NOT_READY`. Exact network rollback passed and owned process residue was zero.
- Native readiness diagnostics retained an early missing-adapter error even after a later successful adapter enumeration, and could replace it with the deadline error on the final iteration. Clear stale errors on a successful snapshot and never overwrite the last meaningful observation with `waitCtx.Err`; rerun is required to reveal the actual status/MTU/address mismatch.
- After adapter readiness was repaired, TUN reached real system DNS capture but failed because `outbound.testAll` selected a 55 ms Shadowsocks node whose actual encrypted stream returned `cipher: message authentication failed`. Endpoint/latency reachability is not outbound data-plane acceptance; automated selection must require a successful System Proxy HTTP readiness transition and exact WinINet rollback before TUN.
- With a real SOCKS outbound, sing-box reached `HEALTH_COMMITTED`, Google 204, GitHub 200, no-proxy HTTPS 200, proxy/direct exit separation, and 30-second stability. Runtime rule application then failed during core reload because Rebind accepted one transient ready observation of the persistent old adapter before the new TUN generation settled; routes bound to the old generation disappeared during control-plane verification.
- TUN adapter readiness must be temporal as well as structural. Require the exact owned identity, Up status, IPv4 MTU, and CIDR to remain unchanged for two seconds; reset on any missing/down/IP Helper error or identity change before applying routes.

# 2026-08-15 Phase 57: crash and boot recovery findings

- Current physical DNS was not Navo residue: both Ethernet and WLAN 2 resolve to DHCP `223.5.5.5`, and their registry `NameServer` values are empty. Current TUN DNS ownership is an NRPT `.` rule to `172.19.0.2`; rollback must remove only that owned rule and never overwrite physical DNS.
- No HKCU/HKLM Run item, scheduled task, or Windows Service registers Navo for OS autostart. The reported post-boot failure therefore occurs during first application launch/capture activation, not an OS service auto-start entry.
- Launcher must wait longer than the maximum Service rollback path and must observe Service exit directly. A 10-second readiness timeout racing a 30-second rollback can terminate startup while reconciliation is still legitimate work.
- Reconciliation errors cannot be logged and ignored. Supervisor Ready/core start on a dirty journal creates a second owner transition before the old DNS/route transaction is resolved.
- Recovery must be single-owner and visible. An ignored Agent `RecoverOwned` call followed by a separate capture recovery path allowed a faulted System Proxy marker to survive into a new activation.
- Windows may expose a canonical Navo sing-tun device as `sing-tun Tunnel #N`. Exact base-only comparison is too strict; the safe compatibility rule is exact known base plus a non-empty decimal suffix, still gated by canonical name, non-hardware identity, GUID/index, and runtime readiness.
- Real after-NRPT process crash proved the durable journal survives exit 91 and a fresh process removes owned NRPT/routes/firewall/adapter state before accepting capture-off. Before/after direct IP and all rollback surfaces matched.
- Fresh elevated System Proxy activation on the current boot proved core readiness and application data plane, not just listener/UI state. Proxy/direct exit separation and Google/GitHub/ChatGPT/OpenAI/Baidu/Xiaomi responses matched the selected runtime mode.
- Final live state preserved v2rayN PID 5000 and exact WinINet `127.0.0.1:10808`; Navo-owned process, adapter, split-route, NRPT, and firewall counts were all zero.
- A real OS reboot after the repaired build remains unperformed; evidence is a fresh isolated application start on the already-running current boot plus a real process crash/restart cycle.

# 2026-08-15 Phase 58: project hardening audit findings

- Current source gates pass, but total Go statement coverage is 51.9%; critical Windows-facing packages are materially lower (`internal/network/tun` 10.9%, `internal/agent/systemproxy` 22.6%, `internal/service` 40.6%).
- The final 1.0.26 sing-box TUN routing-mode artifact is failed; 1.0.28 proves only the sing-box `after-nrpt` crash path and current-boot System Proxy flow, not the complete sing-box/Mihomo TUN matrix or a physical reboot.
- Release identity is split: the directory/ZIP name is 1.0.28 while Wails and diagnostics still report 1.0.0, and the produced PE files expose no FileVersion/ProductVersion.
- The sealed ZIP contains 25 files, but the already-run portable directory contains two additional runtime files outside `SHA256SUMS.txt`; `repair.exe check` still reports zero issues, so package-set verification must be separate and exact.
- README and CLAUDE documentation still describe removed AI/MySQL/Flutter/installer behavior. `.env.example` correctly states AI/MySQL were removed.
- The package includes Mihomo, Xray, and Wintun license files but no sing-box license/notice or aggregate third-party notice.
- The repository tracks 245 acceptance artifacts (about 16.8 MB); many expose network-shaped fields. New raw artifacts should be ignored and published through private/CI retention after sanitization.
- Subscription fetching validates every DNS answer but dials only the first address, losing safe public-address fallback. The asynchronous initial-refresh branch also drops `applyRuntimeConfig` errors.
- Official sing-box v1.13.14 `LICENSE` at `https://github.com/SagerNet/sing-box/blob/v1.13.14/LICENSE` states GPL v3-or-later plus a project-name association restriction. Add the exact upstream text to `third_party/sing-box/LICENSE` and include it in package verification.
# 2026-08-15 Phase 58: project hardening

- VERSION 1.0.29 is now the release source for diagnostics, Go ldflags, package naming, and all three PE file/product versions.
- The package verifier enforces a closed directory set, SHA256SUMS, core manifest hashes, required licenses, PE versions, optional Authenticode, ZIP root ownership, and per-entry length/SHA-256 equality.
- The previous 1.0.28 package is correctly rejected because it lacks VERSION. Controlled extra-file and ZIP-content tamper tests are also rejected.
- Subscription dialing validates the complete DNS answer set before any connection and then falls back across every validated public address. Mixed public/private answers still fail closed.
- Initial asynchronous subscription apply failures are now logged instead of silently discarded.
- CI actions are immutable SHA references. Windows CI adds module verification, 50% coverage, npm audit, and PowerShell parsing; Linux CI adds race and govulncheck.
- README, CLAUDE, and INSTALL_DEPLOY now describe the implemented Wails/Vue, local-only, portable workflow; stale AI, MySQL, Flutter, Android, and installer claims were removed.
- artifacts is ignored for new output, but existing tracked historical artifacts were intentionally not deleted or untracked without an explicit retention decision.
- Local automated gates pass, but elevated TUN routing, physical reboot/cold start, Authenticode, and the all-elevated package runtime smoke remain external acceptance work.
