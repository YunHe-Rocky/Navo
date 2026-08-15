# Navo Multi-Core Architecture Refactor

## Goal

Keep the completed Wails v2 + Vue 3 migration, replace the flat proxy model
with the mandatory typed multi-core architecture, integrate with the user's
cloud MySQL without installing a database, and verify build, startup, IPC,
core switching, data plane and shutdown recovery.

## Phases

- [x] Phase 1: Inventory the current Flutter UI, launcher contract and package layout.
- [x] Phase 2: Introduce the Wails Go host and Vue frontend without changing Service/Agent IPC.
- [x] Phase 3: Replace package and smoke scripts; remove Flutter-specific source/build logic.
- [x] Phase 4: Run Go/frontend tests and produce a release package.
- [x] Phase 5: Run packaged startup, IPC, three-core data-plane and shutdown smoke tests.
- [x] Phase 6: Update installation documentation and final release evidence.
- [x] Phase 7: Audit the current repository against the mandatory multi-core architecture document and create `docs/audit/NAVO_CURRENT_STATE.md`.
- [x] Phase 8: Add strict domain models, validation, typed endpoint specs and compatibility results.
- [x] Phase 9: Add `.env` loading and MySQL configuration/repository/migration infrastructure.
- [x] Phase 10: Introduce independent sing-box, Mihomo and Xray core adapters with native validation and readiness probes.
- [x] Phase 11: Refactor subscription and upstream-proxy pipelines with strict source isolation.
- [x] Phase 12: Implement transactional ActiveSelection, revision history and LastKnownGood rollback.
- [x] Phase 13: Replace the public generic Wails bridge with typed DTOs and bind the Vue UI to application services.
- [x] Phase 14: Implement capture-mode boundaries, recovery and security hardening.
- [x] Phase 15: Run unit, integration, build, package, startup, core-switch, data-plane and shutdown verification.
- [x] Phase 16: Reproduce and fix TUN selection rollback plus CORE_002/CORE_004 restart readiness failure.
- [x] Phase 17: Implement the full Service-backed system tray control center from `NAVO_TRAY_IMPLEMENTATION_SPEC.md`.
- [ ] Phase 18: Reproduce and fix the Windows system-proxy/TUN failures defined by `NAVO_Windows_Proxy_TUN_Troubleshooting_and_Acceptance.md`, then execute the applicable acceptance gates.
- [x] Phase 19: Reduce resident/runtime and repository/package weight with on-demand UI, a consolidated dashboard snapshot, cached asynchronous IP detection, and a clean installer payload boundary.
- [x] Phase 20: Build and structurally validate a one-click Windows installer with install/uninstall metadata, shortcuts, upgrade handling, required elevation, and the complete packaged Navo payload. Live install is deferred because this managed session cannot access Windows Installer.
- [x] Phase 21: Implement `navo_ui_refactor_requirements.md`: split page responsibilities, add unified UI state derivation, dual-path IP details, fixed 30-point traffic sampling, icon-state previews, visual acceptance, native packaging, and refreshed installers.
- [ ] Phase 22: Audit and implement `Navo_TUN虚拟网卡与模式切换_Codex作业指导书.md`: unified capture state machine, serialized transition coordinator, strict TUN adapter lifecycle, rollback/recovery, UI phases, and the applicable Windows acceptance matrix.
- [x] Phase 23: Reproduce and fix packaged startup with no visible UI and repeated console-window flashes; verify one UI process, zero visible console windows, bounded restart behavior and refreshed setup output.
- [ ] Phase 24: Add host/runtime status, safe core update inspection, proxy latency and throughput tests; rebuild the Vue shell into explicit day/night forms (sharp white-black-orange day, rounded purple-black-blue night), remove global refresh, show scrolling progress for reactive actions, merge logs into settings, then verify both themes, async states, responsive UI, production build and packaged runtime.
- [ ] Phase 27: Harden capture recovery for every owned failure boundary: stale process/port, WinINet, TUN adapter, route, DNS, firewall, transaction journal, core incompatibility, core crash, cancellation, application crash and shutdown; preserve unrelated proxy applications and keep UI changes limited to recovery status/actions.
- [ ] Phase 34: Execute `Navo_TUN_One_Pass_Closure_Codex_Guide.md`, close all current TUN implementation gaps, and pass the document's applicable static, automated, elevated data-plane, rollback, and recovery gates.
- [x] Phase 35: Diagnose the post-enable `TUN requires restart` incident from current installed logs, unify TUN health/restart ownership across Agent, Service, Supervisor, and SelfHeal, repair profile identity/DACL handling, ship the fixed installer, and pass installed real-data-plane plus delayed stability/recovery acceptance.
- [ ] Phase 36: Rework overview, connection management, automatic traffic visualization, one-click testing, trusted core upgrades, and dual-link IP monitoring; pass backend, frontend, browser-visible, package, and installed-runtime gates.
- [x] Phase 37: Separate Windows capture selection from proxy routing policy, provide System Proxy and Global Proxy behavior comparable to v2rayN/Clash Verge, and prove foreign-site traffic traverses the selected proxy with exact rollback.
- [x] Phase 39: Replace the portable UI executable and every in-product N brand mark with the canonical `tray_icon.ico`, rebuild the relevant artifacts, and verify icon consistency.
- [x] Phase 40: Repair GitHub core-update downloads so they can use the active Navo loopback proxy with bounded fresh-transport fallback while preserving official-host and SHA-256 verification.
- [ ] Phase 42: Ship the repaired updater into the installed application, prove installed payload identity, and rerun the real sing-box 1.13.18 update transaction.
- [x] Phase 41: Eliminate network-management privilege overreach by restricting adapter, route, DNS, firewall, and proxy mutations to explicitly Navo-owned resources; add regression coverage for physical Ethernet preservation.
- [x] Phase 46: Rebalance the Vue shell typography, text contrast, page hierarchy, and task-oriented navigation groups; verify day/night desktop and compact layouts in a real browser.
- [x] Phase 48: Apply the Phase 46 hierarchy, typography, contrast, spacing, and task-first structure to all seven Vue pages; verify every page in day/night themes and compact layout.
- [ ] Phase 55: Diagnose repeated `Agent / UIIPC request failed`, preserve actionable IPC error context, harden Agent readiness/reconnect behavior, and verify cold-start/restart/timeout paths.
- [ ] Phase 56: Prove every supported capture and routing mode with real traffic and correct data on the current portable build; repair failures and rerun until the full matrix passes.
- [x] Phase 57: Repair crash-safe DNS recovery and cold-boot proxy activation; prove startup reconciliation, deferred activation, actionable failure handling, and exact DNS/WinINet rollback.

### Phase 34 execution

- [x] 34.1 Transaction safety: early Manager ownership, independent rollback context, dirty retention, delayed health commit.
- [x] 34.2 Deterministic activation plan: endpoint resolution, physical route freeze, selected endpoint pinning, strict adapter snapshot.
- [x] 34.3 Journal V2 and exact crash recovery.
- [x] 34.4 Idempotent endpoint, split-route, NRPT, and IPv6 operations.
- [x] 34.5 Control-plane and Service-owned data-plane verification with structured errors/results.
- [x] 34.6 Unit/static/frontend/build gates.
- [ ] 34.7 Elevated Windows sing-box/Mihomo, failure-injection, rollback, crash-recovery acceptance and report.

### Phase 35 execution

- [x] 35.1 Eliminate duplicate TUN health/restart ownership and debounce transient adapter observations.
- [x] 35.2 Repair protected profile DACL ownership and Navo-owned orphan firewall recovery.
- [x] 35.3 Build and install version 1.0.6 with matching release hashes and a clean repair check.
- [x] 35.4 Reconfirm the currently running/installed version, live adapter/routes/DNS, selected core and sanitized runtime logs.
- [x] 35.5 Reproduce the current failure boundary with explicit proxies disabled and direct DNS/TCP/HTTPS probes for Google and GitHub.
- [x] 35.6 Fix the remaining DNS/data-plane issue and add regression coverage.
- [x] 35.7 Rebuild/install and pass elevated Google/GitHub traffic, delayed stability, rollback and clean-reenable acceptance for each supported core.

### Phase 36 execution

- [x] 36.1 Restore the current UI/backend contracts and identify the real data and ownership gaps.
- [x] 36.2 Make overview identity/risk and real-time traffic follow the committed capture mode automatically.
- [x] 36.3 Add chart crosshair plus pointer-adjacent, keyboard-accessible details.
- [x] 36.4 Make line type page-global in Connection Management and move airport subscriptions out of One-click Test.
- [x] 36.5 Implement a user-triggered, integrity-verified, rollback-safe core upgrade transaction.
- [x] 36.6 Repair dual-link detection, cache invalidation, partial-result behavior, and risk summary.
- [ ] 36.7 Pass Go/Node/Vue/browser/package checks and verify the actual desktop behavior.

### Phase 37 execution

- [x] 37.1 Audit current capture/routing contracts and compare the relevant v2rayN/Clash Verge behavior.
- [x] 37.2 Define one authoritative mode/policy state machine with deterministic conflict handling and rollback.
- [x] 37.3 Implement backend/compiler/UI changes without regressing existing TUN ownership and recovery.
- [x] 37.4 Add focused regression coverage and pass Go/Vue/build gates.
- [x] 37.5 Verify real foreign-site HTTP/HTTPS and exit-IP traffic through the selected proxy, then verify disable rollback.

## Current Phase

Phase 55 is in progress. Correlate the 2026-08-14 UIIPC failures with launcher,
Agent, pipe, and UI diagnostics before changing behavior; do not weaken the local-only
Named Pipe boundary or hide failures behind unbounded retries.

Phase 55 source and portable-package repair is complete. Adapter readiness now uses
native Windows IP Helper APIs, and UIIPC errors expose stable codes plus redacted reasons.
Full Go tests/vet, frontend tests/typecheck/build, npm audit, repair, manifest, and package
checks pass. Real elevated TUN traffic/recovery acceptance remains pending; ZIP creation
was blocked by the current execution-approval usage limit.

