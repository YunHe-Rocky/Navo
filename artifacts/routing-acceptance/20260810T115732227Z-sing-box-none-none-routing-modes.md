# Navo TUN acceptance result

- Executed: 2026-08-10T11:57:50.6110170+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 211.90.237.75
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260810T115732227Z-sing-box-none-none-routing-modes.json

## Error

TUN activation failed: {"payload":{"code":"TUN_DNS_VERIFY_FAILED","message":"TUN_DNS_VERIFY_FAILED: stage=DATA_PLANE_VE
RIFIED resource=system_resolver expected=at least one address actual=addresses=0: lookup www.cloudflare.com: i/o timeou
t\nwait for TUN adapter \"Navo\" state missing (last=disabled): context deadline exceeded"},"request_id":"tun-acceptanc
e-1786363079449","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:168 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_DNS_VERIFY_FAILED","message":"TUN_DNS_VER 
   IFY_FAILED: stage=DATA_PLANE_VERIFIED resource=system_resolver expected=at least one address actual=addresses=0: l  
  ookup www.cloudflare.com: i/o timeout\nwait for TUN adapter \"Navo\" state missing (last=disabled): context deadli   
 ne exceeded"},"request_id":"tun-acceptance-1786363079449","type":"ERROR"}
