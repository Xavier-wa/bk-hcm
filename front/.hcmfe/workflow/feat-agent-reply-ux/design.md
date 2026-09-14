# Design：agent回复交互体验优化-前端

> 编写前已通过 `blueking-figma-dev` §0–5.5（经 `wf-design-figma-intake` 包装早停）识别稿面。冲突以稿面 + 用户确认为准。**本阶段不出业务代码。**
>
> 工作流：`feat-agent-reply-ux`  
> TAPD：`https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137818447`  
> PRD：`.hcmfe/workflow/feat-agent-reply-ux/prd.md`

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| 同一轮思考 + 工具调用收进一个过程区；摘要行点击展开/收起 | 独立「展开全部 / 收起」工具栏按钮 |
| 流式默认展开；正常结束 / 非用户中断 / 历史回放默认收起；用户 Stop 保持展开 | 新做结构化入参/出参浏览器（内部仍是描述/参数/返回纯文本，对齐稿面字号色） |
| 同一轮过程、中间说明、最终正文/卡片共用一块浅底（对应稿面「AI Content Bubble」） | 改气泡头图/标题文案、点赞复制、输入框、会话列表 |
| 工具行改稿面外观；参数 `tool_intent` 单独一行画在工具行上方 | 前端工具名中文映射；改 `/history` 顺序或补字段 |
| 全页 chatbot + 浮窗 `ai-assistant` + 历史回放同一套 | 稿面里的 Alert / Table / 确认提交按钮（已有申领卡，不重做） |
| | 后端 / 新 HTTP / 新权限点 |

## 1. 设计稿引用

| 状态 | Figma | Node |
|------|-------|------|
| 展开（过程 + 工具详情 + 思考正文 + 结论卡） | [业务资源 · 首页](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/业务资源?node-id=2167-21158) | `2167:21158`（过程主体 `2167:21170`） |
| 收起（过程摘要 + 结论仍露出） | [业务资源 · 收起态](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/业务资源?node-id=2229-17671) | `2229:17671`（过程摘要 `2229:17688`） |

取数说明：整页节点过大，已下钻 `2167:21170` 拿到展开结构；收起页用 `disableCodeConnect` 取到「可用」上下文（screenshot + 生成结构）。TAPD 描述图是**现状堆叠**，不是目标稿。

## 2. 信息架构 / 布局

同一轮助手回复落在一块「AI Content Bubble」里（白底、`#f1f5f9` 描边、圆角 16、阴影 `0 12px 32px rgba(0,0,0,0.04)`、内边距 16 / 24 / 24、间距 16）。自上而下：

> **产品覆盖稿面**：不设最大宽 900。900 比没有过程区的一轮（消息组内容宽 = 1000 − 16 右内边距 = 984）窄一截，两种轮次并排时宽度不齐；统一取更宽的 984（即铺满消息组）。

```
[气泡头] 图标 + 本轮标题（现网已有，本期不改）
──────── 分割线 #eaebf0
[过程区]
  摘要行（唯一折叠入口）
  展开后按时间顺序：
    · tool-intent 说明（若有）
    · 工具行（标题视实际调用；现网为「调用工具 xxx」+ 状态/耗时）
    · 工具内部详情（自绘「描述 / 参数 / 返回内容」，不走 chat-x `ToolcallRender`）
    · 思考正文
[中间说明] 工具之间的叙述（如库存不足）——进浅底，不随过程区收走
[最终结论] 正文 / HITL / 申领卡 / 表格 ——始终露出
```

- 用户气泡、相邻轮次不套进这块浅底。
- 稿面把「已调用 N 个工具」和「思考了 X 秒」拆成**两行**；产品覆盖为**一行摘要**（见 §6）。
- 稿面工具块里的「描述：…」是工具 schema / 内部详情文案，**不是** `tool-intent`。intent 按 PRD 画在工具行**上方**；稿面未单独画出该行。
- 浮窗与全页共用 `ChatMessageList` → `MessageContainer`，布局规则只做一处。

视觉量级（稿面变量，落地以项目已加载 Token / 相邻样式为准）：

| 元素 | 稿面 |
|------|------|
| 摘要行文字 | 12 / 行高 20；展开 `#4d4f56`，收起 `#979ba5` |
| 摘要行箭头 | 16px；展开「箭头-下」，收起「箭头-右」 |
| 工具行 | 高 24、左右 12、底 `#f5f7fa`、底边 `#eaebf0` |
| 工具名 | 12 / `#4d4f56`。**标题视实际调用，不写死稿面「MCP调用：服务 / 方法」**。现网为 `调用工具 {name}`（完成态常见「调用工具: xxx 调用成功」）；仅当该次调用本身是 MCP 且协议已带 `mcpName` 时，才可用 `MCP调用：{mcpName} / {name}`。不做工具名中文映射。 |
| 工具行右侧耗时 | 12 / `#c4c6cc`，稿面示例 `650ms`（有 duration 才画） |
| 思考/工具详情底 | `#fafbfd`，内边距 12×8，圆角 4，正文 12 / `#979ba5` |

