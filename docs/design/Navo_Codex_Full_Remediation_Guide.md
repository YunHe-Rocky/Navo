# Navo 全量问题修复作业指导书（交给 Codex 执行）

> 适用仓库：`YunHe-Rocky/Navo`  
> 适用分支基线：执行任务时的最新 `main`  
> 目标平台：Windows 10/11，AMD64  
> 核心：sing-box、Mihomo、Xray  
> 目标：修复当前审查发现的确定性缺陷，并关闭这些缺陷可能造成的断网、状态错乱、配置泄露、权限绕过和不可恢复风险。

---

## 0. Codex 执行原则

这是一次**运行时状态机、Windows 网络事务和安全边界整改**，不是普通的 UI 调整。

必须遵守：

1. **先检查当前代码，再修改。**  
   本文中的文件和函数来自审查时的 `main`。如果最新代码已经变化，先找到等价实现，不要机械替换。

2. **不要在一个巨型改动中同时修改所有模块。**  
   按本文阶段顺序实施。每一阶段都必须独立编译、测试、提交，再进入下一阶段。

3. **禁止静默吞错。**  
   特别是回滚、持久化、核心恢复、路由删除和数据库提交路径，不允许使用 `_ = ...` 忽略关键错误。

4. **禁止“失败后假装成功”。**  
   资源没有确认恢复时，不得设置：
   - `CommittedMode = off`
   - `RecoveryState = READY`
   - `status = success`
   - `state = stopped`

5. **禁止能力表与实现不一致。**  
   某协议、传输方式、Capture 模式没有完整实现时，必须：
   - 从能力表移除；
   - 在配置校验阶段明确拒绝；
   - UI 显示“不支持”；
   - 不得静默降级成 `direct`。

6. **禁止 UI 直接控制底层核心生命周期。**  
   用户操作必须通过统一的 Runtime/Capture 事务协调器。

7. **修改后必须补测试。**  
   不能只修改实现。每个确定性问题都必须至少有一个能够复现旧问题、验证新行为的测试。

8. **不得破坏用户数据。**  
   修改订阅、凭据、运行时状态等持久化格式时，要提供向后兼容读取或一次性迁移。

9. **不要删除现有保护措施。**  
   保留并加强：
   - Wintun SHA-256 校验；
   - Named Pipe ACL；
   - 网络事务 journal；
   - 核心原生配置校验；
   - Job Object；
   - DPAPI。

10. **完成标准不是“代码能编译”。**  
    必须满足本文最后的“总体验收标准”。

---

# 1. 先建立基线

## 1.1 新建工作分支

建议：

```powershell
git checkout main
git pull
git checkout -b fix/runtime-network-hardening
```

## 1.2 保存基线信息

执行：

```powershell
go version
go env GOOS GOARCH
go test ./...
go vet ./...

Push-Location navo_app
npm ci
npm run build
Pop-Location
```

如果基线已经失败：

- 不要为了继续而删除测试；
- 把失败信息记录到 `docs/REMEDIATION_BASELINE.md`；
- 区分“原有失败”和“本次修改引入的失败”。

## 1.3 建立修复报告

创建：

```text
docs/NAVO_REMEDIATION_REPORT.md
```

每完成一阶段，追加：

- 修改内容；
- 根因；
- 涉及文件；
- 新增测试；
- 测试结果；
- 尚未完成事项；
- 手工验证结果。

---

# 2. 第一阶段：修复当前 TUN 路由失败

优先级：**P0，第一时间完成**

涉及文件：

```text
internal/network/manager.go
internal/network/manager_test.go
internal/network/executor_windows.go
```

## 2.1 根因

当前端点旁路路由使用类似逻辑：

```powershell
$r = Find-NetRoute -RemoteIPAddress <endpoint>
New-NetRoute -InterfaceIndex $r.InterfaceIndex -NextHop $r.NextHop
```

`Find-NetRoute` 会返回多个对象，其中包含地址对象和路由对象。  
因此 `$r.InterfaceIndex` 可能变成 `System.Object[]`，无法转换为 `UInt32`。

这就是以下错误的直接原因：

```text
New-NetRoute:
无法将 System.Object[] 类型的值转换为 System.UInt32
```

## 2.2 实现要求

### A. 不允许直接使用未筛选的 `$r`

IPv4 和 IPv6 两个分支都必须修复。

最小安全实现：

```powershell
$route = Find-NetRoute -RemoteIPAddress '<endpoint>' |
    Where-Object {
        $_.PSObject.Properties.Name -contains 'NextHop'
    } |
    Select-Object -First 1

if ($null -eq $route) {
    throw 'no route to proxy endpoint'
}

$interfaceIndex = [uint32]$route.InterfaceIndex
$nextHop = [string]$route.NextHop

if ($interfaceIndex -le 0) {
    throw 'invalid proxy endpoint route interface index'
}
```

然后只能把标量 `$interfaceIndex` 和 `$nextHop` 传给 `New-NetRoute`。

### B. 在修改默认路由前解析端点物理出口

端点旁路路由需要使用系统原始出口，不应在 TUN `/1` 路由创建后再重新查询。

要求：

1. 端点物理出口解析发生在所有 Navo-owned 路由生效之前；
2. 查询结果必须排除 Navo TUN 接口；
3. 查询到的内容应包括：
   - Endpoint IP；
   - Address family；
   - InterfaceIndex；
   - NextHop；
