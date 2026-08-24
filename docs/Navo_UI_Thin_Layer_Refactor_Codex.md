# Navo UI 薄层化重构任务书（Codex 执行版）

> 目标版本：Navo 1.x 重构阶段  
> 任务类型：架构重构 / UI 解耦 / 不改变既有业务行为  
> 核心原则：**UI 只负责表达用户意图与展示系统状态，不负责业务决策和系统控制。**

---

## 1. 任务背景

Navo 当前已经形成比较明确的后端架构，包括：

- `internal/service/`：业务调度与运行期管理
- `internal/agent/`：Windows 本机网络执行
- `internal/network/`：TUN、Route、DNS、网络资源管理
- `internal/connection/`：统一连接事务 / 并发协调
- `internal/supervisor/`：代理内核进程生命周期
- `internal/selfheal/`：故障检测与自动恢复
- `internal/compiler/`、`internal/coreadapter/`：内核配置与适配
- `navo_app/`：Wails 应用与 Vue 前端

当前主要问题不是总体架构错误，而是**UI 层承担的内容过多、前端文件过大、展示逻辑、交互逻辑、业务流程和后端调用过度聚集**。

本次重构不得推翻现有 Connection Coordinator、Capture Transaction、Network Journal、Rollback、SelfHeal、Supervisor 等核心机制。

本任务的目标是：

> **保留现有后端能力，将 UI 收缩成真正的“交互层 / 展示层”。**

---

# 2. 最终架构原则

以后 Navo 的调用关系必须尽量符合：

```text
用户
 │
 ▼
UI
 │
 │  表达“用户想做什么”
 ▼
Application / Wails API Facade
 │
 ▼
Service / Connection Coordinator
 │
 ├── Agent
 ├── Network
 ├── Supervisor
 ├── SelfHeal
 └── Core Adapter
 │
 ▼
Windows / Proxy Core

反向：

后端真实状态
 │
 ▼
Application / API DTO
 │
 ▼
UI
 │
 ▼
用户可理解的状态、提示、图表
```

---

# 3. UI 的职责边界

UI **只允许负责以下内容**。

## 3.1 用户交互

例如：

- 点击按钮
- 选择节点
- 选择内核
- 开关 TUN
- 开关 System Proxy
- 选择路由模式
- 输入订阅地址
- 编辑表单
- 打开/关闭弹窗
- 切换页面、Tab、折叠面板

---

## 3.2 前端级输入检查

UI 可以做即时体验型校验，例如：

- 输入不能为空
- URL 看起来是否合法
- 必填字段是否填写
- 数值是否超出输入框允许范围

但：

> **UI 校验不能成为最终业务规则。**

后端必须继续做真正的业务校验。

例如：

```text
UI:
“订阅地址格式看起来不正确”
```

可以存在。

但：

```text
Xray 是否允许某种 Capture Mode
当前系统是否允许切换 TUN
当前事务是否允许启动
当前内核是否健康
路由配置是否合法
```

这些必须以后端判断为准。

---

## 3.3 将用户行为转换为标准意图

例如用户点击：

```text
开启 TUN
```

UI 应尽量只转换为类似：

```text
SwitchCaptureMode(TUN)
```

或者：

```text
CaptureSwitchRequest {
    target: "tun"
}
```

然后交给后端。

UI 不应知道完成 TUN 切换需要多少步骤。

---

## 3.4 展示后端状态

例如后端返回：

```text
phase = verifying
```

UI 可以转换成人类能理解的内容：

```text
正在检测网络连接……
```

后端返回：

```text
fault = core_start_failed
```

UI 可以展示：

```text
代理内核启动失败，网络状态正在恢复。
```

UI 可以负责：

- 文案
- 图标
- Loading
- Toast
- Modal
- 状态颜色
- 图表
- 动画
- 页面布局

但不能自行“修复”后端状态。

---

# 4. 判断一段代码是否属于 UI 的规则

