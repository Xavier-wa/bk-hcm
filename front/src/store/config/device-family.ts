import { ref } from 'vue';
import { defineStore } from 'pinia';
import http from '@/http';
import { IListResData } from '@/typings';

export interface IDeviceFamilyItem {
  id: string;
  name: string;
  children?: Array<{ id: string; name: string }> | [];
}

export const useConfigDeviceFamilyStore = defineStore('config-device-family', () => {
  const deviceFamilyList = ref<IDeviceFamilyItem[]>();

  const getDeviceFamily = async () => {
    if (deviceFamilyList.value) {
      return deviceFamilyList.value;
    }
    try {
      const res: IListResData<string[]> = await http.get('/api/v1/woa/meta/device_family/list');

      const list = res?.data?.details ?? [];

      // 定义限定类型（精确匹配）
      const limitedTypes = ['标准型', '高IO型', '大数据型', '计算型'];

      // 构建结果数组
      const result: IDeviceFamilyItem[] = limitedTypes
        .filter((name) => list.includes(name)) // 只保留原始数据中存在的项
        .map((name) => ({
          id: name,
          name,
          children: [] as [],
        }));

      // 处理GPU型分类
      const gpuKeywords = ['GPU高主频型', 'GPU计算型', 'GPU型', 'GPU型黑石物理服务器', 'NPU型'];
      const gpuTypes = list.filter((item) => gpuKeywords.some((keyword) => item.includes(keyword)));

      if (gpuTypes.length > 0) {
        result.push({
          id: 'GPU型',
          name: 'GPU型',
          children: gpuTypes.map((item) => ({
            id: item,
            name: item,
          })),
        });
      }

      deviceFamilyList.value = result;

      return deviceFamilyList.value;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    }
  };

  return {
    getDeviceFamily,
  };
});
