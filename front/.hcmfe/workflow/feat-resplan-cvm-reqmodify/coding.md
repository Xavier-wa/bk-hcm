# Coding - 资源预测 2027 CVM 预算提报 · 修改入口 + 截止期限制

- **Requirement**: req-resource-plan-2027-cvm-budget
- **Iteration**: feat-resplan-cvm-reqmodify
- **执行依据**: 本文档 + prd.md / design.md / api.md (已 approve)

## 1. 改动总览

### 1.1 修改入口

| # | 类型 | 文件 / 路径 | 说明 |
|---|----|---|----|
| 1 | 新增 | `views/business/resource-plan/modify/` (整目录 copy 自 `add/`) | 修改页独立目录 |
| 2 | 新增 | `views/business/resource-plan/modify/index.tsx` | 修改页 setup, 含详情拉取 + 回填 |
| 3 | 新增 | `views/business/resource-plan/modify/header/index.tsx` | Header, 文案改「修改资源预测」, 返回目标改为原详情页 |
| 4 | 新增 | `views/business/resource-plan/modify/button/index.tsx` | 提交按钮, 调用 `overwriteBizPlan`, 成功后跳回原详情页 |
| 5 | 沿用 | `views/business/resource-plan/modify/basic/` / `list/` / `memo/` | 直接 copy, 无逻辑改动 |
| 6 | 新增 | `router/module/business.ts` 紧邻 `BizResourcePlanAdd` | 新增 `BizResourcePlanModify` 路由 |
| 7 | 新增 | `store/resourcePlan.ts` | 新增 `overwriteBizPlan(bizId, ticketId, data)` action |
| 8 | 新增 | `typings/resourcePlan.ts` | 新增 `IPlanTicketOverwrite` 类型 (字段全部可选) |
| 9 | 修改 | `components/resource-plan/applications/detail/approval/index.tsx` | 加按钮槽位 (Approval 面板内, 右侧) |
| 10 | 修改 | `views/ticket/children/resource-plan/sub-ticket/sub-ticket-list.vue` | 加按钮 (`#title-extra` 最右侧) |
| 11 | 新增 | `views/ticket/children/resource-plan/detail/use-modifiable.ts` | `useTicketModifiable` 聚合判断 hook |
| 12 | 修改 | `views/ticket/children/resource-plan/detail/index.vue` | 把 `isModifiable` 计算结果作为 prop 透传给 Approval 和 SubTicketList |

### 1.2 添加/修改 Sideslider 改为关闭即销毁 (修复回填概率失败)

| # | 类型 | 文件 / 路径 | 说明 |
|---|----|---|----|
| - | 修改 | `components/resource-plan/add/index.tsx` | `CommonSideslider` 显式传 `renderType='if'` |

**背景**: `CommonSideslider` 默认 `renderDirective='show'`, Sideslider 关闭时内容仅 v-show 隐藏不销毁。`Cvm` 子组件因此一直 mount, 其 `watch device_class` 在跨 demand 编辑切换时, `oldVal` 是上一次编辑的 device_class 值, truthy → 误重置 `device_type=''`, 导致"机型规格"概率回填失败。

**修复**: 改为 `renderType='if'`, sideslider 关闭即销毁内容; 每次打开重新 mount, `onBeforeMount` 重新拉机型列表; watch 重新初始化, `oldVal` 捕获时已是 `initData` 写入的当前 demand 值。用户手动改 device_class 时才触发预期重置, 符合 watch 原本设计意图。

**影响面**: 添加页 + 修改页共用 `Add` 组件, 同时受益。代价: 每次打开 sideslider 多一次 `getDeviceClasses` / `getDeviceTypes` 请求, 频率低, 可接受。

### 1.3 子单列表「终止」按钮可用状态扩展

