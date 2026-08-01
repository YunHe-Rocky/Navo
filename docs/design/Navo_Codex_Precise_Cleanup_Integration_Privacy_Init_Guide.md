# Navo 精准清理、局部模块化与隐私初始化作业指导书（Codex 修订版）

> 适用仓库：`YunHe-Rocky/Navo`  
> 执行主体：Codex  
> 目标平台：Windows 10 / Windows 11  
> 本文档状态：**替代并废止上一份《Navo 本地化重构与隐私初始化作业指导书》**  
> 本文档不是独立重写项目，必须与以下两份文档配合执行：
>
> 1. `Navo_Codex_Full_Remediation_Guide.md`
> 2. `Navo_Codex_Feature_Optimization_Guide.md`

---

# 0. 本次修订的真实范围

本次任务只做四类事情：

```text
1. 删除 MySQL 对应的实现、依赖、配置和调用链；
2. 删除 AI 对应的实现、依赖、配置和调用链；
3. 清理项目中确认无法被调用、没有业务价值的死代码；
4. 增加必要的初始化与设备隐私清理模块。
```

除此以外：

```text
不得为了“架构更漂亮”而删除现有业务；
不得为了“全部本地化”而删除正常联网功能；
不得无理由重写所有目录；
不得删除前两份文档要求修复或新增的功能；
不得把当前暂未接通但前两份文档明确要求实现的功能当成死代码。
```

保留的正常业务包括但不限于：

```text
系统代理
TUN
sing-box
Mihomo
Xray
订阅获取
节点解析
节点切换
本地代理
核心升级
一键测速
流量监测
日志
IP 检测
IP 风险查询
托盘
任务栏和开始菜单图标
网络恢复
核心崩溃恢复
本地配置
必要的 HTTPS 第三方查询
```

---

# 1. 三份文档的优先级与配合关系

## 1.1 第一份文档：故障与安全修复

文件：

```text
Navo_Codex_Full_Remediation_Guide.md
```

它仍然是以下内容的主要执行依据：

```text
TUN 路由
系统代理事务
Capture 状态机
核心生命周期
Host/Supervisor
Service Pipe
网络恢复
订阅安全
配置校验
原子持久化
IPC
Windows 测试
发布门禁
```

本修订文档不能取消这些修复。

## 1.2 第二份文档：产品功能优化

文件：

```text
Navo_Codex_Feature_Optimization_Guide.md
```

它仍然是以下内容的主要执行依据：

```text
升级内核
一键测速
四条流量曲线
监测属性选择
虚拟流量模拟
图表 Tooltip
日志筛选
双联链路
IP 风险摘要
隐藏滚动条
托盘、任务栏、开始菜单和应用图标
```

本修订文档不能把这些暂未实现的功能当成死代码删除。

## 1.3 本文档：精准删除与初始化

本文只覆盖：

```text
MySQL 移除
AI 移除
死代码确认和清理
必要的局部模块化
设备初始化
新设备隐私清理
遗留 MySQL/AI 数据清理
```

## 1.4 冲突处理规则

出现冲突时按以下规则处理：

### AI 冲突

第一份文档中如果要求：

```text
加强 AI 隐私
限制 AI 响应大小
校验 AI 返回规则
修改 AI 诊断名称
```

全部改为：

```text
彻底删除 AI 功能和调用链
```

不再投入成本修复即将删除的 AI 代码。

### MySQL/数据库冲突

第一份文档中如果提到：

```text
MySQL
数据库事务
本地与数据库一致性
revision 数据库提交
selection 数据库提交
```

处理方式改为：

```text
保留 revision、selection、配置版本等业务概念；
删除 MySQL 实现；
将必要数据落到本地 repository；
使用本地事务式写入保证一致性。
```

不能因为删除 MySQL，就把 revision、selection 或运行时回滚业务一起删除。

### “当前无法调用”冲突

如果某模块当前没有调用方，但第二份文档明确要求它在后续接入，例如：