## 3. 关键交互

1. **摘要行是唯一开关**：整行可点（箭头 + 文案），无独立「展开全部 / 收起」按钮。再点一次收回过程，结论始终可见。
2. **流式**：本轮未结束时过程区保持展开，能看到当前思考与进行中的工具。
3. **正常结束 / 异常中断（非用户 Stop）/ 历史回放**：过程区自动收起，只留一行摘要。
4. **用户点「停止生成」**：保持当时展开态；摘要按已发生工具数与已思考时长结算。
5. **展开内容顺序**：按事件时间；每次工具 = intent 行（可缺）+ 自绘工具行 + 自绘内部详情。
6. **点击摘要行不发会话接口**（AC-P01，100ms 内完成）。
7. **无思考且无工具**：不渲染过程区。

## 3.x 关键图标语义（Coding 必读）

> **语义列**由 design 主流程填写（只写视觉语义，不写项目 CSS 类名）。  
> **项目候选列**已由 `hcm-design-icon-intake` 回填。

| 稿面位置 | 语义描述 | Node | 项目候选（类名 / 组件，可空） |
|----------|----------|------|------------------------------|
| 过程区摘要行（展开） | 16px 线性向下箭头，表示已展开、再点可收起 | `2167:21685` / `2167:21709` | `AngleDown`（`bkui-vue/lib/icon`）。产品覆盖稿面「向上」：展开朝下、收起朝右 |
| 过程区摘要行（收起） | 16px 线性向右箭头，表示已收起、再点可展开 | `2229:17690` / `2229:17693` | `AngleRight`（`bkui-vue/lib/icon`） |
| 工具行左侧状态（成功） | 16px 圆形对勾，表示该次工具调用成功 | `2167:21690` | `<i class="hcm-icon bkhcm-icon-circle-success" />`（项目刚补的 iconfont，勿用 `check-circle-fill` / bkui `Success`） |
| 工具行左侧状态（失败） | 稿面无独立失败稿；用同系列圆形错误 | — | `<i class="hcm-icon bkhcm-icon-circle-error" />` |
| 工具行左侧状态（进行中） | 稿面无独立进行中稿；用同系列圆形转圈环 | — | `<i class="hcm-icon bkhcm-icon-loading-circle" />` + `animation: rotate`，14px |
| 工具行右侧耗时 | 14px 线性时钟，缀在 duration 左侧 | `2167:21693` | `<i class="hcm-icon bkhcm-icon-circle-time" />`（项目刚补的 iconfont，勿再兜底 bkui Clock/Time） |
| 气泡头「云」/ 成功圆对（24px） | 现网助手头图，本期不改 | `2167:21172` / `2229:18349` | N/A |

进行中工具行稿面未给独立 icon variant：骨架与成功态相同，状态位用项目 iconfont 的 `bkhcm-icon-loading-circle` 配 `animation: rotate`（同 `OrganizationSelect` 的既有写法），成功 / 失败 / 耗时用上面三枚 `circle-*`。工具没有结果且本轮已结束（Stop / 异常 / 历史回放）时状态未知，状态位留空。

进行中这一枚取 14px，比稿面给定的 16px 状态位小一档：转圈环是细描边，实测 16px 时视觉重量明显压过 `circle-success`，14px 才和它等重（`line-height` 仍留 16px，保证同列各状态基线一致）。先前用过 `<bk-loading size="mini">`，它把 8 枚 oval 铺满整个 16×16，比字形状态位更显眼，已弃用。

## 3.y 组件候选（Coding 必读）

> 通识列来自 `blueking-figma-dev` §4–5。项目三列已由 `hcm-design-comp-intake` 回填。

