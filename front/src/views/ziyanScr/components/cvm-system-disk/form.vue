<script setup lang="ts">
import { computed, watch } from 'vue';
import { ICvmSystemDisk } from './typings';
import { CVM_SYSTEM_DISK_INFO, CvmSystemDiskType } from './constants';
import { ICloudInstanceConfigItem } from '@/typings/ziyanScr';
import { useFormItem } from 'bkui-vue/lib/form';

interface IProps {
  isItDeviceType: boolean;
  currentCloudInstanceConfig: ICloudInstanceConfigItem;
}

const model = defineModel<ICvmSystemDisk>();
const props = defineProps<IProps>();

const formItem = useFormItem();

// IT机型，并且local_disk_type_list有值，则可以选本地盘
const hasLocalDisk = computed(
  () => props.isItDeviceType && props.currentCloudInstanceConfig?.local_disk_type_list?.length,
);
const cvmSystemDiskList = computed(() =>
  hasLocalDisk.value
    ? [
        CvmSystemDiskType.CLOUD_SSD,
        CvmSystemDiskType.CLOUD_PREMIUM,
        ...(props.currentCloudInstanceConfig.local_disk_type_list.map((item) => item.type) as CvmSystemDiskType[]),
      ]
    : [CvmSystemDiskType.CLOUD_SSD, CvmSystemDiskType.CLOUD_PREMIUM],
);

const defaultLimit = { min: 0, max: 50 };
const max = computed(() => {
  const target = props.currentCloudInstanceConfig?.local_disk_type_list?.find(
    (item) => item.type === model.value.disk_type,
  );
  if (target) return target.max_size;

  return CVM_SYSTEM_DISK_INFO[model.value.disk_type]?.max ?? defaultLimit.max;
});
const min = computed(() => {
  const target = props.currentCloudInstanceConfig?.local_disk_type_list?.find(
    (item) => item.type === model.value.disk_type,
  );
  if (target) return target.min_size;

  return CVM_SYSTEM_DISK_INFO[model.value.disk_type]?.min ?? defaultLimit.min;
});

const handleDiskTypeChange = () => {
  model.value.disk_size = min.value;
};

const tips = computed(() =>
  props.isItDeviceType
    ? '注意：2025年08月支持系统盘可配置硬盘类型。在此之前，IT2、IT3、I3、IT5、IT5c机型默认为本地盘50G大小'
    : '注意：2025年08月支持系统盘可配置硬盘类型。在此之前，默认为高性能云盘100G大小',
);

watch(hasLocalDisk, (val) => {
  if (!val && [CvmSystemDiskType.LOCAL_BASIC, CvmSystemDiskType.LOCAL_SSD].includes(model.value.disk_type)) {
    model.value.disk_type = undefined;
    handleDiskTypeChange();
  }
});

watch(model, () => formItem?.validate('change'), { deep: true });
</script>

<template>
  <div class="cvm-system-disk-container">
    <bk-select
      v-model="model.disk_type"
      :popover-options="{ boundary: 'parent' }"
      class="form-control"
      @change="handleDiskTypeChange"
      filterable
    >
      <bk-option
        v-for="type in cvmSystemDiskList"
        :key="type"
        :id="type"
        :name="CVM_SYSTEM_DISK_INFO[type].disk_name"
      />
    </bk-select>
    <hcm-form-number
      v-model="model.disk_size"
      class="form-control"
      prefix="大小"
      suffix="GB"
      :step="50"
      :max="max"
      :min="min"
    />
  </div>
  <div class="tips">{{ tips }}</div>
</template>

<style scoped lang="scss">
.cvm-system-disk-container {
  display: flex;
  align-items: center;
  gap: 8px;

  .form-control {
    width: 240px;
  }
}

.tips {
  font-size: 12px;
  color: #979ba5;
}
</style>
