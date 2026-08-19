# Coding — feat-cvm-apply-subnet-available-ip

## 执行顺序

1. `cvm-subnet-selector`：`ICvmSubnet` 加 `available_ip_count` 字段 + Option name 追加「可用IP」展示

> 排序依据：单组件改动，两场景（申领 + 修改）共用，改一处即可覆盖全部验收项。

## 共享改动 / 提交策略

- 全部改动集中在 `cvm-subnet-selector/index.vue` 一个文件，单提交即可。
- 口径说明文案放在 `network-info-collapse-panel`（子网表单项附近），因组件本身是纯选择器，提示文案由使用方承载更合适。

---

## 单据 1: 海垒申领界面及修改界面增加子网网段"可用IP"数量显示

**TAPD**: [#1069995598137153583](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137153583)
**文件**: `src/views/ziyanScr/components/cvm-subnet-selector/index.vue` `src/views/ziyanScr/hostApplication/components/network-info-collapse-panel/index.vue`
**改动点**:
- `cvm-subnet-selector/index.vue`：`ICvmSubnet` 类型新增 `available_ip_count: number | null`；Option 渲染 name 由 `${subnetId} | ${subnetName}` 改为 `${subnetId} | ${subnetName} | 可用IP: ${available_ip_count ?? '?'}`
- `network-info-collapse-panel/index.vue`：子网表单项下方加口径说明「可用IP 为云厂商未分配可用 IP 数」

## 实现细节

### 1. 字段类型（api.md §5 对齐）

`ICvmSubnet` 新增字段：

```ts
export interface ICvmSubnet {
  id: number;
  region: string;
  zone: string;
  vpc_id: string;
  vpc_name: string;
  subnet_id: string;
  subnet_name: string;
  enable: boolean;
  comment: string;
  available_ip_count: number | null; // 剩余可用 IP，取自 CRP leftIpNum；null 表示查询失败/未取到
}
```

### 2. Option 展示

模板中 Option 的 `:name` 追加可用 IP：

```vue
<Option
  v-for="{ id, subnet_id: subnetId, subnet_name: subnetName, available_ip_count: availableIpCount } in filteredOptionList"
  :key="id"
  :id="id"
  :name="`${subnetId} | ${subnetName} | 可用IP: ${availableIpCount ?? '?'}`"
/>
```

- `availableIpCount ?? '?'`：字段为 `null`（查询失败/未取到）时显示 `?`；`0` 是合法数值，正常显示 `可用IP: 0`（`??` 只兜 null/undefined，不会误伤 0）。
- 不因 IP 为 0 或 null 禁用选项（R-004）。

### 3. 口径说明

`network-info-collapse-panel/index.vue` 子网表单项（`cvm-subnet-selector`）下方，与 `bcs-select-tips` 同级位置追加：

```vue
<div class="subnet-available-ip-tip">可用IP 为云厂商未分配可用 IP 数</div>
```

样式对齐现有 `network-tip` 小字号弱化风格（`font-size: 12px; color: #979ba5;`）。

### 4. 边界与降级

- `null`（查询失败/未取到）→ 显示 `可用IP: ?`；真实 `0` → 显示 `可用IP: 0`（api.md §6 已决策，2026-08-19 更新）
- 仅自研云场景：组件本身即自研云专用（AC-004 天然满足）
- 不拦截、不改提交流程（AC-002）
