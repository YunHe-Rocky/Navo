# Progress

## 2026-07-29 Phase 22: TUN adapter and capture transition

- Read the full implementation guide and restored the file-based work plan.
- Began the mandatory no-code-change audit.
- Located the UI/Agent/Service capture call chain and confirmed state ownership
  is split across Agent runtime, Service runtime, WinINet and Supervisor.
- Located the Wintun, route, DNS, dirty-shutdown reconciliation and undo-journal
  implementations; no runtime implementation changes have been made yet.
- Completed the mandatory pre-change audit at
  `docs/audit/NAVO_TUN_MODE_AUDIT.md`; implementation changes are now allowed.
- Chosen ownership: User Agent is the authoritative capture coordinator;
  Service owns an atomic core/TUN preparation sub-transaction. `off` will map
  to the guide's Unmanaged state and stop the core.
- One combined inspection failed because PowerShell `-LiteralPath` does not
  expand `*.go`; switched to resolved-file enumeration and independent reads.
- First Phase 22 targeted Go test did not compile packages because the old
  `.cache/go-mod` path no longer exists and Go fell back to the protected user
  cache. Domain capture tests themselves passed; rerun is pending with the
  current repository-local module cache.
- With the current cache, domain capture, TUN, Supervisor, and Service tests
  passed. Three Agent tests failed only because they asserted the superseded
  low-level call sequence/error code; the implementation now preserves
  `TUN_REQUIRES_ADMIN` and tests use isolated journals.
- Full `go test ./...` passed after the refactor.
- The first frontend build command was blocked by the host execution policy for
  `npm.ps1`; rerun through `npm.cmd` is pending.
- Logged tool issues: the first JavaScript orchestration used the unavailable
  nested name `exec_command`; switched to `shell_command`. `git status` is not
  available because this workspace does not expose repository metadata.

## 2026-07-29 Windows installer

- Started Phase 19 to turn the packaged Navo directory into a one-click Windows
  installer.
- Restored the existing repository plan and packaging history.
- Defined the acceptance scope: complete payload, elevation, upgrade-safe
  install, shortcuts, uninstall, and a produced installer artifact.
- User approved a lightweight-architecture pass before installer creation.
- Split the work into Phase 19 optimization/payload cleanup and Phase 20
  installer production.
- Completed a bounded disk audit: 1.65 GiB of the roughly 2 GiB workspace is
  repository-local development cache; the current distributable is 174 MiB.
- Confirmed there is no Vue polling loop; the main resident-memory cost is the
  always-hidden Wails/WebView2 process.
- Found and marked the real `.env` release copy as an installer-blocking secret
  packaging issue.
- Confirmed Agent-to-Service is direct/in-process in the combined launcher and
  selected a single network-free Dashboard snapshot as the appropriate bridge
  boundary.
- The first combined Phase 19 patch did not apply because one Agent context
  differed; verified it made no partial changes and switched to bounded patches.
- Added `dashboard.snapshot`, cached-IP rendering, bounded asynchronous IP
  refresh, correct proxy endpoint mapping, and regression tests.
- Changed Wails close behavior to release WebView2 instead of hiding forever.
- Added an on-demand UI process manager: normal launch opens UI, `-silent`
  remains tray-only, closing UI leaves proxy runtime alive, tray/second launch
  can reopen UI, and final shutdown still owns the child process.
- Moved installed `.env` lookup to `%LOCALAPPDATA%\Navo`, removed the developer
  `.env` from package output, and updated deployment guidance.
- Targeted Go tests passed for Agent, launcher, and Wails bridge.
- Full Go tests, `go vet`, Vue typecheck, Vite production build, and package
  rebuild passed.
- Rebuilt payload is 173.73 MiB and verified not to contain `.env`.
- Added a dedicated lightweight lifecycle smoke. Its first run stopped before
  process launch because Windows PowerShell exposed a null `Environment`
  collection; corrected it to `EnvironmentVariables`.
- The live lightweight run proved tray-only startup and a fast Dashboard
  snapshot, but this automation desktop rejected tray icon creation. Closing
  the fallback UI correctly shut down the launcher, so reopen validation moves
  to a controlled process-manager integration test rather than weakening the
  no-tray safety behavior.
- The adapted process smoke measured Dashboard at 7.9 ms and completed graceful
  Service/core shutdown, but its test-only launcher inherited host GOARCH=386
  and crashed after cleanup. The smoke build is now pinned to the release's
  windows/amd64 architecture.
- The final amd64 lightweight smoke passed with Dashboard at 8.2 ms, graceful
  Service/core shutdown, launcher exit code 0, and no Navo-owned residue. The
  automation desktop exercised the documented no-tray fallback path; UI reopen
  is independently covered by the process-manager integration test.
