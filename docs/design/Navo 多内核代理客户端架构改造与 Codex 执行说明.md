# Navo 多内核代理客户端架构改造与 Codex 执行说明

> 文档状态：必须执行
> 目标项目：Navo
> 目标平台：Windows 11 x64
> 当前技术栈：Wails v2、Vue 3、Go、Windows Service、Named Pipe、云端 MySQL
> 推荐项目路径：`D:\dev\Navo`
> 本文档优先级：高于旧版单 sing-box 架构文档
> 执行对象：Codex、后续编码代理、人工开发者和测试人员

# 1. 文档目的

本文档用于纠正 Navo 当前代理架构方向。

Navo 不是单一 sing-box GUI，也不是简单的 Clash 客户端或 v2rayN 仿制品。

Navo 的产品定位是：

> 将 Clash Verge 的订阅、节点、系统代理、TUN 和使用体验，与 v2rayN 的多内核、协议兼容和配置生成能力结合，形成一个统一的 Windows 代理客户端。

必须保留：

1. Mihomo；
2. Xray；
3. sing-box；
4. 机场订阅；
5. 用户购买的全链路代理；
6. 系统代理模式；
7. TUN 模式；
8. 当前已有的 Wails v2 + Vue 3 UI、Go Agent 和 Windows Service 基础。

不得通过删除任何一个内核、删除机场、删除全链路代理来规避问题。

# 2. Codex必须理解的核心需求

Navo 存在两个相互独立的选择维度。

## 2.1 代理内核三选一

运行时必须且只能选择一个代理内核：

```
Mihomo
Xray
sing-box
```

同一时刻只能运行其中一个内核进程。

禁止出现：

```
mihomo.exe 正在运行
xray.exe 同时运行
sing-box.exe 同时运行
```

允许三个内核都安装在 Navo 目录中，但只能有一个处于活动状态。

## 2.2 线路来源二选一

运行时必须且只能选择一种线路来源：

```
机场订阅
全链路代理
```

两类数据可以同时保存在数据库里，但实际连接时只能激活其中一种。

### 机场模式

```
机场订阅
  ↓
选择订阅
  ↓
选择机场节点
  ↓
交给当前选中的内核运行
```

### 全链路代理模式

```
手动添加或导入代理
  ↓
选择 HTTP / HTTPS / SOCKS5 等代理
  ↓
交给当前选中的内核运行
```

禁止在当前版本中自动生成：

```
本机 → 机场节点 → 全链路代理 → 网站
```

这属于链式代理，不是当前需求。

代码内部应将用户界面上的“全链路代理”命名为：

```
SourceTypeUpstreamProxy
```

不得将其错误理解为：

```
ChainProxy
DetourProxy
NestedProxy
```

## 2.3 最终选择模型

Navo 的运行组合为：

| 线路来源   | Mihomo   | Xray     | sing-box |
| ---------- | -------- | -------- | -------- |
| 机场订阅   | 必须支持 | 必须支持 | 必须支持 |
| 全链路代理 | 必须支持 | 必须支持 | 必须支持 |

产品层面必须支持六种组合。

具体机场节点是否能在某个内核运行，取决于该节点协议和内核能力。遇到不兼容节点时必须明确提示，不能偷偷删除字段或伪装成支持。

# 3. “复制 Clash Verge 和 v2rayN”的准确含义

“复制”不是盲目复制它们的源码，而是复制成熟产品已经验证过的职责划分和用户流程。

## 3.1 从 Clash Verge 参考的能力

需要参考：

- 订阅/Profile 管理；
- 节点列表和代理组展示；
- 延迟测试；
- 当前节点切换；
- 系统代理开关；
- TUN 开关；
- 内核运行状态；
- 真实系统代理状态校验；
- 配置应用和恢复；
- 内核 API 日志、流量和连接状态；
- 清晰的错误提示；
- 关闭软件时的网络恢复。

Clash Verge Rev 当前以 Mihomo 为代理核心，并提供系统代理、TUN、配置管理和可视化代理控制。citeturn392756search2turn392756search5

## 3.2 从 v2rayN 参考的能力

需要参考：

- 多内核目录管理；
- 不同节点协议选择不同配置生成器；
- 内核能力判断；
- Xray、sing-box、Mihomo 分别生成原生配置；
- 分享链接解析；
- 订阅解析；
- 前置代理和落地代理的数据建模方式；
- 每个内核独立的启动、校验、日志和健康检测；
- 切换内核时重新生成配置。

v2rayN 当前支持并管理 Xray、Mihomo、sing-box 等多种核心，这说明“一个 GUI 管理多个代理核心”在产品和工程上是可行的。citeturn392756search3turn392756search4

## 3.3 禁止行为

禁止直接复制大段第三方代码后删除版权信息。

禁止假设第三方 GPL 代码可以无条件放入闭源项目。

可以：

- 阅读其目录结构；
- 阅读其接口划分；
- 参考交互流程；
- 参考测试思路；
- 自行实现兼容层；
- 在许可证允许的前提下复用代码并保留归属信息。

必须维护：

```
THIRD_PARTY_NOTICES.md
CORE_MANIFEST.json
LICENSES/
```

# 4. Navo 当前基础与必须保留的部分

Codex不得因为改造多内核而推翻整个项目。

以下基础必须保留：

```
Wails v2 + Vue 3 UI
Go User Agent
Go Windows Service
Named Pipe IPC
云端 MySQL
DPAPI
系统代理恢复
TUN 与网络恢复
日志和诊断
```

当前开发环境固定为：

```
Wails v2
Vue 3
Windows 11 x64
Go 当前项目锁定版本
```

禁止继续：

- 重新引入已淘汰的旧 UI 构建链；
- 修改 Wails/WebView2 底层实现规避架构问题；
- 在包含中文或特殊字符的路径中构建；
- 在多个文件中散落硬编码端口；
- 将业务逻辑写进 Vue 页面；
- 让 Wails UI 直接启动代理内核；
- 让 Wails UI 或 Agent 直接连接 MySQL；
- 让 Wails UI 直接操作 TUN 或系统路由。

