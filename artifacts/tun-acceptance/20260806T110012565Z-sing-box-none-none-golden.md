# Navo TUN acceptance result

- Executed: 2026-08-06T11:00:21.5233917+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 39.174.81.62
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260806T110012565Z-sing-box-none-none-golden.json

## Error

TUN activation failed: {"payload":{"code":"TUN_HTTPS_VERIFY_FAILED","message":"TUN_HTTPS_VERIFY_FAILED: stage=DATA_PLAN
E_VERIFIED resource=www.cloudflare.com expected=TLS and HTTP response actual=failed: Get \"https://www.cloudflare.com/c
dn-cgi/trace\": EOF"},"request_id":"tun-acceptance-1786014025849","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:164 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_HTTPS_VERIFY_FAILED","message":"TUN_HTTPS 
   _VERIFY_FAILED: stage=DATA_PLANE_VERIFIED resource=www.cloudflare.com expected=TLS and HTTP response actual=failed  
  : Get \"https://www.cloudflare.com/cdn-cgi/trace\": EOF"},"request_id":"tun-acceptance-1786014025849","type":"ERRO   
 R"}
