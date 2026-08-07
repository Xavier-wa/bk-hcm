# 只同步调账策略的场景 - 编码实现

## 1. 改动范围

本次改动集中在账单汇总页的「账单同步」弹窗，涉及的文件：

| 文件 | 变更类型 | 说明 |
|---|---|---|
| `front/src/views/bill/bill/summary/primary/bcc-sync-button/index.tsx` | 修改 | 弹窗主组件，新增同步范围 Radio、调账数据展示、sync_mode 提交 |
| `front/src/views/bill/bill/summary/primary/bcc-sync-button/index.module.scss` | 修改 | 调整金额颜色与间距 |
| `front/src/api/bill/index.ts` | 修改 | `syncRecordsBills` 新增 `sync_mode` 入参；新增 `reqBillsAdjustmentSum` 函数 |
| `front/src/typings/bill.ts` | 修改 | 新增 `AdjustmentItemSumResult` / `AdjustmentItemSumReqParams` 类型 |

## 2. 状态设计

在 `bcc-sync-button` 组件 setup 中新增以下响应式状态：

- `syncMode`: `'full' | 'adjustment_only'`，默认 `'full'`。
- `adjustmentInfo`: `AdjustmentItemSumResult | null`，仅调账模式下的汇总数据。

保留并调整现有状态：

- `vendor`：默认仍使用 `VendorEnum.AZURE`（与现网一致）。
- `syncInfo`：全量模式下复用 `reqBillsRootAccountSummarySum` 返回值。
- `isChecked`：确认复选框。
- `isConfirmAllBills`：全量模式下所有一级账号已确认。
- `canSyncBills`：computed，分模式计算可同步条件。

## 3. 可同步条件

```ts
const canSyncBills = computed(() => {
  if (!isChecked.value) return false;
  if (syncMode.value === 'full') return isConfirmAllBills.value;
  return hasAdjustment.value; // 仅调账模式下要求调账净额 > 0
});
```

## 4. 预览数据获取

### 4.1 全量同步

复用现有 `getVendorSyncInfo`，调用 `reqBillsRootAccountSummarySum`。

### 4.2 仅同步调账数据

新增 `getVendorAdjustmentInfo`：

```ts
const getVendorAdjustmentInfo = async (vendor: VendorEnum) => {
  const res = await reqBillsAdjustmentSum({
    filter: {
      op: QueryRuleOPEnum.AND,
      rules: [
        { field: 'vendor', op: QueryRuleOPEnum.EQ, value: vendor },
        { field: 'bill_year', op: QueryRuleOPEnum.EQ, value: props.billYear },
        { field: 'bill_month', op: QueryRuleOPEnum.EQ, value: props.billMonth },
      ],
    },
  });
  adjustmentInfo.value = res.data;
};
```

## 5. 调账金额计算

调账净额 = 调增金额 - 调减金额（按币种分别计算）。

```ts
const adjustmentCNYCost = computed(() => {
  const increase = Number(adjustmentInfo.value?.cost_map?.increase?.CNY?.Cost || 0);
  const decrease = Number(adjustmentInfo.value?.cost_map?.decrease?.CNY?.Cost || 0);
  return increase - decrease;
});

const adjustmentUSDCost = computed(() => {
  const increase = Number(adjustmentInfo.value?.cost_map?.increase?.USD?.Cost || 0);
  const decrease = Number(adjustmentInfo.value?.cost_map?.decrease?.USD?.Cost || 0);
  return increase - decrease;
});

const adjustmentTotalRMBCost = computed(() => {
  const increase = Object.values(adjustmentInfo.value?.cost_map?.increase || {}).reduce(
    (sum, item) => sum + Number(item?.RMBCost || 0),
    0,
  );
  const decrease = Object.values(adjustmentInfo.value?.cost_map?.decrease || {}).reduce(
    (sum, item) => sum + Number(item?.RMBCost || 0),
    0,
  );
  return increase - decrease;
});

const hasAdjustment = computed(
  () => adjustmentTotalRMBCost.value > 0 || adjustmentCNYCost.value > 0 || adjustmentUSDCost.value > 0,
);
```

> 后端返回的字符串金额通过 `Number()` 转为数值后相减；若后端 decimal 字符串精度要求更高，可用 `decimal.js` 替换，但本项目现有 `formatBillCost` 均按 Number 处理。

