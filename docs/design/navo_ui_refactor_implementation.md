# Navo UI 重构实施说明

## 1. 现状分析

- 原运行概览同时承担内核、线路、接管配置和状态展示，形成了不真实的 `01 → 02 → 03` 向导。
- 状态此前由页面内多个 `computed` 临时组合，没有稳定的 `CoreState / ConnectionState / NetworkHealthState / IconState` 边界。
- 托盘图标由 Go 管理，窗口品牌区由 Vue 单独绘制，尚未共享一份跨进程状态事件；本次先统一 Vue 内状态推导，正式系统图标等待用户确认。
- `metrics.current` 原先只读取未接入生产流量的 `monitor.Collector`，现已改为优先读取 Core Metrics Adapter。
- sing-box Clash API 与 Mihomo Controller 使用 loopback-only、随机 secret 的 `/connections` 返回真实累计流量和活动连接。
- Xray 生成配置虽已启用 Stats policy，但尚未暴露本地 Stats API，因此明确显示能力不可用，不生成模拟数据。

## 2. 指标能力审计

| 内核 | 当前可用 | 可扩展真实指标 | 当前限制 |
|---|---|---|---|
| sing-box | 进程、端口、运行时间、出口探测、累计流量、活动连接 | Clash API `/connections` | 已接入 |
| Mihomo | 进程、Controller 健康检查、累计流量、活动连接 | `/traffic`、`/connections`、节点延迟 | `/connections` 已接入 |
| Xray | 进程、端口、运行时间、出口探测 | Stats API 启用后可取 inbound/outbound 累计流量 | 当前生成配置未开启 Stats API |

统一接口使用 `coreadapter.MetricsReader` 输出累计上传、累计下载和活动连接；速率由固定周期采样层计算，避免各页面直接访问内核。

## 3. 新页面职责

1. 运行概览：只读连接状态、双链路 IP、风险摘要、当前速度、60 秒图和运行摘要。
2. 连接管理：线路类型、节点、接管模式、连接/断开；手动内核选择折叠到高级设置。
3. 线路来源：机场订阅和独享代理分开管理，共享统一线路列表。
4. 内核管理：安装、版本、运行状态、指标能力和最近错误。
5. 流量监控：当前速率、固定 30 点的 60 秒图、能力降级说明。
6. IP 检测：Direct/Proxy 双链路详情与公开网络属性风险。
7. 诊断日志：运行日志。
8. 设置：状态图标方向和尺寸预览，正式资源不替换。

## 4. 状态模型

`frontend/src/state.ts` 是窗口 UI 的唯一派生入口：

```text
Dashboard + ActiveRoute + health debounce counters
  -> CoreState
  -> ConnectionState
  -> NetworkHealthState
  -> IconState(default | airport | proxy | error)
```

- 连续失败 3 次进入异常；
- 连续成功 2 次恢复健康；
- Error 优先级最高；
- 机场正常为紫色状态层，独享代理正常为绿色状态层；
- 主操作按钮使用蓝色，避免绿色同时表达操作和线路状态。

## 5. IP 检测与降级

- `App.CheckIP` 新增显式 UI 接口，调用已有 `ip.check`。
- Direct Detector 不使用代理 Transport，Proxy Detector 强制使用 Navo 本地 mixed proxy。
- IP echo 按 ipify、icanhazip、ifconfig.me 顺序降级。
- Geo enrichment 增加 ISP、ASN、网络组织、Mobile、Proxy、Hosting、来源和检测时间。
- 修复 Geo enrichment 复用已取消 context 导致国家/ASN 经常为空的问题。
- Fraud、Abuse、Blacklist、VPN/Tor 没有可靠数据源时明确显示“未配置/不提供”，不伪造纯净度分数。

## 6. 流量采样

- `TrafficRingBuffer(30)` 固定容量；
- 每 2 秒读取一次 `dashboard.snapshot`；
- 由累计字节差分计算 B/s；
- 节点 ID 变化立即清空旧窗口；
- 页面隐藏时继续采样内存数据，但不更新图表 reactive snapshot；
- 代理停止后保留最后一段并标记“采样已停止”。

## 7. 图标方案

当前 N 字形在 16–32px 仍可识别，适合“沿用并优化”。本次只实现状态预览，不修改正式 ICO：

1. 沿用 N 标识，优化像素对齐并增加状态角标；
2. 路由节点抽象符号；
3. 多内核汇聚符号；
4. 可控网络门符号。

用户确认方向后，才生成并替换 EXE、任务栏、标题栏和托盘的多尺寸资源。

## 8. 影响文件与回滚

- 后端：`internal/ipdetect/echo.go`、`internal/service/service.go`、`navo_app/app.go`
- 状态与 API：`frontend/src/state.ts`、`types.ts`、`api.ts`、`env.d.ts`
- UI：`frontend/src/App.vue`、`components/StateGlyph.vue`、`components/TrafficChart.vue`、`style.css`

回滚时可按上述三组分别撤销。IP 字段是向后兼容的 JSON 扩展；`CheckIP` 是新增方法，不改变现有 IPC method。

## 9. 验证

- `vue-tsc --noEmit` 与 Vite production build；
- Edge/Playwright 真实渲染：8 个页面导航；
- 1440×900 和 900×700 两种桌面视口；
- 控制台 error 为 0；
- 900px 视口无水平溢出；
- packaged lightweight smoke 验证 sing-box Metrics Adapter 可用、Dashboard 低延迟以及 launcher/core 干净退出；
- 截图：`release/navo-ui-refactor.png`、`release/navo-ui-refactor-compact.png`。
