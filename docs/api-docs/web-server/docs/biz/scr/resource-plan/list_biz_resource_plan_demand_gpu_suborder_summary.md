### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：按 GPU 需求主单汇总子单数据（按需求分类汇总卡数 / QPM，并带按月列）

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/summary

### 输入参数

| 参数名称   | 参数类型  | 必选 | 描述             |
|-----------|---------|------|-----------------|
| bk_biz_id | int64   | 是   | 业务ID（路径参数） |
| order_id  | string  | 是   | 需求主单 ID |
| statuses  | array   | 否   | 子单状态列表，多值 in；枚举：INIT / PENDING / DONE / REJECT / TERMINATE |
| demand_year | int   | 否   | 需求年份 |
| demand_month | int  | 否   | 需求月份（1-12） |
| demand_type | string | 否   | 需求分类 |

### 调用示例

#### 请求参数示例

```json
{
  "order_id": "0000001k",
  "statuses": ["DONE", "PENDING"],
  "demand_year": 2026,
  "demand_month": 3,
  "demand_type": "大语言模型训练-文生文"
}
```

#### 仅主单汇总

```json
{
  "order_id": "0000001k"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": "0000001k",
    "details": [
      {
        "demand_type": "大语言模型训练-文生文",
        "gpu_num": 128,
        "qpm_max": 0,
        "months": {
          "2026-03": 64,
          "2026-04": 64
        }
      },
      {
        "demand_type": "推理-API",
        "gpu_num": 0,
        "qpm_max": 1000,
        "months": {
          "2026-03": 1000
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
| data    | object | 响应数据                      |

#### data

| 参数名称 | 参数类型 | 描述 |
|---------|---------|------|
| order_id | string | 需求主单 ID |
| details | array | 按 demand_type 汇总的行 |

#### data.details[n]

| 参数名称 | 参数类型 | 描述 |
|---------|---------|------|
| demand_type | string | GPU 需求类别 |
| gpu_num | int64 | 该类别下子单 gpu_num 之和 |
| qpm_max | int64 | 该类别下子单 qpm_max 之和 |
| months | object | 按月度量，key 为 `YYYY-MM`，value 为该月 gpu_num 与 qpm_max 之和（二者通常互斥） |

### 说明

- 只读接口；不修改任何数据。
- 缺少 `order_id` 时返回参数错误。
- 无匹配子单时 `details` 为空数组。
- 聚合口径对齐海垒 GPU 需求详情页「数据汇总」语义（按 `demand_type` 分行，落库字段求和）。
