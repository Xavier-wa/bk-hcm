### 描述

- 该接口提供版本：v1.8.11.1+。
- 该接口所需权限：无。
- 该接口功能描述：校验主机申领时继承套餐的主机信息。

### URL

POST /api/v1/woa/task/check/apply/order/host

### 输入参数

| 参数名称       | 参数类型 | 必选 | 描述                           |
|---------------|--------|------|-------------------------------|
| require_type  | int	 | 是	| 需求类型(3: 机房裁撤 6:滚服项目)  |
| bk_asset_id   | string | 是   | 主机固资号                      |
| region        | string | 是   | 机器所在的地域                   |

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "require_type": 3,
  "bk_asset_id": "TC123456",
  "region":"ap-beijing"
}
```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "device_type": "C3.2XLARGE16",
    "device_group": "标准型",
    "instance_charge_type": "PREPAID",
    "charge_months": "36",
    "billing_start_time": "2024-06-03 17:43:47",
    "old_billing_expire_time": "2027-06-03 17:43:47",
    "new_billing_expire_time": "2027-07-03 17:43:47",
    "bk_cloud_inst_id": "ins-xxxxxx",
    "generation_type": "存量"
  }
}
```

### 响应参数说明

| 参数名称 | 参数类型       | 描述                                  |
|---------|--------------|---------------------------------------|
| result  | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code    | int          | 错误编码。 0表示success，>0表示失败错误   |
| message | string       | 请求失败返回的错误信息                   |
| data	  | object array | 响应数据                               |

#### data

| 参数名称                 | 参数类型  | 描述                                                                                                                      |
|-------------------------|---------|---------------------------------------------------------------------------------------------------------------------------|
| device_type             | string	| 机型                                                                                                                      |
| device_group            | string	| 机型族                                                                                                                     |
| instance_charge_type    | string  | 实例计费模式。(PREPAID：表示预付费，即包年包月、POSTPAID_BY_HOUR：表示后付费，即按量计费、CDHPAID：专用宿主机付费，即只对专用宿主机计费，不对专用宿主机上的实例计费。、SPOTPAID：表示竞价实例付费。)。 |
| charge_months           | int     | 购买时长，单位：月，如果是按量计费类型，则这个字段没有值                                                                            |
| billing_start_time      | string  | 套餐计费起始时间                                                                                                             |
| old_billing_expire_time | string  | 被继承机器套餐计费过期时间                                                                                                     |
| new_billing_expire_time | string  | 新申请机器套餐计费过期时间                                                                                                     |
| bk_cloud_inst_id        | string  | 云主机实例ID                                                                                                                |
| generation_type         | string  | 机型代次                                                                                                                    |