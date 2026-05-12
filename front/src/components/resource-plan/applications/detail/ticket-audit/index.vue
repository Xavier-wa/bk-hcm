<script setup lang="ts">
import { type Component, computed, h, onUnmounted, ref, VNode, watch } from 'vue';
import type {
  IPlanTicketAudit,
  IPlanTicketAuditLog,
  IPlanTicketCrpAudit,
  IPlanTicketItsmAudit,
  TicketByIdResult,
} from '@/typings/resourcePlan';
import { useI18n } from 'vue-i18n';
import { useUserStore } from '@/store';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import type { TimeoutPollAction } from '@/hooks/use-timeout-poll';
import {
  type ITimelineIconStatusType,
  type ITimelineItem,
  ITimelineNodeType,
} from '@/views/ziyanScr/components/ticket-audit/typings';
import http from '@/http';

import { Message } from 'bkui-vue';
import { WeixinPro, ExclamationCircleShape } from 'bkui-vue/lib/icon';
import Panel from '@/components/panel';
import WName from '@/components/w-name';
import TicketAudit from '@/views/ziyanScr/components/ticket-audit/index.vue';
import TimelineTag from '@/views/ziyanScr/components/ticket-audit/children/timeline-tag.vue';
import TimelineContent from '@/views/ziyanScr/components/ticket-audit/children/timeline-content.vue';
import LoadingIcon from '@/views/ziyanScr/components/ticket-audit/icon/loading-icon.vue';
import ApprovalBtn from '@/views/ziyanScr/components/ticket-audit/children/approval-btn.vue';
import ApprovalBtnAdmin from '@/views/ziyanScr/components/ticket-audit/children/approval-btn-admin.vue';
import { getWNameVNodeList } from '@/views/ziyanScr/components/ticket-audit/utils';
import ExpeditingBtn from '@/views/ziyanScr/components/ticket-audit/children/expediting-btn.vue';
import successIcon from '@/assets/image/corret-fill.png';
import { timeFormatter } from '@/common/util';
import { useResSubTicketStore, AdminAudit, SubTicketAudit } from '@/store/ticket/res-sub-ticket';
import { useRoute } from 'vue-router';
import { GLOBAL_BIZS_KEY } from '@/common/constant';

interface Step {
  name: string;
  auto?: boolean;
  link?: VNode;
}
interface IProps {
  detail: Partial<IPlanTicketAudit & SubTicketAudit>;
  fetchData: () => Promise<void>;
  timeoutPollAction: TimeoutPollAction;
  ticketStatus?: TicketByIdResult['status_info'];
}

defineOptions({ name: 'resource-plan-ticket-audit' });
const props = withDefaults(defineProps<IProps>(), {});

const { t } = useI18n();
const { getBusinessApiPath, isBusinessPage } = useWhereAmI();
const userStore = useUserStore();
const route = useRoute();
const bizId = computed(() => Number(route.query[GLOBAL_BIZS_KEY]));

const crpSteps: Step[] = [
  {
    name: t('部门管理员审批'),
    link: h(
      WName,
      { name: 'ICR', alias: t('企微催单（ICR助手）'), class: 'ml8' },
      { icon: () => h(WeixinPro, { width: 16, height: 16, class: 'mr5' }) },
    ),
  },
  { name: t('规划经理审批') },
  { name: t('需求经理审批') },
  { name: t('架平审批'), auto: true },
  { name: t('资源总监审批'), auto: true },
  { name: t('流程结束'), auto: true },
];

const renderItsmAuditLogs = ref<ITimelineItem[]>([]);
const renderAdminAuditLogs = ref<ITimelineItem[]>([]);
const renderCrpAuditLogs = ref<ITimelineItem[]>([]);