# 5. 目标运行架构

Navo 应逐步调整为五层运行结构：

```
┌─────────────────────────────────────────────┐
│ navo-ui.exe                                 │
│ Wails v2 + Vue 3 UI、交互、页面、状态展示     │
└───────────────────┬─────────────────────────┘
                    │ User Agent Named Pipe
┌───────────────────▼─────────────────────────┐
│ navo-agent.exe                              │
│ 用户会话、系统代理、托盘、通知、UI 状态聚合  │
└───────────────────┬─────────────────────────┘
                    │ System Service Named Pipe
┌───────────────────▼─────────────────────────┐
│ navo-service.exe                            │
│ 数据库、凭据、配置事务、订阅、TUN、路由      │
└───────────────────┬─────────────────────────┘
                    │ 私有 Core Host IPC
┌───────────────────▼─────────────────────────┐
│ navo-core-host.exe                          │
│ 内核适配、进程管理、日志、健康检查            │
└───────────────────┬─────────────────────────┘
                    │ 只启动一个
       ┌────────────┼────────────┐
       ▼            ▼            ▼
 mihomo.exe      xray.exe    sing-box.exe
```

## 5.1 UI职责

UI 只负责：

- 展示机场订阅；
- 展示节点；
- 展示全链路代理；
- 选择线路来源；
- 选择运行内核；
- 选择系统代理或 TUN；
- 发起连接、断开、测试和切换；
- 展示兼容性；
- 展示运行日志；
- 展示连接状态；
- 展示出口 IP 和流量。

UI 不负责：

- 解析完整代理配置；
- 生成内核配置；
- 启动内核；
- 设置系统代理；
- 修改路由；
- 管理 Wintun；
- 保存明文密码。

## 5.2 User Agent职责

`navo-agent.exe` 负责：

- 当前 Windows 用户会话；
- 系统代理读取；
- 系统代理快照；
- 设置系统代理；
- 恢复系统代理；
- 托盘；
- 开机登录自启动；
- UI 与 Service 的状态转发；
- 当前用户通知。

系统代理必须由 Agent 操作，不能由 Session 0 中的 Service 代替。

## 5.3 System Service职责

`navo-service.exe` 负责：

- 数据库；
- DPAPI 和敏感数据；
- 订阅请求；
- 订阅解析；
- Endpoint Repository；
- Core Compatibility Resolver；
- 配置编译事务；
- Core Revision；
- Core Host 管理；
- TUN；
- 路由；
- DNS；
- 网络状态恢复；
- 诊断；
- 崩溃回滚。

## 5.4 Core Host职责

`navo-core-host.exe` 负责：

- 根据 Service 下发的受限 Manifest 选择适配器；
- 只启动允许目录中的内核；
- 收集 stdout；
- 收集 stderr；
- 记录退出码；
- 超时终止；
- 检测本地端口；
- 检测控制 API；
- 执行内核配置校验；
- 向 Service 上报结构化状态。

Core Host不得接受 UI 传入的任意：

- 可执行文件路径；
- 命令行参数；
- 工作目录；
- 配置路径；
- 环境变量。

# 6. 推荐仓库结构

Codex必须先根据真实仓库生成映射，不得直接删除现有目录。目标结构如下：

```
Navo/
├─ apps/
│  └─ windows-ui/
│     ├─ lib/
│     │  ├─ app/
│     │  ├─ features/
│     │  │  ├─ dashboard/
│     │  │  ├─ subscriptions/
│     │  │  ├─ airport_nodes/
│     │  │  ├─ upstream_proxies/
│     │  │  ├─ core_selection/
│     │  │  ├─ connection/
│     │  │  ├─ diagnostics/
│     │  │  └─ settings/
│     │  ├─ ipc/
│     │  └─ shared/
│     └─ windows/
│
├─ cmd/
│  ├─ navo-agent/
│  ├─ navo-service/
│  ├─ navo-core-host/
│  └─ navo-repair/
│
├─ internal/
│  ├─ domain/
│  │  ├─ core/
│  │  ├─ source/
│  │  ├─ endpoint/
│  │  ├─ selection/
│  │  ├─ subscription/
│  │  ├─ routing/
│  │  ├─ revision/
│  │  └─ diagnostics/
│  │
│  ├─ application/
│  │  ├─ connect/
│  │  ├─ disconnect/
│  │  ├─ switchcore/
│  │  ├─ switchsource/
│  │  ├─ importsubscription/
│  │  ├─ testendpoint/
│  │  └─ recovernetwork/
│  │
│  ├─ adapters/
│  │  ├─ core/
│  │  │  ├─ mihomo/
│  │  │  ├─ xray/
│  │  │  └─ singbox/
│  │  ├─ subscription/
│  │  │  ├─ clashyaml/
│  │  │  ├─ urilist/
│  │  │  ├─ base64list/
│  │  │  └─ singboxjson/
│  │  └─ proxytest/
│  │
│  ├─ infrastructure/
│  │  ├─ storage/
│  │  ├─ secrets/
│  │  ├─ windows/
│  │  ├─ networking/
│  │  ├─ logging/
│  │  └─ process/
│  │
│  ├─ agent/
│  ├─ service/
│  └─ corehost/
│
├─ contracts/
│  └─ proto/
│
├─ migrations/
│
├─ configs/
│  ├─ testdata/
│  ├─ golden/
│  │  ├─ mihomo/
│  │  ├─ xray/
│  │  └─ singbox/
│  └─ schemas/
│
├─ third_party/
│  ├─ cores/
│  │  ├─ mihomo/
│  │  ├─ xray/
│  │  └─ sing-box/
│  └─ wintun/
│
├─ docs/
│  ├─ adr/
│  ├─ architecture/
│  ├─ audit/
│  ├─ runbooks/
│  └─ test-plans/
│
├─ scripts/
├─ tests/
│  ├─ integration/
│  ├─ e2e/
│  ├─ fault/
│  └─ compatibility/
│
├─ CORE_MANIFEST.json
└─ THIRD_PARTY_NOTICES.md
```

