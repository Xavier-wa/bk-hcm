# Coding：aiagent - chatbot 自定义消息展示 - 指令模板模式（主机申领）

> 设计原则：**沿用 jsonschema 迭代已建立的 CUSTOM → `__type` 消息 → 卡片渲染 → resume 回写** 机制，
> 新增并列的两类主机申领模板（推荐方案 A / 预提单确认 B）+ 调整配置弹窗 C，**不重构** HITL / account_select。
> 本迭代先以 mock 打通 UI/UX，后端真实协议落地后已按真实 eventstream 校准（见 §11）。

## 1. 变更摘要

| 文件 | 变更 | 类型 |
|---|---|---|
| `hooks/chatbot/types.ts` | 新增 `HostApplyDisk` / `HostApplySuborder` / `HostApplyRecommendation` / `HostApplyRecommendValue` / `HostApplyPreorderValue` 及两个消息扩展类型；导出事件名常量 `HOST_APPLY_RECOMMEND_EVENT` / `HOST_APPLY_CONFIRM_EVENT` | 改 |
| `hooks/chatbot/use-event.ts` | `CUSTOM` 分支按事件名 push 消息（`__type: 'host_apply.recommend' / 'host_apply.preorder'`） | 改 |
| `hooks/chatbot/use-stream.ts` | `parseHostApplyRecommendValue` / `parseHostApplyPreorderValue` 解析 + `toHistoryMessage` 按事件名重建；`streamChat` 透传 `forwardedProps` | 改 |
| `hooks/chatbot/use-host-apply.ts` | **新增**：识别 / 内容提取 / 只读态推断（A 用 `__selectedIndex`、B 用 `__confirmedSuborders`） | 新增 |
| `hooks/chatbot/host-apply-display.ts` | **新增**：`suborder` 字段 label 映射、顺序、键值表/磁盘格式化（原样展示编码） | 新增 |
| `hooks/chatbot/use-chatbot.ts` | `sendMessage(content, sessionTag?, resumeValue?, forwardedProps?)` 透传结构化 `forwardedProps` | 改 |
| `store/chatbot/agent.ts` | `streamChat` 合并 `resumeValue` + `forwardedProps` 到 body `forwardedProps` | 改 |
| `components/chatbot/custom-message-card.vue` | **新增**：自定义消息卡通用外壳（标题/横幅/主体/操作区 + 只读收起/展开 + 全页/浮窗响应式容器查询） | 新增 |
| `components/chatbot/host-apply-recommend-card.vue` | **新增**：模板 A 推荐方案卡（翻页 1/N、来源标签、选择方案/添加到配置清单） | 新增 |
| `components/chatbot/host-apply-preorder-card.vue` | **新增**：模板 B 预提单卡（表格 + 全屏最大化 + 行内编辑 + 确认方案/添加到配置清单） | 新增 |
| `components/chatbot/host-apply-preorder-table.vue` | 预提单表格（按 `suborder` 字段渲染列，行尾编辑 icon）；**新增 `show-disk` 可选列**（系统盘/数据盘，供 D 复用） | 改 |
| `components/chatbot/host-apply-adjust-dialog.vue` | **新增**：模板 C 调整配置弹窗（左侧表单 + 校验 + 写回 `suborder`） | 新增 |
| `components/chatbot/host-apply-submit-card.vue` | **新增**：模板 D 确认提交卡（只读表格 + 全屏 + 固定「确认提交/添加到配置清单」按钮，无操作列） | 新增 |
| `components/chatbot/chat-message-list.vue` | `#default` slot 增加 recommend / preorder / **submit** 分支，接 `onSelect` / `onConfirm` / `onAddToList` / `onAction` + resume 回写 | 改 |

## 2. 数据模型（types.ts）

