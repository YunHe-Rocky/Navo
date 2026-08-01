# Navo 后台自动纠错与自愈引擎作业指导书（Claude / Codex 执行版）

> 适用仓库：`YunHe-Rocky/Navo`  
> 执行对象：Claude Code、Codex 或同类编码智能体  
> 目标平台：Windows 10 / Windows 11  
> 功能定位：后台错误识别、自动纠正、自动修复、修复验证、失败熔断  
> UI 要求：本功能不新增独立 UI 页面，不弹出普通成功提示；所有过程通过结构化日志和现有状态接口记录  
> 隐私要求：完全本地执行，不使用 AI，不上传日志，不依赖 MySQL 或任何云端服务

---

# 0. 本文档与现有文档的关系

本文件必须与以下文档配合执行：

```text
1. Navo_Codex_Full_Remediation_Guide.md
2. Navo_Codex_Feature_Optimization_Guide.md
3. Navo_Codex_Precise_Cleanup_Integration_Privacy_Init_Guide.md
```

本文不替代前三份文档。

## 0.1 第一份文档负责

```text
TUN 路由修复
Capture 事务
系统代理事务
核心生命周期
Supervisor
网络恢复
Service 权限
订阅安全
配置校验
IPC
发布门禁
```

本文中的自动修复只能调用第一份文档完成后的安全接口，不能重新实现一套旁路逻辑。

## 0.2 第二份文档负责

```text
结构化日志
日志等级
内部服务分类
日期查询
核心升级
测速
流量监测
双联链路
IP 风险
托盘和图标
```

本文依赖第二份文档提供的结构化日志能力。

## 0.3 第三份文档负责

```text
删除 MySQL
删除 AI
本地 repository
初始化
设备绑定
新电脑隐私清理
死代码清理
```

本文必须完全本地运行，不得重新引入 AI、远程日志分析、MySQL 或云端诊断。

## 0.4 冲突处理

如果本文提出的自动修复与前三份文档冲突：

```text
安全事务和真实状态优先；
自动修复不得绕过 Runtime Transition Coordinator；
自动修复不得直接调用危险底层命令；
自动修复失败后必须保留 faulted/dirty；
不得为了自动恢复而伪造成功状态。
```

---

# 1. 目标定义

Navo 需要一个后台自愈模块，负责处理可预测、可验证、可安全回滚的常见错误。

典型场景：

```text
切换核心失败
核心配置无效
核心进程崩溃
核心端口未监听
核心二进制丢失或损坏
TUN 网卡被禁用
TUN 网卡丢失
端点旁路路由缺失
路由或 DNS 状态不一致
系统代理指向已停止端口
节点切换后代理不可用
双联链路采集器无数据
流量采集器卡死
IP 检测任务卡住
订阅临时网络失败
日志写入失败
```

自动修复流程必须是：

```text
结构化日志产生错误事件
        ↓
错误归一化和聚合
        ↓
读取当前真实状态
        ↓
判断错误是否仍然存在
        ↓
匹配安全修复策略
        ↓
申请资源锁和修复预算
        ↓
执行有限修复
        ↓
验证真实结果
        ↓
成功：记录已恢复
失败：回滚、退避或熔断
```

日志只能作为触发信号，不能作为唯一事实来源。

---

# 2. 核心设计原则

## 2.1 日志触发，状态确认

错误日志出现后必须再次检查：

```text
进程状态
端口状态
核心控制接口
路由表
DNS
系统代理
TUN 适配器
Capture committed state
Network journal
当前节点和核心
采集器最后更新时间
```

例如：

```text
日志显示 core start timeout
但端口随后已经监听
```

这种情况不得再次重启核心。

## 2.2 只能修复已知错误

自动修复必须基于：

```text
稳定错误码
明确前置条件
明确修复动作
明确验证方法
明确回滚方法
明确最大次数
```

未知错误：

```text
只记录
聚合
进入人工诊断或 faulted
```

不得根据任意日志文本猜测并执行危险命令。

## 2.3 有限自动化

每个错误策略必须定义：

```text
单次最多动作数
时间窗口内最大重试次数
最小退避时间
最大退避时间
熔断时间
是否允许重启核心
是否允许修改路由
是否允许修改系统代理
是否要求用户重新选择节点
```

## 2.4 不绕过事务协调器

涉及以下操作时，只能调用统一协调器：

```text
Capture 模式改变
核心切换
节点切换
TUN 重建
系统代理修改
路由和 DNS 修改
核心配置提交
```

禁止自动修复模块直接执行：

```text
New-NetRoute
Remove-NetRoute
netsh
注册表代理写入
裸 core.stop
裸 core.start
```

## 2.5 修复后必须验证

不能因为函数返回 `nil` 就认为修复成功。

必须执行至少一种真实验证：

```text
核心端口监听
核心控制器正常
通过代理完成 HTTPS 请求
TUN 适配器 Up
Navo-owned 路由与目标一致
DNS 与目标一致
系统代理与 committed mode 一致
Direct/Proxy 链路返回新样本
采集器更新时间恢复
```

## 2.6 失败必须可见

自动修复不需要单独 UI，但必须通过日志暴露：

```text
检测到什么错误
采用了什么修复策略
第几次尝试
修复是否成功
验证结果
是否回滚
是否进入熔断
下一次允许尝试时间
是否需要人工处理
```

---

# 3. 建议模块结构

建议新增：

