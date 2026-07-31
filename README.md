# Navo

### AI 驱动的 Windows 智能网络出口管理器

统一管理订阅节点、独立代理、多代理内核、系统代理与 TUN 网络接管。

**Navo 不只是传统的订阅客户端，而是一个面向 Windows 的网络出口控制平台。**

</div>

---

## 项目简介

Navo 是一款面向 Windows 平台开发的多内核代理管理工具。

项目将代理来源、代理内核和网络接管模式进行分层管理，让用户能够在同一个桌面客户端中完成：

- 订阅节点管理
- 独立代理管理
- sing-box、Mihomo、Xray 内核切换
- Windows 系统代理接管
- TUN 虚拟网卡接管
- 入口 IP 与出口 IP 检测
- 实时流量和网络质量监控
- AI 辅助规则生成与网络诊断
- 异常退出后的网络状态恢复

Navo 的核心目标是：

```text
用户意图
    ↓
代理来源选择
    ↓
内核配置编译
    ↓
流量接管
    ↓
出口选择
    ↓
运行状态监控
    ↓
异常恢复与 AI 辅助诊断
```

> [!IMPORTANT]
> Navo 当前仍处于持续开发和验证阶段。
>
> 系统代理模式已经具备较完整的运行链路，TUN 模式仍在针对不同 Windows 网络环境进行兼容性验证。

---

## 核心特性

### 多代理内核

Navo 保留三种代理内核，并采用运行时三选一的方式进行管理：

| 内核 | 系统代理 | TUN 模式 | 说明 |
|---|:---:|:---:|---|
| sing-box | ✅ | ✅ | 默认数据面内核 |
| Mihomo | ✅ | ✅ | 支持 Clash 生态配置 |
| Xray | ✅ | 暂不支持 | 当前 TUN 模式会返回明确的不支持提示 |

每个内核拥有独立的：

- 配置编译器
- 版本检测
- 启动参数
- 原生配置校验
- 健康检查
- 能力声明
- 运行状态管理

同一时间只允许运行一个代理内核，避免端口、路由、DNS 和虚拟网卡发生冲突。

---

### 两种代理来源

Navo 将代理来源划分为两类：

#### 机场订阅

用于导入和管理订阅节点。

支持订阅下载、节点解析、标准化、去重、刷新和节点选择。

#### 独立代理

用于管理单独购买的代理服务，例如：

- HTTP 代理
- SOCKS5 代理
- 静态住宅代理
- 独享代理
- 全链路代理

机场订阅和独立代理在运行时二选一，避免多个代理来源同时控制出口。

---

### 三种网络接管模式

| 模式 | 说明 |
|---|---|
| 不托管 | 停止代理内核，不修改 Windows 系统网络设置 |
| 系统代理 | 设置当前用户的 WinINet 系统代理 |
| TUN 模式 | 使用虚拟网卡接管系统网络流量 |

#### 系统代理模式

适用于：

- 浏览器
- 支持 Windows 系统代理的软件
- 常规 HTTP 和 HTTPS 流量

Navo 不会仅根据进程启动状态判断连接成功。

系统代理启用前会进行真实的代理出口请求，只有数据链路验证通过后，才会写入 Windows 系统代理设置。

#### TUN 模式

适用于：

- 不支持系统代理的软件
- 需要接管 UDP 流量的应用
- 需要全局网络接管的场景
- 需要应用或域名分流的场景

TUN 模式涉及：

- Wintun 虚拟网卡
- Windows 路由表
- DNS 配置
- 防火墙规则
- IPv4 和 IPv6 策略
- DNS 防泄漏
- 崩溃恢复日志

---

## 网络安全与恢复

Navo 会修改 Windows 系统代理、路由、DNS 和虚拟网卡状态，因此项目重点设计了网络事务和恢复机制。

### 事务化模式切换

模式切换不是简单地停止一个内核再启动另一个内核。

完整流程包括：

1. 锁定当前模式切换事务
2. 保存当前网络状态
3. 停止旧的流量接管
4. 清理旧路由、DNS 和系统代理
5. 编译目标内核配置
6. 执行内核原生配置校验
7. 启动代理内核
8. 检查本地监听端口
9. 执行真实出口连接检测
10. 应用新的系统代理或 TUN 状态
11. 失败时自动回滚

### Network Journal

Navo 使用网络操作日志记录路由、DNS、防火墙和虚拟网卡变更。

每一项网络操作都会记录对应的撤销动作。

发生以下情况时，Navo 可以根据日志执行恢复：

- 程序崩溃
- 代理内核异常退出
- Windows 强制关机
- 模式切换失败
- TUN 虚拟网卡异常
- 路由或 DNS 配置残留

### 虚拟网卡保护

