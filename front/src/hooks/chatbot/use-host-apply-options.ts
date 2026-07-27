import { VendorEnum } from '@/common/constant';
import { QueryRuleOPEnum, type QueryFilterType } from '@/typings';
import { getRegions } from '@/api/host/config-management';
import { getImages, getDiskTypes } from '@/api/host/cvm';
import { useCvmDeviceStore } from '@/store/cvm/device';
import { useConfigRequirementStore } from '@/store/config/requirement';
import { RES_ASSIGN_TYPE } from '@/components/device-type-selector/constants';
import { formatDeviceTypeDisplay, seedDeviceMetaCache } from './host-apply-device-meta';

// 主机申领 C「调整配置」弹窗下拉选项：复用 ziyanScr 数据/接口层，统一归一化为 { id, name }，
// 供 hcm-form-list 以 :id-key="'id'" :display-key="'name'" 消费（见 api.md / coding.md）。
// vendor 本期固定 ZIYAN，函数内预留 vendor 分支位，其余云返回空数组待后续实现。

export interface HostApplyOption {
  id: string | number;
  name: string;
}

const isZiyan = (vendor: VendorEnum): boolean => vendor === VendorEnum.ZIYAN;

// 地域：仅取 qcloud（本期 chatbot 不含 idc）
export const getRegionOptions = async (vendor: VendorEnum = VendorEnum.ZIYAN): Promise<HostApplyOption[]> => {
  if (!isZiyan(vendor)) return [];
  const data = await getRegions('qcloud', {});
  return (data?.info ?? []).map((item: { region: string; region_cn?: string }) => ({
    id: item.region,
    name: item.region_cn || item.region,
  }));
};

// 机型：使用单纯的机型列表接口（机型元数据目录，非计费/预测相关接口），按 vendor + 地域过滤
export const getDeviceTypeOptions = async (
  opts: { vendor?: VendorEnum; region?: string } = {},
): Promise<HostApplyOption[]> => {
  const { vendor = VendorEnum.ZIYAN, region } = opts;
  if (!isZiyan(vendor)) return [];
  const filter: QueryFilterType = { op: 'and', rules: [{ field: 'vendor', op: QueryRuleOPEnum.EQ, value: vendor }] };
  if (region) filter.rules.push({ field: 'region', op: QueryRuleOPEnum.EQ, value: region });
  const { getDeviceTypeFullList } = useCvmDeviceStore();
  const { list } = await getDeviceTypeFullList({ filter });
  // 按 device_type 去重；name 附带机型族/CPU/内存
  const seen = new Set<string>();
  const options: HostApplyOption[] = [];
  (list ?? []).forEach((item) => {
    if (item.device_type && !seen.has(item.device_type)) {
      seen.add(item.device_type);
      const meta = {
        device_family: item.device_family,
        cpu_core: item.cpu_core,
        memory: item.memory,
      };
      seedDeviceMetaCache(item.device_type, meta);
      options.push({
        id: item.device_type,
        name: formatDeviceTypeDisplay(item.device_type, meta),
      });
    }
  });
  return options;
};

// 操作系统镜像：可按地域过滤；展示为「名称（镜像 id）」便于精确识别
export const getImageOptions = async (region?: string): Promise<HostApplyOption[]> => {
  const res = await getImages({ region: region ? [region] : [] });
  return (res?.data?.info ?? []).map((item: { image_id: string; image_name?: string }) => ({
    id: item.image_id,
    name: item.image_name ? `${item.image_name}（${item.image_id}）` : item.image_id,
  }));
};

// 需求类型
export const getRequireTypeOptions = async (): Promise<HostApplyOption[]> => {
  const { getRequirementType } = useConfigRequirementStore();
  const list = await getRequirementType();
  return (list ?? []).map((item) => ({ id: item.require_type, name: item.require_name }));
};

// 资源分配方式（静态枚举，去除占位项 0）
export const getResAssignOptions = (): HostApplyOption[] =>
  Object.values(RES_ASSIGN_TYPE)
    .filter((item) => item.value !== 0)
    .map((item) => ({ id: item.value, name: item.label }));

// 磁盘类型
export const getDiskTypeOptions = async (): Promise<HostApplyOption[]> => {
  const res = await getDiskTypes();
  return (res?.data?.info ?? []).map((item: { disk_type: string; disk_name?: string }) => ({
    id: item.disk_type,
    name: item.disk_name || item.disk_type,
  }));
};
