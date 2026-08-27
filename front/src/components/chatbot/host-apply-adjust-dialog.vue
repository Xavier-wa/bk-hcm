<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue';
import { cloneDeep } from 'lodash';
import { Message } from 'bkui-vue';
import { Plus } from 'bkui-vue/lib/icon';

import { VendorEnum } from '@/common/constant';
import { ChargeTypeMap } from '@/typings/plan';
import { type HostApplyDisk, type HostApplySuborder } from '@/hooks/chatbot/types';
import { CVM_DATA_DISK_INFO, type CvmDataDiskType } from '@/views/ziyanScr/components/cvm-data-disk/constants';
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

// 下拉选项工厂固化在 setup 作用域：hcm-form-list 内部用 watchEffect 追踪 list prop，
// 写成内联箭头函数会因每次重渲染都是新引用而重复拉取接口（改容量/数量即触发）。
// 函数体内对 formModel 的读取仍在 watchEffect 的同步追踪范围内，级联刷新不受影响。
const requireTypeList = () => getRequireTypeOptions();
const regionList = () => getRegionOptions(props.vendor);
const deviceTypeList = () => getDeviceTypeOptions({ vendor: props.vendor, region: formModel.region });
const imageList = () => getImageOptions(formModel.region);
const diskTypeList = () => getDiskTypeOptions();
const resAssignList = getResAssignOptions();

// 数据盘总块数上限，与后端 constant.DataDiskTotalNum 保持一致
const MAX_DATA_DISK_NUM = 20;

// 新增行沿用推荐方案的数据盘默认值，删除后再添加可还原成初始配置
const createDataDisk = (): HostApplyDisk => ({ disk_type: 'CLOUD_PREMIUM', disk_size: 500, disk_num: 1 });

const dataDiskTotalNum = computed(() =>
  formModel.data_disk.reduce((total, disk) => total + Number(disk.disk_num || 0), 0),
);

const handleAddDataDisk = () => {
  formModel.data_disk.push(createDataDisk());
};

const handleRemoveDataDisk = (index: number) => {
  formModel.data_disk.splice(index, 1);
};

