<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';
import { useDissolveQuotaStore, type ICpuCoreSummary } from '@/store/dissolve/quota';

defineOptions({ name: 'dissolve-cpu-core-limits' });
const props = defineProps<{
  replicasCpuCores: number;
  bizId: number;
  isBusinessPage: boolean;
}>();

const { t } = useI18n();
const dissolveQuotaStore = useDissolveQuotaStore();
const { cpuCoreSummaryLoading } = storeToRefs(dissolveQuotaStore);
const summaryData = ref<ICpuCoreSummary>({ total_core: 0, delivered_core: 0, available_quota: 0 });

const availableCpuCoreQuota = computed(() => {
  const { available_quota } = summaryData.value ?? {};
  return available_quota;
});

watchEffect(async () => {
  summaryData.value = await dissolveQuotaStore.getCpuCoreSummary(Number(props.bizId), {
    bk_biz_id: props.isBusinessPage ? undefined : Number(props.bizId),
  });
});

defineExpose({ availableCpuCoreQuota, cpuCoreSummaryLoading });
</script>

<template>
  <ul class="wrapper" v-if="!cpuCoreSummaryLoading">
    <li>
      <span>{{ t('裁撤总额度：') }}</span>
      <span class="number">{{ summaryData.total_core ?? '--' }} {{ t('核') }}</span>
    </li>
    <li>
      <span>{{ t('可申领额度：') }}</span>
      <span class="number">{{ availableCpuCoreQuota }} {{ t('核') }}</span>
    </li>
    <li>
      <span>{{ t('需求核数：') }}</span>
      <span class="number">{{ replicasCpuCores }} {{ t('核') }}</span>
    </li>
  </ul>
</template>

<style scoped lang="scss">
.wrapper {
  margin-left: 24px;
  display: flex;
  align-items: center;
  gap: 12px;

  .number {
    color: $warning-color;
  }
}
</style>
