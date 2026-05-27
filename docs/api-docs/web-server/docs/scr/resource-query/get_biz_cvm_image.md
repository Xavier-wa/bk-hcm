### 描述

- 该接口提供版本：v9.9.9。
- 该接口所需权限：业务访问。
- 该接口功能描述：业务维度镜像查询，返回公共镜像 + 该业务的私有镜像。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/config/cvm/image

#### 路径参数说明

| 参数名称      | 参数类型  | 必选 | 描述    |
|-----------|-------|----|-------|
| bk_biz_id | int64 | 是  | 业务 ID |

### 输入参数

| 参数名称   | 参数类型         | 必选 | 描述                |
|--------|--------------|----|--------------------|
| region | string array | 否  | 地域列表，为空则查全部地域 |

### 调用示例

#### 请求参数示例

```json
{
  "region": ["ap-guangzhou", "ap-shanghai"]
}
```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 2,
    "info": [
      {
        "region": "ap-guangzhou",
        "image_id": "img-xxx1",
        "image_name": "TencentOS Server 3.1",
        "type": "PUBLIC_IMAGE",
        "bk_biz_id": -1
      },
      {
        "region": "ap-guangzhou",
        "image_id": "img-xxx2",
        "image_name": "my-custom-image",
        "type": "PRIVATE_IMAGE",
        "bk_biz_id": 100
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int    | 状态码  |
| message | string | 请求信息 |
| data    | Data   | 响应数据 |

#### Data

| 参数名称  | 参数类型           | 描述       |
|-------|----------------|----------|
| count | int64          | 镜像总数量    |
| info  | CvmImage Array | 镜像列表     |

#### CvmImage

| 参数名称       | 参数类型   | 描述                                          |
|------------|--------|---------------------------------------------|
| region     | string | 地域                                          |
| image_id   | string | 镜像云上 ID                                     |
| image_name | string | 镜像名称                                        |
| type       | string | 镜像类型（PUBLIC_IMAGE：公共镜像 / PRIVATE_IMAGE：私有镜像） |
| bk_biz_id  | int64  | 关联业务 ID（-1 表示公共镜像，未分配业务）                    |
