<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { AngleDown } from 'bkui-vue/lib/icon';
import rollRequest from '@blueking/roll-request';
import http from '@/http';
import type { ExportColumn } from '@/utils/common';
import type { PaginationType } from '@/typings';
import type { ModelPropertyColumn } from '@/model/typings';
import type { IDissolveDetail } from '@/store/dissolve/quota';
import ExportToExcelBatchButton from '@/components/export-to-excel-batch-button/index.vue';
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';
import HcmDropdown from '@/components/hcm-dropdown/index.vue';
import ExportDissolveDialog from '@/views/dissolve/components/export-dissolve-dialog/index.vue';
import { EXTENSION_EXPORT_COLUMNS } from './export-column';
import usePage from '@/hooks/use-page';
import useTableSettings from '@/hooks/use-table-settings';
import useTableSelection from '@/hooks/use-table-selection';

interface IDataListProps {
  columns: ModelPropertyColumn[];
  list: IDissolveDetail[];
  pagination: PaginationType;
  condition?: Record<string, any>;
}

const props = defineProps<IDataListProps>();

const emit = defineEmits<{
  select: [selections: IDissolveDetail[]];
}>();

const { handlePageChange, handlePageSizeChange } = usePage();
const { settings } = useTableSettings(props.columns);

const isRowSelectEnable = () => true;

const { selections, checked, handleSelectAll, handleSelectChange } = useTableSelection({
  rowKey: 'id',
  isRowSelectable: isRowSelectEnable,
});

watch(selections, (val) => emit('select', val), { deep: true });

const exportColumns = computed(() => {
  const base = props.columns.map((col) => {
    const baseCol: ExportColumn = { label: col.name as string, field: col.id };
    if (col.exportFormatter) {
      baseCol.exportFormatter = col.exportFormatter;
    }
    return baseCol;
  });
  if (exportExtraParams.value.includeUtilization) {
    return [...base, ...EXTENSION_EXPORT_COLUMNS];
  }
  return base;
});

// 导出参数：由 ExportDissolveDialog 通过 v-model:params 双向绑定，传递给 ExportToExcelBatchButton 的 v-model:extra-params
const exportExtraParams = ref({
  includeUtilization: false,
  utilizationDate: '',
});

// 勾选利用率但未选日期时禁用导出按钮
const isExportConfirmDisabled = computed(
  () => exportExtraParams.value.includeUtilization && !exportExtraParams.value.utilizationDate,
);

