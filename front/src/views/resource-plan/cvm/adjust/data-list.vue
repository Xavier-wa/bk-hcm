<script setup lang="ts">
import { computed, nextTick, ref, onMounted, onBeforeUnmount, useTemplateRef } from 'vue';
import dayjs from 'dayjs';
import isBetween from 'dayjs/plugin/isBetween';
import isoWeek from 'dayjs/plugin/isoWeek';
import { DatePicker } from 'bkui-vue';
import { Ediatable, HeadColumn } from '@blueking/ediatable';
import BatchUpdatePopConfirm from '@/components/batch-update-popconfirm';
import { useResourcePlanStore } from '@/store';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import useDeadlineRestrict from '@/views/business/resource-plan/use-deadline-restrict';
import type { IDiskType, IRegion } from '@/typings/resourcePlan';
import RenderRow from './render-row.vue';
import { createEmptyAdjustRow, resolveDemandClass } from './adjust-payload';
import type { IAdjustRow, IAdjustValidateResult } from './typings';
import { ROLLING_SERVER_BIZ_ID } from './typings';

const tableData = defineModel<IAdjustRow[]>({ required: true });
const emit = defineEmits<{
  (e: 'scrollable-change', scrollable: boolean): void;
}>();
dayjs.extend(isBetween);
dayjs.extend(isoWeek);

const resourcePlanStore = useResourcePlanStore();
// 以 row_key 索引行实例，行卸载时清理，避免移除后残留旧实例参与校验
const rowRefs = ref<Record<string, InstanceType<typeof RenderRow>>>({});

const setRowRef = (rowKey: string, el: InstanceType<typeof RenderRow> | null) => {
  if (el) {
    rowRefs.value[rowKey] = el;
  } else {
    delete rowRefs.value[rowKey];
  }
};

/** 有历史行时预测用途只读继承；仅纯新增表可编辑，且选择时整批同步 */
const hasHistoricalRows = computed(() => tableData.value.some((row) => !row.is_new));
const demandClassEditable = computed(() => tableData.value.length > 0 && !hasHistoricalRows.value);

// 截止期限制在此统一取一份再下发：Hook 自带 onMounted 拉 deadline，放到行里会按行数重复请求
const { isDateDisabled, isInReviewPhase, nonCurrentYearLabel } = useDeadlineRestrict();
const reviewPhaseTip = computed(() =>
  isInReviewPhase.value
    ? `预算评审期间，不允许提交 ${nonCurrentYearLabel} 及之后的预测；如需调整已有单据，请使用单据详情页的「修改需求」入口`
    : '',
);

const { getBizsId } = useWhereAmI();
const showRollingServerProject = computed(() => getBizsId() === ROLLING_SERVER_BIZ_ID);

const obsProjectOptions = ref<{ value: string; label: string }[]>([]);
const regionOptions = ref<IRegion[]>([]);
const deviceClassOptions = ref<string[]>([]);
const diskTypeOptions = ref<IDiskType[]>([]);
const demandSourceOptions = ref<{ value: string; label: string }[]>([]);
const demandClassOptions = ref<{ value: string; label: string }[]>([]);
// 字典并行拉取，共用一个 loading 下发给各行的下拉，与现网各表单项的 loading 对齐
const isOptionsLoading = ref(true);

// Ediatable 未暴露根元素 ref，只能从包裹层取，用于上报表格是否已进入内部纵向滚动
const wrapperRef = useTemplateRef<HTMLElement>('wrapperRef');
let scrollObserver: ResizeObserver | null = null;

const syncScrollable = () => {
  const el = wrapperRef.value?.querySelector('.bk-ediatable');
  if (!el) return;
  emit('scrollable-change', el.scrollHeight > el.clientHeight);
};

onMounted(() => {
  const el = wrapperRef.value?.querySelector('.bk-ediatable');
  if (el) {
    scrollObserver = new ResizeObserver(syncScrollable);
    scrollObserver.observe(el);
    const table = el.querySelector('table');
    if (table) scrollObserver.observe(table);
  }
});

onBeforeUnmount(() => {
  scrollObserver?.disconnect();
  scrollObserver = null;
});

