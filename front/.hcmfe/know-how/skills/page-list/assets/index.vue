<script setup lang="ts">
import { ref, computed, inject, type Ref, watch, reactive, useTemplateRef } from 'vue';
import { useRoute } from 'vue-router';
import { Plus } from 'bkui-vue/lib/icon';
import { ModelPropertySearch, ModelPropertyColumn } from '@/model/typings';
import useSearchQs from '@/hooks/use-search-qs';
import usePage from '@/hooks/use-page';
import { VendorEnum } from '@/common/constant';
import { transformFlatCondition } from '@/utils/search';
import { useWhereAmI } from '@/hooks/useWhereAmI';

// ---- 1. 注入当前云厂商（多类型模块需要，单类型可删除）----
const currentVendor = inject<Ref<VendorEnum>>('currentVendor', ref(VendorEnum.TCLOUD));

// ---- 2. Store 与路由 ----
// import { useXxxStore } from '@/store/<模块路径>';
// const xxxStore = useXxxStore();
const route = useRoute();
const { getBizsId } = useWhereAmI();
const bizId = computed(() => getBizsId());

// ---- 3. 分页 ----
const { pagination, getPageParams } = usePage();

// ---- 4. 搜索条件（多类型用 Factory，单类型直接定义）----
// import { SearchConditionFactory } from './children/list/search/condition-factory';
// const searchModel = computed(() => SearchConditionFactory.createModel(currentVendor.value));
// const searchFields = computed<ModelPropertySearch[]>(() => searchModel.value.getProperties());
const searchFields = ref<ModelPropertySearch[]>([]); // 占位：实际从 condition-factory 或静态定义获取

// ---- 5. 表格列（多类型用 Factory，单类型直接定义）----
// import { TableColumnFactory } from './children/list/data-list/column-factory';
// const columnModel = TableColumnFactory.createModel(currentVendor.value);
// const dataListColumns = computed<ModelPropertyColumn[]>(() => columnModel.getProperties());
const dataListColumns = ref<ModelPropertyColumn[]>([]); // 占位

// ---- 6. 列表数据 ----
const listData = ref<any[]>([]);
const condition = ref<Record<string, any>>({});

// ---- 7. URL 查询同步 ----
const searchQs = useSearchQs({ key: 'filter', properties: searchFields.value });

// ---- 8. 交互状态（sideslider / dialog）----
const createState = reactive({ isShow: false, data: null });
const editState = reactive({ isShow: false, data: null });
const detailsState = reactive({ isShow: false, data: null });
const deleteState = reactive({ isShow: false, data: null as any });

// ---- 9. 监听路由查询变化，触发数据加载 ----
watch(
  () => route.query,
  async (query) => {
    condition.value = searchQs.get(query);

    pagination.current = Number(query.page) || 1;
    pagination.limit = Number(query.limit) || pagination.limit;

    const sort = (query.sort || 'created_at') as string;
    const order = (query.order || 'DESC') as string;

    // TODO: 替换为实际 Store API 调用
    // const { list = [], count } = await xxxStore.getList(bizId.value, currentVendor.value, {
    //   ...transformFlatCondition(condition.value, searchFields.value),
    //   page: getPageParams(pagination, { sort, order }),
    // });
    // pagination.count = count;
    // listData.value = list;
  },
  { immediate: true },
);

// ---- 10. 搜索/重置/操作事件 ----
const handleSearch = (c: Record<string, any>) => searchQs.set(c);
const handleReset = () => searchQs.clear();

// import routerAction from '@/router/utils/action';
// import { MENU_BUSINESS_XXX_DETAILS } from '@/constants/menu-symbol';

const handleCreate = () => { createState.isShow = true; createState.data = null; };
const handleEdit = (row: any) => { editState.isShow = true; editState.data = { ...row }; };

// Sideslider 模式
const handleViewDetails = (row: any) => { detailsState.isShow = true; detailsState.data = row; };

// 独立路由模式（替代上面的 handleViewDetails）
// const handleViewDetails = (id: string) => {
//   routerAction.redirect(
//     { name: MENU_BUSINESS_XXX_DETAILS, params: { id }, query: { bizs: getBizsId() } },
//     { history: true },
//   );
// };

const handleDelete = (row: any) => { deleteState.isShow = true; deleteState.data = row; };

// TODO: 替换为实际权限 Symbol
// import { AUTH_CREATE_XXX } from '@/constants/auth-symbols';
const AUTH_CREATE_XXX = Symbol('auth_create_xxx');
</script>

<template>
  <div class="xxx-list">
    <!-- 搜索区域 -->
    <Search
      :fields="searchFields"
      :condition="condition"
      @search="handleSearch"
      @reset="handleReset"
    />

    <!-- 表格区域 -->
    <div class="table-panel">
      <div class="toolbar">
        <hcm-auth :sign="{ type: AUTH_CREATE_XXX, relation: [bizId] }" v-slot="{ noPerm }">
          <bk-button theme="primary" :disabled="noPerm" @click="handleCreate">
            <Plus style="font-size: 22px" />
            新建
          </bk-button>
        </hcm-auth>
      </div>
      <DataList
        v-bkloading="{ loading: false }"
        :columns="dataListColumns"
        :list="listData"
        :pagination="pagination"
        @view-details="handleViewDetails"
        @edit="handleEdit"
        @delete="handleDelete"
      />
    </div>
  </div>

  <!-- 新建 Sideslider -->
  <bk-sideslider v-model:is-show="createState.isShow" render-directive="if" width="640" title="新建">
    <template #default>
      <!-- <CreateForm ref="createFormRef" :data="createState.data" /> -->
    </template>
    <template #footer>
      <div class="sideslider-footer">
        <bk-button theme="primary" @click="createState.isShow = false">提交</bk-button>
        <bk-button @click="createState.isShow = false">取消</bk-button>
      </div>
    </template>
  </bk-sideslider>

  <!-- 编辑 Sideslider -->
  <bk-sideslider v-model:is-show="editState.isShow" render-directive="if" width="640" title="编辑">
    <template #default>
      <!-- <EditForm ref="editFormRef" :data="editState.data" /> -->
    </template>
    <template #footer>
      <div class="sideslider-footer">
        <bk-button theme="primary" @click="editState.isShow = false">提交</bk-button>
        <bk-button @click="editState.isShow = false">取消</bk-button>
      </div>
    </template>
  </bk-sideslider>

  <!-- 详情 Sideslider -->
  <bk-sideslider v-model:is-show="detailsState.isShow" render-directive="if" width="640" title="详情">
    <template #default>
      <!-- <Details :data="detailsState.data" /> -->
    </template>
  </bk-sideslider>
</template>

<style lang="scss" scoped>
.xxx-list {
  height: 100%;

  .table-panel {
    background: #fff;
    border-radius: 2px;
    box-shadow: 0 2px 4px 0 #1919290d;
    margin: 24px;
    padding: 16px 24px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }
}

.sideslider-footer {
  display: flex;
  align-items: center;
  gap: 6px;

  .bk-button {
    min-width: 88px;
  }
}
</style>
