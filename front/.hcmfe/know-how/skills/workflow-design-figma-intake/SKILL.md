---
name: workflow-design-figma-intake
description: Design 阶段写 design.md。有 Figma 时先 MCP/PNG 识别稿面；无设计稿的小需求或 bugfix 可写精简豁免版 design.md（无需 Figma）。当 workflow 进入 design 阶段时使用。
---

# Workflow Design Figma Intake

## 使用时机

工作流进入 **design** 阶段时，**先分支**：

| 场景 | 做法 |
|------|------|
| **有设计稿**（PRD 含 Figma / 用户供图） | 必须先识别稿面，再写完整 `design.md`；**禁止**凭 PRD 臆测稿面 |
| **无设计稿**（小需求、bugfix、纯逻辑、沿用现网 UI） | 写**精简豁免版** `design.md`（见下节）；不调 Figma MCP |

识别过程**不落库**中间产物；阶段最终只沉淀 **`design.md`**（有稿时另登记 Figma URL）。

## 输入

- `.hcmfe/workflow/<id>/prd.md`（Figma 链接 / node-id）
- 用户提供的 **Figma 链接** 或 **导出 PNG**（对话附件，不写入仓库）

## 产物（强制）

| 文件 | 路径 |
|------|------|
| 产品设计文档 | `.hcmfe/workflow/<id>/design.md` |
| Figma 引用 | `hcmfe_workflow_artifact_add stage=design ref=<figma url>` |

> **不产出** `design-intake.md`、`design-assets/` 等过程文件。

## 模板

- `./assets/design-template.md` —— 有设计稿时的 `design.md` 结构
- `./assets/design-no-figma-template.md` —— **无设计稿豁免**时的精简结构

## 设计稿识别策略

### 1. 解析 Figma URL

提取 `fileKey`、`nodeId`（`2006-6056` → `2006:6056`）。禁止空猜 nodeId。

### 2. 优先 Figma MCP

| 顺序 | 工具 | 说明 |
|------|------|------|
| 1 | `get_design_context` | 首选 |
| 2 | `get_screenshot` | context 失败时 |
| 3 | `get_metadata` | 仅需结构时 |

**限次 / 鉴权失败 / 连续 2 次同类错误** → 立即降级，勿反复重试。

### 3. 降级：用户供图

- 请用户提供 **Figma 链接**（带 node-id）或 **导出 PNG**（对话附件）
- 用 Read 读图；**不要将 PNG 保存到 workflow 目录**
- 识别结果直接写入 `design.md` 对应章节

### 4. 应有稿但未拿到（阻塞）

- **不是**「无设计稿豁免」：PRD/用户已表明需要新 UI，但 MCP 失败且用户未供图
- `design.md` 可暂写 PRD 中不依赖视觉的部分，并列出缺失 node / 截图
- **阻塞 approve design**，直到识别完成或用户改口走「无设计稿豁免」

## 无设计稿豁免（小需求 / bugfix）

**前提**：用户明确本迭代无独立设计稿（无 UI 变更 / 沿用现网 / 纯后端或逻辑修复等）。

1. 写 `.hcmfe/workflow/<id>/design.md`，结构可精简，**必须**包含：
   - 标题含「无设计稿」
   - **豁免原因**（用户原话或摘要）
   - **UI 参照**：现网页面路径、组件，或 PRD 中与界面相关的文字约束（若有）
   - 与 PRD 验收的映射（仅 UI 相关项，可写「沿用现网，见 PRD §x」）
2. `hcmfe_workflow_artifact_add stage=design ref=.hcmfe/workflow/<id>/design.md`（**不登记** Figma URL）。
3. 用户确认豁免说明 → `approve` → `next`。

**整段跳过 design 文件**（连 `design.md` 都不要）：仅当用户明确要求时，由 workflow driver 用 `hcmfe_workflow_set_stage` + `force: true` + `note` 从 prd 跳到 api；AI 不得自行 force。

## 操作步骤（有设计稿）

