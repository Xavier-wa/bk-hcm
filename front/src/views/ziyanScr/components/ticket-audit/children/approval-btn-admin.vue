<script setup lang="ts">
import { computed, inject, onMounted, Ref, ref, useAttrs } from 'vue';
import { useI18n } from 'vue-i18n';
import useFormModel from '@/hooks/useFormModel';
import { useResSubTicketStore, SubTicketItem, SubTicketDetail } from '@/store/ticket/res-sub-ticket';
import { useRoute } from 'vue-router';
import { GLOBAL_BIZS_KEY } from '@/common/constant';

interface IProps {
  loading?: boolean;
  confirmHandler: (formModel: IFormModel) => Promise<any>;
}
interface IFormModel {
  approval: boolean | null;
  use_transfer_pool: boolean;
  operate_info: string;
}

const props = withDefaults(defineProps<IProps>(), {
  loading: false,
});
const emit = defineEmits<(e: 'shown' | 'hidden') => void>();
const attrs = useAttrs();
const store = useResSubTicketStore();
const { t } = useI18n();
const route = useRoute();
const bizId = computed(() => Number(route.query[GLOBAL_BIZS_KEY]));

const ticketDetail = inject<Ref<SubTicketDetail>>('ticketDetail');
const ticketListData = inject<Ref<SubTicketItem>>('ticketListData');

const isShow = ref(false);
const isConfirmLoading = ref(false);
const { formModel, resetForm } = useFormModel<IFormModel>({
  approval: null,
  use_transfer_pool: false,
  operate_info: '',
});

// 确定按钮是否可点击：必须选择同意或拒绝
const isConfirmDisabled = computed(() => formModel.approval === null);

// 切换审批意见时重置 use_transfer_pool
const handleApprovalChange = () => {
  formModel.use_transfer_pool = false;
};

const handleShown = () => {
  emit('shown');
};
const handleHidden = () => {
  emit('hidden');
};

const handleConfirm = async () => {
  isConfirmLoading.value = true;
  try {
    // 传递普通对象副本，避免 resetForm 后影响已传递的数据
    await props.confirmHandler({
      approval: formModel.approval as boolean, // 此时一定已选择
      use_transfer_pool: formModel.use_transfer_pool,
      operate_info: formModel.operate_info,
    });
    isShow.value = false;
    resetForm();
  } catch (error) {
    console.error(error);
    return Promise.reject(error);
  } finally {
    isConfirmLoading.value = false;
    getSummaryData();
  }
};

const currentCore = computed(() => {
  return ticketListData.value?.updated_info?.cvm?.cpu_core;
});

// 额度获取和展示
const remainCore = ref(0);
const approvalCore = ref(0);
const getSummaryData = async () => {
  // 获取剩余额度
  const params: any = {
    obs_project: ticketDetail.value.demands.map((it) => it?.updated_info?.obs_project),
    technical_class: ticketDetail.value.demands.map((it) => it?.updated_info?.cvm?.technical_class),
    year: new Date().getFullYear(),
  };
  if (bizId.value) {
    params.bk_biz_id = [bizId.value];
  }
  const remainRes = await store.getTransferQuotaSummary(params, bizId.value); // 获取剩余额度
  const approvalRes = await store.getTransferQuotaConfigs(); // 获取审批额度

  remainCore.value = remainRes.data?.remain_quota || 0;
  approvalCore.value = approvalRes.data?.audit_quota || 0;
};

onMounted(() => {
  getSummaryData();
});
</script>

<template>
  <bk-button
    class="approval-btn"
    :class="attrs.class"
    size="small"
    theme="primary"
    :loading="loading"
    @click="isShow = true"
  >
    {{ t('立即处理') }}
  </bk-button>
  <bk-dialog v-model:is-show="isShow" :title="t('审批')" @shown="handleShown" @hidden="handleHidden">
    <div class="info">
      <bk-form label-width="120">
        <bk-form-item :label="t('审批节点：')">管理员审批</bk-form-item>
        <bk-form-item :label="t('当前核数：')">
          <span class="light">{{ currentCore }} 核</span>
        </bk-form-item>
        <bk-form-item :label="t('审批额度：')">
          <span class="light">{{ approvalCore }} 核</span>
        </bk-form-item>
        <bk-form-item :label="t('中转池剩余额度：')">
          <span class="light">{{ remainCore }} 核</span>
        </bk-form-item>
      </bk-form>
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
      <bk-button @click="isShow = false">
        {{ t('取消') }}
      </bk-button>
    </template>
  </bk-dialog>
</template>

<style scoped lang="scss">
.approval-btn {
  font-size: 14px;
  font-weight: normal;
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

.info {
  width: 100%;
  background-color: #f5f7fa;
  margin-bottom: 20px;
  padding: 8px;

  .light {
    color: #f59500;
  }

  :deep(.bk-form-item) {
    margin-bottom: 8px;
  }

  :deep(.bk-form-label) {
    padding-right: 0;
  }
}
</style>
