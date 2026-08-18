### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：业务访问。
- 该接口功能描述：资源回收设备列表查询（管理员视角，可跨业务筛选）。

### URL

POST /api/v1/woa/task/findmany/recycle/host

### 输入参数


| 参数名称         | 参数类型         | 必选  | 描述                                                                               |
| ------------ | ------------ | --- | -------------------------------------------------------------------------------- |
| bk_biz_id    | int array    | 是   | 业务 ID 列表                                                                         |
| order_id     | int array    | 否   | 资源回收单号列表，数量最大 20                                                                 |
| suborder_id  | string array | 否   | 资源回收子单号列表，数量最大 20                                                                |
| device_type  | string array | 否   | 机型列表，数量最大 20                                                                     |
| bk_zone_name | string array | 否   | 地域列表，数量最大 20                                                                     |
| sub_zone     | string array | 否   | 园区列表，数量最大 20                                                                     |
| stage        | string array | 否   | 回收阶段列表，数量最大 20。枚举值：COMMIT、DETECT、AUDIT、TRANSIT、RETURN、RETURN_PLAN、DONE、TERMINATE |
| status       | string array | 否   | 回收状态列表，数量最大 20                                                                   |
| recycle_type | string array | 否   | 回收类型列表，数量最大 20                                                                   |
| bk_username  | string array | 否   | 提单人列表，数量最大 20                                                                    |
| ip           | string array | 否   | 设备内网 IP 列表，数量最大 500                                                              |
| bk_asset_id  | string array | 否   | 固资号列表，数量最大 500                                                                   |
| return_start | string       | 否   | 完成时间筛选起点，格式 `YYYY-MM-DD`，对应 `return_time`；须与 `return_end` 成对传入                   |
| return_end   | string       | 否   | 完成时间筛选终点，格式 `YYYY-MM-DD`（含当天）；须与 `return_start` 成对传入                             |
| create_start | string       | 否   | 创建时间筛选起点，格式 `YYYY-MM-DD`，对应 `create_at`；须与 `create_end` 成对传入                     |
| create_end   | string       | 否   | 创建时间筛选终点，格式 `YYYY-MM-DD`（含当天）；须与 `create_start` 成对传入                             |
| page         | object       | 是   | 分页信息                                                                             |




#### page


| 参数名称         | 参数类型 | 必选  | 描述                  |
| ------------ | ---- | --- | ------------------- |
| start        | int  | 否   | 记录开始位置，start 起始值为 0 |
| limit        | int  | 是   | 每页限制条数，最大 5000      |
| enable_count | bool | 是   | 本次请求是否为获取数量还是详情的标记  |




### 调用示例



#### 获取详细信息请求参数示例

```json
{
  "bk_biz_id": [1234, 123],
  "suborder_id": ["1-1"],
  "return_start": "2024-01-01",
  "return_end": "2024-07-28",
  "page": {
    "start": 0,
    "limit": 20,
    "enable_count": false
  }
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
    "count": 1,
    "info": [
      {
        "order_id": 1,
        "suborder_id": "1-1",
        "bk_biz_id": 1234,
        "bk_biz_name": "xx",
        "bk_username": "xxx",
        "bk_asset_id": "112457896358",
        "ip": "10.0.0.1",
        "instance_id": "12548587",
        "device_type": "S5.4XLARGE64",
        "bk_zone_name": "深圳",
        "sub_zone": "深圳-光明",
        "module_name": "深圳-光明-M4",
        "operator": "xxx",
        "bak_operator": "xxx",
        "input_time": "2022-08-13 16:18:39",
        "status": "DONE",
        "return_id": "12345789547",
        "return_link": "https://xxxxxxxxx.xxx.com",
        "return_tag": "计划外弹性外采购设备退回",
        "return_cost_rate": 0.2,
        "return_plan_msg": "通过",
        "return_time": "2022-08-14 16:18:39",
        "create_at": "2022-08-13T15:04:05.004Z",
        "update_at": "2022-08-14T16:18:39.004Z"
      }
    ]
  }
}
```



### 响应参数说明


| 参数名称    | 参数类型   | 描述                           |
| ------- | ------ | ---------------------------- |
| result  | bool   | 请求成功与否。true: 请求成功；false 请求失败 |
| code    | int    | 错误编码。0 表示 success，>0 表示失败错误  |
| message | string | 请求失败返回的错误信息                  |
| data    | object | 响应数据                         |




#### data


| 参数名称  | 参数类型         | 描述             |
| ----- | ------------ | -------------- |
| count | int          | 当前规则能匹配到的总记录条数 |
| info  | object array | 回收设备列表         |




#### [data.info](http://data.info)


| 参数名称             | 参数类型      | 描述          |
| ---------------- | --------- | ----------- |
| order_id         | int       | 资源回收单号      |
| suborder_id      | string    | 资源回收子单号     |
| bk_biz_id        | int       | 业务 ID       |
| bk_biz_name      | string    | 业务名称        |
| bk_username      | string    | 提单人         |
| ip               | string    | 设备 IP       |
| asset_id         | string    | 设备固资号       |
| instance_id      | string    | 实例 ID       |
| device_type      | string    | 机型          |
| bk_zone_name     | string    | 地域          |
| sub_zone         | string    | 园区          |
| module_name      | string    | Module 名称   |
| operator         | string    | 维护人         |
| bak_operator     | string    | 备份维护人       |
| input_time       | string    | 入库时间        |
| status           | string    | 回收状态        |
| return_id        | string    | 退回单号        |
| return_link      | string    | 退回单链接       |
| return_tag       | string    | 退回标记        |
| return_cost_rate | float     | 成本分摊比例      |
| return_plan_msg  | string    | 返还预测校验信息    |
| return_time      | string    | 完成时间，未完成时为空 |
| create_at        | timestamp | 记录创建时间      |
| update_at        | timestamp | 记录更新时间      |


