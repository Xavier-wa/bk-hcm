---
name: blueking-figma-dev
description: 当用户提供 Figma 链接、file key、node-id、截图或设计标注，并要求在项目中还原、实现或修改界面时使用。也适用于侧栏/抽屉、多状态画板、创建引导等 BlueKing Vue 界面落地。
compatibility: 需要可用的 Figma MCP，以及与目标项目组件库版本严格匹配的 BlueKing 基础组件和扩展组件 skill。
---

# BlueKing Vue Figma → 代码

把 Figma 设计翻译成符合目标 BlueKing Vue2/Vue3 项目约定的代码。本 skill 负责编排设计取数、组件映射、组件文档确认、项目原生实现和验证。

Figma 生成代码只能作为结构和视觉参考。组件映射只产生候选；组件 API 必须以与目标项目版本严格匹配的组件 skill reference 为最终依据。

## 核心边界

- 仅处理使用 Vue2 或 Vue3 且采用 BlueKing 组件生态的项目。
- `components.md`、版本 map、`icons.md` 和 `theme.md` 用于候选映射，不定义组件 API。
- 项目相邻代码用于确认业务封装、导入方式和编码风格，不得在 API 冲突时覆盖组件 skill reference。
- 普通语义 HTML/CSS 布局、页面容器和业务组合可按项目约定直接实现。
- 仓库门禁（分支策略、方案确认、脚本授权等）优先于本 skill 的实现步骤；与本 skill 冲突时以仓库规则为准。

## 工作流

### 0. 仓库门禁（若有）

进入 Figma 取数或业务代码侦察前，先检查目标仓库 / `AGENTS.md` / 用户规则是否要求：

- 确认分支策略（切换/新建/基线分支）
- 确认实现方案后再写代码
- 执行脚本、dev server、浏览器自动化前需用户授权

有未完成的门禁时先问用户，完成后再进入步骤 1。不要一上来就扫业务代码或连续拉取 Figma。

### 1. 识别项目与版本

读取 `references/project-discovery.md`，根据 package、锁文件、实际 imports、相邻组件和运行入口确认：

- Vue2 或 Vue3。
- `bk-magic-vue` 或 `bkui-vue` 的版本及使用方式。
- `@blueking/tdesign-ui`、`@blueking/search-select-v3`、`@blueking/date-picker` 的安装和使用情况。
- 业务封装、Icon、Theme、i18n、数据和验证约定。

用户已给出目标目录时：目标目录与 1–2 个同类相邻实现，应与步骤 2 的 Figma 取数**并行**，不要等设计上下文全部完成后再找相邻代码。

同模块/同功能已有稳定样式时，样式落地优先对齐相邻实现；不要仅因仓库笼统写了 Token 规则就强行改用 `var(--xxx)`。Token 选用细则见 `references/theme.md`。

无法确认 Vue 或组件库版本时停止并询问；不得根据仓库名或固定目录猜测。

### 2. 获取 Figma 数据

从 URL 解析 `fileKey` 和 `nodeId`（`node-id=1-2` 转为 `1:2`），按 Figma 集成要求使用：

- `get_design_context`：结构和设计意图的主入口。
- `get_screenshot`：视觉对照基准。
- `get_metadata`：大节点先看层级，再按子节点下钻。
- `get_variable_defs`：读取设计变量语义。
- 资源工具：获取真实图片或 SVG，不使用占位资源。

大画板（含 Changelog / Notes / 多状态对照）时：先用 `get_metadata` + 目标截图读旁注与多态；不必对每个子 frame 都调用 `get_design_context`。

#### Code Connect 中断

若 `get_design_context` 返回 Code Connect 询问（是否连接设计组件到代码库）：

1. **严格按 MCP 返回的脚本原文询问用户**，等待明确答复后再继续。
2. 用户同意：按脚本调用 `get_code_connect_suggestions` 等后续工具。
3. 用户拒绝：再次调用 `get_design_context`，并设 `disableCodeConnect: true`（若工具参数支持）；以拿到设计上下文为准。
4. 重试仍失败或被环境拦截：进入下方「设计上下文降级」，不要整单停死。

#### 设计上下文降级

| 等级 | 条件 | 可做什么 |
| --- | --- | --- |
| 完整 | `get_design_context` + screenshot | 正常出方案并实现 |
| 可用 | screenshot + metadata（含旁注 / 多态 / Notes） | 可出实现方案；缺关键细节再问 |
| 不足 | 仅空节点、无截图、无结构 | 停止并说明阻塞 |

工具认证失败、节点无权限，或降级后仍达不到「可用」时，停止猜测并说明阻塞。

### 3. 收集 annotations 与状态

读取 `references/annotations-and-context.md`。除目标节点外，还要检查父级 frame/section、相关兄弟 variants、annotations、画布说明、红线、示例数据和图层命名，补齐 loading、empty、error、disabled、hover、selected、overflow 等已定义状态。