依赖方向必须保持：

```
UI → IPC Contract
Agent → Application + Windows User APIs
Service → Application + Domain + Infrastructure
Core Host → Core Adapter + Process Infrastructure
Core Adapter → Domain Interfaces
Domain → 不依赖 Windows、Wails、Vue、MySQL、具体内核
```

# 7. 领域模型重构

## 7.1 CoreType

```
package core

type Type string

const (
    TypeMihomo  Type = "mihomo"
    TypeXray    Type = "xray"
    TypeSingBox Type = "sing-box"
)

func (t Type) Valid() bool {
    switch t {
    case TypeMihomo, TypeXray, TypeSingBox:
        return true
    default:
        return false
    }
}
```

## 7.2 SourceType

```
package source

type Type string

const (
    TypeAirportSubscription Type = "airport_subscription"
    TypeUpstreamProxy       Type = "upstream_proxy"
)
```

不得添加隐式的：

```
TypeAirportAndProxy
TypeChainProxy
TypeAutoMix
```

## 7.3 CaptureMode

线路来源与流量接管模式不是同一个概念。

```
type CaptureMode string

const (
    CaptureModeOff         CaptureMode = "off"
    CaptureModeSystemProxy CaptureMode = "system_proxy"
    CaptureModeTUN         CaptureMode = "tun"
)
```

最终运行状态由三个维度组成：

```
CoreType
SourceType
CaptureMode
```

例如：

```
Mihomo + 机场节点 + 系统代理
Xray + 全链路代理 + 系统代理
sing-box + 机场节点 + TUN
```

## 7.4 ActiveSelection

```
type ActiveSelection struct {
    CoreType       core.Type
    SourceType     source.Type
    CaptureMode    CaptureMode

    SubscriptionID *string
    EndpointID     *string
    UpstreamProxyID *string

    UpdatedAt time.Time
}
```

必须进行领域校验。

### 机场模式

```
SourceType = airport_subscription
SubscriptionID != nil
EndpointID != nil
UpstreamProxyID == nil
```

### 全链路代理模式

```
SourceType = upstream_proxy
UpstreamProxyID != nil
SubscriptionID == nil
EndpointID == nil
```

任何不符合此规则的数据都不得进入配置编译器。

# 8. Endpoint不得继续使用简化通用结构

禁止使用下面这种结构表达所有节点：

```
type ProxyNode struct {
    Host     string
    Port     int
    Username string
    Password string
    Protocol string
}
```

它无法表达 Reality、TLS、WebSocket、gRPC、XHTTP、QUIC 和其他协议参数。

正确方案是公共字段加协议专用配置。

```
type Endpoint struct {
    ID         string
    ProviderID string
    Name       string
    Protocol   Protocol
    Server     string
    Port       uint16
    Enabled    bool

    SpecVersion int
    Spec        EndpointSpec

    RawFormat   string
    RawHash     string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
type EndpointSpec interface {
    endpointSpec()
    Validate() error
}
```

示例：

```
type VLESSSpec struct {
    UUID       string
    Flow       string
    Encryption string
    TLS        TLSOptions
    Transport  TransportOptions
}

type VMessSpec struct {
    UUID       string
    AlterID    int
    Security   string
    TLS        TLSOptions
    Transport  TransportOptions
}

type TrojanSpec struct {
    PasswordRef string
    TLS         TLSOptions
    Transport   TransportOptions
}

type ShadowsocksSpec struct {
    Method      string
    PasswordRef string
    Plugin      *PluginOptions
}

type Hysteria2Spec struct {
    PasswordRef string
    TLS         TLSOptions
    Obfs        *ObfsOptions
}

type HTTPProxySpec struct {
    TLS         bool
    UsernameRef *string
    PasswordRef *string
    Headers     map[string]string
}

type SOCKS5ProxySpec struct {
    UsernameRef *string
    PasswordRef *string
    UDPRequested bool
}
```

所有密码、订阅 URL、Token 和 API Secret 必须使用凭据引用，不得直接存在普通结构和日志中。

# 9. 订阅处理链路

订阅导入必须拆成以下流程：

```
Subscription Fetcher
    ↓
响应限制与安全检查
    ↓
Format Detector
    ↓
对应 Parser
    ↓
Normalized Endpoint
    ↓
领域校验
    ↓
去重
    ↓
事务写入数据库
```

## 9.1 Fetcher要求

必须支持：

- HTTP 和 HTTPS；
- 合理超时；
- 最大响应大小；
- 最大重定向次数；
- 禁止访问本机敏感地址；
- User-Agent；
- ETag；
- Last-Modified；
- 状态码记录；
- Content-Type 记录；
- UTF-8 和常见编码处理。

禁止在日志中记录完整订阅 URL。

应输出：

```
Subscription fetch completed
HTTP status: 200
Response size: 152 KB
Content-Type: text/yaml
Detected format: clash-yaml
Parsed nodes: 48
Accepted nodes: 43
Rejected nodes: 5
```

## 9.2 Parser Registry

```
type SubscriptionParser interface {
    Name() string
    Detect(input []byte, contentType string) int
    Parse(ctx context.Context, input []byte) (ParseResult, error)
}
```

至少拆分为：

```
ClashYAMLParser
PlainURIListParser
Base64URIListParser
SingBoxJSONParser
```

Parser只负责解析。

Parser不得：

- 启动代理内核；
- 写系统代理；
- 选择活动节点；
- 生成最终内核配置；
- 直接修改当前运行状态。

## 9.3 Clash订阅规则

只提取订阅中的节点数据和必要元数据。

不得直接信任并运行机场下发的：

- TUN 配置；
- DNS 配置；
- external-controller；
- 脚本；
- 外部 UI；
- 任意文件路径；
- 任意规则提供器；
- 任意下载地址。

远程配置不能控制 Navo 的本地系统能力。

# 10. 全链路代理模型

全链路代理是独立线路来源，不属于机场节点。