```text
核心升级
代理流量采集
双联链路
IP 风险摘要
日志日期筛选
托盘状态切换
```

它属于：

```text
待接入业务
```

不是死代码，不得删除。

### 旧第三份文档冲突

上一份：

```text
Navo_Codex_Local_First_Modularization_Privacy_Init_Guide.md
```

不再执行。

以本文档为准，禁止按照旧文档进行全量目录重写或过度删除。

---

# 2. 推荐执行顺序

为避免业务遗漏，Codex 按以下顺序执行。

## 阶段 A：建立完整调用清单

先不删除任何代码。

输出：

```text
docs/CODE_REACHABILITY_AND_REMOVAL_PLAN.md
```

记录：

```text
所有 MySQL 文件和入口
所有 AI 文件和入口
所有 IPC 方法
所有 Wails 方法
所有菜单和页面
所有后台 goroutine
所有 repository 实现
所有未被调用函数
所有 build-tag 文件
所有通过注册表、map、init()、反射或字符串分发调用的代码
```

## 阶段 B：删除 AI

因为 AI 与核心网络业务耦合较低，应优先完整删除。

## 阶段 C：删除 MySQL并保留必要业务

把 MySQL 承担的必要状态转换为本地持久化。

## 阶段 D：增加初始化和设备隐私清理

初始化模块只负责设备绑定、遗留清理和安全启动，不重写所有业务。

## 阶段 E：执行第一份文档的 P0/P1 修复

完成 TUN、状态机、系统代理、Supervisor、恢复等问题。

## 阶段 F：执行第二份文档的功能优化

完成升级核心、流量、日志、IP 风险等功能。

## 阶段 G：第二轮死代码清理

只有当前两份文档中的功能全部接通后，才能再次确认剩余不可达代码。

---

# 3. Codex 总执行原则

1. 先做调用图，后删除；
2. 删除必须覆盖实现、入口、配置、UI、测试和文档；
3. 不允许只隐藏页面；
4. 不允许只注释代码；
5. 不允许保留“以后可能使用”的 MySQL 或 AI 依赖；
6. 不允许删除前两份文档明确要求实现的业务；
7. 不允许把 Windows build-tag 代码误判为不可达；
8. 不允许把 IPC 字符串分发的方法误判为不可达；
9. 不允许把 Wails 动态绑定的方法误判为不可达；
10. 不允许把 `init()` 注册的实现误判为不可达；
11. 不允许删除故障恢复和回滚代码，只因为正常流程暂未调用；
12. 不允许删除测试辅助代码，只因为生产包不引用；
13. 不允许清理失败后继续加载旧敏感数据；
14. 所有删除都必须通过编译、测试和搜索复核；
15. 所有业务保留与删除结论必须写入报告。

---

# 4. 第一阶段：调用可达性与删除清单

优先级：P0

## 4.1 需要检查的入口

至少从以下入口建立调用链：

```text
cmd/navo/main.go
其他 cmd/*/main.go
Wails App 暴露方法
Agent IPC Dispatch
Service IPC Dispatch
系统托盘菜单
启动初始化
后台监测任务
Supervisor
CoreAdapter
订阅刷新
日志任务
核心升级任务
```

## 4.2 必须识别的动态调用

Go 的普通引用搜索不够，必须检查：

```text
switch method
map[string]Handler
反射调用
Wails Bind
init() 注册
接口注入
build tags
Windows 专用文件
测试专用 fake
JSON 方法名称
前端字符串形式 IPC
```

## 4.3 输出分类

每个可疑模块只能归入以下一类：

```text
A. 正常可达业务
B. 前两份文档要求后续接入
C. MySQL 专属代码，删除
D. AI 专属代码，删除
E. 已被新实现替代的旧代码
F. 完全不可达且无业务价值
G. 暂时无法确认，保留并继续调查
```

禁止把 `G` 直接删除。

## 4.4 可删除的证据要求

判定为 E 或 F 至少满足：

