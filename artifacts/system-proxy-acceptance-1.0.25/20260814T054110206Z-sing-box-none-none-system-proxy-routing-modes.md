# Navo TUN acceptance result

- Executed: 2026-08-14T05:41:14.3343006+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 183.158.168.94
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260814T054110206Z-sing-box-none-none-system-proxy-routing-modes.json

## Error

Exception calling "GetResult" with "0" argument(s): "发送请求时出错。"
At D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:385 char:9
+         $Value = $Client.GetStringAsync("https://api4.ipify.org").Get ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : NotSpecified: (:) [], MethodInvocationException
    + FullyQualifiedErrorId : HttpRequestException
