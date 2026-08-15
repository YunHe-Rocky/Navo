# Navo TUN acceptance result

- Executed: 2026-08-14T05:25:07.7947179+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260814T052506033Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

Unable to find type [System.Net.Http.HttpClientHandler].
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:332 char:24
+             $Handler = [System.Net.Http.HttpClientHandler]::new()
+                        ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidOperation: (System.Net.Http.HttpClientHandler:TypeName) [], RuntimeException
    + FullyQualifiedErrorId : TypeNotFound
