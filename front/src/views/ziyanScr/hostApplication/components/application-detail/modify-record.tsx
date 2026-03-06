import { defineComponent, ref, watch } from 'vue';
import { Sideslider, Table } from 'bkui-vue';
import { useFieldVal } from '@/views/ziyanScr/cvm-produce/component/property-display/field-map';
import { timeFormatter } from '@/common/util';
import http from '@/http';
import { useWhereAmI } from '@/hooks/useWhereAmI';

const { BK_HCM_AJAX_URL_PREFIX } = window.PROJECT_CONFIG;

export default defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '变更记录',
    },
    showObj: {
      type: Object,
      default: () => ({}),
    },
  },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    // 需要跟后端、产品确定哪些字段不展示，zone被zones替代
    const FIELD_BLACK_LIST = ['replicas', 'disk_size', 'disk_type', 'zone'];

    const { getBusinessApiPath, isBusinessPage } = useWhereAmI();
    const { getFieldCn, getFieldCnVal } = useFieldVal();
    const isDisplay = ref(false);
    watch(
      () => props.modelValue,
      (val) => {
        isDisplay.value = val;
        if (val) {
          fetchRecord();
        }
      },
      {
        immediate: true,
      },
    );
    const updateShowValue = () => {
      emit('update:modelValue', false);
    };
    const recordList = ref([]);

    const statusNames = { 0: '待审批', 1: '已调整需求', 2: '审批失败', 3: '审批拒绝', 4: '审批超时/作废' };
    const recordColumns = [
      {
        type: 'expand',
      },
      {
        label: 'ID',
        field: 'id',
      },
      {
        label: '操作人',
        field: 'bk_username',
      },
      {
        label: '操作时间',
        field: 'created_at',
        render: ({ row }: any) => {
          return <span>{timeFormatter(row.created_at)}</span>;
        },
      },
    ];

    if (!isBusinessPage) {
      recordColumns.splice(3, 0, {
        label: '状态',
        field: 'status',
        render: ({ row }: { row: { status: keyof typeof statusNames } }) => <>{statusNames[row.status]}</>,
      });
    }

    const expandRowTable = [
      {
        label: '修改属性',
        field: 'key',
        align: 'left',
        render: ({ row }: any) => {
          return <span>{getFieldCn(row.key)}</span>;
        },
      },
      {
        label: '变更前',
        field: 'prev',
        align: 'left',
        render: ({ row }: any) => {
          return <span>{getFieldCnVal(row.key, row.prev, row.prevRow)}</span>;
        },
      },
      {
        label: '变更后',
        field: 'cur',
        align: 'left',
        render: ({ row }: any) => {
          return <span>{getFieldCnVal(row.key, row.cur, row.curRow)}</span>;
        },
      },
    ];
    const handleDetail = (details: any) => {
      return Object.keys(details?.cur_data).reduce((prev, cur) => {
        if (!FIELD_BLACK_LIST.includes(cur)) {
          const obj = {
            key: cur,
            prev: details.pre_data[cur],
            cur: details.cur_data[cur],
            hasChange: details.pre_data[cur] !== details.cur_data[cur],
            prevRow: details.pre_data,
            curRow: details.cur_data,
          };
          prev.push(obj);
        }

        return prev;
      }, []);
    };
    const fetchRecord = async () => {
      const res = await http.post(
        `${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/find/apply/record/modify`,
        {
          suborder_id: [props.showObj.suborderId],
          status: isBusinessPage ? [1] : undefined,
          page: { start: 0, limit: 500, count: false, sort: 'created_at', order: 'DESC' },
        },
      );
      const list = res.data?.info || [];
      list.forEach((item: any) => {
        item.detailList = handleDetail(item.details);
      });
      recordList.value = list;
    };

    return () => (
      <Sideslider
        v-bind={attrs}
        width='1000'
        v-model:isShow={isDisplay.value}
        title={props.title}
        before-close={updateShowValue}>
        {{
          default: () => (
            <div class='sideslider-content'>
              <div class='des-top'>
                <div>
                  <label>所属业务</label>
                  <span>{props.showObj.bkBizId}</span>
                </div>
                <div>
                  <label>变更对象</label>
                  <span>资源申请子单</span>
                </div>
                <div>
                  <label>变更对象ID</label>
                  <span>{props.showObj.suborderId}</span>
                </div>
              </div>
              <Table data={recordList.value} columns={recordColumns}>
                {{
                  expandRow: (row: any) => (
                    <div class='record-expand'>
                      <Table data={row.detailList} columns={expandRowTable}></Table>
                    </div>
                  ),
                }}
              </Table>
            </div>
          ),
        }}
      </Sideslider>
    );
  },
});