1. 所有生产入口均无调用；
2. 无动态注册；
3. 无字符串 IPC 调用；
4. 无 Wails 绑定；
5. 无 build-tag 平台用途；
6. 无第一、第二份文档中的计划用途；
7. 无迁移或恢复用途；
8. 删除后完整测试通过；
9. 搜索不到遗留配置或调用；
10. 在删除报告中说明替代实现或无业务原因。

---

# 5. 第二阶段：完整删除 AI 部分

优先级：P0

## 5.1 删除的后端范围

检查并删除等价代码：

```text
internal/ai/
internal/service/ai_settings.go
AI provider
AI client
AI diagnosis
AI rule generation
AI response parser
AI model selection
AI prompt builder
AI API key store
AI task/status
AI telemetry
```

已知可能涉及：

```text
internal/ai/ai.go
internal/ai/diagnosis.go
internal/ai/rulegen.go
internal/service/ai_settings.go
```

文件名变化时搜索等价实现。

## 5.2 删除 IPC 和 DTO

删除所有 AI 方法，例如：

```text
ai.settings.get
ai.settings.set
ai.provider.test
ai.diagnose
ai.rule.generate
ai.task.status
```

同时删除：

```text
dispatch case
request DTO
response DTO
事件
任务状态
错误码
前端客户端方法
Wails binding
```

## 5.3 删除 UI

删除：

```text
AI 设置
AI API Key
AI 服务商
AI 模型
AI 诊断
AI 规则生成
AI 状态卡
AI 菜单
AI 提示文案
```

删除后必须检查：

```text
菜单不会留下空入口
路由不会留下 404 页面
设置不会留下空白分组
前端不会继续请求已删除 IPC
```

## 5.4 保留本地诊断业务

如果现有 AI 模块中混入了有价值的本地诊断：

```text
核心状态
端口状态
TUN 状态
路由状态
DNS 状态
系统代理状态
日志错误分类
```

不要把这些本地规则一并删除。

处理方式：

```text
把与 AI 无关的纯本地诊断移动到现有诊断/网络模块；
去掉 AI 名称和 AI 依赖；
保留第一份文档需要的故障诊断价值。
```

最终名称可以是：

```text
本地诊断
网络诊断
运行状态检查
```

## 5.5 删除 AI 配置与敏感残留

初始化迁移时清除：

```text
AI API key
AI provider URL
AI model
AI prompt
AI request cache
AI response cache
AI task history
AI 诊断结果缓存
```

必须检查：

```text
普通设置 JSON
secure store
环境变量
注册表
日志
缓存目录
旧配置文件
```

## 5.6 删除依赖

从以下位置删除 AI SDK 或专属包：

```text
go.mod
go.sum
package.json
package-lock.json
构建脚本
环境变量模板
README
CI
```

只删除确认仅服务于 AI 的依赖。

## 5.7 与第二份文档的关系

第二份文档中的：

```text
IP 风险查询
核心更新
测速
订阅下载
```

属于普通 HTTPS 业务，不属于 AI，不得删除。

---

# 6. 第三阶段：完整删除 MySQL 部分

优先级：P0

## 6.1 删除 MySQL 专属范围

检查并删除：

```text
MySQL driver
database/sql 的 MySQL 打开逻辑
连接池
DSN
自动建表
migration
MySQL repository
MySQL health check
重连逻辑
MySQL 设置页
数据库状态卡
数据库日志
MySQL 环境变量
Docker MySQL
CI MySQL service
```

搜索：

```powershell
rg -n "mysql|go-sql-driver|gorm|sql\.Open|MYSQL_|DATABASE_URL|DSN" .
```

## 6.2 不删除业务模型

删除 MySQL 时必须保留仍有业务价值的概念：

```text
订阅
节点
当前节点选择
运行时状态
配置 revision
配置历史
核心选择
用户设置
流量偏好
日志偏好
初始化状态
```

这些不能因为之前由 MySQL 保存就一起删除。

