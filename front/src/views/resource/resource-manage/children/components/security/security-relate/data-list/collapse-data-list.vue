<script setup lang="ts">
import { computed, onBeforeMount, ref, useTemplateRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import usePage from '@/hooks/use-page';
import { Senarios, useWhereAmI } from '@/hooks/useWhereAmI';
import { useVerify } from '@/hooks';
import {
  type ISecurityGroupDetail,
  type SecurityGroupRelResourceByBizItem,
  useSecurityGroupStore,
} from '@/store/security-group';
import { useBusinessGlobalStore } from '@/store/business-global';
import { transformSimpleCondition } from '@/utils/search';

import {
  RELATED_RES_NAME_MAP,
  RELATED_RES_OPERATE_DISABLED_TIPS_MAP,
  RelatedResourceOperateType,
  RELATED_RES_PROPERTIES_MAP,
  SecurityGroupRelatedResourceName,
} from '@/constants/security-group';
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';
import HcmDropdown from '@/components/hcm-dropdown/index.vue';
import { AngleDown } from 'bkui-vue/lib/icon';

import dataList from './index.vue';
import bind from '../bind/index.vue';
import batchUnbind from '../unbind/batch.vue';
import singleUnbind from '../unbind/single.vue';

const props = defineProps<{
  detail: ISecurityGroupDetail;
  bkBizId: number;
  tabActive: SecurityGroupRelatedResourceName;
  resCount: number;
  condition: Record<string, any>;
}>();
const emit = defineEmits(['operate-success']);

const { t } = useI18n();
const { getBizsId, whereAmI } = useWhereAmI();
const securityGroupStore = useSecurityGroupStore();
const { getBusinessNames } = useBusinessGlobalStore();

// 预鉴权
const { handleAuth, authVerifyData } = useVerify();
const authAction = computed(() => {
  return whereAmI.value === Senarios.business ? 'biz_iaas_resource_operate' : 'iaas_resource_operate';
});

const isExpand = ref(props.bkBizId === getBizsId());
const iconClass = computed(() => (isExpand.value ? 'bkhcm-icon-angle-up-fill' : 'bkhcm-icon-right-shape'));
const businessName = computed(() => {
  if (props.bkBizId === -1) return t('未分配');
  return getBusinessNames(props.bkBizId)?.[0];
});
const isCurrentBusiness = computed(() => getBizsId() === props.bkBizId);
const isOperateDisabled = computed(() => {
  // 暂不支持负载均衡相关的操作
  return props.tabActive === SecurityGroupRelatedResourceName.CLB;
});

// 复制按钮禁用：当前业务下无数据时置灰
const copyDisabled = computed(() => relResList.value.length === 0);

// 是否显示复制按钮（CVM 和 CLB 都支持）
const showCopyButtons = computed(
  () =>
    props.tabActive === SecurityGroupRelatedResourceName.CVM ||
    props.tabActive === SecurityGroupRelatedResourceName.CLB,
);

// CVM: 内网IPv4 / CLB: 负载均衡IPv4
const copyIPv4Content = computed(() =>
  props.tabActive === SecurityGroupRelatedResourceName.CVM ? getAllPrivateIPv4s : getAllPublicIPv4s,
);
const copyIPv4Text = computed(() =>
  props.tabActive === SecurityGroupRelatedResourceName.CVM ? t('内网IP(ipv4)') : t('负载均衡VIP(ipv4)'),
);

// CVM: 内网IPv6 / CLB: 负载均衡IPv6
const copyIPv6Content = computed(() =>
  props.tabActive === SecurityGroupRelatedResourceName.CVM ? getAllPrivateIPv6s : getAllPublicIPv6s,
);
const copyIPv6Text = computed(() =>
  props.tabActive === SecurityGroupRelatedResourceName.CVM ? t('内网IP(ipv6)') : t('负载均衡VIP(ipv6)'),
);

// 复制主机ID (CVM) / 负载均衡ID (CLB)
const copyIDText = computed(() =>
  props.tabActive === SecurityGroupRelatedResourceName.CVM ? t('主机ID') : t('负载均衡ID'),
);
const copyIDContent = computed(() => getAllCloudIDs);

// 拉取当前业务全部关联资源（带缓存，避免重复请求）
let allRelResCache: Promise<SecurityGroupRelResourceByBizItem[]> | null = null;
const fetchAllRelRes = async (): Promise<SecurityGroupRelResourceByBizItem[]> => {
  if (!allRelResCache) {
    allRelResCache =
      pagination.count <= pagination.limit
        ? Promise.resolve(relResList.value) // 优化：当总数小于等于每页数量时，直接返回当前列表数据
        : securityGroupStore.fetchAllRelatedResourcesByBiz(
            props.detail.id,
            props.bkBizId,
            props.tabActive,
            transformSimpleCondition(props.condition, RELATED_RES_PROPERTIES_MAP[props.tabActive]),
          );
  }
  return allRelResCache;
};

// 复制全部内网 IPv4（CVM 专用）
const getAllPrivateIPv4s = async () => {
  const list = await fetchAllRelRes();
  return list.flatMap((item) => item.private_ipv4_addresses || []).join('\n') || '--';
};

// 复制全部内网 IPv6（CVM 专用）
const getAllPrivateIPv6s = async () => {
  const list = await fetchAllRelRes();
  return list.flatMap((item) => item.private_ipv6_addresses || []).join('\n') || '--';
};

// 复制全部负载均衡 IPv4（CLB 专用）
const getAllPublicIPv4s = async () => {
  const list = await fetchAllRelRes();
  return list.flatMap((item) => item.public_ipv4_addresses || []).join('\n') || '--';
};

// 复制全部负载均衡 IPv6（CLB 专用）
const getAllPublicIPv6s = async () => {
  const list = await fetchAllRelRes();
  return list.flatMap((item) => item.public_ipv6_addresses || []).join('\n') || '--';
};

// 复制全部资源 ID（异步拉取全量后拼接 cloud_id）
const getAllCloudIDs = async () => {
  const list = await fetchAllRelRes();
  return (
    list
      .map((item) => item.cloud_id)
      .filter(Boolean)
      .join('\n') || '--'
  );
};

const relResList = ref<SecurityGroupRelResourceByBizItem[]>([]);
const { pagination, getPageParams } = usePage();

const handleToggle = async () => {
  isExpand.value = !isExpand.value;
  if (isExpand.value) {
    await getList();
  }
};

const loading = ref(false);
const getList = async (
  tabActive = props.tabActive,
  condition = props.condition,
  sort = 'created_at',
  order = 'DESC',
) => {
  loading.value = true;
  allRelResCache = null;
  try {
    const res = await securityGroupStore.queryRelatedResourcesByBiz(props.detail.id, props.bkBizId, tabActive, {
      filter: transformSimpleCondition(condition, RELATED_RES_PROPERTIES_MAP[props.tabActive]),
      page: getPageParams(pagination, { sort, order }),
    });

    relResList.value = res.list;
    // 设置页码总条数
    pagination.count = res.count;
  } finally {
    loading.value = false;
  }
};

const selected = ref<SecurityGroupRelResourceByBizItem[]>([]);
const bindVisible = ref(false);
const batchUnbindVisible = ref(false);
const singleUnbindVisible = ref(false);
const singleUnbindOperateRow = ref<SecurityGroupRelResourceByBizItem>(null);
const handleShowOperateDialog = (
  operate: 'bind' | 'single-unbind' | 'batch-unbind',
  row?: SecurityGroupRelResourceByBizItem,
) => {
  if (!authVerifyData.value?.permissionAction?.[authAction.value]) {
    handleAuth(authAction.value);
    return;
  }
  switch (operate) {
    case 'bind':
      bindVisible.value = true;
      break;
    case 'single-unbind':
      singleUnbindVisible.value = true;
      singleUnbindOperateRow.value = row;
      break;
    case 'batch-unbind':
      batchUnbindVisible.value = true;
      break;
  }
};
const handleOperateSuccess = () => {
  emit('operate-success');
};

const datalistRef = useTemplateRef('data-list');
const reload = (tabActive: SecurityGroupRelatedResourceName, condition: Record<string, any>) => {
  datalistRef.value.handleClear();
  if (pagination.current === 1) {
    getList(tabActive, condition);
  } else {
    pagination.current = 1;
  }
};

watch([() => pagination.current, () => pagination.limit], () => {
  getList();
});

onBeforeMount(() => {
  if (isCurrentBusiness.value) getList();
});

defineExpose({ isExpand, reload });
</script>

<template>
  <div class="collapse-wrap">
    <div class="tools">
      <i class="hcm-icon" :class="iconClass" @click="handleToggle"></i>
      <span class="name">{{ businessName }}</span>
      <!-- 只允许对本业务的实例进行绑定和解绑 -->
      <template v-if="isCurrentBusiness">
        <bk-tag class="tag" theme="success" type="filled">{{ t('当前业务') }}</bk-tag>
        <bk-button
          theme="primary"
          text
          :class="{ 'hcm-no-permision-text-btn': !authVerifyData?.permissionAction?.[authAction] }"
          :disabled="isOperateDisabled"
          v-bk-tooltips="{
            content: RELATED_RES_OPERATE_DISABLED_TIPS_MAP[RelatedResourceOperateType.BIND],
            disabled: !isOperateDisabled,
          }"
          @click="handleShowOperateDialog('bind')"
        >
          <i class="hcm-icon bkhcm-icon-plus-circle-shape mr2"></i>
          {{ t('新增绑定') }}
        </bk-button>
        <bk-divider direction="vertical" type="solid" class="divider" />
        <template v-if="showCopyButtons">
          <div class="copy-dropdown">
            <hcm-dropdown :disabled="copyDisabled" text-button theme="primary">
              {{ t('复制') }}
              <angle-down class="dropdown-icon" />

              <template #menus>
                <copy-to-clipboard
                  type="dropdown-item"
                  :text="copyIPv4Text"
                  :content="copyIPv4Content"
                  :disabled="copyDisabled"
                />
                <copy-to-clipboard
                  type="dropdown-item"
                  :text="copyIPv6Text"
                  :content="copyIPv6Content"
                  :disabled="copyDisabled"
                />
                <copy-to-clipboard
                  type="dropdown-item"
                  :text="copyIDText"
                  :content="copyIDContent"
                  :disabled="copyDisabled"
                />
              </template>
            </hcm-dropdown>
          </div>
        </template>
        <bk-button
          theme="primary"
          text
          class="unbind-btn"
          :class="{ 'hcm-no-permision-text-btn': !authVerifyData?.permissionAction?.[authAction] }"
          :disabled="!selected.length || isOperateDisabled"
          v-bk-tooltips="{
            content: RELATED_RES_OPERATE_DISABLED_TIPS_MAP[RelatedResourceOperateType.UNBIND],
            disabled: !isOperateDisabled,
          }"
          @click="handleShowOperateDialog('batch-unbind')"
        >
          {{ t('批量解绑') }}
        </bk-button>
      </template>
      <!-- 其他业务的实例，在当前业务只读，不可以操作 -->
      <template v-else>
        <span class="overview">
          {{ RELATED_RES_NAME_MAP[tabActive] }}：
          <span class="number">{{ resCount }}</span>
        </span>
      </template>
    </div>
    <data-list
      ref="data-list"
      v-show="isExpand"
      :loading="loading"
      :resource-name="tabActive"
      operation="base"
      :list="relResList"
      :pagination="pagination"
      :has-selections="isCurrentBusiness"
      :has-settings="isCurrentBusiness"
      :is-row-select-enable="() => true"
      @select="(selections) => (selected = selections)"
    >
      <template v-if="isCurrentBusiness" #operate="{ row }">
        <bk-button
          theme="primary"
          text
          :class="{ 'hcm-no-permision-text-btn': !authVerifyData?.permissionAction?.[authAction] }"
          :disabled="isOperateDisabled"
          v-bk-tooltips="{
            content: RELATED_RES_OPERATE_DISABLED_TIPS_MAP[RelatedResourceOperateType.UNBIND],
            disabled: !isOperateDisabled,
          }"
          @click="handleShowOperateDialog('single-unbind', row)"
        >
          {{ t('解绑') }}
        </bk-button>
      </template>
    </data-list>

    <template v-if="bindVisible">
      <bind v-model="bindVisible" :tab-active="tabActive" :detail="detail" @success="handleOperateSuccess" />
    </template>

    <template v-if="batchUnbindVisible">
      <batch-unbind
        v-model="batchUnbindVisible"
        :selections="selected"
        :tab-active="tabActive"
        :detail="detail"
        @success="handleOperateSuccess"
      />
    </template>

    <template v-if="singleUnbindVisible">
      <single-unbind
        v-model="singleUnbindVisible"
        :row="singleUnbindOperateRow"
        :tab-active="tabActive"
        :detail="detail"
        @success="handleOperateSuccess"
      />
    </template>
  </div>
</template>

<style scoped lang="scss">
.collapse-wrap {
  border: 1px solid #dcdee5;

  .tools {
    padding: 0 24px 0 8px;
    display: flex;
    align-items: center;
    height: 32px;
    background: #f0f1f5;
    font-size: 12px;

    .name {
      margin: 0 8px;
      color: #313238;
    }

    .tag {
      margin-right: 16px;
      height: 16px;
    }

    .copy-dropdown {
      .dropdown-icon {
        font-size: 16px;
        color: inherit;
      }
    }

    .unbind-btn {
      margin-left: auto;
    }

    .overview {
      margin-left: 50px;

      .number {
        color: #313238;
      }
    }
  }
}

.divider {
  margin: 0 12px;
  height: 12px;
}
</style>