// 导出请求（由 ExportToExcelBatchButton 调用，自动传入 signal + extraParams）
const exportAllRequest = async (signal: AbortSignal, extraParams?: Record<string, any>) => {
  const params: Record<string, any> = {};
  const cond = props.condition || {};

  if (cond.bk_biz_ids?.length) params.bk_biz_ids = cond.bk_biz_ids;
  if (cond.project_ids?.length) params.project_ids = cond.project_ids;
  if (cond.group_ids?.length) params.group_ids = cond.group_ids;
  if (cond.operators?.length) params.operators = cond.operators;
  if (cond.inner_ips) params.inner_ips = cond.inner_ips;
  if (cond.asset_ids) params.asset_ids = cond.asset_ids;
  if (cond.status) params.status = cond.status;

  // 勾选附带主机利用率时传入 ES 快照日期（yyyyMMdd 格式）
  if (extraParams?.includeUtilization && extraParams?.utilizationDate) {
    params.snapshot_date = (extraParams.utilizationDate as string).replace(/\//g, '');
  }

  const list = await rollRequest({
    httpClient: http,
    pageEnableCountKey: 'count',
  }).rollReqUseTotalCount(
    '/api/v1/woa/dissolve/host/detail/export/list',
    params,
    {
      limit: 5000,
      total: props.pagination.count,
      listGetter: (res: { data: { details: IDissolveDetail[] } }) => res.data.details,
      countGetter: (res: { data: { count: number } }) => res.data.count,
    },
    { signal },
  );

  return list;
};

const selectedInnerIps = computed(() =>
  selections.value
    .map((row) => row.inner_ip)
    .filter(Boolean)
    .join('\n'),
);

const selectedAssetIds = computed(() =>
  selections.value
    .map((row) => row.asset_id)
    .filter(Boolean)
    .join('\n'),
);

// TODO: 业务跳转待实现
// const handleApply = (_row: IDissolveDetail) => {};

// TODO: 业务跳转待实现
// const handleRecycle = (_row: IDissolveDetail) => {};

defineExpose({
  resetSelections: () => {
    selections.value = [];
  },
});
</script>

<template>
  <div class="dissolve-data-list">
    <div class="data-list-actions">
      <div class="data-list-actions-left">
        <hcm-dropdown :disabled="!selections.length">
          批量复制
          <AngleDown class="dropdown-icon" />
          <template #menus>
            <copy-to-clipboard
              type="dropdown-item"
              text="内网IP"
              :content="selectedInnerIps"
              error-msg="没有可复制内容"
            />
            <copy-to-clipboard
              type="dropdown-item"
              text="固资号"
              :content="selectedAssetIds"
              error-msg="没有可复制内容"
            />
          </template>
        </hcm-dropdown>
      </div>
      <div class="data-list-actions-right">
        <ExportToExcelBatchButton
          use-custom-dialog
          v-model:extra-params="exportExtraParams"
          :confirm-disabled="isExportConfirmDisabled"
          :request="exportAllRequest"
          :columns="exportColumns"
          filename="裁撤明细信息"
          text="导出"
          name="裁撤主机"
          show-icon
          :pick-num="pagination.count"
          :disabled="pagination.count === 0"
        >
          <template #loading-content>
            <div class="dissolve-export-loading">
              <div>
                <bk-loading :loading="true" mode="spin" theme="primary" :opacity="0"></bk-loading>
              </div>
              <div class="loading-text-container">
                <div class="loading-title">导出中...</div>
                <div class="loading-description">正在生成 Excel，请稍候</div>
              </div>
            </div>
          </template>
          <template #dialog-content>
            <ExportDissolveDialog v-model:params="exportExtraParams" :total-count="pagination.count" />
          </template>
        </ExportToExcelBatchButton>
      </div>
    </div>

    <bk-table
      row-hover="auto"
      :data="list"
      :pagination="pagination"
      :max-height="'calc(100vh - 500px)'"
      :settings="settings"
      :is-row-select-enable="isRowSelectEnable"
      remote-pagination
      show-overflow-tooltip
      row-key="id"
      selection-key="id"
      :checked="checked"
      stripe
      @selection-change="handleSelectChange"
      @select-all="handleSelectAll"
      @page-value-change="handlePageChange"
      @page-limit-change="handlePageSizeChange"
    >
      <bk-table-column type="selection" min-width="30" fixed="left" />
      <bk-table-column
        v-for="(column, index) in columns"
        :key="index"
        :prop="column.id"
        :label="column.name"
        :min-width="column.minWidth"
      >
        <template #default="{ row }">
          <display-value
            :property="column"
            :value="(row as any)[column.id]"
            :display="column?.meta?.display"
            v-bind="column?.props"
          />
        </template>
      </bk-table-column>
      <!-- <bk-table-column fixed="right" label="操作" min-width="120">
        <template #default="{ row }">
          <bk-button text theme="primary" @click="handleApply(row)">申领</bk-button>
          <bk-button text theme="primary" style="margin-left: 8px" @click="handleRecycle(row)">回收</bk-button>
        </template>
      </bk-table-column> -->
    </bk-table>
  </div>
</template>

<style lang="scss" scoped>
.dissolve-data-list {
  background: #fff;
  box-shadow: 0 2px 4px 0 #1919290d;
  border-radius: 2px;
  margin: 24px 24px 0;
  padding: 16px 24px;

  .data-list-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .data-list-actions-left,
  .data-list-actions-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .dropdown-icon {
    margin-left: 4px;
  }
}

.dissolve-export-loading {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
  padding: 40px 0;
  flex-direction: column;
  gap: 16px;

  .loading-text-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }

  .loading-title {
    font-size: 18px;
    font-weight: 600;
    color: #313238;
    line-height: 24px;
  }

  .loading-description {
    font-size: 14px;
    color: #63656e;
    line-height: 22px;
  }
}
</style>