- 只在 TUN 模式下创建和使用虚拟网卡
- 不依赖用户可修改的网卡显示名称识别设备
- 监控虚拟网卡是否被禁用或删除
- 网卡异常时停止代理内核
- 再次启动时重新检查并恢复运行环境
- 离开 TUN 模式时清理 Navo 创建的网络状态

### 网络修复工具

项目包含独立的：

```text
repair.exe
```

建议先执行只读检查：

```powershell
.\repair.exe check
```

确认问题后，再使用对应的修复操作。

不要在不了解影响的情况下直接修改系统路由或 DNS。

---

## AI 辅助能力

Navo 支持兼容 OpenAI API 格式的 AI 服务。

AI 功能不会直接替代代理内核，而是作为网络控制面的辅助能力。

目前主要用于：

- 自然语言生成分流规则
- 网络异常诊断
- 配置和规则解释
- 节点选择建议
- 错误日志分析
- 网络状态说明

AI 服务地址、模型和密钥由用户自行配置。

未配置 AI 服务时，不影响基础代理功能运行。

---

## IP 与流量监控

Navo 提供网络出口和运行状态监控能力，包括：

- 当前入口 IP
- 当前出口 IP
- IP 国家或地区
- 当前代理内核
- 当前代理来源
- 当前接管模式
- 当前上传速度
- 当前下载速度
- 节点延迟
- DNS 探测
- TCP 探测
- HTTPS 探测
- 规则命中统计
- 网络错误日志

Dashboard 使用本地运行状态快照，不需要持续高频访问外部接口。

---

## 桌面与托盘

Navo 使用 Windows 原生托盘与 Wails 桌面界面组合。

托盘负责提供可靠的后台控制能力，即使桌面界面关闭，代理运行状态也可以继续保持。

托盘计划提供以下分级控制：

- 开启或关闭连接
- 选择机场订阅或独立代理
- 选择代理节点
- 选择 sing-box、Mihomo 或 Xray
- 选择系统代理或 TUN
- 选择全局、规则或直连策略
- 打开主界面
- 完全退出 Navo

程序还具有单实例保护。

重复启动 Navo 时，不会再启动一套代理服务，而是尝试唤醒已有窗口。

---

## 技术栈

| 模块 | 技术 |
|---|---|
| 控制面 | Go 1.26 |
| 桌面框架 | Wails v2 |
| 前端框架 | Vue 3 |
| 前端语言 | TypeScript |
| 构建工具 | Vite |
| 数据面 | sing-box、Mihomo、Xray |
| 系统代理 | Windows WinINet |
| TUN | Wintun |
| 系统集成 | Win32 API、PowerShell |
| 本地通信 | Windows Named Pipe |
| 凭据保护 | Windows DPAPI |
| 云端数据 | MySQL，可选 |
| AI | OpenAI 兼容 API |
| 配置格式 | JSON、YAML |
| 发布脚本 | PowerShell |

---

## 架构概览

```text
┌─────────────────────────────────────────────┐
│                  navo.exe                   │
│                                             │
│  ┌────────────────┐   ┌─────────────────┐  │
│  │ User Agent     │   │ Service         │  │
│  │                │   │                 │  │
│  │ 系统代理       │──▶│ 配置编译        │  │
│  │ UI 通信        │   │ 状态机          │  │
│  │ 托盘控制       │   │ 网络事务        │  │
│  └───────┬────────┘   └────────┬────────┘  │
│          │                     │            │
└──────────┼─────────────────────┼────────────┘
           │                     │
   Windows Named Pipe            │
           │                     ▼
┌──────────▼─────────┐   ┌───────────────────┐
│ navo_app.exe       │   │ Proxy Core        │
│                    │   │                   │
│ Wails + Vue 3      │   │ sing-box          │
│ TypeScript + Vite  │   │ Mihomo            │
│ WebView2           │   │ Xray              │
└────────────────────┘   └───────────────────┘
```

Navo 的正式发布入口为：

```text
navo.exe
```

桌面界面作为独立的 Wails 子进程运行：

```text
app_ui/navo_app.exe
```

代理内核同样作为受控子进程运行。

Windows Job Object 用于管理子进程生命周期，降低主程序异常退出后遗留代理内核或 WebView2 进程的风险。

---

## 项目结构

