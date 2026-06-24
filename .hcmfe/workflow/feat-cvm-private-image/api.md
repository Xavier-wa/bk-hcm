# API: 自研云-主机申领-私有镜像支持

## 接口概述

| 项目 | 说明 |
|------|------|
| 接口版本 | v9.9.9 |
| 接口权限 | 业务访问 |
| 功能描述 | 业务维度镜像查询，返回公共镜像 + 该业务的私有镜像 |

---

## 接口定义

### 请求

```
POST /api/v1/woa/bizs/{bk_biz_id}/config/cvm/image
```

#### 路径参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|----------|---------|:----:|------|
| bk_biz_id | int64 | 是 | 业务 ID（通过 `useWhereAmI().getBizsId()` 获取） |

#### 请求体参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|----------|---------|:----:|------|
| region | string[] | 否 | 地域列表，为空则查全部地域 |

#### 请求示例

```json
{
  "region": ["ap-guangzhou", "ap-shanghai"]
}
```

---

### 响应

#### 响应示例

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

#### 响应字段说明

**顶层响应**

| 字段名 | 类型 | 描述 |
|--------|------|------|
| code | int | 状态码，0 表示成功 |
| message | string | 请求信息 / 错误描述 |
| data | Data | 响应数据 |

**Data 对象**

| 字段名 | 类型 | 描述 |
|--------|------|------|
| count | int64 | 镜像总数量 |
| info | CvmImage[] | 镜像列表 |

**CvmImage 对象**

| 字段名 | 类型 | 描述 |
|--------|------|------|
| region | string | 地域，如 `ap-guangzhou` |
| image_id | string | 镜像云上 ID |
| image_name | string | 镜像显示名称 |
| type | string | 镜像类型：`PUBLIC_IMAGE`（公共镜像）/ `PRIVATE_IMAGE`（私有镜像） |
| bk_biz_id | int64 | 关联业务 ID，`-1` 表示公共镜像（未分配业务），大于 0 表示该业务的私有镜像 |

---

## 前端使用要点

### 调用时机
- **与现有镜像列表接口的调用时机完全一致**（替换原接口，不新增调用点）
- 需要 `bk_biz_id`（通过 `useWhereAmI().getBizsId()` 获取）和 `region` 两个入参

### 数据处理
- `type` 字段区分公共/私有镜像（前端可据此做后续可选的类型标签展示）
- `bk_biz_id = -1` 表示公共镜像，可用于分组或筛选
- 非 target 业务调用此接口时，返回的 `info` 中只有 `PUBLIC_IMAGE` 类型的条目（行为与原接口一致）

### 错误码处理
- `code === 0`：成功
- `code !== 0`：失败，降级到原有镜像获取逻辑（fallback）
