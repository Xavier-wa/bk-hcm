## Context

hc-service 自研云（ziyan）证书同步主流程位于 `cmd/hc-service/logics/res-sync/ziyan/cert.go`：

1. `listCertFromCloud`：调用 ziyan SSL SDK 拉取证书，转换为 `[]typecert.TCloudCert`。
2. `listCertFromDB`：从数据服务拉取本账号/地域下已有证书 `[]corecert.TCloudCert`。
3. `common.Diff[TCloudCert, corecert.TCloudCert]`：按 `GetCloudID` 比对，产出 `addSlice` / `updateMap` / `delCloudIDs`；差异判定函数 `isCertChange` 已包含 `cloud.BkBizID != db.BkBizID` 比较项。
4. `deleteCert` / `createCert` / `updateCert`：落库。当前 `createCert` 使用固定 `opt.BkBizID`（同步调用方恒传 `constant.UnassignedBiz`）；`convCloudToDBUpdate` 不写 `BkBizID`。

**现状问题**：同步全程不消费云上业务标签，所有证书 `bk_biz_id` 恒为 `-1`，无法自动归属业务。

**已具备的能力**：`pkg/ziyan/resource_tag.go`
- `ParseResourceMetaIgnoreErr(tagMap map[string]string) *ResourceMeta`：从标签 map 解析出 `Bs2NameID`（取 `TagKeyBs2` 标签值 `名称_ID` 中的 ID）。
- `GetBkBizIdByBs2(kt, dataCli, ccCli, bs2NameIDs []int64) ([]int64, error)`：批量将二级业务 ID 反查为 `bk_biz_id`，未命中返回 `constant.UnassignedBiz`；内部优先 global_config 缓存，回退 CMDB `SearchBusiness`。
- `NotFoundID = -1`、`TagKeyBs2`（二级业务标签键）。

**已有范式**：`load_balancer.go` 的 `fillBkBizId` 与 `security_group_usage_biz.go` 的 `assignSGToBiz` 已复用上述能力实现「Bs2 → bk_biz_id」，本方案沿用同一范式。

## Goals / Non-Goals

**Goals:**
- 自研云证书同步时，按云上 `二级业务(Bs2)` 标签自动归属业务，减少人工分配。
- 仅补充未分配证书（OQ4=仅补充未分配）：DB 已分配（含手动分配，正值）的结果不被同步覆盖。
- 复用项目既有 `Bs2 → bk_biz_id` 能力，保持实现一致、可维护。
- 同步失败/无标签/标签无法映射时不影响整体同步，证书保持 `UnassignedBiz`。

**Non-Goals:**
- 不在云上写标签（FR1 云打标已取消）。
- 不改动证书「分配接口」（`POST /api/v1/cloud/certs/assign/bizs`）与前端分配按钮。
- 不涉及非自研云（tcloud/aws/azure 等）证书同步路径。
- 不处理证书回收/解绑流程。

## Decisions

### 决策 1：复用 `fillBkBizId` 范式，新增 `fillCertBkBizId`

**选择**：在 `ziyan.cert.go` 新增 `(cli *client) fillCertBkBizId(kt, certFromCloud, certFromDB)`，结构与 `load_balancer.go.fillBkBizId` 一致。

**理由**：
- 项目已有成熟范式，行为、错误处理（fail-fast `return err` + `logs.Errorf`）、批量调用 `GetBkBizIdByBs2` 均经过验证，降低风险。
- `cli.dbCli` / `cli.cmdbCli` 在 ziyan client 上均已存在（`security_group_usage_biz.go` 同款调用），无需新增依赖。

