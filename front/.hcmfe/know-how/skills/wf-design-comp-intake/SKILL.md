---
name: wf-design-comp-intake
disable-model-invocation: true
description: HCM 业务组件/页面模式选型 — 回填 design.md §3.y「复用层级 / 落码入口 / 项目路径」，指向 page-* 或 comp-*，供 coding 落码。由 HCM workflow-nodes 在 design.post 挂载（排在 icon-intake 之后）。
---

# HCM Design 业务组件选型

> HCM 专属。承接 `wf-design-figma-intake` 写好的 `design.md` §3.y 通识列，
> 补全项目复用决策。**只回填 §3.y 项目三列**，不改其它章节，**不出业务代码**。

## 使用时机

design 主流程写完 `design.md`（含 §3.y 通识列）**之后**，
由 HCM `workflow-nodes.json` 的 design.post 触发（建议在 `wf-design-icon-intake` 之后）。

- 无 UI / 纯逻辑 → `bkdevbuddy_workflow_node_skip`（note 说明）。
- §3.y 通识表缺失 → 先回到 `wf-design-figma-intake` 补齐，再跑本 skill。

## 决策顺序（严格）

| 优先级 | 条件 | 复用层级 | 落码入口 | 项目路径/说明 |
|--------|------|----------|----------|---------------|
| 1 | 整页新列表/表单/详情 | `page` | `page-list` / `page-form` / `page-detail` | 拟建模块目录或「新建于 …」 |
| 2 | 局部嵌入（抽屉/侧栏/页内嵌表单\|表格\|详情） | `comp` | `comp-form` / `comp-data-list` / `comp-detail` | 先读 `comp-field-model`；填写挂载点（如 sideslider）+ 宿主路径 |
| 3 | 仅命中相邻业务封装、非标准模式 | `adjacent` | `无` | Grep 到的封装路径 |
| 4 | 都没有 | `none` | `无` | 说明将手写+bkui；列入 §3.y 待确认 |

**禁止**未检索项目内已有 list/form/detail 模式就把整页稿面一律标 `none`。  
**禁止**本阶段执行 `page-*` / `comp-*` 出码步骤。

## 操作步骤

1. 读 `<dataDir>/workflow/<id>/design.md` §3.y、§2、§8 与 PRD 范围。
2. 对每一行通识语义，按上表决策，回填三列（可多行共享同一落码入口）。
3. 不新建过程文件；不改 §3.x / 其它章节。
4. 全部映射完成 → `bkdevbuddy_workflow_node_complete`；无 UI → `bkdevbuddy_workflow_node_skip`（note）。

## Coding 落码自检（供 coding 复用）

| 环节 | 检查项 |
|------|--------|
| coding 开始前 | 读 §3.y 落码入口与项目路径 |
| 落码 | 有 skill 名则打开该 skill；`adjacent` 跟路径；`none` 记偏离原因到 coding.md |
| 联调 | 对照稿面验收是否误用整页骨架做局部嵌入（或反之） |

## 注意事项

- 通用认稿流程见 public `wf-design-figma-intake`；图标见 `wf-design-icon-intake`。
- 具体某需求的映射落到该需求的 `design.md`，不沉淀进本 skill。
- 脱敏规则同 workflow driver。
