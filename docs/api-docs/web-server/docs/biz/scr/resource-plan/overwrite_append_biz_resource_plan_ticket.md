### 描述

- 该接口提供版本：v1.9.2.11+。
- 该接口所需权限：业务-资源预测操作。
- 该接口功能描述：业务视角覆盖追加资源预测。支持在同一资源预测主单内先按筛选条件覆盖（删除）已有预测，再追加新的预测明细，并可选择跳过 ITSM 审批直接进入拆单。

**业务说明**：

- `type` 为**必填**主单类型，取值：`budget_declare`（预算申报）/ `add`（新增）/ `adjust`（调整）/ `delete`（取消）。系统不再按 cancel/add 明细自动推导主单类型。
- `type=budget_declare` 时创建预算申报主单，拆单统一走 adjust 路径；其子单 HCM 管理员审批跳过（含跨年）。
- `overwrite` 控制覆盖逻辑，字段为 `true` 时：按 业务 + 项目类型 + 技术分类 + 期望交付时间范围 筛选本地资源预测（res_plan_demand），按现有预测删除流程处理。
- 携带 `demands` 时：复用创建资源预测单据逻辑追加新预测。
- 仅覆盖不新增：`overwrite` 传 `true` 且不传 `demands`，仅执行筛选删除。
- `overwrite` 为 `false` 且未传 `demands` 时，视为参数非法，返回参数错误。
- `skip_itsm` 为 `true` 时跳过 ITSM 审批，直接进入拆单阶段。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append

### 路径参数

| 参数名称      | 参数类型   | 必选 | 描述    |
|-----------|--------|----|-------|
| bk_biz_id | int    | 是  | 业务 ID |

### 输入参数

| 参数名称             | 参数类型         | 必选 | 描述                                                         |
|------------------|--------------|----|------------------------------------------------------------|
| type             | string       | 是  | 主单类型，枚举值：budget_declare（预算申报）、add（新增）、adjust（调整）、delete（取消） |
| overwrite        | bool         | 否  | 覆盖开关，为 true 时按 overwrite_filter 删除已有预测，默认 false            |
| overwrite_filter | object       | 否  | 覆盖筛选条件，overwrite 为 true 时必填                                |
| skip_itsm        | bool         | 否  | 跳过 ITSM 审批开关，为 true 时跳过 ITSM 审批直接进入拆单，默认 false            |
| demand_class     | string       | 否  | 预测的需求类型（枚举值：CVM、CA）。携带 demands 时必填                        |
| demands          | object array | 否  | 追加的需求列表，仅覆盖不新增时可不传                                         |
| applicant        | string       | 是  | 提单人 |
| remark           | string       | 否  | 预测说明，携带 demands 时最少20字，最多1024字                            |

#### overwrite_filter

| 参数名称              | 参数类型         | 必选 | 描述                        |
|-------------------|--------------|----|---------------------------|
| obs_projects      | string array | 是  | OBS 项目类型列表                |
| technical_classes | string array | 否  | 技术分类列表，不传时筛选全部        |
| expect_time_range | object       | 是  | 期望交付时间范围，按期望交付时间筛选待删除预测   |

#### overwrite_filter.expect_time_range

| 参数名称  | 参数类型   | 必选 | 描述                                       |
|-------|--------|----|------------------------------------------|
| start | string | 是  | 起始时间，格式为 YYYY-MM-DD，例如 2024-01-01         |
| end   | string | 是  | 结束时间，不能早于 start，格式为 YYYY-MM-DD，例如 2024-01-01 |

#### demands[i]

| 参数名称             | 参数类型         | 必选 | 描述                                                |
|------------------|--------------|----|---------------------------------------------------|
| obs_project      | string       | 是  | OBS项目类型                                           |
| expect_time      | string       | 是  | 期望交付时间，格式为YYYY-MM-DD，例如2024-01-01                 |
| return_plan_time | string       | 否  | 预期退回时间，格式为YYYY-MM-DD。当OBS项目类型为"短租项目"时，该字段必填       |
| region_id        | string       | 是  | 地区/城市ID                                           |
| zone_id          | string       | 否  | 可用区ID                                             |
| demand_source    | string       | 否  | 需求分类/变更原因                                         |
| remark           | string       | 否  | 需求备注                                              |
| demand_res_types | string array | 是  | 预测资源类型列表(枚举值：CVM、CBS)，需求包含CVM时，传递CVM，包含CBS时，传递CBS |
| cvm              | object       | 否  | 申请的CVM信息                                          |
| cbs              | object       | 否  | 申请的CBS信息                                          |

#### demands[i].cvm

| 参数名称        | 参数类型   | 必选 | 描述                 |
|-------------|--------|----|--------------------|
| res_mode    | string | 是  | 资源模式(枚举值：按机型、按机型族) |
| device_type | string | 是  | 机型规格               |
| os          | int    | 是  | OS数，单位：台           |
| cpu_core    | int    | 是  | CPU核心数，单位：核        |
| memory      | int    | 是  | 内存大小，单位：GB         |

#### demands[i].cbs

| 参数名称      | 参数类型   | 必选 | 描述                                                |
|-----------|--------|----|---------------------------------------------------|
| disk_type | string | 是  | 云盘类型(枚举值：CLOUD_PREMIUM(高性能云硬盘)、CLOUD_SSD(SSD云硬盘)) |
| disk_io   | int    | 是  | 磁盘IO吞吐需求，无特殊要求填写15；高性能云盘上限150，SSD云硬盘上限260         |
| disk_size | int    | 是  | 云盘大小，单位：GB                                        |

### 调用示例

覆盖并追加，跳过 ITSM：

```json
{
  "type": "budget_declare",
  "overwrite": true,
  "overwrite_filter": {
    "obs_projects": [
      "常规项目"
    ],
    "technical_classes": [
      "标准型",
      "高IO型",
      "大数据型"
    ],
    "expect_time_range": {
      "start": "2024-09-01",
      "end": "2024-12-31"
    }
  },
  "skip_itsm": true,
  "demand_class": "CVM",
  "demands": [
    {
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
  ],
  "applicant": "zhangsan",
  "remark": "这是一个备注，这是一个备注，这是一个备注"
}
```

仅覆盖不新增：

```json
{
  "type": "delete",
  "overwrite": true,
  "overwrite_filter": {
    "obs_projects": [
      "常规项目"
    ],
    "technical_classes": [
      "推理GPU"，
      "训练GPU"
    ],
    "expect_time_range": {
      "start": "2024-09-01",
      "end": "2024-12-31"
    }
  },
  "applicant": "zhangsan",
  "remark": "仅清理已有预测"
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

| 参数名称 | 参数类型   | 描述       |
|------|--------|----------|
| id   | string | 资源预测主单ID |
