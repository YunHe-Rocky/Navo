# Navo 功能优化作业指导书（Codex 执行版）

> 适用仓库：`YunHe-Rocky/Navo`  
> 文档用途：直接交给 Codex 分析、修改、测试和提交  
> 目标平台：Windows 10 / Windows 11  
> 任务类型：界面、内核升级、测速、流量监测、日志、IP 风险和图标统一优化  
> 注意：本文是独立优化任务，不替代已有的《Navo 全量问题修复作业指导书》

---

# 0. Codex 总指令

你正在修改的是一个 Windows 代理客户端，包含本机网络、系统代理、TUN、核心进程、流量采集、日志、IP 检测和 Wails 前端。

本次任务不是简单改页面标题。必须保证：

1. 每一个按钮都有真实后端能力；
2. 每一条曲线都来自明确的数据源；
3. 每一个筛选条件都在数据层生效；
4. 所有状态都能区分“未执行、执行中、成功、失败、无数据”；
5. 不允许用随机数伪装真实数据；
6. 虚拟流量只允许用于测试或演示，必须显式标识；
7. 不允许为了界面好看而吞掉后端错误；
8. 不允许修改后破坏系统代理、TUN、核心生命周期和恢复逻辑；
9. 必须补测试；
10. 必须按阶段提交，不允许一次提交一个无法审查的巨型改动。

开始前先阅读并理解：

```text
internal/monitor/
internal/ipdetect/
internal/agent/dashboard.go
internal/service/
internal/coreadapter/
internal/compiler/
internal/host/
internal/supervisor/
navo_app/
scripts/
```

优先搜索：

```text
Core
Kernel
Version
Download
Traffic
Monitor
Network
Latency
SpeedTest
Log
Dashboard
IP
Risk
Tray
Icon
Wails
```

如果当前代码结构与本文名称不同，修改等价模块，不要机械创建重复模块。

---

# 1. 建议执行分支与提交顺序

创建分支：

```powershell
git checkout main
git pull
git checkout -b feat/dashboard-monitoring-optimization
```

建议按以下顺序提交：

```text
1. refactor(core): replace core management with secure core upgrade workflow
2. feat(latency): replace route source card with one-click latency testing
3. feat(traffic): add four-source traffic monitoring and selectable series
4. feat(chart): add tooltip details and synthetic traffic simulation
5. feat(logs): add clear, level, service and date filters
6. fix(monitor): restore dual-link monitoring data pipeline
7. feat(ip-risk): always evaluate direct and proxy IP attributes
8. style(ui): hide visual scrollbars without breaking scroll
9. style(icon): unify tray, taskbar, executable and start-menu icons
10. test(release): add contract, UI and Windows integration coverage
```

每次提交必须包含：

- 实现代码；
- 测试；
- 必要的文档；
- 不得混入无关格式化。

---

# 2. 第一阶段：将“内核管理”改为“升级内核”

优先级：P1  
目标：用户不再直接“管理内核文件”，而是查看当前版本、检查更新、下载匹配版本、校验并安全升级。

涉及模块重点：

```text
internal/coreadapter/
internal/host/
internal/supervisor/
internal/service/
internal/coremanifest/
cmd/navo/
navo_app/
```

## 2.1 页面和文案调整

将所有用户可见的：

```text
内核管理
核心管理
管理内核
```

统一改为：

```text
升级内核
```

页面至少展示：

```text
当前内核
当前版本
可用新版本
安装状态
最后检查时间
下载进度
校验状态
升级结果
```

分别展示：

- sing-box；
- Mihomo；
- Xray。

不要把三个核心合并成一个模糊版本号。

## 2.2 核心升级数据模型

建议新增：

```go
type CoreUpdateInfo struct {
    CoreID           string    `json:"core_id"`
    InstalledVersion string    `json:"installed_version"`
    LatestVersion    string    `json:"latest_version"`
    UpdateAvailable  bool      `json:"update_available"`
    DownloadURL      string    `json:"download_url,omitempty"`
    AssetName        string    `json:"asset_name,omitempty"`
    ExpectedSHA256   string    `json:"expected_sha256,omitempty"`
    ReleaseNotes     string    `json:"release_notes,omitempty"`
    PublishedAt      time.Time `json:"published_at,omitempty"`
    LastCheckedAt    time.Time `json:"last_checked_at,omitempty"`
    State            string    `json:"state"`
    Error            string    `json:"error,omitempty"`
}
```

状态必须明确，例如：

```text
idle
checking
update_available
up_to_date
downloading
verifying
installing
completed
failed
rollback
```

## 2.3 版本检查

Codex 需要实现安全的版本检查接口，例如：

```text
core.update.check
core.update.status
core.update.install
core.update.cancel
```

要求：

1. 根据核心类型查询对应发布源；
2. 识别 Windows AMD64 对应资产；
3. 版本比较使用语义版本，不允许纯字符串比较；
4. 网络失败不影响当前核心运行；
5. 查询结果应缓存；
6. 不允许每次打开页面就高频请求；
7. 支持手动“一键检查更新”。

## 2.4 下载要求

下载必须：

