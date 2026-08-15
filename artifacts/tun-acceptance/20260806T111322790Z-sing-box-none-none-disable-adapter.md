# Navo TUN acceptance result

- Executed: 2026-08-06T11:13:31.9846997+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 39.174.81.62
- Direct IP after: 
- Result: failed
- Rollback: failed
- JSON evidence: 20260806T111322790Z-sing-box-none-none-disable-adapter.json

## Error

Disable-NetAdapter : 找不到任何“Name”属性等于“Navo”的 MSFT_NetAdapter 对象。请验证属性值，然后重试。
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:440 字符: 21
+ ...             Disable-NetAdapter -Name "Navo" -Confirm:$false -ErrorAct ...
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (Navo:String) [Disable-NetAdapter], CimJobException
    + FullyQualifiedErrorId : CmdletizationQuery_NotFound_Name,Disable-NetAdapter
Rollback check: Network rollback mismatch for nrpt
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:304 字符: 13
+             throw "Network rollback mismatch for $Key"
+             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (Network rollback mismatch for nrpt:String) [], RuntimeException
    + FullyQualifiedErrorId : Network rollback mismatch for nrpt
