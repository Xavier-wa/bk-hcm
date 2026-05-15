import type { RouteRecordRaw } from 'vue-router';
import Meta from '@/router/meta';
import { MENU_INDEX } from '@/constants/menu-symbol';

export default [
  {
    name: MENU_INDEX,
    path: '/chatbot/:sessionCode?',
    component: () => import('./chatbot/index.vue'),
    meta: {
      ...new Meta({
        menu: {
          i18n: '首页',
        },
        layout: {
          breadcrumbs: {
            show: false,
          },
        },
      }),
    },
  },
] as RouteRecordRaw[];