Codex 在迁移代码时必须使用下面这条判断标准：

> **如果完全删除 GUI，改成 CLI，这段逻辑是否仍然必须存在？**

如果答案是“是”，那么这段逻辑原则上就不应该属于 Vue UI。

---

## 4.1 必须移出 UI 的典型逻辑

以下类型禁止继续长期存在于 Vue 页面层：

- 内核启动/停止流程
- 内核切换流程
- 节点切换业务流程
- TUN 完整切换步骤
- System Proxy 完整切换步骤
- Route 修改逻辑
- DNS 修改逻辑
- Wintun 生命周期逻辑
- rollback 决策
- SelfHeal 决策
- 网络恢复决策
- Core compatibility 判断的权威实现
- 故障分类
- 数据面健康判定
- 事务串行化
- 并发冲突处理
- 配置编译
- 核心进程管理
- “失败后应该执行什么”的业务决策

---

## 4.2 可以继续留在 UI 的逻辑

例如：

- 当前 Tab
- 下拉框是否展开
- Modal 是否显示
- 表单草稿
- hover 状态
- Loading 表现
- Toast 内容
- 图表采样后的视觉转换
- 用户可读文案
- CSS class
- ViewModel
- 当前页面选择
- 纯展示排序/过滤
- 不具有业务权威性的格式化

---

# 5. 本次重构的核心目标

## 目标 A：缩小 `App.vue`

当前 `App.vue` 过大。

重构后：

```text
App.vue
```

只能承担：

- 应用 Shell
- 顶层布局
- 页面/模块组合
- 少量全局生命周期绑定
- 全局错误展示入口
- 必须的顶层状态注入

禁止继续在 `App.vue` 中：

- 编写完整业务操作流程
- 堆积所有页面模板
- 保存所有业务状态
- 编写大量 API 调用
- 编写复杂数据转换
- 编写几十个互相依赖的事件处理函数

### 建议目标

尽量将：

```text
App.vue <= 300~400 行
```

如果实际架构导致略高，可以合理放宽，但必须确保它已经成为“应用壳层”，而不是业务巨型组件。

不要为了满足行数指标机械拆文件。

---

# 6. 前端推荐目录结构

Codex 必须先检查现有 `navo_app/frontend/src/`，在不破坏现有构建方式的前提下逐步调整。

推荐方向：

```text
navo_app/frontend/src/
├── App.vue
├── api/
│   ├── client.ts
│   ├── capture.ts
│   ├── core.ts
│   ├── node.ts
│   ├── subscription.ts
│   ├── diagnostics.ts
│   └── system.ts
│
├── components/
│   ├── common/
│   ├── status/
│   ├── traffic/
│   └── ...
│
├── features/
│   ├── capture/
│   │   ├── CapturePanel.vue
│   │   ├── useCapture.ts
│   │   ├── model.ts
│   │   └── presenter.ts
│   │
│   ├── core/
│   │   ├── CorePanel.vue
│   │   ├── useCore.ts
│   │   └── model.ts
│   │
│   ├── nodes/
│   ├── subscriptions/
│   ├── routing/
│   ├── diagnostics/
│   └── traffic/
│
├── state/
│   ├── runtime.ts
│   ├── preferences.ts
│   └── ui.ts
│
├── types/
├── utils/
└── styles/
    ├── tokens.css
    ├── base.css
    ├── layout.css
    ├── components.css
    └── ...
```

此目录只是目标方向。

**不得为了完全照抄目录而破坏现有代码。**

Codex 应按照现有项目实际内容进行适配。

---

# 7. State 必须拆成两种

这是本次重构的重要原则。

## 7.1 Domain / Runtime State

来源于后端真实状态，例如：

```text
当前 Core
当前 Node
Capture Mode
运行阶段
Core Health
网络验证结果
SelfHeal 状态
连接事务状态
代理端口
当前出口 IP
```

