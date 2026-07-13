<script setup lang="ts">
import { ref, computed, inject, type Ref, watch } from 'vue';
import type { ModelPropertySearch } from '@/model/typings';
import type { IDissolveProjectCycle } from '@/store/dissolve/quota';
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

const formValues = ref<Record<string, any>>({
  time_periods: [],
});

// 从父组件注入数据
const dissolveProjects = inject<Ref<IDissolveProjectCycle[]>>('dissolveProjects', ref([]));
const projectTypeList = inject<Ref<Array<{ value: number; label: string }>>>('projectTypeList', ref([]));

// 根据选中的时间段计算项目类型选项
const projectTypeOptions = computed(() => {
  const selectedPeriods = formValues.value.time_periods || [];
  if (!selectedPeriods.length) return [];

  // 找到选中的时间段对应的项目
  const matchedProjects = dissolveProjects.value.filter((p) => selectedPeriods.includes(`${p.start}~${p.end}`));

  // 通过 id 匹配 projectName
  const merged = new Map<number | string, { value: number | string; label: string }>();
  matchedProjects.forEach((p) => {
    (p.projects || []).forEach((proj) => {
      if (!merged.has(proj.id)) {
        const projectType = projectTypeList.value.find((pt) => pt.value === proj.id);
        merged.set(proj.id, {
          value: proj.id,
          label: projectType?.label || '未知',
        });
      }
    });
  });

  return [...merged.values()];
});

// 当前可选的所有项目类型 ID
const allProjectIds = computed(() => projectTypeOptions.value.map((item) => item.value));

// 裁撤时间段选项列表（映射为 hcm-search-list 所需格式）
const timePeriodOptions = computed(() =>
  dissolveProjects.value.map((p) => ({
    value: `${p.start}~${p.end}`,
    label: `${p.start} 至 ${p.end}`,
  })),
);

// 将 URL 中的 project_ids 归一化为 ['all']（当已全选时）
// 注意：直接基于 dissolveProjects + time_periods 计算全量 ID，而非使用 allProjectIds
// 因为 allProjectIds → projectTypeOptions → formValues.time_periods 形成了循环依赖：
// 在 watch(props.condition) 的同语句赋值中 formValues 尚未更新，导致归一化失败
const normalizeProjectIds = (condition: Record<string, any>) => {
  const projectIds = condition?.project_ids;
  if (projectIds === undefined || projectIds === null) return ['all'];

  // 获取当前有效的时间段（优先从 condition 取，再 fallback 到 formValues）
  const timePeriods = condition?.time_periods || formValues.value?.time_periods || [];
  if (!timePeriods.length) return projectIds;

  // 直接从原始数据计算全量可选项目 ID，避免依赖 projectTypeOptions 间接读取 formValues
  const matchedProjects = dissolveProjects.value.filter((p) => timePeriods.includes(`${p.start}~${p.end}`));
  const allIds = new Set<number | string>();
  matchedProjects.forEach((p) => {
    (p.projects || []).forEach((proj) => allIds.add(proj.id));
  });

  if (!allIds.size) return projectIds;
  if (Array.isArray(projectIds) && projectIds.length === allIds.size) {
    const allSelected = [...allIds].every((id) => projectIds.includes(id));
    if (allSelected) return ['all'];
  }
  return projectIds;
};

// 构造实际提交参数：选择"全部"时展开为所有项目 ID
const getSearchParams = () => {
  const params = { ...formValues.value };
  if (Array.isArray(params.project_ids) && params.project_ids.includes('all')) {
    params.project_ids = allProjectIds.value as number[];
  }
  return params;
};

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
    time_periods: defaultSelectedTime.value,
    project_ids: allProjectIds.value.length > 0 ? ['all'] : [],
  };
  emit('search', getSearchParams());
};

const defaultSelectedTime = computed(() => {
  return dissolveProjects.value.filter((p) => p.default).map((p) => `${p.start}~${p.end}`) || [];
});

watch(
  () => props.condition,
  (condition) => {
    formValues.value = {
      ...condition,
      project_ids: normalizeProjectIds(condition),
    };
  },
  { deep: true, immediate: true },
);

// 裁撤时间段变化时，项目类型默认重置为"全部"
// 同时防止时间段被完全清空——至少保留一个选中项
watch(
  () => formValues.value.time_periods,
  (newVal, oldVal) => {
    // 防止被完全清空
    if (newVal && newVal.length === 0 && timePeriodOptions.value.length > 0) {
      formValues.value.time_periods = [timePeriodOptions.value[0].value];
      return;
    }

    if (newVal !== undefined && JSON.stringify(newVal) !== JSON.stringify(oldVal)) {
      formValues.value.project_ids = allProjectIds.value.length > 0 ? ['all'] : [];
    }
  },
  { deep: true },
);

// allProjectIds 就绪后，重新归一化 project_ids（修复 tab 切换时"全部"不点亮的时序问题）
watch(allProjectIds, (ids) => {
  if (ids.length > 0) {
    formValues.value.project_ids = normalizeProjectIds(formValues.value);
  }
});

// 裁撤项目数据变化时，自动同步默认选中并触发搜索
const dissolveWatchInitialized = ref(false);
watch(
  dissolveProjects,
  (projects) => {
    if (!projects?.length) {
      formValues.value = { ...formValues.value, time_periods: [] };
      if (dissolveWatchInitialized.value) handleSearch();
      return;
    }

    const defaultSelected = projects.filter((p) => p.default).map((p) => `${p.start}~${p.end}`);

    if (!dissolveWatchInitialized.value) {
      // 首次加载：若 URL 已有参数则保留，否则应用默认值
      dissolveWatchInitialized.value = true;
      if (!formValues.value.time_periods?.length) {
        formValues.value = { ...formValues.value, time_periods: defaultSelected, project_ids: ['all'] };
        handleSearch();
      }
      return;
    }

    // 后续变化（如配置保存）：始终应用最新默认值，直接更新视图 + 标准搜索
    formValues.value = { ...formValues.value, time_periods: defaultSelected, project_ids: ['all'] };
    handleSearch();
  },
  { immediate: true, deep: true },
);
</script>

<template>
  <div class="dissolve-search">
    <grid-container layout="vertical" :column="4" :gap="[16, 60]">
      <grid-item-form-element
        v-for="field in fields"
        :key="field.id"
        :label="field.name"
        :class="field.id === 'time_periods' ? 'required-field' : ''"
      >
        <!-- 裁撤时间段 -->
        <hcm-search-list
          v-if="field.id === 'time_periods'"
          v-model="formValues[field.id]"
          :list="timePeriodOptions"
          :multiple="true"
          :filterable="false"
          :clearable="false"
          id-key="value"
          display-key="label"
          style="width: 100%"
        />
        <OrgTreeSelector style="width: 100%" v-else-if="field.id === 'group_ids'" v-model="formValues[field.id]" />
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

  :deep(.required-field .item-label::after) {
    content: '*';
    margin-left: 6px;
    color: #ea3636;
  }
}
</style>