## 6.3 本地替代原则

只替代原来确实依赖 MySQL 的必要数据。

优先复用项目已有本地存储：

```text
现有 JSON repository
现有 file store
现有 securestore
现有 runtime state
现有 journal
```

不要重复建立第二套本地存储。

仅当现有存储无法承担必要业务时，新增小型本地 repository。

## 6.4 不要求替换成 SQLite

本任务不是把 MySQL 换成另一个数据库。

除非现有数据模型确实需要事务查询，否则优先：

```text
结构化本地文件
原子替换
文件锁
版本字段
备份
```

禁止未经必要性分析直接引入 SQLite。

## 6.5 Revision 与 Selection

如果当前存在：

```text
revision repository
active selection
配置版本提交
```

必须保留其业务意义。

建议本地事务顺序：

```text
1. 生成候选 revision
2. 编译和原生校验
3. 应用运行时
4. 原子保存 revision
5. 原子更新 active selection
6. 提交 committed runtime
```

任一步失败：

```text
恢复旧运行时
保留旧 active selection
不得产生半提交状态
```

第一份文档中“本地与数据库状态一致性”改为：

```text
本地 revision、active selection 与 committed runtime 一致性。
```

## 6.6 MySQL 遗留配置清理

初始化迁移时清除：

```text
MYSQL_HOST
MYSQL_PORT
MYSQL_USER
MYSQL_PASSWORD
MYSQL_DATABASE
DATABASE_URL
DSN
数据库状态缓存
连接错误历史
旧数据库配置文件
```

如果 `.env` 包含其他正常配置：

```text
只能删除 MySQL 字段；
不能删除整个 .env；
不能误删代理、核心或其他功能配置。
```

## 6.7 删除依赖后复核

确认：

```text
go.mod 无 MySQL driver
主程序无 DB 连接
启动不等待数据库
UI 无数据库状态
断网时不因数据库失败而阻塞
```

---

# 7. 第四阶段：局部模块化，而非全量重写

优先级：P1

## 7.1 只整理被修改区域

允许重构：

```text
MySQL 删除后的 repository 边界
AI 删除后的本地诊断边界
初始化模块
设备身份模块
隐私清理模块
死代码清理后的公共接口
```

不要求：

```text
把整个仓库重写成全新分层；
一次移动所有文件；
重命名所有 package；
修改所有公共 API；
全面替换现有架构。
```

## 7.2 模块目标

至少保证：

```text
初始化不混入 UI；
设备身份不混入订阅；
隐私清理不混入核心升级；
本地 repository 不混入 Service Handler；
AI 删除后没有空壳；
MySQL 删除后没有 DB 命名残留。
```

## 7.3 Handler 保持轻量

修改涉及的 IPC/Wails Handler 应只负责：

```text
参数检查
调用业务服务
返回结果
```

不应在 Handler 中直接：

```text
删除文件
操作 DPAPI
扫描旧配置
修改 device-state
```

---

# 8. 第五阶段：增加最小必要初始化模块

优先级：P0

建议模块：

```text
internal/initialization/
```

可进一步分为：

```text
initializer.go
device_state.go
legacy_cleanup.go
privacy_reset.go
migration.go
```

不要借此移动全部现有业务代码。

## 8.1 初始化模块职责

只负责：

```text
确定本地数据目录
验证目录权限
读取数据版本
验证设备绑定状态
清理遗留 MySQL/AI 配置
检测配置是否来自其他设备/用户
执行必要隐私清理
创建缺失的默认配置
确认可以安全进入主程序
```

不负责：

```text
TUN 激活
系统代理启用
节点测速
核心升级
流量监测
日志查询
IP 风险查询
```

## 8.2 初始化必须早于业务加载

启动顺序：

```text
1. 单实例互斥
2. 确定数据目录
3. 初始化 bootstrap 日志
4. 读取 device-state
5. 验证设备绑定
6. 执行遗留清理/迁移
7. 初始化 repositories
8. 执行第一份文档要求的当前设备网络恢复
9. 创建 Agent/Service/UI
10. 进入主界面
```

