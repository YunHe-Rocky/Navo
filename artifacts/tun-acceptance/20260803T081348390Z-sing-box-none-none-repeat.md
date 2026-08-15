# Navo TUN acceptance result

- Executed: 2026-08-03T08:14:00.9424265+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 124.160.204.143
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260803T081348390Z-sing-box-none-none-repeat.json

## Error

TUN activation failed: {"payload":{"code":"TUN_EXIT_IP_MISMATCH","message":"TUN_EXIT_IP_MISMATCH: stage=DATA_PLANE_VERI
FIED resource=proxy_exit_ip expected=TUN != 124.160.204.143 and TUN == local-proxy actual=tun=165.254.151.219 proxy=: G
et \"https://www.cloudflare.com/cdn-cgi/trace\": read tcp 127.0.0.1:51420-\u003e127.0.0.1:12080: wsarecv: An existing c
onnection was forcibly closed by the remote host."},"request_id":"tun-acceptance-1785744843877","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:153 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_EXIT_IP_MISMATCH","message":"TUN_EXIT_IP_ 
   MISMATCH: stage=DATA_PLANE_VERIFIED resource=proxy_exit_ip expected=TUN != 124.160.204.143 and TUN == local-proxy   
  actual=tun=165.254.151.219 proxy=: Get \"https://www.cloudflare.com/cdn-cgi/trace\": read tcp 127.0.0.1:51420-\u00   
 3e127.0.0.1:12080: wsarecv: An existing connection was forcibly closed by the remote host."},"request_id":"tun-acc    
eptance-1785744843877","type":"ERROR"}
