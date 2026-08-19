import { h } from 'vue';
import type { ModelPropertyColumn } from '@/model/typings';
import { type IResourcePlanTicketItem, useResourcePlanTicketStore } from '@/store/ticket/resource-plan';
import { timeFormatter } from '@/common/util';

export const properties: ModelPropertyColumn[] = [
  {
    id: 'status',
    name: '单据状态',
    type: 'enum',
    option: async () => {
      const resourcePlanTicketStore = useResourcePlanTicketStore();
      const list = await resourcePlanTicketStore.getTicketStatusList();
      return list.reduce((acc, { status, status_name }) => {
        acc[status] = status_name;
        return acc;
      }, {} as Record<string, string>);
    },
    meta: {
      display: {
        appearance: 'dynamic-status',
        appearanceProps: {
          statusObject: {
            success: ['done'],
            fail: ['failed', 'partial_failed', 'rejected', 'partial_rejected'],
            ing: ['auditing'],
            stop: ['revoked', 'terminated'],
          },
        },
      },
    },
  },
  {
    id: 'ticket_type_name',
    name: '类型',
    type: 'string',
  },
  {
    id: 'bk_biz_name',
    name: '业务',
    type: 'string',
  },
  {
    id: 'op_product_name',
    name: '运营产品',
    type: 'string',
  },
  {
    id: 'plan_product_name',
    name: '规划产品',
    type: 'string',
  },
  {
    id: 'updated_info.cvm.cpu_core',
    name: 'CPU核数',
    type: 'string',
    align: 'right',
    render: ({ row }: { row?: IResourcePlanTicketItem }) => {
      const type = row.ticket_type;
      const value = row.updated_info.cvm.cpu_core - row.original_info.cvm.cpu_core;
      if (isNaN(value)) {
        return '--';
      }
      let prefix = value > 0 ? '+' : '';
      if (value === 0) {
        prefix = type === 'delete' ? '-' : '+';
      }
      return `${prefix}${value}`;
    },
  },
  {
    id: 'audited_updated_info.cvm.cpu_core',
    name: 'CPU核数(已审批数)',
    type: 'string',
    align: 'right',
    render: ({ row }: { row?: IResourcePlanTicketItem }) => {
      const { ticket_type: type, status } = row;

      // 类型为调整且状态为部分失败时，不展示已审批数
      if (type === 'adjust' && status === 'partial_failed') {
        return '--';
      }

      const value = row.audited_updated_info.cvm.cpu_core - row.audited_original_info.cvm.cpu_core;
      if (isNaN(value)) {
        return '--';
      }
      let prefix = value > 0 ? '+' : '';
      if (value === 0) {
        prefix = type === 'delete' ? '-' : '+';
      }
      // 新增、预算申报均为资源增量，展示为绿色正向；其余（调整/取消）为红色
      const isIncrement = type === 'add' || type === 'budget_declare';
      return h('span', { style: { color: isIncrement ? '#299e56' : '#ea3636' } }, `${prefix}${value}`);
    },
  },
  {
    id: 'applicant',
    name: '提单人',
    type: 'user',
  },
  {
    id: 'remark',
    name: '备注',
    type: 'string',
    defaultHidden: true,
  },
  {
    id: 'created_at',
    name: '创建时间',
    type: 'datetime',
    defaultHidden: true,
  },
  {
    id: 'submitted_at',
    name: '提单时间',
    type: 'datetime',
  },
  {
    id: 'updated_at',
    name: '完成时间',
    type: 'datetime',
    render: ({ row }: { row?: IResourcePlanTicketItem }) => {
      if (row.status === 'done') {
        return timeFormatter(row.updated_at);
      }
      return '--';
    },
  },
];
