<script setup lang="ts">
import Panel from '@/components/panel/panel.vue';
import { computed, h, ref, useTemplateRef, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Button, Message } from 'bkui-vue';
import { timeFormatter } from '@/common/util';
import { useTable } from '@/hooks/useResourcePlanTable';
import useTableSelection from '@/hooks/use-table-selection';
import { useI18n } from 'vue-i18n';
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';
import { IPageQuery } from '@/typings';
import Stage from './components/stage.vue';
import SubTicketDetail from './sub-ticket-detail.vue';
import BatchApprovalDialog from './components/batch-approval-dialog.vue';
import { useResSubTicketStore, SubTicketItem, STATUS_ENUM, STAGE_ENUM } from '@/store/ticket/res-sub-ticket';
import { useUserStore } from '@/store';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import { debounce } from 'lodash';
import StatusText from './components/status-text.vue';
import { TicketStatus, TicketByIdResult } from '@/typings/resourcePlan';
import routerAction from '@/router/utils/action';
import { MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY } from '@/constants/menu-symbol';

interface Props {
  ticketStatus: TicketStatus; // 主单状态
  demands?: TicketByIdResult['demands']; // 主单需求列表，用于批量审批
  isModifiable?: boolean; // 是否展示「修改需求」按钮
}
const props = withDefaults(defineProps<Props>(), {
  isModifiable: false,
});
// 补全类型泛型
const emits = defineEmits<{
  retryTicket: [];
}>();
const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const subTicketStore = useResSubTicketStore();
const userStore = useUserStore();
const detailRef = useTemplateRef('detailRef');
const batchApprovalDialogRef = useTemplateRef('batchApprovalDialogRef');
const bizId = computed(() => Number(route.query[GLOBAL_BIZS_KEY]));

// 批量审批相关 - 使用 useTableSelection hook
// 业务视图下不显示批量审批功能
const isBusinessView = computed(() => !!bizId.value);

const isRowSelectable = ({ row }: { row: SubTicketItem }) => {
  // 只有在部门审批阶段且状态为审批中的才能选择
  return row.stage === 'admin_audit' && row.status === 'auditing';
};
const {
  selections: selectedItems,
  handleSelectChange: handleSelectionChange,
  handleSelectAll,
  resetSelections,
} = useTableSelection({ isRowSelectable });

// 当前用户是否有审批权限（通过查询任意一个符合条件子单的 audit 信息判断）
const hasApprovalAuth = ref(false);

// 检查当前用户是否有审批权限
const checkApprovalAuth = async (items: SubTicketItem[]) => {
  // 筛选符合条件的子单（部门审批 + 审批中）
  const auditingItems = items.filter((item) => item.stage === 'admin_audit' && item.status === 'auditing');
  if (auditingItems.length === 0) {
    hasApprovalAuth.value = false;
    return;
  }

  // 取第一条查询审批信息
  try {
    const { data } = await subTicketStore.getAudit(auditingItems[0].id, bizId.value);
    const adminAudit = data?.admin_audit;
    if (!adminAudit?.current_steps) {
      hasApprovalAuth.value = false;
      return;
    }

    // 检查 current_steps 中是否有当前用户的审批权限
    const currentUser = userStore.username;
    hasApprovalAuth.value = adminAudit.current_steps.some(
      (step) => step.processors_auth && step.processors_auth[currentUser] === true,
    );
  } catch {
    hasApprovalAuth.value = false;
  }
};

// 是否显示选择列（当前与批量审批按钮一致，后续可独立扩展）
const showSelectionColumn = computed(() => {
  return !isBusinessView.value && hasApprovalAuth.value;
});

// 是否显示批量审批按钮（业务视图下不显示，且当前用户需要有审批权限）
const showBatchApprovalBtn = computed(() => {
  return !isBusinessView.value && hasApprovalAuth.value;
});

// 批量审批按钮禁用状态
const batchApprovalBtnDisabled = computed(() => {
  return selectedItems.value.length === 0;
});

const handleBatchApproval = () => {
  batchApprovalDialogRef.value?.open();
};

const handleBatchApprovalSuccess = () => {
  resetSelections();
  // 5秒后刷新数据
  setTimeout(() => {
    triggerApi();
  }, 5000);
};