- 使用 HTTPS；
- 有超时；
- 支持取消；
- 限制最大文件体积；
- 下载到专用临时目录；
- 下载时不覆盖当前核心；
- 显示真实进度；
- 禁止路径穿越；
- 禁止使用发布包内提供的任意文件名直接拼接路径；
- 只接受预期的可执行文件或压缩包格式。

下载接口必须返回：

```text
已下载字节
总字节
百分比
当前速度
预计剩余时间（可选）
```

## 2.5 校验要求

升级前必须验证：

1. SHA-256；
2. 发布资产名称；
3. 压缩包内部路径；
4. 解压后可执行文件存在；
5. 可执行文件版本可读取；
6. 执行原生配置检查命令可启动；
7. 核心类型与目标一致。

如果官方发布源没有可靠 SHA-256：

- 使用 Navo 自己维护的受信 manifest；
- manifest 必须随 Navo 发布并签名或至少固化 hash；
- 不得只因为下载成功就安装。

## 2.6 安装事务

升级顺序：

```text
1. 下载到临时目录
2. 校验
3. 解压到 staging 目录
4. 执行版本检查
5. 停止当前核心
6. 备份当前核心
7. 原子替换
8. 启动新核心
9. 执行健康检查
10. 成功后删除旧备份或保留最近一个
```

失败时：

```text
1. 停止新核心
2. 恢复旧核心
3. 启动旧核心
4. 验证旧核心可用
5. 返回升级失败并说明是否已回滚
```

禁止：

- 下载后直接覆盖正在运行的 exe；
- 安装失败后不恢复旧版本；
- 同时升级多个核心而没有互斥；
- 在 TUN transition 中途升级；
- 核心正在切换节点时升级。

## 2.7 UI 行为

按钮建议：

```text
检查更新
下载并升级
取消下载
查看版本说明
```

升级按钮必须根据状态禁用或启用。

升级中显示：

```text
正在下载
正在校验
正在安装
正在验证
正在回滚
```

不能只显示一个无意义的旋转图标。

## 2.8 测试

至少覆盖：

- 已是最新；
- 有新版本；
- 网络失败；
- 下载中取消；
- SHA-256 不匹配；
- 压缩包路径穿越；
- 新核心无法启动；
- 回滚成功；
- 回滚失败进入 faulted；
- 三个核心分别独立升级；
- 升级期间禁止并发启动第二个升级。

---

# 3. 第二阶段：将“线路来源”改为“一键测速 / 延迟”

优先级：P1

## 3.1 页面调整

原“线路来源”区域改为：

```text
一键测速
```

展示：

```text
当前节点
TCP 延迟
代理握手延迟
HTTP 首包延迟
完整请求耗时
测试时间
测试状态
```

按钮文案：

```text
开始测速
重新测速
停止测速
```

不要只测端口然后显示“线路可用”。

## 3.2 测速分层

至少区分：

### A. TCP 连接延迟

```text
连接节点 server:port 所需时间
```

只能说明端口可达。

### B. 代理握手延迟

通过目标核心和目标节点完成协议握手。

可判断：

- UUID；
- 密码；
- TLS；
- Reality；
- SNI；
- 传输层；

是否基本可用。

### C. HTTP 首包延迟

通过代理请求受控 HTTPS 地址，记录：

```text
DNS
Connect
TLS
TTFB
Total
```

### D. 实际出口确认

检查请求是否真正从代理出口发出，并返回出口 IP。

## 3.3 数据模型

建议：

```go
type LatencyResult struct {
    OutboundID       string        `json:"outbound_id"`
    State            string        `json:"state"`
    TCPConnect       time.Duration `json:"tcp_connect_ms"`
    ProxyHandshake   time.Duration `json:"proxy_handshake_ms"`
    DNS              time.Duration `json:"dns_ms"`
    TLS              time.Duration `json:"tls_ms"`
    TTFB             time.Duration `json:"ttfb_ms"`
    Total            time.Duration `json:"total_ms"`
    ExitIP           string        `json:"exit_ip,omitempty"`
    CheckedAt        time.Time     `json:"checked_at"`
    ErrorCode        string        `json:"error_code,omitempty"`
    ErrorMessage     string        `json:"error_message,omitempty"`
}
```

## 3.4 测速限制

- 默认只测当前选中节点；
- 批量测速必须限制并发；
- 必须可取消；
- 不得改变当前用户 Capture 状态；
- 不得污染系统代理；
- 临时测试核心必须使用隔离端口；
- 测试结束必须清理临时核心和配置；
- 失败原因要区分：
  - DNS；
  - TCP；
  - TLS；
  - 身份验证；
  - 核心配置；
  - 超时；
  - 出口检测。

## 3.5 UI 颜色建议

延迟颜色应可配置，默认：

```text
0–80ms      优秀
81–150ms    良好
151–250ms   一般
251–500ms   较差
>500ms      很差
失败         红色错误
未测试       中性
```

不要只使用红绿两色，避免色觉用户无法区分。必须同时显示文字或图标。

---

# 4. 第三阶段：流量监测改为四条数据曲线

优先级：P1

需要同时监测：

```text
1. 本机上传
2. 本机下载
3. 开启服务后的代理上传
4. 开启服务后的代理下载
```

## 4.1 指标定义必须明确

