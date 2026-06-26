<script setup lang="ts">
import { computed, ref, useTemplateRef, watch, watchEffect } from 'vue';
import { useI18n } from 'vue-i18n';
import http from '@/http';
import CvmVpcSelector, { type ICvmVpc } from '@/views/ziyanScr/components/cvm-vpc-selector/index.vue';
import CvmSubnetSelector, { type ICvmSubnet } from '@/views/ziyanScr/components/cvm-subnet-selector/index.vue';
import BcsSelectTips from '../application-form/bcs-select-tips';

interface IProps {
  region: string;
  zone: string;
  disabled?: boolean;
  disabledVpc?: boolean;
  disabledSubnet?: boolean;
  vpcProperty?: string;
  subnetProperty?: string;
}

/** 配置类型 */
type ConfigType = 'auto' | 'pressure-test';

/** 压测子网索引：{ [region]: { [vpcId]: subnetId[] } } */
interface IPressureIndex {
  [region: string]: {
    [vpcId: string]: string[];
  };
}

/** 压测子网接口响应结构 */
interface ILoadTestSubnetsResponse {
  data: IPressureIndex;
}

defineOptions({ name: 'internet-info-collapse-panel' });

const vpc = defineModel<string>('vpc');
const subnet = defineModel<string>('subnet');

const props = withDefaults(defineProps<IProps>(), {
  disabled: false,
  disabledVpc: false,
  disabledSubnet: false,
  vpcProperty: 'spec.vpc',
  subnetProperty: 'spec.subnet',
});
const emit = defineEmits<{
  (e: 'changeVpc', val: ICvmVpc): void;
  (e: 'changeSubnet', val: ICvmSubnet): void;
}>();

const { t } = useI18n();

// ===== 原有状态 =====
const isExpand = ref(false);
const selectCvmVpc = ref<ICvmVpc>(null);
const selectedCvmSubnet = ref<ICvmSubnet>(null);
const cvmVpcSelectorRef = useTemplateRef('cvm-vpc-selector');
const cvmSubnetSelectorRef = useTemplateRef('cvm-subnet-selector');

// ===== 新增：配置类型 =====
const configType = ref<ConfigType>('auto');

// ===== 新增：压测索引 =====
const pressureIndex = ref<IPressureIndex>({});

// ===== 新增：压测选项是否可用 =====
const pressureTestDisabled = computed(() => {
  const regionData = pressureIndex.value[props.region];
  return !regionData || Object.keys(regionData).length === 0;
});

// ===== 新增：过滤函数（传给选择器，由选择器内部请求数据后调用） =====
const vpcFilter = computed(() => {
  const regionPressure = pressureIndex.value[props.region];
  if (!regionPressure) return undefined;
  const pressureVpcIds = new Set(Object.keys(regionPressure));
  // auto 模式：VPC 列表展示全部，不做排除
  return configType.value === 'auto'
    ? undefined
    : (list: ICvmVpc[]) => list.filter((v) => pressureVpcIds.has(v.vpc_id));
});

const subnetFilter = computed(() => {
  const regionPressure = pressureIndex.value[props.region];
  if (!regionPressure) return undefined;
  const vpcId = vpc.value;
  const pressureSubnetIds = new Set(regionPressure[vpcId] || []);
  return configType.value === 'auto'
    ? (list: ICvmSubnet[]) => list.filter((s) => !pressureSubnetIds.has(s.subnet_id))
    : (list: ICvmSubnet[]) => list.filter((s) => pressureSubnetIds.has(s.subnet_id));
});

// ===== 新增：请求函数 =====
const fetchPressureIndex = async () => {
  try {
    const res = await http.get<ILoadTestSubnetsResponse>('/api/v1/woa/config/load_test_subnets');
    pressureIndex.value = res.data || {};
  } catch {
    pressureIndex.value = {};
  }
};

// 压测索引：组件挂载时获取一次
fetchPressureIndex();

// ===== 新增：configType 切换逻辑 =====
const handleConfigTypeChange = () => {
  vpc.value = '';
  subnet.value = '';
  selectCvmVpc.value = null;
  selectedCvmSubnet.value = null;
};

// 当 region 变化且无压测配置时强制切回自动匹配
watch([() => props.region, pressureIndex], () => {
  if (pressureTestDisabled.value && configType.value === 'pressure-test') {
    configType.value = 'auto';
  }
});

// ===== 修改场景自动推断 =====
const autoInferConfigType = () => {
  const regionData = pressureIndex.value[props.region];
  if (!regionData) {
    configType.value = 'auto';
    return;
  }
  const vpcVal = vpc.value;
  const subnetVal = subnet.value;
  const isPressureVpc = vpcVal ? vpcVal in regionData : false;
  const isPressureSubnet = subnetVal && vpcVal ? regionData[vpcVal]?.includes(subnetVal) : false;
  configType.value = isPressureVpc || isPressureSubnet ? 'pressure-test' : 'auto';
};

// ===== 原有逻辑（保持不变） =====
const handleToggle = (v: boolean) => {
  isExpand.value = v;
  // 首次展开时执行自动推断
  if (v) {
    autoInferConfigType();
  }
};

const handleCvmVpcChange = (val?: ICvmVpc) => {
  selectCvmVpc.value = val;
  selectedCvmSubnet.value = null;
  subnet.value = '';
  emit('changeVpc', val);
};

const handleCvmSubnetChange = (val?: ICvmSubnet) => {
  selectedCvmSubnet.value = val;
  emit('changeSubnet', val);
};