注意：

```text
当前设备的网络恢复可以执行；
外部设备复制来的旧 network journal 不能执行。
```

## 8.3 初始化结果

建议：

```go
type Result struct {
    FirstRun       bool
    Migrated       bool
    ForeignContext bool
    PrivacyReset   bool
    Ready          bool
    ErrorCode      string
}
```

---

# 9. 第六阶段：设备绑定和新电脑隐私清理

优先级：P0

## 9.1 目的

当用户把 Navo 数据目录从电脑 A 复制到电脑 B，或复制到另一个 Windows 用户时：

```text
电脑 B 不得读取电脑 A 的订阅、节点凭据、日志和运行状态。
```

## 9.2 推荐判定方式

使用随机安装密钥配合 Windows DPAPI Current User Scope。

首次初始化：

```text
生成随机 install secret
使用 DPAPI Current User Scope 加密
保存 device-state.dat
保存 install ID、版本和校验值
```

验证时：

```text
DPAPI 可解密 + 完整性校验通过
    当前设备/用户上下文有效

DPAPI 无法解密
    判定为 foreign context
    执行隐私清理

可解密但完整性校验失败
    判定数据损坏或篡改
    停止正常启动并进入安全处理
```

不使用以下单一条件：

```text
电脑名
MAC 地址
磁盘序列号
主板序列号
CPU ID
```

这些可以作为非敏感诊断信号，但不能作为唯一判定。

## 9.3 需要自动清除的旧配置

foreign context 下清除：

```text
订阅 URL
订阅 token
节点配置
代理用户名密码
UUID 和私钥
上游代理凭据
当前节点选择
临时核心配置
运行时配置
旧网络 journal
旧系统代理 ownership
旧 TUN 状态
IP 查询缓存
测速结果
流量历史
日志
MySQL 遗留配置
AI 遗留配置
可能包含敏感数据的临时文件
```

## 9.4 不应无理由删除的内容

可以保留：

```text
核心二进制
程序资源
通用图标
非敏感默认模板
```

关于主题、语言等非敏感偏好：

```text
默认也可以重置；
如决定保留，必须与敏感文件物理分离；
不得因此读取旧敏感配置。
```

## 9.5 foreign context 不执行旧恢复记录

必须先识别设备上下文，再处理 network journal。

foreign context 时：

```text
不执行旧 route undo
不执行旧 DNS restore
不执行旧 system proxy restore
不尝试恢复旧 TUN 适配器
直接清除旧设备 journal
```

原因：

```text
旧 InterfaceIndex、网关、注册表和适配器不属于当前电脑。
```

## 9.6 同机正常启动不能误清

必须验证以下情况不会触发清除：

```text
Windows 重启
Navo 正常升级
Navo 安装目录变化
电脑名变化
网卡变化
IP 变化
核心升级
用户更换代理节点
```

如果用户换了 Windows 账户，DPAPI Current User 解密失败，应按新用户环境处理。

## 9.7 清理失败

如果任意高敏感文件不能删除或不能失效：

```text
Ready = false
不得启动核心
不得加载旧订阅
不得启用系统代理
不得启用 TUN
```

显示：

```text
隐私初始化失败
重试
打开数据目录
退出
```

不能提供“忽略并继续”。

---

# 10. 第七阶段：清理当前不可调用的代码

优先级：P1，分两轮执行

## 10.1 第一轮

在第一、第二份文档尚未完成前，只允许删除：

```text
AI 专属不可达代码
MySQL 专属不可达代码
已确认由现实现完全替代的重复实现
明显无入口的旧实验代码
```

## 10.2 第二轮

前两份文档全部完成后，再检查：

```text
无 IPC 调用
无 Wails 调用
无 Service/Agent 调用
无后台任务
无测试用途
无 build-tag 用途
无迁移用途
无恢复用途
无未来文档要求
```

