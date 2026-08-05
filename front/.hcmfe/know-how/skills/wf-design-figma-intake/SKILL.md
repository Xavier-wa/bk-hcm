---
name: wf-design-figma-intake
description: Design 阶段写 design.md（不出业务代码）。有 Figma 时包装执行 blueking-figma-dev §0–5.5 认稿/映射后落盘；无设计稿可写精简豁免版。当 workflow 进入 design 阶段时使用。
---

# Workflow Design Figma Intake

包装 `blueking-figma-dev` 的 **认稿 / 映射 / 方案** 能力，服务工作流 **design** 阶段。本 skill 负责早停、写 `design.md`、登记 artifact；**禁止**进入 figma-dev §6 实现与 §7 验证，**禁止**改业务源码。

## 使用时机

工作流进入 **design** 阶段时，**先分支**：

| 场景 | 做法 |
|------|------|
| **有设计稿**（PRD 含 Figma / 用户供图） | 读并执行 `blueking-figma-dev` §0–5.5，再写完整 `design.md`；**禁止**凭 PRD 臆测稿面 |
| **无设计稿**（小需求、bugfix、纯逻辑、沿用现网 UI） | 写**精简豁免版** `design.md`；不跑 figma-dev、不调 Figma MCP |

识别过程**不落库**中间产物；阶段最终只沉淀 **`design.md`**（有稿时另登记 Figma URL）。

## 输入

- `<dataDir>/workflow/<id>/prd.md`（Figma 链接 / node-id）
- 用户提供的 **Figma 链接** 或 **导出 PNG**（对话附件，不写入仓库）
- 已安装 skill：`blueking-figma-dev`（有稿时必需；缺失则提示 `bkdevbuddy sync` / 安装 public skills）

## 产物（强制）

| 文件 | 路径 |
|------|------|
| 产品设计文档 | `<dataDir>/workflow/<id>/design.md` |
| Figma 引用 | `bkdevbuddy_workflow_artifact_add stage=design ref=<figma url>` |

> **不产出** `design-intake.md`、`design-assets/` 等过程文件。  
> **不写入** 业务源码（`.vue` / `.ts` 实现等）。

## 模板

- `./assets/design-template.md` —— 有设计稿（含 §3.x / §3.y / §8）
- `./assets/design-no-figma-template.md` —— 无设计稿豁免

## 有设计稿：包装执行 blueking-figma-dev（早停）

1. 确认 `blueking-figma-dev` 与本 skill 已安装。
2. 读 `<dataDir>/know-how/skills/blueking-figma-dev/SKILL.md`（及步骤所需 `references/*`）。
3. **只执行** figma-dev 工作流 **§0–5.5**：
   - §0 仓库门禁（若有）
   - §1 项目与版本侦察（目标目录已知时可与取数并行）
   - §2 获取 Figma 数据（含 Code Connect 中断与设计上下文降级）
   - §3 annotations 与状态
   - §4 映射组件、Icon 与 Theme
   - §5 组件 skill reference 确认门
   - §5.5 方案确认门 → 结论写入 `design.md` **§8**，供用户在 Design 审批时确认
4. **硬停**：不要执行 §6 按项目原生方式实现、§7 验证视觉与行为。
5. 将结论写入 `<dataDir>/workflow/<id>/design.md`（套 `design-template.md`），至少包含：
   - §1–§7 既有章节（布局/交互/状态/PRD 映射等）
   - **§3.x** 关键图标语义（**只填语义 + Node**；项目候选列留给项目图标 skill）
   - **§3.y** 组件候选表（**只填通识四列**：稿面区域/语义、组件候选、体系、文档确认；**不要**用 figma-dev 泛化侦察填死「复用层级 / 落码入口 / 项目路径」，可留空留给 project post）
   - **§8** 实现边界纪要
   组件 skill 未安装时，§3.y「文档确认」记「未读」并列入待确认，不阻塞 design（本阶段不出码）。
