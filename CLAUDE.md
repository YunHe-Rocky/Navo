# Navo — AI 驱动的智能网络出口管理器

## 项目定位

Navo 是一个 Windows 智能代理客户端，以 sing-box 为数据面核心，提供 AI 辅助的流量管理。它不是一个传统"机场客户端"，而是**网络出口的智能控制平面**。

```
用户意图 → 智能规则 → 流量识别 → 自动出口选择 → 实时监控 → AI 优化
```

## 技术栈

| 层 | 技术 | 说明 |
|----|------|------|
| 数据面 | sing-box 1.13+ | 网络转发核心 |
| 控制面 | Go 1.26 | 生命周期/状态机/配置编译 |
| 系统集成 | PowerShell / Wintun | 路由/DNS/防火墙/TUN 适配器 |
| 通信 | Named Pipe IPC + Flutter MethodChannel | 本地进程间通信，无 HTTP 控制层 |
| AI | OpenAI 兼容 API | 规则生成/诊断/解释，UI 可配置 |
| UI | Flutter 3.44 | 桌面 UI（window_manager + tray_manager） |

## 项目结构

```
Navo/
├── cmd/
│   ├── navo/main.go              ← 单进程桌面启动器（Service + Agent + Flutter UI）
│   │   └── tray_windows.go       ← 原生 Win32 托盘图标（Shell_NotifyIcon）
│   ├── navo-svc/main.go          ← 独立 Windows Service 入口（开发测试用）
│   ├── navo-agent/main.go        ← 独立 User Agent 入口（开发测试用）
│   └── repair/main.go            ← 网络修复工具
│
├── internal/
│   ├── ai/                       ← AI 规则生成/诊断/解释
│   │   ├── ai.go                 后端接口（OpenAI 兼容 HTTP）
│   │   ├── rulegen.go            自然语言 → 路由规则
│   │   ├── diagnosis.go          AI + 离线规则诊断
│   │   └── explain.go            AI + 规则中文解释
│   │
│   ├── agent/                    ← 用户会话代理
│   │   └── systemproxy/          Windows WinINET 系统代理管理
│   │
│   ├── compiler/                 ← 领域模型 → sing-box JSON
│   │   ├── model.go              Config/Outbound/Rule/DNS/TUN 类型
│   │   ├── generator.go          JSON 生成（TUN/路由/DNS）
│   │   ├── validator.go          语义校验
│   │   └── compiler.go           编译管道 + 配置版本管理
│   │
│   ├── health/                   ← sing-box 健康检查
│   │   └── checker.go            进程存活 + 端口监听检测
│   │
│   ├── host/                     ← sing-box 进程生命周期
│   │   ├── host.go               CoreHost 接口定义
│   │   └── singbox.go            SingBoxHost 实现
│   │
│   ├── ipc/                      ← 进程间通信协议
│   │   ├── envelope.go           消息信封 + 方法常量
│   │   └── messages.go           请求/响应/事件类型
│   │
│   ├── ipdetect/                 ← IP 归属检测
│   │   └── echo.go               IP echo 服务查询 + 缓存
│   │
│   ├── monitor/                  ← 网络质量监控
│   │   ├── probe.go              TCP/DNS/HTTPS 主动探测
│   │   ├── collector.go          流量统计 + 规则命中
│   │   └── store.go              JSON 文件持久化指标
│   │
│   ├── network/                  ← TUN 网络管理
│   │   ├── types.go              Config/IPv6Mode/Executor/Platform
│   │   ├── manager.go           事务性路由/DNS/防火墙管理
│   │   ├── journal.go           网络操作日志（崩溃恢复）
│   │   ├── reconciler.go        启动前网络状态校正
│   │   ├── platform_windows.go  管理员权限/Wintun 检查
│   │   ├── executor_windows.go   PowerShell 命令执行
│   │   └── tun/                  TUN 适配器接口（Wintun/netsh）
│   │
│   ├── pipe/                     ← 命名管道传输（并发多实例 + Deadline）
│   │   ├── pipe.go              帧协议（NVOP magic + length）
│   │   ├── pipe_windows.go       Windows Named Pipe（32 实例并发 Accept + CancelIoEx 超时）
│   │   └── pipe_stub.go          Unix domain socket 存根（非 Windows 测试用）
│   │
│   ├── recovery/                 ← 崩溃恢复状态持久化
│   │   └── reconciler.go        DIRTY_SHUTDOWN 检测与恢复
│   │
│   ├── securestore/               ← Windows DPAPI 加密
│   │   ├── protect_windows.go     CryptProtectData/CryptUnprotectData
│   │   └── protect_stub.go        非 Windows 存根
│   │
│   ├── service/                  ← 服务编排层
│   │   ├── service.go           全部模块接线 + IPC dispatch
│   │   ├── runtime.go           运行时配置生成/应用/模式切换
│   │   └── ai_settings.go      AI 配置持久化（DPAPI 保护）
│   │
│   ├── storage/                  ← 键值存储
│   │   └── store.go             JSON 文件原子写入
│   │
│   ├── subscription/             ← 机场订阅管理
│   │   ├── fetcher.go           HTTPS 下载 + SSRF 防护
│   │   ├── normalizer.go        去重/标准化
│   │   ├── subscription.go      订阅管理器（定时更新）
│   │   └── parser/              协议解析器
│   │       ├── clash.go         Clash YAML
│   │       ├── ss.go            Shadowsocks URI
│   │       ├── vmess.go         VMess JSON
│   │       ├── vless.go         VLESS URI
│   │       ├── trojan.go        Trojan URI
│   │       └── socks.go         SOCKS5 URI
│   │
│   └── supervisor/               ← 核心状态机
│       ├── state.go             状态定义 + 合法转移表
│       └── supervisor.go        启动/停止/崩溃恢复编排
│
├── configs/                      ← sing-box 配置文件
│   ├── test_local.json          mixed inbound + direct（本地测试）
│   └── test_tun.json            TUN inbound + DNS + hijack
│
├── navo_app/                    ← Flutter 桌面 UI
│   ├── lib/
│   │   ├── main.dart             窗口/托盘管理 + 导航框架
│   │   ├── theme.dart            OLED 暗色主题 + 语义色板
│   │   ├── services/api.dart     Named Pipe MethodChannel 客户端
│   │   ├── screens/              仪表盘/出站/订阅/TUN/AI/设置
│   │   └── widgets/              侧边栏/状态栏
│   └── windows/runner/           C++ 原生 MethodChannel 传输层
│       └── named_pipe_client.cpp Flutter → Agent Pipe 客户端
│
├── third_party/
│   ├── sing-box/                 sing-box.exe 1.13.14
│   └── wintun/                   wintun.dll 0.14.1
│
├── scripts/
│   └── package.ps1               Windows 发布打包脚本
│
├── go.mod                        module navo (零外部 Go 依赖)
├── Makefile                      build/test/vet/coverage/package
└── CLAUDE.md                     本文件
```

