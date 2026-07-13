# API：机房裁撤-功能优化重构-前端

> **状态说明**：基于 `api-doc/` 7 份真实后端接口文档整理，字段名和路径均为实际值。
> **脱敏说明**：本文档所有接口路径均为相对路径，不含真实域名/主机信息。

## 0. 接口清单总览

| # | 接口 | 方法 | 路径 | 用途 |
|---|------|------|------|------|
| 1 | 裁撤总览列表查询 | POST | `/api/v1/woa/dissolve/table/list` | 总览 Tab 表格数据 |
| 2 | 裁撤明细列表查询 | POST | `/api/v1/woa/dissolve/host/detail/list` | 明细 Tab 表格数据 |
| 3 | 裁撤明细导出 | POST | `/api/v1/woa/dissolve/host/detail/export/list` | 明细 Tab 导出 Excel |
| 4 | 裁撤主机同步 | POST | `/api/v1/woa/dissolve/recycled_host/sync` | 同步弹窗 |
| 5 | 裁撤配置获取 | GET | `/api/v1/woa/dissolve/config` | 配置回填 + 时间段选项 |
| 6 | 裁撤配置保存 | PUT | `/api/v1/woa/dissolve/config/upsert` | 配置弹窗保存 |
| 7 | 裁撤项目列表 | GET | `/api/v1/woa/dissolve/projects` | 搜索栏项目类型下拉 |

---

## 1. 裁撤总览列表查询

```http
POST /api/v1/woa/dissolve/table/list
Content-Type: application/json
```

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| project_ids | int[] | 否 | 裁撤项目 ID 列表 |
| group_ids | int64[] | 否 | 组织 ID 列表 |
| bk_biz_ids | int64[] | 否 | 业务 ID 列表 |
| operators | string[] | 否 | 负责人列表 |
| regions | string[] | 否 | 地域 ID 列表 |

```json
{
  "project_ids": [1, 2],
  "group_ids": [1111],
  "bk_biz_ids": [100],
  "operators": ["test"],
  "regions": ["ap-guangzhou"]
}
```

### 响应

