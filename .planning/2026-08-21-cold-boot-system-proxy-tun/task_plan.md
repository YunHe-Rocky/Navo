# Navo Cold-Boot and System Proxy/TUN Incident

## Goal

Reproduce and repair the current post-boot inability to use Navo and the UI path where enabling System Proxy reports a TUN failure, while preserving unrelated v2rayN processes and the existing UI thin-layer refactor.

## Constraints

- Treat `off`, `system_proxy`, and `tun` as separate mutually exclusive capture modes.
- Do not stop or mutate v2rayN or other user-owned processes.
- Preserve the existing uncommitted UI thin-layer refactor.
- Listener/UI/process state is not acceptance; verify real data-plane traffic and exact rollback where privileges permit.
- Do not claim cold-boot acceptance without an actual reboot/logon run.

## Phases

- [complete] Phase 1: Capture current boot, startup-registration, running-binary, listener, log, transition, and UI-call evidence.
- [complete] Phase 2: Trace the System Proxy/TUN intent from Vue through Wails, Agent, Service, and startup recovery; identify the first incorrect boundary.
- [complete] Phase 3: Implement fail-closed no-route activation, capture-scoped fault reporting, and the documented opt-in login startup/boot-connect path with regression coverage, without overlapping unrelated frontend work.
- [complete] Phase 4: Run focused/full source gates and build a verified portable release.
- [in_progress] Phase 5: Execute safe live System Proxy validation and, with approval/elevation, TUN validation; report the physical reboot gate separately.

## Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Restricted runner failed while applying deny-read ACLs | 1-2 | Switched to approved repository-scoped/read-only commands; no product or network mutation occurred. |
| Combined planning/memory and root-plan reads were truncated | 1 | Switched to bounded skill, current isolated-plan, process, startup, and log reads. |
| First structured-log projection used top-level fields that are nested under `fields` | 1 | Inspect the schema and project only nested allowlisted failure fields on the next read. |
| Initial focused Go test command pointed GOMODCACHE at missing `.cache/go-mod` | 1 | Reused the repository's existing `.cache/go-path/pkg/mod` and offline module cache. |
| Built-in patch helper could not apply repository ACLs | 2 | Used the native Codex patch entrypoint with repository-scoped elevation; edits still went through apply-patch semantics. |
| First frontend build found `captureRouteMissing` was not exported by the application composer | 1 | Added the missing feature-composer destructure and context return; rerun build next. |
| First package run timed out downloading Wails CLI dependencies from `proxy.golang.org` | 1 | Verified the same public module through `goproxy.cn`; reran with a process-local GOPROXY override only. |
| First mirrored package rerun hit a stale repository Go build-cache linker symbol | 1 | Validated `.cache/go-build` is repository-local, cleared only that cache, and re-ran `internal/service` successfully before packaging. |

## Current Phase

Phase 5 is partially complete. The isolated packaged-runtime no-route/System Proxy case passes with one bounded attempt, empty TUN fault, exact v2rayN/WinINet preservation, graceful exit, and post-smoke package verification. Successful proxied traffic, elevated TUN data-plane acceptance, actual user opt-in task registration, and a physical reboot/logon run remain blocked by the current profile having no configured route and by the need for the user's explicit startup choice/elevation/reboot.