| # | 类型 | 文件 / 路径 | 说明 |
|---|----|---|----|
| - | 修改 | `views/ticket/children/resource-plan/sub-ticket/sub-ticket-list.vue` | `terminatedBtnDisabled` 改为基于 `TERMINATABLE_STATUSES` 常量数组判定; 白名单扩展为 `failed` / `partial_failed` / `rejected` / `partial_rejected` |

```ts
const TERMINATABLE_STATUSES: TicketStatus[] = ['failed', 'partial_failed', 'rejected', 'partial_rejected'];
const terminatedBtnDisabled = computed(() => !TERMINATABLE_STATUSES.includes(props.ticketStatus));
```

### 1.4 截止期限制

| # | 类型 | 文件 / 路径 | 说明 |
|---|----|---|----|
| 13 | 新增 | `views/business/resource-plan/constants.ts` | 「非本年起始日」常量 `NON_CURRENT_YEAR_START_DATE` (默认 `'2027-01-01'`, 可为空) |
| 14 | 新增 | `store/resourcePlan.ts` | 新增 `getReportDeadline()` action |
| 15 | 新增 | `views/business/resource-plan/use-deadline-restrict.ts` | `useDeadlineRestrict()` hook: 封装拉取 deadline + 判定函数 (添加页 / 列表共用) |
| 16 | 修改 | `components/resource-plan/add/basic/index.tsx` | 期望到货日期 picker 加 `disabledDate` 钩子 + 评审期提示文案 |
| 17 | 修改 | `components/resource-plan/resource-manage/list/table/index.tsx` (业务侧 + 服务侧共用列表组件) | 行内操作 + 批量操作禁用; tooltip 提示 |

## 2. 详细实现

### 2.1 Store: 新增 overwrite action

文件: `src/store/resourcePlan.ts`

```ts
// 紧邻 createBizPlan 添加
overwriteBizPlan(
  bizId: number,
  ticketId: string,
  data: IPlanTicketOverwrite,
): Promise<{ data: null }> {
  return http.post(
    `/api/v1/woa/bizs/${bizId}/plans/resources/tickets/${ticketId}/overwrite`,
    data,
  );
},
```

### 2.2 Typings: 新增覆盖修改请求体

文件: `src/typings/resourcePlan.ts`

```ts
// 紧邻 IPlanTicket 添加
export interface IPlanTicketOverwrite {
  demand_class?: string;
  demands?: IPlanTicketDemand[];
  remark?: string;
}
```

> 不复用 `IPlanTicket` 是因为后者字段必填、含 `bk_biz_id`; overwrite 的 bizId/ticketId 在 URL 路径中, 不在 body。

### 2.3 「可修改」聚合 Hook

文件: `src/views/ticket/children/resource-plan/detail/use-modifiable.ts` (新增)

```ts
import { computed, type ComputedRef, type Ref } from 'vue';
import type { TicketByIdResult, TicketStatus } from '@/typings/resourcePlan';
import type { SubTicketItem } from '@/store/ticket/res-sub-ticket';

const OVERWRITABLE_MAIN_STATUS: ReadonlySet<TicketStatus> = new Set([
  'rejected',
  'partial_rejected',
  'failed',
  'partial_failed',
  'revoked',
]);

export default function useTicketModifiable(
  ticketDetail: Ref<TicketByIdResult | undefined>,
  subTickets: Ref<SubTicketItem[] | undefined>,
  isBusinessPage: boolean,
): ComputedRef<boolean> {
  return computed(() => {
    if (!isBusinessPage) return false;
    const detail = ticketDetail.value;
    const subs = subTickets.value;
    if (!detail || !subs) return false;

    const isCvm = detail.demands?.some(d => d.demand_class === 'CVM');
    if (!isCvm) return false;

    if (!OVERWRITABLE_MAIN_STATUS.has(detail.status_info.status)) return false;
    if (subs.some(s => s.status === 'done')) return false;

    return true;
  });
}
```