onMounted(async () => {
  isOptionsLoading.value = true;
  try {
    const [obsRes, regionRes, deviceClassRes, diskRes, sourceRes, demandClassRes] = await Promise.all([
      resourcePlanStore.getObsProjects(),
      resourcePlanStore.getRegions(),
      resourcePlanStore.getDeviceClasses(),
      resourcePlanStore.getDiskTypes(),
      resourcePlanStore.getSources(),
      resourcePlanStore.getDemandClasses(),
    ]);

    const obsList = obsRes?.data?.details || [];
    obsProjectOptions.value = (Array.isArray(obsList) ? obsList : []).map((item: any) => {
      if (typeof item === 'string') return { value: item, label: item };
      return {
        value: item.obs_project || item.name || String(item),
        label: item.obs_project || item.name || String(item),
      };
    });

    regionOptions.value = regionRes?.data?.details || [];
    deviceClassOptions.value = deviceClassRes?.data?.details || [];
    diskTypeOptions.value = diskRes?.data?.details || [];
    // 该接口返回的是纯字符串数组，与现网 <bk-option id={source} name={source}/> 的用法一致
    demandSourceOptions.value = (sourceRes?.data?.details || []).map((item: string) => ({
      value: item,
      label: item,
    }));
    demandClassOptions.value = (demandClassRes?.data?.details || []).map((item: string) => ({
      value: item,
      label: item,
    }));
  } catch {
    // 错误提示由 http 层统一弹出，这里只需保证 loading 收尾、选项保持空列表
  } finally {
    isOptionsLoading.value = false;
  }
});

const addRow = () => {
  const demandClass = resolveDemandClass(tableData.value);
  tableData.value.push(createEmptyAdjustRow({ demand_class: demandClass }));
};

const copyRow = (row: IAdjustRow) => {
  const demandClass = resolveDemandClass(tableData.value) || row.demand_class;
  tableData.value.push({
    ...row,
    demand_class: demandClass,
  });
};

/**
 * 与行内期望到货日历同一套禁选：评审期非本年 + 非本周且早于今天。
 * 表头批量改日期复用，避免批量选到行内选不了的日期。
 */
const isExpectTimeDisabled = (date: Date) => {
  if (isDateDisabled(date)) return true;
  const currentDate = dayjs(date);
  const startOfWeek = dayjs().startOf('isoWeek');
  const endOfWeek = dayjs().endOf('isoWeek');
  if (currentDate.isBetween(startOfWeek, endOfWeek, 'day', '[]')) return false;
  return currentDate.isBefore(dayjs(), 'day');
};

/** 表头批量改日期：写入全部行；短租退回日若早于新到货日则清空，避免留下必失败校验 */
const handleBatchExpectTime = (val: string | Date) => {
  if (!val) return;
  const expectTime = dayjs(val).format('YYYY-MM-DD');
  tableData.value.forEach((row) => {
    row.expect_time = expectTime;
    if (row.return_plan_time && dayjs(row.return_plan_time).isBefore(dayjs(expectTime), 'day')) {
      row.return_plan_time = '';
    }
  });
};

/** 对齐行内「13周后」：只填 PopConfirm 内的待确认值，仍要点确定才写回各行 */
const setBatchExpectTimeInThirteenWeeks = (updateValue: (v: string) => void) => {
  updateValue(dayjs().add(13, 'week').format('YYYY-MM-DD'));
};

/**
 * 移除的下限是「表里至少留一行」，不区分历史行/新增行——产品允许纯新增（历史行全移除、只提交新增行）。
 * 真正不能发生的是表格被清空：「新增一行」的入口只长在行操作列上，一行都不剩就再没有入口，
 * 用户除了刷新页面无路可走。
 * 按钮置灰与 removeRow 的实际拦截共用这一份，避免两处规则各写一遍而漂移。
 */
const canRemoveRow = computed(() => tableData.value.length > 1);

const removeRow = (index: number) => {
  if (!canRemoveRow.value) return;
  tableData.value.splice(index, 1);
};

/**
 * 表格内部纵向滚动 + sticky 表头 + 右固定操作列，出错单元格可能同时落在视口、纵向滚动区与横向滚动区之外，
 * 归位分两级：先把表格滚进视口（视口过矮时页面自身仍会滚动），再在表格内部按两个方向让出遮挡尺寸。
 */
const scrollToInvalidCell = (rowEl: HTMLElement) => {
  const scroller = wrapperRef.value?.querySelector('.bk-ediatable');
  if (!(scroller instanceof HTMLElement)) return;
  scroller.scrollIntoView({ block: 'nearest' });

  const target = rowEl.querySelector('.is-error') ?? rowEl;
  const scrollerRect = scroller.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const headHeight = scroller.querySelector('thead')?.getBoundingClientRect().height ?? 0;
  const fixedRightWidth = scroller.querySelector('th.is-right-fixed')?.getBoundingClientRect().width ?? 0;

  const overTop = targetRect.top - scrollerRect.top - headHeight;
  const overBottom = targetRect.bottom - scrollerRect.bottom;
  if (overTop < 0) {
    scroller.scrollTop += overTop;
  } else if (overBottom > 0) {
    scroller.scrollTop += overBottom;
  }

  const overLeft = targetRect.left - scrollerRect.left;
  const overRight = targetRect.right - (scrollerRect.right - fixedRightWidth);
  if (overLeft < 0) {
    scroller.scrollLeft += overLeft;
  } else if (overRight > 0) {
    scroller.scrollLeft += overRight;
  }
};

