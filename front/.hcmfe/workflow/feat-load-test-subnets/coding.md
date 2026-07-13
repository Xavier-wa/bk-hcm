# Coding: 自研云-主机申领-网络信息增加压测子网配置类型

> 对应 Design: `.hcmfe/workflow/feat-load-test-subnets/design.md`
> 对应 API: `.hcmfe/workflow/feat-load-test-subnets/api.md`
> 最后更新: 2026-06-09

---

## 1. 架构方案：filter prop 模式

Panel 仅请求压测索引接口，VPC/子网的详情请求仍由各自 Selector 内部自行管理。Panel 通过 `filter` prop 传入过滤函数，Selector 在 `optionList` 更新后自动应用过滤。

```
load_test_subnets (索引)          选择器内部请求 (VPC/子网详情)
      │                                    │
      │ { region: { vpc: [subnet] } }      │ 返回完整 optionList
      │                                    │
      ▼                                    ▼
 ┌─────────────────────────────────────────────────────────────┐
 │            NetworkInfoCollapsePanel 仅管理 filter 函数       │
 │                                                             │
 │  ★ Panel 只请求 load_test_subnets，不重复请求 VPC/子网      │
 │                                                             │
 │  ★ computed filter 函数（传给选择器）:                       │
 │  vpcFilter: regionPressure ? 按 configType 过滤 : undefined │
 │  subnetFilter: regionPressure ? 按 configType 过滤 : undefined│
 │                                                             │
 │  ★ 选择器内部:                                               │
 │  1. watch region/zone/vpc → 请求接口 → optionList           │
 │  2. filteredOptionList = filter(optionList) → 展示          │
 │                                                             │
 │  ★ 切换 configType 不重新请求接口，仅 computed 切换过滤方向 │
 └─────────────────────────────────────────────────────────────┘
```

---

## 2. TypeScript 类型定义

```ts
/** 配置类型 */
type ConfigType = 'auto' | 'pressure-test';

/** 压测子网索引：{ [region]: { [vpcId]: subnetId[] } } */
interface IPressureIndex {
  [region: string]: {
    [vpcId: string]: string[];
  };
}

/** 压测子网接口响应结构 */
interface ILoadTestSubnetsResponse {
  data: IPressureIndex;
}
```

---

## 3. 组件实现细节

### 3.1 CvmVpcSelector 改动

**新增 prop**:

```ts
filter?: (list: ICvmVpcList) => ICvmVpcList;
```

**新增 computed**: 对原始 `optionList` 应用 filter 生成展示用列表。

```ts
const filteredOptionList = computed(() => (props.filter ? props.filter(optionList.value) : optionList.value));
```

**displayOptionList 调整**: 基于 `filteredOptionList`（而非 `optionList`）排序，确保展示与筛选结果一致。

**不变**: `selectedId` 和 `findCvmVpcByVpcId` 仍基于 `optionList`（完整列表），保证回填值和选中项匹配不受 filter 影响。Selector 内部始终自行请求接口（watch region），`filter` 仅对结果做筛选。

---

### 3.2 CvmSubnetSelector 改动

**新增 prop**:

```ts
filter?: (list: ICvmSubnetList) => ICvmSubnetList;
```

**新增 computed**:

```ts
const filteredOptionList = computed(() => (props.filter ? props.filter(optionList.value) : optionList.value));
```

**模板调整**: 使用 `filteredOptionList` 替代 `optionList` 渲染选项。

**不变**: Selector 内部始终自行请求接口（watch region/zone/vpc）。

---

### 3.3 NetworkInfoCollapsePanel 核心实现

#### 3.3.1 新增状态

```ts
const configType = ref<ConfigType>('auto');           // 默认「自动匹配」
const pressureIndex = ref<IPressureIndex>({});        // 压测索引
```

#### 3.3.2 压测选项可用性

```ts
const pressureTestDisabled = computed(() => {
  const regionData = pressureIndex.value[props.region];
  return !regionData || Object.keys(regionData).length === 0;
});
```

#### 3.3.3 Filter 函数实现

```ts
const vpcFilter = computed(() => {
  const regionPressure = pressureIndex.value[props.region];
  if (!regionPressure) return undefined;
  const pressureVpcIds = new Set(Object.keys(regionPressure));
  // auto 模式：VPC 列表展示全部（不做排除），子网仍排除压测项
  return configType.value === 'auto'
    ? undefined
    : (list: ICvmVpc[]) => list.filter((v) => pressureVpcIds.has(v.vpc_id));
});

const subnetFilter = computed(() => {
  const regionPressure = pressureIndex.value[props.region];
  if (!regionPressure) return undefined;
  const vpcId = vpc.value;
  const pressureSubnetIds = new Set(regionPressure[vpcId] || []);
  return configType.value === 'auto'
    ? (list: ICvmSubnet[]) => list.filter((s) => !pressureSubnetIds.has(s.subnet_id))
    : (list: ICvmSubnet[]) => list.filter((s) => pressureSubnetIds.has(s.subnet_id));
});
```

**过滤关系**：
- auto 模式：VPC 展示全部（不做排除），子网排除压测项
- pressure-test 模式：VPC 仅保留压测项，子网仅保留压测项

#### 3.3.4 请求函数

