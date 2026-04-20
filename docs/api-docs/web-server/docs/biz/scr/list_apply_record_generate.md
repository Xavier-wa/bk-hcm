### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：业务访问。
- 该接口功能描述：资源生产记录查询。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/find/apply/record/generate

### 输入参数

| 参数名称     | 参数类型  | 必选 | 描述         |
|-------------|---------|------|-------------|
| suborder_id | string  | 是   | 资源申请子单号 |
| filter      | object  | 是   | 查询过滤条件   |
| page        | object  | 是   | 分页设置      |

#### filter

| 参数名称 | 参数类型      | 必选 | 描述                                                                                          |
|---------|-------------|------|----------------------------------------------------------------------------------------------|
| op      | enum string | 是   | 操作符（枚举值：and、or）。如果是and，则表示多个rule之间是且的关系；如果是or，则表示多个rule之间是或的关系。 |
| rules   | array       | 是   | 过滤规则，最多设置5个rules。如果rules为空数组，op（操作符）将没有作用，代表查询全部数据。                 |

#### rules[n] （详情请看 rules 表达式说明）

| 参数名称 | 参数类型      | 必选 | 描述                                                              |
|---------|-------------|------|------------------------------------------------------------------|
| field   | string      | 是   | 查询条件Field名称，具体可使用的用于查询的字段及其说明请看下面 - 查询参数介绍 |
| op      | enum string | 是   | 操作符（枚举值：eq、neq、gt、gte、le、lte、in、nin、cs、cis）           |
| value   | 可变类型     | 是   | 查询条件Value值                                                     |

##### rules 表达式说明：

##### 1. 操作符

| 操作符 | 描述                                              | 操作符的value支持的数据类型                               |
|-------|--------------------------------------------------|--------------------------------------------------------|
| eq    | 等于。不能为空字符串                                | boolean, numeric, string                               |
| neq   | 不等。不能为空字符串                                | boolean, numeric, string                               |
| gt    | 大于                                             | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"）|
| gte   | 大于等于                                          | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"）|
| lt    | 小于                                             | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"）|
| lte   | 小于等于                                          | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"）|
| in    | 在给定的数组范围中。value数组中的元素最多设置100个，数组中至少有一个元素  | boolean, numeric, string                 |
| nin   | 不在给定的数组范围中。value数组中的元素最多设置100个，数组中至少有一个元素 | boolean, numeric, string                |
| cs    | 模糊查询，区分大小写                                | string                                                 |
| cis   | 模糊查询，不区分大小写                              | string                                                  |

#### page

| 参数名称 | 参数类型 | 必选 | 描述                                                                                                                                                                                                         |
|---------|--------|------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| count   | bool   | 否   | 是否返回总记录条数。 如果为true，查询结果返回总记录条数 count，但查询结果详情数据 details 为空数组，此时 start 和 limit 参数将无效，且必需设置为0。如果为false，则根据 start 和 limit 参数，返回查询结果详情数据，但总记录条数 count 为0 |
| start   | int    | 否   | 记录开始位置，start 起始值为0                                                                                                                                                                                   |
| limit   | int    | 否   | 每页限制条数，最大500，不能为0                                                                                                                                                                                   |
| sort    | string | 否   | 排序字段，返回数据将按该字段进行排序                                                                                                                                                                               |
| order   | string | 否   | 排序顺序（枚举值：ASC、DESC）                                                                                                                                                                                    |

**注意：**

- count 如果此标记为true，表示此次请求是获取数量。此时其余字段必须为初始化值，start为0,limit为:0。

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "filter": {
    "op": "and",
    "rules": [
      {
        "field": "name",
        "op": "eq",
        "value": "xxxx"
      }
    ]
  },
  "page": {
    "count": false,
    "start": 0,
    "limit": 500
  }
}
```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "result":true,
  "code":0,
  "message":"success",
  "data":{
    "count":1,
    "info":[
      {
        "suborder_id":"1001-1",
        "generate_id":"1",
        "generate_type":"CVM",
        "task_id":"YT000001",
        "task_link":"",
        "request_info":"{\"area\":\"ap-shanghai\",\"zone\":\"ap-shanghai-2\",\"instanceType\":\"S3.MEDIUM4\",\"appId\":\"931\",\"appModuleId\":29309,\"imageName\":\"Tencent Linux Release 1.2 (Final)\",\"dataDisk\":[],\"\operator\":\"xxx\",\"applyNumber\":3,\"noteInfo\":\"test\"}",
        "status":0,
        "message":"success",
        "total_num":3,
        "success_num":3,
        "success_list":[
          "10.0.0.1",
          "10.0.0.2",
          "10.0.0.3"
        ],
        "start_at":"2022-01-02T15:04:05.004Z",
        "end_at":"2022-01-02T15:04:05.004Z"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型       | 描述               |
|------------|--------------|--------------------|
| result     | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code       | int          | 错误编码。 0表示success，>0表示失败错误  |
| message    | string       | 请求失败返回的错误信息 |
| data	     | object array | 响应数据             |

#### data

| 参数名称 | 参数类型       | 描述                    |
|---------|--------------|-------------------------|
| count   | int          | 当前规则能匹配到的总记录条数 |
| info    | object array | 资源生产记录信息列表        |

#### data.info

| 参数名称       | 参数类型    | 描述          |
|---------------|-----------|---------------|
| suborder_id   | string    | 资源申请子单号  |
| generate_id	| string	| 生产记录ID     |
| generate_type	| string	| 生产类型       |
| task_id 	    | string	| 生产任务ID     |
| task_link	    | string	| 生产任务详情链接 |
| request_info	| string	| 生产任务请求信息 |
| status	    | int  	    | 生产记录状态码，0: 成功, 1: 执行中, 其他: 失败 |
| message	    | string	| 生产记录状态信息 |
| total_num	    | int	    | 下单生产总数    |
| success_num	| int	    | 成功生产数量    |
| success_list	| string array | 成功生产资源的IP列表 |
| start_at	    | timestamp	| 生产记录开始时间    |
| end_at	    | timestamp	| 生产记录结束时间    |
