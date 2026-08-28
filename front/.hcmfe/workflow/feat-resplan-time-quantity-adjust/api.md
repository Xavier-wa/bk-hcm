# API：资源预测-时间和数量同时调整（前端实现）

> 阶段目标：冻结前端提交 / 查询契约。本工作流**不改后端**。  
> 范围对齐 design：做 F-001 / F-002 / F-004；**F-003 新日期选择组件不做**（时间字段仍走现有 DatePicker + 既有时间相关接口）。
> 契约唯一依据：工蜂 MR `bcc/hcm!3290` 中的 `adjust_biz_resource_plan_demand.md`（取代此前的 !3267，后者不再参考）。
> !3290 为**纯文档 MR** 且尚未合并，对应后端实现需另行部署——联调前置见 §2.7。
> 本仓库的后端 Go 代码未同步最新实现，不作为契约依据。

## 1. 接口清单

| 用途 | Method | Path | 本需求关系 |
|---|---|---|---|
| 批量调整预测 | POST | `/api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/adjust` | **唯一主提交**（F-001 / F-002 / F-004） |
| 列表拉取待调整行 | POST | `/api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/list` | 进入调整页初始数据 |
| 期望时间可用周/月范围 | POST | `/api/v1/woa/plans/demands/available_times/get` | 改时间校验辅助（沿用，非 F-003） |
| 预测校验 | POST | `/api/v1/woa/plans/resources/demands/verify` | 可选；机型/数量变更前后校验沿用 |

权限：业务-资源预测操作（与现网调整一致，AC-S01）。

## 2. 主接口：`demands/adjust`

### 2.1 请求体（按 MR !3290 冻结）

```ts
interface AdjustReq {
  /** 预测需求类型；与新建枚举一致（如 CVM / CA）。`adjusts` 全为 add（无 demand_id）时必填 */
  demand_class?: string;
  adjusts: AdjustItem[]; // 1..100
}

interface AdjustItem {
  demand_id?: string;                    // update/delay 必填；add 不传
  adjust_type: 'update' | 'delay' | 'add';
  demand_source?: string;                // update/add 必填
  original_info?: AdjustInfo;            // update 必填；delay 可不传（后端按 demand_id 从库取）；add 不传
  updated_info?: AdjustInfo;             // update/add 必填；delay 不传
  expect_time?: string;                  // 仅 delay 必填；update/add 的到货时间走 updated_info.expect_time
}

interface AdjustInfo {
  obs_project: string;
  expect_time: string;            // YYYY-MM-DD
  return_plan_time?: string;      // 短租必填
  region_id: string;
  zone_id?: string;
  demand_source?: string;
  remark?: string;
  demand_res_types: ('CVM' | 'CBS')[];
  cvm?: { res_mode: string; device_type: string; os: number; cpu_core: number; memory: number };
  cbs?: { disk_type: string; disk_io: number; disk_size: number };
}
```

响应：`{ code, message, data: { id } }`，`data.id` 为预测单据（调整单）ID。

