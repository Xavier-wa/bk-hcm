# feat-cvm-private-image 测试清单

> 工作流: `feat-cvm-private-image`
> 关联文档: PRD / Design / API / Coding
> 负责人: <开发自测人 / QA>

## 验证范围

- 功能边界: `cvm-image-selector.vue` 新增 `bizId` prop + API 替换为按业务 ID 查询接口，3 个调用场景（主机申领 / CVM生产 / 主机库存匹配面板）均需验证
- 不在范围: 多厂商镜像选择器 (`image-selector.tsx`)、表单其他字段、提交流程

## 测试环境

- 前端入口: <自研云主机申领页 / CVM生产页 / 主机库存匹配面板 的测试环境地址>
- 测试账号: <test_account>（需准备 target 业务账号 + 非 target 业务账号）
- 数据准备:
  - **目标业务账号**：bizId 有值且后端为该业务配置了私有镜像
  - **非 target 业务账号**：用于回归对比
  - 异常分支：通过 DevTools Network mock 接口返回 500 / 超时

## 用例清单

### P0 - 主流程（必测，覆盖 3 个场景）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P0-01 | 主机申领-目标业务看到私有镜像 | 用 target 业务账号登录主机申领页，已选地域 | 展开镜像选择器 | 列表中包含 PUBLIC_IMAGE + PRIVATE_IMAGE 类型项 |
| P0-02 | 主机申领-非 target 业务无变化 | 用 non-target 业务账号登录主机申领页，已选地域 | 展开镜像选择器 | 与改造前一致，仅有公共镜像 |
| P0-03 | CVM生产单据-目标业务看到私有镜像 | 打开 CVM 生产页面（formModel.bk_biz_id 为 target 值），已选地域 | 展开镜像选择器 | 列表中包含 PUBLIC_IMAGE + PRIVATE_IMAGE |
| P0-04 | CVM生产单据-切换业务后镜像更新 | P0-03 通过 | 修改 formModel.bk_biz_id 为非 target 值 → 重新展开镜像选择器 | 镜像列表变为仅公共镜像 |
| P0-05 | 主机库存匹配面板-目标业务 | 进入主机库存匹配面板（路由带 bizs query），已选地域 | 展开"操作系统"镜像选择器 | 包含私有镜像选项 |
| P0-06 | 选择私有镜像并提交 | P0-01 或 P0-03 通过 | 选择 PRIVATE_IMAGE → 填写其他必填 → 提交 | 提交成功，image_id 正确回传 |

### P1 - 异常与降级

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | 新接口 500 降级 | 任一场景，DevTools 拦截新接口返回 500 | 展开镜像选择器 | 不崩溃、不白屏；下拉列表为空或 fallback 到旧逻辑 |
| P1-02 | bizId 为空走旧接口 | 使用无 bizId 上下文的环境打开页面 | 观察 Network 面板请求 | 发出旧接口请求 `/api/v1/woa/config/findmany/...` |
| P1-03 | type 标签正确展示 | P0-01 通过 | 观察镜像选项渲染 | 私有镜像旁显示 `<bk-tag>` 显示类型值（如 PRIVATE_IMAGE） |
| P1-04 | 不传 bizId 时向后兼容 | common-resource 场景（未传 bizId prop） | 展开镜像选择器 | 行为与改造前一致（内部 getBizsId() 取值 + 兜底到旧接口） |
| P1-05 | 地域联动含私有镜像 | P0-01 通过 | 切换地域 → 重新展开镜像选择器 | 私有镜像随地域正确过滤 |

### P2 - 回归与细节

- [ ] clearable / filterable / disabled 等原有功能不受影响
- [ ] 选择镜像后名称和 ID 回填正常
- [ ] isEqual 防抖仍生效（窗口 resize 不重复触发请求）
- [ ] 主机申领页切换 BusinessSelector 后镜像列表刷新（computedBiz 变化触发）

## 验证结论

> 每条用例填写：PASS / FAIL / Skipped + 简要说明

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | PASS | |
| P0-02 | PASS | |
| P0-03 | PASS | |
| P0-04 | PASS | |
| P0-05 | PASS | |
| P0-06 | PASS | |
| P1-01 | PASS | |
| P1-02 | PASS | |
| P1-03 | PASS | |
| P1-04 | PASS | |
| P1-05 | PASS | |

- 执行人: QA
- 执行日期: 2026-06-09
- 总体结论: PASS
- 后续行动: 合入代码
