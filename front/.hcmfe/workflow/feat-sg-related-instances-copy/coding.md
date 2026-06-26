# Coding — 安全组-关联实例-增加复制按钮

## 1. 实施方案概览

| 步骤 | 文件 | 改动类型 | 说明 |
|------|------|----------|------|
| 1 | `platform.vue` | 修改 | 新增复制下拉按钮 + 跨页选择 |
| 2 | `collapse-data-list.vue` | 修改 | 新增一键复制按钮 |
| 3 | 跨页选择 | 可能新增/修改 | 若当前不支持则需实现 |

---

## 2. 步骤1：platform.vue — 复制下拉按钮

### 2.1 引入依赖

```ts
import HcmDropdown from '@/components/hcm-dropdown/index.vue';
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';
import { getPrivateIPs } from '@/utils/common';
```

### 2.2 新增 computed

```ts
// 复制按钮是否禁用：与批量解绑逻辑一致
const copyDisabled = computed(() => !selected.value.length);

// 勾选行的内网 IP（换行分隔）
const selectedIPs = computed(() =>
  selected.value
    .map((item) => getPrivateIPs(item))
    .filter(Boolean)
    .join('\n'),
);

// 勾选行的实例 ID（cloud_id，换行分隔）
const selectedCloudIds = computed(() =>
  selected.value
    .map((item) => item.cloud_id)
    .filter(Boolean)
    .join('\n'),
);
```

### 2.3 模板新增

在 `.operate-btn-wrap` 中「批量解绑」按钮后面新增：

```vue
<HcmDropdown :disabled="copyDisabled">
  {{ t('复制') }}
  <template #menus>
    <CopyToClipboard
      type="dropdown-item"
      :content="selectedIPs"
      :text="t('复制内网IP')"
      :disabled="copyDisabled"
    />
    <CopyToClipboard
      type="dropdown-item"
      :content="selectedCloudIds"
      :text="t('复制实例ID')"
      :disabled="copyDisabled"
    />
  </template>
</HcmDropdown>
```

### 2.4 跨页选择

当前 `data-list` 通过 `@select` 事件回传选中数据到 `selected`。需确认是否支持跨页：

- **若已支持**：无需改动
- **若未支持**：参照项目中 `useTableSelection` hook 或其他已支持跨页的列表页实现
  - 关键：翻页时保留已选数据，`selected` 需合并跨页选择记录
  - 兼容：确保「批量解绑」依然拿到正确的选中数据

---

## 3. 步骤2：collapse-data-list.vue — 一键复制按钮

### 3.1 引入依赖

```ts
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';
import { getPrivateIPs } from '@/utils/common';
import rollRequest from '@blueking/roll-request';
import http from '@/http';
```

### 3.2 新增 computed 和方法

```ts
// 拉取当前业务全部关联资源（分页时用 rollRequest）
const fetchAllRelRes = async (): Promise<SecurityGroupRelResourceByBizItem[]> => {
  // 如果只有一页数据，直接返回
  if (pagination.count <= pagination.limit) {
    return relResList.value;
  }
  // 多页时用 rollRequest 拉取全部
  const allList = await rollRequest({
    httpClient: http,
    pageEnableCountKey: 'count',
  }).rollReqUseCount<SecurityGroupRelResourceByBizItem>(
    apiPath, // 与 getList 中相同的 API 路径
    {
      filter: transformSimpleCondition(props.condition, RELATED_RES_PROPERTIES_MAP[props.tabActive]),
    },
    {
      limit: 500,
      countGetter: (res) => res.data.count,
      listGetter: (res) => res.data.details,
    },
  );
  return allList;
};

// 复制用的 content 函数（异步获取全部数据后拼接）
const copyAllIPs = async () => {
  const list = await fetchAllRelRes();
  return list
    .map((item) => getPrivateIPs(item))
    .filter(Boolean)
    .join('\n');
};

const copyAllCloudIds = async () => {
  const list = await fetchAllRelRes();
  return list
    .map((item) => item.cloud_id)
    .filter(Boolean)
    .join('\n');
};

// 按钮禁用
const copyDisabled = computed(() => relResList.value.length === 0);
```

### 3.3 模板新增

在 `.tools` 的 `isCurrentBusiness` 分支中，「新增绑定」和「批量解绑」之间新增：

```vue
<CopyToClipboard
  :content="copyAllIPs"
  :text="t('复制内网IP')"
  :disabled="copyDisabled"
/>
<CopyToClipboard
  :content="copyAllCloudIds"
  :text="t('复制主机ID')"
  :disabled="copyDisabled"
/>
```

---

## 4. 组件 API 速查

### CopyToClipboard

| Prop | 类型 | 说明 |
|------|------|------|
| `content` | `string \| () => Promise<string>` | 直接字符串或异步函数 |
| `type` | `'icon' \| 'dropdown-item'` | 渲染模式 |
| `text` | `string` | tooltip / 按钮文本 |
| `disabled` | `boolean` | 禁用 |
| `disabledTips` | `string` | 禁用提示 |

### HcmDropdown

| Prop | 类型 | 说明 |
|------|------|------|
| `disabled` | `boolean` | 禁用 |
| slot `default` | — | 按钮文本 |
| slot `menus` | — | 下拉菜单项 |

### getPrivateIPs

```ts
// 拼接 private_ipv4_addresses + private_ipv6_addresses，逗号分隔
const getPrivateIPs = (data) =>
  [...(data.private_ipv4_addresses || []), ...(data.private_ipv6_addresses || [])].join(',') || '--';
```

---

## 5. 注意事项

- CopyToClipboard 的 `content` 支持异步函数，功能2 利用此特性在点击时才拉取全量数据
- HcmDropdown 的 `disabled` 控制按钮和下拉展开
- 跨页选择改动需回退验证「批量解绑」功能不受影响
- 不使用 `useVerify` / `authVerifyData`，本功能不涉及权限校验
- 复制按钮无权限控制，所有有数据可见的用户均可使用