- WiX 7.0.0 was rejected because it requires user EULA acceptance. Installed
  WiX 5.0.2 cache-locally and retained the standard per-machine MSI design.

## 2026-07-29 TUN data-plane follow-up

- User confirmed system-proxy web browsing works and TUN browsing fails.
- Captured the TUN DNS loop in the actual sing-box log and verified direct ping
  works when capture is off.
- Confirmed the bundled core is sing-box 1.13.14 and checked its official route,
  DNS and ICMP capabilities before changing the compiler.
- Added modern UDP DNS generation for TUN, DNS hijacking, default outbound
  hostname resolution and direct ICMP routing.
- Added compiler assertions and a bundled sing-box 1.13.14 native TUN
  validation test. Full `scripts/test.ps1` now passes.
- Rebuilt `release/Navo`; launcher SHA-256 is
  `4C6230AF678E44F4D57A34B6D5611FF0ECEC55AC427BDDF483201D0EA960CFCD`
  and its packaged manifest still requires Administrator.
- Elevated live smoke did not start because the interactive UAC prompt was not
  approved within the timeout. No Navo process or network state was left by
  that attempt.
- User reproduced the new DNS service startup error: modern UDP DNS must not
  detour to an empty direct outbound.
- Removed the redundant detour, updated canonical typed-DNS validation, and
  added a real 750 ms sing-box service-start liveness test. Targeted and full
  tests pass.
- Repackaged the final launcher with SHA-256
  `229A73314BF53739EF41622C8D16C3AB3183D696E7F41810B291E87112CF8E8E`.
  The post-package UAC launch was not approved within the bounded wait, so no
  final elevated data-plane claim is made.

## 2026-07-29 TUN privilege regression follow-up

- Reopened Phase 18 after the user reproduced non-admin TUN startup failure.
- Confirmed the current source bypasses the intended `TUN_REQUIRES_ADMIN`
  preflight and reaches sing-box, which then fails to configure the adapter.
- One expected test filename (`internal/agent/agent_capture_test.go`) was not
  present; locating the actual capture tests before editing.
- Located capture regression tests in `internal/agent/agent_test.go`; the
  non-admin test currently asserts delegation to a Service that is actually
  in-process and non-privileged.
- Confirmed `cmd/navo-svc install` does not install a Windows Service and no
  release script provisions one.
- Added fail-fast privilege checks for both `capture.set(tun)` and legacy
  `tun.enable`, plus a default Windows token check and non-mutation tests.
- Updated the launcher resource directive to require Administrator. The first
  combined resource/format/test command used the wrong working directory for
  root-relative Go paths; rerunning those gates from repository root.
- Adjusted the elevation design after proving an admin `.syso` breaks automated
  package tests: `cmd/navo` remains testable with `asInvoker`, while packaging
  replaces only the final executable manifest from `winres/admin.json`.
- Full `scripts/test.ps1` passed all Go tests and `go vet`.
- First package attempt could not find the already-installed `go-winres.exe`
  because `%USERPROFILE%\go\bin` was not on the child PowerShell `PATH`; added
  deterministic fallback resolution and a pinned v0.3.3 error message.
- The old release held `navo_app.exe` open. `Get-CimInstance` was denied, so
  bounded `Get-Process.Path` identified launcher 5524, UI 2828 and sing-box
  2316 under `release\Navo`; stopping the launcher removed all three.
- Final elevated TUN smoke still timed out before any release-owned Navo
  process appeared; terminated only its known PowerShell wrapper. Static PE
  manifest verification remains valid, but the interactive UAC/TUN gate is not
  claimed as passed.
- Updated deployment documentation and release smoke startup for the required
  UAC behavior. All three PowerShell scripts parse successfully.
- UAC eventually launched the packaged Administrator manifest after the
  wrapper timeout. Verified launcher/UI/core processes and TCP listener 12080.
- The ordinary acceptance client then reproduced a Named Pipe access denial.
  Replaced the `OW` ACE with the current token user's explicit SID and added
  DACL regression coverage.
- Re-ran full `scripts/test.ps1` after the Pipe ACL change; all Go tests and
  `go vet` passed.

## 2026-07-29

- Started Phase 18 from
  `docs/design/NAVO_Windows_Proxy_TUN_Troubleshooting_and_Acceptance.md`.
- Restored the existing repository plan and confirmed prior Phase 16 addressed
  TUN selection rollback/privilege reporting, but did not by itself satisfy the
  new end-to-end proxy, TUN, DNS, lifecycle, and recovery gates.
