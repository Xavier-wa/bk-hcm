import { defineComponent, ref } from 'vue';
import { useRouter } from 'vue-router';
import cssModule from './index.module.scss';

import { ArrowsLeft } from 'bkui-vue/lib/icon';
import { BkRadioButton, BkRadioGroup } from 'bkui-vue/lib/radio';
import Panel from '@/components/panel';

import { useI18n } from 'vue-i18n';
import useColumns from '@/views/resource/resource-manage/hooks/use-columns';
import { useTable } from '@/hooks/useTable/useTable';
import { reqBillsExchangeRateList, reqBillsSyncRecordList } from '@/api/bill';
import { QueryRuleOPEnum } from '@/typings';

export default defineComponent({
  name: 'BillSummaryOperationRecord',
  setup() {
    const { t } = useI18n();
    const router = useRouter();
    const { columns } = useColumns('billsSummaryOperationRecord');

    const actionTypes = [
      { label: 'sync', text: t('同步'), disabled: false },
      { label: 'confirm', text: t('确认'), disabled: true },
      { label: 'import', text: t('导入'), disabled: true },
    ];
    const activeActionType = ref('sync');

    // 当月汇率映射（key为 `year-month`，value 为 USD->CNY 汇率），与云账单管理头部当月汇率同源
    const exchangeRateMap = ref<Record<string, string>>(null);
    const ensureExchangeRateMap = async () => {
      if (exchangeRateMap.value) return exchangeRateMap.value;
      const res = await reqBillsExchangeRateList({
        filter: {
          op: QueryRuleOPEnum.AND,
          rules: [
            { field: 'from_currency', op: QueryRuleOPEnum.EQ, value: 'USD' },
            { field: 'to_currency', op: QueryRuleOPEnum.EQ, value: 'CNY' },
          ],
        },
        page: { start: 0, limit: 500, count: false },
      });
      const map: Record<string, string> = {};
      (res.data?.details || []).forEach((item: any) => {
        map[`${item.year}-${item.month}`] = item.exchange_rate;
      });
      exchangeRateMap.value = map;
      return map;
    };

    // 为每行注入"人民币+美金"：美金按各行账单月份的汇率转人民币后，再加上人民币金额
    const resolveDataListCb = async (dataList: any[]) => {
      const rateMap = await ensureExchangeRateMap();
      return dataList.map((row) => {
        const rate = Number(rateMap[`${row.bill_year}-${row.bill_month}`] || 0);
        const combined = Number(row.cost || 0) * rate + Number(row.rmb_cost || 0);
        return { ...row, rmb_usd_combined: combined };
      });
    };

    const { CommonTable } = useTable({
      searchOptions: { disabled: true },
      tableOptions: {
        columns,
      },
      requestOption: { apiMethod: reqBillsSyncRecordList, resolveDataListCb },
    });

    return () => (
      <>
        <section class={cssModule.back} onClick={() => router.back()}>
          <ArrowsLeft class={cssModule['back-icon']} />
          <span class={cssModule['back-text']}>{t('返回上一级')}</span>
        </section>
        <Panel class={cssModule.table}>
          <CommonTable>
            {{
              operation: () => (
                <>
                  <span class={cssModule.title}>{t('操作记录')}</span>
                  <BkRadioGroup v-model={activeActionType.value} class={cssModule['action-type']}>
                    {actionTypes.map(({ label, text, disabled }) => (
                      <BkRadioButton
                        label={label}
                        disabled={disabled}
                        v-bk-tooltips={{ content: t('该功能暂未支持'), disabled: !disabled }}>
                        {text}
                      </BkRadioButton>
                    ))}
                  </BkRadioGroup>
                </>
              ),
            }}
          </CommonTable>
        </Panel>
      </>
    );
  },
});
