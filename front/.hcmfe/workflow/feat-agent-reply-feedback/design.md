# Design：用户主动反馈-前端实现（无设计稿）

> **本迭代无独立设计稿**。豁免原因：用户确认交互与控件形态跟随 `@blueking/chat-x` 既有点赞/点踩，沿用现网对话工具栏，不另出 Figma。本阶段不出业务代码。

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| 全页 / 浮窗 Agent 对话里，对每一轮助手回复展示 chat-x 👍 / 👎 | 运营看板、监控、模型评估 |
| 打开现网已隐藏的 like / unlike，接入原因面板与提交回调 | 结束后整段对话弹窗打分 |
| 点踩预设原因用 PRD 四条文案 | 独立「3 秒撤销」、仅点踩出面板、单选 chips |
| | 新图标、新页面、新菜单 |

## 1. 设计稿引用

| 状态 | 说明 |
|------|------|
| N/A | 无新 Figma 稿；视觉以 chat-x `MessageTools` + `MessageUserFeedback` 现网表现为准 |

## 2. UI 参照

- **现网**：`src/components/chatbot/chat-message-list.vue`（全页 `views/chatbot` 与浮窗 `components/ai-assistant` 共用该列表）
- **现状**：`updateTools` 将 `like` / `unlike` **hidden: true**，工具栏只留复制 / 重新生成
- **目标**：取消隐藏，走 chat-x 默认反馈：点 👍/👎 → 弹出 `MessageUserFeedback` → 提交走 `onAgentFeedback`
- **PRD 文字约束**：覆盖全部 Agent 场景；点踩四条原因；不在 HCM 做看板；交互跟随 chat-x

## 3. 关键交互

1. 助手消息组完成后，工具栏常驻 👍 / 👎（chat-x 默认，不手写按钮）。
2. 点击 like / unlike：chat-x 弹出原因面板（骨架 → 原因标签 + 补充说明 + 提交/取消）。
3. 点赞原因列表：跟随 chat-x / 现网默认文案，本迭代不另拟。
4. 点踩原因列表：注入 PRD 四条（可多选）。补充说明占位跟随 chat-x。
5. 提交：`onAgentFeedback(tool, messages, reasonList, otherReason)` 上报该轮反馈。
6. 取消 / 点面板外 / 流式时工具栏是否可点：全部跟 chat-x，不另定。
7. 不新增独立撤销按钮。

## 3.x 关键图标语义（Coding 必读）

| 稿面位置 | 语义描述 | Node | 项目候选（类名 / 组件，可空） |
|----------|----------|------|------------------------------|
| 助手消息组工具栏 · 赞 | 正向反馈入口 | N/A | chat-x `like` 内置图标，不引入 `bkhcm-icon-*` |
| 助手消息组工具栏 · 踩 | 负向反馈入口 | N/A | chat-x `unlike` 内置图标，不引入 `bkhcm-icon-*` |

## 3.y 组件候选（Coding 必读）

| 稿面区域/语义 | 组件候选 | 体系（bkui / magic / 扩展包） | 文档确认（已读 reference / 未读） | 复用层级（page/comp/adjacent/none，可空） | 落码入口（skill 名或「无」，可空） | 项目路径/说明（可空） |
|---------------|----------|------------------------------|----------------------------------|------------------------------------------|-----------------------------------|----------------------|
| 助手消息工具栏 👍/👎 | MessageContainer / MessageTools（`like` `unlike`） | 扩展包 `@blueking/chat-x` | 已读 message-container、user-feedback | adjacent | 无 | `src/components/chatbot/chat-message-list.vue`：去掉 hidden，接 `on-agent-action` / `on-agent-feedback` |
| 点赞/点踩原因面板 | MessageUserFeedback（由 MessageTools 内置弹出） | 扩展包 `@blueking/chat-x` | 已读 user-feedback | adjacent | 无 | 不单独挂载；`onAgentAction` 在 like/unlike 时返回原因字符串数组 |
| 全页 / 浮窗对话宿主 | 现有 chatbot 双入口 | 项目封装 | 已读 docs/modules/chatbot.md | adjacent | 无 | `views/chatbot`、`components/ai-assistant` 已共用 `chat-message-list`，无需新页面 |

待确认：无。不走 `page-*` / `comp-*`（非整页列表/表单，非抽屉表格）。

## 4. 状态流转

```
助手回复进行中（streaming）
  → 工具栏可用性跟 chat-x（现网常用：流式 disabled）
助手回复结束（complete）
  → 工具栏可见 like / unlike
用户点 like 或 unlike
  → 面板 loading → 展示原因
用户提交
  → 回调上报；按钮高亮等跟 chat-x
用户取消
  → 面板关闭，不保留本次填写
```

## 5. 异常 / 边界态

| 场景 | 展示 |
|------|------|
| 原因列表异步未返回 | chat-x 骨架屏 |
| 上报失败 | 沿用项目 Message 提示；不自造失败页 |
| HITL / 申领卡片消息组 | 仍走同一 MessageContainer 工具栏；不在自定义卡片上另做一套 👍/👎 |
| 无 Agent 回复 | 无工具栏，无入口 |

## 6. 与 PRD 差异（如有）

| 项 | PRD 原文倾向 | 确认结论 |
|----|--------------|----------|
| 仅点踩出面板、👍 无弹窗 | TAPD 正文 | **改**：👍/👎 都出 chat-x 原因面板 |
| 原因单选、必须选 chip | TAPD 正文 | **改**：chat-x 多选；可只填补充说明提交 |
| 3 秒撤销 / 就地展开 | TAPD 正文 | **改**：不实现；跟 chat-x Tippy 面板 |
| 点踩四条文案 | PRD F-003 | **保留**，作为 unlike 的 `reasonList` |

## 7. 与 PRD 验收映射

| PRD | Design |
|-----|--------|
| AC-001 常驻 👍/👎 | 取消 hidden，跟 chat-x 工具栏时机 |
| AC-002 点赞出面板并提交 | like → UserFeedback → onAgentFeedback |
| AC-003 点踩四条 + 补充说明 | unlike 的 reasonList = PRD 四条 |
| AC-004 细节跟 chat-x | 不验收 3 秒撤销 |
| AC-005 全页 + 浮窗 | 只改共用 `chat-message-list.vue` |
| AC-006 无看板 | 不新增页面/菜单 |

## 8. 实现边界（design 纪要）

| 项 | 内容 |
|----|------|
| 目标目录意向 | 仅改 `src/components/chatbot/chat-message-list.vue`（及如需抽出的反馈文案常量 / 上报函数，仍落在 chatbot 模块） |
| 组件/手写边界 | **禁止**手写 👍/👎 与原因面板；必须用 chat-x 工具栏 + 内置 UserFeedback |
| 明确不做 | 看板、新图标、page/comp 骨架、改后端 |
| 数据 | 用户要求跳过独立 api 阶段进 coding。上报契约在 coding 标「需后端配合」；未就绪前可先接通回调与本地提示，不造假看板 |
