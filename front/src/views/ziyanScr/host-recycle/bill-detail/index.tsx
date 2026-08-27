import { defineComponent, ref, computed, onMounted, onUnmounted } from 'vue';
import { useUserStore } from '@/store';
import DetailHeader from '@/views/resource/resource-manage/common/header/detail-header';
import { getResourceTypeName, getReturnPlanName, exportTableToExcel, getEntirePath } from '@/utils';
import { getRecycleTaskStatusLabel } from '@/views/ziyanScr/host-recycle/field-dictionary/recycleStatus';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import useSelection from '@/views/resource/resource-manage/hooks/use-selection';
import { useTable } from '@/hooks/useTable/useTable';
import { Loading, ExclamationCircleShape } from 'bkui-vue/lib/icon';
import ExecuteRecord from '../execute-record';
import { useRoute } from 'vue-router';
import './index.scss';
import { Message } from 'bkui-vue';
import http from '@/http';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import { timeFormatter } from '@/common/util';
import { PAGE_BIZ_KEY } from '@/common/constant';

export default defineComponent({
  components: {
    ExecuteRecord,
  },
  setup() {
    const route = useRoute();
    const { getBusinessApiPath, getBizsId } = useWhereAmI();
    const billBaseInfo = ref<any>({});
    const page = ref({
      start: 0,
      limit: 10,
      enable_count: false,
    });
    // 单据所属业务，是本页面唯一可信的业务身份，页面内所有接口都以它为准；
    // 存量入口未携带该参数时回退到全局业务，保持改动前的行为
    const orderBizId = computed(() => Number(route.query[PAGE_BIZ_KEY]) || getBizsId());
    const getOrderApiPath = () => getBusinessApiPath(orderBizId.value);
    const requestParams = computed(() => {
      return {
        bk_biz_id: [orderBizId.value],
        suborder_id: [route.query.suborderId],
        page: page.value,
      };
    });

    const loadOrdersTimer: any = { id: null, count: 0 };
    const loadOrders = async () => {
      try {
        const data = await http
          .post(getEntirePath(`${getOrderApiPath()}task/findmany/recycle/order`), requestParams.value)
          .then((res: any) => res.data);

        const orders = data?.info || [{}];
        const [order = {}] = orders;
        billBaseInfo.value = order;

        // 定时任务(30s刷新一次, 最多刷新60次)
        if (loadOrdersTimer.count < 60 && !['DONE', 'TERMINATE'].includes(order.stage)) {
          loadOrdersTimer.count += 1;
          loadOrdersTimer.id = setTimeout(() => {
            loadOrders();
          }, 30000);
        }
      } catch (error) {
        billBaseInfo.value = {};
      }
    };
    const textTip = (text: string, theme: string) => {
      const themeDes: any = {
        error: '失败',
        success: '成功',
      };
      Message({
        message: `${text}${themeDes[theme]}`,
        theme,
        duration: 1500,
      });
    };
    const fetchRetryOrder = async () => {
      const res = await http.post(getEntirePath(`${getOrderApiPath()}task/start/recycle/order`), {
        suborder_id: requestParams.value.suborder_id,
      });

      if (res.code === 0) {
        textTip('重试', 'success');
        loadOrders();
      }
    };
    const fetchStopOrder = async () => {
      const res = await http.post(getEntirePath(`${getOrderApiPath()}task/terminate/recycle/order`), {
        suborder_id: requestParams.value.suborder_id,
      });

      if (res.code === 0) {
        textTip('终止', 'success');
        loadOrders();
      }
    };
    const fetchSubmitOrder = async () => {
      const res = await http.post(getEntirePath(`${getOrderApiPath()}task/revise/recycle/order`), {
        suborder_id: requestParams.value.suborder_id,
      });

      if (res.code === 0) {
        textTip('去除预检失败IP提交', 'success');
        loadOrders();
      }
    };

    const activeStep = computed(() => {
      const list = ['DETECT', 'AUDIT', 'TRANSIT', 'RETURN', 'RETURN_PLAN', 'DONE'];
      return list.indexOf(billBaseInfo.value.stage) + 1;
    });

    const getStepTitle = (stepName: string, description: string) => {
      const style = {
        display: 'flex',
        alignItems: 'center',
      };
      return (
        <div style={style}>
          <span>{stepName}</span>
          <ExclamationCircleShape
            v-show={description}
            style={{ cursor: 'pointer', marginLeft: '3px' }}
            width={15}
            height={15}
            v-bk-tooltips={{
              content: description,
            }}
          />
        </div>
      );
    };
    const stepArr = computed(() => {
      const list = [
        { title: getStepTitle('预检', activeStep.value === 1 ? billBaseInfo.value.message : '') },
        {
          title: getStepTitle(
            'BG管理员审核',
            activeStep.value === 1 && billBaseInfo.value.status === 'REJECTED'
              ? `驳回原因：${billBaseInfo.value.message}`
              : '',
          ),
        },
        { title: getStepTitle('CR系统处理中', activeStep.value === 3 ? billBaseInfo.value.message : '') },
        { title: getStepTitle('公司资源系统处理中', activeStep.value === 4 ? billBaseInfo.value.message : '') },
        {
          title: getStepTitle('预测数据返还', activeStep.value === 5 ? billBaseInfo.value.message : ''),
        },
        { title: '完成' },
      ];
      return list;
    });
    const billStatus = computed(() => {
      const { status = '' } = billBaseInfo.value;
      if (status === 'DONE') {
        return 'success';
      }
      const isError = status.includes('FAILED') || status.includes('REJECTED') || status.includes('TERMINATE');
      return isError ? 'error' : 'process';
    });

    const remark = ref('');
    const admins = ref(['dommyzhang', 'forestchen']);
    const fetchAuditOrder = async (approval: boolean) => {
      const res = await http.post(getEntirePath(`${getOrderApiPath()}task/audit/recycle/order`), {
        suborder_id: requestParams.value.suborder_id,
        approval,
        remark,
      });

      if (res.code === 0) {
        loadOrders();
      }
    };

    const { selections, handleSelectionChange } = useSelection();
    const { columns } = useColumns('deviceDestroy');
    const operateColList = [
      {
        label: '查看',
        width: 80,
        render: ({ row }: any) => {
          return (
            <div>
              <bk-button size='small' theme='primary' text onClick={() => application(row)}>
                详情
              </bk-button>
            </div>
          );
        },
      },
    ];
    const tableColumns = [...columns, ...operateColList];
    tableColumns.splice(2, 0, {
      label: '内网IP',
      field: 'ip',
      render: ({ row }: { row: { ip: string } }) => {
        return (
          <bk-button text theme='primary' onClick={() => application(row)}>
            {row.ip}
          </bk-button>
        );
      },
    });
    const { CommonTable } = useTable({
      tableOptions: {
        columns: tableColumns,
        extra: {
          onSelect: (selections: any) => {
            handleSelectionChange(selections, () => true, false);
          },
          onSelectAll: (selections: any) => {
            handleSelectionChange(selections, () => true, true);
          },
        },
      },
      requestOption: {
        dataPath: 'data.info',
        sortOption: {
          sort: 'ip',
          order: 'ASC',
        },
      },
      scrConfig: () => {
        return {
          url: `/api/v1/woa/${getOrderApiPath()}task/findmany/recycle/host`,
          payload: {
            ...requestParams.value,
          },
        };
      },
    });
    const clipHostIp = computed(() => {
      return selections.value.map((item) => item.ip).join('\n');
    });
    const exportToExcel = async () => {
      const res = await http.post(getEntirePath(`${getOrderApiPath()}task/findmany/recycle/host`), {
        bk_biz_id: requestParams.value.bk_biz_id,
        suborder_id: requestParams.value.suborder_id,
        page: {
          start: 0,
          limit: 500,
          enable_count: false,
        },
      });

      const totalList = res.data?.info || [];
      exportTableToExcel(totalList, tableColumns, '设备销毁详情');
    };
    const preCheckDetail = ref(false);
    const transferData = ref<{ suborderId?: string; ip?: string; page?: { start: number; limit: number } }>({});
    const application = (row: any) => {
      preCheckDetail.value = true;
      transferData.value = {
        suborderId: row.suborder_id,
        ip: row.ip,
        page: {
          start: 0,
          limit: 10,
        },
      };
    };
    onMounted(() => {
      loadOrders();
    });

    onUnmounted(() => {
      clearTimeout(loadOrdersTimer.id);
    });

    const userStore = useUserStore();
    return () => (
      <>
        <div class={'application-detail-container'}>
          <DetailHeader>单据详情</DetailHeader>
          <div class={'detail-wrapper bill-detail'}>
            <div class='base-info'>
              <h2>基本信息</h2>
              <div class='base-info-top'>
                <div>
                  <label>主单号:</label>
                  <span>{billBaseInfo.value.order_id}</span>
                </div>
                <div>
                  <label>子单号:</label>
                  <span>{billBaseInfo.value.suborder_id}</span>
                </div>
                <div>
                  <label>业务:</label>
                  <span>{billBaseInfo.value.bk_biz_name}</span>
                </div>
                <div>
                  <label>资源类型:</label>
                  <span>{getResourceTypeName(billBaseInfo.value.resource_type)}</span>
                </div>
                <div>
                  <label>退回策略:</label>
                  <span>{getReturnPlanName(billBaseInfo.value.return_plan, billBaseInfo.value.resource_type)}</span>
                </div>
                <div>
                  <label>单据状态:</label>
                  <span
                    class={{ 'c-danger': billStatus.value === 'error', 'c-success': billStatus.value === 'success' }}>
                    {billStatus.value === 'process' ? <Loading /> : ''}
                    {getRecycleTaskStatusLabel(billBaseInfo.value.status)}
                  </span>
                </div>
                <div>
                  <label>设备总数/销毁完成数量:</label>
                  <span>
                    {billBaseInfo.value.total_num}/{billBaseInfo.value.success_num}
                  </span>
                </div>
                <div>
                  <label>处理人:</label>
                  <span>{billBaseInfo.value.handler}</span>
                </div>
                <div>
                  <label>回收人:</label>
                  <span>{billBaseInfo.value.bk_username}</span>
                </div>
                <div>
                  <label>提单时间:</label>
                  <span>{timeFormatter(billBaseInfo.value.create_at)}</span>
                </div>
              </div>
              <div class='base-info-bottom'>
                <bk-button onClick={fetchRetryOrder}>重试</bk-button>
                <bk-button onClick={fetchStopOrder}>终止</bk-button>
                <bk-button onClick={fetchSubmitOrder}>去除预检失败IP提交</bk-button>
              </div>
            </div>
            <div class='handle-process'>
              <h2>处理流程</h2>
              <bk-steps
                line-type='solid'
                size='small'
                status={billStatus.value}
                cur-step={activeStep.value}
                steps={stepArr.value}
              />
              {admins.value.includes(userStore.username) && billBaseInfo.value.status === 'FOR_AUDIT' ? (
                <div class='check-export'>
                  <div class='check-export-man'>
                    <span>当前审批步骤：BG管理员审核</span>
                    <span>审核人：</span>
                    {admins.value.map((item) => {
                      return (
                        <a href={`wxwork://message?username=${item}`} class='username'>
                          {item}
                        </a>
                      );
                    })}
                  </div>
                  <div>
                    <bk-input v-model={remark} type='textarea' rows={2} placeholder='请输入审核意见' />

                    <div class='check-export-trigger'>
                      <bk-button theme='primary' onClick={() => fetchAuditOrder(true)}>
                        审核通过
                      </bk-button>
                      <bk-button theme='danger' onClick={() => fetchAuditOrder(false)}>
                        驳回
                      </bk-button>
                    </div>
                  </div>
                </div>
              ) : null}
            </div>
            <div class='device-destroy'>
              <h2>设备销毁详情</h2>
              <div class='device-destroy-top'>
                <bk-button v-clipboard={clipHostIp.value} disabled={!selections.value.length}>
                  复制选中IP
                </bk-button>
                <bk-button onClick={exportToExcel}>导出全部</bk-button>
              </div>
              <CommonTable></CommonTable>
            </div>
          </div>
          {transferData.value?.suborderId && (
            <execute-record v-model={preCheckDetail.value} dataInfo={transferData.value} />
          )}
        </div>
      </>
    );
  },
});
