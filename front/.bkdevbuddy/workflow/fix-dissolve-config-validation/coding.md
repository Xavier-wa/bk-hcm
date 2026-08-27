# Coding — fix-dissolve-config-validation

## 执行顺序

1. 修复 `time-period-block.vue`：行级列组件引用改为 `useTemplateRef` v-for 集合引用，卸载自动清理，`getValue` 只校验当前行
2. 修复 `index.vue`：`timePeriodBlockRefs` 改为 `useTemplateRef` 集合引用，`remove*` 不再手动维护引用数组
3. 运行 `bkdevbuddy lint --fix` 校验

> 排序依据：先子组件后父组件，同文件聚合。

## 根因

裁撤配置弹窗（`config-dialog`）中：

- `time-period-block.vue` 用 `entryRefsList` 数组 + `:ref` 回调手动维护每行 `SelectColumn` 引用；回调只在挂载时赋值（`el &&`），行卸载时不清除。点击「重置」时父组件整体替换 `dissolve_projects` 数组，被移除行的引用残留；`getValue()` 仍对已卸载列调用校验，空值校验失败抛错，父级 catch 后提示「请检查必填项」。
- `index.vue` 的 `timePeriodBlockRefs` 同样用下标数组手动维护，重置后残留已卸载的 `TimePeriodBlock`，双重触发误报。

## 修复方案

- 改用 Vue `useTemplateRef` 的 v-for 集合引用：Vue 自动在挂载时加入、卸载时移除，校验只遍历当前真实存在的行。
- `removeEntry` / `removeTimePeriod` 只 splice 数据数组，不再手动维护引用数组。
- `getValue()` 增加行数兜底（slice 到当前 `projects` 长度），避免极端时序下引用越界。
- `datePickerRef` 位于 v-for 行内，Vue 会把该 ref 收集为数组，取值时取首个实例（`datePickerRef.value?.[0]`），否则 `getValue` 会因拿到数组而同步抛错。

## 单据 1: 机房裁撤-裁撤配置弹窗必填校验残留修复

**TAPD**: [#1069995598161976460](https://<TAPD_HOST>/tapd_fe/69995598/bug/detail/1069995598161976460)
**文件**: `front/src/views/dissolve/components/config-dialog/time-period-block.vue` `front/src/views/dissolve/components/config-dialog/index.vue`
**改动点**:
- `time-period-block.vue`：`entryRefsList`/`datePickerRef` 改为 `useTemplateRef`；`getValue` 只校验当前行；`removeEntry` 不再手动 splice 引用
- `time-period-block.vue`：v-for 行内 ref 会被收集为数组，`datePickerRef` 取值 `[0]` 后调用 `getValue`
- `index.vue`：`timePeriodBlockRefs` 改为 `useTemplateRef`；`removeTimePeriod` 不再手动 splice 引用；`handleValidate` 增加空数组兜底

## 验证

- 打开裁撤配置弹窗 → 新增一行不填必填 → 重置 → 保存：不再提示「请检查必填项」
- 新增一行不填必填 → 直接保存：仍正常提示并定位必填项
- 删除中间行/新增多行后重置：校验引用与行数保持一致，无残留误报
- E2E 回归（`e2e/specs/dissolve.spec.ts`）：新增空行 → 取消/重置/重开 → 保存不误报；新增空行直接保存展示行内错误；原保存链路正常
