<script setup lang="ts">
import { computed, ref, provide, onMounted } from 'vue';
import { useRoute, type LocationQueryRaw } from 'vue-router';
import HcmAuth from '@/components/auth/auth.vue';
import ErrorPage from '@/views/error-pages/403';
import { useVerify } from '@/hooks';
import { AUTH_UPDATE_DISSOLVE } from '@/constants/auth-symbols';
import { useDissolveQuotaStore, type IDissolveProjectCycle } from '@/store/dissolve/quota';
import routerAction from '@/router/utils/action';
import OverviewTab from './children/overview/index.vue';
import DetailTab from './children/detail/index.vue';
import SyncDialog from './components/sync-dialog/index.vue';
import ConfigDialog from './components/config-dialog/index.vue';

const { authVerifyData } = useVerify();
const hasPermission = computed(() => !!authVerifyData.value?.permissionAction?.service_resource_dissolve_find);

const route = useRoute();
const store = useDissolveQuotaStore();

// 双向绑定：URL query.tab ↔ activeTab
// computed get/set 模式，参考 entry-rsc.vue / entry-biz.vue
const activeTab = computed<string>({
  get() {
    const tab = route.query.tab as string;
    if (tab === 'overview' || tab === 'detail') return tab;
    return 'overview';
  },
  set(value: string) {
    const query = { ...route.query, tab: value } as LocationQueryRaw;
    delete query.sort;
    delete query.order;
    routerAction.redirect({ query }, { replace: true });
  },
});

// 裁撤配置数据 - 提升到外层页面，通过 provide/inject 注入到内部组件
const dissolveProjects = ref<IDissolveProjectCycle[]>([]);
const projectTypeList = ref<Array<{ value: number; label: string }>>([]);

const fetchDissolveConfig = async () => {
  try {
    const config = await store.getDissolveConfig(); // 每次都请求最新数据
    dissolveProjects.value = config?.dissolve_projects || [];
  } catch {
    dissolveProjects.value = [];
  }
};

const fetchProjectTypes = async () => {
  try {
    projectTypeList.value = await store.getProjectTypes();
  } catch {
    projectTypeList.value = [];
  }
};

// 初始化时获取数据
onMounted(() => {
  if (hasPermission.value) {
    fetchDissolveConfig();
    fetchProjectTypes();
  }
});

// 提供响应式数据给子组件
provide('dissolveProjects', dissolveProjects);
provide('projectTypeList', projectTypeList);
provide('fetchDissolveConfig', fetchDissolveConfig);

const syncDialogVisible = ref(false);
const configDialogVisible = ref(false);

const handleSyncSuccess = () => {
  fetchDissolveConfig();
  fetchProjectTypes();
};

const handleConfigSuccess = () => {
  fetchDissolveConfig();
  fetchProjectTypes();
};

const tabComps = [
  {
    label: '裁撤总览',
    name: 'overview',
    component: OverviewTab,
  },
  {
    label: '裁撤明细',
    name: 'detail',
    component: DetailTab,
  },
];
</script>

<template>
  <ErrorPage v-if="!hasPermission" url-key-id="service_resource_dissolve" />
  <div v-else class="dissolve-page">
    <Teleport defer to="#breadcrumbExtra">
      <hcm-auth :sign="{ type: AUTH_UPDATE_DISSOLVE }" v-slot="{ noPerm }">
        <template v-if="!noPerm">
          <bk-button @click="syncDialogVisible = true" class="mr8">
            <i class="hcm-icon bkhcm-icon-configuration mr8"></i>
            同步
          </bk-button>
          <bk-button @click="configDialogVisible = true">
            <i class="hcm-icon bkhcm-icon-configuration mr8"></i>
            裁撤配置
          </bk-button>
        </template>
      </hcm-auth>
    </Teleport>

    <bk-tab v-model:active="activeTab" type="unborder-card">
      <bk-tab-panel v-for="tab in tabComps" render-directive="if" :key="tab.name" :label="tab.label" :name="tab.name">
        <component :is="tab.component" />
      </bk-tab-panel>
    </bk-tab>

    <SyncDialog v-model:is-show="syncDialogVisible" @success="handleSyncSuccess" />
    <ConfigDialog v-model:is-show="configDialogVisible" @success="handleConfigSuccess" />
  </div>
</template>

<style lang="scss" scoped>
.dissolve-page {
  // 移除左右 padding，让子组件白色背景贴满边缘（与权限策略库一致）

  :deep(.bk-tab-header) {
    background: #fff;
  }

  :deep(.bk-tab-content) {
    padding: 0;
  }
}
</style>
