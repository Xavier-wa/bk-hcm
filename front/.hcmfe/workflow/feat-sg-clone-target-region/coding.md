# Coding：安全组-克隆支持选择目标地域

## 架构方案

零新增接口、零后端改动。在现有克隆安全组弹窗（`src/views/resource/resource-manage/children/dialog/clone-security/index.vue`）内：

1. 复用 `useRegionStore().getRegionList({ vendor })` 获取该 vendor（克隆入口已限制 TCLOUD）可用地域全集（store 内部自动追加 `vendor=tcloud` + `status=AVAILABLE` 过滤，返回归一化 `{ id, name }[]`）。
2. 表单模型新增 `targetRegion`，默认值 = 源安全组 `region`（props.data.region），保证默认场景与改造前行为等价（AC-003）。
3. 提交链路 `handleConfirm → business.cloneSecurity` 透传 `target_region`。
4. 成功提示文案改为 `t('已克隆至 {region}！', { region })`（项目内命名插值惯例，参照 uninstall-drive.tsx）。**该 key 必须同步收录进 `src/language/lang.ts`**——vue-i18n 在翻译缺失时 fallback 返回 key 字符串本身且**不执行命名占位符替换**，未收录会导致提示显示为字面量 `已克隆至 {region}！`。

## 变更清单

| 文件 | 变更 |
| --- | --- |
| `src/store/business.ts` | `ICloneSecurityParams` 增加 `target_region: string`；`cloneSecurity` 解构并透传 `target_region` |
| `src/views/resource/resource-manage/children/dialog/clone-security/index.vue` | ① import `useRegionStore`；② `formModel` 增加 `targetRegion: props?.data?.region`；③ `rules` 增加 `targetRegion` 非空校验；④ 新增 `regionList/regionLoading/getRegionList`（try-catch-finally，失败降级空列表）；⑤ `watch(isShow)` 打开时并行调用 `getRegionList()`；⑥ `handleConfirm` 提交 `target_region`，成功提示携带目标地域显示名（列表中找不到则回退显示地域 ID）；⑦ template 在「安全组名称」后新增「目标地域」`bk-select`（filterable + loading） |
| `src/language/lang.ts` | 项目 i18n 中心仓库（跨需求共用）：新增 `'已克隆至 {region}！': ['Cloned to {region}!']`，使上述命名插值真正生效。修复前该 key 缺失，提示显示为字面量 `已克隆至 {region}！` |

## 关键实现决策

* **vendor 来源**：使用 `props.data.vendor`（克隆入口 `src/views/resource/resource-manage/children/plugin/security-group/show-clone.plugin.ts` 已限制 `vendor === VendorEnum.TCLOUD`，语义上面向未来放开其他 vendor 时无需改弹窗）。
* **每次打开弹窗重新拉地域**：调用方以 `v-if` 控制组件实例（`src/views/resource/resource-manage/children/manage/security-manage.vue`），实例销毁重建，`formModel.targetRegion` 初始化每次正确；`watch(isShow, immediate)` 打开即拉取，地域数据保持新鲜。
* **非空校验兜底**：源安全组数据理论上有 `region`（列表列已展示），但增加 `targetRegion` validator 防御脏数据导致提交空值。
* **提示名回退**：`regionList.find(...)?.name || formModel.targetRegion`——地域列表加载失败时仍能提示地域 ID。

## 影响面分析

| 共享代码 | 使用方 | 核实结论 |
| --- | --- | --- |
| `src/store/business.ts` 的 `cloneSecurity` 与 `ICloneSecurityParams` | 全项目唯一调用方 `src/views/resource/resource-manage/children/dialog/clone-security/index.vue`（search_content 核实，含 `ICloneSecurityParams` 全部引用） | 已同步传 `target_region`，无其他使用方受影响 |
| `src/store/region.ts` 的 `getRegionList` | 只读复用，未修改该 store | 零影响 |
| 组件导出 `IData` 与 `ICloneSecurityProps` | `src/views/resource/resource-manage/children/manage/security-manage.vue` | 未修改接口，零影响 |
| `src/language/lang.ts` 的 zh/en 词条映射 | 全项目 `t()` 调用共用（跨需求共享文件） | 仅**新增** `'已克隆至 {region}！'` 一条，不改动既有 key；其他需求文案不受影响 |

## 验证

* `read_lints`：0 错误 0 警告。
* `npx prettier --check`：初次有格式偏差，已 `--write` 修复后通过。
* 手测清单见 `test.md`（验证结论统一标「待 QA 验证」）。

## 风险点

* 后端对非 TCLOUD vendor 传入 `target_region` 的行为未验证——但克隆入口已限制 TCLOUD，不构成现实路径。
* 地域列表接口失败时下拉为空但仍可提交默认值（源地域 ID），不会阻断同地域克隆主流程。
