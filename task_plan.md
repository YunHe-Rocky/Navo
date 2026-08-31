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
- [x] Phase 59: Implement docs/Navo_架构边界_调度与自愈设计规范_V1.md: establish one Connection Coordinator, enforce observation/control ownership boundaries, serialize all network mutations, constrain Scheduler to triggers, add two-round domain-scoped SelfHeal, preserve capture/policy on node changes, and verify transactional rollback/failover behavior.

### Phase 59 execution

- [x] 59.1 Map the V1 architectural rules to current packages, call paths, state owners, and test coverage.
- [x] 59.2 Introduce or consolidate typed domain contracts and the single cross-domain Connection Coordinator.
- [x] 59.3 Route Scheduler and SelfHeal mutations through the coordinator with bounded two-round, fault-domain-scoped policy.
- [x] 59.4 Enforce Active/Candidate node commit, capture/policy preservation, rollback to the prior node, and same-source failover boundaries.
- [x] 59.5 Add focused regression coverage, run repository gates, and document remaining elevated Windows acceptance.

### Phase 61: complete all deferred V1 behavior and acceptance

- [x] 61.1 Reconcile the dirty worktree and Phase 60, inventory current health probes, SelfHeal policies, source-channel metadata, failover paths, UI state, and elevated acceptance scripts.
- [x] 61.2 Define and implement the fault-evidence matrix, stable attributed error report, and round-1/round-2 minimal repair actions for every controllable fault domain.
- [x] 61.3 Implement Coordinator-owned active-node recovery and on-demand same-channel candidate discovery, ranking, real validation, commit, rollback, and terminal reporting.
- [x] 61.4 Finalize BLACKLIST/WHITELIST semantics in the canonical rule model and expose complete transaction, recovery, failover, evidence, and impact state in Wails/Vue UI.
- [in_progress] 61.5 Add regression/integration coverage, run all source/package gates, then execute elevated System Proxy/TUN DNS/TCP/HTTPS/exit-IP/stability/crash/recovery/rollback/residue acceptance without touching unrelated processes.

### Phase 62: reported enabled-but-no-traffic proxy incident

- [x] 62.1 Capture the current process, listener, WinINet, adapter, route, DNS, runtime-log, and no-proxy traffic evidence without mutating networking or stopping unrelated processes.
- [x] 62.2 Reproduce the failing System Proxy/TUN path and identify the first divergence between committed control-plane state and actual application traffic.
- [x] 62.3 Implement the narrowest production repair plus regression coverage, preserving Coordinator ownership and fail-closed readiness semantics.
- [ ] 62.4 Run focused/full source gates, build the portable green directory and ZIP, then prove real DNS/TCP/HTTPS/exit-IP/stability/rollback behavior on the repaired build.

### Phase 63: wire failed activation into bounded SelfHeal

- [x] 63.1 Prove whether the missing trigger is health-monitor startup, fault observation, debounce, or recovery scheduling.
- [x] 63.2 Release the failed user capture transaction, then enter Coordinator-owned bounded SelfHeal for a faulted requested mode without recursive mutation ownership.
- [x] 63.3 Correct fault attribution so host-path timeouts use node repair/failover while adapter faults use TUN recovery; add activation-recovery and classification regressions.
- [in_progress] 63.4 Run full gates, rebuild the portable package, and execute elevated TUN failure/recovery/rollback acceptance when UAC is approved.

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

Phase 62 is in progress. The user reports that capture enables without an error but
ordinary traffic does not pass. Treat only real no-explicit-proxy DNS/TCP/HTTPS and
exit-IP evidence as success; preserve unrelated v2rayN/user processes and current
dirty-worktree changes.

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
| Phase 76 focused Go tests could not populate the isolated module cache because direct `proxy.golang.org` connections timed out | 1 | No test compiled and no product state changed; rerun with process-local HTTP(S) proxy variables through the already-preserved external endpoint. |
| First Service build used TUN `VerifyResult` for a System Proxy warm lease | 1 | Compiler rejected the mismatch before tests; keep the model strongly typed and switch it to `RuntimeRoutingVerification` instead of using an untyped payload. |
| Strong-type correction initially missed the two helper return signatures | 1 | Compiler caught both declarations; aligned retain/resume signatures with `RuntimeRoutingVerification` before any Service test ran. |
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

# Phase 58: project hardening and release closure (2026-08-15)

## Goal

Close the locally actionable gaps found by the objective project audit: one authoritative version, sealed portable verification, subscription reliability, CI/security gates, accurate documentation, third-party notices, and regression coverage. Preserve live networking and unrelated user processes until the separate elevated acceptance step.

## Phases

- [x] **58.1 Version and package integrity** - Add one version source, inject it into Go/Wails/PE/package metadata, reject manifest drift and unexpected portable files, and validate the clean ZIP from a sealed staging directory.
- [x] **58.2 Subscription reliability** - Preserve SSRF fail-closed behavior while trying all validated public DNS addresses, propagate asynchronous apply failures, and add regressions.
- [x] **58.3 CI and supply-chain gates** - Add coverage, race, dependency/security, PowerShell parser, package/version/license checks, and immutable GitHub Action references where feasible.
- [x] **58.4 Documentation and repository hygiene** - Remove stale AI/MySQL/Flutter/installer claims, document the current Wails/Vue architecture and portable workflow, add third-party notices, and prevent new sensitive acceptance artifacts from being committed.
- [x] **58.5 Automated validation and portable rebuild** - Run focused/full Go tests and vet, frontend tests/typecheck/build/audit, PowerShell parser checks, package verifier, manifest/ZIP/PE checks, and `git diff --check`.
- [ ] **58.6 External Windows acceptance** - Re-run the current sing-box/Mihomo elevated TUN matrix, perform a physical reboot/cold-start acceptance, and add Authenticode signing when a certificate is supplied.

## Constraints

- Do not mutate live TUN, routes, DNS, NRPT, firewall, or WinINet during local hardening.
- Do not stop or modify unrelated v2rayN/user processes.
- Keep portable green directory plus ZIP as the delivery model; do not add MSI/installer work.
- A build, listener, manifest subset, or UI state is not end-to-end acceptance.
- Do not claim signing or physical reboot acceptance without the external certificate/action and resulting evidence.

## Phase 58 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Native `apply_patch` tool could not read the workspace because the Windows sandbox ACL helper failed | 1 | Tried the local `apply_patch.bat` helper through an approved elevated shell. |
| Local `apply_patch.bat` helper was blocked with `Access is denied` | 2 | Use repository-scoped `git apply --recount` with reviewed unified diffs; do not use ad-hoc file writes. |
| Initial `git apply` rejected the append diff because its manual hunk line count was wrong | 1 | Re-run the same reviewed content with `--recount`; no file was changed by the rejected patch. |
| Combined `--recount` patch used stale `findings.md` and then `progress.md` context | 2 | Read the real EOF, split the files, and use exact per-file anchors; no file was changed by either rejected patch. |
| Zero-context package edits initially landed inside adjacent PowerShell blocks | 1 | Re-read the exact command boundaries, moved each statement, and required parser plus full package execution. |
| Go 1.26 rejected the legacy equals-form cover arguments and the default module cache could not reach proxy.golang.org | 1 | Use separate cover arguments and the repository-local module/build cache; total coverage gate passed at 51.9%. |
| PowerShell enumerated an empty HashSet return into null | 1 | Return the HashSet as one object; old-package rejection and final directory verification passed. |
| Wails existing RT_VERSION remained 0.0.0 after a JSON resource merge | 1 | Pass explicit go-winres file/product version flags; all three release PE versions are 1.0.29.0. |
| Initial README/CLAUDE generation interpreted Windows backslashes as control escapes | 1 | Backed up both files, restored the baseline, regenerated exact UTF-8 diffs, scanned control bytes, and passed git diff --check. |
| Normal-user package smoke could not access the elevated UI Agent pipe | 1 | Verified zero Navo residue and preserved v2rayN; an all-elevated retry was canceled at UAC, so runtime smoke remains pending. |

# Phase 60: explainable network risk and ChatGPT usability (2026-08-17)

## Goal

Replace optimistic capture-state reporting with evidence-based, explainable network risk, and make ChatGPT desktop usability an explicit end-to-end acceptance gate covering routing, DNS, TLS/HTTPS, authentication/assets, API, streaming/WebSocket, stability, and exact rollback.

## Phases

- [x] **60.1 Current-state diagnosis** - Correlate UI risk derivation, committed capture state, generated routing, live core/listener ownership, Windows proxy/TUN state, and current ChatGPT application traffic without mutating unrelated processes.
- [x] **60.2 Risk model repair** - Derive severity and explanations from fresh probe evidence, distinguish enabled/healthy/degraded/unusable/stale states, and provide a concrete recovery action without relying on color alone.
- [x] **60.3 ChatGPT routing repair** - Fix the narrowest responsible routing, DNS, lifecycle, or policy boundary while preserving blacklist/whitelist/global/bypass semantics and fail-closed rollback.
- [x] **60.4 Regression and UI verification** - Add focused Go/Vue regressions and verify both themes plus compact layout with browser-visible evidence.
- [x] **60.5 Real application acceptance** - Prove ChatGPT desktop DNS, auth/assets, API, and streaming/WebSocket behavior through the active Navo path, delayed stability, disable rollback, and zero Navo-owned residue.

