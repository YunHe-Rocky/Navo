# Navo UI Thin-Layer Migration Audit

This audit classifies the pre-refactor `navo_app/frontend/src/App.vue` before product code changes. The migration rule is: if the behavior is still required by a CLI, it is not UI behavior.

## Baseline

| Item | Pre-refactor result |
|---|---|
| `App.vue` | 1,723 lines; script 1-1053; template 1055-1723 |
| `state.ts` | 239 lines; runtime derivation, health hysteresis, risk presenters, traffic buffer |
| `style.css` | 1,047 lines |
| Go tests/vet | PASS after locked modules were populated in the repository-local cache |
| Frontend tests | PASS, 11/11 |
| Typecheck/build | PASS / PASS |

## Responsibility classification

| Class | Current responsibility | Current location | Target | Decision |
|---|---|---|---|---|
| A. Layout / Template | Shell, sidebar, seven pages, global dialogs | `App.vue:1055` | Shell plus feature/page components | Move; preserve page IDs, labels, ARIA, and layout |
| B. UI State | Page/theme, filters, drafts, dialogs, pending flags, feedback | `App.vue:73` | `state/ui.ts` and feature composables | Keep in UI, split by owner |
| C. Runtime State | Dashboard, routes, subscriptions, logs, IP, host, benchmarks, traffic | `App.vue:75` | `state/runtime.ts` and feature snapshot refs | Backend remains truth; UI stores snapshots only |
| D. Backend API Call | Flat Wails adapter and 30+ component handlers | `api.ts`, `App.vue:323` | Typed `api/` adapters used by features | Centralize by capability |
| E. Business Decision | Optimistic list-mode truth, health hysteresis, benchmark switch/restore | `App.vue:622`, `App.vue:723`, `state.ts:208` | Backend snapshot/application operation | Remove from UI |
| F. Formatting | Status labels, errors, byte/rate/time formatting | `App.vue:876` | `presenters/` and `utils/format.ts` | Keep as presentation logic |
| G. Dialog / Toast | Activity, notice/failure, TUN fault, close choice | `App.vue:283`, template | Common feedback plus capture/close components | Keep as UI state/intents |
| H. Traffic / Chart | Sampling, ring buffer, view series, preview, controlled transfer | `App.vue:554`, traffic page | Traffic feature | Backend metrics remain authoritative |
| I. Subscription | Drafts, forms, list, create/delete/refresh | `App.vue:842`, connection page | Subscription/source feature | UI owns drafts; backend owns validation/persistence |
| J. Core | Selection, update check/install presentation | `App.vue:480`, core page | Core feature | UI sends intents only |
| K. Capture | Mode intent, transition polling, fault retry | `App.vue:594`, connection page | Capture feature | Snapshot committed mode is the only truth |
| L. Routing | Policy/list intents, editor drafts and counts | `App.vue:615`, connection page | Routing feature | Remove optimistic committed-mode assignment |
| M. Diagnostics | IP, route latency, proxy benchmark, transfer, host status, logs | `App.vue:414`, sources/IP/settings pages | Diagnostics/log features | Move target-route switch/restore to backend application operation |

## Concrete boundary defects

1. `setRoutingListMode` writes `dashboard.runtime.list_mode` before a successful snapshot refresh. Replace this with pending UI state and backend snapshot refresh; never synthesize committed truth.
2. `deriveHealth` consumes frontend success/failure counters and can label the network unavailable or healthy independently of the backend snapshot. Derive the view solely from backend capture, readiness, recovery, and metrics fields.
3. `benchmarkRoute` selects a target route, checks capture/core constraints, runs the benchmark, and restores the previous route in `finally`. This is transaction and rollback behavior required by non-GUI clients; expose one backend application operation and make the UI issue a single intent.

## Migration checklist

- [x] Introduce typed feature API adapters and adapter delegation tests.
- [x] Establish normalized runtime snapshot state and frontend-only UI state.
- [x] Extract Capture and Routing controls; remove optimistic list-mode truth.
- [x] Extract Core and Node actions; move route-benchmark orchestration behind one backend intent.
- [x] Extract Subscription/source management.
- [x] Extract Traffic and Diagnostics features.
- [x] Extract Overview, Settings/Logs, close choice, shell navigation, and feedback.
- [x] Reduce `App.vue` to shell, feature composition, lifecycle, and global feedback entry.
- [x] Split CSS by tokens/base/layout/components without changing visual design.
- [x] Add UI-state, presenter/mapping, and API-adapter tests.
- [x] Run full Go, vet, frontend, typecheck, build, and browser-visible regression gates.

## Residual technical debt

- Existing backend errors are still adapted from legacy `Error`/string values in some UI paths. Migrating those to structured error DTOs is a separate backend contract change and remains outside this UI ownership refactor.

## Continuation closure

- `features/application/useNavoApplication.ts` is now a 357-line composition root, down from 1,058 lines at the continuation baseline. It wires feature composables, shell navigation/theme feedback, page-load dispatch, and initial snapshot loading.
- State/actions and resource cleanup are feature-owned by `useCapture`, `useCore`, `useNodes`, `useRouting`, `useSubscriptions`, `useTraffic`, `useDiagnostics`, `useLogs`, `useRuntimeOverview`, `useApplicationFeedback`, and `useCloseBehavior`.
- The temporary flat `src/api.ts` facade was deleted. Feature composables use grouped typed adapters, and direct Wails bridge access remains only in `api/client.ts`.
- Architecture guards now cap `App.vue` at 250 lines and the composition root at 400 lines, require the feature composables, reject API imports from Feature Page components, reject bridge access outside `api/client.ts`, and reject restoration of the flat facade.
- Final frontend result: 19/19 tests PASS, typecheck PASS, production build PASS (62 modules, JS 161.31 kB / 56.49 kB gzip, CSS 59.39 kB / 11.72 kB gzip).
- Final browser result: seven pages, two themes, 760x900 compact layout, zero horizontal overflow, and no browser errors under a mock Wails bridge.
- Final repository result: all Go package tests PASS, `go vet ./...` PASS, and `git diff --check` PASS (line-ending warnings only).

## Compatibility guardrails

- Keep all seven page IDs and existing backend Wails method behavior unless a new thin intent is required to remove UI business orchestration.
- Do not redesign capture, Coordinator, rollback, SelfHeal, Supervisor, protocols, or Windows network operations.
- Preserve task-oriented navigation, Chinese typography, technical monospace content, day/night themes, compact ARIA labels, and existing actions.
- Ambiguous ownership stays unchanged and is recorded as debt rather than triggering a backend rewrite.
