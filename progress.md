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
