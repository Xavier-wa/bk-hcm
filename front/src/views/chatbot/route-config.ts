import type { RouteRecordRaw } from 'vue-router';

import { MENU_BUSINESS, MENU_BUSINESS_CHATBOT } from '@/constants/menu-symbol';
import Meta from '@/router/meta';

/** 业务视角 AI 首页（完整路径 /business/chatbot/:sessionCode?） */
const chatbotBiz: RouteRecordRaw[] = [
  {
    name: MENU_BUSINESS_CHATBOT,
    path: 'chatbot/:sessionCode?',
    component: () => import('./index.vue'),
    meta: {
      ...new Meta({
        owner: MENU_BUSINESS,
        title: '首页',
        activeKey: MENU_BUSINESS_CHATBOT,
        icon: 'hcm-icon bkhcm-icon-home-fill',
        // 平台-智能体助手：无该权限时不展示「首页」入口（见 store/common.ts agent_assistant）
        checkAuth: 'agent_assistant',
        layout: {
          // 复用全局面包屑展示「首页」，首页无需返回箭头
          breadcrumbs: {
            show: true,
            back: false,
          },
        },
      }),
    },
  },
];

export { chatbotBiz };
