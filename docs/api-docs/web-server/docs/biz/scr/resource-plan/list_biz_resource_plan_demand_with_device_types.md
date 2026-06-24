### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：查询业务下资源预测需求列表（带可通配机型明细）。相同聚合键（业务ID、计划类型、可用时间、机型族、核心类型、OBS项目、地区）的 CVM 预测需求会被聚合为一条记录，每条聚合记录展开为多条机型明细，包含原始机型（`is_original=true`）和可通配的其他机型（`is_original=false`）。分页作用于展开后的明细记录。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/list_with_device_types

### 输入参数

| 参数名称              | 参数类型         | 必选 | 描述                                                                                            |
|-------------------|--------------|----|-----------------------------------------------------------------------------------------------|
| bk_biz_id         | int          | 是  | 业务ID，URL路径参数                                                                                  |
| obs_projects      | string array | 否  | OBS项目类型列表，不传时查询全部，数量最大100                                                                     |
| core_types        | string array | 否  | 核心类型列表，不传时查询全部，数量最大100                                                                        |
| device_families   | string array | 否  | 机型族列表，不传时查询全部，数量最大100                                                                         |
| device_classes    | string array | 否  | 机型分类列表，不传时查询全部，数量最大100                                                                        |
| device_types      | string array | 否  | 原始机型规格列表，不传时查询全部，数量最大100                                                                      |
| region_ids        | string array | 否  | 地区/城市ID列表，不传时查询全部，数量最大100                                                                     |
| plan_types        | string array | 否  | 计划类型列表，不传时查询全部，数量最大100                                                                        |
| cpu_cores         | int array    | 否  | CPU核心数筛选列表，用于过滤展开后的机型明细（含原始机型和通配机型），不传时不过滤，数量最大100                                            |
| memories          | int array    | 否  | 内存大小筛选列表（单位GB），用于过滤展开后的机型明细（含原始机型和通配机型），不传时不过滤，数量最大100                                        |
| expiring_only     | bool         | 否  | 是否只查询即将过期的需求，传true时只返回即将过期的需求，传false时查询全部，默认查询全部                                              |
| expect_time_range | object       | 是  | 期望交付时间范围                                                                                      |
| statuses          | string array | 否  | 状态，枚举值：can_apply（可申领）、not_ready（未到申领时间）、expired（已过期）、spent_all（已耗尽）、locked（变更中），不传时查询全部，数量最大5 |
| page              | object       | 是  | 分页设置，分页作用于展开后的明细记录                                                                            |

> **说明**：该接口固定查询 CVM 类型的预测需求，无需传 `demand_classes` 参数。`cpu_cores` 和 `memories`
> 过滤作用于展开阶段，同时过滤原始机型和通配机型。相同聚合键的预测需求会被自动聚合，`demand_ids`
> 字段包含被聚合的所有原始需求ID。

#### expect_time_range

| 参数名称  | 参数类型   | 必选 | 描述                                          |
|-------|--------|----|---------------------------------------------|
| start | string | 是  | 起始时间，格式为YYYY-MM-DD，例如2024-01-01             |
| end   | string | 是  | 结束时间，不能早于start时间，格式为YYYY-MM-DD，例如2024-01-01 |

#### page

| 参数名称  | 参数类型 | 必选 | 描述                                                                                                         |
|-------|------|----|------------------------------------------------------------------------------------------------------------|
| count | bool | 是  | 是否返回总记录条数。如果为true，返回展开后的总条数 count，不返回详情，此时 start 和 limit 须设置为0；如果为false，则根据 start 和 limit 返回明细数据，不返回 count |
| start | int  | 否  | 记录开始位置，start 起始值为0                                                                                         |
| limit | int  | 否  | 每页限制条数，最大500，不能为0                                                                                          |

### 调用示例

#### 基础查询

```json
{
  "expect_time_range": {
    "start": "2024-01-01",
    "end": "2024-12-31"
  },
  "page": {
    "count": false,
    "start": 0,
    "limit": 20
  }
}
```

#### 带筛选条件查询

```json
{
  "obs_projects": [
    "常规项目"
  ],
  "core_types": [
    "大核心"
  ],
  "device_families": [
    "SA3"
  ],
  "cpu_cores": [
    4,
    8
  ],
  "memories": [
    16,
    32
  ],
  "region_ids": [
    "ap-shanghai"
  ],
  "plan_types": [
    "预测内"
  ],
  "expiring_only": false,
  "expect_time_range": {
    "start": "2024-01-01",
    "end": "2024-12-31"
  },
  "statuses": [
    "can_apply"
  ],
  "page": {
    "count": false,
    "start": 0,
    "limit": 20
  }
}
```

