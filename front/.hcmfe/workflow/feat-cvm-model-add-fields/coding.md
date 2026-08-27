# Coding — feat-cvm-model-add-fields

## 执行顺序
1. API 层：新增 GPU 卡类型枚举接口函数
2. 类型层：`ICvmDeviceCreateModel` 增加 `gpu_type`、`tech_class_res_amt` 字段
3. 表单层：`CreateDevice/index.tsx` 新增两个表单字段 + 提交组装

> 排序依据：依赖（先 API 后 UI）/ 同文件聚合

## 共享改动 / 提交策略
- 单需求单提交，commit 信息 `feat: CVM机型-机型追加页面增加两个新字段 --story=137202147`
- 提交范围 = 业务代码 + workflow 产物

---

## 单据 1: CVM机型-机型追加页面增加两个新字段
**TAPD**: [#1069995598137202147](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137202147)
**文件**: `src/api/scrApi/index.tsx` `src/views/ziyanScr/cvm-model/CreateDevice/index.tsx`
**改动点**:
- 新增 GPU 卡类型枚举接口 `GET /api/v1/woa/meta/gpu_type/list`（返回 `data.details` 字符串数组）
- `ICvmDeviceCreateModel` 增加 `gpu_type?: string`、`tech_class_res_amt?: number`
- 表单新增「GPU卡类型」字段：`hcm-form-enum` + `allowCreate`（默认枚举 + 允许自定义输入），枚举来自接口
- 表单新增「技术分类资源量」字段：`hcm-form-number`（>=0）
- 提交组装 `deviceTypes.push` 增加 `gpu_type`、`tech_class_res_amt`

### 根因 / 需求
CVM 机型追加页面需要支持 GPU 机型，新增两个字段：GPU卡类型（有默认枚举但允许自定义）、技术分类资源量。

### 验收
- 打开「创建新机型」弹窗，可见「GPU卡类型」「技术分类资源量」两个新字段
- GPU卡类型下拉展示接口返回的枚举，且允许手动输入自定义值
- 提交时 device_types 携带 gpu_type、tech_class_res_amt 字段
