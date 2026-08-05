## Why

拆单试算接口 `get_biz_apply_recommend_split_suborder` 在增量拆分（入参带 `occupied_suborders`）时，按既定契约**只返回本次新算出的增量子单、不回显已占用子单**。但 `after_tool_hitl` 构造确认卡片 payload 时只读取工具返回结果，于是「已有一条 S2 子单、再加一条 S3」的场景下，确认卡片只剩 S3，之前已确认的 S2 被静默丢弃（TAPD 1069995598136531300）。

由此还派生出第二个问题：用户只能提示 LLM「自己把两条拼起来给我确认」，而结构化确认卡片只能由「工具调用后中断」产生，LLM 无法触发中断，只能输出模仿卡片格式的裸 JSON 文本（因为中断 payload 会被 `doInterrupt` 注入回助手消息作为上下文，模型学会了复读该格式），确认环节退化为不可交互的文本。

根因是「已确认子单清单」在整条链路上没有任何一层确定性地持有：后端拆单接口无状态且只吐增量，Graph state 只累积推荐候选（`StateKeyRecommendCandidates`）而无已确认子单，前端卡片纯渲染 payload，唯一"记得"历史子单的地方是 LLM 上下文里的文本。

## What Changes

- `after_tool_hitl` 处理拆单试算结果时，除解析工具返回的增量子单外，还 SHALL 从触发本次中断的那次工具调用的 arguments 中解析 `occupied_suborders`，按「已占用在前、增量在后」合并为完整清单再构造中断 payload。
- 合并只作用于**展示/确认 payload**：不改拆单接口契约（后端仍只返回增量）、不改请求入参、不改前端渲染逻辑。
- 兜底行为：arguments 缺失 `occupied_suborders`、解析失败或为空时，SHALL 退化为当前行为（仅增量子单），不得阻断中断。
- 中断判定条件维持不变——仍以**增量子单数** `>= 1` 决定是否中断，避免出现「本次增量为 0 却因历史占用而弹出卡片」的假确认。

**非目标（本次不做）**：不引入 Graph state 持有「已确认子单清单」的方案。该方案（探索中的 C 方案）能覆盖「用户在卡片上改行/删行后，下一轮增量拆分的 occupied 基准如何确定」，但需要额外设计与 checkpoint 回放的交互，留待本次上线观察后另开变更评估。

## Capabilities

### New Capabilities
<!-- 无新增能力，本次为既有 Requirement 的行为修正 -->

### Modified Capabilities
- `agent-after-tool-hitl`: 修改「after_tool_hitl 对拆单试算结果按出单中断」Requirement，明确 payload 为「入参已占用子单 + 返回增量子单」的合并结果，并明确中断判定仍只看增量子单数。

## Impact

- 受影响代码：`cmd/agent-server/logics/agent/aftertool/node.go`（`handleSplitSuborderRecommend`、`interruptWithRecommendSuborders`）、`cmd/agent-server/logics/agent/aftertool/parse.go`（`findRecommendToolCall` 需保留原始 arguments，新增 occupied 解析函数；可复用 `extractLimit` 从 `parameters.body_param` / `body_param` 双 schema 取值的既有模式）。
- 不改动 woa-server 拆单接口、`pkg/api/woa-server` 协议、DB 与离线统计；`apply-recommend` 能力的 Requirement 不变。
- 前端无需改动：`host-apply-preorder-card` 本就按 payload 全量渲染，用户点「确认方案」时经 `chat-message-list.vue: handleConfirmPreorder` 将卡片上的完整清单以 resume 值回传，本变更落地后该清单即为合并后的全量。