```json
{
  "code": 0,
  "message": "",
  "data": {
    "items": [
      {
        "bk_biz_id": 100,
        "origin_host_count": 8,
        "origin_cpu_core": 640,
        "current_host_count": 0,
        "current_cpu_core": 0,
        "delivered_cpu_core": 100,
        "progress": "100.00%"
      },
      {
        "bk_biz_id": 0,
        "origin_host_count": 8,
        "origin_cpu_core": 640,
        "current_host_count": 0,
        "current_cpu_core": 0,
        "delivered_cpu_core": 100,
        "progress": "100.00%"
      }
    ]
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| data.items | array | 业务裁撤进度数据，最后一行为「合计」行 |
| data.items[n].bk_biz_id | int64 | 业务 ID；合计行该字段为 0 |
| data.items[n].origin_host_count | int64 | 原始裁撤设备数（命中条件的全部记录） |
| data.items[n].origin_cpu_core | int64 | 原始裁撤 CPU 总核数 |
| data.items[n].current_host_count | int64 | 当前裁撤设备数（`abolish_phase != complete`） |
| data.items[n].current_cpu_core | int64 | 当前裁撤 CPU 总核数 |
| data.items[n].delivered_cpu_core | int64 | 已申领 CPU 核数 |
| data.items[n].progress | string | 裁撤进度 = (原始设备数 − 当前设备数) / 原始设备数，如 `"100.00%"` |

### 前端数据模型

```typescript
interface IDissolveOverview {
  bk_biz_id: number;       // 合计行为 0
  origin_host_count: number;
  origin_cpu_core: number;
  current_host_count: number;
  current_cpu_core: number;
  delivered_cpu_core: number;
  progress: string;        // "100.00%"，渲染时转数字 + 进度条
}
```

### 特殊处理
- 总览接口**不支持分页**（无 page 参数），返回全部业务行 + 一个合计行
- 合计行 `bk_biz_id === 0`，始终置顶，不参与本地排序，业务名称不可点击
- 本地排序通过 `sortedList` computed 控制，bk-table 通过 `sort: { sortFn: () => 0 }` 禁用自动排序

---

## 2. 裁撤明细列表查询

```http
POST /api/v1/woa/dissolve/host/detail/list
Content-Type: application/json
```

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| bk_biz_ids | int64[] | 否 | 业务 ID 列表 |
| project_ids | int[] | 否 | 裁撤项目 ID 列表 |
| group_ids | int64[] | 否 | 运维小组 ID 列表 |
| operators | string[] | 否 | 负责人列表 |
| modules | string[] | 否 | 裁撤模块名称列表 |
| inner_ips | string[] | 否 | 内网 IP 列表 |
| asset_ids | string[] | 否 | 主机固资号列表 |
| status | string | 否 | 裁撤状态：`complete`（已裁撤）/ `incomplete`（未裁撤），不传查全部 |
| page | object | 是 | 分页设置 |

#### page

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| count | bool | 是 | 是否返回总记录条数，为 true 时仅返回 count |
| start | uint | 否 | 记录开始位置 |
| limit | uint | 否 | 每页限制条数，最大 500 |
| sort | string | 否 | 排序字段 |

```json
{
  "bk_biz_ids": [100],
  "status": "incomplete",
  "page": {
    "count": false,
    "start": 0,
    "limit": 50
  }
}
```

### 响应

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 1,
    "details": [
      {
        "id": "00000001",
        "asset_id": "ABC123",
        "inner_ip": "1.1.1.1",
        "module": "module-a",
        "status": "incomplete",
        "project_id": 100,
        "project_name": "2026年第一批裁撤",
        "region": "ap-guangzhou",
        "bk_biz_id": 100,
        "group_id": 200,
        "operators": ["zhangsan", "lisi"],
        "cpu_core": 64
      }
    ]
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| data.count | int64 | 总记录条数 |
| data.details | array | 裁撤主机明细列表 |
| data.details[n].id | string | 记录 ID |
| data.details[n].asset_id | string | 主机固资号 |
| data.details[n].inner_ip | string | 内网 IP |
| data.details[n].module | string | 裁撤模块名称 |
| data.details[n].status | string | `complete`（已裁撤）/ `incomplete`（未裁撤） |
| data.details[n].project_id | int | 裁撤项目 ID |
| data.details[n].project_name | string | 裁撤项目名称 |
| data.details[n].region | string | 地域 ID |
| data.details[n].bk_biz_id | int64 | 业务 ID |
| data.details[n].group_id | int64 | 运维小组 ID |
| data.details[n].operators | string[] | 负责人列表 |
| data.details[n].cpu_core | int | CPU 核心数 |

### 前端数据模型

```typescript
type DissolveDetailStatus = 'complete' | 'incomplete';

