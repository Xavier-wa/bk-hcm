# Coding：证书管理-支持证书分配给业务-前端

- 工作流：feat-cert-assign-biz（lite）
- TAPD story：1069995598137438280（父：1069995598136842526）
- 日期：2026-08-31

## 0. 方案变更记录（2026-08-31 用户反馈）

原方案（复用 `BatchDistribution` 批量弹窗 + title/hideTrigger 扩展）经用户确认不符合预期，**已废弃并还原**。最终方案改为复用**负载均衡"单个分配"弹窗交互**（`load-balancer-manage.vue` 内联 bk-dialog 模式）：

- `BatchDistribution` 组件的 title/hideTrigger/expose 扩展**全部还原**（仅保留一处与需求无关的存量修复：删除 Dialog 上无效的 `theme={'primary'}` prop，修复 TS2322）。
- cert-manager 页面新增内联 `bk-dialog`：标题「证书分配」、内容「当前操作证书为：xxx」+「请选择所需分配的目标业务」+ `hcm-form-business`（数据源 `useAccountBusiness(当前行 account_id)`），与负载均衡单个分配弹窗完全同构。
- 确认提交走同一接口 `resourceStore.assignBusiness('certs', { cert_ids: [id], bk_biz_id })`，成功提示「分配成功」（与负载均衡一致）后刷新列表。
- 操作列「分配」按钮（未分配可点、已分配禁用+tooltip）保持不变。

## 1. 架构方案（最终）

证书管理页 `front/src/views/business/cert-manager/index.tsx`（参照 `load-balancer-manage.vue` 单个分配实现）：

- 状态：`isDialogShow` / `currentOperateItem` / `isDialogBtnLoading` / `selectedBizId`，`useAccountBusiness(computed(() => currentOperateItem.value?.account_id))` 提供目标业务列表。
- 操作列（资源视图）"删除"左侧"分配"文字按钮：未分配（`bk_biz_id === -1`）可点，已分配禁用 + tooltip「该证书已分配业务, 仅可在业务下操作」（与删除按钮同规则）。
- 内联 `bk-dialog`（标题「证书分配」）+ `hcm-form-business` 选择目标业务，确认后调 `assignBusiness` 并刷新列表。

## 1.1 原方案存档（已废弃）

<details><summary>点击展开原方案（复用 BatchDistribution 扩展）</summary>

复用共享组件 `BatchDistribution`（`front/src/views/resource/resource-manage/children/dialog/batch-distribution/index.tsx`），以**向后兼容的可选扩展**支持"单条分配"场景：

- 新增可选 prop `title?: string`（默认 `''`）：传入时覆盖弹窗标题，未传保持原「批量分配/xx分配」。
- 新增可选 prop `hideTrigger?: boolean`（默认 `false`）：隐藏组件内置"批量分配"按钮，供外部（操作列）触发。
- `setup` 第二参数解构 `expose`，向外暴露 `show()` 打开弹窗。

证书管理页新增第二个 `BatchDistribution` 实例（单条分配专用）。

</details>

## 2. 影响面分析

`BatchDistribution` 全部使用方（新增 props 均可选带默认值，零波及）：

| 使用方 | 核实结论 |
| --- | --- |
| resource-manage/children/manage/vpc-manage.vue | 不传新 props，行为不变 |
| resource-manage/children/manage/subnet-manage.vue | 同上 |
| resource-manage/children/manage/security-manage.vue | 同上 |
| resource-manage/children/manage/load-balancer-manage.vue | 同上 |
| resource-manage/children/manage/ip-manage.vue | 同上 |
| resource-manage/children/manage/drive-manage.vue | 同上 |
| business/cert-manager/index.tsx | 原批量入口不变，新增单条实例 |

顺手修复该共享文件一处**存量** TS2322：`Dialog` 上无效的 `theme={'primary'}` prop（bkui-vue Dialog 无此属性，确认按钮主题实为 `confirmButtonTheme`；运行时该值仅作为 attrs 透传，删除无行为影响，且 Dialog 仅此文件使用）。

## 3. 关键实现细节

- 单条分配提交走组件既有 `handleConfirm`：`resourceStore.assignBusiness('certs', { cert_ids: [row.id], bk_biz_id })`——与需求"接口调用都和原来一样"一致。
- 成功/失败提示复用组件既有文案（批量分配成功/失败）——需求明确"仅修改标题"，不扩大改动（澄清 Q4）。
- 权限：与现有"批量分配"按钮一致，不叠加操作鉴权（澄清 Q5）。
- 操作列宽度资源视图 120→140 容纳双按钮；按钮间距用行内样式（项目禁用工具类规范）。

## 4. 校验记录

- eslint：两个改动文件 0 错误
- 编辑器 TS 诊断（read_lints）：0 错误
- prettier：改动按 eslint prettier 规则归一（CLI 的 CRLF warn 为 Windows autocrlf 环境噪音，基线文件同样命中；入库为 LF）
- stylelint：未改动任何样式文件（存量 scss 错误不属于本次范围）

## 5. 风险点

- expose 仅新增能力，不影响既有实例。
- `singleAssignRow` 为普通 ref，弹窗关闭（quickClose/取消）后数据残留，但下次点击"分配"会整体覆盖，无脏数据路径；分配成功后显式清空。
