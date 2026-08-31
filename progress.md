# Progress

## 2026-08-15 Phase 57: crash-safe DNS and boot proxy recovery

- Restored the persistent planning context and prior full connection-lifecycle acceptance rules.
- Started a read-only correlation of current Windows DNS/WinINet, startup ownership, transaction journals, runtime state, processes/listeners, and logs. No live network or user process has been changed.
- Current-state audit found no Navo process/listener/startup registration, preserved the unrelated v2rayN sing-box process, and recorded physical-interface DNS plus WinINet state.
- The normal-user runtime state is stale across the current reboot and still marks its latest revision as a candidate; the visible recovery marker records a prior clean exit and no root capture journal is present.
- Built-in `apply_patch` is unavailable because the WindowsApps executable is denied by the host ACL. Repository edits use equivalent unified patches through `git apply`; initial malformed/CRLF-mismatched patches were corrected without partially changing files.
- Registry inspection corrected the initial suspicion: `223.5.5.5` is DHCP-provided on both physical adapters, not a current Navo override. Current live DNS/NRPT state is clean.
- Source trace confirmed three recovery-order bugs: Launcher timeout shorter than rollback budget, Supervisor fail-open reconciliation, and Agent capture transitions allowed after a startup recovery fault.
- Chosen repair is one fail-closed protocol: align startup/recovery deadlines above the 30-second network rollback budget, block core readiness when reconciliation fails, and automatically retry ownership-aware recovery before any new capture transition.
- Phase 57.1 current-state evidence is complete; deterministic lifecycle regressions are next.
- Implemented the four production changes and added Launcher, Supervisor, and Agent recovery-order regressions; formatting and `git diff --check` pass.
- First two focused-test attempts failed before compilation because the repository/global module caches lacked the pinned dependencies while `GOPROXY=off`; downloaded only `go.mod`-locked modules to `.cache/go-mod` through `goproxy.cn`.
- Focused Launcher and Supervisor tests pass. Agent reached production regressions and found one expected call-trace change: core-switch rollback now performs an additional idempotent `capture.prepare(off)` before restoring the previous capture mode.
- The extra cleanup is the intended fail-closed invariant, not a product regression; update that exact sequence expectation and rerun focused tests.
- Updated only that exact sequence assertion. Launcher, Agent, and Supervisor focused suites then passed once and passed 5/5 consecutive repetitions.
- Confirmed `capture.set` owns a two-minute IPC budget, so the new 45-second recovery budget cannot be cut off by the UI request layer.
- Full `scripts/test.ps1` passed every Go package and `go vet ./...` with exit code 0.
- Audit found one remaining deterministic coverage gap: process death after applying the owned NRPT rule was not followed by a fresh-process journal recovery in unit tests; adding that regression before packaging.
- Added fresh-process `after-nrpt` crash recovery: the first Manager panics after NRPT mutation with the durable V2 journal retained; a new Manager removes only the owned NRPT resource and clears the journal after success. Passed 10/10.
- Added transient WinINet restoration retry coverage: the ownership marker survives the first registry failure and clears only after the second exact restore succeeds. Passed 10/10.
- Re-ran full `scripts/test.ps1` after both additions; every Go package and `go vet ./...` passed with exit code 0.

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
# 2026-08-01 Four-Guide Remediation

- Read the planning-with-files instructions and restored the existing repository planning context.
- Confirmed only untracked inputs are `.claude/` and the four newly supplied Guide documents; no tracked diff is present.
- Indexed all four Guide headings and established the new Phase 28-33 workstream.
- Confirmed AI and MySQL are still fully present and wired into production startup/dispatch; confirmed several Feature Optimization surfaces already exist but require requirement-level gap analysis.
- Read the cleanup/privacy and Full Remediation P0/P1 requirements with UTF-8 decoding and captured their transaction/security/release invariants.
- Baseline Go gate passed: `scripts/test.ps1` completed all packages and `go vet`; host Go reports `go1.26.4 windows/386`.
- Frontend `npm ci` and `npm run typecheck` passed. Baseline lacks the required `npm test` script, so the combined command stopped before build; this is recorded as an original release-gate gap.
- Frontend baseline build passed. Created `docs/CODE_REACHABILITY_AND_REMOVAL_PLAN.md` covering production/dynamic entries, Wails, IPC, tray, goroutines, repositories, build tags, removal classifications, and evidence gates.
- Read the complete Feature Optimization requirements and recorded the delta between current inspection/benchmark/two-series UI and the required transactional updates, layered latency, four-source traffic, structured logs, risk aggregation, interaction, accessibility, and icon acceptance.
- Read Self-Healing sections covering architecture, event/error/action models, core/TUN/route/DNS/system-proxy/outbound/subscription/monitor/log/init policies, safety gates, budgets, deduplication, correlation, locks, and circuit breaking.
- Completed Phase 28 and created `docs/NAVO_REMEDIATION_REPORT.md` with conflict rules, phase evidence, baseline defect, and acceptance status.
- Removed the complete AI production vertical slice: implementation package, settings persistence/tests, Service state/handlers/dispatch, Agent forwarding, IPC constants, and smoke probe. Local non-AI diagnostics remain.
- AI removal passes the complete repository Go test/vet gate. A narrowed production residual scan reports no AI package/config/IPC symbols.
- Replaced MySQL repositories with a versioned local selection/revision store using flushed same-directory atomic replacement; deleted MySQL config, driver, connector, migrations and repositories; `go mod tidy` and full Go test/vet pass.
- Sanitized the project `.env` and `.env.example` to remove all AI/MySQL keys; neither file now contains an environment assignment. Updated smoke/package/deployment text to the local-state model.
## 2026-08-01 Phase 29 completion

- Removed the complete AI runtime/settings/IPC slice and MySQL bootstrap/repository/driver slice.
- Added versioned atomic local revision and selection repositories and wired them into the launcher.
- Added an early Current-User DPAPI device-state gate, precise legacy cleanup, foreign-context privacy reset, tamper blocking, and cleanup-failure blocking.
- Centralized new persistence writes on `internal/fsatomic`; subscription writes now use the same writer.
- `scripts/test.ps1` passes after the completed Phase 29 implementation.
- Advanced to Phase 30 P0/P1 runtime and security remediation.

## 2026-08-01 Phase 30 security boundary batch 1

- Disabled the Service Named Pipe in the combined desktop process; standalone `navo-svc` must explicitly enable it.
- Replaced Pipe polling with an in-process Service readiness signal and fail-closed startup propagation.
- Preserved the launcher-verified core path through `Service.New`; removed post-verification binary discovery override.
- Removed caller-controlled `config_path` from core start/restart handlers.
- Made Job Object initialization failure fatal.
- Replaced the PID lock file with `Local\Navo.<CurrentUserSID>` Named Mutex ownership.
- Removed unconditional image-name `taskkill` startup cleanup.
- Full Go test/vet gate passed after this batch.

## 2026-08-01 Phase 30 lifecycle/API batch

- Removed raw `core.start`, `core.stop`, and `core.restart` from Agent and Service IPC dispatch.
- Removed the Wails/TypeScript `SetCoreRunning` bridge; disconnected benchmarking now fails explicitly until the isolated Phase 31 benchmark runner is implemented.
- Changed `connection.restart` to restart the committed Capture mode through the existing transition transaction.
- Added deterministic regressions for raw UI lifecycle rejection and untrusted `config_path` rejection.
- Go tests/vet, Vue typecheck, and Vue production build pass; Vite transformed 18 modules.

## 2026-08-01 Phase 30 lifecycle/network batch

- Removed Host autonomous crash restart; Host now only reports exit and Supervisor exclusively owns reconciliation, budget, backoff, and restart.
- Bound Supervisor monitoring to its lifecycle context rather than the initiating request context.
- Added crash-observation and request-context regression tests; full Go test/vet passed.
- Fixed TUN endpoint route generation to select one physical route, cast scalar interface/next-hop values, exclude the Navo adapter, and reject same-metric conflicts.
- Added PowerShell UTF-8 output prelude and GBK error-output fallback with tests; full Go test/vet passed.

## 2026-08-01 Phase 30 proxy/subscription security batch

- Rebuilt system-proxy persistence as a complete fail-closed WinINet snapshot with dynamic registry reads, exact missing-value restoration, Current User atomic files, pending/committed ownership, and `InternetSetOptionW` refresh.
- Enforced HTTPS at subscription-add time.
- Added DNS-rebinding-resistant subscription dialing: resolve all addresses, reject private/link-local/multicast/CGNAT/metadata/documentation/reserved ranges, and dial a pinned validated IP while preserving TLS hostname verification.
- Replaced the line-based Clash parser with `yaml.v3`, including nested WS/gRPC/Reality options.
- Replaced server/port/type deduplication with a canonical connectivity fingerprint containing a credential digest and transport/TLS identity.
- Added parser-output validation and in-memory rollback on failed subscription persistence.
- Full Go test/vet passed after fixing the stable-ID fingerprint lookup regression.

## 2026-08-01 Phase 30 recovery hardening

- Updated Service and Agent capture rollback semantics so incomplete rollback remains faulted in the previous committed mode instead of being presented as `off`.
- Started consolidating recovery around `internal/network.Reconciler`; next gate is fail-closed journal handling plus focused and full Go tests.
- Recovery implementation and tests are patched and formatted. The first focused test run was blocked by the managed sandbox denying the user Go module/build caches, so the same test gate must be rerun with the repository-local cache configuration already used by this project.
- `internal/network` and `internal/service` focused suites pass. Agent failures exposed two regressions now corrected: successful rollback no longer performs a redundant `tun.status` call, and dashboard no longer exposes an unowned user WinINet proxy endpoint. Added explicit rollback-failure evidence retention coverage.
- Recovery/rollback gate complete: `go test ./internal/network ./internal/agent ./internal/service` and `go test ./...` both pass. The duplicate no-op `internal/recovery` package is removed; the active reconciler persists under `ConfigDir` and cannot promote partial recovery to `READY`.
- IPC/storage hardening implemented: full frame writes, symmetric non-zero/10MB bounds, Windows write-through atomic replacement, old-file preservation injection test, and cancel/reap tracking for pending named-pipe operations. Added Windows timeout recovery and close-cancellation integration coverage.
- Removed ordinary UI/IPC exposure of `service.shutdown`; per-connection dispatch remains serialized, each request now has a bounded context, and response writes receive a fresh deadline with checked errors. Focused Agent/Service/pipe/fsatomic tests pass.
- Migrating remaining critical persistence paths to the unified `fsatomic` implementation: credentials, capture/network journals, generic store, and upstream proxy repository are converted; their focused suites pass except a mechanical unused import pending cleanup in `storage`.
- Critical persistence migration complete: runtime state, compiler revisions, core-generated configs, credentials, capture/network/recovery journals, repositories, proxy ownership, subscription state, and initialization state now share `fsatomic`. Remaining direct writes are only test/temp artifacts and a timestamped diagnostics export. Removed the unreachable fake `SingBoxHost.Reconcile`; network recovery has one production owner.
- Windows atomic persistence now applies a protected DACL for Current User, SYSTEM, and Administrators to existing state, parent directories, and replacement candidates; dedicated ACL and replacement-failure tests pass.
- Phase 31 four-series backend is implemented: Windows physical-interface totals and core proxy totals remain separate, backend sampling emits four BPS/totals with source state, and reset/sleep/availability transitions are covered by unit tests. Service/monitor/Wails focused suites pass.
- Four-series Vue integration complete: backend samples are consumed directly (no frontend business-counter recomputation), defaults are local/proxy download, user series selection persists safely, colors and line styles are stable across themes, unavailable sources are explicit, and the production Vue typecheck/build passes.
- Structured log foundation and UI are integrated: explicit producer/service fields, stable levels, time/service/level filters, cursor pagination, bounded JSONL persistence, safe clear, redaction, local-time rendering, and optional follow mode without forced scrolling. Store tests, backend focused tests, and Vue production build pass.
- Structured-log concurrency/error-emission follow-up passes focused Go tests and the Vue production build. The initial npm retry failure was only a wrong working directory and passed from `navo_app`.
- Added the Phase 32 self-heal foundation: stable `NAVO_*` models, exact-code policy registry, bounded/deduplicating queue, per-resource mutexes, jittered backoff, attempt windows, open/half-open/closed circuits, atomic ACL-protected state, corrupt-state quarantine, observe-only mode, verification, rollback, and shutdown cancellation.
- Registered explicit ownership/security observer policies and connected Supervisor crash events after Supervisor readiness. SelfHeal stops accepting work before capture/network shutdown; it never restarts a core in parallel with Supervisor.
- SelfHeal focused tests cover unknown-code rejection, observe-only non-mutation, verification-failure rollback, circuit opening, hashed state, duplicate storms, and cancellation. Focused SelfHeal/Service/Supervisor/Agent/Wails/launcher tests pass.
- Added current-route layered latency without route/Capture mutation: real endpoint TCP, proxy CONNECT/handshake timing, TLS, TTFB, total request and proxy exit-IP confirmation. Remote DNS inside the core is explicitly marked unobservable.
- Added two traffic-simulation modes: deterministic labeled chart-only preview isolated from the real 30-sample ring, and user-triggered/cancellable bounded 1-32 MiB real transfer through the active local proxy. Tooltip now defaults to the latest sample and supports pointer, touch and keyboard navigation.
- Renamed the user-facing pages to `一键测速` and `升级内核`; core status now has explicit state/cache and blocks install when no trusted update-asset SHA-256 exists instead of performing an unverified download.
- Fixed the Dashboard uptime contract (`uptime_seconds`) and replaced the plaintext Geo lookup with an HTTPS nested-response parser. Focused IP/Wails tests pass.
- Added real frontend tests using Node's built-in test runner, and changed packaging to run Go tests/vet plus frontend test/typecheck/build before touching release output. Added a Windows GitHub Actions CI gate.
- Removed `repair.exe` fake recovery that overwrote `DIRTY_SHUTDOWN` with NORMAL or deleted the journal without cleanup. Offline repair now reports the issue and delegates mutation to the sole runtime Reconciler. Launcher bootstrap config and diagnostics export now use `fsatomic`.
- Removed stale raw lifecycle IPC constants and updated smoke shutdown to exercise the real native-tray Exit callback instead of the forbidden ordinary `service.shutdown` surface. Removed unavailable repair subcommands and misleading automatic-repair help text.
- Final isolated package gate passed: full Go test/vet, frontend tests/typecheck/build, Wails Windows/amd64 build, package assembly and independent SHA-256 verification. Packaged `repair.exe check` returned zero issues.
- Browser-visible feature smoke passed for layered latency, four-source traffic and tooltip interaction, core-update trust blocking, and SelfHeal structured logs with zero page/console errors.
- Replaced public Wails DTO `time.Time` fields with RFC3339Nano strings; regenerated bindings no longer degrade timestamps to `any` or emit `Not found: time.Time`.
# 2026-08-02 Phase 34 TUN one-pass closure

- Read the planning-with-files workflow and restored the existing repository plan.
- Added Phase 34 for the supplied TUN closure guide.
- Loaded historical Navo TUN acceptance constraints; all drift-prone runtime and binary facts will be rechecked.
- Read the complete TUN one-pass guide and captured its required file set, state ordering, error/rollback semantics, and elevated acceptance gates.
- Confirmed the current branch is `main`; pre-existing `.claude/` and the guide are untracked inputs and will not be modified.
- Inventoried all required network, TUN, Service, compiler, adapter, and test files and located the current ownership/commit/route-inference call sites.
- Began full source/test reading; confirmed Journal V1, caller-context rollback, late route inference, alias-based split routes, and shallow adapter observation are active production behaviors.
- The guide's expected `internal/service/capture_transition_test.go` is absent; recorded this and will add the missing responsibility-level tests instead of assuming coverage.
- Implemented the first production slice: typed activation plan/error/stage models, Windows physical-route freezing, detailed adapter readiness, control-plane verification, Journal V2 resource model, independent rollback context, planned/idempotent Manager operations, pinned endpoint compilation, Service data-plane verifier, and delayed health commit.
- First full Go gate reached compilation. Network tests require migration to the new interfaces; the old Service AST test still looks for the removed shallow adapter wait. Separate managed-host failures occurred in a compiler temp path and one Named Pipe timeout test.
- Replaced the obsolete Manager tests with V2 ownership/idempotency/rollback/tamper coverage and updated the Service AST guard to enforce owner -> start -> activate/control verification -> data verification -> health commit.
- With repository-local Go caches and an isolated TEMP, `go test ./internal/network ./internal/service` passes.
- Added activation-plan tests for literal IP, multi-A metric selection, invalid/IPv6 filtering, missing route, and direct mode; added endpoint identity preservation and no-proxy/UDP verifier tests.
- Disabled the legacy route mutation owner and localized `route print` parser; Reconciler now leaves unowned routes/DNS untouched.
- Targeted `network`, `network/tun`, `service`, `compiler`, and `coreadapter` packages all pass with isolated caches/TEMP.
- Added opt-in production failure/crash injection points gated by `NAVO_TUN_TEST_MODE=1`; in-process failures roll back, while crash points leave V2 evidence for next-start recovery.
- Added `scripts/test-tun-elevated.ps1` and the opt-in `manager_windows_integration_test.go` real-machine entry point; the script passes PowerShell parser validation.
- Full `go test ./...` and `go vet ./...` pass with isolated TEMP and repository-local caches.
- Frontend `npm ci`, `npm test`, `npm run typecheck`, and `npm run build` all pass; Vite built 19 modules.

## 2026-08-03 TUN usability incident

- Restored Phase 34 planning files and historical TUN constraints.
- Confirmed the repository has substantial uncommitted Phase 34 work and recent elevated acceptance artifacts; no unrelated user changes will be discarded.
- Began correlating the latest acceptance JSON, live Windows network state, running processes, and packaged binary timestamps.
- Error: the first read-only PowerShell artifact summary used an invalid pipeline after `foreach`; next attempt uses an explicit result collection.
- Live residue checks show no Navo network residue. Package correlation failed because `release\Navo` no longer exists; next step is to locate the actual output/installed binary and inspect the latest JSON statuses without table truncation.
- Artifact correlation: latest sing-box and Mihomo golden runs pass real TUN data plane and rollback against an isolated clone of the user profile, but the later repeat activation failed once because the local proxy connection was forcibly closed. Subsequent TUN/proxy/TUN, TUN/off/TUN, adapter-disable, and core-crash cases pass.
- Error: the installed-package comparison repeated the invalid `foreach { ... } |` PowerShell form. Further summaries use pipeline `ForEach-Object` or an explicit array only.
- Confirmed the installed Program Files binaries are stale July 29 builds. Current Phase 34 package integrity is clean (`repair.exe check`: zero issues; independent manifest hash issues: zero).
- Read production TUN verification/transition code and current package logs. Normal activation is slow but completes; the adapter-disable case exposed a transition-context exhaustion path during credential lookup and rollback.
- Traced the exhaustion to `rollbackCaptureLocked` compiling an unused off-mode core config after network/core/adapter cleanup. Planned fix: persist disabled TUN runtime state without credential/config compilation, plus regression coverage.
- Implemented idempotent same-mode capture requests, credential-free emergency TUN rollback state reset, and transient Wintun disappearance handling.
- Added regressions for healthy TUN no-op and rollback runtime reset; formatted only touched Go files. `go test ./internal/agent ./internal/service -count=1` passes.
- Rebuilt `release\Navo`; full Go test/vet, frontend test/typecheck/build, Wails production build, `repair.exe check`, and independent payload hashes all pass.
- Built MSI and bootstrapper upgrade installers as version 1.0.1. MSI SHA-256 is `dd84caadd25a5e6f4cbf5e74140b287325758757221a3f8a28c5286bccc3d983`; setup SHA-256 is `5409afb8a416130e5d98a9e6a9623064ea15f80cb55823a9d1b1f0a1aa98b1bc`.
- First silent major-upgrade attempt returned MSI 1603. Verbose evidence identifies Windows Installer 1730: the per-machine 1.0.0 uninstall requires an explicitly elevated token. The old installation remained intact.
- Explicit UAC `RunAs` upgrade completed with MSI exit code 0.
- Verified installed version 1.0.1, exact launcher/UI hash equality with the release payload, and installed repair health. Revalidated v2rayN `127.0.0.1:10808` as both working HTTP and SOCKS5 proxy endpoints.
- Installed proxy-backed golden acceptance reproduced `TUN_PHYSICAL_ROUTE_NOT_FOUND` for `127.0.0.1`; exact rollback passed.
- Fixed activation planning for explicit IPv4 loopback upstreams: pin the local endpoint without an endpoint-bypass route while still freezing the real physical egress for split-route verification. IPv6 loopback remains rejected under IPv6-block mode.
- Added loopback allow/reject regressions. `go test ./internal/network ./internal/service ./internal/agent -count=1` passes.
- Built and installed Navo 1.0.3; full Go/frontend/Wails/package/installer gates passed and installed launcher hash matched the release payload.
- Cleared a stale Navo NRPT rule that redirected the global namespace to `172.19.0.2`; Windows DNS and direct HTTP recovered with no split-route or process residue.
- Installed sing-box golden acceptance passed against an isolated clone of the real profile: `HEALTH_COMMITTED`, DNS/TCP/HTTPS true, proxy exit `165.254.151.219`, direct exit `39.174.81.62`, and exact rollback.
- Lifecycle evidence passed TUN-off-TUN and Navo-owned core crash, but repeat, TUN-system-proxy-TUN, and adapter-disable exposed an Agent health-monitor race returning `CAPTURE_BUSY` or removing the adapter before the requested transition.
- Replaced immediate concurrent rejection with context-aware serialized capture locking; TUN ongoing health now trusts the Service-owned adapter/fail-closed monitor instead of racing transient Supervisor state. Focused Agent/Service/network/fsatomic tests pass.
- Navo 1.0.4 full Go/frontend/Wails/package/installer gate passed. MSI SHA-256 is `35975649f1b4aa9cb62a586d016e9090901cc99fe4599b5547b55b71e0dd041e`.
- Two explicit UAC install attempts were canceled after 120 seconds; a non-elevated direct MSI attempt logged Windows Installer 1603 and did not upgrade. Installed version remains 1.0.3, with zero Navo process or NRPT residue.

