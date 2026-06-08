# 机房裁撤配额系数与偏移管理 - 任务清单

## 1. 常量与枚举定义

- [x] 1.1 在 `pkg/criteria/enumor/` 新增配置键常量：`DissolveQuotaCoefficient`、`DissolveQuotaOffsets`
- [x] 1.2 在 `pkg/criteria/enumor/` 新增偏移类型枚举：`DissolveQuotaOffsetTypeIncrease = "increase"`、`DissolveQuotaOffsetTypeDecrease = "decrease"`，并定义 `DissolveQuotaOffsetType` 类型及 `Validate()` 校验方法
- [x] 1.3 在 `pkg/criteria/enumor/` 新增默认配额系数常量：`DefaultDissolveQuotaCoefficient float64 = 65`

## 2. 类型定义

- [x] 2.1 在 `cmd/woa-server/types/dissolve/` 新增 `QuotaOffsetItem` 结构体（BkBizID、Offset、Type、Memo 字段）
- [x] 2.2 在 `cmd/woa-server/types/dissolve/` 扩展 `Config` 响应结构体，新增 `QuotaCoefficient`、`QuotaOffsets` 字段（数组格式）
- [x] 2.3 在 `cmd/woa-server/types/dissolve/` 扩展 `UpsertConfigReq` 请求结构体，新增 `QuotaCoefficient`、`QuotaOffsets` 字段（数组格式）
- [x] 2.4 在 `cmd/woa-server/types/dissolve/` 新增 `UpdateDissolveQuotaOffsetReq` 请求结构体（Offset、Type、Memo 字段，bk_biz_id 在路径参数）
- [x] 2.5 在 `cmd/woa-server/types/dissolve/` 新增 `UpdateDissolveQuotaOffsetResp` 响应结构体

## 3. 配置逻辑层

- [x] 3.1 在 `cmd/woa-server/logics/dissolve/config/` 新增 `GetQuotaCoefficient()`、`GetQuotaOffsets()` 方法
- [x] 3.2 在 `cmd/woa-server/logics/dissolve/config/` 新增 `UpsertQuotaCoefficient()`、`UpsertQuotaOffsets()` 方法
- [x] 3.3 在 `cmd/woa-server/logics/dissolve/config/` 新增 `UpdateDissolveQuotaOffset()` 方法，单业务偏移修改

## 4. 配置服务层

- [x] 4.1 在 `cmd/woa-server/service/dissolve/` 扩展 `GetDissolveConfig()` 接口处理函数，返回配额系数、偏移配置
- [x] 4.2 在 `cmd/woa-server/service/dissolve/` 扩展 `UpsertDissolveConfig()` 接口处理函数，支持配额系数、偏移配置更新
- [x] 4.3 在 `cmd/woa-server/service/dissolve/` 新增 `UpdateDissolveQuotaOffset()` 接口处理函数

## 5. 路由注册

- [x] 5.1 在 `cmd/woa-server/service/dissolve/` 已有 `GET /api/v1/woa/dissolve/config` 路由
- [x] 5.2 在 `cmd/woa-server/service/dissolve/` 已有 `PUT /api/v1/woa/dissolve/config/upsert` 路由
- [x] 5.3 在 `cmd/woa-server/service/dissolve/` 注册 `PUT /api/v1/woa/dissolve/quota/offset/{bk_biz_id}` 路由

## 6. 额度计算逻辑改造

- [x] 6.1 修改额度汇总逻辑，应用新公式：`可申请额度 = max(0, 裁撤原始核数 × 配额系数/100 + 业务偏移额度 - 已交付核数)`
- [x] 6.2 额度汇总响应新增 `available_quota` 字段（后端计算好的可申请额度，前端直接使用）

## 7. 主机申请校验逻辑改造

- [x] 7.1 修改主机申请校验逻辑，基于新公式计算可申请额度进行校验
- [x] 7.2 申请超额时返回明确的错误信息，包含可申请额度详情

## 8. 单元测试

- [x] 8.1 在 `cmd/woa-server/types/dissolve/types_test.go` 测试 `UpsertConfigReq.Validate()` 方法（系数范围校验、偏移配置校验）
- [x] 8.2 在 `cmd/woa-server/types/dissolve/types_test.go` 测试 `UpdateDissolveQuotaOffsetReq.Validate()` 方法（offset、type枚举、memo长度校验）
- [x] 8.3 在 `cmd/woa-server/types/dissolve/types_test.go` 测试 `ConvertHost()` 函数（nil输入、IsPass布尔转换、非法值错误处理、完整字段映射）
- [x] 8.4 运行单元测试 `go test ./cmd/woa-server/types/dissolve/... -v` 确保测试通过

## 9. 兼容性处理

- [x] 9.1 配额系数未配置时，默认使用 `enumor.DefaultDissolveQuotaCoefficient`（65，百分比）
- [x] 9.2 业务无偏移记录时，偏移值视为 0
- [x] 9.3 现有接口响应向后兼容，仅新增字段
- [x] 9.4 偏移类型校验使用 `enumor.DissolveQuotaOffsetType.Validate()` 方法，避免硬编码

## 10. 集成验证

- [ ] 10.1 在测试环境部署，验证获取配置接口返回正确数据
- [ ] 10.2 验证全量更新配置接口工作正常，操作日志正确记录
- [ ] 10.3 验证单业务偏移修改接口工作正常
- [ ] 10.4 验证偏移上限校验逻辑，超限时正确拒绝
- [ ] 10.5 验证额度汇总接口返回 `available_quota` 字段
- [ ] 10.6 验证主机申请校验使用新公式
