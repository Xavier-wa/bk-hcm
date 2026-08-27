<script setup lang="ts">
import { ref, reactive, watch, computed, nextTick, useTemplateRef } from 'vue';
import { Form, Message } from 'bkui-vue';
import { InfoLine, Plus } from 'bkui-vue/lib/icon';
import { useDissolveQuotaStore, type IDissolveConfig } from '@/store/dissolve/quota';
import Panel from '@/components/panel/panel.vue';
import QuotaOffsetTable from '../../children/quota-offset/index.vue';
import TimePeriodBlock from './time-period-block.vue';
import { timeUTCFormatter } from '@/common/util';
import { useBusinessGlobalStore } from '@/store/business-global';

const isShow = defineModel<boolean>('isShow', { required: true });
const emit = defineEmits<{ (e: 'success'): void }>();

const dissolveStore = useDissolveQuotaStore();
const businessGlobalStore = useBusinessGlobalStore();

const loading = ref(false);
const quotaOffsetTableRef = ref<InstanceType<typeof QuotaOffsetTable>>();
const timePeriodBlockRefs = useTemplateRef<InstanceType<typeof TimePeriodBlock>[]>('timePeriodBlockRefs');
const formRef = ref<InstanceType<typeof Form>>();

const initialState: IDissolveConfig = {
  host_apply_time: '',
  approval_limit: 0,
  quota_coefficient: 0,
  quota_offsets: [],
  dissolve_projects: [],
};

const formData = reactive<IDissolveConfig>({ ...initialState });

// 记录弹窗打开时的原始数据，用于重置
const originalData = ref<IDissolveConfig | null>(null);

watch(isShow, async (v) => {
  if (v) {
    try {
      const config = await dissolveStore.getDissolveConfig();
      const data = config || initialState;
      Object.assign(formData, data);
      if (!Array.isArray(formData.dissolve_projects)) formData.dissolve_projects = [];
      if (!Array.isArray(formData.quota_offsets)) formData.quota_offsets = [];

      // 深拷贝记录原始数据
      originalData.value = JSON.parse(JSON.stringify(formData));
    } catch {
      Object.assign(formData, initialState);
      originalData.value = JSON.parse(JSON.stringify(initialState));
    }
  } else {
    // 弹窗关闭时清空记录的数据
    originalData.value = null;
  }
});

const addTimePeriod = () => {
  formData.dissolve_projects.push({
    start: '',
    end: '',
    default: false,
    projects: [{ id: undefined as any, memo: undefined }],
  });
};

const removeTimePeriod = (index: number) => {
  formData.dissolve_projects.splice(index, 1);
};

const handleValidate = async (hint = true): Promise<boolean> => {
  try {
    const { duplicateBizIds } = getDuplicateBizIds();
    const duplicateValid = duplicateBizIds.size === 0;

    const [formValid, quotaOffsetValid] = await Promise.all([
      formRef.value?.validate(),
      quotaOffsetTableRef.value?.validate?.(),
    ]);

    // 校验所有时间段配置块（集合引用由 Vue 维护，仅包含当前挂载的块）
    const blocks = timePeriodBlockRefs.value ?? [];
    if (blocks.length > 0) {
      await Promise.all(blocks.map((block) => block?.getValue()));
    }

    if (!duplicateValid) {
      const bizNames = Array.from(duplicateBizIds)
        .map((id) => businessGlobalStore.businessFullList.find((biz) => biz.id === id)?.name || String(id))
        .join('、');
      Message({
        theme: 'error',
        message: `不允许选择重复的业务：${bizNames}`,
      });
    }

    return !!formValid && !!quotaOffsetValid && duplicateValid;
  } catch {
    if (hint) Message({ theme: 'warning', message: '请检查必填项' });
    return false;
  }
};

const handleConfirm = async () => {
  const isValid = await handleValidate();
  if (!isValid) return;

  loading.value = true;
  try {
    await dissolveStore.upsertDissolveConfig({ ...formData });
    Message({ theme: 'success', message: '保存成功' });
    isShow.value = false;
    emit('success');
  } catch (e: any) {
    Message({ theme: 'danger', message: e?.message || '保存失败' });
  } finally {
    loading.value = false;
  }
};

