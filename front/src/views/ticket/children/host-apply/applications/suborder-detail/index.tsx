import { defineComponent, PropType, ref, watch } from 'vue';

import { Message, Table } from 'bkui-vue';
import { BkRadioButton, BkRadioGroup } from 'bkui-vue/lib/radio';
import CommonDialog from '@/components/common-dialog';
import CrpTicketAudit from './crp-ticket-audit.vue';
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';

import { useZiyanScrStore } from '@/store';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import usePagination from '@/hooks/usePagination';
import useTimeoutPoll from '@/hooks/use-timeout-poll';

export interface SubOrderInfo {
  step_name: string;
  step_id: number;
  suborder_id: number;
}

export default defineComponent({
  props: {
    subOrderInfo: { type: Object as PropType<SubOrderInfo>, required: true },
  },
  setup(props, { expose }) {
    const DETAIL_STATUS = [
      { label: undefined, name: '全部' },
      { label: 0, name: '成功' },
      { label: 2, name: '失败' },
      { label: 1, name: '执行中' },
    ];
    const scrStore = useZiyanScrStore();

    const isDialogShow = ref(false);
    const list = ref([]);
    const isLoading = ref(false);
    const curStatus = ref();

    const { columns: producingColumns } = useColumns('scrProduction');
    const { columns: initialColumns } = useColumns('scrInitial');
    const { columns: deliveryColumns } = useColumns('scrDelivery');

    const fetchApi = ref<Function>();
    const sortConfig = ref({ sort: 'created_at', order: 'DESC' });
    const tableColumns = ref([]);
    const { pagination, handlePageLimitChange, handlePageValueChange } = usePagination(() => getListData());

    const getListData = async () => {
      if (!fetchApi.value) return;
      isLoading.value = true;
      try {
        const [detailsRes, countRes] = await Promise.all([
          fetchApi.value(
            props.subOrderInfo.suborder_id,
            { limit: pagination.limit, start: pagination.start, count: false, ...sortConfig.value },
            curStatus.value,
          ),
          fetchApi.value(props.subOrderInfo.suborder_id, { limit: 0, start: 0, count: true }, curStatus.value),
        ]);
        list.value = detailsRes.data.info;
        pagination.count = countRes.data?.count;
      } catch (error) {
        list.value = [];
        pagination.count = 0;
      } finally {
        isLoading.value = false;
      }
    };

    const suborderDetailPolling = useTimeoutPoll(
      () => {
        getListData();
      },
      30000,
      { max: 60 },
    );

    watch(
      () => [props.subOrderInfo.suborder_id, props.subOrderInfo.step_id, curStatus.value, isDialogShow],
      () => {
        if (isDialogShow.value) {
          switch (props.subOrderInfo.step_id) {
            case 2: {
              fetchApi.value = scrStore.getProductionDetails;
              sortConfig.value = { sort: 'generate_id', order: 'ASC' };
              tableColumns.value = [{ type: 'expand', minWidth: 50 }, ...producingColumns];
              break;
            }
            case 3: {
              fetchApi.value = scrStore.getInitializationDetails;
              sortConfig.value = { sort: 'ip', order: 'ASC' };
              tableColumns.value = initialColumns;
              break;
            }
            case 4: {
              fetchApi.value = scrStore.getDeliveryDetails;
              sortConfig.value = { sort: 'ip', order: 'ASC' };
              tableColumns.value = deliveryColumns;
              break;
            }
          }

          pagination.start = 0;
          getListData();
        }
      },
      { immediate: true, deep: true },
    );

    watch(
      isDialogShow,
      (isShow) => {
        if (isShow) {
          suborderDetailPolling.resume();
        } else {
          suborderDetailPolling.reset();
        }
      },
      { immediate: true },
    );

    const triggerShow = (v: boolean) => {
      isDialogShow.value = v;
    };

    const handleRowExpand = async ({ row }: any) => {
      row.isExpand = row.isExpand !== undefined ? !row.isExpand : true;
    };

    const isCancelApplyCrpTicketLoading = ref(false);
    const cancelCrpTicket = async () => {
      isCancelApplyCrpTicketLoading.value = true;
      try {
        await scrStore.cancelApplyCrpTicket({ suborder_id: String(props.subOrderInfo.suborder_id) });
        Message({ theme: 'success', message: '撤单成功' });
        getListData();
      } catch (error) {
        console.error(error);
      } finally {
        isCancelApplyCrpTicketLoading.value = false;
      }
    };

    expose({ triggerShow });

    return () => (
      <CommonDialog
        v-model:isShow={isDialogShow.value}
        title={`资源${props.subOrderInfo.step_name}详情`}
        width={1200}
        dialogType='show'>
        <div class='flex-row align-items-center'>
          <BkRadioGroup v-model={curStatus.value}>
            {DETAIL_STATUS.map(({ label, name }) => (
              <BkRadioButton label={label}>{name}</BkRadioButton>
            ))}
          </BkRadioGroup>
          {/* todo: 暂时不限制按钮功能 */}
          {props.subOrderInfo.step_id === 2 && (
            <bk-pop-confirm
              title='撤销单据'
              content='撤销单据后，将取消本次的资源申请！'
              trigger='click'
              placement='top-end'
              onConfirm={cancelCrpTicket}>
              <bk-button style='margin-left: auto' theme='primary' loading={isCancelApplyCrpTicketLoading.value}>
                撤单
              </bk-button>
            </bk-pop-confirm>
          )}
          {props.subOrderInfo.step_id === 3 && (
            <CopyToClipboard
              style='margin-left: auto'
              content={() =>
                scrStore.getInitializationDetailsIps(String(props.subOrderInfo.suborder_id), curStatus.value)
              }
              disabled={!pagination.count}>
              {{
                default: ({ disabled, loading }: { disabled: boolean; loading: boolean }) => (
                  <bk-button theme='primary' disabled={disabled} loading={loading}>
                    复制全部IP
                  </bk-button>
                ),
              }}
            </CopyToClipboard>
          )}
        </div>
        <bk-loading loading={isLoading.value}>
          <Table
            style={{ maxHeight: '600px', marginTop: '16px' }}
            data={list.value}
            remotePagination
            pagination={pagination}
            columns={tableColumns.value}
            showOverflowTooltip
            onPageLimitChange={handlePageLimitChange}
            onPageValueChange={handlePageValueChange}
            onRowExpand={handleRowExpand}>
            {{
              expandRow: (row: any) => {
                return <CrpTicketAudit crpTicketId={row.task_id} subOrderId={String(props.subOrderInfo.suborder_id)} />;
              },
            }}
          </Table>
        </bk-loading>
      </CommonDialog>
    );
  },
});
