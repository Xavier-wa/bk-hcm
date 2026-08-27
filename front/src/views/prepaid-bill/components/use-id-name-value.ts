import { computed, watchEffect, type ComputedRef } from 'vue';
import CombineRequest from '@blueking/combine-request';

type UseIdNameValueOptions<T> = {
  combineKey: symbol;
  getCached: (id: string) => T | undefined;
  fetchByIds: (ids: string[]) => void | Promise<void>;
  format: (item: T | undefined, id: string) => string;
};

export const useIdNameValue = <T>(localValue: ComputedRef<string[]>, options: UseIdNameValueOptions<T>) => {
  const combineRequest = CombineRequest.setup(options.combineKey, (batches: string[][]) => {
    const uniqueIds = [...new Set(batches.reduce((acc, cur) => acc.concat(cur), []))];
    options.fetchByIds(uniqueIds);
  });

  watchEffect(() => {
    if (!localValue.value.length) return;
    const missing = localValue.value.filter((id) => id && !options.getCached(id));
    if (missing.length) {
      combineRequest.add(missing);
    }
  });

  const displayValue = computed(() => {
    const names = localValue.value.map((id) => {
      if (!id) return '';
      const item = options.getCached(id);
      return options.format(item, id);
    });
    return names.filter(Boolean).join(';') || '--';
  });

  return { displayValue };
};
