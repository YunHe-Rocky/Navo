# Navo Windows 安装与部署

## 1. 适用范围

本文档适用于 Navo 的 Windows x64 开发、构建、测试和本地部署。

目标桌面架构：

- Go Service：配置编译、订阅管理、内核管理和网络状态恢复。
- Go Agent：用户会话和 Named Pipe IPC。
- Wails v2 + Vue 3：桌面 UI。
- WebView2：Windows UI 渲染运行时。
- sing-box、Mihomo、Xray：可切换代理内核。

当前仓库已完成 Flutter 到 Wails 的构建链迁移，`scripts/package.ps1`
不再检查或调用 Flutter、Dart、CMake 和 Ninja。

## 2. 系统要求

### 2.1 开发机

- Windows 10 22H2 或 Windows 11，64 位。
- PowerShell 5.1 或更高版本。
- Git。
- Go 1.26.4，版本必须与 `go.mod` 一致。
- Node.js 20 或更高版本，包含 npm。
- Microsoft Edge WebView2 Runtime。
- Visual Studio Build Tools，安装“使用 C++ 的桌面开发”工作负载。

不再需要：

- Flutter SDK。
- Dart SDK。
- Android SDK。

检查环境：

```powershell
git --version
go version
node --version
npm --version
```

WebView2 在常规 Windows 10/11 环境中通常已经安装。如果精简系统缺少
WebView2，安装 Microsoft Evergreen WebView2 Runtime：

https://developer.microsoft.com/microsoft-edge/webview2/

### 2.2 运行机器

- Windows 10 22H2 或 Windows 11，64 位。
- Microsoft Edge WebView2 Runtime。
- 当前发布包由单进程 launcher 托管 TUN/core，因此启动 `navo.exe` 时必须
  批准 Windows UAC。拒绝 UAC 时应用不会启动。

## 3. 获取源码

```powershell
git clone <repository-url> Navo
Set-Location .\Navo
```

检查工作目录：

```powershell
Test-Path .\go.mod
Test-Path .\scripts
Test-Path .\third_party
```

三项结果都必须为 `True`。

## 4. 安装三内核

目录结构必须保持如下形式：

```text
third_party/
├── sing-box/
│   ├── sing-box.exe
│   └── LICENSE
├── mihomo/
│   ├── mihomo.exe
│   └── LICENSE
├── xray/
│   ├── xray.exe
│   ├── geoip.dat
│   ├── geosite.dat
│   └── LICENSE
└── wintun/
    ├── wintun.dll
    └── LICENSE.txt
```

内核应从官方 Release 下载：

- sing-box：https://github.com/SagerNet/sing-box/releases
- Mihomo：https://github.com/MetaCubeX/mihomo/releases
- Xray：https://github.com/XTLS/Xray-core/releases
- Wintun：https://www.wintun.net/

验证可执行文件：

```powershell
.\third_party\sing-box\sing-box.exe version
.\third_party\mihomo\mihomo.exe -v
.\third_party\xray\xray.exe version
```

计算 SHA-256：

```powershell
Get-FileHash .\third_party\sing-box\sing-box.exe -Algorithm SHA256
Get-FileHash .\third_party\mihomo\mihomo.exe -Algorithm SHA256
Get-FileHash .\third_party\xray\xray.exe -Algorithm SHA256
Get-FileHash .\third_party\wintun\wintun.dll -Algorithm SHA256
```

发布前必须将结果与官方 Release 提供的校验值比对。不要使用来源不明的
二次打包内核。

## 5. 安装项目依赖

Go 依赖：

```powershell
go mod download
go mod verify
```

安装前端依赖：

```powershell
Set-Location .\navo_app
npm ci
Set-Location ..
```

CI 和正式构建必须使用 `npm ci`，不要使用会修改锁文件的 `npm install`。

### 5.1 本地状态

Navo 不连接数据库。当前节点选择、配置 revision 和 committed runtime
保存在 `%LOCALAPPDATA%\Navo\state` 的版本化本地文件中，写入使用同目录临时
文件、flush 和原子替换。敏感订阅与代理凭据继续使用 DPAPI Current User
保护，不得写入普通日志或环境变量。

## 6. 构建前检查

执行 Go 单元测试和静态检查：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test.ps1
```

预期结果：

- 所有 `go test ./...` 测试通过。
- `go vet ./...` 没有错误。

还必须执行：

```powershell
Set-Location .\navo_app
npm run build
Set-Location ..
```

`npm run build` 已包含 `vue-tsc --noEmit`。仓库没有伪造的空测试命令；
桌面 UI 行为由 `scripts/ui_smoke.cjs` 和完整打包烟测验证。任何一步失败都
必须停止发布。

## 7. 构建发布包

使用统一打包入口：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package.ps1
```

禁止直接发布 `scripts/build.ps1` 生成的 launcher-only 产物。

目标产物：

```text
release/Navo-<VERSION>-portable-amd64/
├── navo.exe
├── repair.exe
├── VERSION
├── CORE_MANIFEST.json
├── THIRD_PARTY_NOTICES.md
├── SHA256SUMS.txt
├── app_ui/
│   └── navo_app.exe
├── third_party/
│   ├── sing-box/
│   ├── mihomo/
│   ├── xray/
│   └── wintun/
└── README.txt
release/Navo-<VERSION>-portable-amd64.zip
```

严格复验目录和 ZIP：

