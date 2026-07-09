<script setup lang="ts">
import { ref, watch, onMounted } from 'vue';
import {
  Ediatable,
  HeadColumn,
  SelectColumn,
  InputColumn,
  OperationColumn,
  DateTimePickerColumn,
} from '@blueking/ediatable';
import { useDissolveQuotaStore, type IDissolveProjectCycle, type IDissolveProject } from '@/store/dissolve/quota';

const model = defineModel<IDissolveProjectCycle>({ required: true });

const props = defineProps<{
  index: number;
}>();

defineEmits<{ (e: 'remove'): void }>();

const store = useDissolveQuotaStore();
const projectTypeOptions = ref<{ value: number; label: string }[]>([]);

onMounted(async () => {
  try {
    const res = await store.getProjectTypes();
    projectTypeOptions.value = res || [];
  } catch {
    projectTypeOptions.value = [];
  }
});

// 每个 project 行的组件引用（用于校验）
// 使用 Record 避免接口中 InstanceType 泛型导致的 VLS 类型推断问题
const entryRefsList = ref<Record<string, any>[]>([]);

// DateTimePickerColumn 引用（日期范围校验用）
// 使用数组存储，因为每个 TimePeriodBlock 都有一个 datePickerRef
const datePickerRef = ref<InstanceType<typeof DateTimePickerColumn> | null>(null);

// ---- 日期范围双向绑定 ----

const dateRange = ref<[string, string]>(['', '']);

// 同步 model.start/end <-> dateRange
watch(
  () => [model.value.start, model.value.end],
  ([start, end]) => {
    if (dateRange.value[0] !== start || dateRange.value[1] !== end) {
      dateRange.value = [start ?? '', end ?? ''];
    }
  },
  { immediate: true },
);

watch(dateRange, (val) => {
  model.value.start = val?.[0] ?? '';
  model.value.end = val?.[1] ?? '';
});

// ---- projects CRUD ----

const createEmptyProject = (): IDissolveProject => ({
  id: 0 as any, // 新建时临时值，提交时校验
  memo: '',
});

// 确保至少有一行 project
const ensureProjects = () => {
  if (!model.value.projects?.length) {
    model.value.projects = [createEmptyProject()];
    entryRefsList.value = [];
  }
};

// 初始化时确保有 projects
ensureProjects();

const addEntry = () => {
  model.value.projects.push(createEmptyProject());
};

const removeEntry = (entryIndex: number) => {
  model.value.projects.splice(entryIndex, 1);
  entryRefsList.value.splice(entryIndex, 1);
};

// ---- 校验 ----

const getValue = async () => {
  try {
    const allRefs: Promise<any>[] = [];

    // 日期范围校验
    if (datePickerRef.value) {
      allRefs.push(datePickerRef.value.getValue());
    }

    // 每行 project 的校验（仅校验项目类型）
    entryRefsList.value.forEach((refs) => {
      if (refs.typeRef) allRefs.push(refs.typeRef.getValue());
    });

    await Promise.all(allRefs);
    return { ...model.value };
  } catch {
    throw new Error('时间段配置校验失败');
  }
};

defineExpose({ getValue });
</script>

<template>
  <div class="time-period-card">
    <!-- 卡片头部：时间段名称 + 当前裁撤时间 checkbox + 删除 -->
    <div class="card-header">
      <div class="header-left">
        <span class="period-label">时间段{{ props.index + 1 }}</span>
        <bk-checkbox v-model="model.default" size="small" label="当前裁撤时间">
          <span class="current-label">当前裁撤时间</span>
        </bk-checkbox>
      </div>
      <button type="button" class="delete-btn" @click="$emit('remove')">
        <i class="hcm-icon bkhcm-icon-bin" style="color: #ea3636"></i>
      </button>
    </div>

    <!-- Ediatable 可编辑表格：裁撤时间 | 项目类型 | 备注 | 操作 -->
    <Ediatable class="period-ediatable">
      <template #default>
        <HeadColumn :required="true" :min-width="280">裁撤时间</HeadColumn>
        <HeadColumn :required="true" :min-width="200">项目类型</HeadColumn>
        <HeadColumn :required="false" :min-width="200">备注</HeadColumn>
        <HeadColumn :required="false" :min-width="80"></HeadColumn>
      </template>

      <template #data>
        <tr v-for="(project, entryIndex) in model.projects" :key="entryIndex">
          <!-- 裁撤时间列：仅第一行显示，跨所有行 -->
          <td v-if="entryIndex === 0" :rowspan="model.projects.length" class="col-time">
            <DateTimePickerColumn
              :ref="(el: any) => datePickerRef = el"
              type="daterange"
              v-model="dateRange"
              placeholder="请选择"
              format="yyyy-MM-dd"
              :clearable="false"
              :rules="[{ validator: (val: [string, string]) => Boolean(val && val[0] && val[1]), message: '请选择裁撤时间' }]"
            />
          </td>

          <!-- 项目类型列 -->
          <td class="col-type">
            <SelectColumn
              :ref="(el: any) => el && ((entryRefsList[entryIndex] ||= {}).typeRef = el)"
              v-model="project.id"
              :list="projectTypeOptions"
              :rules="[{ validator: (val: number) => Boolean(val) && val > 0, message: '请选择项目类型' }]"
              :clearable="false"
              @clear="project.id = undefined"
            />
          </td>

          <!-- 备注列 -->
          <td class="col-memo">
            <InputColumn
              :ref="(el: any) => el && ((entryRefsList[entryIndex] ||= {}).memoRef = el)"
              v-model="project.memo"
              placeholder="请填写与配置理由"
            />
          </td>

          <!-- 操作列 -->
          <td class="col-action">
            <OperationColumn
              :removeable="model.projects.length > 1"
              show-add
              @add="addEntry"
              @remove="removeEntry(entryIndex)"
            />
          </td>
        </tr>
      </template>
    </Ediatable>
  </div>
</template>

<style lang="scss" scoped>
.time-period-card {
  background: #fff;
  border: 1px solid #dcdee5;
  border-radius: 2px;
  margin-bottom: 16px;
  overflow: hidden;

  &:last-child {
    margin-bottom: 0;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 16px;
    background: #fafbfc;
    border-bottom: 1px solid #dcdee5;

    .header-left {
      display: flex;
      align-items: center;
      gap: 24px;
      font-size: 12px;
    }

    .period-label {
      font-weight: 700;
      color: #313238;
      background-color: #f5f7fa;
      line-height: 20px;
    }

    .current-label {
      font-size: 12px;
      line-height: 20px;
      color: #4d4f56;
    }

    .delete-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      border: none;
      background: transparent;
      cursor: pointer;
      border-radius: 2px;

      &:hover {
        background: #ffebeb;
      }
    }
  }

  .period-ediatable {
    .col-time {
      vertical-align: middle;
    }

    .col-action {
      text-align: center;
    }
  }
}
</style>
