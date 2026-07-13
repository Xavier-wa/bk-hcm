### 描述

- 该接口提供版本：v1.8.11.15+。
- 该接口所需权限：业务访问。
- 该接口功能描述：覆盖修改资源预测主单并重试。适用于主单处于可覆盖状态（审批驳回、部分审批驳回、失败、部分失败、已撤销）时，由业务侧修改主单内容后重新发起审批流程。

**业务说明**：

- 请求体字段均为可选，仅更新传入的字段；未传入的字段保持原值不变。
- 传入 `demands` 时，会整体替换主单原有需求列表及关联资源汇总字段，此时必须同时传入 `ticket_type`。
- `ticket_type` 使用资源预测主单类型枚举：`add` 表示新增，`adjust` 表示调整，`delete` 表示取消。
- 传入 `demands` 时，`demands[i].original_info` 与 `demands[i].updated_info` 的组合由 `ticket_type` 决定：`add` 仅传 `updated_info`；`adjust` 两者均需传入；`delete` 仅传 `original_info`。
- 修改成功后，主单状态重置为「审批中」；原有关联子单将被解绑作废（子单 `ticket_id` 变更为 `drop_{主单ID}`，状态为失败）。
- 若主单下存在已完成的子单，则不允许覆盖修改。

**可覆盖的主单状态**：

| 状态值              | 说明     |
|------------------|--------|
| rejected         | 审批驳回   |
| partial_rejected | 部分审批驳回 |
| failed           | 失败     |
| partial_failed   | 部分失败   |
| revoked          | 已撤销    |

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/{ticket_id}/overwrite

### 路径参数

| 参数名称      | 参数类型   | 必选 | 描述      |
|-----------|--------|----|---------|
| bk_biz_id | int    | 是  | 业务 ID   |
| ticket_id | string | 是  | 预测主单 ID |

### 输入参数

| 参数名称         | 参数类型         | 必选 | 描述                                                         |
|--------------|--------------|----|------------------------------------------------------------|
| ticket_type  | string       | 否  | 覆盖后的单据类型，枚举值：`add`、`adjust`、`delete`。传入 `demands` 时必填，仅与 `demands` 一起使用 |
| demand_class | string       | 否  | 预测的需求类型（枚举值：CVM、CA）。传入时覆盖主单需求类型；不传则沿用原主单类型       |
| demands      | object array | 否  | 需求列表。传入时整体替换主单 demands 及关联资源汇总字段                        |
| remark       | string       | 否  | 预测说明。传入时覆盖；字符数不少于 20、不超过 1024                              |

#### demands[i]

| 参数名称          | 参数类型 | 必选 | 描述                                                                 |
|---------------|------|----|--------------------------------------------------------------------|
| demand_id     | string | 否  | 原预测需求 ID。`ticket_type` 为 `adjust`、`delete` 时，`demand_id` 和 `crp_demand_id` 至少传一个 |
| crp_demand_id | int    | 否  | CRP 预测需求 ID。`ticket_type` 为 `adjust`、`delete` 时，`demand_id` 和 `crp_demand_id` 至少传一个 |
| original_info | object | 否  | 调整前需求信息。`adjust`、`delete` 时必填；`add` 时不能传                         |
| updated_info  | object | 否  | 调整后需求信息。`add`、`adjust` 时必填；`delete` 时不能传                         |

#### demands[i].original_info / demands[i].updated_info

结构与 [创建资源预测单据](create_resource_plan_ticket.md) 中 `demands[i]` 一致。

| 参数名称             | 参数类型         | 必选 | 描述                                             |
|------------------|--------------|----|------------------------------------------------|
| obs_project      | string       | 是  | OBS 项目类型                                       |
| expect_time      | string       | 是  | 期望交付时间，格式为 YYYY-MM-DD，例如 2024-01-01            |
| return_plan_time | string       | 否  | 预期退回时间，格式为 YYYY-MM-DD。当 OBS 项目类型为「短租项目」时，该字段必填 |
| region_id        | string       | 是  | 地区/城市 ID                                       |
| zone_id          | string       | 否  | 可用区 ID                                         |
| demand_source    | string       | 否  | 需求分类/变更原因                                      |
| remark           | string       | 否  | 需求备注                                           |
| demand_res_types | string array | 是  | 预测资源类型列表（枚举值：CVM、CBS）                          |
| cvm              | object       | 否  | 申请的 CVM 信息                                     |
| cbs              | object       | 否  | 申请的 CBS 信息                                     |

#### demands[i].original_info.cvm / demands[i].updated_info.cvm

| 参数名称        | 参数类型   | 必选 | 描述                 |
|-------------|--------|----|--------------------|
| res_mode    | string | 是  | 资源模式（枚举值：按机型、按机型族） |
| device_type | string | 是  | 机型规格               |
| os          | int    | 是  | OS 数，单位：台          |
| cpu_core    | int    | 是  | CPU 核心数，单位：核       |
| memory      | int    | 是  | 内存大小，单位：GB         |

#### demands[i].original_info.cbs / demands[i].updated_info.cbs

| 参数名称      | 参数类型   | 必选 | 描述                                              |
|-----------|--------|----|-------------------------------------------------|
| disk_type | string | 是  | 云盘类型（枚举值：CLOUD_PREMIUM、CLOUD_SSD）               |
| disk_io   | int    | 是  | 磁盘 IO 吞吐需求，无特殊要求填写 15；高性能云盘上限 150，SSD 云硬盘上限 260 |
| disk_size | int    | 是  | 云盘大小，单位：GB                                      |

### 调用示例

仅修改预测说明：

```json
{
  "remark": "根据审批意见调整预测说明，补充业务背景与交付计划说明"
}
```

覆盖为新增单据：

```json
{
  "ticket_type": "add",
  "demand_class": "CVM",
  "remark": "驳回后重新提交预测，已按审批意见调整新增需求明细与交付时间",
  "demands": [
    {
      "updated_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-11-12",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "demand_source": "指标变化",
        "remark": "这里是需求备注",
        "demand_res_types": [
          "CVM"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 100,
          "cpu_core": 400,
          "memory": 800
        }
      }
    }
  ]
}
```

覆盖为调整单据：

```json
{
  "ticket_type": "adjust",
  "demand_class": "CVM",
  "remark": "驳回后重新提交预测，已按审批意见调整变更前后资源信息",
  "demands": [
    {
      "demand_id": "00000001",
      "original_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-11-12",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "demand_res_types": [
          "CVM"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 100,
          "cpu_core": 400,
          "memory": 800
        }
      },
      "updated_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-11-12",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "demand_source": "指标变化",
        "demand_res_types": [
          "CVM"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 120,
          "cpu_core": 480,
          "memory": 960
        }
      }
    }
  ]
}
```

覆盖为取消单据：

```json
{
  "ticket_type": "delete",
  "demand_class": "CVM",
  "remark": "驳回后重新提交预测，已按审批意见确认取消需求范围",
  "demands": [
    {
      "demand_id": "00000001",
      "original_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-11-12",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "demand_res_types": [
          "CVM"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 100,
          "cpu_core": 400,
          "memory": 800
        }
      }
    }
  ]
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                        |
|---------|--------|---------------------------|
| code    | int    | 错误编码。0 表示 success，>0 表示失败 |
| message | string | 请求失败返回的错误信息               |
| data    | object | 响应数据                      |

#### data

无
