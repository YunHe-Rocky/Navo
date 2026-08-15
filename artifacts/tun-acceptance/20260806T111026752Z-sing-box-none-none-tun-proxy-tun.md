# Navo TUN acceptance result

- Executed: 2026-08-06T11:10:34.5865616+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 39.174.81.62
- Direct IP after: 
- Result: failed
- Rollback: failed
- JSON evidence: 20260806T111026752Z-sing-box-none-none-tun-proxy-tun.json

## Error

TUN to system proxy transition failed
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:427 字符: 55
+ ... ype -ne "RESPONSE") { throw "TUN to system proxy transition failed" }
+                           ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN to system proxy transition failed:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN to system proxy transition failed
Rollback check: Network rollback mismatch for nrpt
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:304 字符: 13
+             throw "Network rollback mismatch for $Key"
+             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (Network rollback mismatch for nrpt:String) [], RuntimeException
    + FullyQualifiedErrorId : Network rollback mismatch for nrpt
