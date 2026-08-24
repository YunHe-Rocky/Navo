# Navo Network Environment + Self-Healing Integration
## Codex 实施说明 / 增量重构作业指导书

> **用途**：本文件不是概念介绍，而是给 Codex 直接执行的工程实施说明。  
> **目标仓库**：`YunHe-Rocky/Navo`  
> **审阅基线**：`main` 分支，审阅时最新提交 `a337738105d8eb2fc52e49de0ef38caf2aaad4f9`  
> **基线提交说明**：`feat: update Navo architecture and self-healing`  
> **注意**：开始修改前，必须重新读取当前 `HEAD`。如果仓库已经前进，按当前源码符号和目录适配，不允许机械套用本文件中的旧行号或旧函数位置。

---

# 0. Codex 执行要求

Codex 收到本文件后，应直接进入“读取现有代码 → 增量修改 → 补测试 → 运行验证”的流程。

必须遵守：

1. **不要新增 Rescue / 救援模式。**
2. **不要新增第二套 Connection Coordinator。**
3. **不要重写现有 Self-Healing。**
4. **不要绕过现有 `internal/connection.Coordinator`。**
5. **不要让 Network Environment 直接修改 Windows 网络。**
6. **不要杀死、停止、禁用 v2rayN、Clash、VPN 或其他第三方代理程序。**
7. **不要把“发现第三方网络状态”自动等同于“异常”。**
8. **只有能证明属于 Navo 的资源，Self-Healing 才允许自动恢复/清理。**
9. **用户或第三方软件在 Navo 运行期间修改了网络时，Navo 不得强行覆盖新的外部配置。**
10. **保持现有 Service / Agent / UI DTO 尽量向后兼容。新增字段优先采用可选字段。**
11. **不得为了通过测试降低 ownership、回滚、验证、超时、权限或 fail-closed 安全边界。**
12. 所有新的 Windows 探测必须是：
    - 只读；
    - 有 context；
    - 有超时；
    - 有明确错误；
    - 不依赖无限等待；
    - 不执行持久化的任意命令文本。
13. 修改完成后必须运行本文最后的测试与验收项。

---

# 1. 本次到底要做什么

本次不是“再做一个网络修复系统”。

Navo 当前已经有：

- Connection Coordinator
- Capture transaction
- Network Journal
- ownership
- rollback
- startup recovery
- Self-Healing fault matrix
- repair round
- failover
- fail-closed
- System Proxy ownership protection
- TUN / Route / DNS / NRPT / Firewall recovery 基础设施

因此，本次真正缺少的是：

> **一个统一、只读、可持续更新的 Network Environment（网络环境感知层），把目前散落在 Agent、Service、System Proxy、TUN、Route、DNS、自愈检查中的“观察结果”整理成同一份 Network Environment Snapshot。**

然后：

- **Environment 负责看**
- **Coordinator 负责正常操作的事务边界**
- **Self-Healing 负责异常接管**
- **Agent / Service 负责执行现有领域动作**

最终逻辑：

```text
Windows 当前真实状态
        │
        ▼
Network Environment
只读收集 / 归一化 / 分析
        │
        ▼
Environment Snapshot
        │
        ├──────────────► UI
        │
        ├──────────────► Connection Coordinator 前置判断
        │
        └──────────────► Self-Healing 故障证据
                              │
                              ▼
                        existing repair flow
                              │
                              ▼
                     Agent / Service executors
```

---

# 2. 当前源码结论

## 2.1 Connection Coordinator 已经存在，禁止重做

现有：

`internal/connection/coordinator.go`

它已经定义：

### Operation

- `capture_switch`
- `node_switch`
- `core_switch`
- `source_mutation`
- `policy_change`
- `core_update`
- `recovery`
- `self_heal`

### Origin

- `user`
- `tray`
- `scheduler`
- `self_heal`
- `startup`
- `shutdown`

### Phase

- `queued`
- `preparing`
- `applying`
- `verifying`
- `committing`
- `rolling_back`
- `completed`
- `failed`

它目前的正确定位是：

> **跨领域网络变更的唯一事务门 / 串行化边界。**

不要把它改造成一个巨型 God Object。

Network Environment **不持有这个锁来做日常观察**。

观察必须可以并发进行。

只有网络“修改行为”仍然必须走现有 Coordinator。

---

## 2.2 Agent 已经是正常连接流程的主协调入口

现有：

- `internal/agent/agent.go`
- `internal/agent/capture_transition.go`

代码已经实现：

```text
UI
↓
Agent
↓
Connection Coordinator
↓
Capture/Core/Outbound transaction
↓
Service domain operation
↓
Verify
↓
Commit / Rollback
```

例如 Capture 模式切换已经具备：

- 保存 Transition Journal
- 保存 System Proxy 原始状态
- 启动/准备核心
- System Proxy / TUN 切换
- 数据面检查
- 成功提交
- 失败回滚
- 故障状态记录

因此：

> **不要创建新的 ConnectionService、ConnectionManager 或第二套状态机来替代现有逻辑。**

本次只需要让现有连接流程能够读取 Environment Snapshot 作为结构化前置证据。

---

## 2.3 Self-Healing 已经存在，而且能力比概念阶段更完整

现有：

`internal/selfheal/`

主要包括：

- `engine.go`
- `model.go`
- `recovery.go`
- `policy.go`
- `registry.go`
- `state.go`

已有故障域：

- node
- core
- system_proxy
- tun
- route
- dns
- nrpt
- firewall
- traffic_rule
- physical_network
- detection
- unknown

已有恢复动作：

- `reapply_capture`
- `restart_owned_core`
- `reconcile_owned_capture`
- `reconcile_owned_network`
- `recover_owned_network`
- `reapply_traffic_policy`

并且已经具备：

- 最多两轮恢复
- Verification
- rollback
- per-resource lock
- dedupe
- backoff
- circuit breaker
- failover
- fail-closed

因此：

> **“救援模式”与 Self-Healing 是同一套概念。禁止再增加 Rescue 模块。**

---

## 2.4 当前存在两条 Self-Healing 实现路径，短期不要贸然合并

当前代码实际上有两层：

### A. Service 内的通用 `selfheal.Engine`

由 `internal/service/service.go` 初始化。

当前默认策略主要还是 observer-oriented。

