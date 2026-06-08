<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';
import { Form } from 'bkui-vue';
import InputWithValidate from '@/components/input-with-validate/index.vue';

import { useI18n } from 'vue-i18n';

import { timeFormatter } from '@/common/util';
import { INSTANCE_CHARGE_MAP } from '@/common/constant';
import http from '@/http';
import useCvmChargeType from '../../hooks/use-cvm-charge-type';

defineOptions({ name: 'InheritPackageFormItem' });

const model = defineModel<string>();

const props = defineProps<{
  region: string;
  requireType: number;
}>();

const emit = defineEmits<{
  (e: 'validateSuccess', host: RollingServerHost): void;
  (e: 'validateFailed'): void;
}>();

const { FormItem } = Form;

export interface RollingServerHost {
  device_type: string;
  instance_charge_type: string;
  charge_months: number;
  billing_start_time: string;
  old_billing_expire_time: string;
  bk_cloud_inst_id: string;
  device_group: string;
}

const { t } = useI18n();
const { getMonthName } = useCvmChargeType();

const formItem = useTemplateRef('formItem');
const isCheckLoading = ref(false);
const rollingServerHost = ref<RollingServerHost>();

const checkMessage = ref('');

const checkRollingSeverHost = (bk_asset_id: string) => {
  if (!bk_asset_id) return true;
  return new Promise(async (resolve, reject) => {
    isCheckLoading.value = true;
    try {
      const { region, requireType } = props;
      const res = await http.post(
        '/api/v1/woa/task/check/apply/order/host',
        { require_type: requireType, bk_asset_id, region },
        { globalError: false },
      );
      if (res.code === 0) {
        rollingServerHost.value = res.data;
        // 将校验成功的机器代表信息回传
        emit('validateSuccess', res.data);
        checkMessage.value = '';
        resolve(true);
      } else {
        handleError(res);
        reject(false);
      }
    } catch (error: any) {
      // 校验失败
      handleError(error);
      reject(false);
    } finally {
      isCheckLoading.value = false;
    }
  });
};

const handleError = (error: any) => {
  rollingServerHost.value = null;
  emit('validateFailed');
  checkMessage.value = error.message;
};
</script>

<template>
  <FormItem
    ref="formItem"
    :label="t('继承套餐的机器代表')"
    property="bk_asset_id"
    required
    :description="
      t(
        '选择填写本业务下一台机器作为继承套餐的代表，继承原套餐类型、计费时长、地域大区等信息。填写要求：\n1.必须为本业务下的主机的CC固资号\n2.主机的地域，必须和所选地域匹配，大区必须匹配\n3.机型必须为常规机型\n4.主机必须有计费模式，计费开始时间，计费结束时间',
      )
    "
    :rules="[{ validator: (value: string) => checkRollingSeverHost(value) }]"
  >
    <InputWithValidate
      v-model.trim="model"
      class="w600"
      :loading="isCheckLoading"
      :disabled="!region"
      @click="formItem.validate()"
      :placeholder="t('请输入本业务下一个继承机器的CC固资号，当前必须为同城，待支持跨城')"
    />
    <template #error>{{ checkMessage }}</template>
    <!-- 校验成功，展示继承的机器信息，并且在添加配置清单时设置默认值（计费模式、购买时长...）。 -->
    <ul class="inherit-instance-info" v-if="rollingServerHost">
      <li class="instance-info-item">
        <span class="label">{{ t('机型') }}</span>
        <span class="content">{{ rollingServerHost.device_type || '--' }}</span>
      </li>
      <li class="instance-info-item">
        <span class="label">{{ t('机型族') }}</span>
        <span class="content">{{ rollingServerHost.device_group || '--' }}</span>
      </li>
      <li class="instance-info-item">
        <span class="label">{{ t('计费模式') }}</span>
        <span class="content">{{ INSTANCE_CHARGE_MAP[rollingServerHost.instance_charge_type] || '--' }}</span>
      </li>
      <li class="instance-info-item">
        <span class="label">{{ t('剩余时间') }}</span>
        <span class="content">
          {{ rollingServerHost.charge_months ? getMonthName(rollingServerHost.charge_months) : '--' }}
        </span>
      </li>
      <li class="instance-info-item">
        <span class="label">{{ t('计费起始时间') }}</span>
        <span class="content">{{ timeFormatter(rollingServerHost.billing_start_time) }}</span>
      </li>
      <li class="instance-info-item">
        <span class="label">{{ t('计费过期时间') }}</span>
        <span class="content">{{ timeFormatter(rollingServerHost.old_billing_expire_time) }}</span>
      </li>
    </ul>
  </FormItem>
</template>

<style scoped lang="scss">
.w600 {
  width: 600px;
}

.inherit-instance-info {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-top: 8px;

  .instance-info-item {
    font-size: 12px;
    display: flex;
    flex-direction: column;
    line-height: 20px;

    .label {
      color: $font-deep-color;
    }
  }
}
</style>
