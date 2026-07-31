# NAVO Windows 代理与 TUN 全链路排错及验收手册

> 文档版本：2026-07-29  
> 适用平台：Windows 10 / Windows 11 x64  
> 适用内核：sing-box、Mihomo、Xray  
> 对照实现：Clash Verge Rev、v2rayN  
> 目标：按照本文从上到下执行，所有“硬闸门”通过后，NAVO 的系统代理模式与 TUN 模式应具备可用、可恢复、可定位故障的完整能力。

---

## 0. 结论先行

NAVO 的测试不能只验证：

- 节点延迟成功；
- 核心进程存在；
- TUN 网卡成功生成；
- UI 显示“已连接”。

这些只能证明某一个局部步骤成功，不能证明真实业务链路成功。

本文使用以下两条端到端链路作为最终判定标准。

### 0.1 系统代理模式

```text
支持系统代理的应用
  → Windows 当前用户 WinINet 代理设置
  → NAVO 本地 HTTP/Mixed 入站
  → sing-box / Mihomo / Xray
  → 机场节点或独享 HTTP/SOCKS5 代理
  → 目标网站
```

### 0.2 TUN 模式

```text
任意应用产生的 IP 流量
  → Windows 路由表
  → NAVO TUN/Wintun 网卡
  → sing-box / Mihomo / Xray
  → 机场节点或独享 HTTP/SOCKS5 代理
  → 目标网站
```

### 0.3 最终硬性标准

只有同时满足以下条件，UI 才允许显示 `Running/已连接`：

1. 配置已生成并通过对应内核的原生校验。
2. 核心进程已启动且启动后未立即退出。
3. 本地入站端口由当前核心 PID 真实监听。
4. 本地 HTTP/SOCKS 协议握手成功。
5. 显式经过本地代理的端到端请求成功。
6. 系统代理模式下，当前桌面用户的系统代理已正确应用。
7. TUN 模式下，路由、DNS、物理出口绑定与回环保护正确。
8. 不显式指定代理的请求能够在对应模式下得到代理出口 IP。
9. 停止、崩溃、切换网络后，系统网络能够恢复。
10. 日志能明确指出失败发生在哪一层，而不是只返回“启动失败”。

---

# 1. 三个内核的定位与实现差异

## 1.1 sing-box

### 适合场景

- 作为 NAVO 第一优先级、标准实现内核。
- 机场节点和独享 HTTP/SOCKS5 代理。
- 系统代理与 TUN。
- 现代协议、复杂 DNS、规则路由。
- JSON 配置，适合程序化生成。

### 系统代理入口

必须生成 HTTP 能力入口：

```json
{
  "type": "mixed",
  "tag": "mixed-in",
  "listen": "127.0.0.1",
  "listen_port": 2080
}
```

Windows 系统代理应指向：

```text
127.0.0.1:2080
```

不要仅生成纯 SOCKS 入站后，假设所有 Windows 系统代理应用都能稳定使用。

### TUN 重点

至少检查：

- `type: tun`
- `address`
- `auto_route: true`
- `strict_route`
- `stack`
- DNS 配置
- `route.auto_detect_interface: true`，或者显式绑定物理接口
- 代理服务器 IP 不进入 TUN
- 核心自身流量不进入 TUN

较新 sing-box 版本的 TUN 字段持续演进，必须先读取实际二进制版本，再按版本生成配置，禁止把最新版配置字段直接写入旧版本。

### 原生配置校验

```powershell
& $CoreExe check -c $ConfigFile
if ($LASTEXITCODE -ne 0) {
    throw "sing-box config validation failed"
}
```

---

## 1.2 Mihomo

### 适合场景

- Clash 订阅和 Clash 配置生态。
- Proxy Provider、Proxy Group、Rule Provider。
- 系统代理与 TUN。
- External Controller API，可用于运行状态、日志、代理组选择和连接检查。
- YAML 配置。

### 系统代理入口

推荐统一使用：

```yaml
mixed-port: 2080
allow-lan: false
bind-address: 127.0.0.1
```

Windows 系统代理指向：

```text
127.0.0.1:2080
```

### TUN 重点

至少检查：

```yaml
tun:
  enable: true
  stack: mixed
  device: NAVO-TUN
  auto-route: true
  auto-detect-interface: true
  strict-route: true
  dns-hijack:
    - any:53
```

同时要求：

- DNS 模块已启用。
- 物理出口自动检测正确。
- Firewall 已放行 Mihomo 内核。
- External Controller 只监听本地地址，且配置 secret。
- `mixed-port`、controller port、TUN 名称不能与旧进程冲突。

### 原生配置校验

```powershell
& $CoreExe -t -f $ConfigFile -d $CoreHome
if ($LASTEXITCODE -ne 0) {
    throw "Mihomo config validation failed"
}
```

---

## 1.3 Xray

### 适合场景

- Xray 原生协议和精细路由。
- 独立 HTTP、SOCKS 入站。
- JSON 配置。
- 新版本已提供 Windows TUN 能力，但版本差异必须严格控制。

### 系统代理入口

Xray 不应照搬 sing-box/Mihomo 的 mixed-port 概念。

至少生成独立 HTTP 入站：

```json
{
  "listen": "127.0.0.1",
  "port": 2081,
  "protocol": "http",
  "tag": "http-in",
  "settings": {}
}
```

可另行生成 SOCKS 入站供显式 SOCKS 测试：

```json
{
  "listen": "127.0.0.1",
  "port": 2082,
  "protocol": "socks",
  "tag": "socks-in",
  "settings": {
    "udp": true
  }
}
```

Windows 系统代理应优先指向 HTTP 入站：

```text
127.0.0.1:2081
```

### TUN 重点

较新 Xray 文档中可配置：

```json
{
  "protocol": "tun",
  "settings": {
    "name": "NAVO-TUN",
    "desc": "NAVO",
    "mtu": 1500,
    "gateway": ["10.89.0.1/30"],
    "dns": ["10.89.0.2"],
    "autoSystemRoutingTable": ["0.0.0.0/0"],
    "autoOutboundsInterface": "auto"
  }
}
```

但旧版本 Xray 的 TUN 可能只负责创建网卡，路由和地址仍需应用层设置。

因此 Xray 适配器必须：

1. 记录 Xray 精确版本。
2. 根据版本选择“内核自动路由”或“NAVO 手动路由”。
3. 不允许两套实现同时修改相同路由。
4. 必须确保 `wintun.dll` 与 `xray.exe` 架构匹配，并位于可加载位置。
5. 必须验证 `autoOutboundsInterface` 或 `sockopt.interface`，防止核心出站重新进入 TUN。

### 原生配置校验

```powershell
& $CoreExe run -test -c $ConfigFile
if ($LASTEXITCODE -ne 0) {
    throw "Xray config validation failed"
}
```

---

# 2. 对照 Clash Verge Rev 与 v2rayN 的连接模型

NAVO 应复刻的是它们的“控制流程”，不是 UI。

## 2.1 Clash Verge Rev 模型

```text
生成配置
  → 判断 TUN 是否需要 Service/管理员权限
  → 启动 Mihomo
  → 等待核心 Ready
  → 应用当前用户系统代理
  → 启动代理守护
  → 运行
```

停止顺序：