const handleReset = () => {
  if (originalData.value) {
    Object.assign(formData, JSON.parse(JSON.stringify(originalData.value)));
  } else {
    Object.assign(formData, initialState);
  }
  nextTick(() => {
    handleValidate(false); // 重置校验状态
  });
};

const isHostApplyTimeChanged = computed(() => {
  return timeUTCFormatter(formData.host_apply_time) !== timeUTCFormatter(originalData.value?.host_apply_time);
});

const getDuplicateBizIds = () => {
  // 检查裁撤基数调整表格中是否有重复的业务
  const offsets = formData.quota_offsets || [];
  const bizIdMap = new Map<number, number>();
  const duplicateBizIds = new Set<number>();
  for (const item of offsets) {
    if (item.bk_biz_id !== undefined && item.bk_biz_id !== null) {
      if (bizIdMap.has(item.bk_biz_id)) {
        duplicateBizIds.add(item.bk_biz_id);
      } else {
        bizIdMap.set(item.bk_biz_id, 1);
      }
    }
  }
  return { bizIdMap, duplicateBizIds };
};
const bizIdMapSize = computed(() => getDuplicateBizIds().bizIdMap.size);

const rules = {
  host_apply_time: [
    {
      required: true,
      message: '请选择裁撤开始时间',
      trigger: 'change',
    },
  ],
  approval_limit: [
    {
      required: true,
      message: '请填写自动审批配置',
      trigger: 'change',
    },
    {
      validator: (value: number) => value >= 0 && value <= 100,
      message: '自动审批配置必须在 0-100 之间',
      trigger: 'change',
    },
  ],
  quota_coefficient: [
    {
      required: true,
      message: '请填写裁撤申领上限',
      trigger: 'change',
    },
    {
      validator: (value: number) => value >= 1 && value <= 100,
      message: '裁撤申领上限必须在 1-100 之间',
      trigger: 'change',
    },
  ],
};

const handleAdd = () => {
  quotaOffsetTableRef.value?.addRow();
};
</script>