```
type UpstreamProxy struct {
    ID          string
    Name        string
    Protocol    UpstreamProtocol
    Server      string
    Port        uint16
    UsernameRef *string
    PasswordRef *string

    TLS         bool
    UDPPolicy   UDPPolicy
    Enabled     bool

    CreatedAt time.Time
    UpdatedAt time.Time
}
type UpstreamProtocol string

const (
    UpstreamHTTP   UpstreamProtocol = "http"
    UpstreamHTTPS  UpstreamProtocol = "https"
    UpstreamSOCKS5 UpstreamProtocol = "socks5"
)
```

导入时需要支持常见格式：

```
host:port
host:port:username:password
username:password@host:port
http://username:password@host:port
https://username:password@host:port
socks5://username:password@host:port
```

不得仅通过字段数量猜测协议。

无法确定时必须要求用户选择 HTTP、HTTPS 或 SOCKS5。

HTTP 代理的 UDP 能力不能标记为支持。Xray 官方明确说明 HTTP outbound 只能代理 TCP。citeturn630845search2turn630845search11

SOCKS5 是否实际支持 UDP，需要通过能力探测或用户配置确认，不能仅因协议名为 SOCKS5 就承诺 UDP 可用。

# 11. Core Compatibility Resolver

选择线路后不能直接生成配置，必须先做兼容性判断。

```
type CompatibilityResolver interface {
    Resolve(
        coreType core.Type,
        endpoint Endpoint,
        captureMode CaptureMode,
    ) CompatibilityResult
}
type CompatibilityResult struct {
    Supported bool
    Level     CompatibilityLevel
    Reasons   []CompatibilityReason
    Warnings  []CompatibilityWarning
}
```

可能结果：

```
Supported
SupportedWithLimitations
Unsupported
```

示例：

```
节点：VLESS Reality
内核：Xray
结果：支持

节点：TUIC
内核：Xray
结果：不支持
原因：当前固定 Xray 版本没有对应 outbound 编译器

线路：HTTP代理
模式：TUN
结果：有限支持
警告：UDP 流量无法通过当前 HTTP 出口
```

Mihomo、Xray 和 sing-box 的协议集合并不完全相同。Xray 官方列出的 outbound 包括 HTTP、SOCKS、Shadowsocks、Trojan、VLESS、VMess、WireGuard 和 Hysteria 等；sing-box 的 outbound 集合还包括 TUIC、Hysteria2、AnyTLS、Snell、Naive 等。citeturn630845search6turn630845search8turn630845search9turn630845search14

因此：

- 三个内核都必须支持机场来源；
- 不代表每一个机场节点都兼容三个内核；
- UI 必须根据选中内核置灰不兼容节点；
- 必须显示具体原因；
- 不得静默降级；
- 不得删除未知字段后尝试运行。

# 12. 三个内核必须有三个独立Adapter

不得使用一个巨大的 `switch` 在几十个文件里判断内核。

定义统一接口：

```
type CoreAdapter interface {
    Type() core.Type
    BinaryName() string

    DetectVersion(ctx context.Context, binaryPath string) (Version, error)
    Capabilities(version Version) CapabilitySet

    Compile(
        ctx context.Context,
        request CompileRequest,
    ) (CompiledConfig, error)

    Validate(
        ctx context.Context,
        installation CoreInstallation,
        config CompiledConfig,
    ) ValidationResult

    BuildLaunchSpec(
        installation CoreInstallation,
        config CompiledConfig,
    ) (LaunchSpec, error)

    HealthProbe(
        ctx context.Context,
        runtime RuntimeInfo,
    ) HealthResult

    MetricsReader(runtime RuntimeInfo) MetricsReader
}
```

分别实现：

```
MihomoAdapter
XrayAdapter
SingBoxAdapter
```

# 13. Config Compiler职责

每个编译器只生成对应内核的原生配置。

```
MihomoCompiler → YAML
XrayCompiler → JSON
SingBoxCompiler → JSON
```

禁止：

```
先生成 Mihomo YAML
再转成 Xray JSON
再转成 sing-box JSON
```

禁止把一个内核的运行配置直接交给另一个内核。

## 13.1 编译输入

```
type CompileRequest struct {
    Selection   ActiveSelection
    Endpoint    ResolvedEndpoint
    PortPlan    PortPlan
    DNSPolicy   DNSPolicy
    RoutePolicy RoutePolicy
    RuntimeDir  string
    RevisionID  string
}
```

## 13.2 编译输出

```
type CompiledConfig struct {
    CoreType       core.Type
    RevisionID     string
    MainConfigPath string
    WorkingDir     string
    ContentHash    string
    RedactedView   []byte
    SensitiveFiles []string
}
```

编译器不得直接启动内核。

# 14. 各内核配置要求

## 14.1 Mihomo

生成 Mihomo 原生 YAML。

至少包含：

- 本地 HTTP/SOCKS/Mixed 入口；
- 当前选中的单一代理节点；
- DIRECT；
- REJECT；
- 明确的代理组；
- 明确的默认规则；
- 本地 controller；
- 随机强 Secret；
- 日志等级；
- TUN 配置；
- DNS 配置；
- IPv6 策略；
- 路由规则。

Mihomo 使用 YAML 作为配置格式，并提供控制 API、日志、流量、配置和策略组接口。citeturn792604search3turn792604search4

不得直接运行机场完整 Clash 配置。

## 14.2 Xray

生成 Xray 原生 JSON。

至少包含：

- HTTP inbound；
- SOCKS inbound；
- 当前选中的 outbound；
- freedom；
- blackhole；
- routing；
- DNS；
- policy；
- stats；
- API 或可用的状态接口；
- 当前版本支持的 TUN 配置。

配置校验必须通过：

```
xray run -test -c <config>
```

Xray 官方将 `-test` 定义为只验证配置而不启动服务。citeturn630845search10turn630845search17

## 14.3 sing-box

生成 sing-box 原生 JSON。

至少包含：

