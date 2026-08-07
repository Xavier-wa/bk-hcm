### 描述

- 该接口提供版本：v1.7.1+。
- 该接口所需权限：业务访问。
- 该接口功能描述：业务视角查询资源预测单据。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/list

### 输入参数

| 参数名称              | 参数类型         | 必选 | 描述                             |
|-------------------|--------------|----|--------------------------------|
| ticket_ids        | string array | 否  | 资源预测申请单据ID，精确匹配，不传时查询全部，最多传20个 |
| statuses          | string array | 否  | 单据状态列表，不传时查询全部，最多传20个          |
| ticket_types      | string array | 否  | 单据类型列表，不传时查询全部，最多传20个          |
| applicants        | string array | 否  | 提单人，精确匹配，不传时查询全部，最多传20个        |
| submit_time_range | object       | 否  | 提单时间范围                         |
| page              | object       | 是  | 分页设置                           |

#### submit_time_range

| 参数名称  | 参数类型   | 必选 | 描述                                          |
|-------|--------|----|---------------------------------------------|
| start | string | 是  | 起始时间，不能晚于当前时间，格式为YYYY-MM-DD，例如2024-01-01    |
| end   | string | 是  | 结束时间，不能早于start时间，格式为YYYY-MM-DD，例如2024-01-01 |

#### page

| 参数名称  | 参数类型   | 必选 | 描述                                                                                                                                                                                                                                                                    |
|-------|--------|----|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| count | bool   | 是  | 是否返回总记录条数。 如果为true，查询结果返回总记录条数 count，但不返回查询结果详情数据，此时 start 和 limit 参数将无效，且必需设置为0。如果为false，则根据 start 和 limit 参数，返回查询结果详情数据，但不返回总记录条数 count                                                                                                                             |
| start | int    | 否  | 记录开始位置，start 起始值为0                                                                                                                                                                                                                                                    |
| limit | int    | 否  | 每页限制条数，最大500，不能为0                                                                                                                                                                                                                                                     |
| sort  | string | 否  | 排序字段，返回数据将按该字段进行排序，默认根据submitted_at(提单时间)倒序排序，枚举值为：original_cpu_core(原始CPU核心数)、updated_cpu_core(变更后CPU核心数)、original_memory(原始内存大小)、updated_memory(变更后内存大小)、original_disk_size(原始云盘大小)、updated_disk_size(变更后云盘大小)、submitted_at(提单时间)、created_at(创建时间)、updated_at(更新时间) |
| order | string | 否  | 排序顺序，枚举值：ASC(升序)、DESC(降序)                                                                                                                                                                                                                                             |

### 调用示例

```json
{
  "ticket_ids": [
    "0000000001"
  ],
  "statuses": [
    "init",
    "auditing",
    "done",
    "rejected"
  ],
  "ticket_types": [
    "add",
    "adjust",
    "cancel"
  ],
  "applicants": [
    "zhangsan"
  ],
  "submit_time_range": {
    "start": "2023-03-01",
    "end": "2023-06-01"
  },
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
    "details": [
      {
        "id": "0000000001",
        "bk_biz_id": 111,
        "bk_biz_name": "业务",
        "op_product_id": 222,
        "op_product_name": "运营产品",
        "plan_product_id": 333,
        "plan_product_name": "规划产品",
        "demand_class": "CVM",
        "status": "init",
        "status_name": "待审批",
        "ticket_type": "add",
        "ticket_type_name": "新增",
        "original_info": {
          "cvm": {
            "cpu_core": null,
            "memory": null
          }
        },
        "updated_info": {
          "cvm": {
            "cpu_core": 123,
            "memory": 123
          }
        },
        "audited_original_info": {
          "cvm": {
            "cpu_core": null,
            "memory": null
          }
        },
        "audited_updated_info": {
          "cvm": {
            "cpu_core": 100,
            "memory": 100
          }
        },
        "applicant": "zhangsan",
        "remark": "这里是预测说明",
        "submitted_at": "2019-07-29 11:57:20",
        "completed_at": "2019-07-29 11:58:00",
        "created_at": "2019-07-29 11:57:20",
        "updated_at": "2019-07-29 11:57:20"
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

| 参数名称   | 参数类型         | 描述                                       |
|--------|--------------|------------------------------------------|
| count   | int          | 当前规则能匹配到的总记录条数，仅在 count 查询参数设置为 true 时返回 |
| details | object array | 查询返回的数据，仅在 count 查询参数设置为 false 时返回       |

#### data.details[n]

| 参数名称                  | 参数类型   | 描述                                                                                                      |
|-----------------------|--------|---------------------------------------------------------------------------------------------------------|
| id                    | string | 资源预测需求单据ID                                                                                              |
| bk_biz_id             | int    | 业务ID                                                                                                    |
| bk_biz_name           | string | 业务名称                                                                                                    |
| op_product_id         | int    | 运营产品ID                                                                                                  |
| op_product_name       | string | 运营产品名称                                                                                                  |
| plan_product_id       | int    | 规划产品ID                                                                                                  |
| plan_product_name     | string | 规划产品名称                                                                                                  |
| demand_class          | string | 预测的需求类型                                                                                                 |
| status                | string | 单据状态（枚举值：init, auditing, rejected, partial_rejected, done, failed, partial_failed, revoked, terminated） |
| status_name           | string | 单据状态名称                                                                                                  |
| ticket_type           | string | 单据类型（枚举值：add, adjust, cancel）                                                                           |
| ticket_type_name      | string | 单据类型名称                                                                                                  |
| original_info         | object | 调整前的需求信息 - 原始报备数                                                                                        |
| updated_info          | object | 调整后的需求信息 - 原始报备数                                                                                        |
| audited_original_info | object | 调整前的需求信息 - 已审批数                                                                                         |
| audited_updated_info  | object | 调整后的需求信息 - 已审批数                                                                                         |
| applicant             | string | 提单人                                                                                                     |
| remark                | string | 备注                                                                                                      |
| submitted_at          | string | 提单时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30                                                       |
| completed_at          | string | 完成时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30                                                       |
| created_at            | string | 创建时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30                                                       |
| updated_at            | string | 更新时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30                                                       |

#### data.details[n].(audited_)original_info & data.details[n].(audited_)updated_info

| 参数名称 | 参数类型   | 描述       |
|------|--------|----------|
| cvm  | object | 申请的CVM信息 |
| cbs  | object | 申请的CBS信息 |

#### data.details[n].(audited_)original_info.cvm & data.details[n].(audited_)updated_info.cvm

| 参数名称     | 参数类型 | 描述       |
|----------|------|----------|
| cpu_core | int  | CPU核数（核） |
| memory   | int  | 内存总量（G）  |

#### data.details[n].(audited_)original_info.cbs & data.details[n].(audited_)updated_info.cbs

| 参数名称     | 参数类型 | 描述       |
|----------|------|----------|
| disk_size | int | 云盘大小（G） |
