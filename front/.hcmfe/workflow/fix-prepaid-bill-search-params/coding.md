# Coding — fix-prepaid-bill-search-params

**TAPD**: [#1069995598137607072](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137607072)

> lite。落码入口：`comp-field-model`。列表 filter **只走** `transformSimpleCondition`（与操作记录相同）。多关键词 `cs` 在字段 `filterRules` 里消化，复用 `buildFilterRulesWithSearchSelect`。不新开接口、不改后端。

## 根因

`hcm-search-string` 默认 tag 多值。未走 `filterRules` 时，`transformSimpleCondition` 会把数组原样塞进一条 `cs`，后端要求 `cs.value` 是字符串。

## 嵌套 `or` 在哪生成（操作记录）

不是 `flattenAndRules`，也不是 `transformSimpleCondition` 二次加工。

1. `src/views/operation-log/children/search/condition.ts` 的 `res_name.filterRules`：多关键词 → `{ op: or, rules: [{ field, op: cs, value: 单个字符串 }, ...] }`。
2. `entry-biz.vue` / `entry-rsc.vue` 直接 `transformSimpleCondition(...)`：有 `filterRules` 则把返回值推进外层 `and.rules`（`src/utils/search.ts` 约 135–145 行），不再拆、不再摊平。

时间区间的嵌套 `and` 同理，来自 datetime 默认分支，所以现网 payload 是 `and( 嵌套and(created_at), eq(account_id), 嵌套or(res_name) )`。

## 统一做法（本单）

同类「string + 多关键词 + 包含」只保留这一条链：

```
字段 meta.search.filterRules
  → buildFilterRulesWithSearchSelect(value, field, CS)
  → transformSimpleCondition 推进外层 and
```

- 共享实现已经在 `src/utils/search.ts` 的 `buildFilterRulesWithSearchSelect`（CLB 名称/域名/可用区、监听器名称已在用）。本单在原多值 `or` / 单值解包上增加 trim 与空词过滤（可接受优化）；空结果返回 `null`，由 `transformSimpleCondition` 跳过该字段。
- 操作记录 `res_name` 改为调用该函数，去掉手写 if。
- 预付费 `gpu_type` / `product_name` 同样调用。
- **删除** `flattenAndRules` 和 `buildPrepaidBillFilter` 的再包一层；列表/导出直接 `transformSimpleCondition`。使用时间、订单月份继续用各自 `filterRules` 产出嵌套 `and`（对齐操作记录 `created_at`）。

## 文件

- `src/utils/search.ts`（`buildFilterRulesWithSearchSelect` trim / 空词过滤 / 空值返回 null）
- `src/views/operation-log/children/search/condition.ts`
- `src/views/prepaid-bill/children/list/search/condition.ts`
- `src/views/prepaid-bill/utils.ts`（去掉 flatten / `buildPrepaidBillFilter`）
- `src/views/prepaid-bill/index.vue`

## 改动点

### buildFilterRulesWithSearchSelect

多值 → 外层 `or` + 多条 `{ field, op, value: 单个字符串 }`；单值解包成字符串；trim 后空词丢弃；全部为空则返回 `null`。上层 `transformSimpleCondition` 对 falsy / 无 `value` 且无 `rules` 的结果不 `push`，该字段不会进入最终 filter。

### 字段 filterRules

```ts
filterRules: (value) => buildFilterRulesWithSearchSelect(value, 'product_name', QueryRuleOPEnum.CS)
```

GPU 型号同样，`op` 为 `cs`（包含）。

### 预付费发请求

`index.vue` 列表与导出：`transformSimpleCondition(condition, searchFields)`，类型断言为 `QueryFilterType`。时间非法仍由入口 `isInvalidDateRange` 拦截。

## 不改

- `search.vue`、账号/产品 list、漏斗、store、后端。
- 不把多词 `join` 成一个字符串。

## 验收对照

- AC-001 / AC-002：两关键词 → 嵌套 `or` + 两条 `cs` 字符串，不再报错。
- AC-003：单关键词 → 一条 `cs` + 字符串。
- AC-004：未填不进 rules。
- AC-005：两字段同时填 → 外层 `and` 下两组 `or`。
- AC-006：其他筛选仍由 `transformSimpleCondition` 默认/`filterRules` 产出（时间保持嵌套 `and`）。
- 导出与列表同一份 filter。