## 6. 弹窗模板调整

### 6.1 同步范围 Radio

在云厂商选择下方新增：

```tsx
<section class={cssModule['sync-scope-wrapper']}>
  <div class={cssModule.title}>{t('同步范围')}</div>
  <RadioGroup v-model={syncMode.value}>
    <Radio label='full'>{t('全量同步')}</Radio>
    <Radio label='adjustment_only'>{t('仅同步调账数据')}</Radio>
  </RadioGroup>
</section>
```

### 6.2 同步内容（仅 full 展示）

沿用现有结构，标签使用 `.label` 固定宽度右对齐，金额使用 `.money` 样式，避免标题换行。

### 6.3 调账数据（仅 adjustment_only 展示）

新增 section，标题与全量模式统一为「同步内容」，按 Figma 稿展示 4 行：

```tsx
<section class={cssModule['sync-content-wrapper']}>
  <div class={cssModule.title}>{t('同步内容')}</div>
  <div class={cssModule.item}>
    <span class={cssModule.label}>{t('调账金额（人民币+美金）')}</span>
    <span class={cssModule.money}>￥{formatBillCost(String(adjustmentTotalRMBCost.value))}</span>
  </div>
  <div class={cssModule.item}>
    <span class={cssModule.label}>{t('调账金额（人民币）')}</span>
    <span class={cssModule.money}>￥{formatBillCost(String(adjustmentCNYCost.value))}</span>
  </div>
  <div class={cssModule.item}>
    <span class={cssModule.label}>{t('调账金额（美金）')}</span>
    <span class={cssModule.money}>＄{formatBillCost(String(adjustmentUSDCost.value))}</span>
  </div>
  <div class={cssModule.item}>
    <span class={cssModule.label}>{t('调账记录数量')}</span>
    <span class={cssModule.count}>{adjustmentInfo.value?.count || 0}</span>
  </div>
</section>
```

### 6.4 确认复选框

文案按现有复用，仅调账模式下取消「所有步骤」相关提示，统一使用：

> 已确认同步以上内容，同步过程中不要进行其他操作。

## 7. 提交同步

`handleConfirm` 中提交参数增加 `sync_mode`：

```ts
await syncRecordsBills({
  bill_year: props.billYear,
  bill_month: props.billMonth,
  vendor: vendor.value,
  sync_mode: syncMode.value,
});
```

## 8. 关闭/重置

弹窗关闭时重置：

- `syncMode` → `'full'`
- `isChecked` → false
- `adjustmentInfo` → null
- `syncInfo` → null

## 9. 新增 API 与类型

### 9.1 API 函数

```ts
export const reqBillsAdjustmentSum = async (data: AdjustmentItemSumReqParams): Promise<AdjustmentItemSumResData> => {
  return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/account/bills/adjustment_items/sum`, data);
};
```

### 9.2 类型定义

```ts
export interface AdjustmentItemSumReqParams {
  filter: FilterType;
}

export interface AdjustmentItemSumResult {
  count: number;
  cost_map: Record<'increase' | 'decrease', CostMap>;
}

export type AdjustmentItemSumResData = IQueryResData<AdjustmentItemSumResult>;
```

> `CostMap` 扩展为 `[currency: string]: CurrencyCost`，支持 `USD`、`CNY` 等多币种 key；`CurrencyCost` 为 `USD` 的兼容别名。

## 10. 样式调整

- 标题列 `.label`：宽度 200px，右对齐，`white-space: nowrap`，字号 12px，避免标题换行。
- 金额 `.money`：颜色 `#f59500`，字号 14px，加粗。
- 数量 `.count`：颜色 `#313238`，字号 14px。
- 各 section 间距保持 `16px 0`。
- 新增 `.sync-scope-wrapper` 与 `.vendor-wrapper` 共用同一套间距；内部 Radio 横向排列，间距 24px。

## 11. 风险与降级

- `sync_mode` 字段为新参数，若后端未发布对应版本，接口会返回 `InvalidParameter`。本期需求依赖后端同步支持该字段。
- 调账数据汇总依赖 `adjustment_items/sum` 接口，该接口已存在，复用即可。
- 仅调账模式下不再校验一级账号确认状态，避免与全量同步的确认流程混淆。
