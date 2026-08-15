# Navo TUN acceptance result

- Executed: 2026-08-15T04:50:32.0238029+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: after-nrpt
- Direct IP before: 115.220.145.88
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260815T045017693Z-sing-box-none-after-nrpt-golden.json

## Error

Crash point did not terminate Navo
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:748 字符: 49
+ ... ss.WaitForExit(15000)) { throw "Crash point did not terminate Navo" }
+                              ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (Crash point did not terminate Navo:String) [], RuntimeException
    + FullyQualifiedErrorId : Crash point did not terminate Navo
