# Findings

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
