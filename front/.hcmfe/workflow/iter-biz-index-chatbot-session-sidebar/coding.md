# Coding：业务视角 chatbot 会话侧栏

## 变更摘要

| 文件 | 变更 |
|------|------|
| `store/chatbot/session.ts` | 会话 CRUD 走 `resolveBizApiPath(bkBizId)`；响应增加 `session_tag` |
| `hooks/chatbot/types.ts` | `ChatSession.sessionTag` |
| `hooks/chatbot/use-session.ts` | 注入 `getBkBizId`；列表按 `updated_at` 排序；`reloadSessions` |
| `hooks/chatbot/use-chatbot.ts` | `useWhereAmI().getBizsId`；自动标题 update 带 bizId |
| `views/chatbot/index.vue` | 搜索、置顶/历史+标签文件夹、稿面样式 |
| `views/chatbot/children/session-sidebar-item.vue` | 会话行（置顶图标 / 更多菜单） |

## 行为说明

1. **API**：`POST/PATCH/DELETE` 均使用 `/api/v1/agent/bizs/{bk_biz_id}/sessions/...`；`bk_biz_id` 来自 `getBizsId()`（query `bizs` / store / localStorage）。
2. **分组**：置顶（本地 storage）→ 历史对话（未分组平铺 + `session_tag` 文件夹，文件夹按 max `updated_at` 排序）。
3. **搜索**：侧栏顶部本地过滤 `session_name`，无匹配显示「无匹配会话」。
4. **业务切换**：监听 `route.query.bizs`，调用 `reloadSessions()`。
5. **菜单**：置顶区「取消置顶 / 重命名 / 删除」；历史区「置顶 / 重命名 / 删除」。

## 补充修复（稿面对齐）

- 侧栏背景 `#FFFFFF`（`--sidebar-bg: #fff`）
- 收起态：`2031-1317` **胶囊浮条**（`position: absolute` + `border-radius: 999px` + 阴影），非通栏
- hover 左侧 8px 触发区仍浮层完整侧栏

## 图标映射（design §3.7.1 → 实现）

| 语义 | 实现 |
|------|------|
| 收起-展开 chevron | `bkhcm-icon-right-shape` |
| 收起/展开态-新对话（气泡+加号） | `bkui-vue` `Assistant`（iconfont 无对应） |
| 搜索 | `bkui-vue` `Search` |
| 置顶 | `bkhcm-icon-collect` |
| 更多 | `bkhcm-icon-more-fill` |
| 文件夹 | `bkhcm-icon-file` |
| 收起侧栏 | `bkhcm-icon-shouqi` |

## 未实现（下一迭代）

- 拖拽排序
- 服务端搜索
- SSE 路径业务隔离

## 验证建议

- 有 `bizs` 时列表/新建/重命名/删除正常
- 切换业务后会话列表刷新
- 置顶、搜索、文件夹展开收起
- 路由 `/business/chatbot/:sessionCode?` 与 `bizs` query 保留
