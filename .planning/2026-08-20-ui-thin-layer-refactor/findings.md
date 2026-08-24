# Findings

## Specification

- The task is an implementation request, while the document is the governing technical specification.
- The architectural invariant is: UI expresses user intent and presents backend truth; backend owns business rules, state transitions, Windows networking, rollback, and recovery.
- Required order: baseline, `App.vue` audit, API adapters, incremental feature extraction, runtime/UI state split, shell reduction, bounded Wails facade review, CSS modularization, full regression.
- Required coverage includes UI-state tests, backend-snapshot presenter/mapping tests, and API-adapter delegation tests.
- Final acceptance requires unchanged network semantics, centralized Wails access, feature boundaries, authoritative backend runtime state, and all Go/frontend gates passing.

## Repository Context

- A large pre-existing root planning set tracks many other Navo phases. This task uses an isolated `.planning/2026-08-20-ui-thin-layer-refactor/` plan to avoid overwriting it.
- The first combined file read was truncated; repository constraints and the full specification still require bounded verification before code changes.

## Specification Details

- The frontend may validate input shape for user experience, but compatibility, transaction eligibility, health, routing legality, and recovery decisions remain backend-authoritative.
- Runtime/core/node/capture/connection/self-heal values must be derived from backend snapshots or events; pending UI state must never overwrite committed backend truth.
- Components and composables may translate intent, hold presentation state, call adapters, and map snapshots to view models, but may not orchestrate core, route, DNS, Wintun, rollback, or repair steps.
- API access must follow component -> feature/composable -> adapter -> Wails binding without changing backend API behavior.
- Feature extraction order is Capture, Core, Node, Routing, Subscription, Traffic, Diagnostics, Runtime Overview, with tests after each increment.
- Explicit non-goals include redesigning TUN, Coordinator, SelfHeal, Supervisor, protocols, networking semantics, or the visual design.
- Final output must enumerate modified/added/deleted files, before/after responsibilities, moved/retained logic, API and backend-behavior changes, exact tests/builds, debt, and next steps.

## Relevant History

- Prior UI work preserved seven page IDs/actions and verified day/night plus compact layouts. This refactor must retain those behaviors while moving structure out of `App.vue`.
- Existing shared hierarchy and responsive accessibility are concentrated in `style.css`; modularization must preserve task-oriented navigation, readable Chinese typography, technical monospace usage, and icon-only navigation labels.
- Prior dashboard work includes `TrafficChart.vue` and dual-link/runtime behavior that must remain functionally unchanged.

## Initial Inventory

- No repository `AGENTS.md` files are present.
- Tracked worktree is clean. The user-provided specification is untracked; the only other untracked paths are this task's new `.planning/` files.
- `App.vue` is 1,723 lines, `state.ts` 239, `style.css` 1,047, `types.ts` 400.
- The only direct Wails runtime import found outside the existing adapter is `EventsOn` in `App.vue`; backend method bindings are already partially centralized in `src/api.ts`.
- `navo_app/app.go` is 581 lines; other behavior-heavy Wails package files are already split into updater and diagnostics units, so facade review must be bounded.
- Frontend package root is `navo_app`, not `navo_app/frontend`. Required scripts are `test`, `typecheck`, and `build`; `build` includes typecheck.
- Repository `scripts/test.ps1` provides project-local Go module/build/temp caches and runs `go test ./...` followed by `go vet ./...`.

## Pre-refactor Baseline

- `npm ci`: PASS, 45 packages installed, 46 audited, 0 vulnerabilities; npm emitted only an available-major-version notice.
- `npm test`: PASS, 11/11 tests, 0 skipped/todo/failures.
- `npm run typecheck`: PASS.
- `npm run build`: PASS, Vite 8.1.5, 22 modules transformed; output JS 149.51 kB (52.37 kB gzip), CSS 59.39 kB (11.72 kB gzip).
- Initial `scripts/test.ps1`: environment FAIL before compilation for dependency-bearing packages because five modules could not download from `proxy.golang.org`; 12 dependency-light packages still passed. This is a baseline environment/download failure, not a source-code regression.
- Neither the repository-local nor default user Go module cache contains the five missing dependencies. An alternate bounded module-proxy download is needed before Go baseline can be completed.
- Alternate locked-version module download through `goproxy.cn` succeeded. The unchanged baseline then passed `go test ./...` across all packages and `go vet ./...` with no vet diagnostics.