```text
Navo/
├── cmd/
│   ├── navo/                 # 正式桌面启动器
│   ├── navo-agent/           # 独立 Agent 调试入口
│   ├── navo-svc/             # 独立 Service 调试入口
│   └── repair/               # 网络修复工具
│
├── internal/
│   ├── agent/                # 用户会话与系统代理
│   ├── ai/                   # AI 规则、诊断和解释
│   ├── compiler/             # 代理内核配置编译
│   ├── coreadapter/          # 三内核适配器
│   ├── domain/               # 领域模型
│   ├── health/               # 内核健康检查
│   ├── infrastructure/       # MySQL 和运行配置
│   ├── ipdetect/             # IP 检测
│   ├── monitor/              # 网络质量和流量监控
│   ├── network/              # TUN、路由和 DNS
│   ├── pipe/                 # Windows Named Pipe
│   ├── recovery/             # 崩溃恢复
│   ├── securestore/          # DPAPI 凭据保护
│   ├── service/              # 服务编排
│   ├── subscription/         # 订阅管理和协议解析
│   └── supervisor/           # 内核运行状态机
│
├── navo_app/                 # Wails + Vue 桌面界面
├── third_party/              # 三个代理内核及运行依赖
├── configs/                  # 测试配置
├── docs/                     # 设计、部署和测试文档
├── scripts/                  # 构建、测试和发布脚本
├── CORE_MANIFEST.json        # 内核版本和哈希清单
├── .env.example              # 环境变量示例
├── go.mod
└── progress.md
```

---

## 环境要求

### 运行环境

- Windows 10 或 Windows 11
- 64 位操作系统
- Microsoft Edge WebView2 Runtime
- TUN 模式需要管理员权限
- 允许 Navo 和代理内核通过 Windows 防火墙

### 开发环境

- Go 1.26.4 或兼容版本
- Node.js 与 npm
- PowerShell
- Microsoft Edge WebView2 Runtime
- `go-winres` v0.3.3
- sing-box
- Mihomo
- Xray

安装 `go-winres`：

```powershell
go install github.com/tc-hib/go-winres@v0.3.3
```

---

## 获取源码

```powershell
git clone https://github.com/YunHe-Rocky/Navo.git
cd Navo
```

下载 Go 依赖：

```powershell
go mod download
```

---

## 准备代理内核

完整发布构建需要以下文件：

```text
third_party/
├── sing-box/
│   └── sing-box.exe
├── mihomo/
│   └── mihomo.exe
└── xray/
    └── xray.exe
```

内核版本和 SHA-256 必须与以下文件中的声明一致：

```text
CORE_MANIFEST.json
```

如果内核版本或哈希不一致，Navo 会拒绝启动，防止运行未知或被替换的核心文件。

---

## 构建项目

使用项目提供的 PowerShell 构建脚本：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package.ps1
```

构建脚本会完成：

1. 编译 `navo.exe`
2. 编译 `repair.exe`
3. 安装前端依赖
4. 执行 Vue TypeScript 检查
5. 执行 Vite 生产构建
6. 构建 Wails 桌面程序
7. 复制三种代理内核
8. 复制配置示例和部署文档
9. 生成 SHA-256 文件清单

默认输出目录：

```text
release/Navo/
```

启动程序：

```powershell
.\release\Navo\navo.exe
```

Windows 会显示 UAC 提权窗口，这是因为当前完整版本需要管理 TUN、路由、DNS 和代理内核。

---

## 运行测试

执行完整测试：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test.ps1
```

也可以单独执行 Go 测试：

```powershell
go test ./...
go vet ./...
```

前端检查：

```powershell
cd navo_app
npm.cmd ci
npm.cmd run typecheck
npm.cmd run build
```

测试通过不仅要求配置能够生成，还需要验证：

- 内核原生配置校验
- 内核进程启动
- 本地端口监听
- HTTP 数据面请求
- 系统代理写入与恢复
- 模式切换
- 三内核切换
- 程序正常退出
- 无残留代理进程
- 无残留网络状态

---

## 配置文件

发布版本从以下位置读取配置：

```text
%LOCALAPPDATA%\Navo\.env
```

首先复制示例配置：

```powershell
Copy-Item .\release\Navo\.env.example "$env:LOCALAPPDATA\Navo\.env"
```

### MySQL

MySQL 默认关闭：

```env
NAVO_MYSQL_ENABLED=false
NAVO_MYSQL_REQUIRED=false
```

启用后可以配置：

```env
NAVO_MYSQL_HOST=mysql.example.com
NAVO_MYSQL_PORT=3306
NAVO_MYSQL_DATABASE=navo
NAVO_MYSQL_USER=navo_app
NAVO_MYSQL_PASSWORD=
NAVO_MYSQL_TLS_MODE=verify_identity
```

MySQL 不可用时：

- `NAVO_MYSQL_REQUIRED=false`：使用本地 LastKnownGood 状态继续运行
- `NAVO_MYSQL_REQUIRED=true`：启动失败，不进行明文或不安全降级

### AI

```env
NAVO_AI_BASE_URL=
NAVO_AI_API_KEY=
NAVO_AI_MODEL=
```

不要将真实 API Key、数据库密码、代理密码或订阅地址提交到 GitHub。

---

## 数据与日志

默认数据目录：