- mixed 或 HTTP/SOCKS inbound；
- 当前选中的 outbound；
- direct；
- block；
- route；
- DNS；
- TUN；
- Clash API 或其他可用控制接口；
- 日志；
- 当前版本对应的新配置结构。

配置校验必须使用固定版本支持的：

```
sing-box check -c <config>
```

sing-box 官方提供 `check` 配置检查命令。citeturn176513search0turn176513search1

## 14.4 Mihomo校验命令

MihomoAdapter不得凭记忆硬编码参数。

必须在固定内核版本后：

1. 执行该二进制的 `-h` 或 `--help`；
2. 编写版本测试；
3. 将校验参数封装在 MihomoAdapter；
4. 在集成测试中验证错误配置确实返回非零退出码；
5. 将实际支持的命令记录在 `CORE_MANIFEST.json`。

# 15. Core Manifest

项目根目录必须存在：

```
{
  "schema_version": 1,
  "cores": [
    {
      "type": "mihomo",
      "version": "PINNED_VERSION",
      "relative_path": "third_party/cores/mihomo/.../mihomo.exe",
      "sha256": "PINNED_SHA256",
      "config_format": "yaml"
    },
    {
      "type": "xray",
      "version": "PINNED_VERSION",
      "relative_path": "third_party/cores/xray/.../xray.exe",
      "sha256": "PINNED_SHA256",
      "config_format": "json"
    },
    {
      "type": "sing-box",
      "version": "PINNED_VERSION",
      "relative_path": "third_party/cores/sing-box/.../sing-box.exe",
      "sha256": "PINNED_SHA256",
      "config_format": "json"
    }
  ]
}
```

禁止：

- 使用 `latest`；
- 运行时从任意 URL 下载；
- 未校验 SHA-256；
- 用户从 UI 输入内核路径；
- UI 输入任意启动参数；
- 自动替换内核但不做兼容测试。

# 16. 统一端口计划

所有内核对外暴露统一的逻辑入口：

```
type PortPlan struct {
    HTTPPort       uint16
    SOCKSPort      uint16
    ControllerPort uint16
    DNSPort        *uint16
}
```

端口必须集中管理，不得散落在：

- Wails/Vue 常量；
- Go Service 常量；
- 配置模板；
- PowerShell；
- 测试脚本。

如果当前 Navo 已有默认端口，应保留现有默认值并迁移到 `PortPlan`。

切换内核时，Agent 不应猜测当前内核端口，而应从 Service 获取：

```
type LocalProxyEndpoints struct {
    HTTPAddress  string
    SOCKSAddress string
}
```

系统代理只能在新内核完成健康检查后设置。

# 17. 连接事务

连接必须是事务，不是简单执行 `exec.Command`。

完整流程：

```
接收用户选择
    ↓
校验 CoreType
    ↓
校验 SourceType
    ↓
解析活动 Endpoint
    ↓
兼容性检查
    ↓
创建 Candidate Revision
    ↓
调用对应 Core Compiler
    ↓
写入临时安全目录
    ↓
调用对应内核原生配置校验
    ↓
停止旧内核
    ↓
清理旧运行状态
    ↓
启动新内核
    ↓
等待本地入口端口
    ↓
等待控制 API
    ↓
执行 DNS/TCP/HTTP 出口探测
    ↓
确认出口 IP
    ↓
应用系统代理或 TUN
    ↓
提交 Active Revision
```

任一步失败：

```
记录结构化错误
    ↓
停止失败内核
    ↓
清理临时网络状态
    ↓
恢复 Last Known Good Revision
    ↓
重新健康检查
    ↓
向 UI 返回明确错误
```

禁止在配置校验失败后继续启动内核。

禁止仅通过“进程存在”判断连接成功。

# 18. 切换内核流程

用户从 Mihomo 切换到 Xray 时：

```
保存用户选择
    ↓
不复用 Mihomo YAML
    ↓
使用当前线路重新生成 Xray JSON
    ↓
执行 Xray 原生校验
    ↓
停止 Mihomo
    ↓
启动 Xray
    ↓
健康检测
    ↓
成功后提交
```

失败时恢复 Mihomo 的最后可用 Revision。

切换 sing-box 同理。

必须保证：

- 切换后旧内核进程不存在；
- 旧控制端口已释放；
- 新内核日志来源明确；
- UI 展示的内核与实际进程一致；
- 系统代理指向真实监听端口；
- TUN 路由属于当前内核。

# 19. 切换线路来源流程

## 19.1 机场切换到全链路代理

```
保持当前 CoreType
    ↓
SourceType 改为 upstream_proxy
    ↓
清空活动 SubscriptionID 和 EndpointID
    ↓
设置 UpstreamProxyID
    ↓
重新编译当前内核配置
    ↓
事务应用
```

## 19.2 全链路代理切换到机场

```
保持当前 CoreType
    ↓
SourceType 改为 airport_subscription
    ↓
清空 UpstreamProxyID
    ↓
设置 SubscriptionID 和 EndpointID
    ↓
重新编译当前内核配置
    ↓
事务应用
```

禁止把未选择的另一种线路也写进活动配置。

# 20. Core Supervisor状态机

固定状态：

```
Stopped
Resolving
Compiling
Validating
StoppingPrevious
Starting
WaitingForInbound
Probing
ApplyingCapture
Running
Degraded
RollingBack
Recovering
Failed
```

所有连接、切换、断开操作必须串行。

同一时间只能存在一个变更事务。

```
var ErrOperationInProgress = errors.New("core operation already in progress")
```

Start、Stop、SwitchCore、SwitchSource 必须幂等。

# 21. IPC改造

IPC 请求不能只传字符串。

建议新增：

