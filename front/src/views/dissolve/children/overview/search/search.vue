<script setup lang="ts">
import { ref, computed, inject, type Ref, watch } from 'vue';
import type { ModelPropertySearch } from '@/model/typings';
import type { IDissolveProjectCycle } from '@/store/dissolve/quota';
import { useBusinessGlobalStore } from '@/store/business-global';
import { UNKNOWN_BIZ_OPTION } from '@/views/dissolve/common/unknown-biz';
import GridContainer from '@/components/layout/grid-container/grid-container.vue';
import GridItemFormElement from '@/components/layout/grid-container/grid-item-form-element.vue';
import GridItem from '@/components/layout/grid-container/grid-item.vue';
import OrgTreeSelector from '@/views/dissolve/components/org-tree-selector/index.vue';

interface ISearchProps {
  fields: ModelPropertySearch[];
  condition: Record<string, any>;
}

const props = withDefaults(defineProps<ISearchProps>(), {});

const emit = defineEmits<{
  (e: 'search', condition: Record<string, any>): void;
  (e: 'reset'): void;
}>();

const formValues = ref<Record<string, any>>({});

// 从父组件注入裁撤配置和项目类型列表
const dissolveProjects = inject<Ref<IDissolveProjectCycle[]>>('dissolveProjects', ref([]));
const projectTypeList = inject<Ref<Array<{ value: number; label: string }>>>('projectTypeList', ref([]));

const businessGlobalStore = useBusinessGlobalStore();

// 业务名称下拉选项：授权业务 + "未知"（bk_biz_id === 0 的数据行）
const bizOptions = computed(() => [...businessGlobalStore.businessAuthorizedList, UNKNOWN_BIZ_OPTION]);

// 从裁撤配置中聚合所有已配置的项目（去重），不再和裁撤时间联动
const projectTypeOptions = computed(() => {
  const merged = new Map<number, { value: number; label: string }>();
  dissolveProjects.value.forEach((cycle) => {
    (cycle.projects || []).forEach((proj) => {
      if (!merged.has(proj.id)) {
        const projectType = projectTypeList.value.find((pt) => pt.value === proj.id);
        merged.set(proj.id, { value: proj.id, label: projectType?.label || String(proj.id) });
      }
    });
  });
  return [...merged.values()];
});

// 构造搜索参数，保留 'all' 标记不做展开，展开逻辑由父组件在 API 调用前处理
const getSearchParams = () => ({ ...formValues.value });

const getSearchCompProps = (field: ModelPropertySearch) => {
  const compProps: Record<string, any> = { ...field.props };
  if (field.option !== undefined) compProps.option = field.option;
  if (field.list !== undefined) compProps.list = field.list;
  return compProps;
};

const handleSearch = () => {
  emit('search', getSearchParams());
};

const handleReset = () => {
  formValues.value = {
    project_ids: [],
  };
  emit('search', getSearchParams());
};

// 从 URL 恢复搜索条件
watch(
  () => props.condition,
  (condition) => {
    formValues.value = { ...condition };
  },
  { deep: true, immediate: true },
);
</script>

<template>
  <div class="dissolve-search">
    <grid-container layout="vertical" :column="4" :gap="[16, 60]">
      <grid-item-form-element v-for="field in fields" :key="field.id" :label="field.name">
        <OrgTreeSelector v-if="field.id === 'group_ids'" v-model="formValues[field.id]" style="width: 100%" />
        <hcm-search-list
          v-else-if="field.id === 'project_ids'"
          v-model="formValues[field.id]"
          :list="projectTypeOptions"
          :multiple="true"
          id-key="value"
          display-key="label"
          collapse-tags
          clearable
          show-all
          all-option-id="all"
        />
        <hcm-search-business v-else-if="field.id === 'bk_biz_ids'" v-model="formValues[field.id]" :data="bizOptions" />
        <component
          :is="`hcm-search-${field.type}`"
          v-else
          v-bind="getSearchCompProps(field)"
          v-model="formValues[field.id]"
        />
      </grid-item-form-element>
      <grid-item :span="4" class="row-action">
        <bk-button theme="primary" @click="handleSearch">查询</bk-button>
        <bk-button @click="handleReset">重置</bk-button>
      </grid-item>
    </grid-container>
  </div>
</template>

<style lang="scss" scoped>
.dissolve-search {
  background: #fff;
  box-shadow: 0 2px 4px 0 #1919290d;
  border-radius: 2px;
  padding: 16px 24px;

  .row-action {
    padding: 4px 0;

    :deep(.item-content) {
      gap: 10px;
    }

    .bk-button {
      min-width: 86px;
    }
  }
}
</style>
