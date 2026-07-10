<script setup lang="ts">
import { nextTick, reactive, ref, watch } from 'vue';
import { cloneDeep } from 'lodash';
import { Message } from 'bkui-vue';

import { VendorEnum } from '@/common/constant';
import { ChargeTypeMap } from '@/typings/plan';
import { type HostApplySuborder } from '@/hooks/chatbot/types';
import {
  getRegionOptions,
  getDeviceTypeOptions,
  getImageOptions,
  getRequireTypeOptions,
  getResAssignOptions,
  getDiskTypeOptions,
} from '@/hooks/chatbot/use-host-apply-options';

interface Props {
  suborder: HostApplySuborder;
  // 云厂商，本期主机申领固定自研云
  vendor?: VendorEnum;
  // 全屏覆盖层(z-index 10002)内编辑时需抬高弹窗层级，避免被遮挡
  zIndex?: number;
}

const visible = defineModel<boolean>('visible', { default: false });
const props = withDefaults(defineProps<Props>(), { vendor: VendorEnum.ZIYAN });
const emit = defineEmits<{ save: [suborder: HostApplySuborder] }>();

const formRef = ref();

const createState = (suborder: HostApplySuborder) => ({
  replicas: suborder.replicas ?? 1,
  region: suborder.region ?? '',
  zone: suborder.zone ?? '',
  device_type: suborder.device_type ?? '',
  image_id: suborder.image_id ?? '',
  res_assign: suborder.res_assign ?? '',
  require_type: suborder.require_type ?? '',
  charge_text: suborder.charge_type
    ? ChargeTypeMap[suborder.charge_type as keyof typeof ChargeTypeMap] ?? suborder.charge_type
    : '',
  system_disk: {
    disk_type: suborder.system_disk?.disk_type ?? 'CLOUD_PREMIUM',
    disk_size: suborder.system_disk?.disk_size ?? 50,
    disk_num: suborder.system_disk?.disk_num ?? 1,
  },
  data_disk: cloneDeep(suborder.data_disk ?? []),
});

const formModel = reactive(createState(props.suborder));

// 回填期间置位，避免回填触发地域 watch 误清空依赖字段
let isHydrating = false;

// 每次打开以最新 suborder 回填初始值
watch(visible, (val) => {
  if (val) {
    isHydrating = true;
    Object.assign(formModel, createState(props.suborder));
    nextTick(() => {
      isHydrating = false;
    });
  }
});

// 地域变更级联清空可用区/机型/操作系统（仅用户改动时，回填不清空）
watch(
  () => formModel.region,
  () => {
    if (isHydrating) return;
    formModel.zone = '';
    formModel.device_type = '';
    formModel.image_id = '';
  },
);

const rules = {
  replicas: [{ required: true, message: '请输入申请数量', trigger: 'blur' }],
  region: [{ required: true, message: '请选择地域', trigger: 'change' }],
  device_type: [{ required: true, message: '请选择机型', trigger: 'change' }],
};

const handleCancel = () => {
  visible.value = false;
};

const handleSave = async () => {
  try {
    await formRef.value?.validate();
  } catch {
    Message({ theme: 'error', message: '请完善必填项' });
    return;
  }
  // 回写为完整 suborder（保留原 charge_type，计费模式自动推导不可编辑）
  const next: HostApplySuborder = {
    ...cloneDeep(props.suborder),
    replicas: Number(formModel.replicas),
    region: formModel.region,
    zone: formModel.zone,
    device_type: formModel.device_type,
    image_id: formModel.image_id,
    res_assign: formModel.res_assign,
    require_type: formModel.require_type,
    system_disk: {
      disk_type: formModel.system_disk.disk_type,
      disk_size: Number(formModel.system_disk.disk_size),
      disk_num: Number(formModel.system_disk.disk_num),
    },
    data_disk: formModel.data_disk.map((d) => ({
      disk_type: d.disk_type,
      disk_size: Number(d.disk_size),
      disk_num: Number(d.disk_num),
    })),
  };
  emit('save', next);
  visible.value = false;
};
</script>

