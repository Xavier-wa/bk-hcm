### 描述

- 该接口提供版本：v1.9.2.0+。
- 该接口所需权限：无。
- 该接口功能描述：查询压测子网配置。

### URL

GET /api/v1/woa/config/load_test_subnets

### 输入参数

无

### 调用示例

#### 请求示例

```
GET /api/v1/woa/config/load_test_subnets
```

### 响应示例

#### 配置存在时成功响应示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "ap-nanjing": {
      "vpc-aaa111": ["subnet-s22y2418", "subnet-pqnrqykq"],
      "vpc-bbb222": ["subnet-7g6ctd9i", "subnet-ncdnis5q"]
    },
    "ap-shanghai": {
      "vpc-ccc333": ["subnet-99t4p51x", "subnet-3cv8ulcl"]
    }
  }
}
```

#### 配置不存在时成功响应示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 响应参数说明

| 参数名称    | 参数类型 | 描述                                    |
|-----------|---------|----------------------------------------|
| result    | bool    | 请求成功与否。true:请求成功；false请求失败 |
| code      | int     | 错误编码。 0表示success，>0表示失败错误   |
| message   | string  | 请求失败返回的错误信息                    |
| data      | object  | 压测子网映射，外层 key 为 region，中层 key 为 vpc_id，内层 value 为 subnet_id 列表 |

#### data

| 参数名称  | 参数类型 | 描述                                                          |
|---------|---------|--------------------------------------------------------------|
| {region} | object  | 地域标识（如 ap-nanjing），其值为该地域下的 vpc_id -> 子网列表映射 |

#### data.{region}

| 参数名称   | 参数类型      | 描述                                            |
|----------|--------------|------------------------------------------------|
| {vpc_id} | string array | VPC 标识（如 vpc-aaa111），其值为该 VPC 下的子网 ID 列表 |