```text
停止代理守护
  → 清除/恢复当前用户系统代理
  → 停止核心
  → 清理 TUN 和残留状态
```

关键点：

- 生命周期操作串行化。
- 核心未 Ready 前不应用系统代理。
- 停止核心前先清理系统代理。
- 服务模式与普通 Sidecar 模式有明确边界。
- 失败有回滚，不只修改 UI 开关。

## 2.2 v2rayN 模型

```text
构造 CoreConfigContext 快照
  → 生成对应内核配置
  → 停止旧核心
  → 清理旧 TUN
  → 启动新核心
  → 检查进程未退出
  → 等待本地 SOCKS/代理端口完成协议握手
  → 更新系统代理
```

关键点：

- 配置生成使用不可变上下文，避免切换期间配置被修改。
- 启动后检查本地代理协议，而不是只检查进程。
- Windows 使用 Job Object 管理子进程。
- 系统代理使用 WinINet per-connection 设置，并刷新系统代理状态。
- TUN 启动前处理旧设备和旧进程。

---

# 3. NAVO 必须采用的组件边界

```text
┌──────────────────────────────┐
│ NAVO UI / Wails              │
│ 只提交 Desired State         │
└──────────────┬───────────────┘
               │
┌──────────────▼───────────────┐
│ Runtime Orchestrator         │
│ 生命周期、事务、回滚、探测    │
└───────┬──────────────┬───────┘
        │              │
┌───────▼───────┐  ┌───▼──────────────────┐
│ User Agent    │  │ Privileged Service    │
│ 当前用户会话  │  │ LocalSystem           │
│ WinINet Proxy │  │ TUN/Route/DNS/Core    │
└───────────────┘  └───────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │ Core Adapter         │
                    │ sing/mihomo/xray     │
                    └──────────────────────┘
```

## 3.1 当前用户进程负责

- WinINet 系统代理读取、设置、刷新、恢复。
- PAC（如实现）。
- 用户会话状态。
- 系统托盘。
- 系统代理 Guard。
- 用户侧端到端探测。

## 3.2 Windows Service 负责

- 高权限核心启动。
- TUN/Wintun。
- 路由。
- DNS 接口设置。
- 网络修复。
- 核心进程监督。
- 崩溃清理。
- 与 UI/User Agent 的认证 IPC。

## 3.3 禁止事项

- 禁止 Windows Service 直接使用 WinINet 修改当前用户代理。
- 禁止 UI 在核心未 Ready 时显示“已连接”。
- 禁止系统代理指向未监听或只支持 SOCKS 的错误端口。
- 禁止 TUN 只创建网卡而不验证默认路由。
- 禁止三内核共享一份不经适配的配置。
- 禁止配置校验失败后继续启动核心。
- 禁止进程启动成功即判定网络成功。

---

# 4. 标准状态机

```text
Idle
  ↓
Preparing
  ↓
GeneratingConfig
  ↓
ValidatingConfig
  ↓
StoppingPreviousRuntime
  ↓
StartingCore
  ↓
WaitingForInbound
  ↓
ApplyingCapture
  ↓
ProbingEndToEnd
  ↓
Running
```

失败时必须逆序回滚：

```text
Failed
  → ClearSystemProxy
  → RemoveTunRoutes
  → RestoreDns
  → StopCore
  → RemoveStaleAdapterIfOwned
  → RestorePreviousNetworkSnapshot
  → Idle/Error
```

## 4.1 UI 可见状态

| 状态 | UI 文案 | 是否允许显示已连接 |
|---|---|---:|
| Preparing | 正在准备 | 否 |
| ValidatingConfig | 正在校验配置 | 否 |
| StartingCore | 正在启动内核 | 否 |
| WaitingForInbound | 正在等待本地代理 | 否 |
| ApplyingCapture | 正在接管系统流量 | 否 |
| ProbingEndToEnd | 正在验证网络 | 否 |
| Running | 已连接 | 是 |
| Failed | 连接失败 | 否 |

---

# 5. 测试环境准备

以管理员 PowerShell 执行 TUN 和服务测试。系统代理测试还必须在实际桌面用户会话中执行。

## 5.1 统一变量

```powershell
$NavoRoot      = "C:\Program Files\NAVO"
$RuntimeRoot   = "$env:LOCALAPPDATA\NAVO\runtime"
$LogRoot       = "$RuntimeRoot\logs"

$CoreType      = "sing-box"   # sing-box | mihomo | xray
$CoreExe       = "C:\path\to\core.exe"
$ConfigFile    = "C:\path\to\generated-config.json"
$CoreHome      = "C:\path\to\core-home"

$MixedPort     = 2080
$HttpPort      = 2081
$SocksPort     = 2082
$ControllerPort = 9090
$TunName       = "NAVO-TUN"

$ProbeUrl      = "https://www.cloudflare.com/cdn-cgi/trace"
$IpProbeUrl    = "https://api.ipify.org"
```

## 5.2 测试前关闭其他代理软件

```powershell
Get-Process |
    Where-Object {
        $_.ProcessName -match "clash|mihomo|sing-box|xray|v2ray|verge|nekoray"
    } |
    Select-Object ProcessName, Id, Path
```

关闭 Clash Verge、v2rayN、VPN、抓包代理和旧 NAVO 进程。

## 5.3 清除代理环境变量

```powershell
"HTTP_PROXY","HTTPS_PROXY","ALL_PROXY","NO_PROXY",
"http_proxy","https_proxy","all_proxy","no_proxy" |
ForEach-Object {
    Remove-Item "Env:$_" -ErrorAction SilentlyContinue
}
```

## 5.4 记录基线公网 IP

```powershell
$BaselineIp = (curl.exe --silent --show-error --noproxy "*" $IpProbeUrl).Trim()
"Baseline IP: $BaselineIp"
```

## 5.5 记录网络基线

```powershell
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

Get-Date | Out-File "$LogRoot\baseline.txt"
whoami | Out-File "$LogRoot\baseline.txt" -Append
Get-NetAdapter | Format-List * |
    Out-File "$LogRoot\net-adapter-before.txt"
Get-NetIPConfiguration | Format-List * |
    Out-File "$LogRoot\net-ip-before.txt"
Get-NetRoute -AddressFamily IPv4 |
    Sort-Object InterfaceIndex, DestinationPrefix, RouteMetric |
    Format-Table -AutoSize |
    Out-File "$LogRoot\routes-v4-before.txt"
Get-NetRoute -AddressFamily IPv6 |
    Sort-Object InterfaceIndex, DestinationPrefix, RouteMetric |
    Format-Table -AutoSize |
    Out-File "$LogRoot\routes-v6-before.txt"
Get-DnsClientServerAddress |
    Format-List * |
    Out-File "$LogRoot\dns-before.txt"
```

### 硬闸门 E0

- [ ] 直连网络正常。
- [ ] 基线公网 IP 已记录。
- [ ] 其他代理和 VPN 已停止。
- [ ] 测试机器时间正确。
- [ ] 当前用户与管理员会话身份已记录。

---

# 6. 第一层：二进制与能力检查

## 6.1 记录内核版本和文件 Hash

```powershell
& $CoreExe version 2>&1 | Tee-Object "$LogRoot\$CoreType-version.txt"
Get-FileHash $CoreExe -Algorithm SHA256 |
    Format-List |
    Out-File "$LogRoot\$CoreType-hash.txt"
```

