## ADDED Requirements

### Requirement: 已足额交付子单的重试收尾

系统 SHALL 在 generator 判定无需生产新设备（`scheduledCount >= total_num`）时，若按既有「未交付设备」过滤无任何生产记录可重置、且已交付设备数已达子单需求、且无在途生产批次，直接调用 matcher 的 `FinalApplyStep` 为该子单补一次收尾，使其流转到 DONE。

补收尾 MUST 在以下任一情况不执行：存在 `status` 为 Init 或 Handling 的生产记录（在途批次跑完后自然触发收尾）；已存在 `status=Success 且 is_matched=false` 的生产记录（informer 下一轮会捞到，重复执行会导致并发收尾）。

补收尾 MUST 仅在已交付设备数大于等于子单 `total_num` 时执行；已交付数量不足时不介入，收尾由在途批次完成后触发。已交付数量的统计口径 MUST 与 matcher 的收尾判定一致（按设备 `IsDelivered` 计数）。

补收尾 MUST 直接向 `FinalApplyStep` 传入一条 `status=Success` 的生产记录作为入参。收尾作用于整张子单（按 `suborder_id` 重新统计设备），与传入哪条记录无关，任取一条即可。无符合条件的记录时 MUST 输出 warning 日志并返回，不视为错误。

#### Scenario: 全部设备已交付且无在途批次时直接补收尾

- **WHEN** 某子单已有设备数与生产中数量之和不小于 `total_num`，全部设备 `IsDelivered=true`，且无 Init/Handling 批次、无 `Success 且 is_matched=false` 记录
- **THEN** 系统直接调用 `FinalApplyStep` 为该子单执行收尾，子单流转为 DONE

#### Scenario: 存在未交付设备时维持原有过滤逻辑

- **WHEN** 某子单存在 `IsDelivered=false` 的设备
- **THEN** 系统仅重置这些未交付设备对应的生产记录（`is_matched=false`），不触发直接收尾

#### Scenario: 存在在途生产批次时不介入

- **WHEN** 某子单存在 `status` 为 Init 或 Handling 的生产记录
- **THEN** 系统不执行收尾，收尾由在途批次完成后触发

#### Scenario: 已存在未匹配的成功记录时不重复收尾

- **WHEN** 某子单已存在一条 `status=Success 且 is_matched=false` 的生产记录
- **THEN** 系统不再重复收尾，避免同一子单被并发收尾

### Requirement: 直接收尾不重跑生产链路

系统 SHALL 通过 `matcher.FinalApplyStep` 直接收尾，不得通过重置生产记录经 informer 把 `matchHandler → matchDevice → FinalApplyStep` 整条链路再跑一遍。

直接收尾 MUST 不触发初始化、磁盘检查、交付的重复执行；这些步骤只存在于 `matchDevice` 内部，绕过 `matchDevice` 即不会重跑。

终态判定 MUST 仍由 matcher 的 `calcApplyOrderStatus` 决定，不得在 generator 中复制一份终态判断；收尾完成后的通知（`notifyApplyDone` / `checkAndNotifyDelivery`）行为 MUST 与正常收尾一致。

#### Scenario: 已交付设备不被重复初始化或交付或磁盘检查

- **WHEN** matcher 通过直接收尾处理一张全部设备已交付的子单
- **THEN** `matchDevice` 不执行，初始化、磁盘检查、交付均不重复发起

#### Scenario: 收尾后按既有逻辑发送完成通知

- **WHEN** 某子单因直接收尾被置为 DONE，且其所属主单下无其他未完成子单
- **THEN** 系统按既有逻辑发送完成通知，通知行为与正常生产交付完成时一致
