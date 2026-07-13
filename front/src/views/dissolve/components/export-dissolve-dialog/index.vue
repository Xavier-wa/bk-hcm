<script setup lang="ts">
import { watch } from 'vue';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import wName from '@/components/w-name';

interface ExportParams {
  includeUtilization: boolean;
  utilizationDate: string;
}

interface Props {
  params: ExportParams;
  totalCount?: number;
}

const props = withDefaults(defineProps<Props>(), {
  totalCount: 0,
});

const emit = defineEmits<{
  (e: 'update:params', value: ExportParams): void;
}>();

const { t } = useI18n();

const onCheckboxChange = (val: boolean) => {
  emit('update:params', {
    ...props.params,
    includeUtilization: val,
  });
};

// 禁用今天及以后的日期
const disabledDate = (date: Date) => dayjs(date).diff(dayjs(), 'day') >= 0;

// 处理日期变化
const handleDateChange = (value: string | Date) => {
  emit('update:params', {
    ...props.params,
    utilizationDate: dayjs(value).format('YYYY/MM/DD'),
  });
};

// 监听外部 initialized 状态，设置默认值
watch(
  () => props.params,
  (val) => {
    if (!val.utilizationDate) {
      emit('update:params', {
        includeUtilization: val.includeUtilization ?? true,
        utilizationDate: dayjs().subtract(1, 'day').format('YYYY/MM/DD'),
      });
    }
  },
  { immediate: true },
);
</script>

<template>
  <div class="export-dialog-content">
    <div class="export-info">
      <p class="export-count">
        即将导出当前筛选结果，共
        <span class="highlight-number">{{ totalCount }}</span>
        条裁撤主机。
      </p>
      <p class="export-desc">导出内容与列表字段一致，可用于线下分析。</p>
    </div>

    <div class="export-option">
      <bk-checkbox :model-value="params.includeUtilization" @change="onCheckboxChange">
        附带主机利用率（CPU、内存）
      </bk-checkbox>
      <p class="option-description">
        用于评估裁撤可申领置换额度；需额外查询监控数据，导出耗时较长，默认不包含。具体政策请咨询
        <w-name name="ICR" :alias="t('ICR(IEG资源服务助手)')" />
      </p>
    </div>

    <div v-if="params.includeUtilization" class="export-date-section">
      <label class="date-label">
        <span class="required">*</span>
        利用率统计时间
      </label>
      <bk-date-picker
        :model-value="params.utilizationDate"
        type="date"
        format="yyyy/MM/dd"
        placeholder="请选择日期"
        :clearable="false"
        :disabled-date="disabledDate"
        append-to-body
        @change="handleDateChange"
      />
      <p class="date-description">导出将查询该日期的 CPU、内存利用率</p>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.export-dialog-content {
  padding: 0 4px;

  .export-info {
    margin-bottom: 20px;

    .export-count {
      font-size: 14px;
      color: #313238;
      line-height: 22px;
      margin-bottom: 8px;

      .highlight-number {
        color: #3a84ff;
        font-weight: 600;
      }
    }

    .export-desc {
      font-size: 12px;
      color: #979ba5;
      line-height: 20px;
    }
  }

  .export-option {
    padding: 16px;
    background: #f5f7fa;
    border-radius: 4px;
    margin-bottom: 16px;

    :deep(.bk-checkbox-label) {
      font-size: 14px;
      color: #313238;
      font-weight: 500;
    }

    .option-description {
      font-size: 12px;
      color: #979ba5;
      line-height: 20px;
      margin: 8px 0 0;
      padding-left: 20px;
    }
  }

  .export-date-section {
    padding: 16px;
    background: #f5f7fa;
    border-radius: 4px;

    .date-label {
      display: block;
      font-size: 14px;
      color: #313238;
      margin-bottom: 8px;

      .required {
        color: #fe5c5c;
        margin-right: 4px;
      }
    }

    :deep(.bk-date-picker) {
      width: 100%;

      .bk-form-input {
        width: 100%;
      }
    }

    .date-description {
      font-size: 12px;
      color: #979ba5;
      line-height: 20px;
      margin: 8px 0 0;
    }
  }
}
</style>
