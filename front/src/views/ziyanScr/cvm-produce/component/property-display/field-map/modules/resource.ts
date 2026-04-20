import { h } from 'vue';
import { ICvmSystemDisk } from '@/views/ziyanScr/components/cvm-system-disk/typings';
import { getImageName, getNetworkTypeCn } from '../../transform';
import { getRegionCn, getZoneCn } from '@/views/ziyanScr/cvm-web/transform';

import CvmSystemDiskDisplay from '@/views/ziyanScr/components/cvm-system-disk/display.vue';
import CvmDataDiskDisplay from '@/views/ziyanScr/components/cvm-data-disk/display.vue';
import { ICvmDataDisk } from '@/views/ziyanScr/components/cvm-data-disk/typings';
import { RES_ASSIGN_TYPE } from '@/components/device-type-selector/constants';
import type { ICvmDeviceTypeFormData } from '@/components/device-type-selector/typings';

export const imageId = Object.freeze({
  name: 'image_id',
  cn: '镜像',
  transformer: getImageName,
});

export const vpc = Object.freeze({
  name: 'vpc',
  cn: '私有网络',
});

export const subnet = Object.freeze({
  name: 'subnet',
  cn: '私有子网',
  type: String,
});

export const module = Object.freeze({
  name: 'module',
  cn: '模块',
  type: String,
});

export const totalNum = Object.freeze({
  name: 'total_num',
  cn: '需求数量',
});

export const region = Object.freeze({
  name: 'region',
  cn: '地域',
  transformer: getRegionCn,
});

export const zone = Object.freeze({
  name: 'zone',
  cn: '园区',
  transformer: getZoneCn,
});

export const zones = Object.freeze({
  name: 'zones',
  cn: '园区',
  transformer: (zones: string[]) => {
    if (zones?.[0] === 'all') {
      return '全部可用区';
    }
    return zones?.map((zone: string) => getZoneCn(zone))?.join('，') ?? '--';
  },
});

export const resAssign = Object.freeze({
  name: 'res_assign',
  cn: '资源分布方式',
  transformer: (resAssign: ICvmDeviceTypeFormData['resAssignType']) => RES_ASSIGN_TYPE[resAssign]?.label ?? '--',
});

export const deviceType = Object.freeze({
  name: 'device_type',
  cn: '机型',
});

export const networkType = Object.freeze({
  name: 'network_type',
  cn: '网络类型',
  transformer: getNetworkTypeCn,
});

export const systemDisk = Object.freeze({
  name: 'system_disk',
  cn: '系统盘',
  transformer: (systemDisk: ICvmSystemDisk) => h(CvmSystemDiskDisplay, { systemDisk }),
});

export const dataDisk = Object.freeze({
  name: 'data_disk',
  cn: '数据盘',
  transformer: (dataDiskList: ICvmDataDisk[], row: any) =>
    h(CvmDataDiskDisplay, { dataDiskList, diskType: row.disk_type, diskSize: row.disk_size }),
});

export const bkAssetId = Object.freeze({
  name: 'bk_asset_id',
  cn: '继承的固资号',
});

export const inheritInstanceId = Object.freeze({
  name: 'inherit_instance_id',
  cn: '继承的云实例ID',
});