如果 `version` 不支持，执行：

```powershell
& $CoreExe --version 2>&1 |
    Tee-Object "$LogRoot\$CoreType-version.txt"
```

## 6.2 检查架构

```powershell
Get-CimInstance Win32_OperatingSystem |
    Select-Object OSArchitecture, Version, BuildNumber
```

确认：

- Windows x64 使用 x64 内核。
- `wintun.dll` 与内核架构匹配。
- 不允许 x86 内核加载 x64 DLL，或反向组合。
- 内核目录可读。
- 运行目录可写。
- 配置和日志目录不存在权限拒绝。

## 6.3 能力声明

每个 Core Adapter 必须返回机器可读能力：

```json
{
  "core": "sing-box",
  "version": "x.y.z",
  "supports": {
    "systemProxy": true,
    "tun": true,
    "httpInbound": true,
    "socksInbound": true,
    "mixedInbound": true,
    "udp": true,
    "ipv6": true
  },
  "supportedOutboundProtocols": [
    "http",
    "socks",
    "vmess",
    "vless",
    "trojan"
  ]
}
```

不支持的组合必须在生成配置之前失败：

```text
UnsupportedCoreProtocol
UnsupportedCaptureMode
UnsupportedCoreVersion
MissingWintun
```

### 硬闸门 E1

- [ ] 二进制可执行。
- [ ] 精确版本已记录。
- [ ] SHA256 已记录。
- [ ] 架构匹配。
- [ ] Core Adapter 能力声明与实际版本一致。
- [ ] 不支持的协议不会被静默转换。

---

# 7. 第二层：生成配置检查

## 7.1 每次运行必须保存的文件

```text
runtime/
  session-<correlation-id>/
    desired-state.json
    normalized-profile.json
    generated-config.json|yaml
    generated-config.sha256
    validation.stdout.log
    validation.stderr.log
    core.stdout.log
    core.stderr.log
    network-before.json
    network-after.json
    probe-result.json
```

## 7.2 统一归一化模型

机场节点和独享代理必须先转成内部模型，再交给内核适配器：

```json
{
  "captureMode": "system_proxy",
  "core": "sing-box",
  "sourceType": "subscription",
  "outbound": {
    "protocol": "vless",
    "server": "example.com",
    "serverPort": 443,
    "credentials": {},
    "transport": {},
    "tls": {}
  }
}
```

独享 HTTP/SOCKS5：

```json
{
  "captureMode": "tun",
  "core": "mihomo",
  "sourceType": "direct_proxy",
  "outbound": {
    "protocol": "socks5",
    "server": "proxy.example.com",
    "serverPort": 1080,
    "username": "***",
    "password": "***"
  }
}
```

日志中必须脱敏密码、UUID、token、订阅地址和证书私钥。

## 7.3 通用配置断言

- [ ] 只有一个最终默认代理出口。
- [ ] `DIRECT` 与代理出口 tag 不重名。
- [ ] 入站端口不冲突。
- [ ] 入站只监听 `127.0.0.1`，除非显式开启 LAN。
- [ ] DNS 规则不会把 DNS 请求无限转发回自己。
- [ ] 代理服务器域名能够在 TUN 接管前解析。
- [ ] 代理服务器 IP 能够绕过 TUN。
- [ ] 系统代理与 TUN 不会同时接管，除非明确设计双开。
- [ ] 配置中的端口与 UI、Runtime、WinINet 设置完全一致。
- [ ] 生成配置 Hash 与实际启动时使用的文件一致。

---

# 8. 第三层：原生配置校验

## 8.1 sing-box

```powershell
& $CoreExe check -c $ConfigFile 1>"$LogRoot\validation.stdout.log" 2>"$LogRoot\validation.stderr.log"
$ValidationExit = $LASTEXITCODE
```

## 8.2 Mihomo

```powershell
& $CoreExe -t -f $ConfigFile -d $CoreHome 1>"$LogRoot\validation.stdout.log" 2>"$LogRoot\validation.stderr.log"
$ValidationExit = $LASTEXITCODE
```

## 8.3 Xray

```powershell
& $CoreExe run -test -c $ConfigFile 1>"$LogRoot\validation.stdout.log" 2>"$LogRoot\validation.stderr.log"
$ValidationExit = $LASTEXITCODE
```

## 8.4 判定

```powershell
if ($ValidationExit -ne 0) {
    Get-Content "$LogRoot\validation.stderr.log"
    throw "Hard gate failed: native config validation"
}
```

### 硬闸门 E2

- [ ] 原生校验退出码为 0。
- [ ] stderr 无 fatal/error。
- [ ] 校验失败时 NAVO 不启动核心。
- [ ] 错误信息原样传递到 UI 的“详细错误”中。
- [ ] 校验使用的配置与正式启动使用的配置为同一文件、同一 Hash。

---

# 9. 第四层：脱离 NAVO 手动启动核心

这一步用于区分：

```text
内核/配置问题
```

与：

```text
NAVO 服务、IPC、进程管理问题
```

## 9.1 sing-box

```powershell
$CoreProcess = Start-Process `
    -FilePath $CoreExe `
    -ArgumentList @("run", "-c", $ConfigFile) `
    -WorkingDirectory (Split-Path $CoreExe) `
    -RedirectStandardOutput "$LogRoot\core.stdout.log" `
    -RedirectStandardError "$LogRoot\core.stderr.log" `
    -PassThru
```

## 9.2 Mihomo

```powershell
$CoreProcess = Start-Process `
    -FilePath $CoreExe `
    -ArgumentList @("-f", $ConfigFile, "-d", $CoreHome) `
    -WorkingDirectory $CoreHome `
    -RedirectStandardOutput "$LogRoot\core.stdout.log" `
    -RedirectStandardError "$LogRoot\core.stderr.log" `
    -PassThru
```

## 9.3 Xray

```powershell
$CoreProcess = Start-Process `
    -FilePath $CoreExe `
    -ArgumentList @("run", "-c", $ConfigFile) `
    -WorkingDirectory (Split-Path $CoreExe) `
    -RedirectStandardOutput "$LogRoot\core.stdout.log" `
    -RedirectStandardError "$LogRoot\core.stderr.log" `
    -PassThru
```

## 9.4 进程稳定性检查

```powershell
Start-Sleep -Milliseconds 500

$CoreProcess.Refresh()
if ($CoreProcess.HasExited) {
    Get-Content "$LogRoot\core.stderr.log"
    throw "Core exited immediately. ExitCode=$($CoreProcess.ExitCode)"
}
```

NAVO 正式实现中不能固定睡眠后直接判定成功，必须使用超时轮询：

```text
最多等待 10 秒
  → 每 100 ms 检查进程是否退出
  → 检查目标入站是否开始监听
  → 完成协议握手
```

### 硬闸门 E3

- [ ] 核心手动启动后至少稳定运行 10 秒。
- [ ] stdout/stderr 可持续采集。
- [ ] 启动失败能获得退出码。
- [ ] 不存在第二个旧核心占用同一端口。
- [ ] 核心工作目录正确。

---

# 10. 第五层：本地端口与协议握手

## 10.1 查看监听端口

