import { CLB_SPECS } from '@/common/constant';
import { QueryRuleOPEnum, RulesItem } from '@/typings';
import { LOAD_BALANCER_INSTANCE_SPEC_NAME, LoadBalancerInstanceSpec } from './constants';

type InstanceSpecSource = { exclusive?: number; sla_type?: string };

/**
 * 实例规格展示值
 * 云上把规格拆成 exclusive(是否独占型) 与 sla_type(性能容量型档位) 两个正交字段，后端只返回原始值，由前端合成一列
 */
export const getLoadBalancerInstanceSpecName = (data: InstanceSpecSource) => {
  if (data?.exclusive === 1) return LOAD_BALANCER_INSTANCE_SPEC_NAME[LoadBalancerInstanceSpec.EXCLUSIVE];
  if (data?.sla_type) return CLB_SPECS[data.sla_type] ?? data.sla_type;
  if (data?.exclusive === 0) return LOAD_BALANCER_INSTANCE_SPEC_NAME[LoadBalancerInstanceSpec.SHARED];
  return '--';
};

// 共享型需要排除性能容量型，因此是两条规则
const buildInstanceSpecRule = (spec: string): RulesItem =>
  spec === LoadBalancerInstanceSpec.EXCLUSIVE
    ? { field: 'extension.exclusive', op: QueryRuleOPEnum.JSON_EQ, value: 1 }
    : {
        op: QueryRuleOPEnum.AND,
        rules: [
          { field: 'extension.exclusive', op: QueryRuleOPEnum.JSON_EQ, value: 0 },
          { field: 'extension.sla_type', op: QueryRuleOPEnum.JSON_EQ, value: '' },
        ],
      };

/**
 * 实例规格查询条件
 * 展示值来自 exclusive / sla_type，筛选则统一走 extension 内字段的 json_eq
 */
export const buildLoadBalancerInstanceSpecFilterRules = (value: string | string[]): RulesItem => {
  const specs = (Array.isArray(value) ? value : [value]).filter(Boolean);
  if (specs.length === 1) return buildInstanceSpecRule(specs[0]);
  return { op: QueryRuleOPEnum.OR, rules: specs.map(buildInstanceSpecRule) };
};
