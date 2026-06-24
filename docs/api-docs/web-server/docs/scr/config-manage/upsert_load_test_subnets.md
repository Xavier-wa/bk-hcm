### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：全局配置创建权限。
- 该接口功能描述：创建或更新压测子网配置。该接口为全量替换语义，每次调用都会完整覆盖已有配置。

### URL

POST /api/v1/woa/config/load_test_subnets/upsert

### 输入参数

请求体为 region -> vpc_id -> [subnet_id] 三层映射对象。

| 参数名称   | 参数类型      | 必选 | 描述                                            |
|----------|--------------|------|------------------------------------------------|
| {region} | object       | 是   | 地域标识（如 ap-nanjing），其值为该地域下的 vpc_id -> 子网列表映射 |
| {vpc_id} | string array | 是   | VPC 标识（如 vpc-aaa111），其值为该 VPC 下的子网 ID 列表 |

### 调用示例

#### 请求参数示例

```json
{
  "ap-nanjing": {
    "vpc-aaa111": ["subnet-s22y2418", "subnet-pqnrqykq"],
    "vpc-bbb222": ["subnet-7g6ctd9i", "subnet-ncdnis5q"]
  },
  "ap-shanghai": {
    "vpc-ccc333": ["subnet-99t4p51x", "subnet-3cv8ulcl"]
  }
}
```

#### 清空配置请求参数示例

```json
{}
```

### 响应示例

#### 成功响应示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": null
}
```

### 响应参数说明

| 参数名称    | 参数类型 | 描述                                    |
|-----------|---------|----------------------------------------|
| result    | bool    | 请求成功与否。true:请求成功；false请求失败 |
| code      | int     | 错误编码。 0表示success，>0表示失败错误   |
| message   | string  | 请求失败返回的错误信息                    |
| data      | object  | 请求返回的数据，成功时为 null            |