## 单进程架构

`navo.exe` 是唯一的发布入口，Service 和 Agent 作为 goroutine 运行在同一进程内，不再需要独立的进程间 Named Pipe 通信。Agent→Service 通信通过直接函数调用 `SendToServiceFn` 完成，无需序列化开销。

```
navo.exe (单进程)
├── Service goroutine   → sing-box.exe (数据面子进程)
├── Agent goroutine     → 系统代理 + UI Named Pipe 监听
│   └── 原生 Win32 托盘 (Shell_NotifyIcon，左右键可操作)
└── navo_app.exe (Flutter UI 子进程，--start-hidden 启动，仅托盘可见)
```

**通信链路**：

| 链路 | 传输 | 说明 |
|------|------|------|
| Flutter → Agent | Named Pipe `Navo.UI.Agent.v1` | Flutter MethodChannel → C++ Native → Pipe |
| Agent → Service | 进程内直接调用 `svc.Dispatch()` | `SendToServiceFn` 注入，无 Pipe |
| Service → Core | CoreHost 接口 | 进程内方法调用 |

**原生托盘**（`cmd/navo/tray_windows.go`）：
- 左键 → `ShowWindow` + `SetForegroundWindow` 显示 Flutter 窗口
- 右键 → `TrackPopupMenu` 弹出菜单（打开 Navo / 退出）
- 不依赖 Flutter tray_manager，作为可靠兜底