/**
 * 各列组件的 getValue() 是纯粹按当前值跑一遍 rules（与是否交互过无关），失败即 reject 并在单元格上留下错误态，
 * 所以「从头到尾没被碰过」的必填项只能靠提交前的这次整表校验兜住。
 * 逐行 catch 而不用 Promise.all 的快速失败：出错行序号要收全，Message 才能报出总数。
 */
const validate = async (): Promise<IAdjustValidateResult> => {
  // 先等挂载落定：刚新增的行还没有 ref，否则会被当成「不存在的行」静默跳过校验
  await nextTick();
  // 仅校验当前仍挂载的行
  const mountedRows = tableData.value
    .map((row, index) => ({ index, instance: rowRefs.value[row.row_key] }))
    .filter((item) => Boolean(item.instance));
  const results = await Promise.all(
    mountedRows.map((item) =>
      item.instance
        .getValue()
        .then(() => true)
        .catch(() => false),
    ),
  );

  const invalidRows = mountedRows.filter((_item, i) => !results[i]);
  if (invalidRows.length) {
    // 错误态渲染完成后再量尺寸，避免按旧布局定位
    await nextTick();
    const firstInvalidEl = invalidRows[0].instance.$el;
    if (firstInvalidEl instanceof HTMLElement) scrollToInvalidCell(firstInvalidEl);
  }

  return { valid: invalidRows.length === 0, invalidRowIndexes: invalidRows.map((item) => item.index) };
};

defineExpose({
  validate,
  addRow,
});
</script>

<template>
  <!-- Ediatable 自带横向滚动容器，这里补高度约束让它同时承担纵向滚动，表头才能吸顶 -->
  <div ref="wrapperRef" class="cvm-adjust-data-list">
    <Ediatable>
      <template #default>
        <HeadColumn required :width="200">
          期望到货时间
          <!-- 稿面 2664:50073 / 2664:47705：表头批量编辑；复用项目 BatchUpdatePopConfirm + 现有 DatePicker（非整套 F-003 新日历） -->
          <template #append>
            <BatchUpdatePopConfirm title="期望到货时间" @update-value="handleBatchExpectTime">
              <template #content="{ value, updateValue }">
                <div class="batch-expect-time-content">
                  <!--
                    type=datetime：与行内一致，为拿到面板「确定」栏以便挂 footer「13周后」。
                    append-to-body + 高 z-index：避免日历被 PopConfirm 盖住。
                  -->
                  <DatePicker
                    :model-value="value || ''"
                    type="datetime"
                    format="yyyy-MM-dd"
                    placeholder="请选择日期"
                    :clearable="false"
                    :disabled-date="isExpectTimeDisabled"
                    :append-to-body="true"
                    ext-popover-cls="batch-expect-time-picker-dropdown"
                    @change="updateValue"
                  >
                    <template #footer>
                      <div
                        class="batch-expect-time-picker-footer"
                        @mousedown.prevent.stop
                        @mouseup.prevent.stop
                        @click.stop
                      >
                        <div
                          class="batch-expect-time-thirteen-weeks"
                          @click="setBatchExpectTimeInThirteenWeeks(updateValue)"
                        >
                          13周后
                        </div>
                      </div>
                    </template>
                  </DatePicker>
                </div>
              </template>
            </BatchUpdatePopConfirm>
          </template>
        </HeadColumn>
        <HeadColumn :required="demandClassEditable" :width="120">预测用途</HeadColumn>
        <HeadColumn required :width="129">项目类型</HeadColumn>
        <HeadColumn required :width="129">城市</HeadColumn>
        <HeadColumn :required="false" :width="129">可用区</HeadColumn>
        <HeadColumn required :width="129">资源类型</HeadColumn>
        <HeadColumn required :width="128">机型类型</HeadColumn>
        <HeadColumn required :width="166">机型规格</HeadColumn>
        <HeadColumn required :width="120" memo="CVM 为所需实例数（台）；CBS 为需要的云磁盘块数（块）">
          实例数量
        </HeadColumn>
        <HeadColumn :required="false" :width="128">CPU总核数 (核)</HeadColumn>
        <HeadColumn :required="false" :width="128">内存总量 (GB)</HeadColumn>
        <HeadColumn required :width="129">云盘类型</HeadColumn>
        <HeadColumn required :width="143" memo="CVM 为每个实例的云盘容量；CBS 为每块云盘的容量">
          云盘容量 /实例 (GB)
        </HeadColumn>
        <HeadColumn :required="false" :width="118" memo="CVM 为所有实例的系统盘、数据盘总容量；CBS 为需要的云磁盘总量">
          云盘总量 (GB)
        </HeadColumn>
        <HeadColumn
          :required="false"
          :width="143"
          memo="磁盘IO吞吐需求，无特殊要求填写15；高性能云盘上限150，SSD云硬盘上限260"
        >
          单实例磁盘IO(MB/s)
        </HeadColumn>
        <HeadColumn :required="false" :width="150">短租退回日期</HeadColumn>
        <HeadColumn
          :required="false"
          :width="140"
          memo="仅调整单内新增的预测可填；已有预测的变更原因与单据绑定，不可修改"
        >
          变更原因
        </HeadColumn>
        <HeadColumn fixed="right" :required="false" :width="112" />
      </template>
      <template #data>
        <RenderRow
          v-for="(row, index) in tableData"
          :key="row.row_key"
          v-model="tableData[index]"
          :ref="(el: any) => setRowRef(row.row_key, el)"
          :index="index"
          :removeable="canRemoveRow"
          :obs-project-options="obsProjectOptions"
          :show-rolling-server-project="showRollingServerProject"
          :region-options="regionOptions"
          :device-class-options="deviceClassOptions"
          :disk-type-options="diskTypeOptions"
          :demand-source-options="demandSourceOptions"
          :demand-class-options="demandClassOptions"
          :demand-class-editable="demandClassEditable"
          :options-loading="isOptionsLoading"
          :is-date-disabled="isDateDisabled"
          :review-phase-tip="reviewPhaseTip"
          @add="addRow"
          @row-copy="copyRow"
          @remove="removeRow"
        />
      </template>
    </Ediatable>
  </div>
