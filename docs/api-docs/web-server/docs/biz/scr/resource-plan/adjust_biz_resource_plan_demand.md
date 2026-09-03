### 描述

- 该接口提供版本：v1.7.1.0+。
- 该接口所需权限：业务-资源预测操作。
- 该接口功能描述：批量调整资源预测需求，支持仅调整数量/规格、仅调整到货时间、同时调整两者，以及在调整单内新增一条预测需求。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/adjust

### 输入参数

| 参数名称         | 参数类型         | 必选  | 描述                                                                       |
| ------------ | ------------ | --- | ------------------------------------------------------------------------ |
| demand_class | string       | 否   | 预测需求类型。当 **adjusts 全部为 add**（请求中无任何 demand_id）时 **必填**，枚举与新增单一致，如 CVM、CA |
| adjusts      | object array | 是   | 调整列表，数量最大100                                                             |

#### adjusts[i]

| 参数名称          | 参数类型   | 必选  | 描述                                                                                  |
| ------------- | ------ | --- | ----------------------------------------------------------------------------------- |
| demand_id     | string | 否   | 预测需求ID，adjust_type为add时不需要，其余类型必填                                                   |
| adjust_type   | string | 是   | 调整类型。枚举值：**delay**：仅调整期望交付时间（整单延期）；**update**：调规格/数量，或换机型且改期；**add**：调整单内新增预测       |
| demand_source | string | 否   | 需求分类/变更原因。**adjust_type 为 update 或 add 时**必填；每条 adjust 独立填写                         |
| original_info | object | 否   | 调整前需求信息。**update 时必填**；delay、add 不需要传（delay 后端按 demand_id 查库）                       |
| updated_info  | object | 否   | 调整后/新增的需求信息。**update、add 时必填**；delay 不需要传                                           |
| expect_time   | string | 否   | 修改后的期望交付时间，格式 YYYY-MM-DD。**delay 时必填**；update/add 的到货时间用 `updated_info.expect_time` |

#### adjusts[i].original_info & adjusts[i].updated_info

| 参数名称             | 参数类型         | 必选  | 描述                                                |
| ---------------- | ------------ | --- | ------------------------------------------------- |
| obs_project      | string       | 是   | OBS项目类型                                           |
| expect_time      | string       | 是   | 期望交付时间，格式为YYYY-MM-DD，例如2024-01-01                 |
| return_plan_time | string       | 否   | 预期退回时间，格式为YYYY-MM-DD。当OBS项目类型为"短租项目"时，该字段必填       |
| region_id        | string       | 是   | 地区/城市ID                                           |
| zone_id          | string       | 否   | 可用区ID                                             |
| remark           | string       | 否   | 需求备注                                              |
| demand_res_types | string array | 是   | 预测资源类型列表(枚举值：CVM、CBS)，需求包含CVM时，传递CVM，包含CBS时，传递CBS |
| cvm              | object       | 否   | 申请的CVM信息                                          |
| cbs              | object       | 否   | 申请的CBS信息                                          |

#### adjusts[i].original_info.cvm & adjusts[i].updated_info.cvm

| 参数名称        | 参数类型   | 必选  | 描述                 |
| ----------- | ------ | --- | ------------------ |
| res_mode    | string | 是   | 资源模式(枚举值：按机型、按机型族) |
| device_type | string | 是   | 机型规格               |
| os          | int    | 是   | OS数，单位：台           |
| cpu_core    | int    | 是   | CPU核心数，单位：核        |
| memory      | int    | 是   | 内存大小，单位：GB         |

#### adjusts[i].original_info.cbs & adjusts[i].updated_info.cbs

| 参数名称      | 参数类型   | 必选  | 描述                                                |
| --------- | ------ | --- | ------------------------------------------------- |
| disk_type | string | 是   | 云盘类型(枚举值：CLOUD_PREMIUM(高性能云硬盘)、CLOUD_SSD(SSD云硬盘)) |
| disk_io   | int    | 是   | 磁盘IO吞吐需求，无特殊要求填写15；高性能云盘上限150，SSD云硬盘上限260         |
| disk_size | int    | 是   | 云盘大小，单位：GB                                        |

### 调用示例

以下示例展示三种调整场景：`0000001z` 为组合调整（同时调整到货时间与机型/数量）；`0000002a` 为纯延期（仅调整到货时间）；第三条为在本调整单内新增一条预测需求。

```json
{
  "adjusts": [
    {
      "demand_id": "0000001z",
      "adjust_type": "update",
      "demand_source": "指标变化",
      "original_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-11-12",
        "return_plan_time": "2025-01-01",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "remark": "这里是需求备注",
        "demand_res_types": [
          "CVM",
          "CBS"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 123,
          "cpu_core": 123,
          "memory": 123
        },
        "cbs": {
          "disk_type": "CLOUD_PREMIUM",
          "disk_io": 123,
          "disk_size": 1024
        }
      },
      "updated_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-12-01",
        "return_plan_time": "2025-01-20",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "remark": "这里是需求备注",
        "demand_res_types": [
          "CVM",
          "CBS"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 123,
          "cpu_core": 123,
          "memory": 123
        },
        "cbs": {
          "disk_type": "CLOUD_PREMIUM",
          "disk_io": 123,
          "disk_size": 1024
        }
      }
    },
    {
      "demand_id": "0000002a",
      "adjust_type": "delay",
      "expect_time": "2025-01-01"
    },
    {
      "adjust_type": "add",
      "demand_source": "指标变化",
      "updated_info": {
        "obs_project": "常规项目",
        "expect_time": "2024-12-01",
        "return_plan_time": "2025-01-20",
        "region_id": "ap-shanghai",
        "zone_id": "ap-shanghai-2",
        "demand_res_types": [
          "CVM"
        ],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 2,
          "cpu_core": 16,
          "memory": 32
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
  "data": {
    "id": "00000001"
  }
}
```
### 响应参数说明

| 参数名称    | 参数类型   | 描述                        |
| ------- | ------ | ------------------------- |
| code    | int    | 错误编码。 0表示success，>0表示失败错误 |
| message | string | 请求失败返回的错误信息               |
| data    | object | 响应数据                      |

#### data

| 参数名称 | 参数类型   | 描述     |
| ---- | ------ | ------ |
| id   | string | 预测单据ID |