### B. Agent 内 Capture Self-Healing

主要位于：

`internal/agent/capture_transition.go`

其中已经有：

- `captureHealthFault`
- `classifyCaptureFault`
- `recoverUnhealthyCapture`
- `executeCaptureRepair`
- failover
- fail-closed

这条路径目前承担了真正的 Capture 网络异常恢复。

### 本次处理原则

本次 **不强制把两套执行器一次性合并**。

原因：

如果同时让：

- Service `selfheal.Engine`
- Agent `recoverUnhealthyCapture`

对同一 System Proxy/TUN/Route/DNS 故障开启自动修复，

可能产生：

- 双重重试
- 双重回滚
- Coordinator 竞争
- recovery loop
- 状态互相覆盖

所以本次：

> **统一“概念、故障证据和 Environment 数据源”，暂时保留现有 Agent Capture Self-Healing 为 Capture 网络故障的主要恢复入口。**

后续如果要完全统一 Engine，必须单独做架构迁移，不属于本次第一阶段。

---

# 3. 为什么还需要 Network Environment

现在 Navo 已经能检查很多东西，但这些检查是散落的。

例如：

- `systemproxy.Manager.Status()`
- `systemproxy.CurrentConfig()`
- `tun.status`
- `core.status`
- `runtime.verify`
- `captureHealthFault()`
- `verifyCaptureReadiness()`
- Network Journal
- TUN adapter status
- Route/DNS/NRPT verification
- Self-Healing FaultEvidence

问题不是“没有检查能力”。

问题是：

> **没有一份统一的“Windows 当前真实网络状态”。**

这会导致后续继续扩展时，各模块继续自己读取、自己解释、自己分类。

长期容易出现：

```text
UI 认为正常
Agent 认为异常
Self-Healing 认为 TUN 错误
Service 认为 Route 错误
SystemProxy Status 又只表示 Navo ownership
```

所以要增加：

```text
Network Environment Snapshot
```

成为统一的观察结果。

---

# 4. 一个非常重要的现有语义问题

当前：

`internal/agent/systemproxy/proxy.go`

里面有：

```go
func CurrentConfig() (ProxyConfig, error)
```

这个函数读取：

> **Windows 当前真实 WinINet System Proxy**

并且不取得 ownership，也不修改系统。

这个非常适合 Environment。

但是：

```go
func (m *Manager) Status() ProxyConfig
```

其 `Enabled` 最终会经过：

```go
cfg.Enabled = m.owns(*cfg)
```

所以它表达的更接近：

> **“当前代理是不是 Navo 正在拥有的代理”**

而不是：

> **“Windows System Proxy 到底有没有开启”**

这是本次必须解决的语义分离。

以后至少要明确区分：

```text
Raw Windows State
≠
Navo Ownership State
```

例如：

```text
Windows System Proxy:
Enabled = true
Server = 127.0.0.1:10808

Navo Ownership:
Owned = false
```

这个状态非常有价值。

它意味着：

> Windows 有代理，但不是 Navo 的。

Environment 可以显示：

```text
第三方 / 外部 System Proxy 正在使用
```

但是：

> **不允许 Navo 擅自关闭它。**

---

# 5. 新模块定位

建议新增：

```text
internal/networkenv/
```

不要直接命名 `environment`，避免和：

- OS environment
- `.env`
- runtime environment

产生概念混淆。

建议文件：

```text
internal/networkenv/
├── model.go
├── analyzer.go
├── store.go
├── collector.go
├── analyzer_test.go
└── store_test.go
```

第一阶段 `networkenv` 必须保持：

> **平台中立 + 只保存模型、归一化结果、分析规则、缓存。**

不要让这个 package 直接：

- 修改注册表
- 修改 Route
- 修改 DNS
- 创建/删除 TUN
- 启停 Core
- 调用 repair
- 杀进程

---

# 6. 推荐数据模型

可以按当前源码适配命名，但语义必须保留。

```go
package networkenv

type HealthState string

const (
    HealthUnknown     HealthState = "unknown"
    HealthChecking    HealthState = "checking"
    HealthHealthy     HealthState = "healthy"
    HealthDegraded    HealthState = "degraded"
    HealthUnavailable HealthState = "unavailable"
)

type Ownership string

const (
    OwnerNone     Ownership = "none"
    OwnerNavo     Ownership = "navo"
    OwnerExternal Ownership = "external"
    OwnerUnknown  Ownership = "unknown"
)

type Severity string

const (
    SeverityInfo  Severity = "info"
    SeverityWarn  Severity = "warning"
    SeverityError Severity = "error"
)

type Snapshot struct {
    Version     int         `json:"version"`
    CollectedAt time.Time   `json:"collected_at"`
    Health      HealthState `json:"health"`
    Stale       bool        `json:"stale"`

    Transition TransitionSnapshot `json:"transition"`

    Physical    PhysicalSnapshot    `json:"physical"`
    SystemProxy SystemProxySnapshot `json:"system_proxy"`
    TUN         TUNSnapshot         `json:"tun"`
    DNS         DNSSnapshot         `json:"dns"`
    Routes      RouteSnapshot       `json:"routes"`

    Findings []Finding `json:"findings"`
}
```

---

## 6.1 TransitionSnapshot

Environment 必须知道当前是不是正在发生合法变更。

否则在：

```text
System Proxy -> TUN
```

切换过程的中间几百毫秒内，可能刚好观察到：

```text
Proxy 已关
TUN 尚未完成
Route 部分写入
```

然后错误判定成系统异常。

建议：

```go
type TransitionSnapshot struct {
    Busy        bool   `json:"busy"`
    ID          string `json:"id,omitempty"`
    Operation   string `json:"operation,omitempty"`
    Phase       string `json:"phase,omitempty"`
    FaultDomain string `json:"fault_domain,omitempty"`
}
```

来源直接读取：

```go
a.coordinator.Snapshot()
```

Environment **不锁住 Coordinator**。

只把当前事务状态写入 Snapshot。

Analyzer 在：

- applying
- committing
- rolling_back
- recovery
- self_heal

期间应减少瞬态误报。

---

# 7. System Proxy Snapshot

建议至少：

