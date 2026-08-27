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
  MENU_BUSINESS_HOST_MANAGEMENT,
  MENU_SERVICE,
  MENU_PLATFORM_MANAGEMENT,
  MENU_ROLLING_SERVER_MANAGEMENT,
} from '@/constants/menu-symbol';
import { businessViews, serviceViews, platformManagementViews, billViews } from '@/views';
import common from './module/common';
import resource from './module/resource';
import resourceInside from './module/resource-inside';
import resourcePlan from './module/resource-plan';
// import service from './module/service';
import serviceInside from './module/service-inside';
// import business from './module/business';
import scheme from './module/scheme';
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
  ...billViews,
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

// 鉴权结果未就绪时一律按无权限处理（api.md §4）
const getPermissionAction = (auth: { permissionAction?: Record<string, boolean> } | null): Record<string, boolean> =>
  auth?.permissionAction ?? {};

// 冷启动鉴权门闩。锁在「请求已发起」而不是「结果已就绪」：重定向链上 from 仍是 START_LOCATION，
// 会反复进入拉取分支，共用同一个 promise 才不会重复打 verify。
// 失败时置回 null，让下一次导航可以重试，否则 authVerifyData 永远为空，整场会话都被判为无权限。
let authVerifyPromise: Promise<unknown> | null = null;
const ensureAuthVerified = (verifyAuthData: any[]) => {
  if (!authVerifyPromise) {
    const { getAuthVerifyData } = useVerify(); // 权限中心权限
    authVerifyPromise = getAuthVerifyData(verifyAuthData).catch((err) => {
      console.error('auth verify failed', err);
      authVerifyPromise = null;
    });
  }
  return authVerifyPromise;
};

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
  const permissionAction = getPermissionAction(authVerifyData);
  const hasBizAccess = !!permissionAction.biz_access;
  // 去掉末尾斜杠：vue-router 默认 strict: false，/business/ 也会匹配到 /business 这条记录
  const toPath = to?.path && to.path !== '/' ? to.path.replace(/\/+$/, '') : to?.path;
  // chatbot 比 biz_access 更具体，命中 agent_assistant 且已授权时会提前放行，必须先拦业务访问
  if (toPath?.startsWith('/business') && !hasBizAccess) {
    next({ name: '403', params: { id: 'biz_access' }, query: to?.query });
    return;
  }

  // /business 自身没有页面，落地页按「平台-智能体助手」决定。
  // 放在守卫里而不是路由 redirect / beforeEnter：redirect 在匹配阶段执行，此时鉴权还没回；
  // beforeEnter 在业务内跳转时会因为路由记录复用被跳过，会停在无组件的 /business 上
  if (toPath === '/business') {
    next({
      name: permissionAction.agent_assistant ? MENU_BUSINESS_CHATBOT : MENU_BUSINESS_HOST_MANAGEMENT,
      query: to.query,
    });
    return;
  }

  // 是否需要鉴权
  const needAuth = !!currentFindAuthData?.id;
  // 是否有权限
  const hasAuth = !!permissionAction[currentFindAuthData?.id];

  if (!needAuth) {
    if (to?.name === '403') {
      // 申请页只在「确实缺这个权限」时停留；已有该权限说明是失效链接，才弹回首页
      const applyId = to.params?.id as string;
      const needApply = !!applyId && !permissionAction[applyId];
      next(!needApply && hasBizAccess ? { path: '/', query: to.query } : undefined);
    } else next();
    return;
  }

  if (hasAuth) {
    next();
    return;
  }

  next({ name: '403', params: { id: currentFindAuthData?.id }, query: to?.query });
};

router.beforeEach((to: RouteLocationNormalized, from: RouteLocationNormalized, next: NavigationGuardNext) => {
  const commonStore = useCommonStore();
  const { pageAuthData, authVerifyData } = commonStore; // 所有需要检验的查看权限数据
  // 命中 to.path 的鉴权项可能存在多个（如业务访问 biz_access 的 /^\/business/ 与平台
  // agent_assistant 的 /^\/business\/chatbot/ 会同时匹配 /business/chatbot），需取「路径最具体」的一项，
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
  // 用「鉴权结果是否就绪」而不是「是否冷启动（from.path === '/'）」分流：verify 失败时 authVerifyData 仍为空，
  // 若只在冷启动拉取，之后所有站内跳转都会读到空权限、被判为无权限，直到刷新才恢复
  if (!authVerifyData?.permissionAction) {
    // 首次进入 / 刷新 / 上次 verify 失败：先把权限拉回来，落地页与无权限去向都交给 toCurrentPage 决定。
    // 不要在这里直接 next 到别的路由：重定向后 from 仍是 START_LOCATION，会再次进入本分支。
    // 冷启动时业务尚未选中，先从 URL / localStorage 解析业务 id，注入到带 bk_biz_id 的鉴权项
    const resolvedBizId =
      Number(to.query[GLOBAL_BIZS_KEY] || localStorageActions.get(GLOBAL_BIZS_KEY, (value) => value)) || 0;
    const verifyAuthData = pageAuthData.map((item: any) =>
      Object.prototype.hasOwnProperty.call(item, 'bk_biz_id') ? { ...item, bk_biz_id: resolvedBizId } : item,
    );
    // 鉴权失败也要 next，否则导航一直挂起、页面空白（api.md：verify 失败按无权限处理）
    ensureAuthVerified(verifyAuthData).then(() => {
      const { authVerifyData: latestAuth } = commonStore;
      toCurrentPage(latestAuth, currentFindAuthData as any, next, to);
    });
    return;
  }

  if (['/scheme/recommendation', '/scheme/deployment/list'].includes(to.path)) {
    next();
    return;
  }

  // 始终传入 to，保证冷启动与站内跳转对 agent_assistant 无权限去向一致，且能保留 bizs query
  toCurrentPage(authVerifyData, currentFindAuthData as any, next, to);
});

export default router;
