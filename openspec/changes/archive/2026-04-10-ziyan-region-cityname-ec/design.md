## Context

### 背景

自研云（Ziyan）是 HCM 支持的云厂商之一，其地域（Region）和可用区（Zone）信息通过腾讯云 SDK 的 DescribeRegions 接口同步。当前地域同步逻辑位于 `cmd/hc-service/logics/res-sync/ziyan/region.go`。

### 当前问题

1. **city_name 不完整**：云上 DescribeRegions 接口返回的 RegionName 格式为"华中地区(武汉)"，从中提取的 city_name 为"武汉"，不包含 EC 后缀
2. **与 CRP 数据不匹配**：CRP（资源平台）系统中使用的 city_name 是完整形式（如"武汉EC"），导致匹配失败
3. **影响范围**：可用区同步 `zone.go` 通过 `getCityNameFromRegion()` 从 DB 查询 region 获取 city_name，问题会传导到可用区数据

### CRP 接口能力

CRP 提供 `queryZoneCityList` 接口，可查询所有可用区与城市映射关系，返回数据包含完整的 cityName（含 EC 后缀）。

**接口协议**：JSON-RPC 2.0，复用现有 `CVMClientInterface` 的 HTTP 客户端。

**接口地址**：
- 测试环境：`xxx/yunti-demand/external?api_key=no`
- 正式环境：`xxx/yunti-demand/external?api_key=no`

> 注意：该接口地址与现有 CVM API 客户端的 `CvmApiAddr` 不同，但协议一致（JSON-RPC 2.0）。由于 `CVMClientInterface` 通过 `rest.ClientInterface` 发送请求，`QueryZoneCityList` 方法需使用独立的 `SubResourcef` 路径 `/yunti-demand/external`，而非复用现有 CVM 子路径。

**请求**：
```json
{
    "jsonrpc": "2.0",
    "id": "0",
    "method": "queryZoneCityList",
    "params": {}
}
```

**响应**：
```json
{
    "jsonrpc": "2.0",
    "id": "0",
    "x_trace_id": "a9b5fb2d430535d5",
    "result": [
        {
            "ciyId": 28,
            "cityName": "上海",
            "region": "ap-shanghai",
            "areaName": "华东地区",
            "zone": "ap-shanghai-4",
            "zoneId": 200004,
            "zoneName": "上海四区",
            "defaultCampus": "上海-富特",
            "country": "中国内地",
            "customhouseTitle": "境内",
            "regionName": "华东地区"
        }
    ]
}
```

**重要特性**：
- 该接口 **不支持按 region 过滤查询**，每次请求均返回全量数据
- result 数组中可能包含多条同一 region 的记录（不同 zone），需去重后构建 `region -> cityName` 映射

### 现有基础设施

- `pkg/thirdparty/cvmapi/` - CVM API 客户端封装，已具备调用 CRP 接口的基础能力
- `cmd/hc-service/logics/res-sync/ziyan/region.go` - 地域同步主逻辑
- `cmd/hc-service/logics/res-sync/ziyan/zone.go` - 可用区同步逻辑，依赖 region 的 city_name

## Goals / Non-Goals

**Goals:**

1. 地域同步时通过 CRP 接口获取包含 EC 后缀的完整 city_name
2. 复用现有 `pkg/thirdparty/cvmapi` 基础设施，避免重复建设
3. 保持向后兼容，接口调用失败时回退到原有逻辑
4. 最小化代码改动范围，专注解决 city_name 不完整问题

**Non-Goals:**

1. 不修改 zone 同步逻辑（zone 仍从 region 获取 city_name）
2. 不修改 area_name 提取逻辑
3. 不新增独立配置管理（复用 CVM API 配置）
4. 不做历史数据批量迁移（通过重新同步自然更新）

## Decisions

### Decision 1: 在 cvmapi 包中封装 queryZoneCityList 接口

**方案**：在 `pkg/thirdparty/cvmapi/cvmapi.go` 中新增 `QueryZoneCityList` 方法

**接口路径**：`/yunti-demand/external`

**请求结构体**（添加到 `cvmapi_request.go`）：
```go
// QueryZoneCityListReq 查询可用区与城市映射请求
type QueryZoneCityListReq struct {
    ReqMeta `json:",inline"`
    Params  *QueryZoneCityListParams `json:"params"`
}

// QueryZoneCityListParams 查询可用区与城市映射参数（空即可，接口返回全量数据）
type QueryZoneCityListParams struct{}
```

**响应结构体**（添加到 `cvmapi_response.go`）：
```go
// QueryZoneCityListResp 查询可用区与城市映射响应
type QueryZoneCityListResp struct {
    RespMeta `json:",inline"`
    Result   []ZoneCityInfo `json:"result"`
}

// ZoneCityInfo CRP 可用区与城市映射信息
type ZoneCityInfo struct {
    CiyId            int    `json:"ciyId"`
    CityName         string `json:"cityName"`
    Region           string `json:"region"`
    AreaName         string `json:"areaName"`
    Zone             string `json:"zone"`
    ZoneId           int    `json:"zoneId"`
    ZoneName         string `json:"zoneName"`
    DefaultCampus    string `json:"defaultCampus"`
    Country          string `json:"country"`
    CustomhouseTitle string `json:"customhouseTitle"`
    RegionName       string `json:"regionName"`
}
```