// 历史节点的tag展示：itsm没有提供name字段，直接展示message；crp正常展示name字段
const getHistoryStepTag = (log: IPlanTicketAuditLog, auditType: 'itsm' | 'crp') => {
  const key = auditType === 'itsm' ? 'message' : 'name';
  return h(TimelineTag, null, log[key]);
};
// 历史节点的content展示：itsm只展示操作时间；crp展示操作人、信息、操作时间
const getHistoryStepContent = (
  { operate_at, operator, message = '', operate_info }: IPlanTicketAuditLog & { operate_info?: string },
  auditType: 'itsm' | 'crp',
) => {
  if (auditType === 'itsm') {
    return h(TimelineContent, { class: 'time-value' }, operate_at);
  }

  const contentChildren = [h('p', { class: 'message' }, [h(WName, { name: operator, class: 'mr4' }), `${message}`])];

  // 展示审批意见（如果有）
  if (operate_info) {
    contentChildren.push(h('p', { class: 'operate-info' }, `审批意见：${operate_info}`));
  }

  contentChildren.push(h('p', { class: 'time-value' }, timeFormatter(operate_at)));

  return h(TimelineContent, null, contentChildren);
};

const getHistoryStepItems = (
  audit: IPlanTicketItsmAudit | IPlanTicketCrpAudit | AdminAudit,
  auditType: 'itsm' | 'crp',
) => {
  const { logs, status } = audit || {};

  // 兼容 crp init/failed 的情况
  if (!logs) return [];

  return logs.map<ITimelineItem>((log, index) => {
    // 如果是最后一个节点，且单据状态为revoked、canceled、rejected，则展示为danger，否则为success
    const type: ITimelineIconStatusType =
      ['revoked', 'canceled', 'rejected'].includes(status) && index === logs.length - 1 ? 'danger' : 'success';

    return {
      tag: getHistoryStepTag(log, auditType),
      content: getHistoryStepContent(log, auditType),
      nodeType: ITimelineNodeType.VNode,
      type,
    };
  });
};

// auditing展示审批操作、failed展示审批失败原因
const getCurrentStepItems = (
  { current_steps, status, status_name }: IPlanTicketItsmAudit | IPlanTicketCrpAudit | AdminAudit,
  showApproval = true,
  auditType?: string,
) => {
  // 兼容 crp init/failed 的情况
  if (!current_steps) return [];

  return current_steps.map<ITimelineItem>(({ name, processors }) => {
    // 过滤无效审批人
    const displayProcessors = processors.filter((processor) => processor);
    const isAuditing = status === 'auditing';
    // auditing状态，且用户为审批人时，可以进行审批处理
    const hasApprovalBtn = showApproval && isAuditing && displayProcessors.includes(userStore.username);
    const comp: Component = auditType === 'admin' ? ApprovalBtnAdmin : ApprovalBtn;
    const WNameVNodeList = getWNameVNodeList(displayProcessors);
    // 审批人信息，若无则不显示审批中文本
    const content = WNameVNodeList.length ? [status_name ? `${status_name}，` : '', `请联系 `, WNameVNodeList] : null;

    return {
      tag: h(TimelineTag, { isCurrent: true }, [
        name,
        hasApprovalBtn
          ? h(
              comp,
              {
                key: `approval-btn-${name}`,
                class: 'ml24',
                loading: approvalLoading.value,
                confirmHandler: auditType === 'admin' ? approvalAdminAudit : approvalItsmAudit,
                onShown: () => {
                  // 当进入审批操作的时候，暂停定时刷新任务
                  props.timeoutPollAction?.reset();
                },
                onHidden: () => {
                  // 当退出审批操作的时候，恢复定时刷新任务
                  props.timeoutPollAction?.resume();
                },
              },
              t('立即处理'),
            )
          : null,
      ]),
      content: h(TimelineContent, null, content),
      nodeType: ITimelineNodeType.VNode,
      icon: isAuditing ? h(LoadingIcon) : undefined,
      type: isAuditing ? 'primary' : 'danger',
    };
  });
};

const renderItsmLogs = (audit: IPlanTicketItsmAudit) => {
  return [...getHistoryStepItems(audit, 'itsm'), ...getCurrentStepItems(audit)];
};