## `App.vue` Responsibility Outline

- Script occupies lines 1-1054 and template begins at 1055; there is no component-local style block.
- Runtime/domain data is held in page-local refs: dashboard, routes, subscriptions, logs, IP detection, host status, benchmarks, core updates, and traffic snapshots.
- UI-only data is mixed beside it: page/theme, filters, drafts, dialog visibility, pending maps, notice/failure/activity presentation, simulation controls, and close preference UI.
- Backend intent handlers are grouped but centralized in the component: capture/runtime/routing/core/node/subscription/diagnostics/log/host/update actions.
- Presentation mapping/formatting is also centralized: capture/runtime/recovery/fault/phase/source labels plus bytes/rate/uptime/time/duration/error formatting.
- Five timers/listeners and the top-level mounted lifecycle coordinate dashboard/metrics/capture/log refresh and close requests.
- Existing `api.ts` already centralizes all backend method calls in one flat object, but it is untyped by feature and does not own Wails runtime events.
- Existing `state.ts` mixes runtime-derived application state, traffic buffering, risk presenters, and connection/health derivation. It does not yet separate runtime state from UI state modules.
- Current frontend tests cover close preference, runtime/risk derivation, and traffic helpers only. Adapter delegation, explicit UI-state transitions, and phase/recovery presenters are missing.

## API Adapter Result

- Wails access is now split into typed runtime, capture, routing, core, nodes, subscriptions, diagnostics, logs, and system adapters.
- Each adapter factory accepts an injectable backend/runtime provider, so delegation is executable in Node tests without recreating backend logic.
- The legacy flat `api` object remains as a temporary compatibility facade while features migrate; it delegates to the new grouped `apis` object.
- Direct `EventsOn` and window-theme runtime calls were removed from `App.vue` and routed through `apis.system`.
- Focused result: 13/13 frontend tests PASS (2 new adapter tests) and typecheck PASS.

## Feature, Runtime, and Facade Result

- `App.vue` is now 301 lines and contains shell/navigation/global feedback/TUN fault/close dialog composition only; its former seven pages are Feature Page components.
- Application setup and top-level polling/lifecycle moved to `features/application/useNavoApplication.ts`; pages consume the injected application context and do not access Wails directly.
- Runtime snapshots are normalized in `state/runtime.ts`; dashboard refresh and metric sampling replace runtime truth from backend snapshots rather than patching an optimistic frontend copy.
- UI state contracts live in `state/ui.ts`; page, dialog, loading, notice, and failure transitions are explicitly presentation-only and tested.
- Runtime phase/capture/routing/recovery presenters live in `features/runtime/presenter.ts`; verifying and rolling-back mappings are covered.
- Frontend success/failure health counters were removed. Network health now derives only from backend recovery/readiness/capture/metrics facts.
- Optimistic `dashboard.runtime.list_mode` assignment was removed. A committed mutation followed by refresh failure leaves the prior visible snapshot and reports that synchronization will retry.
- Target-route benchmark selection/restoration moved from Vue into `routeBenchmarkApplication`; the Wails method is a thin delegate and tests cover switch -> benchmark -> restore, capture rejection before mutation, and restore failure.
- Focused result after boundary changes: `go test ./navo_app` PASS, 17/17 frontend tests PASS, typecheck PASS.

## CSS and Browser Result

- The original 1,047 CSS lines were split at complete rule boundaries into tokens, shell, components, dialogs, pages, responsive, hierarchy, and connection modules; `style.css` now only imports them in the original order.
- Production CSS remained byte-equivalent after Vite processing: identical asset hash `D5vcQRCT` and identical 59.39 kB / 11.72 kB gzip size before and after modularization.
- Python Playwright smoke uses a mock Wails bridge and installed Edge, so it cannot mutate real proxy/TUN/network state.
- Browser PASS: all seven page IDs/navigation entries, page-local headings, backend-confirmed System Proxy intent, day/night themes, 760x900 compact layout, zero horizontal overflow, zero page errors, and zero console errors.
- Visual inspection of desktop day, desktop night, and compact night screenshots found no broken layout, empty feature region, clipped control, or unintentional hierarchy/theme change.
- After removing page-owned values from shell destructuring, `App.vue` is 192 lines and exposes only 31 shell/global fields from the application context.

