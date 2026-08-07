# BlueKing Vue 专用重构基线

评估对象：重构前的“任意前端核心 + BlueKing 可选 adapter”版本。

## 总结

- 十类场景中，6 项满足、2 项部分满足、2 项不满足。
- Vue2/Vue3 基础组件和三个扩展包已经有组件 skill 路由基础。
- 主要缺口是通用前端定位、可选 adapter、旧目录层级、Vue2 扩展包入口、静态映射兜底和手写授权门。
- API 真相源表述不统一：部分规则以组件 skill 为先，部分规则又让安装源码或相邻用法在冲突时胜出。

## 1. Vue2 基础表单

- 结果：满足。
- 观察：Vue2 映射包含 `bkInput`、`bkSelect`、`bkButton`，并路由到 `bk-magicbox-vue-components`。
- 差距：仍通过可选 BlueKing adapter 进入，而不是 BlueKing Vue 专用固定流程。

## 2. Vue3 Table 与 Popover

- 结果：满足。
- 观察：Vue3 映射包含 Table、Popover、Tooltips/OverflowTitle 候选，并路由到 `bkui-vue-components`。
- 差距：API 最终依据仍存在组件 skill 与安装源码冲突时谁胜出的表述矛盾。

## 3. TDesign 增强表格

- 结果：满足。
- 观察：`@blueking/tdesign-ui` 已固定路由到 `blueking-tdesign-ui`，并要求项目依赖或现有用法提供证据。
- 差距：扩展包入口仍放在 Vue3 map 的可选章节。

## 4. SearchSelect V3 Vue3

- 结果：满足。
- 观察：Vue3 map 能将 `@blueking/search-select-v3` 路由到 `blueking-search-select-v3`。
- 差距：无关键行为缺口。

## 5. SearchSelect V3 Vue2

- 结果：部分满足。
- 观察：主流程有包级组件 skill 路由。
- 差距：Vue2 map 只包含 bk-magic-vue 自带 `bkSearchSelect`，没有 Vue2 使用扩展包的入口、识别条件和优先级；组件入口还错误指向 Vue3 map 可选章节。

## 6. DatePicker Vue2/Vue3

- 结果：部分满足。
- 观察：Vue3 map 能区分 bkui-vue DatePicker 与 `@blueking/date-picker`。
- 差距：Vue2 map 只有 bk-magic-vue 的 `bkDatePicker`，缺少 `@blueking/date-picker` Vue2 入口；组件入口同样只指向 Vue3 map。

## 7. 静态映射未命中

- 结果：不满足。
- 观察：当前规则只要求“得到候选后”从组件 skill 索引定位 reference。
- 差距：没有规定 map 未命中时先搜索组件 skill 完整索引和同义语义。

## 8. 组件 skill 也未命中

- 结果：不满足。
- 观察：当前规则允许记录能力缺口后做最小定制。
- 差距：缺少“列出检索范围和能力缺口 → 询问用户是否允许手写 → 获批后实现”的授权门。

## 9. Token 与 annotation 冲突

- 结果：满足。
- 观察：未接入 Token 时会回退项目样式体系或 Figma 精确值；annotation 与截图冲突时会暂停询问。
- 差距：无关键行为缺口。

## 10. 视觉、状态和响应式验证

- 结果：满足。
- 观察：现有验证规则覆盖统一截图、逐区域核对、状态、行为、响应式 viewport 和交付记录。
- 差距：无关键行为缺口。

## 定位与结构缺口

- `SKILL.md` 仍声明支持 React、Vue、Svelte 和其他 Web 前端。
- BlueKing 仍是可选 adapter，需要额外判断是否启用。
- 核心资料仍位于 `references/blueking/`，引用层级反映旧架构。
- 组件 skill 不可用时仍允许回退安装源码；新设计要求消费方保证组件 skill 与项目版本严格匹配，并以组件 skill 文档作为 API 最终依据。

# BlueKing Vue 专用重构结果

## 总结

- 十类场景全部满足。
- 主流程只处理 BlueKing Vue2/Vue3，不再存在通用前端或 adapter 分支。
- 五类组件库均固定路由到匹配版本组件 skill，组件 reference 是 API 最终依据。
- SearchSelect V3 与 DatePicker 均明确区分 Vue2/Vue3 入口 reference。
- 静态 map 未命中时会搜索组件 skill 完整索引；组件能力仍未命中时先询问是否允许手写。
- Token、annotation、状态、响应式和视觉验证规则保持完整。

## 场景结果

1. Vue2 基础表单：满足；`bkInput`、`bkSelect`、`bkButton` 路由到 `bk-magicbox-vue-components`。
2. Vue3 Table 与 Popover：满足；路由到 `bkui-vue-components` 并按提示语义区分候选。
3. TDesign 增强表格：满足；仅在已安装且目标区域或相邻业务封装实际使用时路由到 `blueking-tdesign-ui`。
4. SearchSelect V3 Vue3：满足；读取 `blueking-search-select-v3` 的 Vue3 入口 reference。
5. SearchSelect V3 Vue2：满足；读取 `blueking-search-select-v3` 的 Vue2 入口 reference，不误用 `bkSearchSelect`。
6. DatePicker Vue2/Vue3：满足；按 Vue 版本读取 `blueking-date-picker` 对应入口 reference。
7. 静态 map 未命中：满足；继续搜索组件 skill 完整索引和同义语义。
8. 组件能力未命中：满足；列明检索范围和能力缺口，用户批准后才允许最小手写。
9. Token 与 annotation 冲突：满足；不输出未接入变量，冲突时暂停询问。
10. 视觉、状态和响应式验证：满足；覆盖统一截图基准、variants、viewport、交互状态和交付记录。

## 验证边界

- 已完成文档规则静态评估和 IDE 诊断。
- 未执行脚本、命令行 lint、构建或真实 Figma/应用运行验证。