- Fixed system-proxy false success: Agent now completes an explicit HTTP
  end-to-end request through the selected local proxy before writing WinINet.
- Removed Service-side WinINet handlers; current-user proxy ownership remains
  exclusively in User Agent.
- Removed the Agent elevation gate so a normal user-session Agent delegates TUN
  privilege decisions to the Service.
- Added a protected Named Pipe DACL for SYSTEM, Administrators, and the object
  owner, enabling the split normal-Agent/elevated-Service model without granting
  access to unrelated local users.
- Hardened Mihomo TUN output with `auto-detect-interface`, DNS hijack, and an
  enabled DNS module. Xray now returns an explicit unsupported error instead of
  silently generating a config without TUN.
- Isolated `tun_smoke.ps1` from the real `%LOCALAPPDATA%`.
- Validation passed: targeted Agent/Service/Compiler/CoreAdapter tests; full
  `go test ./...`; `go vet ./...`; Vue typecheck and Vite production build;
  Wails Windows package; packaged startup, three-core switching, HTTP data
  plane, stop/start, and clean shutdown smoke.
- Non-elevated TUN smoke returned the real sing-box `Access is denied`, kept
  WinINet and TUN disabled, and restored the prior running core.
- Elevated acceptance could not be completed: one run proved the old default
  Pipe ACL blocked the non-elevated client and motivated the DACL fix; after
  rebuilding, the final UAC-run attempt timed out before a new launcher log was
  created. All spawned test processes were cleaned up.

## 2026-07-27

- Confirmed the error comes from the obsolete Flutter packaging script.
- Started the Flutter-to-Wails migration.
- Recreated persistent planning files because none were present in the workspace.
- Inventoried the Flutter UI bridge, launcher process contract and reusable
  Named Pipe transport.
- Verified the local Go and Node toolchains and identified two remaining
  Flutter-specific launcher assumptions.
- Added the pinned Wails v2.12.0 Go dependency using repository-local caches.
- Added the Wails UI host, a framework-neutral Named Pipe bridge, pinned Vue
  build configuration, and functional screens for core, proxy, TUN, nodes,
  subscriptions, AI and logs.
- Generated `package-lock.json`; npm audit reports zero vulnerabilities.
- TypeScript checks and the Vite production build pass with Vue 3.5.40,
  Vite 8.1.5 and TypeScript 6.0.3.
- Built `navo_app.exe` successfully with Wails v2.12.0 and no Flutter/Dart SDK.
- Replaced Flutter-specific launcher window/resource assumptions with the
  stable `NavoAppWindow` class and framework-neutral tray icon path.
- Replaced the package script with a Go/npm/Wails pipeline and removed Flutter
  source, CMake runner, Dart metadata and legacy packaging files.
- Updated build/test/clean/smoke scripts and Makefile for repository-local
  caches, Wails artifacts and all three required cores.
- Fixed startup restoration so a persisted Mihomo/Xray selection is always
  compiled into that core's configuration before the first process start.
- Full packaged smoke passed Wails startup, IPC, three core switches, three
  local HTTP data-plane requests, core stop/start, graceful shutdown and
  zero newly-created residual processes.
- Changed launcher shutdown to let the kill-on-close Job Object terminate
  WebView2 children only after the parent completes graceful shutdown.
- Preserved `runtime_state.json` across normal shutdown so core and routing
  selections survive restart.
- Final Go test/vet gate passed for every package, including the Wails host.
- Final Vue typecheck and Vite production build passed; npm audit reported
  zero vulnerabilities.
- Final package build completed without any Flutter/Dart/CMake/Ninja lookup.
- Final smoke passed all read-only IPC methods, three-core switching, HTTP
  status 200 through each core, core stop/start, graceful launcher exit and
  residual process checks.
- `repair.exe check` returned zero issues.
- All 21 entries in `SHA256SUMS.txt` were independently recalculated and match.
- Scanned 116 source/script/document files and found zero obsolete Flutter
  build-path references.

## 2026-07-28

- Began the mandatory multi-core architecture document task.
- Re-read the 2,122-line source as UTF-8 after detecting PowerShell mojibake.
- Confirmed the document requires a repository audit as the first and only
  implementation action before later refactoring phases.
- Recorded the user's permanent Wails v2 + Vue 3 decision and changed the
  persistence target from SQLite to the existing cloud MySQL.
- Corrected the architecture execution document so its stack, boundaries,
  audit checklist, migration plan and mandatory test commands match Wails and
  MySQL; no legacy UI build tooling is required.
- Packaged the current Wails application successfully. The subsequent smoke run
  was interrupted; its test-owned Python helper was stopped, while a pre-existing
  sing-box process was left untouched.