const getStaticCrpLogs = (options?: Partial<ITimelineItem>) =>
  crpSteps.map<ITimelineItem>(({ name }) => ({
    tag: h(TimelineTag, null, name),
    nodeType: ITimelineNodeType.VNode,
    ...options,
  }));
const renderCrpLogs = (audit: IPlanTicketCrpAudit) => {
  // 空值兼容
  if (!audit?.crp_sn) return getStaticCrpLogs();

  // 获取crp审批流信息
  const { status, logs, current_steps } = audit;

  // 撤销、失败、结束、拒绝，done 这五种状态，直接输出response
  if (['revoked', 'rejected', 'canceled', 'failed', 'done'].includes(status)) {
    // 存在一种crp自动跳过的情况
    if (status === 'done' && !logs.length && !current_steps.length) {
      return getStaticCrpLogs({ type: 'success' });
    }
    return [...getHistoryStepItems(audit, 'crp'), ...getCurrentStepItems(audit)];
  }

  const currentStepIdx = crpSteps.findIndex((step) => step.name === current_steps?.[0]?.name);

  // auditing 状态展示全部流程
  return crpSteps.map<ITimelineItem>(({ name, auto, link }, index) => {
    // 已审批通过的节点
    if (index < currentStepIdx) {
      return {
        tag: h(TimelineTag, null, [logs[index]?.name ?? name, status === 'auditing' ? link : null]),
        content: auto || !logs[index] ? '' : getHistoryStepContent(logs[index], 'crp'),
        nodeType: ITimelineNodeType.VNode,
        type: 'success',
      };
    }

    // 当前节点
    if (index === currentStepIdx) return getCurrentStepItems(audit, false)[0]; // TODO: CRP暂不支持审批

    // 未来节点
    return { tag: h(TimelineTag, null, logs[index]?.name ?? name), content: '', nodeType: ITimelineNodeType.VNode };
  });
};

const renderAdminLogs = (audit: AdminAudit) => {
  // 如果是通过/自动通过状态则不展示
  if (['skip'].includes(audit?.status)) return;
  if (audit?.status === 'done') {
    audit?.logs?.forEach((log) => {
      log.message = '已审批，审批通过';
    });
  }
  renderAdminAuditLogs.value = [...getHistoryStepItems(audit, 'crp'), ...getCurrentStepItems(audit, true, 'admin')];
};

const expeditingBtnProps = computed(() => {
  const { itsm_audit, crp_audit } = props.detail || {};
  const { processors = [], processors_auth = {} } =
    itsm_audit?.current_steps?.[0] || crp_audit?.current_steps?.[0] || {};

  // 过滤无效审批人
  const displayProcessors = processors.filter((processor) => processor);

  const processorsWithBizAccess = displayProcessors.filter((processor) => {
    if (!isBusinessPage) return processor; // 资源下不判断权限
    return processors_auth[processor];
  }); // 有权限的审批人
  const processorsWithoutBizAccess = displayProcessors.filter((processor) => !processors_auth[processor]); // 无权限的审批人

  const platform = itsm_audit?.status === 'auditing' ? 'ITSM' : 'CRP';
  const copyText = `${t('复制')} ${platform} ${t('审批单')}`;
  const ticketLink = itsm_audit?.status === 'auditing' ? itsm_audit?.itsm_url : crp_audit?.crp_url;

  return {
    checkPermission: platform !== 'CRP',
    processors: displayProcessors,
    processorsWithBizAccess,
    processorsWithoutBizAccess,
    copyText,
    ticketLink,
    defaultShow: false,
  };
});

