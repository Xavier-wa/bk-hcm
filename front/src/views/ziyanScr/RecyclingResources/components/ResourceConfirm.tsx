import { defineComponent, onMounted, ref, watch } from 'vue';
import { Loading, Table } from 'bkui-vue';
import './index.scss';
import apiService from '../../../../api/scrApi';
import useSelection from '@/views/resource/resource-manage/hooks/use-selection';
export default defineComponent({
  name: 'ResourceConfirm',
  props: {
    recycleForm: {
      type: Object,
      default: () => ({}),
    },
    pagination: {
      type: Object,
      default: () => ({}),
    },
    returnPlan: {
      type: Object,
      default: () => ({
        cvm: 'IMMEDIATE',
        pm: 'IMMEDIATE',
        skipConfirm: '',
      }),
    },
    bizs: String,
  },
  emits: ['updateConfirm', 'Tablehosts'],
  setup(props, { emit }) {
    const { selections, handleSelectionChange } = useSelection();
    const hostList = ref([]);
    const loading = ref(false);
    const tableList = ref([]);
    const suborderId = ref();
    const bkBizId = ref();
    const drawer = ref(false);
    const drawerTitle = ref('');
    const getReturnPlanName = (returnPlan: string, resourceType: string) => {
      if (returnPlan === 'IMMEDIATE') {
        return resourceType === 'IDCPM' ? '立即销毁(隔离2小时)' : '立即销毁';
      }
      if (returnPlan === 'DELAY') {
        let label = '延迟销毁';
        if (resourceType === 'IDCPM') {
          label += '(隔离1天)';
        } else if (resourceType === 'QCLOUDCVM') {
          label += '(隔离15天)';
        }
        return label;
      }
      return '';
    };
    const getResourceTypeName = (resourceType: string | number) => {
      const resourceTypeMap = {
        QCLOUDCVM: '腾讯云虚拟机',
        IDCPM: 'IDC物理机',
        OTHERS: '其他',
      };
      return resourceTypeMap[resourceType];
    };
    watch(
      () => props.pagination.limit,
      () => {
        getList(false);
      },
    );
    watch(
      () => props.pagination.start,
      () => {
        getList(false);
      },
    );
    watch(
      () => selections.value.length,
      () => {
        emit('updateConfirm', selections.value, drawerTitle.value, false);
      },
    );
    const columns = ref([
      { type: 'selection', width: 30, minWidth: 30, isDefaultShow: true },
      {
        label: '业务',
        field: 'bk_biz_name',
      },
      {
        label: '资源类型',
        field: 'resource_type',
        render: ({ cell }) => {
          return <span>{getResourceTypeName(cell)}</span>;
        },
      },
      {
        label: '回收类型',
        field: 'recycle_type',
      },
      {
        label: '回收选项',
        render: ({ data }) => {
          return <span>{getReturnPlanName(data.return_plan, data.resource_type)}</span>;
        },
      },
      {
        label: '资源总数',
        field: 'total_num',
        render: ({ data, cell }) => {
          return (
            <>
              <span>{cell}</span>&nbsp;&nbsp;&nbsp;&nbsp;
              <bk-button type='text' size='small' onClick={() => handleRowClick(data)}>
                详情
              </bk-button>
            </>
          );
        },
      },
      {
        label: '回收成本',
        field: 'cost_concerned',
        render: ({ cell }) => {
          return (
            <>
              <span>{cell ? '涉及' : '不涉及'}</span>
            </>
          );
        },
      },
      {
        label: '备注',
        field: 'remark',
        render: ({ cell }) => {
          return (
            <>
              <span>{cell ? cell : '--'}</span>
            </>
          );
        },
      },
    ]);
    const handleRowClick = (row: { bk_biz_name: string; suborder_id: any; bk_biz_id: any }) => {
      drawer.value = true;
      drawerTitle.value = row.bk_biz_name;
      suborderId.value = [row.suborder_id];
      bkBizId.value = [row.bk_biz_id];
      emit('updateConfirm', selections.value, drawerTitle.value, drawer.value);
      getList(false);
      getList(true);
    };
    /** 获取资源回收单据预览列表 */
    const getPreRecycleList = async () => {
      loading.value = true;
      emit('updateConfirm', [], '', drawer.value);

      const {
        returnPlan,
        recycleForm: { ips, remark },
      } = props;
      const { info } = await apiService.getPreRecycleList({ ips, remark, returnPlan });
      tableList.value = info || [];
      loading.value = false;
    };
    const getList = async (enableCount = false) => {
      const page = {
        limit: enableCount ? undefined : props.pagination.limit,
        start: enableCount ? undefined : props.pagination.start,
        enable_count: enableCount,
      };
      const data = await apiService.getRecycleHosts({
        suborder_id: suborderId.value,
        page,
        bk_biz_id: bkBizId.value,
      });
      const obj = props.pagination;
      if (enableCount) obj.count = data?.count;
      else {
        hostList.value = data?.info || [];
      }
      emit('Tablehosts', hostList.value, obj);
    };
    onMounted(() => {
      getPreRecycleList();
    });
    return () => (
      <div class='div-ResourceSelect'>
        <Loading class='loading-container' loading={loading.value}>
          <Table
            align='left'
            row-hover='auto'
            data={tableList.value}
            rowKey='id'
            columns={columns.value}
            remotePagination
            showOverflowTooltip
            {...{
              onSelectionChange: (selections: any) => handleSelectionChange(selections, () => true),
              onSelectAll: (selections: any) => handleSelectionChange(selections, () => true, true),
            }}></Table>
        </Loading>
      </div>
    );
  },
});