这些状态的**真相源必须是后端**。

UI 不得自行维护另一套“猜测状态”。

例如禁止形成：

```text
UI 认为 TUN 已经开启
后端实际上正在 rollback
```

---

## 7.2 UI State

只属于前端：

```text
当前 Tab
Modal 是否开启
Sidebar 是否折叠
输入框草稿
Loading 显示
图表视觉范围
选中的视觉项
Toast 队列
```

UI State 可以完全由前端控制。

---

# 8. 禁止使用“乐观业务状态”覆盖真实状态

例如用户点击：

```text
开启 TUN
```

禁止 UI 立刻：

```ts
captureMode.value = "tun"
```

并把它当成真实结果。

正确方式应类似：

```text
用户点击
 ↓
UI 设置 operationPending = true
 ↓
调用后端
 ↓
后端进入 PREPARING / APPLYING / VERIFYING
 ↓
UI 根据后端 Snapshot 展示
 ↓
后端 COMMITTED
 ↓
真实 Capture Mode 变成 TUN
```

或者：

```text
失败
 ↓
ROLLING_BACK
 ↓
恢复旧状态
 ↓
UI 展示真实结果
```

---

# 9. API / Wails Bridge 设计原则

前端不应该到处直接调用 Wails 绑定。

需要形成统一 API 层。

例如：

```ts
// api/capture.ts

export function switchCaptureMode(mode: CaptureMode) {
    return backend.SwitchCaptureMode(mode)
}
```

组件：

```ts
await switchCaptureMode("tun")
```

而不是每个组件都知道后端绑定细节。

---

# 10. Wails 后端入口也应逐渐薄化

如果当前 `navo_app/app.go` 或其他 Wails binding 文件存在较大的业务流程，也应逐步拆开。

目标：

```text
Vue
 ↓
Wails Facade
 ↓
Application / Service
 ↓
Domain / Agent / Network
```

Wails API 方法应该尽量像：

```go
func (a *App) SwitchCaptureMode(
    ctx context.Context,
    req SwitchCaptureModeRequest,
) (RuntimeSnapshot, error) {
    return a.runtime.SwitchCaptureMode(ctx, req)
}
```

而不是：

```go
func (a *App) SwitchCaptureMode(...) {
    // 检查几十种状态
    // 停核心
    // 改 DNS
    // 改 Route
    // 创建网卡
    // rollback
    // 再启动……
}
```

---

# 11. 后端核心权责不允许被前端复制

以下模块继续作为核心权威：

```text
internal/connection/
    → 连接事务、单写者、并发协调

internal/service/
    → 业务调度

internal/agent/
    → 用户会话和 Windows 网络控制

internal/network/
    → Route / DNS / TUN / 网络资源

internal/supervisor/
    → Core 生命周期

internal/selfheal/
    → 故障检测和恢复

internal/compiler/
internal/coreadapter/
    → Core 配置与兼容适配
```

如果 UI 当前已经实现了与这些模块重复的规则：

> **删除 UI 规则，保留后端规则，并通过状态/DTO 将结果反馈给 UI。**

---

# 12. 错误处理规范

禁止页面自己根据字符串猜业务结果，例如：

```ts
if (err.includes("tun")) ...
```

优先由后端返回结构化错误，例如：

```text
code
category
message
recoverable
operation
phase
```

前端只负责：

```text
结构化错误
 ↓
用户可读文案
```

如果本次重构暂时无法一次性修改所有错误类型，可以先建立统一的错误适配器，并保留兼容路径。

不得因此大范围重写现有后端错误体系。

---

# 13. 状态展示应基于 Snapshot / Event

UI 应尽量消费：

```text
RuntimeSnapshot
ConnectionSnapshot
CaptureSnapshot
CoreState
SelfHealState
TrafficSnapshot
```

而不是自己根据多个变量推断真实系统状态。

例如：

