下面这份不是给产品看的，而是给 **Codex/开发 Agent 执行用** 的。重点是告诉它：

- 这个功能是什么；

- 现有架构应该怎么接；

- 数据从哪里来；

- 不允许怎么实现；

- 开发顺序是什么。

  ------

  

```
# Navo System Tray Implementation Specification

## 1. 目标

重构 Navo Windows 系统托盘功能。

当前托盘功能过于简单：
```

打开
退出

```
无法满足多内核、多线路、多模式管理需求。

新的托盘必须成为：

> Navo 快速状态查看和代理控制入口

但不是代理核心控制层。

---

# 2. 架构约束

## 2.1 托盘禁止直接操作核心

System Tray 不允许：

- 直接启动 mihomo.exe
- 直接启动 xray.exe
- 直接启动 sing-box.exe
- 直接生成核心配置
- 直接修改 SQLite
- 直接设置 Windows 系统代理
- 直接操作 TUN
- 直接修改路由


所有操作必须经过：
```

Tray UI

↓

navo-agent

↓

IPC

↓

navo-service

↓

core-host

↓

proxy core

```
---

# 3. 核心状态来源

托盘显示的数据必须来自 Service RuntimeState。

禁止：
```

Tray 自己维护状态

```
禁止：
```

点击按钮 = 已连接

```
真实状态必须来自：
```

Core Runtime
+
Health Check
+
Network Probe
+
Capture State

```
---

# 4. 托盘菜单结构

最终结构：
```

Navo

├── 状态
│
├── 连接控制
│
├── 线路选择
│
├── 内核模式
│
├── 流量接管
│
├── 规则模式
│
├── 诊断工具
│
├── 设置
│
└── 退出

```
---

# 5. 状态菜单

## 目的

显示当前真实运行状态。


示例：
```

状态

🟢 已连接

核心:
sing-box

线路:
机场-US-LosAngeles-01

接管:
系统代理

规则:
规则代理

出口:
US Residential

延迟:
120ms

```
---

异常：
```

🔴 连接失败

原因:
配置校验失败

操作:
查看日志
重新连接
恢复网络

```
---

状态来源：

Backend:
```

RuntimeState
CoreRuntime
ActiveSelection
HealthResult

```
---

# 6. 连接控制

菜单：
```

连接控制

├── 开启代理
├── 关闭代理
└── 重启核心

```
---

## 6.1 开启代理

调用：
```

Connect()

```
流程：
```

读取 ActiveSelection

↓

检查:

CoreType
SourceType
Endpoint
Compatibility

↓

生成配置

↓

核心配置校验

↓

启动核心

↓

等待端口

↓

健康检查

↓

应用系统代理/TUN

↓

更新 RuntimeState

```
---

## 6.2 关闭代理

注意：

关闭代理 != 退出程序


流程：
```

DisableCapture

↓

StopCore

↓

ReleasePort

↓

RestoreNetwork

```
---

## 6.3 重启核心

流程：
```

StopCore

↓

CompileConfig

↓

ValidateConfig

↓

StartCore

↓

HealthCheck

```
---

# 7. 线路选择

## 对应后端模型
```

SourceType
+
Endpoint

```
支持：
```

机场订阅

或者

上游代理

```
二选一。


---

# 7.1 菜单结构
```

线路选择

├── 机场订阅
│
│   ├── Provider A
│   │
│   │   ├── Node-01 🟢
│   │   ├── Node-02 🟢
│   │   └── Node-03 🔴
│
│
└── 上游代理
    │
    ├── Proxy-A
    │
    │   ├── Endpoint-01 🟢
    │   └── Endpoint-02 🔴

```
---

# 7.2 节点颜色状态

节点状态必须动态计算。


## 绿色

表示：
```

Protocol supported

- 

Current Core supported

- 

Config compile success

- 

Connection test success

```
显示：
```

Node-01 🟢

```
---

## 红色

表示不可使用。


例如：
```

Node-03 🔴

```
展开：
```

原因:

Core:
sing-box

Protocol:
TUIC

Error:
unsupported protocol

```
---

# 7.3 节点状态模型

新增：

```go
type EndpointStatus struct {

    EndpointID string

    Available bool

    Color string

    Reason string

    CheckedAt time.Time

}
```

------

# 8. 内核模式

对应：

```
CoreType
```

菜单：

```
内核模式

├── Mihomo
├── Xray
└── sing-box
```

------

状态：

```
Mihomo 🟢

