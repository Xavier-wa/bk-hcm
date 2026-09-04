# API：安全组-克隆支持选择目标地域

> 原则：本需求**零新增接口、零后端改动**。仅 1 个既有接口扩展 1 个请求字段 + 1 个既有接口复用。

## 接口 1：克隆安全组（既有接口，请求体新增字段）

- **URL**：
  - 业务下：`POST /api/v1/cloud/bizs/{bk_biz_id}/security_groups/{id}/clone`
  - 资源运营下：`POST /api/v1/cloud/security_groups/{id}/clone`
  - （路径由现有 `getBusinessApiPath()` 按视图自动区分，前端不新增判断逻辑）
- **请求体**：

| 字段 | 类型 | 必填 | 说明 | 变更 |
| --- | --- | --- | --- | --- |
| name | string | 是 | 新安全组名称 | 既有 |
| manager | string | 否 | 负责人 | 既有（默认回填源值，可修改） |
| bak_manager | string | 否 | 备份负责人 | 既有（默认回填源值，可修改） |
| **target_region** | string | **是** | **目标地域 ID**（如 `ap-guangzhou`） | **本需求新增**；默认值 = 源安全组 region |

- **响应**：沿用现有响应结构（现有前端调用仅消费成功/失败，不消费响应体细节，无需前端变更）。
- **错误码**：沿用全局 HTTP 错误提示框架，前端不新增错误分支。
- **契约依据**：TAPD 原始需求明确"后端接口已支持 target_region"（示例 `ap-nanjing`）。

## 接口 2：地域列表（既有接口，直接复用，零改动）

- **URL**：`POST /api/v1/cloud/vendors/tcloud/regions/list`
- **用途**：填充克隆弹窗「目标地域」下拉选项（当前业务/该 vendor 可用地域全集）。
- **请求体**：

```json
{
  "filter": {
    "op": "and",
    "rules": [
      { "field": "vendor", "op": "eq", "value": "tcloud" },
      { "field": "status", "op": "eq", "value": "AVAILABLE" }
    ]
  },
  "start": 0,
  "limit": 500,
  "count": true
}
```

- **响应**：

| 字段 | 说明 |
| --- | --- |
| data.count | 总数 |
| data.details[] | 地域数组，含 `region_id`（地域 ID，作为下拉 value）、`region_name` / `display_name`（显示名） |

- **vendor 范围依据**：克隆操作仅对 TCLOUD 开放（`show-clone.plugin.ts`：`vendor === VendorEnum.TCLOUD`），故地域枚举固定 `tcloud`。
- **前端复用**：现有 `useRegionStore().getRegionList({ vendor: 'tcloud' })` 已封装上述调用并归一化为 `{ id, name }` 列表，直接复用，不改该 store。

## 时序与约束

1. 打开克隆弹窗时（或首次展开地域下拉时）调用接口 2，取回地域全集；默认选中源安全组 `region`。
2. 点击确认 → 校验通过 → 调用接口 1（携带 `target_region`）→ 成功提示携带目标地域显示名 → 关闭弹窗、停留当前列表。
3. `target_region` 恒有值（默认源地域），保证默认场景与改造前行为等价（AC-003）。