4. 结果必须进入事务上下文或 journal，保证回滚时使用同一份已确认信息；
5. 不得在回滚期间重新猜测出口。

如果现有 `Executor` 只能返回 `error`，可以：

- 增加只用于系统查询的 `QueryExecutor`；
- 或生成单个 PowerShell 脚本，在脚本内部先解析并保存标量，再执行后续命令；
- 不要通过解析本地化文本结果获取接口索引。

### C. 端点路由幂等

重复激活时：

- 已存在完全相同的 Navo-owned `/32` 或 `/128` 路由，应视为成功；
- 已存在冲突路由时，不得直接覆盖非 Navo 路由；
- 删除时只删除本次事务拥有的精确路由。

## 2.3 PowerShell 输出编码

`internal/network/executor_windows.go` 不能把 Windows PowerShell 5.1 输出直接假定为 UTF-8。

要求在脚本前统一设置：

```powershell
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
$OutputEncoding = [System.Text.UTF8Encoding]::new()
```

Go 端仍要保留合理的解码兜底，避免中文错误变成乱码。

## 2.4 测试要求

新增或更新测试：

### 单元测试

验证生成命令：

- 包含路由对象筛选；
- 包含 `Select-Object -First 1`；
- 包含 `[uint32]` 标量转换；
- 不再出现未筛选的 `$r.InterfaceIndex`；
- IPv4、IPv6 均覆盖。

### Windows 集成测试

建议新增：

```text
internal/network/manager_windows_integration_test.go
```

使用环境变量控制：

```text
NAVO_RUN_WINDOWS_INTEGRATION=1
```

至少测试：

1. `Find-NetRoute` 返回两个以上对象；
2. Wi-Fi 与有线同时存在；
3. 存在 VPN/虚拟网卡；
4. 返回的 InterfaceIndex 是单个整数；
5. 添加和删除端点旁路路由后路由表恢复；
6. 测试失败时执行清理。

---

# 3. 第二阶段：建立唯一的运行时事务入口

优先级：**P0**

当前问题不是单个函数，而是核心、节点、系统代理、TUN 分别存在自己的修改入口。

涉及文件重点：

```text
internal/service/capture_transition.go
internal/service/service.go
internal/agent/capture_transition.go
internal/agent/agent.go
internal/supervisor/supervisor.go
navo_app/app.go
```

## 3.1 目标架构

创建统一运行时目标模型，例如：

```go
type DesiredRuntime struct {
    Mode               capture.Mode
    CoreID             core.Type
    SelectedOutboundID string
    TUNName            string
    TUNMTU             int
}
```

创建唯一协调器，例如：

```go
type RuntimeTransitionCoordinator struct {
    // dependencies
}

func (c *RuntimeTransitionCoordinator) Apply(
    ctx context.Context,
    desired DesiredRuntime,
) (CommittedRuntime, error)
```

所有会影响网络数据面的操作必须进入此协调器：

- Capture 模式切换；
- 开启/关闭系统代理；
- 开启/关闭 TUN；
- 切换节点；
- 切换核心；
- 更新当前节点；
- 删除当前节点；
- 刷新订阅导致当前节点参数变化；
- 修改 TUN 名称、MTU；
- 恢复网络。

## 3.2 统一事务顺序

推荐顺序：

```text
1. 获取全局 transition lock
2. 生成 transition ID
3. 读取并保存当前 committed runtime
4. 构造候选配置，不修改当前 committed 状态
5. 校验候选配置
6. 解析新代理端点的物理出口
7. 标记 transition pending
8. 停止接受新的 Capture 修改
9. 停止旧核心
10. 回滚旧系统代理/TUN owned resources
11. 编译新核心配置
12. 启动新核心
13. 等待本地端口和控制器就绪
14. 如果是 TUN：
    - 等待适配器 Ready
    - 应用端点旁路路由
    - 应用 owned split routes
    - 应用 DNS/NRPT/IPv6 策略
15. 如果是系统代理：
    - 事务性启用系统代理
16. 执行真实数据面探测
17. 提交 runtime 状态
18. 提交 transition journal
19. 释放锁
```

任何一步失败：

```text
1. 保留原始错误
2. 使用独立 rollback context
3. 回滚所有已应用资源
4. 恢复旧配置和旧核心
5. 验证旧数据面
6. 只有确认恢复成功，才恢复旧 committed 状态
7. 回滚不完整时进入 FAULTED/DIRTY
```

## 3.3 UI 禁止直接调用核心启动停止

当前 `navo_app/app.go` 中的：

```go
SetCoreRunning(true/false)
```

不能直接发送：

```text
core.start
core.stop
```

修改为用户语义操作：

- 关闭：`capture.set mode=off`
- 开启：恢复用户上次选择的 Capture 模式，或明确选择 `system_proxy/tun`

公共 UI API 不得暴露裸 `core.start/core.stop/core.restart`。

Service 内部仍可保留私有核心生命周期方法，但不得作为普通 UI 命令使用。

## 3.4 节点切换必须更新 TUN 旁路

在 TUN 已提交状态下，切换节点必须：

1. 解析新节点全部有效 IP；
2. 创建新端点旁路；
3. 切换核心配置；
4. 验证新节点真实代理可用；
5. 删除旧旁路；
6. 提交 selected outbound。

不能只调用核心 `SwapConfig()`。

域名节点还要处理 DNS 轮转：