### 本机上传 / 下载

表示 Windows 物理或用户选定网络接口的总流量速度。

默认排除：

- Loopback；
- Navo TUN 虚拟网卡；
- Wintun；
- Hyper-V 内部接口；
- 已明确标记为虚拟或非活动接口。

但设置页应允许用户选择包含哪些接口。

### 代理上传 / 下载

表示经过当前 Navo 核心代理服务的数据量。

不能简单用“本机总流量减去启用前流量”推算。

优先数据来源：

1. 核心 controller/API 真实统计；
2. Navo 转发层字节计数；
3. TUN 适配器真实字节计数；
4. 无法可靠采集时明确显示“不支持/无数据”。

不同内核统计口径要统一，至少统一为：

```text
bytes per second
累计上传字节
累计下载字节
```

## 4.2 防止重复统计

TUN 模式下：

- 本机物理网卡看到加密后的代理流量；
- TUN 网卡看到进入代理前流量；
- 核心统计又看到代理流量。

页面需要明确它们是不同口径，不应求和为“总流量”。

标签建议：

```text
本机出口上传
本机入口下载
代理业务上传
代理业务下载
```

## 4.3 采样模型

建议统一采样结构：

```go
type TrafficSample struct {
    Timestamp          time.Time `json:"timestamp"`
    LocalUploadBps     uint64    `json:"local_upload_bps"`
    LocalDownloadBps   uint64    `json:"local_download_bps"`
    ProxyUploadBps     uint64    `json:"proxy_upload_bps"`
    ProxyDownloadBps   uint64    `json:"proxy_download_bps"`
    LocalUploadTotal   uint64    `json:"local_upload_total"`
    LocalDownloadTotal uint64    `json:"local_download_total"`
    ProxyUploadTotal   uint64    `json:"proxy_upload_total"`
    ProxyDownloadTotal uint64    `json:"proxy_download_total"`
    SourceState        string    `json:"source_state"`
}
```

建议：

- 后端每秒或每两秒采样；
- 前端不自行重新计算业务口径；
- 默认保留最近 60 秒；
- 可扩展 5 分钟、15 分钟；
- 长时间页面隐藏时减少刷新频率；
- 返回时间戳，不能只返回数组下标。

## 4.4 计数器处理

必须处理：

- 计数器重置；
- 核心重启；
- 网卡切换；
- uint 溢出；
- 采样延迟；
- 系统睡眠；
- 服务关闭；
- 数据暂时不可用。

如果当前值小于上次值，应视为 reset，不得算出超大速度。

## 4.5 四条曲线的默认显示

默认启用：

```text
本机下载
代理下载
```

其他两条可由用户勾选。

也可以保留上次用户选择。

---

# 5. 第四阶段：增加可选监测属性与颜色区分

优先级：P1

## 5.1 选择框

在流量图上方增加复选框或多选下拉框：

```text
☑ 本机上传
☑ 本机下载
☑ 代理上传
☑ 代理下载
```

后续可扩展：

```text
延迟
丢包率
连接数
CPU
内存
```

但本次至少完成四个流量指标。

## 5.2 持久化用户选择

将用户选择保存在本地设置：

```go
type TrafficChartPreferences struct {
    VisibleSeries []string          `json:"visible_series"`
    SeriesColors  map[string]string `json:"series_colors"`
    WindowSeconds int               `json:"window_seconds"`
}
```

启动后恢复。

无效或旧字段要安全忽略并使用默认值。

## 5.3 颜色规范

四条曲线必须固定使用不同颜色。

要求：

- 颜色在浅色和深色背景下均可辨认；
- 不能只靠颜色区分；
- 图例、线型或图标同时区分；
- 用户取消某条曲线后，颜色不能重新分配给其他指标；
- tooltip 颜色与曲线一致；
- 颜色定义集中管理，不得散落硬编码。

建议语义：

```text
本机上传：暖色 + 上箭头
本机下载：冷色 + 下箭头
代理上传：紫色系 + 上箭头
代理下载：绿色系 + 下箭头
```

具体色值由现有设计系统决定，不要在业务代码内写死。

## 5.4 无数据状态

某指标无数据时：

- 图例仍可显示；
- 明确标记“无数据”；
- 不要绘制为 0 并误导用户；
- 后端应提供状态原因，例如：
  - service_off；
  - core_unsupported；
  - collector_error；
  - no_active_interface；
  - permission_denied。

---

# 6. 第五阶段：虚拟文件大小模拟流量曲线

优先级：P2  
用途：测试曲线、演示界面、验证四条数据线，不得伪装生产真实流量。

## 6.1 功能入口

该功能应放在：

```text
设置 → 开发者工具 / 流量监测测试
```

正式用户界面可以隐藏在“高级设置”中。

页面允许选择：

```text
模拟本机流量
模拟代理流量
模拟上传
模拟下载
模拟双向
```

设置虚拟文件大小：

```text
1 MB
10 MB
100 MB
500 MB
自定义
```

## 6.2 两种模拟模式

### A. 纯数据模拟

只向图表数据管线注入带标记的 synthetic sample。

优点：

- 不消耗真实网络；
- 测试图表稳定；
- 可用于自动测试。

必须显示：