Phase 56 is in progress. TUN live execution requires explicit user approval for temporary
adapter/route/DNS mutation. System Proxy has a separate non-TUN runner; its first sandboxed
attempt was blocked by DPAPI identity isolation before startup, with exact WinINet rollback.

Phase 48 is complete. All seven pages now share a task-first intro, stable content spacing,
larger readable metadata, proportional Chinese typography, and clearer primary/result/
secondary hierarchy. Protocols, addresses, hashes, and logs retain technical data fonts.
No page behavior or Phase 47 network work was changed.

Phase 48 error log: the first broad CSS replacement expected two adjacent overview rules
in the opposite order and failed verification before applying. Switched to a bounded
Phase 48 override layer to avoid conflicts with the existing dirty stylesheet.
Phase 48 error log: frontend unit tests passed, but production build could not find
`vue-tsc` because the existing `node_modules/.bin` dependency entry had been removed
since the previous phase. Restore exactly from package-lock with repository-local cache.
Phase 48 error log: after dependency restoration, typecheck exposed the health-label
object lookup as unsafe for the full `NetworkHealthState` union. Replaced it with an
explicit switch and unknown fallback.
Phase 48 error log: the seven-page desktop smoke completed, but the final compact test
could not locate “连接管理” because its visible label is hidden at 760px. Added an explicit
navigation `aria-label` so icon-only mode keeps page name and task purpose accessible.
Phase 48 error log: the expanded all-text audit found only the decorative connection step
indices `01/02` at 9px. Raised `.axis-index` to the shared 11px caption token.

Phase 46 is complete. The Vue shell now separates core operations, monitoring/diagnostics,
and system management; Chinese UI typography uses proportional fonts, labels no longer
force uppercase, and secondary text has stronger contrast. Existing page IDs and actions
remain unchanged. Unit/build gates and day/night desktop plus compact browser smoke passed.

Phase 46 error log: the first browser helper invocation started `npm` from the repository root, where no package manifest exists, so port 4186 never opened. Retry uses an explicit `Set-Location navo_app` server command.
Phase 46 error log: the first rendered assertion matched the same page-purpose text in both the sidebar description and header. Scoped the assertion to `.page-heading p` and retained the duplicate copy intentionally because it serves separate navigation and page-context roles.
Phase 46 error log: Playwright's partial accessible-name match for the one-character day-theme button also matched “日志”. Changed the test selector to an exact accessible-name match.

Phase 37 is complete for the reported TUN restart incident. Installed 1.0.12
uses bounded DNS cold-start retries and a 30-second live-core readiness window.
Real-profile sing-box passed the complete initial/delayed/re-enabled lifecycle;
Mihomo twice re-enabled to `HEALTH_COMMITTED` with Service-owned Google 204,
GitHub 200, matching exits, exact rollback, and zero owned residue. Phase 34
remains open only for its broader failure-injection and crash-recovery matrix.

### Phase 38 execution

- [x] Separate capture scope from routing policy in product semantics and UI.
- [x] Restore System Proxy ownership semantics for browser and proxy-aware applications.
- [x] Preserve TUN as the browser and arbitrary-application IP capture path.
- [x] Expose user-owned blacklist/whitelist editing and persist it into core-neutral configs.
- [x] Rebalance the desktop layout and verify desktop/narrow compositions.
- [x] Run full source/package gates and real System Proxy 4-policy application data-plane acceptance.
- [in_progress] Rerun elevated TUN 4-policy application data-plane acceptance after the adapter-rebind fix.

Phase 38 error log: the first findings/progress append targeted a sentence that existed only in `progress.md`; inspected both file tails and reapplied with their actual contexts.
Phase 38 error log: the first combined test patch applied the runtime test hunks before a later stale context failed; inspected the exact diff and resumed only the missing Wails assertion.
Phase 38 error log: targeted Go rerun nested a double-quoted PowerShell command, so the outer shell expanded `$env:*` before the child started and fell back to the public module cache. Replaced it with a repository-local `.cache` script using the established cache paths.
Phase 38 error log: initial `gofmt` formatted writable files but could not create temp replacements beside three protected files; reran the same bounded Go-only list with approved repository access.
Phase 38 error log: a broad recursive Playwright package inventory entered protected historical TUN profiles and emitted access-denied noise; bounded all later browser tooling to the known `.cache/python-playwright/playwright/driver/package` path.
Phase 38 error log: the routing-timeout patch applied Agent and Wails hunks before the final test context mismatch; verified the applied code and added only the missing Wails assertion.
Phase 38 error log: the first package run passed all Go tests then stopped at `npm ci` because two UI-smoke Node processes retained the exact Rolldown binding. Identified PIDs 17956/20888 by loaded module, stopped only those processes, and verified `OWNER_COUNT=0` before retrying.
Phase 38 error log: the bounded `Stop-Process` command returned exit 1 only because its final `Get-Process` found neither terminated PID; a separate loaded-module audit confirmed cleanup succeeded.
Phase 38 error log: the first elevated application-flow matrix exposed stale adapter-bound routes after a live TUN core reload; the explicit local proxy remained healthy while no-proxy application traffic timed out. Added transactional adapter/resource rebind and TUN-native verification before accepting policy changes.
Phase 38 error log: the corrected elevated rerun could not be started because the active execution policy blocks new UAC requests. Completed the System Proxy matrix with an amd64 as-invoker test launcher built from the same source while leaving the signed release artifact unchanged; elevated TUN retest remains open.

### Phase 39 execution

- [x] 39.1 Inventory executable resources and all UI/logo N assets without replacing ordinary text.
- [x] 39.2 Make `tray_icon.ico` the canonical build and UI branding source.
- [x] 39.3 Rebuild the portable UI executable and validate executable/UI icon resources.

Phase 39 error log: .NET could not decode `build/windows/icon.ico` or the isolated canonical 256px frame (`Requested range extends past the end of the array`); switched to the bundled Pillow runtime, selected the ICO's largest frame, and generated the Wails PNG successfully.
Phase 39 error log: the webapp-testing Python runner started Vite but the bundled Python runtime lacks `playwright`; switched to the repository's already-proven cached Playwright Node driver rather than installing or mutating dependencies.

### Phase 40 execution

- [x] 40.1 Reproduce the failing transport path and confirm the official asset/redirect trust boundary.
- [x] 40.2 Add validated loopback-proxy-aware, bounded fresh-transport download attempts without weakening integrity checks.
- [x] 40.3 Add regression tests for proxy selection, fallback, retries, and host restrictions.
- [x] 40.4 Run focused, repository, isolated package, repair/hash, and read-only live-download gates; installed replacement remains a separate action.

Phase 40 error log: direct focused `go test` used the protected user Go build cache and failed with `Access is denied`; rerun with repository-local `.cache` paths, matching the established Navo gate.
Phase 41 error log: the first focused test used `.cache/gopath`, which lacks the populated module cache and attempted blocked direct downloads; rerun with the repository-standard `.cache/go-path` from `scripts/test.ps1`.
Phase 41 error log: the first combined read-only audit piped directly after a PowerShell `foreach` block and failed parser validation; collect rows into an explicit array before formatting.
Phase 40 error log: the first full gate collided with the live product listener because three Reconciler tests hard-coded port 12080; changed only those fixtures to reserve and release a dynamic loopback port, preserving the user's running process.
Phase 40 error log: isolated packaging reached the Windows launcher build and exposed an existing duplicate short declaration at `internal/service/runtime.go:200`; changed `:=` to `=` because both `outbounds` and `err` are already in scope, then resumed the same isolated package target.
Phase 40 error log: `curl.exe` could not perform the read-only GitHub HEAD probe because local Schannel returned `SEC_E_NO_CREDENTIALS`; switched to an opt-in live test that exercises the updater's own Go TLS, redirect, bounded download, size, SHA-256, and archive validation path.

### Phase 41 execution

- [x] 41.1 Inventory every privileged network mutation and identify paths capable of disabling or reconfiguring a physical Ethernet adapter.
- [x] 41.2 Introduce a fail-closed ownership boundary for Navo-managed adapters, routes, DNS, firewall rules, and proxy state.
- [x] 41.3 Add regression tests proving physical Ethernet/Wi-Fi adapters are never disabled or treated as Navo-owned.
- [x] 41.4 Run focused and repository validation; report any elevated Windows acceptance still pending.

Phase 41 source closure: persisted and IPC adapter names are canonicalized or rejected; network planning accepts only `Navo`; route binding additionally requires `Wintun Userspace Tunnel` with `HardwareInterface=false`. Focused tests, full `go test ./...`, `go vet ./...`, and production-source forbidden-mutation scans passed. Current live adapter enumeration remains unverified because the non-elevated shell returned `Access denied`; no installed binary was replaced in this phase.

### Phase 42 execution

- [x] 42.1 Prove the reported retry still used installed 1.0.12 rather than the Phase 40 binary.
- [x] 42.2 Rebuild the latest Phase 40+41 source into an isolated release and a versioned MSI.
- [x] 42.3 Upgrade the installed application without touching unrelated proxy processes; verify installed hashes and repair state.
- [ ] 42.4 Run the installed sing-box update and confirm the manifest/binary version advances to 1.13.18.

Phase 42 error log: the old elevated launcher tray window was inaccessible from the normal token, so graceful tray exit could not run. MSI installed 1.0.13 correctly but the pre-install process remained mapped; terminated only verified Navo PID 21576 and its job children through UAC, then launched the installed 1.0.13.
Phase 42 error log: the normal shell cannot query the elevated Agent pipe (`Access denied`), so installed dashboard state cannot be asserted from this token. New processes are running, but local proxy port 12080 is currently closed until a node connection is active.

## Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Flutter SDK missing at script default path | 1 | Replace the obsolete Flutter package pipeline with Wails. |
| Previous planning files were absent and catch-up helper path did not exist | 1 | Recreated planning files in the repository root and continued from repository state. |
| `go get` could not write the user-wide module and checksum caches | 2 | Use repository-local `GOPATH`, `GOMODCACHE` and `GOCACHE` paths. |
| npm could not write `%LOCALAPPDATA%\npm-cache` | 1 | Pass a repository-local cache directory to every npm command. |
| `vue-tsc` 3.3.8 cannot load TypeScript 7's private `lib/tsc` export | 1 | Pin the frontend compiler to the current TypeScript 6 release line. |
| Sandbox denied deletion of two pre-existing Flutter files | 1 | Remove only the verified legacy script and test directory with elevated filesystem access. |
| Go test script was invoked from `navo_app` with a root-relative path | 1 | Re-run the Go gate from the repository root as a separate command. |
| Packaged smoke test reported core not running after startup | 1 | Inspect isolated runtime state and launcher/core logs before changing code. |
| Smoke core loop assumed startup always selected sing-box | 1 | Explicitly select every target core so persisted state cannot affect the test. |
| Full smoke passed core switching/data plane but launcher exited with code 2 | 1 | Inspect shutdown log and launcher exit path; keep functional results intact. |
| Git status/diff unavailable because the workspace has no Git repository metadata | 1 | Use direct filesystem scans and executable/hash verification instead. |
| Automatic removal of smoke-generated `release\Navo\data` and `log` was denied | 1 | Left the diagnostic artifacts intact; they do not alter signed package files or runtime behavior. |
| A broad PowerShell source scan entered built binaries and timed out | 1 | Restrict subsequent scans to known source directories and extensions. |
| A multi-file patch reported a late context mismatch after applying earlier hunks | 1 | Verify each intended file and apply any remaining hunks separately. |
| Core adapter tests assumed POSIX permission bits on Windows | 1 | Keep restrictive file creation and defer Windows ACL assertions to the dedicated security phase. |
| `coreadapter` initially retained one unused import | 1 | Remove the import and rerun the targeted package tests. |
| Playwright's bundled Chromium was absent | 1 | Run the desktop-view smoke with the installed Edge channel; no browser was added to the product. |
| UI test helper left Vite processes holding Rolldown | 1 | Identify only Navo-owned process command lines, terminate them, and rerun packaging. |
| Smoke script indexed a null `ProcessStartInfo.EnvironmentVariables` | 1 | Scope environment inheritance around process start and move cleanup under `try/finally`. |
| Wails WebView2 reported resource in use | 1 | Give Navo an explicit isolated WebView2 data directory under `LOCALAPPDATA`. |
| Exit smoke left Xray/UI after shutdown | 2 | Serialize shutdown against core switching, use the correct Extended Job Object limit class, and add explicit UI termination fallback. |
| Versioned `go run` queried the network during packaging | 1 | Build and cache the locked Wails v2.12.0 CLI from the local module source. |
| TUN selection falls back to off and core restart reports `CORE_004` for port 12080 | 1 | Replaced two-step UI switching with `capture.set`, made TUN apply synchronous, detected early process exit, and surfaced the real Windows privilege requirement without changing the previous capture state. |
| Final smoke launcher exited once with Windows `0xC0000005` during shutdown | 1 | No residue remained; two immediate full reruns passed all three cores, HTTP data plane and clean shutdown. Recorded as a non-reproduced Windows lifecycle event rather than hiding it. |
| Initial Phase 17 targeted test used the protected user Go cache and default module proxy | 1 | Re-ran with repository-local `GOCACHE`/`GOMODCACHE` and the established proxy; all targeted packages passed. |
| First multi-file tray patch missed an exact Win32 function context | 1 | Split the change into bounded patches, verified each applied file and reran compilation tests. |
| Execution policy blocked direct invocation of `scripts/test.ps1` | 1 | Invoked the repository script with `powershell.exe -ExecutionPolicy Bypass`; tests and vet passed. |
| Focus-based tray Exit automation did not activate the popup menu | 2 | Located the native `#32768` menu window and clicked its final item by verified bounds; launcher exited cleanly with code 0. |
| Full smoke again exposed the historical intermittent Windows `0xC0000005` launcher shutdown race | 2 | No functional assertion or residue failed; two subsequent identical full runs passed. Kept this as a known intermittent lifecycle issue instead of reporting perfect stability. |
| Initial source slice referenced a non-existent `internal/service/runtime_transaction.go` and old adapter filenames | 1 | Located the current implementation in `internal/service/runtime.go` and the actual adapter registry files before making changes. |
| First `tun_smoke.ps1` run used the real `%LOCALAPPDATA%` and failed under the managed filesystem | 1 | Isolated the script under `.cache\tun-smoke\localappdata`, matching the full smoke harness. |
| First elevated TUN run could not reach the elevated default-ACL Named Pipe | 1 | Added an explicit protected DACL for SYSTEM, Administrators, and the object owner. |
| Packaged sing-box copy had a transient hash mismatch after rebuild | 1 | Restored it from the verified source binary, rechecked SHA-256 against `CORE_MANIFEST.json`, and reran packaged smoke successfully. |
| Final elevated UAC acceptance did not start a new logged launcher and timed out | 1 | Terminated the spawned PowerShell test processes; retained the passing non-elevated rollback evidence and reported the elevated gate as pending. |
| Current non-admin runtime bypassed the documented TUN elevation preflight | 1 | Found `IsElevatedFn` orphaned from `setCaptureMode`; restore the guard at every public TUN entry and add regression coverage. |
| Expected `internal/agent/agent_capture_test.go` did not exist | 1 | Locate the actual Agent test file by bounded filename/content search. |
| Targeted format/test command ran from `cmd/navo` and used root-relative paths | 1 | Keep resource generation in `cmd/navo`, then run formatting and Go tests from repository root. |
| Embedding `requireAdministrator` in the checked-in `.syso` made `go test ./cmd/navo` require UAC | 1 | Keep development resources `asInvoker`; patch only the packaged launcher manifest after compilation. |
| Packaging could not resolve `go-winres.exe` from `PATH` | 1 | Resolve the standard `%USERPROFILE%\go\bin` installation as a fallback and fail with the pinned install command when absent. |
| Release packaging could not replace the running `navo_app.exe` | 1 | Identified only release-owned PIDs 5524/2828/2316, stopped launcher 5524, and verified its Job Object removed UI/core children. |
| CIM command-line inventory returned access denied | 1 | Used `Get-Process.Path` to bound the shutdown to executables under `release\Navo`. |
| Post-UAC-attempt repack briefly found `navo_app.exe` locked with no owning release process | 1 | Verified ACL/read-only state and a reversible rename round-trip; lock released, then retried packaging. |
| Non-elevated acceptance client could not connect to elevated `Navo.UI.Agent.v1` | 1 | Replaced elevated-object-owner `OW` ACL with the concrete current token user SID; tests cover narrow same-user access. |
| Elevated acceptance launcher remained active and locked the release after the wrapper timed out | 1 | Keep the already-running fixed release available for the user to select TUN; package the final Pipe ACL follow-up after it exits. |
| TUN forwarded Windows DNS for `172.19.0.2:53` into the selected SOCKS outbound | 1 | Add an internal DNS server, `hijack-dns`, outbound bootstrap resolver, and direct ICMP route; validate with bundled sing-box 1.13.14. |
| TUN compiler test omitted `TUNConfig.Enabled` and therefore did not generate default DNS | 1 | Correct the fixture to represent the Service runtime contract, then rerun compiler and native validation. |
| Native TUN integration fixture omitted required `UpstreamProxyID` | 1 | Supply the typed source identifier required by `ActiveSelection.Validate` and rerun the full gate. |
| Post-fix elevated smoke timed out before UAC approval and no Navo process started | 1 | Keep native validation and packaged manifest evidence; require one interactive launch to complete DNS/ICMP/data-plane acceptance. |
| sing-box runtime rejected `dns-direct` because the new UDP DNS dialer also specified `detour: direct` | 1 | Remove the redundant detour and add a real core-start liveness test because `sing-box check` did not catch service initialization failure. |
| Canonical validator still required legacy `address` for modern typed UDP DNS | 1 | Validate typed server/server_port fields by DNS server type while retaining legacy address compatibility. |
| Phase 19 multi-file patch missed the exact current `agent.Config` alignment | 1 | No hunk was applied; split the implementation into bounded file patches using current source context. |
| Installer safety patch used stale English deployment text | 1 | Kept the script unchanged, located the current Chinese section, and applied script/document updates with exact contexts. |
| Lightweight smoke exposed null `ProcessStartInfo` environment collections | 2 | No Navo process started; use the established bounded parent-environment override and restore it immediately after launch. |
| Lightweight smoke desktop could not create a tray icon | 3 | Stop inferring tray state from shutdown timing; use the startup-observed fallback state and keep reopen coverage in the injected process-manager test. |
| Lightweight smoke launcher inherited host `GOARCH=386` and crashed after graceful shutdown | 1 | The release is amd64; pin the smoke build to `windows/amd64` so Win32 structure layout and release architecture match. |
| Initial `dotnet tool search` used protected user caches and then hit sandboxed NuGet access | 2 | Redirect all .NET/NuGet state into `.cache` and use approved access to the official NuGet source. |
| WiX 7.0.0 required interactive acceptance of the OSMF EULA | 1 | Do not accept legal terms for the user; replace the cache-local tool with WiX 5.0.2. |
| First WiX source nested `ComponentGroup` under `Directory` | 1 | Move the harvested payload component group to the Wix document root and reference `INSTALLFOLDER`. |
| First Phase 22 orchestration used unavailable nested tool name `exec_command` | 1 | Discover the enabled nested tool and use `shell_command`. |
| Phase 22 `git status` again found no repository metadata | 1 | Preserve files by bounded source inspection and direct before/after verification. |
| Phase 22 combined read used `-LiteralPath` with a wildcard | 1 | Enumerate exact files first and read each resolved path; keep other reads in independent calls. |
| Phase 22 targeted tests referenced removed `.cache/go-mod` | 1 | Locate the current centralized repository cache and rerun without falling back to the protected user Go cache. |
| Agent capture tests still asserted the removed `tun.status -> tun.enable` sequence and used the production journal path | 1 | Update them to assert the single `capture.prepare` boundary, isolated journal paths, and zero Service mutation before elevation. |
| PowerShell execution policy blocked `npm.ps1` during the frontend gate | 1 | Invoke the same installed npm runtime through `npm.cmd`. |
| Phase 22 format command referenced `internal/service/supervisor.go` instead of `internal/supervisor/supervisor.go` | 1 | Correct the path and rerun the full Go test/vet gate. |
| Browser helper could not import Python Playwright | 1 | Use the bundled Node Playwright runtime and installed Edge channel. |
| First capture UI smoke focused a disabled retry button while refresh was busy | 1 | Watch both fault visibility and loading state; focus retry only after the action is enabled. |
| Vite helper left Rolldown-loaded Node children that blocked packaging | 1 | Identify only processes with the Navo Rolldown module loaded, stop those PIDs, then repackage. |
| Elevated TUN smoke elevated only Navo, leaving its IPC client at medium integrity | 1 | Update `tun_smoke.ps1` to elevate the complete test process once and keep launcher/client integrity levels equal. |
| Repeated cleanup mistook v2rayN-owned sing-box for Navo and caused v2rayN to respawn it with console flashes | 1 | Identify the creator via the Windows process performance counter (`v2rayN.exe`, PID 5968), stop touching it, and verify no Navo process remains. |
| Phase 23 targeted `go test` used protected user Go caches and attempted the public module proxy | 1 | Use the repository-standard `scripts/test.ps1`, which pins local `GOCACHE`, `GOMODCACHE` and the established module policy. |
| Phase 23 package command closed the managed runner pipe before returning | 1 | Inspect timestamps and live build processes first; resume only the incomplete packaging stage instead of repeating the combined command blindly. |
| Phase 23 combined format/test command ran from `navo_app` with repository-root paths | 1 | Run Go formatting/tests from the repository root, then run npm from `navo_app` separately. |
| Phase 23 `gofmt` command accidentally included `scripts/lightweight_smoke.ps1` | 1 | Format Go sources only; validate the PowerShell script through its smoke execution. |
| Phase 23 headless capture UI helper exceeded its 60-second runner limit | 1 | The preview listener was gone and no new accessible Node process remained; rely on the passing Vue typecheck/build, Wails DTO regression tests and real packaged single-window smoke for this startup fix. |
| Python Playwright is not installed in the bundled Windows Python runtime | 1 | Use the already-bundled Playwright Node runtime with installed Edge, preserving the same headless browser acceptance boundary without adding a product dependency. |
| UI preview helper timed out because `--prefix` targeted `navo_app/frontend` while `package.json` is in `navo_app` | 1 | Keep frontend sources under `navo_app/frontend`, but start the existing Vite scripts from their actual package root `navo_app`. |
| Corrected UI preview package root still timed out because this package has no `preview` script | 1 | Use the existing `dev` script against the already-built source; do not mutate package scripts solely for the smoke harness. |
| First Phase 24 browser smoke used an exact-text selector for a numeric value whose unit is in the same element | 1 | Target the stable benchmark value slots and assert their text content, matching the rendered accessible composition. |
| Default release rebuild found elevated Navo PID 5220 locking `release/Navo/log/navo.log`; normal stop and same-user Pipe shutdown were both denied | 2 | Preserve the running user session and unrelated v2rayN core; add a validated optional package output name and build Phase 24 into an isolated release directory without changing the default release contract. |
| Vue inferred the optional boolean parameter of `checkIP` as the native click event payload | 1 | Invoke `checkIP()` explicitly from button bindings while retaining `checkIP(false)` for silent lifecycle loads. |
| Dual-theme smoke assumed night was always the initial theme, but the product intentionally follows persisted/system preference | 1 | Explicitly select and validate each form; initial theme is not an acceptance invariant. |
| Browser normalizes a zero CSS custom property as `0` rather than `0px` | 1 | Accept both equivalent computed forms while still requiring the exact orange accent token. |
| Isolated package `npm ci` found the Rolldown native binding locked by orphaned Navo Vite Node processes | 1 | Inspect loaded modules and stop only Node PIDs that have Navo's exact `@rolldown` binding loaded; preserve unrelated Node processes. |
| Phase 27 combined full gate invoked npm from the repository root where no `package.json` exists | 1 | Go tests/vet already passed; rerun only the Vue build from the actual `navo_app` package root with the repository-local npm cache. |
| Elevated Phase 31 TUN smoke reached HTTPS but local PowerShell rejected the server TLS trust chain | 1 | The same HTTPS request fails directly on this host; use Microsoft's HTTP connect-test endpoint and require both HTTP success and its fixed response body. |
| First Phase 33 package attempt could not replace the newly built launcher during manifest patch | 1 | No Navo process owns the package; verify the exact file lock/rename and retry only this isolated output after the transient scanner lock releases. |
| First non-elevated Phase 34 smoke could not initialize the launcher (`0xc0000142`) before application logging | 1 | Inspect package-owned processes and generated smoke profile; rerun with a fresh isolated profile only after proving no live owner or system residue. |
| Guide referenced `internal/service/capture_transition_test.go`, but the repository has no such file | 1 | Use the existing capture/ownership tests as the baseline and add the required service transaction tests in a new file. |
| Broad recursive Go symbol scan entered generated/cache trees and timed out | 1 | Restrict source scans to `internal`, `cmd`, and `navo_app` source trees and use bounded extensions.
| First Phase 34 Go gate exposed stale Manager/Platform test doubles and the old AST adapter-wait assertion | 1 | Rewrite the network transaction tests for V2 resources and update the ownership assertion to require `Manager.Activate` after core start.
| Full Go gate also hit pre-existing managed-host failures in `compiler` temp-file access and a Named Pipe timeout test | 1 | Keep them separate from Phase 34 compile failures; rerun targeted packages with an isolated `TEMP` and then rerun the full repository gate.
| Non-elevated structured adapter inspection test received `Get-NetAdapter: Access denied` | 1 | Treat permission denial as an expected non-admin environment result while still failing the test on PowerShell parser errors; the production preflight requires elevation.
| Phase 34 follow-up artifact summary used an invalid PowerShell pipeline after `foreach` | 1 | Collect objects into an explicit array before formatting; no product state changed. |
| Expected Phase 34 package path `release\Navo` is missing during follow-up | 1 | Locate actual release/install outputs, rebuild the current code, and record package identity before any new elevated acceptance. |
| Installed-package comparison repeated invalid `foreach { ... } |` syntax | 2 | Stop using that construct; use `ForEach-Object` or collect into an array before piping. |
| Capture model was guessed as `internal/domain/capture/model.go` but the file does not exist | 1 | Enumerated the package and switched to the actual `state.go`/`mode.go` files. |
| TUN manager implementation was guessed as `manager_windows.go`/`manager.go` | 1 | Enumerated the package and switched to the actual `tun_windows.go`/`tun.go` files. |
| Silent MSI major upgrade returned 1603 / Windows Installer 1730 | 1 | The caller was unsandboxed but not elevated; rerun the exact MSI with explicit `RunAs` UAC so removal of the per-machine 1.0.0 product is authorized. |
| Post-install hash summary again piped directly after a `for` block | 3 | Use only explicit collection assignment before formatting for all remaining PowerShell loop summaries. |
| Generated-upstream result string parsed `$UpstreamProtocol:` as a scoped variable | 1 | Delimit all interpolated URI components with `$()` and rerun parser validation. |
| First profile-ACL test compile returned one value from a two-value helper | 1 | Return `nil, err` from the descriptor constructor and rerun the same bounded gate. |
| Phase 35 full Go gate used the protected host TEMP and hit the known one-shot Named Pipe availability race | 1 | Rerun the repository gate with `TEMP`/`TMP` pinned to `.cache/tmp`; retain the first failure as environment/flaky evidence rather than masking it. |
| Phase 35 race gate inherited host `GOARCH=386`, which Windows race builds do not support | 1 | Pin the release architecture to `GOOS=windows` and `GOARCH=amd64` and rerun the same targeted race gate. |
| Pinned Windows amd64 race gate still inherited `CGO_ENABLED=0` | 1 | Enable CGO explicitly and verify whether the host has the required amd64 C toolchain before classifying the gate. |
| Windows amd64 race build has no GCC/Clang/MinGW compiler on this host | 1 | Record the race gate as toolchain-blocked; retain passing normal tests and do not install a machine-wide compiler as an implicit side effect. |
| First Phase 35 packaged smoke launch returned Windows `0xc0000142` before application logging | 1 | Inspect only package-owned processes and the isolated smoke profile, then retry from a clean verified state; do not treat a loader initialization event as a TUN result. |
| Clean-state Phase 35 packaged smoke repeated `0xc0000142` with no process or profile output | 2 | Audit the smoke launcher environment and packaged PE dependencies/manifest instead of repeating the same launch unchanged. |
| Elevated tray-exit helper timed out without closing installed 1.0.5 | 1 | Continue rebuilding from the unlocked workspace release; use Windows Installer Restart Manager or a later explicit foreground UAC action for the installed process instead of repeating a hidden helper unchanged. |
| First 1.0.6 upgrade UAC was not confirmed within the 60-second execution window | 1 | Verify no installer was created and 1.0.5 remained intact, then retry once with the UAC requirement surfaced to the user and a visible passive installer. |
| Windows Installer upgraded files to 1.0.6 but the old 1.0.5 launcher/UI processes remained mapped and the next launch UAC timed out | 1 | Terminate only the verified old Navo launcher PID and its Job Object children, then launch the installed 1.0.6 cleanly; preserve unrelated v2rayN processes. |
| Background UAC taskkill waiter was canceled/closed after three minutes while old Navo remained running | 2 | Require one desktop action: approve the UAC prompt or use Navo tray Exit. Do not claim final TUN acceptance until the old mapped process is gone and 1.0.6 is launched/tested. |
| Generated-config inventory used an invalid PowerShell regex and emitted repeated parse errors until timeout | 1 | Replace recursive regex filtering with bounded directory and extension enumeration; do not repeat the malformed expression. |
| Bounded config inventory repeated the known invalid `foreach { ... } |` PowerShell form | 1 | Collect loop output into an explicit array before formatting; this exact parser failure is not retried. |
| Full `scripts/test.ps1` used the protected host TEMP and failed `TestWriteTempConfig` with access denied | 1 | Pin `TEMP` and `TMP` to repository-local `.cache/tmp` in the test script, matching the package gate, then rerun the full gate. |
| Post-package file summary assumed `navo_app.exe` was at the release root | 1 | Integrity and repair had already passed; read `SHA256SUMS.txt` for the actual packaged relative path instead of repeating the stale layout assumption. |
| First installed acceptance wrapper passed unquoted `C:\Program Files\Navo` through `Start-Process -ArgumentList` and exited before creating an artifact | 1 | Quote the installed package-root argument explicitly and rerun; no Navo process or network mutation occurred. |
| Phase 37 frontend gate invoked `npm.ps1`, which the host execution policy blocks, and PowerShell reported a misleading process exit code | 1 | Use `npm.cmd` explicitly and gate each command result; do not treat the wrapper's zero exit as a passing test. |
| Phase 37 four-policy compile left the removed direct-mode tuple value unused in TUN planning | 1 | Remove the obsolete `mode` read; TUN now always requires a selected outbound for all four supported proxy policies. |
| First Phase 37 browser mock omitted `GetHostStatus`, and its error path left Edge open until the wrapper timed out | 1 | Complete the Wails mock and close the browser in the rejection path before rerunning the same UI assertions. |
| Resumed Phase 37 package passed Go/frontend gates but could not replace `release/Navo/app_ui/navo_app.exe` | 1 | Identify the exact process mapping the release UI binary before stopping anything; preserve the installed/user session and unrelated processes. |
| First Restart Manager lock inspection returned Win32 error 29 | 1 | Allocate the documented 33-character session-key output buffer instead of passing a pre-filled 32-character key, then rerun the read-only inspection. |
| First routing-acceptance summary piped directly after nested `foreach` blocks | 1 | Collect mode rows into an explicit array before formatting; do not rerun the invalid empty-pipeline form. |
| Phase 37 package `npm ci` could not replace the Rolldown native binding because a Navo Vite Node process retained the exact module | 1 | Identify the single Node PID by exact loaded-module path, stop only that process, verify the lock is gone, and resume packaging. |
| Installed Mihomo lifecycle script failed on one redundant post-reenable HTTP request after Service had already verified Google/GitHub | 1 | Preserve the passing product and exact-rollback evidence; add at most three one-second-spaced retries per external site, then rerun the complete lifecycle gate. |
| Installed Mihomo lifecycle rerun hit a real first-request Cloudflare `EOF` during the second Service activation | 1 | Retry only read-only DNS/TCP/HTTPS verification with independent transports and bounded timeouts; keep all network mutation single-shot and fail closed after retry exhaustion. |
| Direct elevated shell launch retained a non-admin token | 1 | Launch the complete acceptance process through Windows `RunAs` and keep its IPC client at the same integrity level. |
| First Phase 37 routing matrix failed after TUN strategy change with `DNS query loopback in transport[dns-proxy]` | 1 | Preserve and reuse the active TUN Activation Plan so routing-only recompiles retain the pinned outbound endpoint IP. |
| Second Phase 37 matrix reached a core restart window and returned an unverified offline `whitelist` commit | 1 | Wait for Supervisor running state before suppressing restart and reject any active-mode update that leaves running state before the transaction. |
| Final package gate hit a one-shot `selfheal-state.json` file lock | 1 | Rerun the unchanged full isolated gate; every Go/Vue/Wails/package check passed. |
| First final Mihomo matrix cold start missed the fixed 10-second listener deadline | 1 | Preserve the rollback artifact and rerun once; the same package then passed all eight combinations with exact cleanup. |
# 2026-08-01 Four-Guide Remediation

