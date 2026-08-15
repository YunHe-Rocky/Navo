# Navo TUN acceptance result

- Executed: 2026-08-03T11:25:04.4206618+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 124.160.204.143
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260803T112447438Z-sing-box-none-none-golden.json

## Error

TUN activation failed: {"payload":{"code":"TUN_PHYSICAL_ROUTE_NOT_FOUND","message":"TUN_PHYSICAL_ROUTE_NOT_FOUND: stage
=PREFLIGHT resource=127.0.0.1: no routable address resolved"},"request_id":"tun-acceptance-1785756308559","type":"ERROR
"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:156 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_PHYSICAL_ROUTE_NOT_FOUND","message":"TUN_ 
   PHYSICAL_ROUTE_NOT_FOUND: stage=PREFLIGHT resource=127.0.0.1: no routable address resolved"},"request_id":"tun-acc  
  eptance-1785756308559","type":"ERROR"}
