<script setup lang="ts">
import { computed, ref, watch, onMounted, onBeforeUnmount, useTemplateRef } from 'vue';
import { useRoute, onBeforeRouteLeave } from 'vue-router';
import { Message, InfoBox } from 'bkui-vue';
import { ArrowsRight } from 'bkui-vue/lib/icon';
import routerAction from '@/router/utils/action';
import { MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS } from '@/constants/menu-symbol';
import { useCvmAdjustStore } from '@/store/resource-plan/cvm-adjust';
import { isDateInRange } from '@/utils/plan';
import DataList from './data-list.vue';
import {
  buildAdjustPayload,
  collectExpectTimesToValidate,
  demandDetailToAdjustRow,
  hasMixedDemandClass,
  isRowUnchanged,
} from './adjust-payload';
import type { IAdjustRow } from './typings';

const route = useRoute();
const adjustStore = useCvmAdjustStore();
const dataListRef = useTemplateRef<InstanceType<typeof DataList>>('dataListRef');

const tableData = ref<IAdjustRow[]>([]);
const initialSnapshot = ref('');
const removeDialogVisible = ref(false);
const submitting = ref(false);
const loadFailed = ref(false);
/** 取消已二次确认时，跳过路由离开守卫，避免连弹两次 InfoBox */
const skipLeaveConfirm = ref(false);

// 底部区块跟随内容流，内容溢出可视区时才 sticky 悬浮
// 悬浮态（操作区才需要底色与上边线）有两个来源：
// 1. 表格被压缩到内部滚动，底部区块此时悬于滚动内容之上
// 2. 视口过矮导致页面自身滚动，哨兵不可见
const bottomSentinelRef = useTemplateRef<HTMLElement>('bottomSentinelRef');
const isBottomStuck = ref(false);
const isTableScrollable = ref(false);
const isFooterFloating = computed(() => isTableScrollable.value || isBottomStuck.value);
let bottomObserver: IntersectionObserver | null = null;

const unchangedCount = computed(() => tableData.value.filter((row) => isRowUnchanged(row)).length);

/**
 * 「移除未修改」的不可用原因，空串表示可点击。
 * 允许把历史行全部移除只提交新增行，唯一不允许的是移除后表格为空——
 * 「新增一行」的入口只长在行操作列上，表格清空后用户就再没有入口了。
 */
const removeUnchangedDisabledReason = computed(() => {
  if (unchangedCount.value === 0) return '不存在未修改数据';
  if (unchangedCount.value === tableData.value.length) return '移除后表格将为空，请先新增一行';
  return '';
});

const canRemoveUnchanged = computed(() => removeUnchangedDisabledReason.value === '');

const hasDirtyChanges = computed(() => JSON.stringify(tableData.value) !== initialSnapshot.value);

/**
 * 提交不可用原因，空串表示可提交。
 * 只收「数据层面不允许提交」的情形：填写项校验不放进来，改由点击时整表校验拦截——
 * 表格里长期置灰只能给一句笼统 tooltip，用户无从知道是哪一行哪一列出错。
 */
const submitDisabledReason = computed(() => {
  if (loadFailed.value) return '预测数据加载失败或预测用途不一致，请返回列表按同一用途重新选择';
  if (unchangedCount.value > 0) return '存在未修改数据，请先移除未修改后再提交';
  // 纯新增各行可分别选择；不一致时禁用提交（不做整批同步）
  if (hasMixedDemandClass(tableData.value)) return '预测用途须保持一致，请统一各行后再提交';
  if (buildAdjustPayload(tableData.value).adjusts.length === 0) return '没有可提交的变更';
  return '';
});

const canSubmit = computed(() => submitDisabledReason.value === '');

const sumField = (rows: IAdjustRow[], field: 'remained_cpu_core' | 'remained_memory' | 'remained_disk_size') =>
  rows.reduce((acc, row) => acc + (Number(row[field]) || 0), 0);

/** 预览数字千分位，形如 15,246；CPU 可能是小数，保留原小数部分 */
const formatPreviewNumber = (value: number) => value.toLocaleString('en-US');

