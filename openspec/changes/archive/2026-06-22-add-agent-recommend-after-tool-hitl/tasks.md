## 1. 常量与 state key

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 新增两个 `after_tool_hitl` 中断 key（`recommend_select` / `recommend_suborder_confirm`）、resume 自定义事件名 `AfterToolHITLResumeForwardedEventName`、tool-proxy 全名常量 `ProxyExecuteToolFullName`
- [x] 1.2 在 `pkg/criteria/enumor/aiagent.go` 新增节点枚举 `CvmApplyNodeAfterToolHITL` 并纳入 `Validate`
- [x] 1.3 新增推荐工具名常量（`get_biz_apply_recommend_by_static` / `_by_plan` / `_split_suborder`）与 `DefaultRecommendLimit=5`
- [x] 1.4 新增推荐候选累积 state key（`recommend:candidates`）

## 2. Graph 编排改造（graph_build.go）

- [x] 2.1 新增 `after_tool_hitl` 节点并注册到 Graph
- [x] 2.2 将 `tool` → `llm` 固定边改为条件路由 `aftertool.MakeRoutingFunc`
- [x] 2.3 路由函数：识别最近 assistant 消息 `tool_calls` 是否含推荐工具，分流到 `after_tool_hitl` 或 `llm`
- [x] 2.4 添加 `after_tool_hitl` → `llm` 循环边
- [x] 2.5 验证 ToolsNode 能否直接挂条件边（O1）：framework `AddConditionalEdges` 不限节点类型，编译通过，无需中转节点

## 3. after_tool_hitl 节点实现

- [x] 3.1 解析最近一轮工具调用与工具结果（区分三个推荐工具，含 tool-proxy `execute_tool` 解包）
- [x] 3.2 离线偏好推荐 by_static：覆盖写候选 state；`len(Items) >= limit` 则构造 payload 后中断，否则回 `llm`
- [x] 3.3 预测余量推荐 by_plan：在离线偏好候选之后追加预测余量有效项合并（过滤无 `Suborder` 的无效项，顺序拼接、不做组合键去重）；合并 `len > 0` 则构造 payload 后中断，否则回 `llm`
- [x] 3.4 拆单试算 split_suborder：`len(Suborders) >= 1` 则构造主单+子单 payload 后中断，否则回 `llm`
- [x] 3.5 payload 统一在 `graph.Interrupt` 之前构造（规避 InterruptError 不落 delta）
- [x] 3.6 resume 后将用户选择并入消息并回 `llm`（复用 `message.BuildFallbackResumeDelta`）
- [x] 3.7 离线偏好推荐缺 `limit` 入参时的兜底判定（`extractLimit` 缺省 `DefaultRecommendLimit=5`，O2）
- [x] 3.8 resume 优先采用前端 `forwardedProps` 结构化回复（`RuntimeState[StateKeyForwardedResumeValue]`），命中时发 `after_tool_hitl.resume_forwarded` 自定义事件，未命中回退自由文本值
- [x] 3.9 工具名解析兼容 tool-proxy（`execute_tool` / `tool_proxy_execute_tool` 解包 `tool_name`）与带前缀工具名，结果解包剥离 tool-proxy/MCP content/网关/rest 多层封套（`parse.go`）

## 4. 配置与校验

- [x] 4.1 确认 MCP 工具映射（三个推荐工具 operationId）在 APIGW yaml 中已存在，无需新增网关配置
- [x] 4.2 `openspec validate add-agent-recommend-after-tool-hitl --strict` 通过
- [ ] 4.3 自测：离线偏好够数中断 / 不足回 llm；预测余量合并中断；拆单试算出单中断（需联调环境 + 真实 MCP，待运行时验证）