## Goal

Implement the four newly supplied design guides in their declared priority order: precise cleanup/privacy initialization first, then P0/P1 remediation, feature optimization, and the bounded self-healing engine. Preserve existing user changes and require code/test/runtime evidence for every completed acceptance item.

## Phases

- [x] Phase 28: Build a requirement-to-code baseline and conflict-resolution matrix from all four guides.
- [x] Phase 29: Remove AI and MySQL-specific surfaces while preserving local diagnostics and local revision/selection behavior; add early initialization and foreign-device privacy cleanup.
- [ ] Phase 30: Implement and verify the Full Remediation P0/P1 security, transaction, recovery, lifecycle, IPC, validation, subscription, persistence, and release gates.
- [ ] Phase 31: Implement the Feature Optimization UI/backend contracts, monitoring, logs, core upgrade, latency testing, IP risk, icons, and accessibility requirements.
- [ ] Phase 32: Implement the bounded self-healing engine with structured events, stable error codes, policy registry, budgets, backoff, circuit breaking, persistence, observability, and fault-injection tests.
- [x] Phase 33: Run full Go/frontend/package/static-remnant gates and applicable Windows data-plane/recovery acceptance; document elevation-dependent gates separately.

## Current Phase

Phases 30-32 remain partially open only where the guides require trust material or privileged/manual acceptance that this change set cannot manufacture. P0/P1 remediation, four-source traffic, structured logs, layered latency, controlled simulation, frontend/release gates, and the bounded SelfHeal foundation are implemented. Trusted automatic core installation, multi-provider IP-risk aggregation, additional production SelfHeal policies, standalone external-pipe identity, and the elevated Windows matrix remain explicit. The four guides and `.claude/` are user-owned inputs and remain preserved.

## Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Frontend baseline command referenced `navo_app/package.json` while already inside `navo_app`, then `npm run test` failed because no test script exists | 1 | Record missing frontend test gate as a baseline defect; rerun package inspection and build from the correct directory without repeating the bad path. |
| Combined AI removal patch applied early hunks before failing on an already-removed import context | 1 | Verified the actual diff, then applied only the remaining smoke/file-deletion hunks. |
| First AI residual scan matched the harmless test domain `openai.com` | 1 | Narrowed the scan to AI package/import/config/IPC symbols instead of a broad `ai.` substring. |
| Removing `ai_settings.go` also removed shared `response`/`failure` helpers used across Service | 1 | Moved the generic IPC response constructors into `internal/service/response.go` and kept AI code deleted. |
| Direct targeted `go test` used the protected user Go build cache | 1 | Use the repository-standard `scripts/test.ps1`, which redirects Go caches into the workspace. |
| Full Go gate found three remaining AI handler tests later in `service_test.go` | 1 | Remove the AI-only tests and retain monitor/diagnostics tests that do not depend on AI. |
| Progress/report patch targeted a status line that was never added after the earlier partial patch | 1 | Inspected file tails and appended the verified AI-removal status using current context. |
| Credential-file sanitization patch was rejected after its initial `.env` and `.env.example` hunks had already applied | 1 | Verified `.env` now contains only two comments and no keys, then applied remaining tracked script/document cleanup separately. |
| Tracked cleanup patch retried an already-applied `.env.example` hunk and failed | 1 | Inspected the actual file and omitted completed hunks from the next patch. |
| PowerShell did not expand `internal\initialization\*.go` for `gofmt` | 1 | Format explicit files or pipe the enumerated Go file paths to `gofmt`. |
| Current User DPAPI is unavailable under the managed test token, breaking tests that persisted real encrypted credentials | 1 | Keep production on required Current User scope; inject test-only protectors/memory credential stores rather than weakening production privacy. |
| Direct invocation of `scripts/test.ps1` was blocked by the host execution policy | 1 | Run the same repository gate through `powershell.exe -NoProfile -ExecutionPolicy Bypass -File`; full tests and vet passed. |
| Endpoint route command test enabled IPv6 tunnel mode without its required gateway | 1 | Add the valid fixture gateway and rerun the full gate; production validation correctly rejected the incomplete fixture. |
| Connectivity fingerprint migration updated two Merge key sites but left the new-item loop on the legacy server/port/type key | 1 | Replace the remaining lookup with the canonical fingerprint; the stable-ID regression exposed the mismatch. |
| Stricter UUID validation exposed an adapter test fixture using the placeholder `id` | 1 | Replace compiler/adapter VLESS fixtures with a valid UUID; retain strict production validation. |
| Phase 43 plan append initially targeted a similarly named heading that existed only in `progress.md` | 1 | Inspected both file tails and appended the new phase at their actual current boundaries. |
| First Phase 43 validation mixed root Go paths with the `navo_app` working directory and invoked policy-blocked `npm.ps1` | 1 | Run `gofmt`/Go tests from repository root and invoke `npm.cmd` from `navo_app` as separate commands. |
| Parallel Vite production build cleaned hashed `frontend/dist` assets while `go test ./navo_app` evaluated `go:embed` | 1 | Serialize frontend build before Go/Wails package tests; do not read the embed tree while Vite replaces it. |
| First browser smoke measured the top-left glyph 7.05px below center | 1 | Found `.brand span` overriding the glyph's grid display; scope text styling to `.brand > div` and rerun the same geometry assertion. |
| Treating `NIM_MODIFY` failure as proof that the icon was absent caused a conflicting `NIM_ADD` attempt | 1 | Use `Shell_NotifyIconGetRect` as the authoritative existence probe and add only when Shell confirms the icon is absent. |
| Managed Go test process was rejected at `startTray`'s initial `NIM_ADD` before the refresh path ran | 3 | Corrected the line-level diagnosis; keep the opt-in test for an interactive desktop token and rely on unit/full-build gates in the managed runner. Installed Launcher logs independently prove initial tray creation succeeds there. |
| Phase 45 package reached `npm ci` but Rolldown native binding was locked | 1 | Inspect exact module owners and stop only stale Navo UI-smoke/Vite processes before one clean package rerun. |
| Second Phase 45 package attempt hit the known intermittent SelfHeal temporary state-file sharing violation | 1 | Prove the SelfHeal package repeatedly in isolation before another full package attempt; do not weaken production file locking. |
| ZIP required-file check initially compared slash paths against PowerShell `Compress-Archive` backslash entry names | 1 | Normalize entry separators before comparison; all five required files are present. |
| Live 1.0.15 portable smoke found the user's old installed 1.0.13 launcher/UI still running | 1 | Refuse to terminate user processes without approval; exit PIDs 5996/24076 before the new root `navo.exe` can own the single-instance mutex. |
| First Phase 44 full gate used a 1-second shell timeout and left overlapping test work; the immediate rerun hit a transient SelfHeal temp-file lock | 1 | Wait for the orphaned Go child to finish, prove `internal/selfheal` passes alone, then rerun the full gate serially. |
| First Phase 44 package attempt could not unlink the Rolldown native binding | 1 | Identified PIDs 22572 and 24384 as the only exact Navo module owners, stopped only those stale UI-smoke Node processes, verified zero remaining owners, and reran successfully. |
| Installing the verified 1.0.14 MSI was rejected because the user had not explicitly authorized a system-level upgrade | 1 | Keep the installer ready and request explicit approval; do not bypass the installation boundary. |

