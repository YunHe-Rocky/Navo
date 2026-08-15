# Navo TUN acceptance result

- Executed: 2026-08-10T11:35:42.7733753+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 211.90.237.75
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260810T113527100Z-sing-box-none-none-routing-modes.json

## Error

TUN routing mode whitelist failed to apply
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:523 字符: 29
+ ...                        throw "TUN routing mode $Mode failed to apply"
+                            ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN routing mod...failed to apply:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN routing mode whitelist failed to apply
