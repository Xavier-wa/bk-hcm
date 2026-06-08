import { defineComponent, PropType, inject, ref } from 'vue';
import { Message } from 'bkui-vue';
import { useI18n } from 'vue-i18n';
import { useResourcePlanStore } from '@/store';
import routerAction from '@/router/utils/action';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import { MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS } from '@/constants/menu-symbol';
import cssModule from './index.module.scss';
import type { IPlanTicket } from '@/typings/resourcePlan';

export default defineComponent({
  props: {
    modelValue: Object as PropType<IPlanTicket>,
    ticketId: {
      type: String,
      required: true,
    },
  },

  setup(props) {
    const { t } = useI18n();
    const resourcePlanStore = useResourcePlanStore();
    const isLoading = ref(false);
    const validate = inject<() => Promise<void>>('validate');

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
        await resourcePlanStore.overwriteBizPlan(props.modelValue.bk_biz_id, props.ticketId, {
          demand_class: props.modelValue.demand_class,
          demands: props.modelValue.demands,
          remark: props.modelValue.remark,
        });
        redirectToDetail();
      } catch (error: any) {
        Message({
          message: error.message || error,
          theme: 'error',
        });
      } finally {
        isLoading.value = false;
      }
    };

    const handleCancel = () => {
      redirectToDetail();
    };

    return () => (
      <section>
        <bk-button onClick={handleClick} loading={isLoading.value} theme='primary' class={cssModule.button}>
          {t('提交')}
        </bk-button>
        <bk-button onClick={handleCancel} disabled={isLoading.value} class={cssModule.button}>
          {t('取消')}
        </bk-button>
      </section>
    );
  },
});
