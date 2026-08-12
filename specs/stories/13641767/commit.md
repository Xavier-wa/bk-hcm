# Commit 记录

## Commit Message

```
feat(resource-plan): 新增预算申报主单类型与 overwrite_append 必填 type

资源预测主单支持 budget_declare（预算申报）：overwrite_append 必填 type，
拆单统一走 SplitAdjustTicket，子单 HCM 管理员强制 skip（含跨年）；
非 overwrite_append 创建入口拒绝该类型；列表/meta 支持筛选；同步 API 文档。

--story=1069995598136417767
```

## Commit Hash

ddefd4e02902b157b7a1ff258c6c5663598885e8

## 变更统计

| 指标 | 值 |
|------|-----|
| 总变更行数 | 2347 |
| 新增代码 | 2334 |
| 删除代码 | 13 |
| 逻辑代码 | 57 |
| 测试代码 | 319 |
| 文档变更 | 1971 |
| 变更文件数 | 36 |

## 成本汇总

### 总体

| 指标 | 值 |
|------|-----|
| 总耗时 | 0 s（cost-events.jsonl 缺失，未计量） |
| 总成本 | 0 credit |
| 总输入 tokens | 0 |
| 总输出 tokens | 0 |
| 总缓存 tokens | 0 |
| subagent 调用次数 | 0（无 cost-events，未对账） |

### 各阶段

| 阶段 | 耗时 | 成本 | 输入 tokens | 输出 tokens | 缓存 tokens | 调用次数 |
|------|------|------|------------|------------|------------|---------|
| — | — | — | — | — | — | — |

## 时间

- 开始时间：2026-07-23T20:13:48+08:00
- 完成时间：2026-07-24T11:03:56+08:00

## 校验结论

| 维度 | Verdict |
|------|---------|
| arch | LGTM |
| security | LGTM |
| codereview | LGTM（MEDIUM：拆单路由镜像测试；LOW：derive 死代码可后续清理） |
| test | LGTM（集成 skip stub 按 tasks 备注可接受） |
