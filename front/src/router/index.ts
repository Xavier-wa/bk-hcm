import {
  createRouter,
  RouteRecordRaw,
  NavigationGuardNext,
  createWebHashHistory,
  RouteLocationNormalized,
} from 'vue-router';
import {
  MENU_BUSINESS,
  MENU_BUSINESS_CHATBOT,
  MENU_SERVICE,
  MENU_PLATFORM_MANAGEMENT,
  MENU_ROLLING_SERVER_MANAGEMENT,
} from '@/constants/menu-symbol';
import { businessViews, serviceViews, platformManagementViews } from '@/views';
import common from './module/common';
import resource from './module/resource';
import resourceInside from './module/resource-inside';
import resourcePlan from './module/resource-plan';
// import service from './module/service';
import serviceInside from './module/service-inside';
// import business from './module/business';
import scheme from './module/scheme';
import bill from './module/bill';
import { useCommonStore } from '@/store';
import { useVerify } from '@/hooks';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import { localStorageActions } from '@/common/util';
import { isArray, isRegExp, isString } from 'lodash';

const routes: RouteRecordRaw[] = [
  ...common,
  ...resource,
  ...resourceInside,
  ...resourcePlan,
  // ...service,
  ...serviceInside,
  ...scheme,
  ...bill,
  {
    name: MENU_PLATFORM_MANAGEMENT,
    path: '/platform',
    redirect: { name: MENU_ROLLING_SERVER_MANAGEMENT },
    children: platformManagementViews,
  },
  {
    path: '/',
    redirect: '/business',
  },
  {
    path: '/403',
    redirect: '/403',
  },
  {
    name: MENU_BUSINESS,
    path: '/business',
    children: businessViews,
  },
  {
    name: MENU_SERVICE,
    path: '/service',
    children: serviceViews,
  },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
});

// 进入目标页面
const toCurrentPage = (
  authVerifyData: {
    permissionAction: Record<string, boolean>;
    urlParams: {
      system_id: string;
      actions: Array<{
        id: string;
        name: string;
        related_resource_types: Array<any>;
      }>;
    };
  },
  currentFindAuthData: {
    action: string;
    id: string;
    path: string;
    type: string;
  },
  next: NavigationGuardNext,
  to?: RouteLocationNormalized,
) => {
  // 是否需要鉴权
  const needAuth = !!currentFindAuthData?.id;
  // 是否有权限
  const hasAuth = !!authVerifyData?.permissionAction?.[currentFindAuthData?.id];

  if (!needAuth) {
    if (to?.name === '403') next(!!authVerifyData?.permissionAction?.biz_access ? { path: '/' } : undefined);
    else next();
    return;
  }

  if (hasAuth) next();
  else next({ name: '403', params: { id: currentFindAuthData?.id } });
};

router.beforeEach((to: RouteLocationNormalized, from: RouteLocationNormalized, next: NavigationGuardNext) => {
  const commonStore = useCommonStore();
  const { pageAuthData, authVerifyData } = commonStore; // 所有需要检验的查看权限数据
  // 命中 to.path 的鉴权项可能存在多个（如业务访问 biz_access 的 /^\/business/ 与业务 chatbot
  // biz_agent_assistant 的 /^\/business\/chatbot/ 会同时匹配 /business/chatbot），需取「路径最具体」的一项，
  // 否则会被更宽泛的规则覆盖，导致细粒度权限（如 chatbot 入口）鉴权失效
  const matchAuthPath = (path: unknown) => {
    if (isString(path)) return path === to.path;
    if (isArray(path)) return path.includes(to.path);
    if (isRegExp(path)) return path.test(to.path);
    return false;
  };
  const getAuthPathSpecificity = (path: unknown) => {
    if (isString(path)) return path.length;
    if (isRegExp(path)) return path.source.length;
    if (isArray(path)) return path.reduce((max: number, p) => Math.max(max, String(p).length), 0);
    return 0;
  };
  const currentFindAuthData = pageAuthData
    .filter((e: any) => matchAuthPath(e.path))
    .sort((a: any, b: any) => getAuthPathSpecificity(b.path) - getAuthPathSpecificity(a.path))[0];
  if (from.path === '/') {
    // 刷新或者首次进入请求权限接口
    const { getAuthVerifyData } = useVerify(); // 权限中心权限
    // 冷启动时业务尚未选中，先从 URL / localStorage 解析业务 id，
    // 注入到业务维度鉴权项，使业务视角 chatbot 权限(biz_agent_assistant)可被正确鉴权
    const resolvedBizId =
      Number(to.query[GLOBAL_BIZS_KEY] || localStorageActions.get(GLOBAL_BIZS_KEY, (value) => value)) || 0;
    const verifyAuthData = pageAuthData.map((item: any) =>
      Object.prototype.hasOwnProperty.call(item, 'bk_biz_id') ? { ...item, bk_biz_id: resolvedBizId } : item,
    );
    getAuthVerifyData(verifyAuthData).then(() => {
      const { authVerifyData } = commonStore;
      // 业务视角 chatbot 权限：有权限默认进 chatbot 首页，否则按常规鉴权回退（资源管理主机页）
      const hasBizChatbotAccess = !!authVerifyData?.permissionAction?.biz_agent_assistant;
      if (hasBizChatbotAccess && (to.path === '/' || to.path === '/business' || to.path === '/business/host')) {
        next({ name: MENU_BUSINESS_CHATBOT, query: to.query });
        return;
      }
      toCurrentPage(authVerifyData, currentFindAuthData as any, next, to);
    });
  } else if (['/scheme/recommendation', '/scheme/deployment/list'].includes(to.path)) {
    next();
  } else {
    toCurrentPage(authVerifyData, currentFindAuthData as any, next);
  }
});

export default router;
