# Navo Four-Guide Remediation Report

Date started: 2026-08-01

## Conflict resolution and execution order

1. `Navo_Codex_Precise_Cleanup_Integration_Privacy_Init_Guide.md` controls AI/MySQL deletion, local persistence, initialization, device binding, privacy reset, and dead-code evidence.
2. `Navo_Codex_Full_Remediation_Guide.md` controls P0/P1 runtime, network, recovery, lifecycle, security, IPC, persistence, and release remediation. Its AI hardening is replaced by complete AI removal; database consistency becomes local revision/selection/committed-runtime consistency.
3. `Navo_Codex_Feature_Optimization_Guide.md` controls product features and prevents incomplete-but-required feature code from being deleted.
4. `Navo_Claude_Codex_Self_Healing_Engine_Guide.md` is implemented only after the underlying coordinator, structured logs, monitoring, initialization, and persistence boundaries are trustworthy. It may observe and delegate; it cannot become an alternate mutation owner.

The obsolete `Navo_Codex_Local_First_Modularization_Privacy_Init_Guide.md` is not an implementation authority.

## Baseline

See `docs/REMEDIATION_BASELINE.md`.

- Go tests and vet: passed.
- Frontend typecheck/build: passed.
- Frontend test gate: missing in baseline.

## Reachability and removal evidence

See `docs/CODE_REACHABILITY_AND_REMOVAL_PLAN.md`.

## Phase status

| Phase | Status | Evidence |
|---|---|---|
| Requirement/code baseline | Complete | Four Guide priority matrix, tracked/untracked audit, baseline tests, dynamic reachability inventory. |
| AI removal | Complete | AI package/settings/Service/Agent/IPC/smoke slice removed; legacy AI state is precisely removed during initialization; full Go test/vet passed. |
| MySQL removal/local repositories | Complete | MySQL bootstrap, repositories, config and driver removed; versioned atomic local revision/selection repositories are wired into the launcher and tested. |
| Initialization/privacy | Complete | Current-User DPAPI device state runs before configuration/logging; foreign-context cleanup, tamper blocking, cleanup-failure blocking and precise legacy environment cleanup are covered by tests. |
| Full remediation P0/P1 | In progress | Lifecycle, network, WinINet, persistence, IPC framing, recovery, subscription/parser, config validation, uptime/HTTPS Geo and release gates are implemented and passing focused tests; standalone external-pipe identity and elevated Windows matrix remain. |
| Feature optimization | In progress | Four-source traffic, selection, simulation, accessible tooltip, structured logs, layered active-route latency and explicit core-update trust blocking are implemented; signed/pinned core installation and multi-provider risk aggregation remain. |
| Self-healing | In progress | Bounded engine, exact registry, budgets/backoff/circuit, hashed atomic state, structured events, Supervisor ownership integration and focused fault tests are implemented; additional production detector/policy coverage remains. |
| Final release/Windows acceptance | Partial | Full Go/frontend/Wails/package/hash/static/UI gates pass. Current elevated three-core/TUN/rollback/manual Windows matrix remains pending. |

## Baseline defect resolution

`navo_app/package.json` now has a deterministic frontend test gate. Packaging runs Go tests/vet and frontend tests/typecheck/build before replacing the requested release output.

## Phase 29 verification

- `scripts/test.ps1`: passed after AI/MySQL removal and initialization changes.
- Production source scan: no AI or MySQL runtime implementation/config/dispatch references remain. Remaining matches are deliberate legacy-cleanup patterns and their tests.
- `.env` and `.env.example`: zero configuration assignments; retired credentials and endpoints are not retained.
- DPAPI policy remains Current User in production. Tests use injected protectors because the managed test token cannot initialize a Windows user profile.

## Final verification

- `scripts/package.ps1 -OutputName Navo-four-guide`: passed; generated `release/Navo-four-guide`.
- Full Go `test ./...` and `vet ./...`: passed as package preconditions.
- Frontend Node tests: 2 passed; Vue typecheck and Vite production build passed.
- Wails Windows/amd64 production build: passed; generated bindings include the new latency, traffic and log contracts.
- Public timestamp fields use RFC3339Nano strings, so regenerated Wails TypeScript bindings remain typed and emit no `time.Time` lookup warnings.
- Independent `SHA256SUMS.txt` recomputation: every packaged file matched.
- Packaged `repair.exe check`: zero issues. Offline repair no longer mutates or advertises unavailable adapter/route/DNS actions; runtime Reconciler remains the sole recovery owner.
- Browser-visible mocked-Wails smoke: one-click layered latency, four-source traffic/tooltip, trusted-update blocking and SelfHeal log rendering passed with zero console/page errors.
- Production residual scan: no AI package, MySQL store/config, active retired credential assignment, or plaintext Geo endpoint remains. Retired key names survive only in precise legacy cleanup and its tests; rejected raw lifecycle names survive only in negative tests.

## Explicitly incomplete acceptance

- Automatic core installation is fail-closed until Navo ships trusted future asset URLs/names and SHA-256 values; release-page inspection is not labeled installation.
- IP risk is not yet multi-provider evidence aggregation.
- The new SelfHeal engine has bounded infrastructure and ownership-safe observer policies, but does not yet cover the guide's full production detector/policy catalog.
- Standalone external Service pipe still needs a stronger client SID/session-token contract. The combined production desktop path keeps that pipe disabled.
- `go test -race` is not available on this host because the required C compiler is absent.
- Current elevated Windows three-core system-proxy/TUN data-plane, rollback/recovery, DPI/theme/taskbar/start-menu and install/upgrade/uninstall matrix was not executed in this turn.
