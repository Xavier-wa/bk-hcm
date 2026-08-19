<script setup lang="ts">
import { inject, Ref, ref, shallowReactive, useTemplateRef, watch } from 'vue';
import { Close, Success } from 'bkui-vue/lib/icon';
import { timeFormatter } from '@/common/util';
import { useCvmDeviceStore, type IInheritCvm, type IInheritedHostGroup } from '@/store/cvm/device';
import { RequirementType } from '@/store/config/requirement';
import useCvmChargeType from '@/views/ziyanScr/hooks/use-cvm-charge-type';
import AssetList from './asset-list.vue';

const model = defineModel<string>();

const props = defineProps<{
  bizId: number;
  region: string;
  requireType: RequirementType;
  inheritInstanceId: string;
  /** 当前选中的机型族（来自 device-type-dialog 的 condition.deviceGroup） */
  deviceGroup?: string;
  /** 是否启用固资号推荐（仅滚服类型；裁撤类型保持纯输入框+手动校验的原逻辑） */
  enableRecommend?: boolean;
}>();

const emit = defineEmits<{
  checkSuccess: [cvm: IInheritCvm];
  checkFail: [];
  /** 下拉内 Tab 切换时同步到外部机型族 */
  deviceGroupChange: [deviceGroup: string];
}>();

const cvmDeviceStore = useCvmDeviceStore();

const { getMonthName, cvmChargeTypeNames } = useCvmChargeType();

const isInfoMode = inject<Ref<boolean>>('isInfoMode');

const isShowDetails = ref(false);

const isDropdownOpen = ref(false);

const selectRef = useTemplateRef('selectRef');

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

const handleCheck = async (assetId?: string) => {
  const id = assetId ?? model.value;
  if (!id) {
    inheritCvm.value = null;
    checkState.error = undefined;
    checkState.message = '';
    emit('checkFail');
    return;
  }
  try {
    const res = await cvmDeviceStore.getInheritCvm(
      {
        require_type: props.requireType,
        bk_biz_id: props.bizId,
        bk_asset_id: id,
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

// 从下拉列表选中固资号后自动校验并关闭下拉
const handleAssetSelect = (assetId: string) => {
  model.value = assetId;
  handleCheck(assetId);
  // 关闭 bk-select 下拉（hidePopover 为组件公开方法）
  (selectRef.value as any)?.hidePopover();
};

// 下拉内 Tab 切换时自动推荐对应机型族的第一个固资号，并同步外部机型族
const handleTabChange = (deviceFamily: string) => {
  emit('deviceGroupChange', deviceFamily);
  const assetId = getAutoFillAssetId(hostGroups.value, deviceFamily);
  if (assetId) handleAssetSelect(assetId);
};

// 机型族固定列表（无"全部"Tab），用于向接口请求候选
const DEVICE_FAMILIES = ['标准型', '高IO型', '大数据型', '计算型', 'GPU型'];

// 候选固资分组数据（提升到父级：region 变化时即触发请求与回填，不依赖下拉是否打开）
const hostGroups = ref<IInheritedHostGroup[]>([]);

// 当前激活的机型族 Tab（托管在父级，关闭下拉后状态可保留）
const activeTab = ref('');

// 自动回填规则：优先标准型，否则取第一个非空分组
const getAutoFillAssetId = (groups: IInheritedHostGroup[], preferredFamily?: string): string => {
  const visible = groups.filter((g) => g.hosts.length > 0);
  // 若指定了机型族，优先匹配该分组
  if (preferredFamily) {
    const target = visible.find((g) => g.device_family === preferredFamily);
    if (target?.hosts[0]) return target.hosts[0].bk_asset_id;
    // 指定机型族无候选时不回填
    return '';
  }
  const standard = visible.find((g) => g.device_family === '标准型');
  const target = standard ?? visible[0];
  return target?.hosts[0]?.bk_asset_id ?? '';
};

// region（可用区）变化时仅拉取候选数据（不自动回填）
// 回填时机：(1) 弹窗首次打开且 model 为空 (2) 机型族切换 (3) 下拉 Tab 切换
// 仅滚服类型启用固资号推荐，裁撤类型不拉取推荐数据
watch(
  () => props.region,
  async (region) => {
    if (!props.enableRecommend) return;
    if (!region || !props.bizId) {
      hostGroups.value = [];
      return;
    }
    try {
      const groups = await cvmDeviceStore.getInheritedHostList({
        bk_biz_id: props.bizId,
        region,
        device_families: DEVICE_FAMILIES,
      });
      hostGroups.value = groups;
    } catch (error) {
      console.error(error);
      hostGroups.value = [];
    }
  },
  { immediate: true },
);

// 弹窗首次打开且 model 为空时自动推荐一个固资号（仅滚服类型）
let isFirstMount = true;
watch(
  () => hostGroups.value.length,
  () => {
    if (!props.enableRecommend) return;
    if (!isFirstMount) return;
    isFirstMount = false;
    if (!model.value && hostGroups.value.length > 0) {
      const assetId = getAutoFillAssetId(hostGroups.value);
      if (assetId) handleAssetSelect(assetId);
    }
  },
);

// 机型族切换（来自父级弹窗的 condition.deviceGroup，仅滚服类型联动）
watch(
  () => props.deviceGroup,
  (deviceGroup) => {
    if (!props.enableRecommend) return;
    if (!deviceGroup || hostGroups.value.length === 0) return;
    if (deviceGroup === '全部') {
      // 切换到"全部机型"时清空固资号
      model.value = '';
      handleCheck('');
      return;
    }
    // 切换到具体机型族时推荐该族的第一个固资号，并同步下拉 Tab 状态
    activeTab.value = deviceGroup;
    const assetId = getAutoFillAssetId(hostGroups.value, deviceGroup);
    if (assetId) {
      handleAssetSelect(assetId);
    } else {
      // 对应机型族无候选固资号时清空
      model.value = '';
      handleCheck('');
    }
  },
);

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
    <!-- 滚服类型：固资号推荐下拉；裁撤类型：纯输入框（保持原逻辑） -->
    <bk-select
      v-if="enableRecommend"
      ref="selectRef"
      allow-create
      :clearable="true"
      custom-content
      :popover-min-width="900"
      :scroll-height="500"
      class="asset-id-input"
      @toggle="isDropdownOpen = $event"
    >
      <!-- 自定义下拉框内容为空白 -->
      <template #default>
        <AssetList
          v-if="isDropdownOpen"
          :host-groups="hostGroups"
          :active-tab="activeTab"
          @update:active-tab="(tab) => (activeTab = tab)"
          @select="handleAssetSelect"
          @tab-change="handleTabChange"
        />
      </template>

      <!-- 自定义触发器，隐藏右侧箭头 -->
      <template #trigger>
        <bk-input behavior="simplicity" size="small" v-model="model" />
      </template>
    </bk-select>
    <bk-input v-else behavior="simplicity" class="asset-id-input" size="small" v-model="model" />
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
      @click="() => handleCheck()"
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