```go
type SystemProxySnapshot struct {
    Enabled       bool   `json:"enabled"`
    ProxyServer   string `json:"proxy_server,omitempty"`
    BypassList    string `json:"bypass_list,omitempty"`
    AutoConfigURL string `json:"auto_config_url,omitempty"`
    AutoDetect    bool   `json:"auto_detect"`

    Ownership Ownership `json:"ownership"`
    OwnedByNavo bool     `json:"owned_by_navo"`

    LocalEndpoint bool `json:"local_endpoint"`
    Reachable     bool `json:"reachable"`
    ReachableKnown bool `json:"reachable_known"`
}
```

### Raw 状态来源

使用现有：

```go
systemproxy.CurrentConfig()
```

### Ownership 来源

使用现有 Navo proxy manager 的 ownership 语义。

不要读取一份 raw 状态以后仅凭：

```text
127.0.0.1:xxxx
```

就说它是：

- v2rayN
- Clash
- Navo

这不可靠。

---

# 8. 第三方代理识别原则

第一阶段不需要准确识别“到底是 Clash 还是 v2rayN”。

只需要安全判断：

```text
External
```

即可。

例如：

```text
System Proxy:
127.0.0.1:10808
Ownership:
external
```

UI 可以显示：

> 检测到外部 System Proxy

不要直接显示：

> v2rayN 正在控制系统代理

除非后续有明确证据链：

- listening PID
- process executable path
- process identity
- endpoint match
- signature / known identity

即使识别出第三方程序：

> **也只能提示，不能杀进程。**

---

# 9. TUN Snapshot

不要只保存“有没有 Navo TUN”。

Environment 需要区分：

```text
Navo TUN
External virtual adapters
```

建议：

```go
type TUNSnapshot struct {
    Navo TUNAdapterSnapshot `json:"navo"`

    ExternalPresent bool                 `json:"external_present"`
    External        []ExternalAdapterRef `json:"external,omitempty"`
}
```

Navo TUN 至少包含：

```go
type TUNAdapterSnapshot struct {
    Present        bool      `json:"present"`
    Enabled        bool      `json:"enabled"`
    Name           string    `json:"name,omitempty"`
    InterfaceGUID  string    `json:"interface_guid,omitempty"`
    InterfaceIndex int       `json:"interface_index,omitempty"`
    Ownership      Ownership `json:"ownership"`
    FaultID        string    `json:"fault_id,omitempty"`
    LastError      string    `json:"last_error,omitempty"`
}
```

### 安全边界

外部虚拟网卡存在：

```text
≠
Navo 必须清除
```

它可能来自：

- Clash
- VPN
- Hyper-V
- WSL
- Docker
- VMware
- VirtualBox
- 企业网络工具

所以：

> Environment 可以观察；Self-Healing 不允许修改不属于 Navo 的 adapter。

---

# 10. DNS / Route / NRPT / Firewall 的处理

这里不要重新制造 ownership 系统。

现有：

`internal/network/manager.go`

已经是：

> **Navo-created TUN network resources 的唯一 owner**

现有 Network Journal V2 也已经记录：

- endpoint route
- split route
- NRPT
- firewall
- CreatedByNavo
- interface identity
- session ID

因此 Environment 的正确做法是：

> **读取并汇总现有 ownership / journal / verification 结果。**

不要另建：

```text
environment_routes.json
environment_dns.json
environment_ownership.json
```

否则会出现两个真相源。

---

# 11. Network Journal 仍然是恢复权威

现有：

`internal/network/journal.go`

已经明确：

> V2 Journal 是恢复 authority，不允许执行磁盘中持久化的任意命令。

这一点必须保留。

Environment 可以产生类似：

```text
journal_present = true
journal_session_id = xxx
journal_dirty = true
```

但是：

> Environment 不执行 Journal。

真正恢复继续调用现有 Network Manager / Reconciler。

---

# 12. Collector 分层

Windows 网络状态有两个权限域。

因此不要硬塞到一个 Collector 里面。

建议：

```text
User Scope
    ↓
Agent
    ↓
WinINet / System Proxy

Machine Scope
    ↓
Service
    ↓
TUN / Route / DNS / NRPT / Firewall / adapters
```

最终在 Agent 聚合：

```text
Agent
├── User-scoped observation
│   └── System Proxy
│
├── Service machine observation
│   ├── TUN
│   ├── physical adapter
│   ├── Route
│   ├── DNS
│   ├── NRPT
│   └── network journal state
│
└── networkenv.Analyzer
        ↓
    final Snapshot
```

---

# 13. 推荐新增 Service 只读接口

建议新增 IPC 方法：

```text
network.observe
```

不要命名：

```text
network.fix
network.clean
network.rescue
```

`network.observe` 必须只读。

返回 machine-scoped snapshot。

例如：

```json
{
  "physical": {},
  "tun": {},
  "dns": {},
  "routes": {},
  "journal": {}
}
```

### 建议代码位置

优先：

```text
internal/service/network_observe.go
internal/service/network_observe_windows.go
internal/service/network_observe_stub.go
internal/service/network_observe_test.go
```

如果当前 `internal/network` 已经存在适合的只读 Platform 方法，优先复用。

如果没有，再新增只读 observer。

不要为了 Environment 去绕过 `internal/network` 的 ownership 结构直接做另一套恢复实现。

---

# 14. Agent Environment 聚合器

新增：

```text
internal/agent/environment.go
internal/agent/environment_test.go
```

职责：

1. 读取 `systemproxy.CurrentConfig()`
2. 读取当前 Navo System Proxy ownership
3. 调用 Service `network.observe`
4. 读取 Connection Coordinator Snapshot
5. 读取 Capture Snapshot
6. 交给 `networkenv.Analyzer`
7. 存入 `networkenv.Store`

建议 Agent 增加：

```go
environment *networkenv.Store
```

或者：

```go
environmentStore *networkenv.Store
```

---

# 15. Environment Store

避免 UI、health monitor、Self-Healing 每次都自己重复扫描 Windows。

建议：

```go
type Store struct {
    mu       sync.RWMutex
    snapshot Snapshot
}

func (s *Store) Load() Snapshot
func (s *Store) Publish(Snapshot)
```

必须：

- copy-on-read / defensive copy
- 不返回内部可变 slice/map 引用
- 支持初始 unknown/checking 状态

---

# 16. 刷新策略

不要每 2 秒完整执行一整套重型 PowerShell Route/DNS/Firewall 扫描。