interface IDissolveDetail {
  id: string;
  asset_id: string;
  inner_ip: string;
  module: string;
  status: DissolveDetailStatus;
  project_id: number;
  project_name: string;
  region: string;
  bk_biz_id: number;
  group_id: number;
  operators: string[];
  cpu_core: number;
}
```

---

## 3. 裁撤明细导出

```http
POST /api/v1/woa/dissolve/host/detail/export/list
Content-Type: application/json
```

### 请求参数

与明细列表查询相同，额外增加：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| snapshot_date | string | 否 | ES 快照日期（格式 yyyyMMdd），传入时补充主机利用率扩展字段 |
| page.limit | uint | 否 | 最大 5000（导出用更大值） |

```json
{
  "bk_biz_ids": [100],
  "status": "incomplete",
  "snapshot_date": "20260601",
  "page": {
    "count": false,
    "start": 0,
    "limit": 5000
  }
}
```

### 响应

字段同明细列表查询，额外包含 `extension` 对象（仅传入 snapshot_date 且快照命中时返回）：

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 1,
    "details": [
      {
        "id": "00000001",
        "asset_id": "ABC123",
        "inner_ip": "1.1.1.1",
        "module": "module-a",
        "status": "incomplete",
        "project_id": 100,
        "project_name": "2026年第一批裁撤",
        "region": "ap-guangzhou",
        "bk_biz_id": 100,
        "group_id": 200,
        "operators": ["zhangsan"],
        "cpu_core": 64,
        "extension": {
          "outer_ip": "",
          "device_type": "S5.LARGE",
          "module_name": "module-a",
          "idc_unit_name": "广州-1",
          "sfw_name_version": "TencentOS Server 3.1",
          "go_up_date": "2023-01-01",
          "raid_name": "RAID1",
          "logic_area": "logic-area-a",
          "device_layer": "应用层",
          "cpu_score": 0.12,
          "mem_score": 0.34,
          "inner_net_traffic_score": 0.1,
          "disk_io_score": 0.2,
          "disk_util_score": 0.3,
          "is_pass": true,
          "mem4linux": 16,
          "inner_net_traffic": 1.2,
          "outer_net_traffic": 0.5,
          "disk_io": 100,
          "disk_util": 0.3,
          "disk_total": 500,
          "group_name": "运维一组",
          "center": "业务中心A"
        }
      }
    ]
  }
}
```

### extension 字段

| 字段 | 类型 | 描述 |
|------|------|------|
| outer_ip | string | 公网IP |
| device_type | string | SCM设备类型 |
| module_name | string | 裁撤模块名称 |
| idc_unit_name | string | 存放机房管理单元 |
| sfw_name_version | string | 操作系统 |
| go_up_date | string | 上架时间 |
| raid_name | string | RAID结构 |
| logic_area | string | 逻辑区域 |
| device_layer | string | 设备技术分类 |
| cpu_score | float | CPU得分 |
| mem_score | float | 内存得分 |
| inner_net_traffic_score | float | 内网流量得分 |
| disk_io_score | float | 磁盘IO得分 |
| disk_util_score | float | 磁盘IO使用率得分 |
| is_pass | bool | 是否达标 |
| mem4linux | float | 内存使用量(G) |
| inner_net_traffic | float | 内网流量(Mb/s) |
| outer_net_traffic | float | 外网流量(Mb/s) |
| disk_io | float | 磁盘IO(Blocks/s) |
| disk_util | float | 磁盘IO使用率 |
| disk_total | float | 磁盘总量(G) |
| group_name | string | 运维小组 |
| center | string | 业务中心 |

### 前端实现

- 使用 `rollRequest` 分页拉取全部数据（limit=5000），按 `pagination.count` 总量控制
- 基础列从 `ModelPropertyColumn[]` 动态生成 `ExportColumn[]`
- 勾选「附带主机利用率」时追加 extension 的 22 个字段（定义在 `export-column.ts`）
- 字段路径使用 `extension.xxx` 嵌套格式，`exportTableToExcel` 原生支持
- 请求传入 `snapshot_date`（yyyyMMdd 格式，由日期选择器值转换）

---

## 4. 裁撤主机同步

```http
POST /api/v1/woa/dissolve/recycled_host/sync
Content-Type: application/json
```

### 请求

无请求体参数（同步范围为后端配置的全部项目类型）。

```json
{}
```

### 响应

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": null
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| result | bool | 请求成功与否 |
| code | int | 错误编码，0 表示成功 |
| message | string | 错误信息 |

### 前端处理
- 同步为异步操作，可能耗时较长
- 前端展示 loading，成功后关闭弹窗 + toast「同步成功」+ 刷新当前 Tab
- 失败后保持弹窗 + toast 错误信息

---

## 5. 裁撤配置获取

```http
GET /api/v1/woa/dissolve/config
```

