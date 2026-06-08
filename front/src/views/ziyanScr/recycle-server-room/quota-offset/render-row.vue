<script setup lang="ts">
import { useTemplateRef } from 'vue';
import { SelectColumn, InputColumn, OperationColumn } from '@blueking/ediatable';
import BusinessSelector from '@/components/business-selector/business.vue';
import type { IQuotaOffset } from '@/store/dissolve/quota';

const localData = defineModel<IQuotaOffset>();

const props = defineProps<{
  index: number;
  removeable?: boolean;
}>();

const emit = defineEmits<{
  (e: 'remove', index: number): void;
  (e: 'add'): void;
}>();

const bizRef = useTemplateRef<InstanceType<typeof BusinessSelector>>('bizRef');
const typeRef = useTemplateRef<InstanceType<typeof SelectColumn>>('typeRef');
const offsetRef = useTemplateRef<InstanceType<typeof InputColumn>>('offsetRef');
const memoRef = useTemplateRef<InstanceType<typeof InputColumn>>('memoRef');

const adjustTypeList = [
  { value: 'increase', label: '调增' },
  { value: 'decrease', label: '调减' },
];

const handleRemove = () => {
  emit('remove', props.index);
};

const handleAdd = () => {
  emit('add');
};

const getValue = async () => {
  try {
    // 收集所有需要校验的ref
    const allRefs = [bizRef.value, typeRef.value, offsetRef.value, memoRef.value].filter(Boolean);
    // 并行触发各列组件的校验
    const [bizValue, typeValue, offsetValue, memoValue] = await Promise.all(allRefs.map((r) => r.getValue()));
    return {
      bk_biz_id: bizValue,
      type: typeValue,
      offset: offsetValue,
      memo: memoValue,
    };
  } catch {
    // 校验失败，重新抛出异常
    throw new Error('校验失败');
  }
};

defineExpose({
  getValue,
});
</script>

<template>
  <tr>
    <td>
      <BusinessSelector
        ref="bizRef"
        v-model="localData.bk_biz_id"
        :display="{ on: 'cell' }"
        :rules="[{ validator: (val: number) => val !== undefined && val !== null && val > 0, message: '请选择业务' }]"
      />
    </td>
    <td>
      <SelectColumn
        ref="typeRef"
        v-model="localData.type"
        :list="adjustTypeList"
        :rules="[{ validator: (val: string) => Boolean(val), message: '请选择调整类型' }]"
      />
    </td>
    <td>
      <InputColumn
        ref="offsetRef"
        v-model.number="localData.offset"
        type="number"
        clearable
        :min="0"
        :rules="[
          { validator: (val: number) => Boolean(val) || val === 0, message: 'CPU核数不能为空' },
        ]"
      />
    </td>
    <td>
      <InputColumn
        ref="memoRef"
        v-model="localData.memo"
        :rules="[{ validator: (val: string) => Boolean(val) && val.trim().length > 0, message: '备注不能为空' }]"
      />
    </td>
    <td>
      <OperationColumn :removeable="removeable" @add="handleAdd" @remove="handleRemove" />
    </td>
  </tr>
</template>