**集成方式**:
- 在 `detail/index.vue` 内调用 `useTicketModifiable(ticketDetail, subTicketsRef, isBusinessPage)`
- 子单数据已在 `SubTicketList` 内部拉取, 需要把子单列表 `ref` 透出, 或在 detail 这一层独立拉一次
- **方案**: 让 `SubTicketList` 通过 `defineExpose` 暴露 `tableData` (子单列表), detail 通过 `subTicketListRef.value?.tableData` 读取

### 2.4 路由

**Symbol 常量** (`src/constants/menu-symbol.ts` 紧邻 `MENU_BUSINESS_RESOURCE_PLAN_CVM` 添加):

```ts
export const MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY = 'menu_business_resource_plan_cvm_modify';
```

> 项目中 menu-symbol 实际是字符串常量 (非 `Symbol()`), 沿用现有约定。

**路由配置** (`src/router/module/business.ts` `BizResourcePlanAdd` 路由后紧邻添加):

```ts
{
  path: '/business/resource-plan/cvm/modify',
  name: MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY,
  component: () => import('@/views/business/resource-plan/modify'),
  meta: {
    activeKey: 'bizResourcePlan',
  },
},
```

跳转 URL 示例: `/business/resource-plan/cvm/modify?id={ticket_id}&bizs={bk_biz_id}`

所有跳转改用 `routerAction.redirect({ name: MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY, query: {...} })`。

### 2.5 修改页主组件

文件: `src/views/business/resource-plan/modify/index.tsx`

差异点对比 `add/index.tsx`:

```tsx
// 关键骨架 (仅列出差异)
setup() {
  const route = useRoute();
  const { getBizsId } = useWhereAmI();
  const resourcePlanStore = useResourcePlanStore();

  const ticketId = computed(() => route.query.id as string);
  const isLoading = ref(true);
  const planTicket = ref<IPlanTicket>({
    bk_biz_id: getBizsId(),
    demand_class: 'CVM',
    remark: '',
    demands: [],
  });

  // 拉详情并回填
  const loadTicket = async () => {
    isLoading.value = true;
    try {
      const { data } = await resourcePlanStore.getBizResourcesTicketsById(
        getBizsId(),
        ticketId.value,
      );
      planTicket.value = mapTicketDetailToPlanTicket(data, getBizsId());
    } finally {
      isLoading.value = false;
    }
  };
  onBeforeMount(loadTicket);

  // 其余 (basicRef / listRef / memoRef / Add sideslider 等) 与 add 完全一致
  return () => (
    <bk-loading loading={isLoading.value}>
      <Header />  {/* 文案/返回不同 */}
      <section class={cssModule.home}>
        <Basic v-model={planTicket.value} ref={basicRef} />
        <List
          class={cssModule['mt-16']}
          ref={listRef}
          v-model={planTicket.value}
          onShow-add={handleShowAdd}
          onShow-modify={handleShowModify}
        />
        <Memo class={cssModule['mt-16']} ref={memoRef} v-model={planTicket.value} />
        <Button class={cssModule['mt-16']} v-model={planTicket.value} ticketId={ticketId.value} />
        <Add ... />
      </section>
    </bk-loading>
  );
}
```

#### 字段映射函数 `mapTicketDetailToPlanTicket`

文件: `src/views/business/resource-plan/modify/utils.ts` (新增)

```ts
import type { TicketByIdResult, IPlanTicket, IPlanTicketDemand } from '@/typings/resourcePlan';

export function mapTicketDetailToPlanTicket(
  detail: TicketByIdResult,
  bizId: number,
): IPlanTicket {
  return {
    bk_biz_id: bizId,
    demand_class: detail.demands?.[0]?.demand_class || 'CVM',
    remark: detail.base_info?.remark || '',
    demands: (detail.demands || []).map(d => mapDemand(d)),
  };
}

function mapDemand(d: TicketByIdResult['demands'][number]): IPlanTicketDemand {
  const u = d.updated_info || ({} as any);
  return {
    obs_project: u.obs_project || '',
    expect_time: u.expect_time || '',
    region_id: u.region_id || '',
    region_name: u.region_name || '',
    zone_id: u.zone_id || '',
    zone_name: u.zone_name || '',
    demand_source: '指标变化',
    demand_class: d.demand_class || 'CVM',
    remark: u.remark || '',
    demand_res_types: u.demand_res_types || ['CVM', 'CBS'],
    demand_res_type: '',
    cvm: u.cvm
      ? { ...u.cvm, os: u.cvm.os || '' }
      : undefined,
    cbs: u.cbs as any,
    adjustType: undefined as any,
    demand_id: '',
  } as IPlanTicketDemand;
}
```