- Created `docs/audit/NAVO_CURRENT_STATE.md` with the current tree, architecture,
  both failure chains, root causes, sing-box coupling, protocol-field loss,
  network/health/IPC/revision/database state, exact core hashes, missing tests,
  migration risks, phased plan, first-phase files and stable no-touch modules.
- Verified the corrected design document has zero references to the retired UI
  stack and the audit contains every section required by the execution brief.
- Stopped at the audit boundary without changing business code, MySQL, system
  proxy, TUN, routes or core binaries.
## 2026-07-28 Multi-core implementation

- Started Phase 8 after explicit user authorization.
- Added strict domain packages for core, source, capture, selection, endpoint,
  upstream proxy and compatibility models.
- Added unit coverage for source exclusivity, protocol/spec matching, transport,
  TLS/Reality, upstream UDP and compatibility invariants.
- Added an ignored workspace `.env`, safe `.env.example`, first-file dotenv
  loader and validated MySQL environment configuration.
- Wired dotenv loading into `cmd/navo` before Service construction and added
  `.env.example` to release packaging.
- Verified `go test ./internal/domain/...`,
  `go test ./internal/infrastructure/config ./cmd/navo`.
- Selected `github.com/go-sql-driver/mysql` v1.10.0 after checking its current
  official documentation; its Go 1.24 minimum is compatible with this module's
  Go 1.26.4 directive.
- Added the MySQL connector with TLS verification, connection limits, dial and
  I/O timeouts, context cancellation and no plaintext fallback.
- Added versioned, checksummed and lock-protected Schema migrations for all 12
  mandatory architecture tables, plus explicit down definitions.
- Added the transactional cloud `ActiveSelection` repository and injected it
  only into Service. UI and Agent receive neither DSN nor database handle.
- Optional MySQL failure now logs degraded mode and keeps local LastKnownGood;
  `NAVO_MYSQL_REQUIRED=true` makes startup fail closed.
- Verified MySQL/config/domain/service/launcher tests after wiring.
- Introduced three concrete `CoreAdapter` implementations with independent
  version detection, capabilities, native compilation, native validation
  arguments, launch specs and readiness probes.
- Changed Mihomo output from JSON-as-YAML to an actual native YAML document.
- Added a pinned root `CORE_MANIFEST.json`; launcher now verifies all three
  SHA-256 hashes and exact binary versions before Service startup.
- Fixed the false-health root cause in the current host: sing-box, Mihomo and
  Xray now parse their own config format for a readiness port, and a running
  process with no declared port is unhealthy instead of implicitly healthy.
- Added version-aware compatibility resolution with explicit reasons/warnings
  and six core/source combination coverage.
- Ran each bundled binary's real native validation command against output from
  its concrete adapter; Mihomo, Xray and sing-box all accepted the generated
  configs.
- Began strict source separation: upstream HTTP/HTTPS/SOCKS5 proxies now have
  an independent typed repository and can no longer be created as arbitrary
  subscription outbounds.
- Added a credential abstraction with Windows DPAPI file storage and an
  in-memory test implementation. Upstream usernames/passwords and subscription
  URLs are persisted only as opaque references.
- Subscription URL listing no longer returns the secret URL to UI clients, and
  legacy plaintext URL entries are migrated into the credential store.
## 2026-07-28 Final architecture and release verification

- Added typed core/source/capture/endpoint/selection/revision domain models.
- Added sing-box, Mihomo and Xray adapters, native validation, manifest hash verification and readiness probes.
- Added secure `.env`/`.env.example` handling and TLS-only optional cloud MySQL integration without installing MySQL.
- Added MySQL migrations, ActiveSelection repository and transactional Core Revision repository.
- Separated airport subscriptions from independent HTTP/HTTPS/SOCKS5 upstream proxies.
- Moved subscription URLs, upstream credentials and cached subscription endpoints behind DPAPI storage.
- Replaced the public generic Wails API with typed Go/Vue DTOs and rebuilt the desktop UI.
- Added Candidate/Active/LastKnownGood runtime metadata and synchronous apply/rollback semantics.
- Added system-proxy ownership snapshots, crash recovery and original-state restoration.
- Fixed WebView2 data isolation, Job Object cleanup and shutdown/core-switch races.
- Kept a real ignored `.env` and packaged both `.env` and `.env.example` with blank credentials.
- `scripts/test.ps1`: all Go tests and `go vet` passed.
- UI smoke: Edge automation passed overview, route-source separation, secure subscription display and 900px minimum window.
- `scripts/package.ps1`: Vue production build and Wails v2 Windows/amd64 package passed.
- `scripts/smoke.ps1`: packaged startup, IPC, all three core switches, HTTP data plane, stop/start, clean launcher exit and zero Navo child residue passed.
- Release output: `release/Navo`.
- Not executed: live cloud MySQL connection because no real server credentials were supplied.

