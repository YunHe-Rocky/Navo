# Navo TUN 一次性闭环修复作业指导书（Codex 强制执行版）

> 适用仓库：`YunHe-Rocky/Navo`  
> 适用分支：执行时最新 `main`  
> 当前基线参考：`73504ee` 及其后续提交  
> 执行主体：Codex / Claude Code / 同类编码智能体  
> 目标平台：Windows 10/11 x64  
> 文档目标：在**保留 Navo 自主管理 TUN 路由、DNS、IPv6 和恢复事务**的前提下，把 TUN 从“命令执行过”修成“真实数据面可用、失败必回滚、崩溃可恢复”的完整功能。

---

## 0. 执行命令

你不是来继续分析、补充设计文档或只修复当前错误信息的。

你必须：

1. 阅读本文件列出的全部生产代码和测试代码；
2. 在当前代码基础上完成实现；
3. 不改变 Navo 的产品方向；
4. 不把 TUN 路由责任重新交给 sing-box 或 Mihomo；
5. 不允许只修改一处 PowerShell；
6. 不允许只添加 Mock 单元测试后宣称完成；
7. 必须添加可在管理员 Windows 环境执行的真实集成测试；
8. 必须完成编译、单元测试、静态检查和 Windows 实机验收；
9. 任何验收项未通过，任务状态必须保持“未完成”；
10. 最终提交必须同时包含代码、测试、验收脚本和结果报告。

本次任务只处理 TUN 完整闭环。不要顺带重写 UI、订阅、AI、数据库或无关模块。

---

# 1. 不可改变的产品与架构约束

## 1.1 保留当前 TUN 所有权模型

必须继续采用：

```text
代理核心：
- 创建并运行 TUN/Wintun 数据面；
- 接收进入虚拟网卡的数据包；
- 执行代理协议、分流和 DNS 处理。

Navo：
- 冻结物理出口；
- 创建代理端点旁路；
- 创建 IPv4 /1 路由；
- 管理 NRPT DNS；
- 管理 IPv6 阻断或隧道策略；
- 验证控制面和真实数据面；
- 记录结构化网络事务；
- 失败逆序回滚；
- 崩溃后恢复。
```

禁止把 sing-box 或 Mihomo 改成：

```text
auto_route: true
strict_route: true
```

当前目标是把 Navo 自主管理方案做正确，而不是绕过它。

## 1.2 多内核边界

必须保留：

```text
sing-box：支持系统代理和 TUN
Mihomo：支持系统代理和 TUN
Xray：当前不支持 Navo TUN，必须明确拒绝
```

不得为了快速通过测试删除 Mihomo 或三内核能力模型。

第一阶段真实数据面验收以 sing-box 为黄金路径；Mihomo 必须随后通过同一套公共事务验收。公共 Windows 网络事务不得复制两套。

## 1.3 失败安全原则

TUN 的最终原则是：

```text
未验证成功 = 未启动成功
回滚未完成 = 系统仍为 DIRTY
控制面正常但真实出网失败 = 未启动成功
进程存在但出口不正确 = 未启动成功
```

任何失败都必须：

```text
停止当前核心
撤销 Navo 创建的 NRPT
撤销 Navo 创建的 IPv6 防火墙规则
撤销 Navo 创建的 /1 路由
撤销 Navo 创建的端点旁路
释放 TUN 网卡句柄
删除或保留可恢复 journal
恢复到不托管状态
```

只有全部撤销成功，才允许返回“网络已恢复”。

---

# 2. 当前问题的根因

本任务不得再以“修复某一条报错”为目标。当前问题是事务链不完整。

## 2.1 Manager 所有权保存过晚

当前 `internal/service/capture_transition.go` 中，`prepareTUNLocked` 在 `manager.Activate()` 完全成功后才执行：

```go
s.networkManager = manager
```

如果 `Activate()` 中途修改了系统网络，同时内部回滚失败，外层回滚拿不到该 Manager，当前进程无法再次恢复。

必须在任何网络修改前保存回滚所有权。

## 2.2 失败回滚复用已取消的 context

`internal/network/manager.go` 在操作失败时使用启动事务的 `ctx` 回滚。

当失败原因本身是超时或取消时，回滚 PowerShell 会立即被取消，导致残留路由、NRPT 或防火墙规则。

回滚必须始终使用独立的 `context.Background()` 派生超时上下文。

## 2.3 健康版本提交过早

