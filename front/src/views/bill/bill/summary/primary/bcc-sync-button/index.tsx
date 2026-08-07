import { computed, defineComponent, ref, watch } from 'vue';
import cssModule from './index.module.scss';

import { Alert, Button, Checkbox, Dialog, Message, Radio } from 'bkui-vue';
import VendorRadioGroup from '@/components/vendor-radio-group';

import { useI18n } from 'vue-i18n';
import { VendorEnum } from '@/common/constant';
import {
  reqBillsAdjustmentSum,
  reqBillsExchangeRateList,
  reqBillsRootAccountSummaryList,
  reqBillsRootAccountSummarySum,
  syncRecordsBills,
} from '@/api/bill';
import { QueryRuleOPEnum } from '@/typings';
import { AdjustmentItemSumResult, BillsRootAccountSummaryState, BillsSummarySum } from '@/typings/bill';
import { formatBillCost } from '@/utils';

const { Group: RadioGroup } = Radio;

export default defineComponent({
  props: {
    billYear: { type: Number, required: true },
    billMonth: { type: Number, required: true },
  },
  setup(props, { expose }) {
    const { t } = useI18n();

    const isShow = ref(false);
    const isLoading = ref(false);
    const vendor = ref(VendorEnum.AZURE);
    const syncMode = ref<'full' | 'adjustment_only'>('full');
    const syncInfo = ref<BillsSummarySum>(null);
    const adjustmentInfo = ref<AdjustmentItemSumResult>(null);
    // 当月汇率（1美金兑换的人民币值），数据来源与云账单管理头部的当月汇率一致
    const exchangeRate = ref('');

    const isChecked = ref(false);
    const isConfirmAllBills = ref(false);

    const adjustmentCNYCost = computed(() => {
      const increase = Number(adjustmentInfo.value?.cost_map?.increase?.CNY?.Cost || 0);
      const decrease = Number(adjustmentInfo.value?.cost_map?.decrease?.CNY?.Cost || 0);
      return increase - decrease;
    });

    const adjustmentUSDCost = computed(() => {
      const increase = Number(adjustmentInfo.value?.cost_map?.increase?.USD?.Cost || 0);
      const decrease = Number(adjustmentInfo.value?.cost_map?.decrease?.USD?.Cost || 0);
      return increase - decrease;
    });

    // 总金额（人民币+美金）：总金额（美金）按当月汇率转成人民币后，再加上总金额（人民币）
    const totalCombinedRMBCost = computed(() => {
      const usdCost = Number(syncInfo.value?.cost_map?.USD?.Cost || 0);
      const rmbCost = Number(syncInfo.value?.cost_map?.USD?.RMBCost || 0);
      const rate = Number(exchangeRate.value || 0);
      return usdCost * rate + rmbCost;
    });

    // 调账金额（人民币+美金）：调账金额（美金）按当月汇率转成人民币后，再加上调账金额（人民币）
    const adjustmentCombinedRMBCost = computed(() => {
      const rate = Number(exchangeRate.value || 0);
      return adjustmentUSDCost.value * rate + adjustmentCNYCost.value;
    });

    const canSyncBills = computed(() => {
      if (!isChecked.value) return false;
      if (syncMode.value === 'full') return isConfirmAllBills.value;
      return true;
    });

    const syncDisabledReason = computed(() => {
      if (!isChecked.value) return t('请勾选确认复选框');
      return t('当前云厂商下所有一级账号账单有未确认的账单，无法同步');
    });

    const triggerShow = (v: boolean) => {
      isShow.value = v;
    };

    const getVendorSyncInfo = async (vendor: VendorEnum) => {
      const res = await reqBillsRootAccountSummarySum({
        bill_year: props.billYear,
        bill_month: props.billMonth,
        filter: { op: QueryRuleOPEnum.AND, rules: [{ field: 'vendor', op: QueryRuleOPEnum.EQ, value: vendor }] },
      });
      syncInfo.value = res.data;
    };

    // 获取当月汇率（USD -> CNY），与云账单管理头部展示的当月汇率保持一致
    const getExchangeRate = async () => {
      const res = await reqBillsExchangeRateList({
        filter: {
          op: QueryRuleOPEnum.AND,
          rules: [
            { field: 'year', op: QueryRuleOPEnum.EQ, value: props.billYear },
            { field: 'month', op: QueryRuleOPEnum.EQ, value: props.billMonth },
            { field: 'from_currency', op: QueryRuleOPEnum.EQ, value: 'USD' },
            { field: 'to_currency', op: QueryRuleOPEnum.EQ, value: 'CNY' },
          ],
        },
        page: { start: 0, limit: 10, count: false },
      });
      exchangeRate.value = res.data?.details[0]?.exchange_rate || '';
    };

    const getVendorAdjustmentInfo = async (vendor: VendorEnum) => {
      const res = await reqBillsAdjustmentSum({
        filter: {
          op: QueryRuleOPEnum.AND,
          rules: [
            { field: 'vendor', op: QueryRuleOPEnum.EQ, value: vendor },
            { field: 'bill_year', op: QueryRuleOPEnum.EQ, value: props.billYear },
            { field: 'bill_month', op: QueryRuleOPEnum.EQ, value: props.billMonth },
          ],
        },
      });
      adjustmentInfo.value = res.data;
    };

    // 只有当某个云厂商下所有一级账号账单都处于确认状态后，才能进行同步
    const checkVendorAllBillsAreConfirmed = async (vendor: VendorEnum) => {
      const res = await reqBillsRootAccountSummaryList({
        bill_year: props.billYear,
        bill_month: props.billMonth,
        filter: { op: QueryRuleOPEnum.AND, rules: [{ field: 'vendor', op: QueryRuleOPEnum.EQ, value: vendor }] },
        // 一级账号一般不会超过500个
        page: { count: false, start: 0, limit: 500 },
      });
      isConfirmAllBills.value = !res.data.details.find((item) => item.state !== BillsRootAccountSummaryState.confirmed);
    };

    const resetState = () => {
      syncMode.value = 'full';
      isChecked.value = false;
      syncInfo.value = null;
      adjustmentInfo.value = null;
      exchangeRate.value = '';
    };

    const handleConfirm = async () => {
      isLoading.value = true;
      try {
        await syncRecordsBills({
          bill_year: props.billYear,
          bill_month: props.billMonth,
          vendor: vendor.value,
          sync_mode: syncMode.value,
        });
        Message({ theme: 'success', message: t('提交同步请求成功') });
        triggerShow(false);
        resetState();
      } finally {
        isLoading.value = false;
      }
    };

    const loadPreviewData = () => {
      if (syncMode.value === 'full') {
        getVendorSyncInfo(vendor.value);
        getExchangeRate();
        checkVendorAllBillsAreConfirmed(vendor.value);
      } else {
        getVendorAdjustmentInfo(vendor.value);
        getExchangeRate();
      }
    };

    watch(isShow, (show) => {
      if (!show) {
        resetState();
        return;
      }
      loadPreviewData();
    });

    watch([() => vendor.value, () => syncMode.value], () => {
      if (!isShow.value) return;
      isChecked.value = false;
      loadPreviewData();
    });

    expose({ triggerShow });

    return () => (
      <>
        <Button theme='primary' onClick={() => triggerShow(true)}>
          {t('同步')}
        </Button>
        <Dialog
          v-model:isShow={isShow.value}
          width={800}
          title={t('账单同步')}
          isLoading={isLoading.value}
          onConfirm={handleConfirm}>
          {{
            default: () => (
              <>
                <Alert theme='warning'>{t('账单同步，会将当前云厂商的数据同步给公司OBS平台')}</Alert>
                <section class={cssModule['vendor-wrapper']}>
                  <div class={cssModule.title}>{t('云厂商')}</div>
                  <VendorRadioGroup v-model={vendor.value} size='small' />
                </section>
                <section class={cssModule['sync-scope-wrapper']}>
                  <div class={cssModule.title}>{t('同步范围')}</div>
                  <RadioGroup v-model={syncMode.value}>
                    <Radio label='full'>{t('全量同步')}</Radio>
                    <Radio label='adjustment_only'>{t('仅同步调账数据')}</Radio>
                  </RadioGroup>
                </section>
                {syncMode.value === 'full' ? (
                  <section class={cssModule['sync-content-wrapper']}>
                    <div class={cssModule.title}>{t('同步内容')}</div>
                    <div class={cssModule.block}>
                      <div class={cssModule.item}>
                        <span class={cssModule['short-label']}>{t('总金额（人民币+美金）')}</span>
                        <span class={cssModule.money}>￥{formatBillCost(String(totalCombinedRMBCost.value))}</span>
                      </div>
                      <div class={cssModule.item}>
                        <span class={cssModule['short-label']}>{t('总金额（人民币）')}</span>
                        <span class={cssModule.money}>￥{formatBillCost(syncInfo.value?.cost_map?.USD?.RMBCost)}</span>
                      </div>
                      <div class={cssModule.item}>
                        <span class={cssModule['short-label']}>{t('总金额（美金）')}</span>
                        <span class={cssModule.money}>＄{formatBillCost(syncInfo.value?.cost_map?.USD?.Cost)}</span>
                      </div>
                      <div class={cssModule.item}>
                        <span class={cssModule['short-label']}>{t('业务数量')}</span>
                        <span class={cssModule.count}>{syncInfo.value?.count || 0}</span>
                      </div>
                    </div>
                  </section>
                ) : (
                  <section class={cssModule['sync-content-wrapper']}>
                    <div class={cssModule.title}>{t('同步内容')}</div>
                    <div class={cssModule.block}>
                      <div class={cssModule.item}>
                        <span class={cssModule.label}>{t('调账金额（人民币+美金）')}</span>
                        <span class={cssModule.money}>￥{formatBillCost(String(adjustmentCombinedRMBCost.value))}</span>
                      </div>
                      <div class={cssModule.item}>
                        <span class={cssModule.label}>{t('调账金额（人民币）')}</span>
                        <span class={cssModule.money}>￥{formatBillCost(String(adjustmentCNYCost.value))}</span>
                      </div>
                      <div class={cssModule.item}>
                        <span class={cssModule.label}>{t('调账金额（美金）')}</span>
                        <span class={cssModule.money}>＄{formatBillCost(String(adjustmentUSDCost.value))}</span>
                      </div>
                      <div class={cssModule.item}>
                        <span class={cssModule.label}>{t('调账记录数量')}</span>
                        <span class={cssModule.count}>{adjustmentInfo.value?.count || 0}</span>
                      </div>
                    </div>
                  </section>
                )}
                <Alert theme='info' class={cssModule.mb12}>
                  {t('在操作前，请确保当前账单核对无误后，再进行同步操作。检查的步骤如下：')}
                  <br />
                  {t('1.一级账号的本地金额和一级账号的云上金额核对')}
                  <br />
                  {t('2.一级账号总金额和二级账号总金额核对')}
                  <br />
                  {t('3.一级账号当月金额和一级账号上月金额环比')}
                </Alert>
                <Checkbox v-model={isChecked.value}>{t('已确认所有步骤正确，可以触发同步操作')}</Checkbox>
              </>
            ),
            footer: () => (
              <>
                <Button
                  class={cssModule.button}
                  theme='primary'
                  disabled={!canSyncBills.value}
                  onClick={handleConfirm}
                  loading={isLoading.value}
                  v-bk-tooltips={{
                    content: syncDisabledReason.value,
                    disabled: canSyncBills.value,
                  }}>
                  {t('同步')}
                </Button>
                <Button class={cssModule.button} onClick={() => triggerShow(false)}>
                  {t('取消')}
                </Button>
              </>
            ),
          }}
        </Dialog>
      </>
    );
  },
});
