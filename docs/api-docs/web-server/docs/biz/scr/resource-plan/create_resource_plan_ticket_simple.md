### 描述

- 该接口提供版本：v1.9.2.0+。
- 该接口所需权限：业务-资源预测操作。
- 该接口功能描述：简化创建资源预测单据。

与 [create_resource_plan_ticket](./create_resource_plan_ticket.md) 相比，请求体省略 `demand_source`、`demand_res_types` 以及
CVM 的 `res_mode`：服务端会将 `demand_source` 固定为「指标变化」，将 `res_mode` 固定为「按机型」，并根据是否传入 `cvm` / `cbs`
自动推导 `demand_res_types`；CVM 的台数、CPU、内存可按简化规则填写并由服务端结合机型元数据展开为完整预测参数。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/create_simple

| 参数名称         | 参数类型         | 必选 | 描述                  |
|--------------|--------------|----|---------------------|
| demand_class | string       | 是  | 预测的需求类型(枚举值：CVM、CA) |
| demands      | object array | 是  | 需求列表                |
| remark       | string       | 是  | 预测说明，最少20字，最多1024字  |

#### demands[i]

| 参数名称             | 参数类型   | 必选 | 描述                                               |
|------------------|--------|----|--------------------------------------------------|
| obs_project      | string | 是  | OBS项目类型（取值需符合资源预测场景的 OBS 项目枚举，如：常规项目、短租项目、滚服项目等） |
| expect_time      | string | 是  | 期望交付时间，格式为YYYY-MM-DD，例如2024-01-01                |
| return_plan_time | string | 否  | 预期退回时间，格式为YYYY-MM-DD。当OBS项目类型为「短租项目」时，该字段必填      |
| region_id        | string | 是  | 地区/城市ID                                          |
| zone_id          | string | 否  | 可用区ID                                            |
| remark           | string | 否  | 需求备注，最多255字                                      |
| cvm              | object | 否  | 申请的CVM信息（与 `cbs` 至少填其一）                          |
| cbs              | object | 否  | 申请的CBS信息（与 `cvm` 至少填其一）                          |

说明：`demand_source` 由服务端填「指标变化」；`demand_res_types` 由服务端根据是否传入 `cvm` / `cbs` 自动包含 CVM、CBS。

#### demands[i].cvm

| 参数名称        | 参数类型   | 必选 | 描述                                                                                  |
|-------------|--------|----|-------------------------------------------------------------------------------------|
| device_type | string | 是  | 机型规格（须为系统元数据中存在的机型）                                                                 |
| os          | number | 否  | 台数；与 `cpu_core` 至少填其一。填写时服务端按机型单台 CPU/内存推导总 CPU、总内存；可用 `cpu_core` / `memory` 覆盖对应总量 |
| cpu_core    | int    | 否  | CPU 核心数；与 `os` 至少填其一。仅填此项时，服务端按机型规格反推台数与内存                                          |
| memory      | int    | 否  | 内存大小，单位：GB；可选，用于覆盖按台数或反推得到的内存总量                                                     |

#### demands[i].cbs

| 参数名称      | 参数类型   | 必选 | 描述                                                |
|-----------|--------|----|---------------------------------------------------|
| disk_type | string | 是  | 云盘类型(枚举值：CLOUD_PREMIUM(高性能云硬盘)、CLOUD_SSD(SSD云硬盘)) |
| disk_io   | int    | 是  | 磁盘IO吞吐需求，无特殊要求填写15；高性能云盘上限150，SSD云硬盘上限260         |
| disk_size | int    | 是  | 云盘大小，单位：GB                                        |

### 调用示例

```json
{
  "demand_class": "CVM",
  "demands": [
    {
      "obs_project": "常规项目",
      "expect_time": "2024-11-12",
      "return_plan_time": "2025-01-01",
      "region_id": "ap-shanghai",
      "zone_id": "ap-shanghai-2",
      "remark": "这里是需求备注",
      "cvm": {
        "device_type": "S5.2XLARGE16",
        "os": 10
      },
      "cbs": {
        "disk_type": "CLOUD_PREMIUM",
        "disk_io": 15,
        "disk_size": 1024
      }
    }
  ],
  "remark": "这是一个备注，这是一个备注，这是一个备注"
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

| 参数名称 | 参数类型   | 描述     |
|------|--------|--------|
| id   | string | 预测单据ID |