## 2026-08-08 delayed TUN restart incident

- Restored the Phase 34 plan and historical acceptance constraints using the planning-with-files workflow.
- Confirmed the user still runs installed 1.0.3 while 1.0.4 exists only in `release\Navo`; no current Navo network/process residue exists.
- Confirmed the interactive user is denied access to `%LOCALAPPDATA%\Navo`; next step is an elevated sanitized log/ACL export followed by a full identity and health-owner audit.
- Added `scripts/export-tun-incident.ps1`, an administrator-only, read-only profile/network evidence exporter with credential redaction; PowerShell parser validation passes.
- Elevated profile export was canceled by UAC after 120 seconds; recorded and switched to the accessible installed package log instead of repeating the same attempt.
- Installed log proves successful data-plane commit is followed seconds later by Supervisor stop and Agent secondary recovery; source mapping confirms duplicate Agent/Service TUN health ownership drives the restart modal.
- Audited the Windows adapter observer and Service status path. Found a single-sample destructive monitor with a TOCTOU recheck gap plus two unsynchronized reads in `tun.status`; these are now the primary production fix targets.
## 2026-08-08 Phase 35 implementation

- Confirmed Supervisor and SelfHeal are not competing TUN recovery owners; SelfHeal is observer-only and Supervisor owns only core crash restart.
- Added Service-side activation session identity, consecutive confirmed-failure tracking, non-destructive handling for ambiguous adapter states, and lock-held reinspection before rollback.
- Reworked `tun.status` to use one native adapter observation for state/GUID/index and to reuse committed address/MTU metadata only when identity matches.
- Removed Agent-side TUN rollback from the health loop. Agent now polls only `tun.status`, mirrors a Service-issued `fault_id`, and marks the committed mode off after Service has rolled back.
- Added regression coverage for consecutive same-session confirmation, transient/unknown-state immunity, adapter identity mismatch, and prevention of a second Agent rollback.
- `go test ./internal/service ./internal/agent` passed with repository-local caches after the ownership/debounce changes.
- Replaced process-SID-only file DACL construction with exact SID preservation for user owners discovered along the destination path ancestry; SYSTEM and Administrators remain, while Users/Everyone/Authenticated Users remain excluded.
- Added an early launcher profile ACL migration before privacy/state/log initialization. It repairs existing protected Navo state recursively, skips the disposable WebView2 cache, and prevents older elevated builds from keeping the interactive profile owner locked out.
- Targeted Windows gate passed: `go test ./internal/fsatomic ./internal/service ./internal/agent ./cmd/navo`.
- First full repository gate reached all packages; product changes passed, while `compiler` used the protected host TEMP and one Named Pipe close test hit its known availability race. A corrected isolated-TEMP rerun is required.
- Corrected full repository Go test/vet gate passed with `TEMP` and `TMP` pinned to `.cache/tmp`; the prior Named Pipe race did not recur.
- Frontend `vue-tsc` and Vite production build passed.
- Targeted Go race validation is toolchain-blocked: Windows amd64 and CGO were pinned, but no GCC/Clang/MinGW compiler is installed. Normal full Go tests/vet remain green.
- Complete `scripts/package.ps1` gate passed: all Go tests/vet, frontend tests/typecheck/build, Wails Windows/amd64 build, and release assembly completed at `release/Navo`.
- `npm ci` reported one high-severity dependency advisory; this was not auto-fixed because an unreviewed dependency mutation is outside the TUN remediation and may introduce runtime incompatibility.
- Release integrity check passed with zero issues. The packaged smoke harness cannot cross the managed desktop's `RunAs` UAC boundary (`0xc0000142` before process creation, twice); no Navo process or isolated-profile residue was created, so this is not a product/TUN failure and requires the explicit installation/UAC path.
- Built and successfully installed Navo 1.0.5 through approved UAC. Windows Installer reports 1.0.5; installed launcher/UI SHA-256 exactly match `release/Navo`, and installed `repair.exe check` reports zero issues.
- Installed 1.0.5 launched successfully under UAC: launcher, UI, Service, Agent, tray, WebView2, and deferred-off startup all reached ready state. The remaining ACL/data-plane checks must run at the elevated desktop identity because the managed shell is intentionally a different sandbox SID.
- Elevated sanitized export confirms the profile owner is `YUNHE\28484` and the protected DACL contains exact full-control ACEs for SYSTEM, Administrators, and SID ending `-1001`; the profile ACL migration succeeded without broad grants.
- Off-state export found a lost-journal Navo IPv6 firewall rule. Added startup/activation/deactivation orphan recovery that removes only rules matching Navo's reserved session-name, outbound/block direction/action, and exact `::/1` plus `8000::/1` address shape; ambiguous rules fail closed.
- Targeted network/Service/Agent/fsatomic/launcher tests pass after orphan recovery. A hidden elevated tray-exit helper timed out and left 1.0.5 running; workspace release files are unaffected, so the final rebuild can continue safely.
- Rebuilt the final release after orphan-firewall recovery; the complete Go/vet/frontend/Wails/package gate passed again.
- Built MSI 1.0.6 (SHA-256 `2245b04fa5fbefda54927dc82f6a061b18811f4bd0486f494d982ed173a30ee8`) and upgraded through UAC/Windows Installer after it closed 1.0.5. Installed version is 1.0.6, launcher hash matches the release, and installed repair check is clean.
- Windows still has the previously launched 1.0.5 PID 12836/UI PID 22220 mapped after the upgrade. Two bounded UAC/taskkill attempts and a three-minute background waiter were not approved/completed; final 1.0.6 launch and real TUN stability/data-plane acceptance are blocked on the desktop user exiting that old tray instance or approving UAC.

## 2026-08-09 TUN external-site usability follow-up

- Restored Phase 35 planning files and the Navo Windows release acceptance rules.
- Added explicit live Google/GitHub DNS, TCP and HTTPS data-plane gates plus delayed stability and rollback/re-enable acceptance.
- Preserved the substantial existing Phase 34/35 worktree; no historical changes were discarded.
- Confirmed installed version 1.0.6, no active Navo process, and disabled current-user WinINet. Preserved the unrelated v2rayN-owned sing-box process.
- Non-elevated adapter/route/DNS inspection is denied for the managed shell; elevated acceptance remains required for authoritative live state.
- Audited the production data-plane verifier and acceptance script: both lack Google/GitHub probes, and historical golden runs include transient HTTPS EOF/no-exit-change failures.
- Identified direct DNS selection as the primary code-level hypothesis: sing-box uses `223.5.5.5` and Mihomo uses host DHCP DNS even while target traffic is proxied.
- Verified the intended fix against official sing-box/Mihomo DNS schemas: proxy-routed DoH is supported by both bundled-core configuration models.
- Implemented proxy-routed DoH defaults: sing-box uses modern HTTPS DNS detoured through the selected outbound; Mihomo sends target DNS through `#NAVO` while retaining DHCP only for bootstrap/direct resolution.
- Added Google and GitHub DNS/TCP/HTTPS checks to the Service-owned TUN commit gate and the elevated acceptance script; direct mode remains exempt from proxy-only external-site expectations.
- Added compiler and verifier regressions for proxied DNS configuration and explicit Google/GitHub coverage.
- Formatted only the touched Go files. The elevated PowerShell script passes parser validation.
- Targeted `go test ./internal/compiler ./internal/service -count=1` passed with repository-local caches and isolated TEMP.
- Bundled sing-box and Mihomo both accepted the new proxied-DNS TUN configs through their native validators. `git diff --check` is clean.
- The first full Go gate reached all packages; only `TestWriteTempConfig` failed because `scripts/test.ps1` still inherited the protected host TEMP. Updated the script to use repository-local `.cache/tmp`, matching packaging.
- Corrected full `scripts/test.ps1` gate passed: all Go tests and `go vet ./...` completed successfully.
- Complete `scripts/package.ps1 -OutputName Navo` gate passed: Go test/vet, frontend tests/typecheck/build, Wails Windows/amd64 production build, and package assembly.
- Independent SHA-256 manifest verification and packaged `repair.exe check` passed with zero issues. A follow-up summary command only failed after those gates because it assumed the UI executable was at the release root; the manifest is authoritative for layout.
- Elevated release acceptance passed for sing-box and Mihomo against an isolated clone of the real user profile. Both cores passed Google/GitHub DNS, TCP and HTTPS, exit-IP equivalence, and exact rollback.
- Added a bounded `StabilitySeconds` acceptance option plus saved delayed/re-enabled evidence. A 30-second sing-box `tun-off-tun` run passed initial, delayed and second-activation Google 204/GitHub 200 checks and left no Navo process or network residue.
- Built MSI and one-click setup version 1.0.7. MSI SHA-256 is `171d04647f555c26c6e600c6d42febc68e7af472858951e483c898515fc6e050`; setup SHA-256 is `4c5459c43ac2498a6c57acff01856b38c841d5f3a816de2d7df67d41e64d3b04`.
- Upgraded through UAC successfully. Windows Installer reports 1.0.7, the installed payload matches every release manifest hash, and installed repair health reports zero issues.
- The first installed acceptance wrapper exited before script startup because `C:\Program Files\Navo` was not quoted through `Start-Process -ArgumentList`; no new artifact or Navo process was created. The retry uses explicit argument quoting.
- Installed 1.0.7 sing-box golden acceptance passed with a 30-second delayed health check, Google 204/GitHub 200 initially and after delay, matching TUN/proxy exit IPs, exact rollback, and no residue.
- Installed Mihomo `tun-off-tun` reached `HEALTH_COMMITTED` initially, after 30 seconds, and after re-enable; Service recorded Google 204/GitHub 200 each time and exact rollback passed. The script status was failed only because its additional duplicate HTTP request transiently errored after re-enable.
- Hardened the independent external-site acceptance probe with three bounded attempts separated by one second; production Service verification remains single-pass and fail-closed.
- The rerun exposed a real production readiness edge: Mihomo second activation received Cloudflare HTTPS `EOF` on its first verification request and correctly rolled back. Added bounded Service-side retries for read-only Cloudflare and external-site probes; network mutation is never retried and exhaustion still rolls back fail-closed.
- Formatted the verifier changes and passed `go test ./internal/service -count=1`, including transient-EOF retry coverage.
- Logged and abandoned one malformed recursive config-inventory regex after it timed out; subsequent inspection is bounded by known acceptance directories and extensions.
- The first bounded inventory command hit the already-known PowerShell empty-pipeline parser pattern after `foreach`; switched to an explicit result array.
- Rebuilt the final release after bounded production readiness retries; the complete Go/vet/frontend/Wails/package gate passed again.
- Built and installed Navo 1.0.8. MSI SHA-256 is `8f99a003c3f2868927e2f8dd03558cfb59221f8d13a28476a6b3a6b48db9ce88`; one-click setup SHA-256 is `57ad8916223ff37cce08f96f7f687ff36289fb40d1e6bc99f637d5931aec4946`.
- Windows Installer reports 1.0.8; all 24 installed manifest entries match the release and installed `repair.exe check` reports zero issues.
- The first final installed Mihomo lifecycle run recorded three successful Service-owned `HEALTH_COMMITTED` Google/GitHub checks but failed its redundant PowerShell probe after route churn because a shared `HttpClient` reused stale pooled sockets. Changed the independent verifier to create a fresh transport for every bounded attempt; parser validation passed.
- Final installed Mihomo `tun-off-tun` acceptance passed: initial, 30-second delayed and re-enabled stages returned Google 204/GitHub 200, TUN and proxy exit IP `165.254.151.219`, direct IP restored to `211.90.237.75`, exact rollback, and zero network residue.
- Final installed sing-box `tun-off-tun` acceptance passed with the same three-stage Google/GitHub, exit-IP, rollback and zero-residue evidence.
- Final user-state audit: current-user WinINet is disabled and no executable under `C:\Program Files\Navo` remains running. The final release report is recorded in `docs/TUN_ACCEPTANCE_REPORT.md`.

## 2026-08-10 Phase 37 system/global proxy routing

- Restored the existing multi-phase plan and preserved the substantial dirty worktree from Phases 34-36.
- Started an audit of capture-mode ownership, core routing generation, UI mode controls, and real foreign-site data-plane acceptance.
- Confirmed the backend already has validated `rule`, `global`, and `direct` routing transactions, but the UI omitted them and startup forcibly discarded saved routing choices.
- Added an explicit routing-mode persistence marker and a safe legacy migration to smart routing; explicit future choices survive restart.
- Added a typed Vue routing-policy control next to capture scope, renamed TUN's user-facing label to Global Proxy, retained TUN terminology in its explanation, and reused existing async/rollback feedback.
- Targeted Go tests passed for `internal/service` and `navo_app`; the first frontend command did not execute because PowerShell blocked `npm.ps1`, so it is being rerun through `npm.cmd`.
- Frontend unit tests, Vue typecheck, and production build passed through `npm.cmd`.
- User refined the required routing selector to four choices. Browser QA is paused while the backend policy model is corrected; the temporary direct-mode UI will not be delivered.
- Implemented `bypass_mainland`, `global`, `blacklist`, and `whitelist` as ordered core-neutral policies; blacklisted TUN destinations use proxy-routed DoH, and upstream selection no longer resets the policy to global.
- Focused Go tests, frontend tests/typecheck/build, browser-visible desktop/narrow UI smoke, and the full Go/vet gate pass.
- The first package run stopped at `npm ci` because one Navo Vite Node process retained the exact Rolldown native binding; release assembly has not yet passed.
- Resumed Phase 37 from the persisted plan on 2026-08-10. Installed/running desktop state is still Navo 1.0.8; the next unfinished gate is precise removal of the workspace Vite/Rolldown lock followed by package, install, and real routing-policy data-plane acceptance.
- The Rolldown lock is gone. The resumed package passed all Go/vet and frontend tests/typecheck/build, then failed only while replacing `release/Navo/app_ui/navo_app.exe`; the exact image owner must be identified before a bounded shutdown.
- Added a read-only Restart Manager lock inspector. Outside the managed registry sandbox it identified release UI PID 19680 as the exact image owner; no guess-based process cleanup is required.
- The mapped release UI process exited on its own. The clean rerun passed the complete Go/vet, frontend tests/typecheck/build, Wails Windows/amd64 build, and release assembly gates; `release/Navo` is ready for the Phase 37 installer.
- Built and installed Navo 1.0.10 through UAC. MSI SHA-256 is `1f938e0443c1ece5d40bd5be61ca0858bf4ae5413cab91695ad6eadc4e712d43`; installed launcher hash matches the release and installed repair health reports zero issues.
- First installed sing-box `routing-modes` run failed initial bypass-mainland TUN with `TUN_DNS_VERIFY_FAILED` on the system resolver. Rollback passed and left zero Navo resources; the failure is retained for root-cause analysis before any matrix rerun.

## 2026-08-09 Phase 36 dashboard and diagnostics adjustment

- Restored the completed Phase 35 state and current Wails/Vue/backend contracts using the planning workflow.
- Applied the UI/UX review workflow: preserve existing tokens and typography, use a dense monitoring hierarchy, provide crosshair/tooltip keyboard parity, visible async states, and reduced-motion-safe interaction.
- Audited overview, connection, one-click test, core upgrade, traffic monitor and dual-link pages plus their Wails and Service handlers.
- Confirmed the five user reports map to concrete implementation gaps rather than missing navigation: automatic traffic source selection, chart interaction geometry, page responsibility split, absent core install transaction, and unconditional/stale proxy IP detection.
- Overview and Traffic Monitor now choose physical-interface counters only when capture is off and core counters for system proxy/TUN; IP risk follows the active link.
- Traffic charts now expose pointer/keyboard sample selection, crosshair lines, point markers and a pointer-adjacent overlay.
- Connection Management owns the page-global source type and airport/upstream source management; One-click Test contains diagnostics only.
- Core updates now use fixed official repositories, exact Windows amd64 asset names, GitHub SHA-256 release digests, bounded download/extraction, executable version validation, active-core stop/start, atomic payload replacement and rollback with capture restoration.
- Dual-link detection now bypasses stale mode-sensitive cache, reports inactive proxy state without failing direct detection, and exposes independent availability states.
- Targeted Go packages, frontend tests/typecheck/build and a browser-visible six-page smoke passed; screenshots are under `artifacts/phase36-ui`.
- Browser tooling note: the skill server helper passed, but cached Python Playwright had an incompatible native `greenlet`; the same bundled Playwright Node runtime completed the smoke without installing dependencies.
- Full `scripts/test.ps1` passed across all Go packages and vet. The final package pipeline passed Go/frontend/Wails Windows amd64 gates after precisely stopping stale workspace Vite processes and one elevated old Navo process that held the prior release log.
- `release/Navo` passes `repair.exe check` with zero issues and independent manifest verification with zero hash mismatches.
- Built Navo 1.0.9 installers: MSI SHA-256 `58c51b7dcc0e672fca9458b84197356e9444fd6559d67171ac92df8637c9c90b`; one-click EXE SHA-256 `3af9c4d6e3fa58e4924ac076ffbf01b6b39e4adbac48bda5b3372e5df4c62dba`.
- Installed 1.0.9/UAC and a new real Google/GitHub TUN lifecycle run were not executed in this phase; the previously accepted installed 1.0.8 process was stopped to release its workspace log lock.

## 2026-08-10 Phase 37 final routing acceptance

- Delivered the requested routing choices: 绕过大陆, 全局, 黑名单, 白名单. The UI persists the selection and no longer forces global mode during startup.
- Added core-neutral ordered policies, proxy-routed blacklist DNS, endpoint-pin preservation, Supervisor steady-state coordination, and fail-closed Google/GitHub verification with automatic policy rollback.
- Focused tests, full Go/vet, frontend unit tests, Vue typecheck/build, Wails Windows/amd64 packaging, desktop/narrow browser smoke, `repair.exe check`, and 24-entry package SHA-256 verification passed.
- Packaged sing-box and Mihomo both passed 8/8 combinations across TUN and System Proxy. Every mode returned Google 204 and GitHub 200 through the local proxy.
- Both cores changed direct exit `211.90.237.75` to proxy/TUN exit `165.254.151.219`, restored the direct exit after disable, and left zero route, NRPT, firewall, adapter, or package-process residue.
- Preserved the failed intermediate artifacts: endpoint DNS loopback, Supervisor restart-window rejection, and one Mihomo cold-start timeout. Each failed transaction rolled back cleanly; the corrected/retried final artifacts passed.
- Reopened acceptance because the later installed 1.0.10 artifact reproduced `TUN_DNS_VERIFY_FAILED`; packaged evidence alone is not final proof.
- Added bounded cold-start system-resolver readiness retries and regression coverage. `go test ./internal/service -count=1` passed.
- Installed 1.0.11 proved the local-proxy test topology loops under TUN; switched final acceptance to the user's actual remote upstream.
- Installed 1.0.11 sing-box real-profile `tun-off-tun` passed. Mihomo then reproduced `CORE_004` on re-enable twice, after its initial and delayed Google/GitHub checks had passed.
- Increased live-core listener readiness from 10 to 30 seconds, passed targeted and full Go/vet/frontend/Wails gates, built and installed 1.0.12.
- Installed 1.0.12 Mihomo re-enabled to `HEALTH_COMMITTED` in two runs with Service Google 204/GitHub 200; the redundant PowerShell probe transiently failed after commit, and exact rollback/zero residue passed.
- Installed 1.0.12 sing-box full lifecycle passed including initial, 30-second delayed, re-enabled external traffic and final rollback. Installer repair check reports zero issues and no installed Navo process remains.

## 2026-08-11 Phase 38 capture and policy correction

