import { ref } from 'vue';
import { defineStore } from 'pinia';
import http from '@/http';
import { IPageQuery, IListResData, IQueryResData } from '@/typings';
import { enableCount, resolveBizApiPath } from '@/utils/search';

export interface ICpuCoreSummary {
  total_core: number;
  delivered_core: number;
  available_quota: number;
}

export interface IQuotaOffset {
  bk_biz_id: number;
  type: 'increase' | 'decrease';
  offset: number;
  memo: string;
}

export interface IDissolveProject {
  id: number;
  memo: string;
}

export interface IDissolveProjectCycle {
  start: string;
  end: string;
  default: boolean;
  projects: IDissolveProject[];
}

export interface IDissolveConfig {
  host_apply_time: string;
  approval_limit: number;
  quota_coefficient: number;
  quota_offsets: IQuotaOffset[];
  dissolve_projects: IDissolveProjectCycle[];
}

export interface IDissolveOverview {
  bk_biz_id: number;
  bk_biz_name?: string; // 前端映射字段
  progress: string | number; // API 返回字符串，前端映射为数字
  progress_str?: string; // 进度百分比字符串表示
  origin_host_count?: number; // API 原始字段
  origin_cpu_core?: number; // API 原始字段
  current_host_count?: number; // API 原始字段
  current_cpu_core?: number; // API 原始字段
  delivered_cpu_core?: number; // API 原始字段
  origin?: { host_count: number; cpu_count: number }; // 前端映射字段
  delivered_core?: number; // 前端映射字段
  plan?: { host_count: number; cpu_count: number }; // 前端映射字段
}

export interface IDissolveDetail {
  id: string;
  asset_id: string;
  inner_ip: string;
  module: string;
  status: 'complete' | 'incomplete';
  project_id: number;
  project_name: string;
  region: string;
  bk_biz_id: number;
  group_id: number;
  operators: string[];
  cpu_core: number;
  device_type: string;
  expect_abolish_time?: string;
}

export interface IDissolveOverviewListParams {
  expect_abolish_times?: string[];
  project_ids?: number[];
  group_ids?: number[];
  bk_biz_ids?: number[];
  operators?: string[];
  regions?: string[];
}

export interface IDissolveDetailListParams {
  bk_biz_ids?: number[];
  expect_abolish_times?: string[];
  project_ids?: number[];
  group_ids?: number[];
  operators?: string[];
  modules?: string[];
  inner_ips?: string[];
  asset_ids?: string[];
  status?: string;
  page: IPageQuery;
}

export interface IOptionItem {
  value: string;
  label: string;
}

export interface IDissolveProjectType {
  id: number;
  projectName: string;
  projectType: string;
}

export const useDissolveQuotaStore = defineStore('dissolve-quota', () => {
  const cpuCoreSummaryLoading = ref(false);
  const dissolveConfigLoading = ref(false);
  const upsertDissolveConfigLoading = ref(false);
  const overviewListLoading = ref(false);
  const detailListLoading = ref(false);
  const syncLoading = ref(false);

  const getCpuCoreSummary = async (bizId: number, params: { bk_biz_id?: number } = {}) => {
    cpuCoreSummaryLoading.value = true;
    try {
      const api = `/api/v1/woa/${resolveBizApiPath(bizId)}dissolve/cpu_core/summary`;
      const res: IQueryResData<ICpuCoreSummary> = await http.post(api, params);
      return res?.data;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      cpuCoreSummaryLoading.value = false;
    }
  };

  const getDissolveConfig = async () => {
    dissolveConfigLoading.value = true;
    try {
      const api = '/api/v1/woa/dissolve/config';
      const res: IQueryResData<IDissolveConfig> = await http.get(api);
      return res?.data;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      dissolveConfigLoading.value = false;
    }
  };

  const upsertDissolveConfig = async (params: IDissolveConfig) => {
    upsertDissolveConfigLoading.value = true;
    try {
      const api = '/api/v1/woa/dissolve/config/upsert';
      const res: IQueryResData<null> = await http.put(api, params);
      return res?.data;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      upsertDissolveConfigLoading.value = false;
    }
  };

  const getOverviewList = async (params: IDissolveOverviewListParams) => {
    overviewListLoading.value = true;
    try {
      const res: IQueryResData<{ items: IDissolveOverview[] }> = await http.post(
        '/api/v1/woa/dissolve/table/list',
        params,
      );
      return res?.data;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      overviewListLoading.value = false;
    }
  };

  const getDetailList = async (params: IDissolveDetailListParams) => {
    detailListLoading.value = true;
    const api = '/api/v1/woa/dissolve/host/detail/list';
    try {
      const [listRes, countRes] = await Promise.all<
        [Promise<IListResData<IDissolveDetail[]>>, Promise<IListResData<IDissolveDetail[]>>]
      >([http.post(api, enableCount(params, false)), http.post(api, enableCount(params, true))]);
      const [{ details: list = [] }, { count = 0 }] = [listRes?.data ?? {}, countRes?.data ?? {}];
      return { list: list || [], count };
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      detailListLoading.value = false;
    }
  };

  const syncDissolve = async () => {
    syncLoading.value = true;
    try {
      const res: IQueryResData<null> = await http.post('/api/v1/woa/dissolve/recycled_host/sync', {});
      return res?.data;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      syncLoading.value = false;
    }
  };

  const getProjectTypes = async () => {
    try {
      const res: IQueryResData<IDissolveProjectType[]> = await http.get('/api/v1/woa/dissolve/projects');
      return (res?.data || []).map((item) => ({
        value: item.id,
        label: item.projectName,
      }));
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    }
  };

  const getExpectAbolishTimeList = async () => {
    try {
      const res: IQueryResData<{ expect_abolish_times: string[] }> = await http.post(
        '/api/v1/woa/dissolve/expect_abolish_time/list',
        {},
      );
      return res?.data?.expect_abolish_times || [];
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    }
  };

  const getExpectAbolishTimeOptions = async (): Promise<Record<string, string>> => {
    try {
      const times = await getExpectAbolishTimeList();
      return times.reduce((acc: Record<string, string>, time: string) => ({ ...acc, [time]: time }), {});
    } catch {
      return {};
    }
  };

  const getIdcNames = async (params: { regions?: string[]; zones?: string[] }) => {
    try {
      const res: IQueryResData<IOptionItem[]> = await http.post('/api/v1/woa/dissolve/idc_names/list', params);
      return res?.data || [];
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    }
  };

  return {
    cpuCoreSummaryLoading,
    dissolveConfigLoading,
    upsertDissolveConfigLoading,
    overviewListLoading,
    detailListLoading,
    syncLoading,
    getCpuCoreSummary,
    getDissolveConfig,
    upsertDissolveConfig,
    getOverviewList,
    getDetailList,
    syncDissolve,
    getProjectTypes,
    getExpectAbolishTimeList,
    getExpectAbolishTimeOptions,
    getIdcNames,
  };
});
