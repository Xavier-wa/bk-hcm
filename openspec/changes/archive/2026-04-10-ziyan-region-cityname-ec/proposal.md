## Why

自研云（Ziyan）在同步地域信息时，从云上 DescribeRegions 接口返回的 RegionName（如"华中地区(武汉)"）中提取的 city_name（如"武汉"）不包含 EC 后缀。然而 CRP（资源平台）系统中使用的 city_name 是完整的包含 EC 后缀的形式（如"武汉EC"）。这导致在需要将 HCM 的 city_name 与 CRP 数据进行匹配的场景下，匹配失败，影响业务功能的正常运行。

## What Changes

1. **新增 CRP 接口调用封装**：封装 `queryZoneCityList` 接口调用，通过 region 查询获取完整的 city_name（含 EC 后缀）
2. **修改地域同步逻辑**：在 `cmd/hc-service/logics/res-sync/ziyan/region.go` 中，同步地域时调用 CRP 接口获取完整 city_name，替换从 RegionName 提取的不完整 city_name
3. **配置扩展**：新增 CRP 接口配置项（测试环境/正式环境地址、api_key）

## Capabilities

### New Capabilities

- `ziyan-region-cityname-sync`: 自研云地域同步时通过 CRP 接口补全 city_name（含 EC 后缀）的能力

### Modified Capabilities

- 无（本变更为纯新增能力，不涉及现有 spec 的修改）

## Impact

- **代码影响**：
  - `cmd/hc-service/logics/res-sync/ziyan/region.go` - 修改地域同步逻辑，集成 CRP 接口调用
  - `pkg/thirdparty/cvmapi/` 或新增包 - CRP `queryZoneCityList` 接口封装
  - 配置文件 - 新增 CRP 接口相关配置项

- **依赖影响**：
  - 新增对 CRP 外部接口的依赖
  - 地域同步流程增加一次外部 HTTP 调用（可缓存优化）

- **数据影响**：
  - 新增/更新的地域数据 city_name 字段将包含 EC 后缀
  - 历史数据可选择性迁移（通过重新触发同步）

- **兼容性**：
  - 非 BREAKING CHANGE，仅为数据完整性修复
  - 下游系统若已适配 EC 后缀，将获得正确的匹配结果
