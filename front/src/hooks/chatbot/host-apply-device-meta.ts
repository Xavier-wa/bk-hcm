import { VendorEnum } from '@/common/constant';
import { QueryRuleOPEnum } from '@/typings';
import { useCvmDeviceStore } from '@/store/cvm/device';

// 主机申领 chatbot：按 device_type 查询机型族/CPU/内存，会话级缓存，供卡片与表格 enrichment。

export interface HostApplyDeviceMeta {
  device_family?: string;
  cpu_core?: number;
  memory?: number;
}

// null = 已请求但无结果/失败，避免重复打接口
const metaCache = new Map<string, HostApplyDeviceMeta | null>();
const inflight = new Map<string, Promise<HostApplyDeviceMeta | null>>();

/** 格式：`S5.LARGE8 (标准型, 4核8G)`；无 meta 或片段全缺时回退编码 */
export const formatDeviceTypeDisplay = (deviceType: string, meta?: HostApplyDeviceMeta | null): string => {
  if (!deviceType) return '';
  if (!meta) return deviceType;

  const inner: string[] = [];
  if (meta.device_family) inner.push(meta.device_family);

  const specParts: string[] = [];
  if (meta.cpu_core !== undefined && meta.cpu_core !== null) specParts.push(`${meta.cpu_core}核`);
  if (meta.memory !== undefined && meta.memory !== null) specParts.push(`${meta.memory}G`);
  if (specParts.length) inner.push(specParts.join(''));

  return inner.length ? `${deviceType} (${inner.join(', ')})` : deviceType;
};

/** 列表接口已拿到的条目可写入缓存，供后续卡片复用 */
export const seedDeviceMetaCache = (deviceType: string, meta: HostApplyDeviceMeta | null) => {
  if (!deviceType) return;
  metaCache.set(deviceType, meta);
};

const fetchOne = async (deviceType: string): Promise<HostApplyDeviceMeta | null> => {
  try {
    const { getOneDevicetype } = useCvmDeviceStore();
    const item = await getOneDevicetype({
      filter: {
        op: 'and',
        rules: [
          { field: 'vendor', op: QueryRuleOPEnum.EQ, value: VendorEnum.ZIYAN },
          { field: 'device_type', op: QueryRuleOPEnum.EQ, value: deviceType },
        ],
      },
    });
    if (!item) return null;
    return {
      device_family: item.device_family,
      cpu_core: item.cpu_core,
      memory: item.memory,
    };
  } catch {
    return null;
  }
};

export const ensureDeviceMeta = async (deviceType: string): Promise<HostApplyDeviceMeta | null> => {
  if (!deviceType) return null;
  if (metaCache.has(deviceType)) return metaCache.get(deviceType) ?? null;

  const pending = inflight.get(deviceType);
  if (pending) return pending;

  const task = fetchOne(deviceType).then((meta) => {
    metaCache.set(deviceType, meta);
    inflight.delete(deviceType);
    return meta;
  });
  inflight.set(deviceType, task);
  return task;
};

export const ensureDeviceMetaMap = async (
  deviceTypes: string[],
): Promise<Record<string, HostApplyDeviceMeta | null>> => {
  const unique = [...new Set(deviceTypes.filter(Boolean))];
  const entries = await Promise.all(unique.map(async (dt) => [dt, await ensureDeviceMeta(dt)] as const));
  return Object.fromEntries(entries);
};
