import { ChargeTypeMap } from '@/typings/plan';
import { RES_ASSIGN_TYPE } from '@/components/device-type-selector/constants';
import { getRegionCn, getZoneCn } from '@/views/ziyanScr/cvm-web/transform';
import { getImageName, getDiskTypesName } from '@/views/ziyanScr/cvm-produce/component/property-display/transform';

import type { HostApplyDisk, HostApplySuborder } from './types';

// 主机申领 suborder 字段展示工具：键值表的 label 映射、字段顺序与 value 格式化。
// 字段值为后端编码，展示时统一转为中文名称（vendor=ZIYAN，复用 ziyanScr transform/常量，见 api.md）。
// 例外：device_type 机型本期原样展示；require_type 由模板内联 <req-type-value> 组件渲染。

export interface SpecDisplayItem {
  key: string;
  label: string;
  value: string;
}

// 键值表展示顺序（设计稿 §3）
const SPEC_FIELD_ORDER: Array<keyof HostApplySuborder> = [
  'require_type',
  'region',
  'zone',
  'device_type',
  'image_id',
  'res_assign',
  'replicas',
  'charge_type',
  'system_disk',
  'data_disk',
];

const SPEC_FIELD_LABEL: Record<string, string> = {
  require_type: '需求类型',
  region: '地域',
  zone: '可用区',
  device_type: '机型',
  image_id: '操作系统',
  res_assign: '资源分配方式',
  replicas: '申请数量',
  charge_type: '计费模式',
  system_disk: '系统盘',
  data_disk: '数据盘',
};

const formatDisk = (disk?: HostApplyDisk): string =>
  disk && disk.disk_type ? `${getDiskTypesName(disk.disk_type)} ${disk.disk_size}GB` : '';

const formatDataDisks = (disks?: HostApplyDisk[]): string =>
  (disks ?? [])
    .filter((d) => d && d.disk_type)
    .map((d) => `${getDiskTypesName(d.disk_type)} ${d.disk_size}GB × ${d.disk_num}`)
    .join('；');

// 取某字段的展示值（空值返回 ''，由调用方决定是否渲染）
const isNil = (val: unknown): boolean => val === undefined || val === null;

// 可用区 all 语义化展示，其余走 ziyan zone 名称映射
const formatZone = (zone?: string): string => {
  if (isNil(zone) || zone === '') return '';
  return zone === 'all' ? '全部可用区' : getZoneCn(zone);
};

export const getSpecFieldText = (suborder: HostApplySuborder, key: keyof HostApplySuborder): string => {
  switch (key) {
    case 'replicas':
      return isNil(suborder.replicas) ? '' : `${suborder.replicas} 台`;
    case 'system_disk':
      return formatDisk(suborder.system_disk);
    case 'data_disk':
      return formatDataDisks(suborder.data_disk);
    case 'region':
      return isNil(suborder.region) ? '' : getRegionCn(suborder.region);
    case 'zone':
      return formatZone(suborder.zone);
    case 'image_id': {
      if (isNil(suborder.image_id)) return '';
      const imageName = getImageName(suborder.image_id);
      // 命中名称时展示「名称（id）」，未命中时 getImageName 回退为 id，直接展示 id
      return imageName === suborder.image_id ? suborder.image_id : `${imageName}（${suborder.image_id}）`;
    }
    case 'charge_type':
      return isNil(suborder.charge_type)
        ? ''
        : ChargeTypeMap[suborder.charge_type as keyof typeof ChargeTypeMap] ?? String(suborder.charge_type);
    case 'res_assign': {
      const val = suborder.res_assign;
      if (isNil(val) || val === '') return '';
      return RES_ASSIGN_TYPE[val as keyof typeof RES_ASSIGN_TYPE]?.label ?? String(val);
    }
    default: {
      // device_type 等：原样展示
      const val = suborder[key];
      return isNil(val) ? '' : String(val);
    }
  }
};

export const getSpecFieldLabel = (key: string): string => SPEC_FIELD_LABEL[key] ?? key;

// 将 suborder 转为有序键值列表，缺失/空值字段不渲染
export const toSpecDisplayItems = (suborder: HostApplySuborder): SpecDisplayItem[] =>
  SPEC_FIELD_ORDER.map((key) => ({
    key,
    label: getSpecFieldLabel(key),
    value: getSpecFieldText(suborder, key),
  })).filter((item) => item.value !== '');
