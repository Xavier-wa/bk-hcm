import type { ExportColumn } from '@/utils/common';

/**
 * 主机利用率扩展字段导出列定义
 * 勾选"附带主机利用率"时追加到 Excel 导出中
 */
export const EXTENSION_EXPORT_COLUMNS: ExportColumn[] = [
  { label: '公网IP', field: 'extension.outer_ip' },
  { label: 'SCM设备类型', field: 'extension.device_type' },
  { label: '裁撤模块名称', field: 'extension.module_name' },
  { label: '存放机房管理单元', field: 'extension.idc_unit_name' },
  { label: '操作系统', field: 'extension.sfw_name_version' },
  { label: '上架时间', field: 'extension.go_up_date' },
  { label: 'RAID结构', field: 'extension.raid_name' },
  { label: '逻辑区域', field: 'extension.logic_area' },
  { label: '设备技术分类', field: 'extension.device_layer' },
  { label: 'CPU得分', field: 'extension.cpu_score' },
  { label: '内存得分', field: 'extension.mem_score' },
  { label: '内网流量得分', field: 'extension.inner_net_traffic_score' },
  { label: '磁盘IO得分', field: 'extension.disk_io_score' },
  { label: '磁盘IO使用率得分', field: 'extension.disk_util_score' },
  { label: '是否达标', field: 'extension.is_pass' },
  { label: '内存使用量(G)', field: 'extension.mem4linux' },
  { label: '内网流量(Mb/s)', field: 'extension.inner_net_traffic' },
  { label: '外网流量(Mb/s)', field: 'extension.outer_net_traffic' },
  { label: '磁盘IO(Blocks/s)', field: 'extension.disk_io' },
  { label: '磁盘IO使用率', field: 'extension.disk_util' },
  { label: '磁盘总量(G)', field: 'extension.disk_total' },
  { label: '运维小组', field: 'extension.group_name' },
  { label: '业务中心', field: 'extension.center' },
];