- Added Phase 38 to the persistent plan.
- Restored the latest Phase 37 source/package/installed acceptance history and preserved the dirty worktree.
- Generated the required desktop network-utility design-system review and started the current source audit.
- Confirmed the existing UI conflates TUN capture with global routing and exposes fixed, non-editable blacklist/whitelist policies.
- Located the System Proxy conflict: Navo does not suppress PAC/WPAD while owning the manual proxy and has no post-WinINet transaction probe.
- Defined the rule-setting implementation boundary as user-owned domain suffix/CIDR lists shared by both capture modes.
- Visually inspected the existing desktop and narrow Phase 37 screenshots and recorded the equal-column stretch as the page-proportion root cause.
- Completed the Service/Agent/Wails/Vue contract trace for runtime status and capture transitions; the existing dashboard path can carry user rules without adding polling or ownership layers.
- Implemented PAC/WPAD suppression with exact WinINet snapshot restoration and post-mutation ownership verification.
- Added normalized user blacklist/whitelist persistence, domain/CIDR compilation, typed Wails/Vue transport, runtime verification, and rollback.
- Rebuilt the connection page as two independent axes and moved `off` to a separate stop action; added the inline user-rule editor.
- Phase 38 targeted Go tests passed for Agent, System Proxy, Service, and Wails. Frontend unit tests, typecheck, production build, and `git diff --check` passed.
- Edge headless interaction smoke passed at 1280x900 and 760x900 with two capture choices, four policies, saved CIDR rule, no page errors, and no horizontal overflow. Screenshots are under `artifacts/phase38-ui`.
- Serial full repository Go tests and `go vet ./...` passed. The first package attempt then reached only the known Vite/Rolldown file lock; two exact smoke-owned Node processes were removed and no binding owner remains.
- The clean package rerun passed all Go/vet, frontend test/typecheck/build, Wails Windows/amd64, and release assembly gates. `release/Navo-phase38` passes `repair.exe check` with zero issues and all `SHA256SUMS.txt` entries match.
- Strengthened the elevated routing acceptance to prove customer rule mutation, WinINet ownership, PAC/WPAD suppression, default-system-proxy application traffic, TUN no-proxy application traffic, and exact WinINet rollback across both capture methods and all four policies.
- The first real application-flow TUN run caught a production gap hidden by Phase 37 explicit-proxy probes: policy reload restarted Wintun, but split routes could remain bound to the previous interface identity. The run failed closed, restored all network resources, and preserved the evidence under `artifacts/phase38-acceptance`.
- Added `network.Manager.Rebind`, live-core reload route/DNS ownership refresh, rollback rebind, and TUN-native data-plane verification for routing mode/rule commits. Network/Service/Agent/Wails targeted tests and the complete package pipeline passed; `release/Navo-phase38` was rebuilt successfully.
- Real System Proxy application acceptance passed all four policies. WinINet moved from the protected v2rayN proxy `127.0.0.1:10808` to Navo `127.0.0.1:12080`, Google returned 204 and GitHub 200 through default WinINet applications in every mode, blacklist/global/bypass used exit `165.254.151.219`, whitelist-listed IP probes used direct exit `211.90.237.75`, and the exact original WinINet state was restored.
- v2rayN protection passed: its sing-box PID 18704 and listener 127.0.0.1:10808 remained unchanged. Final residue check found zero Navo processes, listeners, split routes, NRPT rules, firewall rules, or adapters.
- The corrected elevated TUN matrix is the only remaining acceptance item; the current execution policy rejects new UAC launches, so no post-fix elevated data-plane claim is recorded.
# 2026-08-11 Phase 39 icon unification

- Restored the existing plan and prior release/package context.
- Applied the UI icon consistency review: one canonical brand asset, preserved proportions, no guessed or mixed logo variants.
- Started source/resource inventory before mutating the portable artifact.
- Extracted and visually compared the canonical tray asset and current portable executable icon: blue ribbon versus old Wails W.
- Replaced every N-shaped `StateGlyph` rendering and the frontend favicon with the canonical tray asset.
- Synchronized the Wails Windows ICO and regenerated the 1024px Wails app icon from the canonical 256px ICO frame; visual QA passed.
- Frontend unit tests (3/3), Vue typecheck, Vite production build, and Wails Windows/amd64 production build passed.
- Browser smoke passed at 1280x900: three canonical tray glyphs, zero legacy N-shaped SVGs, zero page errors; screenshot saved under `artifacts/phase39-icon/overview.png`.
- Replaced `release/Navo-1.0.12-portable-x64/app_ui/navo_app.exe`, updated `SHA256SUMS.txt`, verified all manifest entries with zero mismatches, and passed packaged `repair.exe check` with zero issues.

# 2026-08-11 Phase 40 core-update download timeout

- Restored the persistent Phase 36-39 context and inspected `navo_app/core_update.go` plus existing updater tests.
- Root cause confirmed: the trusted updater uses the default Go transport and therefore bypasses the active Windows/Navo proxy when no `HTTP_PROXY` or `HTTPS_PROXY` environment variable is present.
- Confirmed the official asset redirects to an already-trusted GitHub release-assets host; no source or digest relaxation is required.
- Started a focused transport-only repair. Existing user changes remain preserved.
- Implemented loopback-only Navo/WinINet proxy discovery, endpoint deduplication, environment/direct fallback, fresh bounded transports, and read-only current WinINet inspection.
- Added regression coverage for route priority/deduplication, unsafe proxy rejection, fresh-transport fallback, and GitHub release-assets redirect trust.
- The first focused test did not compile because Go attempted the protected user build cache; the next run uses repository-local caches.
- Repository-local focused tests passed: `go test ./navo_app ./internal/agent/systemproxy -count=1` (`navo_app` 1.497s, `systemproxy` 0.815s).
- The first full gate passed the updater packages but three unrelated Reconciler fixtures collided with the live port 12080. Replaced the hard-coded test port with a dynamically reserved loopback port; no process was stopped.
- The complete Go/vet gate then passed. Isolated packaging passed Go, frontend tests, Vue typecheck, and Vite build before a pre-existing `runtime.go:200` duplicate `:=` stopped Windows launcher compilation; applied the one-token declaration correction.
- The corrected isolated package completed at `release/Navo-phase40`; packaged repair reports zero issues and all 24 manifest hashes independently match.
- Current read-only state: loopback ports 12080 and 10808 accept TCP connections, while WinINet is disabled. A curl HEAD probe hit the host's Schannel credential error, so live validation is continuing through the updater's own Go HTTP stack.
- Opt-in live updater test passed through `127.0.0.1:12080`: official sing-box 1.13.18 archive downloaded completely (21,042,668 bytes), SHA-256/size matched GitHub metadata, and the expected executable extracted successfully. No runtime payload was mutated.
- Final `scripts/test.ps1` passed all Go packages and `go vet ./...` after the live-test addition; final `git diff --check` passed.

## 2026-08-11 Phase 42 installed updater closure

- Confirmed the retry ran installed Navo 1.0.12: installed UI hash `7194...` differs from Phase 40 `a097...`, and installed sing-box remains 1.13.14.
- Added a validated `PayloadName` option to the MSI builder so an isolated, fully verified release can be installed without copying it over `release/Navo`.
- Built `release/Navo-phase42` from the latest Phase 40+41 source; all Go/vet/frontend/Wails/package gates passed. Built MSI 1.0.13 with SHA-256 `80328a7fc43f863062cc293e411f291754534719485649c99f10cfc63f4dc6d4`.
- Installed 1.0.13 through UAC. Installed UI hash exactly matches the isolated release and packaged repair reports zero issues.
- MSI left the pre-install elevated Navo process mapped. Graceful tray access and normal `Stop-Process` were denied, so UAC terminated only the verified Navo launcher PID 21576 and its job child; v2rayN was not targeted.
- Launched installed 1.0.13 through UAC. New launcher PID 5996 and UI PID 9220 have post-install start times. Port 12080 is currently closed, so the remaining acceptance action is to connect a node and trigger the installed sing-box update.
## 2026-08-11 Phase 41: Network privilege-boundary audit

- User reported intermittent privilege overreach that can disable Ethernet.
- Started a bounded audit of all adapter, route, DNS, firewall, and system-proxy mutation paths.
- Safety invariant: Navo may mutate only explicitly owned proxy/TUN resources; physical Ethernet and Wi-Fi adapters must remain untouched.
- Located the unsafe boundary: arbitrary persisted/IPC `TUNName` values flowed into privileged adapter lifecycle and network planning.
- Added canonical adapter ownership enforcement at state loading, IPC handlers, runtime compilation, adapter cleanup, network config validation, and activation-plan construction.
- Added regressions for poisoned `runtime_state.json`, `tun.enable`, `tun.config`, and activation plans targeting physical adapter names.
- First focused test reached only dependency setup because it used the wrong local module-cache directory; no code test ran. Switched to the established `.cache/go-path` cache.
- Strengthened adapter readiness from name/address/MTU matching to explicit Wintun driver and non-hardware identity checks before any split-route binding.
- Focused `internal/network` and `internal/service` tests plus vet passed.
- Full `scripts/test.ps1` passed every Go package and `go vet ./...`; production-source forbidden adapter-admin mutation scan returned `NONE`.
- Live `Get-NetAdapter` verification was blocked by the current non-elevated token (`Access denied`). Source closure is complete; the installed binary was not replaced.

## 2026-08-11 Phase 43: Close-action confirmation and title-bar alignment

- Restored the existing tray/lifecycle rule: Go/Wails owns the single tray and process lifecycle; a complete exit must use the bounded graceful-shutdown path.
- Preserved the dirty worktree and appended a focused phase instead of replacing prior plans or unrelated changes.
- Started tracing the current Wails close callback, Vue title bar, shutdown bridge, and session-scoped persistence boundary.
- Implemented the Wails close interception, accessible minimize/exit chooser, boot-scoped 24-hour preference, UI-to-Agent launcher-exit request, and fixed-box brand icon alignment.
- The first validation command used the wrong working directory for Go paths and hit the host's `npm.ps1` execution-policy block; the corrected validation uses repository-root Go commands and `npm.cmd` from `navo_app`.
- Frontend tests 6/6 and Vue typecheck passed. Agent and Launcher Go packages passed; the Wails package encountered only a parallel `dist` replacement race and is being rerun after the production build completes.
- Serialized production build and targeted Go tests then passed. The first real browser geometry check reproduced the reported icon defect at 7.05px vertical offset and identified the over-broad `.brand span` selector as the cause.
- Scoped the brand text selector; repeated browser geometry measured `dx=0`, `dy=0`.
- Browser interaction smoke passed minimize, safe exit, remembered exit after UI reload, focus placement, day/night rendering, and zero page/console errors. Screenshots are under `artifacts/phase43-close`.
- Final frontend production build, full `scripts/test.ps1`, explicit `go vet ./...`, Wails 2.12.0 Windows/amd64 production build, and `git diff --check` passed. The temporary Vite listener on 4179 was removed.
- Phase 43 is complete. Existing unrelated dirty-worktree changes were preserved.

## 2026-08-11 Phase 44: Dual-link IP detection repair

- User reported that dual-link IP detection is unusable.
- Restored the existing Phase 36 implementation context and started tracing the Wails/backend/provider/UI path.
- Preserving all unrelated dirty-worktree changes; edits will remain focused on the reproduced failure.
- Reproduced the production detector failure on both direct and proxy transports: all providers returned unavailable even though the same endpoints worked with the default HTTP/2 transport.
- Confirmed the exact error by matching the detector transport: negotiated HTTP/2 frames were handed to the HTTP/1.x parser and rejected as malformed responses.
- Restored HTTP/2 on dedicated per-link transports, added a fast AWS IP echo provider, and close idle sockets before fresh/mode-sensitive checks.
- Focused `internal/ipdetect` tests passed, including a new HTTP/2-only provider regression.
- Real detector probes now pass: direct `211.90.237.75` in 1.36 s and proxy `165.254.151.219` through the existing 10808 proxy in 3.56 s.
- Full repository Go tests and `go vet ./...` passed after serially clearing a transient overlapping-test file lock.
- The first isolated package attempt found two stale Navo UI-smoke Node owners of the Rolldown binding; stopped only PIDs 22572 and 24384, then verified zero remaining owners.
- `release/Navo-phase44` passed Go, frontend tests, Vue typecheck/build, Wails production build, `repair.exe check` with zero issues, and independent SHA-256 verification with zero mismatches.
- Built `release/installer/Navo-Setup-1.0.14-x64.msi`, SHA-256 `dda98ee558c6a0f8999a2addbb436e22ed94d473977214481ceeaef31cf30ce5`.
- Current installed 1.0.13 UI hash is `a097...`; fixed 1.0.14 payload UI hash is `fae6...`. System-level upgrade was not performed because explicit installation approval is required.
- User clarified the standing delivery rule: Navo must always be delivered as a portable green version, not an installer.
- Removed only the newly generated 1.0.14 MSI; preserved historical release artifacts.
- Renamed the verified payload to `release/Navo-1.0.14-portable-x64` and created `release/Navo-1.0.14-portable-x64.zip`.
- Final portable checks: repair issues `0`, manifest mismatches `0`, ZIP entries `26`, required ZIP files missing `0`; ZIP SHA-256 is `3bd71ce27765eb1a4637169c151fcd6fb331810a7e29688c6dd0aec13660201c`.

## 2026-08-11 Phase 45: Minimize-to-tray visibility repair

- Reproduced the version/ownership mismatch from current logs and binary hashes.
- Started a focused repair that keeps the existing sole Launcher-owned tray architecture and makes frontend minimize fail closed unless the Launcher confirms tray refresh.
- Early Shell test interpretations were superseded by line-level inspection: the managed token was rejected during initial icon creation, before refresh logic ran.
- Corrected the integration diagnosis: the managed Go test token is rejected during initial shell registration, before `EnsureVisible`. Production now probes with `Shell_NotifyIconGetRect`, re-adds only when absent, handles `TaskbarCreated`, and keeps the UI visible on any confirmation failure.
- Full repository tests passed. The first green-package attempt reached only the known Rolldown native module lock during `npm ci`; owner inspection is in progress before a single clean rerun.
- Identified PID 10924 as the sole exact Rolldown module owner, stopped only that stale UI-smoke process, and verified zero remaining owners.
- The next package gate encountered the known transient SelfHeal test temp-file sharing violation; isolated repeat validation is running before retrying the package.
- `internal/selfheal` passed 10/10 isolated repetitions; the next complete package gate passed Go tests/vet, frontend tests/typecheck/build, Wails Windows/amd64 build, and release assembly.
- Built `release/Navo-1.0.15-portable-x64` and ZIP. Repair issues: 0; manifest mismatches: 0; ZIP entries: 25; required files missing after separator normalization: 0; no 1.0.15 MSI was created.
- Live portable smoke stopped before launch because installed 1.0.13 PIDs 5996/24076 are still user-owned and running. No existing Navo process was terminated.
# 2026-08-12 Phase 46: UI hierarchy and typography

- Restored the existing plan and current dirty worktree before editing.
- Generated the required desktop operations-dashboard design guidance and Vue/accessibility checks.
- Started a focused App.vue/style.css shell refactor; backend and capture/routing behavior remain out of scope.
- Frontend tests 6/6 and the Vue production build passed. The first browser helper invocation used the wrong npm working directory and never opened its listener; corrected browser validation is pending.
- Implemented three task-oriented navigation groups with per-page descriptions and a matching page-context header. Renamed ambiguous menu labels to “网络测速”, “内核管理”, and “设置与日志” without changing their page IDs.
- Switched Chinese headings and semantic labels to Segoe UI Variable, reserved Cascadia Code for data/identifiers, removed forced uppercase, and raised night/day secondary-text contrast.
- Final browser smoke passed in Edge at 1280x900 night/day and 760x900 compact layouts: 3 groups, 7 pages, no horizontal overflow, page purpose follows navigation, and zero page/console errors. Screenshots are under `artifacts/phase46-ui`.
- Final `git diff --check` passed. Phase 46 is complete; no backend, routing, updater, or lifecycle code was changed.
# 2026-08-12 Phase 47: ChatGPT and Codex usability

- User requires ChatGPT and Codex to work immediately after enabling System Proxy or TUN.
- Restored the Phase 37-46 release, routing, DNS, rollback, and portable-delivery context.
- Started a focused audit of current official OpenAI network requirements and Navo rule/DNS compilation paths.
- Root cause identified: blacklist mode only guarantees `openai.com` and `chatgpt.com`, while official ChatGPT/Codex operation spans additional auth/assets/upload/challenge domains; persisted rule lists also prevent a default-only update from reaching existing users.
- Added a centralized official-domain set and an `openai-services` invariant route before the editable blacklist. Existing persisted rules are preserved, but no longer cause OpenAI dependencies to escape through the direct final outbound.
- Focused Service/compiler/coreadapter tests and vet passed; PowerShell acceptance-script parsing and `git diff --check` passed.
- First full package attempt passed every Go package and stopped at the known Rolldown native binding lock during `npm ci`; exact owner identification is pending.
- Stopped only exact Rolldown owners 2508/9360 and verified zero remaining owners. The clean rerun passed Go and frontend tests, then exposed a pre-existing Phase 46 type mismatch: `NetworkHealthState.unknown` lacked a label-map key; applied the one-line exhaustive mapping fix.
- User rejected the coupled four-mode model. Terminated the pending wrapper, confirmed the elevated run had already failed closed with `rollback=passed_after_failure`, and verified no Navo process remained.
- Started the corrected three-axis refactor: capture, base route, and persistent blacklist/whitelist overrides are now modeled independently; legacy list-mode migration and overlap rejection are included.
- Full Go test suite and `go vet ./...` passed. Frontend tests 6/6, Vue typecheck, production build, PowerShell parser validation, and `git diff --check` passed.
- `webapp-testing` browser smoke passed desktop and narrow layouts. Screenshots are under `artifacts/phase47-ui-isolation`; interaction assertions proved capture=`system_proxy` survived route=`direct` and both survived list save.
- Removed ChatGPT/OpenAI from the product activation gate and retained strict exact-status OpenAI probes only in release acceptance. Proxy/TUN startup therefore remains transport-only and independent of application policy or service WAF state.
- Final gates passed with repository-local caches: full `go test ./...`, `go vet ./...`, frontend typecheck, tests 6/6, production build, package repair, and `git diff --check`.
- Produced `release/Navo-1.0.16-portable-x64` and `release/Navo-1.0.16-portable-x64.zip`; 25/25 ZIP entries verified, package repair found zero issues, ZIP SHA-256 is `F7A0E7D5A019EA8EDAA1AD78451DAF3EDB229C8AD310D1AF40C2DF53442B32EB`.
- Corrected elevated TUN/System Proxy isolation acceptance did not execute because UAC was canceled. Phase 47.4 remains open; no network or registry state was changed by that attempt.
- Reopened Phase 47 after the user clarified that blacklist and whitelist are peer modes, not persistent overrides under System Proxy/TUN. Started a mode-state and migration audit before changing the implementation.
- Added persisted `routing_list_mode=off|blacklist|whitelist`, a dedicated `runtime.list_mode.set` IPC operation, Wails binding/API/DTO support, and compiler isolation that emits only the explicitly active list.
- Removed cross-list overlap rejection because mutually exclusive modes cannot conflict at runtime. Saving either list no longer activates it; capture and default-route changes do not mutate list mode.
- Browser smoke passed desktop/narrow layouts and the full independence interaction sequence. Full Go tests/vet, frontend typecheck/build/tests, PowerShell parser, and `git diff --check` passed.
- Built `release/Navo-1.0.17-portable-x64` and ZIP. Package repair found zero issues; 25/25 entries verified; ZIP SHA-256 is `7683D4F9E02D8677253FFF56F5EBBAE1BE4E0882D8AABEA7174F3CC1AB0FF387`. This supersedes the incorrect 1.0.16 mode semantics.
- Normalized theme text colors in `style.css`: day mode uses dark on-surface text for labels/actions/notices, while night selected/action text uses the light on-surface token. Edge computed-style assertions and visual screenshots passed for both themes after the 180ms transition.
- Built and verified `release/Navo-1.0.18-portable-x64.zip`: package repair 0 issues, 25/25 entries, SHA-256 `6975B40EA429D9237A9D3C5B82138F5D9F34DB69D1365F3A2DB159474DAE1DC6`.
# 2026-08-12 Phase 48: Full-page UI normalization

- Reopened UI scope after user clarified that all pages, not only the menu, must be adjusted.
- Restored Phase 46/47 context, preserved the dirty worktree, and audited all seven page templates plus their page-specific CSS.
- Generated the required full-dashboard design system and Vue/accessibility guidance. Implementation is limited to App.vue/style.css presentation and does not alter page behavior.
- Updated all page templates and added a consolidated full-page style layer. Frontend tests pass 6/6; build is waiting on deterministic lockfile dependency restoration because `vue-tsc` is absent from the current node_modules tree.
- Restored dependencies from package-lock using the repository-local npm cache, fixed the full network-health union, and passed production typecheck/build plus unit tests 6/6.
- Edge smoke passed all seven night pages, three representative day pages, and the 760px compact connection page with zero page/console errors and no horizontal overflow. Added explicit navigation aria-labels after compact icon-only mode exposed missing accessible names.
- Visually inspected the connection, network detection, core management, traffic, speed-test, and settings screenshots; page composition is consistent and no feature section was removed.
- Raised the remaining protocol/log/step-index technical labels to the shared 11px caption token. Final all-visible-text audit passed across all seven pages.
- Final validation: frontend tests 6/6, Vue typecheck/Vite production build, seven night screenshots, three representative day screenshots, one 760px compact screenshot, zero page/console errors, zero horizontal overflow, and `git diff --check` passed.

# 2026-08-13 Phase 49: Cold-start capture and peer list interaction

- Restored `task_plan.md`, `findings.md`, and `progress.md`; preserved the existing dirty worktree.
- Added Phase 49 for cold-start proxy/TUN reconciliation, peer blacklist/whitelist interaction, regression/browser verification, and portable acceptance.
- Generated the required Navo desktop operations design guidance and Vue accessibility/state recommendations before implementation.
- Inspected the current 1.0.18 portable log and, with read-only elevated access, the protected runtime state and sing-box log. Identified cold-start endpoint DNS timeout and the missing capture revalidation after live core switching.
- Added the shared retrying endpoint resolver, System Proxy endpoint IP pinning, TLS identity preservation, and TUN reuse of the cold-start resolver.
- Added Agent-owned atomic core switching with capture stop/re-enable validation and previous-core rollback.
- Changed list activation to reset to off on every launch while preserving list contents.
- Reworked the connection control into four peer entries: System Proxy, TUN, Blacklist, and Whitelist. Clicking a list enables it, opens only its editor, and focuses the textarea; closing the list hides the editor and disables list routing.
- First focused Go gate passed Network, Service, and Wails packages but Agent did not compile because `capture_transition.go` lacked the new `strings` import. Added the import before rerunning.
- Second focused Go gate passed Network, Service, and Wails packages; the Agent rollback test exposed only an incomplete expected trace because TUN shutdown correctly performs `tun.status`. Updated the assertion without changing runtime code.
- A rerun accidentally selected an empty repository-local Go module cache and timed out fetching dependencies. The failure occurred before package setup; subsequent checks reuse the populated machine module cache.
- Focused Go packages passed after selecting the populated `.cache/go-mod`; frontend typecheck, 6/6 tests, and production build also passed.
- The first Edge smoke reached the connection page and confirmed four rendered entries, then stopped because the test expected `TUN` instead of the established `TUN 代理` label. Updated only the assertion.
- Edge smoke passed desktop/compact and day/night interaction checks with 3 screenshots, zero console/page errors, and zero horizontal overflow; visual inspection confirmed the four peer controls and conditional single-list editor.
- Full `go test ./...` and `go vet ./...` passed. The first 1.0.19 package run then hit the known Rolldown file lock after its Go gates. Stopped only verified Navo port-4193 Vite PIDs 12596/22240 and their npm parents 25268/23904; unrelated Node processes were left untouched.
- The second package run encountered the known transient SelfHeal temp-file sharing violation. `internal/selfheal` then passed 10/10 isolated repetitions using a dedicated temp root.
- The clean package retry passed full Go tests/vet, frontend tests/typecheck/build, and Wails production build. Created `release/Navo-1.0.19-portable-x64` and ZIP; repair found 0 issues, manifest mismatches were 0 across 25 files, ZIP has 26 entries including the root directory, and SHA-256 is `EBFDA86F29656194E26D30EFE3E25CD1963B5F72BC710E4B9CA9BA276BECE4DE`.
- PowerShell parser checks and `git diff --check` passed. The requested elevated isolated real TUN/System Proxy + Google/GitHub data-plane run remains unperformed because the environment requires separate explicit authorization for those network mutations and destinations.