<template>
  <bk-dialog v-model:is-show="visible" title="调整配置" :width="640" :quick-close="false" :z-index="zIndex">
    <bk-form ref="formRef" class="ha-form" :model="formModel" :rules="rules" form-type="default" :label-width="120">
      <bk-form-item label="申请数量" property="replicas" required>
        <bk-input v-model="formModel.replicas" type="number" :min="1" />
      </bk-form-item>
      <bk-form-item label="需求类型" property="require_type">
        <hcm-form-list
          v-model="formModel.require_type"
          :list="() => getRequireTypeOptions()"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择需求类型"
        />
      </bk-form-item>
      <bk-form-item label="地域" property="region" required>
        <hcm-form-list
          v-model="formModel.region"
          :list="() => getRegionOptions(vendor)"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择地域"
        />
      </bk-form-item>
      <bk-form-item label="机型" property="device_type" required>
        <hcm-form-list
          v-model="formModel.device_type"
          :list="() => getDeviceTypeOptions({ vendor, region: formModel.region })"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请先选择地域"
        />
      </bk-form-item>
      <bk-form-item label="操作系统" property="image_id">
        <hcm-form-list
          v-model="formModel.image_id"
          :list="() => getImageOptions(formModel.region)"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择操作系统"
        />
      </bk-form-item>
      <bk-form-item label="资源分配方式" property="res_assign">
        <hcm-form-list
          v-model="formModel.res_assign"
          :list="getResAssignOptions()"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择资源分配方式"
        />
      </bk-form-item>
      <bk-form-item label="计费模式">
        <bk-input v-model="formModel.charge_text" class="ha-charge-input" readonly />
        <div class="ha-form-tip">基于预测状态自动推导</div>
      </bk-form-item>
      <bk-form-item label="系统盘" required>
        <div class="ha-disk-row">
          <hcm-form-list
            v-model="formModel.system_disk.disk_type"
            class="ha-disk-type"
            :list="() => getDiskTypeOptions()"
            :id-key="'id'"
            :display-key="'name'"
          />
          <bk-input v-model="formModel.system_disk.disk_size" class="ha-disk-size" type="number" :min="1" suffix="GB">
            <template #prefix><span class="ha-input-affix">容量</span></template>
          </bk-input>
        </div>
      </bk-form-item>
      <bk-form-item label="数据盘">
        <div v-for="(disk, index) in formModel.data_disk" :key="index" class="ha-disk-row ha-data-disk">
          <hcm-form-list
            v-model="disk.disk_type"
            class="ha-disk-type"
            :list="() => getDiskTypeOptions()"
            :id-key="'id'"
            :display-key="'name'"
          />
          <bk-input v-model="disk.disk_size" class="ha-disk-size" type="number" :min="1" suffix="GB">
            <template #prefix><span class="ha-input-affix">容量</span></template>
          </bk-input>
          <bk-input v-model="disk.disk_num" class="ha-disk-num" type="number" :min="1">
            <template #prefix><span class="ha-input-affix">数量</span></template>
          </bk-input>
        </div>
      </bk-form-item>
    </bk-form>

    <template #footer>
      <bk-button theme="primary" @click="handleSave">保存修改</bk-button>
      <bk-button class="ha-dialog-footer-cancel" @click="handleCancel">取消</bk-button>
    </template>
  </bk-dialog>
</template>

<style scoped lang="scss">
// 表单填写区整体灰底（设计稿），输入框/下拉保持白底悬浮其上
.ha-form {
  padding: 16px 24px;
  background: #f5f7fa;
  border-radius: 2px;

  // 末项的自带底部间距会让灰底区上下视觉边距不一致，置零后由 padding 统一上下边距
  :deep(.bk-form-item:last-child) {
    margin-bottom: 0;
  }
}

.ha-form-tip {
  margin-top: 4px;
  font-size: 12px;
  line-height: 18px;
  color: #979ba5;
}

// 计费模式只读项：扁平浅灰只读框（参考图1），浅边框 + 灰字，区别于灰底填写区
.ha-charge-input {
  :deep(.bk-input) {
    background-color: #fafbfd;
    border-color: #f0f1f5;
  }

  /* stylelint-disable-next-line selector-class-pattern */
  :deep(.bk-input--text) {
    background-color: #fafbfd;
    color: #63656e;
  }
}

.ha-disk-row {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;

  .ha-disk-type {
    flex: 0 0 140px;
  }

  .ha-disk-size {
    flex: 1;
    min-width: 0;
  }

  .ha-disk-num {
    flex: 0 0 110px;
  }
}

// 输入框前缀（容量/数量）：灰底 addon 样式，对齐设计稿
.ha-input-affix {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  height: 100%;
  padding: 0 8px;
  color: #63656e;
  white-space: nowrap;
  background: #f5f7fa;
  border-right: 1px solid #dcdee5;
}

.ha-data-disk {
  margin-bottom: 8px;
}

.ha-dialog-footer-cancel {
  margin-left: 8px;
}
</style>
