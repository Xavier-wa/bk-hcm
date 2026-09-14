## T-01: 扩展证书云类型承载业务归属

- [x] 1.1 在 `pkg/adaptor/types/cert/tcloud.go` 的 `TCloudCert` 结构体中新增 `BkBizID int64 \`json:"bk_biz_id"\`` 字段（对齐 `typeslb.TCloudClb`）
- [x] 1.2 为 `TCloudCert` 新增 `GetTagMap()` 方法：遍历 `one.Tags`（`[]*ssl.Tags`），将非空 `TagKey`/`TagValue` 收集为 `apicore.TagMap`（取值用 `converter.PtrToVal`）

## T-02: 新增 fillCertBkBizId 复用 Bs2→bk_biz_id 能力

- [x] 2.1 在 `cmd/hc-service/logics/res-sync/ziyan/cert.go` 新增 `fillCertBkBizId`（入参 `certFromDB` 为 `[]*corecert.Cert[corecert.TCloudCertExtension]`，与 `listCertFromDB` 返回类型一致）
- [x] 2.2 构建 `dbBkBizMap`（cloudID → 现有 DB `bk_biz_id`）；遍历 `certFromCloud` 调 `ziyan.ParseResourceMetaIgnoreErr(cert.GetTagMap())` 收集 `bs2NameIds`（无标签时取 `ziyan.NotFoundID`）
- [x] 2.3 调用 `ziyan.GetBkBizIdByBs2(kt, cli.dbCli, cli.cmdbCli, bs2NameIds)`，校验返回长度与证书数一致；遍历结果：`dbBiz > 0` 时 `cert.BkBizID = dbBiz`（保留，OQ4），否则 `cert.BkBizID = bizIds[i]`（云标签结果，-1 表示未分配）
- [x] 2.4 错误处理与 `load_balancer.go.fillBkBizId` 一致：失败 `logs.Errorf` 后 `return err`（fail-fast）；包级增加必要的 `fmt`、`ziyan` 导入

## T-03: 接入同步 diff 流程并落库

- [x] 3.1 在 `Cert()` 中 `listCertFromCloud` 与 `listCertFromDB` 之后、`common.Diff` 之前，调用 `fillCertBkBizId(kt, certFromCloud, certFromDB)`
- [x] 3.2 修改 `createCert`：新建证书的 `BkBizID` 由 `opt.BkBizID` 改为 `one.BkBizID`（消费云标签解析结果）
- [x] 3.3 修改 `convCertCloudToDBUpdate`（实现名，对应任务中的 `convCloudToDBUpdate`）：更新请求补充 `BkBizID: one.BkBizID` 字段，使未分配证书在同步中被回填

## T-04: 单元测试

- [x] 4.1 为 `fillCertBkBizId` 增加单测，覆盖：云标签可映射 → 写入业务 B；无标签 → `UnassignedBiz`；DB 已分配（>0）→ 保留原值不被覆盖；DB 未分配（-1）→ 被云标签填充；批量 `GetBkBizIdByBs2` 返回值与索引对齐
- [ ] 4.2 为 `createCert`/`convCertCloudToDBUpdate` 增加断言：新建与更新均写入 `one.BkBizID`，且已分配证书经 diff 不产生覆盖写
  - 已完成：`convCertCloudToDBUpdate`（`TestConvCertCloudToDBUpdateBkBizID`）、已分配不 diff（`TestIsCertChange_assignedCertNoDiff` + fill 保留用例）
  - 未完成：`createCert` 落库路径无单测（现有 `TestCreateCertUsesCloudBkBizID` 仅断言结构体字段，未覆盖 `createCert` 函数）

## T-05: 回归与验证

- [x] 5.1 本地编译 `go build ./cmd/hc-service/...` 与 `go vet` 通过
- [x] 5.2 运行 ziyan res-sync 证书相关单测与邻近 LB/SG 同步单测，确认无回归（`go test ./cmd/hc-service/logics/res-sync/ziyan/...` 通过）
- [ ] 5.3 人工/联调验证：对一张带 `二级业务(Bs2)` 标签的自研云证书触发同步，确认 `bk_biz_id` 自动归属；对已手动分配证书再次同步，确认不被覆盖
