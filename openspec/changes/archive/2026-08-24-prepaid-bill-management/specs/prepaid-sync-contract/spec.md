## ADDED Requirements

### Requirement: sync 接口字段口径

系统的 `POST /api/v1/account/bills/prepaid_items/sync` 接口 SHALL 以单一字段口径为唯一真相源，其余 capability MUST NOT 重新定义任何字段、类型、约束或错误码。请求体 SHALL 为批量结构：顶层只含 `items` 数组，单次条数 MUST 在 1-100 之间。字段命名 MUST 统一为 snake_case，主数据字段 MUST 与预付费主表 `account_bill_prepaid_item` 的字段一一对齐（`rmb_cost` 除外，见下）。主数据 `cost` MUST 严格大于 0；分摊明细金额不要求为正，仅要求合计等于主数据 `cost`。调整类型 `type` MUST 由条目角色固定（调增 / 调减），MUST NOT 由金额正负判定。金额 MUST 只接收 `cost` 一个字段，`rmb_cost` MUST NOT 出现在请求体中、MUST 由服务端按 `currency` 派生（`CNY` 时与 `cost` 同值，其他币种落 0），HCM MUST NOT 做任何汇率换算。字段口径的演进 MUST 为 Additive：只允许新增字段，禁止删除或修改既有字段的语义。

#### Scenario: 批量条数超上限被拒

- **GIVEN** 一个含 101 条 `items` 的请求体
- **WHEN** 执行请求结构体校验
- **THEN** 整批被拒绝，返回 `errf.InvalidParameter`

#### Scenario: 人民币金额按币种派生

- **GIVEN** 一条 `currency=USD`、`cost=1000` 的订单与一条 `currency=CNY`、`cost=1000` 的订单
- **WHEN** 落库主表与派生调账
- **THEN** `USD` 订单的 `rmb_cost` 为 0，`CNY` 订单的 `rmb_cost` 为 1000
- **AND** 服务端不做任何汇率换算

> 该字段口径的草案产出、发调用方确认与冻结标记属流程动作，见 tasks.md 第 1 组与 design.md 的 D-11，不在本 spec 的断言范围内。

#### Scenario: 主数据字段与主表逐字段对齐

- **GIVEN** sync 接口的字段口径已确定
- **WHEN** 将接口的主数据字段与预付费主表 `account_bill_prepaid_item` 的字段清单逐字段比对
- **THEN** 主数据字段与主表字段一一对齐，无遗漏、无多余
- **AND** 全部为 snake_case 命名

#### Scenario: 主数据负数金额被拒

- **GIVEN** 主数据 `cost` 传入负数或零
- **WHEN** 执行校验
- **THEN** 返回 `errf.InvalidParameter`
- **AND** 调整类型不因金额正负而改变，仍由条目角色固定

#### Scenario: 字段口径演进保持向后兼容

- **GIVEN** 字段口径已在使用中
- **WHEN** 引入字段变更
- **THEN** 只允许新增字段
- **AND** 既有字段不被删除、其语义不被修改

### Requirement: 请求与响应 DTO 定义

系统 SHALL 定义 `PrepaidItemSyncReq`（批量外壳，只含 `items`）、`PrepaidItemSyncItem`（单个订单的主数据 + N 条分摊明细）、`PrepaidSplitItem`（分摊明细）、`PrepaidItemSyncResp`（只含 `results`）与 `PrepaidItemSyncResult`（单单结果）五个结构体。批量条数 MUST 通过 validator tag `validate:"max=100"` 强制上限为 100；分摊明细条数 N MUST 至少 1 条，MUST NOT 设置条数上限。金额字段 MUST 使用 decimal 语义承载，不得使用浮点类型。

#### Scenario: 响应体逐单返回结果

- **GIVEN** 一次含 N 单的批量写入
- **WHEN** 检查响应体
- **THEN** `results` 含 N 项且顺序与请求 `items` 一致
- **AND** 每项含 `uuid` / `order_year` / `order_month` / `id` / `success` / `message`，成功项的 `id` 为该预付费账单的主表 ID
- **AND** 逐单结果 MUST NOT 携带错误码，失败原因只由 `message` 承载

### Requirement: 逐单独立事务与批内唯一键

