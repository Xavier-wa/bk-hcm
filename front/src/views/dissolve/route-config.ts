import type { RouteRecordRaw } from 'vue-router';
import Meta from '@/router/meta';
import { MENU_SERVICE_DISSOLVE, MENU_SERVICE } from '@/constants/menu-symbol';

export default [
  {
    path: 'dissolve',
    name: MENU_SERVICE_DISSOLVE,
    component: () => import('@/views/dissolve/index.vue'),
    meta: {
      ...new Meta({
        owner: MENU_SERVICE,
        title: '机房裁撤',
        activeKey: MENU_SERVICE_DISSOLVE,
        // checkAuth: 'service_resource_dissolve_find',
        layout: {
          breadcrumbs: { show: true, back: false },
        },
        menu: {
          relative: MENU_SERVICE_DISSOLVE,
        },
        icon: 'hcm-icon bkhcm-icon-dissolve',
      }),
    },
  },
] as RouteRecordRaw[];