## 2026-07-28 TUN transaction and readiness repair

- Reproduced the reported UI rollback and `CORE_004` against the user's real runtime state.
- Confirmed the original sing-box failure was Windows `Access is denied` while configuring the TUN interface, not a port 12080 conflict.
- Replaced the Vue two-command capture switch with one typed `capture.set` transaction in Agent.
- Made `tun.enable` synchronous so success means the candidate core is healthy and committed.
- Reworked core startup monitoring to catch early process exit and include the last process output instead of waiting ten seconds for a misleading port timeout.
- Added an elevation preflight returning `TUN_REQUIRES_ADMIN` before any core restart or proxy mutation.
- Fixed system-proxy ownership so Navo never disables another proxy application's registry state without an ownership record.
- Added TUN transaction, non-admin preflight, early-exit diagnostic and proxy ownership regression tests.
- Full `scripts/test.ps1` passed, including `go test ./...` and `go vet ./...`.
- Vue typecheck, Vite production build, Wails v2 build and final package creation passed.
- Final non-admin TUN smoke preserved proxy/TUN/core state and returned the intended privilege error.
- Final three-core smoke passed twice consecutively: sing-box, Mihomo and Xray each returned HTTP 200, core stop/start passed, and shutdown left no Navo-owned residue.
- One preceding smoke exited once with Windows `0xC0000005` during shutdown; it was not reproduced in the next two identical full runs.
- Administrator TUN smoke could not pass the interactive Windows UAC boundary in the automation session; the timed-out attempt left no Navo/TUN residue.

## 2026-07-28 Full system tray implementation

- Began Phase 17 after explicit user authorization.
- Re-read `NAVO_TRAY_IMPLEMENTATION_SPEC.md` and confirmed the current native
  tray only implements Show and Exit; all status and proxy-control sections
  remain to be wired.
- Added a Service-owned `EndpointStatus` cache with compatibility, probe color,
  failure reason, timestamp and latency; tray reads it through Agent snapshots.
- Added Agent-composed tray snapshots and connection/recovery transactions so
  native tray code never starts cores or changes system capture directly.
- Replaced the two-command menu model with status, connection, airport/upstream
  endpoints, three cores, capture, route, diagnostics, settings and exit.
- Implemented recursive Win32 menu rendering with checked/disabled states,
  on-demand state refresh and asynchronous Agent action dispatch.
- Added sanitized diagnostic export, separate core/application log reads, cached
  exit-IP display and active-selection preflight for Connect.
- `go test ./...` passed after the implementation; package and real process
  lifecycle validation remain pending.
- Added `tray.snapshot` to the packaged named-pipe smoke contract and verified
  every required Service state section is present.
- `scripts/test.ps1` passed all Go tests and `go vet ./...`.
- Vue typecheck, Vite production build, Wails v2 Windows/amd64 build and release
  packaging passed; output remains `release/Navo`.
- Packaged smoke passed `tray.snapshot`, all three core switches, HTTP 200 data
  plane, core stop/start and clean residue checks on two consecutive final runs.
- Real Win32 UI smoke opened the native root menu, captured
  `release/tray-menu-smoke.png`, clicked the actual Exit item, received exit code
  0 and logged a complete Agent/Service/core shutdown.
- The historical launcher shutdown race reproduced twice as Windows
  `0xC0000005` before the two final passes. It remains an intermittent known
  issue outside the tray feature; no process residue remained.
## 2026-07-29 Phase 20

- WiX 5.0.2 已生成 `release/installer/Navo-Setup-1.0.0-x64.msi`（50,839,552 bytes）。
- SHA256: `7e28bf2cacae7e87b30fb19eb11790d5238931e71e9aecc903470e4dc3345436`。
- 当前受管环境无法访问 Windows Installer 服务，`msiexec /a` 与 ICE 校验均因系统错误 2502/2503/2203 中止；继续使用 WiX decompile 做不依赖服务的结构校验。
- WiX decompile 成功：确认 `navo.exe`、26 个 payload、桌面/开始菜单快捷方式及 `MajorUpgrade`。
- 清理可重建缓存后，`.cache` 从约 1.65 GiB 降至 52.91 MiB；保留 `release/Navo`、MSI 成品与 WiX 编译器。

## 2026-07-29 Phase 21

