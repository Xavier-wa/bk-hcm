### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：无。
- 该接口功能描述：获取资源详细信息。

### URL

POST /api/v1/woa/task/find/apply/record/deliver

### 输入参数

| 参数名称     | 参数类型  | 必选 | 描述         |
|-------------|---------|------|-------------|
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
  "filter":{
    "condition":"AND",
    "rules":[
      {
        "field":"require_type",
        "operator":"equal",
        "value":1
      },
      {
        "field":"region",
        "operator":"equal",
        "value":"ap-shanghai"
      },
      {
        "field":"zone",
        "operator":"equal",
        "value":"ap-shanghai-2"
      },
      {
        "field":"device_type",
        "operator":"equal",
        "value":"S3ne.LARGE8"
      },
      {
        "field":"label.device_group",
        "operator":"equal",
        "value":"标准型"
      }
    ]
  },
  "page":{
    "start":0,
    "limit":100,
    "sort":"region"
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
        "require_type": 1,
        "region":"ap-shanghai",
        "zone":"ap-shanghai-2",
        "device_type":"S3ne.LARGE8",
        "cpu": 4,
        "mem": 8,
        "disk": 100,
        "label":{
          "device_group":"标准型"
        },
        "capacity_flag": 2
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
| info    | object array | 资源容量信息详情列表       |

#### data.info

| 参数名称       | 参数类型 | 描述          |
|---------------|--------|---------------|
| require_type	| int	 | 需求类型 |
| region	    | string | 地域    |
| zone	        | string | 可用区  |
| device_type	| string | 设备型号 |
| cpu	        | int	 | CPU核数，单位个 |
| mem	        | int	 | 内存大小，单位G |
| disk	        | int	 | 磁盘大小，单位G |
| label         | string | 实例族，当前支持的实例族：标准型、高IO型、大数据型、计算型 |
| capacity_flag	| int	 | 容量标识。1: "库存充足", 2: "少量库存", 3: "库存紧张", 4: "无库存" |

#### data.info.label

| 参数名称      | 参数类型   | 描述        |
|--------------|----------|-------------|
| device_group | string   | 实例族，当前支持的实例族：GAMESERVER, DBSERVICE, HIGHFREQ |