```powershell
$Ports = @($MixedPort, $HttpPort, $SocksPort, $ControllerPort)

Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
    Where-Object { $_.LocalPort -in $Ports } |
    Select-Object LocalAddress, LocalPort, OwningProcess,
        @{Name="ProcessName";Expression={(Get-Process -Id $_.OwningProcess).ProcessName}}
```

## 10.2 检查端口归属

```powershell
function Assert-PortOwner {
    param(
        [int]$Port,
        [int]$ExpectedPid
    )

    $listeners = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue

    if (-not $listeners) {
        throw "Port $Port is not listening"
    }

    if ($listeners.OwningProcess -notcontains $ExpectedPid) {
        throw "Port $Port is owned by another process"
    }
}
```

## 10.3 SOCKS5 真实握手

```powershell
function Test-Socks5Handshake {
    param(
        [string]$Host = "127.0.0.1",
        [int]$Port
    )

    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $client.Connect($Host, $Port)
        $stream = $client.GetStream()
        $stream.ReadTimeout = 2000
        $stream.WriteTimeout = 2000

        [byte[]]$greeting = 0x05, 0x01, 0x00
        $stream.Write($greeting, 0, $greeting.Length)

        [byte[]]$reply = New-Object byte[] 2
        $read = $stream.Read($reply, 0, 2)

        return $read -eq 2 -and $reply[0] -eq 0x05 -and $reply[1] -ne 0xFF
    }
    finally {
        $client.Dispose()
    }
}
```

执行：

```powershell
Test-Socks5Handshake -Port $SocksPort
```

对于 sing-box/Mihomo mixed-port，也可对 mixed-port 执行 SOCKS5 握手。

## 10.4 HTTP 代理握手

```powershell
curl.exe `
    --silent `
    --show-error `
    --fail `
    --proxy "http://127.0.0.1:$HttpPort" `
    $ProbeUrl
```

对于 mixed-port：

```powershell
curl.exe `
    --silent `
    --show-error `
    --fail `
    --proxy "http://127.0.0.1:$MixedPort" `
    $ProbeUrl
```

### 硬闸门 E4

- [ ] 端口真实监听。
- [ ] 端口属于本次启动的核心 PID。
- [ ] SOCKS5 greeting 返回合法响应。
- [ ] HTTP CONNECT/HTTP proxy 请求成功。
- [ ] 不允许仅用 `Test-NetConnection` 的 TCP 成功代替协议握手。

---

# 11. 第六层：显式代理端到端测试

这一步尚未启用系统代理或 TUN，只验证：

```text
应用 → 本地代理入口 → 核心 → 出口节点 → 公网
```

## 11.1 mixed/HTTP 入站

```powershell
$ExplicitProxyIp = (
    curl.exe `
        --silent `
        --show-error `
        --fail `
        --proxy "http://127.0.0.1:$MixedPort" `
        $IpProbeUrl
).Trim()

"Explicit proxy IP: $ExplicitProxyIp"
```

Xray HTTP 入站：

```powershell
$ExplicitProxyIp = (
    curl.exe `
        --silent `
        --show-error `
        --fail `
        --proxy "http://127.0.0.1:$HttpPort" `
        $IpProbeUrl
).Trim()
```

## 11.2 SOCKS 入站

```powershell
$ExplicitSocksIp = (
    curl.exe `
        --silent `
        --show-error `
        --fail `
        --proxy "socks5h://127.0.0.1:$SocksPort" `
        $IpProbeUrl
).Trim()
```

`Socks5h` 的域名解析通过 SOCKS 代理完成，可区分本地 DNS 与代理侧 DNS 问题。

## 11.3 判定

```powershell
if (-not $ExplicitProxyIp) {
    throw "Explicit proxy returned empty IP"
}

if ($ExplicitProxyIp -eq $BaselineIp) {
    throw "Traffic did not use proxy outbound"
}
```

> 某些独享代理出口可能与当前网络出口相同或被上游 NAT 处理。此时不要只依赖 IP 不同，还要检查核心日志中实际命中的 outbound、连接计数和远端连接。

### 硬闸门 E5

- [ ] 显式 HTTP/Mixed 代理可访问公网。
- [ ] 显式 SOCKS5 可访问公网。
- [ ] 出口 IP 或核心连接日志证明流量经过目标 outbound。
- [ ] 机场节点与独享代理分别测试。
- [ ] 如果本关失败，禁止继续排查系统代理或 TUN；故障必定仍在核心配置、出口或本地入口层。

---

# 12. 第七层：机场与独享代理来源测试

## 12.1 机场订阅

第一轮禁止使用自动选择、负载均衡、fallback 或复杂规则。

必须固定：

- 一个已知可用节点。
- 一个内核明确支持的协议。
- `GLOBAL` 或默认路由只指向该节点。
- DNS 使用最小可运行配置。
- 不加载远程 rule-set。
- 不加载远程 UI。
- 不自动更新 GEO 数据。

建议先选三个内核共同支持的节点协议做横向测试，再测试内核专属协议。

## 12.2 独享 HTTP 代理

先绕过 NAVO，直接测试上游：

```powershell
$ProxyUri = "http://USERNAME:PASSWORD@HOST:PORT"

curl.exe `
    --silent `
    --show-error `
    --fail `
    --proxy $ProxyUri `
    $IpProbeUrl
```

不要把真实密码写入长期日志或提交到 Git。

## 12.3 独享 SOCKS5 代理

```powershell
$ProxyUri = "socks5h://USERNAME:PASSWORD@HOST:PORT"

curl.exe `
    --silent `
    --show-error `
    --fail `
    --proxy $ProxyUri `
    $IpProbeUrl
```

## 12.4 出口解析要求

| 来源 | 内部协议类型 | 必须验证 |
|---|---|---|
| 机场订阅 | VMess/VLESS/Trojan/Hysteria2 等 | 字段完整、TLS/SNI/transport 正确 |
| HTTP 代理 | HTTP CONNECT | 用户名、密码、Host、Port |
| SOCKS5 代理 | SOCKS5 | 认证、远程 DNS、UDP 能力 |
| 无认证代理 | HTTP/SOCKS5 | 禁止生成空字符串凭证导致错误认证 |

### 硬闸门 E6

- [ ] 上游代理脱离 NAVO 可用。
- [ ] NAVO 归一化模型字段完整。
- [ ] 三个内核生成的配置语义一致。
- [ ] 不支持的协议会明确失败。
- [ ] 机场与独享代理分别通过显式代理测试。

---

# 13. 第八层：NAVO 托管核心测试

在系统代理和 TUN 都关闭的情况下，通过 NAVO 启动相同配置。

## 13.1 检查服务

```powershell
Get-CimInstance Win32_Service |
    Where-Object { $_.Name -match "NAVO" } |
    Select-Object Name, State, StartMode, StartName, ProcessId
```

记录：

- Service Name
- 运行账户
- Service PID
- Core PID
- Session ID
- UI PID
- User Agent PID

## 13.2 检查会话

```powershell
Get-Process |
    Where-Object { $_.ProcessName -match "navo|sing-box|mihomo|xray" } |
    Select-Object ProcessName, Id, SessionId, Path
