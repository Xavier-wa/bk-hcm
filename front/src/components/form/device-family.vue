<script setup lang="ts">
import { computed, ref, useAttrs, watchEffect } from 'vue';
import { useConfigDeviceFamilyStore, type IDeviceFamilyItem } from '@/store/config/device-family';

defineOptions({ name: 'hcm-form-device-family' });

const model = defineModel<string | string[]>();

const props = withDefaults(
  defineProps<{
    multiple?: boolean;
    clearable?: boolean;
    filterable?: boolean;
    disabled?: boolean;
    showAll?: boolean;
    allOptionId?: string;
    filter?: (list: IDeviceFamilyItem[]) => IDeviceFamilyItem[];
    optionDisabled?: (item?: IDeviceFamilyItem) => boolean;
    optionDisabledTips?: (item?: IDeviceFamilyItem) => { content: string; disabled: boolean };
    appearance?: 'select' | 'capsule';
    defaultFirst?: boolean;
  }>(),
  {
    multiple: false,
    filterable: true,
    optionDisabled: () => false,
    optionDisabledTips: () => ({ content: '', disabled: true }),
    appearance: 'select',
    defaultFirst: false,
  },
);

const emit = defineEmits<(e: 'change', val: IDeviceFamilyItem[], oldVal: IDeviceFamilyItem[]) => void>();

const attrs = useAttrs();

const list = ref<IDeviceFamilyItem[]>([]);

const localModel = computed({
  get() {
    if (props.multiple && model.value && !Array.isArray(model.value)) {
      return [model.value];
    }
    return model.value;
  },
  set(value) {
    model.value = value;
  },
});

const configDeviceFamilyStore = useConfigDeviceFamilyStore();

const handleChange = (val: string | string[]) => {
  const finalVal = Array.isArray(val) ? val : [val];
  const oldVal = Array.isArray(model.value) ? model.value : [model.value];
  const item = list.value.filter((item) => finalVal.includes(item.id));
  const oldItem = list.value.filter((item) => oldVal.includes(item.id));
  emit('change', item, oldItem);
};

watchEffect(async () => {
  list.value = await configDeviceFamilyStore.getDeviceFamily();
  if (props.filter) {
    list.value = props.filter(list.value);
  }

  if (props.defaultFirst && list.value.length > 0) {
    const isEmpty = props.multiple
      ? !Array.isArray(model.value) || model.value.length === 0
      : model.value === undefined || model.value === '' || model.value === null;

    if (isEmpty) {
      const firstItem = list.value[0];
      model.value = props.multiple ? [firstItem.id] : firstItem.id;
      emit('change', [firstItem], []);
    }
  }
});
</script>

<template>
  <bk-select
    v-if="appearance === 'select'"
    v-model="localModel"
    :clearable="clearable"
    :multiple="multiple"
    :shwo-all="showAll"
    :all-option-id="allOptionId"
    @change="handleChange"
    v-bind="attrs"
  >
    <bk-option
      v-for="(item, index) in list"
      :disabled="optionDisabled(item)"
      v-bk-tooltips="{ ...optionDisabledTips(item), boundary: 'parent', placement: 'left' }"
      :key="index"
      :value="item.id"
      :label="item.name"
    />
  </bk-select>
  <bk-radio-group
    v-else-if="appearance === 'capsule' && !multiple"
    type="capsule"
    v-model="localModel"
    v-bind="attrs"
    @change="handleChange"
  >
    <bk-radio-button v-if="showAll" :key="allOptionId" :label="allOptionId">全部</bk-radio-button>
    <bk-radio-button
      v-for="item in list"
      :key="item.id"
      :label="item.id"
      :disabled="optionDisabled(item)"
      v-bk-tooltips="optionDisabledTips(item)"
    >
      {{ item.name }}
    </bk-radio-button>
  </bk-radio-group>
</template>

<style lang="scss" scoped></style>
