import { Ref, computed } from 'vue';
import { useRoute } from 'vue-router';
import { useAccountStore } from '@/store';
import { getQueryStringParams, localStorageActions } from '@/common/util';
import { GLOBAL_BIZS_KEY } from '@/common/constant';

export const useWhereAmI = (): {
  whereAmI: Ref<Senarios>;
  isResourcePage: boolean;
  isBusinessPage: boolean;
  isServicePage: boolean;
  isSchemePage: boolean;
  isZiyanscr: boolean;
  getBusinessApiPath: (bizId?: number | string) => string;
  getBizsId: () => number;
} => {
  const route = useRoute();
  const senario = computed(() => {
    if (!route) return;
    if (/^\/resource\/.+$/.test(route?.path)) return Senarios.resource;
    if (/^\/business\/.+$/.test(route.path)) return Senarios.business;
    if (/^\/service\/.+$/.test(route.path)) return Senarios.service;
    if (/^\/scheme\/.+$/.test(route.path)) return Senarios.scheme;
    if (/^\/ziyanscr\/.+$/.test(route.path)) return Senarios.ziyanscr;
    if (/^\/bill\/.+$/.test(route.path)) return Senarios.bill;
    if (/^\/platform\/.+$/.test(route.path)) return Senarios.platform;
    if (/^\/403\/.+$/.test(route.path)) return Senarios.unauthorized;
    return Senarios.unknown;
  });

  const getBizsId = () => {
    const { bizs } = useAccountStore();
    return Number(
      bizs || getQueryStringParams(GLOBAL_BIZS_KEY) || localStorageActions.get(GLOBAL_BIZS_KEY, (value) => value),
    );
  };

  /**
   * 页面数据自带业务归属时（如单据详情页的单据所属业务），必须传入 bizId。
   * 全局业务由业务选择器异步初始化，页面挂载阶段读取会因时序竞态拿到其它业务，导致跨业务操作。
   * 传入的 bizId 取不到有效值时（如 URL 未携带对应参数）回退到全局业务，避免拼出非法路径。
   *
   * The bizId parameter specifies which business the API path belongs to, defaults to the global business.
   * @returns 业务下需要拼接的 API 路径
   */
  const getBusinessApiPath = (bizId?: number | string) => {
    if (senario.value !== Senarios.business) return '';
    const pageBizId = Number(bizId);
    const validBizId = Number.isFinite(pageBizId) && pageBizId > 0 ? pageBizId : getBizsId();
    return `bizs/${validBizId}/`;
  };

  return {
    whereAmI: senario,
    isResourcePage: senario.value === Senarios.resource,
    isBusinessPage: senario.value === Senarios.business,
    isServicePage: senario.value === Senarios.service,
    isSchemePage: senario.value === Senarios.scheme,
    isZiyanscr: senario.value === Senarios.ziyanscr,
    getBusinessApiPath,
    getBizsId,
  };
};

export enum Senarios {
  business = 'business',
  resource = 'resource',
  service = 'service',
  scheme = 'scheme',
  ziyanscr = 'ziyanscr',
  bill = 'bill',
  platform = 'platform',
  unknown = 'unknown',
  unauthorized = 'unauthorized',
}
