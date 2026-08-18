## Why

TAPD 136432654：CRP（云梯）侧基于安全要求下线 IP 白名单鉴权方式，要求调用方在 8 月底前统一切换为签名鉴权（api_key + api_secret）。

现状 HCM 调用 CRP 的所有接口（`pkg/thirdparty/cvmapi`）都是把 api_key 写死在代码常量 `CvmApiKeyVal` 里，以 `?api_key=<写死的 key>` 的白名单方式请求。上云后智能网关代理导致出口 IP 动态变化，白名单方式已不再受支持，且密钥写死在代码中不符合敏感信息管理要求。

签名规则参考云梯侧提供的 API 鉴权文档（iWiki 文档 4019306305，链接见 TAPD 需求单）。

## What Changes

- **api_key 配置化**：把写死的 `CvmApiKeyVal` 常量移除，改由 `woa_server.yaml` 的 `cvm` 配置块与 `hc_service.yaml` 的 `crp` 配置块提供 `api_key`、`api_secret`，启动时强校验。
- **接口鉴权方式升级为签名**：`pkg/thirdparty/cvmapi` 下全部 34 个 CRP 接口调用，请求 query 参数由单一 `api_key` 改为 `api_key` + `api_ts` + `api_sign` 三元组，`api_sign` 为 HmacSHA1(api_secret, api_ts+api_key) 的十六进制串，每次请求实时计算（云梯侧签名默认 10 分钟过期）。
- **消除残留硬编码**：`cmd/woa-server/logics/plan/fetcher/crp.go` 中把 api_key 当作业务入参 `UserName` 的用法，改为读取配置项。
- **避免密钥泄露**：客户端初始化失败时的日志不再整体打印包含 secret 的配置结构体。

## Capabilities

### New Capabilities

- `crp-api-sign-auth`: 定义 HCM 调用 CRP（云梯）接口的签名鉴权参数生成规则、密钥配置来源与启动校验行为。

### Modified Capabilities

<!-- 无已存在 spec 需要修改 -->

## Impact

- **woa-server**：`cvm` 配置块新增 `api_key`、`api_secret`，缺失则启动失败；`OldCVM`（`cvm.old_host`）与 `CVM`（`cvm.host`）共用同一组密钥。
- **hc-service**：`crp` 配置块新增 `api_key`、`api_secret`，缺失则启动失败。
- **部署配套**：`cmd/woa-server/etc/woa_server.yaml`、`cmd/hc-service/etc/hc_service.yaml` 样例需更新；Helm 侧 `hcservice`/`woaserver` configmap 都渲染 `.Values.cvm`，需在部署侧维护的各环境 values 的 `cvm` 块补齐 `api_key`、`api_secret` 后再发版（chart 默认 values 不含该配置块，保持现状）。
- **兼容性**：不保留白名单降级路径。上线前各环境配置必须先补齐密钥，否则服务启动即失败（与 tjj、xship 等既有第三方客户端配置校验行为一致）。
- **接口协议**：CRP 接口的 URL 路径、请求体、响应结构均不变，仅 query 鉴权参数变化，对上层业务逻辑无影响。
