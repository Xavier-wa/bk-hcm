### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务-退回计划操作。
- 该接口功能描述：业务视角覆盖追加退回计划。支持在同一主单内先按筛选条件覆盖（删除）已有退回计划，再追加新的退回计划明细。

**业务说明**：

- `overwrite` 控制覆盖逻辑，字段为 `true` 时：按 业务 + 项目类型 + 技术分类 + 计划退回时间范围 筛选 CRP 已有退回计划并删除。
- 携带 `return_details` 时：追加新的退回计划明细。
- 仅覆盖不新增：`overwrite` 传 `true` 且不传 `return_details`，仅执行筛选删除。
- `overwrite` 为 `false` 且未传 `return_details` 时，视为参数非法，返回参数错误。
- HCM 本地仅持久化单据与流转状态，不持久化退回计划明细，明细以 CRP 为准。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/returns/tickets/overwrite_append

### 路径参数

| 参数名称      | 参数类型 | 必选 | 描述    |
|-----------|------|----|-------|
| bk_biz_id | int  | 是  | 业务 ID |

### 输入参数

| 参数名称             | 参数类型         | 必选 | 描述                                                |
|------------------|--------------|----|---------------------------------------------------|
| overwrite        | bool         | 否  | 覆盖开关，为 true 时按 overwrite_filter 删除已有退回计划，默认 false |
| overwrite_filter | object       | 否  | 覆盖筛选条件，overwrite 为 true 时必填                       |
| return_details   | object array | 否  | 追加的退回计划明细列表，仅覆盖不新增时可不传                            |
| applicant        | string       | 是  | 提单人，作为 CRP 提单人，用于覆盖当前调用账号（如 admin）                |
| remark           | string       | 否  | 说明                                                |

#### overwrite_filter

| 参数名称              | 参数类型         | 必选 | 描述                        |
|-------------------|--------------|----|---------------------------|
| obs_projects      | string array | 是  | OBS 项目类型列表                |
| technical_classes | string array | 否  | 技术分类列表，不传时筛选全部            |
| plan_time_range   | object       | 是  | 计划退回时间范围，按预计退回时间筛选待删除退回计划 |

#### overwrite_filter.plan_time_range

| 参数名称  | 参数类型   | 必选 | 描述                                           |
|-------|--------|----|----------------------------------------------|
| start | string | 是  | 起始时间，格式为 YYYY-MM-DD，例如 2024-01-01            |
| end   | string | 是  | 结束时间，不能早于 start，格式为 YYYY-MM-DD，例如 2024-01-01 |

#### return_details[i]

| 参数名称                | 参数类型   | 必选 | 描述                                                          |
|---------------------|--------|----|-------------------------------------------------------------|
| obs_project         | string | 是  | OBS 项目类型                                                    |
| plan_time           | string | 是  | 计划退回时间，格式为 YYYY-MM-DD。CRP 要求不早于当前时间 + 35 天                  |
| resource_pool_name  | string | 否  | 资源池（枚举：自研池、公有池，默认自研池）                                       |
| region_id           | string | 是  | 地区/城市ID                                                     |
| zone_id             | string | 否  | 可用区ID                                                       |
| instance_model      | string | 否  | 实例规格。与 instance_type 二选一，与 cvm_amount 搭配使用                  |
| cvm_amount          | int    | 否  | 退回实例数。传 instance_model 时必填                                  |
| instance_type       | string | 否  | 实例类型。与 instance_model 二选一，与 core_type_name、core_amount 搭配使用 |
| core_type_name      | string | 否  | 核心类型。传 instance_type 时必填                                    |
| core_amount         | int    | 否  | 退回核心数。传 instance_type 时必填                                   |
| return_reason_class | string | 否  | 退回原因大类，为空时使用默认值「成本优化&利用率提升」                                 |
| desc                | string | 否  | 备注                                                          |

### 调用示例

覆盖并追加退回计划（按实例规格 + 实例数）：

```json
{
  "overwrite": true,
  "overwrite_filter": {
    "obs_projects": [
      "常规项目"
    ],
    "technical_classes": [
      "云上计算标准"
    ],
    "plan_time_range": {
      "start": "2024-09-01",
      "end": "2024-12-31"
    }
  },
  "return_details": [
    {
      "obs_project": "常规项目",
      "product_name": "互娱资源公共平台",
      "plan_time": "2024-11-12",
      "resource_pool_name": "自研池",
      "region_id": "ap-shanghai",
      "zone_id": "ap-shanghai-2",
      "instance_model": "SA2.LARGE8",
      "cvm_amount": 10,
      "return_reason_class": "成本优化&利用率提升",
      "desc": "年度预算退回计划"
    }
  ],
  "applicant": "zhangsan",
  "remark": "这是一个退回计划说明"
}
```

仅覆盖不新增：

```json
{
  "overwrite": true,
  "overwrite_filter": {
    "obs_projects": [
      "常规项目"
    ],
    "technical_classes": [
      "云上计算标准"
    ],
    "plan_time_range": {
      "start": "2024-09-01",
      "end": "2024-12-31"
    }
  },
  "applicant": "zhangsan",
  "remark": "仅清理已有退回计划"
}
```

仅追加（按实例类型 + 核心类型 + 核心数）：

```json
{
  "overwrite": false,
  "return_details": [
    {
      "obs_project": "常规项目",
      "plan_time": "2024-11-12",
      "resource_pool_name": "自研池",
      "region_id": "ap-shanghai",
      "instance_type": "标准型SA2",
      "core_type_name": "小核心",
      "core_amount": 80,
      "desc": "追加退回计划"
    }
  ],
  "applicant": "zhangsan",
  "remark": "这是一个退回计划说明"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "00000001"
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

| 参数名称 | 参数类型   | 描述        |
|------|--------|-----------|
| id   | string | 退回计划主单 ID |
