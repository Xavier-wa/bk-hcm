## ADDED Requirements

### Requirement: 自研云证书同步时按二级业务(Bs2)标签自动归属业务

系统 SHALL 在自研云（ziyan）证书同步流程中，解析每张证书云上资源的 `二级业务(Bs2)` 标签，经 `ziyan.GetBkBizIdByBs2` 反查 `bk_biz_id`，并写入证书记录的 `bk_biz_id` 字段；仅支持自研云（ziyan）证书同步。

当证书云上无 `二级业务(Bs2)` 标签或标签无法映射到业务时，该证书 SHALL 保持 `constant.UnassignedBiz`，不报错、不阻断整体同步。

#### Scenario: 云上带二级业务标签的证书同步后自动归属

- **GIVEN** 自研云证书在云上带有 `二级业务(Bs2)=xxx_yyy`（yyy 为可映射的二级业务 ID）
- **WHEN** hc-service 执行该证书同步（新建或更新）
- **THEN** 系统经 `ziyan.GetBkBizIdByBs2` 解析并将该证书 `bk_biz_id` 写入对应业务 B，业务视图可见该证书

#### Scenario: 云上无标签的证书保持未分配

- **GIVEN** 自研云证书在云上无 `二级业务(Bs2)` 标签，或标签无法映射到业务
- **WHEN** 同步执行
- **THEN** 证书 `bk_biz_id` 保持 `constant.UnassignedBiz(-1)`，同步不报错、不阻断

#### Scenario: 新建证书在首次同步即完成归属

- **GIVEN** 云上新增一张带 `二级业务(Bs2)` 标签的证书，HCM 中尚无该证书记录
- **WHEN** 同步执行进入创建分支
- **THEN** 新建证书的 `bk_biz_id` 直接使用云标签解析结果（而非固定 `UnassignedBiz`）

---

### Requirement: 同步自动归属不覆盖已手动分配结果（OQ4=仅补充未分配）

系统 SHALL 在自研云证书同步的自动归属过程中，对 DB 中 `bk_biz_id` 已分配（大于 0，含 HCM 手动分配）的证书保留其现有业务归属，不以云标签结果覆盖。

#### Scenario: 已手动分配的证书不被云标签覆盖

- **GIVEN** 证书在 HCM 已被手动分配业务 A（`bk_biz_id = A > 0`），但其云上 `二级业务(Bs2)` 标签指向业务 B（或任意值）
- **WHEN** 同步执行
- **THEN** 该证书 `bk_biz_id` 保持 A，同步不产生覆盖式更新，`isCertChange` 不将其判为变更

#### Scenario: 仅未分配证书被云标签填充

- **GIVEN** 证书 `bk_biz_id = constant.UnassignedBiz(-1)` 且云上 `二级业务(Bs2)` 标签可映射到业务 B
- **WHEN** 同步执行进入更新分支
- **THEN** 该证书 `bk_biz_id` 被刷新为 B（仅补充未分配项）

#### Scenario: 多次同步幂等

- **GIVEN** 证书已在某次同步中由云标签自动归属到业务 B
- **WHEN** 后续再次同步，云标签与 DB 归属一致
- **THEN** 不产生新的更新写操作（diff 判定无变化），行为幂等
