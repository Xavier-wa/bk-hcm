import { useZiyanScrStore } from '@/store';
import { Message, Table } from 'bkui-vue';
import { BkRadioButton, BkRadioGroup } from 'bkui-vue/lib/radio';
import { defineComponent, ref, watch } from 'vue';
import { DETAIL_STATUS } from './constants';
import './index.scss';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import usePagination from '@/hooks/usePagination';
import useTimeoutPoll from '@/hooks/use-timeout-poll';
import CrpTicketAudit from '@/views/ticket/children/host-apply/applications/suborder-detail/crp-ticket-audit.vue';
import CopyToClipboard from '@/components/copy-to-clipboard/index.vue';

export default defineComponent({
  props: {
    suborderId: {
      required: true,
      type: Number,
    },
    stepId: {
      required: true,
      type: Number,
    },
    isShow: {
      required: true,
      type: Boolean,
    },
  },
  setup(props) {
    const list = ref([]);
    const isLoading = ref(false);
    const scrStore = useZiyanScrStore();
    const curStatus = ref();
    const { columns: producingColumns } = useColumns('scrProduction');
    const { columns: initialColumns } = useColumns('scrInitial');
    const { columns: deliveryColumns } = useColumns('scrDelivery');

    const fetchData = ref<Function>();
    const sortConfig = ref({ sort: 'created_at', order: 'DESC' });
    const tableColumns = ref([]);
    const { pagination, handlePageLimitChange, handlePageValueChange } = usePagination(() => getListData());
    const getListData = async () => {
      isLoading.value = true;
      try {
        const [detailsRes, countRes] = await Promise.all([
          fetchData.value(
            props.suborderId,
            { limit: pagination.limit, start: pagination.start, count: false, ...sortConfig.value },
            curStatus.value,
          ),
          fetchData.value(props.suborderId, { limit: 0, start: 0, count: true }, curStatus.value),
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
      () => [props.suborderId, props.stepId, curStatus.value, props.isShow],
      () => {
        if (props.isShow) {
          switch (props.stepId) {
            case 2: {
              fetchData.value = scrStore.getProductionDetails;
              sortConfig.value = { sort: 'generate_id', order: 'ASC' };
              tableColumns.value = [{ type: 'expand', minWidth: 50 }, ...producingColumns];
              break;
            }
            case 3: {
              fetchData.value = scrStore.getInitializationDetails;
              sortConfig.value = { sort: 'ip', order: 'ASC' };
              tableColumns.value = initialColumns;
              break;
            }
            case 4: {
              fetchData.value = scrStore.getDeliveryDetails;
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
      () => props.isShow,
      (isShow) => {
        if (isShow) {
          suborderDetailPolling.resume();
        } else {
          suborderDetailPolling.reset();
        }
      },
      { immediate: true },
    );

    const handleRowExpand = async ({ row }: any) => {
      row.isExpand = row.isExpand !== undefined ? !row.isExpand : true;
    };

    const isCancelApplyCrpTicketLoading = ref(false);
    const cancelCrpTicket = async () => {
      isCancelApplyCrpTicketLoading.value = true;
      try {
        await scrStore.cancelApplyCrpTicket({ suborder_id: String(props.suborderId) });
        Message({ theme: 'success', message: '撤单成功' });
        getListData();
      } catch (error) {
        console.error(error);
      } finally {
        isCancelApplyCrpTicketLoading.value = false;
      }
    };

    return () => (
      <div style={'width: 100%;'}>
        <div class='flex-row align-items-center'>
          <BkRadioGroup v-model={curStatus.value}>
            {DETAIL_STATUS.map(({ label, name }) => (
              <BkRadioButton label={label}>{name}</BkRadioButton>
            ))}
          </BkRadioGroup>
          {/* todo: 暂时不限制按钮功能 */}
          {props.stepId === 2 && (
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
          {props.stepId === 3 && (
            <CopyToClipboard
              style='margin-left: auto'
              content={() => scrStore.getInitializationDetailsIps(String(props.suborderId), curStatus.value)}
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
            class={'maxheigth tablelist'}
            data={list.value}
            remotePagination
            pagination={pagination}
            columns={tableColumns.value}
            onPageLimitChange={handlePageLimitChange}
            onPageValueChange={handlePageValueChange}
            onRowExpand={handleRowExpand}>
            {{
              expandRow: (row: any) => {
                return <CrpTicketAudit crpTicketId={row.task_id} subOrderId={String(props.suborderId)} />;
              },
            }}
          </Table>
        </bk-loading>
      </div>
    );
  },
});