建议分成两类。

## Fast Observation

周期：

```text
2 秒左右
```

内容：

- raw System Proxy
- Navo ownership
- capture state
- coordinator transaction
- TUN status
- core/capture 已知状态
- physical link 简要状态

---

## Deep Observation

周期：

```text
30 秒左右
```

以及以下事件强制触发：

- Navo 启动完成
- Capture mode 切换完成
- Capture mode 切换失败
- Core switch 完成
- Outbound switch 完成
- Self-Healing round 完成
- Self-Healing 最终成功
- Self-Healing 最终失败
- UI 用户手动刷新
- resume / network change（如果现有事件机制方便接入）

内容：

- Route
- DNS
- NRPT
- Firewall ownership
- Network Journal
- deeper connectivity evidence

如果第一阶段为了降低改动复杂度，可以先统一 5 秒刷新。

但必须满足：

> 观察失败不能卡死 UI，也不能阻塞 Coordinator 的网络事务。

---

# 17. Analyzer

新增：

```text
internal/networkenv/analyzer.go
```

Analyzer 只做：

```text
Facts
↓
Rules
↓
Findings
```

不执行修复。

建议：

```go
type Finding struct {
    Code         string    `json:"code"`
    Severity     Severity  `json:"severity"`
    Domain       string    `json:"domain"`
    Summary      string    `json:"summary"`
    Detail       string    `json:"detail,omitempty"`
    Ownership    Ownership `json:"ownership"`
    Recoverable  bool      `json:"recoverable"`
    Transitional bool      `json:"transitional"`
}
```

---

# 18. 第一阶段 Finding 建议

至少支持：

```text
ENV_SYSTEM_PROXY_EXTERNAL
ENV_SYSTEM_PROXY_STALE_NAVO
ENV_SYSTEM_PROXY_ENDPOINT_UNREACHABLE

ENV_EXTERNAL_TUN_PRESENT
ENV_NAVO_TUN_MISSING
ENV_NAVO_TUN_DISABLED

ENV_NAVO_ROUTE_RESIDUAL
ENV_NAVO_DNS_INCONSISTENT
ENV_NAVO_NRPT_INCONSISTENT
ENV_NAVO_FIREWALL_INCONSISTENT

ENV_NETWORK_JOURNAL_PENDING

ENV_PHYSICAL_NETWORK_UNAVAILABLE
ENV_CAPTURE_DATAPLANE_FAILED

ENV_OBSERVATION_PARTIAL
```

---

# 19. 不能把“External”自动标成 Error

例：

```text
Navo = Off
Windows Proxy = 127.0.0.1:10808
Ownership = External
```

这是：

```text
INFO
```

而不是：

```text
ERROR
```

因为用户可能就是在使用其他代理软件。

只有例如：

```text
Navo 正在 System Proxy 模式
但 Windows Proxy 已经被外部程序改写
```

才是：

```text
Conflict / Navo ownership lost
```

Navo 的动作也不是“夺回来”。

正确动作：

```text
停止认为自己拥有该资源
↓
进入安全状态 / fail closed
↓
提示用户网络已经由外部程序接管
```

---

# 20. Environment → Self-Healing 适配

不要让 `internal/networkenv` import `internal/selfheal`。

避免形成：

```text
Environment
↓
SelfHeal
↓
Environment
```

这种耦合。

建议在 Agent 增加 adapter：

```text
internal/agent/environment_faults.go
```

职责：

```text
networkenv.Finding
↓
selfheal.FaultEvidence
```

例如：

```text
ENV_NAVO_TUN_MISSING
↓
FaultDomainTUN
↓
CodeTUNAdapterMissing
```

```text
ENV_NAVO_DNS_INCONSISTENT
↓
FaultDomainDNS
↓
CodeDNSMismatch
```

```text
ENV_PHYSICAL_NETWORK_UNAVAILABLE
↓
FaultDomainPhysicalNetwork
↓
CodePhysicalNetworkDown
```

---

# 21. Self-Healing 自动修复边界

| 场景 | Environment | Self-Healing |
|---|---|---|
| 外部 System Proxy 正常存在，Navo Off | 提示 | 不修 |
| 外部 TUN 存在，Navo Off | 提示 | 不修 |
| Navo ownership marker 存在但本地 core 已不存在 | 报 stale Navo | 可修 |
| Navo TUN session 对应 adapter 丢失 | 报 Navo fault | 可修 |
| Navo-owned Route 缺失 | 报 Navo fault | 可修 |
| Navo-owned Route 残留 | 报 Navo residual | 可修 |
| Navo-owned DNS/NRPT 不一致 | 报 Navo fault | 可修 |
| 第三方改变了 System Proxy | ownership lost | 不覆盖第三方 |
| 物理网卡断开 | physical fault | 不修改 |
| 网关不可达 | physical fault | 不修改 |
| 检测本身失败 | detection | 不盲修 |
| 无法确定归属 | unknown | 不盲修 |

原则：

> **不能证明是 Navo 的，就不自动动。**

---

# 22. 用 Environment 替换脆弱的字符串分类

当前：

`internal/agent/capture_transition.go`

中的：

```go
classifyCaptureFault(...)
```

大量依赖：

```go
strings.Contains(...)
```

例如根据 error message 中是否出现：

- dns
- route
- adapter
- system proxy
- core
- listener

来判断 FaultDomain。

这可以作为 fallback。

但长期不是稳定的领域协议。

本次建议：

### 第一阶段

新增结构化环境 Finding → FaultEvidence 映射。

优先：

```text
typed Environment Finding
```

如果没有对应 Finding：

```text
fallback 到 classifyCaptureFault(error)
```

### 迁移完成后

`classifyCaptureFault` 只承担：

> 未结构化旧错误兼容。

这样以后不会因为错误文案变化就把：

```text
DNS fault
```

识别成：

```text
Node fault
```

---

# 23. Connection Coordinator 如何使用 Environment

Coordinator 本身不需要依赖 Environment。

Agent 在：

```text
beginConnection()
```

之前/之后读取最新 Snapshot。

不要修改：

```text
connection.Coordinator
```

让它变成网络扫描器。

建议连接流程：