# 2026-08-13 Phase 50: Proactive capture hardening review

- Restored Phase 49 planning context and started a structured correctness/security/performance review of the new cold-start resolver, capture/core transaction, routing-list session state, and portable release path.
- Transaction audit found an outcome-ambiguity bug when the `core.select` Named Pipe response is lost after Service has already switched cores. Preparing a failing Agent regression before changing production code.
- The new regression failed as expected: after a simulated lost response, current code restored TUN while `mihomo` remained active instead of restoring `sing-box`.
- Added actual-core reconciliation around failed/ambiguous core selection. Focused core-switch tests now pass. Review then found a UI-only outcome ambiguity when list activation succeeds but its dashboard refresh fails.
- Browser fault injection reproduced the list split: backend mode changed to blacklist, but the editor remained absent for 30 seconds after the follow-up dashboard request failed. Updated the UI to distinguish committed mutation from refresh status.
- Browser fault injection now passes. IPC audit then confirmed Agent retries mutations but Service currently has no response replay/deduplication by request ID.
- Added the IPC replay regression and confirmed the red state: different requests using the same ID both executed and the second returned `METHOD_NOT_FOUND` instead of an identity conflict.
- Implemented Service-wide at-most-once replay for identified IPC requests plus a concurrent duplicate regression. Focused Service and Agent packages pass; one existing test was corrected to use a unique ID per distinct method.
- Full `go test ./...`, `go vet ./...`, frontend typecheck, frontend tests 6/6, and frontend production build pass after the three Phase 50 repairs.
- A supplemental `go test -race` cannot run because the installed Go toolchain is `windows/386`, where race instrumentation is unsupported. The concurrent replay test still exercises the owner/waiter path in the normal runtime.
- The post-fix browser fault-injection smoke already passed with 3 screenshots and zero console/page errors. A redundant rerun selected Python runtimes without the Playwright module; this is an environment-only retry failure, not a product or assertion failure.
- The first 1.0.20 package attempt passed all Go gates and stopped at the known Rolldown binding lock. Restart Manager identified only Navo port-4193 Vite PID 4684 and cmd parent 26508; stopped those exact residual test processes and verified the lock was released.
- Added an exact `nanoid 3.3.17` override after package audit found the transitive 3.3.16 advisory. Repository-local install, npm audit (0 vulnerabilities), typecheck, tests 6/6, production build, and the clean package rerun all passed.
- Built `release/Navo-1.0.20-portable-x64` and ZIP. Repair reports 0 issues; SHA manifest independently verified 24/24 entries; package and ZIP each contain 25 files; required executables/manifest are present; ZIP SHA-256 is `AE0BCB29A17519B322AA0D65D7C572D8ED3B980EEC5E284BD7A1DB59A8C720A7`.

# 2026-08-13 Phase 52: Full connection-chain stability audit

- Restored `task_plan.md`, `findings.md`, and `progress.md`, ran the planning catch-up helper, inspected the dirty worktree, and reviewed the Navo Windows release-acceptance constraints.
- Added Phase 52 covering baseline ownership, full failure-boundary review, focused repairs, automated gates, and real System Proxy/TUN data-plane acceptance.
- Read-only repository discovery hit protected recursive paths once; switched to bounded source-directory inspection and confirmed no additional nested `AGENTS.md` instructions in the known source roots.
- Captured a read-only live baseline: no Navo process or listener, WinINet disabled, and only unrelated v2rayN-owned processes present. No process or network state was changed.
- Began chunked review of the two required Windows proxy/TUN acceptance guides after the first combined output exceeded the tool display budget.
- Reviewed the configuration, native-validation, core-readiness, protocol-handshake, explicit-proxy, Service/IPC, WinINet, and System Proxy crash/ownership gates from the full-chain guide.
- Completed the full-chain guide review through TUN adapter/route/DNS/data-plane, crash/network-change/reboot recovery, permissions/IPC security, diagnostic, regression-matrix, and release-blocker sections.
- Reviewed the one-pass guide through transaction ownership, independent rollback context, delayed health commit, deterministic endpoint pinning, strict adapter readiness, idempotent exact resources, control-plane validation, and no-proxy data-plane validation.
- Completed the one-pass guide, then mapped the production functions across Agent, System Proxy, Service capture/runtime/replay, Network Manager/plan/journal/recovery, core host, compiler, and Wails boundaries.
- Ran the full Go baseline through `scripts/test.ps1`. Every package except `internal/selfheal` passed; the suite stopped on a Windows sharing violation reading `selfheal-state.json`, so the vet stage remains pending.
- Traced the failing SelfHeal flow through Engine rollback, circuit-budget completion, and the test's unsynchronized external file read. Next step is to inspect the atomic writer and add a deterministic completion boundary/regression.
- Confirmed the atomic writer itself flushes/closes before replacement. Expanded the fix scope to two production lifecycle defects: canceling half-open backoff can permanently wedge the circuit, and restarting one Engine can process stale pre-stop queued events.
- Added focused SelfHeal regressions. The stale queued-event test failed red as expected; corrected only the half-open test setup to establish an opened circuit through the real begin/complete sequence before rerunning.
- Implemented the SelfHeal circuit/lifecycle repair and formatted the affected files. `go test ./internal/selfheal -count=20` passed.
- Completed line-by-line review of Agent capture transitions, core-switch rollback/reconciliation, startup recovery, and health monitoring. Flagged the abandoned recovery goroutine and optional System Proxy post-apply probe for production-wiring verification.
- Reviewed Agent request transport and launcher wiring. Confirmed production bypasses Named Pipe through a context-free direct dispatch, no capture probe is wired, and Named Pipe carries stale deadlines into the next Send.
- Reviewed System Proxy registry ownership/restore implementation, launcher Agent construction, and the complete Service capture/TUN transaction plus adapter monitor. Next review targets Service IPC cancellation/replay and runtime restart/rollback semantics.
- Reviewed Service connection deadlines, replay cache, runtime mode/rule/list transactions, active routing verification, config swap/TUN rebind, revision commit, and rollback. Confirmed node-selection serialization and bounded-rollback defects; LKG commit timing remains a wider refactor decision.
- Reviewed Supervisor swap/stop semantics and Pipe channel lifecycle. Confirmed `SwapConfig` proceeds after a failed stop, while Agent retry paths neither install the new deadline before Send nor close failed channels. Preparing focused red regressions for these transport and lifecycle boundaries.
- Expanded the mutation audit through upstream proxy and subscription handlers. Found additional unbounded background core reloads and an outbound-delete credential/config split; these now share the same transaction-fix scope instead of being treated as unrelated CRUD behavior.
- Added context-aware Agent direct dispatch and Named Pipe retries, installed deadlines before writes, and close stale channels on every reconnect. Startup recovery now cancels the actual Service call instead of abandoning a mutating goroutine; production launcher wiring passes the request context through.
- Added a context-aware Service connection lock and extended it across capture, core switch/update, node selection, routing, upstream-proxy mutations, and subscription refresh/removal. Long Service IPC deadlines now match Agent transaction classes.
- Initial compile-only validation correctly found eight direct unit-test calls using the old handler signatures; updated those tests to pass explicit contexts before functional execution.
- Focused Agent, Service, and Supervisor suites passed after the context/serialization changes. Added regressions for canceled recovery, busy transport cancellation, busy Service transaction cancellation, replay-handler panic completion, failed Supervisor swap, and failed force-stop restart.
- Promoted outbound selection to an Agent-owned stop/select/re-enable transaction. Service refuses a direct live-TUN endpoint change, and active runtime revisions remain candidates until data-plane verification commits them healthy.
- Wired profile-local WinINet ownership state plus a real current-user WinINet default-proxy HTTPS probe into the production launcher. Compile-only checks pass for Agent/System Proxy/Service/Supervisor/launcher; functional rerun follows the new outbound regression.
- Full Go tests and `go vet ./...` passed through `scripts/test.ps1`; frontend tests passed 6/6, Vue typecheck/build passed, npm audit reports zero vulnerabilities, and `git diff --check` passed.
- Removed the remaining unsafe live TUN-config reload and made subscription removal reversible until runtime activation succeeds. Focused Service/Subscription/Agent/Supervisor suites pass after these final transaction changes.
- PowerShell parser checks passed for the test/package/elevated-TUN scripts. The clean `Navo-1.0.22-portable-x64` package run passed full Go tests/vet, npm ci/audit, frontend tests 6/6, typecheck/build, and Wails Windows/amd64 production build.
- New portable repair check reports 0 issues. Independent manifest verification passed 24/24 hashes; launcher and UI are PE32+ amd64 (`0x8664`); package and ZIP each contain the same 25 files under one root. ZIP SHA-256: `7B0AEC97C4D74FC71B0F73454886D9700087C0E1316CB2AE3E5E6DD082EB5ECD`.
- A new subscription rollback regression initially hit sandbox-profile DPAPI unavailability; it was made environment-independent with an in-memory credential store and identity protectors, then passed 5/5 focused repetitions. The final full repository Go test/vet gate passed afterward.
- Invoked the isolated 1.0.22 elevated System Proxy/TUN acceptance, but Windows UAC was canceled before Navo launched. No retry or privilege bypass was attempted.
- Post-cancel read-only checks found WinINet still disabled, no Navo process, no known Navo listener, and no acceptance output. Only the pre-existing unrelated v2rayN sing-box process remained; it was not touched. Non-elevated adapter inspection returned `Access denied`, so adapter absence was not inferred.
- Phase 52 automated repair and packaging are complete. Real elevated DNS/TCP/HTTPS, Google/GitHub, exit-IP, delayed-stability, crash/recovery, and exact-rollback acceptance remains pending explicit UAC consent.

# 2026-08-13 Phase 53: UI request identity collision

- User reproduced `REQUEST_ID_REUSE` for `ui-1786631794855-13`. Started tracing UI request identity without weakening Service replay protection.
- Traced the collision to Agent-to-Service fan-out, not Service or Vue. Added deterministic Agent/dashboard regressions; both fail red because distinct Service methods receive the same UI parent ID.
- Added Agent-owned Service request identities with a random per-process session and atomic sequence. Focused collision/dashboard regressions pass 10/10 while verifying UI response correlation and caller request immutability.
- Agent/Service regression packages pass 3/3; full Go test/vet, frontend tests 6/6, typecheck/build, npm audit, and diff check pass.
- Built `release/Navo-1.0.23-portable-x64` and ZIP. Repair: 0 issues; manifest: 24/24; files: 25/25; PE: amd64; ZIP SHA-256: `5B5DCEBFAD61F29BD03794C75D2CC8587E3F40E4722C923F0380B14067AE4DD4`.

# 2026-08-13 Phase 51: Layout refinement

- User requested a layout optimization after the capture/list behavior and 1.0.20 portable build were completed.
- Restored the persistent plan and prior full-page UI decisions. Started a presentation-only audit using the required UI/UX design-system workflow; backend and interaction semantics remain out of scope.
- Generated the required operations-dashboard design system plus Vue/accessibility guidance. Visually inspected the current 1280x900 and 760x900 connection screenshots and recorded the hierarchy, density, card-width, and compact step-layout issues before editing.
- Implemented a presentation-only connection-page hierarchy: compact intro, two-column source/node preparation area, single primary traffic-control panel, stronger selected markers, balanced compact routing grid, and a notification position that no longer covers the page title. Existing handlers, modes, ARIA state, themes, and content remain unchanged.
- First browser geometry run stopped on the new strict intro-height assertion at 92.42px; server cleanup completed automatically. Reworked only the intro composition into a horizontal title/description band before retrying.
- Second browser run passed all current interaction/layout assertions and produced three Phase 51 screenshots with zero console/page errors. Visual inspection accepted the new hierarchy and compact three-column route row, but found a small toast/theme-switch overlap that the current assertion did not cover; tightening both CSS and the browser gate before final acceptance.
- Final browser run passed after adding the theme-switch collision assertion: desktop/compact layouts, day/night rendering, list fault-injection interaction, intro height, source/node alignment, compact three-column route alignment, zero horizontal overflow, and zero console/page errors. Final visual inspection found no residual header collision.
- Final frontend typecheck, tests 6/6, production build, and `git diff --check` passed. The first 1.0.21 package run passed all Go packages and then hit the known Rolldown file lock left by a Vite child; exact owner cleanup is in progress before one retry.
- Restart Manager identified only Navo port-4194 Vite PIDs 24964/26004 and cmd parents 22224/21536. Stopped those exact residual browser-test processes, verified the binding lock was gone, and preserved unrelated processes.
- Clean 1.0.21 package rerun passed full Go tests, npm audit with zero vulnerabilities, frontend tests 6/6, Vue typecheck/build, and Wails Windows/amd64 production build.
- Built `release/Navo-1.0.21-portable-x64` and ZIP. Repair reports 0 issues; manifest independently verified 24/24 hashes; package/ZIP contain 25 files; required executables and manifest are present; ZIP SHA-256 is `96A6FE2B1C3E68D892E77183944957BED151CD7F4CD31251D7BA018BED0B135F`.
- Final audit corrected a browser-evidence weakness: screenshots named `night` had inherited the headless light preference. Updated the smoke to select night explicitly before night/compact checks and retain the explicit day switch for the day screenshot.
- Corrected browser smoke passed with explicit night and day themes, three screenshots, zero overflow, and zero console/page errors. Visually inspected the actual dark screenshot and accepted contrast, hierarchy, selection visibility, and header spacing.
- The helper left one final Navo port-4194 Vite child despite reporting shutdown. Verified PID 23024 and cmd parent 23196 by full command line, stopped only those two test processes, and confirmed the listener was gone.
# 2026-08-14 Phase 54: System Proxy readiness HTTP 502

- Restored the persistent planning context and prior Windows proxy/TUN acceptance rules.
- Located the reported error in `Agent.EnableProxy`: WinINet remains fail-closed because `probeHTTPProxy` rejects HTTP 502 before registry mutation.
- Confirmed the readiness probe currently tries three public plaintext HTTP captive-portal endpoints through the local proxy; all non-2xx/3xx responses collapse to the last status without endpoint/body/core-log context.
- Started bounded live-profile, generated-config, listener, and core-log correlation without touching existing Navo or unrelated processes.
- Exact core evidence identifies the upstream failure: all three probe destinations reached sing-box, were routed through the selected SOCKS outbound, then timed out dialing `119.147.134.162:8001` after 5 seconds each. Navo rolled back to capture-off and stopped its core.
- The first all-node IPC test exceeded the 60-second shell budget; its Navo instance exited cleanly. A compact retry had a PowerShell parser typo before process launch, so it caused no runtime mutation.
- A later elevated portable launch proved the protected UI pipe privilege boundary: the lower-privilege caller received `Access denied`. Exact test-owned launcher/UI PIDs 15376/5520 were removed through one approved UAC cleanup; the unrelated v2rayN sing-box PID was preserved.
- Implemented System Proxy selected-outbound preflight plus actionable capture error codes, retained the real HTTP data-plane gate, and changed probe diagnostics from last-error-only to all-endpoint aggregation.
- Added regressions for unreachable/reachable selected outbound and three-endpoint HTTP 502 aggregation. Agent and Service tests pass 3/3 repetitions.
- Full repository Go tests and vet passed after restoring the isolated module cache.
- Packaging exposed and repaired an independent Named Pipe cold-start race. The new delayed-listener test passed 20 repetitions on the host architecture and the full pipe suite passed 5 amd64 repetitions; a subsequent full repository test/vet gate passed.
- The latest package attempt passed every Go package and stopped only at `npm ci` with a registry `ECONNRESET`; release output remained untouched by the package script's source gates.
- The mirror-backed package retry passed all Go tests/vet, npm install, frontend tests 6/6, Vue typecheck/Vite build, and Wails Windows/amd64 production build.
- Built `release/Navo-1.0.24-portable-x64` and ZIP. Repair reports 0 issues; independent manifest verification passes 24/24; package contains 25 files and ZIP 26 entries under its root. ZIP SHA-256 is `B0B1D7447546D4F60286DB4EF367FDF00FBB502CDFA5D7CF1B54A68F45F7FF79`.
- Extended the elevated acceptance script to clone encrypted subscription endpoint storage and optionally select the fastest live outbound in the isolated profile. PowerShell parser and `git diff --check` pass.
- Requested the real 1.0.24 TUN/System Proxy routing-mode acceptance, but UAC was canceled before test process creation. No retry or bypass was attempted; real traffic/rollback acceptance remains pending.
# 2026-08-14 Phase 55: UIIPC request failures

- Restored the persistent plan and prior Named Pipe reliability context.
- Started source/log correlation for repeated UIIPC failures. No user-owned process or network state has been changed.
- Read-only correlation against the protected profile and portable launcher log identified the exact request IDs, failure codes, successful core startup, HTTP 502 data-plane failure, and false-negative PowerShell adapter readiness boundary.
- Implemented native Windows adapter readiness inspection and actionable UIIPC error logging.
- Focused `go test ./internal/network ./internal/agent -count=1` passed.
- Full `scripts/test.ps1` passed all Go packages and `go vet ./...`; `git diff --check` passed.
- Frontend tests 6/6, Vue typecheck/build, and `npm audit` with zero vulnerabilities passed after updating the stale build-time nanoid override from 3.3.17 to 3.3.18.
- Built `release/Navo-1.0.25-portable-x64`; repair reports 0 issues, independent manifest verification passes 24/24, and the directory contains 25 files.
- ZIP creation was not performed because the execution approval system rejected the operation after its usage limit was reached. No workaround or destructive action was attempted.
- Real elevated TUN activation, DNS/TCP/HTTPS/exit-IP, delayed stability, and rollback were not run in this turn; no UAC test was requested and no existing process/network state was changed.

# 2026-08-14 Phase 56: all-mode real-data acceptance