#### 仅查询总条数

```json
{
  "expect_time_range": {
    "start": "2024-01-01",
    "end": "2024-12-31"
  },
  "page": {
    "count": true,
    "start": 0,
    "limit": 0
  }
}
```

### 响应示例

#### count 模式

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 15,
    "details": []
  }
}
```

#### detail 模式

两条预测需求（`demand_id: "0000001z"` 和 `demand_id: "0000001a"`）因具有相同聚合键被聚合为一条记录，`demand_ids` 包含两个ID。展开为
1 条原始机型 + 2 条通配机型，共 3 条明细记录：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 3,
    "details": [
      {
        "demand_ids": [
          "0000001z",
          "0000001a"
        ],
        "bk_biz_id": 111,
        "bk_biz_name": "业务A",
        "status": "can_apply",
        "status_name": "可申领",
        "demand_class": "CVM",
        "demand_res_type": "CVM",
        "expect_time": "2024-06-01",
        "can_apply_time": "2024-05-25",
        "expired_time": "2024-07-01",
        "return_plan_time": null,
        "total_cpu_core": 240,
        "applied_cpu_core": 160,
        "remained_cpu_core": 80,
        "region_id": "ap-shanghai",
        "region_name": "上海",
        "zone_id": "ap-shanghai-2",
        "zone_name": "上海二区",
        "plan_type": "预测内",
        "obs_project": "常规项目",
        "technical_class": "计算型",
        "device_family": "SA3",
        "core_type": "大核心",
        "disk_type": "CLOUD_SSD",
        "disk_type_name": "SSD云盘",
        "disk_io": 0,
        "device_type": "SA3.8XLARGE128",
        "is_original": true,
        "device_type_class": "CommonType",
        "device_class": "计算型SA3",
        "cpu_core": 8,
        "memory": 128,
        "total_os": 30,
        "applied_os": 20,
        "remained_os": 10
      },
      {
        "demand_ids": [
          "0000001z",
          "0000001a"
        ],
        "bk_biz_id": 111,
        "bk_biz_name": "业务A",
        "status": "can_apply",
        "status_name": "可申领",
        "demand_class": "CVM",
        "demand_res_type": "CVM",
        "expect_time": "2024-06-01",
        "can_apply_time": "2024-05-25",
        "expired_time": "2024-07-01",
        "return_plan_time": null,
        "total_cpu_core": 240,
        "applied_cpu_core": 160,
        "remained_cpu_core": 80,
        "region_id": "ap-shanghai",
        "region_name": "上海",
        "zone_id": "ap-shanghai-2",
        "zone_name": "上海二区",
        "plan_type": "预测内",
        "obs_project": "常规项目",
        "technical_class": "计算型",
        "device_family": "SA3",
        "core_type": "大核心",
        "disk_type": "CLOUD_SSD",
        "disk_type_name": "SSD云盘",
        "disk_io": 0,
        "device_type": "SA3.4XLARGE64",
        "is_original": false,
        "device_type_class": "CommonType",
        "device_class": "计算型SA3",
        "cpu_core": 4,
        "memory": 64,
        "total_os": 60,
        "applied_os": 40,
        "remained_os": 20
      },
      {
        "demand_ids": [
          "0000001z",
          "0000001a"
        ],
        "bk_biz_id": 111,
        "bk_biz_name": "业务A",
        "status": "can_apply",
        "status_name": "可申领",
        "demand_class": "CVM",
        "demand_res_type": "CVM",
        "expect_time": "2024-06-01",
        "can_apply_time": "2024-05-25",
        "expired_time": "2024-07-01",
        "return_plan_time": null,
        "total_cpu_core": 240,
        "applied_cpu_core": 160,
        "remained_cpu_core": 80,
        "region_id": "ap-shanghai",
        "region_name": "上海",
        "zone_id": "ap-shanghai-2",
        "zone_name": "上海二区",
        "plan_type": "预测内",
        "obs_project": "常规项目",
        "technical_class": "计算型",
        "device_family": "SA3",
        "core_type": "大核心",
        "disk_type": "CLOUD_SSD",
        "disk_type_name": "SSD云盘",
        "disk_io": 0,
        "device_type": "SA3.2XLARGE32",
        "is_original": false,
        "device_type_class": "CommonType",
        "device_class": "计算型SA3",
        "cpu_core": 2,
        "memory": 32,
        "total_os": 120,
        "applied_os": 80,
        "remained_os": 40
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                     |
|---------|--------|------------------------|
| code    | int    | 错误编码，0表示success，>0表示失败 |
| message | string | 请求失败返回的错误信息            |
| data    | object | 响应数据                   |

#### data

| 参数名称    | 参数类型         | 描述                                        |
|---------|--------------|-------------------------------------------|
| count   | int64        | 展开后的明细总条数，仅在 page.count=true 时返回，否则为0     |
| details | object array | 展开后的机型明细列表，仅在 page.count=false 时返回，否则为空数组 |

#### data.details[n]

**预测需求主数据字段：**

| 参数名称              | 参数类型         | 描述                                                                                |
|-------------------|--------------|-----------------------------------------------------------------------------------|
| demand_ids        | string array | 预测需求ID列表。单条预测时包含一个ID；聚合预测时包含被聚合的所有原始需求ID                                          |
| bk_biz_id         | int64        | 业务ID                                                                              |
| bk_biz_name       | string       | 业务名称                                                                              |
| status            | string       | 需求状态，枚举值：can_apply（可申领）、not_ready（未到申领时间）、expired（已过期）、spent_all（已耗尽）、locked（变更中） |
| status_name       | string       | 需求状态名称                                                                            |
| demand_class      | string       | 预测需求类型，固定为 CVM                                                                    |
| demand_res_type   | string       | 预测资源类型，枚举值：CVM                                                                    |
| expect_time       | string       | 期望交付日期，格式YYYY-MM-DD                                                               |
| can_apply_time    | string       | 可申领时间，格式YYYY-MM-DD                                                                |
| expired_time      | string       | 申领截止日期，格式YYYY-MM-DD                                                               |
| return_plan_time  | string       | 预期退回时间，格式YYYY-MM-DD，仅短租项目存在，其他为null                                               |
| total_cpu_core    | int64        | 预测总CPU核数（聚合后为所有被聚合需求的累加值）                                                         |
| applied_cpu_core  | int64        | 已申请CPU核数（聚合后为所有被聚合需求的累加值）                                                         |
| remained_cpu_core | int64        | 剩余CPU核数（聚合后为所有被聚合需求的累加值）                                                          |
| region_id         | string       | 地区/城市ID                                                                           |
| region_name       | string       | 地区/城市名称                                                                           |
| zone_id           | string       | 可用区ID                                                                             |
| zone_name         | string       | 可用区名称                                                                             |
| plan_type         | string       | 计划类型，枚举值                                                                          |
| obs_project       | string       | OBS项目类型，枚举值                                                                       |
| technical_class   | string       | 技术分类                                                                              |
| device_family     | string       | 机型族                                                                               |
| core_type         | string       | 核心类型，枚举值                                                                          |
| disk_type         | string       | 云盘类型，枚举值                                                                          |
| disk_type_name    | string       | 云盘类型名称                                                                            |
| disk_io           | int64        | 云盘IO                                                                              |

**机型明细字段（展开后）：**

| 参数名称              | 参数类型    | 描述                                                                |
|-------------------|---------|-------------------------------------------------------------------|
| device_type       | string  | 机型规格，可能是原始机型或通配机型                                                 |
| is_original       | bool    | 是否为原始机型。true=预测需求本身的机型，排在同预测的通配机型前；false=可通配的其他机型                 |
| device_type_class | string  | 通/专用机型类型，对应 device_type 表的同名字段，枚举值：CommonType（通用）、SpecialType（专用） |
| device_class      | string  | 机型大类，对应 device_type 表的同名字段，如"计算型SA3"、"大数据型DA5"、"高IO型ITA5"         |
| cpu_core          | int64   | 该机型单台CPU核数                                                        |
| memory            | int64   | 该机型单台内存大小（GB）                                                     |
| total_os          | decimal | 总OS数，计算公式：`total_cpu_core / cpu_core`，可能为小数                       |
| applied_os        | decimal | 已申请OS数，计算公式：`applied_cpu_core / cpu_core`，可能为小数                   |
| remained_os       | decimal | 剩余OS数，计算公式：`remained_cpu_core / cpu_core`，可能为小数                   |
