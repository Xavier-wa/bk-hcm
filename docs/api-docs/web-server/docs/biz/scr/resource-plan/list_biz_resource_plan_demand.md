### 描述

- 该接口提供版本：v1.7.1.0+。
- 该接口所需权限：业务访问。
- 该接口功能描述：查询业务下资源预测需求列表。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/list

### 输入参数

| 参数名称              | 参数类型         | 必选 | 描述                                                                                            |
|-------------------|--------------|----|-----------------------------------------------------------------------------------------------|
| demand_ids        | string array | 否  | 预测需求ID列表，不传时查询全部，数量最大100                                                                      |
| obs_projects      | string array | 否  | OBS项目类型列表，不传时查询全部，数量最大100                                                                     |
| demand_classes    | string array | 否  | 预测需求类型列表，不传时查询全部，数量最大100                                                                      |
| core_types        | string array | 否  | 核心类型列表，不传时查询全部，数量最大100                                                                        | 
| device_families   | string array | 否  | 机型族列表，不传时查询全部，数量最大100                                                                         |
| device_classes    | string array | 否  | 机型分类列表，不传时查询全部，数量最大100                                                                        |
| device_types      | string array | 否  | 机型规格列表，不传时查询全部，数量最大100                                                                        |
| region_ids        | string array | 否  | 地区/城市ID列表，不传时查询全部，数量最大100                                                                     |
| zone_ids          | string array | 否  | 可用区ID列表，不传时查询全部，数量最大100                                                                       |
| plan_types        | string array | 否  | 计划类型列表，不传时查询全部，数量最大100                                                                        |
| expiring_only     | bool         | 否  | 是否只查询即将过期的需求，传true时只返回即将过期的需求，传false时查询全部，默认查询全部                                              |
| expect_time_range | object       | 是  | 期望交付时间范围                                                                                      |
| statuses          | string array | 否  | 状态，枚举值：can_apply（可申领）、not_ready（未到申领时间）、expired（已过期）、spent_all（已耗尽）、locked（变更中），不传时查询全部，数量最大5 |
| page              | object       | 是  | 分页设置                                                                                          |

#### expect_time_range

| 参数名称  | 参数类型   | 必选 | 描述                                          |
|-------|--------|----|---------------------------------------------|
| start | string | 是  | 起始时间，不能晚于当前时间，格式为YYYY-MM-DD，例如2024-01-01    |
| end   | string | 是  | 结束时间，不能早于start时间，格式为YYYY-MM-DD，例如2024-01-01 |

#### page

| 参数名称  | 参数类型   | 必选 | 描述                                                                                                                                                                        |
|-------|--------|----|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| count | bool   | 是  | 是否返回总记录条数。 如果为true，查询结果返回总记录条数 count，但不返回查询结果详情数据，此时 start 和 limit 参数将无效，且必需设置为0。如果为false，则根据 start 和 limit 参数，返回查询结果详情数据，但不返回总记录条数 count                                 |
| start | int    | 否  | 记录开始位置，start 起始值为0                                                                                                                                                        |
| limit | int    | 否  | 每页限制条数，最大500，不能为0                                                                                                                                                         |
| sort  | string | 否  | 排序字段，返回数据将按该字段进行排序，默认根据submitted_at(提单时间)倒序排序，枚举值为：cpu_core(CPU核心数)、memory(内存大小)、disk_size(云盘大小)、expect_time(期望交付时间)、submitted_at(提单时间)、created_at(创建时间)、updated_at(更新时间) |
| order | string | 否  | 排序顺序，枚举值：ASC(升序)、DESC(降序)                                                                                                                                                 |

### 调用示例