// 审批操作
const approvalLoading = ref(false);
let approvalLoadingTimer: ReturnType<typeof setTimeout> | null = null;
const approvalItsmAudit = async ({ approval, remark }: { approval: boolean; remark: string }) => {
  const { ticket_id } = props.detail;
  const { state_id } = props.detail.itsm_audit.current_steps[0];
  const params = { state_id, approval, remark };
  await http.post(`/api/v1/woa/${getBusinessApiPath()}plans/resources/tickets/${ticket_id}/approve_itsm_node`, params);

  Message({ theme: 'success', message: t('请求已提交，5s后自动刷新') });
  // 5s后刷新itsm审批流信息
  approvalLoading.value = true;
  renderItsmAuditLogs.value = renderItsmLogs(props.detail.itsm_audit); // 重新渲染timeline
  approvalLoadingTimer = setTimeout(() => {
    approvalLoading.value = false;
    props.fetchData();
  }, 5000);
};
const approvalAdminAudit = async (params: { approval: boolean; use_transfer_pool: boolean; operate_info?: string }) => {
  const store = useResSubTicketStore();
  const { id } = props.detail;

  await store.approveAdminNode(id, params, bizId.value);

  Message({ theme: 'success', message: t('请求已提交，5s后自动刷新') });
  // 5s后刷新itsm审批流信息
  approvalLoading.value = true;
  approvalLoadingTimer = setTimeout(async () => {
    approvalLoading.value = false;
    await props.fetchData();
    renderAdminLogs(props?.detail?.admin_audit); // 重新渲染timeline
  }, 5000);
};

watch(
  () => props.detail,
  (detail) => {
    if (!detail) {
      renderItsmAuditLogs.value = [];
      renderCrpAuditLogs.value = [];
      renderAdminAuditLogs.value = [];
      return;
    }

    const { itsm_audit, crp_audit, admin_audit } = detail;
    if (admin_audit) {
      renderAdminLogs(admin_audit);
      // if (!crp_audit) return Message({ theme: 'error', message: t('CRP单据信息异常') });
      if (admin_audit?.status === 'done' || admin_audit?.status === 'skip') {
        if (!crp_audit) {
          renderCrpAuditLogs.value = getStaticCrpLogs();
        } else {
          renderCrpAuditLogs.value = renderCrpLogs(crp_audit);
        }
      } else if (admin_audit?.status === 'auditing') renderCrpAuditLogs.value = getStaticCrpLogs();
      return;
    }

    if (!itsm_audit) return;
    //  Message({ theme: 'error', message: t('ITSM单据信息异常') });
    renderItsmAuditLogs.value = renderItsmLogs(itsm_audit);

    // crp_audit 可能为空, 为空则不展示crp审批信息
    if (!crp_audit) return (renderCrpAuditLogs.value = []);
    if (itsm_audit?.status === 'done') renderCrpAuditLogs.value = renderCrpLogs(crp_audit);
    // 如果itsm处于审批中状态，crp审批信息展示静态数据
    else if (itsm_audit?.status === 'auditing') renderCrpAuditLogs.value = getStaticCrpLogs();
  },
  { immediate: true, deep: true },
);

const hasAuditAuth = (type: 'itsm_audit' | 'crp_audit') => {
  return (
    isBusinessPage &&
    props.detail?.[type]?.status === 'auditing' &&
    props.detail?.[type]?.current_steps?.[0]?.processors.includes(userStore.username) &&
    !props.detail?.[type]?.current_steps?.[0]?.processors_auth[userStore.username]
  );
};

const itsmSkip = computed(() => {
  return props?.ticketStatus?.itsm_sn === 'skip' || !props?.detail?.itsm_audit;
});

onUnmounted(() => {
  clearTimeout(approvalLoadingTimer);
  props.timeoutPollAction?.reset();
});
</script>