- Continued toward the persistent objective: every supported capture/routing mode must be usable and return correct real data.
- Restored the full acceptance rules: listener/UI state is insufficient; require ordinary no-proxy or WinINet application DNS/TCP/HTTPS, Google/GitHub, exit-IP, stability, exact rollback, and zero owned residue.
- Current runnable package is `release/Navo-1.0.25-portable-x64`. No Navo process/listener is active; only the unrelated v2rayN-owned sing-box process is present and must remain untouched.
- The existing elevated runner covers sing-box/Mihomo, TUN and System Proxy, bypass_mainland/global/direct, explicit blacklist/whitelist/off list modes, Google/GitHub application probes, WinINet ownership, exit-IP, delayed health, and rollback in an isolated cloned profile.
- Elevated TUN execution was rejected before launch by the safety reviewer; no UAC/process/network mutation occurred.
- Implemented and parser-validated a non-TUN `system-proxy-routing-modes` scenario, then built an amd64 as-invoker launcher from the exact 1.0.25 source in `.cache/system-proxy-acceptance-1.0.25`.
- First sandboxed System Proxy attempt failed during privacy initialization because Windows DPAPI is unavailable to the sandbox identity. The scenario confirmed exact WinINet rollback and left no Navo process.
- The first approved isolated-profile run completed privacy initialization but exposed that the temporary launcher had inherited host `GOARCH=386`; the x86 Job Object layout failed before IPC. Rebuilt the test-only launcher explicitly as PE32+ amd64 (`0x8664`).
- System Proxy then activated successfully at `127.0.0.1:12080`, verified custom rules, Google 204, GitHub 200, ChatGPT edge 403, and OpenAI API 401 through WinINet. Every failed run restored the pre-existing v2rayN WinINet configuration exactly.
- The matrix proved `bypass_mainland` and `global` with real application traffic, then found a product failure in `direct`: runtime verification still required Google/GitHub through the direct route, so Google TLS timeout caused `RUNTIME_MODE_VERIFY_FAILED` and rollback.
- Repaired route-semantic verification: System Proxy direct mode now verifies Baidu 200 plus Xiaomi captive-portal 204; TUN activation and later TUN runtime verification classify explicit `runtimeModeDirect` as direct even when a proxy outbound remains selected. Focused Service tests pass.
- The first direct-mode rerun reached `outbound/direct[direct]` but both direct probes failed DNS. Core logs proved sing-box auto-selected VMware VMnet8 and sent the WLAN-configured `8.8.8.8` resolver through the isolated VMware subnet.
- Added a frozen physical egress to the canonical compiler model, sing-box `route.default_interface`, and Mihomo `interface-name`; non-TUN runtime apply resolves the current Windows physical route, while TUN retains its preflight-frozen interface.
- Fixed `Find-NetRoute` candidate filtering: Windows returned an empty synthetic candidate before the valid `WLAN 2 -> 192.168.12.1` route. Empty `NextHop` entries are now rejected before sorting. Focused Compiler/Network/Service tests pass.
- Strengthened the System Proxy matrix with public-IP invariants: proxy modes must differ from the direct baseline, direct must match it, blacklist routes a matching ipify domain through the proxy, and whitelist routes the same domain directly.
- Final System Proxy matrix passed. `bypass_mainland` and `global` returned proxy exit `165.254.151.219`; `direct` returned the host baseline `183.158.168.94`; blacklist forced the matching IP service through the proxy, whitelist forced it direct, and off restored global behavior.
- Google 204, GitHub 200, ChatGPT edge 403, OpenAI API 401, Baidu 200, and Xiaomi 204 all passed through the intended route semantics. Capture ended stopped, direct IP after matched before, and WinINet was restored byte-for-byte to the pre-existing v2rayN configuration.
- Full `scripts/test.ps1` passed every Go package; frontend tests passed 6/6, typecheck/build passed, npm audit reported zero vulnerabilities, and `git diff --check` passed.
- Built `release/Navo-1.0.26-portable-x64` and ZIP. Repair reports 0 issues; manifest verification passes 24/24; directory and ZIP each contain the same 25 files under one root; launcher/UI are amd64 PE `0x8664`; ZIP SHA-256 is `0D821375EC2269FBBBDF26DA4D4DB0AFECE40658A412DB4F1C64C5E37451F682`.
- Final residue check found zero Navo launcher/UI processes. The only sing-box process remains the unrelated v2rayN binary PID 1776 and was not touched.
- Created `.cache/system-proxy-acceptance-1.0.26` from the final portable directory and replaced only the UAC-manifest launcher with an amd64 as-invoker build from the same source. All other 24 package files match the final release byte-for-byte.
- Current-package System Proxy matrices passed for all supported cores: sing-box 1.13.14, Mihomo 1.19.29, and Xray 26.3.27. Each proved bypass/global/direct, blacklist/whitelist/off, application probes, direct/proxy exit separation, capture-off, and exact WinINet rollback.
- Extended the elevated runner to admit Xray only for System Proxy and reject Xray TUN before launch, matching advertised capabilities.
- Strengthened the pending TUN `routing-modes` scenario: direct uses Baidu/Xiaomi instead of blocked Google, every no-proxy public-IP retry uses a fresh transport, and both TUN and post-TUN System Proxy list modes now require direct/proxy exit-IP semantics instead of status-only evidence. Parser and diff checks pass.
- Updated `test-tun-elevated-matrix.ps1`: `full` now includes sing-box and Mihomo `routing-modes`, a dedicated `routing` suite is available, and every case receives the configured 30-second stability window. Both elevated scripts pass PowerShell parsing and diff checks.
- Elevated TUN authorization remains absent after three consecutive goal turns. No adapter, route, NRPT/DNS, IPv6, or firewall mutation was attempted; the remaining full-objective work is now blocked at the explicit protected-operation boundary.
- User resumed the goal with “TUN也要验证啊”. Read-only baseline passed: current 1.0.26 acceptance package exists, no Navo-owned process is active, v2rayN SOCKS5 `127.0.0.1:10808` is reachable, and WinINet still points to the preserved v2rayN configuration.
- The first resumed elevated routing launch was rejected before process creation because the user had not explicitly approved the concrete temporary adapter/route/DNS/NRPT/IPv6/firewall mutations and short outage risk. No workaround or network mutation was attempted.
- Audited the elevated runner without starting Navo or touching the network. The previous rollback snapshot covered only split routes, Navo-tagged NRPT/firewall artifacts, the Navo adapter, and non-Navo IPv4 DNS; it did not prove default routes, all NRPT state, physical IPv6 bindings, final WinINet state for TUN scenarios, or package-owned process residue.
- Strengthened `test-tun-elevated.ps1` to compare IPv4/IPv6 default plus split routes, all NRPT rules, Navo firewall/adapter state, physical IPv4 DNS, and non-Navo IPv6 bindings. Every scenario now asserts exact WinINet rollback.
- Added final package-path process residue detection after graceful launcher shutdown. Only processes whose executable resolves inside the tested package may be force-cleaned; any such residue makes the acceptance case fail and is preserved in JSON evidence. PowerShell parsing passes; no UAC or network mutation was attempted.
- The user explicitly authorized the complete elevated TUN matrix and its bounded temporary adapter/route/DNS/NRPT/IPv6-firewall changes. Phase 56 live execution resumed.
- Added `AutoSelectReachableOutbound` passthrough to the matrix wrapper so every isolated case selects a currently reachable outbound instead of inheriting a stale profile selection.
- Preflight passed for the final 1.0.26 package: PowerShell parsers pass, package repair reports 0 issues, the real profile exists, and there were no Navo/core processes or Navo adapter before launch.
- Two authorized `RunAs` launch attempts waited 30 and 120 seconds respectively but never returned from the desktop UAC boundary. After each timeout there was no acceptance output directory, matrix/Navo process, or network mutation. Do not claim that any TUN case ran.
- Added `StopOnFailure` to the matrix wrapper so a single interactive elevated launch can run the complete suite while stopping at the first genuine product/test failure for repair instead of continuing through unrelated network mutations.
- Launched the complete elevated matrix through a detached UAC helper. The first sing-box `routing-modes` case selected a live 51 ms outbound, then failed `TUN_ADAPTER_NOT_READY` after sing-box reported the Navo TUN inbound started.
- Failure rollback passed: before/after routes, NRPT, firewall, adapter, DNS, IPv6 bindings, and WinINet matched; process residue was empty. The matrix stopped as designed.
- Fixed native readiness evidence preservation so a later adapter snapshot clears an earlier missing-adapter error and the deadline cannot overwrite the last meaningful observation with a generic context error.
- Diagnostic rerun exposed the exact snapshot: `Navo`, description `sing-tun Tunnel`, non-hardware, Up, address `172.19.0.1/30`, but link-layer `MIB_IF_ROW2.Mtu=65535`. The configured IPv4 MTU must be read from `MIB_IPINTERFACE_ROW.NlMtu` instead.
- Added the current sing-box `sing-tun Tunnel` description to the strict owned-description allowlist while retaining name, non-hardware, interface index, GUID, LUID, status, address, and MTU checks. Unknown software tunnels remain rejected by regression.
- Removed volatile NetRoute `State` from rollback snapshots after the only before/after difference was VMware route state `2 -> 1`; exact prefix/interface/next-hop/metric/protocol checks remain.
- Network and Service focused tests passed. Compiler initially hit protected `%TEMP%`, then passed with repository `TEMP/TMP/GOTMPDIR`; diff check passed.
- The adapter-fix rerun crossed readiness and captured ordinary system DNS, then failed closed at `TUN_DNS_VERIFY_FAILED`. Core evidence showed the auto-selected 55 ms Shadowsocks node rejected encrypted traffic with authentication failures; rollback passed and residue was zero.
- Strengthened `AutoSelectReachableOutbound`: candidates are now selected by latency only for ordering, then each must pass a real temporary System Proxy HTTP data-plane activation and exact WinINet restoration before it may be used for TUN. Candidate IDs, latency, verdict, and stable failure code are retained in artifacts.
- Real candidate preflight rejected eight low-latency but unusable subscription nodes and selected the working `MiYaIp` SOCKS outbound. sing-box TUN then passed HEALTH_COMMITTED, DNS/TCP/HTTPS, Google 204, GitHub 200, no-proxy HTTPS 200, direct/proxy exit separation, and the 30-second stability interval.
- Applying customer routing rules exposed a core-reload race: Rebind accepted the still-visible persistent adapter on a single observation, applied routes to interface 60, then the adapter generation transitioned and removed them. Rollback still passed with zero residue.
- Added a two-second continuous-stability gate for the exact owned adapter identity/status/IPv4 MTU/CIDR. Missing/down/IP Helper errors and identity changes reset the gate. Focused Network/Service tests and new temporal regressions pass.

# 2026-08-15 Phase 57: crash-safe DNS and boot proxy recovery

- Current-boot baseline had no Navo process/listener or autostart registration. The only live core was v2rayN sing-box PID 5000; WinINet was `127.0.0.1:10808`; Ethernet and WLAN 2 DNS were DHCP-provided `223.5.5.5` with empty static `NameServer`.
- Lifecycle audit found four startup faults: launcher readiness waited 10 seconds against a 30-second rollback budget; Supervisor continued after reconciliation errors; Agent ignored an early System Proxy recovery error; Agent capture recovery timed out at 20 seconds before Service rollback could finish.
- Launcher startup wait is now 45 seconds and preserves Service exit causes. Supervisor fails closed on reconciliation errors/dirty recovery. Agent uses one authoritative 45-second recovery path and blocks a new capture transition until faulted/recovering ownership has been reconciled.
- Added launcher timing/exit tests, Supervisor corrupt-recovery fail-closed coverage, Agent fault-before-transition recovery coverage, durable after-NRPT crash-journal recovery, and retryable System Proxy registry recovery.
- Focused packages passed once and five repetitions; NRPT crash recovery and System Proxy transient recovery each passed ten repetitions. Full repository Go tests/vet passed, frontend tests passed 6/6, npm audit reported zero vulnerabilities, Vue typecheck/build passed, PowerShell parsing passed, and `git diff --check` passed.
- The first real 1.0.27 crash run rolled back exactly but exposed `sing-tun Tunnel #2`: Windows had appended a numeric device suffix, while strict ownership allowed only the unsuffixed base description. This was the direct cause of `TUN_ADAPTER_NOT_READY` seen after boot.
- Ownership now accepts only the exact known Wintun/sing-tun base descriptions or a pure numeric ` #N` suffix while retaining canonical `Navo` name, non-hardware identity, valid GUID/index, Up state, MTU, and address checks. Unknown/malformed descriptions remain rejected.
- Rebuilt `release/Navo-1.0.28-portable-x64`; full package gates passed.
- Elevated 1.0.28 `after-nrpt` crash injection passed: process exit code 91, durable restart recovery, capture stopped/missing, exact route/NRPT/firewall/adapter/DNS/IPv6/WinINet rollback, direct IP before/after `115.220.145.88`, and no package-owned process residue.
- Elevated fresh isolated-profile System Proxy activation passed on the current boot for bypass-mainland/global/direct. Google 204, GitHub 200, ChatGPT edge 403, OpenAI API 401, Baidu 200, and Xiaomi 204 followed the expected route semantics; proxy exit was `165.254.151.219`, direct exit was `115.220.145.88`.
- WinINet changed only while owned by Navo (`127.0.0.1:12080`) and returned byte-for-byte to v2rayN `127.0.0.1:10808`. Final live residue was zero Navo process/adapter/split route/NRPT/firewall; physical DNS remained DHCP `223.5.5.5`; v2rayN PID 5000 remained running.
- Removed only two runtime files created by validation inside the package (`data/navo.ico`, `log/navo.log`) before delivery. Final repair reports 0 issues, `SHA256SUMS.txt` verifies 24/24, directory/ZIP match 25/25, both PE files are amd64 `0x8664`, and ZIP SHA-256 is `32632EE5D390625D88FA575B025BAFFD02A6EF75A7275F4D5F1D95723CAE7410`.
- A physical Windows reboot after installing this source was not performed; acceptance covers a fresh process/isolated profile on the current boot plus a real forced process crash and restart.

# 2026-08-15 Phase 58: project hardening and release closure

- Restored the persistent plan and audit evidence. The worktree started clean on `main` aligned with `origin/main`.
- Began local-only hardening for version identity, sealed package verification, subscription reliability, CI/security gates, documentation, licenses, and artifact hygiene. No live network state or user process has been changed.
# 2026-08-15 Phase 58 completion

## Implemented

- Added VERSION 1.0.29 and internal/buildinfo injection.
- Added subscription multi-IP fallback regressions and asynchronous apply error logging.
- Added THIRD_PARTY_NOTICES, the upstream sing-box license, strict verify-package.ps1, sealed versioned directory/ZIP output, and PE metadata checks.
- Hardened CI with pinned Actions, coverage, race, govulncheck, npm audit, module verification, and PowerShell parsing.
- Replaced stale architecture/deployment documentation and ignored new artifacts output.

## Validation

- Go full test: PASS.
- go vet: PASS.
- Go total coverage: 51.9%, threshold 50% PASS.
- Frontend tests: 6/6 PASS.
- Frontend typecheck/build: PASS.
- npm audit: 0 vulnerabilities.
- PowerShell parser: package.ps1 and verify-package.ps1 PASS.
- Negative verifier: old package, unexpected file, and tampered ZIP content all rejected.
- Final package verifier: 28 files, 27 manifest entries, directory and ZIP PASS.
- PE versions: navo.exe, repair.exe, app_ui/navo_app.exe all 1.0.29.0.
- Final archive SHA-256: E3C1356F9BA01B9F908AD78EFD3874470C8111317E30D98C2F5719FAFF7B2CFA.
- git diff --check and documentation control-character scan: PASS.
- Normal-user runtime smoke: FAIL at elevated Named Pipe ACL; zero Navo residue and v2rayN preserved.
- All-elevated runtime smoke: NOT RUN because UAC was canceled.
- Post-smoke sealing: removed only generated data\navo.ico and log\navo.log; final directory/ZIP verifier passed again.

## Pending external acceptance

- Current sing-box/Mihomo elevated TUN routing and crash/recovery matrix.
- Physical reboot/cold-start and upgrade rollback.
- Authenticode signing with a supplied certificate.
- All-elevated package runtime smoke.

# 2026-08-16 Phase 59: architecture boundary refactor

- Read the complete V1 specification and restored the existing persistent planning context.
- Added Phase 59 without replacing prior incomplete Windows acceptance phases.
- Confirmed initial worktree scope: the user-provided V1 document is untracked; unrelated tracked changes were not observed in the initial short status.
- Initial sandboxed reads and all direct apply_patch entrypoints failed while applying managed deny-read ACLs. Switched to approved, exact UTF-8 text replacement for the planning files; no repository mutation occurred during failed attempts.
## Phase 59.2 progress

- Added internal/connection Coordinator with typed operation/origin/phase, cancellable queueing, non-blocking acquisition, current/last transaction snapshots, and failure evidence.
- Added focused tests for serialized control, observation-safe snapshots, context cancellation, busy-owner reporting, and failed transaction recording.
- Focused result: go test ./internal/connection PASS.
- Error log: the first new-file unified diff used incorrect hunk line counts and truncated both files at EOF. The first tail repair was also rejected as corrupt. Read exact numbered tails, applied bounded EOF patches, then gofmt and focused tests passed.
- Replaced Agent captureMu business ownership with the typed Connection Coordinator for capture, node, core, source, routing-policy, core-update, startup recovery, and scheduler-triggered recovery operations.
- Connection restart and manual network recovery now hold one transaction across stop plus restore/recover; no unrelated control request can interleave between phases.
- capture.status now exposes current/queued/last transaction evidence while existing capture fields remain compatible.
- internal/agent tests PASS (2.064s).
- Error log: the first machine-generated Agent patch failed check because CRLF-normalized diff context did not match under strict whitespace. Reapplied the same reviewed patch with whitespace-only tolerance.
- Error log: the first reusable patch helper trimmed the leading dot from .tmp and pointed git at tmp. No source file was changed; reran with explicit .tmp paths and both patches applied.
## Phase 59.3-59.4 progress

- Added explicit runtime SelectedOutbound, ActiveOutbound, and CandidateOutbound identities with safe migration from pre-V1 state.
- Candidate remains uncommitted while the core is stopped or verification is in flight; commitHealthyRuntime promotes it to Active and clears Candidate only after healthy verification.
- Service runtime/list/diagnostics responses and Wails/Vue DTOs now expose selected_id, active_id, and candidate_id. The route list labels Candidate as 待验证 and keeps traffic/health views bound to committed Active.
- Agent rollback compares SelectedOutbound so a failed candidate is actually replaced by the prior Active before capture reactivation. Connection enable accepts a Candidate so the first real activation can validate and commit it.
- SelfHeal now has typed fault domains and a universal MaxRepairRounds=2. Defaults, config normalization, policy budgets, and runtime budget evaluation cannot exceed two attempts; built-in policies remain observe-only.
- SelfHeal full tests PASS. Vue tests 6/6 PASS, vue-tsc PASS, Vite production build PASS.
- Error log: an attempted SelfHeal engine rewrite that would actively execute two rounds was rejected by the safety reviewer because concrete domain repair actions are not authorized by the V1 document. Replaced it with the materially safer hard-cap-only implementation; no new automatic network mutation was added.
- Error log: a PowerShell helper name R resolved to the Invoke-History alias and produced an empty patch. Renamed the helper and reran; runtime and Service response patches applied.
- Error log: the first combined Service/diagnostics patch applied runtime and Service changes before a stale diagnostics spacing anchor failed. Applied the remaining diagnostics field insertion separately.
- Error log: the first DTO patch added Route-only fields correctly in Go, then a broad TypeScript active-field replacement also touched CoreOption and a two-line Vue anchor did not match. Removed the CoreOption fields and reapplied Vue changes with exact line anchors.
- Error log: initially looked for package.json under navo_app/frontend; the project manifest is at navo_app/package.json. Ran all npm gates from navo_app.
## Phase 59 completion

### Final implementation

- Added one typed cross-domain Connection Coordinator in Agent and routed capture, node, core, source, routing-policy, core-update, restart, recovery, startup, shutdown, and scheduled capture recovery through it.
- Added observable transaction phases and regression coverage proving policy mutation waits behind capture mutation while the active phase remains readable.
- Split runtime Selected/Active/Candidate identities; health commit promotes Candidate only after verification, and failed user switching restores prior Active plus capture mode.
- Added typed SelfHeal fault domains and a hard two-attempt ceiling without enabling unspecified automatic network mutations.
- Added docs/audit/NAVO_ARCHITECTURE_BOUNDARY_V1_IMPLEMENTATION.md with implementation mapping and explicit section 22 deferrals.

### Final validation

- scripts/test.ps1: PASS (go test ./... and go vet ./...).
- Frontend tests: 6/6 PASS.
- npm.cmd run typecheck: PASS.
- npm.cmd run build: PASS.
- git diff --check: PASS; only core.autocrlf warnings were emitted.
- No elevated System Proxy/TUN mutation or live data-plane acceptance was run for this source-architecture task.

### Error log

- A fresh direct go test invocation used the empty user module cache and failed downloading proxy.golang.org dependencies. Re-ran through scripts/test.ps1 with repository .cache paths; the complete gate passed.
- A temporary SelfHeal cleanup replacement wrote a literal newline marker; removed it, ran gofmt, and revalidated.
- The first App.vue exact replacement missed because the file used CRLF; reapplied a bounded regex patch.
- The first phase-edit helper name resolved to the PowerShell Invoke-History alias and made no source change. Replaced it with bounded regex edits.
- The first verification-phase regex used the static Regex.Replace overload as if its fourth argument were a count and inserted two helper-scope references. Removed both references before compilation and added a focused phase assertion.
- Service legacy runtime tests initially exposed pre-V1 empty revision_status compatibility. activeOutboundID now treats only explicit candidate as uncommitted; full Service tests pass.
- A generated Wails transaction DTO edit was malformed during an array-to-string PowerShell conversion; replaced the complete struct boundary, ran gofmt, and added JSON decoding coverage.

# 2026-08-17 Phase 60

- Restored the existing long-running plan, added Phase 60, and retained all unrelated dirty-worktree changes.
- Read the `planning-with-files` and `ui-ux-pro-max` instructions completely; generated the required dense network-diagnostics design guidance and Vue accessibility guidance.
- Recovered the prior OpenAI routing rollout: source-level canonical route work passed, but application-flow/streaming acceptance was never completed.
- Captured current read-only process, WinINet, listener, and ChatGPT socket evidence. Navo is not running; current proxy ownership belongs to v2rayN on port 10808.
- Next: trace dashboard/capture state derivation and current OpenAI probe coverage, reproduce stale-enabled behavior in tests, then implement the narrowest repair.
- Activated `openai-docs`, searched and opened current official OpenAI documentation, and found no complete supported domain allowlist in the allowed official documentation set.
- Confirmed source-level OpenAI readiness gap: activation probes are generic Google/GitHub only. Next action is to design testable application readiness evidence and capture-state integration.
- Completed Phase 60.1 with live process/ownership evidence, source-chain proof, current OpenAI Docs lookup, and direct-versus-proxy no-credential control probes.
- Began Phase 60.2: selected an authoritative capture-readiness evidence contract and separate connection-risk versus IP-attribute summaries.

# 2026-08-17 Phase 61: complete all V1 deferred work

- User explicitly expanded scope to complete section 22 behavior and elevated acceptance.
- Restored planning-with-files instructions, V1 specification, Phase 59 evidence, current Phase 60 findings, and historical ownership/acceptance constraints.
- Started Phase 61.1 inventory. No network or process mutation has occurred.
- Error: direct apply_patch again failed with managed Windows deny-read ACLs before changing any file. Continued with reviewed temporary-copy diffs and git apply checks.

## Phase 61.1 inventory progress

- Completed the first source inventory of SelfHeal, Agent capture monitoring, Service endpoint probes, source-channel metadata, Phase 60 overlap, and current worktree scope.
- Confirmed the missing production chain: attributed two-round repair -> full verification -> same-channel on-demand discovery -> candidate transaction -> terminal error/impact report.