```
enum CoreType {
  CORE_TYPE_UNSPECIFIED = 0;
  CORE_TYPE_MIHOMO = 1;
  CORE_TYPE_XRAY = 2;
  CORE_TYPE_SING_BOX = 3;
}

enum SourceType {
  SOURCE_TYPE_UNSPECIFIED = 0;
  SOURCE_TYPE_AIRPORT_SUBSCRIPTION = 1;
  SOURCE_TYPE_UPSTREAM_PROXY = 2;
}

enum CaptureMode {
  CAPTURE_MODE_OFF = 0;
  CAPTURE_MODE_SYSTEM_PROXY = 1;
  CAPTURE_MODE_TUN = 2;
}

message ActiveSelection {
  CoreType core_type = 1;
  SourceType source_type = 2;
  CaptureMode capture_mode = 3;

  optional string subscription_id = 4;
  optional string endpoint_id = 5;
  optional string upstream_proxy_id = 6;
}

message ApplySelectionRequest {
  ActiveSelection selection = 1;
}

message CompatibilityResponse {
  bool supported = 1;
  repeated string reasons = 2;
  repeated string warnings = 3;
}
```

UI 发起连接时只提交结构化 ID，不得提交：

- 内核可执行文件路径；
- 完整密码；
- 任意配置文件；
- 任意启动参数。

# 22. UI改造要求

## 22.1 首页必须显示

```
当前内核：Mihomo / Xray / sing-box
线路来源：机场 / 全链路代理
当前线路：节点名或代理名
接管模式：系统代理 / TUN
运行状态：连接中 / 已连接 / 降级 / 失败
出口 IP
```

## 22.2 内核选择

使用单选组件：

```
运行内核

● Mihomo
○ Xray
○ sing-box
```

显示：

- 是否已安装；
- 固定版本；
- 是否通过哈希校验；
- 是否支持当前节点；
- 是否支持当前 CaptureMode；
- 不兼容原因。

## 22.3 线路来源

使用单选组件：

```
线路来源

● 机场订阅
○ 全链路代理
```

选择机场后显示：

- 订阅；
- 节点；
- 协议；
- 延迟；
- 当前内核兼容性。

选择全链路代理后显示：

- 代理名称；
- 协议；
- 地址；
- 端口；
- 是否认证；
- TCP 状态；
- UDP 状态；
- 当前内核兼容性。

## 22.4 连接按钮

连接按钮只有在以下条件全部成立时可点击：

```
内核已安装
内核哈希有效
线路已选择
节点或代理有效
兼容性检查通过
没有其他操作进行中
```

## 22.5 不允许虚假状态

UI 显示“已连接”必须同时满足：

```
选中内核进程正在运行
本地代理端口正在监听
配置 Revision 与当前选择一致
出口探测通过
系统代理或 TUN 状态符合选择
```

不能只因为用户点击了按钮就显示成功。

# 23. 数据库调整

目标数据库为用户云服务器中已有的 MySQL。Codex 不负责安装或运维 MySQL，
只负责 Navo 的接入、Schema、迁移、事务和连接可靠性。

连接约束：

- 只有 Go Service 的 Infrastructure 层可以连接 MySQL；
- Wails UI 和 Agent 不得持有数据库账号或直接访问 MySQL；
- 必须启用 TLS、连接超时、连接池上限和凭据保护；
- Schema 迁移必须可重复、可回滚并记录版本；
- 云端暂时不可用时不得丢失当前可用 Revision，也不得误报连接成功；
- 不得把 MySQL 暴露为无 TLS 的公网明文连接。

建议新增或调整以下表：

```
core_installations
subscriptions
providers
endpoints
endpoint_specs
upstream_proxies
active_selection
core_revisions
core_runtime_events
compatibility_cache
system_proxy_snapshots
network_recovery_records
```

## 23.1 active_selection

必须是单例记录或受唯一约束控制。

```
id = singleton
core_type
source_type
capture_mode
subscription_id
endpoint_id
upstream_proxy_id
updated_at
```

数据库约束和领域校验必须共同确保二选一。

## 23.2 core_revisions

```
revision_id
core_type
source_type
endpoint_reference
config_hash
created_at
validation_status
startup_status
health_status
is_active
is_last_known_good
```

不得把完整明文密码放入 Revision 元数据。

# 24. 旧数据迁移

Codex不得直接清空用户数据。

迁移步骤：

1. 备份当前 JSON 持久化文件和云端 MySQL 目标库（如已有数据）；
2. 检测旧 JSON 数据格式和 MySQL Schema Version；
3. 在 MySQL 事务中将旧单内核设置迁移为：

```
core_type = sing-box
```

1. 将旧订阅节点迁移到 `endpoints`；
2. 将旧 HTTP/SOCKS 数据迁移到 `upstream_proxies`；
3. 根据旧活动配置推断 SourceType；
4. 无法可靠推断时保持断开状态；
5. 不自动建立机场与代理链；
6. 迁移完成后运行完整领域校验；
7. 保留旧 JSON 备份、MySQL 备份和迁移日志。

# 25. 诊断与错误模型

必须使用稳定错误码。

```
CORE_NOT_INSTALLED
CORE_HASH_MISMATCH
CORE_VERSION_UNSUPPORTED
CORE_CONFIG_COMPILE_FAILED
CORE_CONFIG_INVALID
CORE_START_FAILED
CORE_EXITED_EARLY
CORE_HEALTH_TIMEOUT
LOCAL_PORT_NOT_LISTENING
CONTROLLER_NOT_READY

SOURCE_SELECTION_INVALID
ENDPOINT_NOT_FOUND
UPSTREAM_PROXY_NOT_FOUND
ENDPOINT_UNSUPPORTED_BY_CORE
CAPTURE_MODE_UNSUPPORTED_BY_CORE

SUBSCRIPTION_FETCH_FAILED
SUBSCRIPTION_FORMAT_UNSUPPORTED
SUBSCRIPTION_PARSE_FAILED
SUBSCRIPTION_EMPTY

DNS_PROBE_FAILED
TCP_PROBE_FAILED
HTTP_PROBE_FAILED
EGRESS_IP_PROBE_FAILED

SYSTEM_PROXY_APPLY_FAILED
SYSTEM_PROXY_RESTORE_FAILED
TUN_START_FAILED
TUN_RECONCILE_FAILED
ROLLBACK_FAILED
```

