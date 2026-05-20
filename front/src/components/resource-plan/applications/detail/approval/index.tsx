import { defineComponent, VNode, type PropType } from 'vue';
import StatusUnknown from '@/assets/image/Status-unknown.png';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';
import Panel from '@/components/panel';
import cssModule from './index.module.scss';
import routerAction from '@/router/utils/action';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import { MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY } from '@/constants/menu-symbol';
// import ExpeditingBtn from '@/views/ziyanScr/components/ticket-audit/children/expediting-btn.vue';

import type { IPlanTicketAudit, TicketByIdResult } from '@/typings/resourcePlan';
import { SubTicketAudit, SubTicketDetail } from '@/store/ticket/res-sub-ticket';

export default defineComponent({
  props: {
    statusInfo: Object as PropType<Partial<TicketByIdResult['status_info'] & SubTicketDetail['status_info']>>,
    errorMessage: String,
    ticketAuditDetail: {
      type: Object as PropType<Partial<IPlanTicketAudit & SubTicketAudit>>,
      default: () => ({}),
    },
    isModifiable: {
      type: Boolean,
      default: false,
    },
  },
  setup(props) {
    const { t } = useI18n();
    const route = useRoute();

    const handleModify = () => {
      routerAction.redirect({
        name: MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY,
        query: {
          id: route.query?.id as string,
          [GLOBAL_BIZS_KEY]: route.query?.[GLOBAL_BIZS_KEY],
        },
      });
    };

    const renderIcon = () => {
      switch (props.statusInfo?.status) {
        case 'auditing':
          return <bk-loading style='transform: scale(0.5)' mode='spin' theme='primary' loading></bk-loading>;
        case 'rejected':
        case 'partial_rejected':
          return <i class='hcm-icon bkhcm-icon-38moxingshibai-01'></i>;
        case 'done':
          return <i class='hcm-icon bkhcm-icon-7chenggong-01'></i>;
        case 'failed':
        case 'partial_failed':
        case 'terminated':
          return <i class='hcm-icon bkhcm-icon-close-circle-fill'></i>;
        case 'revoked':
          return <img src={StatusUnknown} style={{ width: '22px', marginRight: '10px' }} />;
        default:
          return <img src={StatusUnknown} style={{ width: '22px', marginRight: '10px' }} />;
      }
    };

    const renderAuditStatus = (): VNode => {
      const { itsm_audit, crp_audit, admin_audit } = props?.ticketAuditDetail || {};
      const isAuditing =
        itsm_audit?.status === 'auditing' || crp_audit?.status === 'auditing' || admin_audit?.status === 'auditing';
      if (!isAuditing) return null;

      const tagText = admin_audit?.status === 'auditing' ? '管理员审批' : itsm_audit?.status_name;
      if (!tagText) return null;
      return (
        <div class='flex-row align-items-center'>
          <span class={cssModule['audit-status']}>
            {t('当前处于')}
            <bk-tag theme='info' class='ml4 mr4'>
              {tagText}
            </bk-tag>
            {t('节点')}
          </span>
        </div>
      );
    };

    return () => (
      <Panel class={cssModule.home}>
        <span class={cssModule.status}>
          {renderIcon()}
          <span>{props.statusInfo?.status_name}</span>
          {renderAuditStatus()}
          {props.errorMessage && (
            <div class={cssModule['error-message']}>
              <i class={`hcm-icon bkhcm-icon-alert ${cssModule['error-message-color']}`} />
              <span>{props.errorMessage}</span>
            </div>
          )}
        </span>
        {props.isModifiable && (
          <bk-button class={cssModule['modify-btn']} onClick={handleModify}>
            {t('修改需求')}
          </bk-button>
        )}
      </Panel>
    );
  },
});