```text
internal/selfheal/
  engine.go
  detector.go
  classifier.go
  coordinator.go
  policy.go
  registry.go
  state_verifier.go
  budget.go
  circuit_breaker.go
  history.go
  error_codes.go
  events.go
  actions.go
  result.go

  policies/
    core.go
    capture.go
    tun.go
    route.go
    dns.go
    system_proxy.go
    subscription.go
    monitoring.go
    logging.go
    initialization.go
```

基础接口：

```go
type Engine interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Submit(event ErrorEvent)
}

type Policy interface {
    Code() ErrorCode
    Match(ctx context.Context, event ErrorEvent, snapshot RuntimeSnapshot) (bool, error)
    Repair(ctx context.Context, input RepairInput) RepairResult
}

type StateVerifier interface {
    Snapshot(ctx context.Context) (RuntimeSnapshot, error)
    Verify(ctx context.Context, expectation Expectation) VerificationResult
}
```

不要把所有策略写进一个巨大 `switch` 文件。

---

# 4. 错误事件模型

## 4.1 结构化错误事件

建议：

```go
type ErrorEvent struct {
    ID            string            `json:"id"`
    Timestamp     time.Time         `json:"timestamp"`
    Code          ErrorCode         `json:"code"`
    Level         LogLevel          `json:"level"`
    Service       string            `json:"service"`
    Component     string            `json:"component"`
    Operation     string            `json:"operation"`
    Message       string            `json:"message"`
    Retryable     bool              `json:"retryable"`
    CorrelationID string            `json:"correlation_id,omitempty"`
    TransitionID  string            `json:"transition_id,omitempty"`
    CoreID        string            `json:"core_id,omitempty"`
    OutboundID    string            `json:"outbound_id,omitempty"`
    CaptureMode   string            `json:"capture_mode,omitempty"`
    Fields        map[string]string `json:"fields,omitempty"`
}
```

不得把敏感值放入 `Fields`。

## 4.2 日志等级

自愈引擎主要消费：

```text
WARN
ERROR
FATAL
```

部分 INFO 事件用于确认恢复：

```text
core.ready
capture.committed
network.recovered
monitor.sample_received
```

DEBUG/TRACE 默认不触发修复。

## 4.3 服务分类

至少支持：

```text
Launcher
Initialization
Agent
Service
RuntimeTransition
Supervisor
CoreHost
CoreAdapter
sing-box
Mihomo
Xray
TUN
Route
DNS
SystemProxy
Subscription
CoreUpdate
NetworkMonitor
TrafficMonitor
LinkMonitor
IPDetection
Logging
SelfHeal
```

---

# 5. 稳定错误码规范

错误码格式：

```text
NAVO_<DOMAIN>_<ERROR>
```

例如：

```text
NAVO_CORE_START_TIMEOUT
NAVO_TUN_ADAPTER_DISABLED
NAVO_ROUTE_ENDPOINT_BYPASS_MISSING
```

错误码不得包含动态文本。

## 5.1 核心错误

```text
NAVO_CORE_BINARY_NOT_FOUND
NAVO_CORE_BINARY_HASH_MISMATCH
NAVO_CORE_CONFIG_INVALID
NAVO_CORE_NATIVE_CHECK_FAILED
NAVO_CORE_START_FAILED
NAVO_CORE_START_TIMEOUT
NAVO_CORE_PORT_NOT_LISTENING
NAVO_CORE_CONTROLLER_UNAVAILABLE
NAVO_CORE_CRASHED
NAVO_CORE_CRASH_LOOP
NAVO_CORE_STOP_TIMEOUT
NAVO_CORE_SWITCH_FAILED
NAVO_CORE_ROLLBACK_FAILED
NAVO_CORE_VERSION_INCOMPATIBLE
```

## 5.2 Capture 和运行时错误

```text
NAVO_CAPTURE_TRANSITION_FAILED
NAVO_CAPTURE_STATE_MISMATCH
NAVO_CAPTURE_ROLLBACK_FAILED
NAVO_RUNTIME_COMMITTED_STATE_INVALID
NAVO_RUNTIME_TRANSITION_TIMEOUT
NAVO_RUNTIME_LOCK_TIMEOUT
NAVO_RUNTIME_NETWORK_DIRTY
```

## 5.3 TUN 错误

```text
NAVO_TUN_ADAPTER_NOT_FOUND
NAVO_TUN_ADAPTER_DISABLED
NAVO_TUN_ADAPTER_NOT_READY
NAVO_TUN_ADAPTER_DUPLICATED
NAVO_TUN_ADDRESS_MISSING
NAVO_TUN_DRIVER_UNAVAILABLE
NAVO_TUN_DRIVER_HASH_MISMATCH
NAVO_TUN_CLEANUP_FAILED
```

## 5.4 路由错误

```text
NAVO_ROUTE_QUERY_FAILED
NAVO_ROUTE_MULTIPLE_OBJECTS
NAVO_ROUTE_INTERFACE_INVALID
NAVO_ROUTE_GATEWAY_NOT_FOUND
NAVO_ROUTE_ENDPOINT_BYPASS_MISSING
NAVO_ROUTE_ENDPOINT_BYPASS_STALE
NAVO_ROUTE_SPLIT_ROUTE_MISSING
NAVO_ROUTE_OWNERSHIP_CONFLICT
NAVO_ROUTE_CLEANUP_FAILED
```

## 5.5 DNS 错误

```text
NAVO_DNS_APPLY_FAILED
NAVO_DNS_STATE_MISMATCH
NAVO_DNS_PRIMARY_MISSING
NAVO_DNS_FALLBACK_MISSING
NAVO_DNS_RESTORE_FAILED
NAVO_DNS_QUERY_TIMEOUT
```

## 5.6 系统代理错误