```text
“正在启动”
“正在切换”
“正在验证”
“正在回滚”
“运行正常”
“故障”
```

应该尽量来自后端真实阶段。

---

# 14. 样式文件拆分

当前全局 `style.css` 过大时，需要逐步拆分。

建议：

```text
styles/
├── tokens.css
├── reset.css
├── base.css
├── layout.css
├── forms.css
├── panels.css
├── dialogs.css
├── status.css
└── ...
```

同时：

- 页面私有样式优先跟随组件
- 公共视觉 token 统一管理
- 禁止仅为了“拆文件”大量复制 CSS
- 禁止产生新的样式覆盖地狱
- 保持当前 UI 外观基本不变

---

# 15. 组件拆分原则

不要按照“每 100 行一个组件”机械拆分。

按照**业务视觉区域 / 交互职责**拆。

例如：

```text
Dashboard
├── RuntimeOverview
├── ExitIPCard
├── TrafficPanel
├── CoreStatus
└── CaptureStatus

ProxyControl
├── CoreSelector
├── NodeSelector
├── CaptureModeSelector
└── RoutingModeSelector

Subscription
├── SubscriptionList
├── SubscriptionEditor
└── SubscriptionStatus
```

一个组件应尽量：

> “可以用一句话说明它负责什么。”

---

# 16. Composable 的职责

如使用 Vue composable：

```text
useCapture()
useCore()
useNodes()
useSubscriptions()
useDiagnostics()
```

它们可以：

- 调用前端 API adapter
- 管理该 Feature 的 pending 状态
- 组织 ViewModel
- 格式化数据
- 管理页面级交互

但不允许把后端业务逻辑偷偷搬到 composable。

例如禁止：

```text
useCapture()
  ├─ 停旧内核
  ├─ 修改 Route
  ├─ 修改 DNS
  ├─ 创建 TUN
  └─ 自己 rollback
```

正确：

```text
useCapture()
  └─ api.switchCaptureMode("tun")
```

---

# 17. 本次重构明确“不做”的事情

为了降低风险，本轮不要同时做以下工作：

- 不重新设计 TUN 状态机
- 不重新设计 Connection Coordinator
- 不替换 Wintun
- 不重新设计 SelfHeal 算法
- 不重写 Core Supervisor
- 不修改代理协议
- 不大规模更换 UI 设计
- 不加入大型新功能
- 不修改现有网络行为语义
- 不因为重构顺手修改大量无关代码

本轮主题只有：

> **UI 薄层化 + 前端模块化 + Wails 入口职责清晰化。**

---

# 18. Codex 执行步骤

---

## Phase 0：建立重构基线

先执行现有测试和构建。

至少执行：

```powershell
go test ./...
go vet ./...
```

进入前端目录后执行项目现有脚本：

```powershell
npm ci
npm test
npm run typecheck
npm run build
```

如果实际 `package.json` 脚本名称不同，以仓库实际定义为准。

记录：

- 当前失败项
- 当前 warning
- 当前测试数量
- 当前 build 状态

不要把重构前已经存在的问题误判为本次重构导致。

---

## Phase 1：审计 `App.vue`

对 `App.vue` 中现有内容分类。

至少分类为：

```text
A. Layout / Template
B. UI State
C. Runtime State
D. Backend API Call
E. Business Decision
F. Formatting
G. Dialog / Toast
H. Traffic / Chart
I. Subscription
J. Core
K. Capture
L. Routing
M. Diagnostics
```

输出一个迁移清单。

先分类，再改代码。

---

## Phase 2：建立统一 API 层

把散落的 Wails 调用集中至：

```text
src/api/
```

或者现有项目更合理的位置。

原则：

```text
Vue Component
  ↓
Feature / Composable
  ↓
API Adapter
  ↓
Wails Binding
```

不得改变后端 API 行为。

---

## Phase 3：拆 Feature

优先顺序：