- 保存解析时间和 TTL 能力；
- 在节点连接失败且域名重新解析发生变化时，触发受控旁路更新；
- 不允许后台直接修改路由，必须进入 transition lock。

## 3.5 核心切换必须进入同一事务

`core.select` 不得独立停止和启动 Supervisor。

必须通过统一目标：

```go
desired.CoreID = newCoreID
```

然后由 Runtime Transition Coordinator：

- 检查核心真实能力；
- 编译该核心配置；
- 执行原生 `check/test`；
- 切换核心；
- 重建 TUN 旁路；
- 验证数据面；
- 提交或回滚。

## 3.6 状态源唯一

建议：

- Service 保存系统真实 `CommittedRuntime`，是权威状态；
- Agent 保存面向 UI 的 transition snapshot；
- Agent 不得在没有 Service 验证结果时自行宣告资源已关闭；
- UI 只显示 Service committed state 和 transition state的组合结果。

---

# 4. 第三阶段：修复回滚和恢复语义

优先级：**P0**

涉及：

```text
internal/service/capture_transition.go
internal/agent/capture_transition.go
internal/network/manager.go
internal/network/journal.go
internal/network/reconciler.go
internal/network/tun/route_windows.go
internal/network/tun/dns_windows.go
```

## 4.1 回滚失败不得清空 Manager 引用

当前类似逻辑：

```go
err := s.networkManager.Deactivate(ctx)
s.networkManager = nil
```

必须改成：

```go
err := s.networkManager.Deactivate(ctx)
if err == nil {
    s.networkManager = nil
} else {
    // 保留引用和 journal，标记 dirty
}
```

如果进程即将退出，也必须确保 journal 保留足够信息供下次恢复。

## 4.2 回滚失败不得设置 off

以下任意一项未确认恢复时：

- 系统代理；
- TUN `/1` 路由；
- 端点旁路；
- DNS/NRPT；
- IPv6 firewall；
- 核心进程；
- Wintun 适配器；

状态必须是：

```text
State = faulted
NetworkDirty = true
CommittedMode = previous mode 或 unknown
RequiresRepair = true
```

不能是：

```text
CommittedMode = off
```

## 4.3 合并两套恢复体系

当前有：

- `network.Manager` journal；
- `network.Reconciler` recovery state；
- 旧 `routeManager/dnsManager` 的本地化文本清理。

要求：

1. 以结构化 network journal 为主要恢复依据；
2. Reconciler 只负责发现脏状态并调用 journal-based Recover；
3. 不再依赖 `route print` 或本地语言 `netsh` 输出搜索 `"Navo"`；
4. 删除或隔离旧 RouteManager，避免它被当成可靠恢复路径；
5. 如果 journal 损坏：
   - 不得假定上次正常退出；
   - 标记 `FAULTED`；
   - 保留文件并生成诊断副本；
   - 只执行安全、精确、可验证的恢复动作。

## 4.4 READY 条件

只有满足以下条件才写 `READY`：

```text
IssuesUnfixed == 0
network journal 已清除或 committed
系统代理 ownership 已清除
Navo-owned 路由为 0
Navo-owned DNS/NRPT 为 0
核心不存在或处于目标状态
适配器处于目标状态
```

否则保持：

```text
DIRTY 或 FAULTED
```

## 4.5 删除假清理逻辑

`cleanupStaleFiles()` 不能只打印：

```text
would clean stale file
```

要求：

- 只删除明确属于 Navo 的临时文件模式；
- 不能删除未知文件；
- 每次删除记录结果；
- 删除失败进入 `IssuesUnfixed`；
- 增加单元测试。

## 4.6 修复 IPv6 与 DNS 旧实现

如果旧实现仍被使用：

### IPv6

IPv6 路由必须使用：

```text
netsh interface ipv6 ...
```

不能固定使用 IPv4。

### 多 DNS

第一个 DNS：

```text
set dns
```

后续 DNS：

```text
add dns index=2
add dns index=3
```

关闭时恢复原状态，而不是简单 `delete dns all`。

更推荐使用 PowerShell/CIM 或 Windows API，以结构化对象操作，避免本地化文本解析。

---

# 5. 第四阶段：删除 Host/Supervisor 双重重启

优先级：**P0**

涉及：

```text
internal/host/singbox.go
internal/supervisor/supervisor.go
internal/host/interface.go（如存在）
```

## 5.1 唯一责任

### Host 只负责

- 启动进程；
- 停止进程；
- 回收进程；
- 保存当前 PID；
- 保存退出错误；
- 更新 Host 状态；
- 提供退出通知。

### Supervisor 负责

- 状态机；
- 崩溃重启；
- 退避；
- 最大次数；
- restart suppression；
- 与 Capture transition 协调。

Host 不得自行重启。

## 5.2 修改 Host monitor

当前 Host monitor 中的自动重启代码必须删除。

进程意外退出时只做：

```go
h.status.State = HostStateFailed
h.status.LastError = ...
```

并通知 Supervisor。

## 5.3 Supervisor monitor 生命周期

不得使用一次 IPC 请求的 context 作为长期监控 context。

创建 Supervisor 自有 lifecycle context：

```go
lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
```

它只能在：

- Supervisor.Stop；
- Supervisor 被替换；
- Service shutdown；

时取消。

请求 context 只限制启动过程，不限制后续长期监控。

## 5.4 重启预算