> **注意**: 详情接口 `updated_info` 的 `region_name` / `zone_name` / `remark` / `cvm.os` 均已下发, typings (`TicketDemands`) 已补完, 直接透传, 不再依赖空值兜底 + 表单反查。剩余仅 `demand_source` / `demand_res_type` / `disk_num` / `disk_per_size` 等纯前端字段保留默认值。

### 2.6 Header (差异)

文件: `src/views/business/resource-plan/modify/header/index.tsx`

```tsx
import routerAction from '@/router/utils/action';
import { MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS } from '@/constants/menu-symbol';

// 与 add/header 仅 2 处差异:
// 1. 文案: t('新增资源预测') → t('修改资源预测')
// 2. handleClick: 跳回原详情页
const handleClick = () => {
  routerAction.redirect({
    name: MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS,
    query: { ...route.query },
  });
};
```

> 新代码全部按规范使用 `routerAction`, 不沿用 add 模块的 `router.push` 写法。

### 2.7 Button (差异)

文件: `src/views/business/resource-plan/modify/button/index.tsx`

```tsx
props: {
  modelValue: Object as PropType<IPlanTicket>,
  ticketId: String,
},
setup(props) {
  // 与 add/button 几乎一致, 区别:
  // 1. 调 overwriteBizPlan 而非 createBizPlan
  // 2. 提交后跳回原详情页 (用 props.ticketId), 不依赖响应 id
  // 3. 取消按钮跳回原详情页, 不是列表

  const redirectToDetail = () => {
    routerAction.redirect({
      name: MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS,
      query: {
        id: props.ticketId,
        [GLOBAL_BIZS_KEY]: props.modelValue.bk_biz_id,
      },
    });
  };

  const handleClick = async () => {
    try {
      isLoading.value = true;
      await validate();
      await resourcePlanStore.overwriteBizPlan(
        props.modelValue.bk_biz_id,
        props.ticketId,
        {
          demand_class: props.modelValue.demand_class,
          demands: props.modelValue.demands,
          remark: props.modelValue.remark,
        },
      );
      redirectToDetail();
    } catch (error: any) {
      Message({ message: error.message || error, theme: 'error' });
    } finally {
      isLoading.value = false;
    }
  };

  const handleCancel = () => {
    redirectToDetail();
  };
}
```

### 2.8 详情页 detail/index.vue (集成入口)

修改点:

1. **import** 新建的 `useTicketModifiable` hook、`MENU_BUSINESS_TICKET_RESOURCE_PLAN_MODIFY` 等
2. **取子单列表**: 通过 `subTicketListRef.value?.tableData` (需要 sub-ticket-list.vue defineExpose tableData)
3. **创建 `isModifiable` computed**:
   ```ts
   const isModifiable = useTicketModifiable(
     ticketDetail,
     computed(() => subTicketListRef.value?.tableData),
     isBusinessPage,
   );
   ```
4. **透传给 Approval 和 SubTicketList**:
   - `<Approval :is-modifiable="isModifiable" :ticket-id="route.query.id" :biz-id="getBizsId()" ... />`
   - `<SubTicketList :is-modifiable="isModifiable" :ticket-id="route.query.id" :biz-id="getBizsId()" ... />`

### 2.9 Approval 组件加按钮

