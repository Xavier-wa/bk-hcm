## Context

`after_tool_hitl` 是挂在 `tool` 节点后置条件边上的中断节点，`handleSplitSuborderRecommend` 目前只消费工具**返回**（`parseSuborders(result)`）来构造确认卡片 payload。而拆单试算接口在增量拆分场景下只返回增量子单、不回显入参 `occupied_suborders`（见 `openspec/specs/apply-recommend/spec.md` 与归档变更 `2026-06-10-apply-recommend-split-suborder`），两者叠加导致确认卡片丢掉本批次此前已确认的子单。

约束：拆单接口契约已归档且被其他调用方共享，本次不动；前端卡片为无状态渲染，不具备跨轮累积能力；Graph state 当前只有 `StateKeyRecommendCandidates`（推荐候选），没有已确认子单的容器。

## Goals / Non-Goals

**Goals:**

- 增量拆分场景下，确认卡片展示本批次的完整子单清单（已占用 + 本次增量）。
- 修复限定在 agent 侧单一节点内，不扩散到接口契约、协议、DB 与前端。
- 合并失败时可安全退化，不引入新的中断阻塞路径。

**Non-Goals:**

- 不在 Graph state 中持有「已确认子单清单」（探索中的 C 方案）。
- 不改变拆单试算接口的请求参数与返回契约。
- 不解决「用户在卡片上改行/删行后，下一轮增量拆分的 occupied 基准如何确定」——该问题超出本方案能力边界。

## Decisions

**决策一：占用子单从工具调用 arguments 还原，而非由后端回显或由 LLM 拼接。**

三个候选中，让后端回显需要推翻已归档的 `apply-recommend` Requirement 并影响全部调用方，收益与本次目标重叠但代价不成比例；让 LLM 自行拼接是当前的事实做法，已被证明不可靠——结构化确认卡片只能由工具调用后中断产生，LLM 拼出的结果只能是文本，且 `doInterrupt` 会把中断 payload 注入回助手消息，反过来诱导模型复读该格式伪造卡片。

从 arguments 还原则是确定性的：触发中断的那次 `tool_call` 就在消息历史里，`findRecommendToolCall` 已持有它，`occupied_suborders` 原样躺在 `body_param` 中。代价是 `findRecommendToolCall` 需要额外携带原始 arguments。

**决策二：复用 `ResolveToolCall` 已解包的 `Arguments`，而非复刻 `extractLimit` 的双 schema 解析。**

工具调用参数存在 tool-proxy 信封与直连两种形态。`extractLimit` 之所以要先试 `parameters.body_param` 再试 `body_param`，是因为 `findRecommendToolCall` 虽然调用了 `toolproxy.ResolveToolCall` 却只取了 `Name`，转手把**原始**的 `tc.Function.Arguments` 传了下去——双 schema 解析实际是在补这个信息丢失。

而 `ResolveToolCall` 的 `Arguments` 本就是解包后的内层 `parameters` 对象（直连时即原始入参），两种形态在此已统一为「顶层含 `body_param`」的同一形状。因此改为在 `recommendToolCall` 上一并保留 `resolved.Arguments`，occupied 解析只需单一 schema，不必让第二处代码再对参数嵌套做假设。

`extractLimit` 维持原样不动：它当前工作正常，而 by_static / by_plan 两条路径回归敏感，顺带重构的收益不足以抵消风险，留作独立清理。

**决策三：中断判定只看增量子单数，不看合并后总数。**

若以合并后总数判定，当本次增量因余量/库存不足为 0 时，历史占用子单会把总数撑到 `>= 1`，从而弹出一张「没有任何新内容」的确认卡片，用户无从判断本次申领是否成功。保持以增量判定，空结果仍按既有 Requirement 回 `llm`，由模型走「回 R1 重新推荐有货机型」的既有分支。

**决策四：合并为顺序拼接，不做去重。**

与同文件 `mergeCandidates` 的既有取舍一致（顺序拼接 + 过滤无效项，不做组合键去重）。占用子单由模型从上一轮确认结果透传，语义上与本次增量互斥；引入去重需要定义子单的等价键（机型 + 地域 + 可用区 + 磁盘 + 计费模式），在缺乏实际重复案例前属于过度设计。

## Risks / Trade-offs

- **模型未传或漏传 `occupied_suborders`** → 本方案退化为现状（只展示增量），不会比现在更差；但也无法兜底。这是 B 方案的固有天花板，需要 C 方案才能根治。上线后应按实际 trace 评估模型传参稳定性。
- **模型把已合并清单原样作为下一轮 occupied 传入** → 属于预期用法，合并结果仍正确；但若模型同时把增量也重复塞进 occupied，因不做去重会出现重复行。上线后观察，必要时再补去重。
- **卡片上改行/删行后模型仍按旧清单传 occupied** → 展示与用户实际确认结果不一致。本方案不覆盖，记入 Open Questions。
- **`findRecommendToolCall` 结构变更** → 该函数同时服务 by_static / by_plan 两条路径，扩展返回值时需保证既有两条路径行为不变，靠单测锁定。

## Migration Plan

纯逻辑变更，无 DB、协议与配置改动，随 agent-server 镜像发布即可生效；回滚即回滚镜像，不存在数据兼容问题。

## Open Questions

- 模型在多轮增量申领中传 `occupied_suborders` 的稳定性如何？（决定是否需要推进 C 方案）
- 用户在确认卡片上编辑或删除某条子单后，下一轮增量拆分的 occupied 基准应以卡片实际内容为准还是以上一轮工具入参为准？