```text
User request
↓
读取 latest Environment Snapshot
↓
做安全前置判断
↓
Coordinator.Begin(...)
↓
existing transition
↓
Verify
↓
Finish
↓
强制 refresh Environment
```

---

# 24. Capture 开启前的 Environment 判断

例如用户点击：

```text
System Proxy
```

如果 Environment：

```text
External System Proxy active
Navo currently off
```

不要静默覆盖。

第一阶段建议行为：

### 安全策略

如果用户明确点击：

```text
开启 Navo System Proxy
```

这是主动 takeover intent。

可以：

1. 保存当前外部 System Proxy 为 pre-Navo snapshot；
2. Navo 获取 ownership；
3. 运行期间保护 ownership；
4. Navo 退出/关闭时恢复原配置；
5. 如果运行期间第三方再次修改，则 Navo 不覆盖新的第三方配置。

这与当前 `systemproxy.Manager` 的 backup/ownership 思路一致。

Environment 负责把这个事实展示清楚。

---

# 25. 启动 Navo ≠ 自动开启代理

这一条继续保留。

当前 `cmd/navo/main.go` 已经使用：

```go
DeferCoreStart: true
```

所以设计上已经接近该原则。

以后启动流程应该表达成：

```text
启动 Navo
↓
恢复可以证明属于 Navo 的未完成事务
↓
读取当前 Environment
↓
显示状态
↓
保持 Capture Off
↓
等待用户操作
```

注意这里有一个重要区别：

> **“不自动开启代理”不等于“启动时绝对不能修改任何网络”。**

如果上次 Navo 崩溃，并且 Network Journal / ownership marker 能证明：

```text
这些残留资源是 Navo 创建的
```

那么启动时 Self-Healing 允许做 ownership-safe recovery。

这是恢复旧事务，不是自动开启新代理。

---

# 26. 当前 Startup Recovery 不要删除

现有 Service 启动阶段会恢复 Network Journal。

现有 Agent 启动阶段也有：

```go
recoverCaptureOnStartup()
```

并且使用：

```text
OperationRecovery
OriginStartup
```

这套机制不要为了增加 Environment 而删除。

本次应该做：

```text
existing startup recovery
+
Environment observation
+
better evidence
```

而不是：

```text
Environment 替换 Journal Recovery
```

---

# 27. 推荐启动顺序

在不破坏现有结构的前提下：

```text
1. initialization.Run
2. Service New
3. Service startup
4. existing Network Journal recovery
5. Service ready
6. Agent New
7. Agent UI pipe ready
8. publish Environment = checking
9. initial Environment observation
10. existing Agent capture startup recovery
11. refresh Environment
12. start environment monitor
13. UI render final snapshot
```

如果 Service Recovery 在 Environment 第一次完整观察之前发生：

> 允许。

因为它依据的是 Navo 自己的 Journal/ownership，不是无来源猜测。

---

# 28. 正常连接流程

## System Proxy

```text
Environment snapshot
↓
User requests System Proxy
↓
Connection Coordinator
↓
Capture transaction
↓
Service prepare core
↓
SystemProxy.Manager.Enable
↓
verify ownership
↓
verify proxy path
↓
commit
↓
refresh Environment
```

---

## TUN

```text
Environment snapshot
↓
User requests TUN
↓
Connection Coordinator
↓
Capture transaction
↓
Service prepare core
↓
Network Manager activation
↓
TUN
↓
Route / NRPT / Firewall / DNS
↓
control-plane verification
↓
data-plane verification
↓
commit
↓
refresh Environment
```

---

# 29. 异常流程

```text
Environment / existing health check
↓
发现 Navo-owned fault
↓
生成 typed FaultEvidence
↓
释放普通 user transaction
↓
Self-Healing TryBegin(OperationSelfHeal)
↓
repair round 1
↓
verify
↓
失败则 rollback / round 2
↓
必要时 same-channel failover
↓
仍失败
↓
fail closed
↓
refresh Environment
```

现有 `recoverUnhealthyCapture` 的基本框架继续保留。

---

# 30. Self-Healing 就是异常总管

以下行为统一视为 Self-Healing：

- startup recovery
- Capture activation failure recovery
- TUN adapter missing
- TUN disabled
- Route recovery
- DNS recovery
- NRPT recovery
- Firewall recovery
- System Proxy ownership fault
- stale Navo ownership
- Network Journal dirty recovery
- Capture data-plane recovery
- Core abnormal recovery
- failover
- fail closed

不要再创建：

```text
RescueManager
RescueMode
EmergencyNetworkRepair
NetworkDoctorController
```

如果 UI 需要：

```text
[立即修复]
```

它只是：

> Self-Healing 的手动触发入口。

不是新模块。

---

# 31. `repair.exe` 的定位

当前：

`cmd/repair/main.go`

已经明确：

- `repair check` = read-only
- `repair fix` = read-only report
- `repair reset` = read-only report
- 真正 mutation 由 runtime Reconciler 负责

这个方向是正确的。

因此：

> `repair.exe` 不等于 Rescue Mode。

建议未来让它尽量复用 `networkenv` 的模型/诊断规则，输出同类 Finding。

但是本次不要强行让离线 repair 工具执行自动修复。

---

# 32. Dashboard DTO

当前 Dashboard 已经有：

- core
- cores
- proxy
- runtime
- tun
- capture
- metrics
- ip

新增：

```text
environment
```

建议：

```go
"environment": a.environmentSnapshot()
```

位置：

`internal/agent/dashboard.go`

---

# 33. UI TypeScript DTO

当前：

`navo_app/frontend/src/types.ts`

已经存在：

```ts
export type NetworkHealthState =
  | "unknown"
  | "checking"
  | "healthy"
  | "degraded"
  | "unavailable";
```

可以直接复用。

新增：

```ts
export type NetworkEnvironmentOwnership =
  | "none"
  | "navo"
  | "external"
  | "unknown";
```

以及：

```ts
export interface NetworkEnvironmentFinding {
  code: string;
  severity: "info" | "warning" | "error";
  domain: string;
  summary: string;
  detail?: string;
  ownership: NetworkEnvironmentOwnership;
  recoverable: boolean;
  transitional: boolean;
}
```

然后：

```ts
export interface NetworkEnvironmentSnapshot {
  version: number;
  collected_at: string;
  health: NetworkHealthState;
  stale: boolean;
  ...
}
```

