# aws_gpu_instance_types 配置迁移说明

本变更将 `global_config` 中 `config_type=account_bill`、`config_key=aws_gpu_instance_types`
的 `config_value` 由「GPU 实例类型列表（JSON 数组）」改造为「实例类型 → 卡型 的映射（JSON 对象）」。

- **执行方**：运营（手动刷新 DB 配置），不在代码仓库执行。
- **执行时机**：必须在部署改造后的 task-server **之前** 完成（参见 `design.md` 迁移计划）。
- **关键约束**：新对象的「键集合」MUST 与改造前数组元素**完全一致**，以保证 `ResClassId`
  的 GPU 判定结果不变。

## 新格式 config_value（实例类型 → 卡型 JSON 对象）

> 卡型来源：`docs/reqs/GPU账单卡型拆分.md` F-003。`config_value` 仅存卡型，不含卡数。

```json
{
  "g4dn.xlarge": "T4",
  "g4dn.2xlarge": "T4",
  "g4dn.4xlarge": "T4",
  "g4dn.8xlarge": "T4",
  "g4dn.16xlarge": "T4",
  "g4ad.xlarge": "T4",
  "g7e.48xlarge": "L40S",
  "g5.xlarge": "A10G",
  "g5.2xlarge": "A10G",
  "g5.4xlarge": "A10G",
  "g5.8xlarge": "A10G",
  "g5.12xlarge": "A10G",
  "g5.16xlarge": "A10G",
  "g5.24xlarge": "A10G",
  "g5.48xlarge": "A10G",
  "g5g.xlarge": "A10G",
  "g6.xlarge": "L4",
  "g6.2xlarge": "L4"
}
```

## 新增 gcp_gpu_instance_prefixes 配置（实例族前缀 → 短卡型名 JSON 对象）

本变更在 `global_config` 中**新增**一条 `config_type=account_bill`、`config_key=gcp_gpu_instance_prefixes`
的记录，承载 GCP 卡型识别的 L2 实例族前缀映射。

- **执行方**：运营（手动录入 DB 配置），不在代码仓库执行。
- **执行时机**：在部署改造后的 task-server **之前** 完成。
- **匹配语义**：代码侧按忽略大小写 + 词边界匹配，命中多个前缀时取前缀字符串最长者对应的卡型
  （消解 `A3` 与 `A3Ultra`/`A3 Ultra` 歧义）。
- **缺失影响**：该配置缺失仅导致 L2 不命中（L1 显式关键词仍生效），不阻断上报，回滚时可不删除。
- **取值**：`config_value` 仅存短卡型名，与 AWS 卡型短名风格对齐，不含卡数。

```json
{
  "A3Ultra": "H200",
  "A3 Ultra": "H200",
  "A3": "H100",
  "A2": "A100",
  "G2": "L4",
  "G4": "RTX6000PRO"
}
```

## 回滚

若需回滚 `aws_gpu_instance_types` 配置格式，须**同时**回滚 task-server 与本配置（两者格式强耦合，参见 `design.md`）。
`gcp_gpu_instance_prefixes` 为新增配置，缺失不阻断上报，回滚时可保留或删除。