1. 读 PRD，收集 Figma node 列表。
2. MCP 或用户 PNG 识别稿面（过程仅在对话中完成）。
3. 写 `.hcmfe/workflow/<id>/design.md`（套 `design-template.md`）：
   - 含 Figma 引用表、布局、交互、状态、与 PRD 差异
   - 稿面与 PRD 冲突以 **稿面 + 用户确认** 为准，在 design.md 注明
   - **必须**填写 §关键图标语义（见下），供 coding 阶段选型
4. `hcmfe_workflow_artifact_add` 登记 Figma URL + `design.md`。
5. 用户「确认 Design」→ `approve` → `next`。

## 关键图标语义（Design 必填，Coding 必读）

有稿时，在 `design.md` 增加 **「关键图标语义」** 小节（只写**视觉语义**，不写 CSS 类名 / 组件名）：

| 稿面位置 | 语义描述（示例） | Node |
|----------|------------------|------|
| 收起窄条-上 | 右指向 chevron，展开侧栏 | `2031-1317` |
| 收起窄条-下 | 对话气泡内含加号，新建会话 | `2031-1317` |
| … | … | … |

**缺失该表** → coding 阶段易误用错误图标，视为 design 产物不完整（可补写后再 approve）。

## Coding 阶段图标选型（有 `design.md` 时）

实现稿面图标时，**严格按顺序**：

```
design.md §关键图标语义
    → Grep front/src/assets/iconfont/style.css（按语义关键词）
    → 无匹配再查 bkui-vue/lib/icon
    → 在 coding.md 记录「语义 → 最终 iconfont 类名 / bkui 组件」
```

| 优先级 | 来源 | 用法 |
|--------|------|------|
| 1 | `front/src/assets/iconfont/style.css` | Grep `.bkhcm-icon-*`；模板 `<i class="hcm-icon bkhcm-icon-xxx" />` |
| 2 | `bkui-vue/lib/icon` | iconfont **无稿面语义对应**时再引入 |

**禁止**未查 iconfont 就把稿面图标全部映射为 bkui-vue。

### chatbot 侧栏参考映射（随稿面更新）

| 语义 | iconfont（优先） | bkui-vue（兜底） |
|------|------------------|------------------|
| 搜索 | （无专用） | `Search` |
| 新对话（渐变按钮 / 展开态） | （无气泡+加号） | `Assistant` |
| 收起窄条-新对话 | （无气泡+加号） | `Assistant` |
| 收起窄条-展开 | `bkhcm-icon-right-shape` | `AngleRight` |
| 收起侧栏（展开态底部） | `bkhcm-icon-shouqi` | `CollapseLeft` |
| 置顶 | `bkhcm-icon-collect` | — |
| 更多 | `bkhcm-icon-more-fill` | `Ellipsis` |
| 文件夹 | `bkhcm-icon-file`（收起降透明度） | `Folder` / `FolderOpen` |

### 收起窄条布局（稿面 `2031-1317`，Coding 必对）

- **形态**：左侧**竖向胶囊浮条**（非通栏、非占满高度侧栏）
- **样式**：白底 `#FFFFFF`、大圆角（stadium / pill）、轻阴影向右浮出
- **内容**：上 chevron 展开；下气泡+加号新对话；两图标垂直居中留白
- **交互**：点击上→展开完整侧栏；点击下→新建会话；左侧 8px 触发区 hover 仍浮层完整侧栏

## 链路自检（Design → Coding）

| 环节 | 检查项 |
|------|--------|
| Design | `design.md` 含 §关键图标语义 + 收起窄条形态描述（若本需求有侧栏） |
| Coding 开始前 | 读 `design.md` 图标表 + 本 SKILL §Coding 图标选型 |
| Coding 落码 | Grep `style.css`；`coding.md` 记录映射；禁止臆测 plus-circle 代替气泡+加号 |
| 联调 | 对照 Figma `2031-1317` / 用户 PNG 验收收起胶囊，而非通栏窄条 |

## 注意事项

- `design.md` 正文不写技术实现；**图标语义表**与**布局形态**除外（仍不写类名）。
- 识别用 PNG 仅作会话输入，**不入库**。
- 脱敏规则同 workflow driver（域名占位符等）。
