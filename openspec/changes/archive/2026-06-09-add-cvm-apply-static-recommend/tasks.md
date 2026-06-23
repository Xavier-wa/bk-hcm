## 1. 配置项

- [x] 1.1 在 `pkg/cc/` 新增 `ApplyRecommend` 配置（默认申请数量，默认 10），含默认值与校验
- [x] 1.2 同步 `etc/woa_server.yaml` 新增配置段
- [x] 1.3 同步 `docs/support-file/helm` woa-server values 配置

## 2. API 协议（pkg/api/woa-server/cvm_apply_recommend.go）

- [x] 2.1 新增请求结构体 `ApplyRecommendByStaticReq`（必填 bk_username/limit；可选 A 类 require_type/region/device_type/image_id、B 类 zone/res_assign/replicas），实现 `Validate()`
- [x] 2.2 新增响应结构体 `ApplyRecommendByStaticResp` 与方案元素（含 source 来源字段及单子单字段：需求类型/地域/可用区/机型/image_id/资源分配方式/申请数量/计费模式/系统盘/数据盘）

## 3. 推荐查询 helper 扩展（cmd/woa-server/service/task/recommend.go）

- [x] 3.1 为 `listUserRecommend` / `listBizRecommend` 增加可选 A 类过滤 rules 入参（向后兼容，空入参行为不变）
- [x] 3.2 确认 `GetBizApplyRecommendTop` 调用传空过滤，行为与 SQL 不变

## 4. 接口实现（cmd/woa-server/service/task/recommend.go）

- [x] 4.1 新增 Handler：解析路径 `bk_biz_id`、解码与校验请求、业务访问鉴权
- [x] 4.2 Step 参数归一：A 类过滤、申请数量（入参/默认配置）、计费 PREPAID
- [x] 4.3 Step 查静态推荐候选：user 优先 biz 补足，A 类收窄，按四元组去重；A 类查不到→返回空
- [x] 4.4 Step 库存校验：先判 `require_type.NotNeedVerifyCapacity()` 跳过；其余批量 `DeviceCapacity.List`（capacity ≥ 申请数量），未传 zone→region 任一 zone 满足即保留、传了 zone→按该 zone 校验
- [x] 4.5 Step 补默认值：image_id（静态表）、res_assign（zone 联动）、charge=PREPAID、系统盘/数据盘默认值
- [x] 4.6 Step 组装：每候选→1 方案，user 优先 biz，按 count 倒序取前 N，全空返回空

## 5. 路由注册（cmd/woa-server/service/task/service.go）

- [x] 5.1 在 `bizService` 注册 `POST /apply/recommend/by_static_recommend`

## 6. 验证与文档

- [x] 6.1 `go build` / `go vet` 通过
- [ ] 6.2 自测：未传 zone / 传 zone / A 类过滤无匹配 / 绿通跳过库存 / 库存不足 / 申请数量默认与入参 等场景（需运行环境，待手测）
- [x] 6.3 回归确认 `GetBizApplyRecommendTop` 行为不变（代码级：helper 变长参数零传入，SQL 不变）
- [x] 6.4 新增接口文档 `docs/api-docs/api-server/docs/zh/get_biz_apply_recommend_by_static.md`