全部满足才能删除。

## 10.3 重点排查类型

```text
已经废弃的旧 route manager
已经废弃的旧 DNS manager
重复的核心启动路径
重复的配置编译入口
没有 dispatch case 的 Handler
前端不存在调用的 Wails 方法
永远不注册的 provider
空实现 adapter
仅返回 UNSUPPORTED 的废弃 API
仅打印 “would clean” 的假实现
```

注意：

第一份文档可能要求重写或替换这些代码。  
必须先完成替代实现，再删除旧实现。

## 10.4 “无法调用但有业务价值”

如果代码当前无法调用，但业务明确需要：

```text
修复调用链，而不是删除。
```

例如：

```text
双联链路监测
四条流量统计
核心更新
日志筛选
IP 风险
托盘状态图标
```

处理顺序：

```text
确认第二份文档要求
补后端入口
补 IPC/Wails
补前端调用
补测试
```

---

# 11. 与第一份文档的逐项对接

## 11.1 必须继续执行

```text
TUN Find-NetRoute 标量修复
统一 Runtime Transition
节点切换更新旁路路由
核心切换进入事务
回滚失败保留 dirty/faulted
删除 Host/Supervisor 双重重启
单进程关闭外部 Service Pipe
删除任意 config_path
系统代理完整恢复
核心完整性校验
多内核能力一致性
订阅 SSRF 防护
安全原子写入
IPC 完整写入
Windows 发布门禁
```

## 11.2 AI 相关项目改为删除

第一份文档中的 AI 安全整改不执行实现修复，改为：

```text
删除 AI 模块
删除 AI IPC
删除 AI UI
删除 AI 配置
删除 AI key
清理历史数据
```

## 11.3 数据库一致性改为本地一致性

所有类似：

```text
数据库 revision
数据库 selection
MySQL transaction
```

改为：

```text
本地 revision repository
本地 active selection
原子文件事务
committed runtime 一致性
```

## 11.4 不删除恢复代码

即使恢复模块当前入口存在问题，也必须按第一份文档修复，而不是按“不可调用”删除。

---

# 12. 与第二份文档的逐项对接

以下功能全部保留并实现：

```text
升级内核
一键测速
四条流量
流量属性复选
虚拟流量测试
曲线 Tooltip
日志清空
日志级别筛选
内部服务筛选
日期筛选
双联链路
Direct/Proxy IP 风险
隐藏滚动条
统一图标
```

## 12.1 不属于 AI 的远程查询

以下功能可以使用普通受控 HTTPS，不因删除 AI 而删除：

```text
检查核心版本
下载核心
刷新订阅
测速
出口 IP
IP 风险查询
```

要求仍按照第二份文档执行：

```text
超时
取消
HTTPS
缓存
隐私提示
响应大小限制
错误状态
```

## 12.2 日志服务分类

第二份文档的日志服务列表中如果包含：

```text
AI
```

移除该分类。

其他分类保留：

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
```

## 12.3 初始化不能清掉正常新功能配置

同机升级时必须保留：

```text
流量曲线选择
日志筛选偏好
主题
语言
核心更新状态
图标状态配置
IP 风险设置
```

仅在 foreign context 隐私重置时按安全策略重置。

---

# 13. 本地持久化边界

删除 MySQL 后，优先复用已有存储并分类。

## 13.1 非敏感配置

例如：

```text
主题
语言
窗口状态
曲线选择
日志筛选偏好
核心更新检查时间
```

可使用本地结构化文件。

## 13.2 敏感配置

例如：

```text
订阅地址
token
节点密码
UUID
私钥
代理认证
```

必须使用：

```text
DPAPI Current User
明确 ACL
安全原子替换
```

## 13.3 临时运行配置

核心生成配置可能包含敏感数据：

```text
必须限制 ACL
使用受控 runtime 目录
旧配置及时清理
拒绝写入普通日志
```

## 13.4 不新增远程存储

不得使用：

```text
MySQL
PostgreSQL
云数据库
远程配置中心
远程账号同步
```

---

# 14. 测试要求

## 14.1 删除完整性

执行：

```powershell
rg -n "mysql|go-sql-driver|MYSQL_|DATABASE_URL|DSN" .
rg -n "OpenAI|DeepSeek|Claude|AI API|ai\.settings|ai\.diagnose|ai\.rule" .
```

允许出现的位置：

```text
迁移清理代码
历史说明文档
本修复报告
```

运行时代码不得残留。

## 14.2 编译与测试

```powershell
go test ./...
go test -race ./...
go vet ./...

