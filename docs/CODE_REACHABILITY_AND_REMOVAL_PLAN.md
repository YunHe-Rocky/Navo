# Navo Code Reachability and Removal Plan

Date: 2026-08-01

This inventory is the deletion gate required by the cleanup/privacy Guide. A module is not deleted merely because a static reference search is empty; string-dispatched IPC, Wails binding, tray callbacks, interfaces, build tags, recovery/migration paths, and tests are treated as live entry mechanisms.

## Production entry points

| Entry | Reachability | Classification | Decision |
|---|---|---|---|
| `cmd/navo/main.go` | Packaged single-process launcher | A | Keep; remove MySQL bootstrap and insert initialization before repository/network loading. |
| `cmd/navo-agent/main.go` | Standalone development/diagnostic Agent CLI | A/G | Keep until Service-pipe hardening decides its supported deployment boundary. |
| `cmd/navo-svc/main.go` | Standalone Service development/deployment CLI | A/G | Keep behind explicit external-pipe configuration; not used by single-process release. |
| `cmd/repair/main.go` | Recovery CLI (`check`, `fix`, `reset`, TUN/route/DNS actions) | A | Keep; recovery is explicitly protected from dead-code deletion. |
| `navo_app/main.go` | Wails UI executable | A | Keep. `wails.Run` dynamically binds `*App`. |

## Dynamic dispatch boundaries

### Agent UI Named Pipe

`internal/agent/agent.go` dispatches string methods. Direct Agent-owned commands include dashboard/tray snapshots, UI show, connection/capture changes, network recovery, and system-proxy status. It forwards core, TUN, subscription, outbound, runtime, metrics, IP, diagnostics, logs, and AI commands to Service.

AI forwarding methods (`ai.rule.generate`, `ai.diagnose`, `ai.explain`, `ai.config.get`, `ai.config.set`, `ai.config.test`) are classification D and must be removed together with the Service cases.

Raw core lifecycle aliases (`core.start`, `core.stop`, `core.restart`) are reachable but violate the target public API. They are classification E: remove from public Agent/Wails routing only after user-semantic capture operations cover all callers.

### Service dispatcher

`internal/service/service.go` uses a method-name switch for core, TUN, capture preparation, subscription, outbound, runtime, metrics, IP, recovery, diagnostics, core logs, application logs, and AI operations. These cases are dynamically reachable even when no ordinary Go call references their handlers.

- AI cases: D, delete.
- Public `config_path`/raw lifecycle cases: E, replace with controlled runtime targets/revision IDs.
- Subscription, outbound, capture, metrics, IP, logs, recovery: A/B, preserve and remediate.

### Wails binding

`navo_app/main.go` binds one `*App`. All exported `App` methods are dynamically callable:

- Dashboard/IP/routes/subscriptions/logs: A.
- Capture/runtime/route/subscription/upstream mutations: A, but must remain thin handlers over Agent/Service transactions.
- `SetCoreRunning`: E; replace its raw lifecycle semantics with capture operations.
- Host status, proxy benchmark, core-update check/release page: B; Feature Optimization requires completing them, so they are not dead code.

### Tray callbacks

`cmd/navo/tray_windows.go`, `tray_menu.go`, and `tray_backend.go` dynamically invoke show/exit, capture mode, routing mode, core, and outbound actions. They are A. Tray state and icons are Feature Optimization acceptance surfaces and must not be removed.

## UI pages and menus

Wails currently exposes these Vue pages from `navo_app/frontend/src/App.vue`:

| Page | Classification | Decision |
|---|---|---|
| 运行概览 | A | Keep. |
| 连接管理 | A | Keep; bind only to runtime transition operations. |
| 线路来源 | B | Keep and convert to required layered one-click latency testing. |
| 内核管理 | B | Rename/rebuild as 升级内核 with verified transactional update flow. |
| 流量监控 | B | Keep and implement four independently selectable real counters. |
| 网络检测 | B | Keep direct/proxy isolation and risk aggregation. |
| 设置/日志 | B | Keep; implement structured filters, clearing, history/live behavior, and redaction. |

No reachable AI page/menu is present in the current Vue shell. Backend AI code remains reachable through IPC and must still be removed; absence of UI alone is not sufficient cleanup.

## Background work and lifecycle owners

