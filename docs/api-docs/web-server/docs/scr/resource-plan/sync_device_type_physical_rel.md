### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：平台-全局配置。
- 该接口功能描述：手动触发 CVM 机型与物理机机型族映射的全量同步。

### URL

POST /api/v1/woa/res_syncs/device_type_physical_rels/sync

### 输入参数

无

### 调用示例

```json
{}
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
| code    | int    | 错误编码。 0表示success，>0表示失败错误 |
| message | string | 请求失败返回的错误信息               |
| data    | object | 响应数据                      |

#### data

无
