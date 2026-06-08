<script setup lang="ts">
import { onMounted, ref, useTemplateRef } from 'vue';
import dayjs from 'dayjs';
import { timeFormatter } from '@/common/util';

export interface IFormModel {
  returnForecast: boolean;
  returnForecastTime: string;
}

const props = defineProps<{
  initValue: IFormModel;
}>();

const emits = defineEmits<{
  confirm: [model: IFormModel];
}>();

const displayText = ref(''); // 自定义内容的选择器，其展示文本由手动控制
const formRef = useTemplateRef('formRef');
const selectRef = useTemplateRef('selectRef');

const formModel = ref<IFormModel>({
  returnForecast: false,
  returnForecastTime: '',
});

const alwaysShow = ref(false);

// 设置禁用日期:不能早于当天/不能晚于当年最后一天
const disabledDate = (date: Date) => {
  const today = dayjs().startOf('day');
  const endOfYear = dayjs().endOf('year');
  return date.valueOf() < today.valueOf() || date.valueOf() > endOfYear.valueOf();
};

const changePopoverShow = (v: boolean) => {
  if (v) {
    selectRef.value?.showPopover();
  } else {
    selectRef.value?.hidePopover();
  }
};

const handleToggle = () => {
  if (alwaysShow.value) {
    changePopoverShow(true);
    return;
  }
};

const handelFormModelChange = () => {
  if (formModel.value.returnForecast) {
    formModel.value.returnForecastTime = timeFormatter(formModel.value.returnForecastTime, 'YYYY-MM-DD');
    displayText.value = `保留预测，${formModel.value.returnForecastTime} 开始使用预测`;
  } else {
    formModel.value.returnForecastTime = '';
    displayText.value = '放弃预测';
  }
};

const handleConfirm = async () => {
  await formRef.value.validate();

  handelFormModelChange();
  emits('confirm', formModel.value);

  alwaysShow.value = false;
  changePopoverShow(false);
};

onMounted(() => {
  formModel.value = props.initValue;
  handelFormModelChange();
});
</script>

<template>
  <bk-select
    ref="selectRef"
    :model-value="displayText"
    custom-content
    :clearable="false"
    :filterable="false"
    placeholder="请选择是否保留预测"
    scroll-height="250"
    trigger="manual"
    disabled
    @toggle="handleToggle"
  >
    <div class="pop-container">
      <bk-form ref="formRef" form-type="vertical" :model="formModel">
        <bk-form-item label="预测情况" property="returnForecast" required>
          <bk-radio-group class="radio-group" v-model="formModel.returnForecast" type="card">
            <!-- 10月版本禁用保留预测 -->
            <bk-radio-button :label="true" disabled>保留预测</bk-radio-button>
            <bk-radio-button :label="false">放弃预测</bk-radio-button>
          </bk-radio-group>
        </bk-form-item>
        <bk-form-item v-if="formModel.returnForecast" label="开始使用预测时间" property="returnForecastTime" required>
          <bk-date-picker
            style="width: 100%"
            v-model="formModel.returnForecastTime"
            :disabled-date="disabledDate"
            append-to-body
            clearable
            ext-popover-cls="date-picker-popover-custom"
            placeholder="请选择开始使用预测时间"
            @open-change="(v: boolean) => (alwaysShow = v)"
          />
        </bk-form-item>
      </bk-form>

      <div class="basic-button-list">
        <bk-button class="mr10" theme="primary" @click="handleConfirm">确定</bk-button>
        <bk-button @click="changePopoverShow(false)">取消</bk-button>
      </div>
    </div>
  </bk-select>
</template>

<style lang="scss" scoped>
.pop-container {
  padding: 12px 15px;
}

:global(.date-picker-popover-custom) {
  z-index: 9999 !important;
}

.basic-button-list {
  display: flex;
  justify-content: flex-end;
}
</style>