当前 `startCoreForCapture()` 在核心启动后立即调用：

```go
commitHealthyRuntime(ctx)
```

此时只证明核心进程和本地端口可能正常，尚未证明：

- TUN 网卡地址正确；
- 代理服务器旁路正确；
- `/1` 路由正确；
- DNS 正常；
- 普通 HTTPS 可用；
- 出口 IP 符合预期；
- 没有代理端点回环。

必须将 `commitHealthyRuntime` 移动到全部 TUN 验证完成之后。

## 2.4 端点旁路不是确定性的

当前代码只保存端点 IP，执行网络操作时再调用 `Find-NetRoute` 查出口。

这会受以下因素影响：

- TUN 网卡已经出现；
- 其他 VPN；
- Hyper-V；
- WSL；
- Wi-Fi 与有线并存；
- 路由指标变化；
- DNS 轮询；
- 多 A 记录；
- 核心实际连接的 IP 和旁路 IP 不一致。

必须在修改系统网络前冻结完整物理出口，并确保核心本次连接使用相同 IP。

## 2.5 网卡存在不等于网卡可用

不得只验证网卡名称存在或状态为 Enabled。

必须验证：

- 唯一匹配；
- GUID；
- InterfaceIndex；
- OperationalStatus；
- IPv4 地址；
- MTU；
- 直连网段；
- 不存在重复同名网卡。

## 2.6 `/1` 路由不是幂等事务

当前 `/1` 路由主要直接调用 `New-NetRoute`。

必须改为：

```text
精确路由存在：视为成功
同前缀但属性冲突：明确失败
完全不存在：创建
```

禁止覆盖或删除不属于 Navo 的路由。

## 2.7 Journal 记录的信息不足

当前 journal 主要记录动作名称和命令。

必须升级为结构化资源记录，能够精确描述 Navo 实际创建了什么。恢复时不得信任 journal 内任意命令字符串。

## 2.8 缺少真实数据面验收

当前测试大量使用 `fakeExecutor` 和 `fakePlatform`，只能证明命令字符串生成，不能证明 Windows TUN 能出网。

必须加入管理员 Windows 集成测试，并且生产启动过程也必须执行最小真实数据面验证。

---

# 3. 完成定义（Definition of Done）

只有同时满足以下条件，任务才算完成。

## 3.1 功能完成

启用 TUN 后必须验证：

```text
核心进程运行
核心控制端可用
TUN 网卡唯一且状态正确
TUN 地址正确
代理端点走原物理网卡
普通公网 IP 走 Navo 网卡
两条 IPv4 /1 路由存在
NRPT 规则存在且属于当前事务
IPv6 策略存在且属于当前事务
普通 DNS 查询成功
禁用显式代理的 TCP 连接成功
禁用显式代理的 HTTPS 请求成功
出口 IP 验证成功
UDP 探测成功或返回明确的协议不支持原因
```

## 3.2 失败恢复完成

在每个操作步骤注入失败，必须满足：

```text
系统网络恢复
journal 状态正确
非 Navo 路由未被删除
非 Navo NRPT 未被删除
非 Navo 防火墙规则未被删除
核心停止
TUN 状态不显示 running
LastKnownGood 不被错误提交
```

## 3.3 崩溃恢复完成

在以下阶段强制结束进程后，下一次启动必须恢复：

```text
端点旁路创建后
第一条 /1 路由创建后
第二条 /1 路由创建后
NRPT 创建后
IPv6 规则创建后
数据面验证过程中
```

## 3.4 工程门禁完成

必须通过：

```powershell
go test ./...
go vet ./...
npm ci
npm test
npm run typecheck
npm run build
```

同时必须通过本文件定义的管理员 Windows TUN 集成测试。

---

# 4. 必须检查和修改的文件

至少检查以下文件。不得只改其中一两个文件。

```text
internal/service/capture_transition.go
internal/service/runtime.go
internal/service/service.go

internal/network/types.go
internal/network/manager.go
internal/network/journal.go
internal/network/platform_windows.go
internal/network/executor_windows.go
internal/network/reconciler.go

internal/network/tun/tun_windows.go
internal/network/tun/route_windows.go
internal/network/tun/dns_windows.go
所有 InspectAdapter / WaitForAdapterState 相关文件

internal/compiler/generator.go
internal/compiler/multi_core.go
internal/coreadapter/base.go
internal/coreadapter/adapters.go

internal/network/manager_test.go
internal/service/capture_transition_test.go
相关现有测试
```