```text
模拟数据
```

不得与真实数据混合后不标记。

### B. 真实传输模拟

通过本地测试服务器或受控测试端点传输指定大小数据。

用途：

- 验证本机 NIC 数据；
- 验证代理数据；
- 验证双链路采集。

要求：

- 用户明确点击开始；
- 显示预计消耗；
- 可取消；
- 限速；
- 不得默认上传用户文件；
- 使用随机字节流或零数据流；
- 不持久化内容；
- 代理测试必须确认走代理；
- 测试结束后清理临时资源。

## 6.3 模拟结果

记录：

```text
目标大小
实际传输大小
耗时
平均速度
峰值速度
是否经过代理
出口 IP
取消或失败原因
```

## 6.4 禁止行为

- 不允许后台自动开始大流量测试；
- 不允许把纯模拟数据计入真实累计流量；
- 不允许为了制造曲线下载不受控的大文件；
- 不允许使用用户真实文件；
- 不允许在移动热点环境下默认运行。

---

# 7. 第六阶段：曲线图自动展开信息框

优先级：P1

用户鼠标移动到图表途中时，自动显示信息框，方便读取该时间点的数据。

## 7.1 Tooltip 交互

必须支持：

- 鼠标 hover；
- 触屏点击；
- 最近数据点吸附；
- 十字辅助线；
- 当前时间；
- 所有已勾选曲线的值；
- 当前单位；
- 数据来源状态。

示例：

```text
23:41:16
本机上传      1.2 MB/s
本机下载      8.4 MB/s
代理上传      820 KB/s
代理下载      6.7 MB/s
```

## 7.2 自动单位

统一单位转换：

```text
B/s
KB/s
MB/s
GB/s
```

同一个 tooltip 内尽量使用统一或易比较单位。

累计值使用：

```text
KB
MB
GB
TB
```

禁止把 bit/s 与 byte/s 混用。

## 7.3 信息框行为

- 靠近右侧时向左展开；
- 靠近顶部时向下展开；
- 不能遮挡鼠标点；
- 不得超出窗口；
- 鼠标离开图表后关闭；
- 触屏再次点击或点击外部关闭；
- 暂停图表时 tooltip 保持可读；
- 页面缩放时位置正确。

## 7.4 性能

不能在每次 mousemove 时重新请求后端。

Tooltip 只读取已缓存的前端 sample。

曲线更新建议：

- 数据采集和绘图解耦；
- 使用 requestAnimationFrame；
- 页面不可见时降低绘图频率；
- 不要每个 sample 重建整个图表组件。

---

# 8. 第七阶段：日志功能增强

优先级：P1

设置中的日志功能需要新增：

1. 清空文本信息；
2. 日志级别分级；
3. 内部服务分级；
4. 起始日期和截止日期查询；
5. 快速定位结果。

涉及模块：

```text
日志采集
日志存储
Agent/Service/Core 日志
前端设置日志页
```

## 8.1 日志结构化

不要继续只保存纯文本。

建议统一：

```go
type LogEntry struct {
    ID        uint64    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Level     string    `json:"level"`
    Service   string    `json:"service"`
    Component string    `json:"component,omitempty"`
    Message   string    `json:"message"`
    Fields    map[string]any `json:"fields,omitempty"`
}
```

必须至少支持等级：

```text
TRACE
DEBUG
INFO
WARN
ERROR
FATAL
```

如果现有系统没有 TRACE/FATAL，可按实际支持调整，但 UI 与后端必须一致。

## 8.2 内部服务分类

至少区分：

```text
Launcher
UI
Agent
Service
Supervisor
sing-box
Mihomo
Xray
TUN
SystemProxy
NetworkMonitor
IPDetection
Subscription
CoreUpdate
AI
```

不得根据日志文本内容临时猜测服务名。

日志产生时就要带 `service/component` 字段。

## 8.3 清空日志

增加按钮：

```text
清空当前显示
清空全部日志
```

推荐默认只提供“清空全部日志”，并弹出确认。

清理流程必须：

- 停止当前日志文件写入或安全轮转；
- 删除/截断受控日志文件；
- 重新打开 writer；
- 不影响正在运行的核心；
- 返回清理结果；
- 清理失败显示具体原因；
- 不删除崩溃转储和用户导出文件。

建议支持：

```text
仅清空文本显示
清空持久化日志
```

两个行为要明确区分。

## 8.4 日志级别筛选

UI 增加多选：

```text
DEBUG
INFO
WARN
ERROR
```

筛选要在后端查询和前端实时流中都生效。

大量日志不能全部加载后再由前端筛选。

## 8.5 服务筛选

增加服务多选：

```text
Agent
Service
Core
TUN
SystemProxy
...
```

允许：

```text
全部
仅选中
```

选择状态可持久化。

## 8.6 日期筛选

增加：

```text
起始日期时间
截止日期时间
查询
重置
```

要求：

- 使用本地时区显示；
- 后端统一以带时区时间戳查询；
- 起始时间不得大于截止时间；
- 截止日期默认包含当天末尾；
- 用户选择日期后快速定位第一条命中日志；
- 支持分页或游标；
- 大日志文件不得一次性全部读入内存。

## 8.7 实时日志与历史查询

