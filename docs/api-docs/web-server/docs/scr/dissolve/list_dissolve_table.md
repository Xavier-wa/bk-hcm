### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：服务-机房裁撤。
- 该接口功能描述：查询整体裁撤表格信息。

### URL

POST /api/v1/woa/dissolve/table/list

### 输入参数

| 参数名称       | 参数类型        | 必选 | 描述      |
|------------|-------------|----|---------|
| project_ids | int array   | 否  | 裁撤项目 ID |
| group_ids  | int64 array | 否  | 组织 ID   |
| bk_biz_ids | int64 array | 否  | 业务 ID   |
| operators  | string array | 否 | 负责人     |
| regions    | string array | 否 | 地域 ID   |

### 调用示例

查询组织 ID 为 1111、业务 ID 为 100、负责人为 test、地域为 ap-guangzhou 的裁撤总览信息。

```json
{
  "project_ids": [1, 2],
  "group_ids": [1111],
  "bk_biz_ids": [100],
  "operators": ["test"],
  "regions": ["ap-guangzhou"]
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "items": [
      {
        "bk_biz_id": 100,
        "origin_host_count": 8,
        "origin_cpu_core": 640,
        "current_host_count": 0,
        "current_cpu_core": 0,
        "delivered_cpu_core": 100,
        "progress": "100.00%"
      },
      {
        "bk_biz_id": -1,
        "origin_host_count": 8,
        "origin_cpu_core": 640,
        "current_host_count": 0,
        "current_cpu_core": 0,
        "delivered_cpu_core": 100,
        "progress": "100.00%"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int32  | 状态码  |
| message | string | 请求信息 |
| data    | object | 响应数据 |

#### data

| 参数名称  | 参数类型  | 描述                          |
|-------|-------|-----------------------------|
| items | array | 业务裁撤进度数据，最后一行为「合计」行         |

#### data.items[n]

| 参数名称               | 参数类型   | 描述                                   |
|--------------------|--------|--------------------------------------|
| bk_biz_id          | int64  | 业务 ID；合计行该字段为 -1, 主机对应的业务未知时，业务id为0  |
| origin_host_count  | int64  | 原始裁撤设备数（命中条件的全部记录）                   |
| origin_cpu_core    | int64  | 原始裁撤 CPU 总核数                         |
| current_host_count | int64  | 当前裁撤设备数（`abolish_phase != complete`） |
| current_cpu_core   | int64  | 当前裁撤 CPU 总核数                         |
| delivered_cpu_core | int64  | 已申领 CPU 核数                           |
| progress           | string | 裁撤进度 = (原始设备数 − 当前设备数) / 原始设备数       |
