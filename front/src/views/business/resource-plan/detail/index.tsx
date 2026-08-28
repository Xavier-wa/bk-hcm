// 资源管理 (业务)下 资源预测 详情
import { defineComponent, ref } from 'vue';
import Table from '@/components/resource-plan/resource-manage/detail/list/index';
import Basic from '@/components/resource-plan/resource-manage/detail/basic/index';
import DetailHeader from '@/views/resource/resource-manage/common/header/detail-header';
import cssModule from './index.module.scss';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { Button } from 'bkui-vue';
import routerAction from '@/router/utils/action';
import { MENU_BUSINESS_RESOURCE_PLAN_CVM_ADJUST } from '@/constants/menu-symbol';

export default defineComponent({
  setup() {
    const route = useRoute();
    const { t } = useI18n();

    const currentBusinessId = ref<number>();
    const updateCurrentBusinessId = (id: number) => {
      currentBusinessId.value = id;
    };

    const handleAdjust = () => {
      const searchModel = JSON.parse(route.query.searchModel as string);
      routerAction.redirect(
        {
          name: MENU_BUSINESS_RESOURCE_PLAN_CVM_ADJUST,
          query: {
            planIds: route.query.demandId,
            start: searchModel.expect_time_range.start,
            end: searchModel.expect_time_range.end,
          },
        },
        { history: true },
      );
    };

    return () => (
      <>
        <DetailHeader>{t('资源预测详情')}</DetailHeader>
        <section class={cssModule['resource-forecast-details-section']}>
          <Button class={cssModule.button} onClick={handleAdjust}>
            {t('调整预测')}
          </Button>
          <Basic class={cssModule['mb-16']} isBiz={true} updateCurrentBusinessId={updateCurrentBusinessId}></Basic>
          <Table isBiz={true} currentBusinessId={currentBusinessId.value}></Table>
        </section>
      </>
    );
  },
});