```text
NAVO_SYSTEM_PROXY_SNAPSHOT_FAILED
NAVO_SYSTEM_PROXY_APPLY_FAILED
NAVO_SYSTEM_PROXY_STATE_MISMATCH
NAVO_SYSTEM_PROXY_TARGET_UNAVAILABLE
NAVO_SYSTEM_PROXY_RESTORE_FAILED
NAVO_SYSTEM_PROXY_EXTERNAL_CHANGE
NAVO_SYSTEM_PROXY_OWNERSHIP_INVALID
```

## 5.7 节点和订阅错误

```text
NAVO_OUTBOUND_NOT_FOUND
NAVO_OUTBOUND_CONFIG_INVALID
NAVO_OUTBOUND_HANDSHAKE_FAILED
NAVO_OUTBOUND_EXIT_CHECK_FAILED
NAVO_OUTBOUND_SWITCH_FAILED
NAVO_SUBSCRIPTION_FETCH_TIMEOUT
NAVO_SUBSCRIPTION_FETCH_TLS_FAILED
NAVO_SUBSCRIPTION_PARSE_FAILED
NAVO_SUBSCRIPTION_STATE_CORRUPT
NAVO_SUBSCRIPTION_APPLY_FAILED
```

## 5.8 监测错误

```text
NAVO_TRAFFIC_COLLECTOR_NO_DATA
NAVO_TRAFFIC_COLLECTOR_STALE
NAVO_TRAFFIC_COUNTER_RESET
NAVO_LINK_MONITOR_DIRECT_NO_DATA
NAVO_LINK_MONITOR_PROXY_NO_DATA
NAVO_LINK_MONITOR_STALE
NAVO_IP_DETECTION_TIMEOUT
NAVO_IP_RISK_PROVIDER_FAILED
```

## 5.9 日志和初始化错误

```text
NAVO_LOG_WRITE_FAILED
NAVO_LOG_ROTATION_FAILED
NAVO_LOG_STORAGE_UNAVAILABLE
NAVO_INIT_DEVICE_STATE_INVALID
NAVO_INIT_PRIVACY_RESET_FAILED
NAVO_INIT_MIGRATION_FAILED
NAVO_INIT_STORAGE_UNAVAILABLE
```

---

# 6. 错误分类

每个错误必须定义以下属性：

```go
type ErrorDefinition struct {
    Code                ErrorCode
    Category            string
    Severity            LogLevel
    Retryable           bool
    AutoRepairable      bool
    RequiresAdmin       bool
    RequiresTransition  bool
    MaxAttempts         int
    Window              time.Duration
    InitialBackoff      time.Duration
    MaxBackoff          time.Duration
    CircuitBreakFor     time.Duration
}
```

分类：

```text
Transient
  短暂网络、超时、资源尚未就绪

StateMismatch
  committed state 与真实系统状态不一致

Configuration
  配置无效或核心不支持

ResourceMissing
  文件、适配器、路由、端口缺失

Conflict
  端口占用、路由冲突、外部代理修改

Corruption
  状态文件、journal、配置损坏

Security
  hash 不匹配、设备绑定失败、ACL 失败

Permanent
  自动修复无法解决，需用户重新配置或升级
```

---

# 7. 修复动作模型

## 7.1 标准动作

```go
type RepairAction string

const (
    ActionRetryOperation             RepairAction = "retry_operation"
    ActionRebuildConfig              RepairAction = "rebuild_config"
    ActionRestartCore                RepairAction = "restart_core"
    ActionRollbackCore               RepairAction = "rollback_core"
    ActionRecreateTUN                RepairAction = "recreate_tun"
    ActionEnableTUNAdapter           RepairAction = "enable_tun_adapter"
    ActionReconcileNetwork           RepairAction = "reconcile_network"
    ActionRebuildEndpointBypass      RepairAction = "rebuild_endpoint_bypass"
    ActionRestoreSystemProxy         RepairAction = "restore_system_proxy"
    ActionDisableCapture             RepairAction = "disable_capture"
    ActionRestartCollector           RepairAction = "restart_collector"
    ActionRefreshSubscription        RepairAction = "refresh_subscription"
    ActionRotateLog                  RepairAction = "rotate_log"
    ActionEnterFaulted               RepairAction = "enter_faulted"
)
```

## 7.2 每个动作必须有

```text
前置条件
资源锁
超时
取消
执行方法
验证方法
回滚方法
重试预算
安全限制
日志
```

---

# 8. 核心相关自动修复策略

## 8.1 核心配置无效

错误：

```text
NAVO_CORE_CONFIG_INVALID
NAVO_CORE_NATIVE_CHECK_FAILED
```

自动处理：

```text
1. 停止提交候选配置；
2. 保留当前 committed 配置；
3. 重新从当前业务模型编译一次；
4. 再执行核心原生 check；
5. 如果仍失败，禁止启动候选核心；
6. 回滚到上一个已验证 revision；
7. 验证旧核心真实可用；
8. 记录失败字段和配置 revision ID。
```

禁止：

```text
重复使用同一份无效文件重试；
删除当前可用配置；
静默降级到 direct；
记录完整密码或 UUID。
```

## 8.2 核心启动超时

错误：

```text
NAVO_CORE_START_TIMEOUT
NAVO_CORE_PORT_NOT_LISTENING
```

先验证：

```text
进程是否存在
进程是否已经退出
端口是否被其他进程占用
核心日志是否显示配置错误
控制器是否可访问
```

策略：