1. Capture
2. Core
3. Node
4. Routing
5. Subscription
6. Traffic
7. Diagnostics
8. General Runtime Overview

每拆一个 Feature：

- 保持 UI 一致
- 保持后端调用一致
- 增加或迁移测试
- 执行前端测试

不要一次性重写整个前端。

---

## Phase 4：拆 Runtime State 与 UI State

确保：

```text
后端运行状态
```

与：

```text
前端视觉状态
```

不再混为一体。

如现有 `state.ts` 承担过多职责，应拆分。

---

## Phase 5：缩小 `App.vue`

完成后 `App.vue` 应主要剩：

- Application Shell
- Feature Composition
- Top-level lifecycle
- 顶层状态入口
- 全局提示入口

业务处理函数应基本消失。

---

## Phase 6：检查 Wails Facade

检查：

```text
navo_app/app.go
navo_app/*.go
```

如果其中有明显属于：

```text
service
agent
network
supervisor
selfheal
```

的业务流程，则将实现委托回正确层。

本阶段要求：

> **仅处理明显越界逻辑，不进行大规模后端重写。**

---

## Phase 7：CSS 模块化

在功能稳定后拆样式。

不要在业务重构同时大改视觉效果。

---

## Phase 8：测试与回归

重新执行：

```powershell
go test ./...
go vet ./...
```

前端：

```powershell
npm test
npm run typecheck
npm run build
```

如果项目有其他 CI 命令，也必须执行。

---

# 19. 必须补充的测试

本轮至少补以下类型测试。

## 19.1 UI State 测试

例如：

- Modal
- Tab
- Close preference
- Loading
- 纯展示状态

---

## 19.2 Presenter / Mapping 测试

例如：

```text
phase=verifying
→ “正在验证网络”
```

```text
phase=rolling_back
→ 回滚状态显示
```

重点验证：

> 后端状态到 UI ViewModel 的映射。

---

## 19.3 API Adapter 测试

验证：

```text
Feature
→ 调用了正确的 Wails API
```

禁止在前端测试里重新实现后端网络逻辑。

---

# 20. 重构完成后的理想调用示例

## 开启 TUN

### UI

```text
用户点击 TUN
 ↓
CapturePanel
 ↓
useCapture.switchMode("tun")
 ↓
captureApi.switchMode("tun")
```

### 后端

```text
Wails API
 ↓
Service / Application
 ↓
ConnectionCoordinator
 ↓
Capture Transaction
 ↓
Agent / Network
 ↓
Verify
 ↓
Commit / Rollback
```

### UI 返回

```text
Runtime Snapshot
 ↓
Presenter
 ↓
CapturePanel
 ↓
用户看到：
“运行中”
或
“切换失败，已恢复”
```

---

# 21. 非常重要：UI 不得猜系统状态

禁止这种模式：

```ts
await startTun()
isTunEnabled.value = true
```

正确方式：

```ts
await requestSwitchToTun()
await refreshRuntimeSnapshot()
```

或者通过后端事件更新。

最终：

> **UI 展示的是后端事实，而不是 UI 自己认为发生了什么。**

---

# 22. 未来扩展要求

这次重构完成后，Navo 的核心后端应该能够自然支持：

```text
Windows GUI
Tray
CLI
未来 Android 控制端
未来其他客户端
```

也就是说：

> 业务逻辑不能依赖 Vue 才能成立。

未来即使实现：

```powershell
navo capture tun
```

也应该调用与 GUI 相同的核心业务能力。

---

# 23. 代码质量要求

Codex 修改代码时必须：

- 保持高内聚、低耦合
- 不制造循环依赖
- 不复制后端业务规则
- 不增加无意义抽象
- 不为了“架构漂亮”增加过多 wrapper
- 不一次性重写整个项目
- 保持兼容现有行为
- 新增公共函数必须职责单一
- 关键状态转换必须可测试
- 错误必须继续向上返回
- 不静默吞掉异常
- 不改变现有安全边界
- 不降低现有 rollback / self-heal 能力

