# Navo TUN acceptance result

- Executed: 2026-08-06T11:09:08.3621527+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 39.174.81.62
- Direct IP after: 
- Result: failed
- Rollback: failed
- JSON evidence: 20260806T110900366Z-sing-box-none-none-repeat.json

## Error

TUN activation failed: {"payload":{"code":"CAPTURE_BUSY","message":"capture transition is already in progress"},"reques
t_id":"tun-acceptance-1786014597373","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:164 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"CAPTURE_BUSY","message":"capture transition i 
   s already in progress"},"request_id":"tun-acceptance-1786014597373","type":"ERROR"}
Rollback check: TUN journal remains after rollback: D:\WorkSpace\Navo\.cache\tun-acceptance\sing-box-none-none-20260806T110900366Z\loca
lappdata\Navo\tun_network_journal.json
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:308 字符: 34
+ ... nt -gt 0) { throw "TUN journal remains after rollback: $($Journals.Fu ...
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN journal rem...rk_journal.json:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN journal remains after rollback: D:\WorkSpace\Navo\.cache\tun-acceptance\sing-box-non 
   e-none-20260806T110900366Z\localappdata\Navo\tun_network_journal.json