必须新增：

```text
internal/network/activation_plan.go
internal/network/activation_plan_windows.go
internal/network/verify_windows.go
internal/network/manager_windows_integration_test.go

internal/service/tun_dataplane_verify.go
internal/service/tun_dataplane_verify_windows.go
internal/service/tun_dataplane_verify_test.go

scripts/test-tun-elevated.ps1
docs/TUN_ACCEPTANCE_REPORT.md
```

文件名允许根据现有包结构微调，但职责不得丢失。

---

# 5. 新的核心数据结构

## 5.1 AdapterSnapshot

在 `internal/network/types.go` 或专用文件新增：

```go
type AdapterSnapshot struct {
    Name              string
    InterfaceIndex    uint32
    InterfaceGUID     string
    InterfaceLUID     uint64
    OperationalStatus string
    MTU               int
    IPv4Addresses     []string
    IPv6Addresses     []string
}
```

要求：

- 生产代码不得只靠可修改的显示名称识别网卡；
- journal 和验证至少保存 InterfaceIndex 与 GUID；
- 如果无法获得 LUID，可以暂时保留字段并使用 GUID；
- 网卡快照必须由 Windows API 或结构化 PowerShell 获取；
- 禁止解析本地化的 `route print` 文本作为主要事实来源。

## 5.2 EndpointRoutePlan

新增：

```go
type EndpointRoutePlan struct {
    EndpointHost    string
    EndpointIP      string
    AddressFamily   string

    InterfaceIndex uint32
    InterfaceGUID  string
    InterfaceAlias string
    NextHop         string

    RouteMetric     int
    InterfaceMetric int
}
```

## 5.3 TUNActivationPlan

新增：

```go
type TUNActivationPlan struct {
    SessionID string

    CoreID          string
    AdapterName     string
    TUNIPv4Address  string
    TUNIPv4Peer     string
    TUNDNSIPv4      string
    MTU             int

    SelectedOutboundID string
    OriginalServerHost string
    PinnedServerIP     string

    EndpointRoutes []EndpointRoutePlan

    IPv6Mode IPv6Mode

    CreatedAt time.Time
}
```

计划对象必须在：

```text
核心启动前
TUN 网卡出现前
Navo 路由修改前
NRPT 修改前
```

构建完成。

## 5.4 结构化 Journal V2

将 journal 版本升级为 2。

建议结构：

```go
type journalResourceKind string

const (
    resourceEndpointRoute journalResourceKind = "endpoint_route"
    resourceSplitRoute    journalResourceKind = "split_route"
    resourceNRPTRule      journalResourceKind = "nrpt_rule"
    resourceFirewallRule  journalResourceKind = "firewall_rule"
)

type journalResource struct {
    Kind journalResourceKind `json:"kind"`

    DestinationPrefix string `json:"destination_prefix,omitempty"`
    AddressFamily     string `json:"address_family,omitempty"`
    InterfaceIndex    uint32 `json:"interface_index,omitempty"`
    InterfaceGUID     string `json:"interface_guid,omitempty"`
    NextHop           string `json:"next_hop,omitempty"`
    RouteMetric       int    `json:"route_metric,omitempty"`

    NRPTNamespace string `json:"nrpt_namespace,omitempty"`
    NRPTComment   string `json:"nrpt_comment,omitempty"`

    FirewallDisplayName string `json:"firewall_display_name,omitempty"`

    CreatedByNavo bool `json:"created_by_navo"`
}

type journalAction struct {
    Name     string          `json:"name"`
    Status   actionStatus    `json:"status"`
    Resource journalResource `json:"resource"`
}
```

禁止继续把可执行任意命令作为恢复事实来源。

可以读取 V1 journal，但只能通过白名单动作名迁移到安全删除逻辑。不得执行 V1 中存储的命令。

---

# 6. 正确的 TUN 启动状态机

必须把当前流程改成以下顺序。

```text
IDLE
  ↓
PREFLIGHT
  ↓
BASELINE_CAPTURED
  ↓
CONFIG_COMPILED
  ↓
CORE_STARTED
  ↓
ADAPTER_READY
  ↓
NETWORK_APPLIED
  ↓
CONTROL_PLANE_VERIFIED
  ↓
DATA_PLANE_VERIFIED
  ↓
HEALTH_COMMITTED
  ↓
RUNNING
```

任意步骤失败：