<template>
  <panel v-if="renderItsmAuditLogs.length || detail?.admin_audit || itsmSkip" class="panel" :title="t('审批信息')">
    <div class="step-wrap">
      <h3 class="label">{{ t('业务审批') }}：</h3>
      <ticket-audit
        class="content"
        :ticket-link="detail?.itsm_audit?.itsm_url"
        :logs="renderItsmAuditLogs"
        :copy-text="t('复制ITSM审批单')"
      >
        <template #title>
          <div class="aduit-title">
            <p v-if="!itsmSkip">
              <img v-if="detail?.itsm_audit?.status === 'done'" width="17" height="17" :src="successIcon" alt="" />
              {{ t('ITSM 平台审批') }}
            </p>
            <p v-else>
              <img width="17" height="17" :src="successIcon" alt="" />
              审批自动通过
            </p>
          </div>
        </template>
        <template #tools>
          <bk-popover
            v-if="hasAuditAuth('itsm_audit')"
            max-width="270"
            placement="top"
            :content="t('当前审批人无「业务访问」权限，请复制ITSM链接，在企业微信联系审批人进行催单')"
          >
            <exclamation-circle-shape class="ml12" width="18" height="18" fill="#EA3636" style="cursor: pointer" />
          </bk-popover>
        </template>
        <template #toolkit>
          <ExpeditingBtn v-if="detail?.itsm_audit?.status === 'auditing'" v-bind="expeditingBtnProps" />
        </template>
      </ticket-audit>
    </div>

    <!-- admin_audit比较特殊 只有一个节点 -->
    <div class="step-wrap" v-if="detail?.admin_audit">
      <h3 class="label">{{ t('部门审批') }}：</h3>
      <ticket-audit class="content" :logs="renderAdminAuditLogs">
        <template #title>
          <div class="aduit-title">
            <p v-if="detail?.admin_audit?.status === 'done'">
              <img width="17" height="17" :src="successIcon" alt="" />
              管理员审批通过
            </p>
            <p v-else-if="detail?.admin_audit?.status === 'skip'">
              <img width="17" height="17" :src="successIcon" alt="" />
              管理员审批自动通过
            </p>
            <p v-else>管理员审批</p>
          </div>
        </template>
        <template #toolkit>
          <ExpeditingBtn
            v-if="detail?.admin_audit?.status === 'auditing'"
            :processors="detail.admin_audit?.current_steps?.[0]?.processors"
            :default-show="false"
            :check-permission="false"
          />
        </template>
      </ticket-audit>
    </div>

    <div class="step-wrap" v-if="renderCrpAuditLogs.length">
      <h3 class="label">{{ t('公司审批') }}：</h3>
      <ticket-audit
        class="content"
        :ticket-link="detail?.crp_audit?.crp_url"
        :logs="renderCrpAuditLogs"
        :copy-text="t('复制CRP审批单')"
      >
        <template #title>
          <div class="aduit-title">
            <p>
              <img v-if="detail?.crp_audit?.status === 'done'" width="17" height="17" :src="successIcon" alt="" />
              {{ t('CRP 平台审批') }}
            </p>
          </div>
        </template>
        <template #tools>
          <bk-popover
            v-if="hasAuditAuth('crp_audit')"
            max-width="270"
            placement="top"
            :content="t('当前审批人无「业务访问」权限，请复制CRP链接，在企业微信联系审批人进行催单')"
          >
            <exclamation-circle-shape class="ml12" width="18" height="18" fill="#EA3636" style="cursor: pointer" />
          </bk-popover>
        </template>
        <template #toolkit>
          <ExpeditingBtn v-if="detail?.crp_audit?.status === 'auditing'" v-bind="expeditingBtnProps" />
        </template>
      </ticket-audit>
    </div>
  </panel>
</template>

<style scoped lang="scss">
.panel {
  margin-bottom: 16px;

  .step-wrap {
    display: flex;
    padding: 0 60px 0 36px;

    .label {
      flex-shrink: 0;
      font-size: 14px;
      width: 80px;
      color: $danger-color;
    }

    .content {
      flex: 1;
    }

    &:not(:last-of-type) {
      margin-bottom: 16px;
    }
  }
}

.aduit-title {
  color: $font-deep-color;
  font-weight: 700;

  p {
    display: flex;
    align-items: center;
  }

  img {
    margin-right: 9px;
  }
}

:deep(.operate-info) {
  color: #63656e;
  margin-top: 4px;
}
</style>
