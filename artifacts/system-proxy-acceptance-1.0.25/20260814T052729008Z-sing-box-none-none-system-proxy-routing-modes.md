# Navo TUN acceptance result

- Executed: 2026-08-14T05:27:34.9527877+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 183.158.168.94
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260814T052729008Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

chatgpt failed through the WinINet system proxy after 3 attempts: chatgpt returned HTTP 403; expected 200
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:347 char:21
+ ...             throw "$($Probe.Name) returned HTTP $Status; expected $($ ...
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (chatgpt returned HTTP 403; expected 200:String) [], RuntimeException
    + FullyQualifiedErrorId : chatgpt returned HTTP 403; expected 200
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:366 char:13
+             throw "$($Probe.Name) failed through the WinINet system p ...
+             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (chatgpt failed ...3; expected 200:String) [], RuntimeException
    + FullyQualifiedErrorId : chatgpt failed through the WinINet system proxy after 3 attempts: chatgpt returned HTTP  
   403; expected 200
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:347 char:21
+ ...             throw "$($Probe.Name) returned HTTP $Status; expected $($ ...
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
        + CategoryInfo          : OperationStopped: (chatgpt returned HTTP 403; expected 200:String) [], RuntimeExcept 
   ion
    + FullyQualifiedErrorId : chatgpt returned HTTP 403; expected 200