Dashboard 第一阶段建议：

```ts
environment?: NetworkEnvironmentSnapshot;
```

保持前后端灰度兼容。

稳定后可以转成 required。

---

# 34. UI 不要继续把所有东西堆进 App.vue

当前：

`navo_app/frontend/src/App.vue`

体积已经非常大。

本次新增 Environment UI 时，建议新增组件：

```text
navo_app/frontend/src/components/NetworkEnvironmentCard.vue
```

如果需要详情：

```text
NetworkEnvironmentDetails.vue
```

App.vue 只负责组合。

---

# 35. UI 第一阶段应该长什么样

用户不需要看到所有 Windows 细节。

Overview 建议：

```text
网络环境
● 正常

System Proxy
Navo

TUN
未启用

DNS
正常

网络控制
Navo

冲突
无
```

外部代理：

```text
网络环境
● 正常

System Proxy
外部代理正在使用

Navo
未接管

冲突
无
```

残留：

```text
网络环境
⚠ 检测到 Navo 网络残留

Route
存在未完成恢复

System Proxy
正常

[立即修复]
```

物理网络断开：

```text
网络环境
● 无网络

物理网络不可用

Navo 不会修改物理网卡配置
```

---

# 36. “修复”按钮的行为

UI：

```text
[立即修复]
```

只能触发：

```text
Self-Healing
```

建议新增 Agent 方法，例如：

```text
environment.repair
```

但只允许处理 Snapshot 中：

```text
Recoverable == true
Ownership == navo
```

的 finding。

不能设计成：

```text
一键清除所有代理
重置所有 DNS
清空所有 Route
禁用所有虚拟网卡
```

这是禁止项。

---

# 37. 手动修复与自动修复应共用同一动作

不要：

```text
自动修复 → SelfHeal path
手动修复 → 独立脚本
```

应该：

```text
Environment Finding
        │
        ├── automatic trigger
        │
        └── manual trigger
                ↓
         same Self-Healing orchestration
```

保证行为一致。

---

# 38. 现有 monitorCaptureHealth 的改造方式

当前：

`monitorCaptureHealth()`

每 2 秒检查 Capture。

不要直接删除。

第一阶段改成：

```text
monitorCaptureHealth
↓
优先读取最新 Environment Snapshot
↓
如果 Snapshot 过期/证据不足
↓
执行现有 targeted probe
↓
生成 typed fault
```

这样风险最小。

最终可以逐步让 Environment 成为主要事实源。

---

# 39. 观察期间的过渡态抑制

当：

```text
coordinator.Busy == true
```

并且 Phase 是：

- applying
- committing
- rolling_back

Environment Analyzer 不应该马上把中间状态当持久故障。

Finding 应：

```text
Transitional = true
```

或者暂时不升级为 recoverable fault。

只有：

- transaction 完成后仍然存在；
- transaction failed；
- 超过合理窗口；
- 明确违反 ownership；

才触发 Self-Healing。

---

# 40. Snapshot 新鲜度

建议：

```go
const SnapshotStaleAfter = 10 * time.Second
```

具体值可根据现有轮询调整。

如果：

```text
now - CollectedAt > threshold
```

UI 显示：

```text
状态更新中
```

Self-Healing 不应该因为一份严重过期的 Snapshot 直接修改网络。

必须重新确认。

---

# 41. Self-Healing 二次确认原则

即使 Environment 发现：

```text
ENV_NAVO_ROUTE_RESIDUAL
```

真正修之前，现有 repair path 仍然应该：

```text
FaultPresent / current state verification
```

不能：

```text
10 秒前 Snapshot 说有问题
↓
现在直接删 Route
```

Snapshot 是证据。

不是 mutation authority。

---

# 42. 物理网络永远不是 Self-Healing 修改对象

如果：

- Ethernet down
- Wi-Fi down
- gateway down
- DHCP 出问题
- 上游路由不可达

Environment 可以检测。

Self-Healing 可以：

- 提示
- 等待
- 暂停代理重试
- fail closed

但不能：

- 重置用户网卡
- 改用户静态 IP
- 改网关
- 改 DHCP
- 禁用/启用物理网卡

除非未来有一个明确、独立、用户主动授权的功能。

本次禁止。

---

# 43. 需要修改的文件清单

## 新增

```text
internal/networkenv/model.go
internal/networkenv/analyzer.go
internal/networkenv/store.go
internal/networkenv/collector.go
internal/networkenv/analyzer_test.go
internal/networkenv/store_test.go

internal/agent/environment.go
internal/agent/environment_faults.go
internal/agent/environment_test.go

internal/service/network_observe.go
internal/service/network_observe_windows.go
internal/service/network_observe_stub.go
internal/service/network_observe_test.go

navo_app/frontend/src/components/NetworkEnvironmentCard.vue
```

文件名可根据当前 HEAD 小幅调整。

---

## 重点修改

```text
internal/agent/agent.go
internal/agent/dashboard.go
internal/agent/capture_transition.go

internal/service/service.go

navo_app/frontend/src/types.ts
navo_app/frontend/src/App.vue
navo_app/frontend/src/state.ts
```

如果 UI 状态并不在 `state.ts` 管理，则按当前源码适配。

---

## 尽量不大改

```text
internal/connection/coordinator.go
internal/selfheal/engine.go
internal/selfheal/recovery.go
internal/network/manager.go
internal/network/journal.go
internal/agent/systemproxy/proxy.go
```

这些是现有安全基础。

除非为了暴露只读状态或增加小接口，否则不要重构其核心语义。

---

# 44. 推荐实施阶段

---

## Phase 1：只做模型和只读 Snapshot

目标：

> 不改变任何网络行为。

完成：

- `internal/networkenv`
- raw System Proxy
- Navo ownership
- current TUN status
- Coordinator transaction
- basic physical network
- Dashboard `environment`
- UI 卡片

此阶段：

```text
Self-Healing 行为完全不改
```

先验证 Environment 的观察结果正确。

---

## Phase 2：增加 machine-scoped read-only observation

增加：

- Route summary
- DNS summary
- NRPT summary
- Firewall ownership summary
- Network Journal summary
- external virtual adapter summary

仍然：

```text
只读
```

---

## Phase 3：Environment Findings

增加 Analyzer：