文件: `src/components/resource-plan/applications/detail/approval/index.tsx`

新增 props:
```ts
isModifiable: { type: Boolean, default: false },
ticketId: String,
bizId: Number,
```

模板片段:
```tsx
<Panel class={cssModule.home}>
  <span class={cssModule.status}>
    {renderIcon()}
    <span>{props.statusInfo?.status_name}</span>
    {renderAuditStatus()}
    {props.errorMessage && (...)}
  </span>
  {props.isModifiable && (
    <bk-button class={cssModule['modify-btn']} onClick={handleModify}>
      {t('修改需求')}
    </bk-button>
  )}
</Panel>
```

`cssModule.status` 改为 `display: flex; align-items: center; justify-content: space-between` (如已具备则只补 `flex: 1`); 按钮文案 i18n `t('修改需求')`。

```ts
import routerAction from '@/router/utils/action';
import { MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY } from '@/constants/menu-symbol';

const handleModify = () => {
  routerAction.redirect({
    name: MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY,
    query: {
      id: props.ticketId,
      [GLOBAL_BIZS_KEY]: props.bizId,
    },
  });
};
```

### 2.10 SubTicketList 加按钮

文件: `src/views/ticket/children/resource-plan/sub-ticket/sub-ticket-list.vue`

新增 props:
```ts
isModifiable: { type: Boolean, default: false },
ticketId: String,
bizId: Number,
```

`#title-extra` 末尾追加:
```vue
<bk-button
  v-if="isModifiable"
  style="margin-left: 21px"
  @click="handleModify"
>
  {{ t('修改需求') }}
</bk-button>
```

`defineExpose` 暴露 `tableData` 给父组件:
```ts
defineExpose({ getData, tableData });
```

跳转方法与 §2.9 一致。

### 2.11 截止期限制 - 前端常量

文件: `src/views/business/resource-plan/constants.ts` (新增)

```ts
// 非本年预测起始日 (YYYY-MM-DD); 期望到货日期 >= 此日期即视为「非本年预测」
// 空字符串 '' 视为不做区间限制 (语义与 report_deadline 接口空值一致)
// 后续年度切换或需要解除限制时, 直接修改本常量
export const NON_CURRENT_YEAR_START_DATE = '2027-01-01';
```

> 不放 `common/constant.ts` 是因为该常量与资源预测业务强绑定, 就近放置便于维护。

### 2.12 截止期限制 - Store action

文件: `src/store/resourcePlan.ts` (紧邻 `overwriteBizPlan` 添加)

```ts
getReportDeadline(): Promise<{ data: { deadline: string } }> {
  return http.get('/api/v1/woa/plans/resources/tickets/report_deadline');
},
```

### 2.13 截止期限制 - 复用 Hook

文件: `src/views/business/resource-plan/use-deadline-restrict.ts` (新增)

```ts
import { ref, computed, onMounted, type Ref, type ComputedRef } from 'vue';
import { useResourcePlanStore } from '@/store';
import { NON_CURRENT_YEAR_START_DATE } from './constants';

// 今天 (YYYY-MM-DD), 不引入时区
const today = (): string => {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
};

interface UseDeadlineRestrictReturn {
  deadline: Ref<string>;
  isInReviewPhase: ComputedRef<boolean>;
  // 添加页 picker disabledDate 钩子用; date 为 Date 对象, 内部转 YYYY-MM-DD 字符串比较
  isDateDisabled: (date: Date) => boolean;
  // 列表行 / 单据级别判定: 给定 expect_time (YYYY-MM-DD) 是否应禁用操作
  shouldDisableRow: (expectTime: string) => boolean;
}

// 截止期限制聚合 Hook; 内部自动拉取 deadline (仅一次), 暴露判定函数
export default function useDeadlineRestrict(): UseDeadlineRestrictReturn {
  const store = useResourcePlanStore();
  const deadline = ref('');
  const startDate = NON_CURRENT_YEAR_START_DATE;

  onMounted(async () => {
    try {
      const { data } = await store.getReportDeadline();
      deadline.value = data?.deadline || '';
    } catch (err) {
      // 失败兜底: 按申报期处理, 不弹 toast
      console.warn('[resource-plan] getReportDeadline failed', err);
      deadline.value = '';
    }
  });

  const isInReviewPhase = computed(() => {
    if (!deadline.value) return false;
    if (!startDate) return false; // 起始日常量为空 → 不限制
    return today() >= deadline.value;
  });

  const formatDate = (d: Date): string => {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  };

  const isDateDisabled = (date: Date): boolean => {
    if (!isInReviewPhase.value) return false;
    return formatDate(date) >= startDate;
  };

  const shouldDisableRow = (expectTime: string): boolean => {
    if (!isInReviewPhase.value) return false;
    if (!expectTime) return false;
    return expectTime >= startDate;
  };

  return { deadline, isInReviewPhase, isDateDisabled, shouldDisableRow };
}
```