```ts
const fetchPressureIndex = async () => {
  try {
    const res = await http.get<ILoadTestSubnetsResponse>('/api/v1/woa/config/load_test_subnets');
    pressureIndex.value = res.data || {};
  } catch {
    pressureIndex.value = {};
  }
};

// 组件挂载时获取一次，缓存在内存中
fetchPressureIndex();
```

#### 3.3.5 配置类型切换逻辑

```ts
const handleConfigTypeChange = () => {
  vpc.value = '';
  subnet.value = '';
  selectCvmVpc.value = null;
  selectedCvmSubnet.value = null;
};
```

#### 3.3.6 压测选项可用性检查

```ts
watch([() => props.region, pressureIndex], () => {
  if (pressureTestDisabled.value && configType.value === 'pressure-test') {
    configType.value = 'auto';
  }
});
```

#### 3.3.7 修改场景自动推断

```ts
const autoInferConfigType = () => {
  const regionData = pressureIndex.value[props.region];
  if (!regionData) {
    configType.value = 'auto';
    return;
  }
  const vpcVal = vpc.value;
  const subnetVal = subnet.value;
  const isPressureVpc = vpcVal ? vpcVal in regionData : false;
  const isPressureSubnet = subnetVal && vpcVal ? regionData[vpcVal]?.includes(subnetVal) : false;
  configType.value = isPressureVpc || isPressureSubnet ? 'pressure-test' : 'auto';
};
```

**触发时机**: panel expand（`handleToggle(true)`）时调用一次。

#### 3.3.8 模板关键结构

**标题行**（`#default` slot）：

- `bk-collapse-panel` 使用 `#default` slot 自定义标题区域
- 包含标题文本、提示图标+文本、折叠状态概览

**配置类型 Radio**（`#content` slot 内顶部）：

- `bk-form-item` + `bk-radio-group`，`v-if="!props.disabled"`
- 「用于压测」按钮绑定 `v-bk-tooltips`，disabled 时显示提示

**压测须知**（Radio 和 VPC 之间）：

- `bk-alert type="warning"`，`v-if="configType === 'pressure-test'"`

**VPC/子网选择器**：

- 传入 `:filter="vpcFilter"` / `:filter="subnetFilter"`

---

### 3.4 bk-collapse-panel Slot 方案

`bk-collapse-panel` 的 slot 优先级：`#default` > `#header` > `title` prop。

采用 `#default` slot，统一在 header 区域渲染标题 + 提示 + 折叠概览，移除原 `:title` prop。

---

## 4. 样式实现

新增样式类：

```scss
// 标题行提示
.network-header { display: inline-flex; align-items: center; gap: 8px; font-weight: 700; }
.network-title { color: #313238; }
.network-tip { display: inline-flex; align-items: center; gap: 4px; font-weight: normal; font-size: 12px; color: #979ba5;
  .network-tip-text { max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
}

// 压测须知
.pressure-test-notice { margin-bottom: 16px;
  ul { margin: 4px 0 0; padding-left: 16px; list-style: disc;
    li { font-size: 12px; line-height: 20px; color: #63656e; }
  }
}
```

---

## 5. 降级处理

| 场景 | 行为 |
|------|------|
| `GET /load_test_subnets` 请求失败 | `pressureIndex` 保持 `{}`，filter 返回 `undefined`（展示全部），`pressureTestDisabled = true` |
| `GET /load_test_subnets` 返回 `data: {}` | 判定无压测配置，`pressureTestDisabled = true` |
| 当前 region 不在 `pressureIndex` 中 | `pressureTestDisabled = true` |

---

## 6. 实现步骤

| 步骤 | 文件 | 内容 |
|------|------|------|
| 1 | `cvm-vpc-selector/index.vue` | 新增 `filter` prop + `filteredOptionList` computed |
| 2 | `cvm-subnet-selector/index.vue` | 新增 `filter` prop + `filteredOptionList` computed |
| 3 | `network-info-collapse-panel/index.vue` | 新增类型定义 + configType 状态 |
| 4 | 同上 | 新增标题行提示图标和文本（#default slot） |
| 5 | 同上 | 新增配置类型 RadioButton UI |
| 6 | 同上 | 实现压测接口调用和 filter 函数 |
| 7 | 同上 | 实现压测选项可用性检查（region watch） |
| 8 | 同上 | 实现压测使用须知 |
| 9 | 同上 | 实现修改场景自动推断逻辑 |
| 10 | 同上 | 新增样式 |

---

## 7. 风险点

| 风险 | 缓解措施 |
|------|---------|
| Selector filter 未传入时行为异常 | `filter` 默认 `undefined`，`filteredOptionList` 直接返回 `optionList`，零影响 |
| filter 函数频繁调用造成性能问题 | filter 通过 computed 返回，仅在依赖变化时更新；选择器内部 `filteredOptionList` 也由 computed 缓存 |
| `load_test_subnets` 返回全地域索引，响应体可能较大 | 缓存全量索引在内存中，仅请求一次，region 切换不重新请求 |
| 自动推断时序问题（回填值就绪但索引未加载完） | Panel expand 后调用 `autoInferConfigType`，此时 `pressureIndex` 已在组件挂载时获取 |
| 第三处引用 `cvm-produce` 受 selector 改动影响 | `filter` 默认 `undefined`，不传时完全走原有逻辑，零影响 |
| configType 切换后选中值残留 | `handleConfigTypeChange` 中立即清空 `vpc` / `subnet` / `selectCvmVpc` / `selectedCvmSubnet` |