const previewItems = computed(() => {
  const beforeRows = tableData.value.filter((row) => !row.is_new && row.baseline).map((row) => row.baseline!);
  // 预览对比：调整前 = 当前仍在表中的原始行基线合计；调整后 = 当前表合计
  // 新增行只计入调整后
  const afterCpu = sumField(tableData.value, 'remained_cpu_core');
  const afterMemory = sumField(tableData.value, 'remained_memory');
  const afterDisk = sumField(tableData.value, 'remained_disk_size');
  const beforeCpu = sumField(beforeRows as IAdjustRow[], 'remained_cpu_core');
  const beforeMemory = sumField(beforeRows as IAdjustRow[], 'remained_memory');
  const beforeDisk = sumField(beforeRows as IAdjustRow[], 'remained_disk_size');

  return [
    {
      label: 'CPU调整数 (核)：',
      before: formatPreviewNumber(beforeCpu),
      after: formatPreviewNumber(afterCpu),
      changed: beforeCpu !== afterCpu,
    },
    {
      label: '内存调整数 (GB)：',
      before: formatPreviewNumber(beforeMemory),
      after: formatPreviewNumber(afterMemory),
      changed: beforeMemory !== afterMemory,
    },
    {
      label: '云盘调整数 (GB)：',
      before: formatPreviewNumber(beforeDisk),
      after: formatPreviewNumber(afterDisk),
      changed: beforeDisk !== afterDisk,
    },
  ];
});

/** 设计稿的浅黄底是「有变更」的强调态，无变更时不上色 */
const hasPreviewChange = computed(() => previewItems.value.some((item) => item.changed));

const loadData = async () => {
  const planIds = ((route.query.planIds as string) || '')
    .split(',')
    .map((id) => id.trim())
    .filter(Boolean);
  const start = (route.query.start as string) || '';
  const end = (route.query.end as string) || '';

  if (!planIds.length) {
    loadFailed.value = false;
    tableData.value = [];
    initialSnapshot.value = JSON.stringify(tableData.value);
    return;
  }

  try {
    const details = await adjustStore.listDemands({
      demand_ids: planIds,
      expect_time_range: { start, end },
    });
    const rows = (details || []).map((item: Record<string, any>) => demandDetailToAdjustRow(item));
    // 列表入口已拦混选；手动改 planIds 仍可能混入不同用途，进页再兜底一次
    if (hasMixedDemandClass(rows)) {
      loadFailed.value = true;
      tableData.value = [];
      initialSnapshot.value = JSON.stringify(tableData.value);
      Message({
        theme: 'warning',
        message: '所选预测包含不同预测用途，请返回列表按同一预测用途重新选择后再调整',
      });
      return;
    }
    // 历史行 demand_class 原样回填
    tableData.value = rows;
    initialSnapshot.value = JSON.stringify(tableData.value);
    loadFailed.value = false;
  } catch {
    loadFailed.value = true;
    tableData.value = [];
    initialSnapshot.value = JSON.stringify(tableData.value);
    Message({ theme: 'error', message: '预测数据加载失败，请刷新后重试' });
  }
};

watch(
  () => route.query,
  () => {
    loadData();
  },
  { immediate: true },
);

onMounted(() => {
  window.addEventListener('beforeunload', handleBeforeUnload);
  if (bottomSentinelRef.value) {
    bottomObserver = new IntersectionObserver(([entry]) => {
      isBottomStuck.value = !entry.isIntersecting;
    });
    bottomObserver.observe(bottomSentinelRef.value);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload);
  bottomObserver?.disconnect();
  bottomObserver = null;
});

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!hasDirtyChanges.value) return;
  event.preventDefault();
  event.returnValue = '关闭提示';
}

onBeforeRouteLeave((to, _from, next) => {
  if (skipLeaveConfirm.value || [MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS].includes(to.name as string)) {
    next();
    return;
  }
  if (!hasDirtyChanges.value) {
    next();
    return;
  }
  InfoBox({
    title: '确定离开当前页面?',
    subTitle: '离开会导致编辑的内容丢失',
    confirmText: '离开',
    cancelText: '取消',
    onConfirm: () => next(),
    onClose: () => next(false),
  });
});