```text
A. 进程存在但尚未 Ready
   再等待一次短宽限期，不立即杀进程。

B. 进程已退出
   由 Supervisor 按崩溃预算处理。

C. 端口被其他进程占用
   不杀未知进程；
   重新分配受控端口；
   重编译并通过 Runtime Transition 重启。

D. 配置错误
   执行配置回滚，不做无意义重启。
```

## 8.3 核心崩溃

错误：

```text
NAVO_CORE_CRASHED
```

处理：

```text
1. 确认真正异常退出，不是用户 Stop；
2. 检查 restart suppression；
3. 检查连续崩溃预算；
4. 第一次：按 Supervisor 退避重启；
5. 第二次：重新执行原生配置检查后重启；
6. 第三次：回滚上一 revision；
7. 继续失败：进入 CRASH_LOOP/Faulted。
```

不得让 Host 和 SelfHeal 同时重启。  
唯一重启执行者仍是 Supervisor。

## 8.4 核心切换失败

错误：

```text
NAVO_CORE_SWITCH_FAILED
```

处理：

```text
1. Runtime Transition 保留旧核心和旧配置快照；
2. 新核心启动失败后停止新核心；
3. 恢复旧 CoreAdapter；
4. 恢复旧配置；
5. TUN 模式重建旧端点旁路；
6. 验证旧代理数据面；
7. 成功则回到旧 committed state；
8. 失败则进入 faulted，不得写成功。
```

## 8.5 核心二进制缺失

错误：

```text
NAVO_CORE_BINARY_NOT_FOUND
```

策略：

```text
1. 检查受控 core 目录；
2. 检查升级 staging 是否存在完整已验证版本；
3. 检查最近备份核心；
4. 优先恢复最近 hash 已验证的备份；
5. 不允许从 PATH 或当前目录随意搜索；
6. 无备份则保持 Capture off/faulted。
```

是否自动下载：

```text
默认不自动下载；
只有用户已开启“允许后台核心更新/修复下载”时才可下载；
下载仍按第二份文档的核心升级校验流程执行。
```

## 8.6 Hash 不匹配

错误：

```text
NAVO_CORE_BINARY_HASH_MISMATCH
```

这是安全错误。

处理：

```text
1. 禁止运行该二进制；
2. 隔离到 quarantine；
3. 尝试恢复最近已验证备份；
4. 无备份则保持核心停止；
5. 不允许通过关闭校验自动修复；
6. 记录安全事件。
```

---

# 9. TUN 自动修复策略

## 9.1 TUN 网卡被禁用

错误：

```text
NAVO_TUN_ADAPTER_DISABLED
```

前置条件：

```text
CommittedMode == tun
当前 transition 不在关闭 TUN
适配器确认为 Navo-owned
```

处理：

```text
1. 暂停核心自动重启；
2. 停止当前数据面；
3. 尝试启用适配器；
4. 等待 OperationalStatus；
5. 重新应用 Runtime Transition 的 TUN 目标状态；
6. 重建端点旁路、split routes 和 DNS；
7. 验证代理 HTTPS；
8. 成功后恢复 supervisor。
```

如果启用失败：

```text
尝试受控销毁并重建 Navo TUN；
仍失败则 Capture 转 faulted；
不得无限创建重复网卡。
```

## 9.2 TUN 网卡丢失

错误：

```text
NAVO_TUN_ADAPTER_NOT_FOUND
```

处理：

```text
1. 确认 committed mode 为 tun；
2. 检查驱动；
3. 清理同一 ownership 下的无效句柄和临时状态；
4. 创建唯一 TUN；
5. 等待 Ready；
6. 通过统一 Runtime Transition 重建网络；
7. 验证；
8. 更新 journal。
```

## 9.3 重复 TUN 网卡

错误：

```text
NAVO_TUN_ADAPTER_DUPLICATED
```

处理：

```text
1. 识别当前 committed adapter；
2. 按 GUID、ownership、创建记录判断；
3. 只删除明确属于 Navo 且非当前目标的重复网卡；
4. 不删除其他 VPN/Wintun 网卡；
5. 重建当前状态并验证。
```

## 9.4 TUN 未 Ready

错误：

```text
NAVO_TUN_ADAPTER_NOT_READY
```

策略：

```text
第一次：等待短宽限期；
第二次：重置 adapter；
第三次：销毁并重建；
失败：faulted。
```

每阶段均受时间和次数限制。

---

# 10. 路由自动修复策略

## 10.1 Find-NetRoute 多对象

错误：

```text
NAVO_ROUTE_MULTIPLE_OBJECTS
```

该错误应由第一份文档从根本修复。

自愈引擎不得简单取 `[0]`。

运行时策略：

```text
1. 中止当前网络事务；
2. 保留旧 committed 网络；
3. 使用已修复的结构化路由解析器重新解析；
4. 再次执行 transition；
5. 仍失败则 faulted。
```

## 10.2 端点旁路缺失

错误：

```text
NAVO_ROUTE_ENDPOINT_BYPASS_MISSING
```

前置条件：

```text
CommittedMode == tun
当前核心和节点已确定
当前端点 IP 已解析
```

处理：

```text
1. 获取当前 endpoint；
2. 解析其物理出口；
3. 检查是否排除 Navo TUN；
4. 通过 network.Manager 的受控事务补建 /32 或 /128；
5. 验证 endpoint 路径不经过 TUN；
6. 验证代理连接。
```

## 10.3 旁路路由过期

错误：

```text
NAVO_ROUTE_ENDPOINT_BYPASS_STALE
```

发生场景：

```text
节点切换
域名 DNS 轮转
核心切换
网关变化
Wi-Fi/有线切换
```

处理：