```text
FAILED
  ↓
ROLLING_BACK
  ↓
ROLLED_BACK
```

回滚失败：

```text
DIRTY
```

不得跳过状态。

建议新增结构化阶段常量：

```go
type TUNStage string

const (
    TUNStagePreflight            TUNStage = "PREFLIGHT"
    TUNStageBaselineCaptured     TUNStage = "BASELINE_CAPTURED"
    TUNStageConfigCompiled       TUNStage = "CONFIG_COMPILED"
    TUNStageCoreStarted          TUNStage = "CORE_STARTED"
    TUNStageAdapterReady         TUNStage = "ADAPTER_READY"
    TUNStageNetworkApplied       TUNStage = "NETWORK_APPLIED"
    TUNStageControlPlaneVerified TUNStage = "CONTROL_PLANE_VERIFIED"
    TUNStageDataPlaneVerified    TUNStage = "DATA_PLANE_VERIFIED"
    TUNStageHealthCommitted      TUNStage = "HEALTH_COMMITTED"
)
```

每个阶段写入结构化日志，但不要为每一步创建第二套状态机。Service Capture Coordinator 是唯一事务所有者。

---

# 7. 第一步：在网络修改前冻结物理出口

## 7.1 解析所选端点

读取当前选中的 outbound。

如果服务器已经是 IP：

```text
直接使用该 IP
```

如果服务器是域名：

```text
使用启用前的系统 DNS 解析
收集所有地址
过滤无效、回环、组播和不可路由地址
根据当前 IPv6 策略过滤
为每个候选地址查找物理出口
选取具有有效物理出口的首选地址
```

解析失败、没有地址或找不到物理出口时，必须在修改路由前失败。

禁止继续返回空旁路列表并启动 TUN。

## 7.2 冻结物理路由

对最终选定端点，在 TUN 网卡出现前执行结构化查询。

PowerShell 可以使用：

```powershell
Find-NetRoute -RemoteIPAddress <endpoint>
```

但必须：

```text
只选择具有 NextHop 属性的路由对象
过滤 Navo 接口
过滤状态非 Up 的接口
按 RouteMetric + InterfaceMetric 排序
Select-Object -First 1
转换成明确标量
读取对应接口 GUID 和状态
```

不得在 `/1` 路由创建后重新推断物理出口。

## 7.3 固定核心本次连接 IP

核心配置必须使用本次计划选择的 `PinnedServerIP`。

例如原节点：

```text
server = proxy.example.com
SNI = proxy.example.com
WS Host = proxy.example.com
```

本次运行配置应为：

```text
server = 203.0.113.8
SNI = proxy.example.com
WS Host = proxy.example.com
```

不得破坏：

- TLS SNI；
- Reality server name；
- WebSocket Host；
- HTTP Host；
- gRPC service name。

这样可以保证：

```text
Navo 旁路的 IP
=
核心实际连接的 IP
```

如果某协议或核心无法安全固定 IP，必须明确返回能力不支持，不得猜测。

---

# 8. 第二步：修复事务所有权和回滚 context

## 8.1 Manager 必须提前保存

修改 `internal/service/capture_transition.go`。

目标逻辑：

```go
manager, err := s.newTUNNetworkManager(plan)
if err != nil {
    return nil, err
}

// 在任何网络修改之前保存所有权
s.networkManager = manager

if err := manager.Preflight(ctx); err != nil {
    return nil, err
}
```

如果后续失败，外层 `rollbackCaptureLocked` 必须能再次使用该 Manager。

只有满足以下条件才可置空：

```go
if err := s.networkManager.Deactivate(rollbackCtx); err == nil {
    s.networkManager = nil
}
```

## 8.2 回滚必须独立于启动 context

在 `internal/network/manager.go` 增加统一回滚入口：

```go
func (m *Manager) rollbackAfterFailure(value *journal) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    return m.rollbackLocked(ctx, value)
}
```

所有失败路径必须使用独立回滚 context。

外层 Capture 回滚也必须使用：

```go
context.WithTimeout(context.Background(), captureRollbackTimeout)
```

禁止将已经超时或取消的请求 context 传给网络恢复。

## 8.3 回滚结果不得伪装成功

如果任何撤销动作失败：

- journal 中保留未撤销资源；
- `networkManager` 不得置空；
- Recovery State 保持 DIRTY；
- 返回组合错误；
- UI 不得显示“不托管且网络正常”；
- 下次启动必须优先执行恢复。

