import { defineComponent, PropType, inject, ref } from 'vue';
import { Message } from 'bkui-vue';
import { useI18n } from 'vue-i18n';
import { useResourcePlanStore } from '@/store';
import routerAction from '@/router/utils/action';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import { MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS } from '@/constants/menu-symbol';
import cssModule from './index.module.scss';
import type { IPlanTicket, IPlanTicketOverwriteDemand, TicketDemands } from '@/typings/resourcePlan';

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

    const buildDemandPayload = (): IPlanTicketOverwriteDemand[] => {
      const ticketType = props.modelValue.ticket_type;

      return props.modelValue.demands.map((d) => {
        const updatedInfo: TicketDemands = {
          obs_project: d.obs_project,
          expect_time: d.expect_time,
          return_plan_time: d.return_plan_time || undefined,
          region_id: d.region_id,
          zone_id: d.zone_id || undefined,
          demand_source: d.demand_source,
          remark: d.remark || undefined,
          demand_res_types: d.demand_res_types,
          cvm: d.cvm
            ? {
                res_mode: d.cvm.res_mode,
                device_type: d.cvm.device_type,
                os: d.cvm.os,
                cpu_core: d.cvm.cpu_core,
                memory: d.cvm.memory,
              }
            : undefined,
          cbs: d.cbs
            ? {
                disk_type: d.cbs.disk_type,
                disk_io: d.cbs.disk_io,
                disk_size: d.cbs.disk_size,
              }
            : undefined,
        };

        // add 只传 updated_info；delete 只传 original_info；adjust 两个都传
        if (ticketType === 'add') {
          return { updated_info: updatedInfo };
        }
        if (ticketType === 'delete') {
          return { demand_id: d.demand_id || undefined, original_info: d.original_info ?? null };
        }
        return {
          demand_id: d.demand_id || undefined,
          original_info: d.original_info ?? null,
          updated_info: updatedInfo,
        };
      });
    };

    const handleClick = async () => {
      try {
        isLoading.value = true;
        await validate();
        await resourcePlanStore.overwriteBizPlan(props.modelValue.bk_biz_id, props.ticketId, {
          ticket_type: props.modelValue.ticket_type,
          demand_class: props.modelValue.demand_class,
          demands: buildDemandPayload(),
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
