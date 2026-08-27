<script setup lang="ts">
import { computed } from 'vue';
import { getModel } from '@/model/manager';
import type { ModelPropertyDisplay } from '@/model/typings';
import GridContainer from '@/components/layout/grid-container/grid-container.vue';
import GridItem from '@/components/layout/grid-container/grid-item.vue';
import MainAccountValue from '@/views/prepaid-bill/components/main-account-value.vue';
import RootAccountValue from '@/views/prepaid-bill/components/root-account-value.vue';
import OperationProductValue from '@/views/prepaid-bill/components/operation-product-value.vue';
import { DetailsField } from './field';
import type { IPrepaidBillItem } from '@/views/prepaid-bill/typings';

defineProps<{
  data: IPrepaidBillItem;
}>();

const model = getModel(DetailsField);
const groupedFields = computed(() => model.getPropertiesByGroup<ModelPropertyDisplay>());
</script>

<template>
  <div class="prepaid-bill-details">
    <div v-for="(fields, group) in groupedFields" :key="group" class="details-panel">
      <div class="panel-title">{{ group }}</div>
      <grid-container :column="3" :content-min-width="0" :label-width="150">
        <grid-item v-for="field in fields" :key="field.id" :label="field.name">
          <template v-if="field.id === 'product_id'">
            <OperationProductValue :value="data.product_id" />
          </template>
          <template v-else-if="field.id === 'main_account_id'">
            <MainAccountValue :value="data.main_account_id" />
          </template>
          <template v-else-if="field.id === 'root_account_id'">
            <RootAccountValue :value="data.root_account_id" />
          </template>
          <display-value
            v-else
            :property="field"
            :value="field.meta?.display?.render ? data : data[field.id]"
            :display="{ ...field.meta?.display, on: 'info' }"
          />
        </grid-item>
      </grid-container>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.prepaid-bill-details {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.details-panel {
  background: #fff;
  border-radius: 2px;
  box-shadow: 0 2px 4px 0 #1919290d;
  padding: 16px 24px;

  .panel-title {
    font-size: 14px;
    font-weight: 700;
    color: #313238;
    line-height: 22px;
    margin-bottom: 16px;
  }
}
</style>
