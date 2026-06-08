<script setup lang="ts">
import http from '@/http';
import { Select } from 'bkui-vue';
import { computed, ref, watch } from 'vue';

export interface ICvmSubnet {
  id: number;
  region: string;
  zone: string;
  vpc_id: string;
  vpc_name: string;
  subnet_id: string;
  subnet_name: string;
  enable: boolean;
  comment: string;
}
type ICvmSubnetList = Array<ICvmSubnet>;

defineOptions({ name: 'CvmSubnetSelector' });

const model = defineModel<string>();

const props = defineProps<{ region: string; zone: string; vpc: string; disabled: boolean }>();

const emit = defineEmits<(e: 'change', val: ICvmSubnet) => void>();

const { Option } = Select;

const optionList = ref<ICvmSubnetList>([]);

const selectedId = computed({
  get() {
    // 接口入参subnet_id, select值为id, 此处做转换
    const selectedItem = optionList.value.find((item) => item.subnet_id === model.value);
    return selectedItem?.id || undefined;
  },
  set(val) {
    const selectedItem = optionList.value.find((item) => item.id === val);
    emit('change', selectedItem);
    // 接口入参subnet_id, select值为id, 此处做转换
    model.value = selectedItem?.subnet_id ?? '';
  },
});

const findCvmSubnetBySubnetId = (subnet_id: string) => {
  return optionList.value.find((item) => item.subnet_id === subnet_id);
};

const getOptionList = async (data: { region: string; zone: string; vpc: string }) => {
  const res = await http.post('/api/v1/woa/config/findmany/config/cvm/subnet', data);
  optionList.value = res.data.info;
};

watch(
  [() => props.region, () => props.zone, () => props.vpc],
  ([region, zone, vpc]) => {
    if (region && zone && vpc) {
      getOptionList({ region: props.region, zone, vpc });
    } else {
      optionList.value = [];
    }
  },
  { immediate: true },
);

defineExpose({ findCvmSubnetBySubnetId });
</script>

<template>
  <Select class="w600" v-model="selectedId" :disabled="props.disabled" filterable>
    <Option
      v-for="{ id, subnet_id: subnetId, subnet_name: subnetName } in optionList"
      :key="id"
      :id="id"
      :name="`${subnetId} | ${subnetName}`"
    />
  </Select>
</template>