两种模式要区分：

```text
实时跟随
历史查询
```

用户滚动查看历史时，不得每来一条新日志就把滚动位置强制拉到底部。

提供：

```text
继续跟随
```

## 8.8 脱敏

日志不得出现：

- 代理密码；
- UUID 完整值；
- 订阅 URL token；
- AI API key；
- Authorization header；
- Cookie；
- DPAPI 明文；
- 完整敏感配置。

## 8.9 测试

覆盖：

- 按 level；
- 按 service；
- 按日期；
- 组合筛选；
- 清空显示；
- 清空持久化；
- 日志轮转中清理；
- 时间边界；
- 大量日志分页；
- 敏感字段脱敏。

---

# 9. 第八阶段：修复“双联链路监测无法获取数据”

优先级：P0/P1

## 9.1 先定位“双联链路”的定义

Codex 必须先检查现有产品中的“双联链路监测”究竟表示：

- 本机直连 + 代理链路；
- 入口 + 出口；
- TCP + HTTP；
- 两个探测目标；
- 两张网卡；
- Direct IP + Proxy IP。

不要猜测后直接重写。

在代码中确认：

```text
前端字段
Wails 调用
Agent 方法
Service 方法
collector
返回 JSON
```

## 9.2 全调用链诊断

逐层检查：

```text
采集器是否启动
采集 context 是否提前取消
channel 是否有消费者
字段名是否一致
时间戳是否有效
是否被 service_off 条件提前 return
是否因为 proxy 未开启而跳过 direct
错误是否被吞掉
前端是否把 null 当成 0
```

增加诊断状态：

```go
type LinkMonitorStatus struct {
    DirectState string `json:"direct_state"`
    ProxyState  string `json:"proxy_state"`
    DirectError string `json:"direct_error,omitempty"`
    ProxyError  string `json:"proxy_error,omitempty"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

## 9.3 正确行为

### 代理关闭

仍然执行：

```text
直连链路监测
直连 IP 检测
直连延迟
直连风险摘要
```

代理链路显示：

```text
服务未开启
```

而不是整块无数据。

### 代理开启

同时执行：

```text
直连链路
代理链路
```

代理链路必须明确通过 Navo 代理访问，不得使用系统默认网络误判。

## 9.4 请求隔离

Direct client：

```text
不读取系统代理环境变量
不使用 Navo 本地代理
不走 TUN（如果操作系统无法保证，则明确说明 direct 口径）
```

Proxy client：

```text
显式连接 Navo 本地代理端口或受控测试入口
```

不要让两个 client 共用一个带环境代理的默认 `http.Client`。

## 9.5 定时与缓存

- 每次节点切换立即刷新代理链路；
- Capture 模式变化立即刷新；
- 普通状态按合理周期刷新；
- 失败使用退避；
- 不得每秒请求公共 IP 服务；
- 显示上次成功时间和缓存状态。

---

# 10. 第九阶段：IP 风险摘要始终执行

优先级：P1

无论是否开启代理，IP 风险摘要都必须执行。

## 10.1 页面显示两个对象

建议分成：

```text
本机出口 IP
代理出口 IP
```

代理关闭时：

```text
本机出口 IP：正常检测
代理出口 IP：服务未开启
```

不能因为代理关闭而不检测本机 IP。

## 10.2 必须判断的属性

### A. IP 纯净度

必须把“纯净度”拆成可解释指标，不能只给一个神秘百分数。

至少考虑：

```text
代理/VPN 检测
Tor 出口
公开代理记录
滥用记录
垃圾邮件记录
黑名单记录
历史欺诈风险
共享程度
ASN 风险
数据中心特征
地理漂移
DNS/时区一致性（仅在有可靠数据时）
```

给出：

```text
低风险
中风险
高风险
未知
```

并列出原因。

如果仍计算分数，必须展示：

```text
分数来源
更新时间
数据供应商
可用字段
未知字段
```

不得把无法查询当成“纯净”。

### B. IP 类型

至少分类：

```text
家庭住宅
移动网络
数据中心/机房
企业网络
教育/政府
卫星网络
公共代理
VPN
Tor
未知
```

类型判断应允许多标签，例如：

```text
数据中心 + VPN
住宅 + 共享出口
移动网络 + CGNAT
```

不要强行单选导致误导。

### C. 其他有价值信息

建议展示：

```text
国家/地区
城市
经纬度精度说明
ASN
AS 名称
ISP
组织
运营商
主机名/PTR
时区
网络段
是否 Anycast
是否 Hosting
是否 Mobile
是否 Proxy
是否 VPN
是否 Tor
是否 Bogon
黑名单命中数
滥用置信度
最近报告时间
```

只展示数据源真实提供的字段。

## 10.3 多数据源聚合

不要依赖单一 IP API。

设计 provider 接口：

```go
type IPRiskProvider interface {
    Name() string
    Lookup(ctx context.Context, ip net.IP) (IPRiskEvidence, error)
}
```

聚合结果：

