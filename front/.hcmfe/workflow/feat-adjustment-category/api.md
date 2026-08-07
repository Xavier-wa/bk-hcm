# API：三方云核算-调账类别调整-前端

## 1. API 阶段范围

本阶段只定义前端依赖的 HTTP 契约和前端序列化规则，不修改后端代码。后端能力由兄弟需求
`1069995598136722867` 提供；前端按已确认路径先行开发，后端接口就位后联调。

关联页面：

- 新增/编辑调账表单：`front/src/views/bill/bill/adjust/create/`
- 调账列表：`front/src/views/bill/bill/adjust/index.tsx`
- 现有账单 API：`front/src/api/bill/index.ts`
- 现有账单 Store：`front/src/store/useBillStore.ts`

## 2. 数据模型契约

### 2.1 资源类别

前端枚举和提交值统一使用后端四值：

| 展示名称 | `res_class` | 资源子类语义 |
|----------|-------------|--------------|
| CPU | `cpu` | 无，提交空字符串 |
| GPU卡 | `gpu_card` | 卡型字符串 |
| GPU API | `gpu_api` | 模型厂商字符串 |
| GPU其他 | `gpu_other` | 无，提交空字符串 |

旧值 `gpu` 不再作为创建/更新请求值；存量数据由后端刷新为 `gpu_card`。前端不再生成旧值。

### 2.2 调账明细

在现有 `AdjustmentItem` 读写模型上增加：

```ts
res_sub_class: string;
```

该字段是后端单字段，含义由同一条记录的 `res_class` 决定：

- 表单层可以维护两个互斥的视图字段：卡型选择值、模型厂商选择值；
- 请求序列化时只能生成一个 `res_sub_class`；
- 以最终 `res_class` 为准，仅序列化当前类别对应的视图字段；
- 不得将已隐藏类别的旧字段一并提交。

### 2.3 前端提交归一化

```text
res_class = cpu       → res_sub_class = ""
res_class = gpu_other → res_sub_class = ""
res_class = gpu_card  → res_sub_class = 当前卡型值，无值时 ""
res_class = gpu_api    → res_sub_class = 当前模型厂商值，无值时 ""
```

资源子类在前端不增加必填校验；所有请求必须显式带 `res_sub_class` 字符串，不能传
`undefined` 或省略字段。后端最终校验规则以兄弟需求的实现为准，联调时重点验证支持厂商和空候选
场景的错误反馈。

## 3. 新增资源子类候选接口

### 3.1 查询 GPU 卡型

```http
GET /api/v1/account/vendors/{vendor}/bills/adjustment_items/gpu_cards
```

路径参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `vendor` | `VendorEnum` | 是 | 云厂商，如 `aws`、`gcp`、`huawei` |

成功响应沿用项目统一响应包络：

```json
{
  "code": 0,
  "message": "",
  "data": ["A100", "H100", "H200"]
}
```

`data` 类型为 `string[]`，前端直接作为资源子类下拉候选，不做文案转换或二次拼接。

### 3.2 查询模型厂商

```http
GET /api/v1/account/vendors/{vendor}/bills/adjustment_items/api_brands
```

路径参数与响应格式同卡型接口：

```json
{
  "code": 0,
  "message": "",
  "data": ["claude", "gemini", "jina", "kimi"]
}
```

`data` 类型为 `string[]`，前端直接作为 GPU API 的资源子类候选。

### 3.3 请求条件与空数据

- 只有 `vendor ∈ { aws, gcp, huawei }` 时允许请求上述两个接口；
- 其他云厂商不请求接口，前端直接将候选列表视为空数组 `[]`；
- 资源类别为 `cpu` 或 `gpu_other` 时不请求接口，候选列表为 `[]`；
- `huawei` 仍属于允许请求的三种云厂商，候选结果以接口返回为准；接口返回空数组时前端展示空下拉；
- 用户切换云厂商或资源类别时，清空已不适用的资源子类值；请求响应只更新当前类别、当前云厂商对应的候选列表；
- 必须避免旧请求晚于新请求返回时覆盖当前选择，可使用现有请求封装的取消/序列号机制，或在响应落地前校验当前 `vendor + res_class`。

前端 API 层建议提供两个语义化方法：

```ts
reqBillsAdjustmentGpuCards(vendor: VendorEnum): Promise<string[]>;
reqBillsAdjustmentApiBrands(vendor: VendorEnum): Promise<string[]>;
```

方法内部负责统一解包 `data`；接口错误继续交由现有 HTTP 错误处理链路处理，不把错误响应转换成候选值。

## 4. 现有调账接口字段调整

