## Context

主机申领 Agent 采用 trpc-agent-go 的 Graph 编排（见 `cmd/agent-server/logics/agent/graph_build.go`）。当前 HITL 仅有「工具调用前」一个中断：LLM 调用声明性工具 `human_confirm` → 路由到 `hitl` 节点 → `graph.Interrupt`。`tool` 节点执行完工具后是固定回边 `AddEdge("tool","llm")`，没有「调完工具再判断要不要中断」的能力。

在线推荐三接口已在 `cmd/woa-server/service/task/recommend.go` 落地，并经 APIGW 暴露为 MCP 工具：

- `get_biz_apply_recommend_by_static`（离线偏好推荐：离线偏好 + 库存，返回若干单子单方案）
- `get_biz_apply_recommend_by_plan`（预测余量推荐：预测余量 + 库存，返回若干单子单方案）
- `get_biz_apply_recommend_split_suborder`（拆单试算：单据拆分，返回主单 + 多子单）

本变更只做 Graph 编排改造，不改动上述接口实现。

## Goals / Non-Goals

- Goals：
  - 新增 `after_tool_hitl` 工具调用后中断节点，支撑「调工具 → 判断方案 → 中断/回 LLM」闭环。
  - 三个推荐工具调用后进入 `after_tool_hitl` 判定。
- Non-Goals：
  - 推荐流程的 Skill 沉淀（本期不做）。
  - 同机型族查询工具与「扩展换机型再拆单」闭环（本期不做）。
  - 三个推荐接口业务逻辑、`human_confirm`/`hitl` 既有逻辑、前端渲染改造。

## Decisions

### D1：保留 `hitl` 节点与 `human_confirm` 工具名不变，仅新增 `after_tool_hitl` 节点

`human_confirm` 是面向 LLM 的工具名，语义直白（"找人确认"），改名为架构术语会削弱 LLM 触发准确性；节点名 `hitl` 为纯内部标识，改名收益仅为对称。故两者均不动，只新增 `after_tool_hitl` 节点与配套路由。

### D2：`tool` 节点改为条件路由，按"是否调用了推荐工具"分流

将 `AddEdge("tool","llm")` 改为 `AddConditionalEdges("tool", aftertool.MakeRoutingFunc(), {...})`。路由函数检查最近一条 assistant 消息的 `tool_calls`：含任一推荐工具 → `after_tool_hitl`；否则 → `llm`（保持原行为）。路由与解析均兼容 tool-proxy 模式（LLM 调 `execute_tool`，真实工具名在 `tool_name` 参数中）与直连模式。

- Alternatives considered：在 `tool` 与 `llm` 间插一个普通路由节点。仅当框架不支持从 ToolsNode 直接挂条件边时采用（O1 已验证支持，未采用）。

### D3：`after_tool_hitl` 节点内做确定性判定，中断与否按工具区分（非对称）

| 触发工具 | 判定 | 满足 → | 不满足 → |
|---|---|---|---|
| 离线偏好推荐 by_static | `len(Items) >= limit` | 构造 payload 后 `Interrupt` | 不中断，回 `llm`（LLM 续调预测余量推荐） |
| 预测余量推荐 by_plan | 合并(离线偏好+预测余量)后 `len > 0` | 构造合并 payload 后 `Interrupt` | 不中断，回 `llm`（LLM 决策/问用户） |
| 拆单试算 split | `len(Suborders) >= 1` | 构造主单+子单 payload 后 `Interrupt` | 不中断，回 `llm` |

判定为确定性代码（非交给 LLM），与 iWiki「在 after_tool_hitl 节点对工具结果进行判断」一致。

### D4：候选累积用 Graph state，离线偏好推荐覆盖写、预测余量推荐合并写（选项②+2a）

新增 state key（`recommend:candidates`，以 JSON 字符串存储）累积推荐候选：

