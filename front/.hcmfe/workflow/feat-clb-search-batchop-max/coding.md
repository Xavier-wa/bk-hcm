# Coding — 负载均衡配置检索批量操作上限放开

## 背景

TAPD: `https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598135727363`

配置检索结果页监听器 Tab 批量操作（删除 / 导出 / 复制）在选中数量超过 1000 时被禁用；需求放开至 5000。

## 范围

| 项 | 决策 |
|---|---|
| 批量操作上限 | 1000 → **5000** |
| 总量 10000 展示 LargeData | **不变** |
| 后端批量删除 1000 限制 | **不在本次范围**（后端另修） |
| 当页全选跨页保留 | **不纳入** |

## 改动点

### `front/src/views/load-balancer/device/main-content/listener-table.vue`

- `const max = 1000` → `const max = 5000`
- `moreData` 逻辑不变：超过上限时展示警告条并禁用批量操作按钮
- RS Tab 已在 `rs-ip-group.vue` 使用 `MAX_COUNT = 5000`，无需改动

## 验证要点

1. 监听器 Tab 选中 ≤5000 条：批量删除 / 导出 / 复制可用
2. 选中 >5000 条：警告条出现，批量按钮禁用
3. 总量 >10000 时仍展示 LargeData 占位页（行为不变）
