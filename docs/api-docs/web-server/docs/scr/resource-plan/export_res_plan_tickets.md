### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：平台-单据管理。
- 该接口功能描述：批量导出资源预测单据为 Excel 文件，并按照需求拆分，每条需求对应一行数据。

### URL

POST /api/v1/woa/plans/resources/tickets/export

### 输入参数

| 参数名称 | 参数类型         | 必选 | 描述                  |
|------|--------------|----|--------------------|
| ids  | string array | 是  | 资源预测单据ID列表，不能为空，支持全量导出 |

### 调用示例

```json
{
  "ids": [
    "0000000001",
    "0000000002"
  ]
}
```

### 响应示例

#### 导出成功结果示例

Content-Type: application/octet-stream
Content-Disposition: attachment; filename="res_plan_tickets_1716112800.xlsx"
[二进制文件流]