```text
1. 解析新端点集合；
2. 先添加新旁路；
3. 验证新核心连接；
4. 再删除旧旁路；
5. 提交 journal。
```

不得先删旧旁路造成断线窗口。

## 10.4 Split Route 缺失

错误：

```text
NAVO_ROUTE_SPLIT_ROUTE_MISSING
```

处理：

```text
1. 读取 committed route plan；
2. 比对真实路由；
3. 只补建缺失的 Navo-owned 精确路由；
4. 遇到 ownership conflict 时停止自动修改；
5. 验证 route plan。
```

## 10.5 路由冲突

错误：

```text
NAVO_ROUTE_OWNERSHIP_CONFLICT
```

默认不自动覆盖第三方路由。

处理：

```text
1. 识别冲突路由归属；
2. 如果是旧 Navo journal 遗留，尝试安全清理；
3. 如果属于其他 VPN/管理员配置，进入 degraded/faulted；
4. 记录冲突详情但不删除未知路由。
```

---

# 11. DNS 自动修复策略

## 11.1 DNS 状态不一致

错误：

```text
NAVO_DNS_STATE_MISMATCH
```

处理：

```text
1. 读取 committed DNS plan；
2. 读取真实 DNS；
3. 检查是否发生外部修改；
4. 如果没有外部 ownership 冲突，重新应用；
5. 使用 set primary + add fallback；
6. Flush DNS cache；
7. 测试域名解析；
8. 验证代理端点域名和探测域名。
```

## 11.2 DNS 查询超时

错误：

```text
NAVO_DNS_QUERY_TIMEOUT
```

策略：

```text
第一次：重试备用 DNS；
第二次：重启核心 DNS listener；
第三次：重应用 DNS plan；
仍失败：回滚到上一个已验证 DNS plan。
```

不能自动把用户流量长期切换到不受控公共 DNS 而不记录。

## 11.3 DNS 恢复失败

错误：

```text
NAVO_DNS_RESTORE_FAILED
```

处理：

```text
保留 dirty；
保留 journal；
按原始 snapshot 再执行一次；
验证失败后进入 faulted；
不得写 off/ready。
```

---

# 12. 系统代理自动修复策略

## 12.1 系统代理指向不可用端口

错误：

```text
NAVO_SYSTEM_PROXY_TARGET_UNAVAILABLE
```

先判断：

```text
CommittedMode 是 system_proxy 还是 off/tun？
核心是否正在受控启动？
端口是否被错误进程占用？
```

策略：

```text
A. committed == system_proxy 且核心应运行
   尝试修复核心；
   成功后验证代理。

B. committed == off 或 tun
   系统代理属于残留；
   通过 system proxy transaction 恢复原 snapshot。

C. 无法判断 ownership
   不覆盖；
   记录外部修改。
```

## 12.2 状态不一致

错误：

```text
NAVO_SYSTEM_PROXY_STATE_MISMATCH
```

处理：

```text
1. 读取完整 committed proxy expectation；
2. 读取真实 ProxyEnable、ProxyServer、ProxyOverride、PAC、AutoDetect；
3. 比较 ownership hash；
4. 如果仍由 Navo 拥有，重新应用；
5. 如果外部程序已修改，记录 external change，不强制覆盖。
```

## 12.3 恢复失败

错误：

```text
NAVO_SYSTEM_PROXY_RESTORE_FAILED
```

策略：

```text
1. 使用保存的完整 snapshot 再恢复；
2. 调用 WinINet refresh；
3. 回读验证；
4. 仍失败保留 ownership dirty；
5. 进入 faulted。
```

---

# 13. 节点和订阅自动修复策略

## 13.1 节点握手失败

错误：

```text
NAVO_OUTBOUND_HANDSHAKE_FAILED
```

自动策略：

```text
1. 复核节点配置是否完整；
2. 检查核心能力与协议是否匹配；
3. 检查时间、SNI、TLS 和端点 DNS；
4. 重新解析端点并更新 TUN 旁路；
5. 只重试有限次数；
6. 不自动改密码、UUID、协议或 TLS 校验。
```

如果当前节点不可用：

```text
默认不自动切换到另一个节点，除非用户明确开启自动故障转移；
开启时只能选择已通过真实代理测试的候选；
切换必须通过 Runtime Transition；
记录切换原因和新节点 ID。
```

## 13.2 节点切换失败

错误：

```text
NAVO_OUTBOUND_SWITCH_FAILED
```

处理：

```text
恢复旧节点；
恢复旧端点旁路；
验证旧代理；
失败则 faulted。
```

## 13.3 订阅超时

错误：

```text
NAVO_SUBSCRIPTION_FETCH_TIMEOUT
```

策略：

```text
指数退避重试；
保留现有已验证节点；
不清空订阅；
不影响当前核心；
不把暂时网络失败写成订阅删除。
```

## 13.4 订阅解析失败

错误：

```text
NAVO_SUBSCRIPTION_PARSE_FAILED
```

处理：

```text
保留旧订阅快照；
保存脱敏解析错误；
不反复解析相同内容；
等待下一次手动或计划刷新；
不得自动猜测并修改节点协议。
```

## 13.5 订阅状态损坏

错误：

```text
NAVO_SUBSCRIPTION_STATE_CORRUPT
```

处理按前三份文档：

```text
隔离损坏文件；
尝试最近原子备份；
验证备份；
无法恢复则保持安全空状态并提示初始化错误；
不得直接用空数组覆盖损坏文件。
```

---

# 14. 监测模块自动修复策略

## 14.1 流量采集器无数据

错误：

```text
NAVO_TRAFFIC_COLLECTOR_NO_DATA
NAVO_TRAFFIC_COLLECTOR_STALE
```