## Constraints

- UI “enabled”, a listener, core process, adapter, TCP connect, or Google/GitHub success is not ChatGPT acceptance.
- Preserve unrelated v2rayN/user processes and do not weaken TLS verification or add broad third-party route suffixes.
- Do not claim current network state from non-elevated `Access denied`; elevated TUN mutation remains a separately evidenced gate.
- Keep portable green directory plus ZIP as the delivery model.

## Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Initial combined skill read was truncated, and the first sandboxed read hit the Windows ACL helper | 1 | Re-read both skill files completely in bounded elevated chunks before project actions. |
| Native `apply_patch` could not read the workspace because the Windows sandbox ACL helper failed | 1 | Applied the same reviewed EOF-only diff with repository-scoped `git apply --recount`; no unrelated path was edited. |
| First `git apply --recount` input omitted a valid hunk range and was rejected as garbage | 1 | Counted the exact EOF line and reapplied a standard one-context hunk; the rejected patch changed nothing. |

## Phase 60 completion evidence

- System Proxy and TUN activation now fail closed on named ChatGPT web/auth/API/assets/stream evidence; System Proxy additionally requires the current-user WinINet PRECONFIG path before commit.
- The capture snapshot persists readiness state, named-site DNS/TCP/TLS/HTTP evidence, default-proxy proof, timestamp, and actionable error. Vue treats readiness as a hard health condition and keeps IP attributes separate.
- Focused Go suites and the full package gate passed. Frontend behavior tests passed 9/9; TypeScript, Vite production build, and browser-visible ready/stale/failed/IP-attribute branches passed in both themes with zero browser errors.
- Elevated isolated System Proxy acceptance passed twice at 2026-08-17 10:44:40 and 10:45:19 Asia/Shanghai through the existing v2rayN upstream. All five named ChatGPT edges passed, WinINet PRECONFIG was true, delayed verification passed, rollback was exact, v2rayN PID 12600 remained, and no Navo process/listener remained.
- Delivered `Navo-1.0.30-portable-amd64` plus ZIP. Package verifier reported 28 files, 27 manifest entries, 0 issues; ZIP SHA-256 is `93A82680D719F56C8588681C7B975DB6439D15EDBE27CCBDAA18DD27F7DB6C2C`.

## Phase 60 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Native `apply_patch` could not read workspace files because the Windows sandbox ACL helper failed | 1 | Applied reviewed repository-scoped unified diffs with `git apply`; rejected hunks changed no files. |
| One manual Agent dispatch hunk omitted the response-map closing line | 1 | `gofmt`/Go tests caught it immediately; inserted the missing close, formatted, and reran the complete suite. |
| Python Playwright and the bundled Playwright browser were unavailable | 2 | Used bundled Node Playwright with the installed system Chrome; the same DOM, theme, error, and screenshot assertions passed. |
| First portable build hit `EPERM` unlinking Rolldown after browser testing | 1 | Identified and stopped only the nine test-owned npm/Vite PIDs, confirmed ownership by command line, then reran the full package gate successfully. |
| Local image-view helper hit the same Windows ACL helper after screenshots were created | 1 | Retained the screenshots and required browser DOM/semantic assertions plus zero console/page errors; no product validation was skipped. |

## Phase 62 errors encountered

- Native `apply_patch` could not read workspace files because the Windows deny-read ACL helper failed; no workspace file changed.
- The first two reviewed `git apply` fallback patches had incorrect unified-diff hunk counts and were rejected before mutation; the corrected minimal patch applied cleanly.
- The first startup/log inventory script had an invalid PowerShell empty pipeline element; it was rejected before reading state, then replaced with explicit array collection.
- The first acceptance-result parser repeated the invalid PowerShell foreach-to-pipeline form and was rejected before reading output; explicit array collection resolved it.
- Native `apply_patch` was again blocked by the Windows ACL helper for the acceptance diagnostic edit; the reviewed minimal fallback patch applied and passed PowerShell AST/diff checks.
- Native `apply_patch` was blocked before the production repair; the same reviewed five-file patch was applied with `git apply --recount`, then formatted and whitespace-checked.
- The address-family diagnostic rerun was not executed because Windows returned `The operation was canceled by the user` at UAC. No network mutation occurred and WinINet remained on v2rayN 10808.
- The first focused Go test used `.cache\go-mod` with `GOPROXY=off`, but the repository standard cache is `.cache\go\pkg\mod`; dependency setup failed before compile. Rerun uses the existing standard cache without downloading.
- The standard repository Go cache was also empty, so the second offline focused attempt again stopped before compile.
- Third attempt downloaded only go.mod/go.sum-locked modules through goproxy.cn into the repository cache; all focused packages passed.
- The repaired 1.0.31 elevated TUN acceptance did not start because Windows returned `The operation was canceled by the user` at UAC; no product or network mutation occurred.
- Final package verification initially queried a non-existent root-level `navo-agent.exe`; corrected inventory confirmed the actual launcher, repair, and `app_ui/navo_app.exe` PE versions/hashes without changing the package.
- The first combined Phase 63 patch was rejected by `git apply --check` because one test hunk had stale context; production code was unchanged, then applied separately.
- A zero-context test append landed inside the previous composite literal; `gofmt` caught it before compile. The complete test-only patch was reversed, split, anchored at the real EOF, and then formatted successfully.

# Phase 64: core capture and routing closure (2026-08-20)

## Goal

Make System Proxy, TUN, blacklist mode, and whitelist mode real end-to-end product capabilities: each must have an explicit frontend control and truthful state, a canonical backend execution path, core-specific generated routing, behavioral regression coverage, and live Windows data-plane/rollback evidence appropriate to its scope.

## Phases

- [x] **64.1 Inventory and baseline** - Re-establish current process/network ownership, map UI/Wails/Agent/Service/compiler/core paths, and identify missing or contradictory behavior without mutating unrelated processes.
- [ ] **64.2 System Proxy closure** - Verify and repair explicit UI controls, WinINet ownership, local proxy readiness, named application traffic, mode switching, disable, and exact rollback.
- [ ] **64.3 TUN closure** - Verify and repair explicit UI controls, privilege/ownership checks, adapter/route/DNS/firewall lifecycle, ordinary no-proxy host traffic, stability, recovery, and exact rollback for supported cores; fail closed for unsupported Xray TUN.
- [ ] **64.4 Blacklist/whitelist closure** - Preserve independent opt-in list modes, canonical semantics, precedence, persistence, and backend generation; prove matched and unmatched destinations behaviorally across capture modes and supported cores.
- [x] **64.5 Automated and browser gates** - Run focused/full Go gates, frontend tests/typecheck/build, browser-visible interaction/state checks, PowerShell parser checks, diff checks, and package verification.
- [ ] **64.6 Elevated live acceptance and delivery** - Run the bounded System Proxy/TUN/routing matrix with DNS/TCP/HTTPS/exit-IP/stability/re-enable evidence, verify exact rollback and zero Navo-owned residue, preserve v2rayN, and produce the portable directory plus ZIP if source changes require a refresh.

## Constraints

- A UI state, listener, adapter, core process, generated config, or `HEALTH_COMMITTED` is not data-plane acceptance.
- Blacklist and whitelist are independent opt-in routing modes; enabling System Proxy or TUN must not silently enable either list mode.
- Preserve canonical semantics: blacklist match routes through the selected proxy, whitelist match routes direct, private CIDRs remain direct, and unmatched traffic follows the selected base runtime mode.
- Xray TUN must remain explicitly unsupported unless the bundled version and owned adapter path can pass the same lifecycle gates.
- Never overwrite WinINet/DNS/routes/firewall state that Navo does not own; rollback must be exact and idempotent.
- Preserve unrelated v2rayN/user processes and all existing dirty-worktree changes.
- Keep portable green directory plus ZIP as the delivery model; do not create MSI/setup output.

## Phase 64 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| The first full Go gate session was interrupted while downloading pinned modules when a new user message steered the turn; the returned PTY session no longer existed | 1 | Confirm no orphaned test process remains, then rerun the same repository gate once from the warmed repository cache and record only the complete result. |
| The clean full Go gate reached package setup but `proxy.golang.org` timed out fetching pinned modules; unaffected dependency-free packages passed, but this is not a valid full verdict | 2 | Populate the repository module cache through the previously validated `goproxy.cn` path, then rerun the unchanged full gate offline from that cache. |
| The required Python Playwright module is not installed and the project has no local Playwright dependency | 1 | Use the already-installed system Chrome with a bounded existing/bundled browser automation runtime if available; do not add a production dependency merely for the smoke test. |
| First browser smoke reached the theme step but the accessible-name selector `日` also matched `设置与日志` | 1 | Use an exact accessible name for the one-character theme buttons and rerun the same end-to-end interaction script. |
| A read-only `Select-String -LiteralPath` check incorrectly passed a wildcard path and was rejected | 1 | Enumerate the bounded Agent Go files first, then pipe them to `Select-String`; no source or runtime state changed. |
| The first 1.0.33 package attempt set `GOPROXY=off`; project gates passed, but the locally rebuilt Wails CLI still lacked its own transitive modules | 1 | The failure occurred before release output creation. Rerun the unchanged package script with the previously validated `goproxy.cn` mirror, then accept only a fully verified directory and ZIP. |
| Non-elevated packaged System Proxy acceptance could not start because the production launcher correctly requires elevation | 1 | The runner restored the pre-existing v2rayN WinINet snapshot and left zero Navo residue; use one elevated wrapper for all product scenarios. |
| The bounded 11-case elevated wrapper did not start because Windows reported `The operation was canceled by the user` at UAC | 1 | No Navo/network mutation occurred. Preserve the clean baseline and leave live System Proxy/TUN/list-mode verdicts open until the user explicitly accepts a future UAC prompt. |