Xray 🟢

sing-box 🟢
```

颜色含义：

## Green

```
Installed

Hash valid

Compatible
```

## Yellow

```
Installed

But current endpoint incompatible
```

## Red

```
Missing

or invalid
```

------

切换核心：

调用：

```
SwitchCore(CoreType)
```

流程：

```
Check Compatibility

↓

Compile New Config

↓

Validate Config

↓

Stop Old Core

↓

Start New Core

↓

Health Check

↓

Commit
```

失败：

```
Rollback Previous Core
```

------

# 9. 流量接管

对应：

```
CaptureMode
```

菜单：

```
流量接管

├── 关闭
├── 系统代理
└── TUN
```

------

# 9.1 关闭

状态：

```
No Traffic Capture
```

------

# 9.2 系统代理

显示：

```
系统代理 🟢


HTTP:
127.0.0.1:7890

SOCKS:
127.0.0.1:7891
```

调用：

```
ApplySystemProxy()
```

------

# 9.3 TUN

显示：

```
TUN 🟢

Adapter:
Navo-TUN

DNS:
Enabled
```

调用：

```
EnableTun()
```

------

# 10. 规则模式

对应：

```
RouteMode
```

菜单：

```
规则模式

├── 全局代理
├── 规则代理
└── 全局直连
```

------

# 10.1 全局代理

所有流量：

```
Proxy
```

------

# 10.2 规则代理

默认模式：

```
Rule Match

China:
DIRECT

Foreign:
Proxy
```

------

# 10.3 全局直连

所有：

```
DIRECT
```

用途：

- 网络测试
- 排查代理问题

------

# 11. 诊断工具

菜单：

```
诊断工具

├── 测试当前线路
├── 测试出口IP
├── 查看核心日志
├── 查看连接日志
├── 网络恢复
└── 导出诊断包
```

------

# 12. IPC API需求

新增 Tray 需要以下 Service API。

## 获取状态

```
GetRuntimeState()
```

返回：

```
type RuntimeState struct {

    Status string

    CoreType CoreType

    SourceType SourceType

    EndpointID string

    CaptureMode CaptureMode

    RouteMode RouteMode

    ExitIP string

    Latency int

}
```

------

## 获取线路列表

```
ListEndpoints()
```

返回：

```
type EndpointView struct {

    ID string

    Name string

    Provider string

    Available bool

    Reason string

}
```

------

## 切换线路

```
SwitchEndpoint(endpointID)
```

------

## 切换核心

```
SwitchCore(coreType)
```

------

## 切换接管模式

```
SwitchCaptureMode(mode)
```

------

## 切换规则模式

```
SwitchRouteMode(mode)
```

------

# 13. 数据刷新机制

托盘不能频繁轮询。

推荐：

```
Service

↓

Event Stream

↓

Agent

↓

Tray
```

事件：

```
CoreStarted

CoreStopped

EndpointChanged

HealthChanged

IPChanged

CaptureChanged

ErrorOccurred
```

------

# 14. 开发顺序

## Phase 1

完成：

- RuntimeState
- Tray 状态显示
- 开关代理

------

## Phase 2

完成：

- Endpoint 列表
- 节点颜色
- 节点切换

------

## Phase 3

完成：

- 三核心切换

------

## Phase 4

完成：

- System Proxy
- TUN

------

## Phase 5

完成：

- RouteMode
- 诊断工具

------

# 15. 禁止事项

禁止：

- Tray 自己保存代理状态
- Tray 自己判断节点可用
- Tray 自己解析订阅
- Tray 自己生成配置
- Tray 自己启动核心
- Tray 使用假状态
- 节点失败不显示原因
- 自动切换用户未选择核心
- 自动切换用户未选择线路

------

# 16. 完成标准

Tray 完成后必须满足：

```
可以查看真实连接状态

可以关闭/开启代理

可以选择机场节点

可以选择上游代理

可以查看节点可用性

可以查看失败原因

可以切换 Mihomo/Xray/sing-box

可以切换系统代理/TUN

可以切换规则模式

异常可以恢复网络

所有状态与 Service RuntimeState 一致
END
这份和前面的 **多内核架构文档** 是一套的。

前一个解决：
> “Navo 怎么运行”

这个解决：
> “用户怎么控制 Navo”

Codex 看到后应该不会再把托盘写成一个简单右键菜单，而会把它当成 **Service 状态的可视化控制层**。
```