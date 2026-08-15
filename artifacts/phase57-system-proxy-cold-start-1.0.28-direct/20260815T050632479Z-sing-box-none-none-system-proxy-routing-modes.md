# Navo TUN acceptance result

- Executed: 2026-08-15T05:06:40.1439829+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 115.220.145.88
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260815T050632479Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

Exception calling "Start" with "1" argument(s): "The requested operation requires elevation"
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:201 char:16
+         return [System.Diagnostics.Process]::Start($Info)
+                ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : NotSpecified: (:) [], MethodInvocationException
    + FullyQualifiedErrorId : Win32Exception