错误必须包含：

```
type AppError struct {
    Code       string
    Message    string
    Operation  string
    CoreType   *core.Type
    RevisionID *string
    Cause      error
    Details    map[string]string
}
```

UI 显示用户可理解的信息，诊断页面显示脱敏后的技术详情。

# 26. 日志要求

每次连接至少记录：

```
operation_id
revision_id
selected_core
selected_source
selected_endpoint_id
capture_mode
compiler_result
validator_exit_code
validator_stderr
process_id
local_port_status
controller_status
probe_result
rollback_result
```

必须脱敏：

- 订阅 URL；
- UUID；
- 密码；
- Token；
- Proxy Authorization；
- API Secret；
- 完整配置；
- 用户访问域名；
- Cookie；
- Header。

允许保存一份脱敏后的最终配置，用于诊断。

不得只记录：

```
连接失败
启动失败
未知错误
```

# 27. 配置与凭据安全

敏感数据必须：

- 使用 DPAPI；
- 严格 ACL；
- 临时配置仅允许 Service 和 Core Host 访问；
- 退出后清理临时配置；
- 崩溃启动时扫描并清理旧临时目录；
- 日志经过统一 Redactor；
- 诊断包默认不含完整配置；
- 诊断包由用户主动导出。

订阅 URL 本身也应视为凭据。

# 28. TUN要求

最终目标是三个内核都能在 Navo 中进入 TUN 模式。

但不得伪造支持状态。

每个 CoreAdapter 必须提供：

```
SupportsTUN(version Version) bool
```

在某个内核的固定版本和 TUN Adapter 未通过测试前：

- UI 显示“当前版本暂不支持”；
- 不允许启动；
- 系统代理模式仍可使用；
- 不得偷偷启动另一个代理内核承担 TUN；
- 不得同时运行 sing-box 和 Xray 来伪装成 Xray TUN。

最终完成标准仍然是：

```
Mihomo TUN 通过
Xray TUN 通过
sing-box TUN 通过
```

需要测试：

- DNS；
- UDP；
- IPv6；
- 路由回环；
- 睡眠恢复；
- 切换网络；
- Service 重启；
- Core 崩溃；
- 强制关机后的恢复；
- WSL；
- Docker；
- Hyper-V。

# 29. 系统代理要求

系统代理模式必须在三个内核中统一工作。

流程：

```
新内核启动
    ↓
本地 HTTP/SOCKS 入口确认监听
    ↓
Agent 保存原系统代理快照
    ↓
Agent 设置代理
    ↓
广播 Windows 设置变化
    ↓
执行实际访问探测
```

关闭时：

```
停止接收新请求
    ↓
恢复系统代理快照
    ↓
广播设置变化
    ↓
停止内核
```

程序崩溃、重启和卸载都必须有恢复路径。

# 30. 测试要求

## 30.1 单元测试

必须覆盖：

- ActiveSelection 二选一校验；
- CoreType 三选一校验；
- 每个协议 Parser；
- 每个 Core Compatibility；
- 每个 Core Compiler；
- 日志脱敏；
- 数据库迁移；
- Revision 回滚；
- 端口计划；
- 错误码映射。

## 30.2 Golden Config测试

目录：

```
configs/golden/mihomo/
configs/golden/xray/
configs/golden/singbox/
```

相同 Endpoint 输入分别生成三种配置，并与审查过的 Golden 文件比较。

## 30.3 六种组合测试

必须至少覆盖：

```
机场 + Mihomo
机场 + Xray
机场 + sing-box
HTTP/SOCKS代理 + Mihomo
HTTP/SOCKS代理 + Xray
HTTP/SOCKS代理 + sing-box
```

机场测试必须使用固定脱敏 Test Fixture，不得依赖用户真实订阅。

全链路代理测试应使用本地 Mock HTTP CONNECT 和 Mock SOCKS5 Server。

## 30.4 内核原生校验测试

每个 Golden Config 都必须传入对应固定内核执行原生校验。

不得只进行 JSON/YAML 语法校验。

## 30.5 集成测试

至少验证：

```
核心启动
本地端口监听
HTTP 请求通过
SOCKS 请求通过
出口选择正确
日志来源正确
断开后进程退出
失败后旧 Revision 恢复
```

## 30.6 Windows端到端测试

必须在真实 Windows 11 VM 执行：

```
系统代理连接
系统代理关闭
系统代理崩溃恢复
TUN 开启
TUN 关闭
切换三个内核
切换两种线路来源
睡眠恢复
切换 Wi-Fi
重启系统
杀死 UI
杀死 Agent
杀死 Service
杀死 Core Host
杀死代理内核
卸载恢复
```

# 31. Definition of Done

功能不能以“按钮能点击”作为完成标准。

一个运行组合只有在以下条件全部满足时才算完成：

```
配置编译成功
内核原生配置校验成功
选中内核进程启动成功
其他两个内核未运行
本地入口端口监听成功
控制接口或健康探测成功
DNS 探测成功
HTTP 探测成功
出口 IP 符合预期
系统代理或 TUN 状态正确
停止后网络正常恢复
日志已脱敏
失败时可回滚
自动化测试通过
```

整个多内核改造完成必须满足：

```
六种组合全部存在测试
三个内核都能被单独选择
机场和全链路代理都能被单独选择
机场和全链路代理不能同时激活
三个内核不能同时运行
不兼容节点有明确提示
旧数据可迁移
错误可以诊断
```

# 32. Codex实施顺序

禁止一次性重写整个项目。

## 阶段0：仓库审计

Codex必须先生成：

```
docs/audit/NAVO_CURRENT_STATE.md
```

内容包括：

- 当前目录树；
- Wails UI 入口；
- Agent入口；
- Service入口；
- 当前内核启动位置；
- 当前配置生成位置；
- 当前订阅解析位置；
- 当前代理数据结构；
- 当前系统代理实现；
- 当前 TUN 实现；
- 当前 JSON 持久化、MySQL Schema 与数据库连接状态；
- 当前 IPC 消息；
- 当前测试；
- 当前三个内核文件是否存在；
- 机场和全链路代理失败的实际调用链；
- 重复逻辑；
- 硬编码；
- 明文凭据；
- 未捕获错误。