定义：

```text
采集器已启动；
目标接口或核心应有数据能力；
超过设定时间没有新 sample。
```

处理：

```text
1. 检查 collector goroutine 是否存活；
2. 检查 context 是否被错误取消；
3. 检查目标网卡是否变化；
4. 检查核心 controller 是否变化；
5. 重新绑定数据源；
6. 重启 collector；
7. 验证 sample 时间戳更新。
```

不得：

```text
用随机数补数据；
把无数据写成 0；
为了恢复采集重启整个核心。
```

## 14.2 双联链路 Direct 无数据

错误：

```text
NAVO_LINK_MONITOR_DIRECT_NO_DATA
```

处理：

```text
1. 检查 Direct client 不使用系统代理；
2. 检查探测目标；
3. 重建 Direct client；
4. 更换受控备用探测目标；
5. 验证 direct sample。
```

## 14.3 双联链路 Proxy 无数据

错误：

```text
NAVO_LINK_MONITOR_PROXY_NO_DATA
```

处理：

```text
1. 确认 Capture/核心是否应运行；
2. 检查代理本地端口；
3. 重建 Proxy client；
4. 验证通过代理的 HTTPS 和出口 IP；
5. 如果核心故障，交给核心策略，不在 monitor 内直接重启。
```

## 14.4 IP 检测超时

错误：

```text
NAVO_IP_DETECTION_TIMEOUT
```

策略：

```text
切换备用 HTTPS provider；
使用缓存并标记 stale；
不影响 Capture；
不高频重试。
```

---

# 15. 日志系统自动修复策略

## 15.1 日志写入失败

错误：

```text
NAVO_LOG_WRITE_FAILED
```

处理：

```text
1. 检查目录是否存在；
2. 检查文件句柄；
3. 检查磁盘空间；
4. 尝试重新打开当前日志；
5. 失败则切换到受控 fallback bootstrap log；
6. 保留内存环形缓冲中的最近错误；
7. 不因普通日志失败停止代理核心。
```

## 15.2 日志轮转失败

错误：

```text
NAVO_LOG_ROTATION_FAILED
```

处理：

```text
关闭当前 writer；
使用唯一时间戳文件重试；
避免覆盖；
失败则继续有限写当前文件并记录告警；
达到文件硬限制时切换 fallback。
```

## 15.3 自愈日志防递归

SelfHeal 自己写日志失败时，不能再次产生无限 SelfHeal 事件。

必须：

```text
识别 source == SelfHeal；
日志错误进入内存 fallback；
设置递归保护；
在固定时间后再尝试恢复。
```

---

# 16. 初始化和隐私错误

## 16.1 设备状态无效

错误：

```text
NAVO_INIT_DEVICE_STATE_INVALID
```

处理：

```text
不得用自愈方式绕过 DPAPI；
不得尝试读取旧敏感配置；
按第三份文档进入 foreign context 或数据损坏流程。
```

## 16.2 隐私清理失败

错误：

```text
NAVO_INIT_PRIVACY_RESET_FAILED
```

这是阻断错误。

处理：

```text
禁止核心启动；
禁止 Capture；
禁止加载旧订阅；
记录无法清除的文件类别；
只允许重试清理或退出。
```

自愈引擎不能“自动忽略”。

---

# 17. 修复预算与熔断

## 17.1 预算键

建议：

```text
ErrorCode + ResourceID + CoreID + OutboundID
```

例如：

```text
NAVO_CORE_CRASHED:sing-box
NAVO_ROUTE_ENDPOINT_BYPASS_MISSING:1.2.3.4
```

## 17.2 默认预算

建议初始值：

```text
普通瞬时错误：
  3 次 / 5 分钟

核心崩溃：
  3 次 / 10 分钟

TUN 重建：
  2 次 / 10 分钟

路由/DNS 重应用：
  2 次 / 5 分钟

系统代理恢复：
  2 次 / 5 分钟

安全错误：
  0 次危险重试
```

实际参数应集中配置。

## 17.3 退避

```text
1s
2s
5s
10s
30s
```

加少量 jitter，避免多个任务同步重试。

## 17.4 熔断

达到预算：

```text
状态进入 open circuit
停止自动动作
保留监测
只记录状态变化
在冷却期后允许一次 half-open 验证
```

安全和隐私错误默认不会自动 half-open。

---

# 18. 去重、关联和风暴保护

## 18.1 日志去重

相同错误在短时间大量出现时：

```text
聚合 count
保留首次和最近时间
不为每条日志启动一个修复 goroutine
```

## 18.2 Correlation ID

同一 Runtime Transition 的错误使用同一：

```text
transition_id
correlation_id
```

自愈引擎只能为一次 transition 创建一个修复任务。

## 18.3 单资源互斥

至少需要：

```text
全局 Runtime Transition 锁
Core 级锁
Network 级锁
Collector 级锁
Log 级锁
```

锁顺序必须固定，避免死锁。

## 18.4 错误风暴

如果每秒大量错误：

```text
限制队列长度；
按 error key 合并；
保留 FATAL；
丢弃重复 DEBUG/WARN；
记录 dropped count。
```

---

# 19. 状态持久化

保存有限的自愈历史：

```text
state/selfheal-state.json
```

包含：

```text
错误码
资源 ID 的 hash
尝试次数
最后尝试时间
熔断截止时间
最后结果
```

不得保存：

```text
密码
订阅 URL
完整节点地址（非必要）
完整 UUID
API key
```

文件需：

```text
原子写入
ACL
版本字段
损坏隔离
```

