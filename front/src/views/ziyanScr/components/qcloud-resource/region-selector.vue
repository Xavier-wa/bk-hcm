<template>
  <bk-select
    v-model="selected"
    :list="list"
    :id-key="idKey"
    :display-key="displayKey"
    :loading="configQcloudResourceStore.qcloudRegionListLoading"
    :multiple="multiple"
    :multiple-mode="multiple ? 'tag' : 'default'"
    :collapse-tags="collapseTags"
    filterable
  />
</template>

<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue';
import { useConfigQcloudResourceStore, IQcloudRegionItem } from '@/store/config/qcloud-resource';

interface IProps {
  multiple?: boolean;
  collapseTags?: boolean;
  idKey?: string;
  displayKey?: string;
}

defineOptions({ name: 'qcloud-region-selector' });

const model = defineModel<string[] | string>();
withDefaults(defineProps<IProps>(), {
  multiple: true,
  collapseTags: true,
  idKey: 'region',
  displayKey: 'region_cn',
});
const emit = defineEmits<{
  change: [qcloudZones: IQcloudRegionItem[]];
}>();

const configQcloudResourceStore = useConfigQcloudResourceStore();

const selected = computed({
  get() {
    return model.value;
  },
  set(val) {
    const qcloudZones = list.value.filter((item) => val.includes(item.region));
    emit('change', qcloudZones);
    model.value = val;
  },
});
const list = ref<IQcloudRegionItem[]>();
watchEffect(async () => {
  list.value = await configQcloudResourceStore.getQcloudRegionList();
});
</script>