```

预期：

- UI/User Agent 位于当前桌面 Session。
- Service 通常位于 Session 0。
- 核心位置取决于 NAVO 设计，但必须与权限模型一致。
- 系统代理操作必须发生在用户会话。

## 13.3 IPC 返回值要求

禁止仅返回：

```json
{
  "success": true
}
```

必须返回：

```json
{
  "correlationId": "uuid",
  "phase": "Running",
  "corePid": 1234,
  "configHash": "sha256",
  "captureMode": "system_proxy",
  "inbounds": [
    {
      "protocol": "mixed",
      "address": "127.0.0.1",
      "port": 2080,
      "ready": true
    }
  ],
  "probe": {
    "success": true,
    "egressIp": "x.x.x.x"
  }
}
```

### 硬闸门 E7

- [ ] NAVO 托管启动与手动启动使用相同配置。
- [ ] NAVO 托管后本地显式代理仍通过。
- [ ] UI 能显示核心 PID、内核版本、配置 Hash。
- [ ] Core stdout/stderr 未丢失。
- [ ] Service 命令成功不等于 Runtime 成功。
- [ ] IPC 能返回准确失败阶段。

---

# 14. 第九层：Windows 系统代理设置

## 14.1 强制架构原则

WinINet 不应从 Windows Service 上下文调用。

推荐：

```text
UI/User Agent
  → 保存当前用户代理快照
  → 设置 WinINet per-connection options
  → INTERNET_OPTION_SETTINGS_CHANGED
  → INTERNET_OPTION_REFRESH
  → 查询并确认实际值
```

Service 只负责核心和 TUN。

## 14.2 读取当前用户代理注册表

```powershell
$InternetSettings = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings"

Get-ItemProperty $InternetSettings |
    Select-Object ProxyEnable, ProxyServer, ProxyOverride, AutoConfigURL
```

### sing-box/Mihomo 预期

```text
ProxyEnable = 1
ProxyServer = 127.0.0.1:2080
AutoConfigURL = 空
```

### Xray 预期

```text
ProxyEnable = 1
ProxyServer = 127.0.0.1:2081
AutoConfigURL = 空
```

## 14.3 注意 WinHTTP 与 WinINet 分离

```powershell
netsh winhttp show proxy
```

`Direct access` 不代表 WinINet 系统代理失败。

NAVO 本阶段验收的是当前用户 WinINet 代理，不是 WinHTTP 服务代理。

## 14.4 默认系统代理解析检查

```powershell
$target = [Uri]$ProbeUrl
$systemProxy = [System.Net.WebRequest]::DefaultWebProxy
$resolvedProxy = $systemProxy.GetProxy($target)

"System default proxy for $target => $resolvedProxy"
```

预期返回 NAVO 本地代理地址，而不是目标网站自身地址。

## 14.5 使用系统默认代理发起请求

```powershell
$handler = [System.Net.Http.HttpClientHandler]::new()
$handler.UseProxy = $true
$handler.Proxy = [System.Net.WebRequest]::DefaultWebProxy

$client = [System.Net.Http.HttpClient]::new($handler)
$client.Timeout = [TimeSpan]::FromSeconds(15)

try {
    $SystemProxyResult = $client.GetStringAsync($IpProbeUrl).GetAwaiter().GetResult().Trim()
    "System proxy IP: $SystemProxyResult"
}
finally {
    $client.Dispose()
    $handler.Dispose()
}
```

## 14.6 浏览器验证

- Edge 新建 InPrivate 窗口。
- Chrome 新建无痕窗口。
- 访问 IP 检测页面。
- 确认出口与显式代理测试一致。
- 确认关闭 NAVO 后浏览器恢复直连。

### 硬闸门 S1

- [ ] 系统代理由当前用户进程设置。
- [ ] ProxyServer 指向真实 HTTP/Mixed 入站。
- [ ] 当前用户查询结果与 NAVO Desired State 一致。
- [ ] WinINet 已收到 refresh 通知。
- [ ] .NET 使用系统默认代理请求成功。
- [ ] Edge/Chrome 成功访问。
- [ ] 出口符合目标代理。
- [ ] 系统代理关闭后原代理设置被恢复，而不是无条件清空。

---

# 15. 系统代理 Guard、停止与崩溃恢复

## 15.1 启动前快照

必须保存：

```json
{
  "proxyEnable": 0,
  "proxyServer": "",
  "proxyOverride": "",
  "autoConfigUrl": "",
  "capturedAt": "timestamp",
  "userSid": "S-1-..."
}
```

## 15.2 正常停止顺序

```text
阻止新切换请求
  → 停止 Proxy Guard
  → 恢复原始 WinINet 设置
  → 刷新 WinINet
  → 验证恢复结果
  → 停止核心
```

## 15.3 核心崩溃测试

```powershell
Stop-Process -Id $CorePid -Force
```

预期：

- 3～5 秒内 NAVO 检测到核心退出。
- 系统代理自动恢复。
- UI 进入 `Failed`，不得继续显示已连接。
- 浏览器恢复直连。
- 记录 `CoreExitedUnexpectedly` 和退出码。

## 15.4 User Agent 崩溃测试

- 结束 User Agent。
- Service 不应继续假装系统代理可用。
- User Agent 重启后应读取实际 WinINet 状态。
- 如果本地端口已不存在，必须恢复代理快照。

## 15.5 防止覆盖用户手动修改

恢复前检查当前值是否仍由 NAVO 持有：

```text
当前 ProxyServer == NAVO 设置值
  → 可以恢复快照

当前 ProxyServer 已被用户或其他软件修改
  → 不直接覆盖
  → 提示代理状态发生外部变更
```

### 硬闸门 S2

- [ ] 核心崩溃后不会留下失效系统代理。
- [ ] NAVO 正常退出后恢复原设置。
- [ ] 电脑注销/登录后状态正确。
- [ ] 用户手动修改代理不会被错误覆盖。
- [ ] 快速开关 10 次无竞态和残留。

---

# 16. 第十层：TUN 网卡基础检查

> 已知 NAVO 当前能够生成 TUN 网卡。网卡存在仅代表本节第一项成功，不代表 TUN 可用。

## 16.1 查看 TUN 网卡

```powershell
Get-NetAdapter -IncludeHidden |
    Where-Object {
        $_.Name -match "NAVO|Wintun|sing|mihomo|xray" -or
        $_.InterfaceDescription -match "NAVO|Wintun|sing|mihomo|xray"
    } |
    Format-List Name, InterfaceDescription, ifIndex, Status, MacAddress, LinkSpeed
```

## 16.2 检查地址

```powershell
Get-NetIPAddress |
    Where-Object {
        $_.InterfaceAlias -match "NAVO|Wintun|sing|mihomo|xray"
    } |
    Format-Table InterfaceAlias, AddressFamily, IPAddress, PrefixLength
```

## 16.3 检查接口 Metric

```powershell
Get-NetIPInterface |
    Where-Object {
        $_.InterfaceAlias -match "NAVO|Wintun|sing|mihomo|xray"
    } |
    Format-Table InterfaceAlias, AddressFamily, InterfaceMetric, ConnectionState, Dhcp