新电脑 foreign context 时清除该历史。

---

# 20. 日志输出规范

每次修复至少写四类事件：

## 20.1 检测

```text
SELFHEAL_DETECTED
```

字段：

```text
error_code
resource
source_service
count
```

## 20.2 开始

```text
SELFHEAL_REPAIR_STARTED
```

字段：

```text
policy
attempt
max_attempts
correlation_id
```

## 20.3 结果

成功：

```text
SELFHEAL_REPAIR_SUCCEEDED
```

失败：

```text
SELFHEAL_REPAIR_FAILED
```

字段：

```text
action
verification
duration_ms
rollback
```

## 20.4 熔断

```text
SELFHEAL_CIRCUIT_OPENED
SELFHEAL_CIRCUIT_HALF_OPEN
SELFHEAL_CIRCUIT_CLOSED
```

UI 日志筛选中，用户可通过：

```text
Level
Service = SelfHeal
日期
错误码
```

查看这些日志。

不新增独立自愈 UI。

---

# 21. 真实状态快照

建议：

```go
type RuntimeSnapshot struct {
    CapturedAt time.Time

    DesiredMode   string
    CommittedMode string
    Transitioning bool
    NetworkDirty  bool

    Core struct {
        ID               string
        State            string
        PID              int
        BinaryVerified   bool
        ConfigRevision   string
        PortListening    bool
        ControllerReady  bool
    }

    TUN struct {
        Expected bool
        Exists   bool
        Enabled  bool
        Ready    bool
        Name     string
        GUID     string
    }

    Network struct {
        EndpointBypassesValid bool
        SplitRoutesValid      bool
        DNSValid              bool
        JournalState          string
    }

    SystemProxy struct {
        Expected       bool
        StateMatches   bool
        TargetReachable bool
        OwnershipValid bool
    }

    Monitoring struct {
        TrafficLastSample time.Time
        DirectLastSample  time.Time
        ProxyLastSample   time.Time
    }
}
```

快照读取失败时，不执行危险修复。

---

# 22. 策略注册表

示例：

```go
func DefaultPolicies(deps Dependencies) []Policy {
    return []Policy{
        NewCoreStartTimeoutPolicy(deps),
        NewCoreCrashPolicy(deps),
        NewCoreSwitchRollbackPolicy(deps),
        NewTUNDisabledPolicy(deps),
        NewTUNMissingPolicy(deps),
        NewEndpointBypassPolicy(deps),
        NewDNSMismatchPolicy(deps),
        NewSystemProxyMismatchPolicy(deps),
        NewTrafficCollectorPolicy(deps),
        NewLinkMonitorPolicy(deps),
        NewLogWriterPolicy(deps),
    }
}
```

每个 Policy 独立测试。

未知错误码没有默认“万能修复”。

---

# 23. 自动修复配置

虽然不新增 UI，可使用本地内部配置：

```go
type SelfHealConfig struct {
    Enabled              bool
    ObserveOnly          bool
    QueueSize            int
    VerificationTimeout  time.Duration
    DefaultMaxAttempts   int
    StateFile            string
}
```

生产默认：

```text
Enabled = true
ObserveOnly = false
```

开发和测试可使用：

```text
ObserveOnly = true
```

Observe-only 只记录本来会采取什么动作，不实际修改系统。

敏感或高风险策略可以单独默认关闭，例如自动下载核心。

---

# 24. 启动与关闭顺序

启动：

```text
1. 初始化和隐私检查完成
2. 日志系统 Ready
3. repositories Ready
4. Runtime Transition Coordinator Ready
5. Supervisor Ready
6. SelfHeal Engine 启动
7. Agent/UI 启动
```

关闭：

```text
1. SelfHeal 停止接收新事件
2. 取消未开始任务
3. 等待正在执行的安全点
4. 停止监测
5. 执行正常 Capture shutdown
6. 写入最终状态
```

关闭期间不应启动新的自动恢复。

---

# 25. 测试要求

## 25.1 单元测试

每个 Policy 至少覆盖：

```text
匹配正确错误
不匹配其他错误
真实状态已恢复时不动作
前置条件不满足
修复成功
修复失败
验证失败
回滚成功
回滚失败
预算耗尽
熔断
取消
超时
```

## 25.2 模拟错误测试

使用 Fake：

```text
FakeRuntimeCoordinator
FakeSupervisor
FakeNetworkManager
FakeSystemProxyManager
FakeTUNManager
FakeMonitor
FakeLogStore
FakeClock
```

不要让普通单元测试修改真实路由或注册表。

## 25.3 Windows 集成测试

至少覆盖：

```text
核心启动失败后回滚
核心崩溃重启预算
核心端口被占用
TUN 网卡被禁用
TUN 网卡被删除
端点旁路被手工删除
DNS 被手工修改
系统代理残留
节点切换后旧旁路
流量采集停止
Direct/Proxy 监测停止
日志目录临时不可写
```

每个测试结束必须清理。

## 25.4 Chaos 测试

建议增加受控故障注入：

```text
NAVO_FAULT_INJECT=core_start_timeout
NAVO_FAULT_INJECT=tun_disabled
NAVO_FAULT_INJECT=route_missing
NAVO_FAULT_INJECT=dns_mismatch
NAVO_FAULT_INJECT=collector_stale
```

仅开发构建可用，正式构建禁用。

## 25.5 非回归

自动修复模块加入后必须确认：

```text
用户正常切换核心不会被误判
用户正常关闭核心不会被自动重启
用户主动关闭代理不会被重新开启
外部程序修改代理不会被强制覆盖
其他 VPN 路由不会被删除
无流量时不会伪造流量
新电脑隐私重置不会被自愈绕过
```