- Current dirty-worktree Go baseline: scripts/test.ps1 PASS, including go test ./... and go vet ./....
- Baseline confirms Phase 60 readiness changes compile before Phase 61 recovery/failover edits.
- Completed Phase 61 integration-point and acceptance-harness inventory; no live mutation performed.
- Identified a remaining Scheduler boundary violation in Service TUN monitoring; queued for Phase 61.2 repair.
- Phase 61.1 complete. Started Phase 61.2 fault matrix, two-round recovery, and reporting.

## Phase 60 completion

- Implemented capture readiness DTOs end-to-end across domain Snapshot, Service verification payloads, Agent IPC, Wails bridge, TypeScript state, and Vue UI.
- Added ChatGPT web/auth/API/assets/stream probes to System Proxy and TUN activation. Added WinINet application probes and the `runtime.verify` / `capture.verify` manual recheck path.
- Replaced the overview IP risk summary with evidence-based connection availability risk, freshness, failure reason, and recovery action. Kept IP attributes in a separate explicitly limited diagnostic panel.
- Added Go regressions for required sites, route-mode semantics, readiness parsing, and WinINet status contracts. Added executable TypeScript risk-model tests; frontend total is 9/9 PASS.
- Full portable gate PASS: Go test/vet, npm ci/test/typecheck/build, Wails/launcher/repair builds, PE/version/file-set/manifest/ZIP checks, 0 package issues.
- Browser PASS with system Chrome: ready night, ready day, failed live refresh, IP attribute separation, zero console/page errors.
- Elevated System Proxy PASS through existing v2rayN: five named application sites, default WinINet path, 15-second delayed recheck, exact rollback, v2rayN preserved, zero Navo residue.
- Released `release/Navo-1.0.30-portable-amd64` and `release/Navo-1.0.30-portable-amd64.zip`; ZIP SHA-256 `93A82680D719F56C8588681C7B975DB6439D15EDBE27CCBDAA18DD27F7DB6C2C`.

## Phase 61.2 fault matrix progress

- Added the complete V1 fault-evidence matrix, a closed Coordinator-owned repair action set, stable missing error codes, and structured evidence/round/candidate/final-impact recovery reports.
- Added matrix regressions for full domain coverage, two-round bounds, read-only domains, node-only failover, and defensive copies.
- internal/selfheal tests: PASS.
- Error: the first focused go test used the empty user module cache and yielded only a dependency download attempt. Re-ran with repository .cache environment and obtained PASS.

- First combined recovery regression exposed one obsolete single-OFF test assumption; production code compiled and Service/SelfHeal passed. Updated the test to assert exactly two activation rounds, exhausted report, and final OFF.
- Agent focused recovery tests: PASS.
- Phase 61.2 complete. Started Phase 61.3 same-channel on-demand failover.

- [2026-08-17T10:57:54.0286432+08:00] Phase61.2 complete：12-domain evidence/action matrix 与两轮 repair 已实现，selfheal/agent focused tests PASS。
- [2026-08-17T10:57:54.0286432+08:00] Phase61.3 in progress：开始实现 same-channel candidate discovery/ranking/validation/rollback。

- [2026-08-17T11:01:19.7315542+08:00] apply_patch blocked by Windows sandbox ACL; exact anchored UTF-8 rewrite fallback completed for failover API, followed by gofmt.

- [2026-08-17T11:03:07.6917503+08:00] failover focused test first run BLOCKED before compile: repository TEMP directory was missing; recreated .cache/tmp and reran with repository Go caches.

- [2026-08-17T11:04:27.3473839+08:00] failover focused test BLOCKED at dependency setup: repository module cache was empty and proxy.golang.org IPv6 downloads timed out; no compile/test result yet. Will reuse scripts/test.ps1 cache bootstrap or approved network path at full-gate stage.

- [2026-08-17T11:08:21.1026763+08:00] Phase61.3 focused gates PASS：internal/service and internal/agent failover pool/ranking/full-readiness commit tests passed via repository caches and goproxy.cn fallback.

- [2026-08-17T12:19:35.3297468+08:00] Resumed after user interruption; no residual Go process found. Phase61.3 marked complete; Phase61.4 in progress.

- [2026-08-17T12:24:55.0909326+08:00] Phase61.4 implementation complete pending gates：Wails recovery DTO, Vue recovery card/state override, explicit rule copy, and 3x3 routing matrix tests added.

- [2026-08-17T12:25:38.5763615+08:00] Phase61.4 focused gates PASS：Go service/agent/Wails tests, frontend 10/10 tests and vue-tsc passed. Phase61.5 in progress.

- [2026-08-17T12:27:06.9984860+08:00] Frontend full gate PASS：10/10 tests, vue-tsc, Vite build. First full Go gate INVALID due to test/build concurrency: navo_app go:embed read old hashed Vite assets while frontend build atomically replaced dist; rerunning Go serially against stable dist.

- [2026-08-17T12:28:11.6117769+08:00] Full source gate PASS after serial rerun：go test ./... and go vet ./... passed; frontend 10/10, typecheck, build passed.

- [2026-08-17T12:28:30.5508021+08:00] Package gate did not start because git diff --check found one trailing blank line in findings.md; normalized EOF and rerunning.

- [2026-08-17T12:30:51.7792030+08:00] Default package output BLOCKED at cleanup：release/Navo-1.0.30-portable-amd64/app_ui/navo_app.exe is locked. Preserving the running Navo process; building a separate verified 1.0.30 architecture-complete portable output.

- [2026-08-17T12:34:20.5903575+08:00] Portable package PASS：release/Navo-1.0.30-architecture-complete-amd64 and ZIP verified, 28 archive files, issues_found=0, SHA-256 979C16B5485616BAF3405D0CE87E80E8C144348215C1D9A80A041E87E4733DE6. Default output remained untouched because locked.

- [2026-08-17T12:35:39.3407189+08:00] Elevated acceptance BLOCKED pending user authority：current session is non-admin and active Navo PIDs 10008/10804 trigger the acceptance script's fail-safe. v2rayN PIDs 11564/12600 remain explicitly out of scope. Need approval to gracefully exit only those Navo processes and user acceptance of UAC; source profile will be copied into an isolated acceptance profile and original WinINet/network state verified/restored.

- [2026-08-17T12:57:49.6139074+08:00] Normal Navo exit attempts failed：tray exit timed out, CloseMainWindow did not exit, ui.exit IPC returned Access denied (current shell non-elevated). No forced termination performed; v2rayN remains preserved.

- [2026-08-17T13:01:05.4231972+08:00] Elevated ui.exit NOT EXECUTED：Windows reported UAC operation canceled; Navo PIDs 10008/10804 remain, so System Proxy/TUN acceptance cannot safely start. No network mutation or forced termination occurred; v2rayN remains untouched.

- [2026-08-17T13:11:52.6374692+08:00] Retried elevated ui.exit outcome：elevated helper returned 1 because the protected pipe closed during coordinated shutdown, but verified Navo process set is empty; accepted as shutdown outcome, not a clean IPC acknowledgement. v2rayN preserved.

- [2026-08-17T13:14:25.5002698+08:00] First System Proxy acceptance invocation INVALID：portable navo.exe requires elevation and startup was rejected before any product/network mutation. Rollback evidence passed, WinINet remained 127.0.0.1:10808, no Navo process residue; rerunning the complete acceptance inside an elevated PowerShell.


- [2026-08-17T13:27:43.2046092+08:00] Elevated acceptance diagnosed：outbound.select correctly returned V1 candidate_id while stopped, but acceptance still required legacy active_id and rejected every reachable node. Harness now accepts active/candidate selection and requires post-System-Proxy active commit; PowerShell AST gate PASS.


- [2026-08-17T13:39:42.7035005+08:00] Elevated System Proxy partial PASS：two-phase candidate was promoted only after real dataplane. MiYa upstream passed; bypass_mainland/global passed Google, GitHub, ChatGPT web/auth/API/assets/stream and WinINet application probes. Direct mode correctly failed closed because this host's direct ChatGPT TLS was forcibly closed; exact WinINet rollback passed, no Navo residue. Continuing TUN suites that do not require unavailable direct ChatGPT.


- [2026-08-17T13:47:32.7288074+08:00] First TUN golden partial PASS：activation reached HEALTH_COMMITTED and internal DNS/TCP/HTTPS plus all named ChatGPT edges passed; independent sequential ChatGPT confirmation then timed out transiently. Exact network rollback and zero process residue passed. External confirmation remains strict but now uses bounded 5x20s retry.

## 2026-08-18 Phase 62 enabled-but-no-traffic incident

- Restored the active plan, prior acceptance evidence, and dirty-worktree scope.
- Confirmed the previous TUN run stopped at partial acceptance: internal readiness passed while independent application traffic timed out.
- Started read-only live evidence capture; no process or network mutation has occurred.

- Captured the read-only live baseline: Navo absent, port 12080 absent, WinINet/v2rayN 10808 active, no connected TUN adapter, physical DNS/default route intact.
- Direct Google timed out while direct GitHub passed; explicit v2rayN Google returned 204. The host and existing upstream are usable.
- Phase 62.1 continues with bounded startup/log/profile evidence before any controlled reproduction.
- Preserved v2rayN and made no network/process mutation.

- Located the preserved 08:56 Navo failure window and its structured/runtime/core logs.
- Confirmed Navo has no current Windows boot-start registration; this will require implementation/acceptance after the traffic root cause is fixed.

- Narrowed the persisted control/data mismatch to candidate runtime state coexisting with an active local mixed listener; tracing candidate promotion and UI enable derivation next.
- Corrected the runtime-state inference against log timestamps; tracing false-positive readiness/application traffic within the committed five-minute System Proxy window.

- Completed last-run core-window classification; no DNS/dial timeout explains the report, and real proxied routes were present. Preparing controlled current-binary reproduction with exact WinINet restoration.

- Controlled current-binary System Proxy reproduction completed: proxy modes passed, direct mode failed closed, rollback/residue safety passed.
- Phase 62.2 now targets the mismatch between real proxy success and the user's unusable experience: capture scope/default action, application-level behavior, and UI/metrics truthfulness.

- Identified the first actionable product-chain defect: the main connect action always selects limited System Proxy despite presenting itself as generic full proxy activation.

- Reproduced the user-visible TUN false success on the current package with 5 independent Google timeouts after `HEALTH_COMMITTED`.
- Phase 62.2 root-cause trace now targets Service verification transport versus actual Windows route/adapter data path; full rollback and residue checks passed.

- Traced the verifier/client divergence to IP-family behavior under hard IPv6 block; preparing an instrumented active-TUN `curl -4/-6` run.

- Added and statically validated address-family diagnostics. Live rerun did not execute due to canceled UAC; continuing with Agent-side host-path verification design.

- Confirmed the implementation gap that caused false TUN success; starting the minimal Agent/WinINet repair plus regressions.

- Applied the five-file production repair and TUN current-user failure regression; gofmt and targeted diff whitespace checks passed.
- Phase 62.1/62.2 complete; Phase 62.3 focused verification in progress.
- First focused test was invalid before compile due to the wrong repository module-cache path; switching to the standard `scripts/test.ps1` cache layout.

- Focused Go tests PASS after restoring pinned dependencies to the repository cache.

- Correlated failed curl to TUN and SOCKS core entries; moved the data-plane repair from route/IPv6 speculation to burst-concurrency control plus independent host verification.

- Applied sequential Service external probes and added a max-concurrency regression; gofmt and `git diff --check` passed.
- Focused tests PASS: `internal/service`, `internal/agent`, `internal/agent/systemproxy`, and `cmd/navo`.
- Phase 62.3 complete; Phase 62.4 full gates/package/live acceptance in progress.

- Changed the primary connect action from hidden System Proxy to full-device TUN and added a regression; frontend PASS 11/11 plus typecheck/build.
- Built and verified `release/Navo-1.0.31-proxy-dataplane-amd64` plus ZIP: 28 files, 27 manifest entries, 0 issues.
- Final ZIP SHA-256: `A70C9D1806E47B293C13398C1D2ACE7B926D9BD2C25C2E5ECD499A2E3C062A93`; Navo launcher/repair/UI PE versions are all 1.0.31.0.
- Repaired-build elevated TUN acceptance BLOCKED: UAC canceled before script startup. No network mutation occurred.
- Verified post-cancel residue: no Navo process or 12080 listener; v2rayN process/listener and WinINet 10808 preserved.

## Phase 63 SelfHeal trigger repair

- Confirmed the missing trigger spans two gaps: old TUN false-positive observation and current activation failure ending at Faulted/OFF before the post-commit monitor can schedule SelfHeal.

- Implemented activation-to-SelfHeal handoff after Coordinator release, failed-activation eligibility, and host-timeout versus adapter fault attribution.
- Added regressions for recovery on the first bounded repair round, two-round exhaustion/final OFF, classification, canceled context, non-admin rejection, and existing running-path readiness failure.
- Focused `internal/agent` SelfHeal trigger suite PASS.

## 2026-08-20 Phase 64

- Restored the existing planning files, Navo release-acceptance constraints, current dirty-worktree scope, and the last repaired-build acceptance boundary.
- Added the Phase 64 System Proxy/TUN/blacklist/whitelist closure matrix. No live network or process mutation has occurred in this phase yet.
- Next: capture a read-only live ownership baseline and map the UI-to-core execution paths before deciding whether production repairs are still required.
- Read both repository proxy/TUN acceptance guides completely. Their hard gate remains ordinary data-plane traffic plus exact recovery; configuration/UI/listener evidence is intermediate only.
- Captured the initial non-admin baseline: Navo absent, v2rayN and its sing-box preserved, no Navo adapter observed. Continued with source-only mapping; no live mutation has occurred.
- Mapped UI/Wails/Agent/Service list-mode and routing-rule paths. They are implemented as serialized runtime mutations with active-config swap, verification, healthy commit, and rollback rather than frontend-only state.
- Confirmed the elevated acceptance script includes matching-destination behavioral assertions for blacklist/whitelist under both System Proxy and TUN. Next: validate compiler output/core capability mapping and run the deterministic source gates.
- First `scripts/test.ps1` run was interrupted during pinned-module preparation before tests produced a verdict. The PTY session was gone when resumed; checking for orphaned test processes before one clean rerun.
- Second full Go gate was blocked at dependency setup by `proxy.golang.org` timeouts for pinned `go.sum` modules. This is an environment failure, not a PASS or product test failure; preparing the repository cache through the established mirror before an offline rerun.
- Pinned module cache bootstrap through `goproxy.cn` PASS. Offline full Go gate PASS: `go test ./...` and `go vet ./...`; Service completed in 30.865s and all connection/routing/network/self-heal packages passed.
- Moving to frontend behavior/type/build gates, then browser-visible state and interaction checks.
- Frontend PASS: tests 11/11, `vue-tsc --noEmit`, and Vite production build. Production assets generated successfully; starting browser-visible System Proxy/TUN/list-mode interaction verification next.
- `webapp-testing` helper usage was checked as required. Python Playwright is unavailable and the project intentionally has no Playwright dependency; locating the environment-bundled automation runtime used by prior Navo browser acceptance.
- Bundled Playwright plus system Chrome launched successfully. The first run completed capture/list interactions and failed only on a non-exact theme selector; tightened the selector before rerun.
- Browser smoke PASS: System Proxy/TUN and blacklist/whitelist/off calls, rule-save payloads, selected ARIA state, both themes, screenshot, and zero browser errors all verified. Screenshot retained under ignored `.cache` evidence.
- Began Phase 64 backend repair: identified a post-policy-swap Agent host-path verification gap that is not covered by the existing Service verifier. Designing exact previous-policy rollback plus fail-closed OFF behavior and regressions before packaging/live acceptance.
- Completed the active-policy host-path transaction repair and three regressions; targeted Agent tests and the full offline Go/vet gate PASS.
- Advanced VERSION to 1.0.33 because 1.0.32 is the prior SelfHeal deliverable. First package attempt passed all project and frontend gates but stopped before release creation while rebuilding Wails CLI with network lookup disabled; rerunning with the validated Go mirror.
- Wails dependency retry through `goproxy.cn` PASS. Built and verified `release/Navo-1.0.33-portable-amd64` and ZIP; archive SHA-256 is `E49D06916346FC50D852BC409F56FE5412B82642A42A1BA968C9062CBC9CE8A3`.
- Added exact method-boundary rollback coverage for base mode, list mode, and both rule arrays. Focused Agent tests plus the final complete Go/vet gate PASS; all 17 PowerShell scripts parse with zero errors.
- Packaged System Proxy/TUN live matrix remains pending: the first non-admin launch was correctly rejected and rolled back cleanly, then the consolidated 11-case elevated wrapper was canceled at UAC before startup. Verified zero Navo residue and unchanged v2rayN/WinINet ownership afterward.

## 2026-08-20 Phase 65

- Started the user-requested V1 architecture normalization. Fully read the architecture specification and planning skill, restored existing plans/diffs, and read the prior V1 audit memory/rollout summary.
- Added a six-part implementation and acceptance plan. Phase 65.1 is in progress: re-map current mutation/state ownership after the substantial Phase 61–64 changes before editing production code.
- Logged the managed ACL read failure, output truncation, and patch-tool failures; resolved them with approved bounded reads and a repository-scoped standard unified diff. No product/network state has been mutated in Phase 65 yet.
- Enumerated the current control surface. Agent coverage is broader than the prior V1 audit, but Service domain locks and Service-side observer/SelfHeal plumbing require direct caller tracing before the single-coordinator requirement can be marked complete.
- Read the Coordinator, Agent dispatch/transaction implementations, Service dispatch/mutators, TUN scheduler, failover pool, and SelfHeal defaults. Confirmed a real single Agent transaction token and request-only scheduler foundations; continuing with update, tray, monitoring, and Active/Candidate edge cases.
- Audited monitor/SelfHeal, tray connection aliases, source updates, capture transition, and Wails core update. Identified three actionable production gaps: tray primary capture mismatch, legacy background-context aliases/SelfHeal origin, and non-atomic multi-call core update orchestration.
- Confirmed explicit Candidate/Active commit is healthy, then found a backend-only async subscription apply path that can outlive the Agent transaction. Current UI forces synchronous wait, but Phase 65 will enforce the boundary in Service rather than rely on callers.
- Reviewed candidate rollback and source/core-update tests. Kept the verified Service rollback design, and narrowed changes to transaction callers plus an Agent-owned bounded core-update critical section.
- Re-read Phase 65 before the major design decision and completed the bounded core-update session design; implementation will start with request-context/tray/SelfHeal/source fixes, then add session regressions.
- Completed Phase 65.1 ownership map and moved Phase 65.2 into implementation.
- Unified tray primary connection with TUN, propagated request cancellation through runtime observation and all legacy capture aliases, and corrected SelfHeal transaction origin.
- Removed Service-side asynchronous subscription runtime applies; non-wait add is persistence-only and non-wait refresh fails closed.
- Added boundary regressions and passed focused offline tests: `internal/agent` 2.570s and `internal/service` 23.846s.
- Logged and resolved the ACL/CRLF patch-transport failures with a repository-scoped LF patch file that is deleted after each `git apply`; target files pass formatting and whitespace checks.
- Starting Agent-owned core-update session implementation; no live process, proxy, route, DNS, adapter, or WinINet state has been changed in Phase 65.
- Implemented Agent-owned core-update begin/commit/rollback sessions across Wails file replacement, plus timeout and shutdown recovery; removed UI raw stop/start access.
- Added and passed core-update serialization, commit, rollback, timeout, raw-step rejection, timer-cleanup, and IPC-timeout regressions.
- Completed a differential audit and fixed the timer goroutine leak plus deferred rollback retry state.
- Fixed Candidate/Active authority: Agent no longer treats selected Candidate as Active, Service rollback snapshots the verified Active, and inconsistent live capture without Active fails closed.
- Added and passed failed-node restoration and candidate-policy preservation tests; Agent passed in 2.368s and Service passed in 29.979s on the confirming runs.
- Reconfirmed same-source-channel failover isolation, backend capture mutual exclusion, capture-independent routing policy matrices, Navo-only ownership, observer-only Scheduler/Service behavior, two-round SelfHeal cap, evidence reporting, and fail-closed exhaustion through source plus passing package suites.
- Phase 65.2, 65.3, and 65.4 are complete. Phase 65.5 full Go/vet/frontend/browser/PowerShell/package gates is in progress.
- No live proxy, route, DNS, firewall, adapter, process, or WinINet mutation has occurred in Phase 65 yet.
- Repaired route verification after live direct-mode testing exposed two false requirements: Service and Agent no longer demand ChatGPT/OpenAI in direct mode; focused and full Agent/Service/SystemProxy tests pass.
- Added strict Candidate/Active acceptance assertions, list-mode `off` idempotency, explicit `changed:false` handling, and bounded application-probe retry coverage.
- Final full package gate PASS for 1.0.35: all Go packages and vet, frontend 11/11 plus typecheck/build, Wails Windows/amd64 build, and portable directory/ZIP verification (28 files, 27 manifest entries, 0 issues).
- Delivered `release/Navo-1.0.35-portable-amd64.zip`, SHA-256 `5F8156F422241110091C5F60E9FDEF2C31E398881EA955B6CD255201D420B1E9`.
- Real System Proxy acceptance PASS on sing-box, Mihomo, and Xray: direct/whitelist exit `115.227.89.28`; Global/blacklist/list-off exit `63.124.165.24`; Google 204, GitHub 200, ChatGPT 403, OpenAI API 401; exact WinINet rollback and zero Navo residue after every run.
- Phase 65.5 complete. Phase 65.6 remains pending because the formal 11-case elevated runner was canceled at UAC before startup; verified no TUN/network mutation and preserved v2rayN/listener ownership.

## 2026-08-20 Phase 66

