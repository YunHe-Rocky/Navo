# Navo TUN acceptance result

- Executed: 2026-08-10T12:18:46.0304424+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: mihomo 1.19.29
- Failure point: none
- Crash point: none
- Direct IP before: 211.90.237.75
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260810T121834357Z-mihomo-none-none-tun-off-tun.json

## Error

TUN activation failed: {"payload":{"code":"TUN_CORE_START_FAILED","message":"TUN_CORE_START_FAILED: stage=CORE_STARTED 
resource=mihomo: failed to start core: CORE_004: port 12080 not ready: timeout waiting for port 12080 after 10s"},"requ
est_id":"tun-acceptance-1786364427927","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:168 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_CORE_START_FAILED","message":"TUN_CORE_ST 
   ART_FAILED: stage=CORE_STARTED resource=mihomo: failed to start core: CORE_004: port 12080 not ready: timeout wait  
  ing for port 12080 after 10s"},"request_id":"tun-acceptance-1786364427927","type":"ERROR"}