Push-Location navo_app
npm ci
npm run typecheck
npm run test
npm run build
Pop-Location
```

## 14.3 业务回归

必须确认删除 MySQL/AI 后：

```text
程序正常启动
订阅可添加和刷新
节点可选择
系统代理可启停
TUN 可启停
核心可启动和切换
网络恢复可执行
日志可查看
IP 检测可执行
测速可执行
核心升级可执行
流量监测可执行
托盘正常
```

## 14.4 初始化测试

覆盖：

```text
全新安装
同机第二次启动
同机软件升级
数据目录复制到新电脑
数据目录复制到新 Windows 用户
device-state 损坏
DPAPI 解密失败
隐私清理成功
隐私清理失败
MySQL 遗留字段清理
AI 遗留字段清理
foreign context 不执行旧 journal
同机正常 journal 仍按第一份文档恢复
```

## 14.5 死代码删除测试

每次删除一组旧代码后：

```text
运行全量测试
运行前端构建
运行 Windows smoke
检查 IPC 方法清单
检查 Wails 方法清单
检查菜单和页面
```

不能一次删除大量代码后再统一排错。

---

# 15. 禁止的伪修复

Codex 不得：

1. 按旧第三份文档重写整个项目目录；
2. 删除 MySQL 时连业务模型一起删；
3. 删除 AI 时连本地网络诊断一起删；
4. 只隐藏 AI/MySQL UI；
5. 保留 AI API key “备用”；
6. 保留 MySQL driver “备用”；
7. 把 MySQL 换成 SQLite 而不分析必要性；
8. 把所有联网功能误判为云端业务并删除；
9. 删除第一份文档要求修复的恢复代码；
10. 删除第二份文档要求接入的新功能代码；
11. 只凭 `rg` 无引用就删除 Wails/IPC 动态入口；
12. 删除 Windows build-tag 文件；
13. 删除测试 fake 和 integration helper；
14. foreign context 执行旧设备 network journal；
15. 隐私清理失败后继续启动；
16. 删除整个 `.env` 而不是只清理 MySQL/AI 字段；
17. 通过吞错保证启动；
18. 没有业务回归测试就声明完成。

---

# 16. 建议提交顺序

```text
1. docs(cleanup): map MySQL, AI and unreachable call paths
2. remove(ai): delete AI backend, IPC, UI and persisted secrets
3. refactor(diagnostics): retain non-AI local diagnostics
4. remove(mysql): delete MySQL runtime, config and dependencies
5. refactor(storage): preserve required business state in local repositories
6. feat(init): add minimal initialization and migration coordinator
7. feat(privacy): add DPAPI device binding and foreign-context reset
8. cleanup(dead-code): remove verified obsolete implementations
9. test(regression): prove core proxy business remains functional
10. docs(report): record coordination with remediation and feature guides
```

不要把十项压成一个提交。

---

# 17. 总体验收标准

## 文档配合

- [ ] 第一份文档的故障修复未被取消；
- [ ] 第二份文档的产品功能未被误删；
- [ ] 旧第三份文档已明确废止；
- [ ] AI 冲突统一按删除处理；
- [ ] MySQL 冲突统一按本地 repository 处理；
- [ ] 当前不可达但计划接入的业务得到保留。

## MySQL

- [ ] 无 MySQL 运行时；
- [ ] 无 MySQL driver；
- [ ] 无 DSN；
- [ ] 无 MySQL 设置 UI；
- [ ] 无数据库启动依赖；
- [ ] 必要业务数据仍可本地保存；
- [ ] revision/selection 没有被误删；
- [ ] MySQL 遗留配置会被清理。

## AI

- [ ] 无 AI 后端；
- [ ] 无 AI IPC；
- [ ] 无 AI UI；
- [ ] 无 AI key；
- [ ] 无 AI SDK；
- [ ] 无 AI 缓存；
- [ ] 有价值的本地诊断仍保留；
- [ ] AI 遗留数据会被清理。

## 初始化与隐私

- [ ] 初始化在业务加载之前；
- [ ] 使用 DPAPI Current User 绑定；
- [ ] 新电脑无法读取旧凭据；
- [ ] 新电脑自动清除旧敏感配置；
- [ ] 新电脑不执行旧 network journal；
- [ ] 同机重启不会误清；
- [ ] 清理失败拒绝启动；
- [ ] 不依赖单一硬件指纹。

## 死代码

- [ ] 每个删除项都有可达性证据；
- [ ] 动态入口已检查；
- [ ] build tags 已检查；
- [ ] 恢复与迁移用途已检查；
- [ ] 前两份文档用途已检查；
- [ ] 删除后业务回归通过。

## 核心业务

- [ ] 系统代理正常；
- [ ] TUN 正常；
- [ ] 节点与订阅正常；
- [ ] 三个核心正常；
- [ ] 核心升级正常；
- [ ] 测速正常；
- [ ] 四条流量正常；
- [ ] 日志正常；
- [ ] IP 风险正常；
- [ ] 托盘和图标正常；
- [ ] 网络恢复正常。

---

# 18. Codex 最终交付报告格式

```markdown
# Navo 精准清理与初始化完成报告