**独立模式**：`navo-svc.exe` 和 `navo-agent.exe` 保留用于开发测试——它们通过 Named Pipe 通信，不需要混合启动器。

## IPC 协议

### 帧格式

Named Pipe 上所有消息都使用帧协议封装：

```
[4 bytes "NVOP"] [4 bytes LE length] [N bytes JSON payload]
```

最大 payload 10MB，超过则拒绝发送。

### 全部方法

| 类别 | 方法 |
|------|------|
| 核心 | `core.start/stop/restart/status`, `service.shutdown` |
| 出口 | `outbound.list/select` |
| 运行时 | `runtime.status/mode.set` |
| TUN | `tun.enable/disable/status/config` |
| 订阅 | `subscription.add/remove/list/refresh` |
| 指标 | `metrics.current` |
| IP | `ip.check` |
| AI | `ai.rule.generate/diagnose/explain`, `ai.config.get/set/test` |

## 关键设计决策

### TUN 模式
- sing-box 持有 Wintun 数据面适配器，Service 负责控制面（路由/DNS/防火墙）
- **Split-default routing**：`0.0.0.0/1` + `128.0.0.0/1` 覆盖默认路由，保留原始默认路由
- **NRPT DNS 防泄漏**：`Add-DnsClientNrptRule -Namespace '.'` 将所有 DNS 导向 TUN
- **Network Journal**：每条操作先写 pending undo → 成功后标记 applied → 失败时逆序回滚
- **IPv6 三模式**：`block`（防火墙阻断，默认） / `tunnel`（`::/1` + `8000::/1`）/ `passthrough`

### 安全
- **Fetcher SSRF 防护**：仅 HTTPS、禁止内网 IP/localhost/file://、3 次重定向上限、10MB 上限
- **双层密钥**：Agent DPAPI User Scope（用户凭证）/ Service DPAPI Machine Scope（系统密钥）
- **订阅安全**：只提取节点信息（server/port/protocol/password/TLS），禁止控制本地端口/文件/脚本
- **Named Pipe ACL**：仅允许当前用户 + SYSTEM，显式拒绝 Everyone

### 崩溃恢复
- **Supervisor 状态机**：Stopped → Reconcile → Ready → Starting → Running
- **Reconciler**：检查 recovery_state.json → 清理残留路由/DNS/TUN → 标记 READY
- **指数退避**：3s → 10s → 30s → Failed（不再自动重启）
- **Network Journal**：独立于进程崩溃，启动时自动回滚未完成的网络操作

### 编译器
- 领域模型到 sing-box JSON 的纯翻译层，无外部依赖
- 配置版本管理（pending → active → rollback → rejected）
- 支持 10 种出站类型 + 10 种规则类型 + TUN + DNS

### Named Pipe 服务端
- **并发 Accept**：32 个 pipe 实例各自在独立 goroutine 中 `ConnectNamedPipe`，通过 channel 投递到 `Accept()`。避免单实例模式下 Flutter 并发请求触发 `ERROR_PIPE_BUSY` 重试风暴
- **Deadline 实现**：`pipeConn.Read/Write` 使用 `WaitForSingleObject` + 计算超时 + `CancelIoEx` 取消，替代原来的 `INFINITE` 永久阻塞
- 帧格式：`[4 bytes "NVOP"] [4 bytes LE length] [N bytes JSON payload]`

### 托盘
- **Go 原生托盘**（`cmd/navo/tray_windows.go`）：Win32 `Shell_NotifyIcon` + `TrackPopupMenu`，不依赖 Flutter
- **Flutter 托盘**（`navo_app/lib/main.dart`）：`tray_manager` 插件，右键菜单含代理切换/TUN/路由模式
- 窗口始终 `--start-hidden` 启动，用户通过托盘左键或"打开 Navo"菜单项唤醒

