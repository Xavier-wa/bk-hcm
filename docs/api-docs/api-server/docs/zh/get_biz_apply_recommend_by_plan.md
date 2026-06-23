### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：基于实时预测余量叠加库存校验，为业务返回在线推荐方案列表。预测内有余量时计费模式为包年包月（PREPAID），所有地域预测内均无满足门槛的候选时整体回退预测外，计费模式转为按量计费（POSTPAID_BY_HOUR）。仅保留满足余量门槛（预测余量 ≥ 申请数量 × 机型核数）且库存充足（capacity ≥ 申请数量）的候选项，按预测余量倒序返回。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/by_plan

### 路径参数

| 参数名称    | 参数类型 | 必选 | 描述   |
|------------|--------|------|--------|
| bk_biz_id  | int    | 是   | 业务ID |

### 输入参数

| 参数名称     | 参数类型 | 必选 | 描述                                                                 |
|-------------|--------|------|--------------------------------------------------------------------|
| limit       | int    | 是   | 返回推荐方案数量上限，范围 [1, 20]                                              |
| require_type | int   | 否   | 需求类型：1-常规项目 / 2-春节保障 / 3-机房裁撤 / 6-滚服项目 / 7-小额绿通 / 8-春保资源池 / 9-短租项目；未传默认 1-常规项目 |
| region      | string | 否   | 地域，如 `ap-beijing`，最大长度 128；未传则遍历预测池内全部地域                       |
| device_type | string | 否   | 机型，如 `S5.MEDIUM4`，最大长度 64                                          |
| image_id    | string | 否   | 镜像ID，最大长度 64                                                       |
| zone        | string | 否   | 可用区，如 `ap-beijing-1`；指定后 `res_assign` 不返回，最大长度 64                  |
| res_assign  | int    | 否   | 资源分配方式：1-有资源区域优先 / 2-分Campus生产；未传默认为 1，指定具体 `zone` 时不适用            |
| replicas    | int    | 否   | 申请数量；未传则使用服务端配置的默认申请数量，最小值 1                                       |

### 调用示例

#### 请求参数示例

```json
{
  "limit": 3,
  "require_type": 1,
  "region": "ap-beijing",
  "replicas": 10
}
```

### 响应示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "suborder": {
          "require_type": 1,
          "region": "ap-beijing",
          "zone": "all",
          "device_type": "S5.MEDIUM4",
          "image_id": "img-xxxxxxxx",
          "res_assign": 1,
          "replicas": 10,
          "charge_type": "PREPAID",
          "system_disk": {
            "disk_type": "CLOUD_PREMIUM",
            "disk_size": 100,
            "disk_num": 1
          },
          "data_disk": [
            {
              "disk_type": "CLOUD_PREMIUM",
              "disk_size": 500,
              "disk_num": 1
            }
          ]
        }
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称 | 参数类型 | 描述                                    |
|---------|---------|------------------------------------------|
| result  | bool    | 请求成功与否。true: 请求成功；false: 请求失败 |
| code    | int     | 错误编码。0 表示 success，>0 表示失败错误   |
| message | string  | 请求失败返回的错误信息                     |
| data    | object  | 响应数据                                 |

#### data

| 参数名称 | 参数类型        | 描述         |
|---------|---------------|--------------|
| items   | object array  | 推荐方案列表，按预测余量倒序  |

#### items[n]

| 参数名称 | 参数类型 | 描述                                                    |
|---------|---------|----------------------------------------------------------|
| suborder | object | 推荐子单信息                                              |

#### items[n].suborder

| 参数名称    | 参数类型       | 描述                                                                                         |
|------------|--------------|----------------------------------------------------------------------------------------------|
| require_type | int        | 需求类型：1-常规项目 / 2-春节保障 / 3-机房裁撤 / 6-滚服项目 / 7-小额绿通 / 8-春保资源池 / 9-短租项目 |
| region     | string       | 地域                                                                                          |
| zone       | string       | 可用区；未传 `zone` 入参时值为 `"all"`（表示不限可用区）                                          |
| device_type | string      | 机型                                                                                          |
| image_id   | string       | 镜像ID；优先取入参，未传时回查历史子单最近一次使用的镜像，无历史时取默认镜像                          |
| res_assign | int          | 资源分配方式：1-有资源区域优先 / 2-分Campus生产；指定具体 `zone` 时不返回该字段                     |
| replicas   | int          | 申请数量                                                                                       |
| charge_type | string      | 计费模式：`PREPAID`-包年包月（命中预测内）/ `POSTPAID_BY_HOUR`-按量计费（回退预测外）               |
| system_disk | object      | 系统盘规格                                                                                     |
| data_disk  | object array | 数据盘规格列表                                                                                  |

#### items[n].suborder.system_disk / data_disk[n]

| 参数名称  | 参数类型 | 描述                                                            |
|----------|---------|------------------------------------------------------------------|
| disk_type | string | 磁盘类型：`CLOUD_PREMIUM`-高性能云硬盘 / `CLOUD_SSD`-SSD云硬盘   |
| disk_size | int    | 磁盘大小，单位 GB                                                 |
| disk_num  | int    | 磁盘数量                                                         |