```

### 硬闸门 T1

- [ ] 网卡存在。
- [ ] 网卡状态 Up。
- [ ] 网卡地址与配置一致。
- [ ] MTU 与配置一致。
- [ ] 只有本次 Runtime 拥有的网卡被操作。
- [ ] 不存在多个同名或陈旧 NAVO TUN 网卡。
- [ ] 核心退出后网卡按设计删除或进入可识别的清理状态。

---

# 17. 第十一层：TUN 路由检查

## 17.1 查看关键 IPv4 路由

```powershell
Get-NetRoute -AddressFamily IPv4 |
    Where-Object {
        $_.DestinationPrefix -in @(
            "0.0.0.0/0",
            "0.0.0.0/1",
            "128.0.0.0/1"
        )
    } |
    Sort-Object DestinationPrefix, RouteMetric |
    Format-Table DestinationPrefix, NextHop, InterfaceAlias,
        InterfaceIndex, RouteMetric, Protocol, State
```

常见正确实现有两类：

### 方案 A：默认路由指向 TUN

```text
0.0.0.0/0 → NAVO-TUN
```

### 方案 B：使用两个 /1 覆盖默认路由

```text
0.0.0.0/1   → NAVO-TUN
128.0.0.0/1 → NAVO-TUN
```

两种方案均可，但必须稳定并能恢复。

## 17.2 查看代理服务器实际出口路由

先解析代理服务器：

```powershell
$ProxyHost = "proxy.example.com"
$ProxyPort = 443

$ProxyIps = Resolve-DnsName $ProxyHost -Type A |
    Where-Object IPAddress |
    Select-Object -ExpandProperty IPAddress -Unique

$ProxyIps
```

检查每个代理 IP：

```powershell
foreach ($ProxyIp in $ProxyIps) {
    Find-NetRoute -RemoteIPAddress $ProxyIp |
        Format-List IPAddress, InterfaceAlias, InterfaceIndex,
            NextHop, RouteMetric
}
```

预期：

- 代理服务器 IP 走物理 Wi-Fi/Ethernet。
- 不得走 NAVO-TUN。
- SourceAddress 是物理网卡地址。
- 如果代理域名解析出多个 IP，全部处理或采用稳定的动态更新策略。

## 17.3 检查核心连接实际接口

```powershell
Test-NetConnection `
    -ComputerName $ProxyHost `
    -Port $ProxyPort `
    -InformationLevel Detailed
```

查看：

- `InterfaceAlias`
- `SourceAddress`
- `TcpTestSucceeded`

## 17.4 路由回环判定

如果出现以下现象，优先判定为回环：

- TUN 网卡存在但所有请求超时。
- 核心日志持续重复连接代理服务器。
- 代理服务器连接数快速增加。
- 路由查询显示代理服务器 IP 走 TUN。
- 停止 TUN 后节点立即恢复。

### 修复原则

#### sing-box

至少使用：

```json
{
  "route": {
    "auto_detect_interface": true
  }
}
```

或显式为 outbound 绑定物理接口。

#### Mihomo

至少使用：

```yaml
tun:
  auto-detect-interface: true
```

必要时显式指定：

```yaml
interface-name: "Wi-Fi"
```

#### Xray

优先：

```json
{
  "autoOutboundsInterface": "auto"
}
```

或给 outbound `sockopt.interface` 绑定物理接口。

### 硬闸门 T2

- [ ] 默认流量进入 TUN。
- [ ] 代理服务器 IP 明确走物理接口。
- [ ] 核心自身出站不进入 TUN。
- [ ] 局域网网关、DHCP、必要本地网段未被错误劫持。
- [ ] 路由停止后完全恢复。
- [ ] Wi-Fi 切换后代理服务器绕行路由会重新计算。

---

# 18. 第十二层：TUN DNS 检查

## 18.1 查看接口 DNS

```powershell
Get-DnsClientServerAddress |
    Where-Object {
        $_.InterfaceAlias -match "NAVO|Wintun|sing|mihomo|xray"
    } |
    Format-List InterfaceAlias, AddressFamily, ServerAddresses
```

## 18.2 普通解析

```powershell
Resolve-DnsName example.com -Type A
```

## 18.3 指定公共 DNS 测试

```powershell
Resolve-DnsName example.com -Server 1.1.1.1 -Type A
Resolve-DnsName example.com -Server 8.8.8.8 -Type A
```

启用 DNS hijack 时，即使应用指定公共 DNS，也应按设计被 TUN/DNS 模块处理。

## 18.4 域名与 IP 分离测试

```powershell
curl.exe --silent --show-error --fail --noproxy "*" https://api.ipify.org
Resolve-DnsName www.cloudflare.com
```

判定：

| IP 请求 | 域名请求 | 可能问题 |
|---|---|---|
| 成功 | 失败 | DNS 配置或 DNS 劫持 |
| 失败 | 成功 | 路由、出口、TLS 或核心数据面 |
| 都失败 | — | TUN 路由或核心回环 |
| 都成功 | — | 继续 UDP/IPv6/恢复测试 |

## 18.5 三内核重点

### sing-box

- 根据实际版本决定是否使用 `dns_mode`/`dns_address`。
- `strict_route` 在 Windows 可用于减少多宿主 DNS 泄漏。
- 如果显式设置 `dns_address`，确认是否还需要显式 `hijack-dns` 规则。
- DNS outbound 不得再次进入自身 DNS。

### Mihomo

至少验证：

```yaml
dns:
  enable: true
  listen: 0.0.0.0:1053
  enhanced-mode: fake-ip
```

以及：

```yaml
tun:
  dns-hijack:
    - any:53
```

如果使用 `redir-host`，需单独回归。

### Xray

- 当前版本可在 TUN settings 中设置 Windows 接口 DNS。
- 旧版本需要 NAVO 手动设置接口 DNS。
- 内置 DNS 的出站同样必须绕过 TUN 回环。
- 不要把公共 DNS 设置为 TUN 内不可达地址。

### 硬闸门 T3

- [ ] 域名解析成功。
- [ ] 指定公共 DNS 的请求按设计处理。
- [ ] 核心日志可确认 DNS 请求路径。
- [ ] 没有 DNS 请求循环。
- [ ] 停止 TUN 后 DNS 恢复。
- [ ] 物理接口 DNS 未被永久覆盖。
- [ ] 浏览器没有明显 DNS 泄漏。

---

# 19. 第十三层：TUN 端到端测试

此时必须确保：

- 所有代理环境变量已清除。
- Windows 系统代理已关闭。
- 只启用 TUN。
- `curl.exe` 使用 `--noproxy "*"`。

## 19.1 TCP/HTTPS

```powershell
$TunIp = (
    curl.exe `
        --silent `
        --show-error `
        --fail `
        --max-time 15 `
        --noproxy "*" `
        $IpProbeUrl
).Trim()

"TUN IP: $TunIp"
```

## 19.2 详细连接

```powershell
curl.exe `
    --verbose `
    --max-time 15 `
    --noproxy "*" `
    $ProbeUrl 2>&1 |
    Tee-Object "$LogRoot\tun-curl-verbose.log"
```

## 19.3 UDP/DNS

```powershell
Resolve-DnsName example.com -Server 1.1.1.1 -Type A
```

同时查看核心日志，确认 UDP/DNS 流量命中 TUN 入站和目标 outbound。

## 19.4 浏览器和非系统代理应用

至少测试：