- 用户主动 Start：重置连续崩溃预算；
- 内部崩溃重启：不重置；
- 运行稳定达到设定时长后，可重置连续崩溃计数；
- Capture transition 设置 suppression 后，不允许任何层自动重启；
- 达到最大次数进入 `FAILED`，不能无限循环。

## 5.5 测试

覆盖：

1. 核心启动后请求 context 结束，核心仍运行；
2. 核心崩溃只由 Supervisor 重启一次；
3. suppression 期间不重启；
4. 连续失败达到上限；
5. 正常 Stop 不触发崩溃重启；
6. Capture 回滚期间 Host 不会偷偷启动。

---

# 6. 第五阶段：关闭 Service 权限绕过

优先级：**P0 安全**

涉及：

```text
internal/service/service.go
internal/pipe/pipe_windows.go
cmd/navo/main.go
cmd/navo-svc/*（如存在）
```

## 6.1 单进程正式版不监听 Service Pipe

为 `service.Config` 增加显式选项，例如：

```go
EnableExternalPipe bool
```

规则：

- `cmd/navo` 单进程版本：`false`
- 独立 Service 开发/部署版本：`true`
- 测试必须显式设置，不能隐式猜测

`Service.Run()` 只有在该配置为 true 时才能启动：

```text
Navo.Agent.Service.v1
```

单进程中调用路径为：

```text
UI Pipe → Agent → svc.Dispatch()
```

## 6.2 删除外部 `config_path`

`core.start`、`core.restart` 等外部消息不得接受文件路径。

公开接口只能接受：

- 受控 runtime 目标；
- 核心 ID；
- revision ID；

配置路径由 Service 内部从受控目录解析。

如必须校验路径：

1. `filepath.Abs`；
2. `filepath.Clean`；
3. 确认位于 `ConfigDir`；
4. 拒绝 symlink/junction 逃逸；
5. 文件必须由 Navo revision repository 登记；
6. 运行前再次原生校验。

最佳方案仍是：外部完全不传路径。

## 6.3 Pipe 身份与会话

独立 Service 模式还要加强：

- Named Pipe ACL 保留 SYSTEM、Administrators、目标用户；
- 校验客户端进程 SID；
- 每次启动生成随机会话令牌；
- Agent 首次握手获得令牌；
- 高权限方法必须携带令牌；
- 限制消息速率和并发；
- `service.shutdown` 不应对普通 UI 调用开放。

## 6.4 测试

- 单进程版不存在 Service Pipe；
- UI Pipe 正常；
- 未授权客户端无法调用高权限方法；
- 传入任意 `config_path` 被拒绝；
- 正常 Agent 事务仍可工作。

---

# 7. 第六阶段：重写系统代理事务

优先级：**P0/P1**

涉及：

```text
internal/agent/systemproxy/proxy.go
internal/agent/systemproxy/proxy_windows.go
```

## 7.1 必须保存完整 WinINet 状态

快照至少包含：

```text
ProxyEnable
ProxyServer
ProxyOverride
AutoConfigURL
AutoDetect
```

必须区分：

- 注册表值不存在；
- 值存在但为空；
- 值存在且有内容。

不能只用空字符串表示所有情况。

建议字段包含 `Present bool`。

## 7.2 读取失败必须终止启用

删除以下危险行为：

```go
if getProxy fails {
    backup empty config
}
```

读取失败时：

```text
系统代理启用失败
原配置保持不变
不创建 committed ownership
```

## 7.3 Ownership 两阶段提交

启用流程：

```text
1. 读取完整原始配置
2. 原子写 backup
3. 原子写 pending ownership
4. 写入 Navo 目标配置
5. 清理或禁用冲突 PAC/AutoDetect
6. 调用 WinINet refresh
7. 回读并验证
8. ownership 标记 committed
```

任一步失败：

- 使用 backup 恢复；
- 验证恢复；
- 失败则保留 dirty ownership。

## 7.4 恢复完整状态

恢复时：

- 原值不存在：删除当前注册表值；
- 原值为空：写回空值；
- 原值存在：写回原值；
- 恢复 AutoDetect；
- 恢复 PAC；
- 恢复 bypass；
- 恢复 enable 状态。

不能仅在字段非空时才写回。

## 7.5 使用正确 WinINet 刷新

实现：

```text
InternetSetOptionW(
    NULL,
    INTERNET_OPTION_SETTINGS_CHANGED,
    NULL,
    0
)

InternetSetOptionW(
    NULL,
    INTERNET_OPTION_REFRESH,
    NULL,
    0
)
```

`WM_SETTINGCHANGE("Environment")` 不能作为主刷新方式。

## 7.6 动态注册表缓冲区

`RegQueryValueExW` 应：

1. 先查询长度；
2. 按长度分配；
3. 再读取；
4. 校验类型。

不允许固定 4096 字节后静默忽略错误。

## 7.7 外部代理变更保护

如果启用 Navo 后，其他程序修改了系统代理：

- Navo Disable 不得覆盖更新后的外部设置；
- 清理自己的 ownership；
- 向用户提示“代理设置已被其他程序修改，未强制覆盖”。

现有 ownership 思路可以保留，但要扩展为完整配置哈希，而不是只比较 `ProxyServer`。

---

# 8. 第七阶段：修复内核完整性与进程安全

优先级：**P0/P1**

涉及：

```text
cmd/navo/main.go
internal/service/service.go
internal/host/singbox.go
internal/coremanifest/*
```

