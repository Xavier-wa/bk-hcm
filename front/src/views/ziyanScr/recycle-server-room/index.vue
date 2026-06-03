<script setup lang="ts">
import { ref, computed } from 'vue';
import Search from './search';
import Table from './table';
import QuotaOffsetTable from './quota-offset/index.vue';
import { useVerify } from '@/hooks';
import ErrorPage from '@/views/error-pages/403';
import { InfoLine } from 'bkui-vue/lib/icon';
import { Form, Message } from 'bkui-vue';
import { IDissolveConfig, useDissolveQuotaStore } from '@/store/dissolve/quota';
import { AUTH_FIND_DISSOLVE, AUTH_UPDATE_DISSOLVE } from '@/constants/auth-symbols';
import { useBusinessGlobalStore } from '@/store/business-global';
import { timeUTCFormatter } from '@/common/util';

const dissolveStore = useDissolveQuotaStore();
const businessGlobalStore = useBusinessGlobalStore();

const { authVerifyData } = useVerify();
const moduleNames = ref<string[]>([]);

// 在search中，模块名是 `which_stages__module_name` 的格式，这里需要提取出 module_name
const moduleNameList = computed(() => {
  const names = moduleNames.value.map((item) => item.split('__')[1]).filter(Boolean);
  return [...new Set(names)];
});

const isShowConfig = ref(false);
const formRef = ref<InstanceType<typeof Form>>();
const originalHostApplyTime = ref<string>('');
const formData = ref<IDissolveConfig>({
  host_apply_time: '',
  approval_limit: 0,
  quota_coefficient: 0,
  quota_offsets: [],
});

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

const quotaOffsetTableRef = ref<InstanceType<typeof QuotaOffsetTable>>();

const handleConfirm = async () => {
  try {
    const [formValid, quotaOffsetValid] = await Promise.all([
      formRef.value?.validate(),
      quotaOffsetTableRef.value?.validate?.(),
    ]);

    if (!formValid) {
      return;
    }
    if (quotaOffsetValid === false) {
      Message({
        theme: 'warning',
        message: '请检查裁撤基数调整表格',
      });
      return;
    }

    const { duplicateBizIds } = getDuplicateBizIds();
    if (duplicateBizIds.size > 0) {
      const bizNames = Array.from(duplicateBizIds)
        .map((id) => businessGlobalStore.businessFullList.find((biz) => biz.id === id)?.name || String(id))
        .join('、');
      Message({
        theme: 'error',
        message: `不允许选择重复的业务：${bizNames}`,
      });
      return;
    }

    await dissolveStore.upsertDissolveConfig(formData.value);
    Message({
      theme: 'success',
      message: '提交成功',
    });
    isShowConfig.value = false;
  } catch (e: any) {
    console.error(e);
  }
};

const handleShowConfig = async () => {
  formData.value = await dissolveStore.getDissolveConfig();
  if (!Array.isArray(formData.value.quota_offsets)) {
    formData.value.quota_offsets = [];
  }
  originalHostApplyTime.value = formData.value.host_apply_time || '';
  isShowConfig.value = true;
};

const handleAdd = () => {
  quotaOffsetTableRef.value?.addRow();
};

