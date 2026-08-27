<script setup lang="ts">
import { computed } from 'vue';
import { usePrepaidBillStore } from '@/store/prepaid-bill';
import { useIdNameValue } from './use-id-name-value';
import type { IMainAccountItem } from '@/views/prepaid-bill/typings';

const props = defineProps<{
  value: string | string[];
}>();

const prepaidBillStore = usePrepaidBillStore();

const localValue = computed(() => {
  if (!props.value) return [];
  return Array.isArray(props.value) ? props.value.map(String) : [String(props.value)];
});

const { displayValue } = useIdNameValue<IMainAccountItem>(localValue, {
  combineKey: Symbol.for('prepaid-bill-main-account-value'),
  getCached: (id) => prepaidBillStore.mainCache.get(id),
  fetchByIds: (ids) => prepaidBillStore.getMainAccountsByIds(ids),
  format: (item, id) => item?.name || item?.cloud_id || id,
});
</script>

<template>
  <bk-overflow-title class="full-width" resizeable type="tips">
    {{ displayValue }}
  </bk-overflow-title>
</template>
