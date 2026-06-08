import { defineComponent, ref, provide, watch, nextTick, computed, onBeforeMount } from 'vue';
import { useRoute } from 'vue-router';
import { Message } from 'bkui-vue';
import { useResourcePlanStore } from '@/store';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import cssModule from './index.module.scss';
import Header from './header';
import Basic from './basic';
import List from './list';
import Memo from './memo';
import Button from './button';
import Add from '@/components/resource-plan/add';
import { mapTicketDetailToPlanTicket } from './utils';
import type { IPlanTicket, IPlanTicketDemand } from '@/typings/resourcePlan';
import { GLOBAL_BIZS_KEY } from '@/common/constant';

export default defineComponent({
  setup() {
    const route = useRoute();
    const { getBizsId } = useWhereAmI();
    const resourcePlanStore = useResourcePlanStore();

    const ticketId = computed(() => route.query.id as string);
    const bizId = computed(() => Number(route.query?.[GLOBAL_BIZS_KEY]) || getBizsId());
    const isLoading = ref(true);

    const basicRef = ref();
    const listRef = ref();
    const memoRef = ref();
    const isShowAdd = ref(false);
    const initDemand = ref<IPlanTicketDemand>();
    const planTicket = ref<IPlanTicket>({
      bk_biz_id: bizId.value,
      demand_class: 'CVM',
      remark: '',
      demands: [],
    });
    const initAddParams = ref({});

    const handleShowAdd = () => {
      initDemand.value = undefined;
      isShowAdd.value = true;
    };

    const handleShowModify = (data: IPlanTicketDemand) => {
      initDemand.value = data;
      isShowAdd.value = true;
    };

    const validate = () => {
      return Promise.all([basicRef.value.validate(), listRef.value.validate(), memoRef.value.validate()]);
    };

    // 拉取详情并回填到表单
    const loadTicket = async () => {
      if (!ticketId.value) {
        Message({ message: '缺少单据 ID', theme: 'error' });
        isLoading.value = false;
        return;
      }
      try {
        isLoading.value = true;
        const res = await resourcePlanStore.getBizResourcesTicketsById(bizId.value, ticketId.value);
        planTicket.value = mapTicketDetailToPlanTicket(res.data, bizId.value);
      } catch (error: any) {
        Message({ message: error.message || error, theme: 'error' });
      } finally {
        isLoading.value = false;
      }
    };

    watch(
      () => route.query.action,
      (action) => {
        if (action === 'add') {
          initAddParams.value = JSON.parse(decodeURIComponent(route.query.payload as string));
          nextTick(() => {
            handleShowAdd();
          });
        }
      },
      { immediate: true },
    );

    onBeforeMount(loadTicket);

    provide('validate', validate);

    return () => (
      <bk-loading loading={isLoading.value}>
        <Header />
        <section class={cssModule.home}>
          <Basic v-model={planTicket.value} ref={basicRef}></Basic>
          <List
            class={cssModule['mt-16']}
            ref={listRef}
            v-model={planTicket.value}
            onShow-add={handleShowAdd}
            onShow-modify={handleShowModify}
          />
          <Memo class={cssModule['mt-16']} ref={memoRef} v-model={planTicket.value}></Memo>
          <Button class={cssModule['mt-16']} v-model={planTicket.value} ticketId={ticketId.value} />
        </section>
        <Add
          v-model:isShow={isShowAdd.value}
          v-model={planTicket.value}
          initDemand={initDemand.value}
          initAddParams={initAddParams.value}
          currentGlobalBusinessId={planTicket.value.bk_biz_id}
        />
      </bk-loading>
    );
  },
});