没有完成审计前不得大规模移动文件。

## 阶段1：冻结领域模型

实现：

```
CoreType
SourceType
CaptureMode
ActiveSelection
Endpoint
UpstreamProxy
CompatibilityResult
```

添加领域单元测试。

此阶段不改 UI，不启动内核。

## 阶段2：抽象现有sing-box逻辑

将当前 sing-box 逻辑移动到：

```
internal/adapters/core/singbox/
```

使其实现 `CoreAdapter`。

必须保证当前 sing-box 功能不退化。

## 阶段3：加入MihomoAdapter

完成：

- Mihomo Compiler；
- Mihomo Validator；
- Mihomo Launcher；
- Mihomo Health Probe；
- Mihomo Metrics；
- Golden Config；
- 机场和全链路代理测试。

## 阶段4：加入XrayAdapter

完成：

- Xray Compiler；
- Xray Validator；
- Xray Launcher；
- Xray Health Probe；
- Xray Metrics；
- Golden Config；
- 机场和全链路代理测试。

## 阶段5：Compatibility Resolver

建立版本化兼容矩阵。

不得在 Wails/Vue 中硬编码协议兼容性。

## 阶段6：统一订阅模型

重构 Parser Registry。

验证常见订阅格式。

输出详细的接受和拒绝报告。

## 阶段7：统一全链路代理模型

重构 HTTP、HTTPS 和 SOCKS5。

三个 Adapter 分别生成原生 outbound。

## 阶段8：Active Selection事务

完成：

- 内核三选一；
- 来源二选一；
- 配置事务；
- Revision；
- 回滚；
- 状态机。

## 阶段9：IPC和UI

最后再改 Wails/Vue UI：

- 内核选择；
- 来源选择；
- 兼容性；
- 状态；
- 诊断；
- 错误展示。

## 阶段10：TUN和恢复

在系统代理模式全部稳定后，再逐个完成：

```
Mihomo TUN
sing-box TUN
Xray TUN
```

# 33. Codex每个任务的强制输出

每次提交修改后必须输出：

```
1. Root Cause
2. 修改目标
3. 修改文件清单
4. 每个文件为什么修改
5. 数据流变化
6. 已执行命令
7. 测试结果
8. 尚未完成的限制
9. 是否修改数据库
10. 是否需要人工 Windows 测试
```

必须实际运行：

```
go test ./...
go vet ./...
npm.cmd run typecheck --prefix navo_app
npm.cmd run build --prefix navo_app
```

如果项目已配置其他 Lint，必须一并运行。

不能只说“理论上可以”。

# 34. Codex禁止事项

Codex不得：

- 删除 Mihomo；
- 删除 Xray；
- 删除 sing-box；
- 删除机场；
- 删除全链路代理；
- 把机场和全链路代理自动串联；
- 同时启动多个代理内核；
- 使用一个通用配置文件启动三个内核；
- 继续扩大简化 ProxyNode；
- 丢弃 Reality、TLS、Transport 等字段；
- 把不兼容节点标记为可用；
- 只修 UI 不修配置链路；
- 只检查进程存在；
- 配置校验失败后仍启动；
- 捕获错误后只返回“连接失败”；
- 重新引入已淘汰的旧 UI 构建链；
- 修改 Wails/WebView2 底层实现规避架构问题；
- 引入未经批准的大型依赖；
- 直接复制 GPL 代码而不处理许可证；
- 在日志输出密码和订阅 URL；
- 使用真实用户订阅做自动测试；
- 一次提交数十个无关改动；
- 在没有测试的情况下宣布完成。

# 35. Codex收到本文档后的第一条执行任务

Codex应立即执行以下任务，而不是直接开始重写：

```
你正在修改 Navo Windows 代理客户端。

先不要编写多内核实现，也不要修改 UI。

第一步完整审计当前仓库，并创建：
docs/audit/NAVO_CURRENT_STATE.md

审计目标：

1. 找出当前 Wails UI、Agent、Service 和代理内核的所有入口。
2. 找出机场订阅从请求到节点保存、配置生成、内核启动的完整调用链。
3. 找出全链路 HTTP/SOCKS 代理从保存到配置生成、内核启动的完整调用链。
4. 找出当前两个功能都无法使用的共同故障点。
5. 找出所有与 sing-box 强耦合的接口、结构和目录。
6. 找出所有简化代理节点结构及其丢失的协议字段。
7. 找出当前系统代理、TUN、DNS、路由、端口和健康检查实现。
8. 找出当前 JSON 持久化、MySQL Schema、IPC Contract 和 Core Revision 逻辑。
9. 找出三个内核二进制是否已经存在，以及当前版本、路径和哈希。
10. 找出缺少测试的模块。

文档中必须包含：

- 当前目录树；
- 当前架构图；
- 两条失败调用链；
- Root Cause；
- 可复用代码；
- 必须重构代码；
- 数据迁移风险；
- 按阶段修改计划；
- 第一阶段建议修改的具体文件；
- 不得修改的现有稳定模块。

完成审计后停止，不得直接大规模修改代码。
等待审计结果被确认后，再实施领域模型和 CoreAdapter 抽象。
```

# 36. 最终架构原则

Navo 的正确实现不是：

```
三个内核共用一份配置
机场和代理共用一个简化结构
出问题后不断增加 if/else
```

而是：

```
统一产品模型
    ↓
单一活动选择
    ↓
协议完整数据
    ↓
兼容性判断
    ↓
对应内核独立编译
    ↓
对应内核原生校验
    ↓
只启动一个内核
    ↓
真实健康检查
    ↓
事务提交或回滚
```

最终必须做到：

> 三个内核全部保留，运行时三选一；机场和全链路代理全部保留，运行时二选一；配置分别生成，状态统一管理，错误可以诊断，失败可以恢复。