**关键决策**:
- Hook 内部自管 `onMounted` 拉取, 调用方无需关心时机
- `today()` 客户端本地时区 (与后端 `deadline` 配置时通常按本地约定一致)
- 全部用 `YYYY-MM-DD` 字符串字典序比较, 不引入 dayjs / moment
- 「任一为空 → 不限制」体现在 `isInReviewPhase` 中

### 2.14 添加页 - basic/index.tsx 改动

文件: `src/components/resource-plan/add/basic/index.tsx`

```tsx
import useDeadlineRestrict from '@/views/business/resource-plan/use-deadline-restrict';

setup() {
  // ...现有逻辑
  const { isInReviewPhase, isDateDisabled } = useDeadlineRestrict();

  // 非本年起始日的"年份"(用于提示文案动态拼接)
  const nonCurrentYear = computed(() => {
    const y = NON_CURRENT_YEAR_START_DATE.slice(0, 4);
    return y || '';
  });

  return () => (
    <bk-form-item label={t('期望到货日期')} property="expect_time" required>
      <bk-date-picker
        v-model={props.modelValue.expect_time}
        type="date"
        disabledDate={isDateDisabled}  // ← 关键新增
      />
      {isInReviewPhase.value && (
        <div class={cssModule['review-phase-tip']}>
          {t('预算评审期间, 不允许提交 {year} 及之后的预测; 如需调整已有单据, 请使用单据详情页的「修改需求」入口', { year: nonCurrentYear.value })}
        </div>
      )}
    </bk-form-item>
  );
}
```

样式: `.review-phase-tip { font-size: 12px; color: #979ba5; margin-top: 4px; line-height: 16px; }` (使用项目既有次要文字色)

> **注意**: 修改页的 basic 是 copy 自 add 的, 上述改动若直接改 `components/resource-plan/add/basic/index.tsx` 会影响修改页。需要确认修改页 basic 是否依赖此公共组件:
> - 若**修改页 basic 是 copy 出来的独立目录** → 公共组件改动只影响添加页, 符合 prd §3.4「修改页不限制」
> - 若**修改页 basic 仍引用公共组件** → 需在公共组件内加 `scene: 'add' | 'modify'` prop, modify 模式跳过 `useDeadlineRestrict()`; 默认 `add`
>
> 实施时按修改页 (§2.5) 实际结构确认: 当前 §2.5 计划 copy 整个 add 目录到 modify 目录, 那么公共组件 `components/resource-plan/add/basic` 仍是 add 专属, 改动**只影响添加页** ✓

### 2.15 列表组件 - table/index.tsx 改动

文件: `src/components/resource-plan/resource-manage/list/table/index.tsx`

业务侧 (`isBiz=true`) 与服务侧 (`isBiz=false`) 复用同一组件, 通过同一 Hook 统一控制:

