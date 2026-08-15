# Navo TUN acceptance result

- Executed: 2026-08-14T08:13:12.4130268+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 183.158.168.94
- Direct IP after: 
- Result: failed
- Rollback: failed
- JSON evidence: 20260814T081256633Z-sing-box-none-none-routing-modes.json

## Error

TUN activation failed: {"payload":{"code":"TUN_ADAPTER_NOT_READY","message":"configure owned TUN routes and DNS: TUN_AD
APTER_NOT_READY: stage=ADAPTER_READY resource=Navo expected=Up 172.19.0.1/30 MTU=1500 actual=name=Navo description=sing
-tun Tunnel hardware=false status=Up index=60 guid={409DD5C4-2E6C-6FBF-B422-02692F33EE87} mtu=65535 addresses=[172.19.0
.1/30]: context deadline exceeded"},"request_id":"tun-acceptance-1786695216809","type":"ERROR"}
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:170 字符: 9
+         throw "TUN activation failed: $($Activation | ConvertTo-Json  ...
+         ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (TUN activation ..."type":"ERROR"}:String) [], RuntimeException
    + FullyQualifiedErrorId : TUN activation failed: {"payload":{"code":"TUN_ADAPTER_NOT_READY","message":"configure o 
   wned TUN routes and DNS: TUN_ADAPTER_NOT_READY: stage=ADAPTER_READY resource=Navo expected=Up 172.19.0.1/30 MTU=15  
  00 actual=name=Navo description=sing-tun Tunnel hardware=false status=Up index=60 guid={409DD5C4-2E6C-6FBF-B422-02   
 692F33EE87} mtu=65535 addresses=[172.19.0.1/30]: context deadline exceeded"},"request_id":"tun-acceptance-17866952    
16809","type":"ERROR"}
Rollback check: Network rollback mismatch for routes
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:487 字符: 13
+             throw "Network rollback mismatch for $Key"
+             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (Network rollback mismatch for routes:String) [], RuntimeException
    + FullyQualifiedErrorId : Network rollback mismatch for routes
