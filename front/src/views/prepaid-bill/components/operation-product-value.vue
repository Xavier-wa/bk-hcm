<script setup lang="ts">
import { computed } from 'vue';
import { usePrepaidBillStore } from '@/store/prepaid-bill';
import { useIdNameValue } from './use-id-name-value';
import type { IOperationProductItem } from '@/views/prepaid-bill/typings';

const props = defineProps<{
  value: string | number | Array<string | number>;
}>();

const prepaidBillStore = usePrepaidBillStore();

const localValue = computed(() => {
  if (props.value === null || props.value === undefined || props.value === '') return [];
  return (Array.isArray(props.value) ? props.value : [props.value]).map(String);
});

const { displayValue } = useIdNameValue<IOperationProductItem>(localValue, {
  combineKey: Symbol.for('prepaid-bill-operation-product-value'),
  getCached: (id) => prepaidBillStore.productCache.get(Number(id)),
  fetchByIds: (ids) => prepaidBillStore.getOperationProductsByIds(ids),
  format: (item, id) => item?.op_product_name || id,
});
</script>

<template>
  <bk-overflow-title class="full-width" resizeable type="tips">
    {{ displayValue }}
  </bk-overflow-title>
</template>