- Edge/Chrome，系统代理关闭。
- 一个明确不依赖 WinINet 代理的应用。
- 一个 UDP 场景。
- 大文件下载。
- 长连接或视频播放。
- Git 或包管理器。
- 目标游戏/业务应用。

## 19.5 IPv6

仅在基线网络具备 IPv6 时执行：

```powershell
curl.exe -6 --silent --show-error --fail --noproxy "*" https://api64.ipify.org
Resolve-DnsName example.com -Type AAAA
```

如果 NAVO 暂不支持 IPv6，应明确：

- 禁用 IPv6 TUN 路由；
- 或让 IPv6 明确直连；
- 或阻断并提示；
- 不允许无声明泄漏。

## 19.6 ICMP 注意

`ping` 不能作为唯一 TUN 成功标准。

部分用户态 TUN 栈可能本地响应 ICMP，成功的 ping 不一定证明真实远端可达。

### 硬闸门 T4

- [ ] 在无系统代理、无代理环境变量情况下，HTTPS 通过 TUN 成功。
- [ ] 出口符合代理。
- [ ] DNS 成功。
- [ ] UDP 场景成功。
- [ ] 浏览器和非系统代理应用成功。
- [ ] IPv6 行为符合产品声明。
- [ ] 核心日志证明流量由 TUN 入站接收。
- [ ] TUN 关闭后立即恢复直连。

---

# 20. 第十四层：生命周期与网络恢复

## 20.1 正常关闭

预期顺序：

```text
停止接收新命令
  → 停止健康检查
  → 清除 TUN 路由
  → 恢复 DNS
  → 停止核心
  → 删除或释放 TUN
  → 验证直连
```

## 20.2 强杀核心

```powershell
Stop-Process -Id $CorePid -Force
```

预期：

- NAVO 发现退出。
- 路由和 DNS 自动恢复。
- 直连在规定时间内恢复。
- UI 进入 Failed。
- 不遗留无法访问网络的 TUN 默认路由。

## 20.3 强杀 Service

```powershell
Stop-Service -Name "NAVOService" -Force
```

预期：

- Service SCM recovery 或 User Agent 网络修复生效。
- 不留下黑洞路由。
- 不留下失效系统代理。
- 下次启动执行残留扫描和修复。

## 20.4 快速切换

连续执行 10 次：

```text
System Proxy ON
System Proxy OFF
TUN ON
TUN OFF
切换节点
切换内核
```

要求：

- 所有操作串行化。
- 新操作取消旧操作时有 generation/correlation ID。
- 旧操作不得在新状态完成后重新写回系统设置。
- 不出现两个核心。
- 不出现多个 TUN。
- 不出现端口冲突。

## 20.5 网络切换

依次测试：

- Wi-Fi A → Wi-Fi B
- Wi-Fi → 有线
- 有线 → Wi-Fi
- 断网 → 恢复
- 睡眠 → 唤醒
- DHCP 地址变化
- 默认网关变化

要求：

- 重新检测物理出口。
- 重新解析代理域名。
- 更新代理服务器绕行路由。
- 旧连接可失败，但新连接必须恢复。
- 不需要用户手动重启电脑。

## 20.6 重启测试

分别测试：

1. NAVO 关闭状态重启 Windows。
2. 系统代理开启状态重启 Windows。
3. TUN 开启状态重启 Windows。
4. 核心运行中强制关机后启动。

预期：

- 启动时先做残留扫描。
- 用户未启用“开机连接”时，系统保持直连。
- 启用“开机连接”时，只有端到端探测通过后才显示已连接。
- 无旧路由、旧 DNS、旧代理、旧 TUN 黑洞。

### 硬闸门 R1

- [ ] 正常停止恢复网络。
- [ ] Core 崩溃恢复网络。
- [ ] Service 崩溃恢复网络。
- [ ] 强制关机后下次启动可修复。
- [ ] 网络切换可自动恢复。
- [ ] 睡眠唤醒可恢复。
- [ ] 快速开关无竞态。
- [ ] 同时只存在一个 Active Runtime。

---

# 21. 防火墙、权限与安全软件测试

## 21.1 Windows Firewall

```powershell
Get-NetFirewallProfile |
    Select-Object Name, Enabled, DefaultInboundAction, DefaultOutboundAction
```

检查内核规则：

```powershell
Get-NetFirewallApplicationFilter |
    Where-Object {
        $_.Program -match "sing-box|mihomo|xray|navo"
    } |
    Format-List *
```

Mihomo 的 `system/mixed` TUN stack 可能受防火墙放行影响。

## 21.2 管理员权限

测试两种启动：

- UI 普通用户启动。
- Service 已安装并运行。
- UI 不提升权限也能通过 IPC 开启 TUN。
- Service 未安装时，TUN 明确提示 `ServiceRequired`。
- 不允许静默回退为“生成网卡但不接管流量”。

## 21.3 IPC 安全

必须验证：

- Named Pipe ACL 只允许当前用户、管理员和 Service。
- 命令包含 session owner/correlation proof。
- 非当前用户不能控制其他用户代理设置。
- 路径参数做规范化，禁止任意文件执行。
- 配置文件权限不泄露凭证。

### 硬闸门 SEC1

- [ ] 普通 UI + 高权限 Service 模式可用。
- [ ] Service 未就绪时明确失败。
- [ ] 防火墙开启时系统代理和 TUN 均可用。
- [ ] Named Pipe 不允许未授权用户控制。
- [ ] 配置和日志凭证已脱敏。

---

# 22. 三内核 × 两来源 × 两模式回归矩阵

每个单元格都必须完成：

```text
配置原生校验
→ 核心启动
→ 本地协议握手
→ 显式代理端到端
→ Capture Mode 端到端
→ 停止恢复
```

| 内核 | 来源 | 系统代理 | TUN | 结果 |
|---|---|---:|---:|---|
| sing-box | 机场固定节点 | 必测 | 必测 | ☐ |
| sing-box | 独享 HTTP 代理 | 必测 | 必测 | ☐ |
| sing-box | 独享 SOCKS5 代理 | 必测 | 必测 | ☐ |
| Mihomo | 机场固定节点 | 必测 | 必测 | ☐ |
| Mihomo | 独享 HTTP 代理 | 必测 | 必测 | ☐ |
| Mihomo | 独享 SOCKS5 代理 | 必测 | 必测 | ☐ |
| Xray | 机场固定节点 | 必测 | 必测 | ☐ |
| Xray | 独享 HTTP 代理 | 必测 | 必测 | ☐ |
| Xray | 独享 SOCKS5 代理 | 必测 | 必测 | ☐ |

如果特定版本或协议不支持某组合，结果必须是：

```text
SKIPPED_UNSUPPORTED
```

并附：

- 内核版本。
- 不支持原因。
- 能力声明。
- UI 提示。

不允许表现为启动超时或无限重试。

---

# 23. 故障定位决策树

## 23.1 配置校验失败

```text
E2 失败
  → Core Adapter 生成错误
  → 字段版本不匹配
  → 订阅转换错误
  → 路径/权限错误
```

不要排查系统代理或 TUN。

## 23.2 核心启动后立即退出

```text
E3 失败
  → 查看 stderr 和退出码
  → 端口冲突
  → Wintun 缺失
  → 资源文件缺失
  → 工作目录错误
  → 配置运行时错误
```