---

# 9. 第三步：延迟健康提交

拆分当前：

```go
startCoreForCapture()
```

成为至少两个职责：

```go
startCoreOnly(ctx)
commitHealthyRuntime(ctx)
```

TUN 模式执行：

```text
startCoreOnly
等待核心就绪
等待网卡完整就绪
应用网络事务
验证控制面
验证真实数据面
commitHealthyRuntime
```

系统代理模式可以保留自己的完整成功判定，但不得因为此次改动而退化。

`LastKnownGood`、Revision Active 和 `status=running` 只能在数据面验证成功后写入。

---

# 10. 第四步：严格等待 TUN 网卡就绪

新增：

```go
func WaitForAdapterReady(
    ctx context.Context,
    expectedName string,
    expectedAddress string,
    expectedMTU int,
) (AdapterSnapshot, error)
```

成功条件：

```text
只存在一个目标网卡
GUID 非空
InterfaceIndex > 0
OperationalStatus = Up
包含 172.19.0.1/30 对应的接口地址
MTU 与配置一致
不是 Loopback
没有同名冲突网卡
```

如果地址尚未配置，继续等待，不得立即添加 `/1` 路由。

超时错误必须包含当前快照，示例：

```text
TUN_ADAPTER_NOT_READY:
name=Navo
state=Up
index=27
addresses=[]
expected=172.19.0.1/30
```

---

# 11. 第五步：幂等应用网络资源

网络操作顺序固定为：

```text
1. 端点旁路
2. IPv4 0.0.0.0/1
3. IPv4 128.0.0.0/1
4. NRPT DNS
5. IPv6 策略
```

回滚顺序必须完全相反。

## 11.1 端点旁路

不得在应用时再次调用不受约束的 `Find-NetRoute`。

直接使用 `TUNActivationPlan.EndpointRoutes` 中冻结的数据。

创建前检查：

```text
接口仍存在
接口 GUID 未变化
接口状态仍为 Up
接口不是 Navo
NextHop 有效
```

检查现有精确路由：

```text
目标前缀
InterfaceIndex
NextHop
RouteMetric
```

规则：

```text
完全一致：幂等成功，不标记 CreatedByNavo
同目标前缀但属性冲突：失败
不存在：创建并标记 CreatedByNavo
```

## 11.2 `/1` 路由

对：

```text
0.0.0.0/1
128.0.0.0/1
```

执行同样的精确幂等检查。

使用 AdapterSnapshot 的 InterfaceIndex，不再主要依赖 InterfaceAlias。

下一跳使用：

```text
172.19.0.2
```

但配置字段拆分为：

```go
TUNIPv4Address = "172.19.0.1/30"
TUNIPv4Peer    = "172.19.0.2"
TUNDNSIPv4     = "172.19.0.2"
```

值暂时相同不代表概念相同。

## 11.3 NRPT

规则必须拥有唯一 Comment：

```text
Navo:TUN:<sessionID>
```

创建前检查：

```text
相同 Comment 和配置存在：幂等成功
Namespace "." 存在但不是 Navo 所有：不得删除或覆盖
```

撤销只能删除 Comment 精确匹配当前事务的规则。

## 11.4 IPv6

当前 `IPv6Block` 必须：

- 使用唯一 DisplayName；
- 规则名称包含 sessionID；
- 只删除当前事务创建的规则；
- 验证规则实际 Enabled；
- 不得删除用户或其他软件的 IPv6 规则。

---

# 12. 第六步：控制面硬验证

网络资源应用后立即执行：

```go
func VerifyControlPlane(
    ctx context.Context,
    plan TUNActivationPlan,
    adapter AdapterSnapshot,
) error
```

必须检查：

## 12.1 代理端点不回环

对每个端点执行最佳路由查询。

必须满足：

```text
InterfaceIndex == 冻结的物理 InterfaceIndex
InterfaceGUID == 冻结的物理 InterfaceGUID
NextHop == 冻结的物理 NextHop
InterfaceIndex != TUN InterfaceIndex
```

否则返回：

```text
TUN_ENDPOINT_LOOP_DETECTED
```

## 12.2 公网流量进入 TUN

至少对两个不同公网地址查询，例如：

```text
1.1.1.1
8.8.8.8
```

必须命中 Navo TUN InterfaceIndex。

若企业或测试环境禁止这些地址，可以由测试配置传入，但生产默认必须有稳定探测目标。