**实现方法**（添加到 `cvmapi.go`）：
```go
func (c *cvmApi) QueryZoneCityList(ctx context.Context, header http.Header) (*QueryZoneCityListResp, error) {
    resp := new(QueryZoneCityListResp)
    err := c.client.Post().
        WithContext(ctx).
        Body(NewQueryZoneCityListReq(&QueryZoneCityListParams{})).
        SubResourcef("/yunti-demand/external").
        WithParam(CvmApiKey, CvmApiKeyVal).
        WithHeaders(header).
        Do().Into(resp)
    if err != nil {
        return nil, err
    }
    if resp.Error.Code != 0 {
        return nil, fmt.Errorf("query zone city list failed, code: %d, message: %s",
            resp.Error.Code, resp.Error.Message)
    }
    return resp, nil
}
```

**理由**：
- 复用现有 CVM API 客户端的连接管理和 JSON-RPC 协议
- 遵循项目既有模式（`ReqMeta` + `Params` 请求结构，`RespMeta` + `Result` 响应结构）
- 与 `QueryMatchTask`、`MatchSwapGroup` 等 CRP 接口封装保持一致
- 无需新增配置文件和客户端初始化代码

**替代方案**：
- 新建独立的 CRP 客户端包 → **否决**：增加维护成本，违背复用原则

### Decision 2: 在 region 同步流程中集成 CRP 调用（一次性全量获取 + map 查询）

**方案**：在 `Region()` 方法入口处一次性调用 CRP 接口获取全量 region-city 映射，构建 `map[region]cityName`，后续 `createRegion`、`updateRegion`、`isRegionChange` 均从该 map 中查询。

**调用流程**：
```go
func (cli *client) Region(kt *kit.Kit, opt *SyncRegionOption) (*SyncResult, error) {
    // ... 省略前置校验 ...

    // 新增：一次性获取 CRP 全量 region-city 映射
    regionCityMap, err := cli.getRegionCityMapFromCRP(kt)
    if err != nil {
        logs.Warnf("get region-city map from CRP failed, err: %v, fallback to extract, rid: %s", err, kt.Rid)
    }

    // 原有 diff 逻辑，传入 regionCityMap
    addSlice, updateMap, delCloudIDs := common.Diff[typesregion.TCloudRegion, cloudcore.TCloudZiyanRegion](
        regionFromCloud, regionFromDB, func(cloud typesregion.TCloudRegion, db cloudcore.TCloudZiyanRegion) bool {
            return isRegionChange(cloud, db, regionCityMap)
        })

    // createRegion / updateRegion 也传入 regionCityMap
    if len(addSlice) > 0 {
        if err = cli.createRegion(kt, opt, addSlice, regionCityMap); err != nil { ... }
    }
    if len(updateMap) > 0 {
        if err = cli.updateRegion(kt, opt, updateMap, regionCityMap); err != nil { ... }
    }
}
```

**`getRegionCityMapFromCRP` 方法设计**：
```go
// getRegionCityMapFromCRP 调用 CRP queryZoneCityList 接口，构建 region -> cityName 映射
// 返回 map[regionID]cityName，同一个 region 可能有多条记录（不同 zone），取第一条的 cityName
func (cli *client) getRegionCityMapFromCRP(kt *kit.Kit) (map[string]string, error) {
    resp, err := cli.crpCli.QueryZoneCityList(kt.Ctx, kt.Rid())
    if err != nil {
        return nil, err
    }
    regionCityMap := make(map[string]string)
    for _, item := range resp.Result {
        if _, exists := regionCityMap[item.Region]; !exists && item.CityName != "" {
            regionCityMap[item.Region] = item.CityName
        }
    }
    return regionCityMap, nil
}
```

**`createRegion` / `updateRegion` 中的使用**：
```go
areaName, cityName := extractAreaAndCityName(one.RegionName)
// 优先使用 CRP 返回的完整 city_name
if crpCityName, ok := regionCityMap[one.RegionID]; ok {
    cityName = crpCityName
}
```

**`isRegionChange` 中的使用**：
```go
func isRegionChange(cloud typesregion.TCloudRegion, db cloudcore.TCloudZiyanRegion,
    regionCityMap map[string]string) bool {
    // ... regionID / regionName 对比不变 ...

    areaName, cityName := extractAreaAndCityName(cloud.RegionName)
    // 优先使用 CRP 返回的完整 city_name 进行对比
    if crpCityName, ok := regionCityMap[cloud.RegionID]; ok {
        cityName = crpCityName
    }
    if cityName != db.CityName {
        return true
    }
    // ...
}
```