系统 SHALL 逐单处理批量请求中的每个订单，单单使用独立事务，某单失败 MUST NOT 影响其余单据落库。失败单 MUST 在其结果项中以 `success=false` 与 `message` 记录失败原因，接口整体 MUST 仍返回成功以便调用方按 `uuid` 逐条判定并单独重推。批内 `(uuid, order_year, order_month)` MUST NOT 重复，重复时 MUST 整批拒绝并返回 `errf.InvalidParameter`。

#### Scenario: 单单失败不影响其余单据

- **GIVEN** 一个含 3 单的请求，其中第 2 单的主表已定账
- **WHEN** 调用 sync 接口
- **THEN** 第 1、3 单落库成功且各自返回预付费账单 ID
- **AND** 第 2 单的结果项 `success=false` 且 `message` 给出定账拒绝原因
- **AND** 接口整体返回成功

#### Scenario: 批内业务唯一键重复整批拒绝

- **GIVEN** 一个含两条相同 `(uuid, order_year, order_month)` 的请求
- **WHEN** 执行请求结构体校验
- **THEN** 整批被拒绝，返回 `errf.InvalidParameter` 且错误信息指明重复的业务唯一键

### Requirement: R-015 硬校验分档

系统 SHALL 对每个订单执行硬校验，任一项不通过 MUST 返回 `errf.InvalidParameter` 且错误信息 MUST 指明具体不通过的字段。校验项为：`currency` 枚举合法且与该二级账号所属 summary root 的 `currency` 一致；`vendor` 通过 `enumor.Vendor.Validate()`；`gpu_type` 必填；`order_at` 按 `constant.DateTimeLayout` 可解析；`usage_start_at` / `usage_end_at` 为非必填，两者都传入时须可解析且 `usage_start_at` < `usage_end_at`；分摊月份 month ∈ [1,12]；主数据 `cost` > 0；分摊金额不要求为正；`device_num` / `card_num` 为非必填，传入时不得为负；N 条分摊金额合计 = 优惠后总价；分摊明细至少 1 条且不设条数上限。

`usage_start_at` / `usage_end_at` 改为非必填后，系统 MUST NOT 再校验「分摊月份落在使用起止区间内」，账期合法性只由 `bill_month` 的 1-12 取值域与分摊合计等式保证。

#### Scenario: 全部硬校验字段合法时通过

- **GIVEN** 全部硬校验字段取值合法
- **WHEN** 调用请求结构体的校验方法
- **THEN** 校验通过，无错误返回

#### Scenario: 硬校验反例逐一被拒并指明字段

- **GIVEN** 分别构造 `currency=JPY`（非法枚举）、`vendor` 非法、`gpu_type` 缺失、`order_at` 缺失、`usage_start_at` ≥ `usage_end_at`、分摊月份 = 13、`cost` = 0、`card_num` = -1、`device_num` = -1
- **WHEN** 逐一执行校验
- **THEN** 每个用例均返回 `errf.InvalidParameter`
- **AND** 错误信息指明具体不通过的字段名

#### Scenario: 使用起止时间缺失时不做区间约束

- **GIVEN** 一个不传 `usage_start_at` / `usage_end_at` 的订单，分摊月份为任意合法月份
- **WHEN** 执行校验
- **THEN** 校验通过，不因缺少使用区间而被拒

#### Scenario: 分摊金额合计等于总价时通过

- **GIVEN** 优惠后总价 = 3000.00，分摊明细为 1000.00 / 1000.00 / 1000.00
- **WHEN** 执行校验
- **THEN** 校验通过

#### Scenario: 单条分摊金额不要求为正

- **GIVEN** 优惠后总价 = 3000.00，分摊明细为 -100.00 / 0 / 3100.00
- **WHEN** 执行校验
- **THEN** 校验通过

#### Scenario: 分摊金额合计不等于总价时被拒

- **GIVEN** 优惠后总价 = 3000.00，分摊明细为 1000.00 / 1000.00 / 999.99
- **WHEN** 执行校验
- **THEN** 返回 `errf.InvalidParameter`
- **AND** 错误信息指明分摊合计（2999.99）与总价（3000.00）不一致

#### Scenario: currency 与二级账号比对函数可被映射后复用

- **GIVEN** 「`currency` 与该二级账号一致」的比对基准依赖云账号映射结果，而映射动作发生在写入域
- **WHEN** 契约域提供该比对能力
- **THEN** 契约域提供一个可复用的比对函数，由写入域在映射完成后调用
- **AND** 比对基准为该二级账号所属 summary root 的 `currency`

### Requirement: CurrencyCode 枚举校验方法

