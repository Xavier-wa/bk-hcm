# BlueKing 组件选型

本文件负责 BlueKing Vue2/Vue3 组件选型入口；完整候选分别位于：

- Vue3 / bkui-vue：`component-map-vue3.md`
- Vue2 / bk-magic-vue：`component-map-vue2.md`

映射表只回答“可能使用哪个组件”。拿到候选后，必须先使用与目标项目组件库及版本严格匹配的组件 skill，并读取目标组件 reference；组件 API 以对应组件 skill reference 为最终依据，未完成文档确认前禁止写组件代码。

## 一、识别组件体系

| 项目证据/组件库 | 组件映射 | 固定组件 skill 路由 |
| --- | --- | --- |
| 使用 bk-magic-vue，Vue2 | `component-map-vue2.md` | bk-magic-vue → `bk-magicbox-vue-components` |
| 使用 bkui-vue，Vue3 | `component-map-vue3.md` | bkui-vue → `bkui-vue-components` |
| 使用 `@blueking/tdesign-ui` | Vue3 map 可选包章节 | `@blueking/tdesign-ui` → `blueking-tdesign-ui` |
| 使用 `@blueking/search-select-v3` | Vue2/Vue3 map 可选包章节 | `@blueking/search-select-v3` → `blueking-search-select-v3` |
| 使用 `@blueking/date-picker` | Vue2/Vue3 map 可选包章节 | `@blueking/date-picker` → `blueking-date-picker` |
| 同时存在多套组件库 | 以目标目录实际 imports 和业务封装为准 | 只读取最终选中的体系 |

不得从固定目录名选择 Vue2/Vue3。配套组件 skill 与目标项目版本不能严格匹配时，停止并询问用户，不得改用其他版本文档或安装包源码补全 API。

## 二、选型步骤

1. 按行为分类：操作、输入、选择、导航、数据展示、反馈、浮层或布局。
2. 用 Figma 图层名、中文同义词和交互语义查询对应 map。
3. 检查目标项目是否已有业务封装；有则优先复用。
4. 识别候选所属组件库，使用上表固定路由对应的组件 skill。
5. 在组件 skill 索引中定位并读取目标组件 reference。
6. 从文档确认 props、events、slots、methods、类型、默认值和示例。
7. 完成文档确认后才允许出码；只有现有组件无法满足行为时才进入手写审批。

静态 map 未命中时，必须搜索对应组件 skill 的完整索引，并使用组件名、Figma 图层名、中文同义词和交互语义继续检索。完整索引仍未命中时，列出已检索的 skill、组件和具体能力缺口，询问用户是否允许手写；只有用户批准后才能最小手写。

普通 HTML/CSS 布局和业务组合不属于手写基础组件，可按目标项目既有方式实现。

## 三、映射结果格式

每个设计元素应形成：

```text
设计语义：
Figma 图层名/同义词：
组件候选：
目标项目已有封装：
配套组件 skill 检索名：
需要确认的 API：
仍未解决的问题：
```

“需要确认的 API”只列要查的类别，如 props、event、slot，不在本 skill 中猜具体值。

## 四、可选 BlueKing 包

`@blueking/tdesign-ui`、`@blueking/date-picker`、`@blueking/search-select-v3` 只有满足以下条件之一时才成为候选：

- 目标项目已安装并在目标区域使用。
- 目标项目已安装，且相邻业务封装明确基于该包。

没有 `@blueking/tdesign-ui` 证据时，Table 仍先查询当前组件体系的 Table；没有 tooltip 包证据时，不硬编码项目专属提示方案。

三个可选包必须按固定路由：

- `@blueking/tdesign-ui` → `blueking-tdesign-ui`
- `@blueking/search-select-v3` → `blueking-search-select-v3`
- `@blueking/date-picker` → `blueking-date-picker`

## 五、常见误区

| 误区 | 正确处理 |
| --- | --- |
| Figma 叫 `Button`，直接猜 API | 只定位 Button 候选，再查组件 skill |
| 项目安装了包，就全局使用 | 确认目标目录实际用法 |
| 从 Vue3 文档复制到 Vue2 | 按版本读取不同 map 和组件 skill |
| 看到 Table 就强制 TDesign | 只有依赖与现有用法成立才启用 |
| 把纯文字提示、富内容 Popover、点击弹层视为同一组件 | 先按触发方式和内容复杂度区分 |
| 基础组件样式不完全一致就手写 | 先查完整索引、props、slots 和项目封装；仍缺能力则申请手写 |
| 映射到组件名后直接写代码 | 先使用对应组件 skill，读取目标 reference |
| 静态 map 未命中就凭记忆继续 | 搜索对应组件 skill 完整索引和同义语义；仍未命中则列明缺口并询问 |

## 六、手写兜底

手写前必须写清已检索的 skill、组件和现有组件缺失的具体能力，并取得用户批准。实现保持局部和最小，不把单次差异升级为新的通用组件库。