// 表格
const hoverIndex = ref(-1);
// 选择列配置
const selectionColumn = {
  type: 'selection',
  width: 32,
  minWidth: 32,
};
const baseColumns: any[] = [
  {
    label: '子单号',
    field: 'id',
    render: ({ row }: { row?: SubTicketItem }) => {
      return h(
        Button,
        {
          text: true,
          theme: 'primary',
          onClick: () => {
            router.replace({ query: { ...route.query, subId: row.id } });
          },
        },
        row.id,
      );
    },
  },
  {
    label: '子单状态',
    field: 'status',
    render({ cell, row }: { cell?: string; row?: SubTicketItem }) {
      const ICON_TYPE: Record<string, any> = {
        init: 'loading',
        auditing: 'loading',
        done: 'success',
        rejected: 'failed',
        failed: 'failed',
        invalid: 'default',
      };
      const txt = STATUS_ENUM[cell] || '---';
      return h(StatusText, { text: txt, type: ICON_TYPE[cell], errorMessage: row.message });
    },
  },
  {
    label: '审批步骤',
    field: 'stage',
    render({ cell, row, index }: { cell?: string; index?: number; row?: SubTicketItem }) {
      if (cell === 'crp_audit') {
        return h(Stage, {
          showExpeditingBtn: row.status === 'auditing' || row.status === 'init',
          text: STAGE_ENUM[cell],
          ticketData: row,
          showActions: hoverIndex.value === index,
        });
      }
      return STAGE_ENUM[cell];
    },
  },
  {
    label: '单据类型',
    field: 'ticket_type_name',
  },
  {
    label: 'CPU核数',
    field: 'updated_info.cvm.cpu_core',
    isDefaultShow: true,
    render: ({ row }: { row?: SubTicketItem }) => {
      const type = row.sub_ticket_type;
      const value = row.updated_info.cvm.cpu_core - row.original_info.cvm.cpu_core;
      let color = '';

      switch (type) {
        case 'add':
          color = '#299e56'; // 绿色
          break;
        case 'cancel':
        case 'adjust':
          color = '#ea3636'; // 红色
          break;
        case 'transfer':
          color = value >= 0 ? '#299e56' : '#ea3636'; // 绿色
          break;
        default:
          color = '#ea3636'; // 红色
      }
      if (isNaN(value)) {
        return '--';
      }
      let prefix = value > 0 ? '+' : '';
      if (value === 0) {
        prefix = type === 'cancel' ? '-' : '+';
      }
      return h('span', { style: { color } }, `${prefix}${value}`);
    },
  },
  {
    label: '单据生成时间',
    field: 'created_at',
    render({ cell }: any) {
      return timeFormatter(cell);
    },
  },
  {
    label: '单据完成时间',
    field: 'updated_at',
    render({ cell }: any) {
      return timeFormatter(cell);
    },
  },
];

// 根据是否显示选择列决定表格列配置
const tableColumns = computed(() => {
  if (showSelectionColumn.value) {
    return [selectionColumn, ...baseColumns];
  }
  return baseColumns;
});

const getData = (page: IPageQuery) => {
  return subTicketStore.getList(
    {
      page,
      ticket_id: route.query?.id as string,
    },
    bizId.value,
  );
};
const { tableData, pagination, isLoading, handlePageChange, handlePageSizeChange, handleSort, triggerApi } =
  useTable(getData);
pagination.value.limit = 500; // 不分页，设置limit为最大值
const retryBtnLoading = ref(false);

// 监听列表数据变化，检查审批权限
watch(
  tableData,
  (items) => {
    if (!isBusinessView.value && items.length > 0) {
      checkApprovalAuth(items);
    }
  },
  { immediate: true },
);

// 数据
const ticketLinkArr = computed(() => {
  return tableData.value.reduce((acc, cur) => {
    if (cur.status === 'auditing' && cur.crp_url) acc.push(cur.crp_url);
    return acc;
  }, []);
});
const successMsg = computed(() => {
  return `复制${ticketLinkArr.value.length}条CRP待审批链接`;
});
// 是否有failed的单据
const hasFailedTicket = computed(() => {
  return tableData.value.some((item) => item.status === 'failed');
});
// 是否禁止终止按钮: failed / partial_failed / rejected / partial_rejected 四种状态下可终止
const TERMINATABLE_STATUSES: TicketStatus[] = ['failed', 'partial_failed', 'rejected', 'partial_rejected'];
const terminatedBtnDisabled = computed(() => !TERMINATABLE_STATUSES.includes(props.ticketStatus));

