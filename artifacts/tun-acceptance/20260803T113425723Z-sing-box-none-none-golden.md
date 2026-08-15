# Navo TUN acceptance result

- Executed: 2026-08-03T11:34:41.1710489+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 124.160.204.143
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260803T113425723Z-sing-box-none-none-golden.json

## Error

TUN activation failed: {"payload":{"code":"CAPTURE_TRANSITION_FAILED","message":"replace capture transition journal: re
place atomic file: MoveFileExW: Access is denied."},"request_id":"tun-acceptance-1785756884619","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:164 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"CAPTURE_TRANSITION_FAILED","message":"replace 
    capture transition journal: replace atomic file: MoveFileExW: Access is denied."},"request_id":"tun-acceptance-17  
  85756884619","type":"ERROR"}