const handleRemoveUnchanged = () => {
  removeDialogVisible.value = true;
};

const confirmRemoveUnchanged = () => {
  tableData.value = tableData.value.filter((row) => !isRowUnchanged(row));
  removeDialogVisible.value = false;
};

/** 校验新增行与到货时间变更行是否落在后端可申领周范围内 */
const validateExpectTimes = async () => {
  const expectTimes = collectExpectTimesToValidate(tableData.value);
  if (!expectTimes.length) return true;

  let ranges: { expectTime: string; range?: { start: string; end: string } }[];
  try {
    ranges = await Promise.all(
      expectTimes.map(async (expectTime) => ({
        expectTime,
        range: (await adjustStore.getAvailableTime(expectTime))?.date_range_in_week,
      })),
    );
  } catch {
    Message({ theme: 'error', message: '期望到货时间校验失败，请稍后重试' });
    return false;
  }

  const invalid = ranges.find(({ expectTime, range }) => !range || !isDateInRange(expectTime, range));
  if (invalid) {
    const rangeTip = invalid.range ? `，可选范围为 ${invalid.range.start} ~ ${invalid.range.end}` : '';
    Message({ theme: 'error', message: `期望到货时间 ${invalid.expectTime} 不在可选范围内${rangeTip}` });
    return false;
  }
  return true;
};

/** 出错行已由 validate() 滚动定位并标红，提示只负责说明范围 */
const buildValidateFailMessage = (invalidRowIndexes: number[]) => {
  if (!invalidRowIndexes.length) return '请完善表格必填项后再提交';
  const firstRowNo = invalidRowIndexes[0] + 1;
  if (invalidRowIndexes.length === 1) return `第 ${firstRowNo} 行填写项未通过校验，请检查标红的单元格`;
  return `共 ${invalidRowIndexes.length} 行填写项未通过校验，已定位到第 ${firstRowNo} 行，请检查标红的单元格`;
};

const handleSubmit = async () => {
  if (loadFailed.value) {
    Message({ theme: 'error', message: '预测数据加载失败或预测用途不一致，请返回列表按同一用途重新选择' });
    return;
  }

  const validateResult = await dataListRef.value?.validate();
  if (!validateResult?.valid) {
    Message({ theme: 'error', message: buildValidateFailMessage(validateResult?.invalidRowIndexes ?? []) });
    return;
  }

  const payload = buildAdjustPayload(tableData.value);
  if (!payload.adjusts.length) {
    Message({ theme: 'warning', message: '没有可提交的变更' });
    return;
  }
  if (payload.adjusts.length > 100) {
    Message({ theme: 'warning', message: '单次最多提交 100 条调整，请拆分后再提交' });
    return;
  }

  submitting.value = true;
  try {
    if (!(await validateExpectTimes())) return;
    const data = await adjustStore.submitAdjust(payload);
    if (!data?.id) return;
    Message({ theme: 'success', message: '提交成功' });
    initialSnapshot.value = JSON.stringify(tableData.value);
    routerAction.redirect({
      name: MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS,
      query: { id: data.id },
    });
  } catch {
    // 保留现场；错误由全局 http 拦截提示
  } finally {
    submitting.value = false;
  }
};

const handleCancel = () => {
  if (!hasDirtyChanges.value) {
    routerAction.back();
    return;
  }
  InfoBox({
    title: '确定取消调整?',
    subTitle: '取消后未保存的修改将丢失',
    onConfirm: () => {
      skipLeaveConfirm.value = true;
      routerAction.back();
    },
  });
};
</script>