以下接口路径保持现有页面契约不变，仅在请求/响应类型中增加 `res_sub_class`，并更新
`res_class` 四值描述：

### 4.1 创建

```http
POST /api/v1/account/bills/adjustment_items/create
```

请求 `items[]` 中增加：

```json
{
  "res_class": "gpu_card",
  "res_sub_class": "H200"
}
```

对于 `cpu`、`gpu_other` 或没有可选子类的场景，必须发送：

```json
{
  "res_class": "gpu_other",
  "res_sub_class": ""
}
```

### 4.2 更新

```http
PATCH /api/v1/account/bills/adjustment_items/{id}
```

更新请求中增加可选字段：

```ts
res_class?: ResClassEnum;
res_sub_class?: string;
```

前端实际提交时仍必须按第 2.3 节显式生成 `res_sub_class`，特别是从 `gpu_card` / `gpu_api`
切换到 `cpu` / `gpu_other` 时显式传空字符串。

### 4.3 列表

```http
POST /api/v1/account/bills/adjustment_items/list
```

列表数据每条记录增加：

```json
{
  "res_class": "gpu_api",
  "res_sub_class": "gemini"
}
```

空值直接返回空字符串；前端列表列渲染时将空字符串显示为 `--`。

## 5. 前端调用与缓存边界

### 5.1 调用时机

- 新增行默认 `res_class = cpu`，不触发子类接口请求；
- 切换为 `gpu_card` 后，在支持厂商下请求 `gpu_cards`；
- 切换为 `gpu_api` 后，在支持厂商下请求 `api_brands`；
- 云厂商变化时，清空所有行的 `res_sub_class`，再按各行当前类别重新请求所需候选；
- 编辑表单回填时，先按记录的 `vendor + res_class` 获取候选，再回填对应资源子类；CPU/GPU其他不请求。

### 5.2 缓存

可按 `{ vendor, res_class }` 缓存候选列表：

- `{ aws, gpu_card }` → `gpu_cards`；
- `{ gcp, gpu_card }` → `gpu_cards`；
- `{ huawei, gpu_card }` → `gpu_cards`；
- `{ aws, gpu_api }` → `api_brands`；
- `{ gcp, gpu_api }` → `api_brands`；
- `{ huawei, gpu_api }` → `api_brands`；
- 非支持厂商、CPU、GPU其他不产生网络请求。

缓存只用于候选展示，不能绕过提交时的 `res_sub_class` 归一化。

## 6. 错误与兼容性

- 接口返回非 0 `code` 时，沿用现有 HTTP 错误提示，不写入错误内容到下拉列表；
- 支持厂商接口返回空数组属于正常空态，不显示错误提示；
- 非支持厂商不得调用候选接口，避免后端返回参数错误；
- `res_sub_class` 为空时列表展示 `--`；
- 不新增权限点，沿用账单管理权限；
- 本阶段不修改后端 API 文档源文件，不修改 Go 代码。

## 7. 后端联调核对清单

| 检查项 | 预期 |
|--------|------|
| 两个候选接口路径 | 与第 3 节完全一致 |
| 响应包络 | `code`、`message`、`data: string[]` |
| `vendor` 支持范围 | `aws`、`gcp`、`huawei` 可请求，其余不请求 |
| huawei 候选结果 | 以接口实际返回为准；允许空数组 |
| 创建/更新字段 | 接收 `res_sub_class`，列表返回同名字段 |
| CPU/GPU其他 | 请求显式携带 `res_sub_class: ""` |
| 类别切换 | 隐藏字段旧值不会残留到请求 |
| 列表空值 | 前端将空字符串渲染为 `--` |

## 8. 已知契约风险

后端兄弟需求 `1069995598136722867` 的当前描述包含一条与本前端已确认 PRD 不一致的规则：后端文档写明
华为的卡型/模型厂商清单为空，并推导 `huawei` 只能使用 `cpu` / `gpu_other`，而当前前端 PRD 已确认
三种已适配厂商为 `aws`、`gcp`、`huawei`，四个资源类别对所有厂商展示，资源子类是否可用以接口返回为准。

本 API 文档遵循前端已确认口径：

- `huawei` 可请求两个候选接口；
- 接口返回空数组时前端呈现空下拉；
- 前端不额外隐藏 `gpu_card` / `gpu_api`，也不增加资源子类必填校验；
- 需要在后端接口就位后联调确认：华为选择 `gpu_card` / `gpu_api` 且子类为空时，后端是否允许提交，或是否需要重新确认 PRD。

该风险不阻塞前端 API 封装，但会作为 Coding/Test 阶段的联调验收项，不应在前端静默改成另一套规则。