6. `bkdevbuddy_workflow_artifact_add` 登记 Figma URL + `design.md`。
7. 若项目 `workflow-nodes` 在 design.post 挂了图标 skill（如 HCM `wf-design-icon-intake`）：主流程结束后按 nodes 执行回填 §3.x 项目候选列。
8. 若挂了 `wf-design-comp-intake`（skill）/ 节点 `hcm-design-comp-intake`（HCM）：在 icon 回填**之后**执行，回填 §3.y 项目三列（复用层级 / 落码入口 / 项目路径）。
9. 全部 post 节点完成后，请用户确认 Design → `approve` → `next`。

### 图标落盘规则（防与 icons.md 冲突）

- §4 可采用 figma-dev 选型**顺序思想**（相邻资源 → 语义匹配 → 再组件库）。
- 写入 §3.x 的正式内容只有**视觉语义**；bkui export 名若已想到，只可作备注，**不得**填进「项目候选」列取代项目 iconfont 回填。
- **禁止**根据 `blueking-figma-dev/references/icons.md` 清单直接填死项目类名。

### 降级与阻塞

- MCP 限次 / 鉴权失败 / 连续 2 次同类错误 → 按 figma-dev「设计上下文降级」；可请用户供 PNG（对话附件，不入库）。
- 降级后仍不足「可用」上下文 → 阻塞 approve；可暂写不依赖视觉的章节并列出缺失项。
- 「应有稿但未拿到」**不是**无稿豁免。

## 无设计稿豁免（小需求 / bugfix）

**前提**：用户明确本迭代无独立设计稿（无 UI 变更 / 沿用现网 / 纯后端或逻辑修复等）。

1. 写精简 `design.md`（`design-no-figma-template.md`）：标题含「无设计稿」、豁免原因、UI 参照、PRD 验收映射。
2. `artifact_add` 登记 `design.md`（**不登记** Figma URL）。
3. 用户确认 → `approve` → `next`。
4. 无图标可处理时，项目 icon 节点可 `workflow_node_skip`（note 说明）。

**整段跳过 design 文件**：仅当用户明确要求时，由 workflow driver `set_stage` + `force: true` + `note`；AI 不得自行 force。

## Coding 阶段读法（有 `design.md` 时）

```
design.md §3.y 落码入口（有则打开对应 page-* / comp-*）
    + §3.y 通识列 + §8 + §3.x
    → 入口=无且 adjacent → 按项目路径复用
    → none / 空 → Grep 相邻 → 组件库；coding.md 记录映射
```

**禁止**无视 §3.x / §3.y 重新按 `icons.md` 或记忆重映射。  
稿面变更、表缺失或需视觉回归时，coding 可再跑 figma-dev（含 §6–7）；日常以 `design.md` 为选型真相源。

具体图标路径/类名由**项目级 skill** 定义（见项目 `workflow-nodes.json`）。public 不写项目专属路径。

## 防滑检查清单（design 结束前）

- [ ] 未执行 figma-dev §6–7
- [ ] 工作区无因本阶段新增/修改的业务实现文件（仅 `design.md` / artifact 相关）
- [ ] design 阶段未切换或新建 git 分支（除非用户明确要求）
- [ ] 有稿时 §3.x 语义列、§3.y 通识四列、§8 已填（或显式注明 N/A）
- [ ] 有项目 comp-intake 时 §3.y 项目三列已回填或显式 skip
- [ ] 已 `artifact_add`

## 链路自检

| 环节 | 检查项 |
|------|--------|
| Design | 包装执行了 figma-dev §0–5.5；`design.md` 含 §3.x / §3.y / §8 |
| Design.post | 项目图标 skill（如有）回填 §3.x 项目候选列 |
| Design.post | comp-intake（如有，icon 之后）回填 §3.y 项目三列 |
| Coding | 优先 §3.y 落码入口；读表落选；不与 `icons.md` 打架 |
| 联调 | 对照 Figma / PNG 验收语义 |

## 注意事项

- `design.md` 不写完整组件 API / 业务实现代码；§3.y / §8 / 布局形态除外。
- 脱敏规则同 workflow driver。