| 稿面区域/语义 | 组件候选 | 体系（bkui / magic / 扩展包） | 文档确认（已读 reference / 未读） | 复用层级（page/comp/adjacent/none，可空） | 落码入口（skill 名或「无」，可空） | 项目路径/说明（可空） |
|---------------|----------|------------------------------|----------------------------------|------------------------------------------|-----------------------------------|----------------------|
| 过程区折叠（单摘要行、受控展开） | 优先业务组合（chat-x 消息组 + 自管展开态）；bkui `Collapse` 仅作对照，稿面不是多面板手风琴 | bkui / `@blueking/chat-x` | 未读（本阶段不出码；Collapse 多半不采用） | adjacent | 无 | `src/components/chatbot/custom-message-card.vue`（摘要行展开/收起）；`src/components/chatbot/chat-message-list.vue` + `useMessageGroup` |
| 工具行外观（现网「调用工具 xxx」，非写死 MCP 文案） | **必须手写**语义 HTML/CSS 行；稿面与 chat-x `ToolcallRender` 外壳不一致，禁止套组件再改皮肤 | 手写布局 | N/A（非基础组件） | none | 无 | `src/components/chatbot/process-tool-row.vue`；标题跟实际调用（`mcpName` 有值才用 MCP 格式） |
| 工具内部详情 | 手写「描述 / 参数 / 返回内容」三段纯文本，对齐稿面 `#fafbfd` / 12px / `#979ba5`；不是结构化参数浏览器 | 手写布局 | N/A | none | 无 | 同 `process-tool-row.vue`；数据仍来自协议 `description` / `arguments` / tool result，不复用 `ToolcallRender` |
| 思考正文 | 现有 chat-x `ReasoningMessage` / 相邻推理气泡 | 扩展包 chat-x | 未读 | adjacent | 无 | chat-x `ReasoningMessage`；事件在 `src/hooks/chatbot/use-event.ts` |
| 同一轮浅底 / 消息组 | 现有 `MessageContainer` + `useMessageGroup`；必要时包一层浅底 | 扩展包 chat-x | 未读 | adjacent | 无 | `src/components/chatbot/chat-message-list.vue`；全页 `src/views/chatbot/index.vue` 与浮窗 `src/components/ai-assistant/index.vue` 共用 |
| 稿面 Alert / Table / Button（申领结论） | 现有 HITL / 主机申领卡，**本期不重做** | 项目业务封装 | N/A | adjacent | 无 | `src/components/chatbot/hitl-interrupt-card.vue`、`host-apply-*-card.vue`；本期不改 |

待确认（map + skill 索引未命中、需手写等）：

- 过程区折叠更接近「一行摘要 + 受控内容」，静态 map 的 `Collapse` 语义接近但交互不符（两段标题、默认手风琴）。默认按相邻 chat-x + 手写摘要行落地；若 coding 发现必须用 bkui Collapse，再读组件 skill reference。
- Figma Code Connect 未映射；本阶段已用 `disableCodeConnect` 取数，不阻塞 Design。
- `tool-intent` 说明改从该次调用参数 `tool_intent` 读取（`/agui` 的 `TOOL_CALL_ARGS` 与 `/history` 的 `toolCalls[].function.arguments` 同一字段）。旧 `tool-intent-*` 消息只丢弃。配对成功后不再单独占助手气泡。

## 4. 状态流转

```
无过程内容 ──不渲染过程区──► 只展示结论/中间说明（若有）

有思考和/或工具
        │
        ▼
   [流式中] 强制展开
        │
        ├─ 正常 RunFinished / 非用户中断 ──► 自动收起
        ├─ 用户 Stop ──► 保持展开，摘要按已发生内容结算
        └─ 历史 SNAPSHOT / 回放 ──► 按已结束，默认收起

收起 ◄──点击摘要行──► 展开
```

摘要文案（单位小写 `s`、两位小数）：

| 条件 | 文案 |
|------|------|
| 有工具（N≥1） | `调用N个工具，思考耗时X.XXs` |
| 仅思考 | `思考耗时X.XXs`（禁止「调用 0 个工具」） |
| 有工具、拿不到耗时（`/history`） | `调用N个工具` |
| 仅思考、拿不到耗时（`/history`） | `已完成思考` |
| 无思考且无工具 | 不展示过程区 |

耗时只有前端计时一个来源（协议无 duration 字段，见 api.md §2.4）；`/history` 重放不出这段时钟，此时思考条标题只写「已思考完成」、摘要省掉耗时段，**不补 0**。工具行右侧稿面是 `650ms`：前端按 `toolCallId` 从 `TOOL_CALL_END` 掐到 `TOOL_CALL_RESULT`（不含模型吐入参那段），有值才展示，单位沿用协议/现网（ms）；**不要**把摘要行的 `X.XXs` 套到工具行上，也不进摘要汇总。

## 5. 异常 / 边界态