// 容量取值范围随磁盘类型而变（如 SSD 云硬盘下限 20G、高性能云盘 10G），与 ziyanScr 数据盘编辑器同一份常量；
// 本地盘类型无区间配置，回退到后端 DataDiskMinSize / DataDiskMaxSize 的兜底值
const getDiskSizeMin = (diskType?: string) => CVM_DATA_DISK_INFO[diskType as CvmDataDiskType]?.min ?? 0;
const getDiskSizeMax = (diskType?: string) => CVM_DATA_DISK_INFO[diskType as CvmDataDiskType]?.max ?? 32000;

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
  // 数据盘整体非必填（零块合法），但已存在的行三个字段都不能为空——与 ziyanScr 申领表单同一条规则。
  // 空容量/空数量经 Number('') 会静默变成 0，后端按 [10, 32000] 拒单，故必须在前端先拦住
  data_disk: [
    {
      validator: (value: HostApplyDisk[]) => {
        if (value.length === 0) return true;
        return value.every((item) => item.disk_type && item.disk_size && item.disk_num);
      },
      message: '数据盘信息不能为空',
      trigger: 'change',
    },
  ],
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
  <bk-dialog v-model:is-show="visible" title="调整配置" :width="800" :quick-close="false" :z-index="zIndex">
    <bk-form ref="formRef" class="ha-form" :model="formModel" :rules="rules" form-type="default" :label-width="120">
      <bk-form-item label="申请数量" property="replicas" required>
        <bk-input v-model="formModel.replicas" type="number" :min="1" />
      </bk-form-item>
      <bk-form-item label="需求类型" property="require_type">
        <hcm-form-list
          v-model="formModel.require_type"
          :list="requireTypeList"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择需求类型"
        />
      </bk-form-item>
      <bk-form-item label="地域" property="region" required>
        <hcm-form-list
          v-model="formModel.region"
          :list="regionList"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择地域"
        />
      </bk-form-item>
      <bk-form-item label="机型" property="device_type" required>
        <hcm-form-list
          v-model="formModel.device_type"
          :list="deviceTypeList"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请先选择地域"
        />
      </bk-form-item>
      <bk-form-item label="操作系统" property="image_id">
        <hcm-form-list
          v-model="formModel.image_id"
          :list="imageList"
          :id-key="'id'"
          :display-key="'name'"
          placeholder="请选择操作系统"
        />
      </bk-form-item>
      <bk-form-item label="资源分配方式" property="res_assign">
        <hcm-form-list
          v-model="formModel.res_assign"
          :list="resAssignList"
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
            :list="diskTypeList"
            :id-key="'id'"
            :display-key="'name'"
          />
          <bk-input v-model="formModel.system_disk.disk_size" class="ha-disk-size" type="number" :min="1" suffix="GB">
            <template #prefix><span class="ha-input-affix">容量</span></template>
          </bk-input>
        </div>
      </bk-form-item>
      <bk-form-item property="data_disk">
        <template #label>
          数据盘
          <i
            v-bk-tooltips="{ content: '数据盘大小范围为20G-32000G，且为10的倍数' }"
            class="hcm-icon bkhcm-icon-prompt text-gray cursor ha-label-tip"
          ></i>
        </template>
        <bk-button v-if="formModel.data_disk.length === 0" @click="handleAddDataDisk">
          <plus class="ha-disk-add-icon" />
        </bk-button>
        <div v-for="(disk, index) in formModel.data_disk" :key="index" class="ha-disk-row ha-data-disk">
          <hcm-form-list
            v-model="disk.disk_type"
            class="ha-disk-type"
            :list="diskTypeList"
            :id-key="'id'"
            :display-key="'name'"
          />
          <bk-input
            v-model="disk.disk_size"
            class="ha-disk-size"
            type="number"
            :step="10"
            :min="getDiskSizeMin(disk.disk_type)"
            :max="getDiskSizeMax(disk.disk_type)"
            suffix="GB"
          >
            <template #prefix><span class="ha-input-affix">容量</span></template>
          </bk-input>
          <bk-input
            v-model="disk.disk_num"
            class="ha-disk-num"
            type="number"
            :min="1"
            :max="MAX_DATA_DISK_NUM - dataDiskTotalNum + Number(disk.disk_num || 0)"
          >
            <template #prefix><span class="ha-input-affix">数量</span></template>
          </bk-input>
          <bk-button
            v-bk-tooltips="{
              content: `数据盘总块数最多 ${MAX_DATA_DISK_NUM} 块`,
              disabled: dataDiskTotalNum < MAX_DATA_DISK_NUM,
            }"
            class="ha-disk-action"
            text
            :disabled="dataDiskTotalNum >= MAX_DATA_DISK_NUM"
            @click="handleAddDataDisk"
          >
            <i class="hcm-icon bkhcm-icon-plus-circle-shape"></i>
          </bk-button>
          <bk-button class="ha-disk-action" text @click="handleRemoveDataDisk(index)">
            <i class="hcm-icon bkhcm-icon-minus-circle-shape"></i>
          </bk-button>
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

  // 容量框留出下限：数据盘行还要放数量框与增删按钮，仅靠 flex 收缩会把数值挤没
  .ha-disk-size {
    flex: 1;
    min-width: 160px;
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

// 行内增删按钮：与 ziyanScr 数据盘编辑器一致的浅灰圆形加减号
.ha-disk-action {
  flex: 0 0 auto;

  .hcm-icon {
    font-size: 16px;
    color: #c4c6cc;
  }

  &.is-disabled .hcm-icon {
    color: #eaebf0;
  }
}

// 数据盘删空后行内按钮一并消失，用大加号按钮兜住空态入口
.ha-disk-add-icon {
  font-size: 24px;
}

// 表单项标签后的说明图标（项目里常配的 ml4 工具类实际未定义，此处自带间距）
.ha-label-tip {
  margin-left: 4px;
}

.ha-dialog-footer-cancel {
  margin-left: 8px;
}
</style>
