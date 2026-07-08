### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：平台-机房裁撤管理。
- 该接口功能描述：更新裁撤主机信息。

### URL

PUT /api/v1/woa/dissolve/recycled_host/update

### 输入参数

| 参数名称         | 参数类型         | 必选 | 描述                                                                 |
|--------------|--------------|----|--------------------------------------------------------------------|
| id           | string	      | 是	 | 主机唯一标识                                                             |
| asset_id     | string	      | 否	 | 主机固资号                                                              |
| inner_ip     | string       | 否	 | 主机ip                                                               |
| device_type  | string       | 否	 | 机型                                                                 |
| module       | string       | 否	 | 机器所在的裁撤模块                                                          |
| abolish_phase | string       | 否	 | 裁撤阶段，枚举值：incomplete(裁撤未完成)/complete(裁撤完成)/bsiComplete(业务退回)/retain(保留暂不裁撤) |
| project_name | string       | 否	 | 裁撤项目名称                                                             |
| project_id   | int          | 否	 | 裁撤项目ID                                                             |
| region       | string       | 否	 | 地域ID                                                               |
| bk_biz_id    | int64        | 否	 | 业务ID                                                               |
| group_id     | int64        | 否	 | 组织ID                                                               |
| operators    | string array | 否	 | 负责人列表                                                              |
| cpu_core     | int          | 否	 | CPU核心数                                                             |
| is_ignore    | bool         | 否	 | 是否忽略该主机                                                            |

### 调用示例

```json
{
  "id": "1",
  "asset_id": "TC123456",
  "inner_ip": "127.0.0.1",
  "device_type": "S5.LARGE",
  "module": "深圳-锦绣-M12",
  "abolish_phase": "incomplete",
  "project_name": "2024年裁撤项目",
  "project_id": 1001,
  "region": "ap-guangzhou",
  "bk_biz_id": 100,
  "group_id": 200,
  "operators": ["zhangsan", "lisi"],
  "cpu_core": 8,
  "is_ignore": false
}
```

### 响应示例

```json
{
  "result":true,
  "code":0,
  "message":"success",
  "data": null
}
```

### 响应参数说明

| 参数名称    | 参数类型      | 描述               |
|------------|-------------|--------------------|
| result     | bool        | 请求成功与否。true:请求成功；false请求失败 |
| code       | int         | 错误编码。 0表示success，>0表示失败错误  |
| message    | string      | 请求失败返回的错误信息 |
| data	     | object      | 响应数据             |
