# Navo TUN acceptance result

- Executed: 2026-08-14T05:28:46.1489574+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 183.158.168.94
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260814T052839974Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

System proxy routing mode direct failed to apply
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:595 char:17
+ ...               throw "System proxy routing mode $Mode failed to apply"
+                   ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (System proxy ro...failed to apply:String) [], RuntimeException
    + FullyQualifiedErrorId : System proxy routing mode direct failed to apply