### 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "host_apply_time": "2024-09-01T12:00:00Z",
    "approval_limit": 100,
    "quota_coefficient": 65,
    "quota_offsets": [
      {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"},
      {"bk_biz_id": 100002, "offset": 30, "type": "decrease", "memo": "额度回收"}
    ],
    "dissolve_projects": [
      {
        "start": "2026-01-01",
        "end": "2026-03-31",
        "default": true,
        "projects": [
          {"id": 100, "memo": "第一批"}
        ]
      }
    ]
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| data.host_apply_time | string | 统计机房裁撤主机的开始时间（ISO 8601） |
| data.approval_limit | string | 自动审批配置百分比，范围 0-100 |
| data.quota_coefficient | float64 | 配额系数（百分比），范围 1-100，默认 65 |
| data.quota_offsets | QuotaOffsetItem[] | 业务偏移配置数组 |
| data.dissolve_projects | DissolveProjectCycle[] | 裁撤周期配置数组 |

#### QuotaOffsetItem

| 字段 | 类型 | 说明 |
|------|------|------|
| bk_biz_id | int64 | 业务 ID |
| offset | int64 | 偏移值 |
| type | string | `increase`（调增）/ `decrease`（调减） |
| memo | string | 调整原因 |

#### DissolveProjectCycle

| 字段 | 类型 | 说明 |
|------|------|------|
| start | string | 裁撤周期开始时间（yyyy-MM-dd） |
| end | string | 裁撤周期结束时间（yyyy-MM-dd） |
| default | bool | 是否为当前裁撤周期（可有多个） |
| projects | DissolveProjectItem[] | 裁撤项目列表 |

#### DissolveProjectItem

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 裁撤项目 ID |
| memo | string | 项目备注 |

### 前端数据模型

```typescript
interface IQuotaOffsetItem {
  bk_biz_id: number;
  offset: number;
  type: 'increase' | 'decrease';
  memo: string;
}

interface IDissolveProjectItem {
  id: number;
  memo: string;
}

interface IDissolveProjectCycle {
  start: string;
  end: string;
  default: boolean;
  projects: IDissolveProjectItem[];
}

interface IDissolveConfig {
  host_apply_time: string;
  approval_limit: string;
  quota_coefficient: number;
  quota_offsets: IQuotaOffsetItem[];
  dissolve_projects: IDissolveProjectCycle[];
}
```

### 复用场景
- **裁撤配置弹窗**：打开时回填表单
- **搜索栏时间段选项**：从 `dissolve_projects` 渲染，`default: true` 的周期默认选中
- **同步弹窗**：从所有周期的 `projects` 中收集项目类型供展示

---

## 6. 裁撤配置保存

```http
PUT /api/v1/woa/dissolve/config/upsert
Content-Type: application/json
```

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| host_apply_time | string | 否 | 统计开始时间 |
| approval_limit | string | 否 | 自动审批比例 |
| quota_coefficient | float64 | 否 | 配额系数 |
| quota_offsets | QuotaOffsetItem[] | 否 | 全量覆盖，传 `[]` 清空 |
| dissolve_projects | DissolveProjectCycle[] | 否 | 全量覆盖，传 `[]` 清空；非 nil 时不可为空数组 |

```json
{
  "host_apply_time": "2024-09-01T12:00:00Z",
  "approval_limit": 100,
  "quota_coefficient": 65,
  "quota_offsets": [
    {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"}
  ],
  "dissolve_projects": [
    {
      "start": "2026-01-01",
      "end": "2026-03-31",
      "default": true,
      "projects": [
        {"id": 100, "memo": "第一批"}
      ]
    }
  ]
}
```

### 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": null
}
```

### 注意
- `quota_offsets` 和 `dissolve_projects` 为全量覆盖语义
- `bk_biz_id` 在 `QuotaOffsetItem` 中必须大于 0 且不能重复
- `DissolveProjectItem.id` 必须大于 0

---

## 7. 裁撤项目列表

```http
GET /api/v1/woa/dissolve/projects
```

### 响应

```json
{
  "code": 0,
  "message": "",
  "data": [
    {
      "id": 100,
      "projectName": "2026年第一批裁撤",
      "projectType": "machine"
    }
  ]
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| data[n].id | int | 裁撤项目 ID |
| data[n].projectName | string | 项目名称 |
| data[n].projectType | string | 项目类型：`machine` / `yunxi` / `lingxing` |

### 前端使用
- 搜索栏「项目类型」下拉选项的数据源
- `projectType` 字段用于区分不同类型的裁撤项目

---

## 8. 权限符号

| 权限符号 | 用途 |
|---------|------|
| `AUTH_FIND_DISSOLVE` | 总览/明细列表查看 |
| `AUTH_UPDATE_DISSOLVE` | 同步按钮 + 裁撤配置按钮 |
| `AUTH_PLATFORM_DISSOLVE` | 平台管理-机房裁撤（配置接口权限） |

使用方式：`<hcm-auth :sign="{ type: AUTH_UPDATE_DISSOLVE }">` 包裹按钮，`noPerm` 时隐藏。

---

## 9. 状态枚举

| 后端值 | 显示文本 | 说明 |
|--------|---------|------|
| `complete` | 已裁撤 | 裁撤流程已完成 |
| `incomplete` | 未裁撤 | 裁撤流程进行中或未开始 |

---

## 10. 完整的 store 接口调用汇总

```typescript
// store/dissolve/quota.ts

// 总览列表
getOverviewList(params: {
  project_ids?: number[];
  group_ids?: number[];
  bk_biz_ids?: number[];
  operators?: string[];
  regions?: string[];
}): Promise<{ items: IDissolveOverview[] }>

// 明细列表（分页）
getDetailList(params: {
  bk_biz_ids?: number[];
  project_ids?: number[];
  group_ids?: number[];
  operators?: string[];
  modules?: string[];
  inner_ips?: string[];
  asset_ids?: string[];
  status?: 'complete' | 'incomplete';
  page: { count: boolean; start: number; limit: number; sort?: string };
}): Promise<{ count: number; details: IDissolveDetail[] }>

// 明细导出（分页拉取全部）
exportDetailList(params: {
  bk_biz_ids?: number[];
  project_ids?: number[];
  group_ids?: number[];
  operators?: string[];
  modules?: string[];
  inner_ips?: string[];
  asset_ids?: string[];
  status?: 'complete' | 'incomplete';
  snapshot_date?: string;
  page: { count: boolean; start: number; limit: number; sort?: string };
}): Promise<{ count: number; details: IDissolveDetail[] }>

// 主机同步
syncRecycledHost(): Promise<void>

// 配置获取
getConfig(): Promise<IDissolveConfig>

// 配置保存
upsertConfig(data: IDissolveConfig): Promise<void>

// 项目列表
getProjects(): Promise<{ id: number; projectName: string; projectType: string }[]>
```

---

## 11. 前端关键实现注意

1. **总览汇总行**：`bk_biz_id === 0`，通过 `sortedList` computed 始终置顶，本地排序不参与
2. **bk-table 排序**：总览页使用 `sort: { sortFn: () => 0 }` 禁用表格自动排序，完全由 `sortedList` 控制
3. **明细分页**：使用 `page: { count: false, start, limit }` 结构，`count` 参数控制是否只返回计数
4. **导出分页**：使用 `rollRequest` + `pageEnableCountKey: 'count'` 分页拉取，limit=5000
5. **导出 extension 列**：22 个字段定义在 `detail/data-list/export-column.ts`，通过 `extension.xxx` 嵌套路径导出
6. **配置字段名**：后端使用 `dissolve_projects`（非 `time_periods`），周期字段为 `default`（非 `is_current`）