## Final Gates

- `scripts/test.ps1`: PASS; all Go packages passed `go test ./...`, then `go vet ./...` completed with no diagnostics.
- `npm test`: PASS, 17/17, 0 skipped/todo/failures.
- `npm run typecheck`: PASS.
- `npm run build`: PASS; Vite 8.1.5, 51 modules, JS 156.18 kB / 55.55 kB gzip; CSS unchanged at 59.39 kB / 11.72 kB gzip.
- `scripts/ui_thin_layer_smoke.py` through the webapp-testing server helper: PASS, seven pages, two themes, 760x900 compact, zero horizontal overflow, zero page/console errors.
- `git diff --check`: PASS; only the repository's normal LF-to-CRLF warnings were emitted by Git status/diff inspection.

## Residual Debt

- The 1,058-line application controller is now UI-only but can be split further into per-feature composables in a later no-behavior-change pass.
- The flat `api.ts` compatibility facade remains while callers move to grouped adapters.
- Legacy string errors remain behind the adapter compatibility boundary; structured backend error DTO migration was intentionally deferred.

## Continuation Inventory

- The user requested continuation, so the previously reported controller/API debt is now in scope without reopening completed backend or visual work.
- `useNavoApplication.ts` is 1,058 lines and exposes a stable application-context surface consumed by seven Feature Page components; preserving that return surface allows internal extraction without page-template churn.
- The controller is the only remaining importer of the flat `src/api.ts` compatibility facade. Every call maps directly to an existing grouped adapter, so the facade can retire after this caller migrates.
- Low-coupling extraction candidates are shared formatting, feedback/activity state, logs, and close behavior. Higher-coupling groups are diagnostics, traffic, core, routing, nodes, subscriptions, and capture/runtime.
- Existing UI history requires all seven page IDs/actions, current typography/navigation hierarchy, two themes, and compact accessibility to remain unchanged; this continuation must remain a focused ownership refactor.
- The flat facade had no callers other than the application controller. All 29 calls were migrated one-for-one to `apis.runtime/capture/routing/core/nodes/subscriptions/diagnostics/logs/system`, after which `src/api.ts` was deleted.
- Facade retirement regression is clean: 17/17 frontend tests and `vue-tsc --noEmit` pass.
- The application controller is now 357 lines (down from the continuation baseline of 1,058) and is composition-oriented: it wires eleven composables, shell navigation/theme feedback, feature load dispatch, and initial snapshot loading.
- Feature ownership is now explicit for feedback, close behavior, logs, diagnostics, core, routing, subscriptions/upstreams, traffic, nodes, capture, and runtime overview presentation.
- Traffic polling/visibility listeners, log polling, close-request listeners, feedback timers, and capture-transition polling are cleaned up by their owning composables.
- New architecture guards enforce `App.vue <= 250` lines, controller `<= 400` lines, required composable presence, no Feature Page API imports, bridge access only in `api/client.ts`, and continued absence of the flat facade.
- Focused continuation result: 19/19 frontend tests PASS, typecheck PASS, and production build PASS with 62 modules; CSS remains `D5vcQRCT` at 59.39 kB / 11.72 kB gzip.
- Browser-visible continuation result: PASS for all seven pages, both themes, 760x900 compact layout, zero horizontal overflow, and mock-Wails isolation from real network state.
- Full continuation gates: all Go package tests PASS, `go vet ./...` PASS, 19/19 frontend tests PASS, typecheck/build PASS, and `git diff --check` PASS with line-ending warnings only.
- The only remaining item recorded as debt is structured backend error DTO migration, which would change the backend contract and is not a UI thin-layer ownership issue.

## `App.vue` Detailed Audit (lines 1-720)

