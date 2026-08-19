# Coding — feat-cvm-apply-inherit-asset-recommend

## 执行顺序

1. **store 层**：新增 `getInheritedHostList` 方法 + 类型定义 → `src/store/cvm/device.ts`
2. **asset-list.vue**：对接接口 + Tab 逻辑 + 推荐标记 + 空态文案 + 选中事件
3. **asset-match.vue**：传递 props 给 AssetList + 自动回填 + 自动校验 + 可用区联动

> 排序依据：依赖关系（store → 组件）

## 共享改动 / 提交策略

- 跨单公共改动：无（单绑单据）
- 提交与关单策略：每单一提交（默认；与 git-commit skill 一致）

---

## 单据 1: 主机申领-滚服项目-继承固资号选择推荐-前端

**TAPD**: [#1069995598136963986](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598136963986)
**文件**: `src/store/cvm/device.ts` `src/components/device-type-selector/cvm-apply/children/asset-list.vue` `src/components/device-type-selector/cvm-apply/children/asset-match.vue`
**改动点**:

### 1. store 层新增（`src/store/cvm/device.ts`）

- 新增类型 `IInheritedHost` / `IInheritedHostGroup` / `IInheritedHostListReq`
- 新增 `getInheritedHostList` 方法：
  - 接口 A（有 bk_biz_id）：`POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list`
  - 接口 B（无 bk_biz_id）：`POST /api/v1/woa/rolling_servers/inherited_hosts/list`
  - 使用 `resolveBizApiPath` 选择接口（与 `getInheritCvm` 一致）
  - 请求参数：`{ region, device_families }`（bk_biz_id 在路径或 body）
  - 响应：`data.info`（按机型族分组）
- 新增 `inheritedHostListLoading` ref

### 2. asset-list.vue 改造

- **接收 props**：`bizId` / `region` / `requireType`（从 asset-match.vue 传入）
- **调用接口**：下拉面板展开时调用 `getInheritedHostList`
- **Tab 列表**：从接口响应 `data.info` 动态生成（标准型/高IO型/大数据型/计算型/GPU型）
- **候选数为 0 的 Tab 隐藏**：`hosts.length === 0` 的分组不展示 Tab
- **Tab 无设备文案**：选中 Tab 但 `hosts` 为空时展示「该机型族没有符合要求的CVM实例」
- **表格数据**：当前 Tab 的 `hosts` 列表
- **推荐标记**：`is_recommended === true` 的行展示橙色圆点 badge
- **计费模式展示**：`instance_charge_type`（PREPAID→包年包月 / POSTPAID_BY_HOUR→按量计费）+ `(剩余{charge_months}月)`
- **时间格式化**：`billing_start_time` / `billing_expire_time`（RFC3339 → 展示格式）
- **行点击选中**：emit `select` 事件，传 `bk_asset_id` 给 asset-match.vue
- **loading 状态**：接口加载中展示 bk-table loading
- **去掉 mock 数据**

### 3. asset-match.vue 改造

- **传递 props 给 AssetList**：`bizId` / `region` / `requireType`
- **监听 AssetList select 事件**：选中固资号后填入 model + 自动调用 `handleCheck`
- **自动回填策略**：接口返回后，有标准型优先标准型第一个，没有标准型取第一个非空分组的第一个数据
- **可用区联动**：
  - 可用区选中（非"全部"）→ 触发推荐
  - 切换可用区 → 重新推荐，覆盖之前选择
  - 可用区选到「全部」→ 清空固资号 + 解锁
- **下拉面板展开触发**：点击 trigger 输入框展开（已有 `@toggle` 逻辑）

### 4. 计费模式名称映射

- 复用 `useCvmChargeType` hook 的 `cvmChargeTypeNames`（已有）
- PREPAID → 包年包月
- POSTPAID_BY_HOUR → 按量计费