```go
type IPRiskSummary struct {
    IP             string           `json:"ip"`
    Scope          string           `json:"scope"`
    RiskLevel      string           `json:"risk_level"`
    PurityScore    *int             `json:"purity_score,omitempty"`
    NetworkTypes   []string         `json:"network_types"`
    ASN            string           `json:"asn,omitempty"`
    ISP            string           `json:"isp,omitempty"`
    Country        string           `json:"country,omitempty"`
    City           string           `json:"city,omitempty"`
    IsProxy        *bool            `json:"is_proxy,omitempty"`
    IsVPN          *bool            `json:"is_vpn,omitempty"`
    IsTor          *bool            `json:"is_tor,omitempty"`
    IsHosting      *bool            `json:"is_hosting,omitempty"`
    IsMobile       *bool            `json:"is_mobile,omitempty"`
    AbuseScore     *int             `json:"abuse_score,omitempty"`
    BlacklistHits  *int             `json:"blacklist_hits,omitempty"`
    Reasons        []string         `json:"reasons"`
    Providers      []ProviderStatus `json:"providers"`
    CheckedAt      time.Time        `json:"checked_at"`
    Cached         bool             `json:"cached"`
}
```

## 10.4 结果冲突

不同供应商可能冲突。

规则：

- 保留 provider 原始判断；
- 聚合层显示“供应商判断不一致”；
- 不要偷偷选一个结果；
- 风险结果注明置信度；
- `unknown` 不能按低风险处理。

## 10.5 隐私和频率

- 明确告诉用户查询会把 IP 发送给第三方供应商；
- 允许关闭第三方风险查询；
- 基础本机 IP 检测仍可执行；
- 结果缓存；
- 节点切换后重新查；
- 供应商限速；
- 不记录完整 API key；
- 不使用明文 HTTP。

## 10.6 错误状态

分别展示：

```text
IP 获取失败
风险数据获取失败
部分供应商失败
数据已过期
未知类型
```

不能统一显示空白。

---

# 11. 第十阶段：隐藏滚动条但保留滚动能力

优先级：P2

用户要求隐藏滚动条，不能因此禁止页面滚动。

## 11.1 CSS 统一类

创建统一工具类，例如：

```css
.scrollbar-hidden {
  overflow: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.scrollbar-hidden::-webkit-scrollbar {
  width: 0;
  height: 0;
  display: none;
}
```

只应用在需要隐藏视觉滚动条的容器。

## 11.2 可访问性

必须保留：

- 鼠标滚轮；
- 触摸滚动；
- 键盘 PageUp/PageDown；
- Home/End；
- 焦点可见；
- 内部内容不被裁剪。

不允许全局：

```css
overflow: hidden
```

导致页面无法滚动。

## 11.3 日志页例外

日志页可以隐藏原生滚动条，但需要：

- 保留滚动位置提示；
- 显示“回到底部”按钮；
- 历史查询可定位；
- 大数据使用虚拟列表。

---

# 12. 第十一阶段：统一优化所有图标

优先级：P1/P2

必须检查并统一：

```text
exe 文件图标
窗口标题栏图标
任务栏图标
系统托盘图标
开始菜单图标
桌面快捷方式图标
安装程序图标
通知图标
关于页面图标
应用内品牌图标
```

## 12.1 图标状态

Navo 图标至少支持以下状态：

```text
默认/未开启
代理已开启且可用
代理已开启但不可用
正在连接
故障/需修复
```

如果项目已有品牌状态定义，沿用并统一，不要再创建另一套互相冲突的颜色语义。

状态图标要求：

- 缩小到 16×16 仍可辨认；
- 不能只靠细小装饰；
- 托盘深浅背景都清晰；
- 红色只用于错误；
- 动画状态不能频繁闪烁；
- 正常状态颜色和 UI 状态一致。

## 12.2 Windows ICO

主 ICO 应包含多尺寸：

```text
16×16
20×20
24×24
32×32
40×40
48×48
64×64
128×128
256×256
```

不允许只有一张大 PNG 强行缩放。

## 12.3 资源唯一来源

建立统一资源目录，例如：

```text
assets/branding/
  navo-source.svg
  navo.ico
  tray/
  status/
```

构建脚本、Wails、安装程序和快捷方式都从该目录取资源。

避免项目内出现多个旧版 `icon.ico`。

## 12.4 托盘图标

根据运行状态实时切换：

```text
off
connecting
healthy
degraded
faulted
```

Tooltip 显示：

```text
Navo
当前模式
当前节点
状态
```

右键菜单至少保持：

```text
打开 Navo
开启/关闭
当前模式
退出
```

托盘状态切换不能创建多个重复托盘图标。

## 12.5 任务栏与开始菜单

检查：

- AppUserModelID；
- 快捷方式目标路径；
- 快捷方式 IconLocation；
- 安装升级后缓存刷新；
- 窗口图标与 exe 一致；
- 开始菜单没有默认空白图标；
- 卸载后快捷方式清理。

## 12.6 图标验收

在以下环境截图验证：

```text
Windows 10 浅色
Windows 10 深色
Windows 11 浅色
Windows 11 深色
100% DPI
125% DPI
150% DPI
```

---

# 13. 前后端接口建议

Codex 可以根据当前 IPC 风格调整名称，但功能必须等价。

## 13.1 核心升级

```text
core.update.check
core.update.list
core.update.install
core.update.cancel
core.update.status
```