## 12.3 `/1` 路由精确存在

检查：

```text
DestinationPrefix
InterfaceIndex
NextHop
RouteMetric
PolicyStore
```

## 12.4 NRPT 精确存在

检查：

```text
Namespace = "."
NameServers 包含 TUNDNSIPv4
Comment = Navo:TUN:<sessionID>
```

## 12.5 IPv6 规则存在

在 `IPv6Block` 模式下检查当前 session 规则存在并启用。

控制面任何一步失败都必须回滚。

---

# 13. 第七步：真实数据面硬验证

新增 Service 层验证器：

```go
type TUNDataPlaneVerifier interface {
    Verify(ctx context.Context, request VerifyRequest) (VerifyResult, error)
}
```

## 13.1 禁止使用显式本地代理

HTTP 客户端必须明确禁用系统和环境代理：

```go
transport := &http.Transport{
    Proxy: nil,
}
```

必要时清空：

```text
HTTP_PROXY
HTTPS_PROXY
ALL_PROXY
NO_PROXY
```

测试请求不得连接：

```text
127.0.0.1:<mixed-port>
```

因为那只能证明核心代理入口可用，不能证明 TUN。

## 13.2 DNS 验证

使用普通系统解析器解析至少一个域名。

必须防止缓存产生假阳性：

- 使用随机子域或多个目标；
- 记录解析耗时；
- 记录结果；
- 不能只检查缓存 API 返回。

## 13.3 TCP 验证

禁用显式代理，连接远端 443。

## 13.4 HTTPS 验证

禁用显式代理执行 HTTPS GET。

响应只需满足：

```text
TLS 建立成功
HTTP 状态有效
读取有限字节成功
```

不得下载大内容。

## 13.5 出口 IP 验证

启用 TUN 前保存 Direct IP。

启用后通过禁用显式代理的请求获取 TUN Exit IP。

成功规则：

```text
如果选中的是代理出口：
TUN Exit IP 应与 Direct IP 不同
并且与通过本地代理入口检测到的代理出口一致，或满足配置的预期。

如果运行模式为 direct：
允许与 Direct IP 相同。
```

不得因为接口和路由存在就跳过出口验证。

## 13.6 UDP 验证

至少执行一个 UDP DNS 探测或等价的 UDP 探测。

如果当前节点协议、网络或核心明确不支持 UDP，必须返回结构化能力结果，不能静默算成功。

建议：

```go
type UDPVerificationStatus string

const (
    UDPVerified    UDPVerificationStatus = "verified"
    UDPUnsupported UDPVerificationStatus = "unsupported"
    UDPFailed      UDPVerificationStatus = "failed"
)
```

TUN 产品声明支持 UDP 时，`UDPFailed` 必须导致启动失败。

---

# 14. 第八步：成功提交

只有以下都成功：

```text
核心就绪
网卡就绪
网络应用成功
控制面验证成功
数据面验证成功
```

才能：

```go
commitHealthyRuntime(ctx)
s.clearTUNFault()
return status=running
```

成功结果必须包含：

```json
{
  "status": "running",
  "mode": "tun",
  "stage": "HEALTH_COMMITTED",
  "pid": 1234,
  "adapter": {
    "name": "Navo",
    "interface_index": 27,
    "interface_guid": "..."
  },
  "verification": {
    "dns": true,
    "tcp": true,
    "https": true,
    "exit_ip": "...",
    "udp": "verified"
  }
}
```

敏感节点凭据不得进入日志或响应。

---

# 15. 第九步：回滚与恢复

## 15.1 当前事务回滚

固定顺序：

```text
删除当前 session IPv6 规则
删除当前 session NRPT
删除 Navo 创建的 128.0.0.0/1
删除 Navo 创建的 0.0.0.0/1
删除 Navo 创建的端点旁路
停止核心
释放 TUN
```

每删除一个资源后立即更新 journal。

## 15.2 崩溃恢复

启动时发现 V2 journal：

- 逐项验证资源是否仍存在；
- 只删除精确匹配且 `CreatedByNavo=true` 的资源；
- 已不存在视为幂等成功；
- 属性冲突时不得误删，保持 DIRTY 并给出人工诊断信息。

## 15.3 Reconciler 边界

`Reconciler` 只能作为恢复协调者。

禁止继续依赖：

```text
解析本地化 route print 文本
按字符串包含 "Navo" 批量删除路由
删除无法证明归属的系统资源
```

