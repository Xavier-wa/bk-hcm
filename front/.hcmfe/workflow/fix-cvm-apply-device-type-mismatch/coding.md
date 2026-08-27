# Coding — fix-cvm-apply-device-type-mismatch

## 执行顺序

1. bug 1069995598162077646 — 在 `handleZoneSelect` 和 `handleDeviceGroupChange` 末尾清空 `condition.deviceType/cpu/mem`

> 排序依据：两处改动同文件同模式，一并完成

## 共享改动 / 提交策略

- 跨单公共改动：无（单张 bug）
- 提交与关单策略：单提交（与 git-commit skill 一致）

---

## 单据 1: 【HCM】主机申领-机型列表和选择 没有匹配

**TAPD**: [#1069995598162077646](https://<TAPD_HOST>/tapd_fe/69995598/bug/detail/1069995598162077646)
**文件**: `src/components/device-type-selector/cvm-apply/children/device-type-dialog.vue`
**改动点**:

1. **改动 A**（L527-537）: 在 `handleZoneSelect` 和 `handleDeviceGroupChange` 末尾追加三行清空 `condition.deviceType/cpu/mem`
2. **改动 B**（L785）: 将 `bk-option` 的 `:key="index"` 改为 `:key="item.device_type"`

### 根因（两处独立缺陷）

**缺陷 1 — condition 残留**：`handleZoneSelect` / `handleDeviceGroupChange` 切换时只清了 `selectedRowKeys`（表格勾选），没有清 `condition.deviceType/cpu/mem`（下拉 v-model）。`condition.deviceType` 残留旧可用区的机型 → 下拉里旧 option 仍带勾，表格筛选基于旧 condition。

**缺陷 2 — bk-option :key="index" 导致 optionsMap 污染**（更隐蔽）：
- bk-option 组件只在 `onBeforeMount` 调用 `select.register(optionID, proxy)` 注册自身到 `optionsMap`，没有 watch `optionID` 来重新注册
- `:key="index"` 使得切换可用区后，Vue 复用相同 index 的 option 组件，只更新 props，不触发 unmount/remount
- `optionsMap` 的 key（旧 ID）没变，但 proxy.optionName（computed 读的是当前 name prop）变成了新列表同位置的数据
- 用户新选 `BMS4.20XLARGE384` → `optionsMap.get('BMS4.20XLARGE384')` 找到旧 index 3 的 proxy → proxy 的 name 已被 patched 为另一个机型 → trigger 显示错误的机型名

`selectedLabel` 渲染链（bkui-vue select/index.js）：
```
selectedLabel computed → optionsMap.get(value).optionName → proxy.optionName (computed, 读当前 name)
```

### 修复

**改动 A**（清空 condition）：
```ts
condition.deviceType = [];
condition.cpu = [];
condition.mem = [];
```

**改动 B**（修复 key）：
```diff
- <bk-option v-for="(item, index) in option.deviceTypeList" :key="index" ... />
+ <bk-option v-for="item in option.deviceTypeList" :key="item.device_type" ... />
```

改为 `:key="item.device_type"` 后，切换可用区时 old options 被销毁(new key) → `onBeforeUnmount` unregister → 新 options 创建 → `onBeforeMount` register → `optionsMap` 始终正确。

### 验收

- AC-001：切换可用区后，下拉 trigger 显示新列表第一项，下拉里旧选项不再带勾，下拉重新选择后 trigger 正确显示所选机型
- AC-002：切换机型族后，下拉 trigger 与表格首项一致，下拉里无残留勾选
- AC-003：重新选择机型后，trigger 文本与所选机型一致（不再出现选 A 显 B）
- AC-004：固资号匹配后 trigger 与表格首项一致（待 QA 验证）

### 状态

- [x] 改动 A（清空 condition）已完成
- [x] 改动 B（修复 :key）已完成
- [x] lint 通过（无新错误，仅有既有的 cSpell 拼写提示和 h 未使用 hint）
- [ ] 用户确认推进到 test
