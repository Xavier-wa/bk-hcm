<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Message, Tag } from 'bkui-vue';
import { useResSubTicketStore, SubTicketItem } from '@/store/ticket/res-sub-ticket';
import { useRoute } from 'vue-router';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import useFormModel from '@/hooks/useFormModel';
import { TicketByIdResult } from '@/typings/resourcePlan';

interface IFormModel {
  approval: boolean | null;
  use_transfer_pool: boolean;
  operate_info: string;
}

interface Props {
  selectedItems: SubTicketItem[];
  demands?: TicketByIdResult['demands'];
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: 'success'): void;
}>();

const { t } = useI18n();
const store = useResSubTicketStore();
const route = useRoute();
const bizId = computed(() => Number(route.query[GLOBAL_BIZS_KEY]));

const isShow = ref(false);
const isConfirmLoading = ref(false);
const { formModel, resetForm } = useFormModel<IFormModel>({
  approval: null,
  use_transfer_pool: false,
  operate_info: '',
});

// 未选择审批意见时禁用确定按钮
const isConfirmDisabled = computed(() => formModel.approval === null);

// 切换审批意见时重置 use_transfer_pool
const handleApprovalChange = () => {
  formModel.use_transfer_pool = false;
};

// 子单合计总核数：累加选中子单的 updated_info.cvm.cpu_core
const totalCpuCore = computed(() => {
  return props.selectedItems.reduce((sum, item) => {
    const core = item.updated_info?.cvm?.cpu_core || 0;
    return sum + core;
  }, 0);
});

// 中转池剩余额度
const remainCore = ref(0);
const isQuotaLoading = ref(false);

// 获取中转池剩余额度
const getQuotaSummary = async () => {
  if (!props.demands?.length) return;

  isQuotaLoading.value = true;
  try {
    const params: any = {
      obs_project: props.demands.map((it) => it?.updated_info?.obs_project),
      technical_class: props.demands.map((it) => (it?.updated_info?.cvm as any)?.technical_class),
      year: new Date().getFullYear(),
    };
    if (bizId.value) {
      params.bk_biz_id = [bizId.value];
    }
    const res = await store.getTransferQuotaSummary(params, bizId.value);
    remainCore.value = res.data?.remain_quota || 0;
  } catch {
    remainCore.value = 0;
  } finally {
    isQuotaLoading.value = false;
  }
};

// 弹窗打开时获取额度
watch(isShow, (val) => {
  if (val) {
    getQuotaSummary();
  }
});

const open = () => {
  isShow.value = true;
};

const close = () => {
  isShow.value = false;
  resetForm();
};

const handleConfirm = async () => {
  if (!props.selectedItems.length) {
    Message({ theme: 'warning', message: t('请先选择要审批的子单') });
    return;
  }

  isConfirmLoading.value = true;
  try {
    const params = {
      sub_ticket_ids: props.selectedItems.map((item) => item.id),
      approval: formModel.approval as boolean,
      use_transfer_pool: formModel.use_transfer_pool,
      operate_info: formModel.operate_info || undefined,
    };

    await store.batchApproveAdminNode(params, bizId.value);
    Message({ theme: 'success', message: t('批量审批请求已提交，5s后自动刷新') });
    close();
    emit('success');
  } catch (error) {
    console.error(error);
  } finally {
    isConfirmLoading.value = false;
  }
};

defineExpose({
  open,
  close,
});
</script>

<template>
  <bk-dialog v-model:is-show="isShow" :title="t('批量审批')" width="600" @hidden="resetForm">
    <div class="info">
      <p class="selected-info">
        {{ t('已选择') }}
        <span class="count">{{ selectedItems.length }}</span>
        {{ t('个子单进行批量审批') }}
      </p>
      <div class="selected-ids">
        <Tag v-for="item in selectedItems" :key="item.id" class="id-tag">
          {{ item.id }}
        </Tag>
      </div>
      <div class="quota-info">
        <div class="quota-item">
          <span class="label">{{ t('子单合计总核数') }}：</span>
          <span class="value highlight">{{ totalCpuCore }} {{ t('核') }}</span>
        </div>
        <div class="quota-item">
          <span class="label">{{ t('中转池剩余额度') }}：</span>
          <span class="value highlight">{{ remainCore }} {{ t('核') }}</span>
        </div>
      </div>
    </div>

    <bk-form form-type="vertical" :model="formModel">
      <bk-form-item :label="t('审批意见')" property="approval" required>
        <bk-radio-group v-model="formModel.approval" @change="handleApprovalChange">
          <bk-radio :label="true">{{ t('同意') }}</bk-radio>
          <bk-radio :label="false">{{ t('拒绝') }}</bk-radio>
        </bk-radio-group>
      </bk-form-item>
      <!-- 使用中转池额度：只在选择"同意"时显示 -->
      <bk-form-item v-if="formModel.approval === true" property="use_transfer_pool">
        <bk-checkbox v-model="formModel.use_transfer_pool">{{ t('使用中转池额度') }}</bk-checkbox>
      </bk-form-item>
      <bk-form-item :label="t('审批理由')" property="operate_info">
        <bk-input
          v-model="formModel.operate_info"
          type="textarea"
          :placeholder="t('未输入')"
          :maxlength="100"
          :rows="3"
          :resize="false"
        />
      </bk-form-item>
    </bk-form>

    <template #footer>
      <bk-button theme="primary" :loading="isConfirmLoading" :disabled="isConfirmDisabled" @click="handleConfirm">
        {{ t('确定') }}
      </bk-button>
      <bk-button @click="close">
        {{ t('取消') }}
      </bk-button>
    </template>
  </bk-dialog>
</template>

<style scoped lang="scss">
.info {
  background-color: #f5f7fa;
  padding: 12px 16px;
  margin-bottom: 20px;
  border-radius: 2px;

  .selected-info {
    color: #63656e;
    font-size: 14px;
    margin-bottom: 8px;

    .count {
      color: #3a84ff;
      font-weight: 600;
    }
  }

  .selected-ids {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 12px;

    .id-tag {
      margin: 0;
    }
  }

  .quota-info {
    .quota-item {
      display: flex;
      align-items: center;
      line-height: 24px;

      .label {
        color: #63656e;
      }

      .value {
        &.highlight {
          color: #f59500;
          font-weight: 600;
        }
      }
    }
  }
}

:deep(.bk-dialog-footer) {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;

  .bk-button {
    min-width: 88px;
  }
}

// 审批表单间距调整
:deep(.bk-form-item) {
  margin-bottom: 12px;
}
</style>
