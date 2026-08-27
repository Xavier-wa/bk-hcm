<script setup lang="ts">
import { computed, h, onUnmounted, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import { useUserStore } from '@/store';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import { PAGE_BIZ_KEY } from '@/common/constant';
import useTimeoutPoll from '@/hooks/use-timeout-poll';
import { type ITimelineItem, ITimelineNodeType } from '@/views/ziyanScr/components/ticket-audit/typings';
import http from '@/http';

import { Message } from 'bkui-vue';
import { ExclamationCircleShape } from 'bkui-vue/lib/icon';
import TicketAudit from '@/views/ziyanScr/components/ticket-audit/index.vue';
import TimelineTag from '@/views/ziyanScr/components/ticket-audit/children/timeline-tag.vue';
import TimelineContent from '@/views/ziyanScr/components/ticket-audit/children/timeline-content.vue';
import ApprovalBtn from '@/views/ziyanScr/components/ticket-audit/children/approval-btn.vue';
import LoadingIcon from '@/views/ziyanScr/components/ticket-audit/icon/loading-icon.vue';
import { getWNameVNodeList } from '@/views/ziyanScr/components/ticket-audit/utils';

export interface IItsmTicketAudit {
  order_id: number;
  itsm_ticket_id: string;
  itsm_ticket_link: string;
  status: 'RUNNING' | 'FINISHED' | 'TERMINATED';
  current_steps: {
    name: string;
    processors: string[];
    state_id: number;
    processors_auth: {
      [key: string]: boolean;
    };
  }[];
  logs: {
    operator: string;
    operate_at: string;
    message: string;
    source: string;
  }[];
}

const props = defineProps<{ data: IItsmTicketAudit; isLoading: boolean; refreshApi: () => Promise<void> }>();

const userStore = useUserStore();
const { t } = useI18n();
const route = useRoute();
const { getBusinessApiPath } = useWhereAmI();

// 审批必须落在单据所属业务下，不能取顶部业务选择器的全局业务，否则会出现跨业务审批
const orderBizId = computed(() => Number(route.query[PAGE_BIZ_KEY]));

const getDefaultData = (): Partial<IItsmTicketAudit> => ({ itsm_ticket_link: '', logs: [], current_steps: [] });
const auditData = reactive(getDefaultData());
const renderLogs = ref<ITimelineItem[]>([]);

// 审批操作
const approvalLoading = ref(false);
let approvalLoadingTimer: ReturnType<typeof setTimeout> | null = null;
const approvalItsmAudit = async ({ approval, remark }: { approval: boolean; remark: string }) => {
  const { order_id } = auditData;
  const { state_id } = auditData.current_steps[0];
  const params = { order_id, state_id, approval, remark };

  await http.post(`/api/v1/woa/${getBusinessApiPath(orderBizId.value)}task/audit/apply/ticket`, params);

  Message({ theme: 'success', message: t('请求已提交，5s后自动刷新') });
  // 5s后刷新itsm审批流信息
  approvalLoading.value = true;
  renderLogs.value = getRenderLogs(auditData as IItsmTicketAudit); // 重新渲染timeline
  approvalLoadingTimer = setTimeout(() => {
    approvalLoading.value = false;
    props.refreshApi();
  }, 5000);
};

const getRenderLogs = ({ logs, current_steps }: IItsmTicketAudit) => {
  // 本期直接输出 logs + current_steps
  return [
    ...logs.map<ITimelineItem>(({ message, operate_at }) => ({
      tag: h(TimelineTag, null, message),
      content: h(TimelineContent, { class: 'time-value' }, operate_at),
      nodeType: ITimelineNodeType.VNode,
      type: 'success',
    })),
    ...current_steps.map<ITimelineItem>(({ name, processors }) => {
      if (processors.length === 1 && processors[0] === '系统自动处理') {
        return {
          tag: h(TimelineTag, { isCurrent: true }, name),
          content: h(TimelineContent, null, processors),
          nodeType: ITimelineNodeType.VNode,
          icon: h(LoadingIcon),
        };
      }

      // 当前用户为审批人时，可以进行审批处理
      const hasHandleBtn = processors.includes(userStore.username);

      return {
        tag: h(TimelineTag, { isCurrent: true }, [
          name,
          hasHandleBtn
            ? h(
                ApprovalBtn,
                {
                  class: 'ml24',
                  loading: approvalLoading.value,
                  confirmHandler: approvalItsmAudit,
                  onShown: () => {
                    // 当进入审批操作的时候，暂停定时刷新任务
                    refreshTask.reset();
                  },
                  onHidden: () => {
                    // 当退出审批操作的时候，恢复定时刷新任务
                    refreshTask.resume();
                  },
                },
                t('立即处理'),
              )
            : null,
        ]),
        content: h(TimelineContent, null, [getWNameVNodeList(processors), t('正在审批中...')]),
        nodeType: ITimelineNodeType.VNode,
        icon: h(LoadingIcon),
      };
    }),
  ];
};

watch(
  () => props.data,
  (val) => {
    if (val) {
      Object.assign(auditData, val);
      // 构建timeline节点
      renderLogs.value = getRenderLogs(val);
      // 如果单据处于处理中(RUNNING)状态, 创建定时任务(30s刷新一次, 最多刷新60次)
      if (val.status === 'RUNNING') {
        refreshTask.resume();
      }
    } else {
      Object.assign(auditData, getDefaultData());
    }
  },
);

const refreshTask = useTimeoutPoll(
  () => {
    props.refreshApi();
  },
  30000,
  { max: 60 },
);

onUnmounted(() => {
  clearTimeout(approvalLoadingTimer);
  refreshTask.reset();
});
</script>

<template>
  <ticket-audit
    class="itsm-ticket-audit"
    :title="t('ITSM平台审批')"
    :loading="isLoading"
    :ticket-link="auditData.itsm_ticket_link"
    :logs="renderLogs"
    :copy-text="t('复制ITSM审批单')"
  >
    <template #tools>
      <bk-popover
        v-if="
          auditData.status === 'RUNNING' &&
          auditData.current_steps?.[0]?.processors.includes(userStore.username) &&
          !auditData.current_steps?.[0]?.processors_auth[userStore.username]
        "
        max-width="270"
        placement="top"
        :content="t('当前审批人无「业务访问」权限，请复制ITSM链接，在企业微信联系审批人进行催单')"
      >
        <exclamation-circle-shape class="ml12" width="18" height="18" fill="#EA3636" style="cursor: pointer" />
      </bk-popover>
    </template>
  </ticket-audit>
</template>

<style scoped lang="scss">
.itsm-ticket-audit {
  padding: 0 16px;
}
</style>
