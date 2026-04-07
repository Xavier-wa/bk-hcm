### 描述

- 该接口提供版本：v1.6.11+。
- 该接口所需权限：无。
- 该接口功能描述：资源预测需求校验。

### URL

POST /api/v1/woa/plans/resources/demands/verify

### 输入参数

| 参数名称         | 参数类型         | 必选 | 描述                             |
|--------------|--------------|----|--------------------------------|
| bk_biz_id    | int	         | 是	 | CC业务ID                         |
| require_type | int	         | 是	 | 需求类型。1: 常规项目; 2: 春节保障; 3: 机房裁撤 |
| suborders	   | object array | 是  | 资源申请子需求单信息                     |

#### suborders

| 参数名称                | 参数类型   | 必选 | 描述                                                                                                        |
|---------------------|--------|----|-----------------------------------------------------------------------------------------------------------|
| resource_type	      | string | 是	 | 需求资源类型。"QCLOUDCVM": 腾讯云虚拟机, "IDCPM": IDC物理机, "QCLOUDDVM": Qcloud富容器, "IDCDVM": IDC富容器                     |
| replicas		          | int	   | 是	 | 需求资源数量                                                                                                    |
| anti_affinity_level | string | 否	 | 反亲和策略，默认值为"ANTI_NONE"。 "ANTI_NONE": 无要求, "ANTI_CAMPUS": 分Campus, "ANTI_MODULE": 分Module, "ANTI_RACK": 分机架 |
| remark	             | string | 否	 | 备注                                                                                                        |
| spec	               | object | 是	 | 资源需求声明                                                                                                    |

#### spec for QCLOUDCVM

| 参数名称              | 参数类型  | 必选 | 描述                                                  |
|---------------------|----------|-----|-----------------------------------------------------|
| region              | string   | 是  | 地域                                                  |
| zone                | string   | 是  | 可用区                                                 |
| device_type         | string   | 是  | 机型                                                  |
| image_id            | string   | 是  | 镜像ID                                                |
| disk_size           | int      | 是  | 数据盘磁盘大小，单位G                                         |
| disk_type	          | string   | 是  | 数据盘磁盘类型。"CLOUD_SSD": SSD云硬盘, "CLOUD_PREMIUM": 高性能云盘 |
| network_type        | string   | 是  | 网络类型。"ONETHOUSAND": 千兆, "TENTHOUSAND": 万兆           |
| vpc	              | string   | 否  | 私有网络，默认为空                                           |
| subnet              | string   | 否  | 私有子网，默认为空                                           |
| charge_type         | string   | 否  | 计费模式 (PREPAID:包年包月，POSTPAID_BY_HOUR:按量计费)，默认:包年包月   |
| charge_months       | int      | 否  | 计费时长，单位：月(计费模式为包年包月时，该字段必传)                         |
| bk_asset_id	      | string	 | 否  | 主机继承的固资号                                        |
| inherit_instance_id | string	 | 否  | 主机继承的云实例ID                                      |

#### spec for IDCPM

| 参数名称         | 参数类型    | 必选 | 描述                                        |
|--------------|---------|----|-------------------------------------------|
| region       | string  | 是  | 地域                                        |
| zone         | string  | 是  | 可用区                                       |
| device_type  | string	 | 是  | 机型                                        |
| os_type      | string	 | 是  | 操作系统                                      |
| raid_type    | string	 | 是  | RAID类型                                    |
| network_type | string  | 是  | 网络类型。"ONETHOUSAND": 千兆, "TENTHOUSAND": 万兆 |
| isp          | string  | 否  | 外网运营商                                     |

### 调用示例

```json
{
  "bk_biz_id": 3,
  "require_type": 1,
  "suborders": [
    {
      "resource_type": "QCLOUDCVM",
      "replicas": 2,
      "anti_affinity_level": "ANTI_NONE",
      "remark": "",
      "spec": {
        "region": "ap-shanghai",
        "zone": "ap-shanghai-2",
        "device_type": "S3.LARGE8",
        "image_id": "img-r5igp4bv",
        "disk_size": 200,
        "disk_type": "CLOUD_PREMIUM",
        "network_type": "TENTHOUSAND",
        "vpc": "",
        "subnet": "",
        "charge_type": "PREPAID",
        "charge_months": 1
      }
    }
  ]
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "verifications": [
      {
        "verify_result": "PASS",
        "reason": ""
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                        |
|---------|--------|---------------------------|
| code    | int    | 错误编码。 0表示success，>0表示失败错误 |
| message | string | 请求失败返回的错误信息               |
| data	   | object | 响应数据                      |

#### data

| 参数名称           | 参数类型         | 描述                                       |
|----------------|--------------|------------------------------------------|
| verifications	 | object array | 资源申请子需求单的预测校验信息，校验信息顺序与请求入参的资源申请子需求单顺序一致 |

#### data.verifications[i]

| 参数名称          | 参数类型   | 描述                                          |
|---------------|--------|---------------------------------------------|
| verify_result | string | 预测校验结果，PASS:通过，FAILED:未通过，NOT_INVOLVED: 不涉及 |
| reason        | string | 预测校验结果原因                                    |