---

# 26. 验收矩阵

| 错误 | 自动动作 | 最大尝试 | 验证 | 失败结果 |
|---|---|---:|---|---|
| 核心启动超时 | 状态复核、重试或回滚 | 3 | 端口 + HTTPS | Faulted |
| 核心崩溃 | Supervisor 退避重启 | 3 | 进程 + HTTPS | Crash loop |
| 核心切换失败 | 恢复旧核心 | 1 | 旧代理可用 | Faulted |
| TUN 被禁用 | 启用或重建 | 2 | Adapter + HTTPS | Faulted |
| TUN 丢失 | 受控重建 | 2 | Adapter + routes | Faulted |
| 端点旁路缺失 | 补建旁路 | 2 | 路由 + 代理 | Faulted |
| DNS 不一致 | 重应用计划 | 2 | DNS 查询 | Dirty/Faulted |
| 系统代理残留 | 恢复 snapshot | 2 | Registry + target | Dirty/Faulted |
| 订阅超时 | 退避刷新 | 3 | 获取成功 | 保留旧数据 |
| 流量无数据 | 重绑采集器 | 2 | 新 sample | Degraded |
| 双联链路无数据 | 重建 client | 2 | 新 sample | Degraded |
| 日志写失败 | 重开/fallback | 2 | 写入成功 | 内存 fallback |
| Hash 不匹配 | 隔离、恢复备份 | 1 | Hash + start | 安全阻断 |
| 隐私清理失败 | 不自动绕过 | 0 | 清理完成 | 启动阻断 |

---

# 27. 禁止的伪修复

Claude/Codex 不得：

1. 只解析日志文本关键词，不使用错误码；
2. 看到 ERROR 就重启整个程序；
3. 看到核心错误就无限重启；
4. 让 SelfHeal 和 Supervisor 同时重启核心；
5. 直接执行 PowerShell 修改路由，绕过 network.Manager；
6. 自动删除未知路由；
7. 自动杀死占用端口的未知进程；
8. 系统代理被外部修改后强制覆盖；
9. 回滚失败后写 success；
10. 未验证数据面就标记恢复；
11. 用随机数填充监测数据；
12. 自动修改用户节点密码、UUID 或 TLS；
13. Hash 不匹配时关闭安全校验；
14. 为修复核心丢失从 PATH 随便找 exe；
15. 隐私初始化失败后继续加载旧配置；
16. 自愈日志错误触发无限递归；
17. 每条重复日志创建一个 goroutine；
18. 自动修复没有次数上限；
19. 使用 AI 分析日志；
20. 上传日志到远程服务；
21. 新增 MySQL 保存修复历史；
22. 没有测试就声称“自动修复完成”。

---

# 28. 建议提交顺序

```text
1. feat(errors): introduce stable structured error codes
2. feat(logging): emit normalized error and recovery events
3. feat(selfheal): add engine, registry, budget and circuit breaker
4. feat(selfheal-core): add core recovery policies
5. feat(selfheal-network): add TUN, route, DNS and proxy policies
6. feat(selfheal-monitor): add traffic and link monitor policies
7. feat(selfheal-logging): add log writer fallback policy
8. test(selfheal): add fake clock, policy and circuit tests
9. test(windows): add controlled Windows recovery integration tests
10. docs(selfheal): document policies, limits and known exclusions
```

---

# 29. Codex / Claude 最终交付格式

```markdown
# Navo 自动纠错与自愈引擎完成报告

## 1. 架构
- Engine：
- Detector：
- Policy Registry：
- State Verifier：
- Budget：
- Circuit Breaker：

## 2. 错误码
| 错误码 | 产生位置 | 自动修复 | 最大尝试 |
|---|---|---|---:|

## 3. 核心修复策略
- ...

## 4. TUN 和网络修复策略
- ...

## 5. 系统代理修复策略
- ...

## 6. 订阅和节点策略
- ...

## 7. 监测策略
- ...

## 8. 日志策略
- ...

## 9. 安全限制
- 不自动处理：
- 熔断条件：
- Faulted 条件：

## 10. 自动测试
```text
go test ./...
go test -race ./...
go vet ./...
```

## 11. Windows 故障注入结果
- 核心启动失败：
- 核心崩溃：
- TUN 禁用：
- TUN 删除：
- 路由缺失：
- DNS 改变：
- 系统代理残留：
- 采集器停止：

## 12. 非回归验证
- 主动关闭不会自动重启：
- 外部路由不会误删：
- 新电脑隐私清理不会绕过：
- 无 AI/MySQL：

## 13. 未完成或仅观察项
- ...

## 14. 总体验收
- 是否通过：
- 未通过项：
```

没有真实状态验证和测试证据的策略，不得标记为完成。

---

# 30. 最终运行效果

正常情况下：

```text
各模块输出结构化日志
        ↓
SelfHeal 只监听 WARN/ERROR/FATAL 和关键状态事件
        ↓
发现已知错误码
        ↓
再次读取真实状态
        ↓
错误已经消失：不做任何动作
错误仍存在：执行有限修复策略
        ↓
验证修复后的真实数据面
        ↓
成功：记录恢复
失败：回滚、退避或熔断
```

最终必须做到：

```text
常见错误能自动恢复
未知错误不乱修
安全错误不绕过
用户主动操作不被反向纠正
所有修复都有限次
所有修复都可验证
所有失败都可追踪
全部逻辑本地执行
不依赖 UI
不依赖 AI
不依赖 MySQL
不上传日志
```
