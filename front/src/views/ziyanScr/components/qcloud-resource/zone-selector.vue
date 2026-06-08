<template>
  <bk-select
    v-model="selected"
    :list="list"
    :id-key="idKey"
    :display-key="displayKey"
    :loading="configQcloudResourceStore.qcloudZoneListLoading"
    :multiple="multiple"
    :multiple-mode="multiple ? 'tag' : 'default'"
    :collapse-tags="collapseTags"
    filterable
  />
</template>

<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue';
import { useConfigQcloudResourceStore, IQcloudZoneItem } from '@/store/config/qcloud-resource';

interface IProps {
  region: string[];
  multiple?: boolean;
  collapseTags?: boolean;
  idKey?: string;
  displayKey?: string;
}

defineOptions({ name: 'qcloud-zone-selector' });

const model = defineModel<string[] | string>();
const props = withDefaults(defineProps<IProps>(), {
  multiple: true,
  collapseTags: true,
  idKey: 'zone',
  displayKey: 'zone_cn',
});
const emit = defineEmits<{
  change: [qcloudZones: IQcloudZoneItem[]];
}>();

const configQcloudResourceStore = useConfigQcloudResourceStore();

const selected = computed({
  get() {
    return model.value;
  },
  set(val) {
    const qcloudZones = list.value.filter((item) => val.includes(item.zone));
    emit('change', qcloudZones);
    model.value = val;
  },
});
const list = ref<IQcloudZoneItem[]>();
watchEffect(async () => {
  list.value = await configQcloudResourceStore.getQcloudZoneList(props.region);
});
</script>
