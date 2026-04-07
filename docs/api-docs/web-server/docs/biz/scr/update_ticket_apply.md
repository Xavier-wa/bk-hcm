### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：业务-IaaS资源创建。
- 该接口功能描述：修改资源申请单据。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/modify/apply

### 输入参数

| 参数名称       | 参数类型       | 必选 | 描述             |
|---------------|--------------|------|-----------------|
| suborder_id   | string	   | 是	  | 资源申请子单号     |
| bk_username   | string       | 是	  | 资源申请提单人     |
| replicas      | int          | 是	  | 剩余生产数量      |
| remark	    | string       | 否	  | 备注             |
| spec	        | object       | 是   | 资源需求声明       |

#### spec
| 参数名称       | 参数类型        | 必选 | 描述          |
|---------------|---------------|------|--------------|
| region        | string	    | 是   | 地域          |
| zone          | string	    | 否   | 可用区（跟zones参数，需要传其中一个，该参数即将废弃）|
| device_type   | string	    | 是   | 机型          |
| image_id      | string        | 是   | 镜像ID        |
| disk_size     | int           | 是   | 数据盘磁盘大小，单位G |
| disk_type	    | string	    | 是   | 数据盘磁盘类型。"CLOUD_SSD": SSD云硬盘, "CLOUD_PREMIUM": 高性能云盘 |
| network_type  | string	    | 是   | 网络类型。"ONETHOUSAND": 千兆, "TENTHOUSAND": 万兆 |
| vpc	        | string        | 否   | 私有网络，默认为空 |
| subnet        | string        | 否   | 私有子网，默认为空 |
| zones         | string array  | 否   | 多可用区(选“全部”时传all)                             |
| res_assign    | int           | 否   | 资源分配方式(1表示“有资源区域优先”、2表示“分Campus生产”)  |
| bk_asset_id   | string        | 否   | 固资号，"滚服项目"、"机房裁撤"继承的固资号               |

### 调用示例

#### 获取详细信息请求参数示例

#### CVM申请示例
```json
{
  "suborder_id":"1001-1",
  "bk_username":"xx",
  "replicas":10,
  "spec":{
    "region":"ap-shanghai",
    "device_type":"S3.LARGE8",
    "image_id":"img-r5igp4bv",
    "disk_size":200,
    "disk_type":"CLOUD_PREMIUM",
    "network_type":"TENTHOUSAND",
    "vpc":"",
    "subnet":"",
    "zones": [
      "ap-nanjing-1",
      "ap-nanjing-2"
    ],
    "res_assign": 1
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
  "data":null
}
```

### 响应参数说明

| 参数名称    | 参数类型       | 描述               |
|------------|--------------|--------------------|
| result     | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code       | int          | 错误编码。 0表示success，>0表示失败错误  |
| message    | string       | 请求失败返回的错误信息 |
| data	     | object       | 响应数据             |