```text
%LOCALAPPDATA%\Navo\
```

默认日志：

```text
%LOCALAPPDATA%\Navo\log\navo.log
```

运行状态、网络恢复日志和本地 LastKnownGood 数据也会保存在 Navo 数据目录中。

---

## 安全设计

Navo 包含以下安全措施：

- 订阅仅允许 HTTPS 下载
- 阻止访问 localhost 和内网地址，降低 SSRF 风险
- 限制订阅响应大小和重定向次数
- 代理凭据使用 Windows DPAPI 保护
- Named Pipe 限制为当前用户、管理员和 SYSTEM
- MySQL 支持 TLS 身份验证
- 不允许数据库 TLS 失败后自动降级为明文
- 三个代理内核进行版本和 SHA-256 校验
- 网络操作支持日志、回滚和崩溃恢复
- 发布包不会包含开发环境中的真实 `.env`

---

## 当前开发重点

- [x] Wails + Vue 桌面界面
- [x] sing-box、Mihomo、Xray 三内核适配
- [x] 系统代理真实数据面验证
- [x] 内核切换和状态恢复
- [x] Windows 原生托盘
- [x] 可选 MySQL 状态存储
- [x] AI 配置、规则和诊断接口
- [x] 独立网络修复工具
- [x] 内核版本与哈希验证
- [x] 打包脚本和 SHA-256 清单
- [ ] 完成更多真实 Windows 环境下的 TUN 验收
- [ ] 完善虚拟网卡禁用和删除后的自动恢复
- [ ] 完善模式切换期间的零丢包处理
- [ ] 完善节点质量与 IP 纯净度展示
- [ ] 完善流量折线图和历史统计
- [ ] 发布稳定安装包
- [ ] Android 客户端与云端同步

---

## 常见问题

### 为什么启动时需要管理员权限？

Navo 的完整版本需要管理：

- TUN 虚拟网卡
- Windows 路由
- DNS
- 防火墙规则
- 代理内核进程

因此发布版本会请求管理员权限。

### 为什么提示缺少 WebView2？

Wails 桌面界面依赖 Microsoft Edge WebView2 Runtime。

请安装或修复 WebView2 Runtime 后重新启动 Navo。

### 为什么切换到 Xray 后无法启用 TUN？

当前 Xray 适配器暂不提供 TUN 模式。

可以使用：

- Xray + 系统代理
- sing-box + TUN
- Mihomo + TUN

Navo 会明确返回不支持错误，不会生成一个看似成功但实际无法工作的 Xray TUN 配置。

### 为什么程序显示启动成功，但网络仍不可用？

请查看：

```text
%LOCALAPPDATA%\Navo\log\navo.log
```

重点检查：

- 代理节点是否有效
- 当前核心是否支持所选协议
- 本地端口是否被占用
- Windows 防火墙是否拦截
- DNS 是否正常
- TUN 是否获得管理员权限
- 内核文件哈希是否正确
- 上游代理用户名和密码是否正确

### 程序异常退出后网络不正常怎么办？

首先执行：

```powershell
.\repair.exe check
```

根据检查结果再执行修复操作。

不要直接删除未知路由、虚拟网卡或 DNS 配置。

---

## 参与贡献

欢迎通过 Issue 和 Pull Request 参与项目。

提交前请确保：

- 不包含订阅地址、密码、Token 或 API Key
- `go test ./...` 通过
- `go vet ./...` 通过
- Vue 类型检查通过
- Vite 生产构建通过
- 不破坏系统代理和 TUN 状态恢复
- 新增内核功能时补充真实数据面测试
- 涉及 Windows 网络状态时提供回滚逻辑

建议使用清晰的提交信息：

```text
feat: add ...
fix: resolve ...
refactor: improve ...
test: add ...
docs: update ...
```

---

## 开源许可证

当前仓库尚未提供 `LICENSE` 文件。

在正式添加开源许可证之前，项目代码默认保留全部权利，不代表允许任意复制、修改、重新发布或商业使用。

---

## 免责声明

Navo 仅用于合法的网络管理、软件开发、技术研究和个人网络环境配置。

使用者应遵守所在国家或地区的法律法规，以及网络服务提供商和相关平台的使用条款。

本项目不提供：

- 代理节点
- 机场订阅
- 静态住宅 IP
- 网络访问账号
- 第三方服务凭据

因代理服务质量、错误配置、网络环境变化、第三方内核、节点安全问题或不当使用造成的损失，由使用者自行承担。

---

<div align="center">

**Navo**

让代理来源、网络出口、运行模式和系统状态更加清晰、可控、可恢复。

</div>

把这段保存为仓库根目录的 `README.md` 即可。当前仓库里还没有 `README.md` 和 `LICENSE`，所以不会覆盖已有文件。
