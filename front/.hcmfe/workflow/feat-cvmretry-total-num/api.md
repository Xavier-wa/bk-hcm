# API: CVM 申请 - 修改需求重试 - 数量显示优化

## 涉及的 API

### 1. 拉取申请单列表（含详情，用于修改页面数据回填）

**接口**：`POST /api/v1/woa/{businessPath}task/findmany/apply`

**请求参数**：

```json
{
  "bk_biz_id": [123],
  "suborder_id": ["101735-1"],
  "get_product": true,
  "page": { "start": 0, "limit": 1 }
}
```

**响应（相关字段摘录）**：

```json
{
  "data": {
    "info": [
      {
        "suborder_id": "101735-1",
        "origin_num": 3,
        "total_num": 2,
        "product_num": 0,
        "success_num": 0,
        "pending_num": 0,
        ...
      }
    ]
  }
}
```

### 2. 提交修改申请

**接口**：`POST /api/v1/woa/{businessPath}task/modify/apply`

**请求参数**：

```json
{
  "suborder_id": "101735-1",
  "bk_username": "user",
  "replicas": 2,
  "spec": { ... }
}
```

## 相关字段说明

| 字段 | 类型 | 含义 | 本次变更 |
|------|------|------|----------|
| `origin_num` | number | 首次提单时的原始需求数量 | ❌ 不再用于展示和计算 |
| `total_num` | number | 当前最新总需求数量（可能经过多次修改） | ✅ 改为使用此字段 |
| `product_num` | number | 已生产数量 | 不变 |
| `pending_num` | number | 待生产数量 | 不变 |
| `success_num` | number | 成功生产数量 | 不变 |

## 无后端变更

本次为纯前端修复，后端无需配合修改。