- 将原概览的内核、线路和接管配置移入独立的连接管理/线路来源/内核管理页面；左侧导航扩展为 8 个明确职责页面。
- 新增统一 `AppState`/`IconState` 推导、3 次失败/2 次成功防抖、2 秒采样与固定 30 点 `TrafficRingBuffer`。
- 新增 `App.CheckIP`，Direct/Proxy 双 Transport 返回国家、城市、ISP、ASN、网络组织、来源、时间与公开网络属性。
- 修复 IP Geo enrichment 使用已取消 context 导致字段经常为空的问题。
- 正式图标未替换；设置页展示沿用优化与 3 个重设计方向及 16/24/32/48 视觉状态。
- `scripts/test.ps1` 全部 Go tests 与 `go vet` 通过；Vue typecheck/Vite build 通过。
- Edge UI smoke 通过 8 页面、1440×900/900×700、无横向溢出、0 console error。
- Wails production package 已重建到 `release/Navo`。
- sing-box 与 Mihomo 已通过 `CoreMetricsAdapter` 接入 loopback-only `/connections` 真实累计流量和活动连接；Xray 无本地 Stats API 时明确降级。
- packaged lightweight smoke 断言 sing-box metrics adapter 可用，Dashboard 12.8 ms，launcher exit code 0 且完整关停。
- 新 setup EXE SHA256：`823ca522589a95595b25feec76bdb347d628e8e1d287e6c818518da28b2c7296`。

## 2026-07-29 Phase 22

- Completed the mandatory pre-change audit in
  `docs/audit/NAVO_TUN_MODE_AUDIT.md`.
- Added the capture state machine, adapter truth model and atomic transition
  journal with unit coverage.
- Added serialized Agent/Service coordination, cancellation checks, bounded
  probes, fail-safe `off` rollback and startup recovery.
- Added deterministic Wintun GUID/create-or-open lifecycle, real adapter
  monitoring and journal-owned route/DNS cleanup.
- Added phase-aware Vue mode switching, duplicate-action lockout, one-shot TUN
  fault dialog, focus handling and Escape close.
- `scripts/test.ps1` passed all Go tests and `go vet`.
- Vue typecheck and production build passed.
- Edge UI smoke passed transition phases, locked controls, committed mode,
  fault dialog focus/Escape and 900x700 overflow checks.
- Rebuilt `release/Navo`, MSI and
  `release/installer/Navo-Setup-1.0.0-x64_setup.exe`.
- Setup EXE is 50.57 MiB; SHA-256 is
  `1ee2b0ba3109e574da278712fc4c6ef668c8dc9e4a63289ee38453b2e23b8454`.
- Extracted setup resources prove the embedded icon hash exactly matches
  `winres/#1_0000.ico`.
- Updated elevated smoke to elevate launcher and IPC client together. Real
  elevated adapter/route/DNS/HTTP acceptance remains pending and is not claimed.

## 2026-07-29 Phase 23

- Reproduced the reported symptom from source and the latest packaged log.
- Identified the three-second `Get-NetAdapter` PowerShell monitor as the direct
  repeated-console source.
- Confirmed the Wails UI and WebView2 did initialize; no UI crash is present in
  the captured run.
- Added a shared Windows process policy that combines `HideWindow` with
  `CREATE_NO_WINDOW` and applied it to runtime core, validation, compiler,
  taskkill and network command paths.
- Replaced the three-second PowerShell adapter poll with native
  `GetAdaptersAddresses`, eliminating the recurring child process completely.
- Deferred combined-desktop core startup until a capture mode is selected, so
  launch no longer starts and immediately stops a core.
- Added bounded UI foreground activation after the Wails window is created.
- Added Windows flag, native adapter and UI focus regression tests.
- Single-window packaged smoke proved one Wails window was created and the
  launcher exited cleanly, while exposing a 15-second stale-adapter recovery
  delay.
- Added the missing Wails capture/TUN DTO fields and defensive Vue dashboard
  normalization so backend schema skew cannot blank the interface.
- Moved Agent UI pipe startup ahead of capture recovery and bounded stale
  disabled-adapter cleanup to 1.5 seconds.
- `scripts/test.ps1` passed all Go tests and `go vet`; the Windows no-console
  child-process test passed.
- Vue typecheck and Vite production build passed.
- Rebuilt `release/Navo`; the final single-window packaged smoke held exactly
  one visible UI window for two seconds, observed no duplicate UI process,
  returned dashboard data in 17.1 ms, and exited with launcher code 0.
- Rebuilt MSI and `release/installer/Navo-Setup-1.0.0-x64_setup.exe`. The setup
  EXE is 52,996,096 bytes with SHA-256
  `c2b42abe4b70eb9352c7f47de855577a0fb75e402c0025c53fb5734078856458`.
