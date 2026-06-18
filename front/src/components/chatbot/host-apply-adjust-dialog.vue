<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import { cloneDeep } from 'lodash';
import { Message } from 'bkui-vue';
import { Plus, Close } from 'bkui-vue/lib/icon';

import { type HostApplySuborder } from '@/hooks/chatbot/types';

interface Props {
  suborder: HostApplySuborder;
  // 全屏覆盖层(z-index 10002)内编辑时需抬高弹窗层级，避免被遮挡
  zIndex?: number;
}

const visible = defineModel<boolean>('visible', { default: false });
const props = defineProps<Props>();
const emit = defineEmits<{ save: [suborder: HostApplySuborder] }>();

// 磁盘类型候选暂用静态枚举（动态来源待后端，见 api.md）
const DISK_TYPE_OPTIONS = ['CLOUD_PREMIUM', 'CLOUD_SSD', 'CLOUD_BASIC', 'CLOUD_HSSD'];

const formRef = ref();

const createState = (suborder: HostApplySuborder) => ({
  replicas: suborder.replicas ?? 1,
  region: suborder.region ?? '',
  device_type: suborder.device_type ?? '',
  image_id: suborder.image_id ?? '',
  res_assign: suborder.res_assign ?? '',
  require_type: suborder.require_type ?? '',
  charge_text: suborder.charge_type ?? '',
  system_disk: {
    disk_type: suborder.system_disk?.disk_type ?? 'CLOUD_PREMIUM',
    disk_size: suborder.system_disk?.disk_size ?? 50,
    disk_num: suborder.system_disk?.disk_num ?? 1,
  },
  data_disk: cloneDeep(suborder.data_disk ?? []),
});

const formModel = reactive(createState(props.suborder));

// 每次打开以最新 suborder 回填初始值
watch(visible, (val) => {
  if (val) Object.assign(formModel, createState(props.suborder));
});

const rules = {
  replicas: [{ required: true, message: '请输入申请数量', trigger: 'blur' }],
  region: [{ required: true, message: '请输入地域', trigger: 'blur' }],
  device_type: [{ required: true, message: '请输入机型', trigger: 'blur' }],
};

const addDataDisk = () => {
  formModel.data_disk.push({ disk_type: 'SSD', disk_size: 100, disk_num: 1 });
};
const removeDataDisk = (index: number) => {
  formModel.data_disk.splice(index, 1);
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
    <bk-form ref="formRef" :model="formModel" :rules="rules" form-type="default" :label-width="120">
      <bk-form-item label="申请数量" property="replicas" required>
        <bk-input v-model="formModel.replicas" type="number" :min="1" />
      </bk-form-item>
      <bk-form-item label="地域" property="region" required>
        <bk-input v-model="formModel.region" placeholder="请输入地域" />
      </bk-form-item>
      <bk-form-item label="机型" property="device_type" required>
        <bk-input v-model="formModel.device_type" placeholder="请输入机型" />
      </bk-form-item>
      <bk-form-item label="操作系统" property="image_id">
        <bk-input v-model="formModel.image_id" placeholder="请输入操作系统" />
      </bk-form-item>
      <bk-form-item label="资源分配方式" property="res_assign">
        <bk-input v-model="formModel.res_assign" placeholder="请输入资源分配方式" />
      </bk-form-item>
      <bk-form-item label="需求类型" property="require_type">
        <bk-input v-model="formModel.require_type" placeholder="请输入需求类型" />
      </bk-form-item>
      <bk-form-item label="计费模式">
        <bk-input v-model="formModel.charge_text" readonly />
        <div class="ha-form-tip">基于预测状态自动推导</div>
      </bk-form-item>
      <bk-form-item label="系统盘" required>
        <div class="ha-disk-row">
          <bk-select v-model="formModel.system_disk.disk_type" class="ha-disk-type" :clearable="false">
            <bk-option v-for="t in DISK_TYPE_OPTIONS" :key="t" :id="t" :name="t" />
          </bk-select>
          <bk-input v-model="formModel.system_disk.disk_size" type="number" :min="1" suffix="GB" />
        </div>
      </bk-form-item>
      <bk-form-item label="数据盘">
        <div v-for="(disk, index) in formModel.data_disk" :key="index" class="ha-disk-row ha-data-disk">
          <bk-select v-model="disk.disk_type" class="ha-disk-type" :clearable="false">
            <bk-option v-for="t in DISK_TYPE_OPTIONS" :key="t" :id="t" :name="t" />
          </bk-select>
          <bk-input v-model="disk.disk_size" type="number" :min="1" suffix="GB" />
          <bk-input v-model="disk.disk_num" type="number" :min="1" suffix="块" />
          <Close class="ha-disk-remove" @click="removeDataDisk(index)" />
        </div>
        <bk-button text theme="primary" @click="addDataDisk">
          <Plus />
          <span>添加数据盘</span>
        </bk-button>
      </bk-form-item>
    </bk-form>

    <template #footer>
      <bk-button theme="primary" @click="handleSave">保存修改</bk-button>
      <bk-button class="ha-dialog-footer-cancel" @click="handleCancel">取消</bk-button>
    </template>
  </bk-dialog>
</template>

<style scoped lang="scss">
.ha-form-tip {
  margin-top: 4px;
  font-size: 12px;
  line-height: 18px;
  color: #979ba5;
}

.ha-disk-row {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;

  .ha-disk-type {
    flex: 0 0 180px;
  }
}

.ha-data-disk {
  margin-bottom: 8px;

  .ha-disk-remove {
    flex-shrink: 0;
    font-size: 18px;
    color: #979ba5;
    cursor: pointer;

    &:hover {
      color: #ea3636;
    }
  }
}

.ha-dialog-footer-cancel {
  margin-left: 8px;
}
</style>
