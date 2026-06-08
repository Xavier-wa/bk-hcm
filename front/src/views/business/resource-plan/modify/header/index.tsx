import { defineComponent } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';
import routerAction from '@/router/utils/action';
import { MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS } from '@/constants/menu-symbol';
import cssModule from './index.module.scss';

export default defineComponent({
  setup() {
    const { t } = useI18n();
    const route = useRoute();

    const handleClick = () => {
      routerAction.redirect({
        name: MENU_BUSINESS_TICKET_RESOURCE_PLAN_DETAILS,
        query: { ...route.query },
      });
    };

    return () => (
      <span class={cssModule.home}>
        <i class={`${cssModule.arrow} hcm-icon bkhcm-icon-arrows--left-line`} onClick={handleClick}></i>
        {t('修改资源预测')}
      </span>
    );
  },
});
