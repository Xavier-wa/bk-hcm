import type { RouteRecordRaw } from 'vue-router';
import Meta from '@/router/meta';
import { MENU_BILL_PREPAID, MENU_BILL_PREPAID_DETAIL } from '@/constants/menu-symbol';

export const prepaidBillMenuRoutes: RouteRecordRaw[] = [
  {
    path: '/bill/prepaid',
    name: MENU_BILL_PREPAID,
    component: () => import('@/views/prepaid-bill/index.vue'),
    meta: {
      ...new Meta({
        title: '预付费管理',
        activeKey: MENU_BILL_PREPAID,
        isShowBreadcrumb: true,
        icon: 'hcm-icon bkhcm-icon-bill-manage',
        checkAuth: 'main_account_find',
        layout: {
          breadcrumbs: { show: true, back: false },
        },
      }),
    },
  },
];

export default [
  ...prepaidBillMenuRoutes,
  {
    path: '/bill/prepaid/detail/:id',
    name: MENU_BILL_PREPAID_DETAIL,
    component: () => import('@/views/prepaid-bill/details/index.vue'),
    meta: {
      ...new Meta({
        title: '预付费详情',
        activeKey: MENU_BILL_PREPAID,
        notMenu: true,
        isShowBreadcrumb: true,
        checkAuth: 'main_account_find',
        layout: {
          breadcrumbs: { show: true, back: true },
        },
      }),
    },
  },
] as RouteRecordRaw[];
