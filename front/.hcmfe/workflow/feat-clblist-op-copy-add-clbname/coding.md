# coding.md — 负载均衡-列表-复制负载均衡名称

## 需求简述

在 CLB 负载均衡列表的批量复制下拉菜单中增加“负载均衡名称”复制项。

## 改动文件

### `front/src/views/load-balancer/clb/children/batch-copy.vue`

| 改动 | 说明 |
|------|------|
| 新增 `selectedLoadBalancerNames` computed | 从勾选的 selection items 中提取 `item.name`，换行拼接 |
| 新增“负载均衡名称”复制项 | 在“负载均衡ID”前插入 `<copy-to-clipboard>` 条目 |

**下拉菜单复制项顺序**（修改后）：
1. 负载均衡名称 ← 新增
2. 负载均衡ID
3. 负载均衡VIP
4. 负载均衡域名

## 影响面

- 仅影响 `load-balancer/clb/load-balancer-table.vue` 的批量复制下拉菜单
- Device 模块的 `batch-copy.vue` 是独立组件，不受影响
