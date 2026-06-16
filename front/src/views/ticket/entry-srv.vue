<template>
  <div class="tab-container">
    <bk-tab type="card-grid" v-model:active="applyType" class="header-tab" @update:active="saveActiveType">
      <bk-tab-panel v-for="(item, index) in tabList" :name="item.name" :label="item.label" :key="index">
        <component v-if="item.name === applyType" :is="item.Component" :rules="item.rules"></component>
      </bk-tab-panel>
    </bk-tab>
  </div>
</template>

<script setup lang="ts">
import { provide, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { ApplicationsType } from './typings';
import CommonTable from './children/common-table.vue';
import ResourcePlanList from './children/resource-plan/list/list-srv.vue';
import { QueryRuleOPEnum } from '@/typings';

const router = useRouter();
const route = useRoute();
const { t } = useI18n();

const applyType = ref(route.query?.type || 'all');

const saveActiveType = (val: string) => {
  router.replace({ query: { type: val } });
};

const tabList = ref<ApplicationsType[]>([
  {
    label: t('账号'),
    name: 'account',
    rules: [
      {
        field: 'operation',
        op: QueryRuleOPEnum.IN,
        value: [
          'add_account',
          'create_main_account',
          'update_main_account',
          'create_sub_account',
          'update_sub_account',
          'delete_sub_account',
          'create_sub_account_secret',
          'delete_sub_account_secret',
          'update_sub_account_secret',
          'apply_permission_policy_library_create',
          'apply_permission_policy_library_update',
          'create_permission_template',
          'update_permission_template',
          'delete_permission_template',
        ],
      },
    ],
    Component: CommonTable,
  },
  {
    label: '资源预测',
    name: 'resource_plan',
    rules: [],
    Component: ResourcePlanList,
  },
]);

provide('isServicePage', true);
</script>

<style lang="scss" scoped>
.tab-container {
  height: 100%;
  padding: 24px;
}
</style>
