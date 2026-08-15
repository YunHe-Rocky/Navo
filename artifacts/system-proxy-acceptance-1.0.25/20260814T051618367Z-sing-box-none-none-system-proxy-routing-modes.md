# Navo TUN acceptance result

- Executed: 2026-08-14T05:16:21.6587661+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260814T051618367Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

Navo exited during startup with code 1
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:124 char:35
+ ... asExited) { throw "Navo exited during startup with code $($Process.Ex ...
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (Navo exited during startup with code 1:String) [], RuntimeException
    + FullyQualifiedErrorId : Navo exited during startup with code 1
