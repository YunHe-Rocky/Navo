# Navo 开发指南

Navo 当前采用 Go + Wails v2 + Vue 3，不包含 Flutter、AI、MySQL 或云端同步。VERSION 是正式版本号的唯一来源。

## 生产架构

~~~
navo.exe（管理员进程）
├── Service：订阅、配置编译、内核 Supervisor、SelfHeal
├── Agent：System Proxy、TUN、DNS、Route、Named Pipe IPC
├── Native Tray
└── app_ui/navo_app.exe（Wails v2 + Vue 3）
~~~

正式 launcher 在单一管理员进程内托管 Service 与 Agent，UI 是受控子进程。cmd/navo-svc 和 cmd/navo-agent 仅用于开发验证。

## 关键边界

- 网络接管状态只有 off、system_proxy、tun。
- Xray 当前不支持 TUN，compiler 和 UI 必须 fail closed。
- 物理 TUN adapter 只接受 Navo 拥有的 canonical Wintun identity，不得修改 Ethernet 或 Wi-Fi。
- System Proxy、DNS、Route 和 TUN 的切换是完整事务，失败必须回滚并保留 Network Journal。
- Service child request ID 与 UI parent request ID 分离；重试不能放宽 replay fingerprint 校验。
- 订阅拉取拒绝 loopback、private、link-local、multicast 和 unspecified 地址，并在全部已验证 public IP 间有界回退。
- 订阅、credential、token 和 runtime config 不得写入日志、artifacts 或仓库。
- 不得停止未确认属于 Navo 的 v2rayN、代理内核或用户进程。

## 目录职责

- cmd/navo：正式 launcher、tray 和 UI 子进程管理。
- cmd/repair：只读检查与显式网络修复。
- internal/service：控制面与 runtime orchestration。
- internal/agent：用户会话和 capture state machine。
- internal/compiler：多内核配置生成与校验。
- internal/network：TUN、DNS、Route、Journal 和 rollback。
- internal/subscription：订阅获取、SSRF 防护和解析。
- internal/securestore：Windows DPAPI。
- internal/buildinfo：ldflags 注入版本。
- navo_app：Wails backend 与 Vue frontend。
- scripts：测试、package、verify、smoke 和管理员验收。
- third_party：固定版本内核、Wintun 和许可证。

## 本地数据

优先使用 %LOCALAPPDATA%\Navo；不可用时回退到 executableDir\data。launcher 日志优先写 executableDir\log\navo.log，目录不可写时回退到本地数据目录。敏感字段使用 DPAPI Current User，日志不得包含 credential 或完整订阅。

## 开发命令

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test.ps1

Set-Location .\navo_app
npm ci
npm test
npm run typecheck
npm run build
Set-Location ..
~~~

launcher-only 构建只用于调试，禁止作为正式发布物：

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
~~~

## 发布流程

唯一正式入口：

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package.ps1
~~~

默认生成 release\Navo-<VERSION>-portable-amd64 绿色目录与同名 ZIP，不生成 MSI/Setup。

package.ps1 必须：

1. 在修改 release 前完成 Go 和 frontend 门禁。
2. 从 VERSION 注入 buildinfo 与三个 PE 的 version metadata。
3. 校验 CORE_MANIFEST.json 和第三方 hash。
4. 要求 sing-box、Mihomo、Xray、Wintun 许可证存在。
5. 生成闭合文件集合 SHA256SUMS.txt。
6. 用 scripts\verify-package.ps1 校验目录和 ZIP。
7. 输出最终 archive SHA-256。

## 测试策略

CI 门禁包括 go test、go vet、50% 总覆盖率回归线、Linux race detector、govulncheck、npm test/typecheck/build/audit，以及 PowerShell parser。覆盖率不能替代关键行为测试；network\tun、systemproxy、service 应优先补状态转换、超时、取消、回滚和重复请求用例。

在管理员 PowerShell 中运行本地 package smoke：

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke.ps1 -PackageRoot .\release\Navo-<VERSION>-portable-amd64
~~~

smoke 必须基于启动前 PID baseline，只清理本次新增且已验证属于 Navo 的进程。

## Windows 发布验收

build、listener、UI status 或 repair check 不是端到端证明。管理员验收必须覆盖：

- System Proxy/TUN 启用、禁用、切换和循环。
- DNS、TCP、HTTPS、出口 IP。
- Google、GitHub、ChatGPT/Codex 的认证、资源与 streaming/WebSocket。
- bypass_mainland、global、direct 与名单规则。
- core crash、自愈、取消 UAC、异常退出和 rollback。
- 睡眠唤醒、物理重启与升级回滚。
- 实际进程 path、hash、VERSION、PE metadata。
- Authenticode 与最终 ZIP hash。

需要 UAC、物理重启、证书或外部账号的项目必须标记 pending，不得用模拟结果代替。

## 修改规则

- 保持 Service、Agent、UI DTO 向后兼容。
- Windows 与 non-Windows 实现用 build tags 隔离。
- 外部 I/O 必须有 context、deadline、大小上限和可观测错误。
- 网络修改前记录 ownership，rollback 只恢复 Navo 实际拥有的状态。
- release、cache、runtime log 和 artifacts 不进入提交。
- 新增第三方 binary 时同步更新 CORE_MANIFEST、notices、许可证和 verifier。
- 发布脚本变更至少验证 parser、负向验包和完整 package。
- 不弱化校验来让测试通过。
