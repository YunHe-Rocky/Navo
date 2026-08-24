# Navo UI Thin-Layer Refactor

## Goal

Implement `docs/Navo_UI_Thin_Layer_Refactor_Codex.md` without changing network behavior: make Vue a thin intent-and-presentation client, centralize Wails access, separate runtime truth from UI state, reduce `App.vue` to an application shell, modularize styles, keep the Wails facade thin, and pass all required Go/frontend gates.

## Constraints

- Preserve all unrelated and pre-existing user changes in the dirty worktree.
- Do not redesign TUN, Connection Coordinator, SelfHeal, Core Supervisor, protocols, or network semantics.
- Backend snapshots remain authoritative for core, node, capture, connection, and recovery state.
- UI may own presentation, drafts, pending indicators, dialogs, tabs, toasts, formatting, and charts only.
- Migrate incrementally and keep the current visual and interaction behavior stable.

## Phases

- [complete] Phase 0: Read repository constraints and the complete specification; inventory the worktree; run and record the pre-refactor Go/frontend baseline.
- [complete] Phase 1: Audit `App.vue` by responsibility and produce a concrete migration checklist.
- [complete] Phase 2: Centralize Wails calls behind typed frontend API adapters with adapter tests.
- [complete] Phase 3: Extract features incrementally in the required order while preserving behavior and adding focused tests.
- [complete] Phase 4: Separate backend-derived runtime state from frontend-owned UI state and remove optimistic business truth.
- [complete] Phase 5: Reduce `App.vue` to shell/composition/global lifecycle and verify responsibility boundaries.
- [complete] Phase 6: Inspect and thin only clearly overreaching Wails facade methods, without backend redesign.
- [complete] Phase 7: Modularize CSS without visual redesign or duplicated override layers.
- [complete] Phase 8: Run focused and full Go/frontend/CI gates; review diffs, residual debt, and final acceptance checklist.
- [complete] Phase 9: Inventory the application controller dependency graph; extract shared feedback/formatting/close behavior and remove the flat API compatibility dependency.
- [complete] Phase 10: Extract logs, diagnostics, traffic, core, routing, node, and subscription state/actions into feature composables while preserving the application context contract.
- [complete] Phase 11: Reduce the application controller to shell composition and lifecycle wiring; add focused composable/adapter regression coverage.
- [complete] Phase 12: Rerun frontend, Go, browser-visible, and diff gates; update the migration audit with final ownership and residual risk.

## Errors Encountered

| Error | Attempt | Resolution |
|---|---:|---|
| Restricted runner failed to create PowerShell/cmd processes while applying deny-read ACLs | 1-2 | Switched to approved read-only escalated commands; no repository mutation occurred. |
| Initial combined read was truncated because existing planning files and the specification are large | 1 | Continue with bounded file inventories and line-ranged reads. |
| Direct patch tool failed to apply deny-read ACLs while updating plan logs | 1-3 | Located npm Codex's native apply-patch mode and use it through an approved elevated runner; all edits retain patch semantics. |
| Looked for `package.json` under `navo_app/frontend` but this checkout does not use that layout | 1 | Recheck the actual project root under `navo_app` before baseline commands. |
| Go baseline could not download five required modules from `proxy.golang.org` | 1 | Classified as pre-refactor environment failure; bounded alternate proxy download succeeded, then the same test/vet baseline passed. |
| First large apply-patch attempt used stdin, but this Codex binary requires a UTF-8 patch argument | 1 | No files changed; switched to bounded chunked apply-patch arguments. |
| Mechanical script extraction included the first `emptyDashboard` declaration line with imports after an earlier import removal shifted line numbers | 1 | Moved the declaration into `useNavoApplication`; rerun typecheck before further extraction. |
| First Feature Page generator command used backslash quote escaping that PowerShell does not support | 1 | Parser failed before execution; changed page patterns to PowerShell single-quoted strings. |
| Generated App imports omitted the `.vue` suffix required by this TypeScript configuration | 1 | Added explicit `.vue` suffixes to all seven page imports. |
| First combined route-benchmark patch contained an empty update hunk before the next file marker | 1 | Patch parser rejected the whole patch before changes; removed the stray hunk marker and reapplied successfully. |
| First browser smoke expected the overview page name inside `.task-page`, but overview intentionally puts that name in the shared shell header | 1 | Assert every page name in `.page-heading h1`; keep feature-local heading assertions only where the page has one. |
| Playwright partial accessible-name matching for “系统代理” also matched the adjacent TUN option description | 1 | Changed the interaction selector to exact accessible-name matching. |
| Exact accessible-name matching did not find the mode button because its accessible name intentionally includes the explanatory `<small>` text | 2 | Scoped to `.mode-entry-options button` containing an exact visible `<strong>` label. |
| Workspace dependency locator did not return after two minutes | 1 | Terminated the read-only lookup, installed Python Playwright into repository-local cache, and reused installed Edge without downloading a browser. |

## Current Phase

All continuation phases are complete.