## 8.1 不得重新搜索已验证核心

`cmd/navo` 已经完成 manifest 和 SHA-256 校验后，Service 必须直接使用传入路径：

```go
binaryPath := cfg.SingBoxPath
```

生产模式禁止再调用 `host.FindBinary()` 覆盖路径。

开发模式如需自定义核心：

- 使用显式命令行参数；
- 显示开发模式警告；
- 不允许默认读取 `NAVO_SINGBOX_PATH`；
- 或自定义核心也必须提供 hash。

## 8.2 Job Object 失败即停止

既然 Job Object 是防止核心和 UI 残留的强制条件，初始化或 Assign 失败时必须终止启动。

不要只打印 warning 后继续。

## 8.3 单实例改用 Named Mutex

删除 `%TEMP%\navo.lock` 作为主互斥机制。

使用：

```text
Local\Navo.<CurrentUserSID>
```

创建 Windows Named Mutex：

- 已存在：唤醒现有窗口并退出；
- 未存在：持有到进程退出；
- 不受 PID 复用和文件竞争影响。

锁文件可保留为诊断信息，但不能作为互斥依据。

## 8.4 禁止按名字批量 taskkill

`cleanupZombies()` 不能仅根据进程名杀：

```text
navo-svc.exe
navo-agent.exe
```

必须验证：

- 可执行文件完整路径；
- ownership/session ID；
- 父进程或 Job Object；
- PID 记录与创建时间。

无法确认属于当前 Navo 安装时，不得终止。

---

# 9. 第八阶段：多内核配置正确性

优先级：**P1**

涉及：

```text
internal/compiler/multi_core.go
internal/compiler/generator.go
internal/compiler/validator.go
internal/coreadapter/adapters.go
```

## 9.1 禁止静默忽略规则

对每个内核：

- 能表达的规则必须准确转换；
- 不能表达的规则必须在 Compile 阶段返回明确错误；
- 不得直接丢弃。

## 9.2 Xray 路由

`GenerateXray()` 必须转换 `cfg.RoutingRules`。

至少支持并测试：

```text
domain
domain_suffix
domain_keyword
ip_cidr
process_name（仅在 Xray 当前平台配置确实支持时）
```

规则顺序必须保持。

不支持的：

```text
domain_regex
geosite
geoip
port_range
```

要么实现，要么明确拒绝。

不能生成“只有 final outbound 的单条规则”后宣称 rule 模式可用。

## 9.3 Xray HTTPS 上游

HTTP 与 HTTPS 上游必须区别：

- HTTP：普通 HTTP proxy；
- HTTPS：到代理服务器的 TLS 配置完整，包含 SNI/证书策略。

不要在构建 stream settings 前提前 return。

## 9.4 Mihomo transport

根据 `Network` 分别生成：

```text
ws       → ws-opts
grpc     → grpc-opts
h2       → h2-opts
http     → http-opts
tcp      → 不生成错误的 ws-opts
```

未知 network 必须报错。

## 9.5 WireGuard

当前模型字段不完整时，立即采用安全方案：

```text
从 sing-box、Mihomo、Xray 能力表移除 WireGuard
Validator 明确拒绝
UI 标记未支持
```

只有补齐并验证以下字段后才能重新启用：

- private key；
- peer public key；
- local address；
- allowed IP；
- reserved；
- MTU；
- endpoint；
- DNS/route 行为。

不能只写 server/port 就标记支持。

## 9.6 能力表一致性测试

新增表驱动测试：

```text
每个 Core × 每个 Protocol × 每个 Capture Mode
```

验证：

- `Capabilities()`；
- `Compatible()`；
- 编译器实际结果；

三者必须一致。

## 9.7 原生校验门禁

生成每个核心配置后必须执行：

```text
sing-box check
mihomo -t
xray run -test
```

原生校验失败不得提交 runtime。

---

# 10. 第九阶段：加强 Config Validator

优先级：**P1**

涉及：

```text
internal/compiler/validator.go
```

必须增加：

## 10.1 ID 与引用完整性

- Outbound ID 非空且唯一；
- Inbound Tag 唯一；
- Rule ID 唯一；
- DNS Tag 唯一；
- `FinalOutbound` 必须存在或为内置 direct/block；
- Rule Outbound 必须存在；
- DNS Final 必须引用存在的 DNS server。

## 10.2 网络字段

- 端口必须 `1..65535`，不允许 0；
- IP/CIDR 必须真实解析；
- domain 不允许空白和非法控制字符；
- TUN MTU 与平台范围一致；
- TUN 地址、网关、IPv4/IPv6 模式一致。

## 10.3 协议字段

按协议检查：

- VMess/VLESS UUID；
- Trojan password；
- SS method/password；
- Reality public key、short ID、SNI；
- WS path/host；
- gRPC service name；
- TLS 与 SNI；
- HTTP/SOCKS authentication；
- Hysteria2/TUIC 必需字段。

## 10.4 规则 Value

按类型解析：

- IP rule 必须 CIDR；
- port 必须整数；
- port range 起止有效；
- process name 不允许路径注入字符；
- domain regex 编译检查；
- 空 value 拒绝。

## 10.5 测试

每条新增校验至少一个：

- 正常 case；
- 错误 case；
- 边界 case。

---

# 11. 第十阶段：订阅安全与解析整改

优先级：**P1**

涉及：

