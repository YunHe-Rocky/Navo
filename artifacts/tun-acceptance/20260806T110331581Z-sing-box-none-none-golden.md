# Navo TUN acceptance result

- Executed: 2026-08-06T11:03:42.7487892+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 39.174.81.62
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260806T110331581Z-sing-box-none-none-golden.json

## Error

TUN activation failed: {"payload":{"code":"TUN_EXIT_IP_MISMATCH","message":"TUN_EXIT_IP_MISMATCH: stage=DATA_PLANE_VERI
FIED resource=proxy_exit_ip expected=TUN != 39.174.81.62 and TUN == local-proxy actual=tun=39.174.81.62 proxy=39.174.81
.62"},"request_id":"tun-acceptance-1786014223803","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:164 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_EXIT_IP_MISMATCH","message":"TUN_EXIT_IP_ 
   MISMATCH: stage=DATA_PLANE_VERIFIED resource=proxy_exit_ip expected=TUN != 39.174.81.62 and TUN == local-proxy act  
  ual=tun=39.174.81.62 proxy=39.174.81.62"},"request_id":"tun-acceptance-1786014223803","type":"ERROR"}