## Phase 43: Close-action confirmation and title-bar alignment

- [x] 43.1 Trace Wails title-bar close, tray hide/show, graceful shutdown, and current settings persistence.
- [x] 43.2 Add an accessible close-action chooser for minimize-to-tray versus complete exit.
- [x] 43.3 Add an option that remembers the selected close action only for the current Windows logon session (until shutdown/restart).
- [x] 43.4 Correct the top-left brand icon alignment without changing the established theme tokens or canonical icon.
- [x] 43.5 Add regression coverage and pass focused Go/frontend/typecheck/build plus browser-visible interaction checks.

## Phase 44: Dual-link IP detection repair

- [x] 44.1 Reproduce and trace direct/proxy IP detection across Wails, cache, providers, and UI state.
- [x] 44.2 Fix the root cause while preserving independent direct/proxy transports and partial availability.
- [x] 44.3 Add regression coverage for the failing path.
- [x] 44.4 Pass focused Go/frontend checks, real direct/proxy detector requests, and verified portable-release gates.

Phase 44 is complete: focused IP tests, full Go test/vet, frontend
test/typecheck/build, Wails production build, portable package repair/hash/ZIP
checks, and real direct/proxy detector probes passed. Per the user's standing
delivery preference, Navo releases are portable green versions only; the 1.0.14
MSI was removed and no installer should be built unless explicitly requested.

## Phase 45: Minimize-to-tray visibility repair

- [x] 45.1 Confirm the reported runtime version/entry point and distinguish UI close from Launcher tray ownership.
- [x] 45.2 Route minimize through UI -> Agent -> Launcher so hiding occurs only after tray refresh succeeds.
- [x] 45.3 Re-register the tray icon after Explorer recreation and show bounded minimize feedback.
- [ ] 45.4 Add regressions and pass frontend, Go, Wails, browser-visible, portable-package, and live new-portable checks.
# Phase 47: ChatGPT and Codex proxy/TUN usability (2026-08-12)

## Goal

Ensure that after enabling either System Proxy or TUN, ChatGPT and Codex traffic uses the selected upstream reliably, including DNS, authentication, API, static assets, and streaming connections, without weakening existing routing policies or rollback guarantees.

## Phases

- [x] **47.1 Source and requirements audit** — Trace rule compilation, DNS, outbound selection, and current OpenAI domain coverage; confirm current official network requirements.
- [x] **47.2 Focused implementation** — Isolate capture, base route, and list overrides; keep the official OpenAI dependency set as editable defaults rather than a hidden invariant.
- [x] **47.3 Regression validation** — Run focused Go tests, full repository tests/vet, frontend/package gates where affected, and configuration validation for both cores.
- [ ] **47.4 Real application acceptance** — Prove no-proxy application traffic to ChatGPT/OpenAI/Codex endpoints through both capture modes, including streaming where testable, then verify exact rollback and zero owned residue.
- [x] **47.5 Portable delivery** — Produce and integrity-check `Navo-1.0.16-portable-x64`; do not claim real application acceptance until 47.4 passes.

## Constraints

- Preserve user-defined blacklist/whitelist/global/bypass behavior and existing DNS leak protections.
- Do not hard-code IP addresses or bypass TLS verification.
- Do not treat a TCP listener, generic website, or package build as ChatGPT/Codex usability proof.
- Do not create an MSI; delivery remains portable ZIP.

## Phase 47 Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| `internal/compiler` test could not create `C:\Users\28484\AppData\Local\Temp\navo\...` under the managed filesystem | 1 | Rerun with `TEMP` and `TMP` bound to repository-local `.cache\temp`; Service and coreadapter tests already passed. |
| First Phase 47 package run passed all Go tests, then `npm ci` hit `EPERM` on the Rolldown native binding | 1 | Identify the exact file owner, stop only a verified stale Navo UI-smoke process, then rerun the package once. |
| Clean package rerun reached Vue typecheck and found `NetworkHealthState` includes `unknown` but the label map omitted that key | 1 | Add the missing explicit `unknown` label, then rerun frontend typecheck before one package retry. |
| First real Phase 47 TUN run used the old four-mode routing model and timed out waiting for the adapter | 1 | User corrected the product model; the run rolled back successfully and left no Navo process. Replace the model before any acceptance retry. |
| Combined focused validation ran npm from repository root after Go tests | 1 | Go packages passed; rerun only frontend commands from `navo_app` with repository-local npm cache. |
| First browser isolation smoke used a non-strict success-text locator that matched progress and final notice | 1 | Scope the assertion to `p.success`; server cleanup completed automatically before rerun. |
| Corrected package run found four orphan Node workers still loading the exact Rolldown binding after browser/dev validation | 1 | Stop only exact module owners 5392/8472/11812/23760, verify zero owners, then perform one clean package rerun. |
| Read-only OpenAI baseline probe used `System.Net.Http` before loading its Windows PowerShell assembly | 1 | Load `System.Net.Http` explicitly, then rerun the same read-only probe without changing proxy state. |
| Windows PowerShell `System.Net.Http` baseline hit the host TLS credential failure before receiving HTTP | 2 | Use a bounded repository-local Go HTTP probe through the same existing proxy; do not treat Schannel as proxy evidence. |
| Final full Go gate could not write the host Go cache and `%TEMP%` inside the managed filesystem | 1 | Bind `GOCACHE`, `GOMODCACHE`, `TEMP`, and `TMP` to repository-local `.cache` paths; full tests and vet then passed. |
| Final frontend gate resolved `npm` to the execution-policy-blocked `npm.ps1` shim | 1 | Invoke `npm.cmd` explicitly; typecheck, 6/6 tests, and production build passed. |
| Corrected real TUN/System Proxy acceptance did not start because the Windows UAC prompt was canceled | 1 | No network or registry mutation occurred. Keep 47.4 open and rerun the elevated isolation matrix when UAC is approved. |
| Phase 47.8 browser helper did not detect Vite port 4191 within 45 seconds | 1 | Inspect the direct dev-server command and retry with a helper-compatible working-directory command; no browser assertions ran. |
| Direct Playwright retry reached port 4192 after its dev-server command had timed out and terminated | 2 | Start Vite and Playwright inside one bounded command and stop the exact spawned process in `finally`. |
| Theme color browser assertion found the night primary action still rendered dark text (`rgb(53,48,48)`) | 1 | Trace the higher-specificity/button state override and bind it explicitly to the night on-surface token. |

## Phase 49: Cold-start capture recovery and peer routing-list entry

- [x] **49.1 Reproduce and trace** - Trace application boot, persisted runtime state, Service/Agent readiness, and the first System Proxy/TUN activation after Windows startup; identify the exact stale-state or readiness failure instead of masking it in the UI.
- [x] **49.2 Repair cold-start activation** - Make the first proxy/TUN click start from a reconciled off state, wait for required control/data-plane readiness, and fail closed with rollback and an actionable error.
- [x] **49.3 Peer blacklist/whitelist interaction** - Present blacklist and whitelist at the same interaction level as System Proxy and TUN. Both remain disabled and closed until explicitly clicked; clicking enables the selected mode and opens its editor, without implicitly enabling capture.
- [x] **49.4 Regression and browser verification** - Add state-machine regressions, run focused/full Go and Vue gates, then verify desktop/compact day/night interaction in Edge.
- [ ] **49.5 Portable acceptance** - Build and integrity-check the next portable ZIP. Treat elevated real proxy/TUN cold-start traffic acceptance as a separate required gate before claiming system-level usability.

### Phase 49 constraints

- Preserve the existing three independent state axes: capture, default route, and explicit list mode.
- Never restore an active capture mode merely because persisted state says it was active before shutdown or reboot.
- List content may persist, but list mode must remain inert until the user explicitly selects blacklist or whitelist.
- Do not overwrite unrelated dirty-worktree changes or stop unrelated proxy processes.

### Phase 49 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| First planning patch matched a mojibake-rendered Unicode separator and could not find the exact line | 1 | Re-anchor the append on an ASCII-only section heading and keep the recorded plan text ASCII-safe. |
| First focused Go gate failed to compile Agent because the new core-switch helper used `strings.TrimSpace` without importing `strings` in `capture_transition.go` | 1 | Add the missing standard-library import, format the file, and rerun the same focused gate. |
| Second focused Go gate reached the new rollback assertion but its expected call trace omitted the verified `tun.status` check performed when stopping TUN | 1 | Keep the production transition unchanged, add the existing health-check call to the test expectation, and rerun the focused gate. |
| Focused rerun bound `GOMODCACHE` to a fresh repository directory and then timed out downloading dependencies from `proxy.golang.org` | 1 | Reuse the machine's populated read-only module cache while retaining repository-local build and temp caches. |
| First Edge smoke expected the abbreviated label `TUN`, while the rendered peer entry retains the established product label `TUN 代理` | 1 | Correct only the browser assertion and rerun the full interaction sequence. |
| First 1.0.19 package run passed all Go gates but `npm ci` could not unlink the Rolldown native binding | 1 | Identify and stop only the two Navo port-4193 Vite workers and their npm parents left by the browser helper, verify the four exact PIDs exited, then retry once. |
| Second 1.0.19 package run hit a Windows sharing violation on a SelfHeal test's temporary state file | 1 | Confirm the package independently passes 10/10 repetitions with an isolated temp root, then run one clean package retry. |
| Elevated isolated TUN/System Proxy acceptance was rejected because it changes host networking and sends Google/GitHub traffic without a separate explicit authorization | 1 | Do not bypass the policy. Keep 49.5 open and request an explicit user approval before running the already prepared isolated acceptance command. |

