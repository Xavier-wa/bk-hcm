## 1. 判定辅助逻辑

- [x] 1.1 在 OBS sync 包补充 AIDeduct 识别与品牌/GPU 判定输入还原 helper（AWS/GCP）
- [x] 1.2 为 helper 补充单测：API 品牌命中、GPU 无品牌、识别失败降级

## 2. AWS OBS convert

- [x] 2.1 修改 `convertAwsBill`：AIDeduct 条目按还原后的品牌/GPU 信号计算 `APIBrandName`、`isGPU`、`ResClassId`、`GpuCardCategory`
- [x] 2.2 确保成本字段仍取分账冲销金额，不因归类修正改动

## 3. GCP OBS convert

- [x] 3.1 修改 `convertGcpBill`：AIDeduct 条目按还原后的品牌/GPU 信号计算同类字段
- [x] 3.2 确保成本字段仍取分账冲销金额

## 4. 验证

- [x] 4.1 运行相关单测并通过
- [x] 4.2 勾选本 tasks 全部完成项；`npx openspec status` 确认 artifacts 齐全