<template>
  <div class="cvm-adjust-page">
    <section
      class="cvm-adjust-main"
      :class="{ 'is-loading': adjustStore.listLoading }"
      v-bkloading="{ loading: adjustStore.listLoading }"
    >
      <DataList ref="dataListRef" v-model="tableData" @scrollable-change="isTableScrollable = $event" />
    </section>

    <div class="cvm-adjust-bottom">
      <section class="cvm-adjust-preview">
        <h3 class="cvm-adjust-preview-title">调整预览</h3>
        <div class="cvm-adjust-preview-body" :class="{ 'is-changed': hasPreviewChange }">
          <p v-for="item in previewItems" :key="item.label" class="cvm-adjust-preview-item">
            <span class="cvm-adjust-preview-label">{{ item.label }}</span>
            <template v-if="item.changed">
              <span class="cvm-adjust-preview-before">{{ item.before }}</span>
              <ArrowsRight class="cvm-adjust-preview-arrow" />
              <span class="cvm-adjust-preview-after">{{ item.after }}</span>
            </template>
            <span v-else class="cvm-adjust-preview-before">无变动</span>
          </p>
        </div>
      </section>

      <div class="cvm-adjust-footer" :class="{ 'is-floating': isFooterFloating }">
        <bk-button
          theme="primary"
          :loading="submitting || adjustStore.submitLoading"
          :disabled="!canSubmit"
          v-bk-tooltips="{
            content: submitDisabledReason,
            disabled: canSubmit,
          }"
          @click="handleSubmit"
        >
          提交调整
        </bk-button>
        <bk-button
          :disabled="!canRemoveUnchanged"
          v-bk-tooltips="{
            content: removeUnchangedDisabledReason,
            disabled: canRemoveUnchanged,
          }"
          @click="handleRemoveUnchanged"
        >
          移除未修改
        </bk-button>
        <bk-button @click="handleCancel">取消</bk-button>
      </div>
    </div>

    <div ref="bottomSentinelRef" class="cvm-adjust-bottom-sentinel"></div>

    <bk-dialog
      title="移除未调整数据"
      width="680"
      :is-show="removeDialogVisible"
      @confirm="confirmRemoveUnchanged"
      @closed="removeDialogVisible = false"
    >
      未调整数据有
      <span class="cvm-adjust-remove-count">{{ unchangedCount }}</span>
      个，此操作将批量移除表格中未调整数据
    </bk-dialog>
  </div>
</template>

<style scoped lang="scss">
.cvm-adjust-page {
  display: flex;
  flex-direction: column;
  height: 100%;

  // 不撑高、可压缩：行少时底部区块紧跟表格，行多时表格被压到剩余空间内滚动
  .cvm-adjust-main {
    display: flex;
    flex: 0 1 auto;
    flex-direction: column;
    min-height: 0;
    padding: 24px;

    // 数据未回来时表格只有表头，给 loading 留出高度；加载完成后仍跟随内容
    &.is-loading {
      min-height: 200px;
    }
  }

  // 调整预览 + 操作区跟随表格区，内容溢出可视区时才吸底悬浮
  .cvm-adjust-bottom {
    position: sticky;
    bottom: 0;
    z-index: 10;
    flex-shrink: 0;
  }

  .cvm-adjust-preview {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 24px 24px;
    background: #fff;
    border-top: 1px solid #dcdee5;
    box-shadow: 0 -2px 4px 0 rgb(0 0 0 / 8%);
  }

  .cvm-adjust-preview-title {
    font-size: 14px;
    font-weight: 700;
    line-height: 22px;
    color: #313238;
  }

  .cvm-adjust-preview-body {
    padding: 8px 32px;
    border-radius: 2px;

    &.is-changed {
      background: #fdf4e8;
    }
  }

  .cvm-adjust-preview-item {
    display: flex;
    gap: 8px;
    align-items: center;
    height: 32px;
    font-size: 12px;
    line-height: 20px;
  }

  .cvm-adjust-preview-label {
    min-width: 84px;
    text-align: right;
    color: #4d4f56;
  }

  .cvm-adjust-preview-before {
    color: #313238;
  }

  .cvm-adjust-preview-arrow {
    font-size: 22px;
    color: #979ba5;
  }

  .cvm-adjust-preview-after {
    font-weight: 700;
    color: #f59500;
  }

  .cvm-adjust-footer {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 8px 24px;

    .bk-button {
      min-width: 88px;
    }

    // 仅悬浮吸底时才需要底色与上边线
    &.is-floating {
      background: #fafbfd;
      border-top: 1px solid #dcdee5;
    }
  }

  .cvm-adjust-bottom-sentinel {
    height: 1px;
  }

  .cvm-adjust-remove-count {
    color: #ea3636;
  }
}
</style>