---

# 24. Codex 每阶段提交要求

建议按阶段提交，而不是一次巨大提交。

例如：

```text
refactor(ui): introduce frontend api adapters

refactor(ui): extract capture feature

refactor(ui): extract core and node features

refactor(ui): split runtime and presentation state

refactor(ui): reduce App.vue to application shell

refactor(ui): modularize frontend styles

test(ui): add state and presenter coverage
```

如果当前仓库工作区已有用户未提交修改：

> **不得覆盖、删除或重置用户已有修改。**

禁止未经确认执行：

```text
git reset --hard
git clean -fd
```

---

# 25. 最终验收标准

完成本轮重构后，必须满足：

- [ ] `App.vue` 已成为应用壳层，而不是业务巨型组件
- [ ] Vue 页面中不存在 TUN/Route/DNS/Wintun 的真实控制流程
- [ ] Vue 页面中不存在 rollback / SelfHeal 业务决策
- [ ] Core 启停和切换流程继续由后端负责
- [ ] Connection Coordinator 继续作为连接变更统一协调者
- [ ] 后端 Runtime State 是系统真实状态来源
- [ ] UI State 与 Runtime State 已分离
- [ ] Wails 调用已集中到统一 API Adapter
- [ ] Feature 已按职责拆分
- [ ] 大型 CSS 已开始模块化
- [ ] 当前 UI 主要视觉和交互没有无关变化
- [ ] 现有核心网络行为没有发生语义变化
- [ ] Go 测试通过
- [ ] Go Vet 通过
- [ ] 前端测试通过
- [ ] TypeScript 类型检查通过
- [ ] 前端构建通过
- [ ] 没有删除用户已有能力
- [ ] 没有新增前端版“第二套业务逻辑”

---

# 26. Codex 执行时的强制检查问题

每迁移一段代码，必须问：

### 问题 1

```text
这段代码如果未来使用 CLI，还需要吗？
```

需要：

```text
→ 应属于后端 / Application / Domain
```

不需要：

```text
→ 可以属于 UI
```

### 问题 2

```text
这段代码是在“决定系统应该怎么做”
还是“决定界面应该怎么显示”？
```

前者：

```text
→ 后端
```

后者：

```text
→ UI
```

### 问题 3

```text
这个状态是谁的真相？
```

如果是：

```text
Core / Network / Capture / SelfHeal / Connection
```

则：

```text
→ 后端是真相源
```

如果是：

```text
Modal / Tab / Hover / Layout / Form Draft
```

则：

```text
→ 前端是真相源
```

---

# 27. 最终目标

本次重构不是单纯为了：

```text
把一个大 Vue 文件拆成很多小 Vue 文件。
```

真正目标是：

```text
让 Navo 本身成为独立的网络控制系统，
而 GUI 仅仅成为 Navo 的一个客户端。
```

最终应达到：

```text
UI = 用户意图输入 + 系统状态展示

Backend = 业务规则 + 状态管理 + 网络控制 + 故障恢复
```

这就是本次重构的核心定义。

---

# 28. Codex 最终输出要求

Codex 完成工作后必须给出：

1. 修改文件列表
2. 新增文件列表
3. 删除文件列表
4. `App.vue` 重构前后职责变化
5. 哪些业务逻辑从 UI 移出了
6. 哪些逻辑仍留在 UI，以及理由
7. Wails API 是否发生变化
8. 后端业务行为是否发生变化
9. 测试执行结果
10. 构建结果
11. 尚未完成的技术债
12. 下一阶段建议

如果发现某段逻辑无法确定应该属于 UI 还是后端：

> 优先保持现状，记录为技术债，不要擅自重写核心行为。

---

## 最后一句原则

> **UI 可以决定“怎么告诉用户”，但不能决定“Windows 网络应该怎么运行”。**