- Received a real updater failure: both Windows System Proxy and direct routes timed out while reading the core asset response body.
- Restored planning records and prior updater constraints. Beginning a source-level timeout/route audit before any live core or capture mutation.
- Confirmed the matching implementation defect: `http.Client.Timeout=45s` covers full asset-body reads, while one shared 2-minute context spans metadata, all routes, download, and validation.
- Proceeding to existing updater regression inspection and a controlled slow-progress/stalled-body test design; no running core or proxy state has been touched.
- Implemented the first bounded downloader repair: no whole-body `http.Client.Timeout`, 45-second metadata budget per route, 10-minute archive budget per route, 30-second reset-on-progress body inactivity timeout, and application-lifecycle cancellation.
- Added slow-progress, stalled-body, cancellation, independent fallback-budget, and zero-client-timeout regressions.
- Focused Windows/amd64 offline updater tests PASS: `ok navo/navo_app 1.572s`. Tightening one timer/progress race before the full package gates.
- Replaced timer/progress channel arbitration with an atomic last-progress timestamp; 10 consecutive focused runs PASS (4.520s) and the complete `navo_app` package PASS (1.558s).
- Official updater-path live download PASS through `http://127.0.0.1:10808`: sing-box 1.13.19 archive, 21,046,252 bytes, 203.18s, integrity/archive/executable extraction verified, no mutation.
- Full Go/vet gate PASS after the final downloader code; package gate also passed all Go packages, frontend 11/11, npm audit, Vue typecheck/Vite, and Wails Windows/amd64.
- Built and verified `release/Navo-1.0.36-portable-amd64` plus ZIP: 28/27/28 file counts, zero issues, PE versions 1.0.36.0, ZIP SHA-256 `1C1E5C0F4636FC59733FD33E6FE050925943FF0472C8D3196B4AA0D494AB74E4`.
- Phase 66 complete. Preserved the Explorer-owned running Navo pair and v2rayN/10808 state; the user must exit the older running Navo and launch 1.0.36 to use the fix.

## 2026-08-23 Phase 67

- Began the requested network-environment self-healing improvement.
- Fully read the planning-with-files skill and restored the existing planning-file head/tail/status context.
- Added Phase 67 before product exploration. No product code, process, proxy, route, DNS, firewall, adapter, listener, or WinINet state has been changed.
- Restored Git scope: the worktree already contains extensive user changes, including an untracked UI thin-layer refactor; these will be preserved and mapped before overlapping edits.
- Queried the relevant memory index and confirmed Coordinator-only mutation, ownership-safe recovery, and real-data-plane acceptance requirements.
- The first full-document read hit an output truncation limit. Recorded the 3,021-line count and SHA-256 and switched to bounded chunk reads; implementation has not started.
- Completed bounded reads for document lines 1-1500. Confirmed observation/mutation ownership split, snapshot model, refresh strategy, analyzer/findings, typed Self-Healing adapter, takeover semantics, and startup ordering.
- Completed bounded reads for lines 1501-3021; the entire specification is now covered without truncation.
- Phase 67.1 continues with current-HEAD symbol/state ownership mapping before product edits.
- Verified exact baseline HEAD and inventoried the overlapping dirty worktree. No existing product changes were overwritten.
- Read the canonical Coordinator/Capture/System Proxy/Dashboard and Self-Heal implementations. Identified safe read-only aggregation and Agent typed-adapter seams.
- Mapped Agent startup, health polling, manual recovery, typed fault construction, two-round recovery, rollback/failover, and fail-closed behavior.
- Mapped Service startup/IPC/TUN state and Network Manager/Journal authority. A read-only journal summary plus platform observer is required; no duplicate persistence will be introduced.
- Mapped the current frontend thin-layer boundaries and confirmed Dashboard normalization/Overview composition as the low-conflict Environment UI path.
- Confirmed the existing owned-network repair executor and identified the special off-mode verification needed for manual residual repair through the same Self-Heal loop.
- Focused Go baseline initially stopped before compile because the repository module cache was empty and offline mode was forced; pinned dependency bootstrap resolved it.
- Backend baseline PASS for Agent/Service/Network/Self-Heal; frontend baseline PASS 19/19 plus TypeScript.
- Phase 67.1 complete. Phase 67.2 implementation is in progress.
- The CRLF-sensitive combined planning patches were rejected without mutation; exact zero-context plan/status updates succeeded.
- Implemented and formatted the read-only internal/networkenv model, Analyzer, and Store.
- networkenv tests PASS, including all eight document cases, partial snapshot behavior, defensive copies, and stale reads.

- Completed Phase 67 source implementation across networkenv, Network, Service, System Proxy, Agent SelfHeal, Wails, and the Vue thin-layer UI.
- Added focused tests for the eight Analyzer cases, Store staleness/copying, Journal observation and no-mutation commands, Service dispatch cancellation, WinINet ownership loss/corruption, typed finding mapping, stale/partial/transitional monitor behavior, external observation-only handling, API delegation, DTO compatibility, and UI repair guards.
- Confirmed full Go test and vet PASS, navo_app PASS after locked Wails dependency cache bootstrap, frontend 22/22 PASS, typecheck/build PASS, and browser-level Edge/Playwright PASS with a saved screenshot.
- Preserved all pre-existing dirty-worktree changes and did not stop or alter any running Navo/v2rayN process or live Windows networking.
- Phase 67.5 remains intentionally open for elevated/manual A-I network mutation and real data-plane/rollback/package acceptance; source completion is not represented as physical TUN acceptance.

## 2026-08-24 Phase 68

- Restored the planning-with-files instructions and existing Navo planning state.
- Queried the relevant Navo memory index and retained the Coordinator, ownership, rollback, and physical data-plane acceptance constraints.
- Added a bounded four-step optimization phase after both native `apply_patch` and its Windows wrapper were blocked by the managed ACL environment; used a reviewed repository-scoped unified diff fallback.
- The first combined findings/progress append used the wrong findings EOF anchor and was rejected atomically; re-read both exact tails before this corrected patch.
- Captured baseline HEAD `a337738`, the full dirty-worktree file list, diff scale, and project instructions without altering any user-owned source change.
- No product code, process, listener, proxy, route, DNS, firewall, adapter, or WinINet state has been changed.
- Compared frontend lazy loading with backend observation batching and selected the higher-impact, lower-overlap backend hotspot.
- The previous combined status patch omitted `--unidiff-zero` for zero-context hunks and was rejected atomically; this corrected invocation preserves the reviewed changes.
- Next: inspect fake-executor behavior and Journal action bounds, then implement red regressions before production code.
- Implemented structured single-process Journal observation in `internal/network/observation.go`.
- Added a command-count/mixed-state regression, item-local error regression, ambiguous-result rejection matrix, and a Windows real-PowerShell contract test.
- Resolved the empty local module cache through the established locked-dependency mirror bootstrap and switched tests back to `GOPROXY=off`.
- Focused Observe/decode suite PASS: `ok navo/internal/network 0.715s`.
- Windows generated-script contract PASS: one hidden PowerShell process, exact healthy/error JSON results, `ok navo/internal/network 2.354s`.
- No live networking query/mutation was performed by the contract test; its scripts contain only constant EXACT and injected-error expressions.
- Phase 68.4 full package-level Go test/vet and final diff verification are now in progress.
- Extended the command-count regression to four Journal resources; final focused contract/count run PASS: `ok navo/internal/network 2.031s`.
- Full Go gate PASS with preserved session output: every package passed and `GO_TEST_ALL_EXIT=0`; Service completed in 33.964s and Network in 9.816s.
- Full vet gate PASS: `GO_VET_ALL_EXIT=0`.
- Frontend gate PASS: 22/22 tests, `vue-tsc --noEmit`, and Vite production build in 3.05s.
- Final diff whitespace check produced no error; the large pre-existing dirty worktree and CRLF warnings were not normalized or overwritten.
- Phase 68 is complete. No launcher, proxy, core, route, DNS, firewall, adapter, WinINet, or user process was started, stopped, queried for mutation, or changed.
- Remaining scope: Phase 67 elevated A-I/TUN physical data-plane and rollback acceptance is still pending and was not represented as completed by this read-only observation optimization.

## 2026-08-24 Phase 69

- Received five repeated Service/IPC request-failed events from the user, all between 20:09:19 and 20:09:57.
- Restored planning state and the prior Named Pipe/request-ID constraints.
- Starting read-only timestamp correlation; no process or network state has been changed.
- Read-only process baseline found no Navo process; preserved v2rayN PID 12972 and its sing-box PID 23476.
- Bounded release-log inventory found only an 8/23 portable log, while `%LOCALAPPDATA%\\Navo\\structured.log.jsonl` is current.
- Windows Application event query for 20:08-20:11 returned no Navo-related crash event.
- The first all-file local-data inventory entered WebView2 caches and was truncated; subsequent reads will target exact Navo log/state paths only.
- Next: parse sanitized structured log entries around the five timestamps and correlate their method/error/request identifiers.
- Confirmed the local structured log ends on 8/23 and no Navo/WebView2 file in that data root changed during the five 8/24 timestamps.
- Mapped Wails per-call pipe dialing, UI request IDs, Agent child Service request IDs, three reconnect attempts, single `serviceMu`, and detailed Agent error fields.
- Source mapping shows Phase 68 background observation failures are stored as environment observation errors rather than emitted as Agent UIIPC request-failed events.
- The user's generic card is consistent with a Service-side `request failed` entry whose fields are hidden from the pasted view; exact method/code/reason remains unproven.
- Next: inspect the Service failure emitter/log path selection and Windows execution-history evidence for the actual binary/data root.
- Found the exact generic emitter: `Service.errorResponse` logs only request ID and code, omitting method and reason.
- Confirmed Agent errors already include a redacted reason, exposing an evidence parity gap.
- BAM execution-history values were access denied; retained other read-only evidence paths rather than expanding registry privileges.
- Next: trace errorResponse call sites/dispatch boundaries and the Vue logs presenter, then design one enriched, backwards-compatible error record.
- Counted 110 Service errorResponse call sites; rejected a broad signature rewrite.
- Confirmed Service request-received entries already contain method and request ID, while error entries omit reason and the Vue list hides all structured fields.
- Chosen backwards-compatible observability seam: enrich reason in the existing error fields and render a safe bounded field subset.
- Next: correlate dashboard polling/fan-out cadence with normal-state Service error producers before implementing the evidence repair.
- Correlated the traffic page to a 2-second full-dashboard poll.
- Proved `metrics.current` is availability-as-data, not an error, when stopped/unsupported; it is not the direct generic Service error producer.
- Dashboard still aborts on any other fan-out Service error.
- Next: inspect log-follow cadence/validation and the remaining status handlers before choosing any behavioral polling repair.
- Ruled log follow cadence out as the direct 2-second pattern: it polls every 3 seconds.
- Confirmed logs.query defaults are valid and errors only on explicit invalid filters.
- Continuing with dashboard status-handler audit before source edits.
- Audited all five dashboard status handlers; normal disconnected/unavailable states return RESPONSE payloads, not Service errors.
- Proved fixed 2-second traffic polling silently retries every failure and lacks an in-flight guard/backoff.
- Completed Phase 69.1 with a hard historical evidence gap: no matching current log file or process remains, and the UI hid the persisted fields.
- Phase 69.2 now targets combined-launcher transport ownership plus a safe reproduction of Service rejection/poll retry behavior.
- Confirmed formal desktop Agent→Service calls are direct in-process Dispatch; ruled out a second Service Named Pipe transport failure.
- Classified the cards as handler ERROR responses whose exact historical cause was destroyed by missing method/reason evidence.
- Completed Phase 69.2 without guessing the original handler.
- Starting Phase 69.3 evidence enrichment and bounded non-overlapping traffic polling implementation.
- Selected pure helpers and test seams for Service error fields, safe log evidence formatting, and bounded polling delay.
- Located structured-log grid CSS for a compact second evidence row.
- A read-only package manifest lookup used the wrong frontend subdirectory; corrected future gates to `navo_app`.
- Recovered from managed ACL read/apply denial using approved repository-scoped reads and atomic `git apply` standard input.
- Applied the Service evidence enrichment plus focused pure-helper tests; no response, request-ID, replay, pipe, or network behavior changed.
- Logged and corrected two atomically rejected patch-format/PTY attempts; neither partially modified source.
- Located the actual Settings feature and LogEntry type files after two stale read-only paths.
- Finalized the frontend boundary: compact safe log evidence plus non-overlapping bounded failure retry.
- Preserving current dashboard response handling and normal 2-second healthy metrics refresh.
- User clarified the target architecture and reported IP-trace/discovery still proves no successful proxy path.
- Paused the Phase 69 frontend-only retry/presentation work; retained the already applied Service evidence helper.
- Read the complete V1 architecture specification, implementation mapping, and cold-boot acceptance constraints.
- Started Phase 70 audit of capture ownership, mode-specific verification, and healthy commit semantics.
- Located the Agent/Service mode transitions, runtime healthy commits, TUN verifier, and capture health monitor.
- Next: inspect the exact readiness predicates and determine why direct exit can still be committed as healthy.
- Found the first concrete false-success boundary: initial activation skips the only formally wired current-user route probe.
- Found the second acceptance gap: System Proxy application checks lack exit-IP identity comparison.
- Chose the minimal ownership-preserving repair instead of merging Service and Agent state: Service certifies the node; Agent exclusively commits the capture valve.
- Next: add explicit-proxy and WinINet exit identity predicates, then wire the mode-aware probe into initial activation.
- Mapped compatible DTO additions and regression seams for Service explicit exit identity and Agent initial route probing.
- Ready to implement the focused Phase 70 valve/acceptance repair.
- Applied the Service explicit proxy exit-IP identity repair and compatible verification evidence fields.
- Applied the Agent initial mode-aware route probe gate plus runtime-mode evidence.
- Applied WinINet default-versus-direct exit identity verification; starting focused regression coverage and compilation.
- Split and applied focused regressions after one atomically rejected combined test patch.
- Restored the three Phase 70 constraints that were omitted by an earlier under-counted planning hunk.
- Focused gate pass so far: `internal/service` and `internal/agent/systemproxy`.
- Corrected the Agent regression to use public runtime-mode wire values; rerunning Agent tests.
- Full `internal/agent` package: PASS after test-contract correction.
- Completed Phase 70.1-70.3; starting full Go/vet and then live data-plane validation if a configured Navo route is available.
- Full Go test gate: PASS.
- Full Go vet gate: PASS; moving to read-only live route/process baseline before any runtime launch or capture mutation.
- Read-only live baseline confirmed no running Navo and preserved v2rayN/sing-box.
- Located stale Navo runtime state and release 1.0.37; validating only safe route/commit metadata before any launch.
- Confirmed the current Navo runtime has no selected/active/candidate outbound and is an uncommitted candidate.
- Found one local repository state file; checking only counts/IDs/source types to see whether a selectable route exists without exposing credentials.
- Repository structure confirms only historical revisions, not a current selectable route/active selection.
- Live proxy-success acceptance remains blocked by missing Navo route; continuing with non-mutating source/build validation and exact residual reporting.
- Frontend 22/22 tests, typecheck, and production build: PASS.
- Phase 70 source work is validated; live real-IP completion is pending a configured/selected Navo route (and elevation for TUN).
- Applied Phase 69 frontend log evidence and bounded metrics polling helpers/tests.
- One touched Vue row has a mixed-indent warning; normalizing it before frontend gates.
- Metrics polling tests: PASS.
- Log presenter test file encoding: FAIL from patch transport; fixing with Unicode escapes before typecheck/build.
- Safely replaced only the two encoding-damaged new files after Git refused to modify untracked paths.
- Recreated log presenter and regression with ASCII-safe Unicode escapes; rerunning complete frontend gates.
- Completed Phase 69 backend/UI evidence and bounded polling work; frontend 26/26, typecheck, build all PASS.
- Final diff whitespace check PASS; Phase 70 live IP acceptance remains pending only because Navo has no selected route.

- Confirmed Navo has no selected/active/candidate outbound; external v2rayN configuration was intentionally not read or reused.
- Bumped the continuing release version to 1.0.38 so the repaired package does not overwrite 1.0.37.
- Completed official packaging after caching fixed Wails dependencies: full Go tests/vet, frontend 26/26, typecheck, build, package/ZIP verifier, and repair check PASS.
- Ran isolated `lightweight_smoke.ps1 -SingleUI`: IPC/UI/tray lifecycle PASS; no-route System Proxy returned `OUTBOUND_REQUIRED`, committed mode stayed off, and TUN fault stayed empty.
- Verified post-smoke residue and removed the single smoke-created package log; directory/ZIP verification still reports zero issues.
- Real System Proxy and elevated TUN DNS/TCP/HTTPS/exit-IP validation remains pending solely on a configured Navo route (and elevation for TUN).

- Correlated the reported request IDs and two-second cadence to active capture health `runtime.verify` calls racing Service TUN transition state.
- Added Service capture-lock serialization for external runtime verification plus `TestRuntimeVerifyWaitsForCaptureTransition`.
- Focused regression, complete Service/Agent suites, full Go tests/vet, frontend 26/26, typecheck, and production build PASS.
- Built and verified 1.0.39 directory/ZIP; repair check and manifest/archive checks report zero issues.
- Two lightweight smokes passed IPC/UI/no-route failure boundaries but failed tray exit automation; stopped retrying, verified zero Navo residue, preserved v2rayN/WinINet, and cleaned smoke logs.
- Pending: real administrator TUN data-plane rerun with a configured Navo outbound, plus independent tray-exit lifecycle diagnosis.

## 2026-08-25 Phase 72

- Received the intermittent TUN-enable overview data-loss report.
- Restored the planning-with-files workflow, current dirty-worktree scope, and the relevant dashboard/request-ID historical constraints.
- Read both relevant historical rollout summaries: Phase 36 dashboard resilience and the Agent child request-ID repair.
- Initial code search identified `internal/agent/dashboard.go` as a serial all-or-nothing fan-out and the refactored overview/runtime state modules as the frontend projection boundary.
- Starting exact source reads and red regressions for TUN-transition overlap.

- Confirmed `loadDashboard` has no in-flight guard, request generation, or stale-response rejection; every completion overwrites the whole dashboard.
- Confirmed TUN switching creates a fixed 350 ms interval of full dashboard requests and only clears it after the long-running capture mutation finishes.
- Confirmed Agent dashboard aggregation is five serial Service calls with per-call locking, so multiple overview requests can interleave at component granularity.
- Root-cause direction: replace refresh fan-out with a single-flight trailing refresh that commits only the newest generation, then add transition-overlap regressions before changing production behavior.

- Added failing regressions for overlapping dashboard requests, stale transition success, stale transition error, temporary route identity loss, and fixed-interval capture polling.
- Applied the shared-loader, route-history, and polling changes after the required patch tool and Git patch route were blocked by managed ACL/untracked-file constraints; every fallback edit used workspace path checks, unique anchors, full preflight, and reread-before-write checks.
- One combined Git patch had incorrect hunk counts and was rejected atomically; the corrected patch precheck then refused an untracked feature file without applying any hunk.
- The first cumulative exact-edit preflight evaluated same-file replacements independently and stopped before writing; the corrected per-file cumulative preflight applied all intended changes.
- Focused regressions PASS 6/6.
- Full frontend suite PASS 30/30.
- First production build FAIL: useTraffic.ts(119,28) could not find name apis; direct dashboard access was removed correctly, but controlled traffic still uses the typed API facade.
- Restored only the required apis import. TypeScript plus Vite production build PASS (67 modules).
- Phase 72.1-72.3 complete. Starting browser-level mocked TUN transition timing verification without changing live networking.

- Browser automation skill path initially failed because neither system nor bundled Python contained Playwright; no package install was attempted.
- Reused bundled Node Playwright and headless Edge with the prescribed managed-server lifecycle.
- Mocked TUN transition browser regression PASS: DASHBOARD_CALLS=6, MAX_CONCURRENT=1, TRANSITION_DATA_LOSS=0, final TUN and stable revision, zero console/page errors.
- The managed image viewer could not read the screenshot due the same ACL helper failure; retained test assertions and copied the screenshot to the task visualization directory.
- Final full frontend rerun PASS 30/30; prior successful TypeScript/Vite build remains valid after the sole API import restoration.
- Final source boundary check found exactly one Wails dashboard bridge call, in the shared loader.
- Detected the helper's residual Vite listener on 4188, verified PID 6928 as this exact workspace test child, stopped only it, and confirmed zero listeners.
- Screenshot evidence: 551123 bytes, SHA-256 ED4FD13E847B7D4DA396400F5806D0083801B32742B4A248B68D107B81F9185B.
- Removed three temporary browser-test cache files; retained only the copied screenshot.
- Phase 72 complete. No live networking or unrelated process was changed; elevated physical TUN acceptance remains separate.

## 2026-08-31 Phase 73