watchEffect(() => {
  selectCvmVpc.value = cvmVpcSelectorRef.value?.findCvmVpcByVpcId(vpc.value);
});
watchEffect(() => {
  selectedCvmSubnet.value = cvmSubnetSelectorRef.value?.findCvmSubnetBySubnetId(subnet.value);
});
watchEffect(() => {
  if (props.disabled) {
    isExpand.value = false;
    vpc.value = '';
    subnet.value = '';
  }
});

defineExpose({ handleToggle });
</script>

<template>
  <bk-collapse class="home">
    <bk-collapse-panel
      :model-value="isExpand"
      icon="right-shape"
      alone
      :disabled="disabled"
      @update:model-value="handleToggle"
    >
      <template #default>
        <span class="network-header">
          <span class="network-title">{{ t('网络信息') }}</span>
          <span class="network-tip">
            <i class="hcm-icon bkhcm-icon-info-line" />
            <span class="network-tip-text">[VPC] 与 [子网] 均默认系统自动匹配，若需用于压测场景可手动切换</span>
          </span>
        </span>
        <span v-if="!isExpand" class="overview">
          <span>
            {{ t('VPC：') }}
            {{ selectCvmVpc ? `${selectCvmVpc.vpc_name}（${selectCvmVpc.vpc_id}）` : t('系统自动分配') }}
          </span>
          <span>
            {{ t('子网：') }}
            {{
              selectedCvmSubnet
                ? `${selectedCvmSubnet.subnet_name}（${selectedCvmSubnet.subnet_id}）`
                : t('系统自动分配')
            }}
          </span>
        </span>
      </template>

      <template #content>
        <bk-form-item v-if="!props.disabled" label="配置类型" required class="config-type-form-item">
          <bk-radio-group v-model="configType" @change="handleConfigTypeChange">
            <bk-radio-button label="auto">{{ t('自动匹配') }}</bk-radio-button>
            <bk-radio-button
              label="pressure-test"
              :disabled="pressureTestDisabled"
              v-bk-tooltips="{
                content: t('当前地域无压测子网'),
                disabled: !pressureTestDisabled,
              }"
            >
              {{ t('用于压测') }}
            </bk-radio-button>
          </bk-radio-group>
        </bk-form-item>

        <template v-if="configType === 'pressure-test'">
          <div class="pressure-test-notice">
            <bk-alert type="warning" :title="t('压测子网使用须知')">
              <ul>
                <li>{{ t('独立出口：使用专用 NAT 及出口 IP，请注意安全组、防火墙策略配置') }}</li>
                <li>{{ t('禁止生产：本子网不建议正式业务部署，仅供压测使用') }}</li>
                <li>{{ t('及时释放：压测结束请立即切换子网，避免占用压测配额') }}</li>
              </ul>
            </bk-alert>
          </div>
        </template>

        <bk-form-item label="VPC" :property="vpcProperty">
          <cvm-vpc-selector
            class="cvm-vpc-selector"
            ref="cvm-vpc-selector"
            v-model="vpc"
            :region="props.region"
            :disabled="props.disabledVpc"
            :filter="vpcFilter"
            :popover-options="{ boundary: 'parent' }"
            @change="handleCvmVpcChange"
          />
          <!-- 如果选择BSC集群的VPC，提供引导提示 -->
          <bcs-select-tips v-if="/(BCS|OVERLAY)/.test(selectCvmVpc?.vpc_name)" :desc="t('所选择的VPC为容器网络')" />
        </bk-form-item>
        <bk-form-item :label="t('子网')" :property="subnetProperty">
          <cvm-subnet-selector
            class="cvm-subnet-selector"
            ref="cvm-subnet-selector"
            v-model="subnet"
            :region="props.region"
            :zone="props.zone"
            :vpc="vpc"
            :disabled="props.disabledSubnet"
            :filter="subnetFilter"
            :popover-options="{ boundary: 'parent' }"
            @change="handleCvmSubnetChange"
          />
          <!-- 如果选择BSC集群的子网，提供引导提示 -->
          <bcs-select-tips
            v-if="/(BCS|OVERLAY)/.test(selectedCvmSubnet?.subnet_name)"
            :desc="t('所选择的子网为容器子网')"
          />
        </bk-form-item>

        <!-- tips -->
        <div class="tips">
          <slot name="tips"></slot>
        </div>
      </template>
    </bk-collapse-panel>
  </bk-collapse>
</template>

<style scoped lang="scss">
.home {
  background-color: #fff;
  box-shadow: 0 2px 4px 0 #1919290d;

  :deep(.bk-collapse-title) {
    font-weight: 700;
  }

  :deep(.bk-collapse-content) {
    padding: 16px 24px;
  }

  .network-header {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-weight: 700;
  }

  .network-title {
    color: #313238;
  }

  .network-tip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-weight: normal;
    font-size: 12px;
    color: #979ba5;

    .network-tip-text {
      max-width: 400px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .overview {
    margin-left: 56px;
    display: inline-flex;
    gap: 40px;
    font-weight: normal;
    color: #313238;
  }

  .pressure-test-notice {
    margin-bottom: 16px;

    ul {
      margin: 4px 0 0;
      padding-left: 16px;
      list-style: disc;

      li {
        font-size: 12px;
        line-height: 20px;
        color: #63656e;
      }
    }
  }

  .tips {
    font-size: 12px;
  }
}
</style>