## 23.3 端口监听但协议握手失败

```text
E4 失败
  → 端口指向错误协议
  → HTTP 与 SOCKS 端口混用
  → mixed 入站未生成
  → 旧进程占用端口
  → 核心尚未 Ready
```

## 23.4 显式代理失败

```text
E5 失败
  → 出口节点错误
  → TLS/SNI/transport 错误
  → 上游代理认证错误
  → DNS 错误
  → 代理协议不受当前内核支持
```

此时系统代理和 TUN 一定不会成功。

## 23.5 显式代理成功，系统代理失败

```text
E5 成功 + S1 失败
  → WinINet 在 Service 中被调用
  → 修改了错误用户 HKCU
  → ProxyServer 端口错误
  → 指向纯 SOCKS 而应用需要 HTTP
  → 未调用 refresh
  → PAC 与全局代理互相覆盖
  → 系统代理启用早于核心 Ready
```

## 23.6 TUN 网卡存在，但无网络

```text
T1 成功 + T4 失败
  → 默认路由未进入 TUN
  → 代理服务器 IP 进入 TUN，发生回环
  → DNS 没有被接管
  → 核心出站未绑定物理接口
  → Firewall 阻断
  → Xray 版本只创建网卡，没有自动路由
```

## 23.7 TUN IP 可访问，域名失败

```text
路由成功
  → DNS 模块错误
  → DNS hijack 缺失
  → TUN 接口 DNS 错误
  → DNS 出站回环
  → strict route 与多网卡冲突
```

## 23.8 停止后断网

```text
恢复事务失败
  → 旧系统代理未恢复
  → 默认路由残留
  → TUN DNS 残留
  → 核心先退出，清理逻辑无法执行
  → 清理只在正常退出路径执行
```

必须增加启动时 Repair，而不是只依赖优雅退出。

---

# 24. NAVO 必须实现的诊断接口

## 24.1 Runtime 状态

```json
{
  "state": "Running",
  "correlationId": "uuid",
  "core": {
    "type": "sing-box",
    "version": "x.y.z",
    "pid": 1234,
    "startedAt": "timestamp",
    "configHash": "sha256"
  },
  "capture": {
    "mode": "tun",
    "applied": true
  },
  "inbounds": [],
  "tun": {
    "adapter": "NAVO-TUN",
    "ifIndex": 42,
    "routesApplied": true,
    "dnsApplied": true,
    "physicalInterface": "Wi-Fi"
  },
  "probe": {
    "success": true,
    "egressIp": "x.x.x.x",
    "latencyMs": 350
  }
}
```

## 24.2 一键诊断

```text
navo diagnose --full
```

至少输出：

- OS 版本。
- 用户 SID、Session ID。
- Service 状态。
- Core 版本、Hash、PID。
- 配置 Hash。
- 监听端口及 PID。
- WinINet 当前状态。
- WinHTTP 当前状态，仅作参考。
- TUN 网卡。
- IPv4/IPv6 路由。
- DNS。
- 物理默认接口。
- 代理服务器解析结果。
- 到代理服务器的实际路由。
- 显式代理探测。
- Capture Mode 探测。
- 最近 200 行核心日志。
- 最近一次错误阶段。
- 脱敏后的归一化配置。

---

# 25. 最终验收标准

## 25.1 系统代理合格

- [ ] 三个内核的原生配置均可校验。
- [ ] 三个内核均能通过本地 HTTP 能力入口访问公网。
- [ ] Windows 当前用户系统代理指向正确端口。
- [ ] Edge/Chrome 可用。
- [ ] 系统代理关闭后恢复原设置。
- [ ] 核心崩溃后不会留下断网代理。
- [ ] 机场与独享代理均可用。
- [ ] 切换内核不会残留旧端口和旧进程。

## 25.2 TUN 合格

- [ ] TUN 网卡创建成功。
- [ ] TUN 地址、MTU 正确。
- [ ] 默认流量进入 TUN。
- [ ] 代理服务器走物理接口。
- [ ] 核心出站不会回环。
- [ ] DNS 正常并符合设计。
- [ ] 无系统代理时浏览器和其他应用可用。
- [ ] UDP 场景可用。
- [ ] IPv6 行为明确。
- [ ] 停止、崩溃、重启、网络切换后均可恢复。
- [ ] 三内核按各自版本策略通过 TUN 测试。

## 25.3 发布阻断条件

出现任一项不得发布：

- UI 显示已连接但端到端探测失败。
- 核心退出后系统代理仍保持开启。
- TUN 关闭后默认路由或 DNS 残留。
- 代理服务器流量进入 TUN。
- Service 修改错误用户的 HKCU。
- 系统代理指向纯 SOCKS 端口且未做兼容验证。
- 配置未通过原生校验仍被启动。
- stderr 未采集。
- 端口被其他进程占用但 NAVO 仍显示运行。
- 不支持的内核/协议组合被静默忽略。
- 凭证出现在日志。

---

# 26. 推荐开发落地顺序

不要同时修三个内核和两种 Capture Mode。

## 第一阶段：sing-box 系统代理

```text
固定独享 HTTP/SOCKS5 出口
→ mixed inbound
→ 显式代理
→ 当前用户 WinINet
→ 失败回滚
```

## 第二阶段：sing-box TUN

```text
创建网卡
→ 默认路由
→ 代理服务器绕行
→ DNS
→ 端到端
→ 崩溃恢复
```

## 第三阶段：Mihomo

复用 Runtime Orchestrator，只替换：

- 配置适配器。
- 启动参数。
- Ready probe/API。
- TUN 配置字段。

## 第四阶段：Xray

复用 Runtime Orchestrator，只替换：

- HTTP/SOCKS 独立入站。
- 配置适配器。
- Xray 版本能力判断。
- Xray TUN 自动路由/手动路由策略。

## 第五阶段：完整回归矩阵

执行第 22 节所有支持的组合。

---

# 27. 官方资料基线

本文实现原则参考以下官方资料和成熟客户端源码：

- Microsoft WinINet 文档：WinINet 不应在 Windows Service 中使用；per-connection 代理修改后需要刷新。
- sing-box 官方 TUN 文档：`auto_route`、`strict_route`、物理接口检测、DNS 和回环保护。
- Mihomo 官方配置文档：mixed-port、TUN stack、auto-route、auto-detect-interface、DNS hijack、External Controller。
- Xray 官方命令行文档：`run -test` 配置校验。
- Xray 官方 TUN 文档：Windows TUN、自动系统路由和自动出站接口绑定；旧版本需注意只创建网卡的差异。
- Clash Verge Rev 源码：核心 Ready 后应用代理、Service/Sidecar 生命周期、代理 Guard 和回滚。
- v2rayN 源码：生成配置、停止旧核心、清理旧 TUN、启动核心、SOCKS5 握手、设置 WinINet、Job Object 管理。

---

# 28. 一句话执行原则

```text
先证明核心能代理，
再证明 Windows 能把流量交给核心，
最后证明停止或崩溃后 Windows 能恢复。
```

只要 E0～E7、S1～S2、T1～T4、R1、SEC1 全部通过，NAVO 的系统代理和 TUN 就不再是“开关看起来打开”，而是具备真实可用的数据链路和完整恢复能力。
