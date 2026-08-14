import { watch } from 'vue';
import { useRoute } from 'vue-router';

import { useCvmDeviceStore } from '@/store/cvm/device';
import { useHostApplyBackfillStore } from '@/store/chatbot/host-apply-backfill';
import { type HostApplySuborder } from '@/hooks/chatbot/types';
import { VendorEnum } from '@/common/constant';
import { QueryRuleOPEnum } from '@/typings';

// 与 cloudResourceForm 输出对齐的云主机配置行（chatbot 回填用）
export interface ChatbotCloudRow {
  resource_type: string;
  remark: string;
  enable_disk_check: boolean;
  anti_affinity_level: string;
  replicas: number;
  source: string;
  spec: {
    device_type: string;
    vpc: string;
    subnet: string;
    replicas: number;
    anti_affinity_level: string;
    image_id: string;
    system_disk: NonNullable<HostApplySuborder['system_disk']>;
    data_disk: NonNullable<HostApplySuborder['data_disk']>;
    network_type: string;
    bk_asset_id: string;
    inherit_instance_id: string;
    cpu: number | undefined;
    res_assign: HostApplySuborder['res_assign'];
    cpu_thread_switch: string | undefined;
    region: string;
    zone: string;
    zones: string[];
    charge_type: HostApplySuborder['charge_type'];
    charge_months: number;
  };
}

interface UseChatbotBackfillOptions {
  // 追加一条云主机配置行到配置清单
  appendCloudRow: (row: ChatbotCloudRow) => void;
  // 回填订单级需求类型
  setRequireType: (requireType: number) => void;
}

// 主机申领页消费 chatbot「添加到配置清单」的回填逻辑：
//   - 全页新标签：规格经 URL query backfill 透传，调用 applyQueryBackfill 解析回填；
//   - 浮窗同页：规格经 host-apply-backfill store 写入，内部 watch 到后追加。
// 从 application-form 主表单抽出，避免该文件继续膨胀。
export function useChatbotBackfill(options: UseChatbotBackfillOptions) {
  const route = useRoute();
  const cvmDeviceStore = useCvmDeviceStore();
  const hostApplyBackfillStore = useHostApplyBackfillStore();

  // 把单条 suborder 规格映射为云主机配置行（对齐 cloudResourceForm 的结构），resource_type 缺省按 QCLOUDCVM 处理
  const buildCloudRowFromChatbot = (sub: HostApplySuborder & { resource_type?: string }): ChatbotCloudRow => {
    const replicas = Number(sub.replicas) || 1;
    return {
      resource_type: sub.resource_type || 'QCLOUDCVM',
      remark: '',
      enable_disk_check: false,
      anti_affinity_level: 'ANTI_NONE',
      replicas,
      source: 'business',
      spec: {
        device_type: sub.device_type ?? '',
        vpc: '',
        subnet: '',
        replicas,
        anti_affinity_level: 'ANTI_NONE',
        image_id: sub.image_id ?? '',
        system_disk: sub.system_disk ?? { disk_type: '', disk_size: 0, disk_num: 1 },
        data_disk: Array.isArray(sub.data_disk) ? sub.data_disk : [],
        network_type: 'TENTHOUSAND',
        bk_asset_id: '',
        inherit_instance_id: '',
        cpu: undefined,
        res_assign: sub.res_assign,
        cpu_thread_switch: undefined,
        region: sub.region ?? '',
        zone: sub.zone ?? '',
        zones: sub.zone ? [sub.zone] : [],
        charge_type: sub.charge_type,
        charge_months: 36,
      },
    };
  };

  // chatbot 不下发每实例 CPU 核数（spec.cpu），而「需求核数」依赖它计算，缺失会导致 NaN。
  // 按机型逐个查机型目录 getOneDevicetype 取 cpu_core，返回 device_type → cpu_core 映射。
  const fetchCpuCoreMap = async (deviceTypes: string[]): Promise<Record<string, number>> => {
    if (!deviceTypes.length) return {};
    const entries = await Promise.all(
      deviceTypes.map(async (deviceType) => {
        const item = await cvmDeviceStore.getOneDevicetype({
          filter: {
            op: 'and',
            rules: [
              { field: 'vendor', op: QueryRuleOPEnum.EQ, value: VendorEnum.ZIYAN },
              { field: 'device_type', op: QueryRuleOPEnum.EQ, value: deviceType },
            ],
          },
        });
        return [deviceType, item?.cpu_core] as const;
      }),
    );
    return Object.fromEntries(entries.filter(([, cpuCore]) => cpuCore !== undefined)) as Record<string, number>;
  };

  // 消费 chatbot 暂存的 suborder 列表，追加到配置清单（chatbot 当前仅产出 ZIYAN QCLOUDCVM）。
  // 需求类型为订单级共享字段，取首条带值的 suborder 回填。
  const backfillFromChatbot = async (suborders: HostApplySuborder[]) => {
    const requireType = suborders.find(
      (item) => item.require_type !== undefined && item.require_type !== null,
    )?.require_type;
    if (requireType !== undefined) {
      options.setRequireType(Number(requireType));
    }
    const deviceTypes = [...new Set(suborders.map((sub) => sub.device_type).filter(Boolean) as string[])];
    const cpuCoreMap = await fetchCpuCoreMap(deviceTypes);
    suborders.forEach((sub) => {
      const row = buildCloudRowFromChatbot(sub);
      if (sub.device_type && cpuCoreMap[sub.device_type] !== undefined) {
        row.spec.cpu = cpuCoreMap[sub.device_type];
      }
      options.appendCloudRow(row);
    });
  };

  // 解析 chatbot 透传的 backfill query（新标签页打开，规格经 URL 序列化承载），解析失败返回空
  const parseChatbotBackfill = (): HostApplySuborder[] => {
    const raw = route?.query?.backfill;
    if (typeof raw !== 'string' || !raw) return [];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed : [];
    } catch (err) {
      console.error('parse chatbot backfill failed', err);
      return [];
    }
  };

  // 全页新标签 query 回填：命中返回 true（调用方据此短路其它来源分支）
  const applyQueryBackfill = async (): Promise<boolean> => {
    const suborders = parseChatbotBackfill();
    if (!suborders.length) return false;
    await backfillFromChatbot(suborders);
    return true;
  };

  // 浮窗同页「添加到配置清单」：store 写入后追加到配置清单（与新标签页 query 路径区分）
  watch(
    () => hostApplyBackfillStore.pending,
    async (suborders) => {
      if (!suborders.length) return;
      await backfillFromChatbot(hostApplyBackfillStore.consume());
    },
  );

  return { applyQueryBackfill };
}