```tsx
import useDeadlineRestrict from '@/views/business/resource-plan/use-deadline-restrict';

setup(props) {
  // ...现有逻辑
  const { isInReviewPhase, shouldDisableRow } = useDeadlineRestrict();

  // 1. 行内操作列: 在现有 disabled 判断基础上加截止期判定
  const isOpDisabled = (row: IListResourcesDemandsItem): boolean => {
    // 既有 disabled 逻辑 (e.g. 状态、权限) ...
    if (shouldDisableRow(row.expect_time)) return true;
    return existingDisabledLogic(row);
  };

  // 2. 行选择是否允许 (允许勾选, 不在 checkbox 层拦截; 由批量按钮兜底)
  // 既有 isRowSelectable 不需要改动

  // 3. 批量操作按钮: 若选中行含非本年行, 禁用
  const batchOpDisabled = computed(() => {
    if (selections.value.length === 0) return true; // 既有
    if (selections.value.some(row => shouldDisableRow(row.expect_time))) return true;
    return false;
  });

  const batchOpDisabledTip = computed(() => {
    if (!isInReviewPhase.value) return '';
    if (selections.value.some(row => shouldDisableRow(row.expect_time))) {
      const year = NON_CURRENT_YEAR_START_DATE.slice(0, 4);
      return t('所选包含 {year} 及之后的非本年预测, 评审期内已锁定操作; 请取消勾选后再批量操作', { year });
    }
    return '';
  });

  // 4. 行内操作按钮 tooltip
  const rowOpDisabledTip = (row: IListResourcesDemandsItem): string => {
    if (shouldDisableRow(row.expect_time)) {
      const year = NON_CURRENT_YEAR_START_DATE.slice(0, 4);
      return t('预算评审期间, 期望到货日期为 {year} 及之后的单据已锁定操作', { year });
    }
    return '';
  };

  return () => (
    // 批量操作按钮区
    <bk-button
      disabled={batchOpDisabled.value}
      v-bk-tooltips={{ content: batchOpDisabledTip.value, disabled: !batchOpDisabledTip.value }}
      onClick={...}>
      {t('批量撤销')}
    </bk-button>

    // 行内操作 (在表格列定义中)
    <bk-table-column label="操作">
      {{
        default: ({ row }: { row: IListResourcesDemandsItem }) => (
          <bk-button
            text
            disabled={isOpDisabled(row)}
            v-bk-tooltips={{ content: rowOpDisabledTip(row), disabled: !rowOpDisabledTip(row) }}
            onClick={...}>
            {t('编辑')}
          </bk-button>
          // 其他操作按钮同理
        ),
      }}
    </bk-table-column>
  );
}
```

**实施时核对项**:
- 该列表组件实际已存在多个操作 (调整 / 取消 / 部分延期 / 批量延期 / 批量撤销 等), 实施时需**逐一**给所有行内操作和批量操作加上禁用 + tooltip
- 既有 `isOpDisabled` / `batchOpDisabled` 若已存在, 在原 `||` / `&&` 链中插入截止期判定即可, 不破坏既有逻辑
- `expect_time` 字段类型为 `string`, 格式 `YYYY-MM-DD`, 与 `NON_CURRENT_YEAR_START_DATE` 字典序对齐

## 3. 边界处理

### 3.1 修改入口相关

| 场景 | 处理 |
|---|---|
| 修改页直接访问无 `id` query | router 守卫无, 进入后 `getBizResourcesTicketsById` 报错, 走全局错误提示, 用户手动返回 |
| 详情接口失败 | `isLoading` 关闭, 整页空表单 (用户感知错误, 可手动返回); 暂不做专门错误页 |
| 子单数据未就绪 | `isModifiable` 自动为 false, 按钮隐藏 |
| 提交时后端拦截 (例如已有 done 子单) | 走 `Message` toast 显示 `error.message` |
| 用户在修改页时单据被推进 (修改页无轮询) | 不处理, 提交时由后端拦截 |

### 3.2 截止期限制相关

