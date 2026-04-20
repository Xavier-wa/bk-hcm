<script setup lang="ts">
import { inject, Ref, ref, shallowReactive, watch } from 'vue';
import { Close, Success } from 'bkui-vue/lib/icon';
import { timeFormatter } from '@/common/util';
import { useCvmDeviceStore, type IInheritCvm } from '@/store/cvm/device';
import { RequirementType } from '@/store/config/requirement';
import useCvmChargeType from '@/views/ziyanScr/hooks/use-cvm-charge-type';

const model = defineModel<string>();

const props = defineProps<{
  bizId: number;
  region: string;
  requireType: RequirementType;
  inheritInstanceId: string;
}>();

const emit = defineEmits<{
  checkSuccess: [cvm: IInheritCvm];
  checkFail: [];
}>();

const cvmDeviceStore = useCvmDeviceStore();

const { getMonthName, cvmChargeTypeNames } = useCvmChargeType();

const isInfoMode = inject<Ref<boolean>>('isInfoMode');

const isShowDetails = ref(false);

const inheritCvm = ref<IInheritCvm>();

const checkState = shallowReactive({
  error: undefined,
  message: '',
});

const onError = (error: any) => {
  inheritCvm.value = null;
  checkState.error = true;
  checkState.message = error.message;
  emit('checkFail');
};

const handleCheck = async () => {
  try {
    const res = await cvmDeviceStore.getInheritCvm(
      {
        require_type: props.requireType,
        bk_biz_id: props.bizId,
        bk_asset_id: model.value,
        region: props.region,
      },
      { globalError: false },
    );

    if (res.code === 0) {
      inheritCvm.value = res.data;
      checkState.error = false;
      checkState.message = '';
      emit('checkSuccess', inheritCvm.value);
    } else {
      onError(res);
    }
  } catch (error: any) {
    onError(error);
  }
};

watch(
  model,
  (value) => {
    // 详情态进入到编辑时需默认获取一次数据
    if (value && isInfoMode.value) {
      handleCheck();
    }
  },
  { immediate: true },
);
</script>

<template>
  <div class="asset-match">
    <div class="required">
      <span
        class="bottom-dashed"
        v-bk-tooltips="{
          content:
            requireType === RequirementType.RollServer
              ? '填写本业务下「一台主机」的CC固资号作为继承对象。新购主机将\n继承：套餐类型、计费时长、大小核心、地域大区等信息。需注意：\n1.不可跨业务使用CC固资号\n2.新购机型应为常规机型。如需专用机型，请选择常规项目申领'
              : '填写本业务下「一台主机」的CC固资号作为继承对象。新购主机将\n继承：套餐类型、计费时长、大小核心、地域大区等信息。\n如本业务无合适的固资号，请联系ICR助手',
        }"
      >
        固资号
      </span>
    </div>
    <bk-input behavior="simplicity" class="asset-id-input" size="small" v-model="model" />
    <div class="check-result">
      <div class="result-item" v-if="checkState.error === false">
        <Success fill="#2CAF5E" width="14" height="14" />
        校验成功
      </div>
      <div class="result-item" v-else-if="checkState.error === true">
        <Close fill="#EA3636" width="14" height="14" />
        <span class="bottom-dashed" v-bk-tooltips="checkState.message">校验失败</span>
      </div>
    </div>
    <bk-button
      theme="primary"
      size="small"
      outline
      :disabled="!model?.length"
      :loading="cvmDeviceStore.inheritCvmLoading"
      @click="handleCheck"
    >
      手动校验
    </bk-button>
    <bk-popover
      placement="right-start"
      theme="light"
      :offset="{ crossAxis: -10, mainAxis: 10 }"
      :padding="0"
      trigger="click"
      @after-show="isShowDetails = true"
      @after-hidden="isShowDetails = false"
    >
      <i
        :class="['hcm-icon', 'bkhcm-icon-file', 'details-icon', { active: isShowDetails }]"
        v-bk-tooltips="'查看详情'"
        v-show="inheritCvm"
      ></i>
      <template #content>
        <div class="cvm-info">
          <div class="info-item">
            <span class="label">机型：</span>
            <span class="content">{{ inheritCvm.device_type || '--' }}</span>
          </div>
          <div class="info-item">
            <span class="label">机型族：</span>
            <span class="content">{{ inheritCvm.device_group || '--' }}</span>
          </div>
          <div class="info-item">
            <span class="label">计费模式：</span>
            <span class="content">{{ cvmChargeTypeNames[inheritCvm.instance_charge_type] || '--' }}</span>
          </div>
          <div class="info-item">
            <span class="label">剩余时间：</span>
            <span class="content">
              {{ inheritCvm.charge_months ? getMonthName(inheritCvm.charge_months) : '--' }}
            </span>
          </div>
          <div class="info-item">
            <span class="label">计费起始时间：</span>
            <span class="content">{{ timeFormatter(inheritCvm.billing_start_time) }}</span>
          </div>
          <div class="info-item">
            <span class="label">计费过期时间：</span>
            <span class="content">{{ timeFormatter(inheritCvm.old_billing_expire_time) }}</span>
          </div>
        </div>
      </template>
    </bk-popover>
  </div>
</template>

<style scoped lang="scss">
.asset-match {
  display: flex;
  align-items: center;
  gap: 8px;
}

.asset-id-input {
  flex: 1;
}

.check-result {
  .result-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
}

.details-icon {
  padding: 6px;
  background: #f0f1f5;
  border-radius: 2px;
  cursor: pointer;

  &:hover {
    background: #dcdee5;
  }

  &.active {
    color: #3a84ff;
    background: #e1ecff;
  }
}

.cvm-info {
  .info-item {
    display: flex;
    align-items: center;
    margin: 8px 0;
    gap: 4px;
    font-size: 12px;

    .label {
      width: 90px;
      text-align: right;
      color: #4d4f56;
    }

    .content {
      color: #313238;
    }
  }
}
</style>