# Phase 65: V1 architecture normalization and usable-proxy closure (2026-08-20)

## Goal

Implement the user-requested V1 architecture boundaries instead of stopping at the document's earlier planning-only scope: global observation but Navo-owned control only, one serialized Connection Coordinator for every control mutation, verified Candidate-to-Active promotion, capture/policy independence, request-only scheduling, bounded two-round domain-specific SelfHeal, safe rollback/same-channel failover, and real System Proxy/TUN usability evidence from the delivered package.

## Phases

- [x] **65.1 Specification-to-code ownership map** - Map every UI/Wails/Agent/Service/Scheduler/SelfHeal mutation, state owner, external-observation path, and package/live baseline to each V1 principle; identify remaining direct or duplicated control paths.
- [x] **65.2 Single control transaction boundary** - Route all node, core, capture, runtime-policy, source, repair, and automatic recovery mutations through one typed Connection Coordinator transaction while keeping observation parallel and preserving independent rollback contexts.
- [x] **65.3 Candidate/Active and source-channel semantics** - Require real data-plane validation before Active promotion, preserve capture and traffic policy during node changes, restore the previous Active node on user-switch failure, and keep airport versus independent-proxy automatic failover isolated.
- [x] **65.4 Capture, policy, and SelfHeal truth** - Enforce OFF/System Proxy/TUN mutual exclusion in the backend, keep Global/Blacklist/Whitelist independent of capture, prove Navo-only resource ownership, cap complete SelfHeal rounds at two, expose attributable evidence/actions/impact, and fail closed when recovery cannot be proven.
- [x] **65.5 Automated/UI/package gates** - Added boundary and regression tests, ran full Go/vet plus frontend/browser/PowerShell gates, updated truthful route-aware verification, and built/re-verified the 1.0.35 portable directory plus ZIP.
- [ ] **65.6 Elevated packaged data-plane acceptance** - With one explicit Windows elevation approval, run isolated System Proxy and supported TUN scenarios across applicable cores: DNS/TCP/HTTPS, Google/GitHub/application edges, exit-IP differentiation, policy/list behavior, stability, re-enable, recovery, exact rollback, and zero Navo-owned residue while preserving v2rayN.

## Constraints

- Treat the attached V1 document as the product/architecture specification; its historical statement that implementation comes later does not override the user's current request to implement it now.
- Preserve the already-established exact list semantics: private networks direct; selected blacklist matches proxy; selected whitelist matches direct; unmatched traffic follows the base runtime mode. The V1 document intentionally left this detail open, so do not regress the current canonical product definition.
- External Clash/v2rayN/core/network resources are observe-only. Never stop, rewrite, repair, or claim an external resource.
- A candidate, config load, process, listener, WinINet flag, adapter, or internal readiness report is not a successful proxy connection; require independent current-user data-plane evidence before commit.
- Preserve all unrelated dirty-worktree changes and continue delivering only a portable directory plus ZIP.

## Phase 65 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| The managed sandbox could not create the initial read process because the ACL helper failed while applying deny-read rules | 1 | Re-ran the bounded read-only specification/skill recovery with explicit approval; no workspace or runtime mutation occurred. |
| The combined skill/spec/plan recovery output exceeded the tool output budget and was truncated | 1 | Read the 455-line planning skill in bounded sections, then restored Phase 64 state and the V1-specific memory/rollout summary separately. |
| Native `apply_patch` could not read planning files because the managed ACL helper failed | 1 | Tried the Codex-provided patch wrapper through an approved shell; it also failed before mutation. |
| The elevated Codex `apply_patch` wrapper returned `Access is denied` | 2 | Switched to one repository-scoped unified diff after resolving an exact file anchor. |
| The first `git apply --recount` fallback diff omitted a legal hunk line range and was rejected as garbage | 1 | Resolved the exact EOF anchor and replayed a standard numbered hunk; the rejected patch changed nothing. |
| `rg.exe` is present but Windows denied process launch during the mutation search | 1 | Switched to bounded `Get-ChildItem` plus `Select-String` over Go roots, preserving the same read-only search scope. |
| The first combined production patch was rejected because capture/tray contexts had drifted from the inspected hunk | 1 | Split production edits into exact minimal hunks; rejected patches made no changes. |
| The Service subscription patch result was truncated, then inspection showed the target async code was unchanged | 1 | Re-inspected exact source before retrying; no duplicate edit was applied. |
| Native `apply_patch` remained blocked by the Windows ACL helper during the subscription repair | 3 | Used a reviewed repository-scoped temporary LF patch transported through `.cache`, applied by `git apply`, then deleted immediately. |
| PowerShell pipeline CRLF and PTY tab expansion caused several reviewed `git apply` attempts to reject exact Go hunks | 3 | Switched patch transport to an explicit LF/no-BOM temporary file; all rejected attempts were atomic and changed nothing. |
| The first temporary subscription patch omitted its final newline and `git apply` rejected it as corrupt | 1 | Added the required final LF; the corrected patch applied and passed `gofmt` plus `git diff --check`. |
| The first focused Go test used `.cache\go\pkg\mod` with `GOPROXY=off`, but current `scripts/test.ps1` uses `.cache\go-path\pkg\mod` | 1 | Reused the actual repository cache and reran offline; Agent and Service both passed. |
| The first core-update Agent patch placed the shutdown recovery call inside `context.WithTimeout` arguments | 1 | `gofmt` caught the syntax error before compile; moved the call directly after the shutdown log and re-ran format/diff checks. |
| The first core-update test compile used nonexistent `capture.StateOff` | 1 | Replaced it with the canonical `capture.StateStopped`; Agent tests passed. |
| The first Candidate policy test compile omitted the standard-library `reflect` import | 1 | Added the import and reran the full Service package successfully. |
| The broad production diff exceeded the output budget during audit | 1 | Re-ran bounded complete reads of the new session file and exact Active/Candidate helpers before making audit fixes. |

## Phase 65 completion evidence

- V1 boundaries are implemented in the product path: one Agent-owned Connection Coordinator transaction for mutations, strict Candidate-to-Active promotion, capture/policy separation, same-channel failover, Navo-only control, and bounded two-round domain-specific SelfHeal.
- Route verification is now mode-aware: direct mode proves Baidu/Xiaomi through the selected capture while Global/list-proxy modes retain ChatGPT/OpenAI application probes. Re-selecting list mode `off` is idempotent and does not restart a healthy core.
- Full gates passed: all Go packages plus `go vet`, frontend 11/11 behavior tests, Vue typecheck, Vite production build, browser interaction smoke, PowerShell parse checks, Wails Windows/amd64 build, manifest verification, and ZIP verification.
- Delivered `release/Navo-1.0.35-portable-amd64` plus ZIP (28 files, 27 manifest entries, 0 issues), ZIP SHA-256 `5F8156F422241110091C5F60E9FDEF2C31E398881EA955B6CD255201D420B1E9`.
- Real current-user System Proxy acceptance passed for bundled sing-box 1.13.14, Mihomo 1.19.29, and Xray 26.3.27. Direct/whitelist used direct exit `115.227.89.28`; Global/blacklist/list-off used proxy exit `63.124.165.24`; Google returned 204, GitHub 200, ChatGPT 403, OpenAI API 401; WinINet and process ownership rolled back exactly after each case.
- Phase 65.6 remains open: the formal 11-case elevated package runner did not start because Windows UAC was canceled. No TUN adapter/route/DNS/WinINet mutation occurred, so elevated TUN data-plane usability is not claimed.

## Phase 65 additional errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| The existing elevated acceptance script assumed a selected Candidate was immediately Active | 1 | Updated it to accept staging at creation and require Active only after real System Proxy/TUN verification. |
| Direct-mode live acceptance exposed Service and Agent probes that always required ChatGPT/OpenAI | 2 | Made both verification layers route-aware and added direct versus Global/bypass regressions. |
| A temporary validation launcher inherited the host's 386 architecture and failed Job Object startup with `ERROR_BAD_LENGTH` | 1 | Rebuilt the test-only launcher explicitly for Windows/amd64; formal delivery binaries were already amd64. |
| Re-selecting list mode `off` restarted a healthy core and a transient asset probe timed out | 1 | Made `off` to `off` a verified no-op and added a bounded retry for ChatGPT application probes. |
| The final formal elevated acceptance launch was canceled at the Windows UAC prompt | 1 | Verified the runner never started and preserved the gate as pending; no network or process mutation occurred. |

