# iter-biz-index-chatbot-session-sidebar 测试清单

> 工作流: `iter-biz-index-chatbot-session-sidebar`
> 关联文档: PRD / Design / API / Coding（同目录 `prd.md` / `design.md` / `api.md` / `coding.md`）
> 负责人: <开发自测人 / QA>

## 验证范围

- 功能边界: 业务视角首页 chatbot **侧边栏会话列表**——会话分组（置顶 / 历史对话：未分组 + 标签文件夹）、本地搜索过滤、置顶/重命名/删除管理能力、会话切换与 URL 同步、业务切换刷新、侧栏展开/收起。
- 业务维度接口: 会话 `list` / `create` / `update`（重命名）/ `delete` 均走 `/api/v1/agent/bizs/{bk_biz_id}/sessions/...`（`bk_biz_id` 来自路由 query `bizs`）。
- 不在范围:
  - 拖拽排序 / 跨组拖放（下一迭代）
  - 服务端搜索、`session_tag` 编辑、文件夹 CRUD（下一迭代）
  - 对话流 SSE（`agui` / `history` / `cancel`）路径业务隔离（不变）
  - 消息区 / HITL / 流式交互、顶层路由与全局布局

## 测试环境

- 前端入口: `<对应业务的测试环境地址，由执行人在测试时确认；URL 形如 /business/chatbot 且带 ?bizs=<bizId>>`
- 测试账号: `<test_account>`（需具备 `biz_agent_assistant` 业务智能体助手权限）
- 数据准备:
  - 在目标业务（有效 `bizs`）下，准备 **≥ 5 个会话**：其中 ≥ 2 个**无 `session_tag`**（未分组）、≥ 3 个分属 **2 个不同 `session_tag`**（标签文件夹），便于验证分组与排序。
  - 至少 1 个会话 `session_name` 含可搜索关键字（如「主机」），用于搜索匹配。
  - 置顶态为前端 localStorage（key 形如 `hcm:chatbot:pinned:<username>`），测试前可清空以从干净状态开始。
  - 接口失败分支：用浏览器 DevTools 把对应 `sessions/create` 或 `sessions/{code}` 请求 mock 成 500 / 断网，验证失败反馈（实现为 `console.error` 静默降级，不抛全局错误）。

## 用例清单

### P0 - 主流程（必测）

| ID | 场景 | 前置 | 操作 | 期望 | 回滚 |
|----|------|------|------|------|------|
| P0-01 | 进入页面加载业务会话列表 | 已登录, URL 带有效 `bizs`, 该业务下有会话 | 打开 `/business/chatbot?bizs=<bizId>` | 侧栏渲染会话列表; 请求走 `/api/v1/agent/bizs/<bizId>/sessions/list`; 列表按 `updated_at` 倒序 | — |
| P0-02 | 分组结构正确（置顶 / 历史对话） | 有未分组 + 有标签会话 | 观察侧栏分组 | 有「置顶」「历史对话」两大组; 历史对话内：**未分组会话平铺在上**，其下为**标签文件夹**；无置顶时不显示「置顶」标题 | — |
| P0-03 | 标签文件夹分组与排序 | ≥ 2 个标签、文件夹内 ≥ 2 会话 | 观察文件夹与组内顺序 | 每个标签一个文件夹，角标显示组内会话数；文件夹按组内 `max(updated_at)` 倒序；文件夹内会话按 `updated_at` 倒序 | — |
| P0-04 | 本地搜索过滤会话名 | 列表 ≥ 3 条，含关键字会话 | 顶部搜索框输入关键字（大小写混合） | 仅显示名称匹配项（大小写不敏感）；置顶 / 未分组 / 文件夹内同时过滤；空文件夹隐藏 | 清空搜索框 |
| P0-05 | 搜索无匹配空态 | 列表非空 | 输入不存在的关键字 | 列表区显示「无匹配会话」；清空后恢复完整列表 | 清空搜索框 |
| P0-06 | 切换会话 + URL 同步 | ≥ 2 个会话 | 点击非当前会话条目 | 主对话区切换到该会话历史；当前条目高亮；URL 变为 `/business/chatbot/<sessionCode>` 且保留 `bizs` query | — |
| P0-07 | 置顶 / 取消置顶 | 历史区某会话 | hover→更多→「置顶」；再对其「取消置顶」 | 置顶后移入「置顶」组平铺；取消后回到历史对应位置（未分组或原文件夹）；刷新页面后置顶态保持（localStorage） | 取消置顶 |
| P0-08 | 重命名会话 | 任一会话 | 更多→「重命名」→改名→Enter（或失焦） | 行内输入框出现；确认后列表名称更新；请求走 `PATCH /api/v1/agent/bizs/<bizId>/sessions/<code>` | 改回原名 |
| P0-09 | 删除会话（含二次确认） | 任一会话 | 更多→「删除」→确认弹窗→点「删除」 | 弹出「删除此会话？」危险确认弹窗；确认后该会话从列表移除；请求走 `DELETE /api/v1/agent/bizs/<bizId>/sessions/<code>`；若被删的是置顶会话，本地置顶记录同步清除 | 重新建会话 |
| P0-10 | 删除当前会话后的回退 | 当前选中会话, 列表还有其它会话 | 删除当前会话 | 自动切到列表第一个会话；若删后无任何会话则回到首页空态（不报错、不白屏） | — |
| P0-11 | 业务切换刷新列表 | 至少两个有权限业务 | 切换顶部业务（URL `bizs` 变化） | 触发 `reloadSessions`；列表刷新为新业务会话；原当前会话在新业务不存在时回到首页空态 | — |