**理由**：
- CRP 接口不支持按 region 过滤，每次调用必返回全量数据
- 在 `Region()` 入口处一次性获取，构建 map 后传递给所有子函数，避免 N 次 HTTP 调用
- `isRegionChange` 也使用 CRP 数据对比，避免因 city_name 来源不同导致每次同步都触发无意义的 update
- 接口调用失败时 map 为 nil，所有函数自动回退到 `extractAreaAndCityName`，降级逻辑自然

**替代方案**：
- 在数据模型层做转换 → **否决**：耦合度过高，不利于后续维护
- 每个子函数各自调用 CRP → **否决**：CRP 接口不支持按 region 过滤，逐个调用无意义且浪费

### Decision 3: CRP 接口调用策略

**方案**：每次 `Region()` 同步调用一次 CRP 接口，不做缓存

**理由**：
- CRP 接口每次返回全量数据，无法增量查询
- 地域同步频率较低（通常手动触发或定时任务），单次全量请求开销可接受
- V1 保持简单实现，避免缓存一致性复杂度
- `getRegionCityMapFromCRP` 封装在 region.go 内部，后续如需缓存可在此方法内添加

**后续优化**：
- 如性能成为瓶颈，可在 `getRegionCityMapFromCRP` 内添加短期内存缓存（如 5 分钟 TTL）

### Decision 4: CRP 客户端依赖传递链设计（复用方案）

**方案**：`ressync.NewClient` 添加 `crpCli` 参数，service 层统一创建后复用

**调用链改造**：

```
service.go:191
└── ressync.NewClient(ad, dataCli, crpCli)     // 唯一修改点，新增 crpCli
         │
         ├──> sync.InitService(cap)            // cap.ResSyncCli 已有 crpCli ✓
         │         └── TCloudZiyan() ──> ziyan.NewClient(..., crpCli)
         │
         ├──> tag.InitTagService(cap)          // 复用 cap.ResSyncCli（无需 crpCli）
         │                                         // 可选重构：改为复用
         └──> vpc.InitVpcService(cap)            // 复用 cap.ResSyncCli（无需 crpCli）
                                                   // 可选重构：改为复用
```

**涉及文件修改**：
1. `cmd/hc-service/service/service.go` - 传递 `crpCli` 给 `ressync.NewClient`
2. `cmd/hc-service/logics/res-sync/client.go` - 添加 `crpCli` 字段，修改 `NewClient` 和 `TCloudZiyan`
3. `cmd/hc-service/logics/res-sync/ziyan/client.go` - 添加 `crpCli` 字段，修改 `NewClient`
4. `cmd/hc-service/logics/res-sync/ziyan/region.go` - 添加 `getRegionCityMapFromCRP` 方法，修改 `Region`/`createRegion`/`updateRegion`/`isRegionChange`

**不复用 tag/vpc 的 syncCli 的原因**：
- tag 和 vpc 当前独立创建 `syncCli` 可能是历史遗留或保持显式依赖
- 它们主要调用 `TCloud()` 而非 `TCloudZiyan()`，不需要 `crpCli`
- 复用 `cap.ResSyncCli` 是可选优化，可后续单独重构

**理由**：
- 改动范围最小：只需修改一处创建点
- tag/vpc 虽独立创建但配置相同，传递 `crpCli` 对它们无害（不使用即可）
- 保持依赖注入设计原则，便于单元测试和 Mock

## Risks / Trade-offs

### Risk 1: CRP 接口可用性影响地域同步

- **风险**：CRP 接口故障可能导致地域同步延迟或失败
- **缓解措施**：
  - 接口调用失败时记录错误日志并回退到原有逻辑
  - 地域同步流程继续执行，不因 CRP 调用失败而中断
  - 监控 CRP 接口调用成功率和延迟

### Risk 2: CRP 接口数据与云上数据不一致

- **风险**：CRP 中的 region 定义可能与云上 DescribeRegions 不一致
- **缓解措施**：
  - 根据 region ID 匹配，如未找到则回退到原有提取逻辑
  - 记录警告日志，便于发现问题

### Risk 3: 新增外部依赖增加系统复杂度

- **风险**：地域同步依赖 CRP 服务，增加系统耦合
- **缓解措施**：
  - 依赖关系为弱依赖（失败可降级）
  - 复用现有 CVM API 基础设施，不增加新配置项

## Migration Plan

### 部署步骤

1. **代码部署**：合并代码后正常滚动更新 `hc-service`
2. **数据更新**：
   - 手动触发一次地域同步任务
   - 或等待下次定时同步任务执行
3. **验证**：
   - 检查数据库中 region 表的 city_name 字段是否包含 EC 后缀
   - 检查 zone 表的 city_name 是否同步更新

### 回滚策略

- **回滚方式**：代码回滚到上一版本
- **数据恢复**：如需要恢复历史数据，可从备份恢复或重新触发同步
- **影响评估**：回滚后新增/更新的地域数据 city_name 将恢复为不含 EC 后缀的形式