```text
facts
↓
findings
```

UI 可以显示：

- external
- stale Navo
- Navo fault
- physical fault
- observation partial

此阶段仍不替换自愈。

---

## Phase 4：Environment → Self-Healing typed adapter

新增：

```text
environment_faults.go
```

优先使用 typed finding 生成：

```text
selfheal.FaultEvidence
```

保留：

```text
classifyCaptureFault(error)
```

作为 fallback。

---

## Phase 5：健康监控接入

让：

```text
monitorCaptureHealth
```

优先读取 Environment。

降低重复探测。

仍保留必要 targeted verification。

---

## Phase 6：手动修复入口

UI：

```text
[立即修复]
```

走现有 Self-Healing。

不要新建 Rescue。

---

# 45. 每个阶段必须能独立提交

建议 commits：

```text
feat(networkenv): add read-only network environment model

feat(service): expose machine network observation snapshot

feat(agent): aggregate user and machine network environment

feat(ui): show network environment status

refactor(selfheal): map environment findings to typed fault evidence

refactor(agent): use environment snapshot in capture health monitoring

feat(ui): route manual repair through self-healing
```

不要一个提交同时重写：

- Coordinator
- SelfHeal
- UI
- Network Manager
- TUN

否则出现回归很难定位。

---

# 46. 必须新增的测试

## networkenv Analyzer

至少覆盖：

### Case 1

```text
Navo off
External System Proxy on
```

结果：

```text
info
not recoverable
```

---

### Case 2

```text
Navo System Proxy active
raw Windows Proxy == owned endpoint
```

结果：

```text
healthy
```

---

### Case 3

```text
Navo ownership marker exists
raw proxy no longer matches
```

结果：

```text
ownership lost
do not overwrite external config
```

---

### Case 4

```text
Navo TUN expected
adapter missing
```

结果：

```text
ENV_NAVO_TUN_MISSING
recoverable
```

---

### Case 5

```text
External TUN exists
Navo off
```

结果：

```text
info
not recoverable
```

---

### Case 6

```text
physical network down
```

结果：

```text
unavailable
not recoverable by Navo network mutation
```

---

### Case 7

```text
Coordinator applying
Route temporarily incomplete
```

结果：

```text
transitional
no premature self-heal
```

---

### Case 8

```text
Network Journal pending
owned Navo resources exist
```

结果：

```text
recoverable
ownership = navo
```

---

# 47. System Proxy 回归测试

必须保证：

### External takeover preservation

1. Navo 开启 System Proxy。
2. 另一个程序把 WinINet 改成另一 endpoint。
3. 用户关闭 Navo。
4. Navo 不得把新的第三方 endpoint 清掉。

现有 `restoreOwned()` 的语义必须保留。

---

# 48. TUN / Route 回归测试

必须保证：

1. Navo 不删除非 Navo Route。
2. Navo 不删除非 Navo Firewall rule。
3. Navo 不修改非 Navo TUN adapter。
4. Journal 中 `CreatedByNavo=false` 的资源不能按 Navo-created 资源删除。
5. crash recovery 后只回滚自己的 transaction。
6. rollback 失败进入 dirty/fault，而不是假装成功。

---

# 49. Self-Healing 回归测试

必须保证：

- 最多两轮完整 repair
- retry budget 不失效
- circuit breaker 不失效
- verification 仍然存在
- failed mutation 仍然 rollback
- node failover 仍然只走合法候选
- physical network 不自动修
- unknown 不自动修
- detection fault 不自动乱修
- Coordinator busy 时不能并发 network mutation

---

# 50. UI 测试

现有 `navo_app/package.json` 支持：

```bash
npm test
npm run typecheck
npm run build
```

必须补：

- environment DTO parser / fallback
- missing environment field 兼容
- healthy UI
- external proxy UI
- recoverable Navo residual UI
- unavailable physical network UI

---

# 51. Go 测试

至少：

```bash
go test ./...
```

Windows 侧如果当前工程已有特定 integration tag，则继续运行现有方式。

新增代码要：

```bash
gofmt
go test ./...
```

并确保：

```text
non-Windows stub
```

可以编译。

当前 `go.mod` 使用：

```text
Go 1.26.4
```

不要引入超出当前工程需要的新依赖来完成简单状态观察。

---

# 52. 前端验证

进入：

```text
navo_app
```

运行：

```bash
npm test
npm run typecheck
npm run build
```

全部通过。

---

# 53. Windows 手工验收矩阵

Codex 修改完成后，应在最终说明中列出以下人工验证状态。

## A. 干净环境

```text
System Proxy off
Navo TUN off
```

预期：

```text
Environment = healthy
Navo = not owner
```

---

## B. v2rayN / Clash 类外部 System Proxy

预期：

```text
Environment sees external proxy
Navo does not modify it merely because Navo started
```

---

## C. Navo System Proxy

预期：

```text
raw state = enabled
owner = navo
capture = system_proxy
```

---

## D. 外部程序中途接管

预期：

```text
Navo detects ownership loss
does not overwrite external endpoint on shutdown
```

---

## E. Navo TUN

预期：

```text
Environment sees Navo adapter/session
Route/DNS state coherent
```

---

## F. 手动禁用 Navo TUN adapter

预期：

```text
Environment detects Navo-owned fault
Self-Healing handles it
No separate rescue mode
```

---

## G. 外部 VPN/TUN

预期：

```text
Environment can report it
Navo does not destroy/disable it
```

---

## H. Navo crash / forced kill

预期：

```text
next startup performs ownership-safe recovery
Environment refreshes after recovery
```

---

## I. 物理断网

预期：

```text
Environment reports physical network unavailable
Self-Healing does not reset physical adapter
```

---

# 54. 日志

Environment 建议使用现有 structured log。

建议事件：

```text
NETWORK_ENV_REFRESHED
NETWORK_ENV_REFRESH_FAILED
NETWORK_ENV_FINDING_ADDED
NETWORK_ENV_FINDING_CLEARED
NETWORK_ENV_STALE
```

不要每 2 秒把完整 Snapshot 写一遍日志。

只记录：

- 状态变化
- finding 变化
- refresh error
- ownership 变化

防止日志爆炸。

---

# 55. 隐私

Environment 不应默认收集：