- A/Layout: navigation metadata and page selection are shell responsibilities; navigation data can move to a shell model while page selection remains UI state.
- B/UI state: theme, filters, drafts, dialog/form visibility, pending flags/maps, activity/toast/failure, simulation settings, and close-choice fields are legitimate frontend state but should be feature-owned.
- C/Runtime state: dashboard/routes/subscriptions/logs/IP/host/benchmark/update/traffic data all originate from backend calls and must move to a runtime store or feature snapshots without adding a second truth source.
- D/API calls: all business calls route through the existing adapter, but 30+ action/load functions remain centralized in `App.vue`.
- E/Business decision violation: `setRoutingListMode` directly assigns `dashboard.value.runtime.list_mode = mode` after the mutation but before a successful snapshot refresh. This is an optimistic UI copy of backend truth and must be removed or replaced with a separate pending intent.
- F/Formatting: network-health, capture/runtime/recovery/fault/repair/phase/source and numeric/time/error mapping should move to presenter/formatter modules.
- G/Dialog/toast: generic `execute`, activity progress, notices/failures, TUN fault focus, confirm prompts, and close choice are presentation concerns; they should be reusable UI services/composables.
- H/Traffic: ring buffer sampling, backend metric snapshot mapping, synthetic preview controls, and chart view selection are a coherent feature; the backend metrics remain authoritative.
- K/Capture: `setCapture` correctly polls backend snapshots during transition and refreshes after completion; it does not set committed capture optimistically.
- L/Routing: parsing drafts is frontend input normalization, but committed list mode must only come from the refreshed dashboard snapshot.
- Load/page dispatch and timer ownership belong to shell lifecycle or feature composables, not a single page component.

## `App.vue` Detailed Audit (lines 721-1400)

- E/Business decision violation: `benchmarkRoute` temporarily switches the selected route, decides that capture must be off, verifies core running state, runs a benchmark, and attempts to restore the prior route in `finally`. This is node-switch/rollback orchestration that would still be required by a CLI and must move behind a backend application operation.
- E/Health truth concern: `updateHealthCounters` creates frontend success/failure hysteresis which feeds `deriveAppState` and can override displayed network health. Network health is a backend/runtime truth under this specification and this frontend counter authority should be removed or reduced to non-authoritative presentation.
- I/Subscription and upstream: form drafts, confirms, visibility, and reset-on-success are UI-owned; create/delete/refresh calls are simple intents and can live in a source feature composable.
- J/Core: update/install pending flags and confirmation are UI-owned; update inspection/install and core selection remain backend intents. No core lifecycle orchestration is present in this range.
- M/Diagnostics: latency/benchmark/traffic-transfer calls and results form a diagnostics feature, but target-route benchmark orchestration currently crosses the boundary.
- Template begins as a full seven-page application inside `App.vue`: shell/nav/global feedback/TUN dialog, overview and connection content are all inline. Each page/visual region should become a component with a one-sentence responsibility.
- Capture controls correctly bind selected/phase text to the backend dashboard snapshot. The TUN retry button sends an intent and does not claim success optimistically.
- Routing rule textarea parsing/counts are presentation/input concerns; persisted/active routing semantics remain backend-owned.

## `App.vue` Detailed Audit (lines 1401-1723 and `state.ts`)

- The remaining inline pages map cleanly to feature components: Diagnostics/Sources (1415-1469), Core (1471-1516), Traffic (1518-1552), IP/Host Diagnostics (1554-1624), Settings/Logs (1626-1665), and Close Dialog (1669-1722).
- B/UI state tests should cover navigation/page selection, dialog visibility/close choices, form visibility/drafts, pending activity, and routing editor state independently of runtime snapshots.
- H/Traffic presentation is large enough for a feature composable plus page/panel components; sampling still consumes backend metrics only.
- `state.ts` confirms frontend health hysteresis is authoritative: `deriveHealth` declares unavailable after three frontend failures and requires two successes for healthy even when backend snapshots say otherwise. Remove these UI counters from runtime truth derivation.
- `state.ts` otherwise acts as a presenter: capture uses `capture.committed_mode`, recovery/readiness/metrics/core/active route come from backend snapshots, and icon/connection/risk are user-facing mappings. These mappings belong in feature presenters, not a monolithic state file.
- The close dialog is legitimate UI state plus two backend intents (minimize/exit) and should be isolated as a shell feature; the backend remains responsible for safe exit and network restoration.