## Phase 50: Proactive capture hardening review

- [x] **50.1 Transaction audit** - Review the Phase 49 endpoint resolver, Service runtime compilation, Agent core-switch transaction, and persistence boundaries for races, incomplete rollback, stale state, and security regressions.
- [x] **50.2 Reproduce and repair** - Add failing tests for confirmed issues, implement the smallest production-safe correction, and preserve fail-closed capture behavior.
- [x] **50.3 Validation and release refresh** - Run focused/full Go and frontend gates, refresh browser checks if UI changes, then rebuild and integrity-check the portable artifact when material code changes occur.

### Phase 50 constraints

- Do not claim real System Proxy/TUN usability without the separately authorized elevated data-plane acceptance.
- Preserve the current user profile, unrelated processes, and unrelated dirty-worktree changes.
- Prefer deterministic rollback and observable errors over retries that can hide persistent failures.

### Phase 50 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| New ambiguous-response regression left `mihomo` active while the Agent restored TUN and returned an error | 1 | This is the intended red test. Add actual-core reconciliation and verified previous-core restoration before capture recovery. |
| Browser fault injection committed blacklist mode in the mock Service but the editor timed out after the dashboard refresh failed | 1 | This is the intended red browser test. Track mutation commit separately, update local mode immediately, then open/close the editor even when refresh needs a later retry. |
| IPC request-ID reuse regression returned the second handler's normal error instead of rejecting conflicting content | 1 | This is the intended red test. Add a unified Service replay cache keyed by request ID and request fingerprint. |
| Existing adapter-ownership test reused one request ID for two different methods and now correctly hit `REQUEST_ID_REUSE` | 1 | Give each distinct test request a unique ID; retain the production protocol constraint. |
| `go test -race` is unsupported by the installed `windows/386` Go toolchain | 1 | Retain the concurrent owner/waiter regression and cover the implementation with full tests plus `go vet`; record race instrumentation as unavailable. |
| Redundant browser rerun used Python runtimes without the Playwright module | 2 | Preserve the already-passed post-fix browser result; do not install new dependencies or misclassify this as a UI failure. |
| First 1.0.20 package attempt hit a Rolldown native binding lock after all Go gates passed | 1 | Use Restart Manager, stop only the verified Navo Vite PID 4684 and its cmd parent 26508, confirm the lock is gone, then rerun once. |
| Initial `npm install` tried the managed user cache and failed with `EPERM` | 1 | Bind npm to the writable repository-local `.cache/npm`; the exact dependency update and audit then passed. |

## Phase 51: Layout refinement

- [x] **51.1 Visual and structural audit** - Inspect the current rendered connection page and shared page shell at desktop and compact widths; identify hierarchy, spacing, alignment, density, and accessibility problems without changing behavior.
- [x] **51.2 Implement layout system** - Refine App.vue/style.css using the existing tokens, typography, themes, and four peer capture/list controls. Preserve all feature sections and state transitions.
- [x] **51.3 Browser and build verification** - Run Vue tests/typecheck/build, verify desktop/compact day/night layouts with zero overflow and browser errors, visually inspect screenshots, and refresh the portable artifact if the UI materially changes.

### Phase 51 constraints

- Preserve System Proxy, TUN, Blacklist, and Whitelist as peer entry points; lists remain closed and disabled until clicked.
- Do not modify backend, capture, routing, or persistence behavior.
- Preserve existing semantic tokens, Segoe UI Variable/Cascadia Code roles, keyboard focus, ARIA states, and reduced-motion handling.
- Preserve unrelated dirty-worktree changes and use the portable green package as the deliverable.

### Phase 51 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| First combined App.vue/CSS patch matched console-mojibake text instead of the file's UTF-8 Chinese | 1 | Reapply the template change with the correct Chinese source text, then add the CSS override separately. |
| First Phase 51 browser geometry gate measured the compacted connection intro at 92.42px, above the 90px target | 1 | Convert the intro to a two-column title/description information band, then rerun the same interaction and layout assertions. |
| First 1.0.21 package attempt passed all Go gates but browser testing left a Vite child holding the Rolldown binding | 1 | Use Restart Manager to identify the exact Navo child, stop only that verified process tree, confirm the lock is released, then retry once. |
| Browser artifact names said `night`, but initial theme followed the headless system preference and rendered day mode | 1 | Explicitly select night before night/compact assertions and explicitly switch to day for the day screenshot; rerun the full browser gate. |

## Phase 52: Full connection-chain stability audit

- [x] **52.1 Baseline and ownership audit** - Preserve the dirty worktree, identify current Navo-owned processes/listeners/state, run deterministic baseline tests, and map every connection transition from UI through IPC, Service, core, Windows capture, verification, and rollback.
- [x] **52.2 Failure-boundary review** - Inspect concurrency, timeouts, cancellation, replay, persistence, DNS/endpoint resolution, core readiness, System Proxy/TUN ownership, route/DNS/firewall transactions, monitoring, and shutdown/recovery for remaining correctness or stability bugs.
- [x] **52.3 Reproduce and repair** - Added focused regressions and repaired SelfHeal circuits/lifecycle, Agent cancellation/retry, replay panic poisoning, Supervisor split-brain restart, Service mutation serialization, node/TUN endpoint switching, healthy-revision commit timing, WinINet application probing, inactive TUN configuration, and reversible source deletion.
- [x] **52.4 Automated verification** - Focused/full Go tests and vet, frontend tests/typecheck/build, npm audit, PowerShell parser checks, `git diff --check`, portable repair, SHA manifest, ZIP structure, and Windows/amd64 PE checks all pass for 1.0.22.
- [ ] **52.5 Real data-plane acceptance** - The isolated current-package System Proxy/TUN command was prepared and invoked, but Windows UAC was canceled before Navo launched. DNS/TCP/HTTPS, Google/GitHub, exit-IP, delayed stability, recovery, and rollback therefore remain unverified on the real host.

### Phase 52 constraints

- A listener, core process, adapter, UI state, `HEALTH_COMMITTED`, build, or package is not end-to-end connection proof.
- Do not stop or modify unrelated v2rayN/user processes. Stop a Navo process only after proving ownership and only when required by the requested validation.
- Do not infer adapter/route/DNS cleanliness from non-elevated `Access denied` output.
- Keep network mutations single-shot and fail closed; bounded retries are allowed only for read-only readiness and traffic probes.
- Deliver only a portable green directory and ZIP if a release refresh is required; do not build an MSI.

