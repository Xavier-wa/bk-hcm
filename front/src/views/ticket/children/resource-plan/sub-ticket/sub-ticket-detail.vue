<script setup lang="ts">
import { ref, computed, provide } from 'vue';
import useTimeoutPoll from '@/hooks/use-timeout-poll';

import Approval from '@/components/resource-plan/applications/detail/approval';
import ResourcePlanTicketAudit from '@/components/resource-plan/applications/detail/ticket-audit/index.vue';
import Basic from '@/components/resource-plan/applications/detail/basic/index.vue';
import ResourcePlanList from '@/components/resource-plan/applications/detail/list/index.vue';
import { timeFormatter } from '@/common/util';
import { useRoute, useRouter } from 'vue-router';
import { useResSubTicketStore, SubTicketItem } from '@/store/ticket/res-sub-ticket';
import { GLOBAL_BIZS_KEY } from '@/common/constant';

// 路由、状态管理、工具函数
const store = useResSubTicketStore();
const route = useRoute();
const router = useRouter();

// 响应式数据
const isShow = ref(false);
const ticketId = ref('');
const ticketDetail = ref();
const ticketAuditDetail = ref();
const isLoading = ref(false);
const errorMessage = ref<string>();
const ticketListData = ref();
const bizId = computed(() => Number(route.query[GLOBAL_BIZS_KEY]));
provide('ticketListData', ticketListData);
provide('ticketDetail', ticketDetail);

// 计算属性：是否显示审批详情组件
const isTicketAuditDetailShow = computed(() => {
  return ticketAuditDetail.value?.admin_audit.status !== 'init';
});
const baseList = computed(() => [
  {
    label: '类型',
    value: ticketDetail.value?.base_info?.type_name,
  },

  {
    label: '创建时间',
    value: timeFormatter(ticketDetail.value?.base_info?.submitted_at, 'YYYY-MM-DD'),
  },
]);

// 获取数据的逻辑
const getResultData = async () => {
  clear();
  try {
    isLoading.value = true;
    const [ticketRes, ticketAuditRes] = await Promise.all([
      store.getDetail(ticketId.value, bizId.value),
      store.getAudit(ticketId.value, bizId.value),
    ]);

    // 错误信息处理
    if (ticketRes.data?.status_info?.status === 'failed') {
      errorMessage.value = ticketRes.data?.status_info?.message;
    } else {
      errorMessage.value = '';
    }

    // 响应式数据赋值
    ticketDetail.value = ticketRes?.data;
    ticketAuditDetail.value = ticketAuditRes?.data;

    // 轮询逻辑：init 或 auditing 状态时自动刷新；sideslider 已关闭则不再唤起轮询
    // （卸载场景由 useTimeoutPoll 的作用域销毁统一兜底，此处仅需处理「关闭但未卸载」）
    if (
      isShow.value &&
      (ticketRes?.data?.status_info?.status === 'init' || ticketRes?.data?.status_info?.status === 'auditing')
    ) {
      autoFlushTask.resume();
    } else {
      autoFlushTask.reset();
    }
  } catch (error) {
    console.error('error', error); // eslint-disable-line no-console
  } finally {
    isLoading.value = false;
  }
};

const clear = () => {
  ticketDetail.value = undefined;
  ticketAuditDetail.value = undefined;
};

// 轮询任务：30 秒自动刷新
const autoFlushTask = useTimeoutPoll(() => {
  getResultData();
}, 30000);

// 导出方法
const open = (data: SubTicketItem) => {
  ticketId.value = data.id;
  getResultData();
  isShow.value = true;
  ticketListData.value = data;
};
const close = () => {
  isShow.value = false;
};
const handleClose = () => {
  // 关闭子单详情时停止轮询
  autoFlushTask.reset();
  // 删除路由上的 subId 参数，如果有的话
  if (route.query.subId) {
    router.replace({
      query: {
        ...route.query,
        subId: undefined,
      },
    });
  }
};

defineExpose({
  open,
  close,
});
</script>

<template>
  <bk-sideslider v-model:is-show="isShow" width="80%" render-directive="if" @hidden="handleClose">
    <template #header>
      <div>
        子单详情
        <span style="color: #979ba5">- {{ ticketId }}</span>
      </div>
    </template>

    <bk-loading :loading="isLoading" style="z-index: 9999">
      <section class="sub-ticket-container">
        <!-- 当前审批节点信息 -->
        <Approval
          class="mb-16"
          :status-info="ticketDetail?.status_info"
          :error-message="errorMessage"
          :ticket-audit-detail="ticketAuditDetail"
        />

        <!-- 审批信息 -->
        <ResourcePlanTicketAudit
          class="mb-16"
          v-if="isTicketAuditDetailShow"
          :detail="ticketAuditDetail"
          :fetch-data="getResultData"
          :timeout-poll-action="autoFlushTask"
        />

        <!-- 基本信息 -->
        <Basic class="mb-16" :data-list="baseList" />
        <!-- 资源预测列表 -->
        <ResourcePlanList
          v-show="!isLoading"
          :demands="ticketDetail?.demands"
          :ticket-type="ticketDetail?.base_info?.type"
          :show-cpu-count="false"
        />
      </section>
    </bk-loading>
  </bk-sideslider>
</template>

<style lang="scss" scoped>
.sub-ticket-container {
  padding: 24px;
  width: 100%;
  overflow-x: hidden;
}

.mb-16 {
  margin-bottom: 16px;
}

:deep(.bk-tab-content) {
  padding: 0;
}

:deep(.bk-modal-content) {
  overflow-y: auto;
}

:deep(.bk-modal-body) {
  background-color: #f5f7fa;
}

.divider {
  margin: 0 24px;
}
</style>
