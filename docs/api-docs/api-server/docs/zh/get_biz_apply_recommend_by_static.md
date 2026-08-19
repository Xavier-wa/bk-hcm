### 描述

- 该接口提供版本：v1.9.2.0+。
- 该接口所需权限：业务访问。
- 该接口功能描述：基于离线静态偏好推荐叠加库存校验，为业务返回在线推荐方案列表。优先使用用户维度的静态推荐记录，不足时以业务维度推荐记录补足，仅保留库存充足（capacity ≥ 申请数量）的候选项。
- 滚服项目（`require_type=6`）候选会额外按机型反查机型族，自动匹配一台可继承的固资并做额度预检；匹配不到可继承固资或额度不足的候选会被丢弃，不出现在推荐列表中。命中的固资信息会回填到子单的 `bk_asset_id`、`inherit_instance_id`、`charge_months`、`billing_start_time`、`billing_expire_time` 字段，且子单的 `charge_type` 继承自该固资的计费模式。非滚服候选不会返回上述字段。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/by_static_recommend

### 路径参数

| 参数名称    | 参数类型 | 必选 | 描述   |
|------------|--------|------|--------|
| bk_biz_id  | int    | 是   | 业务ID |

### 输入参数

| 参数名称     | 参数类型 | 必选 | 描述                                                                 |
|-------------|--------|------|--------------------------------------------------------------------|
| bk_username | string | 是   | 申请人用户名，最大长度 64                                                     |
| limit       | int    | 是   | 返回推荐方案数量上限，范围 [1, 20]                                              |
| require_type | int   | 否   | 需求类型：1-常规项目 / 2-春节保障 / 3-机房裁撤 / 6-滚服项目 / 7-小额绿通 / 8-春保资源池 / 9-短租项目 |
| region      | string | 否   | 地域，如 `ap-beijing`，最大长度 128                                         |
| device_type | string | 否   | 机型，如 `S5.MEDIUM4`，最大长度 64                                          |
| image_id    | string | 否   | 镜像ID，最大长度 64                                                       |
| zone        | string | 否   | 可用区，如 `ap-beijing-1`；指定后 `res_assign` 不返回，最大长度 64                  |
| res_assign  | int    | 否   | 资源分配方式：1-有资源区域优先 / 2-分Campus生产；未传默认为 1，指定具体 `zone` 时不适用            |
| replicas    | int    | 否   | 申请数量；未传则使用服务端配置的默认申请数量，最小值 1                                       |

### 调用示例

#### 请求参数示例

```json
{
  "bk_username": "zhangsan",
  "limit": 3,
  "require_type": 1,
  "region": "ap-beijing",
  "zone": "ap-beijing-1",
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
        "source": "user",
        "suborder": {
          "require_type": 1,
          "region": "ap-beijing",
          "zone": "ap-beijing-1",
          "device_type": "S5.MEDIUM4",
          "image_id": "img-xxxxxxxx",
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
      },
      {
        "source": "biz",
        "suborder": {
          "require_type": 1,
          "region": "ap-beijing",
          "zone": "ap-beijing-1",
          "device_type": "S5.LARGE8",
          "image_id": "img-yyyyyyyy",
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
      },
      {
        "source": "biz",
        "suborder": {
          "require_type": 6,
          "region": "ap-beijing",
          "zone": "ap-beijing-1",
          "device_type": "S5.2XLARGE16",
          "image_id": "img-zzzzzzzz",
          "replicas": 10,
          "charge_type": "PREPAID",
          "charge_months": 10,
          "bk_asset_id": "TC1111111",
          "inherit_instance_id": "ins-1111111",
          "billing_start_time": "2024-11-20T10:15:30+08:00",
          "billing_expire_time": "2027-05-20T10:15:30+08:00",
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
| items   | object array  | 推荐方案列表  |

#### items[n]

| 参数名称 | 参数类型 | 描述                                                    |
|---------|---------|----------------------------------------------------------|
| source  | string  | 推荐来源：`user`-用户维度 / `biz`-业务维度                 |
| suborder | object | 推荐子单信息                                              |

#### items[n].suborder

| 参数名称    | 参数类型       | 描述                                                                                         |
|------------|--------------|----------------------------------------------------------------------------------------------|
| require_type | int        | 需求类型：1-常规项目 / 2-春节保障 / 3-机房裁撤 / 6-滚服项目 / 7-小额绿通 / 8-春保资源池 / 9-短租项目 |
| region     | string       | 地域                                                                                          |
| zone       | string       | 可用区；未传 `zone` 入参时值为 `"all"`（表示不限可用区）                                          |
| device_type | string      | 机型                                                                                          |
| image_id   | string       | 镜像ID                                                                                        |
| res_assign | int          | 资源分配方式：1-有资源区域优先 / 2-分Campus生产；指定具体 `zone` 时不返回该字段                     |
| replicas   | int          | 申请数量                                                                                       |
| charge_type | string      | 计费模式：`PREPAID`-包年包月 / `POSTPAID_BY_HOUR`-按量计费；非滚服候选固定为 `PREPAID`，滚服候选取自命中固资的计费模式 |
| system_disk | object      | 系统盘规格                                                                                     |
| data_disk  | object array | 数据盘规格列表                                                                                  |
| inherit_instance_id | string | 被继承的云主机实例 ID，即命中固资的云主机实例 ID；仅滚服候选返回                             |
| bk_asset_id | string      | 继承固资号；仅滚服候选返回                                                                       |
| charge_months | int       | 购买时长，单位月，取命中固资的剩余月数；仅滚服候选返回，按量计费固资算不出正值时不返回该字段          |
| billing_start_time | string | 继承固资的套餐计费起始时间，RFC3339 格式；仅滚服候选返回，固资无该时间时不返回该字段              |
| billing_expire_time | string | 继承固资的套餐计费到期时间，RFC3339 格式；仅滚服候选返回，按量计费固资无到期时间时不返回该字段     |

#### items[n].suborder.system_disk / data_disk[n]

| 参数名称  | 参数类型 | 描述                                                            |
|----------|---------|------------------------------------------------------------------|
| disk_type | string | 磁盘类型：`CLOUD_PREMIUM`-高性能云硬盘 / `CLOUD_SSD`-SSD云硬盘   |
| disk_size | int    | 磁盘大小，单位 GB                                                 |
| disk_num  | int    | 磁盘数量                                                         |