接口依据：[MR !3290](https://<GIT_HOST>/bcc/hcm/-/merge_requests/3290) 的
`docs/api-docs/web-server/docs/biz/scr/resource-plan/adjust_biz_resource_plan_demand.md`。

### 2.2 提交语义矩阵（F-001 核心）

前端**按行 diff 决定 `adjust_type`**，不再由 UI 二选一 Radio 决定。同一请求的 `adjusts[]` 可混装多行、多种语义，后端聚合成一张调整单。

| 行变更 | `adjust_type` | 关键字段 | 业务语义（对齐 PRD） |
|---|---|---|---|
| 仅配置/数量（机型、OS、CPU、内存、云盘等） | `update` | `original_info` / `updated_info` 配置不同；`expect_time` 前后相同 | 常规修改；可能预算追加 |
| 仅期望到货时间 | `delay` | 只传 `demand_id` + 顶层 `expect_time`；不传 `original_info` / `updated_info` | 整单延期；审批/预算行为回归（AC-007） |
| **时间 + 数量/配置同时变** | `update` | `updated_info.expect_time` ≠ `original_info.expect_time`，且配置字段有 diff；**不要**用 `delay`，**不要**拆两行 | 组合调整；数量部分按常规修改（R-003）；外部 CRP 已确认可同单（Q-009） |
| 调整单内新增 | `add` | 不传 `demand_id` / `original_info`；传 `demand_source` + 完整 `updated_info` | 与修改行混编并归属同一调整单（F-002） |
| 无变更 | — | 不入 `adjusts`（「移除未修改」后过滤） | — |

### 2.1.1 顶层 `demand_class`（F-005）

| 场景 | 前端行为 |
|---|---|
| 列表进入调整 | 勾选行的 `demand_class` 必须唯一，否则禁用批量调整 |
| 进页兜底 | 手动改 `planIds` 导致用途不一致 → 不落表、禁用提交 |
| 混编（含历史行） | 历史行原样回填且列只读；新增行继承；提交顶层带该值 |
| 纯新增（`adjusts` 全为 `add`） | 表格列按行选择；行间不一致时提交禁用；一致时顶层带该值（后端纯 add 必填） |

**组合走 `update` 而非新枚举**：!3290 描述 `update` 为「调规格/数量，或换机型且改期」，`delay` 为「仅调整期望交付时间（整单延期）」；到货时间通过 `updated_info.expect_time` 传递。同机型改期改量同样走 `update`。文档示例里 `0000001z` 就被标注为组合调整。

**禁止**：

- 同一 `demand_id` 拆成 `update` + `delay` 两条再提交（违背「一张调整单」）。
- 组合变更却发 `delay`；`delay` 表达不了改后数量/规格。
- 对 `add` 传 `demand_id` 或 `original_info`。

### 2.3 组合调整请求示例

```json
{
  "adjusts": [
    {
      "demand_id": "<DEMAND_ID>",
      "adjust_type": "update",
      "demand_source": "指标变化",
      "original_info": {
        "obs_project": "常规项目",
        "expect_time": "2026-08-01",
        "region_id": "<REGION_ID>",
        "zone_id": "<ZONE_ID>",
        "demand_source": "指标变化",
        "remark": "",
        "demand_res_types": ["CVM", "CBS"],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 10,
          "cpu_core": 80,
          "memory": 160
        },
        "cbs": {
          "disk_type": "CLOUD_PREMIUM",
          "disk_io": 15,
          "disk_size": 500
        }
      },
      "updated_info": {
        "obs_project": "常规项目",
        "expect_time": "2026-09-15",
        "region_id": "<REGION_ID>",
        "zone_id": "<ZONE_ID>",
        "demand_source": "指标变化",
        "remark": "",
        "demand_res_types": ["CVM", "CBS"],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 12,
          "cpu_core": 96,
          "memory": 192
        },
        "cbs": {
          "disk_type": "CLOUD_PREMIUM",
          "disk_io": 15,
          "disk_size": 600
        }
      }
    }
  ]
}
```

### 2.4 仅改时间示例（保持 delay）

```json
{
  "adjusts": [
    {
      "demand_id": "<DEMAND_ID>",
      "adjust_type": "delay",
      "expect_time": "2026-09-15"
    }
  ]
}
```

### 2.5 调整内新增示例

```json
{
  "adjusts": [
    {
      "adjust_type": "add",
      "demand_source": "指标变化",
      "updated_info": {
        "obs_project": "常规项目",
        "expect_time": "2026-09-15",
        "region_id": "<REGION_ID>",
        "zone_id": "<ZONE_ID>",
        "demand_res_types": ["CVM"],
        "cvm": {
          "res_mode": "按机型",
          "device_type": "S5.2XLARGE16",
          "os": 2,
          "cpu_core": 16,
          "memory": 32
        }
      }
    }
  ]
}
```

### 2.6 错误与边界（前端需处理）

| 场景 | 预期 | 前端 |
|---|---|---|
| `code != 0` | `message` 展示 | Toast / 拦提交 |
| 单次 `adjusts` > 100 | 参数错误 | 前端截断或分批前提示（优先单次 ≤100） |
| 跨年组合 | 本期不做；维持现网「调减 + 调增」 | UI/提交前拦截或沿用现网提示（R-006） |
| 仅 CBS | 不拒（待联调实测复核，见下） | 不拦提交；限制体现在「不让改资源类型」——存量行只读、新增行可选 |
| 空 `adjusts` | 无效 | 「移除未修改」后若无行，禁用提交 |

> **「仅 CBS」这一行的更正**：初稿写的是「后端拒『单独调整 CBS』，保持现网限制」，那是 API 阶段读**本仓库那份过期的 Go 代码**推出来的，!3290 正文里并没有这条限制。后来核对现网前端 `mapDetailToAdjustInfo` 发现相反的证据：纯 CBS 行的 `demand_res_types` 起手就是 `['CBS']`，走同一个 adjust 端点照常提交。现网真正的限制是**不让改资源类型**（调整态 radio 恒禁用），对应本页「存量行只读、新增行可选」。
>
> 因此前端不加拦截。**联调时用一条纯 CBS 的新增行探一次**，若后端确实拒单，再回来补提交前拦截并改回这一行。

### 2.7 联调前置

!3290 目前**状态为 opened、未合并**，且是纯文档 MR（变更只含 `adjust_biz_resource_plan_demand.md`），对应后端实现需另行部署。

对比 !3290 的 diff 可以界定，本页只有 **`add` 依赖新契约**：

| 字段 | 旧表述（diff 的 `-` 侧） | !3290 |
|---|---|---|
| `adjust_type` | 枚举值：update（常规修改）、delay（加急延期） | 增加 **add** |
| `demand_id` | 必选**是** | 改为否，「add 时不需要，其余类型必填」 |
| `original_info` / `updated_info` | 否，「adjust_type 为 update 时必填」 | 措辞细化为「delay 可不传（后端按 demand_id 从库取）」 |

即：**裸 `delay`（只发 `demand_id` + `expect_time`）在旧契约下本就合法**，!3290 只是把它写得更明确，不构成新依赖。现网旧前端 `usePlanStore.convertToAdjust` 对 `delay` 也照发 `original_info` / `updated_info`，那是它自己更保守，并非文档要求。

**不为兼容旧实现而回退**：F-002（调整单内新增）依赖 `add`，该枚举缺席时本页无论如何跑不通；`delay` 本身没有兼容问题，无需改动。

> 本仓库 `cmd/woa-server/` 下的 Go 代码尚未同步后端最新实现，**不作为契约依据**；一切以 !3290 文档为准。

## 3. 辅助接口

### 3.1 列表

- Path：`POST .../plans/resources/demands/list`
- 用途：批量/单个调整入口带入选中行；字段含 `demand_id`、`expect_time`、剩余 OS/CPU/内存/云盘等，映射为表格行 `original` 快照。

### 3.2 可用时间范围

- Path：`POST /api/v1/woa/plans/demands/available_times/get`
- Body：`{ expect_time: "YYYY-MM-DD" }`
- 用途：改时间后校验周/月范围（现有能力）；**不是** F-003 新日历组件。

### 3.3 校验（可选）

- Path：`POST /api/v1/woa/plans/resources/demands/verify`
- 机型/数量变更场景可沿用；组合调整不强制新校验契约。

## 4. F-002：调整内新增

MR !3290 已冻结契约，不再是接口缺口：

- 在同一个 `adjusts[]` 中混入 `adjust_type: "add"`。
- `add` 不传 `demand_id`、`original_info`。
- `add` 必传 `demand_source`、完整 `updated_info`。
- 与 `update` / `delay` 共用一次请求和一个响应 `data.id`，满足同一调整单提交。
- 联调前置见 §2.7：`add` 依赖新枚举与 `demand_id` 放宽，旧契约下不可用。

## 5. F-004：部分延期

- 前端：去掉列表「部分延期」入口。
- 迁移引导为纯前端文案，无新 API。

## 6. 前端类型 / Store 适配要点

| 现状 | 目标 |
|---|---|
| `AdjustType.config \| time \| none` 驱动 UI 互斥 | UI 取消互斥；`none` 仍表示未改；提交时由 diff **推导** `update` / `delay`，新增行固定 `add` |
| `convertToAdjust` 直接透传 `updatedDetail.adjustType` | 改为：时间-only → `delay`；含配置变更（含组合）→ `update`；新增 → `add` |
| 组合时只改顶层 `expect_time` | 组合必须改 `updated_info.expect_time`，且 `adjust_type=update` |
| `demand_id` / `original_info` 始终必填 | `add` 不传二者；`delay` 也不传 `original_info` / `updated_info` |

涉及：`front/src/typings/plan.ts`、`front/src/store/usePlanStore.ts`、`resource-manage/mod/**`。

## 7. 与 Design / PRD 对照

| 需求 | API 结论 |
|---|---|
| AC：同单同时改时间+数量 | 复用 adjust + `update` + `updated_info.expect_time` diff（§2.2） |
| AC-007 纯延期/纯修改回归 | 纯时间仍 `delay`；纯配置仍 `update` |
| F-003 | 无新日历 API；可用 `available_times` |
| F-002 同单新增 | `adjust_type=add`，与其他项混装（§4） |
| F-004 | 列表入口下线；无新接口 |
| 不改后端源码 | 本文件只定契约；缺口交后端任务 |

## 8. Coding 前置

1. 组合调整（含同机型改期改量）固定为 `update` + `updated_info.expect_time`。
2. F-002 固定为 `add`，与 `update` / `delay` 混装。
3. `delay` 只传 `demand_id`、`adjust_type`、`expect_time`。
4. 跨年组合按 PRD R-006 维持现状，不在本期新增处理。
5. 联调环境需部署 !3290 对应后端实现，否则 `add` 不可用（§2.7）；`delay` / `update` 不受影响。

## 9. 按 !3290 复核实现（结论：无需改码）

逐条比对 `views/resource-plan/cvm/adjust/adjust-payload.ts`：

| !3290 要求 | 实现 | 结论 |
|---|---|---|
| `add` 不传 `demand_id` / `original_info` | `convertRowToAdjust` 的 `is_new` 分支只发 `adjust_type` / `demand_source` / `updated_info` | ✓ |
| `delay` 只发 `demand_id` + `expect_time` | 仅时间变更分支即为此形态 | ✓ |
| `update` 的到货时间走 `updated_info.expect_time` | `mapRowToAdjustInfo` 内含 `expect_time`，未发顶层 | ✓ |
| `demand_source` 在 update/add 必填 | 两分支均带 `DEFAULT_DEMAND_SOURCE` 兜底 | ✓ |
| `AdjustInfo` 必填项 `obs_project` / `expect_time` / `region_id` / `demand_res_types` | 均恒发 | ✓ |
| `return_plan_time` 短租必填 | 短租项目或已有值时发出 | ✓ |
| 响应 `data.id` | store 直接透传 | ✓ |

### 9.0 联调修复：`res_mode` 两套表示（已修）

列表接口 `demands/list` 返回的是 **code**（`"res_mode": "device_type"`），而调整接口只认**中文名**（文档枚举值：按机型 / 按机型族）。原实现 `detail.res_mode || '按机型'` 的兜底在 `device_type` 为 truthy 时不触发，原样透传，后端报：

```
{"result":false,"code":2000001,"message":"unsupported res mode: device_type"}
```

现网旧调整页的做法是装载列表时硬写 `res_mode: '按机型'`（`mod/index.tsx` 的 `route.query` watch 里），把 code 直接丢弃。本页改为按 code 映射（`resolveResMode`）：`device_type → 按机型`、`device_family → 按机型族`，已是中文名的原样返回，未知值回落 `按机型`。当前数据全部落在 `按机型`，与现网等价，同时不挡后续放开机型族。

映射发生在 `demandDetailToAdjustRow`，row 与 baseline 同时归一，因此不会引入 `res_mode` 的假 diff。

### 9.1 两处与文档字面不完全贴合的点（沿用现网，暂不改）

1. **`cvm.os` 文档标 `int`，实际是小数**。联调实测列表返回 `"remained_os": "12.5"`（字符串且带小数），提交时按 `os: 12.5` 发出，后端未报该字段错误。表格对实例数只有「> 0」与「单机核数 × 实例数须为整数」两条规则（后者对齐现网），不额外加整数校验——文档的 `int` 与真实数据不符，以数据为准。
2. **无云盘的存量行仍会发 `cbs`，其中 `disk_type` 为空**。`demand_res_types` 对 CVM 行恒为 `['CVM','CBS']`，`cbs` 对象随之恒在；而 `hasNoDisk` 行的云盘四列是只读「-」，`disk_type` 取不到值。文档里 `cbs` 本身可选、但其内部 `disk_type` 标必填，属于边界不贴合。现网 `mapDetailToAdjustInfo` 同样恒发 `cbs` 且 `disk_type` 取原值（无云盘时即空串），行为一致，故不改。

> 文档瑕疵备忘：!3290 的 `0000001z` 示例声明为「同时调整到货时间与机型/数量」，但 JSON 里 `original_info.cvm` 与 `updated_info.cvm` 完全相同（`os` 均为 123），实际只有 `expect_time` / `return_plan_time` 变了。示例本身没体现数量变更，以正文与场景说明为准。