```json
{
  "demand_ids": [
    "0000001z"
  ],
  "obs_projects": [
    "常规项目"
  ],
  "demand_classes": [
    "CVM"
  ],
  "core_types": [
    "大核心"
  ],
  "device_families": [
    "标准型"
  ],
  "device_classes": [
    "标准型S5"
  ],
  "device_types": [
    "S5.2XLARGE16"
  ],
  "region_ids": [
    "ap-shanghai"
  ],
  "zone_ids": [
    "ap-shanghai-2"
  ],
  "plan_types": [
    "预测内"
  ],
  "expiring_only": false,
  "expect_time_range": {
    "start": "2024-01-01",
    "end": "2024-01-01"
  },
  "statuses": [
    "can_apply"
  ],
  "page": {
    "count": false,
    "start": 0,
    "limit": 500
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "overview": {
      "total_cpu_core": 1024,
      "total_applied_core": 1024,
      "in_plan_cpu_core": 512,
      "in_plan_applied_cpu_core": 512,
      "out_plan_cpu_core": 512,
      "out_plan_applied_cpu_core": 512,
      "expiring_cpu_core": 224
    },
    "details": [
      {
        "demand_id": "0000001z",
        "bk_biz_id": 111,
        "bk_biz_name": "业务",
        "op_product_id": 222,
        "op_product_name": "运营产品",
        "plan_product_id": 333,
        "plan_product_name": "规划产品",
        "status": "locked",
        "status_name": "变更中",
        "demand_class": "CVM",
        "demand_res_type": "CVM",
        "expect_time": "2024-01-01",
        "can_apply_time": "2024-01-01",
        "expired_time": "2024-01-28",
        "return_plan_time": "2025-01-01",
        "device_class": "高IO型I6t",
        "device_type": "I6t.33XMEDIUM198",
        "total_os": "56.5",
        "applied_os": "44.5",
        "remained_os": "12",
        "total_cpu_core": 560,
        "applied_cpu_core": 440,
        "remained_cpu_core": 120,
        "total_memory": 560,
        "applied_memory": 440,
        "remained_memory": 120,
        "total_disk_size": 560,
        "applied_disk_size": 440,
        "remained_disk_size": 120,
        "region_id": "ap-shanghai",
        "region_name": "上海",
        "zone_id": "ap-shanghai-2",
        "zone_name": "上海二区",
        "plan_type": "预测内",
        "obs_project": "常规项目",
        "technical_class": "高IO型",
        "device_family": "高IO型",
        "core_type": "大核心",
        "disk_type": "CLOUD_PREMIUM",
        "disk_type_name": "高性能云硬盘",
        "disk_io": 15
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                        |
|---------|--------|---------------------------|
| code    | int    | 错误编码。 0表示success，>0表示失败错误 |
| message | string | 请求失败返回的错误信息               |
| data	   | object | 响应数据                      |

#### data

| 参数名称     | 参数类型         | 描述                                       |
|----------|--------------|------------------------------------------|
| overview | object       | 概览信息                                     |
| count    | int          | 当前规则能匹配到的总记录条数，仅在 count 查询参数设置为 true 时返回 |
| details  | object array | 查询返回的数据，仅在 count 查询参数设置为 false 时返回       |

#### data.overview

| 参数名称                      | 参数类型 | 描述           |
|---------------------------|------|--------------|
| total_cpu_core            | int  | 总CPU核心数      |
| total_applied_core        | int  | 总已执行CPU核心数   |
| in_plan_cpu_core          | int  | 预测内CPU核心数    |
| in_plan_applied_cpu_core  | int  | 预测内已执行CPU核心数 |
| out_plan_cpu_core         | int  | 预测外CPU核心数    |
| out_plan_applied_cpu_core | int  | 预测外已执行CPU核心数 |
| expiring_cpu_core         | int  | 即将到期CPU核心数   |

#### data.details[n]

| 参数名称               | 参数类型   | 描述                                                                                |
|--------------------|--------|-----------------------------------------------------------------------------------|
| demand_id          | string | 预测需求ID                                                                            |
| bk_biz_id          | int    | 业务ID                                                                              |
| bk_biz_name        | string | 业务名称                                                                              |
| op_product_id      | int    | 运营产品ID                                                                            |
| op_product_name    | string | 运营产品名称                                                                            |
| plan_product_id    | int    | 规划产品ID                                                                            |
| plan_product_name  | string | 规划产品名称                                                                            |
| status             | string | 需求状态，枚举值：can_apply（可申领）、not_ready（未到申领时间）、expired（已过期）、spent_all（已耗尽）、locked（变更中） |
| status_name        | string | 需求状态名称                                                                            |
| demand_class       | string | 预测的需求类型，枚举值：CVM、CA                                                                |
| demand_res_type    | string | 预测资源类型，枚举值：CVM、CBS                                                                |
| expect_time        | string | 期望交付日期，格式为YYYY-MM-DD，例如2024-01-01                                                 |
| can_apply_time     | string | 可申领时间，格式为YYYY-MM-DD，例如2024-01-01                                                  |
| expired_time       | string | 预测申领截止日期，格式为YYYY-MM-DD，例如2024-01-01                                               |
| return_plan_time   | string | 预期退回时间，格式为YYYY-MM-DD，例如2024-01-01（仅短租项目存在该字段，其他项目类型该字段为空）                         |
| device_class       | string | 机型类型                                                                              |
| device_type        | string | 机型规格                                                                              |
| total_os           | string | 总OS数量                                                                             |
| applied_os         | string | 已申请OS数量                                                                           |
| remained_os        | string | 剩余OS数量                                                                            |
| total_cpu_core     | int    | 总CPU核数                                                                            |
| applied_cpu_core   | int    | 已申请CPU核数                                                                          |
| remained_cpu_core  | int    | 剩余CPU核数                                                                           |
| total_memory       | int    | 总内存大小                                                                             |
| applied_memory     | int    | 已申请内存大小                                                                           |
| remained_memory    | int    | 剩余内存大小                                                                            |
| total_disk_size    | int    | 总云盘大小                                                                             |
| applied_disk_size  | int    | 已申请云盘大小                                                                           |
| remained_disk_size | int    | 剩余云盘大小                                                                            |
| region_id          | string | 地区/城市ID                                                                           |
| region_name        | string | 地区/城市名称                                                                           |
| zone_id            | string | 可用区ID                                                                             |
| zone_name          | string | 可用区名称                                                                             |
| plan_type          | string | 计划类型                                                                              |
| obs_project        | string | OBS项目类型                                                                           |
| technical_class    | string | 技术分类                                                                              |
| device_family      | string | 机型族                                                                               |
| core_type          | string | 核心类型                                                                              |
| disk_type          | string | 云盘类型                                                                              |
| disk_type_name     | string | 云盘类型名称                                                                            |
| disk_io            | int    | 云盘IO                                                                              |
