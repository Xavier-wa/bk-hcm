---
name: wf-design-icon-intake
disable-model-invocation: true
description: HCM 项目图标识别与选型 — 回填 design.md §3.x「项目候选（类名 / 组件，可空）」列到 HCM iconfont（bkhcm-icon-*）/ bkui-vue3 兜底，供 coding 落码。由 HCM workflow-nodes 在 design.post 挂载执行。
---

# HCM Design 图标识别与使用

> HCM 专属。承接 `wf-design-figma-intake` 写好的 `design.md` §3.x 关键图标语义（主流程已填语义列）。
> 主流程（intake / figma-dev 早停 §0–5.5）**只负责语义列**；本 skill **只回填「项目候选（类名 / 组件，可空）」列**，不改 design.md 其它章节。
> 补全 HCM iconfont / bkui 候选，避免 coding 阶段臆测。

## 使用时机

design 阶段主工作（`wf-design-figma-intake` 写完 `design.md` 含 §3.x 语义列）**之后**，
由 HCM `workflow-nodes.json` 的 design 节点触发执行。

- 无 §3.x 关键图标语义表或语义列为空 → 先回到 `wf-design-figma-intake` 补齐，再跑本 skill。
- 本需求无图标（纯逻辑 / 无 UI 变更）→ `bkdevbuddy_workflow_node_skip`（note 说明）。

## 图标来源（HCM，严格按优先级）

| 优先级 | 来源 | 用法 |
|--------|------|------|
| 1 | `front/src/assets/iconfont/style.css` | Grep `.bkhcm-icon-*`（按语义关键词）；模板 `<i class="hcm-icon bkhcm-icon-xxx" />` |
| 2 | `bkui-vue/lib/icon` | iconfont **无稿面语义对应**时再引入（`import { Xxx } from 'bkui-vue/lib/icon'`） |

**禁止**未查 iconfont（`style.css`）就把稿面图标全部映射为 bkui-vue。

## 操作步骤

1. 读 `<dataDir>/workflow/<id>/design.md` §3.x 关键图标语义，取每条**语义描述**（不改语义列）。
2. 按语义关键词 Grep `front/src/assets/iconfont/style.css` 的 `.bkhcm-icon-*`：
   - 命中 → 记 iconfont 类名（模板 `<i class="hcm-icon bkhcm-icon-xxx" />`）。
   - 无命中 → 查 `bkui-vue/lib/icon` 找语义相近组件兜底。
3. 把「语义 → iconfont 类名 / bkui 组件」映射**回填到 `design.md` §3.x 表的「项目候选（类名 / 组件，可空）」列**（只填该列，不改语义列与其它章节），供 coding 直接选型；不新建过程文件。
4. 全部映射完成 → `bkdevbuddy_workflow_node_complete`；无图标可处理 → `bkdevbuddy_workflow_node_skip`（note）。

## 落码自检（coding 阶段复用）

| 环节 | 检查项 |
|------|--------|
| coding 开始前 | 读 `design.md` §3.x（语义列 + 本 skill 回填的项目候选列） |
| 落码 | 优先 iconfont `bkhcm-icon-*`；`coding.md` 记录最终「语义 → 类名 / 组件」映射 |
| 联调 | 对照 Figma / 用户 PNG 验收图标语义一致，禁止臆测（如用 plus-circle 代替「气泡+加号」） |

## 注意事项

- 只写 HCM 图标来源与类名，通用识别流程见 public `wf-design-figma-intake`。
- 具体某需求（如某侧栏）的图标映射不沉淀到本 skill，落到该需求的 `design.md` / `coding.md`。
- 脱敏规则同 workflow driver。