应优先使用 Network Journal V2。

没有 journal 时，只允许执行保守诊断，不能猜测并批量删除。

---

# 16. 错误码

至少增加：

```text
TUN_PREFLIGHT_FAILED
TUN_ENDPOINT_RESOLVE_FAILED
TUN_PHYSICAL_ROUTE_NOT_FOUND
TUN_ENDPOINT_PIN_FAILED
TUN_CORE_START_FAILED
TUN_ADAPTER_NOT_READY
TUN_ADAPTER_CONFLICT
TUN_ENDPOINT_BYPASS_FAILED
TUN_SPLIT_ROUTE_FAILED
TUN_NRPT_FAILED
TUN_IPV6_POLICY_FAILED
TUN_ENDPOINT_LOOP_DETECTED
TUN_PUBLIC_ROUTE_NOT_CAPTURED
TUN_DNS_VERIFY_FAILED
TUN_TCP_VERIFY_FAILED
TUN_HTTPS_VERIFY_FAILED
TUN_EXIT_IP_MISMATCH
TUN_UDP_VERIFY_FAILED
TUN_ROLLBACK_FAILED
TUN_RECOVERY_DIRTY
```

错误必须包含：

```text
stage
resource
expected
actual
cause
rollback status
```

PowerShell 输出必须保持 UTF-8，并保留 GBK 兼容解码兜底。

---

# 17. 单元测试要求

## 17.1 Manager 事务测试

覆盖：

```text
每个 apply 步骤失败
每个 undo 步骤失败
启动 ctx 已取消时仍能使用独立 ctx 回滚
Manager 所有权提前保存
journal 每个资源状态更新
幂等激活
幂等撤销
冲突路由拒绝
非 Navo 路由不删除
V1 journal 安全迁移
恶意 journal 命令永不执行
```

## 17.2 Activation Plan 测试

覆盖：

```text
IP 端点
域名多 A 记录
解析失败
无物理路由
多网卡按总指标选择
过滤 Navo 网卡
保存 GUID/Index/NextHop
固定 IP 后保留 SNI/Host
```

## 17.3 Service 测试

覆盖：

```text
commitHealthyRuntime 只在最终验证后调用
控制面失败不提交健康
数据面失败不提交健康
回滚失败时 networkManager 不置空
TUN Fault 包含阶段和错误码
```

---

# 18. Windows 管理员集成测试

新增：

```text
internal/network/manager_windows_integration_test.go
```

只在以下条件运行：

```text
GOOS=windows
NAVO_RUN_ELEVATED_TUN_TESTS=1
当前进程具有管理员权限
```

未设置变量时允许 Skip，但正式验收不得 Skip。

## 18.1 测试隔离

集成测试必须：

- 使用唯一 sessionID；
- 使用唯一 NRPT Comment；
- 使用唯一防火墙 DisplayName；
- 在 `defer` 和 `TestMain` 双重清理；
- 测试开始前保存相关网络快照；
- 测试结束后比较恢复结果；
- 不修改持久路由；
- 不删除非测试资源。

## 18.2 必测场景

至少：

```text
单 Wi-Fi
单有线
Wi-Fi + 有线
Hyper-V 已安装
WSL 已安装
节点为 IP
节点为域名
节点域名多 A 记录
启用后禁用 Navo 网卡
启用后删除 Navo 网卡
第一条 /1 后注入失败
第二条 /1 后注入失败
NRPT 后注入失败
IPv6 规则后注入失败
核心异常退出
Navo 主进程异常退出
重复启用
重复关闭
TUN → 系统代理 → TUN
TUN → 不托管 → TUN
```

不能在 CI 云主机上无法建立真实 TUN 时伪造通过。

---

# 19. 验收脚本

新增：

```text
scripts/test-tun-elevated.ps1
```

脚本必须：

1. 检查管理员权限；
2. 检查 sing-box、Mihomo、wintun.dll；
3. 记录启用前：
   - 默认路由；
   - DNS；
   - NRPT；
   - Navo 相关防火墙规则；
   - 网卡；
   - Direct IP；
4. 启动 Navo；
5. 启用 sing-box TUN；
6. 运行控制面和数据面验证；
7. 关闭 TUN；
8. 比较恢复结果；
9. 对 Mihomo 重复；
10. 注入指定失败点并验证回滚；
11. 输出 JSON 和 Markdown 报告；
12. 任一项失败返回非零退出码。

推荐参数：