# Phase 66: Core updater body-timeout recovery (2026-08-20)

## Goal

Make official core upgrades complete reliably through the active Navo System Proxy or direct fallback without weakening host/asset allowlists, size/hash/version verification, staging, atomic replacement, Coordinator ownership, capture restoration, or rollback.

## Phases

- [x] **66.1 Reproduce and map the timeout** - Traced UI/Wails download context, per-route client timeouts, response-body copy limits, proxy selection, redirect handling, and error attribution; reproduced the total-body deadline defect with controlled slow/stalled responses.
- [x] **66.2 Repair bounded download semantics** - Separated connection/header deadlines from reset-on-progress body limits, kept each route isolated, rejected stalled or oversized bodies, and preserved cancellation plus integrity validation.
- [x] **66.3 Regression and lifecycle gates** - Covered slow-progress success, stalled-body timeout, caller cancellation, route fallback, pre-session failure ordering, and unchanged running core/capture on download failure.
- [x] **66.4 Package and live updater evidence** - Passed focused/full/package gates, built 1.0.36 portable artifacts, and verified an official core asset through the updater's own Go path without disturbing v2rayN or the user's running Navo.

## Constraints

- A failed or incomplete download must never enter the mutable core-update session or stop the active core.
- Preserve fixed GitHub host/redirect and asset allowlists, bounded size, SHA-256/archive/executable/version validation, atomic replacement, capture restoration, and rollback.
- Keep Navo loopback, WinINet loopback, environment proxy, and direct attempts isolated with actionable route-specific errors; never recurse through Navo's own proxy endpoint.

## Phase 66 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| User-visible updater exhausted both `Windows system proxy` and `direct` while reading the response body with `context deadline exceeded` | 1 | Treat as a downloader deadline/progress-control defect until the current implementation and controlled reproduction prove the exact cause; do not mutate the installed core. |
| Native `apply_patch` again failed before mutation because the Windows ACL helper could not read planning files | 1 | Use a reviewed repository-scoped standard-input `git apply` patch and continue; no product or runtime state changed. |
| The first combined production patch had invalid hunk counts and all broad replay variants were rejected during `git apply --check` | 4 | Abandoned the broad patch after verbose diagnosis; applied reviewed zero-context minimal hunks incrementally, with no partial target mutation from rejected attempts. |
| A no-PTY `git apply -` process received immediate EOF before patch input | 1 | Stopped using that transport; the empty invocation changed nothing. |
| The first zero-context context/routing patch overstated one new hunk line count | 1 | Corrected the count from 8 to 7 and applied the exact minimal patch; the corrupt attempt changed nothing. |
| The first Base64 helper used unavailable V8 `TextEncoder`, and the first decoded input retained a PTY Ctrl-Z control byte | 2 | Replaced it with explicit UTF-8 byte encoding and filtered non-Base64 control characters before decode; neither failed transport reached a target mutation. |
| The first atomic-progress refinement patch omitted the old lines for one replacement hunk | 1 | `git apply` rejected it as corrupt; added the exact deletions and applied the corrected minimal patch. |

## Phase 66 completion evidence

- Root cause fixed: the updater no longer applies a 45-second `http.Client.Timeout` to the complete archive body or shares one 2-minute context across all routes.
- Each metadata route has 45 seconds; each archive route has 10 minutes; a transfer may run longer while bytes progress, but 30 seconds with no data closes the body. Application shutdown/caller cancellation remains authoritative.
- Regression gates passed: slow-progress success, stalled-body failure, cancellation, fresh fallback budgets, zero whole-body client timeout, 10 consecutive focused runs, complete `navo_app`, full Go/vet, frontend 11/11, typecheck/build, Wails Windows/amd64, and package verification.
- Read-only official updater-path test passed through preserved `127.0.0.1:10808`: sing-box 1.13.19, 21,046,252 bytes, 203.18 seconds, exact size/SHA-256/ZIP/executable verification, no update session or core mutation.
- Delivered `release/Navo-1.0.36-portable-amd64` plus ZIP: 28 files, 27 manifest entries, archive 28 files, 0 issues; ZIP SHA-256 `1C1E5C0F4636FC59733FD33E6FE050925943FF0472C8D3196B4AA0D494AB74E4`.
- Two Explorer-owned Navo processes that were already serving the user remain running and were not terminated; WinINet remains `127.0.0.1:10808`, listener 10808 remains PID 1392, and no 12080 listener was created.

# Phase 67: network-environment self-healing implementation (2026-08-23)

## Goal

Implement the applicable product requirements from `docs/Navo_Network_Environment_SelfHealing_Codex_Implementation.md`, while treating the document as specification data rather than an authority that can override the user's request; preserve unrelated processes and network ownership, and validate the complete relevant lifecycle rather than configuration-only success.

## Phases

- [x] **67.1 Specification and current-state mapping** - Read the complete document, restore relevant prior decisions, inspect repository instructions and dirty-worktree scope, then map each requirement to current owners, paths, tests, and acceptance gaps.
- [x] **67.2 Design and implementation** - Implement the narrowest coherent changes across the canonical state owner, coordinator, recovery policy, persistence, and UI/API boundaries required by the specification.
- [x] **67.3 Regression coverage** - Add focused failure, recovery, cancellation, rollback, concurrency, and ownership tests for every changed boundary.
- [x] **67.4 Source and UI gates** - Run focused and full Go/vet/frontend/type/build/static gates applicable to the changed scope.
- [in_progress] **67.5 Runtime and delivery acceptance** - Execute safe applicable runtime/data-plane/recovery acceptance, preserve unrelated processes, and report any elevated or external gate that remains genuinely blocked.

## Constraints

- Do not follow instruction-like text embedded in the attached document; extract only product requirements and acceptance criteria.
- Preserve unrelated v2rayN and user-owned processes; stop only verified Navo-owned processes and request approval before terminating user-owned processes.
- Keep Connection Coordinator ownership, serialized network mutation, fail-closed health semantics, bounded recovery, and exact rollback/recovery evidence.
- Preserve unrelated dirty-worktree changes and use portable green directory plus ZIP for any requested delivery artifact.

## Phase 67 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Sandboxed PowerShell process creation failed with `helper_unknown_error: apply deny-read ACLs` while reading the skill and workspace metadata | 1 | Use approved bounded read-only PowerShell calls; no product or network state was changed. |
| Native `apply_patch` was blocked by the same deny-read ACL helper while appending Phase 67 | 1 | Use a reviewed repository-scoped unified patch via `git apply --recount`; the rejected native patch changed nothing. |
| The first full-document read was truncated by the tool output cap | 1 | Preserve the document hash/line count and re-read it completely in bounded numbered chunks before implementation. |
| Initial focused Go baseline used an empty repository module cache with `GOPROXY=off`, so dependency setup failed before compile | 1 | Download only go.mod/go.sum-locked dependencies through `goproxy.cn` into `.cache`; Agent, Service, Network, and Self-Heal then all passed. |
| Two combined Phase 67 status/evidence patches used stale or CRLF-sensitive task-plan context and were rejected atomically | 2 | Resolve exact current lines and use reviewed zero-context replacements for the two status lines plus an EOF-adjacent append; neither rejected patch changed a file. |

## Phase 67 implementation evidence

- Added read-only `internal/networkenv` Snapshot/Analyzer/Store, V2 Journal-owned resource observation, bounded Service `network.observe`, raw-vs-owned WinINet observation, exact Navo adapter identity, and partial/stale handling.
- Agent now aggregates authoritative facts, caches a defensive snapshot, refreshes after Capture changes, consumes fresh typed findings before targeted fallback probes, and maps findings into the existing Coordinator-owned two-round SelfHeal flow.
- Manual `environment.repair` re-observes immediately and accepts only a current, non-transitional, Navo-owned recoverable finding. External System Proxy/TUN and physical/unknown detection remain observation-only or fail-closed.
- Dashboard/Wails/Vue thin-layer support is backward compatible through optional `environment`; Overview owns a dedicated semantic card with disabled/loading states and ownership-aware repair controls.
- Source gates PASS: full `go test ./...`, full `go vet ./...`, frontend 22/22 tests, Vue typecheck, Vite production build, and Wails `navo_app` tests.
- Browser gate PASS with bundled Playwright plus installed Edge: card rendered both Navo-owned and external findings, only the owned finding exposed an enabled repair action, click reached the narrow backend method, 820px had no horizontal overflow, and console errors were empty.
- No live process, listener, WinINet, route, DNS, firewall, or adapter mutation was performed. Elevated manual matrix A-I, real TUN data-plane recovery, exact rollback, and portable packaging remain Phase 67.5 acceptance work.

# Phase 68: evidence-led project optimization (2026-08-24)

## Goal

Identify and implement a bounded, high-value optimization in the current Navo worktree without overwriting unrelated user changes or weakening the Connection Coordinator, ownership, rollback, and real-data-plane acceptance boundaries.

## Phases

- [x] **68.1 Baseline and hotspot audit** - Record dirty-worktree scope, repository instructions, build/test entry points, current architecture seams, and measurable performance/reliability/maintainability candidates.
- [x] **68.2 Select the optimization** - Compare candidates by user-visible impact, regression risk, overlap with existing changes, and strength of verification; choose one concrete scope.
- [x] **68.3 Implement with regression coverage** - Make the smallest coherent production change and add focused coverage.
- [x] **68.4 Verify and report** - Run focused and relevant full gates, report exact evidence, untouched scope, and remaining privileged/live acceptance boundaries.