## 1. 与前两份文档的配合
- 第一份文档保留项：
- 第二份文档保留项：
- 冲突处理：

## 2. AI 删除
- 删除文件：
- 删除接口：
- 删除 UI：
- 保留的本地诊断：
- 遗留数据清理：

## 3. MySQL 删除
- 删除文件：
- 删除依赖：
- 删除配置：
- 本地替代：
- 保留的业务模型：

## 4. 初始化模块
- 启动顺序：
- 设备绑定：
- 迁移：
- 错误处理：

## 5. 新电脑隐私清理
- 判定方式：
- 清理范围：
- 是否执行旧 journal：
- 清理失败行为：

## 6. 死代码清理
| 删除模块 | 原入口 | 不可达证据 | 替代实现 | 测试 |
|---|---|---|---|---|

## 7. 业务回归
- 系统代理：
- TUN：
- 订阅：
- 核心：
- 升级内核：
- 测速：
- 流量：
- 日志：
- IP 风险：
- 托盘：

## 8. 自动测试
```text
go test ./...
go test -race ./...
go vet ./...
npm run test
npm run build
```

## 9. Windows 手工验证
- 同机重启：
- 同机升级：
- 新电脑复制：
- 新用户复制：
- 网络恢复：

## 10. 未完成项
- ...

## 11. 总体验收
- 是否通过：
- 未通过项：
```

没有测试和调用链证据的项目，不得标记完成。

---

# 19. 最终目标

完成后，Navo 应满足：

```text
保留全部必要代理业务
    +
完成第一份文档中的安全和故障修复
    +
完成第二份文档中的产品功能优化
    +
彻底移除 MySQL
    +
彻底移除 AI
    +
清除确认无用且无法调用的死代码
    +
新电脑自动清除旧敏感配置
```

不得出现：

```text
为了删除 MySQL 导致节点选择丢失
为了删除 AI 导致本地诊断消失
把尚未接通的流量/IP/升级功能当死代码删除
新电脑执行旧设备路由恢复
同机升级误删用户配置
隐藏页面但后台仍保留 AI 或 MySQL
代码能编译但代理业务已经断裂
```
