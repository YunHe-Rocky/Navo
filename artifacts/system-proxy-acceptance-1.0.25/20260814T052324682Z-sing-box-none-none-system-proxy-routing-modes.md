# Navo TUN acceptance result

- Executed: 2026-08-14T05:23:26.4426897+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260814T052324682Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

Customer routing rules failed to apply
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:568 char:13
+             throw "Customer routing rules failed to apply"
+             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (Customer routing rules failed to apply:String) [], RuntimeException
    + FullyQualifiedErrorId : Customer routing rules failed to apply