- `__type` 内部标记保持稳定短名：`'host_apply.recommend'` / `'host_apply.preorder'`，由解析层从事件名映射，避免长事件名散落各处。
- `suborder` 字段值为后端编码（`region=ap-nanjing`、`charge_type=PREPAID`、`require_type=7`、`disk_type=CLOUD_PREMIUM` 等），**本期原样展示**。
- 详细类型见 api.md §9。

## 3. 下行解析（use-event.ts + use-stream.ts）

- `CUSTOM` 分支与 `hitl.interrupt` / `account_select.interrupt` 并列，按 `name === HOST_APPLY_RECOMMEND_EVENT / HOST_APPLY_CONFIRM_EVENT` 分流，`JSON.parse(value)` 后 push 带 `__type` 的 assistant 消息。
- 历史回放 `toHistoryMessage`：`role==='activity' && activityType==='CUSTOM'` + 事件名命中 → `parseHostApply*Value` 重建（无 id，校验仅判对象 / `suborder` 存在）。

## 4. 上行回写（resume，⚠️ 承载形式待后端确认）

- 链路：卡片 `onSelect/onConfirm` → `chat-message-list` handler → `sendMessage(content, undefined, resumeValue?, forwardedProps?)` → `use-stream.streamChat` → `agent.streamChat` 合并进 body `forwardedProps`。
- **A 选择方案**（`handleSelectPlan`）：记录 `__selectedIndex`，`resumeValue = '帮我基于此方案进行拆单' + JSON.stringify(选中 suborder)`，content=「我选择该申领方案」。
- **B 确认方案**（`handleConfirmPreorder`）：记录 `__confirmedSuborders`，`resumeValue = JSON.stringify(suborders)`（含 C 修改），content 按是否编辑区分文案。
- **D 提交申请单**（`handleSubmitConfirm`）：协议无 `actions`，卡片固定两按钮——「确认提交」记录 `__submitted`、`resumeValue = JSON.stringify(D 的整个 data，含 body_param/path_param)`（同 B 通道）；「添加到配置清单」复用 `handleAddToList` 跳转（不回写，同 A/B）。表格可用区兼容 `zones: string[]`（join 展示）与旧 `zone`。
- A、B、D 承载均为 JSON 字符串，走 `forwardedProps.resumeValue` 通道。
- 因协议无 id，以上为**合理假设**，逻辑集中在 `handleSelectPlan`/`handleConfirmPreorder`/`handleSubmitConfirm`，后端确认 resume 期望值后改这几处即可（见 api.md §6 / QA3）。

## 5. 卡片渲染

- 通用外壳 `custom-message-card.vue`：标题 / `header-extra`（翻页/全屏）/ 主体 / `actions` 插槽 + 只读收起摘要/展开只读态；以 `container-type: inline-size` + `@container` 实现全页（宽）/浮窗（窄）自适应。
- **A 推荐方案卡**：键值两列表（`toSpecDisplayItems` 过滤空值）；多条翻页 1/N；来源标签（`user`→历史配置 / `biz`→业务推荐）；按钮「选择方案」「添加到配置清单」。
- **B 预提单卡**：`bk-table` 渲染 suborders；右上「全屏」最大化覆盖层（全页保留导航 52px / 浮窗整屏，ESC 退出、锁背景滚动）；行尾编辑 icon 触发 C 弹窗；按钮「确认方案」「添加到配置清单」。
- **C 调整弹窗**：`bk-form` 左侧表单（`replicas/region/device_type` 必填），计费模式只读，系统盘/数据盘可编辑（磁盘类型静态枚举对齐后端编码）；保存写回该行 `suborder`，标记 `edited`。
- **D 确认提交卡**：`getSubmitRows` 把嵌套 `suborders[].spec` + 外层 `replicas` + `body_param.require_type` 拍平为 `HostApplySuborder[]`，复用预提单表格（`readonly` 隐藏操作列 + `show-disk` 增列系统盘/数据盘，**全程不可编辑**）；右上全屏复用 B 的最大化交互；协议无 `actions`，**固定渲染**「确认提交」（主按钮，回写 suborders）/「添加到配置清单」（跳转）；交互态横幅「已确认 N 条…」、只读摘要「请确认全部方案信息，并提交申请单」。

