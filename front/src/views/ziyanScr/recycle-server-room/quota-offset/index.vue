<script setup lang="ts">
import { ref } from 'vue';
import { Ediatable, HeadColumn } from '@blueking/ediatable';
import RenderRow from './render-row.vue';
import type { IQuotaOffset } from '@/store/dissolve/quota';

const tableData = defineModel<IQuotaOffset[]>({ required: true, default: () => [] });
// 存储每个 RenderRow 的组件实例
const rowRefs = ref<InstanceType<typeof RenderRow>[]>([]);

const getSingleRow: () => IQuotaOffset = () => ({
  bk_biz_id: undefined,
  type: undefined,
  offset: undefined,
  memo: undefined,
});

const addRow = () => {
  tableData.value?.push(getSingleRow());
};

const removeRow = (index: number) => {
  tableData.value?.splice(index, 1);
  rowRefs.value?.splice(index, 1);
};

const validate = async (): Promise<boolean> => {
  try {
    // 调用每个 RenderRow 的 getValue 方法进行校验
    await Promise.all(rowRefs.value.map((row) => row?.getValue()));
    return true;
  } catch (error) {
    return false;
  }
};

defineExpose({
  validate,
  addRow,
});
</script>

<template>
  <Ediatable>
    <template #default>
      <HeadColumn required :min-width="120">业务</HeadColumn>
      <HeadColumn required :min-width="120">调整类型</HeadColumn>
      <HeadColumn required :min-width="120">CPU核数</HeadColumn>
      <HeadColumn :min-width="120">备注</HeadColumn>
      <HeadColumn :required="false" :min-width="80"></HeadColumn>
    </template>
    <template #data>
      <RenderRow
        v-for="(_row, index) in tableData"
        v-model="tableData[index]"
        :key="index"
        :ref="(el: any) => el && (rowRefs[index] = el)"
        :index="index"
        @add="addRow"
        @remove="removeRow(index)"
      />
    </template>
  </Ediatable>
</template>