| 场景 | 展示 |
|------|------|
| 缺 `tool_intent` 或文案为空 | 不画说明行，只保留样式化工具行 |
| `/history` 中仍夹着旧 `tool-intent-*` 消息 | 丢弃该条；说明从对应 `toolCall` 参数取，不另出气泡 |
| 工具进行中 / 失败 | 行骨架与成功态相同；失败左侧用 `bkhcm-icon-circle-error`，成功用 `bkhcm-icon-circle-success`，耗时用 `bkhcm-icon-circle-time` |
| 仅思考、无工具 | 收起后只有「思考耗时…」 |
| 用户 Stop 时还没有结论 | 过程区保持展开；空白助手泡沿用现网「已停止生成」 |
| 中间说明（工具之间的叙述） | 留在浅底内、过程区外，收起过程时仍可见 |
| 无 `agent_assistant` | 入口与现网一致，不新增权限点 |
| 浮窗 vs 全页 | 同一 `ChatMessageList`，行为一致 |

稿面未定义 hover / disabled / 空态 variant；摘要行沿用现网可点击文字色，不做额外悬浮规范。

## 6. 与 PRD 差异（如有）

| 项 | PRD | 稿面/确认结论 |
|----|-----|---------------|
| 过程摘要行数 | 合并一行：`调用N个工具，思考耗时X.XXs` | 稿面拆成「已调用 N 个工具」+「思考了 X 秒」两行。**以 PRD / 评论覆盖为准，合并一行** |
| 摘要用词与单位 | `调用` + `思考耗时` + 小写 `s` + 两位小数 | 稿面「已调用」「思考了 4.5 秒」。**以 PRD 为准** |
| 工具行标题 | 改稿面样式，未规定写死「MCP调用」 | 稿面示例 `MCP调用：bkui-vue3 / get-component`。**以实际调用为准**：现网是 `调用工具 xxx`，不把所有行改成 MCP 前缀 |
| 工具内部详情 | Q-001 原意是不新做结构化参数浏览器 | 展开稿是「描述/参数/返回内容」纯文本。**外壳与详情都手写对齐稿面**，不套 `ToolcallRender`；仍不做 JSON 浏览器 |
| `tool-intent` 位置 | 工具行上方单独一行 | 稿面未单独画 intent；块内「描述：」视为内部详情。**按 PRD 加一行** |
| 收起范围 | 只收思考 + 工具过程 | 稿面收起后结论卡仍在同一气泡内。对齐 PRD |
| 气泡头 / 申领表 / Alert | 不在本期改 | 稿面完整画了头图、Alert、表格、按钮。**只当环境参考，不重做** |

## 7. 与 PRD 验收映射

| PRD | Design |
|-----|--------|
| AC-001 结束自动收起 + 合并摘要 + 结论仍见 | §3 / §4 收起态；文案表；浅底内结论不进过程区 |
| AC-002 点摘要行展开再收起 | §3.1 唯一入口 |
| AC-003 流式保持展开 | §4 流式强制展开 |
| AC-004 用户 Stop 保持展开并结算摘要 | §4 Stop 分支 |
| AC-005 仅思考摘要 | §4 文案表 |
| AC-006 / AC-007 / AC-007b 工具说明 | §2 顺序；§5 缺省；说明取参数 `tool_intent` |
| AC-008 同一浅底 | §2 AI Content Bubble；中间说明不随过程收走 |
| AC-009 全页 / 浮窗 / 历史 | §0 / §2 共用 `ChatMessageList`；历史按已结束收起 |
| AC-010 内部详情不新做结构化面板 | §3.y 自绘三段文本；§6 差异 |
| AC-P01 100ms、不发新会话接口 | §3.6 |
| AC-S01 权限不变 | §5 |

## 8. 实现边界（design 纪要）

> 对应 `blueking-figma-dev` §5.5。写入此处 **不等于** 授权改业务代码；落码在 coding 阶段。

| 项 | 内容 |
|----|------|
| 目标目录意向 | `front/src/components/chatbot/`（`chat-message-list.vue` 及过程区封装）；`front/src/hooks/chatbot/`（`use-event.ts` / stream / history 的工具说明拆分与过程收起时机）。全页 `views/chatbot` 与浮窗 `ai-assistant` 已共用列表，原则上不各写一套。 |
| 组件/手写边界 | 摘要行 + 浅底 + **整段工具展示**（行头 + 描述/参数/返回）：手写 `process-tool-row.vue`，不套 chat-x `ToolcallRender`。思考正文仍走 chat-x `ReasoningMessage`。结论卡：不动现有 HITL / 申领卡。不新引入依赖。 |
| 明确不做 | 独立全量按钮；稿面完整参数面板；把工具行统一改成「MCP调用：服务 / 方法」；工具名中文表；改 `/agui` `/history`；改后端；改输入框 / 会话列表 / 点赞复制；重做稿面 Alert/Table/按钮。 |
| 数据（真接口 / mock） | 真接口。说明取 `toolCalls[].function.arguments.tool_intent`；不新增会话 API。 |

栈备忘：Vue 3.5 + `bkui-vue` 2.1.0-beta.4 + `@blueking/chat-x` 0.0.50。