### Phase 52 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Initial parallel repository/memory inventory returned only the successful root listing because another read-only subcommand failed | 1 | Split the inventory into bounded independent commands; no repository or runtime state was changed. |
| Broad recursive `AGENTS.md` discovery returned exit 1 under protected/cache paths | 1 | Restrict instruction discovery to the repository root and known source directories; no nested `AGENTS.md` exists there. |
| First Phase 52 planning patch used a mojibake-rendered final-line anchor and did not apply | 1 | Re-anchor the insertion on an ASCII-only section heading; no file was changed by the failed patch. |
| Combined output for the two long proxy/TUN acceptance guides was truncated by the tool display budget | 1 | Read the guides in bounded numbered chunks before making connection-lifecycle decisions. |
| Phase 52 full Go baseline failed in `TestVerificationFailureRollsBackAndOpensCircuit` with a Windows sharing violation on `selfheal-state.json` | 1 | Treat the repeated full-suite-only lock as a connection-recovery persistence bug; reproduce under focused concurrency and repair the file ownership/synchronization boundary instead of retrying the unchanged full gate. |
| Broad StateFile symbol scan entered protected historical acceptance profiles and timed out | 1 | Restrict later symbol searches to `cmd`, `internal`, `navo_app`, and `scripts`; the partial result already confirmed SelfHeal state ownership is initialized by Service. |
| Direct invocation of `scripts/test.ps1` was blocked by the host PowerShell execution policy | 1 | Re-ran the same repository script through `powershell.exe -NoProfile -ExecutionPolicy Bypass`; full Go tests and vet passed. |
| Broad recursive subscription call-site scan crossed protected historical `.cache/tun-acceptance` profiles | 1 | Used the returned source hits, then kept subsequent searches bounded to `internal/service` and `internal/subscription`. |
| First ZIP structure assertion assumed `/` entry separators and counted the root directory as a file | 1 | Inspected the archive directly, normalized Windows `\` separators, excluded directory entries, and verified all 25 package files are rooted and present. |
| New subscription detach regression failed because Windows DPAPI was unavailable in the sandbox temporary profile | 1 | Kept production DPAPI unchanged; rewired the unit test to an in-memory credential store with identity protectors, then passed 5/5 focused repetitions and the full suite. |
| Elevated 1.0.22 System Proxy/TUN acceptance waited for consent and Windows reported `The operation was canceled by the user` | 1 | Did not retry or bypass UAC. Verified read-only that WinINet remained disabled, no Navo process/listener or acceptance artifact was created, and the unrelated v2rayN process remained untouched. |
| Final read-only source/process report used a stale `internal/subscription/manager.go` path, then CIM process enumeration was denied | 1 | Corrected the path to `subscription.go`; used non-mutating `Get-Process` plus listener inspection to reconfirm only unrelated v2rayN sing-box PID 12952 and no known Navo listener. |

## Phase 53: UI request identity collision

- [x] **53.1 Reproduce and trace** - Traced the reported ID through Wails/Agent/Service and reproduced deterministic parent-ID reuse across distinct Service methods.
- [x] **53.2 Repair identity ownership** - Agent now assigns a random process-session namespace plus monotonic child ID per logical Service request, preserves it across transport retries, and restores the UI parent ID only on the return hop.
- [x] **53.3 Validate and refresh** - Focused regressions pass 10/10; Agent/Service pass 3/3; full Go tests/vet, frontend tests/typecheck/build, npm audit, diff check, package repair, manifest, ZIP, and PE checks pass for 1.0.23.

### Phase 53 constraints

- Keep Service replay conflict detection fail closed; do not weaken `REQUEST_ID_REUSE` into silently executing conflicting mutations.
- Retries of one logical request must retain the same ID so at-most-once execution still works after a lost response.
- Preserve existing user-owned changes and do not touch live proxy/TUN or unrelated v2rayN processes.

### Phase 53 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| `rg.exe` was blocked by host `Access denied`; broad fallback entered generated frontend output and produced excessive matches | 1 | Restrict subsequent `Select-String` and file reads to authored frontend/API, Wails bridge, Agent, and Service replay files only. |
| First focused Agent regression could not create the managed user Go build-cache entry | 1 | Re-run with repository-local `.cache\go-build` and `.cache\tmp`; the failure occurred before package setup and is not a product verdict. |
| Repository-local build cache exposed a second setup denial on the default user module-cache lock for `golang.org/x/sys` | 1 | Inspect configured/populated module caches and reuse an existing read-only populated cache while keeping build/temp writes inside the repository. |
| Final combined report treated Git CRLF warnings on stderr as a terminating PowerShell error before printing the already-verified artifact hash | 1 | Run the read-only diff check with native stderr tolerated and gate explicitly on `$LASTEXITCODE`; no source or artifact was changed by the reporting failure. |

## Phase 47 corrected product model

- Previous three-axis interpretation is superseded by the user's second correction.
- [x] **47.6 Mode-level isolation** — Model blacklist and whitelist as independent opt-in modes; activating System Proxy/TUN must not activate either list mode.
- [x] **47.7 Migration and regression** — Preserve stored lists without silently enabling them, test mode transitions, and rebuild the portable release.
- [x] **47.8 Browser-visible verification** — Verify the connection UI presents capture, default route, and list mode as peer control axes without subordinate auto-active list semantics.
- [x] **47.9 Theme text contrast** — Use dark text across day surfaces and light text across night surfaces; retain color accents for state and charts only, then verify both themes in Edge.
# Phase 54: System Proxy readiness HTTP 502 (2026-08-14)

## Goal

Eliminate false or opaque `CAPTURE_TRANSITION_FAILED` failures when enabling System Proxy, while preserving fail-closed WinINet ownership and proving the selected core/upstream with real HTTP/HTTPS data-plane traffic.

## Phases

- [x] **54.1 Reproduce and isolate** — correlate the reported 502 with the exact core log, generated runtime config, selected outbound, probe target, and current process/listener ownership.
- [x] **54.2 Focused production repair** — fix the responsible readiness/routing/lifecycle boundary and add deterministic regressions.
- [x] **54.3 Automated validation** — pass focused tests, full Go test/vet, frontend gates where affected, and diff checks.
- [ ] **54.4 Live acceptance** — enable System Proxy with the real profile, prove default-proxy and explicit-proxy HTTP/HTTPS traffic plus Google/GitHub, then disable and verify exact rollback/residue.

## Constraints

- Do not enable WinINet unless the local proxy completes a real end-to-end request.
- Do not treat a listener, process, TCP connect, or core API response as data-plane success.
- Preserve unrelated v2rayN/user processes and all existing dirty-worktree changes.
- Do not weaken TLS verification or silently bypass the selected upstream.

## Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Initial combined source/log inventory exceeded 10 seconds | 1 | Narrowed searches to exact source files and bounded profile/runtime directories. |
| Live `outbound.testAll` wrapper exceeded 60-second shell limit | 1 | Reuse a 120-second bounded window and emit only compact results. |
| Compact PowerShell retry omitted whitespace in `foreach ($e in $x...)` | 1 | Correct the parser typo before any product process starts; no runtime state changed. |
| Focused Go test could not download modules from `proxy.golang.org` | 1 | Rehydrated repository-local cache from the read-only global cache and fetched only missing pinned versions through `goproxy.cn`; subsequent tests run offline. |
| Elevated portable probe could not access the normal-user test pipe and left two test-owned processes | 1 | Confirmed exact PIDs, then used a single approved UAC cleanup; both test-owned processes were removed without touching v2rayN. |
| First 1.0.24 package run exposed delayed Named Pipe creation race | 1 | Replaced spin retries with a 4-second deadline and `FILE_NOT_FOUND` backoff; delayed-listener regression passed 20x on host arch and 5x on amd64. |
| Second 1.0.24 package run hit a transient amd64 linker failure in `internal/pipe` | 1 | Isolated amd64 package passed 5x and warmed the build cache; next package run passed the Go gate. |
| Third 1.0.24 package run reached `npm ci` then received registry `ECONNRESET` | 1 | Retry against the configured China npm mirror; no release directory was created before the failed source gate. |
| Elevated 1.0.24 routing-mode acceptance was canceled at UAC | 1 | No retry or privilege bypass; retain 54.4 as pending and report the exact external acceptance action. |

# Phase 56: elevated TUN acceptance (2026-08-14)

- [x] Confirm current 1.0.26 package, normal-user network baseline, and absence of Navo-owned processes.
- [x] Strengthen TUN routing/list-mode evidence and rollback/residue assertions without mutating host networking.
- [ ] Run elevated sing-box and Mihomo TUN routing matrix with real DNS/TCP/HTTPS, exit-IP, and stability evidence.
- [ ] Run elevated lifecycle, injected-failure, and crash-recovery suites.
- [ ] Verify final rollback, zero owned residue, repair manifest, and release ZIP after the elevated matrix.

## Authorization boundary

The user explicitly authorized the complete elevated TUN matrix on 2026-08-14, including temporary Navo adapter, route, DNS/NRPT, IPv6 firewall, and short-connectivity-interruption effects. Run only the bounded acceptance scripts and require exact rollback.

## Phase 56 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| `Start-Process -Verb RunAs` did not return before the 30-second execution deadline | 1 | Verified no output directory, matrix/Navo process, adapter, or network mutation; retry with a visible UAC window and a 120-second confirmation window. |
| Visible `RunAs` request also timed out before process creation after 120 seconds | 2 | Verified zero artifacts/processes again; treat desktop UAC interaction as the current external boundary and inspect only already-configured elevated entry points. |
| First real routing case reported `TUN_ADAPTER_NOT_READY` despite an Up adapter with the correct address | 1 | Native evidence proved `MIB_IF_ROW2.Mtu=65535` was the wrong layer; read IPv4 `MIB_IPINTERFACE_ROW.NlMtu` and admit only the two known non-hardware Wintun/sing-tun descriptions. |
| Rollback snapshot differed only on VMware default-route dynamic `State` (`2` to `1`) | 1 | Remove volatile route `State` from exact resource rollback comparison; retain prefix, interface, next hop, metric, and protocol. |
| Compiler test wrote the protected system `%TEMP%` despite repository Go caches | 1 | Set `TEMP`, `TMP`, and `GOTMPDIR` to `.cache\tmp`; focused Compiler rerun passed. |

# Phase 57: crash-safe DNS and boot proxy recovery (2026-08-15)

## Goal

Prevent an abrupt Navo/Windows crash from leaving system DNS unusable, and make proxy activation after Windows boot wait for reconciled profile, Service, Agent, core, and data-plane readiness instead of failing from stale state or startup races.

## Phases

- [x] **57.1 Current-state evidence** - Correlate Windows DNS/WinINet, startup registration, Navo process/listener ownership, journals, runtime state, and logs without mutating live networking.
- [x] **57.2 Lifecycle trace and reproduction** - Trace boot/startup and crash recovery through Launcher, UI, Agent, Service, Supervisor, network journal, DNS ownership, and capture transition; add deterministic failing regressions.
- [x] **57.3 Production repair** - Make DNS restoration crash-safe and boot activation dependency-aware, bounded, idempotent, fail-closed, and actionable while preserving unrelated proxy/DNS owners.
- [x] **57.4 Automated verification** - Focused repetitions, full Go test/vet, frontend tests/typecheck/build, PowerShell parser checks, package gate, and diff checks passed.
- [x] **57.5 Windows acceptance and portable delivery** - Real after-NRPT process crash/restart recovery and fresh isolated-profile System Proxy activation passed on the current boot; exact DNS/WinINet rollback, zero Navo residue, and the 1.0.28 portable ZIP were verified.

## Constraints

- Never overwrite DNS or WinINet settings that no longer match Navo's recorded ownership.
- Recovery must be idempotent and must complete before any automatic capture activation is attempted.
- Do not restore an active capture mode solely from persisted UI/runtime state after reboot.
- Preserve unrelated v2rayN/user processes and all existing dirty-worktree changes.
- A process, listener, adapter, or UI connected state is not acceptance; require real DNS/TCP/HTTPS data-plane traffic and exact rollback.

## Phase 57 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Native `apply_patch` could not read the workspace because the Windows sandbox ACL helper failed | 1 | Applied the same reviewed unified diffs with `git apply`; no unrelated path was edited. |
| The pinned Go module cache was incomplete with `GOPROXY=off` | 2 | Downloaded the pinned modules once through `goproxy.cn`, then reran focused and full tests offline from the workspace cache. |
| First 1.0.27 crash acceptance never reached `after-nrpt`; adapter readiness rejected `sing-tun Tunnel #2` | 1 | Preserved strict name/non-hardware/GUID/index ownership and admitted only known tunnel descriptions with a numeric Windows ` #N` suffix; added positive and negative regressions. |
| Crash runner masked a pre-crash activation error as `Crash point did not terminate Navo` | 1 | Fail immediately on IPC `ERROR`; suppress only the expected transport exception after the process has actually exited. |
| Non-elevated System Proxy wrappers lacked `Get-FileHash` or were rejected by the requireAdministrator launcher | 3 | Reran through the same elevated Windows PowerShell context used by the real portable launcher; every preflight failure restored WinINet. |
| A prior detached validation process was still finishing graceful shutdown | 1 | Safety gate refused the next run; waited for verified Navo PIDs to exit and confirmed v2rayN PID 5000 remained before retrying. |
| The first ZIP included two runtime-generated files and the first manifest check used a nonexistent filename | 1 | Removed only `data/navo.ico`, `log/navo.log`, and the just-created ZIP; verified the real `SHA256SUMS.txt` 24/24 and rebuilt a clean 25-file ZIP. |