```powershell
.\scripts\test-tun-elevated.ps1 `
  -Core sing-box `
  -FailurePoint none `
  -OutputDirectory .\artifacts\tun-acceptance
```

失败点：

```text
after-endpoint-bypass
after-first-split-route
after-second-split-route
after-nrpt
after-ipv6
during-dataplane
```

---

# 20. 验收报告

生成：

```text
docs/TUN_ACCEPTANCE_REPORT.md
```

必须记录：

```text
执行时间
Windows 版本
网卡环境
Hyper-V/WSL 状态
核心版本
Wintun 哈希
每个测试项结果
启用前后出口 IP
启用前后路由摘要
启用前后 DNS/NRPT 摘要
失败注入结果
回滚结果
仍未覆盖的环境
```

不得写“理论上通过”。

只有实际运行的项目才可写 Passed。

---

# 21. 禁止事项

Codex 严禁：

1. 把 `auto_route` 改成 `true` 来绕过 Navo 网络事务；
2. 只修复 `System.Object[]` 后停止；
3. 只判断进程和端口；
4. 只判断网卡名称存在；
5. 使用本地代理端口证明 TUN 成功；
6. 在 TUN 验证前提交 LastKnownGood；
7. 使用已取消的 context 回滚；
8. 回滚失败后把 Manager 置空；
9. 解析本地化 `route print` 作为唯一依据；
10. 按名称包含 `Navo` 批量删除系统资源；
11. 删除不属于当前 session 的 NRPT 或防火墙规则；
12. 让自愈模块直接执行裸 `New-NetRoute`、`Remove-NetRoute`、`netsh`；
13. 添加第二个网络修改所有者；
14. 只增加 Markdown 而不修改代码；
15. 只运行 Mock 测试就宣称 TUN 完成；
16. 为了通过测试降低错误检查；
17. 吞掉失败或只写日志继续运行；
18. 在真实数据面失败时返回 `status=running`。

---

# 22. 推荐实施顺序

严格按以下顺序完成，每阶段测试通过后再进入下一阶段。

## Phase 1：事务安全

```text
Manager 提前保存
独立回滚 context
回滚失败不置空
健康提交延后
```

## Phase 2：确定性计划

```text
AdapterSnapshot
EndpointRoutePlan
TUNActivationPlan
端点解析
物理出口冻结
核心端点 IP 固定
```

## Phase 3：Journal V2

```text
结构化资源
安全恢复
V1 兼容
精确所有权
```

## Phase 4：幂等网络操作

```text
端点旁路
两条 /1
NRPT
IPv6
```

## Phase 5：验证闭环

```text
控制面验证
DNS/TCP/HTTPS/出口 IP/UDP
错误码
结构化结果
```

## Phase 6：真实 Windows 验收

```text
管理员集成测试
失败注入
崩溃恢复
sing-box
Mihomo
报告
```

---

# 23. 最终需要提交的结果

最终回复不得只给总结，必须列出：

```text
修改文件
新增文件
删除文件
关键架构变化
每个根因对应的代码修复
单元测试结果
Windows 管理员集成测试结果
sing-box TUN 结果
Mihomo TUN 结果
失败注入结果
崩溃恢复结果
未完成项
```

如果无法执行管理员 Windows 验收，必须明确写：

```text
实现完成，但 TUN 功能尚未验收完成
```

不得写：

```text
TUN 已彻底修复
```

---

# 24. 最终成功标准

最终用户操作：

```text
选择代理来源
选择节点或独立代理
选择 sing-box 或 Mihomo
点击 TUN
```

Navo 必须做到：

```text
启用前冻结物理出口
核心连接固定端点
创建网卡
精确添加旁路和路由
精确添加 DNS 和 IPv6 策略
验证节点不回环
验证普通流量进入 TUN
验证 DNS、TCP、HTTPS、出口 IP 和 UDP
全部成功后才显示已连接
任何失败立即恢复系统网络
崩溃后下次启动继续精确恢复
```

这才是本任务的完成状态。

---

# 25. Codex 最后一条指令

现在开始直接修改代码。

不要再新建另一份“TUN 修复建议”文档代替实现。  
不要等待用户提供新的报错再逐项打补丁。  
先建立确定性 Activation Plan、唯一事务所有权、结构化 Journal、控制面验证和真实数据面验收，再以 Windows 管理员测试证明结果。

未通过真实 TUN 验收前，不得将任务标记为完成。
