# Navo 架构边界 V1 源码映射与实现状态

基准文件：docs/Navo_架构边界_调度与自愈设计规范_V1.md

## 1. 跨域控制边界

唯一跨域业务协调者位于 User Agent：

- internal/connection/coordinator.go：typed Connection Coordinator。
- internal/agent/capture_transition.go：节点、Core、来源、接管、策略、恢复事务入口。
- internal/agent/tray.go：连接重启与网络恢复在同一事务内完成。
- internal/service/capture_transition.go：Service 内部仅保留 domain-local mutation gate，负责保护自身 Core/TUN/网络资源，不承担跨域业务决策。

Coordinator 记录 operation、origin、phase、fault domain、排队数、当前事务与最近完成结果。观察请求不获取事务；所有会改变连接状态的 Agent 请求必须获取事务。

## 2. 控制与观测分类

允许并行的观察请求：

- dashboard.snapshot
- core.status / core.list
- capture.status / tun.status / proxy.status
- subscription.list / outbound.list / outbound.test
- runtime.status
- metrics.current / ip.check
- logs / diagnostics

必须串行的控制请求：

- capture.set
- core.select / core.update
- outbound.select / outbound create/delete
- subscription add/remove/refresh/update
- runtime mode/rules/list-mode
- connection restart
- network recovery
- startup recovery
- scheduler-triggered capture recovery

capture.status 额外暴露 transaction snapshot，观察层可在控制事务运行时持续刷新。

## 3. Active / Candidate 节点

runtimeState 明确区分：

- SelectedOutbound：当前部署意图，供编译器与运行时使用。
- ActiveOutbound：最后一次通过真实健康验证并提交的节点。
- CandidateOutbound：已选择但尚未提交的候选节点。

状态迁移：

1. applyRuntimeConfig 生成 candidate revision；
2. Core/capture/data-plane 验证期间 Active 保持不变；
3. commitHealthyRuntime 成功后 Candidate 才提升为 Active；
4. 用户主动切换失败时，Agent 根据 SelectedOutbound 恢复原 Active，再恢复原 capture mode；
5. node-only transaction 不修改 runtime mode、list mode 或 capture preference。

旧 runtime_state 兼容规则：显式 candidate 不提升；active 或 pre-V1 空 revision status 保留为已提交状态。

## 4. 接管与网络所有权

现有边界继续生效：

- Agent 仅写 Navo 持有 ownership marker 的 WinINet System Proxy。
- Service/TUN 仅操作 canonical Navo adapter、事务内 owned Route/DNS/NRPT/Firewall。
- 外部 Clash、v2rayN、VMware、Hyper-V、WSL、VPN/TUN、物理网卡和用户 DNS 只观测，不修改。
- SYSTEM_PROXY 与 TUN 的互斥、停止旧模式、完整 rollback、再启用新模式仍由后端事务保证。

## 5. Scheduler 与 SelfHeal

Scheduler/background observer 只能产生请求或事件；实际 mutation 必须进入 Agent Coordinator 或 Service domain-local gate。

SelfHeal V1 安全边界：

- FaultDomain 已覆盖 node、core、system_proxy、tun、route、dns、nrpt、firewall、traffic_rule、physical_network、detection、unknown。
- MaxRepairRounds 固定为 2。
- DefaultConfig、policy budget 和运行时 effective budget 均被截断到 2；policy 无法扩大。
- 默认 production policies 保持 observe-only，不新增未经定义的网络修复动作。
- 每次现有 repair 仍要求 Verify，失败 mutation 仍要求 Rollback，并记录 stable error code、attempt、action、verification、rollback 和 circuit 状态。

## 6. 本阶段未擅自实现

V1 第 22 节明确留待下一阶段定义的内容未自行推断：

- 各 FaultDomain 的证据矩阵；
- 第一轮、第二轮允许执行的具体修复动作；
- 活动节点故障后的自动候选探测与同来源 failover 触发条件；
- BLACKLIST / WHITELIST 名单内的最终精确定义；
- UI 信息架构重新分布。

这些行为需要独立规范后才能启用，否则会扩大自动网络 mutation 权限。当前代码已经具备统一事务、Active/Candidate、source_type 和两轮硬上限，可作为后续实现边界。

## 7. 验证结果

- scripts/test.ps1：PASS，覆盖 go test ./... 与 go vet ./...。
- npm.cmd test -- --run：6/6 PASS。
- npm.cmd run typecheck：PASS。
- npm.cmd run build：PASS。
- git diff --check：PASS，仅出现 core.autocrlf 换行提示。

本阶段是源码架构调整，未执行 elevated System Proxy/TUN mutation 或真实 data-plane acceptance；这些验证不会被源码门禁替代。