const getDuplicateBizIds = () => {
  // 检查裁撤基数调整表格中是否有重复的业务
  const offsets = formData.value.quota_offsets || [];
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

const isHostApplyTimeChanged = computed(() => {
  return timeUTCFormatter(formData.value.host_apply_time) !== timeUTCFormatter(originalHostApplyTime.value);
});
</script>

<template>
  <ErrorPage
    v-if="!authVerifyData.permissionAction.service_resource_dissolve_find"
    url-key-id="biz_ziyan_resource_dissolve"
  />

  <section v-else class="home">
    <Search v-model:module-names="moduleNames"></Search>
    <Table :module-names="moduleNameList"></Table>

    <hcm-auth v-slot="{ noPerm }" :sign="[{ type: AUTH_FIND_DISSOLVE }, { type: AUTH_UPDATE_DISSOLVE }]">
      <template v-if="!noPerm">
        <Teleport defer to="#breadcrumbExtra">
          <bk-button theme="primary" @click="handleShowConfig">裁撤配置</bk-button>
        </Teleport>

        <bk-dialog v-model:is-show="isShowConfig" title="裁撤配置" width="900" render-directive="if">
          <bk-loading title="提交中" :loading="dissolveStore.upsertDissolveConfigLoading">
            <div class="dissolve-config mt25">
              <p class="tips mb15">
                <InfoLine fill="#3a84ff" width="20" height="20" />
                <span class="ml6">配置机房裁撤的全局参数</span>
              </p>

              <bk-form ref="formRef" class="form" form-type="vertical" :model="formData" :rules="rules">
                <bk-form-item property="host_apply_time" required>
                  <template #label>
                    <p class="mb6" style="display: inline-block">裁撤开始时间</p>
                  </template>
                  <hcm-form-datetime
                    v-model="formData.host_apply_time"
                    style="width: 100%"
                    append-to-body
                    clearable
                    placeholder="选择裁撤开始时间"
                    font-size="medium"
                    type="datetime"
                  />
                  <p class="warning" v-show="isHostApplyTimeChanged">
                    请关注，裁撤周期变化，裁撤基数有{{ bizIdMapSize }}个业务调整了额度
                  </p>
                </bk-form-item>

                <bk-form-item property="approval_limit" required>
                  <template #label>
                    <p class="tips-2 mb6">
                      <span>自动审批配置</span>
                      <InfoLine
                        class="ml5"
                        fill="#c7c7c7"
                        width="16"
                        height="16"
                        v-bk-tooltips="{ content: '申领核数达到裁撤核数的比例，则触发人工审核' }"
                      />
                    </p>
                  </template>
                  <bk-input
                    v-model="formData.approval_limit"
                    style="font-size: 14px"
                    type="number"
                    prefix="触发审批值"
                    suffix="%"
                    :precision="2"
                    :max="100"
                    :min="0"
                  />
                </bk-form-item>

                <bk-form-item property="quota_coefficient" required>
                  <template #label>
                    <p class="tips-2 mb6">
                      <span>裁撤申领上限（全局）</span>
                      <InfoLine
                        class="ml5"
                        fill="#c7c7c7"
                        width="16"
                        height="16"
                        v-bk-tooltips="{ content: '调整所有业务实际可申领的上限百分比，即业务原裁撤额度 * 比例' }"
                      />
                    </p>
                  </template>
                  <bk-input
                    v-model="formData.quota_coefficient"
                    style="font-size: 14px"
                    type="number"
                    prefix="申领上限比例"
                    suffix="%"
                    :precision="2"
                    :max="100"
                    :min="1"
                  />
                </bk-form-item>
              </bk-form>

              <div class="mb6 quota-offset-title">
                <div class="tips-2">
                  <span>裁撤基数调整（业务）</span>
                  <InfoLine
                    class="ml5"
                    fill="#c7c7c7"
                    width="16"
                    height="16"
                    v-bk-tooltips="{ content: '按需配置，业务在裁撤实际申领上限额度的基础上，所调整的额度' }"
                  />
                </div>

                <bk-button theme="primary" text class="add-btn" @click="handleAdd">
                  <i class="hcm-icon bkhcm-icon-plus-circle-shape"></i>
                  新增
                </bk-button>
              </div>
              <QuotaOffsetTable ref="quotaOffsetTableRef" v-model="formData.quota_offsets" />
            </div>
          </bk-loading>

          <template #footer>
            <div class="btns">
              <bk-button theme="primary" class="mr10" @click="handleConfirm">确定</bk-button>
              <bk-button @click="isShowConfig = false">取消</bk-button>
            </div>
          </template>
        </bk-dialog>
      </template>
    </hcm-auth>
  </section>
</template>

<style lang="scss" scoped>
.home {
  padding: 24px;
}

.tips {
  display: flex;
  align-items: center;

  span {
    color: #c4c6cc;
  }
}

.tips-2 {
  display: inline-flex;
  align-items: center;
}

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

.quota-offset-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.warning {
  color: #ea3636;
  font-size: 12px;
}
</style>