信息缺失或与截图、图层结构冲突时，暂停相关决策并询问。

### 4. 映射组件、Icon 与 Theme

先读取 `references/components.md`，再按已确认版本读取：

- Vue2 / `bk-magic-vue`：`references/component-map-vue2.md`。
- Vue3 / `bkui-vue`：`references/component-map-vue3.md`。
- Icon：`references/icons.md`。
- Theme/Token：`references/theme.md`。

扩展组件只在目标项目已安装，且目标区域或相邻业务封装正在使用时成为候选。

### 5. 通过组件 skill reference 确认门

对每个组件候选按以下五类固定路由使用与目标项目版本严格匹配的组件 skill：

| 组件库 | 组件 skill |
| --- | --- |
| `bk-magic-vue` / Vue2 | `bk-magicbox-vue-components` |
| `bkui-vue` / Vue3 | `bkui-vue-components` |
| `@blueking/tdesign-ui` | `blueking-tdesign-ui` |
| `@blueking/search-select-v3` | `blueking-search-select-v3` |
| `@blueking/date-picker` | `blueking-date-picker` |

从组件 skill 索引定位并读取目标 reference，确认 props、events、slots、methods、类型、默认值、限制和示例，完成后才允许输出该组件代码。组件 API 以组件 skill reference 为最终依据；相邻代码只决定业务封装、导入方式和编码风格。

使用 `blueking-search-select-v3` 或 `blueking-date-picker` 时，必须根据目标项目的 Vue2/Vue3 版本定位并读取对应入口 reference，不得跨版本复用文档。

静态 map 未命中时，搜索对应组件 skill 的完整索引和同义语义。仍未命中时，列出检索过的 skill、索引、组件候选及能力缺口，并询问用户是否允许手写；只有用户明确批准后，才能做最小手写组件。不得凭记忆、安装源码、Figma 生成代码或其他版本文档补全组件 API。

### 5.5. 方案确认门

非琐碎改动（新页面/新侧栏、多步骤流、多状态交互、目录重构等）在写代码前，先给出简短实现方案并获得用户确认，至少包括：

- 目标目录与文件结构
- 组件候选映射（bkui / 业务封装 / 手写边界）
- 交互与状态范围（含明确不做的事项）
- 数据来源（真实接口 / mock）与是否接入现有页面

仓库已要求「方案确认后再写代码」时，本步为硬门禁。本 skill 只产出 Figma→落地的实现方案，不替代完整产品 brainstorm 文档流。

### 6. 按项目原生方式实现

读取 `references/implementation-strategy.md`，按以下顺序实现：

1. 项目已有业务封装。
2. 组件 skill 确认的 BlueKing 组件。
3. 语义 HTML/CSS 布局和业务组合。
4. 用户明确批准的最小手写组件。

不引入设计和项目未要求的依赖、抽象、状态、交互或响应式规则。

### 7. 验证视觉与行为

读取 `references/visual-verification.md`。逐区域对照截图，按 annotations 和相关 variants 核对状态与交互。

交付出口：

- **有运行环境且用户已授权**：在真实应用中验证关键路径。
- **无运行环境 / 仅交付独立组件未接路由**：对照截图做静态自检，并在回复中列出未验证项；不假装已跑通。
- 执行脚本、测试、lint、构建、dev server 或浏览器自动化前，遵循目标仓库规则和用户授权。

## 必须停下来询问

出现以下情况时暂停相关决策并提出聚焦问题：

- 仓库门禁未完成（分支、方案、脚本授权等）。
- Figma 认证失败、节点无权限，或降级后仍达不到「可用」上下文。
- annotation 与截图或图层结构冲突。
- 必需状态、交互、资源或响应式断点未定义。
- 无法确认 Vue、基础组件库或扩展组件版本。
- 静态 map 和组件 skill 完整索引均未命中，需要手写组件。
- 组件 API、Icon export 或 Token 无法从指定可靠来源确认。
- 非琐碎改动尚未通过方案确认门。

用户未要求方案选项时，不要替未解决的设计冲突选择“安全默认值”。

## 引用索引

- `references/project-discovery.md`：技术栈与项目约定侦察。
- `references/annotations-and-context.md`：目标节点之外的状态与设计意图。
- `references/components.md`：组件体系、固定路由和 API 查阅边界。
- `references/component-map-vue2.md`：Vue2 / `bk-magic-vue` 组件候选。
- `references/component-map-vue3.md`：Vue3 / `bkui-vue` 组件候选。
- `references/icons.md`：Icon 候选和资源处理边界。
- `references/theme.md`：颜色、字体、间距、圆角和阴影候选。
- `references/implementation-strategy.md`：组件、样式、资产和响应式选型。
- `references/visual-verification.md`：视觉、行为、响应式和交付校验。