### AI
- OpenAI 兼容 API（DeepSeek 等）
- 配置方式：UI 内直接编辑（Base URL / API Key / Model / Timeout），或环境变量 `NAVO_AI_BASE_URL` / `NAVO_AI_API_KEY` / `NAVO_AI_MODEL`
- API Key 通过 Windows DPAPI 加密持久化存储
- 离线 fallback：无 API 时使用规则引擎生成诊断和建议
- Prompt 温度 0.1，max_tokens 1024

## 构建与测试

```bash
# 首次构建前：安装 Windows 资源生成工具
go install github.com/tc-hib/go-winres@latest

# 生成/更新 Windows 资源文件（图标 + 版本信息）
cd cmd/navo && go generate && cd ../..

# 构建全部（GUI 模式，无终端窗口，图标已嵌入）
make build                 # → navo.exe (-ldflags="-H windowsgui")

# 运行测试
make test                  # → go test ./... -v -count=1

# 静态检查
make vet

# 覆盖率
make coverage

# 单二进制（图标自动嵌入，.syso 文件在 cmd/navo/ 目录下）
GOARCH=amd64 go build -ldflags="-H windowsgui" -o navo.exe ./cmd/navo
go build -o navo-svc.exe ./cmd/navo-svc
go build -o navo-agent.exe ./cmd/navo-agent
go build -o repair.exe ./cmd/repair

# Flutter UI（构建后需复制到 release 目录）
cd navo_app && flutter build windows
# 然后复制 build/windows/x64/runner/Release/navo_app.exe 和 data/ 到 app_ui/

# 发布打包
make package   # 调用 scripts/package.ps1

# 重启开发（杀掉旧进程 → 构建 → 启动）
go build -ldflags="-H windowsgui" -o release/Navo/navo.exe ./cmd/navo
```

### Windows 图标资源

`navo.exe` 的图标通过 `cmd/navo/winres/navo.ico` 嵌入：
- `.syso` 文件（`cmd/navo/rsrc_windows_amd64.syso`）已提交到版本控制，**日常构建无需 `go generate`**
- 修改图标后运行 `cd cmd/navo && go generate` 重新生成 `.syso`
- `.syso` 架构特定（amd64），Go 编译器自动链接，`go build` 即可
- 托盘图标从嵌入数据自动写入 `data/navo.ico`，无需部署额外的图标文件

**图标一致性**：
| 位置 | 图标来源 | 说明 |
|------|----------|------|
| navo.exe (Explorer) | `cmd/navo/rsrc_windows_amd64.syso` | Go 资源嵌入 |
| navo_app.exe (Explorer) | `navo_app/windows/runner/resources/app_icon.ico` | Flutter Runner.rc |
| 系统托盘 | `cmd/navo/winres/navo.ico`（Go embed 写入 temp） | 运行时自动释放 |
| Flutter 窗口标题栏/任务栏 | `navo_app.exe` 自身资源 | 由 Runner.rc 决定 |
| 所有图标使用同一源文件 | `navo_app/assets/tray_icon.ico` | 统一图标 |

## 集成测试

```bash
# 需要 sing-box + Wintun 二进制
go test -tags=integration ./internal/host/ -v -count=1
```

## AI 服务配置

```bash
set NAVO_AI_BASE_URL=https://api.deepseek.com/v1
set NAVO_AI_API_KEY=sk-xxxxxxxx
set NAVO_AI_MODEL=deepseek-chat
```

未配置时 AI 功能自动降级为离线规则引擎。

## 开发阶段

| Phase | 内容 | 状态 |
|-------|------|------|
| 0 | sing-box 启停核心验证 | ✅ |
| 1 | CoreHost + Supervisor + Config Compiler | ✅ |
| 2 | Windows Service + Agent + IPC + 系统代理 | ✅ |
| 3 | TUN 模式（Wintun/Route/DNS/Recovery） | ✅ |
| 4 | 订阅系统 + 监控系统 + IP 检测 | ✅ |
| 5 | AI 规则生成 + 诊断 + 解释 | ✅ |
| 6 | Flutter UI + 桌面启动器 + 系统托盘 | ✅ |
| 7 | Local-only IPC 重构（移除 HTTP 控制层） | ✅ |
