import { ref } from 'vue';
import { defineStore } from 'pinia';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import { useResourcePlanStore } from '@/store';
import http from '@/http';
import type { IExceptTimeRange } from '@/typings/plan';
import type { IDeviceType, IZone } from '@/typings/resourcePlan';

/** 调整接口 adjust_type（MR !3290） */
export type AdjustApiType = 'update' | 'delay' | 'add';

/** 调整详情体 */
export interface IAdjustInfoPayload {
  obs_project: string;
  expect_time: string;
  return_plan_time?: string;
  region_id: string;
  zone_id?: string;
  demand_source?: string;
  remark?: string;
  demand_res_types: ('CVM' | 'CBS')[];
  cvm?: {
    res_mode: string;
    device_type: string;
    os: number;
    cpu_core: number;
    memory: number;
  };
  cbs?: {
    disk_type: string;
    disk_io: number;
    disk_size: number;
  };
}

/** 调整单项 */
export interface IAdjustItemPayload {
  demand_id?: string;
  adjust_type: AdjustApiType;
  demand_source?: string;
  original_info?: IAdjustInfoPayload;
  updated_info?: IAdjustInfoPayload;
  expect_time?: string;
}

export interface ICvmAdjustListParams {
  demand_ids: string[];
  expect_time_range: { start: string; end: string };
  page?: { count: boolean; start: number; limit: number };
}

export interface ICvmAdjustSubmitParams {
  /** 预测需求类型；adjusts 全为 add 时必填 */
  demand_class?: string;
  adjusts: IAdjustItemPayload[];
}

export interface ICvmAdjustSubmitResult {
  id: string;
}

export const useCvmAdjustStore = defineStore('resource-plan-cvm-adjust', () => {
  const { getBusinessApiPath } = useWhereAmI();

  const listLoading = ref(false);
  const submitLoading = ref(false);

  const listDemands = async (params: ICvmAdjustListParams) => {
    listLoading.value = true;
    try {
      const res = await http.post(`/api/v1/woa/${getBusinessApiPath()}plans/resources/demands/list`, {
        demand_ids: params.demand_ids,
        expect_time_range: params.expect_time_range,
        page: params.page ?? { count: false, start: 0, limit: 500 },
      });
      return res?.data?.details ?? [];
    } finally {
      listLoading.value = false;
    }
  };

  const submitAdjust = async (params: ICvmAdjustSubmitParams): Promise<ICvmAdjustSubmitResult> => {
    submitLoading.value = true;
    try {
      const res = await http.post(`/api/v1/woa/${getBusinessApiPath()}plans/resources/demands/adjust`, params);
      return res?.data ?? { id: '' };
    } finally {
      submitLoading.value = false;
    }
  };

  /** 同一日期的可用范围是固定的，缓存请求避免逐行重复拉取 */
  const availableTimeCache = new Map<string, Promise<IExceptTimeRange>>();

  /** 查询期望到货时间对应的可用周 / 可用月范围 */
  const getAvailableTime = (expectTime: string): Promise<IExceptTimeRange> => {
    const cached = availableTimeCache.get(expectTime);
    if (cached) return cached;

    const request = http
      .post('/api/v1/woa/plans/demands/available_times/get', { expect_time: expectTime })
      .then((res: any) => res?.data)
      .catch((err: unknown) => {
        availableTimeCache.delete(expectTime);
        throw err;
      });
    availableTimeCache.set(expectTime, request);
    return request;
  };

  const resourcePlanStore = useResourcePlanStore();

  // 整表逐行 onMounted 拉可用区/机型，同城市、同规格的行会重复请求；缓存 Promise 顺带合并并发
  const zonesCache = new Map<string, Promise<IZone[]>>();
  const deviceTypesCache = new Map<string, Promise<IDeviceType[]>>();

  /** 按城市查可用区，同一城市只请求一次 */
  const getZones = (regionId: string): Promise<IZone[]> => {
    const cached = zonesCache.get(regionId);
    if (cached) return cached;

    const request = resourcePlanStore
      .getZones([regionId])
      .then((res: any) => res?.data?.details ?? [])
      .catch((err: unknown) => {
        zonesCache.delete(regionId);
        throw err;
      });
    zonesCache.set(regionId, request);
    return request;
  };

  /** 按机型规格查机型，同一规格只请求一次 */
  const getDeviceTypes = (deviceClass: string): Promise<IDeviceType[]> => {
    const cached = deviceTypesCache.get(deviceClass);
    if (cached) return cached;

    const request = resourcePlanStore
      .getDeviceTypes([deviceClass])
      .then((res: any) => res?.data?.details ?? [])
      .catch((err: unknown) => {
        deviceTypesCache.delete(deviceClass);
        throw err;
      });
    deviceTypesCache.set(deviceClass, request);
    return request;
  };

  return {
    listLoading,
    submitLoading,
    listDemands,
    submitAdjust,
    getAvailableTime,
    getZones,
    getDeviceTypes,
  };
});
