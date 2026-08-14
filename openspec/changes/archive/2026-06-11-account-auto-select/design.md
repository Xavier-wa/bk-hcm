## Context

### 现有架构

当前 agent-server 在 graph 模式（`enumor.AgentModeGraph`）下由 `agent.BuildGraph()` 构建一个通用 ReAct 图，拓扑为：

```
START → llm → ConditionalEdge
                ├─ human_confirm → hitl → llm
                ├─ other_tools  → tool → llm
                └─ no_tools     → fallback（interrupt）→ llm
```

本期在现有 graph 入口前插入账号自动识别节点，将 START 直接路由至 `account_select`，账号确认完成后再进入 `llm` 节点开始正常的 ReAct 循环。

### 凭据注入机制

请求到达时，`StateResolver` 将 `bk_ticket`、`bk_username`、`bk_biz_id` 注入 graph State 的 RuntimeState。`account_select` 节点调用内部 cloud-server client，**无需 `bk_ticket`**，仅从 RuntimeState 读取 `bk_biz_id` 即可（内部服务间调用由框架自动处理鉴权）。`bk_biz_id` 的读取方式参考 `instruction_inject.go` 中的 `MakeBizContextInjectCallback`：从 `inv.RunOptions.RuntimeState[constant.SessionBkBizIDStateKey].(int64)` 读取。

### 已有 HITL 模式参考

`hitl/node.go` 已实现 `graph.Interrupt` 的完整调用模式（首次中断 → 保存 checkpoint → resume 返回值），新节点直接复用相同范式。

---

## Goals / Non-Goals

**Goals：**
- 在 graph 入口插入账号自动识别步骤（START → `account_select`，账号确认后 → `llm`）
- 支持 0/1/N 账号三种场景的正确路由
- 通过 `graph.Interrupt` 实现无权限提示和多账号选择两种 HITL 中断协议
- 将选中的 `account_id` 写入 graph State 供下游节点消费

**Non-Goals：**
- 前端 UI 渲染（前端单独处理 interrupt event）
- 账号查询结果缓存（本期不引入）
- 对非 `tcloud-ziyan` vendor 账号的完整申领支持（留 TODO）
- 对 `account_select.interrupt` resume 的 `account_id` 合法性校验（留 TODO）

---

## Decisions

### D-001：账号查询接口调用方式 — 直接 HTTP vs MCP toolproxy

**选择：直接 HTTP（通过项目内部 cloud-server client）**

`account_select` 是一个确定性的 Function Node，所调用的接口（`ListByUsageBizID`）已存在于 cloud-server。Function Node 直接调 HTTP 有以下优势：

- **确定性**：调用路径固定，不依赖 LLM 工具选择
- **轻量**：无需通过 MCP toolproxy 的元工具路由层
- **易测试**：可以直接 mock HTTP client

MCP toolproxy 适合 LLM 决策调用的工具场景，不适合 Function Node 内部的固定接口调用。

> **实现方式**：通过 `pkg/client/cloud-server` 的 client，超时设置 3 秒。调用时**无需 `bk_ticket`**，内部服务间调用由框架处理鉴权，仅需传入 `bk_biz_id`。

> **注意**：`ListByUsageBizID` 接口返回的账号列表**不包含** `enabled`/`reason` 字段，这两个字段是 agent 场景独有的，需要在 `account_select` 节点内部根据 vendor 自行计算（见 D-007）。

### D-002：新节点文件位置

**选择：新建 `cmd/agent-server/logics/agent/cvm/` 子包**

- 与现有 `hitl/` 包保持一致的组织方式（按功能子域分包）
- CVM workflow 节点未来还会增加更多 Function Node（如 `fetch_plans`、`match_spec` 等），统一放 `cvm/` 包便于管理
- 文件命名：`account_select.go`（包含两个节点的实现）

### D-003：`no_permission_fallback` 后的回环节点

**选择：`no_permission_fallback` resume 后路由回 `account_select`**

当前 graph 中不存在 `intent_recognition` 节点。`no_permission_fallback` 在用户确认后应回到 `account_select` 重新查询账号，让用户可以在权限问题解决后重试。

graph 框架支持有环图（通过 checkpoint 保存恢复状态），`hitl`、`tool` 等节点均已通过 `→ llm` 的回环在生产中稳定运行，`no_permission_fallback → account_select` 的回环可复用相同机制。

### D-004：`account_select` 节点中多账号 HITL 的实现位置

**选择：`account_select` 节点自身调用 `graph.Interrupt`（不引入专属子节点）**

无权限场景由 `no_permission_fallback` 专属节点处理；多账号 HITL 由 `account_select` 节点直接在内部 resume，resume 后将 `account_id` 写入 State 并继续（路由到 `get_datetime`）。

