### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：获取主机申领机型 Top-N 推荐，先取用户维度 Top-N，不足时从业务维度补足。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top

### 输入参数

| 参数名称       | 参数类型   | 必选 | 描述                              |
|------------|--------|----|---------------------------------|
| bk_biz_id  | int64  | 是  | 业务ID                            |
| bk_username | string | 是  | 申请人用户名                          |
| limit      | int    | 是  | 返回推荐条数，范围 [1, 20]               |

### 调用示例

```json
{
  "bk_username": "alice",
  "limit": 5
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "require_type": 1,
        "region": "ap-guangzhou",
        "device_type": "S5.LARGE8",
        "image_id": "img-xxxxxxxx",
        "count": 12,
        "source": "user"
      },
      {
        "require_type": 1,
        "region": "ap-beijing",
        "device_type": "M5.LARGE8",
        "image_id": "img-yyyyyyyy",
        "count": 5,
        "source": "biz"
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
| data    | object | 返回数据 |

**data 字段说明**

| 参数名称  | 参数类型  | 描述           |
|-------|-------|--------------|
| items | array | 推荐列表，可能为空数组 |

**items 元素字段说明**

| 参数名称         | 参数类型   | 描述                              |
|--------------|--------|---------------------------------|
| require_type | int    | 需求类型                            |
| region       | string | 地域                              |
| device_type  | string | 机型                              |
| image_id     | string | 镜像ID                     |
| count        | int    | 历史申领次数                          |
| source       | string | 推荐来源，`user` 表示用户维度，`biz` 表示业务维度 |