```text
internal/subscription/fetcher.go
internal/subscription/parser/clash.go
internal/subscription/normalizer.go
internal/subscription/subscription.go
```

## 11.1 添加阶段只允许 HTTPS

`AddWithOptions` 与 Fetcher 规则必须一致：

```text
仅允许 https://
```

HTTP 直接在添加时返回错误。

## 11.2 防止 DNS 重绑定 SSRF

实现安全 DialContext：

1. 解析目标 hostname；
2. 获取所有 IP；
3. 拒绝：
   - loopback；
   - private；
   - link-local；
   - multicast；
   - unspecified；
   - IPv6 ULA；
   - CGNAT；
   - 云 metadata 地址；
   - 保留/基准测试地址；
4. 从已验证 IP 中选择并固定拨号；
5. TLS ServerName 仍使用原 hostname；
6. Redirect 每一跳都重新验证；
7. 不得校验域名后又交回默认 Dialer 重新解析。

## 11.3 Clash 改为真实 YAML 解析

使用项目已有的 `gopkg.in/yaml.v3`。

定义结构体或可靠的中间模型，支持至少：

- ss；
- vmess；
- vless；
- trojan；
- hysteria2；
- tuic；
- socks5；
- http；
- ws-opts；
- grpc-opts；
- reality-opts；
- client-fingerprint；
- skip-cert-verify；
- SNI/servername；
- nested headers。

不能继续使用逐行字符串猜测作为主解析器。

解析错误必须带节点名称和字段路径，但不得泄露密码。

## 11.4 节点去重指纹

不能只用：

```text
server + port + protocol
```

创建 canonical connectivity fingerprint，至少包含：

```text
provider
protocol
server
port
credential identity hash
UUID
method
network
path
host
SNI
TLS
Reality public key
Reality short ID
service name
```

敏感值参与 hash，但不得明文写日志。

## 11.5 Parser 输出必须再校验

每个解析出的 Outbound：

```text
Parser → Normalize → Validator
```

无效节点：

- 不进入 active outbounds；
- 记录 provider-scoped error；
- 不使整个订阅已存在的可用节点丢失。

## 11.6 损坏状态文件

订阅 metadata 解码失败：

1. 不得当空状态继续；
2. 将原文件复制为：
   `subscriptions.json.corrupt.<timestamp>`;
3. 返回明确 load error；
4. 禁止后续自动保存覆盖；
5. UI 提供恢复/重建提示。

## 11.7 删除与刷新必须事务化

删除节点、订阅或更新当前节点时：

```text
候选数据
→ 编译
→ 原生校验
→ 应用 runtime transition
→ 持久化
→ 删除旧凭据
→ 返回成功
```

不允许：

```text
先返回成功
→ 后台异步 apply
```

如保留异步刷新，必须有：

- task ID；
- 状态；
- 最终成功/失败事件；
- 错误可见；
- 不用于破坏性操作。

---

# 12. 第十一阶段：安全持久化与 DPAPI

优先级：**P1**

涉及：

```text
internal/credential/file_store.go
internal/subscription/subscription.go
internal/upstreamproxy/manager.go
internal/network/journal.go
internal/securestore/protect_windows.go
runtime state 相关文件
```

## 12.1 建立统一原子文件写入工具

新增类似：

```text
internal/fsatomic
```

要求：

1. 同目录创建临时文件；
2. 写入；
3. Flush；
4. Close；
5. 使用 Windows `ReplaceFileW` 或 `MoveFileExW`：
   - replace existing；
   - write-through；
6. 原文件不能先删除；
7. 可选保留 `.bak`；
8. 失败时原文件必须仍可读取。

所有关键状态统一使用，不要每个模块自己实现一套。

## 12.2 测试断电窗口

通过注入替换函数或 failpoint 模拟：

- 写临时文件失败；
- Flush 失败；
- Replace 失败；
- 进程在替换前终止。

验证原文件不丢失。

## 12.3 Windows ACL

不能只依赖 `0600/0700`。

对数据目录和敏感文件设置 DACL：

- 当前用户 SID；
- SYSTEM；
- 必要时 Administrators；
- 移除普通 Users 的继承访问。

涉及：

- credentials；
- subscription endpoint cache；
- AI API key；
- runtime config；
- network journal；
- proxy backup。

## 12.4 DPAPI Scope

单进程桌面模式优先使用 Current User Scope。

只有独立 SYSTEM Service 确实需要跨用户读取时，才使用 Local Machine Scope，并必须配合严格 ACL。

把 scope 作为显式配置，不要写死。

---

# 13. 第十二阶段：IPC 稳定性

优先级：**P1/P2**

涉及：

```text
internal/pipe/pipe.go
internal/pipe/pipe_windows.go
```

## 13.1 完整写入

实现 `writeFull`：

```go
func writeFull(w io.Writer, data []byte) error {
    for len(data) > 0 {
        n, err := w.Write(data)
        if err != nil {
            return err
        }
        if n <= 0 {
            return io.ErrShortWrite
        }
        data = data[n:]
    }
    return nil
}
```

Header 和 Payload 都使用完整写入。

## 13.2 Overlapped I/O 生命周期

超时后：

1. `CancelIoEx`；
2. 等待 overlapped 完成；
3. 调用 `GetOverlappedResult` 获取最终状态；
4. 只有确认完成后才能释放 event/overlapped 内存；
5. 关闭连接时取消全部 pending I/O。

## 13.3 请求限制