<template>
  <bk-sideslider
    v-model:is-show="isShow"
    :width="960"
    title="裁撤配置"
    :quick-close="false"
    background-color="#f5f7fa"
    render-directive="if"
  >
    <div class="dissolve-config-sideslider">
      <bk-loading :loading="loading">
        <!-- 基础配置 -->
        <Panel title="基础配置" class="config-panel">
          <p class="section-tip">
            <InfoLine fill="#4D4F56" width="14" height="14" />
            <span>累计占用比例 ≤ 自动审批配置百分比时自动过单；裁撤允许额度 = 原裁撤额度 × 裁撤申领上限百分比。</span>
          </p>

          <bk-form ref="formRef" class="form" form-type="vertical" :model="formData" :rules="rules">
            <div class="basic-fields-row">
              <div class="basic-field-item">
                <span class="field-label">
                  申领统计开始时间
                  <i class="required-mark">*</i>
                </span>
                <bk-form-item property="host_apply_time" required>
                  <hcm-form-datetime
                    v-model="formData.host_apply_time"
                    style="width: 100%"
                    append-to-body
                    clearable
                    placeholder="请选择"
                    type="datetime"
                  />
                  <p class="warning" v-if="isHostApplyTimeChanged">
                    请关注，裁撤周期变化，裁撤基数有{{ bizIdMapSize }}个业务调整了额度
                  </p>
                </bk-form-item>
              </div>

              <div class="basic-field-item">
                <span class="field-label">
                  自动审批配置占比
                  <InfoLine
                    class="ml5"
                    fill="#c7c7c7"
                    width="14"
                    height="14"
                    v-bk-tooltips="{ content: '申领核数达到裁撤核数的比例，则触发人工审核' }"
                  />
                  <i class="required-mark">*</i>
                </span>
                <bk-form-item property="approval_limit" required>
                  <bk-input
                    v-model.number="formData.approval_limit"
                    type="number"
                    suffix="%"
                    :precision="2"
                    :max="100"
                    :min="0"
                  />
                </bk-form-item>
              </div>

              <div class="basic-field-item">
                <span class="field-label">
                  裁撤申领上限占比
                  <InfoLine
                    class="ml5"
                    fill="#c7c7c7"
                    width="14"
                    height="14"
                    v-bk-tooltips="{ content: '调整所有业务实际可申领的上限百分比，即业务原裁撤额度 * 比例' }"
                  />
                  <i class="required-mark">*</i>
                </span>
                <bk-form-item property="quota_coefficient" required>
                  <bk-input
                    v-model="formData.quota_coefficient"
                    type="number"
                    suffix="%"
                    :precision="2"
                    :max="100"
                    :min="1"
                  />
                </bk-form-item>
              </div>
            </div>
          </bk-form>
        </Panel>

        <!-- 裁撤时间段与项目类型 -->
        <Panel title="裁撤时间段与项目类型" class="config-panel">
          <div class="time-period-button">
            <bk-button @click="addTimePeriod">
              <Plus />
              新增时间段
            </bk-button>
            <p class="period-tip">
              <InfoLine fill="#4D4F56" width="14" height="14" />
              <span>配置裁撤时间段与关联的项目类型，用于列表筛选。</span>
            </p>
          </div>

          <div class="time-period-list">
            <template v-for="(_, index) in formData.dissolve_projects" :key="index">
              <TimePeriodBlock
                ref="timePeriodBlockRefs"
                :index="index"
                v-model="formData.dissolve_projects[index]"
                @remove="removeTimePeriod(index)"
              />
            </template>
            <p v-if="!formData.dissolve_projects.length" class="empty-tip">暂无数据</p>
          </div>
        </Panel>

        <!-- 裁撤额度调整 -->
        <Panel title="裁撤额度调整" class="config-panel">
          <p class="section-tip">
            <InfoLine fill="#4D4F56" width="14" height="14" />
            <span>
              按业务单独调整裁撤 CPU 核数，用于裁撤额度变更场景；选择调增或调减后输入核数，调整后核数 =
              原裁撤核数*申请系数 ± 调整核数。
            </span>
          </p>
          <p class="offset-table-action">
            <bk-button theme="primary" text class="add-btn" @click="handleAdd">
              <i class="hcm-icon bkhcm-icon-plus-circle-shape"></i>
              新增
            </bk-button>
          </p>
          <QuotaOffsetTable ref="quotaOffsetTableRef" v-model="formData.quota_offsets" />
        </Panel>
      </bk-loading>
    </div>

    <template #footer>
      <div class="action-btns">
        <bk-button theme="primary" :loading="loading" @click="handleConfirm">保存配置</bk-button>
        <bk-button @click="handleReset">重置</bk-button>
        <bk-button @click="isShow = false">取消</bk-button>
      </div>
    </template>
  </bk-sideslider>
</template>

<style lang="scss" scoped>
.dissolve-config-sideslider {
  color: #4d4f56;
  padding: 16px 24px;

  :deep(.bk-form-item) {
    margin: 0;
  }

  .config-panel {
    padding-bottom: 24px;

    & + .config-panel {
      margin-top: 16px;
    }
  }

  .section-tip {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    line-height: 20px;
    margin-bottom: 16px;
  }

  // ---- 基础配置 三列横排 ----
  .basic-fields-row {
    display: flex;
    gap: 32px;
  }

  .basic-field-item {
    flex: 1;

    // 申领统计开始时间稍宽
    &:first-child {
      flex: 1.3;
    }

    .field-label {
      font-size: 12px;
      line-height: 20px;
      margin-bottom: 6px;
      display: flex;
      align-items: center;

      .required-mark {
        color: #ea3636;
        font-style: normal;
        margin-left: 6px;
      }
    }

    :deep(.bk-input-wrapper) {
      width: 100%;
    }

    :deep(.bk-form-control) {
      width: 100%;
    }
  }

  // ---- 裁撤时间段 ----
  .time-period-button {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 16px;
  }

  .period-tip {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    line-height: 20px;
  }

  .empty-tip {
    color: #c4c6cc;
    text-align: center;
    padding: 40px 0;
    font-size: 12px;
  }
}

.action-btns {
  display: flex;
  gap: 8px;
}

.warning {
  color: #ea3636;
  font-size: 12px;
  margin-top: 8px;
  line-height: 1;
}

.offset-table-action {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;

  .add-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;

    i {
      margin-right: 4px;
    }

    .icon-plus {
      font-size: 14px;
    }
  }
}
</style>
