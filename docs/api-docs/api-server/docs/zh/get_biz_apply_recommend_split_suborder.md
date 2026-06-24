### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：将一个已确定的机型方案，结合实时预测余量与库存，按计费模式拆分组装成 1 个主机申请单据（主单）含多个子单。预测内余量满足的台数组装为包年包月（PREPAID）子单，预测内装不下、溢出预测外余量的台数组装为按量计费（POSTPAID_BY_HOUR）子单；一个入参组合最多拆 2 个子单。申请数量尽量满足，无法满足时只要可分配总量 ≥ 1 即返回部分子单。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/split_suborder

### 路径参数

| 参数名称    | 参数类型 | 必选 | 描述   |
|------------|--------|------|--------|
| bk_biz_id  | int    | 是   | 业务ID |

### 输入参数

| 参数名称     | 参数类型      | 必选 | 描述                                                                 |
|-------------|-------------|------|--------------------------------------------------------------------|
| require_type | int        | 是   | 需求类型：1-常规项目 / 2-春节保障 / 3-机房裁撤 / 6-滚服项目 / 7-小额绿通 / 8-春保资源池 / 9-短租项目 |
| region      | string      | 是   | 地域，如 `ap-beijing`，最大长度 128                                       |
| zone        | string      | 是   | 可用区，`all` 表示全部可用区，或具体可用区如 `ap-beijing-1`，最大长度 64           |
| device_type | string      | 是   | 机型，如 `S5.MEDIUM4`，最大长度 64                                         |
| image_id    | string      | 是   | 镜像ID，最大长度 64                                                      |
| res_assign  | int         | 是   | 资源分配方式：1-有资源区域优先 / 2-分Campus生产                              |
| replicas    | int         | 是   | 申请数量，最小值 1                                                        |
| system_disk | object      | 是   | 系统盘规格                                                              |
| data_disk   | object array | 否  | 数据盘规格列表                                                           |
| occupied_suborders | object array | 否 | 已占用子单数组，用于增量拆分；每个元素 `require_type` 必须等于本次请求的 `require_type` |

#### system_disk / data_disk[n] / occupied_suborders[n]

system_disk 与 data_disk[n] 结构同响应中的磁盘结构；occupied_suborders[n] 结构同响应中的子单结构（扣减仅使用 require_type / region / zone / device_type / charge_type / replicas）。

### 调用示例

#### 请求参数示例

```json
{
  "require_type": 1,
  "region": "ap-beijing",
  "zone": "all",
  "device_type": "S5.MEDIUM4",
  "image_id": "img-xxxxxxxx",
  "res_assign": 1,
  "replicas": 20,
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
  ],
  "occupied_suborders": [
    {
      "require_type": 1,
      "region": "ap-beijing",
      "zone": "all",
      "device_type": "S5.MEDIUM4",
      "charge_type": "PREPAID",
      "replicas": 5
    }
  ]
}
```

### 响应示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "suborders": [
      {
        "require_type": 1,
        "region": "ap-beijing",
        "zone": "all",
        "device_type": "S5.MEDIUM4",
        "image_id": "img-xxxxxxxx",
        "res_assign": 1,
        "replicas": 11,
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
      },
      {
        "require_type": 1,
        "region": "ap-beijing",
        "zone": "all",
        "device_type": "S5.MEDIUM4",
        "image_id": "img-xxxxxxxx",
        "res_assign": 1,
        "replicas": 9,
        "charge_type": "POSTPAID_BY_HOUR",
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

| 参数名称   | 参数类型        | 描述                                          |
|-----------|---------------|-----------------------------------------------|
| suborders | object array  | 拆分后的子单列表（增量场景仅含本次新算出的增量子单）；可分配总量 < 1 时为空数组 |

#### data.suborders[n]

| 参数名称    | 参数类型       | 描述                                                                                         |
|------------|--------------|----------------------------------------------------------------------------------------------|
| require_type | int        | 需求类型：1-常规项目 / 2-春节保障 / 3-机房裁撤 / 6-滚服项目 / 7-小额绿通 / 8-春保资源池 / 9-短租项目 |
| region     | string       | 地域，原样透传入参                                                                              |
| zone       | string       | 可用区，原样透传入参（`all` 或具体可用区）                                                        |
| device_type | string      | 机型，原样透传入参                                                                              |
| image_id   | string       | 镜像ID，原样透传入参                                                                            |
| res_assign | int          | 资源分配方式，原样透传入参                                                                       |
| replicas   | int          | 该子单拆分后实际分配台数，所有子单合计 ≤ 入参 `replicas`                                          |
| charge_type | string      | 计费模式：`PREPAID`-包年包月（预测内 / 不校验预测类型）/ `POSTPAID_BY_HOUR`-按量计费（预测外）        |
| system_disk | object      | 系统盘规格，原样透传入参                                                                         |
| data_disk  | object array | 数据盘规格列表，原样透传入参                                                                      |

#### data.suborders[n].system_disk / data_disk[n]

| 参数名称  | 参数类型 | 描述                                                            |
|----------|---------|------------------------------------------------------------------|
| disk_type | string | 磁盘类型：`CLOUD_PREMIUM`-高性能云硬盘 / `CLOUD_SSD`-SSD云硬盘   |
| disk_size | int    | 磁盘大小，单位 GB                                                 |
| disk_num  | int    | 磁盘数量                                                         |