</template>

<style scoped lang="scss">
.cvm-adjust-data-list {
  display: flex;

  // 不撑高、可压缩：行少时保持自然高度，行多时被父级压到剩余空间内滚动
  flex: 0 1 auto;
  flex-direction: column;
  width: 100%;
  min-height: 0;

  // .bk-ediatable 本身已是滚动容器（overflow-x: scroll 使 overflow-y 计算为 auto），补高度约束即可纵向滚动
  :deep(.bk-ediatable) {
    flex: 0 1 auto;
    min-height: 0;
  }

  // border-collapse 下 sticky 表头的边框会随内容滚走，改用 inset 阴影补线
  :deep(.bk-ediatable table th) {
    position: sticky;
    top: 0;
    z-index: 2;
    box-shadow: inset 0 1px 0 #dcdee5, inset 0 -1px 0 #dcdee5;
  }

  // 右固定表头需同时吸顶与吸右，且压在普通表头之上
  :deep(.bk-ediatable table th.is-right-fixed) {
    z-index: 3;
  }
}

// PopConfirm 内容 teleport 到 body，仍挂在本组件树内，scoped 可命中
.batch-expect-time-content {
  // DatePicker 默认非 100% 宽，相对 280 弹层右侧会空一截
  :deep(.bk-date-picker) {
    display: block;
    width: 100%;
  }

  :deep(.bk-date-picker-rel),
  :deep(.bk-input-wrapper),
  :deep(.bk-input) {
    width: 100%;
  }
}
</style>

<!-- 日历面板 append-to-body，scoped 选不中，靠 ext-popover-cls 限定 -->
<style lang="scss">
.batch-expect-time-picker-dropdown {
  // PopConfirm 弹层 z-index 通常高于 DatePicker 默认 transfer，不抬会被盖住
  z-index: 9999 !important;

  .bk-picker-confirm .bk-picker-confirm-time {
    display: none;
  }

  .bk-date-picker-footer-wrapper {
    position: relative;
  }

  .batch-expect-time-picker-footer {
    box-sizing: border-box;
    width: 0;
    min-width: 100%;
  }

  .batch-expect-time-thirteen-weeks {
    position: absolute;
    top: -42px;
    left: 50%;
    z-index: 2;
    height: 42px;
    padding: 0;
    color: #5594fa;
    line-height: 42px;
    white-space: nowrap;
    cursor: pointer;
    transform: translateX(-50%);
  }
}
</style>