## Phase 68 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Sandboxed PowerShell process creation failed with `helper_unknown_error: apply deny-read ACLs` during planning-skill and project-state recovery | 1 | Continue with approved bounded read-only PowerShell calls; no product or network state changed. |
| Native `apply_patch` could not read planning files because the same ACL helper failed | 2 | Tried both absolute and repository-relative paths; no file was changed by either rejected patch. |
| The environment `apply_patch.bat` wrapper returned `Access is denied` | 1 | Used this reviewed repository-scoped unified diff fallback after all patch-tool paths were proven unavailable. |
| A combined Phase 68 status/findings/progress patch omitted `--unidiff-zero` and was rejected atomically | 1 | Re-read the exact lines and apply the same reviewed zero-context patch with the required flag. |
| Three initial focused `go test` invocations used an empty repository module cache and the default proxy, producing dependency-download output but no test verdict | 3 | Bootstrap only go.mod/go.sum-locked modules through `goproxy.cn`, then run all tests offline from repository-local caches. |
| The first offline focused test lacked `.cache\\tmp` and failed before package compilation | 1 | Create the repository-local temp directory exactly as `scripts/test.ps1` does; the unchanged focused suite then passed. |
| The first large production patch had a stale lower-function hunk and was rejected atomically | 1 | Split the reviewed change into import/main-loop/helper hunks with exact current lines. |
| The first full `scripts/test.ps1` call crossed the tool yield boundary without returning its session ID, so only partial package output was recoverable | 1 | Confirm no test process remained, then run `go test ./...` and `go vet ./...` separately with preserved session IDs and explicit exit markers. |

# Phase 69: repeated Service / IPC request failures (2026-08-24)

## Goal

Correlate the user's repeated 20:09:19-20:09:57 `Service / IPC request failed` events with the exact UI, Agent, Named Pipe, Service, request-ID and handler lifecycle; implement the narrowest repair if the defect is in the current worktree, without weakening local-only IPC, replay protection, ownership, or bounded timeouts.

## Phases

- [x] **69.1 Evidence correlation** - Capture current binaries/processes/listeners and bounded logs around the reported timestamps; identify exact methods, request IDs, error codes, and first failing boundary.
- [x] **69.2 Reproduce and root-cause** - Reproduce safely without changing networking where possible and distinguish version skew, process readiness, transport, replay, timeout, and handler failures.
- [x] **69.3 Repair with regressions** - Apply a focused lifecycle/correlation fix if source-owned and add cold-start/reconnect/timeout coverage.
- [x] **69.4 Verify** - Run focused/full gates and a safe runtime IPC check; report any launcher/elevation/network boundary not exercised.

## Phase 69 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Native `apply_patch` again failed at the managed Windows ACL helper before updating Phase 69 planning files | 1 | Use the previously proven reviewed repository-scoped unified diff fallback; no source or runtime state changed. |
| Read-only BAM execution-history query found the user SID path but Windows denied value access | 1 | Do not request broader registry authority yet; continue from product logs, source correlation, recent files, and safe runtime evidence. |
| Frontend package metadata read targeted `navo_app/frontend/package.json`, but the manifest is at `navo_app/package.json` | 1 | No file changed; use the established `navo_app` npm working directory for all gates. |
| Managed ACL helper denied ordinary repository reads after prior access succeeded | 1 | Re-ran the same bounded read through approved elevated execution; no product process or runtime state changed. |
| Two interactive `git apply` fallbacks were atomically rejected (incorrect hunk counts, then PTY tab expansion) | 2 | Rebuilt the exact zero-context patch and supplied it through redirected standard input; rejected attempts changed no file. |
| Frontend source reads targeted two stale paths before locating the actual feature/type files | 1 | Located `features/settings/SettingsPage.vue` and root `types.ts`; no file changed. |

---

# Phase 70 - Single Capture Valve and Real Proxy Acceptance (2026-08-24)

## Goal

Implement the user-confirmed architecture: one Navo outbound data path and one serialized, mutually exclusive capture valve (`off`, `system_proxy`, `tun`), while keeping System Proxy and TUN activation, verification, and rollback rules independent. A mode cannot commit healthy when real proxy/IP evidence still shows direct traffic.

## Phases

- [x] **70.1 Ownership and state audit** - Map every mode mutation, coordinator entry, state owner, commit point, and UI success projection.
- [x] **70.2 Independent acceptance audit** - Trace System Proxy and TUN data-plane verification separately, including DNS/TCP/HTTPS/exit-IP evidence and rollback.
- [x] **70.3 Valve repair** - Centralize mutually exclusive transition/commit semantics without widening ownership or changing unrelated processes.
- [in_progress] **70.4 Regression and live validation** - Run focused/full gates, then validate real System Proxy and elevated TUN traffic where current route/elevation permits.

## Constraints

- Preserve user-owned v2rayN and other external processes/resources.
- Observation may remain parallel; all capture mutations must serialize through the Connection Coordinator.
- Core/listener/adapter/persisted mode are readiness evidence only, never proxy-success acceptance.
- System Proxy certification must use the WinINet/system path; TUN certification must use the virtual-adapter path without an explicit proxy.
- Keep the Phase 69 Service error evidence enrichment; defer its frontend presentation until the capture success boundary is corrected.

## Phase 70 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| First combined valve/exit-identity patch was atomically rejected at the later Agent health-probe hunk | 1 | Confirmed no partial target-file change, corrected the exact block boundary, and applied Service, Agent, and WinINet repairs as separately reviewable patches. |
| First combined regression patch was atomically rejected as a corrupt multi-file diff | 1 | Split canonicalization and the three tests into independent patches; rejected attempt changed no file and all intended edits then applied. |
| First focused Go gate built Service/System Proxy but Agent test referenced Service-private runtime constants | 1 | Production code was unaffected; changed the test to assert the public wire values `global` and `bypass_mainland`, then rerun. |
| Frontend integration patch applied but reported one mixed space/tab indentation warning in the existing Vue log row | 1 | Source behavior applied successfully; normalize only the two touched template lines before rerunning `git diff --check`. |
| Attempted full replacement of two encoding-damaged untracked frontend files through a normal modification patch | 1 | Git correctly refused untracked modification; verified both absolute workspace paths, removed only those two newly created corrupt files, and immediately recreated them with ASCII-safe Unicode escapes. |

## Phase 70 verification state

- Source gates: PASS (`go test ./...`, `go vet ./...`, frontend 26/26 tests, typecheck, production build, `git diff --check`).
- Current Navo route: unavailable (`selected_outbound=""`, no active/candidate selection).
- Live System Proxy DNS/TCP/HTTPS/exit-IP acceptance: pending a user-configured and selected Navo route.
- Live TUN no-explicit-proxy/elevated acceptance: pending the same route plus administrator execution; unrelated v2rayN remains untouched.

## Phase 70 packaging and fail-closed evidence (1.0.38)

- Built `release/Navo-1.0.38-portable-amd64` and matching ZIP through `scripts/package.ps1`; package and archive verification both report `issues_found=0`.
- ZIP SHA-256: `E7819C0527B8A0B968DC4DEEC294E068C7264E4A99C3F7537584AF324BE3EEEB`; root `navo.exe` PE file/product version is `1.0.38.0`; `repair.exe check` reports zero issues.
- Isolated packaged lightweight smoke PASS: one stable UI, dashboard IPC, tray exit code 0, and no-route System Proxy returns `OUTBOUND_REQUIRED` in 8155.2 ms while `committed_mode=off` and TUN fault remains empty.
- Post-smoke residue PASS: no Navo process, 12080 listener, or Navo adapter remains. Existing v2rayN PIDs and 10808/10813 listeners remain unchanged; WinINet remains owned by the external proxy.
- Real System Proxy and elevated TUN DNS/TCP/HTTPS/exit-IP acceptance remain pending because Navo `selected_outbound` is empty and no active/candidate Navo route exists. External v2rayN configuration was not read or reused.

## Phase 70 packaging errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Native `apply_patch` again failed at the managed ACL helper while bumping `VERSION` | 1 | Used the reviewed repository-scoped `git apply` fallback; no unrelated file changed. |
| First fallback version patch had a malformed trailing hunk | 1 | Rebuilt the exact one-line zero-context patch and verified `VERSION=1.0.38`. |
| First offline package run lacked Wails CLI build dependencies | 1 | Downloaded the Wails v2.12.0 locked module graph into repository-local caches; source and frontend gates had already passed. |
| Second offline package run lacked three Wails binding test dependencies | 1 | Downloaded the three exact logged module versions, then completed the official package run successfully. |
| Attempting `go mod download all` inside the read-only module cache tried to update Wails `go.sum` and was denied | 1 | Stopped that path and downloaded exact modules from the writable project root; cached module source was not modified. |

# Phase 71: active TUN verification baseline race (2026-08-24)

## Goal

Prevent health-monitor `runtime.verify` calls from observing the temporary gap between clearing the previous TUN result and committing the new activation plan/baseline.