## 6. 只读 / 历史（use-host-apply.ts）

- 只读态推断对齐 account_select：存在显式提交标记（`__selectedIndex` / `__confirmedSuborders`）或后续 user 消息 → 只读；否则可交互。
- A 只读默认展示已选下标方案（历史回放无标记退化为首条）；B 只读展示确认时的 suborders（无标记退化为原始 suborders）。

## 7. 不在本次实现 / 已知边界

- **编码→中文映射**：本期原样展示后端编码（region/device_type/image_id/charge_type/require_type/res_assign/disk_type），映射后续补（QA5）。
- **顶部横幅**：后端 B 协议不下发 `banner`，预提单卡不再渲染横幅。
- **A「调整配置」按钮**：稿面暂不支持，按钮注释保留；弹窗已适配 `suborder`，需要时放开即可。
- **添加到配置清单回填**：A/B/D 卡均仅 `routerAction` 跳转主机申请页 + 预留 `_payload` 入口，回填契约留后续需求。
- **约定外事件名**：实测 agent 会返回部分约定外 `name`（含其他 `tool_confirm.interrupt.*`），前端**不识别、保持原生文本气泡**（`use-event.ts` CUSTOM 分支只命中已约定 name，其余 no-op，由伴随的 TEXT_MESSAGE 渲染）。
- **历史已选方案精确还原**：依赖后端 resume 权威值（QA4），确认前为 best-effort。

## 8. 验证建议（手测，详见 test 阶段）

- 进入主机申领 → 选云账号（account_select 链路）→ 收到 A `recommend_select.interrupt`：推荐卡渲染，多条可翻页，来源标签正确。
- A「选择方案」→ `/agui` 请求体带 `forwardedProps.resumeValue`（选中 suborder JSON）→ 收到 B `recommend_suborder_confirm.interrupt`：预提单表格渲染。
- B 行内编辑 → C 弹窗修改保存 → 表格更新；「确认方案」→ 请求体带 `forwardedProps.suborders`（含修改）。
- 全屏最大化/ESC 退出/背景锁滚动；全页与浮窗两端响应式正常。
- 提交后转只读收起态，可展开/收起；刷新或切会话回放历史 → 只读还原（best-effort）。
- lint 通过（已校验，无错误）。

## 9. 与早期建议契约的差异（mock → 真实协议）

| 维度 | 早期建议契约（mock 期） | 后端真实协议 |
|---|---|---|
| A 事件名 | `host_apply.recommend` | `after_tool_hitl.recommend_select.interrupt` |
| B 事件名 | `host_apply.preorder` | `after_tool_hitl.recommend_suborder_confirm.interrupt` |
| A 结构 | `plans: { plan_id, spec }[]` | `recommendations: { source, suborder }[]`（无 id） |
| B 结构 | `configs: { config_id, spec }[]` + `banner` | `suborders: Suborder[]`（无 id、无 banner） |
| 字段值 | 展示串（华南/包年包月…） | 编码（ap-nanjing / PREPAID / 7…） |
| 行/方案定位 | `plan_id` / `config_id` | **下标** |

## 10. mock 移除

- 删除 `store/chatbot/agent-mock.ts`；移除 `agent.ts` 中 `MOCK_ENABLED` 分支与 import。
- 下游 `readSSE` / `handleEvent` / 卡片渲染零改动，真实 `/agui` 直连。

## 11. 备注

- 本 coding.md 为补写（实现已落地 + 后端协议对接完成后一并记录），与 api.md（已校准真实协议）保持一致。
- 待后端确认 QA3（resume 承载）/ QA4（历史权威值）/ QA5（编码映射）后做收尾迭代。