- Received the request for classified log persistence, user level handling/filtering, and Basic Service as the default log view.
- Restored the planning-with-files workflow, read repository rules, checked the relevant Navo memory index, and confirmed a clean worktree.
- Added Phase 73 with explicit compatibility, redaction, legacy-record, and non-destructive-filtering constraints.
- Starting the canonical logger/query/Wails/Vue contract map before changing product code.
- Read the complete relevant log-enhancement specification plus the canonical store, Wails DTOs, composable, Settings template, and current store tests.
- Confirmed severity is already persisted and backend-filtered; the missing visible behavior is a named Basic Service grouping/default rather than a new destructive log operation.
- Continuing producer-to-category mapping and regression seam selection before code changes.
- Completed the canonical producer map: structured logs currently originate as Launcher, Agent, Service-derived method services, and SelfHeal, with deterministic service names available before persistence.
- Ran the required UI/UX design-system, Vue, and accessibility searches. The change will preserve the existing dense diagnostic surface, use semantic labeled controls, keep focus/loading behavior visible, and avoid a broad visual restyle.
- Phase 73.1 is complete. Starting red regressions for persisted category, category query, legacy log compatibility, and Basic Service as the frontend default.
- Added red store regressions for persisted category, category filtering, and deterministic legacy JSONL classification, plus frontend regressions for Basic Service default/labels and distinct service-level versus severity controls.
- Frontend red state is confirmed: 0/2 PASS because the category module and Settings controls do not exist yet.
- The Go red run is blocked before compilation by the empty/missing `x/sys` cache and a failed `proxy.golang.org` download; its source references intentionally require the not-yet-implemented Category contract.
- Phase 73.2 is complete. Implementing the canonical store/query/DTO/UI classification path now.
- Implemented category persistence/legacy derivation/filtering in `internal/logstore`, category validation and metadata IPC in Service/Agent, compatible Wails/TypeScript DTO additions, and the Basic Service frontend default.
- Updated the Settings log UI into separate “日志级别 / 服务分级 / 具体服务” semantic fieldsets, added textual default guidance, category labels on rows, and reused existing theme tokens/layout.
- Ran gofmt on all touched Go files.
- Focused frontend category/API tests PASS 4/4.
- Found the established repository-local Go caches at `.cache/go-build` and `.cache/go-path`; the next Go rerun will use them with `GOPROXY=off`.
- Focused offline Go suites PASS: logstore 0.587s, Service 45.013s, Agent 3.144s.
- Reviewed the complete product diff; `git diff --check` PASS with only repository CRLF conversion notices, and no unrelated source file is modified.
- Normalized the newly touched TypeScript/Vue indentation after diff review.
- Phase 73.3 is complete. Starting full Go test/vet and frontend test/typecheck/build gates before browser-visible verification.
- Full Go test gate PASS across every package using repository-local offline caches.
- Full Go vet gate PASS.
- Full frontend gate PASS: 32/32 tests; Vue TypeScript check and Vite production build PASS with 68 modules in 3.00s.
- Read the complete webapp-testing skill and ran its required `with_server.py --help`. Python Playwright is unavailable, so the browser check will reuse the repository's established Node Playwright/Wails mock approach.
- Located the bundled Node Playwright runtime and existing Navo Wails bridge mocks. Preparing a dedicated logs UI smoke that asserts the initial Basic Service query, severity/category interaction, console cleanliness, day/night rendering, and compact-layout overflow.
- Added `scripts/logs_ui_smoke.cjs` with filtered mock JSONL entries for Basic Service, Network & Capture, and Core Runtime.
- First browser attempt reached the Vue server but timed out on an exact visible-label selector because the navigation button intentionally exposes a longer aria-label. Corrected the selector from source evidence.
- The helper left its Vite child on port 4193; verified PID 28280 as this task's exact workspace/port child, stopped only it, and confirmed `VITE_LISTENER_4193=0`.
- Second browser smoke PASS: initial query category `basic_service`, default INFO/WARN/ERROR, category switch to `network_capture`, concrete TUN filter, severity exclusion, two themes, 760x900 compact layout, zero horizontal overflow, and zero console/page errors.
- The helper again left its directly launched Vite process. Verified PID 27652 by exact command/port/start time, stopped only it, and confirmed `VITE_LISTENER_4193=0`.
- Screenshot files and SHA-256 values were generated and copied to the task visualization directory. The managed image viewer remained blocked by the known ACL helper failure, so manual pixel inspection is not claimed; browser layout/theme/DOM assertions remain the visual acceptance evidence.
- Added an explicit pre-interaction Basic Service night screenshot to the reusable smoke for final evidence.
- Re-ran the final browser smoke under an interactive Vite session and closed it with explicit Ctrl+C; `LISTENER_COUNT=0`.
- Final screenshots: default night `07053E41C23732D15D9C4716B5156939C17D343C5FB438A5FFBE5B02452CD983`; filtered night `1F903157D2270269C139C72E65512CDBB7AC595425EB7C0DEA553C6CD5B1D9C9`; filtered day `9150AA3369CEDFFBF2A1FAFEEF77F480DB2390268B5F28889DFBE10BFBB2A475`; compact day `72288C8569B0D33F891C790C370ABF4D7129956A4A2EC29CA3904A0C1EFBD6D4`.
- Updated README with the structured JSONL path, persisted severity/service classification, combined filtering, and Basic Service default behavior.
- Final smoke script syntax and `git diff --check` PASS; only repository CRLF conversion notices remain.
- Phase 73.4 is complete. Bumped VERSION from 1.0.39 to 1.0.40 and starting the official portable directory/ZIP packaging path.
- Official package script PASS: repeated full Go/frontend gates, Wails production build, directory verifier, archive verifier, and npm audit (0 vulnerabilities).
- Built `release/Navo-1.0.40-portable-amd64` and matching ZIP; directory/archive both report `issues_found=0`.
- `repair.exe check` PASS with zero issues. All three Navo-owned PE files report 1.0.40.0.
- Final ZIP SHA-256: `66D57EC1334E375CBD5273A7C6CC9E87112CDE4FB246672B64E37D7F0FBA6517`; no Navo process is running.
- Phase 73 is complete.

## 2026-08-31 Phase 74

- Received the V2 overview consistency report: one proxy is shown as two boxes, traffic is attributed to a local proxy, and exit detection is absent.
- Restored repository rules, historical active/candidate and Dashboard boundaries, and the completed Phase 73 dirty-worktree scope; no existing changes were reverted.
- Read the complete planning and UI/UX skill instructions and ran the required local design/chart/Vue guidance searches.
- Added Phase 74 with canonical active-connection, single capture valve, DTO compatibility, request-ID, and real-data-plane constraints.
- Starting exact source/DTO/test mapping before changing product code.
- Read-only live state confirms the active proxy is external v2rayN/WinINet at `127.0.0.1:10808`; no Navo process is currently running.
- Mapped the complete split: NetworkEnvironmentCard observes the external proxy; Overview hero/IP cards, traffic series, and Service `ip.check` still depend exclusively on Navo committed capture/Supervisor state.
- A malformed read-only regex search failed once; reran it as literal matching and recorded the error without changing product state.
- Phase 74.1 is complete. Starting red backend/frontend regressions for the external effective-connection projection, observed exit probe, one connection card, and honest traffic attribution.
- Added backend and frontend red regressions; Go failed on the missing external IP seam and all four frontend Phase 74 cases failed as intended.
- Implemented Agent external System Proxy endpoint normalization, dual-link IP detection, keyed cache, Dashboard evidence merge, and external-only `ip.check` interception.
- Restored the old runtime exit compatibility after the first focused regression, then made external selection depend on the published Environment snapshot instead of host-global registry state.
- Added the shared Vue effective-connection projection and routed overview, risk, traffic series, totals, history identity, source copy, and chart legend through it.
- Merged the two overview IP cards into one current-connection evidence card and moved the Environment aggregate to Network Detection.
- One multi-file patch stopped after partial success on a stale runtime default anchor; inspected and completed only the missing field defaults.
- Focused Agent regressions PASS. Frontend focused suite PASS 15/15 and Vue typecheck PASS.
- Phase 74.2 is complete. Phase 74.3 implementation is present; starting affected Go/frontend full gates and browser-visible external-v2rayN verification.
- Corrected the remaining external-action mismatch: overview now offers external exit detection, and the completion message no longer implies Navo ChatGPT readiness while capture is off.
- Frontend final gate PASS: 36/36 tests, Vue typecheck, and Vite production build with 69 modules.
- The first affected Service package run failed one near-bound readiness timeout and a concurrent Xray version probe; both isolated tests passed, the full Service rerun passed, and full `go test ./...` then passed across all packages.
- Full `go vet ./...` PASS.
- Read the complete webapp-testing skill and required server-helper help. Python Playwright is absent; located and reused the existing bundled Node Playwright runtime with headless Edge.
- Added and ran `scripts/external_proxy_ui_smoke.cjs`: one overview connection card, no duplicate IP/environment primary cards, external endpoint and dual-IP evidence, honest system-total traffic labels, diagnostics ownership detail, two themes, compact overflow 0, and zero page/console errors all PASS.
- Closed the exact Vite session with Ctrl+C and confirmed listener count on 4194 is zero.
- Screenshot viewer remained blocked by the known ACL helper; retained three screenshot files and hashes without claiming manual pixel review.
- Current live v2rayN read-only gate PASS: direct and explicit-proxy exit IPs differ; proxy Google 204 and GitHub 200. Navo process count remains zero and external proxy state was not changed.
- Phase 74.3 and 74.4 are complete. Starting the official 1.0.41 portable package path.
- Official package script PASS after repeating full Go/frontend gates, npm audit, Wails production build, directory verification, and archive verification.
- Built `release/Navo-1.0.41-portable-amd64` and matching ZIP; each verifier reports `issues_found=0`, and repair check reports zero issues.
- All Navo PE versions are 1.0.41.0; ZIP SHA-256 is `2F0FDB3E58E21C64248B7BFF4A08B695D3216DC18DC636C145C4E1E630123511`.
- Final runtime hygiene: no Navo process, no Vite listener on 4194, preserved v2rayN PID 17208 / sing-box PID 18036, and unchanged WinINet endpoint.
- Phase 74 is complete.

## 2026-08-31 Phase 75

- Received the `OUTBOUND_REQUIRED` Agent/UIIPC log and the request for broader visual/layout refinement.
- Re-read the complete planning-with-files and UI/UX skills, restored Phase 74 context and the dirty worktree, and preserved all prior changes.
- Added Phase 75 with explicit fail-closed route admission, external-proxy ownership, state-aware recovery, responsive/accessibility, and portable-package constraints.
- Starting the exact UI action/error/layout source map before adding regressions.
- Ran the required UI/UX design-system plus focused UX and Vue searches. Rejected the marketing/mobile/glass/external-font recommendations and retained the applicable desktop hierarchy, density, error recovery, focus, touch-target, responsive, and reduced-motion rules.
- Traced the reported error to login-startup configuration, not normal capture activation. Confirmed normal capture already blocks a missing route locally, while startup configuration does not.
- Identified two related UI defects: misleading Overview activation CTA when no Navo route exists, and global failure feedback without a route-selection recovery action.
- Continuing with Settings/startup and shell/layout source reads before defining red regressions.
- Read Settings/startup/Connection Management and the shared style layers. Confirmed the missing startup preflight, an existing reusable no-route pattern, an unreachable 375px layout due to global `min-width:760px`, inconsistent theme geometry, undersized text, and rigid startup columns.
### 2026-08-31 Phase 75 layout evidence

- Inspected the complete shared shell and responsive rule set plus the exact overview/settings selectors.
- Confirmed the actionable scope: remove the root 760 px floor, add a compact intermediate shell, restructure startup controls, convert feedback into an actionable alert, and reduce duplicated page hierarchy without changing runtime ownership.
- Phase 75.1 is complete. Added red contracts for the shared route requirement, state-aware primary action, startup IPC preflight, actionable feedback, grouped settings controls, shared theme geometry, compact accessible navigation, and 320 px shell reflow.
- Confirmed the new Phase 75 suite is genuinely red: 0/6 PASS. The failures map one-to-one to the reported bypass and current shared-layout defects; no backend acceptance condition has been changed.
- Implemented the functional recovery path and shared layout refinement. Phase 75 focused suite is green 6/6 and Vue typecheck is green; starting the full frontend/build and browser-visible gates.
- First full build PASS (70 modules). Full tests were 41/42: only the pre-existing thin-controller size contract failed because the controller crossed its exact 400-line threshold; removed the extra wrapper without changing behavior and retained the regression record.
- Full frontend rerun PASS 42/42 and `git diff --check` PASS (only repository CRLF notices). Loaded the complete webapp-testing instructions and verified the lifecycle helper CLI; preparing the mocked Wails browser acceptance.
- First managed Edge pass reached all Overview size checks but correctly failed Settings at 375x812 with 82 px inner-main overflow. Vite and Edge were closed by the helper; added bounded offender diagnostics for the next run instead of masking overflow.
- Second offender pass isolated the overflow to `.startup-settings-controls > .startup-toggle`: its checkbox inherited the global 100% text-input width. Added an 18 px flex-none checkbox rule and preserved content reflow; rerunning the full browser matrix.
- Post-fix Edge run cleared every route/layout/theme/focus assertion and reached reduced-motion last. Chromium returned the correct `1e-05s`; fixed the smoke's scientific-notation parser before the final rerun.
- Automated smoke then PASSed, but manual screenshot inspection found its initial `night` filenames were actually light because Edge inherited system preference. Added explicit night/day clicks plus DOM theme assertions and will regenerate the matrix before accepting visual coverage.
- Explicit theme matrix PASSed. Manual night inspection found correct colors/hierarchy and no clipping, then identified that screenshots should wait past the transient 100% progress state; added a bounded steady-state wait and will regenerate final artifacts.
- Steady-state Edge matrix PASSed and representative screenshots were manually inspected. Applied the two final visual QA refinements for 1024 px evidence readability and compact sidebar status truncation; one final source/browser pass remains before packaging.
- Final source/browser rerun PASS: frontend 42/42, Vue typecheck plus Vite production build (70 modules), explicit day/night Edge matrix, eight layout assertions with maximum overflow 0, capture/startup bridge calls 0, visible 2 px keyboard focus, and 0.01 ms reduced motion. Managed Vite/Edge shutdown PASS.
- Phase 75.3 and 75.4 are complete. Confirmed the official `scripts/package.ps1` repeats Go test/vet, npm clean install/test/typecheck/build, Wails build, directory/archive verification, PE metadata, and SHA-256 before output; starting 1.0.42 packaging.
- First 1.0.42 packaging stopped safely at the Go download gate before release output changes: isolated cache downloads timed out on direct IPv6. Read-only live check confirms unchanged V2 WinINet/10808 and HTTP 200 to the exact module via that proxy; rerunning with only child-process HTTP(S) proxy variables.
- Second packaging passed the complete Go test gate but stopped at `npm ci` because orphaned Phase 75 Vite children held Rolldown. Stopped seven processes only after exact Navo path/port correlation; Navo Vite and 4195 listener counts are zero, while unrelated TimeBack and V2 processes remain untouched.
- Third official package run PASSed every gate and produced `release/Navo-1.0.42-portable-amd64` plus ZIP. Independent verifier and `repair.exe check` both report zero issues; all three PE versions are 1.0.42.0.
- Final ZIP: 64,684,118 bytes, SHA-256 `4981F15A7287F4632F720B2A0D3079AECA659FA8B9BB845E6B3AB18DCE76286B`. Final Navo/Vite/4195 counts are zero; WinINet remains `127.0.0.1:10808` and preserved sing-box PID 18036.
- Phase 75 is complete.

## 2026-08-31 Phase 76

- Received two linked requirements: choice-specific Agent priority rather than one enormous global priority order, and bounded component lifetimes to accelerate repeated System Proxy/TUN activation.
- Re-read the complete planning-with-files workflow, restored the completed Phase 75 plan, and reviewed prior Coordinator/CoreHost/request-identity lifecycle guidance.
- Added Phase 76 with typed intent/domain/phase policy, non-preemptible commit/rollback barriers, exact warm-lease keys, mode-specific safety, latency measurement, and portable-release constraints.
- Starting current source/test mapping before choosing whether the lease can retain a process, only preparation artifacts, or both per capture mode.
- Mapped the Agent-side latency/control path. Confirmed the Coordinator is a global channel token with no policy queue; capture activation enters applying before Service preparation and only supports same-mode healthy idempotence.
- Confirmed Agent itself is already persistent. Phase 76 will optimize the Service/Supervisor/CoreHost boundary with exact health-keyed leases rather than adding an Agent-process TTL.
- Mapped Service/Supervisor cold activation. Every target currently tears down core/TUN state first; System Proxy repeats reachability, compile, reconcile, start, verify, and commit, while TUN additionally repeats physical adapter/network/data-plane acceptance.
- Chosen mode split: permit a bounded System Proxy loopback-core warm lease only with crash-restart suppression and exact invalidation; TUN retains no running capture process/routes while off and may reuse only exact preparation artifacts.
- Confirmed every runtime compile produces a new validated candidate and running-core swap. The healthy runtime already exposes revision/config hash/core/outbound/policy identity needed for exact System Proxy lease matching.
- Confirmed Agent's final route probe remains after Service preparation. Warm System Proxy reuse can skip compile/reconcile/start while preserving fresh Agent-side data-plane acceptance.
- Completed the cold-path map. Identified safe TUN parallelism (fresh direct-IP baseline plus fresh activation-plan build) and a required `retain_warm` distinction so failure rollback always forces cold cleanup.
- Verified the shipped combined launcher injects the final Agent route probe. Defined the complete intent/domain queue policy and the red regression matrix; starting tests before production edits.
- Located reusable Supervisor and Agent test fixtures. Ready to add failing scheduler, warm-off/rollback, warm-idle crash, lease validity, and TUN concurrency contracts before implementation.
- Added all planned focused regressions. Initial focused execution was blocked before compile by direct Go module-download timeouts; rerun will use only process-local HTTP(S) proxy variables and will preserve the external proxy process/settings.
- Focused proxy-assisted rerun reached the expected red state in all four affected packages. Confirmed the production Agent probe is an independent current-user System Proxy/TUN path gate; beginning production implementation.
- Implemented and formatted the central intent/domain policy plus deterministic priority waiter queue. Next gate is the focused Coordinator suite before adding lifecycle leases.
- Coordinator focused tests PASS. Added Supervisor warm-idle lifecycle semantics; formatting and focused Supervisor verification are next.
- Supervisor focused package PASS. Beginning Agent `retain_warm` threading and Service exact-identity lease/timer integration.
- Implemented and formatted Agent warm-retention threading with cold-by-default options. Agent focused tests are next before Service lease integration.
- Agent full package PASS. Added Service lease identity/configuration primitives; next is capture-path retain/resume/expiry integration and TUN concurrent reads.
- Integrated System Proxy remember/retain/resume/expiry into Service capture preparation with cold fallback and mandatory TUN cleanup. Next is mutation/shutdown invalidation, TUN parallel preparation, formatting, and focused Service tests.
- Added mutation/shutdown invalidation and concurrent TUN read-only preparation; formatted all Service changes. Running focused Service compile/tests next.
- First Service compile stopped on the expected evidence-type mismatch before tests. Updating the lease/test to exact `RuntimeRoutingVerification`, then rerunning.
- Applied and formatted the strong System Proxy evidence type correction; rerunning Service compile/tests.
- Corrected the remaining retain/resume return signatures exposed by the compiler; rerunning after formatting.
- Service full package PASS (40.122s). Core implementation is green in all four affected packages; adding deeper timer/integration invalidation tests before full-repository gates.
- Added and formatted deeper warm lifecycle tests for same-PID resume, runtime drift, and timed stop. Running the Service package again.
- Service full package rerun PASS (28.750s). Reviewing the complete diff and running all-repository Go/vet gates next.
- Diff/lock review is clean overall and identified two final refinements: unordered TUN read-result collection for fast cancellation, and explicit intent/domain/priority projection into Agent status payloads.
- Located both explicit transaction projections. Preparing compatible DTO additions plus the TUN unordered-result fix before full gates.
- Confirmed additive optional DTO changes are compatible. Applying policy projection and unordered TUN result collection now.
- Applied and formatted both final refinements. Starting full Go test/vet and frontend type/test/build gates.
- Full repository Go tests PASS. Running `go vet ./...`, then frontend tests/typecheck/build.
- `go vet ./...` PASS and frontend tests PASS 42/42. Running frontend typecheck/production build and final diff/race/latency evidence next.
- Frontend typecheck/build PASS (70 modules). Coordinator/Supervisor repeated concurrency gate PASS 20/20. Repeating warm lease lifecycle tests and adding explicit latency-path assertions next.
- Warm/TUN focused repetition PASS 20/20. Final correctness refinement is typed Agent propagation of queued-request supersession before rerunning all gates.
- Implemented typed REQUEST_SUPERSEDED propagation with a focused regression. Formatting and affected/full gate reruns are next.
- Affected Agent/Coordinator/Service/Supervisor packages PASS after the final error mapping. Running final full Go/vet/frontend/diff gates, then version/package.
- Final full Go test/vet PASS. Running final frontend/diff hygiene once more, then bumping to 1.0.43 and invoking the official portable packager.
- Final frontend tests/typecheck/build PASS. Checking exact worktree/process/listener hygiene, then bumping VERSION to 1.0.43 and packaging.
- Pre-package process/network/diff hygiene PASS. External v2rayN ownership is preserved. Bumping VERSION from 1.0.42 to 1.0.43 and starting the official package script.
- Official 1.0.43 package run PASS with directory/archive verifier issues zero and reported ZIP SHA-256. Running independent verifier, repair, PE version/hash, and final process/network residue checks.
- Independent directory/archive verifier PASS with zero issues. Running repair self-check, independent hashes/PE metadata, and final residue checks.
- Repair self-check PASS. ZIP size/hash match; rerunning PE metadata in non-truncated form, then final residue/hygiene and Phase 76 closure.
- Explicit packaged PE verification PASS: all three binaries report 1.0.43.0; SHA-256 values are recorded in `task_plan.md` and `findings.md`.
- Final runtime hygiene PASS: no Navo process, no Navo Vite child, no 4195/12080 listener; WinINet and the external v2rayN sing-box remain unchanged.
- Phase 76.5 is complete. Released verified `release/Navo-1.0.43-portable-amd64` and matching ZIP; the only remaining gate is optional real-host latency/elevated network acceptance with a configured Navo route.
- Phase 76 is complete.