- 浏览器访问历史
- 用户域名历史
- 第三方代理配置内容
- 第三方程序完整配置文件
- 凭据
- 订阅 URL
- 用户名/密码

如果后续要做第三方 process attribution，只保存必要的本地运行证据。

---

# 56. 性能边界

Environment 不允许成为新的性能热点。

建议：

- fast observation 有 deadline
- deep observation 有 deadline
- 单个 collector 失败不拖死整体
- partial snapshot 允许存在
- UI 永远读取缓存
- UI request 不直接触发无限制 PowerShell 扫描
- Service observation 不阻塞 mutation lock 超长时间

---

# 57. Partial Snapshot

某个 collector 失败时：

不要：

```text
整个 Environment API 返回 500
```

应该尽可能返回：

```text
Snapshot
health = degraded / unknown
finding = ENV_OBSERVATION_PARTIAL
```

并记录：

```text
哪个 collector 失败
```

除非连最基本 Snapshot 都无法构造。

---

# 58. 单一事实源原则

必须坚持：

```text
System Proxy raw truth
→ Windows read API

Navo System Proxy ownership
→ existing ownership record/manager

TUN/Route/DNS/NRPT/Firewall ownership
→ existing Network Manager + V2 Journal

Connection mutation state
→ existing Connection Coordinator

Capture lifecycle
→ existing capture.Snapshot

Self-Healing state
→ existing RecoveryReport
```

Environment：

> **聚合这些事实。**

Environment：

> **不重新发明这些事实。**

---

# 59. 本次最容易犯的错误

## 错误 1

新建：

```text
EnvironmentManager.EnableProxy()
```

禁止。

---

## 错误 2

Environment 发现 127.0.0.1:10808：

```text
自动 DisableProxy()
```

禁止。

---

## 错误 3

Environment 检测到 Clash adapter：

```text
Delete adapter
```

禁止。

---

## 错误 4

新增：

```text
RescueMode
```

禁止。

---

## 错误 5

再做：

```text
NewConnectionCoordinator
```

禁止。

---

## 错误 6

同时让 Agent 和 Service 对同一网络 fault 自动修复。

第一阶段禁止。

---

## 错误 7

为方便测试把 ownership 检查删掉。

禁止。

---

## 错误 8

把 physical network fault 当 Navo fault。

禁止。

---

# 60. Definition of Done

只有满足以下全部条件，任务才算完成。

- [ ] 新增统一 Network Environment Snapshot。
- [ ] Environment 全部观察操作只读。
- [ ] raw System Proxy 和 Navo ownership 明确分离。
- [ ] External System Proxy 不会因为启动 Navo 被清除。
- [ ] External TUN/VPN 不会被 Navo 自动删除或禁用。
- [ ] 现有 Connection Coordinator 保持唯一 mutation transaction boundary。
- [ ] 未新增 Rescue Mode。
- [ ] Self-Healing 仍是所有异常恢复的统一概念入口。
- [ ] Environment finding 可以映射到 typed Self-Healing fault evidence。
- [ ] 字符串错误分类仍保留为 fallback，不再是唯一依据。
- [ ] Environment Snapshot 已加入 Agent Dashboard。
- [ ] UI 能显示 healthy / external / Navo residual / unavailable。
- [ ] UI“修复”入口只调用现有 Self-Healing。
- [ ] Snapshot 支持 stale / partial / transitional。
- [ ] startup recovery 不被破坏。
- [ ] Network Journal V2 authority 不被破坏。
- [ ] System Proxy restore ownership 语义不被破坏。
- [ ] TUN/Route/DNS/NRPT/Firewall 只修改 Navo-owned 资源。
- [ ] `go test ./...` 通过。
- [ ] `npm test` 通过。
- [ ] `npm run typecheck` 通过。
- [ ] `npm run build` 通过。
- [ ] Windows 手工验收矩阵有结果记录。
- [ ] 修改后的 README/架构说明不再出现独立“救援模式”概念。

---

# 61. Codex 最终输出要求

Codex 完成实现后，不要只回复：

```text
Done
```

必须给出：

## 1. 实际修改

按文件列出：

```text
file
what changed
why
```

## 2. 架构结果

明确回答：

```text
Environment 在哪里
Coordinator 是否复用现有实现
Self-Healing 如何接管异常
Agent / Service 各做什么
```

## 3. Safety

明确说明：

- external proxy 是否会被修改
- external TUN 是否会被修改
- physical network 是否会被修改
- ownership 如何验证

## 4. Tests

列出实际运行：

```text
go test ./...
npm test
npm run typecheck
npm run build
```

及结果。

## 5. 未完成项

如果某个 Windows-specific 场景无法在当前环境验证：

必须明确写：

```text
Not verified
```

不要假装通过。

---

# 62. 最终架构

完成后，Navo 的网络控制逻辑应该清晰成：

```text
                    Windows
                       │
                       ▼
              Network Environment
              只读观察 / 统一事实
                       │
                       ▼
              Environment Snapshot
                       │
         ┌─────────────┼─────────────┐
         │             │             │
         ▼             ▼             ▼
        UI       Normal Flow     Fault Evidence
                       │             │
                       ▼             ▼
               Connection       Self-Healing
               Coordinator          │
                       │             │
                       └──────┬──────┘
                              ▼
                         Agent / Service
                              │
                   ┌──────────┼──────────┐
                   ▼          ▼          ▼
              System Proxy    TUN    Route/DNS/NRPT
```

一句话：

> **Environment 负责看清 Windows；Coordinator 负责正常事务；Self-Healing 负责异常接管；Agent / Service 负责执行，而且任何自动恢复都必须建立在 Navo ownership 之上。**

---

# 63. 本次改造的最终产品目标

改造前，Navo 更接近：

```text
拥有很多网络控制与恢复能力
但观察结果分散在多个模块
```

改造后应该变成：

```text
统一环境感知
↓
统一事实快照
↓
正常流程与异常流程共享同一份证据
↓
Coordinator 处理正常变化
Self-Healing 处理异常变化
↓
只操作 Navo 自己拥有的网络资源
```

这不是增加一套新系统。

而是把 Navo 已有的：

- Coordinator
- Capture transaction
- ownership
- Network Journal
- Self-Healing
- Agent
- Service
- UI

真正连接成一套完整、可解释、可扩展的 Windows 网络控制体系。
