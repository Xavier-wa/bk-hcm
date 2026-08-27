# 负载均衡设备页 RS 搜索报错修复 — 技术方案

## 概述

修复负载均衡「配置检索」页面中，搜索 RS IP 后清空再搜索另一个 IP 时出现的两个 JavaScript 报错及连锁的 UI 功能失效。

## 问题分析

### 症状

1. **报错 1**：`Cannot read properties of null (reading 'emitsOptions')`
2. **报错 2**：`Cannot read properties of undefined (reading 'targets')`
3. **连锁失效**：IP 分组复选框无法全选、批量调整 RS 权重弹窗打不开

### 根因链条

```
用户清空搜索 → 两次请求并发 + 旧 rowKey 残留
    → selectedRsMap 与 allList 不同步 → 报错 (targets/emitsOptions)
    → selections 为空 → 批量按钮 disabled → 弹窗打不开
```

## 修改清单

### 1. `rs-ip-group.vue` — allList watch 完全重置 selectedRsMap

**位置**：`props.allList` 的 watch（约第 151 行）

**改动前**：
```ts
watch(() => props.allList, (list) => {
  list.forEach((item) => {
    selectedRsMap.value.set(item.rowKey, []);
    IPCheckStatus.value[item.rowKey] = false;
  });
});
```

**改动后**：
```ts
watch(() => props.allList, (list) => {
  const newSelectedRsMap = new Map<string, string[]>();
  const newIPCheckStatus: { [key: string]: boolean } = {};
  list.forEach((item) => {
    newSelectedRsMap.set(item.rowKey, []);
    newIPCheckStatus[item.rowKey] = false;
  });
  selectedRsMap.value = newSelectedRsMap;
  IPCheckStatus.value = newIPCheckStatus;
});
```

**原因**：旧代码仅增量添加 key，不清除旧搜索残留的 key。数据结构保持同步。

### 2. `rs-ip-group.vue` — selectedRsMap deep watch 加 null 防御

**位置**：`selectedRsMap` 的 deep watch（约第 113 行）

**改动**：
```ts
const item = props.allList.find((item) => item.rowKey === key);
// 新增：allList 更新后可能不再包含旧 key
if (!item) continue;
result.push({
  ...item,
  targets: (item.targets || []).filter(...), // 加 || [] 兜底
});
```

**原因**：防御性代码，处理 `selectedRsMap` 与 `allList` 不同步的过渡瞬间。

### 3. `rs-table.vue` — allList 从 let 改为 ref

**位置**：第 30 行

**改动**：`let allList: IRsItem[] = []` → `const allList = ref<IRsItem[]>([])`

**原因**：`allList` 通过 props 传给 `rs-ip-group` 用于计算 IP 分组下的全部 RS ID（`getRowTargetIds`）。非响应式变量在重渲染时序下可能拿到旧值，导致全选勾选失效。Vue 模板自动解包 ref，子组件行为不变。

### 4. `rs-table.vue` — getList 增加 requestId 竞态保护

**位置**：`getList` 函数（约第 97 行）

**改动**：增加 `requestId` 自增计数器，await 后检查是否仍为最新请求，过期请求直接 `return` 跳过所有渲染和 emit。

**原因**：「清空→再搜」场景下两次请求并发，后到者可能覆盖先到者的结果，导致数据错乱和重复渲染。令牌机制确保仅最新一次搜索生效。

## 影响范围

| 场景 | 改动前 | 改动后 |
|------|--------|--------|
| 单次搜索 RS IP | ✅ 正常 | ✅ 正常（行为不变） |
| 搜索 → 清空 → 再搜 | ❌ 报错 + 功能失效 | ✅ 正常 |
| 快速连续搜索 | ❌ 数据闪烁/错乱 | ✅ 仅展示最后一次结果 |
| 批量操作弹窗删除 IP | ✅ 正常 | ✅ 正常（行为不变） |
| 批量操作弹窗（其他 type） | ✅ 正常 | ✅ 正常（行为不变） |
| 监听器 tab / URL 规则 tab | ✅ 不影响 | ✅ 不影响 |

## 涉及文件

```
front/src/views/load-balancer/device/main-content/
  rs-table.vue                           # allList ref 化 + requestId 竞态保护
  children/
    rs-ip-group.vue                      # Map 重置 + null 防御
```

## 未修改的手动验证项

- 确认 `rs-ip-group.vue` 在 `batch-rs-operation-dialog.vue` 中的使用不受影响（该场景 `allList` 与 `rsList` 同源、不变化）