### P1 - 异常 / 边界（必测）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | 新对话进入首页空态（惰性建会话） | 任意 | 点击「新对话」按钮 | 回到首页空态（清空当前选中与输入，URL 去掉 sessionCode），**不立即创建会话**；首次发送消息时才创建会话并归入对应分组 |
| P1-02 | 创建会话接口失败 | mock `sessions/create` 返回 500 | 首页空态发送首条消息触发创建 | 不抛全局报错；控制台有 `[Session] createSession failed`；列表不新增脏数据 |
| P1-03 | 重命名接口失败回滚 | mock `PATCH sessions/<code>` 500 | 重命名并确认 | 名称先乐观更新后**回滚为原名**；控制台有 `renameSession failed` |
| P1-04 | 删除接口失败 | mock `DELETE sessions/<code>` 500 | 删除并确认 | 该会话**不从列表移除**；控制台有 `deleteSession failed` |
| P1-05 | 侧栏收起 / 展开 | 任意 | 点击搜索行右侧收起按钮；再点收起胶囊的展开按钮 | 收起后左上出现圆角胶囊浮条（展开 chevron + 新对话气泡+加号）；展开后分组、当前选中、文件夹展开态保持一致 |
| P1-06 | 文件夹展开 / 收起 | 有标签文件夹 | 点击文件夹行切换 | 默认全部展开（实心文件夹图标 + 子项 + 左侧虚线）；点击收起隐藏子项、切换为空心文件夹图标；角标数量正确 |
| P1-07 | 无 `bizs` / 无效业务 | URL 不带 `bizs` 或业务无效 | 进入 chatbot | 不发起会话 CRUD；侧栏列表为空，不报错 |
| P1-08 | 仅置顶 / 仅历史 / 空列表边界 | 构造对应数据 | 分别观察 | 只展示对应大组；空列表时列表区空态、「新对话」仍可用；均无报错无白屏 |
| P1-09 | 收起态新建对话 | 侧栏收起 | 点击胶囊浮条下方「新对话」图标 | 同 P1-01 行为（回到首页空态） |

### P2 - UI 细节（按需）

- [ ] 会话行 hover 背景 `#F0F1F5`；置顶区右侧常驻置顶图标，历史区 hover 才出现「更多」三点图标
- [ ] 当前选中会话有明确激活态（实现为蓝底 `#e1ecff` / 蓝字，区别于设计稿灰底 `#F0F1F5`——见下方备注）
- [ ] 会话名超长截断 + tooltip 表现与现网一致
- [ ] 「新对话」为渐变蓝底主按钮，文案「新对话」，左侧对话气泡+加号图标
- [ ] 搜索框占位「搜索对话」，背景 `#F0F1F5`，右侧搜索图标；收起窄条态不展示搜索
- [ ] 侧栏背景白色 `#FFFFFF`，宽度约 256px；列表过长可滚动
- [ ] 文件夹角标灰色圆角、子项左缩进 24px + 左侧竖向虚线引导

## 备注（设计/编码差异，便于测试判读，非缺陷）

1. **「新对话」行为**：design §3.2 描述「创建会话并清空输入」，实际实现为**回到首页空态、首次发送时惰性创建会话**（避免产生空会话）。以实现为准，验收按 P1-01。
2. **当前会话激活态配色**：design §3.3 / §8 标注选中态用灰底 `#F0F1F5`，实际实现用蓝底 `#e1ecff` + 蓝字。视觉差异，若产品要求严格对齐稿面需回 coding 调整；当前按实现验收。
3. **新对话图标**：coding.md 曾记为 bkui `Assistant`，实际实现使用 iconfont `bkhcm-icon-chat-plus`。
4. **文件夹图标**：实际使用 `bkhcm-icon-folder-open` / `bkhcm-icon-folder-close`（展开/收起），与 coding.md 中「`bkhcm-icon-file`」记述不同，以实现为准。
5. **接口失败反馈**：当前 create/rename/delete 失败为 `console.error` 静默降级（rename 会回滚），无 Toast 提示。若需要可感知的失败 Toast，属增强项，可记入下一迭代。

## 验证结论

> 每条用例填写：PASS / FAIL / Skipped + 简要说明（FAIL 必须附复现步骤、错误截图链接或日志片段）

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | <PASS / FAIL / Skipped> | |
| P0-02 | <PASS / FAIL / Skipped> | |
| P0-03 | <PASS / FAIL / Skipped> | |
| P0-04 | <PASS / FAIL / Skipped> | |
| P0-05 | <PASS / FAIL / Skipped> | |
| P0-06 | <PASS / FAIL / Skipped> | |
| P0-07 | <PASS / FAIL / Skipped> | |
| P0-08 | <PASS / FAIL / Skipped> | |
| P0-09 | <PASS / FAIL / Skipped> | |
| P0-10 | <PASS / FAIL / Skipped> | |
| P0-11 | <PASS / FAIL / Skipped> | |
| P1-01 | <PASS / FAIL / Skipped> | |
| P1-02 | <PASS / FAIL / Skipped> | |
| P1-03 | <PASS / FAIL / Skipped> | |
| P1-04 | <PASS / FAIL / Skipped> | |
| P1-05 | <PASS / FAIL / Skipped> | |
| P1-06 | <PASS / FAIL / Skipped> | |
| P1-07 | <PASS / FAIL / Skipped> | |
| P1-08 | <PASS / FAIL / Skipped> | |
| P1-09 | <PASS / FAIL / Skipped> | |

- 执行人: <DEVELOPER_NAME>
- 执行日期: 2026-06-05
- 总体结论: Skipped（用户决定本迭代不在 workflow 内执行手测，直接推进到 done；上方 P0/P1 用例清单保留供后续自测 / QA 复测使用）
- 后续行动: 直接合入；如发现问题再回退到 coding 阶段修复