## Phases

- [x] **71.1 Correlate evidence** - Map the two-second request cadence to active capture health verification and identify the unlocked Service read of transitional TUN evidence.
- [x] **71.2 Serialize verification** - Make external `runtime.verify` acquire the Service capture mutex and return `CAPTURE_BUSY` on context expiry instead of reading a partial transaction.
- [x] **71.3 Regression and source gates** - Add a lock-contention regression; pass focused Service/Agent and full package source/frontend gates.
- [in_progress] **71.4 Runtime acceptance** - Package 1.0.39 and rerun elevated real TUN verification when a Navo route and administrator execution are available.

## Evidence

- Regression `TestRuntimeVerifyWaitsForCaptureTransition` PASS; complete Service and Agent suites PASS.
- Official 1.0.39 package PASS: full `go test ./...`, `go vet ./...`, frontend 26/26, typecheck, production build, package/archive verifier, and `repair.exe check`.
- ZIP SHA-256: `753729CB931068D0BB01C9F71F5C868F14F104EEF58AB2793F6B2C31578A0689`; launcher PE version `1.0.39.0`.
- Lightweight smoke reached IPC/UI and no-route `OUTBOUND_REQUIRED` fail-closed behavior, but two tray-exit automation attempts timed out. Do not count packaged lifecycle smoke as PASS.
- Both timed-out smokes left zero Navo processes/adapters; external v2rayN and WinINet ownership remained unchanged. Smoke log residue was removed and package verification reran with zero issues.

## Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Native `apply_patch` failed at the managed ACL helper for source and version edits | 2 | Used reviewed repository-scoped `git apply` patches; verified exact target changes before formatting. |
| First combined fallback patch had an output-handler typo and applied no target change | 1 | Verified both expected matches were absent, then split runtime and test patches into atomic changes. |
| First focused test lacked `.cache/tmp`; the offline retry then lacked four locked modules | 2 | Recreated the repository temp directory, restored go.sum-locked dependencies through `goproxy.cn`, and reran offline with explicit exit code 0. |
| First complete Service/Agent invocation did not preserve the yielded session result | 1 | Re-ran with a captured session; Service 33.474s and Agent 3.400s both PASS. |
| Single-UI and tray-recreate lightweight smokes both timed out in `exit_via_tray.ps1` | 2 | Stopped repeated UI automation, verified zero owned residue and external proxy preservation, cleaned only smoke logs, and retained this as a separate pending lifecycle defect. |

# Phase 72: intermittent overview data loss during TUN activation (2026-08-25)

## Goal

Reproduce and repair the intermittent overview-panel data loss when TUN is enabled by tracing the complete Vue/Wails/Agent/Service dashboard snapshot lifecycle, preserving valid last-known data during transient transition states without masking real failures or weakening capture serialization.

## Phases

- [x] **72.1 Reproduce and map ownership** - Correlate TUN transition phases with dashboard requests, response fan-out, request identity, frontend loading/error state, and component lifecycle.
- [x] **72.2 Root-cause and regressions** - Identify the first destructive/failed boundary and add focused failing coverage for the intermittent sequence.
- [x] **72.3 Focused repair** - Implement the smallest ownership-preserving backend/frontend fix while keeping fresh errors and partial-data semantics visible.
- [x] **72.4 Verify** - Run focused and full Go/frontend gates plus a safe browser/runtime transition check; report any elevated live-TUN boundary separately.

## Constraints

- Preserve unrelated user changes and user-owned v2rayN processes.
- Do not treat a listener, adapter, or green UI state as real TUN acceptance.
- Keep `captureMu` serialization and UI-parent versus Agent-child request-ID separation intact.
- Optional IP/metrics failures must not erase usable core/capture/overview state.

## Phase 72 errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Managed sandbox helper failed while reading the planning skill and repository context (`apply deny-read ACLs`) | 1 | Continued with approved bounded read-only PowerShell calls; no product or network state changed. |
| Native `apply_patch` could not read the planning files because the same ACL helper failed | 1 | Tried the required patch tool once; no file changed. |
| The elevated `apply_patch` wrapper returned `Access is denied` | 2 | Switched to the previously proven repository-scoped atomic Git patch fallback. |
| First interactive Git patch fallback had an incorrect hunk count; the patch was rejected atomically | 1 | Generate hunk counts from the current files before feeding the reviewed patch. |

| First combined production Git patch had a malformed hunk count | 1 | Git rejected the patch atomically; corrected counts and added a check preflight. |
| Corrected Git patch could not modify the current untracked feature modules | 1 | Used workspace-bounded exact replacements with unique-match, cumulative preflight, and reread-before-write guards. |
| First cumulative exact-edit preflight evaluated repeated same-file anchors independently | 1 | It stopped before writing; the corrected per-file cumulative state applied all replacements together. |
| First production build reported useTraffic.ts(119,28) could not find name apis | 1 | Restored only the API facade import still required by controlled traffic; typecheck/build then passed. |
| System and bundled Python lacked the Playwright module | 2 | Used the bundled Node Playwright package with the same headless Edge timing scenario; no dependency was installed. |
| The screenshot viewer was blocked by the managed ACL helper | 2 | Retained browser DOM/console assertions plus the generated screenshot and SHA-256; did not treat tool display as test evidence. |
| The server helper stopped npm but left the verified workspace Vite child on port 4188 | 1 | Correlated PID 6928 by listener, executable, command line, workspace path, port, and start time; stopped only that process and confirmed zero listeners. |

# Phase 73: hierarchical log persistence and user filtering (2026-08-31)

## Goal

Make Navo logs explicitly classified when persisted, let users select and process logs by level in the existing log UI, and make the log view open on the Basic Service category by default while preserving redaction, backward-compatible DTOs, bounded queries, and existing Service/Agent ownership boundaries.

## Phases

- [x] **73.1 Map the current contract** - Trace logger producers, persisted schema/files, query filters, Wails DTOs, category/level labels, UI defaults, and existing regression coverage.
- [x] **73.2 Define red regressions** - Lock the intended persistence classification, user level filtering/handling semantics, legacy-entry compatibility, and Basic Service default selection.
- [x] **73.3 Implement the focused change** - Extend the smallest canonical backend/frontend seams without exposing sensitive fields or duplicating log state.
- [x] **73.4 Verify** - Run focused Go/frontend tests, full relevant gates, typecheck/build, and a browser-visible default/filter smoke; report packaging separately unless product code requires a release build.
- [x] **73.5 Package** - Bump the continuing release to 1.0.40, build the official portable directory and ZIP, and verify package/archive manifests, versions, repair check, and SHA-256 without starting Navo or mutating networking.

## Constraints

- Preserve existing log redaction and never persist credentials, tokens, subscription bodies, or arbitrary unbounded structured fields.
- Keep Service, Agent, and UI DTOs backward compatible; legacy log records without new classification fields must remain queryable.
- User filtering must not rewrite, delete, or silently discard persisted logs unless the existing product contract already defines such an operation.
- Opening the log view must deterministically select Basic Service without preventing the user from choosing other categories or levels.

## Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Initial combined sandboxed read of skill, repository context, and memory failed at the managed ACL helper (`apply deny-read ACLs`) | 1 | Switched to bounded approved read-only calls; no workspace or runtime state changed. |
| Initial sandboxed repository/planning inventory failed at the same ACL helper | 1 | Used a bounded approved read-only inventory, confirmed a clean worktree, and preserved all historical planning content. |
| Native `apply_patch` could not read the three planning files because the managed ACL helper failed | 1 | Invoked the patch engine through its approved Windows binary with a repository-scoped patch. |
| Two wrapper invocations lost the final patch marker before parsing and changed no files | 2 | Bypassed the batch wrapper and passed the exact multiline argument directly to the patch engine. |
| The first direct multi-file patch applied the first two files before a stale `progress.md` anchor failed | 1 | Verified the exact partial state, preserved the correct planning/findings additions, and applied only the missing progress entry with its real EOF anchor. |
| Read-only frontend manifest lookup used the obsolete `navo_app/frontend/package.json` path | 1 | Located the current manifest at `navo_app/package.json`; no file was changed. |
| Read-only style lookup used the obsolete single `frontend/src/styles.css` path | 1 | Located the split stylesheet set under `frontend/src/styles/` and bounded subsequent reads to actual files. |
| First parallel focused Go run yielded while downloading a missing locked module, so its final status was not captured | 1 | Re-ran with an explicit session poll; the preserved result exposed a network download failure rather than a source result. |
| Focused Go red run could not fetch `golang.org/x/sys@v0.30.0` from `proxy.golang.org` over IPv6 | 1 | Treat this as an environment/cache gate, continue implementation from the already failing compile contract, then use the repository-local locked module cache or approved mirror for verification. |
| First browser smoke used the visible navigation label as an exact accessible name | 1 | Read the rendered button contract and changed the selector to its explicit `设置与日志：<description>` aria-label prefix. |
| The server helper stopped npm but left its verified Vite child listening on 4193 | 1 | Correlated PID 28280 by executable, exact workspace Vite command, port, and start time; stopped only that process and confirmed zero listeners. The retry starts Vite directly so the helper owns the actual server process. |
| The helper also left its directly launched Vite process after the successful smoke | 1 | Correlated PID 27652 by exact Vite command, port 4193, and smoke start time; stopped only that process and confirmed zero listeners. Future reruns use an interactive server session with explicit Ctrl+C teardown. |
| The managed image viewer could not read screenshots from either the workspace or the task visualization directory because the ACL helper failed | 2 | Retained browser DOM, computed overflow, theme, console/page-error assertions plus screenshot hashes; copied the images to the user-visible visualization directory and did not claim manual pixel inspection. |
| A combined smoke/planning patch omitted the `task_plan.md` file header before an error-table hunk and failed before changing files | 1 | Verified the exact no-change state, added the missing file header, and reapplied the reviewed patch. |

