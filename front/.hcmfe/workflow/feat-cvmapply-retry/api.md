# API - CVM申请-修改需求重试优化

## 接口变更

### POST /api/v1/woa/config/findmany/config/cvm/charge_type/device_type

**请求参数变更**（新增可选字段）：

```typescript
interface IChargeTypeDeviceTypeParams {
  bk_biz_id: number;
  require_type: RequirementType;
  region: string;
  zone?: string;
  // 新增字段
  suborder_id?: string; // 编辑模式时携带，用于排除当前子订单预测
}
```

**响应**：保持不变

```typescript
interface IChargeTypeDeviceTypeRes {
  info: Array<{
    available: boolean;
    charge_type: string;
    device_types: Array<{
      device_type: string;
      available: boolean;
      remain_core: number;
    }>;
  }>;
  count: number;
}
```

## 调用链

```
application-modify/index.vue (suborderId)
  └─> DeviceTypeCvmSelector (cvm-apply.vue) (suborderId prop)
      └─> useDeviceTypePlan hook (suborderId param)
          └─> useCvmDeviceStore.getChargeTypeDeviceTypeList (suborder_id)
              └─> POST /api/v1/woa/config/findmany/config/cvm/charge_type/device_type
```
