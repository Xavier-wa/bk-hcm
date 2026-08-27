# API - 机房裁撤总览-业务名称未知数据支持显示及明细跳转

## 接口概述

本需求不新增任何接口，仅对现有接口的入参支持 `bk_biz_id=0` 的场景。

## 现有接口

### 1. 裁撤总览列表

- **URL**: `POST /api/v1/woa/dissolve/table/list`
- **用途**: 获取裁撤总览列表数据
- **入参变更**: 无。当搜索栏选择"未知"时，前端传 `bk_biz_ids=[0]`
- **出参变更**: 无。后端需确保能正确返回 `bk_biz_id=0` 的汇总数据行

**请求示例**:
```json
{
  "page": { "start": 0, "limit": 10, "enable_count": true },
  "bk_biz_ids": [0]
}
```

**响应示例**（无变更）:
```json
{
  "count": 1,
  "data": [
    {
      "bk_biz_id": 0,
      "progress": "50%",
      "progress_str": "50%",
      "current_host_count": 5,
      "current_cpu_core": 100,
      "delivered_cpu_core": 50,
      "origin_host_count": 10,
      "origin_cpu_core": 200
    }
  ]
}
```

### 2. 裁撤明细列表

- **URL**: `POST /api/v1/woa/dissolve/host/detail/list`
- **用途**: 获取裁撤明细列表数据（设备明细）
- **入参变更**: 无。当从裁撤总览跳转时，前端传 `bk_biz_ids=[0]`
- **出参变更**: 无。后端需确保能正确返回 `bk_biz_id=0` 对应的设备明细数据

**请求示例**:
```json
{
  "page": { "start": 0, "limit": 10, "enable_count": true },
  "bk_biz_ids": [0]
}
```

## 接口契约约定

### "未知"业务的标识

- `bk_biz_id=0` 统一标识"未知"业务
- 后端需支持 `bk_biz_ids` 参数包含 `0` 的查询
- 后端返回的列表数据中，"未知"业务的行 `bk_biz_id` 字段值统一为 `0`；其他未匹配的 id 不属于"未知"业务，前端展示保持 `--`

### 前端处理

- 前端在裁撤总览跳转到裁撤明细时，对于"未知"业务行，统一传 `bk_biz_ids=[0]`
- 前端在搜索栏选择"未知"选项时，传 `bk_biz_ids=[0]`（可与其他业务 ID 多选）
- 前端不需要新增或修改任何 API 调用函数，仅修改传参值

## 错误码

无新增错误码。现有接口的错误处理保持不变。