## 13.2 测速

```text
latency.test.current
latency.test.cancel
latency.test.status
```

如支持批量：

```text
latency.test.batch
```

## 13.3 流量

```text
traffic.snapshot
traffic.stream.start
traffic.stream.stop
traffic.preferences.get
traffic.preferences.set
traffic.simulation.start
traffic.simulation.stop
```

如果现有系统使用 Agent 推送事件，应优先事件流，不要高频轮询。

## 13.4 日志

```text
logs.query
logs.stream.start
logs.stream.stop
logs.clear.view
logs.clear.persisted
logs.services
logs.levels
```

## 13.5 IP 风险

```text
ip.status
ip.detect.direct
ip.detect.proxy
ip.risk.direct
ip.risk.proxy
ip.risk.refresh
```

---

# 14. 前端状态设计

每个模块必须区分以下状态：

```text
idle
loading
ready
empty
unsupported
failed
stale
```

不要使用一个 `bool loading` 处理所有情况。

示例：

```ts
type AsyncState<T> =
  | { status: 'idle' }
  | { status: 'loading'; previous?: T }
  | { status: 'ready'; data: T }
  | { status: 'empty'; reason: string }
  | { status: 'unsupported'; reason: string }
  | { status: 'stale'; data: T; error: string }
  | { status: 'failed'; error: string };
```

页面必须能显示后端错误，不得无限转圈。

---

# 15. 测试要求

## 15.1 Go 单元测试

至少覆盖：

```text
版本比较
下载 URL/资产选择
SHA-256 校验
升级回滚
流量计数器差值
计数器 reset
四条曲线 sample
日志筛选
日期边界
IP 风险聚合
供应商冲突
缓存过期
直接/代理 HTTP client 隔离
```

## 15.2 前端测试

至少覆盖：

```text
内核升级状态切换
下载进度
测速结果
四曲线选择
颜色映射稳定
tooltip 内容
无数据状态
模拟数据标记
日志筛选组合
日期定位
滚动条隐藏后仍可滚动
IP 风险 direct/proxy 双卡片
托盘状态映射
```

## 15.3 Windows 集成测试

至少验证：

```text
核心升级成功
核心升级失败回滚
核心正在运行时升级
系统代理模式流量
TUN 模式流量
关闭代理后 direct IP 仍检测
开启代理后 direct + proxy 同时检测
网络断开恢复
网卡切换
睡眠唤醒
四条曲线没有负数或巨大尖峰
日志清空不影响服务
托盘状态切换
开始菜单图标正确
```

---

# 16. 性能要求

## 流量监测

- 常规采样不能显著增加 CPU；
- 页面最小化时降低绘图频率；
- 后端样本缓冲有固定上限；
- 不允许无限增长 slice；
- 每条 sample 不重复包含大对象。

## 日志

- 大文件使用分页或游标；
- 实时日志有缓冲上限；
- UI 使用虚拟列表；
- 筛选不应阻塞主线程。

## IP 风险

- 有缓存；
- 有限流；
- 不频繁访问第三方 API；
- 单个供应商失败不阻塞全部结果。

## 图表

- mousemove 不请求后端；
- tooltip 不触发完整组件重渲染；
- 不可见页面暂停动画。

---

# 17. 安全要求

1. 核心升级必须校验下载内容；
2. 防止压缩包路径穿越；
3. 不执行下载包内未知脚本；
4. 不允许外部输入任意安装路径；
5. 日志必须脱敏；
6. IP 风险 API key 不进入日志；
7. 模拟流量不上传用户文件；
8. 第三方 IP 查询必须 HTTPS；
9. 所有长任务支持取消；
10. 临时文件退出后清理；
11. 错误信息不回显密码、token、UUID 全值；
12. 更新过程中不能破坏当前可用核心。

---

# 18. 禁止的伪实现

Codex 不得：

1. 只把“内核管理”文字改成“升级内核”；
2. 用固定版本号假装检查更新；
3. 下载完不校验 hash；
4. 用 TCP 端口连通代替代理真实测速；
5. 用随机数作为真实流量；
6. 把代理关闭时的 proxy 流量画成 0 而不说明；
7. 用本机总流量减法推算代理流量；
8. 把四条曲线全部使用相近颜色；
9. Tooltip 只显示一个点，不显示其他已选曲线；
10. 清空日志只清前端数组但按钮写“清空全部”；
11. 通过文本包含关系猜日志服务类型；
12. 日期筛选只在前端对已经加载的一小部分日志生效；
13. 代理关闭时停止 Direct IP 风险检测；
14. 单一供应商失败就把 IP 标记为安全；
15. 用 `overflow: hidden` 全局禁止滚动；
16. 只替换 exe 图标，不更新托盘和开始菜单；
17. 为制造曲线自动下载大文件；
18. 忽略升级回滚错误；
19. 在测试失败时删除测试；
20. 没有真实测试证据就宣称完成。

---

# 19. 验收清单

## 内核升级

- [ ] 页面名称已改为“升级内核”；
- [ ] 可显示三个核心当前版本；
- [ ] 可检查最新版本；
- [ ] 可下载、取消、显示进度；
- [ ] 下载内容经过校验；
- [ ] 新核心失败可回滚；
- [ ] 升级不会破坏当前网络状态；
- [ ] 不允许并发升级。

