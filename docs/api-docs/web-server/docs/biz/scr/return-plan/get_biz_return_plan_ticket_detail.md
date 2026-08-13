### 描述

- 该接口提供版本：v1.9.2.11+。
- 该接口所需权限：业务访问。
- 该接口功能描述：获取退回计划申请单据详情。返回单据基本信息、流转状态与退回计划明细列表，明细按 original(原始退回计划)/updated(变更后退回计划)结构返回，条目类型由两部分有无推导（追加/覆盖删除/调整）。CRP 单号等流转信息在子单相关接口中返回。

### URL

GET /api/v1/woa/bizs/{bk_biz_id}/plans/returns/tickets/{id}

### 路径参数

| 参数名称      | 参数类型   | 必选 | 描述        |
|-----------|--------|----|-----------|
| bk_biz_id | int    | 是  | 业务 ID     |
| id        | string | 是  | 退回计划主单 ID |

### 输入参数

无

### 调用示例

无

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "00000001",
    "base_info": {
      "type": "adjust",
      "type_name": "调整",
      "applicant": "zhangsan",
      "bk_biz_id": 123,
      "bk_biz_name": "biz_test",
      "op_product_id": 1001,
      "op_product_name": "运营产品",
      "plan_product_id": 1,
      "plan_product_name": "规划产品",
      "virtual_dept_id": 2,
      "virtual_dept_name": "部门",
      "remark": "这里是退回计划说明",
      "submitted_at": "2019-07-29 11:57:20"
    },
    "status_info": {
      "status": "auditing",
      "status_name": "处理中",
      "message": "如果失败，这里会写失败原因"
    },
    "details": [
      {
        "type": "add",
        "type_name": "追加",
        "original": null,
        "updated": {
          "obs_project": "常规项目",
          "plan_time": "2024-11-12",
          "resource_pool_name": "自研池",
          "city": "深圳",
          "zone": "深圳一区",
          "instance_model": "SA2.LARGE8",
          "cvm_amount": 10,
          "instance_type": "",
          "core_type_name": "",
          "core_amount": 0,
          "return_reason_class": "成本优化&利用率提升",
          "desc": "年度预算退回计划"
        }
      },
      {
        "type": "cancel",
        "type_name": "覆盖删除",
        "original": {
          "obs_project": "常规项目",
          "plan_time": "2024-10-01",
          "resource_pool_name": "公有池",
          "city": "上海",
          "zone": "",
          "instance_model": "",
          "cvm_amount": 0,
          "instance_type": "标准型SA2",
          "core_type_name": "小核心",
          "core_amount": 80,
          "return_reason_class": "成本优化&利用率提升",
          "desc": ""
        },
        "updated": null
      },
      {
        "type": "adjust",
        "type_name": "调整",
        "original": {
          "obs_project": "常规项目",
          "plan_time": "2024-09-01",
          "resource_pool_name": "自研池",
          "city": "广州",
          "zone": "广州一区",
          "instance_model": "SA2.LARGE8",
          "cvm_amount": 20,
          "instance_type": "",
          "core_type_name": "",
          "core_amount": 0,
          "return_reason_class": "成本优化&利用率提升",
          "desc": ""
        },
        "updated": {
          "obs_project": "常规项目",
          "plan_time": "2024-09-15",
          "resource_pool_name": "自研池",
          "city": "广州",
          "zone": "广州一区",
          "instance_model": "SA2.LARGE8",
          "cvm_amount": 10,
          "instance_type": "",
          "core_type_name": "",
          "core_amount": 0,
          "return_reason_class": "成本优化&利用率提升",
          "desc": "缩减退回数量"
        }
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

| 参数名称        | 参数类型         | 描述          |
|-------------|--------------|-------------|
| id          | string       | 退回计划申请单号    |
| base_info   | object       | 退回计划申请单基本信息 |
| status_info | object       | 退回计划申请单状态信息 |
| details     | object array | 退回计划明细列表    |

#### data.base_info

| 参数名称              | 参数类型   | 描述                                          |
|-------------------|--------|---------------------------------------------|
| type              | string | 单据类型（枚举值：add(新增)、adjust(调整)、cancel(取消)）     |
| type_name         | string | 单据类型名称                                      |
| applicant         | string | 提单人                                         |
| bk_biz_id         | int    | CC业务ID                                      |
| bk_biz_name       | string | CC业务名                                       |
| op_product_id     | int    | 运营产品ID                                      |
| op_product_name   | string | 运营产品名称                                      |
| plan_product_id   | int    | 规划产品ID                                      |
| plan_product_name | string | 规划产品名称                                      |
| virtual_dept_id   | int    | 虚拟部门ID                                      |
| virtual_dept_name | string | 虚拟部门名称                                      |
| remark            | string | 退回计划说明                                      |
| submitted_at      | string | 提单时间                                        |

#### data.status_info

| 参数名称        | 参数类型   | 描述                                                          |
|-------------|--------|-------------------------------------------------------------|
| status      | string | 单据状态（枚举值：init, auditing, rejected, partial_rejected, revoked, done, failed, partial_failed, terminated） |
| status_name | string | 单据状态名称                                                      |
| message     | string | 单据处理信息，失败时返回失败原因                                            |

#### data.details[i]

| 参数名称     | 参数类型   | 描述                                                        |
|----------|--------|-----------------------------------------------------------|
| type     | string | 明细类型（枚举值：add(追加)、cancel(覆盖删除)、adjust(调整)）                 |
| type_name | string | 明细类型名称                                                    |
| original | object | 原始退回计划，覆盖删除(cancel)、调整(adjust)时提供，追加(add)时为 null           |
| updated  | object | 变更后退回计划，追加(add)、调整(adjust)时提供，覆盖删除(cancel)时为 null          |

> 明细类型由 original/updated 两部分推导：仅 updated 为追加(add)；仅 original 为覆盖删除(cancel)；两者兼有为调整(adjust)。

#### data.details[i].original & data.details[i].updated

| 参数名称                | 参数类型   | 描述                                |
|---------------------|--------|-----------------------------------|
| obs_project         | string | OBS项目类型                           |
| plan_time           | string | 计划退回时间，格式为YYYY-MM-DD，例如2024-01-01 |
| resource_pool_name  | string | 资源池                               |
| city                | string | 城市                                |
| zone                | string | 可用区                               |
| instance_model      | string | 实例规格（与 instance_type 二选一）         |
| cvm_amount          | int    | 退回实例数（对应 instance_model）          |
| instance_type       | string | 实例类型（与 instance_model 二选一）        |
| core_type_name      | string | 核心类型（对应 instance_type）            |
| core_amount         | int    | 退回核心数（对应 instance_type）           |
| return_reason_class | string | 退回原因大类                            |
| desc                | string | 备注                                |
