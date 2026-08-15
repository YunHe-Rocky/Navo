# Navo TUN acceptance result

- Executed: 2026-08-10T11:11:09.3502183+00:00
- Windows: Microsoft Windows NT 10.0.26200.0
- Core: sing-box 1.13.14
- Failure point: none
- Crash point: none
- Direct IP before: 211.90.237.75
- Direct IP after: 
- Result: failed
- Rollback: passed_after_failure
- JSON evidence: 20260810T111053774Z-sing-box-none-none-routing-modes.json

## Error

google failed after 3 attempts: 使用“0”个参数调用“GetResult”时发生异常:“已取消一个任务。”
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:275 字符: 17
+                 $Response = $Client.GetAsync(
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : NotSpecified: (:) [], MethodInvocationException
    + FullyQualifiedErrorId : TaskCanceledException
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:300 字符: 13
+             throw "$($Probe.Name) failed after 3 attempts: $LastError ...
+             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : OperationStopped: (google failed a...nceledException:String) [], RuntimeException
    + FullyQualifiedErrorId : google failed after 3 attempts: 使用“0”个参数调用“GetResult”时发生异常:“已取消一个任务。”
所在位置 D:\WorkSpace\Navo\scripts\test-tun-elevated.ps1:275 字符: 17
+                 $Response = $Client.GetAsync(
+                 ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : NotSpecified: (:) [], MethodInvocationException
    + FullyQualifiedErrorId : TaskCanceledException