## 一键测速

- [ ] 原“线路来源”区域已改为测速；
- [ ] 显示 TCP 延迟；
- [ ] 显示代理握手结果；
- [ ] 显示 TTFB 和总耗时；
- [ ] 可取消；
- [ ] 失败原因可见；
- [ ] 测试不改变当前 Capture 状态。

## 四条流量曲线

- [ ] 本机上传；
- [ ] 本机下载；
- [ ] 代理上传；
- [ ] 代理下载；
- [ ] 口径清晰；
- [ ] 无重复统计误导；
- [ ] 核心重启后无异常尖峰；
- [ ] 服务关闭时状态正确。

## 用户选择和颜色

- [ ] 四条曲线可独立勾选；
- [ ] 选择被持久化；
- [ ] 颜色固定；
- [ ] 颜色和文字/图标同时区分；
- [ ] 无数据不是伪 0。

## 流量模拟

- [ ] 可选择本机或代理；
- [ ] 可选择上传、下载或双向；
- [ ] 可设置虚拟大小；
- [ ] 模拟数据明确标识；
- [ ] 不计入真实累计；
- [ ] 可取消；
- [ ] 无临时文件残留。

## 曲线信息框

- [ ] 鼠标经过自动展开；
- [ ] 触屏可用；
- [ ] 显示时间和所有选中属性；
- [ ] 单位正确；
- [ ] 不超出窗口；
- [ ] 不高频请求后端。

## 日志

- [ ] 可清空界面文本；
- [ ] 可清空持久化日志；
- [ ] 可按等级筛选；
- [ ] 可按内部服务筛选；
- [ ] 可按起止日期筛选；
- [ ] 可快速定位结果；
- [ ] 大日志不卡顿；
- [ ] 敏感内容已脱敏。

## 双联链路

- [ ] 代理关闭时 Direct 有数据；
- [ ] 代理开启时 Direct 和 Proxy 都有数据；
- [ ] 两个 client 路径隔离；
- [ ] 错误和更新时间可见；
- [ ] 节点切换会刷新。

## IP 风险摘要

- [ ] 代理关闭时仍执行 Direct；
- [ ] 代理开启时同时执行 Direct/Proxy；
- [ ] 显示纯净度或风险等级；
- [ ] 显示 IP 类型；
- [ ] 支持住宅、移动、机房等多标签；
- [ ] 显示 ASN/ISP/地区等有价值信息；
- [ ] 数据冲突可见；
- [ ] 未知不当作安全；
- [ ] 第三方查询可关闭；
- [ ] 使用 HTTPS 和缓存。

## 滚动条

- [ ] 视觉滚动条隐藏；
- [ ] 鼠标滚轮仍可用；
- [ ] 触屏仍可用；
- [ ] 键盘仍可用；
- [ ] 日志页可快速回到底部。

## 图标

- [ ] exe 图标正确；
- [ ] 任务栏图标正确；
- [ ] 窗口图标正确；
- [ ] 托盘图标正确；
- [ ] 开始菜单图标正确；
- [ ] 快捷方式图标正确；
- [ ] 状态图标统一；
- [ ] 多 DPI 和深浅主题清晰；
- [ ] 无重复托盘图标。

---

# 20. Codex 最终交付格式

Codex 完成后必须输出：

```markdown
# Navo 功能优化完成报告

## 1. 修改文件
- ...

## 2. 内核升级
- 实现：
- 测试：
- 未完成：

## 3. 一键测速
- 实现：
- 测试：
- 未完成：

## 4. 流量监测
- 四条数据源：
- 采样周期：
- 数据口径：
- 测试：

## 5. 图表交互
- ...

## 6. 日志
- ...

## 7. 双联链路
- 原因：
- 修复：
- 验证：

## 8. IP 风险
- 数据源：
- 聚合规则：
- 隐私说明：
- 测试：

## 9. 图标与滚动条
- ...

## 10. 测试结果
```text
go test ./...
...
```

## 11. Windows 手工验证
- ...

## 12. 已知限制
- ...

## 13. 是否达到全部验收标准
- 是 / 否
- 未达到项：
```

没有测试、截图或可复现验证结果的项目，不得写“已完成”。

---

# 21. 最终产品效果

完成后，Navo 应表现为：

```text
用户打开首页
    ↓
始终看到本机 IP 和风险摘要
    ↓
代理开启后同时看到代理 IP 和风险摘要
    ↓
点击一键测速可得到真实延迟和出口结果
    ↓
流量图可选择四条真实数据曲线
    ↓
鼠标经过曲线可读取精确时间点信息
    ↓
日志可按等级、服务、日期快速定位
    ↓
内核可安全检查、下载、升级、验证和回滚
    ↓
托盘、任务栏、开始菜单和窗口图标保持统一
```

不应再出现：

```text
代理关闭后 IP 风险区域空白
双联链路一直无数据
四条曲线来源不明
随机数伪装流量
日志无法筛选和定位
升级核心只是替换文件
任务栏、托盘、开始菜单图标互相不一致
隐藏滚动条后页面无法滚动
```