- Extracted setup resources and confirmed the embedded icon hash exactly
  matches `winres/#1_0000.ico`.
- Verified zero Navo-named and zero release-owned processes remained after the
  final smoke. v2rayN and its sing-box processes were not touched.

## 2026-07-30 Phase 24

- Recovered the interrupted task from the active task metadata and verified no
  Phase 24 implementation had been written yet.
- Added Phase 24 to the persistent plan.
- Generated the pixel-tech desktop dashboard design direction and recorded the
  network-test accessibility/loading requirements before implementation.
- Added Wails diagnostics for Windows host resources, bounded Cloudflare
  latency/download/upload tests through the active loopback proxy, cancellation,
  per-core SHA-256 verification and official GitHub release checks.
- Added the network diagnostics workspace, core upgrade state and a denser
  high-contrast pixel-technology visual system to the existing Vue UI.
- User rejected the inferred single-theme layout and supplied the exact UI
  contract. Removed the global refresh and standalone log navigation, added a
  unified action progress model, moved logs into settings, and replaced the
  visual direction with separate day/night shape and palette systems.
- Final Edge smoke passed both explicit theme tokens and compositions,
  confirmed no global Refresh or top-level Logs action, observed the scrolling
  progress bar during benchmark/update actions, verified Settings-contained
  logs, 900x700 horizontal fit and zero console errors.
- Rebuilt Vue production assets and the Wails desktop package into the isolated
  `release/Navo-phase24` directory because an elevated older Navo instance still
  owns the default release log and global Pipe.
- `repair.exe check` found zero issues; all 24 SHA256SUMS entries match; all
  five new Wails diagnostic/update bindings are present in the packaged build.
- Refined the theme contract so night now reuses the day composition exactly:
  sidebar, header, content, hero, overview grid and monitor grid keep identical
  browser geometry across the switch.
- Replaced theme-dependent border widths and layout-affecting radii with shared
  component geometry plus palette/shape tokens. Day uses asymmetric sharp
  corners and hard orange/black shadows; night uses the same bounding boxes
  with asymmetric rounded purple/blue surfaces.
- Vue typecheck and production build passed. Edge smoke compared seven major
  layout regions at 1180x760 with zero position/size delta, then revalidated
  both themes, feature progress, settings logs and 900x700 horizontal fit with
  zero console errors.
- Rebuilt `release/Navo-phase24` with the refined theme system. Package repair
  reports zero issues and all 24 `SHA256SUMS.txt` payload entries independently
  match.
- Reduced the day form's pure-white perimeter by introducing warm gray, beige
  and muted orange-brown transitions across body, sidebar, header, controls and
  panel surfaces.
- Unified day/night depth: both now use the same directional offset plus soft
  ambient shadow model, with low-saturation cool blue contrast in day and warm
  purple-red contrast in night. The day proxy-IP feature card no longer reuses
  the night-only dark navy fill.
- Re-ran Vue typecheck/build and the Edge theme smoke with invariant geometry,
  900x700 fit and zero console errors. Rebuilt `release/Navo-phase24`; repair
  again reports zero issues and all 24 hashes match.
- Corrected the over-unified theme pass: day selection surfaces now stay within
  warm beige/orange-brown, while night selection surfaces stay within
  purple/blue. Green is limited to small health/success semantics instead of
  recoloring selected containers.
- Restored night-specific black-purple-blue depth and rounded blur shadows;
  removed the day-style offset shadow from night. Stable post-switch screenshots
  confirm day and night remain geometrically identical but visually distinct.
- Vue typecheck/build and Edge smoke passed again with zero console errors.
  Rebuilt `release/Navo-phase24`; repair found zero issues and all 24 hashes
  independently match.
- Removed the proxy-IP card's fake `featured` state and centralized visual
  selection feedback across navigation, theme switch, capture mode, source
  tabs, routes and cores.
- Replaced full-surface selection recoloring with a subtle same-hue overlay,
  low-contrast border and 3px theme-colored marker. Primary actions retain
  strong color, while selection remains visible without competing with them.
- Re-ran typecheck/build and stable Edge day/night screenshots; geometry stayed
  invariant and console errors remained zero. Rebuilt the isolated package;
  repair found zero issues and all 24 hashes match.
- Split visual behavior by shell region without changing layout: the top status
  area and left menu now use an animated selected-edge trace; the remaining
  content area keeps its existing composition with detail-only refinements.
- The trace runs from the upper-left down the left edge and then across the
  bottom edge. Day uses sharp orange motion over bright beige; night uses
  rounded blue motion over dark purple.
- Restored black card shadows for day and blue card shadows for night. Added a
  360ms delegated card feedback state: day cards deepen to beige and night
  cards lighten to purple, then return without touching feature handlers.
