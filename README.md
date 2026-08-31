# Navo

Navo 是面向 Windows 10/11 x64 的本地代理与网络接管桌面应用。Go 负责控制面与恢复逻辑，Wails v2 + Vue 3 提供桌面 UI，可在 sing-box、Mihomo、Xray 三种内核之间切换。

根目录 VERSION 是正式版本号的唯一来源。项目不依赖云端数据库，不包含 AI 服务，订阅和凭据均保存在本机。

## 核心能力

- 订阅与独立代理：订阅拉取、节点解析、测速、选择和运行时应用。
- 多内核：切换前执行能力检查和配置验证。
- 网络接管：支持 off、System Proxy、TUN；Xray 当前明确不支持 TUN。
- 路由策略：支持绕过大陆、全局代理、全局直连与黑白名单。
- 生命周期保护：模式切换使用事务状态机，失败时回滚并保留 Network Journal。
- 本地安全：敏感数据使用 Windows DPAPI Current User，拒绝明文降级。
- 故障恢复：repair.exe 提供只读检查与显式修复，不默认覆盖用户原有网络配置。
- 桌面体验：原生托盘负责可靠唤醒和退出，Wails UI 展示状态、订阅、路由与诊断。

## 架构

~~~
navo.exe（管理员进程）
├── Service：订阅、编译、内核 Supervisor、SelfHeal
├── Agent：用户会话、System Proxy、TUN、Named Pipe IPC
├── Native Tray
└── app_ui/navo_app.exe：Wails v2 + Vue 3

Service / Agent / UI
        ├── Navo.Service.v1
        ├── Navo.Agent.v1
        └── Navo.UI.Agent.v1
                └── sing-box / Mihomo / Xray + Wintun
~~~

关键目录：

- cmd/navo：正式桌面 launcher。
- cmd/repair：网络检查与恢复工具。
- internal/service：控制面和运行时编排。
- internal/agent：用户会话、模式切换和系统网络操作。
- internal/compiler：三内核配置编译。
- internal/subscription：订阅获取、解析与 SSRF 防护。
- internal/network：TUN、DNS、Route、Journal 和 rollback。
- navo_app：Wails v2 + Vue 3 桌面应用。
- scripts：测试、package、verify、smoke 和 Windows 验收脚本。
- third_party：代理内核、Wintun 及许可证。

## 环境要求

运行环境：

- Windows 10 22H2 或 Windows 11 x64。
- Microsoft Edge WebView2 Runtime。
- 启动 navo.exe 时批准 UAC；拒绝后不会进入半连接状态。

开发环境：

- Go 版本与 go.mod 一致。
- Node.js 20+ 与 npm。
- PowerShell 5.1+。
- Visual Studio Build Tools，包含“使用 C++ 的桌面开发”。
- go-winres v0.3.3，仅正式打包需要。

## 第三方内核

构建脚本不会联网下载内核。必须从官方 Release 准备：

~~~
third_party/
├── sing-box/sing-box.exe
├── mihomo/mihomo.exe
├── xray/xray.exe
└── wintun/wintun.dll
~~~

CORE_MANIFEST.json 固定已接受的版本和 SHA-256。发布前必须核对官方校验值、文件 hash 与许可证，来源和许可证见 THIRD_PARTY_NOTICES.md。

## 开发与验证

Go：

~~~powershell
go mod verify
go test ./...
go vet ./...
~~~

前端：

~~~powershell
Set-Location .\navo_app
npm ci
npm test
npm run typecheck
npm run build
Set-Location ..
~~~

Go 本地门禁：

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test.ps1
~~~

CI 额外执行 50% Go 总覆盖率回归线、Linux race detector、govulncheck、npm high 级别依赖审计和 PowerShell parser。覆盖率不等于真实 Windows 网络验收。

## 发布

Navo 只交付绿色目录和 ZIP，不以 MSI/Setup 作为当前发布物：

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package.ps1
~~~

默认输出：

~~~
release/Navo-<VERSION>-portable-amd64/
release/Navo-<VERSION>-portable-amd64.zip
~~~

打包入口先执行 Go 与前端门禁，再完成：

- 从 VERSION 注入 diagnostics、Go build metadata 和三个 PE 版本。
- 核对 CORE_MANIFEST.json 与三内核/Wintun hash。
- 要求第三方组件携带许可证。
- 生成 SHA256SUMS.txt。
- 拒绝 manifest 之外的额外文件。
- 逐文件校验 ZIP 与绿色目录的长度和 SHA-256。

单独复验：

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify-package.ps1 -PackageRoot .\release\Navo-<VERSION>-portable-amd64 -ArchivePath .\release\Navo-<VERSION>-portable-amd64.zip -ExpectedVersion <VERSION>
~~~

RequireSignature 用于已具备代码签名证书的正式环境。未签名构建不能宣称通过 Authenticode。

## 数据、日志与隐私

- 主数据目录：%LOCALAPPDATA%\Navo。
- launcher 日志：优先写 package\log\navo.log；目录不可写时回退到 %LOCALAPPDATA%\Navo\log\navo.log。
- 结构化日志：默认写入 %LOCALAPPDATA%\Navo\structured.log.jsonl；每条记录保存严重级别、服务分级、具体服务和组件，并在写入前脱敏。
- 设置与日志页支持按严重级别、服务分级、具体服务和时间组合查询；首次打开默认展示“基础服务”，全部取消服务分级表示不限制分类。
- 状态采用同目录临时文件、flush 和原子替换。
- 订阅 URL、代理凭据和运行配置不得写入仓库、CI 日志或公开 artifacts。
- artifacts\ 已忽略；历史诊断产物应在确认留存需求后单独脱敏或清理。
- 从 UI 或托盘正常退出，不终止用户拥有的其他代理进程。

## 发布验收边界

单元测试、build、监听端口或 UI 状态均不代表端到端网络可用。正式发布还要在独立、可回滚的管理员 Windows 环境验证：

1. System Proxy 和 TUN 启用、禁用、切换与重复循环。
2. DNS、TCP、HTTPS 和真实出口 IP。
3. Google、GitHub、ChatGPT/Codex 的认证、静态资源与流式连接。
4. 三种路由模式与黑白名单语义。
5. 内核崩溃、自愈、应用异常退出与网络回滚。
6. 睡眠唤醒、物理重启和旧版本升级。
7. 实际进程 path、hash、VERSION 与 PE metadata。
8. Authenticode 和最终 ZIP SHA-256。

未完成的外部验收必须标为 pending，不能由本地测试替代。

## 常见问题

### 为什么需要管理员权限？

TUN、Route、DNS 和故障恢复涉及系统级网络状态。当前 launcher 在同一管理员进程内托管控制面，因此启动时需要 UAC。

### 为什么 Xray 不能启用 TUN？

当前 Xray adapter 未实现并验证 TUN。系统会显式拒绝该组合，避免生成表面成功但数据面不可用的配置。

### UI 无法启动怎么办？

确认 app_ui\navo_app.exe 存在，并安装或修复 WebView2 Runtime。不要绕过 navo.exe 单独启动 UI。

### 退出后无法联网怎么办？

先以管理员身份执行只读检查：

~~~powershell
.\repair.exe check
~~~

确认问题后再选择具体修复，避免覆盖原有代理、DNS 或 Route。

## 许可证

仓库当前未声明统一的项目开源许可证。第三方组件仍受各自许可证约束，详见 THIRD_PARTY_NOTICES.md；未获得明确授权前，不应将 Navo 源码视为可自由再分发。