| 场景 | 处理 |
|---|---|
| `report_deadline` 接口失败 / 超时 | `deadline = ''`, 按申报期处理, 控制台 warn, 不弹 toast |
| `deadline` 返回非法格式 (非 `YYYY-MM-DD`) | 字典序比较退化, 视为申报期 (兜底); 不主动校验格式 |
| 起始日常量 `NON_CURRENT_YEAR_START_DATE = ''` | `isInReviewPhase = false`, 任何情况不限制 |
| 用户在添加页停留跨日 (今天=deadline 前一天, 明天=deadline 当天) | 不处理 (不重新拉取), 用户重新进入页面时按新规则 |
| 列表中有 `expect_time = ''` 的脏数据 | `shouldDisableRow` 返回 false, 不禁用 (兜底, 避免误拦截) |
| 修改页表单意外引用了截止期 Hook | 通过 §2.5 修改页 copy 独立 basic 目录避免; 公共组件不感知修改场景 |

## 4. 不做的事 (明确边界)

- 不改 add 页的代码 (修改页是 copy 出来的独立目录)
- 不重构老路由 (modify 路由仍写在 `router/module/business.ts`, 与 add 同位置; 但新增代码的**跳转**全部用 `routerAction`)
- 不为修改场景做差异对比 / 修改历史展示
- 不增加二次确认弹窗
- 不处理修改页关闭浏览器时的草稿保存
- 不为修改页加权限守卫 (沿用现有体系: 能进详情即能修改)
- 不做 `report_deadline` 全局缓存 / Pinia 持久化 (按需拉取, 跟随页面挂载)
- 不在修改页 / 修改入口处应用截止期限制
- 不做「自动过滤非本年行后允许批量操作」之类的复杂列表批量行为

## 5. 实施顺序

### 5.1 修改入口

1. typings + store (`IPlanTicketOverwrite` + `overwriteBizPlan`)
2. hook `useTicketModifiable`
3. 路由 (Symbol 常量 + business.ts)
4. modify 目录 (copy + 改 5 处)
5. detail/index.vue + Approval + SubTicketList 集成入口
6. 自测: 进入业务下任一已驳回/失败单据详情, 观察按钮显示, 点击进入修改页, 改后提交看是否跳回详情

### 5.2 截止期限制

7. 常量 `constants.ts` + store `getReportDeadline`
8. Hook `use-deadline-restrict.ts`
9. add/basic 集成 (picker disabledDate + 评审期提示)
10. table/index.tsx 集成 (行内 + 批量 disabled + tooltip)
11. `hcmfe_lint --fix`
12. 自测:
    - 后端配 `deadline = ''` → 添加页无禁选, 列表无禁用
    - 后端配 `deadline = 未来日期` (今天 < deadline) → 同上 (申报期)
    - 后端配 `deadline = 过去日期` (今天 >= deadline) → 添加页 picker 禁选 ≥ 2027-01-01, 评审期提示; 列表「非本年行」操作禁用
    - 接口 mock 返回 500 → 不弹 toast, 按申报期兜底
    - 改前端常量 `NON_CURRENT_YEAR_START_DATE = ''` → 即使 deadline 已到, 也无限制

## 6. 待联调时核对

- 详情接口 `updated_info` 是否真返回 `demand_source` / `cvm.os` (本次 typings 中不可见)
- region/zone name 反查时机, 与 add 页一致即可
- 后端 overwrite 接口实际返回的错误码 / 错误文案
- `report_deadline` 接口路径与实际部署版本一致 (v9.9.9+)
- 列表组件的「业务侧 / 服务侧」实际复用关系再确认一次 (本文档假设 `components/resource-plan/resource-manage/list/table/index.tsx` 为共用)
- `expect_time` 字段在列表行数据中的实际字段名 (typings `IListResourcesDemandsItem` 内核对)

## 7. 阻塞问题

无 - 等待用户确认本方案可实施, 即可写代码。
