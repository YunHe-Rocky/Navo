# Navo TUN acceptance result

- Executed: 2026-08-09T10:48:17.9637376+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: mihomo 1.19.29
- Failure point: none
- Crash point: none
- Direct IP before: 211.90.237.75
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260809T104802862Z-mihomo-none-none-tun-off-tun.json

## Error

TUN activation failed: {"payload":{"code":"TUN_HTTPS_VERIFY_FAILED","message":"TUN_HTTPS_VERIFY_FAILED: stage=DATA_PLAN
E_VERIFIED resource=www.cloudflare.com expected=TLS and HTTP response actual=failed: Get \"https://www.cloudflare.com/c
dn-cgi/trace\": EOF"},"request_id":"tun-acceptance-1786272590501","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:166 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_HTTPS_VERIFY_FAILED","message":"TUN_HTTPS 
   _VERIFY_FAILED: stage=DATA_PLANE_VERIFIED resource=www.cloudflare.com expected=TLS and HTTP response actual=failed  
  : Get \"https://www.cloudflare.com/cdn-cgi/trace\": EOF"},"request_id":"tun-acceptance-1786272590501","type":"ERRO   
 R"}
