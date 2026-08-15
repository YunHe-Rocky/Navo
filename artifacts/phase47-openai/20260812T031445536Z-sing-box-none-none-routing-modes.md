# Navo TUN acceptance result

- Executed: 2026-08-12T03:14:54.0471301+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 183.158.168.94
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260812T031445536Z-sing-box-none-none-routing-modes.json

## Error

TUN activation failed: {"payload":{"code":"TUN_ADAPTER_NOT_READY","message":"configure owned TUN routes and DNS: TUN_AD
APTER_NOT_READY: stage=ADAPTER_READY resource=Navo expected=Up 172.19.0.1/30 MTU=1500 actual=powershell.exe failed: con
text deadline exceeded: : context deadline exceeded"},"request_id":"tun-acceptance-1786504502592","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:168 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_ADAPTER_NOT_READY","message":"configure o 
   wned TUN routes and DNS: TUN_ADAPTER_NOT_READY: stage=ADAPTER_READY resource=Navo expected=Up 172.19.0.1/30 MTU=15  
  00 actual=powershell.exe failed: context deadline exceeded: : context deadline exceeded"},"request_id":"tun-accept   
 ance-1786504502592","type":"ERROR"}
