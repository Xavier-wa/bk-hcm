# Requirement：aiagent - 资源管理首页 chatbot

## 结构说明

| 层级 | ID / 路径 | 说明 |
|------|-----------|------|
| **父需求** | `req-aiagent-biz-index-chatbot` | 对应 TAPD 父 story；聚合跨子需求的 PRD/Design/API 主版本（`merge` 后写入本目录） |
| **子需求迭代** | `.hcmfe/workflow/<workflow-id>/` | 每个 TAPD 子 story 一条 Workflow，独立 state 与阶段产物 |

## 当前子需求迭代

| Workflow ID | TAPD 子 story | 阶段产物 |
|-------------|---------------|----------|
| `iter-biz-index-chatbot-route-layout` | 路由与布局 (`1069995598134649293`) | `.hcmfe/workflow/iter-biz-index-chatbot-route-layout/` |
| `iter-biz-index-chatbot-session-sidebar` | 侧边栏会话列表 (`1069995598134649467`) | `.hcmfe/workflow/iter-biz-index-chatbot-session-sidebar/` |

## 后续子需求

完成当前 Workflow 至 `done` 后：

1. `hcmfe_req_merge_iteration` — 将本子需求产物合并到本 Requirement 主版本
2. `hcmfe_workflow_init` — 新建下一子需求 Workflow（新 `iter-*` ID），`requirement: req-aiagent-biz-index-chatbot`

Git 分支可与多个子需求共用（如 `feat-biz-index-chatbot`）；Workflow ID 须体现**子需求语义**，勿与分支名混用。
