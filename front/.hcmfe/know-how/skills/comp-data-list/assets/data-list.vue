<script setup lang="ts">
import { inject, ref, type Ref, computed } from 'vue';
import { PaginationType } from '@/typings';
import { ModelPropertyColumn } from '@/model/typings';
import usePage from '@/hooks/use-page';
import useTableSettings from '@/hooks/use-table-settings';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import { VendorEnum } from '@/common/constant';

// TODO: 替换为实际的列表项类型和权限 Symbol
// import { AUTH_UPDATE_XXX, AUTH_DELETE_XXX } from '@/constants/auth-symbols';
const AUTH_UPDATE_XXX = Symbol('auth_update_xxx');
const AUTH_DELETE_XXX = Symbol('auth_delete_xxx');

export interface IDataListProps {
  columns: ModelPropertyColumn[];
  list: any[];
  pagination: PaginationType;
}

const props = withDefaults(defineProps<IDataListProps>(), {});

const emit = defineEmits<{
  'view-details': [row: any];
  'delete': [row: any];
  'edit': [row: any];
}>();

// ---- 多类型模块需要注入当前类型，单类型可删除 ----
const currentVendor = inject<Ref<VendorEnum>>('currentVendor', ref(VendorEnum.TCLOUD));

const { getBizsId } = useWhereAmI();
const bizId = computed(() => getBizsId());

const { handlePageChange, handlePageSizeChange, handleSort } = usePage();
const { settings } = useTableSettings(props.columns);
</script>

<template>
  <bk-table
    row-hover="auto"
    :data="list"
    :pagination="pagination"
    :max-height="`calc(100vh - 520px)`"
    :settings="settings"
    remote-pagination
    show-overflow-tooltip
    @page-limit-change="handlePageSizeChange"
    @page-value-change="handlePageChange"
    @column-sort="handleSort"
    row-key="id"
  >
    <!-- 动态列：根据 columns 定义渲染 -->
    <bk-table-column
      v-for="(column, index) in columns"
      :key="index"
      :prop="column.id"
      :label="column.name"
      :sort="column.sort"
      :render="column.render"
    >
      <template #default="{ row }">
        <!-- 名称列：可点击查看详情 -->
        <template v-if="column.id === 'name'">
          <bk-button theme="primary" text @click="emit('view-details', row)">
            {{ row.name || '--' }}
          </bk-button>
        </template>
        <!-- 其他列：通用渲染 -->
        <template v-else>
          <display-value :property="column" :value="row[column.id]" :display="column?.meta?.display" />
        </template>
      </template>
    </bk-table-column>

    <!-- 操作列 -->
    <bk-table-column :show-overflow-tooltip="false" label="操作">
      <template #default="{ row }">
        <div class="actions">
          <hcm-auth :sign="{ type: AUTH_UPDATE_XXX, relation: [bizId] }" v-slot="{ noPerm }">
            <bk-button
              theme="primary"
              text
              :disabled="noPerm"
              @click="emit('edit', row)"
            >
              编辑
            </bk-button>
          </hcm-auth>
          <hcm-auth :sign="{ type: AUTH_DELETE_XXX, relation: [bizId] }" v-slot="{ noPerm }">
            <bk-button
              theme="primary"
              text
              :disabled="noPerm"
              @click="emit('delete', row)"
            >
              删除
            </bk-button>
          </hcm-auth>
        </div>
      </template>
    </bk-table-column>
  </bk-table>
</template>

<style lang="scss" scoped>
.actions {
  display: flex;
  gap: 12px;
}
</style>