这与 TAPD 需求中 [F-004] 的设计一致，并参考了现有 `hitl/node.go` 的 `graph.Interrupt` 模式。

### D-005：interrupt key 常量命名

**选择：在 `pkg/criteria/constant/aiagent.go` 新增**

```go
const (
    NoPermissionInterruptKey  = "no_permission.interrupt"
    AccountSelectInterruptKey = "account_select.interrupt"
)
```

与现有 `HITLInterruptKey = "human_confirm"` 和 `FallbackInterruptKey = "fallback"` 保持同文件管理。

### D-007：enabled/reason 字段的计算位置

**选择：在 `account_select` 节点内部按 vendor 过滤，自行计算 `enabled`/`reason`**

`ListByUsageBizID` 接口返回原始账号列表，不含 `enabled`/`reason` 字段。当前版本 CVM 申领 workflow 仅支持 `vendor = tcloud-ziyan` 的账号，其他 vendor 账号需要标记为不可用并说明原因。

计算规则：
- `vendor == "tcloud-ziyan"` → `enabled = true`, `reason = ""`
- 其他 vendor → `enabled = false`, `reason = "当前仅支持自研云（tcloud-ziyan）账号"`

**count 的统计口径**：`count` 指**所有**返回账号的数量（含不可用账号），`enabled/reason` 仅用于 `account_select.interrupt` 的 options 展示，供前端灰显不可用账号。0/1/N 分支路由逻辑基于全量账号数。

> 后续支持更多 vendor 时，只需扩展此节点内的 vendor 过滤规则，无需修改接口。

### D-006：cloud-server client 注入方式

**选择：为 `BuildGraph` 增加 `*cloudserver.Client` 参数**

`account_select` 节点需要调用 `GET /api/v1/cloud/accounts/bizs/{bk_biz_id}?account_type=resource`。
通过在 `BuildGraph` 签名中增加 `cloudClient *cloudserver.Client` 参数，并由 `runtime.go` 的 `newAGUIRunner` 从 `clientSet.CloudServer()` 传入，节点工厂函数以闭包方式捕获该 client。

此方式与现有 `toolset`、`proxy` 等依赖的注入模式一致，且可按正常 `kit.Kit` 模式构造鉴权头。

---

## 数据流设计

```
[account_select 节点]
  ↓ 读取 inv.RunOptions.RuntimeState[SessionBkBizIDStateKey].(int64) 获取 bk_biz_id
  ↓ GET /api/v1/cloud/accounts/bizs/{bk_biz_id}?account_type=resource (3s timeout, 无需 bk_ticket)
  ↓ 节点内部按 vendor 计算 enabled/reason（tcloud-ziyan → enabled=true，其他 → enabled=false）
  │
  ├─ count == 0 → 路由到 [no_permission_fallback]
  │               ↓ graph.Interrupt(NoPermissionInterruptKey, {type, message})
  │               ↓ resume（任意字符串）→ 路由到 [account_select]（重试）
  │
  ├─ count == 1 → state["account_id"] = accounts[0].ID
  │               → 路由到 [llm]
  │
  └─ count >= 2 → 构建 options（含节点内计算的 enabled/reason）
                  ↓ graph.Interrupt(AccountSelectInterruptKey, {type, message, options})
                  ↓ resume（account_id 字符串）
                  ↓ state["account_id"] = resumeValue
                  → 路由到 [llm]
```

## Interrupt 协议

### `no_permission.interrupt`

```json
{
  "type": "no_permission.interrupt",
  "message": "当前用户在该业务下没有可用的账号权限，无法继续主机申领流程"
}
```

resume value：任意字符串（前端用户确认后回传）

### `account_select.interrupt`

```json
{
  "type": "account_select.interrupt",
  "message": "检测到多个可用账号，请选择用于本次申领的账号",
  "options": [
    {
      "account_id": "xxx",
      "account_name": "账号名称",
      "vendor": "tcloud-ziyan",
      "enabled": true,
      "reason": ""
    }
  ]
}
```

resume value：选中的 `account_id` 字符串

---

## Risks / Trade-offs

- **[风险] 账号接口 3 秒超时不够用** → 本期阈值参考需求文档，后续可通过配置化扩展；超时后写入 `last_error` 并终止流程，不影响其他节点
- **[风险] resume value 不合法（空字符串或非账号 ID）** → 本期仅做非空校验，后续可按需加入账号存在性校验
- **[风险] `no_permission_fallback → account_select` 循环可能无限重试** → resume 后回到 `account_select` 重新查询账号；无权限是外部条件（权限授予后即可通过），本期不加重试计数

## Open Questions

无。所有关键决策已在本文档中确认，与 TAPD 需求澄清结论一致。