增加：

- 每连接最大并发；
- 方法白名单；
- payload 大小限制；
- 长耗时操作 task 化；
- 高权限方法额外授权；
- 错误响应不包含敏感凭据。

## 13.4 测试

- 人为 short writer；
- 超时读写；
- 连接中断；
- 10MB 边界；
- 多客户端；
- listener close 时无 goroutine 泄露。

---

# 14. 第十三阶段：前后端契约与诊断

优先级：**P2**

涉及：

```text
navo_app/app.go
internal/agent/dashboard.go
internal/service/service.go
internal/ipdetect/echo.go
internal/monitor/*
```

## 14.1 修复 uptime 字段

统一为一个 JSON 名称，例如：

```text
uptime_seconds
```

前端结构体和后端完全一致。

增加 contract test，禁止字段漂移。

## 14.2 Dashboard IP 状态

不要把 `direct_ip` 永久写成空字符串。

保存并返回：

- direct/source IP；
- proxy IP；
- 各自错误；
- checked_at；
- 是否为缓存结果。

节点或 Capture 模式切换后应清除旧代理 IP 缓存。

## 14.3 节点测试必须测试代理能力

TCP connect 只能作为预检查。

完整测试至少应：

1. 生成只包含目标节点的临时受控配置；
2. 启动隔离测试核心或使用核心 URLTest 能力；
3. 通过该节点发起 HTTPS 请求；
4. 验证状态码、出口 IP或固定响应；
5. 有总超时和并发限制。

UI 文案要区分：

```text
端口可达
代理握手成功
真实代理可用
```

## 14.4 Geo 查询必须 HTTPS

替换明文：

```text
http://ip-api.com
```

使用 HTTPS 服务或本地 GeoIP 数据库。

限制响应体大小并校验返回 IP 与请求 IP 一致。

## 14.5 AI 隐私与响应限制

AI 请求：

- 使用请求 context；
- 限制响应体大小；
- API 返回非 2xx 时先处理状态码；
- 不在错误中回显完整响应；
- 规则结果执行 Validator；
- 第三方 AI 诊断前明确说明会发送哪些字段；
- 默认脱敏节点服务器和错误中的凭据。

`AI diagnose` 如果只使用本地 QuickAnalyze，UI 不应标记为远程 AI。

---

# 15. 第十四阶段：测试与发布门禁

优先级：**必须在发布前完成**

涉及：

```text
scripts/test.ps1
scripts/package.ps1
scripts/smoke.ps1
Makefile
.github/workflows/*
navo_app/package.json
```

## 15.1 修复 Smoke 状态预期

当前 `DeferCoreStart: true` 时，启动后核心应为 stopped/disconnected。

Smoke 测试必须先确认：

```text
启动后无 Capture
无 TUN
无系统代理
核心未错误运行
```

然后显式执行：

```text
capture.set system_proxy
验证核心与 HTTP 数据面
capture.set off
capture.set tun
验证 TUN 与数据面
capture.set off
```

不能仍假设启动后核心自动 running。

## 15.2 正式 package 强制测试

`package.ps1` 在生成正式目录前必须执行：

```powershell
go test ./...
go vet ./...

Push-Location navo_app
npm ci
npm run typecheck
npm run test
npm run build
Pop-Location
```

正式发布流程还要执行 smoke。

任何一步失败：

```text
禁止生成正式发布目录
禁止生成 SHA256SUMS
返回非 0
```

## 15.3 前端测试

引入前端测试框架并覆盖：

- API JSON contract；
- uptime；
- Capture 状态展示；
- faulted/dirty 状态；
- 核心按钮不直接调用 core.stop；
- TUN 切换错误提示；
- 异步任务最终失败提示。

## 15.4 Windows 集成矩阵

至少人工或自动验证：

```text
Windows 10
Windows 11
中文系统
英文系统
Wi-Fi only
Ethernet only
Wi-Fi + Ethernet
Hyper-V
WSL2
Docker Desktop
其他 VPN/TUN 软件共存
IPv6 enabled
IPv6 disabled
多条默认路由
代理节点为域名
代理节点 DNS 轮转
系统原有 PAC
系统原有手工代理
手动禁用 Navo 网卡
核心崩溃
强制结束 navo.exe
断电/重启恢复
```

## 15.5 GitHub CI

增加 Windows CI：

- Go test；
- Go vet；
- UI typecheck/test/build；
- 配置生成测试；
- 不需要管理员权限的 smoke 子集。

管理员 TUN 集成测试可放在自托管 Windows runner 或发布前人工门禁。

---

# 16. 建议提交顺序

不要一次性提交。

建议：

```text
1. fix(network): select scalar endpoint route and normalize PowerShell output
2. refactor(runtime): add unified runtime transition coordinator
3. fix(recovery): preserve dirty state on rollback failure
4. refactor(supervisor): make supervisor the only crash restart owner
5. security(service): disable external service pipe in combined mode
6. security(core): remove untrusted binary path override
7. fix(systemproxy): transactional full WinINet snapshot and restore
8. fix(compiler): align multi-core capabilities with actual generation
9. fix(subscription): safe HTTPS fetch and YAML parser
10. fix(storage): atomic replacement, ACL and DPAPI scope
11. fix(ipc): full writes and overlapped cancellation
12. test(release): enforce tests and update Windows smoke flow
```

每次提交都要：

```text
代码
测试
报告更新
```

---

