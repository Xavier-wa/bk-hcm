<script setup lang="ts">
import { computed, ref, useAttrs } from 'vue';
import { ImageState, ImageConfigMap } from '@/constants/scr';
import CvmImageSelector, { type ICvmImage } from './cvm-image-selector.vue';

interface IProps {
  region: string[];
  idKey?: string;
  displayKey?: string;
  multiple?: boolean;
  disabled?: boolean;
  bizId?: number | string;
}

defineOptions({ name: 'form-cvm-image-selector' });

const model = defineModel<string | string[]>();

const props = withDefaults(defineProps<IProps>(), {
  idKey: 'image_id',
  displayKey: 'image_name',
  multiple: false,
  disabled: false,
});

const attrs = useAttrs();

const selected = ref<ICvmImage[]>([]);

const selectedNames = computed(() => selected.value.map((item) => item[props.displayKey] ?? '--')?.join('、'));

const StateText = {
  [ImageState.RECOMMENDED]: '推荐',
  [ImageState.DEPRECATED]: '已停止维护',
  [ImageState.PENDING_DEPRECATION]: '即将停止维护',
};
const StateTheme = {
  [ImageState.RECOMMENDED]: 'success',
  [ImageState.DEPRECATED]: '',
  [ImageState.PENDING_DEPRECATION]: 'danger',
};

const transformOptions = (options: any) => {
  const orderIds = [...ImageConfigMap.keys()];
  return options
    .map((item: any, index: number) => {
      const imageId = item[props.idKey];
      const config = ImageConfigMap.get(imageId);
      return {
        ...item,
        ...config,
        index,
        priority: config ? orderIds.indexOf(imageId) : Number.MAX_SAFE_INTEGER,
      };
    })
    .sort((a: any, b: any) => a.priority - b.priority || a.index - b.index);
};

const handleChange = (value: string | string[], items: ICvmImage[]) => {
  selected.value = items;
};
</script>

<template>
  <cvm-image-selector
    v-model="model"
    :transform="transformOptions"
    :region="region"
    :id-key="idKey"
    :display-key="displayKey"
    :multiple="multiple"
    :disabled="disabled"
    :biz-id="bizId"
    v-bind="attrs"
    @change="handleChange"
  >
    <template #option-item="{ option }">
      <div class="image-option-item">
        <div>{{ option[displayKey] }}（{{ option[idKey] }}）</div>
        <bk-tag v-if="option.type" size="small">{{ option.type }}</bk-tag>
        <bk-tag
          v-if="option.state && option.state === ImageState.RECOMMENDED"
          size="small"
          :theme="StateTheme[option.state]"
        >
          {{ StateText[option.state] }}
        </bk-tag>
      </div>
    </template>
  </cvm-image-selector>
  <div class="tips" v-if="selected?.length">
    所选镜像为 {{ selectedNames }}，如需了解更多信息，请参考
    <a href="https://iwiki.woa.com/p/4015588910" target="_blank">https://iwiki.woa.com/p/4015588910</a>
  </div>
</template>

<style lang="scss" scoped>
.image-option-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.tips {
  font-size: 12px;
}
</style>