// 方法
const handleFailedTicket = () => {
  retryBtnLoading.value = true;
  subTicketStore
    .retryTickets(route.query?.id as string, bizId.value)
    .then(() => {
      Message({ theme: 'success', message: '重试成功' });
    })
    .catch(() => {
      Message({ theme: 'error', message: `重试失败` });
    })
    .finally(() => {
      retryBtnLoading.value = false;
      emits('retryTicket');
    });
};
// 防抖 handleFailedTicket
const handleFailedTicketDebounce = debounce(handleFailedTicket, 500);

const handleMouseEnter = (e: any, row: any, index: number) => {
  hoverIndex.value = index;
};
const handleMouseLeave = () => {
  hoverIndex.value = -1;
};

const handleSubTicketShowById = async (id: string) => {
  const list = tableData.value?.length ? tableData.value : (await getData({ limit: 500 })).data.details;
  const subTicketItem = list.find((item) => item.id === id);
  if (subTicketItem) {
    detailRef.value.open(subTicketItem);
  }
};

const terminateBtnLoading = ref(false);
const isShowTerminalDialog = ref(false);
const isTerminated = computed(() => props.ticketStatus === 'terminated');
const handleTerminate = () => {
  subTicketStore
    .terminateTicket(route.query?.id as string, bizId.value)
    .then(() => {
      Message({ theme: 'success', message: '终止成功' });
    })
    .catch(() => {
      Message({ theme: 'error', message: `终止失败` });
    })
    .finally(() => {
      terminateBtnLoading.value = false;
      emits('retryTicket');
    });
};
// 监听 route.query.subId
watch(
  () => route.query.subId,
  (val: string | string[]) => {
    if (val) handleSubTicketShowById(val as string);
  },
  { immediate: true },
);

// 跳转到修改页 (覆盖修改主单)
const handleModify = () => {
  routerAction.redirect({
    name: MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY,
    query: {
      id: route.query?.id as string,
      [GLOBAL_BIZS_KEY]: bizId.value,
    },
  });
};

defineExpose({
  getData: triggerApi,
  tableData,
});
</script>

<template>
  <Panel class="panel" :title="t('子单信息')">
    <template #title-extra>
      <bk-button
        :disabled="isTerminated || !hasFailedTicket"
        style="margin-left: 21px"
        :loading="retryBtnLoading"
        @click="handleFailedTicketDebounce"
      >
        失败单据处理
      </bk-button>
      <copy-to-clipboard :content="ticketLinkArr.join('\n')" :success-msg="successMsg">
        <bk-button
          :disabled="!ticketLinkArr.length"
          v-bk-tooltips="{ content: t('CRP单据已全部审批完成'), disabled: ticketLinkArr.length }"
          style="margin-left: 12px"
        >
          复制CRP待审批链接
        </bk-button>
      </copy-to-clipboard>
      <bk-button
        :disabled="terminatedBtnDisabled"
        style="margin-left: 21px"
        :loading="terminateBtnLoading"
        @click="isShowTerminalDialog = true"
      >
        终止
      </bk-button>
      <bk-button
        v-if="showBatchApprovalBtn"
        theme="primary"
        :disabled="batchApprovalBtnDisabled"
        style="margin-left: 21px"
        @click="handleBatchApproval"
      >
        {{ t('批量审批') }}
      </bk-button>
      <bk-button v-if="isModifiable" style="margin-left: 21px" @click="handleModify">
        {{ t('修改需求') }}
      </bk-button>
    </template>
    <bk-loading :loading="isLoading">
      <bk-table
        :columns="tableColumns"
        :pagination="null"
        :data="tableData"
        :is-row-select-enable="isRowSelectable"
        remote-pagination
        @page-limit-change="handlePageSizeChange"
        @page-value-change="handlePageChange"
        @column-sort="handleSort"
        @row-mouse-enter="handleMouseEnter"
        @row-mouse-leave="handleMouseLeave"
        @select-all="handleSelectAll"
        @selection-change="handleSelectionChange"
      />
    </bk-loading>
  </Panel>
  <SubTicketDetail ref="detailRef" />

  <bk-dialog v-model:is-show="isShowTerminalDialog" title="终止单据" quick-close @confirm="handleTerminate">
    <div>注意：终止单据会将单据置为结束状态</div>
  </bk-dialog>

  <!-- 批量审批弹窗 -->
  <BatchApprovalDialog
    ref="batchApprovalDialogRef"
    :selected-items="selectedItems"
    :demands="props.demands"
    @success="handleBatchApprovalSuccess"
  />
</template>

<style lang="scss" scoped>
.panel {
  box-shadow: none;
  padding-bottom: 25px;
}

.cvm-status-container {
  display: flex;
  align-items: center;
}
</style>