**要点**：
```go
func (cli *client) fillCertBkBizId(kt *kit.Kit, certFromCloud []typecert.TCloudCert,
	certFromDB []corecert.TCloudCert) error {

	dbBkBizMap := make(map[string]int64, len(certFromDB))
	for i := range certFromDB {
		dbBkBizMap[certFromDB[i].GetCloudID()] = certFromDB[i].BkBizID
	}

	bs2NameIds := make([]int64, 0, len(certFromCloud))
	for i := range certFromCloud {
		meta := ziyan.ParseResourceMetaIgnoreErr(certFromCloud[i].GetTagMap())
		bs2 := ziyan.NotFoundID
		if meta != nil {
			bs2 = meta.Bs2NameID
		}
		bs2NameIds = append(bs2NameIds, bs2)
	}

	bizIds, err := ziyan.GetBkBizIdByBs2(kt, cli.dbCli, cli.cmdbCli, bs2NameIds)
	if err != nil {
		logs.Errorf("fail to get bkBizId by bs2NameIds for cert, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if len(bizIds) != len(certFromCloud) {
		return fmt.Errorf("cert bizIds length(%d) not equal to cert length(%d)", len(bizIds), len(certFromCloud))
	}

	for i := range certFromCloud {
		cert := &certFromCloud[i]
		dbBiz := dbBkBizMap[cert.GetCloudID()]
		if dbBiz > 0 {
			// 已分配（含手动分配）：保留 DB 值，不覆盖（OQ4=仅补充未分配）
			cert.BkBizID = dbBiz
			continue
		}
		// 未分配或新建证书：以云上二级业务(Bs2)标签解析结果为准
		cert.BkBizID = bizIds[i]
	}
	return nil
}
```
说明：`GetBkBizIdByBs2` 对 `NotFoundID(-1)` 直接返回 `constant.UnassignedBiz`，索引对齐，故无标签证书解析结果为 `-1`，保持未分配。

---

### 决策 2：OQ4=仅补充未分配 的落地判定

**选择**：以 `dbBiz > 0` 视为「已分配」。已分配证书在 `fillCertBkBizId` 中保持 `cloud.BkBizID = dbBiz`，使 `isCertChange` 不产生差异、不会触发覆盖式更新；仅 `dbBiz <= 0`（含 `UnassignedBiz(-1)` 与同步新建证书无 DB 记录）才写入云标签解析结果。

**理由**：
- 同步调用方恒传 `opt.BkBizID = constant.UnassignedBiz`，历史同步证书在 DB 中 `bk_biz_id = -1`；手动分配写入正值。用 `>0` 区分「未分配」与「已分配」，与全系统 `constant.UnassignedBiz` 语义一致，且能正确覆盖历史 `-1` 证书。
- 与 OQ5（分配接口禁止改绑）互不冲突：同步只补充未分配项，绝不回写已分配记录。

---

### 决策 3：创建/更新均写入解析出的 `BkBizID`

**选择**：
- `createCert`：`BkBizID: one.BkBizID`（替换 `opt.BkBizID`），使新建证书在同步时即按云标签归属。
- `convCloudToDBUpdate`：补充 `BkBizID: one.BkBizID`，使已存在但未分配的证书在后续同步被回填。

**理由**：
- `one.BkBizID` 已由 `fillCertBkBizId` 正确填充（新建/未分配取解析结果、已分配取 DB 原值），diff 阶段 `isCertChange` 据此自然决定是否更新，`createCert`/`updateCert` 直接消费即可，无需重复判断。
- 保留 `opt` 入参（仍可能被其他字段使用），仅替换该字段来源。

---

### 决策 4：扩展 adaptor 类型而非内联字段

**选择**：在 `pkg/adaptor/types/cert/tcloud.go` 的 `TCloudCert` 上新增 `BkBizID int64` 字段与 `GetTagMap()` 方法（对齐 `typeslb.TCloudClb`）。

**理由**：
- `GetTagMap()` 直接复用 `one.Tags`（`[]*ssl.Tag`，含 `TagKey`/`TagValue` 指针字段），与 LB 实现对称，便于后续维护。
- `BkBizID` 承载同步解析结果，使 `fillCertBkBizId`、diff、`createCert`、`convCloudToDBUpdate` 共享同一字段，避免并行 slice/map 传递的脆弱性。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| `GetBkBizIdByBs2` 命中 CMDB 失败（网络/权限）导致同步整体失败 | 与 LB/SG 一致采用 fail-fast；因 Bs2→biz 映射来自 global_config 缓存优先，正常环境极少触发 CMDB 调用。如需更软失败，可后续将此处改为「解析失败则保持 UnassignedBiz、仅告警」（与 FR2 不阻断同步原则一致，留作可选优化） |
| 云上证书标签被误改导致大量证书归属漂移 | OQ4 仅补充未分配，已分配证书不受影响；且 Bs2 标签治理由 CMDB 负责，与历史 LB/SG 行为一致 |
| `BkBizID` 字段新增到 adaptor 类型影响其它消费方 | 仅新增字段与只读方法，默认值 0，现有构造/使用不受影响；tcloud 证书同步未使用该字段 |
| 首屏全量同步对 `GetBkBizIdByBs2` 的批量压力 | 接口内部已做 `slice.Unique` + 内存缓存，重复 bs2 不重复查询，压力可控 |