系统 SHALL 为 `enumor.CurrencyCode` 新增 `Validate()` 方法。现网该枚举无此方法，本次为新增。写法 MUST 参照同文件内 `BillAdjustmentResClass.Validate()`。

#### Scenario: CurrencyCode.Validate 接受合法值拒绝非法值

- **GIVEN** `enumor.CurrencyCode` 已补 `Validate()` 方法
- **WHEN** 分别传入 `CNY`、`USD`、`JPY`
- **THEN** `CNY` 与 `USD` 校验通过
- **AND** `JPY` 返回错误

### Requirement: res_class 与 res_sub_class 由服务端固定填充

请求体 MUST NOT 包含 `res_class` / `res_sub_class`。系统 SHALL 为预付费派生的 N+1 条调账固定填充 `res_class=gpu_card`，`res_sub_class` MUST 取该单的 `gpu_type`。预付费单据本身就是 GPU 卡的整单预购，类别与卡型由服务端推导可避免同一张单内出现类别与卡型互相矛盾的条目，也消除了原先「调用方传值走软校验」的整条口径。

> 本要求取代了原先的软校验档（Q-007 → B）。由于取值不再来自调用方，`gpu_card` 必填子类的约束由 `gpu_type` 的必填性直接满足，软校验与其 Warn 日志一并取消。人工录入路径的硬校验不受影响，见 `bill-adjustment-guard`。

#### Scenario: 派生调账的类别与子类固定

- **GIVEN** 一个 `gpu_type=H100` 的订单，含 3 条分摊明细
- **WHEN** 生成 N+1 条调账
- **THEN** 4 条调账的 `res_class` 均为 `gpu_card`
- **AND** 4 条调账的 `res_sub_class` 均为 `H100`

#### Scenario: 请求中的类别字段被忽略

- **GIVEN** 请求体在分摊明细里额外携带 `res_class` / `res_sub_class`
- **WHEN** 解码请求
- **THEN** 这两个字段不参与落库，落库取值仍为 `gpu_card` 与该单的 `gpu_type`

### Requirement: 错误码语义

系统 SHALL 在契约中给出错误码语义表并在实现中遵循：`errf.InvalidParameter`（2000001）用于 R-015 任一硬校验不通过、分摊合计 ≠ 总价、批内唯一键重复；`errf.RecordNotFound`（2000003）用于云账号映射不到唯一二级账号；`errf.Aborted`（2000006）用于主表 `settle_state=settled` 或关联调账 `push_status=pushing` 拒绝写入/删除；`errf.RecordDuplicated`（2000011）用于同一 `(uuid, 订单月)` 并发重推的唯一键冲突。权限错误 MUST NOT 泄露账号存在性以外的信息。逐单失败时错误码语义体现在该单 `message` 文本中，接口整体仍返回成功。

#### Scenario: 错误码语义表与触发场景一一对应

- **GIVEN** 契约文档已产出
- **WHEN** 检查错误码语义表
- **THEN** `InvalidParameter` / `RecordNotFound` / `Aborted` / `RecordDuplicated` 四类错误码各自的触发场景均已列明
- **AND** 调用方可依据错误码自行决定是否重推

### Requirement: APIGW 资源注册与接口文档

系统 SHALL 在 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 注册 `POST /api/v1/account/bills/prepaid_items/sync` 资源，超时配置 MUST 为 30s。资源 MUST 启用 `X-Bkapi-JWT` 校验，由现成中间件 `gwparser.Parse` 与 api-server REST filter 承接。HCM MUST NOT 自建 AppCode 白名单，调用方身份由 APIGW JWT 保证。接口文档 MUST 置于 `docs/api-docs/web-server/docs/resource/bill/`，版本标注 `v9.9.9+`。

#### Scenario: APIGW 资源已注册且超时为 30s

- **GIVEN** APIGW 资源注册已完成
- **WHEN** 查看 `bk_apigw_resources_bk-hcm.yaml`
- **THEN** 存在 sync 资源条目且超时配置为 30s
- **AND** 接口文档存在于 `docs/api-docs/web-server/docs/resource/bill/` 且版本标注为 `v9.9.9+`

#### Scenario: 无合法 JWT 的请求不到达 account-server

- **GIVEN** 请求未携带合法 `X-Bkapi-JWT`
- **WHEN** 调用 sync 接口
- **THEN** APIGW 或 api-server 层拒绝该请求
- **AND** 请求不到达 account-server
