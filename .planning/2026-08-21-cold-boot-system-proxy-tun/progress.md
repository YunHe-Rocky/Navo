# Progress

## 2026-08-21

- Selected and fully read the planning-with-files skill because this incident requires multi-step lifecycle diagnosis and repair.
- Restored the existing root and isolated UI-thin-layer plans; preserved all current uncommitted frontend work.
- Confirmed no Navo process or listener is currently running; identified and preserved v2rayN and its owned `127.0.0.1:10808` listener.
- Confirmed there is currently no Windows startup registration for Navo.
- Confirmed Navo did launch after this boot and wrote a transition/log failure around 13:58.
- Located the typed System Proxy/TUN dispatch paths. Next: extract the exact nested structured-log failure and correlate it with the UI intent and transition state.
- Completed Phase 1 evidence capture. The UI intent is conclusively System Proxy, not TUN.
- Identified a no-selection direct-only runtime admitted into System Proxy activation; Google/ChatGPT/OpenAI were sent to the direct outbound and timed out.
- Identified misleading TUN-only error/fault reuse on the System Proxy path. Phase 2 now traces selection restoration and the safest startup-registration/product boundary before implementation.
- Confirmed this is not a privacy reset: current startup reported `first_run=false`, `foreign_context=false`, and `privacy_reset=false`.
- Confirmed the profile genuinely has no subscription/upstream source, so no node can be restored and v2rayN must remain observe-only.
- Restored the documented opt-in `开机连接` requirement and chose a current-user, highest-privilege logon task plus explicit stored mode; default remains disabled and startup activation will use Coordinator origin `startup` after recovery.
- Implemented fail-closed `OUTBOUND_REQUIRED` admission for non-direct capture with no selected route.
- Separated System Proxy application-route verification from TUN adapter errors and prevented non-TUN transitions from publishing/rehydrating retryable TUN faults.
- Added an owned, path-bound Windows logon task controller using `--silent --startup`, delayed 15 seconds, default disabled, with bounded cleanup and no adoption of unrelated processes.
- Added Wails/API/thin-feature UI support for explicit `开机连接` mode and a no-route guard on System Proxy/TUN entry buttons.
- Reclassified structured `capture.*`/recovery logs as `Capture`; only real `tun.*` operations retain the `TUN` service label.
- Focused backend packages, 19 frontend tests, frontend typecheck/build, all Go packages, and `go vet ./...` pass before packaging.
- Built and verified `Navo-1.0.37-portable-amd64` plus ZIP: 28 files, 27 checksum entries, zero verifier issues.
- Extended the isolated launcher smoke to assert startup defaults and the no-route System Proxy/TUN fault boundary against a packaged runtime.
- First isolated runtime run preserved v2rayN/10808/WinINet exactly and returned `OUTBOUND_REQUIRED`, but exposed an invalid two-round auto-repair loop plus a tray-exit helper null-exit-code false failure; both now have bounded regression fixes.
- Second isolated runtime run passed the product boundary in 7.26 seconds with one attempt, empty TUN fault, committed `off`, launcher exit 0, and exact v2rayN/10808/WinINet preservation; final verifier only found a smoke-generated tray icon, now handled as a targeted test cleanup.
- Final cleaned package verification passes with 28 directory files, 28 ZIP files, zero issues, launcher/product version `1.0.37.0`, zero Navo process residue, and ZIP SHA-256 `920BEE9BD8362C0ECC8BEEDAB2A11B8D24DF5144886FD8A5B778CE88BF8803EE`.