- Typecheck/build and Edge smoke passed. Smoke now also asserts the edge
  animation and card feedback lifecycle. Rebuilt `release/Navo-phase24`;
  repair reports zero issues and all 24 hashes match.
- Diagnosed the reported missing latency/speed functions from real package
  logs. The Wails bindings existed, but the benchmark action was disabled when
  core startup rolled back; the selected MiYaIp SOCKS upstream also has real
  DNS/connection failures.
- Added visible latency and full-speed actions to the overview speed card.
  Latency invokes `outbound.test` without requiring a running core. Full speed
  temporarily starts a local-only core when capture is off, runs the benchmark,
  then restores the stopped state without changing system proxy or TUN.
- Removed the disabled benchmark gate from network diagnostics and replaced it
  with an actionable "start local core and test" state. Errors now surface the
  actual endpoint/DNS/timeout reason.
- Full Go tests/vet, Vue typecheck/build and Edge smoke passed. Browser smoke
  proves the overview latency call and stopped -> start -> benchmark -> stop
  lifecycle. Rebuilt package repair found zero issues and all 24 hashes match.
  The elevated test instance was removed with zero Navo residue; v2rayN PID
  9552 was preserved.

## 2026-07-30 Phase 26 source diagnostics and native theme integration

- Replaced the fixed dark native window theme with the Windows system default at
  startup and synchronized the native title bar/background with every day/night
  switch through the Wails runtime.
- Added a strict source-type constraint to Connection Management: a route from
  another source type no longer appears selected in the current filter, and a
  new connection cannot start until a matching route is selected.
- Added one shared diagnostics toolbar for airport and dedicated-proxy sources:
  bounded four-worker batch latency, current-route speed test, per-route latency,
  and per-route throughput. Per-route throughput temporarily switches and starts
  only when capture is off, then restores the prior route and core state.
- Core cards now label both current and official latest versions explicitly.
  Settings no longer contains the duplicate interface-form cards and now renders
  the changelog as plain text above the existing diagnostic log.
- Go tests/vet and Vue typecheck/production build pass. The Edge UI smoke proves
  native theme calls, source constraint, both source filters, batch/single
  diagnostics, route/core restoration, current-version labels, changelog, logs,
  desktop fit and zero browser errors.
- Built `release/Navo-phase26`; `repair.exe check` reports zero issues and all 24
  `SHA256SUMS.txt` entries independently match.

## 2026-07-30 Phase 27 recovery hardening

- Added Phase 27 without changing the approved day/night layout.
- Fixed the original duplicate-Wintun ownership conflict by leaving adapter
  creation to the active core, split the Windows IPv6 firewall range into
  accepted halves, and made legacy firewall undo idempotent.
- Added detected core versions and capture capability reporting; the UI only
  disables TUN when the active core explicitly reports it unsupported.
- Full Go tests/vet and Vue typecheck/build passed; `Navo-phase29` repair found
  zero issues and all 24 hashes matched.
- Elevated validation exposed a remaining real failure: sing-box listened on
  12080 but reset the end-to-end HTTP proxy probe. WinINet was not committed and
  no TUN journal or Navo process remained, proving fail-closed rollback for this
  path.
- Fixed core lifetime ownership: the runtime now outlives the startup IPC
  context and is canceled only by Supervisor stop/recovery.
- Elevated Phase 32 proved sing-box system proxy, TUN adapter creation, route
  activation and real Microsoft HTTP data-plane success. Its TUN-off rollback
  then exposed non-idempotent route undo and disabled-adapter cleanup.
- Made every route/DNS/firewall undo explicitly idempotent and added post-core
  Wintun open/destroy cleanup. A subsequent startup consumed the old failed
  network journal and left zero TUN adapters.
- Hardened WinINet ownership so a newer external proxy is never overwritten;
  added direct regression tests for exact-owner restore and external-owner
  relinquish.
- Added runtime capture health monitoring: three consecutive failed observations
  move system proxy/TUN to fail-closed `off` and expose the existing fault state.
- Recovery journals no longer execute stored PowerShell commands; known actions
  are reconstructed from trusted code and tampered actions remain journaled.
- Fixed background IP detection to use per-detector HTTP/1.1 transports after an
  elevated recovery startup exposed a Go HTTP/2 nil dereference.
- Final full Go tests/vet and Vue typecheck/build pass. `Navo-phase34` repair
  reports zero issues and all 24 SHA-256 entries match.
- Final elevated three-core matrix could not be rerun because the platform
  rejected further administrator execution after its usage limit was reached.
  Phase 27 remains pending only at that live acceptance gate.
