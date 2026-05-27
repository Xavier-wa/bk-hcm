# Coding：Chatbot HITL 中断交互（通用选项列表）

## 1. 变更文件清单

| 文件                                                       | 变更类型      | 说明                                                        |
| ---------------------------------------------------------- | ------------- | ----------------------------------------------------------- |
| `src/hooks/chatbot/types.ts`                               | 修改          | 增加 HITL 结构类型定义                                      |
| `src/hooks/chatbot/use-event.ts`                           | 修改          | 实时 `CUSTOM/hitl.interrupt` 事件解析与入列                 |
| `src/hooks/chatbot/use-stream.ts`                          | 修改          | 历史 `MESSAGES_SNAPSHOT` 统一归一化，支持 `activity/CUSTOM` |
| `src/views/index/chatbot/index.vue`                        | 修改          | HITL 消息识别、历史只读态推导、slot 渲染接入                |
| `src/views/index/chatbot/children/hitl-interrupt-card.vue` | 新增/持续调整 | 通用选项卡片，可编辑与只读展示                              |

## 2. 实现细节

### 2.1 `use-stream.ts`：历史消息归一化

新增核心函数：

- `parseHitlInterruptValue(input)`：兼容字符串/对象输入并做结构校验。
- `toHistoryMessage(raw)`：将历史原始消息映射为统一 `Message`。

关键行为：

1. `role === 'activity' && activityType === 'CUSTOM'` 且 `content.name === 'hitl.interrupt'`：
   - 解析 `content.value`
   - 成功则输出 assistant 消息并标记 `__type: 'hitl.interrupt'`
2. 非 `hitl.interrupt` 活动消息降级为文本：`活动消息：{name}`。
3. 在 `MESSAGES_SNAPSHOT` 分支中统一调用 `toHistoryMessage(raw)` 入列。

### 2.2 `index.vue`：历史只读状态推导

新增 `getHitlReadonlyState(message)`：

- 仅处理 `__type === 'hitl.interrupt'` 消息。
- 读取下一条 `user` 消息文本作为候选答案。
- 仅当答案命中 `options` 才回填 `readonlyValue`。
- 未命中时返回只读且空值，避免误把下一条用户消息当“自定义输入”。

模板接入：

- `:readonly="getHitlReadonlyState(message).readonly"`
- `:readonly-value="getHitlReadonlyState(message).value"`

### 2.3 `hitl-interrupt-card.vue`：通用选项交互与只读态

组件能力：

1. 支持预置选项选择与自定义输入互斥。
2. 支持只读模式回显（历史消息）。
3. 文案为通用选项语义（不绑定业务领域）。

近期规则收敛：

- 只读命中选项：仅展示选项高亮。
- 只读未命中/未填写：不展示输入框，统一展示 `未选择或输入自定义`。
- 移除“已选择：xxx”额外文案。

## 3. 渲染链路说明

1. **实时流**：`use-event.ts` 解析 `CUSTOM` → 推入 HITL 消息 → `index.vue` slot 渲染卡片（可交互）。
2. **历史流**：`use-stream.ts` 在 `MESSAGES_SNAPSHOT` 内归一化 activity 消息 → `index.vue` 根据下一条 `user` 计算只读态 → 卡片只读展示。

## 4. 关键设计取舍

- **不把未命中选项的下一条 user 文本当作自定义输入**：避免语义误判。
- **历史态只读优先**：防止历史消息被二次提交，保持可追溯性。
- **统一消息标记 `__type`**：将实时与历史渲染路径收敛到同一组件。

## 5. 编码红线检查

- [x] 无路由/菜单改动
- [x] 无权限模型改动
- [x] 无新增后端 API
- [x] 样式命名使用 kebab-case
- [x] 文案改为通用选项语义
- [x] 历史与实时消息渲染逻辑一致并可回退
