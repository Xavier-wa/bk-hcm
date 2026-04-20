### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：业务访问。
- 该接口功能描述：资源申请改单记录查询。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/find/apply/record/modify

### 输入参数

| 参数名称      | 参数类型       | 必选 | 描述          |
|--------------|--------------|------|--------------|
| suborder_id  | string	array | 是	 | 资源申请子单号 |
| status       | int	array | 否	 | 资源变更状态（0:待审批 1:审批通过 2:审批失败 3:审批拒绝 4:审批超时） |

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "suborder_id":["1001-1"],
  "status": [1]
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
        "id":"1",
        "suborder_id":"1001-1",
        "bk_username":"xxx",
        "details":{
          "pre_data":{
            "replicas":4,
            "region":"ap-shanghai",
            "zone":"ap-shanghai-5",
            "device_type":"SA3.4XLARGE64",
            "image_id":"img-evitcbqz",
            "disk_size":300,
            "disk_type":"CLOUD_SSD",
            "network_type":"TENTHOUSAND",
            "vpc":"",
            "subnet":"",
            "system_disk": {
              "disk_type": "CLOUD_PREMIUM",
              "disk_size": 100,
              "disk_num": 1,
            },
            "data_disk": [{
              "disk_type": "CLOUD_PREMIUM",
              "disk_size": 100,
              "disk_num": 1,
            }],
            "zones": [
              "ap-nanjing-1",
              "ap-nanjing-2"
            ],
            "res_assign": 1
          },
          "cur_data":{
            "total_num":4,
            "replicas":1,
            "region":"ap-shanghai",
            "zone":"ap-shanghai-4",
            "device_type":"SA3.4XLARGE64",
            "image_id":"img-evitcbqz",
            "disk_size":300,
            "disk_type":"CLOUD_SSD",
            "network_type":"TENTHOUSAND",
            "vpc":"",
            "subnet":"",
            "system_disk": {
              "disk_type": "CLOUD_PREMIUM",
              "disk_size": 100,
              "disk_num": 1,
            },
            "data_disk": [{
              "disk_type": "CLOUD_PREMIUM",
              "disk_size": 100,
              "disk_num": 1,
            }],
            "zones": [
              "ap-nanjing-1",
              "ap-nanjing-2"
            ],
            "res_assign": 1
          }
        },
        "create_at":"2022-10-15 18:07:37",
        "status": 1
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

| 参数名称  | 参数类型       | 描述              |
|----------|--------------|-------------------|
| count    | int	      | 记录条数           |
| info	   | object array | 资源申请改单记录列表 |

#### data.info
| 参数名称      | 参数类型   | 描述           |
|-------------|-----------|----------------|
| id	      | string	  | 记录ID         |
| suborder_id | string	  | 资源申请子单号   |
| bk_username | string	  | 改单操作人      |
| details	  | object	  | 改单详情        |
| create_at	  | timestamp | 记录创建时间     |
| status 	  | int       | 资源变更状态（0:待审批 1:审批通过 2:审批失败 3:审批拒绝 4:审批超时）|

#### data.info.details
| 参数名称     | 参数类型   | 描述            |
|-------------|----------|-----------------|
| pre_data	  | object	 | 变更前申请单据信息 |
| cur_data	  | object	 | 变更后申请单据信息 |

#### data.info.details.pre_data && cur_data
| 参数名称         | 参数类型            | 描述                 |
|-----------------|-------------------|----------------------|
| total_num       | int               | 需要交付的需求数量      |
| replicas        | int               | 用户输入的剩余可申请数量 |
| region          | string	          | 地域                  |
| zone            | string	          | 可用区                |
| device_type     | string	          | 机型                  |
| image_id        | string            | 镜像ID                |
| disk_size       | int               | 数据盘磁盘大小，单位G（已废弃，优先使用data_disk字段） |
| disk_type	      | string	          | 数据盘磁盘类型。"CLOUD_SSD": SSD云硬盘, "CLOUD_PREMIUM": 高性能云盘（已废弃，优先使用data_disk字段）|
| network_type    | string	          | 网络类型。"ONETHOUSAND": 千兆, "TENTHOUSAND": 万兆 |
| vpc	          | string            | 私有网络，默认为空       |
| subnet          | string            | 私有子网，默认为空       |
| system_disk     | DiskObject        | 系统盘，磁盘大小：50G-1000G且为50的倍数（IT类型默认本地盘、50G；其他类型默认高性能云盘、100G） |
| data_disk       | array DiskObject  | 数据盘，支持多块硬盘，磁盘大小：20G-32000G且为10的倍数，数据盘数量总和不能超过20块 |
| zones           | string array      | 多可用区(选“全部”时该字段是空数组)                      |
| res_assign      | int               | 资源分配方式(1表示“有资源区域优先”、2表示“分Campus生产”)  |

#### spec for DiskObject
| 参数名称   | 参数类型  | 必选 | 描述                                                      |
|-----------|---------|------|----------------------------------------------------------|
| disk_type | string  | 是   | 磁盘类型，"CLOUD_SSD": SSD云硬盘, "CLOUD_PREMIUM": 高性能云盘 |
| disk_size | int     | 是   | 磁盘大小，单位G                                             |
| disk_num  | int     | 否   | 磁盘数量，所有磁盘数量之和不能超过20块，默认1块                  |