- 处理离线偏好推荐：`candidates = 离线偏好结果`（覆盖写，天然清掉上一轮）。
- 处理预测余量推荐：`candidates = 合并(已有候选, 预测余量结果)`，保留已有离线偏好候选在前、追加预测余量中有效（非 nil 且含 `Suborder`）的项；当前实现（`mergeCandidates`）为顺序拼接 + 过滤无效项，**不做** `require_type|region|device_type|image_id` 组合键去重。

「离线偏好推荐即轮边界覆盖写」免去显式清空逻辑；正确性依赖 LLM「每轮推荐离线偏好先行」的调用约定（由提示词约束）。

- Alternatives considered：
  - 选项①（中断时反向扫 messages 找最近离线偏好结果）：免累积状态，但解析配对 `tool_call.ID` 易错。
  - 选项②+2d（候选打轮次标记）：最稳但记账最重。
  - 取「实现简单」，选 ②+2a。

### D5：中断 payload 在 `graph.Interrupt` 之前就地构造

executor 在 InterruptError 场景不应用节点返回的 delta（与 `fallback`/`account_select` 一致）。因此中断分支必须先用候选/工具结果构造好 payload 再调 `Interrupt`；候选 delta 在中断路径丢失不影响后续（下一步是拆单试算/提单/新一轮离线偏好覆盖）。仅「离线偏好不足 → 回 llm」的非中断路径需要候选 delta 落库，此路径不中断、delta 正常应用。

中断 key 复用 `account_select` 的去重模式：`buildInterruptKey` 在场景基 key（`after_tool_hitl.recommend_select.interrupt` / `after_tool_hitl.recommend_suborder_confirm.interrupt`）后拼接当前消息条数，避免与历史 checkpoint 的 key 冲突。

### D6：resume 优先采用前端 forwardedProps 结构化回复

`after_tool_hitl` 中断恢复时，`doInterrupt` 优先读取前端经 `forwardedProps` 传入、存于 `RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 的结构化回复：命中（非空字符串）时以其作为用户选择，并发自定义事件 `after_tool_hitl.resume_forwarded`（`AfterToolHITLResumeForwardedEventName`，payload `{value}`）供前端观测，同时忽略中断的自由文本 `resumeValue`；未命中时回退到 `resumeValue`（要求为非空字符串，否则报错终止）。两条路径都复用 `message.BuildFallbackResumeDelta` 把用户选择并入消息并回 `llm`。

- 设计意图：`after_tool_hitl` 是通用「走工具后中断」节点（当前接入推荐工具的 by_static / by_plan / split_suborder 三分支），新增「走工具后中断」场景只需在 `GetNode` 的 `switch` 中扩展分支；故 resume 解析逻辑做成与场景无关的通用能力。

## Risks / Trade-offs

- **R1：依赖 LLM 遵守「离线偏好先行」**（D4 正确性前提）→ 通过提示词/调用约定约束；残留风险仅「某轮跳过离线偏好只调预测余量」，由提示词兜底「新一轮必须先离线偏好」。
- **R2：合并跨工具候选只在预测余量中断时发生**，per-call 判定无法覆盖「离线偏好、预测余量各自不足但合并足够」——已按决策改为「预测余量后合并 `len>0` 即中断」，规避该缺口。

## Migration Plan

- 纯新增节点与路由，`hitl`/`human_confirm`/前端不变，无数据迁移。

## Open Questions

- **O1（已解决）**：trpc-agent-go 的 `AddConditionalEdges` 不限节点类型，可从 `tool`（ToolsNode）直接挂条件边，无需中转节点（编译通过验证）。
- **O2（已解决）**：离线偏好推荐 args 缺 `limit` 时由 `extractLimit` 兜底为 `DefaultRecommendLimit=5`。
- **O3（已定型）**：`after_tool_hitl` 中断不再用 payload 字段携带场景，改由自定义事件 `name`（即中断 key 基）区分场景：推荐方案选择为 `after_tool_hitl.recommend_select`，拆单试算确认为 `after_tool_hitl.recommend_suborder_confirm`；payload 仅含数据字段 `recommendations|suborders`（`recommendations` 命名避免与"预测推荐 by_plan"的 plan 语义冲突）。前端渲染该新 interrupt 为后端外依赖项。
