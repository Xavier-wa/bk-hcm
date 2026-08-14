# Coding：推荐卡片展示机型族、CPU、内存

> 依据已审批的 prd.md / design.md / api.md。  
> 本迭代：前端 enrichment，不改 agent SSE；vendor=ZIYAN。

## 1. 改动文件清单

| # | 文件 | 类型 | 说明 |
|---|------|------|------|
| 1 | `src/hooks/chatbot/host-apply-device-meta.ts` | **新增** | 机型 meta 查询 + 会话级缓存：`device_type → { device_family, cpu_core, memory }` |
| 2 | `src/hooks/chatbot/host-apply-display.ts` | 改 | `device_type` 支持注入 meta；`formatDeviceTypeDisplay`；`toSpecDisplayItems` 可异步 enrichment |
| 3 | `src/components/chatbot/host-apply-recommend-card.vue` | 改 | 当前方案 `device_type` 变化时拉 meta，刷新「机型」行 |
| 4 | `src/components/chatbot/host-apply-preorder-table.vue` | 改 | 「机型」列走富文本；按表内去重机型批量拉 meta |
| 5 | `src/hooks/chatbot/use-host-apply-options.ts` | 改 | `getDeviceTypeOptions` 的 `name` 改为富文本 label |

> A 卡若完全走同步 `toSpecDisplayItems`，需在卡片侧持有 meta Map 再传入 display，或 display 提供异步版。优先：**共享 meta cache + 组件侧 watch 触发刷新**。

## 2. 展示格式

```ts
formatDeviceTypeDisplay(deviceType: string, meta?: DeviceMeta | null): string
// 完整：S5.LARGE8 (标准型, 4核8G)
// 部分：有则拼，缺片段省略
// 无 meta：S5.LARGE8
// deviceType 空：''
```

| 片段 | 字段 | 规则 |
|------|------|------|
| 编码 | `device_type` | 始终在最前 |
| 机型族 | `device_family` | 有则放入括号 |
| CPU | `cpu_core` | 拼 `{n}核` |
| 内存 | `memory` | 拼 `{n}G`（不做单位换算） |

## 3. Meta 数据层（#1）

复用 `useCvmDeviceStore().getOneDevicetype`，与 `use-chatbot-backfill.ts` 的 `fetchCpuCoreMap` 同模式，扩展为完整 meta：

```ts
filter: {
  op: 'and',
  rules: [
    { field: 'vendor', op: 'eq', value: VendorEnum.ZIYAN },
    { field: 'device_type', op: 'eq', value: deviceType },
  ],
}
```

| 能力 | 说明 |
|------|------|
| `ensureDeviceMeta(deviceType)` | 单条，命中缓存直接返回 |
| `ensureDeviceMetaMap(deviceTypes[])` | 去重并发，写回缓存 |
| 失败 | 缓存记「空 meta」，展示回退编码，不抛到 UI |

不强制 `region` 过滤（与 api.md Q3 / backfill 一致）。

## 4. Template A（#2 + #3）

- `watch` 当前页 `suborder.device_type` → `ensureDeviceMeta`
- `getSpecFieldText` / `toSpecDisplayItems` 对 `device_type`：若传入 meta Map 则富文本，否则先编码
- 首屏可先显示编码，meta 返回后原地更新（AC-006）
- 其余字段逻辑不变

## 5. Template B/D（#4）

- 机型列：`formatDeviceTypeDisplay(row.device_type, metaMap[row.device_type])`
- `watch(suborders)` → `ensureDeviceMetaMap(去重 device_type)`
- 列宽可略增（现 `min-width=180`），过长靠 `show-overflow-tooltip`

## 6. Template C 下拉（#5）

`getDeviceTypeOptions`：

```ts
// 现状
{ id: device_type, name: device_type }

// 改为（list 已含 family/cpu/memory）
{ id: device_type, name: formatDeviceTypeDisplay(device_type, item) }
```

选中值仍为 `device_type` 编码；无需额外单条请求。

## 7. 不改动范围

- agent SSE / `HostApplySuborder` 类型（可不加可选字段）
- 非 chatbot 申领表单 UI
- 顶部推荐摘要文案
- 后端接口

## 8. 关键图标语义

本迭代无新增图标（design 已声明 N/A）。

## 9. 自测要点（编码后）

1. A 卡：有配置机型 → 富文本；无配置 → 仅编码
2. B/D 表：多行不同机型均 enrichment，同机型只请求一次
3. C 下拉：选项含规格；选中后表单值仍是编码
4. floating / fullpage 行为一致（共用 hook）
5. 接口失败：卡片可操作，机型显示编码

## 10. 实施顺序

1. 新增 `host-apply-device-meta.ts` + `formatDeviceTypeDisplay`
2. 改 `host-apply-display.ts` 接 meta
3. 改 recommend-card / preorder-table
4. 改 `getDeviceTypeOptions`
5. `bkdevbuddy_lint` + drift_check