# 17. 禁止做的“伪修复”

Codex 不得采用以下方式应付：

1. 在 `$r.InterfaceIndex` 后简单加 `[0]`，但不确认第一个对象是不是路由；
2. 捕获 PowerShell 错误后继续启动；
3. 回滚失败仍清 journal；
4. 把 `CommittedMode` 强行设为 off；
5. 通过增大 timeout 掩盖状态机错误；
6. 通过删除测试让构建通过；
7. 把不支持协议降级成 direct；
8. 节点切换时只重启核心，不更新旁路路由；
9. 继续使用逐行 Clash YAML 解析；
10. 继续先删除正式文件再 Rename；
11. 继续让 UI 调用裸 `core.stop`；
12. 继续同时保留 Host 和 Supervisor 两套自动重启；
13. 仅检查端口就标记节点真实可用；
14. 只打印恢复失败日志，但向 UI 返回成功；
15. 使用 `taskkill /im` 清理无法确认归属的进程。

---

# 18. 每阶段验收模板

在 `docs/NAVO_REMEDIATION_REPORT.md` 中使用：

```markdown
## 阶段 X：名称

### 根因
...

### 修改文件
- `path/file.go`
- ...

### 核心实现
...

### 新增测试
- `Test...`
- ...

### 自动测试结果
```text
go test ./...
PASS
```

### Windows 手工验证
- [x] ...
- [ ] ...

### 数据兼容
...

### 残余风险
...
```

---

# 19. 总体验收标准

全部满足后才能声明整改完成。

## 网络事务

- [ ] 多默认路由环境下 TUN 可正常开启；
- [ ] 不再出现 `System.Object[] → UInt32`；
- [ ] TUN 切节点后旁路路由立即更新；
- [ ] TUN 切核心不发生断网和路由残留；
- [ ] 关闭 Navo 后 owned route/DNS/firewall 为 0；
- [ ] 回滚失败明确进入 FAULTED，不伪装 off；
- [ ] 强制结束进程后可自动恢复。

## 核心生命周期

- [ ] 只有 Supervisor 负责崩溃重启；
- [ ] Capture transition 期间不会自动抢跑重启；
- [ ] 请求 context 结束不影响长期运行核心；
- [ ] 达到最大重启次数后稳定进入 FAILED；
- [ ] 正常停止不会触发重启。

## 系统代理

- [ ] 原有 PAC、AutoDetect、ProxyOverride 完整恢复；
- [ ] 原配置读取失败时不会覆盖用户设置；
- [ ] 外部程序修改代理后 Navo 不强制覆盖；
- [ ] 使用 WinINet refresh；
- [ ] 核心停止前代理捕获先进入受控事务。

## 权限和安全

- [ ] 单进程版没有可绕过 Agent 的 Service Pipe；
- [ ] 外部调用不能传任意 config path；
- [ ] 最终运行核心就是 manifest 校验过的文件；
- [ ] Job Object 失败时启动失败；
- [ ] 使用 Named Mutex；
- [ ] 敏感文件拥有明确 Windows ACL；
- [ ] 订阅不存在 DNS 重绑定 SSRF。

## 多内核

- [ ] 能力表、Compatible、Compiler 三者一致；
- [ ] Xray rule 模式真正应用规则，或明确禁用；
- [ ] Mihomo 不再把 gRPC 等错误生成为 ws-opts；
- [ ] WireGuard 完整实现或从能力表移除；
- [ ] 每个生成配置通过核心原生检查。

## 数据

- [ ] 所有关键文件使用安全原子替换；
- [ ] 替换失败时旧数据仍存在；
- [ ] 损坏订阅文件不会被空状态覆盖；
- [ ] 删除节点/订阅失败不会先丢凭据；
- [ ] 本地与数据库状态有一致性处理。

## 测试发布

- [ ] `go test ./...`；
- [ ] `go vet ./...`；
- [ ] 前端 typecheck；
- [ ] 前端测试；
- [ ] 前端 build；
- [ ] Windows smoke；
- [ ] Windows TUN 集成测试；
- [ ] `package.ps1` 强制执行测试；
- [ ] 所有失败均阻止发布。

---

# 20. Codex 最终输出要求

完成后，Codex 必须给出：

1. 修改文件清单；
2. 每项问题对应的修复说明；
3. 未修复问题和原因；
4. 所有测试命令及结果；
5. Windows 手工验证结果；
6. 数据迁移说明；
7. 安全边界变化；
8. 已知限制；
9. 建议发布版本号；
10. 是否达到本文总体验收标准。

没有测试证据的项目不得标记为“已修复”。

---

## 最终目标

修复完成后，Navo 应满足以下核心原则：

```text
用户只表达目标状态
        ↓
统一 Runtime Transition 生成候选状态
        ↓
核心、TUN、路由、DNS、系统代理作为一个事务修改
        ↓
真实数据面验证成功
        ↓
提交 committed state
```

失败时：

```text
保留原始错误
        ↓
回滚所有已应用资源
        ↓
验证旧状态已恢复
        ↓
恢复成功：回到旧 committed state
恢复失败：进入 FAULTED/DIRTY，保留 journal，提示修复
```

禁止再出现：

```text
界面显示已关闭，但系统仍被接管
核心已经停止，但系统代理仍开启
节点已经切换，但路由仍绕过旧节点
回滚失败，却把状态写成 READY
能力表显示支持，实际生成错误配置
```