```powershell
$version = (Get-Content .\VERSION -Raw).Trim()
$packageRoot = ".\release\Navo-$version-portable-amd64"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify-package.ps1 -PackageRoot $packageRoot -ArchivePath "$packageRoot.zip" -ExpectedVersion $version
```

## 8. 发布前冒烟测试

在管理员 PowerShell 中，安装 Python 3 后执行：

```powershell
$version = (Get-Content .\VERSION -Raw).Trim()
$packageRoot = ".\release\Navo-$version-portable-amd64"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke.ps1 -PackageRoot $packageRoot
```

冒烟测试必须覆盖：

1. 启动 `navo.exe`、Service、Agent 和 UI。
2. Named Pipe 请求与响应。
3. sing-box、Mihomo、Xray 依次切换。
4. 三个内核的配置校验和进程启动。
5. 通过本地 HTTP 服务验证真实代理数据面。
6. 内核停止和重新启动。
7. `service.shutdown` 正常退出。
8. 退出后没有新增的 Navo 或内核残留进程。

检查恢复工具：

```powershell
.\release\Navo-<VERSION>-portable-amd64\repair.exe check
```

`issues_found` 必须为 `0`。

TUN 会修改系统路由，自动冒烟测试默认不启用。正式发布前应在独立虚拟机中
额外验证：

- TUN 启用和禁用。
- 管理员权限提示。
- 默认路由恢复。
- 异常终止后的网络恢复。

## 9. 本地部署

不要只复制单个 `navo.exe`。必须部署完整的版本化绿色目录或解压完整 ZIP。

建议安装目录：

```text
C:\Program Files\Navo\
```

测试部署可使用：

```text
D:\Apps\Navo\
```

部署步骤：

1. 退出已有 Navo。
2. 确认 sing-box、Mihomo、Xray 和 `navo_app.exe` 均已退出。
3. 将完整 `release\Navo-<VERSION>-portable-amd64` 目录复制到目标位置。
4. 双击 `navo.exe` 并批准 Windows UAC。
5. 检查 UI、托盘菜单和三个内核状态。

Navo 的可写数据不应放在安装目录：

```text
%LOCALAPPDATA%\Navo\
```

日志默认位于：

```text
<安装目录>\log\navo.log
```

安装目录不可写时回退到 `%LOCALAPPDATA%\Navo\log\navo.log`。

## 10. 升级

升级前备份：

```powershell
$source = Join-Path $env:LOCALAPPDATA "Navo"
$backup = Join-Path $env:LOCALAPPDATA (
    "Navo.backup." + (Get-Date -Format "yyyyMMdd-HHmmss")
)
Copy-Item -LiteralPath $source -Destination $backup -Recurse
```

升级步骤：

1. 从 UI 或托盘正常退出 Navo。
2. 运行 `repair.exe check`，确认网络状态正常。
3. 保留 `%LOCALAPPDATA%\Navo`。
4. 用新版本完整替换安装目录。
5. 启动新版本并执行订阅刷新、内核切换和代理访问验证。

不要混合新旧版本的 `navo.exe`、UI 和内核目录。

## 11. 回滚

1. 正常退出当前版本。
2. 恢复上一版本的完整安装目录。
3. 必要时恢复 `%LOCALAPPDATA%\Navo` 备份。
4. 启动后检查订阅、活动节点、系统代理和运行内核。
5. 执行 `repair.exe check`。

## 12. 卸载

1. 正常退出 Navo。
2. 执行：

```powershell
.\repair.exe check
```

3. 删除 Navo 安装目录。
4. 如果不保留订阅和设置，再删除：

```text
%LOCALAPPDATA%\Navo\
```

删除用户数据不可恢复。需要保留配置时不要执行第 4 步。

## 13. 常见问题

### UI 无法启动

检查：

```powershell
Test-Path .\app_ui\navo_app.exe
```

如果文件存在但无法启动，安装或修复 WebView2 Runtime，并查看
`log\navo.log`。

### 某个内核无法启动

分别执行版本命令，并检查对应内核是否被安全软件隔离。随后检查生成配置：

```text
%LOCALAPPDATA%\Navo\runtime.*.json
```

Xray 的待校验配置文件必须以 `.json` 结尾，否则 Xray 无法识别格式。

### Named Pipe 连接中断

确认 `navo.exe`、Agent 和 `navo_app.exe` 属于同一登录用户会话。不要单独启动
`app_ui\navo_app.exe`。

### 退出后无法联网

以管理员身份执行：

```powershell
.\repair.exe check
```

确认问题后再执行修复操作，避免覆盖用户原本的代理或路由设置。

## 14. 发布检查清单

- [ ] Go 测试全部通过。
- [ ] `go vet` 通过。
- [ ] 前端 typecheck、test、build 全部通过。
- [ ] npm high 级别依赖审计、race detector、govulncheck 通过。
- [ ] 三个内核来自官方 Release，SHA-256 和许可证已核对。
- [ ] VERSION、diagnostics、三个 PE metadata 与发布名一致。
- [ ] 绿色目录、ZIP、SHA256SUMS 和闭合文件集合验证通过。
- [ ] 三内核切换和真实代理数据面通过。
- [ ] 正常退出后没有新增残留进程。
- [ ] `repair.exe check` 返回零问题。
- [ ] 独立虚拟机完成 TUN 和异常恢复测试。
- [ ] 物理重启、升级、回滚和卸载流程均已验证。
- [ ] Authenticode 签名和最终 ZIP SHA-256 已验证。
- [ ] 实际运行 binary 的 path、hash 与发布版本一致。