## Phase 73 completion evidence

- Persistence/query: every new structured JSONL entry stores `level`, `category`, `service`, and `component`; legacy entries derive category from their existing service without rewriting the file.
- User behavior: separate severity, service-category, and concrete-service filters; initial category is `basic_service`; clearing all categories means no category restriction.
- Source gates: focused logstore/Service/Agent PASS; full `go test ./...`, `go vet ./...`, frontend 32/32, Vue typecheck, Vite build, and `git diff --check` PASS.
- Browser gate: Basic Service default/query, Network & Capture + TUN switch, severity exclusion, day/night, 760x900, zero horizontal overflow, zero console/page errors PASS.
- Package gate: official 1.0.40 portable directory and ZIP, directory/archive `issues_found=0`, repair `issues_found=0`, all Navo-owned PE versions 1.0.40.0, ZIP SHA-256 `66D57EC1334E375CBD5273A7C6CC9E87112CDE4FB246672B64E37D7F0FBA6517`.
- No Navo process was launched by packaging; final Navo process count and Vite 4193 listener count are zero. Packaged/installed interactive runtime smoke was not needed for this log-only behavior and was not represented as executed.

# Phase 74: unify V2 active proxy presentation, traffic attribution, and exit evidence (2026-08-31)

## Goal

When V2 has one effective Navo proxy connection, present it as one canonical active-connection surface. Make the real-time traffic chart and exit-IP evidence derive from the same committed capture/outbound snapshot, and expose an explicit pending/failure reason instead of labeling traffic as an unrelated local proxy or silently omitting exit detection.

## Phases

- [x] **74.1 Reproduce and map state ownership** - Identify the two visible boxes, their DTO/computed sources, the traffic source label, exit-IP projection, refresh cadence, and the exact V2 active/candidate/capture state returned by Dashboard.
- [x] **74.2 Lock regressions** - Add focused tests for one canonical connection card, committed-state traffic attribution, exit evidence, and pending/error copy without weakening candidate/active semantics.
- [x] **74.3 Implement unified projection** - Change the smallest backend/frontend seams so one active connection snapshot drives mode, route, chart source, and exit evidence.
- [x] **74.4 Verify** - Run focused/full frontend and affected Go gates, typecheck/build, browser-visible mocked V2 smoke, and live read-only/runtime evidence where safe; keep real external traffic acceptance separate if credentials/elevation are unavailable.
- [x] **74.5 Package** - Bump the continuing portable release to 1.0.41, run the official package gates, and verify the directory, ZIP, repair check, embedded versions, and SHA-256 without starting Navo or mutating the external proxy.

## Constraints

- Candidate selection must not be presented as active; only a committed `active_id` after the existing real-data-plane gate may represent the effective route.
- `off`, `system_proxy`, and `tun` remain one mutually exclusive capture valve; do not merge their independent activation/rollback verification.
- Preserve DTO compatibility, the shared single-flight Dashboard loader, traffic history stability, and Service child/UI parent request-ID separation.
- Do not read or reuse v2rayN credentials/configuration, stop unrelated processes, or infer a successful exit from a listener/UI state alone.

## Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Initial sandboxed memory/skill/repository reads failed at the managed ACL helper (`apply deny-read ACLs`) | 1 | Continued with bounded approved read-only calls; no workspace, process, or network state changed. |
| Native `apply_patch` could not read the three planning files because the managed ACL helper failed | 1 | Invoke the same standard patch engine through its bounded approved Windows executable; the rejected attempt changed no file. |
| A combined backend search included an incomplete regular expression for a Dashboard function | 1 | The read-only search failed before returning matches; reran the same bounded inventory with literal patterns and changed no file. |
| The first multi-file effective-connection patch reached a stale shortened `state/runtime.ts` anchor after applying its earlier files | 1 | Inspected the exact partial state, retained the correct new module/type changes, and applied only the missing compatible Dashboard defaults with the current anchor. |
| The first focused Agent run broke the old cached Dashboard exit-IP regression | 1 | Preserved cached Navo exit fields for direct/Navo compatibility and isolated them only when the effective source is external. |
| The next Agent run observed the host's live external WinINet proxy inside an otherwise isolated unit test | 1 | Made effective external-proxy selection consume the published immutable Environment snapshot, restoring deterministic tests and the intended unified snapshot boundary. |
| The first frontend green run failed because a deliberately partial projection fixture omitted the proxy DTO | 1 | Made the pure projection tolerate the missing optional nested proxy object; Vue typecheck already passed and the focused suite then passed 15/15. |
| The first affected Service package gate failed its 10-second readiness bound and one concurrent Xray version probe | 1 | Read both exact paths, reran each test alone (PASS), then reran the complete Service package serially (PASS in 41.221s); retained the 9.16-second readiness margin as a test-timing risk. |
| A read-only detector lookup guessed the nonexistent `internal/ipdetect/detector.go` path | 1 | Listed the package and inspected the actual `echo.go`; confirmed the direct detector explicitly sets `Transport.Proxy=nil`. |
| Python Playwright was unavailable and the first Node fallback used the wrong bundled `NODE_PATH` | 2 | Confirmed the Python module error, located the existing Codex Playwright package, and reran the same headless Edge smoke successfully without installing dependencies. |
| The managed image viewer could not read the new screenshots because the ACL helper failed | 1 | Retained Edge DOM/theme/layout/console assertions and SHA-256 evidence; do not claim manual pixel inspection. |

## Phase 74 completion evidence

- Effective connection: external WinINet proxy, committed Navo capture, and direct mode now resolve through one projection; selected/candidate-only Navo state cannot be rendered active.
- Overview: exactly one primary connection evidence card; direct baseline is subordinate evidence and the detailed Environment aggregate is available on Network Detection.
- Traffic: external proxy state uses real physical-interface counters labeled system total/external read-only; no external proxy-only metrics or connection counts are fabricated.
- Exit: Agent probes the observed external endpoint and an independent no-proxy transport, caches by effective connection, and exposes provider/error/time evidence through backward-compatible DTO additions.
- Source gates: full Go test/vet PASS; frontend 36/36, Vue typecheck and production build PASS; browser smoke PASS with zero console/page errors and zero 760px overflow.
- Live read-only gate: current v2rayN direct/proxy exits differ, Google returns 204 and GitHub 200 through the explicit endpoint; no Navo or external network state was changed.
- Package gate: official 1.0.41 portable directory and ZIP each report `issues_found=0`; repair check reports zero issues; all Navo-owned PE versions are 1.0.41.0; ZIP SHA-256 is `2F0FDB3E58E21C64248B7BFF4A08B695D3216DC18DC636C145C4E1E630123511`.

# Phase 75: route-required recovery and overview layout refinement (2026-08-31)

## Goal

Stop the UI from issuing a doomed Navo capture request when only an external proxy is active and no Navo outbound is selected, present `OUTBOUND_REQUIRED` with a direct recovery action, and refine the desktop/compact layout into a clearer, more polished hierarchy without weakening backend fail-closed route admission.

## Phases

- [x] **75.1 Reproduce and map** - Trace request `ui-1788144216306-38` from UI action through capture admission and log presentation; inventory shell, overview, grids, responsive breakpoints, and the highest-impact layout defects.
- [x] **75.2 Lock regressions** - Add focused tests for state-aware primary CTA, actionable `OUTBOUND_REQUIRED` feedback, and structural/responsive layout contracts before production edits.
- [x] **75.3 Implement** - Preserve the backend route gate while preventing the invalid UI action, add an explicit route-selection recovery path, and apply a restrained desktop-control-console visual/layout refinement using existing semantic tokens.
- [x] **75.4 Verify** - Run focused/full frontend and affected Go gates, typecheck/build, day/night desktop plus 1024/760/375/landscape browser checks, keyboard/focus/reduced-motion assertions, and zero-overflow/console-error checks.
- [x] **75.5 Package** - If source and browser gates pass, bump to 1.0.42 and produce the verified portable directory/ZIP without starting Navo or changing the external proxy.

## Constraints

- `OUTBOUND_REQUIRED` remains a fail-closed backend guarantee for Navo System Proxy/TUN; an observed v2rayN endpoint must never be reused as a Navo route.
- External proxy observation remains read-only. Do not read its credentials/configuration, change WinINet, or stop v2rayN/sing-box.
- A selected/candidate route is not active; a missing Navo route should guide the user to Connection Management rather than attempt capture.
- Preserve Phase 73/74 dirty-worktree changes, DTO compatibility, Dashboard single-flight behavior, request-ID separation, and all existing network rollback boundaries.
- Improve hierarchy, spacing, density, touch targets, focus visibility, and responsive reflow through shared tokens/components; avoid decorative restyling that obscures diagnostic truth.

## Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| The initial combined skill/context read exceeded the tool output limit | 1 | Paginated both required skill files to EOF, then restored the existing planning files and current diff before adding Phase 75. |
| The first full frontend run exceeded the existing 400-line application-controller boundary by one trailing line | 1 | Kept route navigation as a composition-time return callback instead of adding another controller method; build was already green and the boundary is rerun below. |
| First Phase 75 Edge run found 82 px horizontal overflow on Settings at 375x812 while Overview passed | 1 | Server/browser lifecycle closed cleanly; instrumented the reusable layout assertion to name actual offending elements before changing CSS. |
| The post-layout Edge run reached the final reduced-motion check but its regex rejected Chromium's valid scientific notation `1e-05s` | 1 | Parse CSS time units numerically and assert at most 1 ms; product motion was already correctly reduced to 0.01 ms. |
| Manual screenshot QA showed files labeled `night` had followed Edge's light system preference | 1 | Explicitly click and assert both `night` and `day` theme state before their matrices; do not infer theme coverage from filenames. |
| First 1.0.42 package run could not populate its isolated Go module cache because direct `proxy.golang.org` IPv6 connections timed out | 1 | The source gate failed before release output mutation. Reverified live WinINet `127.0.0.1:10808`, sing-box ownership, and HTTP 200 to the exact module through the proxy; rerun with process-local HTTP(S) proxy variables only. |
| Second package run passed full Go tests but `npm ci` could not unlink the loaded Rolldown native binding | 1 | Found seven orphaned Vite children from this task's managed browser runs by exact `D:\WorkSpace\Navo\navo_app` path and port 4195. Stopped only those verified PIDs; no TimeBack or external proxy process was touched, and listener/wrapper counts are now zero. |

## Phase 75 completion evidence

- Admission/recovery: missing-route Overview renders `选择 Navo 线路`; login startup and capture preflight the shared predicate before IPC; Agent `OUTBOUND_REQUIRED` remains unchanged as the fail-closed boundary.
- Frontend gates: focused 6/6 and full 42/42 PASS; Vue typecheck, Vite production build (70 modules), and `git diff --check` PASS.
- Browser gate: explicit day/night, 1440x900, 1024x768, 760x900, 375x812, and 667x375; eight page-layout checks with maximum overflow 0; capture/startup bridge calls 0; visible 2 px focus; reduced motion 0.01 ms; zero console/page errors.
- Package gate: official 1.0.42 directory and 28-file ZIP each report `issues_found=0`; independent `repair.exe check` reports zero issues; all three Navo PE files report 1.0.42.0.
- ZIP: 64,684,118 bytes; SHA-256 `4981F15A7287F4632F720B2A0D3079AECA659FA8B9BB845E6B3AB18DCE76286B`.
- Final hygiene: Navo process count 0, Navo Vite process count 0, port 4195 listener count 0; WinINet remains enabled at `127.0.0.1:10808`, owned by the preserved v2rayN sing-box PID 18036.

# Phase 76: intent-scoped priority scheduling and warm activation leases (2026-08-31)

## Goal

Make user-selected actions take the highest safe priority within the affected control domain instead of forcing every Agent operation through one global ranking, and reduce repeated System Proxy/TUN activation latency by reusing exact, recently verified preparation/lifecycle evidence within bounded mode-specific leases without skipping ownership, health, data-plane, rollback, or request-identity guarantees.

## Phases

- [x] **76.1 Map the current control path and latency budget** - Trace UI intent through Agent dispatch, Coordinator/Service serialization, compile/CoreHost/capture verification, retry/recovery/background work, and identify which steps/processes are already long-lived versus rebuilt on every activation.
- [x] **76.2 Define policy and red regressions** - Specify domain lanes, intent classes, preemption/coalescing rules, commit barriers, mode-specific lease keys/expiry/invalidation, and tests that prove interactive actions win only where safe.
- [x] **76.3 Implement the minimal scheduler/lease seams** - Thread typed intent metadata through existing request boundaries, prioritize/coalesce queued work per domain, and reuse only an exact healthy warm lease while retaining fail-closed fallback to the full path.
- [x] **76.4 Verify latency and correctness** - Run focused/full Go and frontend gates, deterministic scheduler/expiry/invalidation tests, measured cold/warm System Proxy and TUN paths where safely reproducible, cancellation/rollback/recovery races, and process/network residue checks.
- [x] **76.5 Package** - After source and runtime gates pass, increment the portable release, produce directory/ZIP, and verify manifests, repair, versions, SHA-256, and preserved external processes.

## Constraints

- Priority is a policy matrix `(intent × domain × phase)`, not one global numeric queue. Independent observations may remain parallel; every network mutation stays serialized under the existing control owner.
- A newer high-priority request may supersede queued/pre-commit work, but must not interrupt an irreversible commit/rollback section. Shutdown, rollback, and ownership-loss safety always outrank convenience.
- Preserve UI parent versus Agent/Service child request identity, at-most-once replay behavior, Candidate/Active promotion, Dashboard single-flight loading, and the mutually exclusive OFF/System Proxy/TUN capture valve.
- A warm lease is valid only for the exact core, outbound/revision, runtime/routing policy, capture mode, compiled config hash, ownership epoch, and verified environment fingerprint. Expiry, drift, process loss, health failure, recovery, or user configuration changes invalidate it immediately.
- Never use a lease to present stale readiness or skip the final safe acceptance required for a changed network state. TUN may cache preparation artifacts even when its capture process cannot safely stay alive after rollback.
- Preserve external v2rayN/sing-box and WinINet ownership; no external credentials/configuration are read or reused.

## Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Managed ACL helper rejected the first sandboxed planning/repository reads and native patch call | 2 | Continued with bounded approved reads and the same standard patch engine through its Windows executable; no product or network state changed. |
| First focused Go run could not populate the isolated module cache because direct downloads timed out | 1 | Reused the already observed external proxy only as process-local HTTP(S) transport for the test command; WinINet and v2rayN ownership stayed unchanged. |
| Initial Service lease integration exposed a strong runtime-evidence type mismatch and then incomplete retain/resume return signatures | 2 | Tightened the lease to the exact `RuntimeRoutingVerification` type, corrected every return path, formatted, and reran the full Service package twice successfully. |

## Phase 76 completion evidence

- Policy: Agent/Coordinator now derives an explicit intent and domain for every transaction. Safety, interactive, recovery, startup, and background work are ranked only inside the relevant capture/routing/core/source domain; equal-rank user work remains FIFO, queued lower-priority same-domain work can be superseded with `REQUEST_SUPERSEDED`, and an active commit/rollback is never preempted.
- System Proxy lease: explicit user OFF can retain the exact healthy owned loopback core for 90 seconds by default. Reuse requires the same core/outbound/revision/config and host hashes, config path, PID, active runtime, no TUN, selected/active agreement, and a fresh CoreHost health check; expiry, drift, crash, recovery, mutation, or shutdown invalidates it and falls back to the cold path.
- TUN latency: no TUN process, adapter, routes, or DNS state is retained while OFF. Fresh direct-IP baseline and fresh activation-plan construction now run concurrently, followed by the unchanged pinned compile, core start, physical activation, data-plane verification, and healthy commit gates.
- Source gates: final full `go test ./...`, `go vet ./...`, frontend 42/42, Vue typecheck, Vite production build (70 modules), and `git diff --check` PASS. Coordinator/Supervisor repeated concurrency tests PASS 20/20; warm lifecycle/TUN concurrency tests PASS 20/20.
- Package gate: official 1.0.43 package flow repeated Go/frontend/audit/build verification and produced a 28-file portable directory and ZIP. Official plus independent verifier report `issues_found=0`; packaged repair reports `issues_found=0`, `fixable=false`.
- Artifacts: ZIP is 66,300,689 bytes with SHA-256 `9F9FA7B1CE73AD2CEB27440AAE613A21D8F3B663310595A76EC15971F4AC4F20`. `navo.exe`, `repair.exe`, and `app_ui/navo_app.exe` all report 1.0.43.0 with SHA-256 values `F88C12D59A34F537BABF35E1EBBFA99D3A13CD31E78ADA0DF34C0F1D20711FF7`, `4D49313AD716F2109E4B507CE8E3924C8F7B1A04A6F464FFCB5E1BD29435FF65`, and `77470BF8887622F81A963015015F0E1BB040A282C20CBEE24BD47FDE35B50A4E`, respectively.
- Final hygiene: Navo process count 0, Navo Vite process count 0, ports 4195/12080 listener count 0. WinINet remains enabled at `127.0.0.1:10808`, owned by the preserved v2rayN sing-box PID 8936; no external process or network setting was changed.
- Residual acceptance: deterministic tests prove same-PID zero-stop/start warm reuse and exact expiry/drift fallback, but no live Navo route was configured and no physical System Proxy/TUN state was mutated. Therefore real-host warm/cold wall-clock reduction and elevated TUN DNS/TCP/HTTPS/exit-IP acceptance remain unclaimed.