Production goroutines include Service and Agent launch loops, UI process observation/focus, tray action execution, UI/Service pipe acceptance and per-connection handlers, capture-health monitoring, dashboard/IP fan-out, core process wait/monitoring, health checks, TUN adapter monitoring, subscription refresh, runtime apply tasks, Supervisor monitoring, and Wails diagnostic benchmarks.

Ownership decisions:

- Host process wait/exit notification: A; Host self-restart behavior is E and belongs exclusively to Supervisor.
- Supervisor lifecycle/restart budget: A; keep and harden.
- Capture/network reconciliation and recovery: A; keep and consolidate on structured journals.
- Destructive background runtime apply after returning success: E; replace with synchronous transactions or explicit observable task IDs.
- Core update, traffic, IP risk, structured logging tasks: B; planned integrations, not deletion targets.

## Repository implementations

| Repository | Classification | Decision |
|---|---|---|
| `internal/infrastructure/mysqlstore/RevisionRepository` | C | Deleted after the versioned atomic local replacement passed tests. |
| `internal/infrastructure/mysqlstore/SelectionRepository` | C | Deleted after the versioned atomic local replacement passed tests. |
| `internal/domain/revision.Repository` | A | Preserve business contract. |
| `internal/domain/selection.Repository` | A | Preserve business contract. |
| `internal/storage.Store` | A/E | Reuse only after hardening atomic persistence; remove obsolete SQLite migration comments. |
| credential/subscription/upstream/network journal file stores | A | Preserve, consolidate on one atomic writer and explicit ACL/DPAPI policy. |

The local replacement must preserve candidate validation, runtime application, atomic revision write, atomic active-selection update, committed-runtime consistency, and rollback without half-commit.

## AI removal set (D)

- `internal/ai/` implementation and tests.
- `internal/service/ai_settings.go` and tests.
- AI fields/imports/initialization/dispatch handlers in Service.
- AI forwarding cases in Agent.
- AI IPC DTOs/events/error codes/client methods if found during deletion.
- `ai-settings.json`, protected API key, provider/model/prompt/cache/history legacy data through initialization cleanup.
- AI-only dependencies/config/docs/build references.

Pure local diagnostics embedded in AI code must first move to an existing diagnostics/network package without AI naming or network dependency.

## MySQL removal set (C)

- `internal/infrastructure/config/mysql.go` and tests.
- `internal/infrastructure/mysqlstore/` and tests/migrations.
- MySQL bootstrap/imports and database connection lifecycle in `cmd/navo/main.go`.
- MySQL driver from `go.mod`/`go.sum`.
- MySQL environment fields and legacy DB status/cache/errors through initialization cleanup.

Selection, revision, subscriptions, outbounds, runtime state, preferences, and recovery state are not MySQL-specific and must remain locally persistent.

## Build-tag and platform reachability

Windows-specific files under tray, system proxy, network executor/platform/TUN, Named Pipe, securestore, and winprocess are A. Their `!windows` stubs are also A because non-Windows compilation/tests require them. Windows tests and the `integration`-tag host test are test entry points and must not be treated as unreachable production code.

No `init()`-registered production providers were found in the current Go source inventory. Interface injection is present across repositories, core adapters, Supervisor/Host, Agent/Service dispatch, and platform abstractions.

## Candidate obsolete/unreachable code

| Candidate | Classification | Evidence/next action |
|---|---|---|
| AI implementation | D | Deleted as a complete vertical slice, including dynamic Agent/Service dispatch and settings persistence. |
| MySQL implementation | C | Deleted after launcher migration to tested local revision/selection repositories. |
| Raw public core lifecycle operations | E | Superseded by unified runtime transition; retain private Service lifecycle only. |
| Old localized-text route/DNS cleanup | G | Recovery purpose exists; identify exact replacement and tests before any deletion. |
| `internal/storage` SQLite-phase comments | E | Stale documentation only; implementation remains potentially useful. |
| Standalone Agent/Service commands | G | Development/deployment role exists; retain until external-pipe security boundary is explicit and tested. |

No other module is classified F at this stage. A second unreachable-code pass is permitted only after remediation and Feature Optimization integrations are complete.

## Deletion proof checklist

Each E/F deletion requires: no production entry, no registration/map/string IPC/Wails call, no build-tag use, no migration/recovery/test use, no Guide-planned use, passing full tests/build, no remaining config/call references, and a documented replacement or business reason.
