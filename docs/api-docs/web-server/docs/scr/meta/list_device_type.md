### 描述

- 该接口提供版本：v1.5.1+。
- 该接口所需权限：无。
- 该接口功能描述：查询机型规格列表。

### URL

POST /api/v1/woa/meta/device_type/list

### 输入参数

| 参数名称           | 参数类型         | 必选 | 描述               |
|----------------|--------------|----|------------------|
| device_classes | string array | 否  | 查询机型类型列表，不传时查询全部 |
| device_types   | string array | 否  | 查询设备类型列表，不传时查询全部 |

### 调用示例

```json
{
  "device_classes": [
    "高IO型IT5",
    "标准型S5"
  ],
  "device_types": [
    "SA5.LARGE8"
  ]
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "details": [
      {
        "device_type": "S5.2XLARGE16",
        "core_type": "大核心",
        "cpu_core": 123,
        "memory": 123,
        "gpu_amount": 0,
        "device_class": "标准型SA5",
        "device_family": "标准型"
      },
      {
        "device_type": "IT5.8XLARGE128",
        "core_type": "小核心",
        "cpu_core": 123,
        "memory": 123,
        "gpu_amount": 0,
        "device_class": "标准型SA5",
        "device_family": "标准型"
      },
      {
        "device_type": "PNV5b.12XLARGE192",
        "core_type": "大核心",
        "cpu_core": 48,
        "memory": 192,
        "gpu_amount": 0.25,
        "device_class": "GPU计算型PNV5b",
        "device_family": "GPU型"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int    | 状态码  |
| message | string | 请求信息 |
| data    | object | 响应数据 |

#### data

| 参数名称    | 参数类型         | 描述       |
|---------|--------------|----------|
| details | object array | 机型规格信息列表 |

#### data.details[n]

| 参数名称          | 参数类型   | 描述          |
|---------------|--------|-------------|
| device_type   | string | 机型规格        |
| core_type     | string | 核心类型        |
| cpu_core      | int    | CPU核心数，单位：核 |
| memory        | int    | 内存大小，单位：GB  |
| gpu_amount    | float  | GPU卡数       |
| device_class  | string | 机型分类        |
| device_family | string | 机型族         |
